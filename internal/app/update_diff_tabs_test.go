package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCopyCmd runs the clipboard-write half of a copy handler's
// tea.Batch(copyToSystemClipboard(...), scheduleStatusClear()) result,
// skipping the 5-second status-clear timer the second half would block on.
func runCopyCmd(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	batch, ok := cmd().(tea.BatchMsg)
	require.True(t, ok, "expected copy handler to return a tea.Batch")
	batch[0]()
}

// Motions used to walk the raw text while rendering flat-expanded tabs to
// 4 spaces, so a cursor step landed on a column never drawn. These pin the
// fix: motions land on real tab stops, the clipboard keeps the raw tab.

func diffTabsModel(line string) Model {
	m := baseModelNav()
	m.mode = modeDiff
	m.diffView.left = line
	m.diffView.right = line
	m.diffView.cursor = 0
	m.diffView.cursorSide = 0
	m.diffView.unified = true
	return m
}

// A single 'l' press must skip clean over a tab to the next tab stop, the
// same way it skips a wide glyph in one step, instead of walking through
// columns the tab does not occupy.
func TestDiffMotion_LStepsOverTabAsOneUnit(t *testing.T) {
	m := diffTabsModel("a\tb")

	r1, _ := m.handleDiffNormalKey(keyMsg("l"), nil, 1, 5, 0)
	rm1 := r1.(Model)
	assert.Equal(t, 1, rm1.diffView.visualCurCol, "first step lands on the tab's start")

	r2, _ := rm1.handleDiffNormalKey(keyMsg("l"), nil, 1, 5, 0)
	rm2 := r2.(Model)
	assert.Equal(t, 8, rm2.diffView.visualCurCol, "second step jumps past the whole tab to col 8")

	r3, _ := rm2.handleDiffNormalKey(keyMsg("l"), nil, 1, 5, 0)
	rm3 := r3.(Model)
	assert.Equal(t, 9, rm3.diffView.visualCurCol, "third step lands on 'b'")
}

// vim's 'w' treats a tab as whitespace: it must skip past it to the next
// word, landing on the column the renderer draws for that word.
func TestDiffMotion_WordMotionSkipsTabToNextWord(t *testing.T) {
	m := diffTabsModel("a\tb")
	r, _ := m.handleDiffNormalKey(keyMsg("w"), nil, 1, 5, 0)
	assert.Equal(t, 8, r.(Model).diffView.visualCurCol)
}

// '$' must land on the last character's column, which for a line ending
// right after a tab is past the tab's full expansion, not one past the
// unexpanded rune count.
func TestDiffMotion_DollarLandsPastTheTab(t *testing.T) {
	m := diffTabsModel("ab\tc")
	r, _ := m.handleDiffNormalKey(keyMsg("$"), nil, 1, 5, 0)
	assert.Equal(t, 8, r.(Model).diffView.visualCurCol)
}

// Normal-mode 'y' yanks the raw line text verbatim, so a tab survives
// untouched - the deliberate "no back-mapping" clipboard behavior.
func TestDiffCopy_NormalYankKeepsRawTab(t *testing.T) {
	got := stubClipboardWriteAll(t, func(string) error { return nil })
	m := diffTabsModel("a\tb")

	r, cmd := m.handleDiffNormalCopy(nil, 1)
	require.NotNil(t, cmd)
	runCopyCmd(t, cmd)

	assert.Equal(t, "a\tb", *got)
	assert.Equal(t, "Copied 1 line", r.(Model).statusMessage)
}

// Visual char-mode yank across a tab must extract the raw tab byte, not
// the spaces the renderer draws for it - CutCols is the shared seam that
// makes this true for every visual-copy caller, not just the diff viewer.
func TestDiffCopy_VisualCharYankKeepsRawTab(t *testing.T) {
	got := stubClipboardWriteAll(t, func(string) error { return nil })
	m := diffTabsModel("a\tbc")
	m.diffView.visualMode = true
	m.diffView.visualType = 'v'
	m.diffView.visualStart = 0
	m.diffView.visualCol = 0
	m.diffView.visualCurCol = 8 // covers 'a', the tab, and 'b'

	r, cmd := m.diffVisualCopy(nil)
	require.NotNil(t, cmd)
	runCopyCmd(t, cmd)

	assert.Equal(t, "a\tb", *got)
	assert.False(t, r.(Model).diffView.visualMode)
}
