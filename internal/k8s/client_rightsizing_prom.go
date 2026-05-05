package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// applyPrometheusStrategy is the per-container Prometheus query path
// for the right-sizing advisor. Runs one PromQL query per resource
// (cpu, memory) using the strategy's window/aggregation, parses the
// response into a name -> raw value map, and writes the
// headroom-scaled SUGGESTION cells onto the result.
//
// Failures are soft — a network/HTTP error from the proxy leaves the
// SUGGESTION cells empty (rendered as em-dashes by the overlay) so
// the user still sees Current and Usage columns. The Window field
// gets set to "1d" or "7d" depending on the strategy so the header
// can show the user what time range backs the recommendation.
func (c *Client) applyPrometheusStrategy(ctx context.Context, contextName, namespace string, pods []string, strategy model.RightsizingStrategy, headroom float64, out *model.Rightsizing) {
	if len(pods) == 0 {
		return
	}
	out.Window = promStrategyWindow(strategy)

	cpuQuery := buildPromContainerQuery(namespace, pods, strategy, "cpu")
	cpuResult, err := c.queryPromContainerMetric(ctx, contextName, cpuQuery)
	if err != nil {
		logger.Debug("rightsizing: Prometheus CPU query failed", "err", err)
	}
	memQuery := buildPromContainerQuery(namespace, pods, strategy, "memory")
	memResult, err := c.queryPromContainerMetric(ctx, contextName, memQuery)
	if err != nil {
		logger.Debug("rightsizing: Prometheus memory query failed", "err", err)
	}

	for i := range out.Containers {
		cr := &out.Containers[i]
		// CPU value is in cores; convert to millicores.
		if cores, ok := cpuResult[cr.Name]; ok && cores > 0 {
			milli := int64(cores * 1000.0 * headroom)
			rec := SnapCPUMilliToCanonical(milli)
			cr.CPU.RecommendedRequest = rec
			cr.CPU.RecommendedLimit = scaleLimitFromRatio(cr.CPU.CurrentRequest, cr.CPU.CurrentLimit, rec)
		}
		// Memory value is bytes.
		if bytes, ok := memResult[cr.Name]; ok && bytes > 0 {
			scaled := int64(bytes * headroom)
			rec := SnapMemBytesToCanonical(scaled)
			cr.Mem.RecommendedRequest = rec
			cr.Mem.RecommendedLimit = scaleLimitFromRatio(cr.Mem.CurrentRequest, cr.Mem.CurrentLimit, rec)
		}
	}
}

// promStrategyWindow returns the human-readable window label for the
// given strategy. Used as model.Rightsizing.Window so the overlay
// header can show the user what time range backs the recommendation.
func promStrategyWindow(s model.RightsizingStrategy) string {
	switch s {
	case model.StrategyPromMax1D, model.StrategyPromAvg1D:
		return "1d"
	case model.StrategyPromP957D:
		return "7d"
	}
	return ""
}

// buildPromContainerQuery returns the PromQL query for the requested
// strategy + resource. The query aggregates per container across the
// pods named in `pods` (escaped into a regex alternation).
//
// CPU uses container_cpu_usage_seconds_total wrapped in rate(...) over
// a 5-minute window so the inner sample is cores/sec; the outer
// aggregation (max_over_time / avg_over_time / quantile_over_time)
// folds those samples across the strategy's window.
//
// Memory uses container_memory_working_set_bytes directly because it's
// a gauge (no rate() needed).
func buildPromContainerQuery(namespace string, pods []string, strategy model.RightsizingStrategy, resourceKind string) string {
	podRegex := podsRegex(pods)
	// PromQL string literal uses single-backslash escapes (similar to
	// JSON / shell double-quoted strings). fmt's %q would Go-escape and
	// emit `\\.` for a literal `\.` regex — wrong for PromQL. Build the
	// label selector with a manual quote-wrap instead so the regex
	// metacharacter survives intact.
	commonLabels := `namespace="` + namespace + `",pod=~"` + podRegex + `",container!="POD",container!=""`
	var inner string
	switch resourceKind {
	case "cpu":
		inner = fmt.Sprintf(`rate(container_cpu_usage_seconds_total{%s}[5m])`, commonLabels)
	case "memory":
		inner = fmt.Sprintf(`container_memory_working_set_bytes{%s}`, commonLabels)
	default:
		return ""
	}
	window := promStrategyWindow(strategy)
	subquery := fmt.Sprintf(`%s[%s:5m]`, inner, window)
	aggregated := wrapPromAggregation(strategy, subquery)
	return fmt.Sprintf(`max by (container) (%s)`, aggregated)
}

