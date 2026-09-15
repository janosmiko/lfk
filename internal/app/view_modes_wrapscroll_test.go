package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

// Scroll arithmetic has to count the rows the renderer actually draws. Rune
// division got a CJK line half its height, so follow mode dropped the newest
// sub-lines off the bottom, and overcounted a combining mark the other way.
func TestWrappedLineCount_MatchesTheRenderer(t *testing.T) {
	lines := []string{
		strings.Repeat("日", 20),
		strings.Repeat("a", 20),
		"á" + strings.Repeat("b", 19),
		"a\U0001F600b",
		"",
		"short",
	}
	for _, line := range lines {
		for _, width := range []int{1, 7, 10, 40} {
			assert.Equal(t, len(ui.WrapLine(line, width)), wrappedLineCount(line, width),
				"line %q at width %d", line, width)
		}
	}
}

// A source line taller than the viewport used to pin its first sub-lines,
// on the assumption that the cursor always sat on sub-line 0. The cursor now
// follows its own column, so the window has to follow the cursor (#794).
func TestLogCursorSubLineSkip(t *testing.T) {
	line := strings.Repeat("a", 100) // 10 sub-lines at width 10

	tests := []struct {
		name  string
		col   int
		viewH int
		want  int
	}{
		{"cursor on the first sub-line", 5, 3, 0},
		{"cursor inside the first window", 25, 3, 0},
		{"cursor below the first window", 45, 3, 2},
		{"cursor on the last sub-line", 95, 3, 7},
		{"cursor past the end of the line", 500, 3, 7},
		{"viewport holds every sub-line", 95, 20, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, logCursorSubLineSkip(line, 10, tt.col, tt.viewH))
		})
	}
}

// Horizontal motions change only the column, so they never reached the
// vertical scroll path. Walking right onto a sub-line below the viewport left
// the cursor invisible even after the renderer learned to draw it there.
func TestLogKey_HorizontalMotionScrollsToCursorSubLine(t *testing.T) {
	newModel := func() Model {
		m := baseModelNav()
		m.mode = modeLogs
		m.width, m.height = 40, 12
		m.logView.wrap = true
		m.logView.follow = false
		m.logView.lines = []string{strings.Repeat("a", 2000)}
		m.logView.cursor = 0
		return m
	}

	t.Run("l walks the window down", func(t *testing.T) {
		m := newModel()
		for range 400 {
			ret, _ := m.handleLogKey(keyMsg("l"))
			m = ret.(Model)
		}
		assert.Positive(t, m.logView.wrapTopSkip, "the window follows the cursor's sub-line")
		assertLogCursorRendered(t, m)
	})

	// Walking out to the tail and back is the sequence that matters: the
	// window has to come back up, not just go down.
	t.Run("0 brings the window back to the top", func(t *testing.T) {
		m := newModel()
		for range 400 {
			ret, _ := m.handleLogKey(keyMsg("l"))
			m = ret.(Model)
		}
		require.Positive(t, m.logView.wrapTopSkip, "the window moved down first")

		ret, _ := m.handleLogKey(keyMsg("0"))
		m = ret.(Model)
		assert.Equal(t, 0, m.logView.wrapTopSkip)
		assertLogCursorRendered(t, m)
	})

	t.Run("h walks the window back up", func(t *testing.T) {
		m := newModel()
		for range 400 {
			ret, _ := m.handleLogKey(keyMsg("l"))
			m = ret.(Model)
		}
		for range 400 {
			ret, _ := m.handleLogKey(keyMsg("h"))
			m = ret.(Model)
		}
		assert.Equal(t, 0, m.logView.wrapTopSkip)
		assertLogCursorRendered(t, m)
	})
}

// Movement keys must survive an empty viewport: a filter can leave no lines at
// all, and Home sets the cursor to 0 without checking.
func TestLogKey_MovementOnEmptyWrappedView(t *testing.T) {
	for _, key := range []string{"home", "l", "h", "0", "$", "G", "j", "k"} {
		t.Run(key, func(t *testing.T) {
			m := baseModelNav()
			m.mode = modeLogs
			m.width, m.height = 40, 12
			m.logView.wrap = true
			m.logView.follow = false
			m.logView.lines = nil

			assert.NotPanics(t, func() {
				m.handleLogKey(keyMsg(key))
			})
		})
	}
}

// assertLogCursorRendered fails unless the cursor's sub-line falls inside the
// window the viewport will render.
func assertLogCursorRendered(t *testing.T, m Model) {
	t.Helper()
	availWidth := m.logWrapAvailWidth()
	viewH := max(m.logContentHeight(), 1)
	idx := logCursorSubLineSkip(m.logDisplayLine(0), availWidth, m.logView.visualCurCol, 1)
	assert.GreaterOrEqual(t, idx, m.logView.wrapTopSkip, "cursor sub-line is at or below the window top")
	assert.Less(t, idx, m.logView.wrapTopSkip+viewH, "cursor sub-line is above the window bottom")
}

func TestAdjustLogScrollForCursorWrap_KeepsCursorSubLineVisible(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.wrap = true
	m.logView.lines = []string{strings.Repeat("a", 400)}
	m.logView.cursor = 0
	m.logView.visualCurCol = 350

	viewH := 4
	m.adjustLogScrollForCursorWrap(viewH)

	availWidth := m.logWrapAvailWidth()
	want := logCursorSubLineSkip(m.logDisplayLine(0), availWidth, 350, viewH)
	assert.Equal(t, 0, m.logView.scroll)
	assert.Equal(t, want, m.logView.wrapTopSkip)
	assert.Positive(t, m.logView.wrapTopSkip, "the cursor sits below the first window")
}
