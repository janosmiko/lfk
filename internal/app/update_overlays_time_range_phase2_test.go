package app

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// TestLogTimeRangeTabCyclesFocus exercises the global Tab / Shift+Tab
// handling: focus cycles Presets → Start → End → Presets.
func TestLogTimeRangeTabCyclesFocus(t *testing.T) {
	m := timeRangeOverlayModel(t)

	cycle := []logTimeRangeFocus{
		logTimeRangeFocusStart,
		logTimeRangeFocusEnd,
		logTimeRangeFocusPresets,
	}
	for _, want := range cycle {
		result, _ := m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyTab})
		m = result.(Model)
		assert.Equal(t, want, m.logTimeRangeFocus)
	}

	// Shift+Tab rewinds.
	result, _ := m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = result.(Model)
	assert.Equal(t, logTimeRangeFocusEnd, m.logTimeRangeFocus)
}

// TestLogTimeRangeCustomPresetFocusesStart confirms Enter on the
// Custom… preset switches focus to the Start editor instead of
// closing the overlay.
func TestLogTimeRangeCustomPresetFocusesStart(t *testing.T) {
	m := timeRangeOverlayModel(t)
	m.logTimeRangeCursor = len(m.logTimeRangePresets) - 1 // Custom…
	before := m.logTimeRange

	result, _ := m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(Model)

	assert.Equal(t, overlayLogTimeRange, rm.overlay, "overlay stays open on Custom…")
	assert.Equal(t, logTimeRangeFocusStart, rm.logTimeRangeFocus)
	assert.Equal(t, logTimeRangeModeRelative, rm.logTimeRangeStart.Mode, "Custom seeds Relative mode")
	assert.Equal(t, before, rm.logTimeRange, "committed range untouched")
}

// TestLogTimeRangeEditorAdjustsSpinner walks through the key path a
// user takes after Custom: j lowers the hour spinner, l moves to
// minutes, k bumps it. The final Relative value matches the editor
// state.
func TestLogTimeRangeEditorAdjustsSpinner(t *testing.T) {
	m := timeRangeOverlayModel(t)
	m.logTimeRangeFocus = logTimeRangeFocusStart
	m.logTimeRangeStart = logTimeRangeEditor{
		Mode:     logTimeRangeModeRelative,
		RelHrs:   5,
		RelField: logTimeRangeRelFieldHours,
	}

	// j lowers hours 5 → 4.
	result, _ := m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = result.(Model)
	assert.Equal(t, 4, m.logTimeRangeStart.RelHrs)

	// l moves to minutes.
	result, _ = m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = result.(Model)
	assert.Equal(t, logTimeRangeRelFieldMinutes, m.logTimeRangeStart.RelField)

	// + raises minutes 0 → 1.
	result, _ = m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	m = result.(Model)
	assert.Equal(t, 1, m.logTimeRangeStart.RelMin)

	// Typing '9' overwrites to 9.
	result, _ = m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}})
	m = result.(Model)
	assert.Equal(t, 9, m.logTimeRangeStart.RelMin)
}

// TestLogTimeRangeEditorEnterAdvances confirms Enter from the Start
// editor commits the Start value to the pending range and advances
// focus to the End editor. Enter from End commits the range and
// closes the overlay.
func TestLogTimeRangeEditorEnterAdvances(t *testing.T) {
	m := timeRangeOverlayModel(t)
	m.logTimeRangeFocus = logTimeRangeFocusStart
	m.logTimeRangeStart = logTimeRangeEditor{
		Mode:   logTimeRangeModeRelative,
		RelHrs: 2,
	}
	m.logTimeRangeEnd = logTimeRangeEditor{Mode: logTimeRangeModeNow}

	// Enter from Start → focus moves to End, range not yet committed.
	result, _ := m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)
	assert.Equal(t, logTimeRangeFocusEnd, m.logTimeRangeFocus)
	assert.False(t, m.logTimeRange.IsActive(), "range not committed yet")

	// Enter from End → overlay closes and range is applied.
	result, _ = m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)
	assert.Equal(t, overlayNone, m.overlay, "overlay closed on End Enter")
	assert.True(t, m.logTimeRange.IsActive())
	assert.Equal(t, -2*time.Hour, m.logTimeRange.Start.Relative)
}

// TestLogTimeRangeEditorEscDropsToPresets confirms Esc from an editor
// panel returns focus to the preset column without committing.
func TestLogTimeRangeEditorEscDropsToPresets(t *testing.T) {
	m := timeRangeOverlayModel(t)
	m.logTimeRangeFocus = logTimeRangeFocusStart

	result, _ := m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(Model)

	assert.Equal(t, overlayLogTimeRange, m.overlay, "overlay remains open")
	assert.Equal(t, logTimeRangeFocusPresets, m.logTimeRangeFocus)
}

// TestLogTimeRangeLivePreview verifies that buildLogTimeRangeOverlayState
// reflects the editor values in the preview strip when focus is on
// an editor panel, and reflects the preset cursor value while focus
// is on the preset column.
func TestLogTimeRangeLivePreview(t *testing.T) {
	m := timeRangeOverlayModel(t)

	// Presets focus: preview is the preset under the cursor.
	state := m.buildLogTimeRangeOverlayState()
	assert.NotEmpty(t, state.ActiveRangeDisplay, "Presets focus should show preset chip")

	// Move focus to Start, set a non-zero relative offset, and check
	// the preview now reflects the editor values.
	m.logTimeRangeFocus = logTimeRangeFocusStart
	m.logTimeRangeStart = logTimeRangeEditor{Mode: logTimeRangeModeRelative, RelHrs: 3}
	state = m.buildLogTimeRangeOverlayState()
	assert.Contains(t, state.ActiveRangeDisplay, "-3h", "Editor focus preview should render editor value")
}
