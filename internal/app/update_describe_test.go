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
		mode:            modeDescribe,
		describeContent: "line1\nline2\nline3",
		describeCursor:  2,
		describeScroll:  1,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
	assert.Equal(t, 0, result.describeScroll)
	assert.Equal(t, 0, result.describeCursor)
	assert.Equal(t, 0, result.describeCursorCol)
}

func TestDescribeKeyQReturnsToExplorer(t *testing.T) {
	m := Model{
		mode:            modeDescribe,
		describeContent: "line1",
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(runeKey('q'))
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
}

func TestDescribeKeyQuestionMarkOpensHelp(t *testing.T) {
	m := Model{
		mode:            modeDescribe,
		describeContent: "line1",
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
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
		mode:            modeDescribe,
		describeContent: content,
		describeCursor:  0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.describeCursor)
}

func TestDescribeKeyKMovesCursorUp(t *testing.T) {
	content := strings.Repeat("line\n", 100)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeCursor:  10,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(runeKey('k'))
	result := ret.(Model)
	assert.Equal(t, 9, result.describeCursor)
}

func TestDescribeKeyKAtZeroStays(t *testing.T) {
	m := Model{
		mode:            modeDescribe,
		describeContent: "line1\nline2",
		describeCursor:  0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(runeKey('k'))
	result := ret.(Model)
	assert.Equal(t, 0, result.describeCursor)
}

func TestDescribeKeyGGMovesToTop(t *testing.T) {
	content := strings.Repeat("line\n", 100)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeCursor:  50,
		describeScroll:  45,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	// First g sets pendingG
	ret, _ := m.handleDescribeKey(runeKey('g'))
	result := ret.(Model)
	assert.True(t, result.pendingG)

	// Second g moves cursor to top
	ret2, _ := result.handleDescribeKey(runeKey('g'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.describeCursor)
	assert.False(t, result2.pendingG)
}

func TestDescribeKeyGMovesToBottom(t *testing.T) {
	content := strings.Repeat("line\n", 100)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeCursor:  0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(runeKey('G'))
	result := ret.(Model)
	assert.Greater(t, result.describeCursor, 0)
}

func TestDescribeKeyCtrlDHalfPageDown(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeCursor:  0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	result := ret.(Model)
	// describeContentHeight() = (40 - 4) = 36, half = 18
	assert.Equal(t, 18, result.describeCursor)
}

func TestDescribeKeyCtrlUHalfPageUp(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeCursor:  30,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Equal(t, 12, result.describeCursor) // 30 - 18 = 12
}

func TestDescribeKeyCtrlUClampsToZero(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeCursor:  5,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Equal(t, 0, result.describeCursor)
}

func TestDescribeKeyCtrlFFullPageDown(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeCursor:  0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	result := ret.(Model)
	assert.Equal(t, 36, result.describeCursor) // describeContentHeight() = 36
}

func TestDescribeKeyCtrlBFullPageUp(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeCursor:  60,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	result := ret.(Model)
	assert.Equal(t, 24, result.describeCursor) // 60 - 36 = 24
}

// --- New describe cursor/visual/search tests ---

func TestDescribeKeyHLColumnMovement(t *testing.T) {
	m := Model{
		mode:              modeDescribe,
		describeContent:   "hello world",
		describeCursorCol: 5,
		tabs:              []TabState{{}},
		width:             80,
		height:            40,
	}
	// h moves left
	ret, _ := m.handleDescribeKey(runeKey('h'))
	result := ret.(Model)
	assert.Equal(t, 4, result.describeCursorCol)

	// l moves right
	ret2, _ := result.handleDescribeKey(runeKey('l'))
	result2 := ret2.(Model)
	assert.Equal(t, 5, result2.describeCursorCol)
}

func TestDescribeKeyVisualMode(t *testing.T) {
	m := Model{
		mode:            modeDescribe,
		describeContent: "line1\nline2\nline3",
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	// v enters char visual mode
	ret, _ := m.handleDescribeKey(runeKey('v'))
	result := ret.(Model)
	assert.Equal(t, byte('v'), result.describeVisualMode)

	// esc exits visual mode
	ret2, _ := result.handleDescribeKey(specialKey(tea.KeyEsc))
	result2 := ret2.(Model)
	assert.Equal(t, byte(0), result2.describeVisualMode)
}

func TestDescribeKeyVisualLineMode(t *testing.T) {
	m := Model{
		mode:            modeDescribe,
		describeContent: "line1\nline2\nline3",
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(runeKey('V'))
	result := ret.(Model)
	assert.Equal(t, byte('V'), result.describeVisualMode)
}

func TestDescribeKeySearch(t *testing.T) {
	m := Model{
		mode:            modeDescribe,
		describeContent: "line1\nline2\nline3",
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	// / activates search
	ret, _ := m.handleDescribeKey(runeKey('/'))
	result := ret.(Model)
	assert.True(t, result.describeSearchActive)
}

func TestDescribeKeyCopyCurrentLine(t *testing.T) {
	m := Model{
		mode:            modeDescribe,
		describeContent: "line1\nline2\nline3",
		describeCursor:  1,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, cmd := m.handleDescribeKey(runeKey('y'))
	result := ret.(Model)
	assert.Equal(t, "Copied 1 line", result.statusMessage)
	assert.NotNil(t, cmd)
}

func TestDescribeKeyEscClearsSearchFirst(t *testing.T) {
	m := Model{
		mode:                modeDescribe,
		describeContent:     "line1\nline2",
		describeSearchQuery: "line",
		tabs:                []TabState{{}},
		width:               80,
		height:              40,
	}
	ret, _ := m.handleDescribeKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	// First esc clears search, stays in describe mode
	assert.Equal(t, modeDescribe, result.mode)
	assert.Empty(t, result.describeSearchQuery)
}

func TestDescribeKeyWordMotion(t *testing.T) {
	m := Model{
		mode:              modeDescribe,
		describeContent:   "hello world test",
		describeCursorCol: 0,
		tabs:              []TabState{{}},
		width:             80,
		height:            40,
	}
	ret, _ := m.handleDescribeKey(runeKey('w'))
	result := ret.(Model)
	assert.Equal(t, 6, result.describeCursorCol) // "world" starts at 6
}

// --- handleDiffKey ---

func TestDiffKeyEscReturnsToExplorer(t *testing.T) {
	m := Model{
		mode:     modeDiff,
		diffLeft: "line1\nline2",
		tabs:     []TabState{{}},
		width:    80,
		height:   40,
	}
	ret, _ := m.handleDiffKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
	assert.Equal(t, 0, result.diffScroll)
}

func TestDiffKeyQuestionMarkOpensHelp(t *testing.T) {
	m := Model{
		mode:     modeDiff,
		diffLeft: "line1",
		tabs:     []TabState{{}},
		width:    80,
		height:   40,
	}
	ret, _ := m.handleDiffKey(runeKey('?'))
	result := ret.(Model)
	assert.Equal(t, modeHelp, result.mode)
	assert.Equal(t, modeDiff, result.helpPreviousMode)
}

func TestDiffKeyJMovesCursorDown(t *testing.T) {
	m := Model{
		mode:       modeDiff,
		diffLeft:   strings.Repeat("line\n", 100),
		diffCursor: 0,
		tabs:       []TabState{{}},
		width:      80,
		height:     40,
	}
	ret, _ := m.handleDiffKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.diffCursor)
}

func TestDiffKeyUTogglesUnified(t *testing.T) {
	m := Model{
		mode:        modeDiff,
		diffLeft:    "line1",
		diffUnified: false,
		tabs:        []TabState{{}},
		width:       80,
		height:      40,
	}
	ret, _ := m.handleDiffKey(runeKey('u'))
	result := ret.(Model)
	assert.True(t, result.diffUnified)
}

func TestDiffKeyHashTogglesLineNumbers(t *testing.T) {
	m := Model{
		mode:            modeDiff,
		diffLeft:        "line1",
		diffLineNumbers: false,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDiffKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'#'}})
	result := ret.(Model)
	assert.True(t, result.diffLineNumbers)
}

func TestDiffKeyDigitBuffering(t *testing.T) {
	m := Model{
		mode:          modeDiff,
		diffLeft:      strings.Repeat("line\n", 100),
		diffLineInput: "",
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleDiffKey(runeKey('1'))
	result := ret.(Model)
	assert.Equal(t, "1", result.diffLineInput)

	ret2, _ := result.handleDiffKey(runeKey('5'))
	result2 := ret2.(Model)
	assert.Equal(t, "15", result2.diffLineInput)
}

func TestDiffKeyGWithDigitJumpsToLine(t *testing.T) {
	m := Model{
		mode:          modeDiff,
		diffLeft:      strings.Repeat("line\n", 100),
		diffLineInput: "10",
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleDiffKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 9, result.diffCursor) // 10 - 1 = 9 (0-indexed)
	assert.Empty(t, result.diffLineInput)
}

func TestCovToggleDiffFoldAtCursor(t *testing.T) {
	m := baseModelCov()
	m.diffLeft = "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"
	m.diffRight = "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"

	foldRegions := ui.ComputeDiffFoldRegions(m.diffLeft, m.diffRight)
	m.diffFoldState = make([]bool, len(foldRegions))
	m.diffCursor = 0

	// Should not panic even with no foldable regions (all lines equal = one big fold).
	m.toggleDiffFoldAtCursor(foldRegions)
}

func TestCovToggleDiffFoldAtCursorOutOfBounds(t *testing.T) {
	m := baseModelCov()
	m.diffLeft = ""
	m.diffRight = ""
	m.diffCursor = 100
	m.diffFoldState = nil
	m.toggleDiffFoldAtCursor(nil)
}

func TestCovToggleAllDiffFolds(t *testing.T) {
	m := baseModelCov()
	m.diffFoldState = []bool{false, false, true}
	regions := []ui.DiffFoldRegion{{Start: 0, End: 2}, {Start: 3, End: 5}, {Start: 6, End: 8}}

	// Some collapsed -> expand all.
	m.toggleAllDiffFolds(regions)
	for _, v := range m.diffFoldState {
		assert.False(t, v)
	}

	// None collapsed -> collapse all.
	m.toggleAllDiffFolds(regions)
	for _, v := range m.diffFoldState {
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
	m.describeWrap = false
	result, _ := m.handleDescribeKey(keyMsg(">"))
	rm := result.(Model)
	assert.True(t, rm.describeWrap)
}

func TestCovDescribeKeyEscClearsSearch(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchQuery = "hello"
	result, _ := m.handleDescribeKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.describeSearchQuery)
	assert.Equal(t, modeDescribe, rm.mode)
}

func TestCovDescribeKeyEscExitsView(t *testing.T) {
	m := baseModelDescribe()
	result, _ := m.handleDescribeKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
	assert.Equal(t, 0, rm.describeScroll)
}

func TestCovDescribeKeyMoveDown(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 0
	result, _ := m.handleDescribeKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.describeCursor)
}

func TestCovDescribeKeyMoveUp(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 5
	result, _ := m.handleDescribeKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 4, rm.describeCursor)
}

func TestCovDescribeKeyMoveLeft(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursorCol = 5
	result, _ := m.handleDescribeKey(keyMsg("h"))
	rm := result.(Model)
	assert.Equal(t, 4, rm.describeCursorCol)
}

func TestCovDescribeKeyMoveRight(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursorCol = 0
	result, _ := m.handleDescribeKey(keyMsg("l"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.describeCursorCol)
}

func TestCovDescribeKeyZero(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursorCol = 5
	result, _ := m.handleDescribeKey(keyMsg("0"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.describeCursorCol)
}

func TestCovDescribeKeyZeroInLineInput(t *testing.T) {
	m := baseModelDescribe()
	m.describeLineInput = "12"
	result, _ := m.handleDescribeKey(keyMsg("0"))
	rm := result.(Model)
	assert.Equal(t, "120", rm.describeLineInput)
}

func TestCovDescribeKeyDollar(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 0
	result, _ := m.handleDescribeKey(keyMsg("$"))
	rm := result.(Model)
	// "line0" has 5 chars, so cursor col should be at 4
	assert.Equal(t, 4, rm.describeCursorCol)
}

func TestCovDescribeKeyCaret(t *testing.T) {
	m := baseModelDescribe()
	m.describeContent = "   indented"
	m.describeCursor = 0
	result, _ := m.handleDescribeKey(keyMsg("^"))
	rm := result.(Model)
	assert.Equal(t, 3, rm.describeCursorCol)
}

func TestCovDescribeKeyWordMotions(t *testing.T) {
	m := baseModelDescribe()
	m.describeContent = "hello world foo"
	m.describeCursor = 0
	m.describeCursorCol = 0

	result, _ := m.handleDescribeKey(keyMsg("w"))
	rm := result.(Model)
	assert.Greater(t, rm.describeCursorCol, 0)

	result, _ = rm.handleDescribeKey(keyMsg("b"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.describeCursorCol)

	result, _ = rm.handleDescribeKey(keyMsg("e"))
	rm = result.(Model)
	assert.Greater(t, rm.describeCursorCol, 0)

	result, _ = rm.handleDescribeKey(keyMsg("W"))
	rm = result.(Model)
	assert.Greater(t, rm.describeCursorCol, 0)

	result, _ = rm.handleDescribeKey(keyMsg("B"))
	rm = result.(Model)

	result, _ = rm.handleDescribeKey(keyMsg("E"))
	rm = result.(Model)
	assert.Greater(t, rm.describeCursorCol, 0)
}

func TestCovDescribeKeyCtrlD(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 0
	result, _ := m.handleDescribeKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.Greater(t, rm.describeCursor, 0)
}

func TestCovDescribeKeyCtrlU(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 5
	result, _ := m.handleDescribeKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.Less(t, rm.describeCursor, 5)
}

func TestCovDescribeKeyCtrlF(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 0
	result, _ := m.handleDescribeKey(keyMsg("ctrl+f"))
	rm := result.(Model)
	assert.Greater(t, rm.describeCursor, 0)
}

func TestCovDescribeKeyCtrlB(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 9
	result, _ := m.handleDescribeKey(keyMsg("ctrl+b"))
	rm := result.(Model)
	assert.Less(t, rm.describeCursor, 9)
}

func TestCovDescribeKeyG(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 5
	// First 'g' sets pendingG
	result, _ := m.handleDescribeKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)

	// Second 'g' jumps to top
	result, _ = rm.handleDescribeKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.describeCursor)
	assert.False(t, rm.pendingG)
}

func TestCovDescribeKeyGBig(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 0
	result, _ := m.handleDescribeKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 9, rm.describeCursor)
}

func TestCovDescribeKeyGBigWithLineInput(t *testing.T) {
	m := baseModelDescribe()
	m.describeLineInput = "3"
	result, _ := m.handleDescribeKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.describeCursor) // 3-1=2 (0-indexed)
}

func TestCovDescribeKeyDigit(t *testing.T) {
	m := baseModelDescribe()
	result, _ := m.handleDescribeKey(keyMsg("5"))
	rm := result.(Model)
	assert.Equal(t, "5", rm.describeLineInput)
}

func TestCovDescribeKeyVisualV(t *testing.T) {
	m := baseModelDescribe()
	result, _ := m.handleDescribeKey(keyMsg("v"))
	rm := result.(Model)
	assert.Equal(t, byte('v'), rm.describeVisualMode)
}

func TestCovDescribeKeyVisualShiftV(t *testing.T) {
	m := baseModelDescribe()
	result, _ := m.handleDescribeKey(keyMsg("V"))
	rm := result.(Model)
	assert.Equal(t, byte('V'), rm.describeVisualMode)
}

func TestCovDescribeKeyVisualCtrlV(t *testing.T) {
	m := baseModelDescribe()
	result, _ := m.handleDescribeKey(keyMsg("ctrl+v"))
	rm := result.(Model)
	assert.Equal(t, byte('B'), rm.describeVisualMode)
}

func TestCovDescribeKeyYank(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 0
	_, cmd := m.handleDescribeKey(keyMsg("y"))
	assert.NotNil(t, cmd)
}

func TestCovDescribeKeySlash(t *testing.T) {
	m := baseModelDescribe()
	result, _ := m.handleDescribeKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.describeSearchActive)
}

func TestCovDescribeKeySearchNav(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchQuery = "line"
	// n searches forward
	result, _ := m.handleDescribeKey(keyMsg("n"))
	rm := result.(Model)
	assert.NotEqual(t, -1, rm.describeCursor)

	// N searches backward
	rm.describeCursor = 5
	result, _ = rm.handleDescribeKey(keyMsg("N"))
	rm = result.(Model)
	assert.NotEqual(t, -1, rm.describeCursor)
}

func TestCovDescribeKeyDefault(t *testing.T) {
	m := baseModelDescribe()
	m.describeLineInput = "123"
	result, _ := m.handleDescribeKey(keyMsg("x"))
	rm := result.(Model)
	assert.Empty(t, rm.describeLineInput)
}

func TestCovDescribeVisualKeyEsc(t *testing.T) {
	m := baseModelDescribe()
	m.describeVisualMode = 'V'
	result, _ := m.handleDescribeVisualKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Zero(t, rm.describeVisualMode)
}

func TestCovDescribeVisualKeyToggleV(t *testing.T) {
	m := baseModelDescribe()
	m.describeVisualMode = 'V'
	result, _ := m.handleDescribeVisualKey(keyMsg("V"))
	rm := result.(Model)
	assert.Zero(t, rm.describeVisualMode)
}

func TestCovDescribeVisualKeyToggleSwitchV(t *testing.T) {
	m := baseModelDescribe()
	m.describeVisualMode = 'v'
	result, _ := m.handleDescribeVisualKey(keyMsg("V"))
	rm := result.(Model)
	assert.Equal(t, byte('V'), rm.describeVisualMode)
}

func TestCovDescribeVisualKeyToggleLowerV(t *testing.T) {
	m := baseModelDescribe()
	m.describeVisualMode = 'v'
	result, _ := m.handleDescribeVisualKey(keyMsg("v"))
	rm := result.(Model)
	assert.Zero(t, rm.describeVisualMode)
}

func TestCovDescribeVisualKeyCtrlV(t *testing.T) {
	m := baseModelDescribe()
	m.describeVisualMode = 'B'
	result, _ := m.handleDescribeVisualKey(keyMsg("ctrl+v"))
	rm := result.(Model)
	assert.Zero(t, rm.describeVisualMode)
}

func TestCovDescribeVisualKeyCtrlVOn(t *testing.T) {
	m := baseModelDescribe()
	m.describeVisualMode = 'v'
	result, _ := m.handleDescribeVisualKey(keyMsg("ctrl+v"))
	rm := result.(Model)
	assert.Equal(t, byte('B'), rm.describeVisualMode)
}

func TestCovDescribeVisualKeyMovement(t *testing.T) {
	m := baseModelDescribe()
	m.describeVisualMode = 'V'
	m.describeCursor = 2
	m.describeCursorCol = 2

	result, _ := m.handleDescribeVisualKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 3, rm.describeCursor)

	result, _ = rm.handleDescribeVisualKey(keyMsg("k"))
	rm = result.(Model)
	assert.Equal(t, 2, rm.describeCursor)

	result, _ = rm.handleDescribeVisualKey(keyMsg("l"))
	rm = result.(Model)
	assert.Equal(t, 3, rm.describeCursorCol)

	rm.describeCursorCol = 2
	result, _ = rm.handleDescribeVisualKey(keyMsg("h"))
	rm = result.(Model)
	assert.Equal(t, 1, rm.describeCursorCol)

	result, _ = rm.handleDescribeVisualKey(keyMsg("0"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.describeCursorCol)

	result, _ = rm.handleDescribeVisualKey(keyMsg("$"))
	rm = result.(Model)
	assert.Greater(t, rm.describeCursorCol, 0)

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
	m.describeVisualMode = 'V'
	m.describeCursor = 5

	result, _ := m.handleDescribeVisualKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 9, rm.describeCursor)

	rm.pendingG = false
	result, _ = rm.handleDescribeVisualKey(keyMsg("g"))
	rm = result.(Model)
	assert.True(t, rm.pendingG)

	result, _ = rm.handleDescribeVisualKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.describeCursor)
}

