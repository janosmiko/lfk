package scheduler

import (
	"maps"
	"time"
)

// Default clamps and timeouts. Exported as constants so callers can
// inspect the clamp range and tests can reference the fallback.
const (
	MinWorkersPerContext = 1
	MaxWorkersPerContext = 16
	// DefaultWorkersPerContext is the per-context worker pool size. 8 (up from
	// 4) gives a slow-responding cluster enough concurrent slots that the
	// foreground resource lists, previews, and the background metrics/events
	// fan-out are not all contending for the same handful of workers — the
	// "Low tasks queued but never run" symptom on a slow API server.
	DefaultWorkersPerContext = 8
	DefaultCriticalReserved  = 1
	// DefaultLowReserved is how many workers prefer Low (background) work so
	// metrics, events, and dashboard scans always have a slot even while the
	// foreground floods the general pool. Mirrors CriticalReserved at the
	// other end of the priority range. Clamped to leave >=1 general worker.
	DefaultLowReserved    = 2
	DefaultRequestTimeout = 30 * time.Second

	// DefaultAgingThreshold is how many times a lower-priority lane may be
	// passed over by higher-priority dequeues before it is force-promoted for
	// one task. It bounds worst-case starvation latency under sustained
	// higher-priority load (the user's "Low tasks never run while High keeps
	// arriving" case). Critical is exempt — it is never aged. 0 disables aging
	// (strict priority). 8 gives background work roughly one dispatch in nine
	// under continuous High pressure.
	DefaultAgingThreshold = 8
)

// Package-level config globals populated from internal/ui/config_apply.go.
// These are read at scheduler.New() time. Later mutations have no effect
// on already-running schedulers (consistent with ConfigWatchInterval).
var (
	ConfigWorkersPerContext     = DefaultWorkersPerContext
	ConfigCriticalReserved      = DefaultCriticalReserved
	ConfigLowReserved           = DefaultLowReserved
	ConfigDefaultTimeout        = DefaultRequestTimeout
	ConfigTimeoutsByKind        = map[Kind]time.Duration{} // empty by default
	ConfigShowPriorityInOverlay = true
	ConfigAgingThreshold        = DefaultAgingThreshold
)

// Config bundles the runtime knobs a Registry uses for scheduling. A nil
// *Config is valid and falls back to compiled defaults — used in tests
// that don't care about scheduling specifics.
type Config struct {
	WorkersPerContext int
	CriticalReserved  int
	LowReserved       int
	Default           time.Duration
	ByKind            map[Kind]time.Duration
	AgingThreshold    int
}

// FromGlobals snapshots the current package-globals into a Config. Called
// by Registry.New() to capture config at construction time. The ByKind
// map is deep-copied so mutations on the returned Config do not leak
// back into the globals.
func FromGlobals() *Config {
	byKind := make(map[Kind]time.Duration, len(ConfigTimeoutsByKind))
	maps.Copy(byKind, ConfigTimeoutsByKind)
	workers := ClampWorkers(ConfigWorkersPerContext)
	crit := ClampCriticalReserved(ConfigCriticalReserved, workers)
	return &Config{
		WorkersPerContext: workers,
		CriticalReserved:  crit,
		LowReserved:       ClampLowReserved(ConfigLowReserved, workers, crit),
		Default:           ConfigDefaultTimeout,
		ByKind:            byKind,
		AgingThreshold:    ClampAgingThreshold(ConfigAgingThreshold),
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

// ClampAgingThreshold floors the value at 0 (aging disabled = strict
// priority). There is no upper bound: a large threshold simply means
// background work waits longer behind sustained higher-priority load.
func ClampAgingThreshold(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// ClampLowReserved enforces 0 <= reserved <= workers - critical - 1 so at
// least one general worker always remains for High work (a low-reserved
// worker prefers Low but falls back, so the floor is about guaranteeing High
// is never left without a dedicated slot). Negative values clamp to 0.
func ClampLowReserved(reserved, workers, critical int) int {
	if reserved < 0 {
		return 0
	}
	maxReserved := max(workers-critical-1, 0)
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
