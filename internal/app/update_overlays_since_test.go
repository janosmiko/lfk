package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// sinceOverlayModel returns a Model configured for tests that exercise
// the --since overlay.  It uses baseFinalModel so m.client is non-nil
// and restartLogStreamForSince can call startLogStream without
// panicking on a nil receiver.
func sinceOverlayModel() Model {
	m := baseFinalModel()
	m.overlay = overlayLogSinceInput
	m.logSinceInput.Clear()
	return m
}

// typeInto feeds each rune of s into the overlay as a single-key
// keystroke, returning the model after all inputs have been processed.
func typeInto(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		result, _ := m.handleLogSinceInputKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(Model)
	}
	return m
}

// TestSinceOverlayEnterCommits verifies that typing a valid duration
// and pressing Enter updates logSinceDuration and closes the overlay.
func TestSinceOverlayEnterCommits(t *testing.T) {
	m := sinceOverlayModel()
	m = typeInto(t, m, "5m")

	result, _ := m.handleLogSinceInputKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(Model)

	assert.Equal(t, "5m", rm.logSinceDuration, "valid duration should persist on the model")
	assert.Equal(t, overlayNone, rm.overlay, "overlay should close on commit")
	assert.Equal(t, "", rm.logSinceInput.Value, "input should clear after commit")
}

// TestSinceOverlayEnterEmptyClears verifies that pressing Enter with an
// empty input clears an existing logSinceDuration.
func TestSinceOverlayEnterEmptyClears(t *testing.T) {
	m := sinceOverlayModel()
	m.logSinceDuration = "10m" // pretend a filter is already active

	result, _ := m.handleLogSinceInputKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(Model)

	assert.Equal(t, "", rm.logSinceDuration, "empty Enter should clear the filter")
	assert.Equal(t, overlayNone, rm.overlay, "overlay should close")
}

// TestSinceOverlayEscCancels verifies that Esc closes the overlay
// without touching logSinceDuration, even when the input has text.
func TestSinceOverlayEscCancels(t *testing.T) {
	m := sinceOverlayModel()
	m.logSinceDuration = "1h"
	m = typeInto(t, m, "5m")

	result, _ := m.handleLogSinceInputKey(tea.KeyMsg{Type: tea.KeyEsc})
	rm := result.(Model)

	assert.Equal(t, "1h", rm.logSinceDuration, "existing value untouched by Esc")
	assert.Equal(t, overlayNone, rm.overlay, "overlay should close on Esc")
	assert.Equal(t, "", rm.logSinceInput.Value, "input should clear on Esc")
}

// TestLogKeyTOpensSinceOverlay verifies that pressing `t` while the
// log viewer is active opens the --since prompt with a cleared input,
// without disturbing the existing logSinceDuration.
func TestLogKeyTOpensSinceOverlay(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	m.logSinceDuration = "1h"
	m.logSinceInput.Set("stale") // should be cleared by the hotkey

	result, cmd, handled := m.handleLogActionKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	rm := result.(Model)

	assert.True(t, handled, "t must be handled by the log action key dispatch")
	assert.Nil(t, cmd, "opening the overlay should not schedule a command")
	assert.Equal(t, overlayLogSinceInput, rm.overlay, "overlay should be open")
	assert.Equal(t, "", rm.logSinceInput.Value, "input should be cleared on open")
	assert.Equal(t, "1h", rm.logSinceDuration, "existing duration must be preserved until commit")
}

// TestSinceOverlayInvalidShowsError verifies that Enter with an invalid
// duration shows a status message and keeps existing state intact.
func TestSinceOverlayInvalidShowsError(t *testing.T) {
	m := sinceOverlayModel()
	m.logSinceDuration = "" // nothing active
	m = typeInto(t, m, "abc")

	result, _ := m.handleLogSinceInputKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(Model)

	assert.Equal(t, "", rm.logSinceDuration, "invalid input must not mutate logSinceDuration")
	assert.Equal(t, overlayLogSinceInput, rm.overlay,
		"overlay should stay open after invalid input so user can correct")
	assert.True(t, rm.statusMessageErr, "an error status message should be set")
	assert.Contains(t, rm.statusMessage, "Invalid duration", "status mentions the failure reason")
}
