package app

import (
	"fmt"
	"time"
)

// TimeKind classifies a LogTimePoint as relative (offset from now) or absolute.
type TimeKind int

const (
	// TimeRelative means the endpoint is expressed as an offset from "now".
	// Typically negative (e.g. -24h → "24 hours ago") but may be positive
	// for rare "up to N in the future" cases that humans never ask for.
	TimeRelative TimeKind = iota
	// TimeAbsolute means the endpoint is a concrete wall-clock timestamp.
	TimeAbsolute
)

// LogTimePoint describes one endpoint of a log time range. When Kind is
// TimeRelative, Relative is an offset from "now" — typically negative
// (e.g. -24*time.Hour = "24 hours ago"). When Kind is TimeAbsolute,
// Absolute is a concrete wall-clock timestamp.
//
// The zero value (Kind=TimeRelative, Relative=0, Absolute=zero) resolves
// to `now` and is used to represent "no bound" / "up to now". Callers
// check IsZero() rather than inspecting the struct fields directly.
type LogTimePoint struct {
	Kind     TimeKind
	Relative time.Duration
	Absolute time.Time
}

// Resolve returns the wall-clock time this point represents given the
// reference "now". A zero-value LogTimePoint (Kind==0, both fields zero)
// resolves to `now` — the default "no bound" / "up to now" behaviour.
func (p LogTimePoint) Resolve(now time.Time) time.Time {
	if p.Kind == TimeAbsolute {
		return p.Absolute
	}
	return now.Add(p.Relative)
}

// IsZero reports whether this endpoint is the zero value (no explicit
// time set). Both the Kind==TimeRelative + Relative==0 + Absolute.IsZero()
// combination and the Kind==TimeAbsolute + Absolute.IsZero() combination
// are treated as "no bound" so callers can write uninitialised endpoints
// as struct literals without caring which Kind was the zero default.
func (p LogTimePoint) IsZero() bool {
	switch p.Kind {
	case TimeRelative:
		return p.Relative == 0 && p.Absolute.IsZero()
	case TimeAbsolute:
		return p.Absolute.IsZero()
	}
	return true
}

// Display returns a short human-readable form for the title chip:
//
//	relative negative → "-5m", "-24h", "-3d"
//	relative positive → "+5m" (rare, for end in the future)
//	absolute          → "2026-04-17 15:00"
//	zero              → "now"
//
// The relative form mirrors the user-facing mini-language ("-24h" means
// "24 hours ago"). The absolute form uses minute precision so the chip
// stays narrow — seconds are internally tracked but rarely useful in a
// log-viewer title bar.
func (p LogTimePoint) Display() string {
	if p.IsZero() {
		return "now"
	}
	if p.Kind == TimeAbsolute {
		return p.Absolute.Format("2006-01-02 15:04")
	}
	// Relative. Negative values are rendered with a leading "-" and the
	// compact d/h/m/s suffix closest to the magnitude.
	d := p.Relative
	sign := ""
	if d < 0 {
		sign = "-"
		d = -d
	} else {
		sign = "+"
	}
	// Choose the largest unit that cleanly divides the duration. If
	// nothing divides cleanly we fall back to the smallest sub-unit
	// that still has a non-zero magnitude.
	switch {
	case d%(24*time.Hour) == 0:
		return fmt.Sprintf("%s%dd", sign, int(d/(24*time.Hour)))
	case d%time.Hour == 0:
		return fmt.Sprintf("%s%dh", sign, int(d/time.Hour))
	case d%time.Minute == 0:
		return fmt.Sprintf("%s%dm", sign, int(d/time.Minute))
	default:
		return fmt.Sprintf("%s%ds", sign, int(d/time.Second))
	}
}

// LogTimeRange is a pair of endpoints. Either may be zero (meaning "no
// bound" / "up to now"). When both are zero, IsActive returns false.
type LogTimeRange struct {
	Start LogTimePoint
	End   LogTimePoint
}

// IsActive reports whether the range has at least one bound set. An
// inactive range is equivalent to "no time filter at all" and callers
// (stream args, chip rendering) short-circuit on this.
func (r LogTimeRange) IsActive() bool {
	return !r.Start.IsZero() || !r.End.IsZero()
}

