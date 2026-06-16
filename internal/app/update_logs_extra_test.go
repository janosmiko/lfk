package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// --- handleLogKey ---

func TestLogKeyEscReturnsToExplorer(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines: []string{"line1", "line2"},
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
}

func TestLogKeyQuestionMarkOpensHelp(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines: []string{"line1"},
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('?'))
	result := ret.(Model)
	assert.Equal(t, modeHelp, result.mode)
	assert.Equal(t, modeLogs, result.helpPreviousMode)
}

func TestLogKeyJMovesDown(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:  []string{"a", "b", "c"},
			cursor: 0,
			follow: true,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.logView.cursor)
	assert.False(t, result.logView.follow)
}

func TestLogKeyKMovesUp(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:  []string{"a", "b", "c"},
			cursor: 2,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('k'))
	result := ret.(Model)
	assert.Equal(t, 1, result.logView.cursor)
}

func TestLogKeyFTogglesFollow(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:  []string{"a", "b", "c"},
			cursor: 0,
			follow: false,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('F'))
	result := ret.(Model)
	assert.True(t, result.logView.follow)
	assert.Equal(t, 2, result.logView.cursor)
}

// TestLogKeyToggleWrapBinding verifies the log viewer wraps on the unified
// ToggleWrap keybinding (default ">").
func TestLogKeyToggleWrapBinding(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines: []string{"a"},
			wrap:  false,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('>'))
	result := ret.(Model)
	assert.True(t, result.logView.wrap)
}

// TestLogKeyTabAndZNoLongerWrap guards the wrap-hotkey unification: tab and z
// used to toggle wrap in the log viewer but are now freed (only ToggleWrap does).
func TestLogKeyTabAndZNoLongerWrap(t *testing.T) {
	base := func() Model {
		return Model{
			mode:    modeLogs,
			logView: logViewState{lines: []string{"a"}, wrap: false},
			tabs:    []TabState{{}},
			width:   80,
			height:  40,
		}
	}
	tabRet, _ := base().handleLogKey(specialKey(tea.KeyTab))
	assert.False(t, tabRet.(Model).logView.wrap, "tab should no longer toggle wrap")

	zRet, _ := base().handleLogKey(runeKey('z'))
	assert.False(t, zRet.(Model).logView.wrap, "z should no longer toggle wrap")
}

func TestLogKeyHashTogglesLineNumbers(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:       []string{"a"},
			lineNumbers: false,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'#'}})
	result := ret.(Model)
	assert.True(t, result.logView.lineNumbers)
}

func TestLogKeySTogglesTimestamps(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:      []string{"a"},
			timestamps: false,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('s'))
	result := ret.(Model)
	assert.True(t, result.logView.timestamps)
}

func TestLogKeySlashEntersSearch(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines: []string{"a"},
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('/'))
	result := ret.(Model)
	assert.True(t, result.logView.searchActive)
}

func TestLogKeyNNextMatch(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:       []string{"error first", "info", "error second"},
			searchQuery: "error",
			cursor:      0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('n'))
	result := ret.(Model)
	assert.Equal(t, 2, result.logView.cursor)
}

func TestLogKeyNPrevMatch(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:       []string{"error first", "info", "error second"},
			searchQuery: "error",
			cursor:      2,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('N'))
	result := ret.(Model)
	assert.Equal(t, 0, result.logView.cursor)
}

func TestLogKeyPTogglesPrefixes(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines: []string{"[pod/app/web] some log"},
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	assert.False(t, m.logView.hidePrefixes)
	ret, _ := m.handleLogKey(runeKey('p'))
	result := ret.(Model)
	assert.True(t, result.logView.hidePrefixes)
	ret2, _ := result.handleLogKey(runeKey('p'))
	result2 := ret2.(Model)
	assert.False(t, result2.logView.hidePrefixes)
}

func TestLogSearchJumpsCursorColumn(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"start middle_error end", "no match", "another error_here"},
			searchQuery:  "error",
			cursor:       0,
			visualCurCol: 0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	// Forward search from col 0: finds "error" at col 13 on the same line.
	ret, _ := m.handleLogKey(runeKey('n'))
	result := ret.(Model)
	assert.Equal(t, 0, result.logView.cursor)
	assert.Equal(t, 13, result.logView.visualCurCol)

	// Next match: no more matches on line 0 after col 13, jumps to line 2.
	ret2, _ := result.handleLogKey(runeKey('n'))
	result2 := ret2.(Model)
	assert.Equal(t, 2, result2.logView.cursor)
	assert.Equal(t, 8, result2.logView.visualCurCol)
}

