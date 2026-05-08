package scheduler

import (
	"maps"
	"time"
)

// Default clamps and timeouts. Exported as constants so callers can
// inspect the clamp range and tests can reference the fallback.
const (
	MinWorkersPerContext     = 1
	MaxWorkersPerContext     = 16
	DefaultWorkersPerContext = 4
	DefaultCriticalReserved  = 1
	DefaultRequestTimeout    = 30 * time.Second
)

// Package-level config globals populated from internal/ui/config_apply.go.
// These are read at scheduler.New() time; later mutations have no effect
// on already-running schedulers (consistent with ConfigWatchInterval).
var (
	ConfigWorkersPerContext     = DefaultWorkersPerContext
	ConfigCriticalReserved      = DefaultCriticalReserved
	ConfigDefaultTimeout        = DefaultRequestTimeout
	ConfigTimeoutsByKind        = map[Kind]time.Duration{} // empty by default
	ConfigShowPriorityInOverlay = true
)

// Config bundles the runtime knobs a Registry uses for scheduling. A nil
// *Config is valid and falls back to compiled defaults — used in tests
// that don't care about scheduling specifics.
type Config struct {
	WorkersPerContext int
	CriticalReserved  int
	Default           time.Duration
	ByKind            map[Kind]time.Duration
}

// FromGlobals snapshots the current package-globals into a Config. Called
// by Registry.New() to capture config at construction time. The ByKind
// map is deep-copied so mutations on the returned Config do not leak
// back into the globals.
func FromGlobals() *Config {
	byKind := make(map[Kind]time.Duration, len(ConfigTimeoutsByKind))
	maps.Copy(byKind, ConfigTimeoutsByKind)
	workers := ClampWorkers(ConfigWorkersPerContext)
	return &Config{
		WorkersPerContext: workers,
		CriticalReserved:  ClampCriticalReserved(ConfigCriticalReserved, workers),
		Default:           ConfigDefaultTimeout,
		ByKind:            byKind,
	}
}

// ClampWorkers enforces the [MinWorkersPerContext, MaxWorkersPerContext]
// range. Values outside the range are clipped to the nearest bound.
func ClampWorkers(n int) int {
	if n < MinWorkersPerContext {
		return MinWorkersPerContext
	}
	if n > MaxWorkersPerContext {
		return MaxWorkersPerContext
	}
	return n
}

// ClampCriticalReserved enforces 0 <= reserved <= workers/2 so Critical
// can never starve High and Low entirely.
func ClampCriticalReserved(reserved, workers int) int {
	if reserved < 0 {
		return 0
	}
	maxReserved := workers / 2
	if reserved > maxReserved {
		return maxReserved
	}
	return reserved
}

// TimeoutFor returns the per-Kind override if set, otherwise the default.
// A nil receiver returns DefaultRequestTimeout — used by tests that
// construct a Registry without a Config.
func (c *Config) TimeoutFor(k Kind) time.Duration {
	if c == nil {
		return DefaultRequestTimeout
	}
	if d, ok := c.ByKind[k]; ok && d > 0 {
		return d
	}
	if c.Default > 0 {
		return c.Default
	}
	return DefaultRequestTimeout
}
