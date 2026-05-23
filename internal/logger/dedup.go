package logger

import (
	"sync"
	"time"
)

// DefaultDedupWindow is the rolling window during which identical
// (tag + contextKey) emissions are suppressed. Five minutes keeps a
// recovering-then-failing-again service visible to the user without
// drowning the in-app log and on-disk log in identical lines.
const DefaultDedupWindow = 5 * time.Minute

var (
	dedupMu     sync.Mutex
	dedupState  = map[string]*dedupEntry{}
	dedupOps    int // ShouldEmit call counter, drives periodic pruning
	dedupWindow = DefaultDedupWindow
	dedupNow    = time.Now // overridable in tests
)

// dedupPruneEvery is how often (in ShouldEmit calls) we sweep stale
// entries. Tuned high enough that the O(N) scan cost is amortized to
// near zero, low enough that a high-cardinality key space (varying
// stderr lines, per-pod tags across many namespaces) can't accumulate
// for long. Exposed via a var so tests can lower it.
var dedupPruneEvery = 1024

type dedupEntry struct {
	lastEmit   time.Time
	suppressed int
}

// ShouldEmit reports whether a deduplicated log event identified by
// (tag, contextKey) should be emitted right now. If the window has
// elapsed since the previous emission, it returns true plus the count
// of events suppressed during that window (so callers can surface
// "(suppressed 142x)" in the next line). If still inside the window,
// it returns false and increments the suppressed counter.
//
// The (tag, contextKey) pair is the dedup key. Use a stable tag per
// call site ("node-metrics-load", "stderr-aws-sso") and a contextKey
// that distinguishes legitimately-different occurrences (cluster name,
// the redacted line itself). Two different contexts dedup
// independently — one cluster's outage never masks another's.
func ShouldEmit(tag, contextKey string) (emit bool, suppressed int) {
	key := tag + "\x00" + contextKey
	now := dedupNow()
	dedupMu.Lock()
	defer dedupMu.Unlock()
	dedupOps++
	if dedupOps%dedupPruneEvery == 0 {
		pruneDedupStateLocked(now)
	}
	e, ok := dedupState[key]
	if !ok {
		dedupState[key] = &dedupEntry{lastEmit: now}
		return true, 0
	}
	if now.Sub(e.lastEmit) >= dedupWindow {
		supp := e.suppressed
		e.lastEmit = now
		e.suppressed = 0
		return true, supp
	}
	e.suppressed++
	return false, 0
}

// WarnOnce emits a WARN log at most once per dedup window. Identical
// repeats during the window are silently counted; the next emission
// after the window includes "suppressed_during_window" so the rate is
// not lost. The entry is also published to UIChan for the in-app log
// overlay.
func WarnOnce(tag, contextKey, msg string, args ...any) {
	emit, supp := ShouldEmit(tag, contextKey)
	if !emit {
		return
	}
	if supp > 0 {
		args = append(args, "suppressed_during_window", supp)
	}
	Warn(msg, args...)
	publishUI("WRN", msg, args)
}

// ErrorOnce is the ERROR-level equivalent of WarnOnce. Use for repeated
// failures (auth expired, endpoint missing) that would otherwise spam
// every background tick.
func ErrorOnce(tag, contextKey, msg string, args ...any) {
	emit, supp := ShouldEmit(tag, contextKey)
	if !emit {
		return
	}
	if supp > 0 {
		args = append(args, "suppressed_during_window", supp)
	}
	Error(msg, args...)
	publishUI("ERR", msg, args)
}

// pruneDedupStateLocked evicts entries whose window has fully elapsed
// AND have no suppressed events still owed an emission. The 2x window
// margin keeps an entry around long enough that a flapping failure
// (down, up, down) still emits once-per-window rather than re-emitting
// the first occurrence as "new". Caller must hold dedupMu.
func pruneDedupStateLocked(now time.Time) {
	cutoff := now.Add(-2 * dedupWindow)
	for k, v := range dedupState {
		if v.lastEmit.Before(cutoff) && v.suppressed == 0 {
			delete(dedupState, k)
		}
	}
}

// ResetDedupForTest clears the dedup state. Test-only.
func ResetDedupForTest() {
	dedupMu.Lock()
	dedupState = map[string]*dedupEntry{}
	dedupOps = 0
	dedupMu.Unlock()
}

// SetDedupPruneEveryForTest overrides the prune interval. Test-only.
func SetDedupPruneEveryForTest(n int) {
	dedupMu.Lock()
	dedupPruneEvery = n
	dedupMu.Unlock()
}

// SetDedupWindowForTest overrides the dedup window. Test-only.
func SetDedupWindowForTest(d time.Duration) {
	dedupMu.Lock()
	dedupWindow = d
	dedupMu.Unlock()
}

// SetDedupClockForTest overrides the clock. Test-only.
func SetDedupClockForTest(now func() time.Time) {
	dedupMu.Lock()
	dedupNow = now
	dedupMu.Unlock()
}
