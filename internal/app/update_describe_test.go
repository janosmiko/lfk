package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
)

// --- handleDescribeKey ---

func TestDescribeKeyEscReturnsToExplorer(t *testing.T) {
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: "line1\nline2\nline3",
			cursor:  2,
			scroll:  1,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
	assert.Equal(t, 0, result.describeView.scroll)
	assert.Equal(t, 0, result.describeView.cursor)
	assert.Equal(t, 0, result.describeView.cursorCol)
}

func TestDescribeKeyQReturnsToExplorer(t *testing.T) {
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: "line1",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(runeKey('q'))
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
}

func TestDescribeKeyQuestionMarkOpensHelp(t *testing.T) {
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: "line1",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(runeKey('?'))
	result := ret.(Model)
	assert.Equal(t, modeHelp, result.mode)
	assert.Equal(t, modeDescribe, result.helpPreviousMode)
	assert.Equal(t, "Describe View", result.helpContextMode)
}

func TestDescribeKeyJMovesCursorDown(t *testing.T) {
	content := strings.Repeat("line\n", 100)
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: content,
			cursor:  0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.describeView.cursor)
}

func TestDescribeKeyKMovesCursorUp(t *testing.T) {
	content := strings.Repeat("line\n", 100)
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: content,
			cursor:  10,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(runeKey('k'))
	result := ret.(Model)
	assert.Equal(t, 9, result.describeView.cursor)
}

func TestDescribeKeyKAtZeroStays(t *testing.T) {
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: "line1\nline2",
			cursor:  0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(runeKey('k'))
	result := ret.(Model)
	assert.Equal(t, 0, result.describeView.cursor)
}

