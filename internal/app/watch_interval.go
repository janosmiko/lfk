package app

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

// activeWatchInterval resolves the current watch-tick cadence: the background
// interval while the terminal is unfocused or a focused window has been idle
// past foregroundIdleTimeout, otherwise the foreground interval.
func (m Model) activeWatchInterval() time.Duration {
	if !m.watchThrottle {
		return m.watchInterval
	}
	if !m.focused {
		return m.backgroundWatchInterval
	}
	if m.foregroundIdleTimeout > 0 && time.Since(m.lastInputAt) >= m.foregroundIdleTimeout {
		return m.backgroundWatchInterval
	}
	return m.watchInterval
}

// startWatchChain bumps the generation and returns a tick command for a fresh
// chain at the resolved interval, retiring any in-flight ticks from the prior
// generation. Pointer receiver so the bump lands on the caller's model copy.
func (m *Model) startWatchChain() tea.Cmd {
	m.watchTickGen++
	return scheduleWatchTick(m.activeWatchInterval(), m.watchTickGen)
}

// resolveBackgroundInterval returns the background watch interval from opts when set
// (> 0), otherwise falls back to the config global.
func resolveBackgroundInterval(opts StartupOptions) time.Duration {
	if opts.BackgroundWatchInterval > 0 {
		return ui.ClampWatchInterval(opts.BackgroundWatchInterval)
	}
	return ui.ConfigBackgroundWatchInterval
}

// resolveForegroundIdle returns the foreground idle timeout from opts when set
// (>= 0), otherwise falls back to the config global. The sentinel -1 means
// "not set via CLI"; 0 is a valid value that disables focused-idle throttling.
func resolveForegroundIdle(opts StartupOptions) time.Duration {
	if opts.ForegroundIdleTimeout >= 0 {
		return ui.ClampForegroundIdleTimeout(opts.ForegroundIdleTimeout)
	}
	return ui.ConfigForegroundIdleTimeout
}
