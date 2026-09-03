package k8s

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/tainted"
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
func (c *Client) applyPrometheusStrategy(ctx context.Context, contextName, namespace, kind, name string, strategy model.RightsizingStrategy, headroom float64, out *model.Rightsizing) {
	indexed := c.workloadIsIndexed(ctx, contextName, namespace, kind, name)
	cpuQuery := buildPromContainerQuery(namespace, kind, name, indexed, strategy, "cpu")
	memQuery := buildPromContainerQuery(namespace, kind, name, indexed, strategy, "memory")
	if cpuQuery == "" && memQuery == "" {
		return
	}
	out.Window = promStrategyWindow(strategy)

	cpuResult, err := c.queryPromContainerMetric(ctx, contextName, cpuQuery)
	if err != nil {
		logger.Debug("rightsizing: Prometheus CPU query failed", "err", err)
		out.SourceError = tainted.SanitizeTerminalText(err.Error())
	}
	memResult, err := c.queryPromContainerMetric(ctx, contextName, memQuery)
	if err != nil {
		logger.Debug("rightsizing: Prometheus memory query failed", "err", err)
		if out.SourceError == "" {
			out.SourceError = tainted.SanitizeTerminalText(err.Error())
		}
	}

	for i := range out.Containers {
		cr := &out.Containers[i]
		// CPU value is in cores. Convert to millicores. A rate under 1m
		// truncates to 0, which is "no suggestion", not a 0 request.
		if cores, ok := cpuResult[cr.Name]; ok && cores > 0 && int64(cores*1000.0*headroom) > 0 {
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
// strategy + resource, aggregated per container.
//
// CPU wraps container_cpu_usage_seconds_total in rate(...) over 5m so
// the inner sample is cores/sec. Memory reads
// container_memory_working_set_bytes directly because it is a gauge.
//
// Returns "" for a kind podsRegexForWorkload cannot describe, so the
// caller skips the query rather than selecting the whole namespace.
func buildPromContainerQuery(namespace, kind, name string, indexed bool, strategy model.RightsizingStrategy, resourceKind string) string {
	podRegex := podsRegexForWorkload(kind, name, indexed)
	if podRegex == "" {
		return ""
	}
	// A raw PromQL literal, so the `\.` QuoteMeta emits for a dotted
	// workload name survives: a double-quoted literal processes escapes
	// and rejects `\.` as unknown.
	podMatcher := fmt.Sprintf("pod=~%s%s%s", promRawQuote, podRegex, promRawQuote)
	commonLabels := fmt.Sprintf(`namespace=%q,%s,container!="POD",container!=""`, namespace, podMatcher)
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

// k8sGeneratedChars is rand.alphanums. Its lack of vowels is what keeps
// a Deployment "foo" pattern off the pods of a sibling "foo-api".
const k8sGeneratedChars = `[bcdfghjklmnpqrstvwxz2456789]`

// promRawQuote opens and closes a PromQL raw string literal. Named
// because a bare backtick cannot appear inside one.
const promRawQuote = "`"

// podsRegexForWorkload also matches pods from revisions that no longer
// exist, so a 1d or 7d window survives the rollout that applying a
// recommendation triggers. "" for a kind it cannot describe.
func podsRegexForWorkload(kind, name string, indexed bool) string {
	n := regexp.QuoteMeta(name)
	gen := k8sGeneratedChars
	// Only an Indexed Job puts a digit segment here. Always-on, that
	// group would also swallow a sibling workload named "<name>-3".
	index := ""
	if indexed {
		index = "[0-9]+-"
	}
	switch kind {
	case "Pod":
		return "^" + n + "$"
	case "Deployment":
		// <deployment>-<pod-template-hash>-<suffix>. The hash is a uint32
		// rendered through the same alphabet, so 1-10 characters.
		return "^" + n + "-" + gen + "{1,10}-" + gen + "{5}$"
	case "DaemonSet":
		return "^" + n + "-" + gen + "{5}$"
	case "StatefulSet":
		return "^" + n + "-[0-9]+$"
	case "Job":
		return "^" + n + "-" + index + gen + "{5}$"
	case "CronJob":
		// The CronJob controller names each Job after its scheduled time
		// in minutes since the epoch, then the Job names its own pods.
		return "^" + n + "-[0-9]+-" + index + gen + "{5}$"
	}
	return ""
}

// queryPromContainerMetric runs an instant PromQL query and returns a
// map of container name -> raw float value. Tests inject the request
// pipeline via Client.testPromQuery. Production builds the proxy URL
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
// Demo mode is turned away before ever building a request: the demo
// backend's fake clientset panics inside ProxyGet itself (see
// errPrometheusUnavailableDemo), so it must never be called even once.
func (c *Client) runPrometheusQuery(ctx context.Context, contextName, query string) ([]byte, error) {
	if c.testPromQuery != nil {
		return c.testPromQuery(ctx, contextName, query)
	}
	return c.runPrometheusProxy(ctx, contextName, "/api/v1/query", map[string]string{"query": query})
}

// runPrometheusProxy sends one request to the discovered Prometheus Service
// through the API server proxy and returns the raw body. It owns the four
// behaviors both the instant and the range path need: the cached
// namespace+service hit, eviction of a stale cache entry, the
// namespace-by-service discovery loop, and a per-request timeout so one hung
// probe cannot spend the whole context budget and leave the fallback loop
// unable to succeed.
func (c *Client) runPrometheusProxy(ctx context.Context, contextName, path string, params map[string]string) ([]byte, error) {
	if c.demo {
		return nil, errPrometheusUnavailableDemo
	}
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, fmt.Errorf("clientset: %w", err)
	}
	promTargets, _ := monitoringTargetsFor(ctx, cs, contextName)
	promTargets = withCommonPrefixes(promTargets)

	doQuery := func(t monitoringTarget) ([]byte, error) {
		rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return safeProxyGetRaw(rctx, cs, t.Namespace, t.Service, t.Port, t.path(path), params)
	}
	// Keyed by contextName (not the clientset): clientsetForContext builds a
	// fresh clientset per call, so a clientset key would never hit and re-run
	// service discovery on every query.
	if cached, ok := promSvcCache.Load(contextName); ok {
		data, err := doQuery(cached.(monitoringTarget))
		if err == nil {
			return data, nil
		}
		promSvcCache.Delete(contextName)
	}
	data, hit, err := probePrometheusTargets(promTargets, doQuery)
	if err != nil {
		return nil, err
	}
	promSvcCache.Store(contextName, hit)
	return data, nil
}

// probePrometheusTargets tries each target in order and returns the first
// answer. On failure the error lists every target with what it answered:
// the last guess in the list is always prometheus-operated, so reporting only
// its error hides why the discovered Service was rejected.
func probePrometheusTargets(targets []monitoringTarget, doQuery func(monitoringTarget) ([]byte, error)) ([]byte, monitoringTarget, error) {
	if len(targets) == 0 {
		return nil, monitoringTarget{}, fmt.Errorf("no prometheus service found")
	}
	failures := make([]string, 0, len(targets))
	for _, t := range targets {
		data, err := doQuery(t)
		if err == nil {
			return data, t, nil
		}
		failures = append(failures, t.String()+": "+err.Error())
	}
	return nil, monitoringTarget{}, fmt.Errorf("no prometheus target answered (tried %d targets): %s",
		len(targets), strings.Join(failures, "; "))
}

// parsePrometheusContainerResponse extracts the per-container value
// vector from a PromQL instant query response. The "container" label
// keys the result map. Empty container labels are skipped (the cgroup
// "POD" pause container is filtered out by the query, but defensive
// code here keeps a malformed metric from polluting the map).
func parsePrometheusContainerResponse(data []byte) (map[string]float64, error) {
	return parsePrometheusVector(data, func(labels map[string]string) (string, bool) {
		container := labels["container"]
		return container, container != ""
	})
}
