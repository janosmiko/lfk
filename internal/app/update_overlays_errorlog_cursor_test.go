package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
)

// Pressing 'd' toggles the debug filter, which changes the visible entry set
// and therefore the index of the selected entry. The cursor must follow the
// entry it was on rather than jumping back to the top.
func TestErrorLogDebugTogglePreservesCursor(t *testing.T) {
	// Chronological order; FilteredErrorLogEntries reverses to newest-first.
	entries := []ui.ErrorLogEntry{
		{Level: "ERR", Message: "error 1"},
		{Level: "INF", Message: "info 1"},
		{Level: "ERR", Message: "error 2"},
		{Level: "DBG", Message: "debug 1"},
		{Level: "ERR", Message: "error 3"},
	}
	// Reversed, debug hidden: [error 3, error 2, info 1, error 1]
	// Reversed, debug shown:  [error 3, debug 1, error 2, info 1, error 1]

	t.Run("toggle on keeps selected non-debug entry", func(t *testing.T) {
		m := Model{
			overlayErrorLog:    true,
			errorLog:           entries,
			showDebugLogs:      false,
			errorLogCursorLine: 1, // "error 2"
			tabs:               []TabState{{}},
			width:              80,
			height:             40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('d'))
		result := ret.(Model)
		assert.True(t, result.showDebugLogs)
		assert.Equal(t, 2, result.errorLogCursorLine, "cursor should follow 'error 2' to its new index")
	})

	t.Run("toggle off keeps selected non-debug entry", func(t *testing.T) {
		m := Model{
			overlayErrorLog:    true,
			errorLog:           entries,
			showDebugLogs:      true,
			errorLogCursorLine: 2, // "error 2"
			tabs:               []TabState{{}},
			width:              80,
			height:             40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('d'))
		result := ret.(Model)
		assert.False(t, result.showDebugLogs)
		assert.Equal(t, 1, result.errorLogCursorLine, "cursor should follow 'error 2' to its new index")
	})

	t.Run("toggle off from a debug line lands on nearest surviving entry", func(t *testing.T) {
		m := Model{
			overlayErrorLog:    true,
			errorLog:           entries,
			showDebugLogs:      true,
			errorLogCursorLine: 1, // "debug 1" (will disappear)
			tabs:               []TabState{{}},
			width:              80,
			height:             40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('d'))
		result := ret.(Model)
		assert.False(t, result.showDebugLogs)
		// "debug 1" is gone; the nearest surviving entry is "error 2" at new index 1.
		assert.Equal(t, 1, result.errorLogCursorLine)
	})

	t.Run("empty log resets to top without panic", func(t *testing.T) {
		m := Model{
			overlayErrorLog:    true,
			errorLog:           nil,
			showDebugLogs:      false,
			errorLogCursorLine: 0,
			tabs:               []TabState{{}},
			width:              80,
			height:             40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('d'))
		result := ret.(Model)
		assert.Equal(t, 0, result.errorLogCursorLine)
		assert.Equal(t, 0, result.errorLogScroll)
	})
}