// KubectlSinceArg returns the value to pass to kubectl --since-time=
// (preferred when Start is absolute or precisely-resolvable relative)
// or --since= (fallback when Start is relative and the caller would
// rather pass the duration). Returns ("", "") when Start is zero (no
// lower bound).
//
// Callers should prefer --since-time=RFC3339 for stability across
// kubectl restarts: a relative duration re-evaluates each invocation,
// which would drift if the stream reconnects. This helper resolves the
// relative offset against the provided `now` and emits the resulting
// absolute RFC3339 timestamp.
func (r LogTimeRange) KubectlSinceArg(now time.Time) (flag, value string) {
	if r.Start.IsZero() {
		return "", ""
	}
	resolved := r.Start.Resolve(now)
	// Always emit --since-time=<RFC3339> so reconnects don't drift the
	// window. kubectl understands this flag identically for absolute
	// and resolved-relative inputs.
	return "--since-time", resolved.UTC().Format(time.RFC3339)
}

// KeepLine reports whether a log line (by its parsed timestamp) is
// within this range. Lines without a parseable timestamp are KEPT
// (consistent with how the severity detector handles Unknown lines).
// When End is zero, there is no upper bound. When Start is zero, there
// is no lower bound (but in practice kubectl already clipped at Start).
//
// The lower bound is inclusive, the upper bound is inclusive: a line
// exactly at Start or End is kept. This matches how users think about
// "logs from 9:00 to 10:00" — both endpoints are visible.
func (r LogTimeRange) KeepLine(line string, now time.Time) bool {
	ts, _, ok := parseLogLineTimestamp(line)
	if !ok {
		return true
	}
	if !r.Start.IsZero() {
		lower := r.Start.Resolve(now)
		if ts.Before(lower) {
			return false
		}
	}
	if !r.End.IsZero() {
		upper := r.End.Resolve(now)
		if ts.After(upper) {
			return false
		}
	}
	return true
}

// Display returns a short human-readable form for the title chip,
// e.g. "-24h → -1h", "-24h", "2026-04-17 15:00 → +1h", "2026-04-17 15:00".
// Returns "" when the range is not active.
func (r LogTimeRange) Display() string {
	if !r.IsActive() {
		return ""
	}
	switch {
	case !r.Start.IsZero() && !r.End.IsZero():
		return r.Start.Display() + " \u2192 " + r.End.Display()
	case !r.Start.IsZero():
		return r.Start.Display()
	default:
		// End-only range: "… → 2026-04-17 15:00" reads more naturally
		// than bare "2026-04-17 15:00" because the viewer always
		// implies a lower bound of "start of stream".
		return "\u2026 \u2192 " + r.End.Display()
	}
}

// LogTimeRangePreset is one entry in the preset picker.
type LogTimeRangePreset struct {
	Label string
	Range LogTimeRange
}

// DefaultPresets returns the built-in preset list for the picker. Each
// preset has a Label (for the list UI) and a Range it applies. The
// order is chronological from shortest → longest window, then the
// semantic presets (Today, Yesterday, etc.), then the "Custom…" sentinel.
//
// Presets whose Range.Start is absolute (Today, Yesterday) use the
// provided `now` to anchor to the caller's current day. This keeps the
// picker data-dependent without baking the clock into the overlay.
func DefaultPresets() []LogTimeRangePreset {
	return defaultPresetsAt(time.Now())
}

// defaultPresetsAt is the test-friendly variant of DefaultPresets. Tests
// pass a pinned `now` so the Today/Yesterday anchors are deterministic.
func defaultPresetsAt(now time.Time) []LogTimeRangePreset {
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterdayStart := todayStart.Add(-24 * time.Hour)

	rel := func(d time.Duration) LogTimeRange {
		return LogTimeRange{
			Start: LogTimePoint{Kind: TimeRelative, Relative: d},
		}
	}
	return []LogTimeRangePreset{
		{Label: "Last 5m", Range: rel(-5 * time.Minute)},
		{Label: "Last 15m", Range: rel(-15 * time.Minute)},
		{Label: "Last 30m", Range: rel(-30 * time.Minute)},
		{Label: "Last 1h", Range: rel(-1 * time.Hour)},
		{Label: "Last 4h", Range: rel(-4 * time.Hour)},
		{Label: "Last 24h", Range: rel(-24 * time.Hour)},
		{
			Label: "Last 24h excluding last 1h",
			Range: LogTimeRange{
				Start: LogTimePoint{Kind: TimeRelative, Relative: -24 * time.Hour},
				End:   LogTimePoint{Kind: TimeRelative, Relative: -1 * time.Hour},
			},
		},
		{
			Label: "Today",
			Range: LogTimeRange{
				Start: LogTimePoint{Kind: TimeAbsolute, Absolute: todayStart},
			},
		},
		{
			Label: "Yesterday",
			Range: LogTimeRange{
				Start: LogTimePoint{Kind: TimeAbsolute, Absolute: yesterdayStart},
				End:   LogTimePoint{Kind: TimeAbsolute, Absolute: todayStart},
			},
		},
		{Label: "Custom\u2026", Range: LogTimeRange{}},
	}
}