func TestCovDescribeVisualKeyPageMovement(t *testing.T) {
	m := baseModelDescribe()
	m.describeVisualMode = 'V'
	m.describeCursor = 0

	result, _ := m.handleDescribeVisualKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.Greater(t, rm.describeCursor, 0)

	rm.describeCursor = 9
	result, _ = rm.handleDescribeVisualKey(keyMsg("ctrl+u"))
	rm = result.(Model)
	assert.Less(t, rm.describeCursor, 9)
}

func TestCovDescribeVisualKeyCopyLineMode(t *testing.T) {
	m := baseModelDescribe()
	m.describeVisualMode = 'V'
	m.describeCursor = 2
	m.describeVisualStart = 0
	_, cmd := m.handleDescribeVisualKey(keyMsg("y"))
	assert.NotNil(t, cmd)
}

func TestCovDescribeVisualKeyCopyCharMode(t *testing.T) {
	m := baseModelDescribe()
	m.describeVisualMode = 'v'
	m.describeCursor = 1
	m.describeVisualStart = 0
	m.describeVisualCol = 0
	m.describeCursorCol = 3
	_, cmd := m.handleDescribeVisualKey(keyMsg("y"))
	assert.NotNil(t, cmd)
}

func TestCovDescribeVisualKeyCopyBlockMode(t *testing.T) {
	m := baseModelDescribe()
	m.describeVisualMode = 'B'
	m.describeCursor = 2
	m.describeVisualStart = 0
	m.describeVisualCol = 0
	m.describeCursorCol = 3
	_, cmd := m.handleDescribeVisualKey(keyMsg("y"))
	assert.NotNil(t, cmd)
}

