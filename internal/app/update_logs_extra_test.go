package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// --- handleLogKey ---

func TestLogKeyEscReturnsToExplorer(t *testing.T) {
	m := Model{
		mode:     modeLogs,
		logLines: []string{"line1", "line2"},
		tabs:     []TabState{{}},
		width:    80,
		height:   40,
	}
	ret, _ := m.handleLogKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
}

func TestLogKeyQuestionMarkOpensHelp(t *testing.T) {
	m := Model{
		mode:     modeLogs,
		logLines: []string{"line1"},
		tabs:     []TabState{{}},
		width:    80,
		height:   40,
	}
	ret, _ := m.handleLogKey(runeKey('?'))
	result := ret.(Model)
	assert.Equal(t, modeHelp, result.mode)
	assert.Equal(t, modeLogs, result.helpPreviousMode)
}

func TestLogKeyJMovesDown(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  []string{"a", "b", "c"},
		logCursor: 0,
		logFollow: true,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	ret, _ := m.handleLogKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.logCursor)
	assert.False(t, result.logFollow)
}

func TestLogKeyKMovesUp(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  []string{"a", "b", "c"},
		logCursor: 2,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	ret, _ := m.handleLogKey(runeKey('k'))
	result := ret.(Model)
	assert.Equal(t, 1, result.logCursor)
}

func TestLogKeyFTogglesFollow(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  []string{"a", "b", "c"},
		logCursor: 0,
		logFollow: false,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	ret, _ := m.handleLogKey(runeKey('F'))
	result := ret.(Model)
	assert.True(t, result.logFollow)
	assert.Equal(t, 2, result.logCursor)
}

func TestLogKeyTabTogglesWrap(t *testing.T) {
	m := Model{
		mode:     modeLogs,
		logLines: []string{"a"},
		logWrap:  false,
		tabs:     []TabState{{}},
		width:    80,
		height:   40,
	}
	ret, _ := m.handleLogKey(specialKey(tea.KeyTab))
	result := ret.(Model)
	assert.True(t, result.logWrap)
}

func TestLogKeyZTogglesWrap(t *testing.T) {
	m := Model{
		mode:     modeLogs,
		logLines: []string{"a"},
		logWrap:  false,
		tabs:     []TabState{{}},
		width:    80,
		height:   40,
	}
	ret, _ := m.handleLogKey(runeKey('z'))
	result := ret.(Model)
	assert.True(t, result.logWrap)
}

func TestLogKeyHashTogglesLineNumbers(t *testing.T) {
	m := Model{
		mode:           modeLogs,
		logLines:       []string{"a"},
		logLineNumbers: false,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleLogKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'#'}})
	result := ret.(Model)
	assert.True(t, result.logLineNumbers)
}

