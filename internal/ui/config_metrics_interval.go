package ui

import "time"

// Metrics list-fetch bounds. metrics.k8s.io only recomputes every 15s or so,
// so a faster list fetch costs an API round trip and returns the same numbers.
const (
	DefaultMetricsInterval = 15 * time.Second
	MinMetricsInterval     = 2 * time.Second
)

// ConfigMetricsInterval is the minimum gap between two list-wide CPU/MEM
// fetches on a watch-tick refresh. 0 disables the throttle. User-driven loads
// ignore it. Clamped to [MinMetricsInterval, MaxWatchInterval].
var ConfigMetricsInterval = DefaultMetricsInterval

// ClampMetricsInterval restricts d to [MinMetricsInterval, MaxWatchInterval].
// A non-positive d means "no throttle" and passes through as 0.
func ClampMetricsInterval(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	if d < MinMetricsInterval {
		return MinMetricsInterval
	}
	if d > MaxWatchInterval {
		return MaxWatchInterval
	}
	return d
}

func applyMetricsIntervalConfig(raw string) {
	if raw == "" {
		return
	}
	if d, err := time.ParseDuration(raw); err == nil {
		ConfigMetricsInterval = ClampMetricsInterval(d)
	}
}
