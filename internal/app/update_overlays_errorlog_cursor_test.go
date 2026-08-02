package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
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

// The cursor column must be movable in NORMAL mode (not only char-visual),
// matching the event viewer — the block cursor on the cursor line moves with
// h/l/0/$ without first pressing v.
func TestErrorLogHorizontalCursorInNormalMode(t *testing.T) {
	entries := []ui.ErrorLogEntry{
		{Level: "ERR", Message: "hello world"},
	}
	lineLen := len([]rune(ui.ErrorLogEntryPlainText(entries[0]))) // "HH:MM:SS ERR hello world"
	base := func() Model {
		return Model{overlayErrorLog: true, errorLog: entries}
	}

	t.Run("l moves right in normal mode", func(t *testing.T) {
		m := base()
		assert.Equal(t, 1, m.handleErrorLogOverlayKeyL().errorLogCursorCol)
	})

	t.Run("h moves left in normal mode", func(t *testing.T) {
		m := base()
		m.errorLogCursorCol = 3
		assert.Equal(t, 2, m.handleErrorLogOverlayKeyH().errorLogCursorCol)
	})

	t.Run("h clamps at column 0", func(t *testing.T) {
		m := base()
		assert.Equal(t, 0, m.handleErrorLogOverlayKeyH().errorLogCursorCol)
	})

	t.Run("l clamps at line end", func(t *testing.T) {
		m := base()
		m.errorLogCursorCol = lineLen - 1
		assert.Equal(t, lineLen-1, m.handleErrorLogOverlayKeyL().errorLogCursorCol)
	})

	t.Run("0 jumps to start in normal mode", func(t *testing.T) {
		m := base()
		m.errorLogCursorCol = 5
		assert.Equal(t, 0, m.handleErrorLogOverlayKeyZero().errorLogCursorCol)
	})

	t.Run("dollar jumps to end in normal mode", func(t *testing.T) {
		m := base()
		assert.Equal(t, lineLen-1, m.handleErrorLogOverlayKeyDollar().errorLogCursorCol)
	})

	t.Run("0 with pending line input appends digit instead of moving", func(t *testing.T) {
		m := base()
		m.errorLogLineInput = "1"
		res := m.handleErrorLogOverlayKeyZero()
		assert.Equal(t, "10", res.errorLogLineInput)
		assert.Equal(t, 0, res.errorLogCursorCol)
	})
}

// Word/WORD motions (w/e/b/W/E/B/^) must move the cursor column in normal mode,
// matching the event viewer. Plain text for a zero-time ERR entry is
// "00:00:00 ERR the quick brown".
func TestErrorLogWordMotionInNormalMode(t *testing.T) {
	entries := []ui.ErrorLogEntry{{Level: "ERR", Message: "the quick brown"}}
	at := func(col int) Model {
		return Model{overlayErrorLog: true, errorLog: entries, errorLogCursorCol: col}
	}
	col := func(m tea.Model) int { return m.(Model).errorLogCursorCol }

	tests := []struct {
		name    string
		key     string
		startAt int
		want    int
	}{
		{"w advances to next word start", "w", 0, 3},
		{"W advances to next WORD start (whitespace-delimited)", "W", 0, 9},
		{"e moves to word end", "e", 0, 1},
		{"E moves to WORD end", "E", 0, 7},
		{"b moves to previous word start", "b", 13, 9},
		{"B moves to previous WORD start", "B", 13, 9},
		{"^ moves to first non-whitespace", "^", 13, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := at(tc.startAt)
			assert.Equal(t, tc.want, col(m.handleErrorLogOverlayWordMotion(tc.key)))
		})
	}

	t.Run("motion clamps within the line", func(t *testing.T) {
		m := at(25) // near "brown"
		assert.LessOrEqual(t, col(m.handleErrorLogOverlayWordMotion("w")), len([]rune(ui.ErrorLogEntryPlainText(entries[0])))-1)
	})

	t.Run("word motion routes through the overlay key dispatch", func(t *testing.T) {
		m := at(0)
		res, _ := m.handleErrorLogOverlayKey(tea.KeyPressMsg{Code: 'w', Text: "w"})
		assert.Equal(t, 3, col(res))
	})
}

// A vertical move (j/k) onto a shorter line leaves errorLogCursorCol past that
// line's end; column motions and visual-entry must clamp it on read so the
// cursor stays on a real character (CodeRabbit #325 follow-up).
func TestErrorLogCursorColClampedOnShorterLine(t *testing.T) {
	entries := []ui.ErrorLogEntry{{Level: "ERR", Message: "hi"}}  // short line
	lineLen := len([]rune(ui.ErrorLogEntryPlainText(entries[0]))) // "00:00:00 ERR hi"
	overflow := lineLen + 50

	t.Run("h clamps before moving", func(t *testing.T) {
		m := Model{overlayErrorLog: true, errorLog: entries, errorLogCursorCol: overflow}
		mdl, handled := m.errorLogColumnMotion("h")
		assert.True(t, handled)
		assert.Equal(t, lineLen-2, mdl.errorLogCursorCol) // clamped to lineLen-1, then h
	})

	t.Run("l clamps and stays at end", func(t *testing.T) {
		m := Model{overlayErrorLog: true, errorLog: entries, errorLogCursorCol: overflow}
		mdl, _ := m.errorLogColumnMotion("l")
		assert.Equal(t, lineLen-1, mdl.errorLogCursorCol)
	})

	t.Run("w clamps before motion", func(t *testing.T) {
		m := Model{overlayErrorLog: true, errorLog: entries, errorLogCursorCol: overflow}
		mdl, _ := m.errorLogColumnMotion("w")
		assert.LessOrEqual(t, mdl.errorLogCursorCol, lineLen-1)
	})

	t.Run("entering char-visual clamps the anchor", func(t *testing.T) {
		m := Model{overlayErrorLog: true, errorLog: entries, errorLogCursorCol: overflow}
		res, _ := m.handleErrorLogOverlayKeyV2()
		rm := res.(Model)
		assert.Equal(t, lineLen-1, rm.errorLogVisualStartCol)
		assert.Equal(t, lineLen-1, rm.errorLogCursorCol)
	})
}