func TestLogSearchIntraLineMultipleMatches(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"error first error second error third"},
			searchQuery:  "error",
			cursor:       0,
			visualCurCol: 0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	// First match at col 0 -> next at col 12 -> next at col 25 -> wraps to col 0.
	ret, _ := m.handleLogKey(runeKey('n'))
	r := ret.(Model)
	assert.Equal(t, 0, r.logView.cursor)
	assert.Equal(t, 12, r.logView.visualCurCol)

	ret2, _ := r.handleLogKey(runeKey('n'))
	r2 := ret2.(Model)
	assert.Equal(t, 0, r2.logView.cursor)
	assert.Equal(t, 25, r2.logView.visualCurCol)

	// Next wraps around to col 0.
	ret3, _ := r2.handleLogKey(runeKey('n'))
	r3 := ret3.(Model)
	assert.Equal(t, 0, r3.logView.cursor)
	assert.Equal(t, 0, r3.logView.visualCurCol)
}

func TestLogKeyGGGoesToTop(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:  []string{"a", "b", "c", "d", "e"},
			cursor: 4,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	// First g
	ret, _ := m.handleLogKey(runeKey('g'))
	result := ret.(Model)
	assert.True(t, result.pendingG)

	// Second g
	ret2, _ := result.handleLogKey(runeKey('g'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.logView.cursor)
	assert.False(t, result2.pendingG)
	assert.False(t, result2.logView.follow)
}

func TestLogKeyGGoesToBottom(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:  []string{"a", "b", "c", "d", "e"},
			cursor: 0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 4, result.logView.cursor)
	assert.True(t, result.logView.follow)
}

func TestLogKeyGWithDigitJumpsToLine(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:     []string{"a", "b", "c", "d", "e"},
			cursor:    0,
			lineInput: "3",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 2, result.logView.cursor) // 3 - 1 = 2 (0-indexed)
}

func TestLogKeyCtrlDHalfPageDown(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:  make([]string, 100),
			cursor: 0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	result := ret.(Model)
	assert.Greater(t, result.logView.cursor, 0)
}

func TestLogKeyCtrlUHalfPageUp(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:  make([]string, 100),
			cursor: 50,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Less(t, result.logView.cursor, 50)
}

func TestLogKeyCtrlFFullPageDown(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:  make([]string, 100),
			cursor: 0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	result := ret.(Model)
	assert.Greater(t, result.logView.cursor, 0)
}

func TestLogKeyCtrlBFullPageUp(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:  make([]string, 100),
			cursor: 50,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	result := ret.(Model)
	assert.Less(t, result.logView.cursor, 50)
}

func TestLogKeyDigitBuffering(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:     []string{"a"},
			lineInput: "",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('5'))
	result := ret.(Model)
	assert.Equal(t, "5", result.logView.lineInput)
}

func TestLogKeyZeroMovesToStartOfLine(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"hello world"},
			cursor:       0,
			visualCurCol: 5,
			lineInput:    "",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('0'))
	result := ret.(Model)
	assert.Equal(t, 0, result.logView.visualCurCol)
}

func TestLogKeyZeroWithDigitsPending(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:     []string{"hello"},
			lineInput: "1",
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('0'))
	result := ret.(Model)
	assert.Equal(t, "10", result.logView.lineInput)
}

func TestLogKeyHMovesLeft(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"hello"},
			visualCurCol: 3,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('h'))
	result := ret.(Model)
	assert.Equal(t, 2, result.logView.visualCurCol)
}

func TestLogKeyLMovesRight(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"hello"},
			visualCurCol: 3,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('l'))
	result := ret.(Model)
	assert.Equal(t, 4, result.logView.visualCurCol)
}

func TestLogKeyDollarMovesToEndOfLine(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"hello world"},
			cursor:       0,
			visualCurCol: 0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('$'))
	result := ret.(Model)
	assert.Equal(t, len("hello world")-1, result.logView.visualCurCol)
}

func TestLogKeyCaretMovesToFirstNonWhitespace(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"   hello"},
			cursor:       0,
			visualCurCol: 0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('^'))
	result := ret.(Model)
	assert.Equal(t, 3, result.logView.visualCurCol) // first non-space
}

func TestLogKeyVEntersCharVisualMode(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:  []string{"a", "b", "c"},
			cursor: 1,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('v'))
	result := ret.(Model)
	assert.True(t, result.logView.visualMode)
	assert.Equal(t, rune('v'), result.logView.visualType)
	assert.Equal(t, 1, result.logView.visualStart)
}