func TestDescribeKeyGGMovesToTop(t *testing.T) {
	content := strings.Repeat("line\n", 100)
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: content,
			cursor:  50,
			scroll:  45,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	// First g sets pendingG
	ret, _ := m.handleDescribeKey(runeKey('g'))
	result := ret.(Model)
	assert.True(t, result.pendingG)

	// Second g moves cursor to top
	ret2, _ := result.handleDescribeKey(runeKey('g'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.describeView.cursor)
	assert.False(t, result2.pendingG)
}

func TestDescribeKeyGMovesToBottom(t *testing.T) {
	content := strings.Repeat("line\n", 100)
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: content,
			cursor:  0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(runeKey('G'))
	result := ret.(Model)
	assert.Greater(t, result.describeView.cursor, 0)
}

func TestDescribeKeyCtrlDHalfPageDown(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: content,
			cursor:  0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	result := ret.(Model)
	// describeContentHeight() = (40 - 4) = 36, half = 18
	assert.Equal(t, 18, result.describeView.cursor)
}

func TestDescribeKeyCtrlUHalfPageUp(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: content,
			cursor:  30,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Equal(t, 12, result.describeView.cursor) // 30 - 18 = 12
}

func TestDescribeKeyCtrlUClampsToZero(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: content,
			cursor:  5,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Equal(t, 0, result.describeView.cursor)
}

func TestDescribeKeyCtrlFFullPageDown(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: content,
			cursor:  0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	result := ret.(Model)
	assert.Equal(t, 36, result.describeView.cursor) // describeContentHeight() = 36
}

func TestDescribeKeyCtrlBFullPageUp(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: content,
			cursor:  60,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	result := ret.(Model)
	assert.Equal(t, 24, result.describeView.cursor) // 60 - 36 = 24
}

// --- New describe cursor/visual/search tests ---

func TestDescribeKeyHLColumnMovement(t *testing.T) {
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content:   "hello world",
			cursorCol: 5,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	// h moves left
	ret, _ := m.handleDescribeKey(runeKey('h'))
	result := ret.(Model)
	assert.Equal(t, 4, result.describeView.cursorCol)

	// l moves right
	ret2, _ := result.handleDescribeKey(runeKey('l'))
	result2 := ret2.(Model)
	assert.Equal(t, 5, result2.describeView.cursorCol)
}

func TestDescribeKeyVisualMode(t *testing.T) {
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: "line1\nline2\nline3",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	// v enters char visual mode
	ret, _ := m.handleDescribeKey(runeKey('v'))
	result := ret.(Model)
	assert.Equal(t, byte('v'), result.describeView.visualMode)

	// esc exits visual mode
	ret2, _ := result.handleDescribeKey(specialKey(tea.KeyEsc))
	result2 := ret2.(Model)
	assert.Equal(t, byte(0), result2.describeView.visualMode)
}

func TestDescribeKeyVisualLineMode(t *testing.T) {
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: "line1\nline2\nline3",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(runeKey('V'))
	result := ret.(Model)
	assert.Equal(t, byte('V'), result.describeView.visualMode)
}

func TestDescribeKeySearch(t *testing.T) {
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: "line1\nline2\nline3",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	// / activates search
	ret, _ := m.handleDescribeKey(runeKey('/'))
	result := ret.(Model)
	assert.True(t, result.describeView.searchActive)
}

func TestDescribeKeyCopyCurrentLine(t *testing.T) {
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content: "line1\nline2\nline3",
			cursor:  1,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, cmd := m.handleDescribeKey(runeKey('y'))
	result := ret.(Model)
	assert.Equal(t, "Copied 1 line", result.statusMessage)
	assert.NotNil(t, cmd)
}

func TestDescribeKeyEscClearsSearchFirst(t *testing.T) {
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content:     "line1\nline2",
			searchQuery: "line",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	// First esc clears search, stays in describe mode
	assert.Equal(t, modeDescribe, result.mode)
	assert.Empty(t, result.describeView.searchQuery)
}

func TestDescribeKeyWordMotion(t *testing.T) {
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content:   "hello world test",
			cursorCol: 0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDescribeKey(runeKey('w'))
	result := ret.(Model)
	assert.Equal(t, 6, result.describeView.cursorCol) // "world" starts at 6
}

// --- handleDiffKey ---

func TestDiffKeyEscReturnsToExplorer(t *testing.T) {
	m := Model{
		mode: modeDiff,
		diffView: diffViewState{
			left: "line1\nline2",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDiffKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
	assert.Equal(t, 0, result.diffView.scroll)
}

func TestDiffKeyQuestionMarkOpensHelp(t *testing.T) {
	m := Model{
		mode: modeDiff,
		diffView: diffViewState{
			left: "line1",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDiffKey(runeKey('?'))
	result := ret.(Model)
	assert.Equal(t, modeHelp, result.mode)
	assert.Equal(t, modeDiff, result.helpPreviousMode)
}

func TestDiffKeyJMovesCursorDown(t *testing.T) {
	m := Model{
		mode: modeDiff,
		diffView: diffViewState{
			left:   strings.Repeat("line\n", 100),
			cursor: 0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDiffKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.diffView.cursor)
}

func TestDiffKeyUTogglesUnified(t *testing.T) {
	m := Model{
		mode: modeDiff,
		diffView: diffViewState{
			left:    "line1",
			unified: false,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDiffKey(runeKey('u'))
	result := ret.(Model)
	assert.True(t, result.diffView.unified)
}

func TestDiffKeyHashTogglesLineNumbers(t *testing.T) {
	m := Model{
		mode: modeDiff,
		diffView: diffViewState{
			left:        "line1",
			lineNumbers: false,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDiffKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'#'}})
	result := ret.(Model)
	assert.True(t, result.diffView.lineNumbers)
}

func TestDiffKeyDigitBuffering(t *testing.T) {
	m := Model{
		mode: modeDiff,
		diffView: diffViewState{
			left:      strings.Repeat("line\n", 100),
			lineInput: "",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDiffKey(runeKey('1'))
	result := ret.(Model)
	assert.Equal(t, "1", result.diffView.lineInput)

	ret2, _ := result.handleDiffKey(runeKey('5'))
	result2 := ret2.(Model)
	assert.Equal(t, "15", result2.diffView.lineInput)
}

func TestDiffKeyGWithDigitJumpsToLine(t *testing.T) {
	m := Model{
		mode: modeDiff,
		diffView: diffViewState{
			left:      strings.Repeat("line\n", 100),
			lineInput: "10",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleDiffKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 9, result.diffView.cursor) // 10 - 1 = 9 (0-indexed)
	assert.Empty(t, result.diffView.lineInput)
}

func TestCovToggleDiffFoldAtCursor(t *testing.T) {
	m := baseModelCov()
	m.diffView.left = "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"
	m.diffView.right = "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"

	foldRegions := ui.ComputeDiffFoldRegions(m.diffView.left, m.diffView.right)
	m.diffView.foldState = make([]bool, len(foldRegions))
	m.diffView.cursor = 0

	// Should not panic even with no foldable regions (all lines equal = one big fold).
	m.toggleDiffFoldAtCursor(foldRegions)
}

func TestCovToggleDiffFoldAtCursorOutOfBounds(t *testing.T) {
	m := baseModelCov()
	m.diffView.left = ""
	m.diffView.right = ""
	m.diffView.cursor = 100
	m.diffView.foldState = nil
	m.toggleDiffFoldAtCursor(nil)
}

func TestCovToggleAllDiffFolds(t *testing.T) {
	m := baseModelCov()
	m.diffView.foldState = []bool{false, false, true}
	regions := []ui.DiffFoldRegion{{Start: 0, End: 2}, {Start: 3, End: 5}, {Start: 6, End: 8}}

	// Some collapsed -> expand all.
	m.toggleAllDiffFolds(regions)
	for _, v := range m.diffView.foldState {
		assert.False(t, v)
	}

	// None collapsed -> collapse all.
	m.toggleAllDiffFolds(regions)
	for _, v := range m.diffView.foldState {
		assert.True(t, v)
	}
}

func TestCovDescribeKeyHelp(t *testing.T) {
	m := baseModelDescribe()
	result, _ := m.handleDescribeKey(keyMsg("?"))
	rm := result.(Model)
	assert.Equal(t, modeHelp, rm.mode)
	assert.Equal(t, "Describe View", rm.helpContextMode)
}

func TestCovDescribeKeyToggleWrap(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.wrap = false
	result, _ := m.handleDescribeKey(keyMsg(">"))
	rm := result.(Model)
	assert.True(t, rm.describeView.wrap)
}

func TestCovDescribeKeyEscClearsSearch(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchQuery = "hello"
	result, _ := m.handleDescribeKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.describeView.searchQuery)
	assert.Equal(t, modeDescribe, rm.mode)
}

func TestCovDescribeKeyEscExitsView(t *testing.T) {
	m := baseModelDescribe()
	result, _ := m.handleDescribeKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
	assert.Equal(t, 0, rm.describeView.scroll)
}

func TestCovDescribeKeyMoveDown(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursor = 0
	result, _ := m.handleDescribeKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.describeView.cursor)
}

func TestCovDescribeKeyMoveUp(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursor = 5
	result, _ := m.handleDescribeKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 4, rm.describeView.cursor)
}

func TestCovDescribeKeyMoveLeft(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursorCol = 5
	result, _ := m.handleDescribeKey(keyMsg("h"))
	rm := result.(Model)
	assert.Equal(t, 4, rm.describeView.cursorCol)
}

func TestCovDescribeKeyMoveRight(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursorCol = 0
	result, _ := m.handleDescribeKey(keyMsg("l"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.describeView.cursorCol)
}

func TestCovDescribeKeyZero(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursorCol = 5
	result, _ := m.handleDescribeKey(keyMsg("0"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.describeView.cursorCol)
}

func TestCovDescribeKeyZeroInLineInput(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.lineInput = "12"
	result, _ := m.handleDescribeKey(keyMsg("0"))
	rm := result.(Model)
	assert.Equal(t, "120", rm.describeView.lineInput)
}

func TestCovDescribeKeyDollar(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursor = 0
	result, _ := m.handleDescribeKey(keyMsg("$"))
	rm := result.(Model)
	// "line0" has 5 chars, so cursor col should be at 4
	assert.Equal(t, 4, rm.describeView.cursorCol)
}

func TestCovDescribeKeyCaret(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.content = "   indented"
	m.describeView.cursor = 0
	result, _ := m.handleDescribeKey(keyMsg("^"))
	rm := result.(Model)
	assert.Equal(t, 3, rm.describeView.cursorCol)
}

func TestCovDescribeKeyWordMotions(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.content = "hello world foo"
	m.describeView.cursor = 0
	m.describeView.cursorCol = 0

	result, _ := m.handleDescribeKey(keyMsg("w"))
	rm := result.(Model)
	assert.Greater(t, rm.describeView.cursorCol, 0)

	result, _ = rm.handleDescribeKey(keyMsg("b"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.describeView.cursorCol)

	result, _ = rm.handleDescribeKey(keyMsg("e"))
	rm = result.(Model)
	assert.Greater(t, rm.describeView.cursorCol, 0)

	result, _ = rm.handleDescribeKey(keyMsg("W"))
	rm = result.(Model)
	assert.Greater(t, rm.describeView.cursorCol, 0)

	result, _ = rm.handleDescribeKey(keyMsg("B"))
	rm = result.(Model)

	result, _ = rm.handleDescribeKey(keyMsg("E"))
	rm = result.(Model)
	assert.Greater(t, rm.describeView.cursorCol, 0)
}

func TestCovDescribeKeyCtrlD(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursor = 0
	result, _ := m.handleDescribeKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.Greater(t, rm.describeView.cursor, 0)
}

func TestCovDescribeKeyCtrlU(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursor = 5
	result, _ := m.handleDescribeKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.Less(t, rm.describeView.cursor, 5)
}

func TestCovDescribeKeyCtrlF(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursor = 0
	result, _ := m.handleDescribeKey(keyMsg("ctrl+f"))
	rm := result.(Model)
	assert.Greater(t, rm.describeView.cursor, 0)
}

func TestCovDescribeKeyCtrlB(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursor = 9
	result, _ := m.handleDescribeKey(keyMsg("ctrl+b"))
	rm := result.(Model)
	assert.Less(t, rm.describeView.cursor, 9)
}

func TestCovDescribeKeyG(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursor = 5
	// First 'g' sets pendingG
	result, _ := m.handleDescribeKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)

	// Second 'g' jumps to top
	result, _ = rm.handleDescribeKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.describeView.cursor)
	assert.False(t, rm.pendingG)
}

func TestCovDescribeKeyGBig(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursor = 0
	result, _ := m.handleDescribeKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 9, rm.describeView.cursor)
}

func TestCovDescribeKeyGBigWithLineInput(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.lineInput = "3"
	result, _ := m.handleDescribeKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.describeView.cursor) // 3-1=2 (0-indexed)
}

func TestCovDescribeKeyDigit(t *testing.T) {
	m := baseModelDescribe()
	result, _ := m.handleDescribeKey(keyMsg("5"))
	rm := result.(Model)
	assert.Equal(t, "5", rm.describeView.lineInput)
}

func TestCovDescribeKeyVisualV(t *testing.T) {
	m := baseModelDescribe()
	result, _ := m.handleDescribeKey(keyMsg("v"))
	rm := result.(Model)
	assert.Equal(t, byte('v'), rm.describeView.visualMode)
}

func TestCovDescribeKeyVisualShiftV(t *testing.T) {
	m := baseModelDescribe()
	result, _ := m.handleDescribeKey(keyMsg("V"))
	rm := result.(Model)
	assert.Equal(t, byte('V'), rm.describeView.visualMode)
}

func TestCovDescribeKeyVisualCtrlV(t *testing.T) {
	m := baseModelDescribe()
	result, _ := m.handleDescribeKey(keyMsg("ctrl+v"))
	rm := result.(Model)
	assert.Equal(t, byte('B'), rm.describeView.visualMode)
}

func TestCovDescribeKeyYank(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursor = 0
	_, cmd := m.handleDescribeKey(keyMsg("y"))
	assert.NotNil(t, cmd)
}

func TestCovDescribeKeySlash(t *testing.T) {
	m := baseModelDescribe()
	result, _ := m.handleDescribeKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.describeView.searchActive)
}

func TestCovDescribeKeySearchNav(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchQuery = "line"
	// n searches forward
	result, _ := m.handleDescribeKey(keyMsg("n"))
	rm := result.(Model)
	assert.NotEqual(t, -1, rm.describeView.cursor)

	// N searches backward
	rm.describeView.cursor = 5
	result, _ = rm.handleDescribeKey(keyMsg("N"))
	rm = result.(Model)
	assert.NotEqual(t, -1, rm.describeView.cursor)
}

func TestCovDescribeKeyDefault(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.lineInput = "123"
	result, _ := m.handleDescribeKey(keyMsg("x"))
	rm := result.(Model)
	assert.Empty(t, rm.describeView.lineInput)
}

func TestCovDescribeVisualKeyEsc(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'V'
	result, _ := m.handleDescribeVisualKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Zero(t, rm.describeView.visualMode)
}

func TestCovDescribeVisualKeyToggleV(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'V'
	result, _ := m.handleDescribeVisualKey(keyMsg("V"))
	rm := result.(Model)
	assert.Zero(t, rm.describeView.visualMode)
}

func TestCovDescribeVisualKeyToggleSwitchV(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'v'
	result, _ := m.handleDescribeVisualKey(keyMsg("V"))
	rm := result.(Model)
	assert.Equal(t, byte('V'), rm.describeView.visualMode)
}

func TestCovDescribeVisualKeyToggleLowerV(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'v'
	result, _ := m.handleDescribeVisualKey(keyMsg("v"))
	rm := result.(Model)
	assert.Zero(t, rm.describeView.visualMode)
}

func TestCovDescribeVisualKeyCtrlV(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'B'
	result, _ := m.handleDescribeVisualKey(keyMsg("ctrl+v"))
	rm := result.(Model)
	assert.Zero(t, rm.describeView.visualMode)
}

func TestCovDescribeVisualKeyCtrlVOn(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'v'
	result, _ := m.handleDescribeVisualKey(keyMsg("ctrl+v"))
	rm := result.(Model)
	assert.Equal(t, byte('B'), rm.describeView.visualMode)
}

func TestCovDescribeVisualKeyMovement(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'V'
	m.describeView.cursor = 2
	m.describeView.cursorCol = 2

	result, _ := m.handleDescribeVisualKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 3, rm.describeView.cursor)

	result, _ = rm.handleDescribeVisualKey(keyMsg("k"))
	rm = result.(Model)
	assert.Equal(t, 2, rm.describeView.cursor)

	result, _ = rm.handleDescribeVisualKey(keyMsg("l"))
	rm = result.(Model)
	assert.Equal(t, 3, rm.describeView.cursorCol)

	rm.describeView.cursorCol = 2
	result, _ = rm.handleDescribeVisualKey(keyMsg("h"))
	rm = result.(Model)
	assert.Equal(t, 1, rm.describeView.cursorCol)

	result, _ = rm.handleDescribeVisualKey(keyMsg("0"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.describeView.cursorCol)

	result, _ = rm.handleDescribeVisualKey(keyMsg("$"))
	rm = result.(Model)
	assert.Greater(t, rm.describeView.cursorCol, 0)

	result, _ = rm.handleDescribeVisualKey(keyMsg("^"))
	rm = result.(Model)

	result, _ = rm.handleDescribeVisualKey(keyMsg("w"))
	rm = result.(Model)

	result, _ = rm.handleDescribeVisualKey(keyMsg("b"))
	rm = result.(Model)

	result, _ = rm.handleDescribeVisualKey(keyMsg("e"))
	rm = result.(Model)

	result, _ = rm.handleDescribeVisualKey(keyMsg("W"))
	rm = result.(Model)

	result, _ = rm.handleDescribeVisualKey(keyMsg("B"))
	rm = result.(Model)

	result, _ = rm.handleDescribeVisualKey(keyMsg("E"))
	rm = result.(Model)
}

func TestCovDescribeVisualKeyG(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'V'
	m.describeView.cursor = 5

	result, _ := m.handleDescribeVisualKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 9, rm.describeView.cursor)

	rm.pendingG = false
	result, _ = rm.handleDescribeVisualKey(keyMsg("g"))
	rm = result.(Model)
	assert.True(t, rm.pendingG)

	result, _ = rm.handleDescribeVisualKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.describeView.cursor)
}

func TestCovDescribeVisualKeyPageMovement(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'V'
	m.describeView.cursor = 0

	result, _ := m.handleDescribeVisualKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.Greater(t, rm.describeView.cursor, 0)

	rm.describeView.cursor = 9
	result, _ = rm.handleDescribeVisualKey(keyMsg("ctrl+u"))
	rm = result.(Model)
	assert.Less(t, rm.describeView.cursor, 9)
}

func TestCovDescribeVisualKeyCopyLineMode(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'V'
	m.describeView.cursor = 2
	m.describeView.visualStart = 0
	_, cmd := m.handleDescribeVisualKey(keyMsg("y"))
	assert.NotNil(t, cmd)
}

func TestCovDescribeVisualKeyCopyCharMode(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'v'
	m.describeView.cursor = 1
	m.describeView.visualStart = 0
	m.describeView.visualCol = 0
	m.describeView.cursorCol = 3
	_, cmd := m.handleDescribeVisualKey(keyMsg("y"))
	assert.NotNil(t, cmd)
}

func TestCovDescribeVisualKeyCopyBlockMode(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'B'
	m.describeView.cursor = 2
	m.describeView.visualStart = 0
	m.describeView.visualCol = 0
	m.describeView.cursorCol = 3
	_, cmd := m.handleDescribeVisualKey(keyMsg("y"))
	assert.NotNil(t, cmd)
}

func TestCovDescribeVisualKeyCopyCharModeSameLine(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'v'
	m.describeView.cursor = 0
	m.describeView.visualStart = 0
	m.describeView.visualCol = 0
	m.describeView.cursorCol = 3
	_, cmd := m.handleDescribeVisualKey(keyMsg("y"))
	assert.NotNil(t, cmd)
}

func TestCovDescribeSearchKeyEnter(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchActive = true
	m.describeView.searchInput.Insert("line")
	result, _ := m.handleDescribeSearchKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.describeView.searchActive)
	assert.Equal(t, "line", rm.describeView.searchQuery)
}

func TestCovDescribeSearchKeyEsc(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchActive = true
	result, _ := m.handleDescribeSearchKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.describeView.searchActive)
}

func TestCovDescribeSearchKeyBackspace(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchActive = true
	m.describeView.searchInput.Insert("ab")
	result, _ := m.handleDescribeSearchKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "a", rm.describeView.searchInput.Value)
}

