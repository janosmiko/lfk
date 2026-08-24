package k8s

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"
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
			// Each pair is [timestamp, "value"]; only the value is needed
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