func TestLogKeyUpperVEntersLineVisualMode(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:  []string{"a", "b", "c"},
			cursor: 1,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(runeKey('V'))
	result := ret.(Model)
	assert.True(t, result.logView.visualMode)
	assert.Equal(t, rune('V'), result.logView.visualType)
}

func TestLogKeyCtrlVEntersBlockVisualMode(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:  []string{"a", "b", "c"},
			cursor: 1,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogKey(tea.KeyMsg{Type: tea.KeyCtrlV})
	result := ret.(Model)
	assert.True(t, result.logView.visualMode)
	assert.Equal(t, rune('B'), result.logView.visualType)
}

// --- handleLogVisualKey ---

func TestLogVisualKeyEscExits(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:      []string{"a", "b"},
			visualMode: true,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogVisualKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.logView.visualMode)
}

func TestLogVisualKeyQExits(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:      []string{"a", "b"},
			visualMode: true,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('q'))
	result := ret.(Model)
	assert.False(t, result.logView.visualMode)
}

func TestLogVisualKeyVToggle(t *testing.T) {
	t.Run("v in char mode cancels", func(t *testing.T) {
		m := Model{
			mode: modeLogs,
			logView: logViewState{
				lines:      []string{"a"},
				visualMode: true,
				visualType: 'v',
			},
			tabs:   []TabState{{}},
			width:  80,
			height: 40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('v'))
		result := ret.(Model)
		assert.False(t, result.logView.visualMode)
	})

	t.Run("v in line mode switches to char", func(t *testing.T) {
		m := Model{
			mode: modeLogs,
			logView: logViewState{
				lines:      []string{"a"},
				visualMode: true,
				visualType: 'V',
			},
			tabs:   []TabState{{}},
			width:  80,
			height: 40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('v'))
		result := ret.(Model)
		assert.True(t, result.logView.visualMode)
		assert.Equal(t, rune('v'), result.logView.visualType)
	})
}

func TestLogVisualKeyVVToggle(t *testing.T) {
	t.Run("V in line mode cancels", func(t *testing.T) {
		m := Model{
			mode: modeLogs,
			logView: logViewState{
				lines:      []string{"a"},
				visualMode: true,
				visualType: 'V',
			},
			tabs:   []TabState{{}},
			width:  80,
			height: 40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('V'))
		result := ret.(Model)
		assert.False(t, result.logView.visualMode)
	})

	t.Run("V in char mode switches to line", func(t *testing.T) {
		m := Model{
			mode: modeLogs,
			logView: logViewState{
				lines:      []string{"a"},
				visualMode: true,
				visualType: 'v',
			},
			tabs:   []TabState{{}},
			width:  80,
			height: 40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('V'))
		result := ret.(Model)
		assert.True(t, result.logView.visualMode)
		assert.Equal(t, rune('V'), result.logView.visualType)
	})
}

func TestLogVisualKeyCtrlVToggle(t *testing.T) {
	t.Run("ctrl+v in block mode cancels", func(t *testing.T) {
		m := Model{
			mode: modeLogs,
			logView: logViewState{
				lines:      []string{"a"},
				visualMode: true,
				visualType: 'B',
			},
			tabs:   []TabState{{}},
			width:  80,
			height: 40,
		}
		ret, _ := m.handleLogVisualKey(tea.KeyMsg{Type: tea.KeyCtrlV})
		result := ret.(Model)
		assert.False(t, result.logView.visualMode)
	})

	t.Run("ctrl+v in char mode switches to block", func(t *testing.T) {
		m := Model{
			mode: modeLogs,
			logView: logViewState{
				lines:      []string{"a"},
				visualMode: true,
				visualType: 'v',
			},
			tabs:   []TabState{{}},
			width:  80,
			height: 40,
		}
		ret, _ := m.handleLogVisualKey(tea.KeyMsg{Type: tea.KeyCtrlV})
		result := ret.(Model)
		assert.True(t, result.logView.visualMode)
		assert.Equal(t, rune('B'), result.logView.visualType)
	})
}

func TestLogVisualKeyJKNavigation(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:      []string{"a", "b", "c"},
			visualMode: true,
			cursor:     0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}

	ret, _ := m.handleLogVisualKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.logView.cursor)

	ret2, _ := result.handleLogVisualKey(runeKey('k'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.logView.cursor)
}

