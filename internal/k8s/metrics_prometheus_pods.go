package k8s

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// buildPromPodQuery returns the instant PromQL query for per-pod CPU or memory
// usage, aggregated by (namespace, pod) so a single query covers a namespace
// (or, when namespace is empty, the whole cluster). CPU is converted from cores
// to millicores to match the metrics.k8s.io API path (model.PodMetrics.CPU is
// millicores); memory is bytes.
func buildPromPodQuery(namespace, resourceKind string) string {
	labels := `container!="POD",container!=""`
	if namespace != "" {
		// Defense-in-depth: k8s namespaces are DNS-1123 labels (the API server
		// rejects anything else), but never interpolate a value carrying PromQL
		// label-selector metacharacters that could break out of the quotes.
		if strings.ContainsAny(namespace, "\"\\\n") {
			return ""
		}
		labels = `namespace="` + namespace + `",` + labels
	}
	switch resourceKind {
	case "cpu":
		return fmt.Sprintf(`sum by (namespace, pod) (rate(container_cpu_usage_seconds_total{%s}[3m])) * 1000`, labels)
	case "memory":
		return fmt.Sprintf(`sum by (namespace, pod) (container_memory_working_set_bytes{%s})`, labels)
	default:
		return ""
	}
}

// parsePrometheusPodResponse extracts the per-pod value vector from a PromQL
// instant query response, keyed by "namespace/pod" to match the lookup keys
// the metrics-API path and the app-side enrichment build. Samples missing
// either label are skipped.
func parsePrometheusPodResponse(data []byte) (map[string]float64, error) {
	return parsePrometheusVector(data, func(labels map[string]string) (string, bool) {
		ns, pod := labels["namespace"], labels["pod"]
		if ns == "" || pod == "" {
			return "", false
		}
		return ns + "/" + pod, true
	})
}

// getAllPodMetricsFromPrometheus fetches per-pod CPU and memory usage from the
// configured Prometheus endpoint. namespace == "" queries all namespaces. The
// result is keyed by "namespace/pod" (matching getAllPodMetricsFromAPI). It
// errors only when both the CPU and memory queries fail, so a partial outage
// still surfaces whatever data is available.
func (c *Client) getAllPodMetricsFromPrometheus(ctx context.Context, contextName, namespace string) (map[string]model.PodMetrics, error) {
	cpuMap, cpuErr := c.queryPromPodMetric(ctx, contextName, buildPromPodQuery(namespace, "cpu"))
	if cpuErr != nil {
		logger.Debug("Prometheus pod CPU query failed", "context", contextName, "namespace", namespace, "error", cpuErr)
	}
	memMap, memErr := c.queryPromPodMetric(ctx, contextName, buildPromPodQuery(namespace, "memory"))
	if memErr != nil {
		logger.Debug("Prometheus pod memory query failed", "context", contextName, "namespace", namespace, "error", memErr)
	}
	if cpuErr != nil && memErr != nil {
		return nil, fmt.Errorf("prometheus pod queries failed: cpu: %w, mem: %w", cpuErr, memErr)
	}

	keys := make(map[string]struct{}, len(cpuMap)+len(memMap))
	for k := range cpuMap {
		keys[k] = struct{}{}
	}
	for k := range memMap {
		keys[k] = struct{}{}
	}

	result := make(map[string]model.PodMetrics, len(keys))
	for key := range keys {
		ns, pod, ok := strings.Cut(key, "/")
		if !ok {
			continue
		}
		result[key] = model.PodMetrics{
			Name:      pod,
			Namespace: ns,
			CPU:       int64(math.Round(cpuMap[key])),
			Memory:    int64(math.Round(memMap[key])),
		}
	}
	return result, nil
}

// queryPromPodMetric runs an instant PromQL query and parses it into a
// "namespace/pod" -> value map via the shared Prometheus proxy pipeline.
func (c *Client) queryPromPodMetric(ctx context.Context, contextName, query string) (map[string]float64, error) {
	body, err := c.runPrometheusQuery(ctx, contextName, query)
	if err != nil {
		return nil, err
	}
	return parsePrometheusPodResponse(body)
}

// runPodMetricsRoutes tries the configured pod-metrics routes (Prometheus /
// metrics API) in order, returning the first success. It mirrors
// GetAllNodeMetrics' fallback behavior so a cluster with only one working
// source still resolves. The error returned when every route fails names both
// sources, so the single caller-side warning is actionable.
func runPodMetricsRoutes[T any](contextName string, fromProm, fromAPI func() (T, error)) (T, error) {
	nodeMetrics, hasPrometheus := resolveNodeMetricsConfig(contextName)
	routes := selectNodeMetricsRoutes(nodeMetrics, hasPrometheus)

	var zero T
	var lastErr error
	for i, route := range routes {
		var (
			res T
			err error
		)
		if route == nodeMetricsRoutePrometheus {
			res, err = fromProm()
		} else {
			res, err = fromAPI()
		}
		if err == nil {
			return res, nil
		}
		lastErr = err
		if i+1 < len(routes) {
			logger.Debug("pod metrics route failed, trying fallback",
				"context", contextName, "failed_route", route.String(),
				"fallback_route", routes[i+1].String(), "error", err)
		}
	}
	return zero, fmt.Errorf("no pod metrics source available (metrics-server API or Prometheus): %w", lastErr)
}