// wrapPromAggregation wraps the inner subquery expression with the
// strategy's outer aggregation function. Centralised here so the
// quantile_over_time signature (which takes the quantile as the first
// argument, expression as the second) doesn't end up half-closed when
// composed via fmt placeholders.
func wrapPromAggregation(s model.RightsizingStrategy, subquery string) string {
	switch s {
	case model.StrategyPromMax1D:
		return fmt.Sprintf(`max_over_time(%s)`, subquery)
	case model.StrategyPromAvg1D:
		return fmt.Sprintf(`avg_over_time(%s)`, subquery)
	case model.StrategyPromP957D:
		return fmt.Sprintf(`quantile_over_time(0.95, %s)`, subquery)
	}
	return subquery
}

// podsRegex builds a regex alternation from a slice of pod names. The
// `(p1|p2|p3)` form matches the exact pods backing the workload — no
// label-prefix wildcards. Pod names with regex metacharacters get
// escaped so the query is well-formed even when a controller emits
// names with `.` (rare but valid).
func podsRegex(pods []string) string {
	if len(pods) == 0 {
		return ""
	}
	escaped := make([]string, len(pods))
	for i, p := range pods {
		escaped[i] = regexp.QuoteMeta(p)
	}
	return "^(" + strings.Join(escaped, "|") + ")$"
}

// queryPromContainerMetric runs an instant PromQL query and returns a
// map of container name -> raw float value. Tests inject the request
// pipeline via Client.testPromQuery; production builds the proxy URL
// from the configured Prometheus endpoint and calls Service.ProxyGet.
func (c *Client) queryPromContainerMetric(ctx context.Context, contextName, query string) (map[string]float64, error) {
	body, err := c.runPrometheusQuery(ctx, contextName, query)
	if err != nil {
		return nil, err
	}
	return parsePrometheusContainerResponse(body)
}

// runPrometheusQuery dispatches a Prometheus instant query through the
// configured Service.ProxyGet pipeline. Tests override via testPromQuery.
func (c *Client) runPrometheusQuery(ctx context.Context, contextName, query string) ([]byte, error) {
	if c.testPromQuery != nil {
		return c.testPromQuery(ctx, contextName, query)
	}
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, fmt.Errorf("clientset: %w", err)
	}
	promNs, promSvc, promPort, _, _, _ := resolveMonitoringEndpoints(contextName)

	params := map[string]string{"query": query}
	// Per-request 10s timeout — a single shared context would burn its
	// budget on the first hung probe and leave the fallback loop unable
	// to succeed (every later DoRaw would fail with context.Canceled).
	doQuery := func(ns, svc string) ([]byte, error) {
		rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		result := cs.CoreV1().Services(ns).ProxyGet("http", svc, promPort, "/api/v1/query", params)
		return result.DoRaw(rctx)
	}
	if cached, ok := promSvcCache.Load(cs); ok {
		entry := cached.(promSvcEntry)
		data, err := doQuery(entry.namespace, entry.service)
		if err == nil {
			return data, nil
		}
		promSvcCache.Delete(cs)
	}
	var lastErr error
	for _, ns := range promNs {
		for _, svc := range promSvc {
			data, err := doQuery(ns, svc)
			if err != nil {
				lastErr = err
				continue
			}
			promSvcCache.Store(cs, promSvcEntry{namespace: ns, service: svc})
			return data, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no prometheus service found")
}

// parsePrometheusContainerResponse extracts the per-container value
// vector from a PromQL instant query response. The "container" label
// keys the result map; empty container labels are skipped (the cgroup
// "POD" pause container is filtered out by the query, but defensive
// code here keeps a malformed metric from polluting the map).
func parsePrometheusContainerResponse(data []byte) (map[string]float64, error) {
	var resp prometheusQueryResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing prometheus response: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("prometheus query returned status: %s", resp.Status)
	}
	out := make(map[string]float64, len(resp.Data.Result))
	for _, r := range resp.Data.Result {
		container := r.Metric["container"]
		if container == "" {
			continue
		}
		if len(r.Value) < 2 {
			continue
		}
		var s string
		if err := json.Unmarshal(r.Value[1], &s); err != nil {
			continue
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			continue
		}
		out[container] = v
	}
	return out, nil
}
