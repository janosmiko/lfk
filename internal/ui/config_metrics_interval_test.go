package ui

import (
	"testing"
	"time"
)

func TestApplyMetricsIntervalConfig(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"empty keeps default", "", DefaultMetricsInterval},
		{"valid value applied", "30s", 30 * time.Second},
		{"below min clamps up", "500ms", MinMetricsInterval},
		{"above max clamps down", "30m", MaxWatchInterval},
		{"zero disables the throttle", "0s", 0},
		{"negative disables the throttle", "-5s", 0},
		{"invalid keeps default", "nonsense", DefaultMetricsInterval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ConfigMetricsInterval = DefaultMetricsInterval
			applyMetricsIntervalConfig(tt.raw)
			if ConfigMetricsInterval != tt.want {
				t.Fatalf("got %v want %v", ConfigMetricsInterval, tt.want)
			}
		})
	}
}
