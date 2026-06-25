package ui

import (
	"testing"
	"time"
)

func TestApplyBackgroundWatchIntervalConfig(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"empty keeps default", "", DefaultBackgroundWatchInterval},
		{"valid parses", "45s", 45 * time.Second},
		{"below min clamps up", "100ms", MinWatchInterval},
		{"above max clamps down", "30m", MaxWatchInterval},
		{"invalid keeps default", "nonsense", DefaultBackgroundWatchInterval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ConfigBackgroundWatchInterval = DefaultBackgroundWatchInterval
			applyBackgroundWatchIntervalConfig(tt.raw)
			if ConfigBackgroundWatchInterval != tt.want {
				t.Fatalf("got %v want %v", ConfigBackgroundWatchInterval, tt.want)
			}
		})
	}
}