func TestLogKeySTogglesTimestamps(t *testing.T) {
	m := Model{
		mode:          modeLogs,
		logLines:      []string{"a"},
		logTimestamps: false,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleLogKey(runeKey('s'))
	result := ret.(Model)
	assert.True(t, result.logTimestamps)
}

func TestLogKeySlashEntersSearch(t *testing.T) {
	m := Model{
		mode:     modeLogs,
		logLines: []string{"a"},
		tabs:     []TabState{{}},
		width:    80,
		height:   40,
	}
	ret, _ := m.handleLogKey(runeKey('/'))
	result := ret.(Model)
	assert.True(t, result.logSearchActive)
}

func TestLogKeyNNextMatch(t *testing.T) {
	m := Model{
		mode:           modeLogs,
		logLines:       []string{"error first", "info", "error second"},
		logSearchQuery: "error",
		logCursor:      0,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleLogKey(runeKey('n'))
	result := ret.(Model)
	assert.Equal(t, 2, result.logCursor)
}

func TestLogKeyNPrevMatch(t *testing.T) {
	m := Model{
		mode:           modeLogs,
		logLines:       []string{"error first", "info", "error second"},
		logSearchQuery: "error",
		logCursor:      2,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleLogKey(runeKey('N'))
	result := ret.(Model)
	assert.Equal(t, 0, result.logCursor)
}

func TestLogKeyPTogglesPrefixes(t *testing.T) {
	m := Model{
		mode:     modeLogs,
		logLines: []string{"[pod/app/web] some log"},
		tabs:     []TabState{{}},
		width:    80,
		height:   40,
	}
	assert.False(t, m.logHidePrefixes)
	ret, _ := m.handleLogKey(runeKey('p'))
	result := ret.(Model)
	assert.True(t, result.logHidePrefixes)
	ret2, _ := result.handleLogKey(runeKey('p'))
	result2 := ret2.(Model)
	assert.False(t, result2.logHidePrefixes)
}

func TestLogSearchJumpsCursorColumn(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"start middle_error end", "no match", "another error_here"},
		logSearchQuery:  "error",
		logCursor:       0,
		logVisualCurCol: 0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	// Forward search from col 0: finds "error" at col 13 on the same line.
	ret, _ := m.handleLogKey(runeKey('n'))
	result := ret.(Model)
	assert.Equal(t, 0, result.logCursor)
	assert.Equal(t, 13, result.logVisualCurCol)

	// Next match: no more matches on line 0 after col 13, jumps to line 2.
	ret2, _ := result.handleLogKey(runeKey('n'))
	result2 := ret2.(Model)
	assert.Equal(t, 2, result2.logCursor)
	assert.Equal(t, 8, result2.logVisualCurCol)
}

func TestLogSearchIntraLineMultipleMatches(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"error first error second error third"},
		logSearchQuery:  "error",
		logCursor:       0,
		logVisualCurCol: 0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	// First match at col 0 -> next at col 12 -> next at col 25 -> wraps to col 0.
	ret, _ := m.handleLogKey(runeKey('n'))
	r := ret.(Model)
	assert.Equal(t, 0, r.logCursor)
	assert.Equal(t, 12, r.logVisualCurCol)

	ret2, _ := r.handleLogKey(runeKey('n'))
	r2 := ret2.(Model)
	assert.Equal(t, 0, r2.logCursor)
	assert.Equal(t, 25, r2.logVisualCurCol)

	// Next wraps around to col 0.
	ret3, _ := r2.handleLogKey(runeKey('n'))
	r3 := ret3.(Model)
	assert.Equal(t, 0, r3.logCursor)
	assert.Equal(t, 0, r3.logVisualCurCol)
}

func TestLogKeyGGGoesToTop(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  []string{"a", "b", "c", "d", "e"},
		logCursor: 4,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	// First g
	ret, _ := m.handleLogKey(runeKey('g'))
	result := ret.(Model)
	assert.True(t, result.pendingG)

	// Second g
	ret2, _ := result.handleLogKey(runeKey('g'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.logCursor)
	assert.False(t, result2.pendingG)
	assert.False(t, result2.logFollow)
}

func TestLogKeyGGoesToBottom(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  []string{"a", "b", "c", "d", "e"},
		logCursor: 0,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	ret, _ := m.handleLogKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 4, result.logCursor)
	assert.True(t, result.logFollow)
}

func TestLogKeyGWithDigitJumpsToLine(t *testing.T) {
	m := Model{
		mode:         modeLogs,
		logLines:     []string{"a", "b", "c", "d", "e"},
		logCursor:    0,
		logLineInput: "3",
		tabs:         []TabState{{}},
		width:        80,
		height:       40,
	}
	ret, _ := m.handleLogKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 2, result.logCursor) // 3 - 1 = 2 (0-indexed)
}

func TestLogKeyCtrlDHalfPageDown(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  make([]string, 100),
		logCursor: 0,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	ret, _ := m.handleLogKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	result := ret.(Model)
	assert.Greater(t, result.logCursor, 0)
}

func TestLogKeyCtrlUHalfPageUp(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  make([]string, 100),
		logCursor: 50,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	ret, _ := m.handleLogKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Less(t, result.logCursor, 50)
}

func TestLogKeyCtrlFFullPageDown(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  make([]string, 100),
		logCursor: 0,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	ret, _ := m.handleLogKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	result := ret.(Model)
	assert.Greater(t, result.logCursor, 0)
}

func TestLogKeyCtrlBFullPageUp(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  make([]string, 100),
		logCursor: 50,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	ret, _ := m.handleLogKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	result := ret.(Model)
	assert.Less(t, result.logCursor, 50)
}

