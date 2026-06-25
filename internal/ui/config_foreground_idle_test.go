package ui

import (
	"testing"
	"time"
)

func TestApplyForegroundIdleTimeoutConfig(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"empty keeps default", "", DefaultForegroundIdleTimeout},
		{"valid parses", "300s", 300 * time.Second},
		{"zero disables", "0s", 0},
		{"above max clamps", "30m", MaxWatchInterval},
		{"invalid keeps default", "xyz", DefaultForegroundIdleTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ConfigForegroundIdleTimeout = DefaultForegroundIdleTimeout
			applyForegroundIdleTimeoutConfig(tt.raw)
			if ConfigForegroundIdleTimeout != tt.want {
				t.Fatalf("got %v want %v", ConfigForegroundIdleTimeout, tt.want)
			}
		})
	}
}
