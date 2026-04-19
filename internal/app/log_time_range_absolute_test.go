package app

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// TestAbsFieldNavigation pins h/l clamping behaviour at the ends.
func TestAbsFieldNavigation(t *testing.T) {
	ed := logTimeRangeEditor{
		Mode:    logTimeRangeModeAbsolute,
		AbsTime: time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
	}
	ed.absFieldLeft()
	assert.Equal(t, logTimeRangeAbsFieldYear, ed.AbsField, "left at year clamps")

	ed.AbsField = logTimeRangeAbsFieldSecond
	ed.absFieldRight()
	assert.Equal(t, logTimeRangeAbsFieldSecond, ed.AbsField, "right at second clamps")

	ed.AbsField = logTimeRangeAbsFieldYear
	ed.absFieldRight()
	assert.Equal(t, logTimeRangeAbsFieldMonth, ed.AbsField)
}

// TestAbsAdjustHourWraps covers the "25:00 → 00:00 next day" spec case:
// bumping hour 23 → 24 should roll over to the next day at 00:00 and
// return a status hint.
func TestAbsAdjustHourWraps(t *testing.T) {
	ed := logTimeRangeEditor{
		Mode:     logTimeRangeModeAbsolute,
		AbsTime:  time.Date(2026, 4, 17, 23, 0, 0, 0, time.UTC),
		AbsField: logTimeRangeAbsFieldHour,
	}
	status := ed.absFieldAdjust(1)
	assert.Equal(t, 18, ed.AbsTime.Day(), "day should advance on hour overflow")
	assert.Equal(t, 0, ed.AbsTime.Hour())
	assert.NotEmpty(t, status, "wrap should produce a status hint")
}

// TestAbsAdjustDayClampsFebruary covers the "Feb 30 → Feb 28" spec case.
// Setting day=30 on a February start must clamp down rather than
// spilling into March.
func TestAbsAdjustDayClampsFebruary(t *testing.T) {
	// 2026 is a non-leap year → Feb 28 days.
	ed := logTimeRangeEditor{
		Mode:     logTimeRangeModeAbsolute,
		AbsTime:  time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		AbsField: logTimeRangeAbsFieldDay,
	}
	status := ed.absFieldOverwriteDigit(9) // day=9, valid
	assert.Equal(t, "", status)
	assert.Equal(t, 9, ed.AbsTime.Day())

	// Now bump day by 21 so we try to reach 30.
	status = ed.absFieldAdjust(21)
	assert.Equal(t, 28, ed.AbsTime.Day(), "day should clamp to 28 in February")
	assert.Equal(t, time.February, ed.AbsTime.Month())
	assert.NotEmpty(t, status, "clamp should produce a status hint")
}

// TestAbsAdjustMonthClampsDayOnShrink covers the "switch to February
// while day=30" path. Month change must drag the day down.
func TestAbsAdjustMonthClampsDayOnShrink(t *testing.T) {
	// Start Jan 31, adjust month to February.
	ed := logTimeRangeEditor{
		Mode:     logTimeRangeModeAbsolute,
		AbsTime:  time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC),
		AbsField: logTimeRangeAbsFieldMonth,
	}
	status := ed.absFieldAdjust(1) // Jan → Feb
	assert.Equal(t, time.February, ed.AbsTime.Month())
	assert.Equal(t, 28, ed.AbsTime.Day(), "Feb must clamp day from 31 → 28 in non-leap year")
	assert.NotEmpty(t, status)
}

// TestAbsAdjustMonthClampInBounds makes sure month clamps at 1..12.
func TestAbsAdjustMonthClampInBounds(t *testing.T) {
	ed := logTimeRangeEditor{
		Mode:     logTimeRangeModeAbsolute,
		AbsTime:  time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		AbsField: logTimeRangeAbsFieldMonth,
	}
	ed.absFieldAdjust(1) // trying to go to month 13
	assert.Equal(t, time.December, ed.AbsTime.Month(), "month caps at 12")

	ed.AbsTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ed.absFieldAdjust(-1) // trying to go to month 0
	assert.Equal(t, time.January, ed.AbsTime.Month(), "month floor at 1")
}

// TestAbsOverwriteDigitMonth covers typing a digit onto a month cell:
// single-digit replacement, clamped to 1..12.
func TestAbsOverwriteDigitMonth(t *testing.T) {
	ed := logTimeRangeEditor{
		Mode:     logTimeRangeModeAbsolute,
		AbsTime:  time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC),
		AbsField: logTimeRangeAbsFieldMonth,
	}
	ed.absFieldOverwriteDigit(9) // Month=9 (September)
	assert.Equal(t, time.September, ed.AbsTime.Month())

	// 0 should float up to 1 (there is no month 0).
	ed.absFieldOverwriteDigit(0)
	assert.Equal(t, time.January, ed.AbsTime.Month())
}

// TestAbsoluteEditorEnterCommits exercises the end-to-end: user opens
// the overlay, picks Custom, switches Start mode to Absolute via
// Space, then hits Enter twice (from Start → End → commit). The
// resulting logTimeRange must carry the absolute start value and
// close the overlay.
func TestAbsoluteEditorEnterCommits(t *testing.T) {
	m := timeRangeOverlayModel(t)
	// Jump to Custom preset.
	m.logTimeRangeCursor = len(m.logTimeRangePresets) - 1
	result, _ := m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)
	assert.Equal(t, logTimeRangeFocusStart, m.logTimeRangeFocus)

	// Cycle Mode from Relative to Absolute via Space (relative → absolute).
	result, _ = m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = result.(Model)
	assert.Equal(t, logTimeRangeModeAbsolute, m.logTimeRangeStart.Mode)

	// Enter from Start → End focus, pending Start set.
	result, _ = m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)
	assert.Equal(t, logTimeRangeFocusEnd, m.logTimeRangeFocus)
	assert.Equal(t, TimeAbsolute, m.logTimeRangePendingRange.Start.Kind)

	// Enter from End → commits. End editor is Mode=Now by default so
	// the End endpoint stays zero (no upper bound).
	result, _ = m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)
	assert.Equal(t, overlayNone, m.overlay)
	assert.True(t, m.logTimeRange.IsActive())
	assert.Equal(t, TimeAbsolute, m.logTimeRange.Start.Kind)
}

// TestAbsoluteEditorStatusHintOnClamp confirms the clamp status is
// surfaced to the status bar so the user knows why their typed value
// moved.
func TestAbsoluteEditorStatusHintOnClamp(t *testing.T) {
	m := timeRangeOverlayModel(t)
	m.logTimeRangeFocus = logTimeRangeFocusStart
	m.logTimeRangeStart = logTimeRangeEditor{
		Mode:     logTimeRangeModeAbsolute,
		AbsTime:  time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		AbsField: logTimeRangeAbsFieldDay,
	}
	// Jump day to 30 via the +29 nudge.
	for range 29 {
		result, _ := m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
		m = result.(Model)
	}
	// Feb caps at 28, so clamp status should have fired.
	assert.Equal(t, 28, m.logTimeRangeStart.AbsTime.Day())
	assert.Contains(t, m.statusMessage, "day clamped", "clamp must surface a status hint")
}

// TestDaysInMonth covers the leap-year edge cases.
func TestDaysInMonth(t *testing.T) {
	assert.Equal(t, 29, daysInMonth(2024, time.February), "2024 is a leap year")
	assert.Equal(t, 28, daysInMonth(2026, time.February), "2026 is not a leap year")
	assert.Equal(t, 31, daysInMonth(2026, time.January))
	assert.Equal(t, 30, daysInMonth(2026, time.April))
}