func TestLogKeyDigitBuffering(t *testing.T) {
	m := Model{
		mode:         modeLogs,
		logLines:     []string{"a"},
		logLineInput: "",
		tabs:         []TabState{{}},
		width:        80,
		height:       40,
	}
	ret, _ := m.handleLogKey(runeKey('5'))
	result := ret.(Model)
	assert.Equal(t, "5", result.logLineInput)
}

func TestLogKeyZeroMovesToStartOfLine(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"hello world"},
		logCursor:       0,
		logVisualCurCol: 5,
		logLineInput:    "",
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleLogKey(runeKey('0'))
	result := ret.(Model)
	assert.Equal(t, 0, result.logVisualCurCol)
}

func TestLogKeyZeroWithDigitsPending(t *testing.T) {
	m := Model{
		mode:         modeLogs,
		logLines:     []string{"hello"},
		logLineInput: "1",
		tabs:         []TabState{{}},
		width:        80,
		height:       40,
	}
	ret, _ := m.handleLogKey(runeKey('0'))
	result := ret.(Model)
	assert.Equal(t, "10", result.logLineInput)
}

func TestLogKeyHMovesLeft(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"hello"},
		logVisualCurCol: 3,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleLogKey(runeKey('h'))
	result := ret.(Model)
	assert.Equal(t, 2, result.logVisualCurCol)
}

func TestLogKeyLMovesRight(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"hello"},
		logVisualCurCol: 3,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleLogKey(runeKey('l'))
	result := ret.(Model)
	assert.Equal(t, 4, result.logVisualCurCol)
}

func TestLogKeyDollarMovesToEndOfLine(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"hello world"},
		logCursor:       0,
		logVisualCurCol: 0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleLogKey(runeKey('$'))
	result := ret.(Model)
	assert.Equal(t, len("hello world")-1, result.logVisualCurCol)
}

func TestLogKeyCaretMovesToFirstNonWhitespace(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"   hello"},
		logCursor:       0,
		logVisualCurCol: 0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleLogKey(runeKey('^'))
	result := ret.(Model)
	assert.Equal(t, 3, result.logVisualCurCol) // first non-space
}

func TestLogKeyVEntersCharVisualMode(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  []string{"a", "b", "c"},
		logCursor: 1,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	ret, _ := m.handleLogKey(runeKey('v'))
	result := ret.(Model)
	assert.True(t, result.logVisualMode)
	assert.Equal(t, rune('v'), result.logVisualType)
	assert.Equal(t, 1, result.logVisualStart)
}

func TestLogKeyUpperVEntersLineVisualMode(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  []string{"a", "b", "c"},
		logCursor: 1,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	ret, _ := m.handleLogKey(runeKey('V'))
	result := ret.(Model)
	assert.True(t, result.logVisualMode)
	assert.Equal(t, rune('V'), result.logVisualType)
}

func TestLogKeyCtrlVEntersBlockVisualMode(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  []string{"a", "b", "c"},
		logCursor: 1,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	ret, _ := m.handleLogKey(tea.KeyMsg{Type: tea.KeyCtrlV})
	result := ret.(Model)
	assert.True(t, result.logVisualMode)
	assert.Equal(t, rune('B'), result.logVisualType)
}

// --- handleLogVisualKey ---

func TestLogVisualKeyEscExits(t *testing.T) {
	m := Model{
		mode:          modeLogs,
		logLines:      []string{"a", "b"},
		logVisualMode: true,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleLogVisualKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.logVisualMode)
}

func TestLogVisualKeyQExits(t *testing.T) {
	m := Model{
		mode:          modeLogs,
		logLines:      []string{"a", "b"},
		logVisualMode: true,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('q'))
	result := ret.(Model)
	assert.False(t, result.logVisualMode)
}

func TestLogVisualKeyVToggle(t *testing.T) {
	t.Run("v in char mode cancels", func(t *testing.T) {
		m := Model{
			mode:          modeLogs,
			logLines:      []string{"a"},
			logVisualMode: true,
			logVisualType: 'v',
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('v'))
		result := ret.(Model)
		assert.False(t, result.logVisualMode)
	})

	t.Run("v in line mode switches to char", func(t *testing.T) {
		m := Model{
			mode:          modeLogs,
			logLines:      []string{"a"},
			logVisualMode: true,
			logVisualType: 'V',
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('v'))
		result := ret.(Model)
		assert.True(t, result.logVisualMode)
		assert.Equal(t, rune('v'), result.logVisualType)
	})
}

