package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/janosmiko/lfk/internal/logger"
)

// MetricSeries holds one metric's samples over a window, oldest first.
// A sample Prometheus could not supply is NaN rather than absent: the
// renderer draws NaN as a blank column, and dropping the entry instead
// would slide every later sample to the wrong position on the time axis.
type MetricSeries struct {
	Points []float64
	Step   time.Duration
}

// prometheusRangeResponse is the JSON shape of /api/v1/query_range. It is
// separate from prometheusQueryResponse because a range result carries a
// "values" array per series where an instant result carries one "value",
// so the two cannot share a struct.
type prometheusRangeResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string   `json:"metric"`
			Values [][]json.RawMessage `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

// parsePrometheusMatrix parses a range-query matrix response into a map keyed
// by keyFunc(metricLabels). Series for which keyFunc returns ok=false are
// skipped. It mirrors parsePrometheusVector, which serves the instant path.
func parsePrometheusMatrix(data []byte, keyFunc func(labels map[string]string) (string, bool)) (map[string]MetricSeries, error) {
	var resp prometheusRangeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing prometheus range response: %w", err)
	}
	if resp.Status != "success" {
		if resp.Error != "" {
			return nil, fmt.Errorf("prometheus range query returned status %s: %s", resp.Status, resp.Error)
		}
		return nil, fmt.Errorf("prometheus range query returned status: %s", resp.Status)
	}

	result := make(map[string]MetricSeries, len(resp.Data.Result))
	for _, series := range resp.Data.Result {
		key, ok := keyFunc(series.Metric)
		if !ok {
			continue
		}
		points := make([]float64, 0, len(series.Values))
		for _, pair := range series.Values {
			// Each pair is [timestamp, "value"]. Only the value is needed
			// because the step is uniform and known by the caller.
			if len(pair) < 2 {
				points = append(points, math.NaN())
				continue
			}
			var raw string
			if err := json.Unmarshal(pair[1], &raw); err != nil {
				points = append(points, math.NaN())
				continue
			}
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				points = append(points, math.NaN())
				continue
			}
			points = append(points, v)
		}
		result[key] = MetricSeries{Points: points}
	}
	return result, nil
}

// runPrometheusRangeQuery sends a PromQL range query covering the last
// window, sampled every step, through the shared Prometheus proxy. Tests
// override via testPromRangeQuery.
func (c *Client) runPrometheusRangeQuery(ctx context.Context, contextName, query string, window, step time.Duration) ([]byte, error) {
	if c.testPromRangeQuery != nil {
		return c.testPromRangeQuery(ctx, contextName, query, window, step)
	}
	if c.demo {
		return nil, errPrometheusUnavailableDemo
	}
	end := time.Now()
	params := map[string]string{
		"query": query,
		// RFC3339 rather than a Unix float: Prometheus accepts both, and the
		// string form keeps a failed request legible in a redacted log.
		"start": end.Add(-window).Format(time.RFC3339),
		"end":   end.Format(time.RFC3339),
		"step":  formatPromStep(step),
	}
	return c.runPrometheusProxy(ctx, contextName, "/api/v1/query_range", params)
}

// Plain float seconds, with no unit suffix. Prometheus accepts step
// either as a duration literal or as a float number of seconds, and
// a duration literal must be an integer per unit. step is
// window/points where points is the user-configurable sparkline
// width, so most widths yield a fraction: 5m over 7 columns is
// 42.857142857s, which a duration literal rejects. The suffixless
// float form is valid for every value.
func formatPromStep(step time.Duration) string {
	return strconv.FormatFloat(step.Seconds(), 'f', -1, 64)
}

// GetPodMetricsRange fetches per-pod CPU and memory history from Prometheus
// over the last window, sampled every step. namespace == "" queries every
// namespace. Both maps are keyed by "namespace/pod", matching
// GetAllPodMetrics, so the app-side lookup is identical for both paths.
// CPU points are millicores and memory points are bytes, matching
// model.PodMetrics.
//
// It errors only when both queries fail, so a partial Prometheus outage
// still shows whatever is available. This matches
// getAllPodMetricsFromPrometheus.
func (c *Client) GetPodMetricsRange(ctx context.Context, contextName, namespace string, window, step time.Duration) (cpu, mem map[string]MetricSeries, err error) {
	cpu, cpuErr := c.queryPodRange(ctx, contextName, buildPromPodQuery(namespace, "cpu"), window, step)
	if cpuErr != nil {
		logger.Debug("Prometheus pod CPU range query failed",
			"context", contextName, "namespace", namespace, "error", cpuErr)
	}
	mem, memErr := c.queryPodRange(ctx, contextName, buildPromPodQuery(namespace, "memory"), window, step)
	if memErr != nil {
		logger.Debug("Prometheus pod memory range query failed",
			"context", contextName, "namespace", namespace, "error", memErr)
	}
	if cpuErr != nil && memErr != nil {
		return nil, nil, fmt.Errorf("prometheus pod range queries failed: cpu: %w, mem: %w", cpuErr, memErr)
	}
	return cpu, mem, nil
}

// queryPodRange runs one pod range query and parses it into a
// "namespace/pod" -> series map, stamping the step on every series so the
// renderer does not need to be told separately.
func (c *Client) queryPodRange(ctx context.Context, contextName, query string, window, step time.Duration) (map[string]MetricSeries, error) {
	if query == "" {
		return nil, fmt.Errorf("empty prometheus query, refusing to send")
	}
	body, err := c.runPrometheusRangeQuery(ctx, contextName, query, window, step)
	if err != nil {
		return nil, err
	}
	series, err := parsePrometheusMatrix(body, func(labels map[string]string) (string, bool) {
		ns, pod := labels["namespace"], labels["pod"]
		if ns == "" || pod == "" {
			return "", false
		}
		return ns + "/" + pod, true
	})
	if err != nil {
		return nil, err
	}
	for key, s := range series {
		s.Step = step
		series[key] = s
	}
	return series, nil
}
