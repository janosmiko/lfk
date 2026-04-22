package app

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestLogTimePointResolveRelative pins the relative-offset contract: a
// LogTimePoint with Kind=TimeRelative and Relative=-1h resolves to
// (now - 1h), independent of the system clock.
func TestLogTimePointResolveRelative(t *testing.T) {
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	p := LogTimePoint{Kind: TimeRelative, Relative: -1 * time.Hour}
	got := p.Resolve(now)
	want := now.Add(-1 * time.Hour)
	assert.Equal(t, want, got, "relative resolution should apply offset to provided now")
}

// TestLogTimePointResolveAbsolute verifies absolute points return the
// stored Absolute value verbatim, ignoring `now`.
func TestLogTimePointResolveAbsolute(t *testing.T) {
	abs := time.Date(2026, 4, 16, 9, 30, 0, 0, time.UTC)
	p := LogTimePoint{Kind: TimeAbsolute, Absolute: abs}
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, abs, p.Resolve(now))
}

// TestLogTimePointResolveZeroEqualsNow covers the "no bound" sentinel:
// a zero value must resolve back to `now` so stream args skip --since.
func TestLogTimePointResolveZeroEqualsNow(t *testing.T) {
	var p LogTimePoint
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, now, p.Resolve(now))
	assert.True(t, p.IsZero())
}

// TestLogTimePointDisplay exercises every display branch so the title
// chip never shows a surprising string. One case per branch is enough
// because Display is pure.
func TestLogTimePointDisplay(t *testing.T) {
	cases := []struct {
		name string
		p    LogTimePoint
		want string
	}{
		{"zero", LogTimePoint{}, "now"},
		{"rel -5m", LogTimePoint{Kind: TimeRelative, Relative: -5 * time.Minute}, "-5m"},
		{"rel -24h", LogTimePoint{Kind: TimeRelative, Relative: -24 * time.Hour}, "-1d"},
		{"rel -1h", LogTimePoint{Kind: TimeRelative, Relative: -1 * time.Hour}, "-1h"},
		{"rel +5m", LogTimePoint{Kind: TimeRelative, Relative: 5 * time.Minute}, "+5m"},
		{"rel -30s", LogTimePoint{Kind: TimeRelative, Relative: -30 * time.Second}, "-30s"},
		{"abs", LogTimePoint{Kind: TimeAbsolute, Absolute: time.Date(2026, 4, 17, 15, 0, 0, 0, time.UTC)}, "2026-04-17 15:00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.p.Display())
		})
	}
}

// TestLogTimeRangeIsActive covers the three cases: both zero → inactive,
// either endpoint set → active.
func TestLogTimeRangeIsActive(t *testing.T) {
	zero := LogTimeRange{}
	assert.False(t, zero.IsActive(), "both endpoints zero ⇒ inactive")

	startOnly := LogTimeRange{Start: LogTimePoint{Kind: TimeRelative, Relative: -1 * time.Hour}}
	assert.True(t, startOnly.IsActive())

	endOnly := LogTimeRange{End: LogTimePoint{Kind: TimeRelative, Relative: -1 * time.Hour}}
	assert.True(t, endOnly.IsActive())
}

// TestLogTimeRangeKubectlSinceArg enforces the invariant that the stream
// always gets --since-time=RFC3339 (UTC) even for relative starts, so
// kubectl reconnects don't silently shift the window.
func TestLogTimeRangeKubectlSinceArg(t *testing.T) {
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

	t.Run("no start → no flag", func(t *testing.T) {
		var r LogTimeRange
		flag, value := r.KubectlSinceArg(now)
		assert.Equal(t, "", flag)
		assert.Equal(t, "", value)
	})

	t.Run("relative start resolves to --since-time RFC3339", func(t *testing.T) {
		r := LogTimeRange{Start: LogTimePoint{Kind: TimeRelative, Relative: -1 * time.Hour}}
		flag, value := r.KubectlSinceArg(now)
		assert.Equal(t, "--since-time", flag)
		// 1h before the pinned now is 11:00 UTC.
		assert.Equal(t, "2026-04-17T11:00:00Z", value)
	})

	t.Run("absolute start → exact RFC3339 in UTC", func(t *testing.T) {
		r := LogTimeRange{Start: LogTimePoint{Kind: TimeAbsolute, Absolute: time.Date(2026, 4, 17, 9, 30, 0, 0, time.UTC)}}
		flag, value := r.KubectlSinceArg(now)
		assert.Equal(t, "--since-time", flag)
		assert.Equal(t, "2026-04-17T09:30:00Z", value)
	})
}

