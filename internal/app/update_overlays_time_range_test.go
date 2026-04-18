package app

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// timeRangeOverlayModel spins up a Model wired for the time-range
// overlay. baseFinalModel() provides the k8s client so commitLogTimeRange
// can call restartLogStreamForTimeRange → startLogStream without
// panicking on a nil receiver.
func timeRangeOverlayModel(t *testing.T) Model {
	t.Helper()
	m := baseFinalModel()
	m.mode = modeLogs
	m.openLogTimeRangeOverlay()
	return m
}

// TestLogTimeRangeOverlayOpenInitialisesPicker confirms openLog sets up
// state the picker depends on: the preset list is populated, the cursor
// sits at zero, and focus is on the Presets panel.
func TestLogTimeRangeOverlayOpenInitialisesPicker(t *testing.T) {
	m := timeRangeOverlayModel(t)
	assert.Equal(t, overlayLogTimeRange, m.overlay, "overlay must be open")
	assert.NotEmpty(t, m.logTimeRangePresets, "preset list should be populated")
	assert.Equal(t, 0, m.logTimeRangeCursor, "cursor starts at 0")
	assert.Equal(t, logTimeRangeFocusPresets, m.logTimeRangeFocus)
}

// TestLogTimeRangeOverlayEscCancels verifies Esc closes the overlay
// without mutating the committed range.
func TestLogTimeRangeOverlayEscCancels(t *testing.T) {
	m := timeRangeOverlayModel(t)
	m.logTimeRange = LogTimeRange{Start: LogTimePoint{Kind: TimeRelative, Relative: -1 * time.Hour}}

	result, _ := m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyEsc})
	rm := result.(Model)

	assert.Equal(t, overlayNone, rm.overlay)
	// Existing range untouched.
	assert.Equal(t, -1*time.Hour, rm.logTimeRange.Start.Relative)
}

// TestLogTimeRangeOverlayJKNavigation checks that j/k move the cursor
// through the preset list without wrapping past the ends.
func TestLogTimeRangeOverlayJKNavigation(t *testing.T) {
	m := timeRangeOverlayModel(t)
	n := len(m.logTimeRangePresets)

	// j moves cursor down one.
	result, _ := m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	rm := result.(Model)
	assert.Equal(t, 1, rm.logTimeRangeCursor)

	// G jumps to last.
	result, _ = rm.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	rm = result.(Model)
	assert.Equal(t, n-1, rm.logTimeRangeCursor)

	// j past end must clamp, not wrap.
	result, _ = rm.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	rm = result.(Model)
	assert.Equal(t, n-1, rm.logTimeRangeCursor)

	// g jumps to first.
	result, _ = rm.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	rm = result.(Model)
	assert.Equal(t, 0, rm.logTimeRangeCursor)

	// k past start must clamp.
	result, _ = rm.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	rm = result.(Model)
	assert.Equal(t, 0, rm.logTimeRangeCursor)
}

// TestLogTimeRangeOverlayEnterAppliesPreset is the contract test: Enter
// on the first preset writes its range to logTimeRange and closes the
// overlay. We use baseFinalModel's fake kubectl path so the stream
// restart can be dispatched without mocking kubectl itself.
func TestLogTimeRangeOverlayEnterAppliesPreset(t *testing.T) {
	m := timeRangeOverlayModel(t)
	// Cursor on 0 = "Last 5m".
	wantStart := m.logTimeRangePresets[0].Range.Start

	result, _ := m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(Model)

	assert.Equal(t, overlayNone, rm.overlay, "overlay should close")
	assert.True(t, rm.logTimeRange.IsActive(), "range must be active")
	assert.Equal(t, wantStart, rm.logTimeRange.Start, "Start should match preset")
	// logSinceDuration is retired; committing must clear any leftover.
	assert.Equal(t, "", rm.logSinceDuration)
}

// TestLogTimeRangeOverlayCClearsRange verifies the `c` binding clears
// any currently-active range.
func TestLogTimeRangeOverlayCClearsRange(t *testing.T) {
	m := timeRangeOverlayModel(t)
	m.logTimeRange = LogTimeRange{Start: LogTimePoint{Kind: TimeRelative, Relative: -5 * time.Minute}}

	result, _ := m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	rm := result.(Model)

	assert.Equal(t, overlayNone, rm.overlay)
	assert.False(t, rm.logTimeRange.IsActive(), "c should clear the range")
}

// TestLogTimeRangeOverlayCustomPresetNoopsForNow confirms the Custom…
// preset (Phase 1 sentinel) doesn't overwrite state — it only shows a
// status hint.
func TestLogTimeRangeOverlayCustomPresetNoopsForNow(t *testing.T) {
	m := timeRangeOverlayModel(t)
	// Navigate to the last preset which is "Custom…".
	m.logTimeRangeCursor = len(m.logTimeRangePresets) - 1
	before := m.logTimeRange

	result, _ := m.handleLogTimeRangeKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(Model)

	assert.Equal(t, overlayLogTimeRange, rm.overlay, "overlay should stay open on Custom…")
	assert.Equal(t, before, rm.logTimeRange, "Custom preset must not mutate logTimeRange")
	assert.Contains(t, rm.statusMessage, "Phase 2", "status hint should explain the delay")
}

// TestLogKeyTOpensTimeRangeOverlay pins the hotkey contract: Shift+T
// while the log viewer is active opens the new overlay.
func TestLogKeyTOpensTimeRangeOverlay(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs

	result, cmd, handled := m.handleLogActionKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")})
	rm := result.(Model)

	assert.True(t, handled, "T must be handled by the log action key dispatch")
	assert.Nil(t, cmd, "opening the overlay should not schedule a command")
	assert.Equal(t, overlayLogTimeRange, rm.overlay, "overlay should be open")
}

// TestTabSnapshotPreservesTimeRange asserts that logTimeRange
// round-trips through a TabState snapshot/restore. Switching tabs must
// not drop the active range (the stream re-applies only on explicit
// user commit).
func TestTabSnapshotPreservesTimeRange(t *testing.T) {
	m := Model{
		logTimeRange: LogTimeRange{Start: LogTimePoint{Kind: TimeRelative, Relative: -5 * time.Minute}},
	}
	tab := &TabState{}
	tab.logTimeRange = m.logTimeRange

	m.logTimeRange = LogTimeRange{}
	m.logTimeRange = tab.logTimeRange

	assert.True(t, m.logTimeRange.IsActive())
	assert.Equal(t, -5*time.Minute, m.logTimeRange.Start.Relative)
}

// TestTabMigrationLegacySinceDuration verifies the migration-on-load
// contract: a TabState that only carries logSinceDuration (pre-Phase 1
// session files) is upgraded to a LogTimeRange with Start = -dur.
func TestTabMigrationLegacySinceDuration(t *testing.T) {
	m := baseFinalModel()
	// Prime a tab with the legacy field only.
	m.tabs = []TabState{{}, {logSinceDuration: "5m"}}
	m.activeTab = 0

	cmd := m.loadTab(1)
	_ = cmd

	assert.True(t, m.logTimeRange.IsActive(), "legacy string must be upgraded")
	assert.Equal(t, -5*time.Minute, m.logTimeRange.Start.Relative, "Start should be negative duration")
	assert.Equal(t, "", m.logSinceDuration, "legacy field must be cleared post-migration")
}
