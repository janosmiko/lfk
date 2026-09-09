package k8s

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// dataSpanFullFraction is the share of the window the samples must cover
// to count as complete. The probe's subquery step makes even a fully
// covered window read a little short, so a small gap stays quiet.
const dataSpanFullFraction = 0.9

// promDataSpan returns "" unless the samples fall short of the window,
// so only a workload with too little history flags itself in the header.
func (c *Client) promDataSpan(ctx context.Context, contextName, namespace, kind, name string, indexed bool, strategy model.RightsizingStrategy) string {
	query := buildPromDataSpanQuery(namespace, kind, name, indexed, strategy)
	if query == "" {
		return ""
	}
	span, err := c.queryPromDataSpan(ctx, contextName, query)
	if err != nil {
		logger.Debug("rightsizing: Prometheus data span probe failed", "err", err)
		return ""
	}
	return shortDataSpanLabel(span, promStrategyWindowDuration(strategy))
}

// buildPromDataSpanQuery asks Prometheus for the age of the oldest
// sample in the window. Prometheus does the subtraction, so a clock skew
// between this host and the server cannot shorten the reported span.
func buildPromDataSpanQuery(namespace, kind, name string, indexed bool, strategy model.RightsizingStrategy) string {
	podRegex := podsRegexForWorkload(kind, name, indexed)
	window := promStrategyWindow(strategy)
	if podRegex == "" || window == "" {
		return ""
	}
	selector := promWorkloadSelector(namespace, podRegex)
	return fmt.Sprintf("time() - min(min_over_time(timestamp(container_memory_working_set_bytes{%s})[%s:%s]))",
		selector, window, promSpanProbeStep(strategy))
}

// promSpanProbeStep keeps the probe under a few hundred evaluations. The
// span is rendered coarsely, so a step this wide costs no visible detail.
func promSpanProbeStep(s model.RightsizingStrategy) string {
	switch s {
	case model.StrategyPromMax1D, model.StrategyPromAvg1D:
		return "5m"
	case model.StrategyPromP957D:
		return "1h"
	}
	return ""
}

// promStrategyWindowDuration is promStrategyWindow parsed, for comparing
// against a measured span.
func promStrategyWindowDuration(s model.RightsizingStrategy) time.Duration {
	switch s {
	case model.StrategyPromMax1D, model.StrategyPromAvg1D:
		return 24 * time.Hour
	case model.StrategyPromP957D:
		return 7 * 24 * time.Hour
	}
	return 0
}

// queryPromDataSpan reads the single unlabelled sample the span query
// returns, in seconds.
func (c *Client) queryPromDataSpan(ctx context.Context, contextName, query string) (time.Duration, error) {
	body, err := c.runPrometheusQuery(ctx, contextName, query)
	if err != nil {
		return 0, err
	}
	vec, err := parsePrometheusVector(body, func(map[string]string) (string, bool) {
		return "span", true
	})
	if err != nil {
		return 0, err
	}
	seconds, ok := vec["span"]
	if !ok || math.IsNaN(seconds) || seconds <= 0 {
		return 0, fmt.Errorf("prometheus returned no usable data span")
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func shortDataSpanLabel(span, window time.Duration) string {
	if span <= 0 || window <= 0 || span >= time.Duration(float64(window)*dataSpanFullFraction) {
		return ""
	}
	return formatAge(span)
}
