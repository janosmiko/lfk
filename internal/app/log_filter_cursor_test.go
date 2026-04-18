package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWordMovementRespectsVisibleIndices proves that when the filter is
// active, pressing `e` or `w` on the last visible line does not try to
// read a source line the cursor no longer points to.
//
// m.logCursor is a *visible position* when the filter is active — not an
// index into m.logLines. Word-motion handlers like handleLogKeyE read
// m.logLines[m.logCursor] directly, which is the wrong line. This test
// captures one concrete incorrectness: the end-of-line handler `$` should
// stop at the end of the *visible* line, and word advance should not jump
// to a filtered-out line.
func TestWordMovementRespectsVisibleIndices(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	lines := []string{
		"[INFO] first short", // 0
		"[ERROR] second line is much much longer than the first", // 1
		"[INFO] third short", // 2
	}
	m := Model{
		logLines:            lines,
		logSeverityDetector: d,
	}
	m.logRules = []Rule{SeverityRule{SeverityError}}
	m.logFilterChain = NewFilterChain(m.logRules, IncludeAny, d)
	m.rebuildLogVisibleIndices()
	// Only the ERROR line at source index 1 survives.
	assert.Equal(t, []int{1}, m.logVisibleIndices)

	// Visible cursor 0 now points to source line 1 (the ERROR).
	m.logCursor = 0
	m.logVisualCurCol = 0

	// `$` should jump to the end-of-line of the visible ERROR line (the
	// long one) — not of source line 0 (the short INFO). Before the fix,
	// handleLogKeyDollar reads m.logLines[m.logCursor] directly, so with
	// m.logCursor = 0 it would grab the first INFO line and stop
	// prematurely.
	rm := m.handleLogKeyDollar()
	expectedLen := len([]rune(lines[1]))
	assert.Equal(t, expectedLen-1, rm.logVisualCurCol,
		"$ should land on the last rune of the visible line, not the filtered-out source line at cursor index")
}