func TestLogVisualKeyVVToggle(t *testing.T) {
	t.Run("V in line mode cancels", func(t *testing.T) {
		m := Model{
			mode:          modeLogs,
			logLines:      []string{"a"},
			logVisualMode: true,
			logVisualType: 'V',
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('V'))
		result := ret.(Model)
		assert.False(t, result.logVisualMode)
	})

	t.Run("V in char mode switches to line", func(t *testing.T) {
		m := Model{
			mode:          modeLogs,
			logLines:      []string{"a"},
			logVisualMode: true,
			logVisualType: 'v',
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('V'))
		result := ret.(Model)
		assert.True(t, result.logVisualMode)
		assert.Equal(t, rune('V'), result.logVisualType)
	})
}

func TestLogVisualKeyCtrlVToggle(t *testing.T) {
	t.Run("ctrl+v in block mode cancels", func(t *testing.T) {
		m := Model{
			mode:          modeLogs,
			logLines:      []string{"a"},
			logVisualMode: true,
			logVisualType: 'B',
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleLogVisualKey(tea.KeyMsg{Type: tea.KeyCtrlV})
		result := ret.(Model)
		assert.False(t, result.logVisualMode)
	})

	t.Run("ctrl+v in char mode switches to block", func(t *testing.T) {
		m := Model{
			mode:          modeLogs,
			logLines:      []string{"a"},
			logVisualMode: true,
			logVisualType: 'v',
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleLogVisualKey(tea.KeyMsg{Type: tea.KeyCtrlV})
		result := ret.(Model)
		assert.True(t, result.logVisualMode)
		assert.Equal(t, rune('B'), result.logVisualType)
	})
}

func TestLogVisualKeyJKNavigation(t *testing.T) {
	m := Model{
		mode:          modeLogs,
		logLines:      []string{"a", "b", "c"},
		logVisualMode: true,
		logCursor:     0,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}

	ret, _ := m.handleLogVisualKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.logCursor)

	ret2, _ := result.handleLogVisualKey(runeKey('k'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.logCursor)
}

func TestLogVisualKeyHLNavigation(t *testing.T) {
	t.Run("h moves left in char mode", func(t *testing.T) {
		m := Model{
			mode:            modeLogs,
			logLines:        []string{"hello"},
			logVisualMode:   true,
			logVisualType:   'v',
			logVisualCurCol: 3,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('h'))
		result := ret.(Model)
		assert.Equal(t, 2, result.logVisualCurCol)
	})

	t.Run("l moves right in char mode", func(t *testing.T) {
		m := Model{
			mode:            modeLogs,
			logLines:        []string{"hello"},
			logVisualMode:   true,
			logVisualType:   'v',
			logVisualCurCol: 3,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('l'))
		result := ret.(Model)
		assert.Equal(t, 4, result.logVisualCurCol)
	})

	t.Run("h moves left in block mode", func(t *testing.T) {
		m := Model{
			mode:            modeLogs,
			logLines:        []string{"hello"},
			logVisualMode:   true,
			logVisualType:   'B',
			logVisualCurCol: 3,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('h'))
		result := ret.(Model)
		assert.Equal(t, 2, result.logVisualCurCol)
	})
}

func TestLogVisualKeyZeroMovesToStart(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"hello"},
		logVisualMode:   true,
		logVisualCurCol: 3,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('0'))
	result := ret.(Model)
	assert.Equal(t, 0, result.logVisualCurCol)
}

func TestLogVisualKeyGScrollsToEnd(t *testing.T) {
	m := Model{
		mode:          modeLogs,
		logLines:      []string{"a", "b", "c", "d", "e"},
		logVisualMode: true,
		logVisualType: 'V',
		logCursor:     0,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 4, result.logCursor) // last line index
}

