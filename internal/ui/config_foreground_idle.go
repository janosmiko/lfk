package ui

import "time"

func applyForegroundIdleTimeoutConfig(raw string) {
	if raw == "" {
		return
	}
	if d, err := time.ParseDuration(raw); err == nil {
		ConfigForegroundIdleTimeout = ClampForegroundIdleTimeout(d)
	}
}