// Regression: typing into the describe-view search input now updates
// describeSearchQuery on every keystroke so the highlight overlay paints
// in real time rather than waiting for Enter to commit.
func TestDescribeSearchTypingUpdatesQueryLive(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchActive = true

	result, _ := m.handleDescribeSearchKey(keyMsg("a"))
	rm := result.(Model)
	assert.Equal(t, "a", rm.describeView.searchInput.Value)
	assert.Equal(t, "a", rm.describeView.searchQuery,
		"describeSearchQuery must mirror describeSearchInput while typing so highlights paint live")

	result, _ = rm.handleDescribeSearchKey(keyMsg("b"))
	rm = result.(Model)
	assert.Equal(t, "ab", rm.describeView.searchQuery)

	result, _ = rm.handleDescribeSearchKey(keyMsg("backspace"))
	rm = result.(Model)
	assert.Equal(t, "a", rm.describeView.searchQuery,
		"backspace must keep describeSearchQuery in sync with the input")
}

func TestCovDescribeSearchKeyCtrlW(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchActive = true
	m.describeView.searchInput.Insert("foo bar")
	result, _ := m.handleDescribeSearchKey(keyMsg("ctrl+w"))
	rm := result.(Model)
	assert.NotEqual(t, "foo bar", rm.describeView.searchInput.Value)
}

