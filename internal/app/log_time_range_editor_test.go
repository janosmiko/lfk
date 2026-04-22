package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestEditorFromPointZero sets up a fresh editor from a zero endpoint
// and verifies the defaults the spec asks for: Mode=Now, a 24h seed
// ready in the spinner for when the user toggles Mode=Relative.
func TestEditorFromPointZero(t *testing.T) {
	defaultAbs := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	ed := editorFromPoint(LogTimePoint{}, defaultAbs)
	assert.Equal(t, logTimeRangeModeNow, ed.Mode)
	assert.Equal(t, 24, ed.RelHrs, "Relative seed should default to 24h")
	assert.Equal(t, defaultAbs, ed.AbsTime)
}

// TestEditorFromPointRelativeSplits verifies a relative point is
// decomposed back into d/h/m fields so re-opening the overlay with
// a pre-existing Relative endpoint surfaces the exact values.
func TestEditorFromPointRelativeSplits(t *testing.T) {
	// -2d 3h 15m = 51h 15m
	p := LogTimePoint{Kind: TimeRelative, Relative: -(2*24*time.Hour + 3*time.Hour + 15*time.Minute)}
	ed := editorFromPoint(p, time.Now())
	assert.Equal(t, logTimeRangeModeRelative, ed.Mode)
	assert.Equal(t, 2, ed.RelDays)
	assert.Equal(t, 3, ed.RelHrs)
	assert.Equal(t, 15, ed.RelMin)
}

// TestEditorToPointRelative verifies the spinner values re-compose
// into a single negative LogTimePoint.
func TestEditorToPointRelative(t *testing.T) {
	ed := logTimeRangeEditor{
		Mode:    logTimeRangeModeRelative,
		RelDays: 1,
		RelHrs:  2,
		RelMin:  30,
	}
	p := ed.toPoint()
	assert.Equal(t, TimeRelative, p.Kind)
	// 1d 2h 30m = 26h 30m = 95400s
	want := -(24*time.Hour + 2*time.Hour + 30*time.Minute)
	assert.Equal(t, want, p.Relative)
}

// TestEditorToPointZeroRelativeFallsBackToInactive makes sure a
// Relative editor with every field at zero produces an inactive
// endpoint — the "just don't set an End" shortcut.
func TestEditorToPointZeroRelativeFallsBackToInactive(t *testing.T) {
	ed := logTimeRangeEditor{Mode: logTimeRangeModeRelative}
	p := ed.toPoint()
	assert.True(t, p.IsZero())
}

// TestRelFieldAdjustClamps pins the clamp rules: days cap at 999,
// hours at 23, minutes at 59, nothing goes below zero.
func TestRelFieldAdjustClamps(t *testing.T) {
	ed := logTimeRangeEditor{Mode: logTimeRangeModeRelative, RelHrs: 23}
	ed.RelField = logTimeRangeRelFieldHours
	ed.relFieldAdjust(1)
	assert.Equal(t, 23, ed.RelHrs, "hours must cap at 23")

	ed.relFieldAdjust(-1)
	assert.Equal(t, 22, ed.RelHrs)

	ed.RelField = logTimeRangeRelFieldMinutes
	ed.RelMin = 0
	ed.relFieldAdjust(-1)
	assert.Equal(t, 0, ed.RelMin, "minutes must clamp at zero")

	ed.RelField = logTimeRangeRelFieldDays
	ed.RelDays = 999
	ed.relFieldAdjust(1)
	assert.Equal(t, 999, ed.RelDays, "days cap at 999")
}

// TestRelFieldSetDigitOverwrites confirms typing a digit replaces the
// value outright rather than appending.
func TestRelFieldSetDigitOverwrites(t *testing.T) {
	ed := logTimeRangeEditor{Mode: logTimeRangeModeRelative, RelHrs: 12}
	ed.RelField = logTimeRangeRelFieldHours
	ed.relFieldSetDigit(5)
	assert.Equal(t, 5, ed.RelHrs, "digit should overwrite, not append")
}

// TestRelFieldNavClamps verifies h/l navigation clamps at the first
// and last fields rather than wrapping.
func TestRelFieldNavClamps(t *testing.T) {
	ed := logTimeRangeEditor{Mode: logTimeRangeModeRelative}
	ed.relFieldLeft()
	assert.Equal(t, logTimeRangeRelFieldDays, ed.RelField, "left at days clamps")

	ed.RelField = logTimeRangeRelFieldMinutes
	ed.relFieldRight()
	assert.Equal(t, logTimeRangeRelFieldMinutes, ed.RelField, "right at minutes clamps")
}