func TestLogVisualKeyGgScrollsToTop(t *testing.T) {
	m := Model{
		mode:          modeLogs,
		logLines:      []string{"a", "b", "c", "d"},
		logVisualMode: true,
		logVisualType: 'V',
		logCursor:     3,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('g'))
	result := ret.(Model)
	assert.True(t, result.pendingG)

	ret2, _ := result.handleLogVisualKey(runeKey('g'))
	result2 := ret2.(Model)
	assert.False(t, result2.pendingG)
	assert.Equal(t, 0, result2.logCursor)
}

func TestLogVisualKeyCtrlDU(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line"
	}

	t.Run("ctrl+d moves down half page", func(t *testing.T) {
		m := Model{
			mode:          modeLogs,
			logLines:      lines,
			logVisualMode: true,
			logVisualType: 'V',
			logCursor:     0,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleLogVisualKey(tea.KeyMsg{Type: tea.KeyCtrlD})
		result := ret.(Model)
		assert.Greater(t, result.logCursor, 0)
	})

	t.Run("ctrl+u moves up half page", func(t *testing.T) {
		m := Model{
			mode:          modeLogs,
			logLines:      lines,
			logVisualMode: true,
			logVisualType: 'V',
			logCursor:     50,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleLogVisualKey(tea.KeyMsg{Type: tea.KeyCtrlU})
		result := ret.(Model)
		assert.Less(t, result.logCursor, 50)
	})
}

func TestLogVisualKeyDollarMovesToEnd(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"hello world"},
		logVisualMode:   true,
		logVisualType:   'v',
		logVisualCurCol: 0,
		logCursor:       0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('$'))
	result := ret.(Model)
	assert.Equal(t, 10, result.logVisualCurCol) // len("hello world") - 1 = 10
}

func TestLogVisualKeyCaretMovesToFirstNonWhitespace(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"   hello"},
		logVisualMode:   true,
		logVisualType:   'v',
		logVisualCurCol: 0,
		logCursor:       0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('^'))
	result := ret.(Model)
	assert.Equal(t, 3, result.logVisualCurCol) // first non-ws at index 3
}

func TestLogVisualKeyWordMotions(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"hello world foo"},
		logVisualMode:   true,
		logVisualType:   'v',
		logVisualCurCol: 0,
		logCursor:       0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}

	t.Run("w moves to next word", func(t *testing.T) {
		ret, _ := m.handleLogVisualKey(runeKey('w'))
		result := ret.(Model)
		assert.Greater(t, result.logVisualCurCol, 0)
	})

	t.Run("e moves to end of word", func(t *testing.T) {
		ret, _ := m.handleLogVisualKey(runeKey('e'))
		result := ret.(Model)
		assert.Greater(t, result.logVisualCurCol, 0)
	})

	t.Run("b moves to prev word start", func(t *testing.T) {
		m2 := m
		m2.logVisualCurCol = 6
		ret, _ := m2.handleLogVisualKey(runeKey('b'))
		result := ret.(Model)
		assert.Less(t, result.logVisualCurCol, 6)
	})
}

func TestLogVisualKeyWORDMotions(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"hello world foo"},
		logVisualMode:   true,
		logVisualType:   'v',
		logVisualCurCol: 0,
		logCursor:       0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}

	t.Run("W moves to next WORD", func(t *testing.T) {
		ret, _ := m.handleLogVisualKey(runeKey('W'))
		result := ret.(Model)
		assert.Greater(t, result.logVisualCurCol, 0)
	})

	t.Run("E moves to end of WORD", func(t *testing.T) {
		ret, _ := m.handleLogVisualKey(runeKey('E'))
		result := ret.(Model)
		assert.Greater(t, result.logVisualCurCol, 0)
	})

	t.Run("B moves to prev WORD start", func(t *testing.T) {
		m2 := m
		m2.logVisualCurCol = 6
		ret, _ := m2.handleLogVisualKey(runeKey('B'))
		result := ret.(Model)
		assert.Less(t, result.logVisualCurCol, 6)
	})
}

func TestLogVisualKeyQExitsVisualMode(t *testing.T) {
	m := Model{
		mode:          modeLogs,
		logLines:      []string{"a"},
		logVisualMode: true,
		logVisualType: 'V',
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('q'))
	result := ret.(Model)
	assert.False(t, result.logVisualMode)
}

