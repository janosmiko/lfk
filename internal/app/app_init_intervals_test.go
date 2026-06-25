package app

import (
	"testing"
	"time"

	"github.com/janosmiko/lfk/internal/ui"
)

func TestResolveBackgroundInterval(t *testing.T) {
	prev := ui.ConfigBackgroundWatchInterval
	t.Cleanup(func() { ui.ConfigBackgroundWatchInterval = prev })
	ui.ConfigBackgroundWatchInterval = 30 * time.Second
	// CLI flag overrides config.
	if got := resolveBackgroundInterval(StartupOptions{BackgroundWatchInterval: 45 * time.Second}); got != 45*time.Second {
		t.Fatalf("flag override got %v", got)
	}
	// Unset flag falls back to config.
	if got := resolveBackgroundInterval(StartupOptions{}); got != 30*time.Second {
		t.Fatalf("config fallback got %v", got)
	}
}

func TestResolveForegroundIdle(t *testing.T) {
	prev := ui.ConfigForegroundIdleTimeout
	t.Cleanup(func() { ui.ConfigForegroundIdleTimeout = prev })
	ui.ConfigForegroundIdleTimeout = 120 * time.Second
	if got := resolveForegroundIdle(StartupOptions{ForegroundIdleTimeout: 90 * time.Second}); got != 90*time.Second {
		t.Fatalf("flag override got %v", got)
	}
	// Unset (sentinel -1) falls back to config.
	if got := resolveForegroundIdle(StartupOptions{ForegroundIdleTimeout: -1}); got != 120*time.Second {
		t.Fatalf("config fallback got %v", got)
	}
	// 0 from the flag means disable — must be preserved, not treated as unset.
	if got := resolveForegroundIdle(StartupOptions{ForegroundIdleTimeout: 0}); got != 0 {
		t.Fatalf("0 must disable (got %v)", got)
	}
}