func TestLogVisualKeyHLNavigation(t *testing.T) {
	t.Run("h moves left in char mode", func(t *testing.T) {
		m := Model{
			mode: modeLogs,
			logView: logViewState{
				lines:        []string{"hello"},
				visualMode:   true,
				visualType:   'v',
				visualCurCol: 3,
			},
			tabs:   []TabState{{}},
			width:  80,
			height: 40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('h'))
		result := ret.(Model)
		assert.Equal(t, 2, result.logView.visualCurCol)
	})

	t.Run("l moves right in char mode", func(t *testing.T) {
		m := Model{
			mode: modeLogs,
			logView: logViewState{
				lines:        []string{"hello"},
				visualMode:   true,
				visualType:   'v',
				visualCurCol: 3,
			},
			tabs:   []TabState{{}},
			width:  80,
			height: 40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('l'))
		result := ret.(Model)
		assert.Equal(t, 4, result.logView.visualCurCol)
	})

	t.Run("h moves left in block mode", func(t *testing.T) {
		m := Model{
			mode: modeLogs,
			logView: logViewState{
				lines:        []string{"hello"},
				visualMode:   true,
				visualType:   'B',
				visualCurCol: 3,
			},
			tabs:   []TabState{{}},
			width:  80,
			height: 40,
		}
		ret, _ := m.handleLogVisualKey(runeKey('h'))
		result := ret.(Model)
		assert.Equal(t, 2, result.logView.visualCurCol)
	})
}

func TestLogVisualKeyZeroMovesToStart(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"hello"},
			visualMode:   true,
			visualCurCol: 3,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('0'))
	result := ret.(Model)
	assert.Equal(t, 0, result.logView.visualCurCol)
}

func TestLogVisualKeyGScrollsToEnd(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:      []string{"a", "b", "c", "d", "e"},
			visualMode: true,
			visualType: 'V',
			cursor:     0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 4, result.logView.cursor) // last line index
}

func TestLogVisualKeyGgScrollsToTop(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:      []string{"a", "b", "c", "d"},
			visualMode: true,
			visualType: 'V',
			cursor:     3,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('g'))
	result := ret.(Model)
	assert.True(t, result.pendingG)

	ret2, _ := result.handleLogVisualKey(runeKey('g'))
	result2 := ret2.(Model)
	assert.False(t, result2.pendingG)
	assert.Equal(t, 0, result2.logView.cursor)
}

func TestLogVisualKeyCtrlDU(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line"
	}

	t.Run("ctrl+d moves down half page", func(t *testing.T) {
		m := Model{
			mode: modeLogs,
			logView: logViewState{
				lines:      lines,
				visualMode: true,
				visualType: 'V',
				cursor:     0,
			},
			tabs:   []TabState{{}},
			width:  80,
			height: 40,
		}
		ret, _ := m.handleLogVisualKey(tea.KeyMsg{Type: tea.KeyCtrlD})
		result := ret.(Model)
		assert.Greater(t, result.logView.cursor, 0)
	})

	t.Run("ctrl+u moves up half page", func(t *testing.T) {
		m := Model{
			mode: modeLogs,
			logView: logViewState{
				lines:      lines,
				visualMode: true,
				visualType: 'V',
				cursor:     50,
			},
			tabs:   []TabState{{}},
			width:  80,
			height: 40,
		}
		ret, _ := m.handleLogVisualKey(tea.KeyMsg{Type: tea.KeyCtrlU})
		result := ret.(Model)
		assert.Less(t, result.logView.cursor, 50)
	})
}

func TestLogVisualKeyDollarMovesToEnd(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"hello world"},
			visualMode:   true,
			visualType:   'v',
			visualCurCol: 0,
			cursor:       0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('$'))
	result := ret.(Model)
	assert.Equal(t, 10, result.logView.visualCurCol) // len("hello world") - 1 = 10
}

func TestLogVisualKeyCaretMovesToFirstNonWhitespace(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"   hello"},
			visualMode:   true,
			visualType:   'v',
			visualCurCol: 0,
			cursor:       0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('^'))
	result := ret.(Model)
	assert.Equal(t, 3, result.logView.visualCurCol) // first non-ws at index 3
}

