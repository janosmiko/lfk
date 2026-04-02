package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// --- handleDescribeKey ---

func TestDescribeKeyEscReturnsToExplorer(t *testing.T) {
	m := Model{
		mode:            modeDescribe,
		describeContent: "line1\nline2\nline3",
		describeScroll:  5,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
	assert.Equal(t, 0, result.describeScroll)
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

func TestDescribeKeyJScrollsDown(t *testing.T) {
	content := strings.Repeat("line\n", 100)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeScroll:  0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.describeScroll)
}

func TestDescribeKeyKScrollsUp(t *testing.T) {
	content := strings.Repeat("line\n", 100)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeScroll:  10,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(runeKey('k'))
	result := ret.(Model)
	assert.Equal(t, 9, result.describeScroll)
}

func TestDescribeKeyKAtZeroStays(t *testing.T) {
	m := Model{
		mode:            modeDescribe,
		describeContent: "line1\nline2",
		describeScroll:  0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(runeKey('k'))
	result := ret.(Model)
	assert.Equal(t, 0, result.describeScroll)
}

func TestDescribeKeyGGScrollsToTop(t *testing.T) {
	content := strings.Repeat("line\n", 100)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeScroll:  50,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	// First g sets pendingG
	ret, _ := m.handleDescribeKey(runeKey('g'))
	result := ret.(Model)
	assert.True(t, result.pendingG)

	// Second g scrolls to top
	ret2, _ := result.handleDescribeKey(runeKey('g'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.describeScroll)
	assert.False(t, result2.pendingG)
}

func TestDescribeKeyGScrollsToBottom(t *testing.T) {
	content := strings.Repeat("line\n", 100)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeScroll:  0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(runeKey('G'))
	result := ret.(Model)
	assert.Greater(t, result.describeScroll, 0)
}

func TestDescribeKeyCtrlDHalfPageDown(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeScroll:  0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	result := ret.(Model)
	assert.Equal(t, 20, result.describeScroll) // height/2 = 40/2 = 20
}

func TestDescribeKeyCtrlUHalfPageUp(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeScroll:  30,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Equal(t, 10, result.describeScroll)
}

func TestDescribeKeyCtrlUClampsToZero(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeScroll:  5,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Equal(t, 0, result.describeScroll)
}

func TestDescribeKeyCtrlFFullPageDown(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeScroll:  0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	result := ret.(Model)
	assert.Equal(t, 40, result.describeScroll)
}

func TestDescribeKeyCtrlBFullPageUp(t *testing.T) {
	content := strings.Repeat("line\n", 200)
	m := Model{
		mode:            modeDescribe,
		describeContent: content,
		describeScroll:  60,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleDescribeKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	result := ret.(Model)
	assert.Equal(t, 20, result.describeScroll)
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
