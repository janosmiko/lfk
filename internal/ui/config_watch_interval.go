package ui

import "time"

func applyWatchIntervalConfig(raw string) {
	if raw == "" {
		return
	}
	if d, err := time.ParseDuration(raw); err == nil {
		if clamped := ClampWatchInterval(d); clamped > 0 {
			ConfigWatchInterval = clamped
		}
	}
}