func TestCovDescribeSearchKeyCtrlA(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchActive = true
	m.describeView.searchInput.Insert("abc")
	result, _ := m.handleDescribeSearchKey(keyMsg("ctrl+a"))
	_ = result.(Model)
}

func TestCovDescribeSearchKeyCtrlE(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchActive = true
	m.describeView.searchInput.Insert("abc")
	result, _ := m.handleDescribeSearchKey(keyMsg("ctrl+e"))
	_ = result.(Model)
}

func TestCovDescribeSearchKeyLeftRight(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchActive = true
	m.describeView.searchInput.Insert("abc")
	result, _ := m.handleDescribeSearchKey(keyMsg("left"))
	rm := result.(Model)
	result, _ = rm.handleDescribeSearchKey(keyMsg("right"))
	_ = result.(Model)
}

func TestCovDescribeSearchKeyInsertChar(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchActive = true
	result, _ := m.handleDescribeSearchKey(keyMsg("x"))
	rm := result.(Model)
	assert.Equal(t, "x", rm.describeView.searchInput.Value)
}

func TestCovFindNextDescribeMatchForward(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchQuery = "line5"
	m.describeView.cursor = 0
	m.findNextDescribeMatch(true)
	assert.Equal(t, 5, m.describeView.cursor)
}