func TestLogVisualKeyYankLineMode(t *testing.T) {
	m := Model{
		mode:           modeLogs,
		logLines:       []string{"line1", "line2", "line3"},
		logVisualMode:  true,
		logVisualType:  'V',
		logCursor:      2,
		logVisualStart: 1,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, cmd := m.handleLogVisualKey(runeKey('y'))
	result := ret.(Model)
	assert.False(t, result.logVisualMode, "visual mode should exit after yank")
	assert.Contains(t, result.statusMessage, "Copied 2 lines")
	assert.NotNil(t, cmd)
}

func TestLogVisualKeyYankCharMode(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"hello world"},
		logVisualMode:   true,
		logVisualType:   'v',
		logCursor:       0,
		logVisualStart:  0,
		logVisualCol:    0,
		logVisualCurCol: 4,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, cmd := m.handleLogVisualKey(runeKey('y'))
	result := ret.(Model)
	assert.False(t, result.logVisualMode)
	assert.Contains(t, result.statusMessage, "Copied 1 lines")
	assert.NotNil(t, cmd)
}

func TestLogVisualKeyYankBlockMode(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"abc", "def", "ghi"},
		logVisualMode:   true,
		logVisualType:   'B',
		logCursor:       2,
		logVisualStart:  0,
		logVisualCol:    0,
		logVisualCurCol: 1,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, cmd := m.handleLogVisualKey(runeKey('y'))
	result := ret.(Model)
	assert.False(t, result.logVisualMode)
	assert.Contains(t, result.statusMessage, "Copied 3 lines")
	assert.NotNil(t, cmd)
}

// --- handleLogSearchKey ---

func TestLogSearchKeyEnterCommitsSearch(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"error: test", "info: ok"},
		logSearchActive: true,
		logSearchInput:  TextInput{Value: "error"},
		logCursor:       0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleLogSearchKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.logSearchActive)
	assert.Equal(t, "error", result.logSearchQuery)
}

func TestLogSearchKeyEscCancels(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"a"},
		logSearchActive: true,
		logSearchInput:  TextInput{Value: "test"},
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleLogSearchKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.logSearchActive)
	assert.Empty(t, result.logSearchInput.Value)
}

func TestLogSearchKeyTyping(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"a"},
		logSearchActive: true,
		logSearchInput:  TextInput{Value: ""},
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleLogSearchKey(runeKey('e'))
	result := ret.(Model)
	assert.Equal(t, "e", result.logSearchInput.Value)
}

func TestLogSearchKeyBackspace(t *testing.T) {
	m := Model{
		mode:            modeLogs,
		logLines:        []string{"a"},
		logSearchActive: true,
		logSearchInput:  TextInput{Value: "abc", Cursor: 3},
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleLogSearchKey(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "ab", result.logSearchInput.Value)
}

func TestLowerFOpensFilterModal(t *testing.T) {
	m := Model{
		mode:     modeLogs,
		logLines: []string{"a", "b"},
		tabs:     []TabState{{}},
		width:    80,
		height:   40,
	}
	rm, _ := m.handleLogKey(runeKey('f'))
	result := rm.(Model)
	assert.Equal(t, overlayLogFilter, result.overlay)
	assert.True(t, result.logFilterModalOpen)
	// Overlay opens in list (nav) mode so the user sees existing rules;
	// press `a` to add a new one.
	assert.False(t, result.logFilterFocusInput)
	assert.Equal(t, -1, result.logFilterEditingIdx)
	assert.Equal(t, "", result.logFilterInput.Value)
}

func TestShiftFTogglesFollow(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  []string{"a", "b", "c"},
		logCursor: 0,
		logFollow: false,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	rm, _ := m.handleLogKey(runeKey('F'))
	assert.True(t, rm.(Model).logFollow)
	assert.Equal(t, 2, rm.(Model).logCursor)

	m2 := Model{
		mode:      modeLogs,
		logLines:  []string{"a", "b", "c"},
		logCursor: 0,
		logFollow: true,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	rm2, _ := m2.handleLogKey(runeKey('F'))
	assert.False(t, rm2.(Model).logFollow)
}