func TestCovDescribeVisualKeyCopyCharModeSameLine(t *testing.T) {
	m := baseModelDescribe()
	m.describeVisualMode = 'v'
	m.describeCursor = 0
	m.describeVisualStart = 0
	m.describeVisualCol = 0
	m.describeCursorCol = 3
	_, cmd := m.handleDescribeVisualKey(keyMsg("y"))
	assert.NotNil(t, cmd)
}

func TestCovDescribeSearchKeyEnter(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchActive = true
	m.describeSearchInput.Insert("line")
	result, _ := m.handleDescribeSearchKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.describeSearchActive)
	assert.Equal(t, "line", rm.describeSearchQuery)
}

func TestCovDescribeSearchKeyEsc(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchActive = true
	result, _ := m.handleDescribeSearchKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.describeSearchActive)
}

func TestCovDescribeSearchKeyBackspace(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchActive = true
	m.describeSearchInput.Insert("ab")
	result, _ := m.handleDescribeSearchKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "a", rm.describeSearchInput.Value)
}

func TestCovDescribeSearchKeyCtrlW(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchActive = true
	m.describeSearchInput.Insert("foo bar")
	result, _ := m.handleDescribeSearchKey(keyMsg("ctrl+w"))
	rm := result.(Model)
	assert.NotEqual(t, "foo bar", rm.describeSearchInput.Value)
}