func TestLogVisualKeyWordMotions(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"hello world foo"},
			visualMode:   true,
			visualType:   'v',
			visualCurCol: 0,
			cursor:       0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}

	t.Run("w moves to next word", func(t *testing.T) {
		ret, _ := m.handleLogVisualKey(runeKey('w'))
		result := ret.(Model)
		assert.Greater(t, result.logView.visualCurCol, 0)
	})

	t.Run("e moves to end of word", func(t *testing.T) {
		ret, _ := m.handleLogVisualKey(runeKey('e'))
		result := ret.(Model)
		assert.Greater(t, result.logView.visualCurCol, 0)
	})

	t.Run("b moves to prev word start", func(t *testing.T) {
		m2 := m
		m2.logView.visualCurCol = 6
		ret, _ := m2.handleLogVisualKey(runeKey('b'))
		result := ret.(Model)
		assert.Less(t, result.logView.visualCurCol, 6)
	})
}

func TestLogVisualKeyWORDMotions(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"hello world foo"},
			visualMode:   true,
			visualType:   'v',
			visualCurCol: 0,
			cursor:       0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}

	t.Run("W moves to next WORD", func(t *testing.T) {
		ret, _ := m.handleLogVisualKey(runeKey('W'))
		result := ret.(Model)
		assert.Greater(t, result.logView.visualCurCol, 0)
	})

	t.Run("E moves to end of WORD", func(t *testing.T) {
		ret, _ := m.handleLogVisualKey(runeKey('E'))
		result := ret.(Model)
		assert.Greater(t, result.logView.visualCurCol, 0)
	})

	t.Run("B moves to prev WORD start", func(t *testing.T) {
		m2 := m
		m2.logView.visualCurCol = 6
		ret, _ := m2.handleLogVisualKey(runeKey('B'))
		result := ret.(Model)
		assert.Less(t, result.logView.visualCurCol, 6)
	})
}

func TestLogVisualKeyQExitsVisualMode(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:      []string{"a"},
			visualMode: true,
			visualType: 'V',
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogVisualKey(runeKey('q'))
	result := ret.(Model)
	assert.False(t, result.logView.visualMode)
}

func TestLogVisualKeyYankLineMode(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:       []string{"line1", "line2", "line3"},
			visualMode:  true,
			visualType:  'V',
			cursor:      2,
			visualStart: 1,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, cmd := m.handleLogVisualKey(runeKey('y'))
	result := ret.(Model)
	assert.False(t, result.logView.visualMode, "visual mode should exit after yank")
	assert.Contains(t, result.statusMessage, "Copied 2 lines")
	assert.NotNil(t, cmd)
}

func TestLogVisualKeyYankCharMode(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"hello world"},
			visualMode:   true,
			visualType:   'v',
			cursor:       0,
			visualStart:  0,
			visualCol:    0,
			visualCurCol: 4,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, cmd := m.handleLogVisualKey(runeKey('y'))
	result := ret.(Model)
	assert.False(t, result.logView.visualMode)
	// Single-line char-mode selection just says "Copied" — a character
	// count adds no useful info, and "Copied 1 lines" was misleading
	// after viw/vaw landed.
	assert.Equal(t, "Copied", result.statusMessage)
	assert.NotNil(t, cmd)
}

func TestLogVisualKeyYankBlockMode(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"abc", "def", "ghi"},
			visualMode:   true,
			visualType:   'B',
			cursor:       2,
			visualStart:  0,
			visualCol:    0,
			visualCurCol: 1,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, cmd := m.handleLogVisualKey(runeKey('y'))
	result := ret.(Model)
	assert.False(t, result.logView.visualMode)
	assert.Contains(t, result.statusMessage, "Copied 3 lines")
	assert.NotNil(t, cmd)
}

// --- buildLogYankText: clipboard mirrors what the user sees ---
//
// The renderer strips timestamps and pod prefixes per the user's toggles
// (see ui.applyLineRewrites); the yank handler must apply the same
// transformations so the clipboard mirrors the on-screen display.

func TestBuildLogYankTextLineModeStripsTimestampWhenOff(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines: []string{
				"2024-01-15T10:30:00.000000000Z first line",
				"2024-01-15T10:30:01.000000000Z second line",
			},
			visualMode:   true,
			visualType:   'V',
			cursor:       1,
			visualStart:  0,
			timestamps:   false,
			hidePrefixes: false,
		},
	}
	clip, count := m.buildLogYankText()
	assert.Equal(t, 2, count)
	assert.Equal(t, "first line\nsecond line", clip,
		"timestamps must be stripped from clipboard when logTimestamps is off")
}

func TestBuildLogYankTextLineModeStripsPrefixWhenHidden(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines: []string{
				"[pod/api/main] first line",
				"[pod/api/main] second line",
			},
			visualMode:   true,
			visualType:   'V',
			cursor:       1,
			visualStart:  0,
			timestamps:   true,
			hidePrefixes: true,
		},
	}
	clip, count := m.buildLogYankText()
	assert.Equal(t, 2, count)
	assert.Equal(t, "first line\nsecond line", clip,
		"pod prefixes must be stripped from clipboard when logHidePrefixes is on")
}