// TestLogTimeRangeKeepLine covers the three interesting cases: line
// inside, line outside, line without a parseable timestamp (kept).
func TestLogTimeRangeKeepLine(t *testing.T) {
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	start := LogTimePoint{Kind: TimeRelative, Relative: -1 * time.Hour}
	end := LogTimePoint{Kind: TimeRelative, Relative: -30 * time.Minute}
	r := LogTimeRange{Start: start, End: end}

	// In window: 11:30 (now - 30m, exactly at End, inclusive).
	inside := "2026-04-17T11:30:00Z log at edge"
	assert.True(t, r.KeepLine(inside, now))

	// Outside lower bound: 10:00 (before Start).
	belowLower := "2026-04-17T10:00:00Z too old"
	assert.False(t, r.KeepLine(belowLower, now))

	// Outside upper bound: 11:45 (after End).
	aboveUpper := "2026-04-17T11:45:00Z too new"
	assert.False(t, r.KeepLine(aboveUpper, now))

	// No timestamp → kept (consistent with how unknown lines survive
	// severity filtering).
	assert.True(t, r.KeepLine("random noise no timestamp", now))
}

// TestLogTimeRangeDisplay covers the four display branches.
func TestLogTimeRangeDisplay(t *testing.T) {
	assert.Equal(t, "", LogTimeRange{}.Display())

	startOnly := LogTimeRange{Start: LogTimePoint{Kind: TimeRelative, Relative: -24 * time.Hour}}
	assert.Equal(t, "-1d", startOnly.Display())

	bothRel := LogTimeRange{
		Start: LogTimePoint{Kind: TimeRelative, Relative: -24 * time.Hour},
		End:   LogTimePoint{Kind: TimeRelative, Relative: -1 * time.Hour},
	}
	// The arrow is unicode 0x2192 ("→"). Using Contains so the exact
	// glyph doesn't have to be pasted into the assertion string.
	out := bothRel.Display()
	assert.True(t, strings.Contains(out, "-1d"))
	assert.True(t, strings.Contains(out, "-1h"))

	endOnly := LogTimeRange{End: LogTimePoint{Kind: TimeRelative, Relative: -1 * time.Hour}}
	out2 := endOnly.Display()
	assert.True(t, strings.Contains(out2, "-1h"))
	// Leading ellipsis signals the implicit "start of stream" bound.
	assert.True(t, strings.Contains(out2, "\u2026"))
}

// TestDefaultPresetsLabels spot-checks that the user-requested presets
// are present and in the expected order. The "Last 24h excluding last
// 1h" preset is called out explicitly.
func TestDefaultPresetsLabels(t *testing.T) {
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	presets := defaultPresetsAt(now)
	labels := make([]string, len(presets))
	for i, p := range presets {
		labels[i] = p.Label
	}

	// Presence checks for the named presets in the prompt.
	wantSubstrings := []string{"Last 5m", "Last 24h", "Today", "Yesterday", "Last 24h excluding last 1h", "Custom"}
	for _, want := range wantSubstrings {
		found := false
		for _, got := range labels {
			if strings.Contains(got, want) {
				found = true
				break
			}
		}
		assert.Truef(t, found, "preset list missing %q, got %v", want, labels)
	}
}

// TestDefaultPresetsTodayAnchors pins the Today preset to the local
// midnight of the caller's clock. This is the test that catches a
// regression where Today leaks UTC midnight or yesterday's anchor.
func TestDefaultPresetsTodayAnchors(t *testing.T) {
	now := time.Date(2026, 4, 17, 14, 0, 0, 0, time.UTC)
	presets := defaultPresetsAt(now)
	var today LogTimeRangePreset
	for _, p := range presets {
		if p.Label == "Today" {
			today = p
			break
		}
	}
	// Today must have absolute Start = midnight of the day of `now`, End zero.
	assert.Equal(t, TimeAbsolute, today.Range.Start.Kind)
	wantStart := time.Date(2026, 4, 17, 0, 0, 0, 0, now.Location())
	assert.Equal(t, wantStart, today.Range.Start.Absolute)
	assert.True(t, today.Range.End.IsZero())
}

// TestDefaultPresetsLast24hExcludingLast1h pins the exact bounds of the
// user-specified "last 24h but skip the last hour" preset.
func TestDefaultPresetsLast24hExcludingLast1h(t *testing.T) {
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	presets := defaultPresetsAt(now)
	var window LogTimeRangePreset
	for _, p := range presets {
		if p.Label == "Last 24h excluding last 1h" {
			window = p
			break
		}
	}
	assert.Equal(t, TimeRelative, window.Range.Start.Kind)
	assert.Equal(t, -24*time.Hour, window.Range.Start.Relative)
	assert.Equal(t, TimeRelative, window.Range.End.Kind)
	assert.Equal(t, -1*time.Hour, window.Range.End.Relative)
}