func TestCovDescribeSearchKeyCtrlA(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchActive = true
	m.describeSearchInput.Insert("abc")
	result, _ := m.handleDescribeSearchKey(keyMsg("ctrl+a"))
	_ = result.(Model)
}

func TestCovDescribeSearchKeyCtrlE(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchActive = true
	m.describeSearchInput.Insert("abc")
	result, _ := m.handleDescribeSearchKey(keyMsg("ctrl+e"))
	_ = result.(Model)
}

func TestCovDescribeSearchKeyLeftRight(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchActive = true
	m.describeSearchInput.Insert("abc")
	result, _ := m.handleDescribeSearchKey(keyMsg("left"))
	rm := result.(Model)
	result, _ = rm.handleDescribeSearchKey(keyMsg("right"))
	_ = result.(Model)
}

func TestCovDescribeSearchKeyInsertChar(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchActive = true
	result, _ := m.handleDescribeSearchKey(keyMsg("x"))
	rm := result.(Model)
	assert.Equal(t, "x", rm.describeSearchInput.Value)
}

func TestCovFindNextDescribeMatchForward(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchQuery = "line5"
	m.describeCursor = 0
	m.findNextDescribeMatch(true)
	assert.Equal(t, 5, m.describeCursor)
}

func TestCovFindNextDescribeMatchBackward(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchQuery = "line3"
	m.describeCursor = 5
	m.findNextDescribeMatch(false)
	assert.Equal(t, 3, m.describeCursor)
}

func TestCovFindNextDescribeMatchNoQuery(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchQuery = ""
	m.describeCursor = 5
	m.findNextDescribeMatch(true)
	assert.Equal(t, 5, m.describeCursor) // unchanged
}

func TestCovFindNextDescribeMatchNotFound(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchQuery = "nonexistent"
	m.describeCursor = 0
	m.findNextDescribeMatch(true)
	assert.Equal(t, 0, m.describeCursor) // unchanged
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
	m.describeCursor = 100 // out of bounds
	m.ensureDescribeCursorVisible()
	assert.LessOrEqual(t, m.describeCursor, 9)
}

func TestCovDescribeKeyDispatchToSearch(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchActive = true
	m.describeSearchInput.Insert("test")
	result, _ := m.handleDescribeKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.describeSearchActive)
}

func TestCovDescribeKeyDispatchToVisual(t *testing.T) {
	m := baseModelDescribe()
	m.describeVisualMode = 'V'
	m.describeCursor = 2
	result, _ := m.handleDescribeKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 3, rm.describeCursor)
}