func TestCovFindNextDescribeMatchBackward(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchQuery = "line3"
	m.describeView.cursor = 5
	m.findNextDescribeMatch(false)
	assert.Equal(t, 3, m.describeView.cursor)
}

func TestCovFindNextDescribeMatchNoQuery(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchQuery = ""
	m.describeView.cursor = 5
	m.findNextDescribeMatch(true)
	assert.Equal(t, 5, m.describeView.cursor) // unchanged
}

func TestCovFindNextDescribeMatchNotFound(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchQuery = "nonexistent"
	m.describeView.cursor = 0
	m.findNextDescribeMatch(true)
	assert.Equal(t, 0, m.describeView.cursor) // unchanged
}

func TestCovDescribeContentHeight(t *testing.T) {
	m := baseModelDescribe()
	m.height = 40
	h := m.describeContentHeight()
	assert.Equal(t, 36, h) // 40-4

	m.height = 5
	h = m.describeContentHeight()
	assert.Equal(t, 3, h) // minimum
}

func TestCovEnsureDescribeCursorVisible(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursor = 100 // out of bounds
	m.ensureDescribeCursorVisible()
	assert.LessOrEqual(t, m.describeView.cursor, 9)
}

func TestCovDescribeKeyDispatchToSearch(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.searchActive = true
	m.describeView.searchInput.Insert("test")
	result, _ := m.handleDescribeKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.describeView.searchActive)
}

func TestCovDescribeKeyDispatchToVisual(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.visualMode = 'V'
	m.describeView.cursor = 2
	result, _ := m.handleDescribeKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 3, rm.describeView.cursor)
}