func TestBuildLogYankTextLineModeStripsBothWhenOff(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines: []string{
				"[pod/api/main] 2024-01-15T10:30:00.000000000Z first line",
				"[pod/api/main] 2024-01-15T10:30:01.000000000Z second line",
			},
			visualMode:   true,
			visualType:   'V',
			cursor:       1,
			visualStart:  0,
			timestamps:   false,
			hidePrefixes: true,
		},
	}
	clip, count := m.buildLogYankText()
	assert.Equal(t, 2, count)
	assert.Equal(t, "first line\nsecond line", clip,
		"both timestamps and prefixes must be stripped when both toggles are off")
}

func TestBuildLogYankTextLineModeKeepsBothWhenOn(t *testing.T) {
	raw := []string{
		"[pod/api/main] 2024-01-15T10:30:00.000000000Z first line",
		"[pod/api/main] 2024-01-15T10:30:01.000000000Z second line",
	}
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        raw,
			visualMode:   true,
			visualType:   'V',
			cursor:       1,
			visualStart:  0,
			timestamps:   true,
			hidePrefixes: false,
		},
	}
	clip, count := m.buildLogYankText()
	assert.Equal(t, 2, count)
	assert.Equal(t, raw[0]+"\n"+raw[1], clip,
		"raw lines must be preserved when both toggles are on")
}

func TestBuildLogYankTextCharModeOperatesOnDisplayedForm(t *testing.T) {
	// Char-mode column positions are in display-line space (the line
	// after stripping). Selecting cols 0-4 of "hello world" should yield
	// "hello" even though the raw line begins with "[pod/x/y] 2024-...".
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines: []string{
				"[pod/x/y] 2024-01-15T10:30:00.000000000Z hello world",
			},
			visualMode:   true,
			visualType:   'v',
			cursor:       0,
			visualStart:  0,
			visualCol:    0,
			visualCurCol: 4,
			timestamps:   false,
			hidePrefixes: true,
		},
	}
	clip, count := m.buildLogYankText()
	assert.Equal(t, 1, count)
	assert.Equal(t, "hello", clip,
		"char-mode column slice must operate on the displayed form")
}

func TestBuildLogYankTextBlockModeOperatesOnDisplayedForm(t *testing.T) {
	// Block-mode column positions are in display-line space. Three lines
	// "abc" / "def" / "ghi" with timestamps & prefix stripped, slice cols 0-1.
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines: []string{
				"[pod/x/y] 2024-01-15T10:30:00.000000000Z abc",
				"[pod/x/y] 2024-01-15T10:30:01.000000000Z def",
				"[pod/x/y] 2024-01-15T10:30:02.000000000Z ghi",
			},
			visualMode:   true,
			visualType:   'B',
			cursor:       2,
			visualStart:  0,
			visualCol:    0,
			visualCurCol: 1,
			timestamps:   false,
			hidePrefixes: true,
		},
	}
	clip, count := m.buildLogYankText()
	assert.Equal(t, 3, count)
	assert.Equal(t, "ab\nde\ngh", clip,
		"block-mode column slice must operate on the displayed form")
}

// --- handleLogSearchKey ---

func TestLogSearchKeyEnterCommitsSearch(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"error: test", "info: ok"},
			searchActive: true,
			searchInput:  TextInput{Value: "error"},
			cursor:       0,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogSearchKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.logView.searchActive)
	assert.Equal(t, "error", result.logView.searchQuery)
}

func TestLogSearchKeyEscCancels(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"a"},
			searchActive: true,
			searchInput:  TextInput{Value: "test"},
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogSearchKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.logView.searchActive)
	assert.Empty(t, result.logView.searchInput.Value)
}

func TestLogSearchKeyTyping(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"a"},
			searchActive: true,
			searchInput:  TextInput{Value: ""},
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogSearchKey(runeKey('e'))
	result := ret.(Model)
	assert.Equal(t, "e", result.logView.searchInput.Value)
}

func TestLogSearchKeyBackspace(t *testing.T) {
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{"a"},
			searchActive: true,
			searchInput:  TextInput{Value: "abc", Cursor: 3},
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
	ret, _ := m.handleLogSearchKey(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "ab", result.logView.searchInput.Value)
}
