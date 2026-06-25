package ui

import "time"

func applyBackgroundWatchIntervalConfig(raw string) {
	if raw == "" {
		return
	}
	if d, err := time.ParseDuration(raw); err == nil {
		if clamped := ClampWatchInterval(d); clamped > 0 {
			ConfigBackgroundWatchInterval = clamped
		}
	}
}
