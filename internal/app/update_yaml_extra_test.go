package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// yamlContent helper: 50 lines to ensure scrolling works with default height 40.
func makeYAMLContent(n int) string {
	var lines strings.Builder
	for i := range n {
		if i > 0 {
			lines.WriteString("\n")
		}
		lines.WriteString("  key: value")
	}
	return lines.String()
}

func baseYAMLModel() Model {
	return Model{
		mode:        modeYAML,
		yamlContent: makeYAMLContent(50),
		tabs:        []TabState{{}},
		width:       80,
		height:      40,
	}
}

// --- handleYAMLKey: Normal mode navigation ---

func TestYAMLKeyEscExitsToExplorer(t *testing.T) {
	m := baseYAMLModel()
	m.yamlScroll = 5
	m.yamlCursor = 3
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
	assert.Equal(t, 0, result.yamlScroll)
	assert.Equal(t, 0, result.yamlCursor)
}

func TestYAMLKeyEscClearsSearchFirst(t *testing.T) {
	m := baseYAMLModel()
	m.yamlSearchText = TextInput{Value: "hello", Cursor: 5}
	m.yamlMatchLines = []int{1, 2}
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, modeYAML, result.mode, "should stay in YAML mode when clearing search")
	assert.Equal(t, "", result.yamlSearchText.Value)
	assert.Nil(t, result.yamlMatchLines)
}

func TestYAMLKeyHelpOpens(t *testing.T) {
	m := baseYAMLModel()
	ret, _ := m.handleYAMLKey(runeKey('?'))
	result := ret.(Model)
	assert.Equal(t, modeHelp, result.mode)
	assert.Equal(t, modeYAML, result.helpPreviousMode)
	assert.Equal(t, "YAML View", result.helpContextMode)
}

func TestYAMLKeyJKNavigation(t *testing.T) {
	t.Run("j moves cursor down", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlCursor = 0
		ret, _ := m.handleYAMLKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.yamlCursor)
	})

	t.Run("k moves cursor up", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlCursor = 5
		ret, _ := m.handleYAMLKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 4, result.yamlCursor)
	})

	t.Run("k at zero stays", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlCursor = 0
		ret, _ := m.handleYAMLKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 0, result.yamlCursor)
	})
}

func TestYAMLKeyGgScrollsToTop(t *testing.T) {
	m := baseYAMLModel()
	m.yamlCursor = 20
	m.yamlScroll = 10

	ret, _ := m.handleYAMLKey(runeKey('g'))
	result := ret.(Model)
	assert.True(t, result.pendingG)

	ret2, _ := result.handleYAMLKey(runeKey('g'))
	result2 := ret2.(Model)
	assert.False(t, result2.pendingG)
	assert.Equal(t, 0, result2.yamlCursor)
	assert.Equal(t, 0, result2.yamlScroll)
}

func TestYAMLKeyGScrollsToBottom(t *testing.T) {
	m := baseYAMLModel()
	ret, _ := m.handleYAMLKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 49, result.yamlCursor) // 50 lines, 0-indexed
}

func TestYAMLKeyCtrlDU(t *testing.T) {
	t.Run("ctrl+d moves down half page", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlCursor = 0
		ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlD})
		result := ret.(Model)
		assert.Equal(t, 20, result.yamlCursor) // height 40 / 2 = 20
	})

	t.Run("ctrl+u moves up half page", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlCursor = 30
		ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlU})
		result := ret.(Model)
		assert.Equal(t, 10, result.yamlCursor) // 30 - 20 = 10
	})

	t.Run("ctrl+u clamps at zero", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlCursor = 5
		ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlU})
		result := ret.(Model)
		assert.Equal(t, 0, result.yamlCursor)
	})
}

func TestYAMLKeyCtrlFB(t *testing.T) {
	t.Run("ctrl+f moves down full page", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlCursor = 0
		ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlF})
		result := ret.(Model)
		assert.Equal(t, 40, result.yamlCursor) // height = 40
	})

	t.Run("ctrl+b moves up full page", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlCursor = 45
		ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlB})
		result := ret.(Model)
		assert.Equal(t, 5, result.yamlCursor) // 45 - 40 = 5
	})
}

func TestYAMLKeySlashEntersSearch(t *testing.T) {
	m := baseYAMLModel()
	ret, _ := m.handleYAMLKey(runeKey('/'))
	result := ret.(Model)
	assert.True(t, result.yamlSearchMode)
	assert.Equal(t, "", result.yamlSearchText.Value)
}

// Regression: typing into the YAML search input now updates
// yamlMatchLines on every keystroke so the highlight overlay paints
// in real time. Previously yamlMatchLines stayed nil until Enter, so
// the user had no feedback on whether their query matched anything
// while typing.
func TestYAMLSearchTypingUpdatesMatchesLive(t *testing.T) {
	m := baseYAMLModel()
	// Content with a known matching pattern.
	m.yamlContent = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: nginx\n  namespace: default\nspec:\n  containers:\n  - name: nginx\n    image: nginx:1.27\n"
	m.yamlSearchMode = true

	// Type "n" → nginx + namespace + name lines should be matched.
	result, _ := m.handleYAMLKey(runeKey('n'))
	rm := result.(Model)
	assert.Equal(t, "n", rm.yamlSearchText.Value)
	assert.NotEmpty(t, rm.yamlMatchLines,
		"yamlMatchLines must populate on first keystroke so highlights paint live")

	// Type "g" → "n" + "g" filters down to nginx-bearing lines.
	result, _ = rm.handleYAMLKey(runeKey('g'))
	rm = result.(Model)
	assert.Equal(t, "ng", rm.yamlSearchText.Value)
	require.NotEmpty(t, rm.yamlMatchLines)

	// Backspace must keep the match set in sync, not leave stale state.
	result, _ = rm.handleYAMLKey(specialKey(tea.KeyBackspace))
	rm = result.(Model)
	assert.Equal(t, "n", rm.yamlSearchText.Value)
	assert.NotEmpty(t, rm.yamlMatchLines, "matches must recompute after backspace")
}

func TestYAMLKeyHLMoveCursorColumn(t *testing.T) {
	t.Run("h moves left", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualCurCol = 5
		ret, _ := m.handleYAMLKey(runeKey('h'))
		result := ret.(Model)
		assert.Equal(t, 4, result.yamlVisualCurCol)
	})

	t.Run("h clamps at fold prefix len", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualCurCol = yamlFoldPrefixLen
		ret, _ := m.handleYAMLKey(runeKey('h'))
		result := ret.(Model)
		assert.Equal(t, yamlFoldPrefixLen, result.yamlVisualCurCol)
	})

	t.Run("l moves right", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualCurCol = 5
		ret, _ := m.handleYAMLKey(runeKey('l'))
		result := ret.(Model)
		assert.Equal(t, 6, result.yamlVisualCurCol)
	})
}

func TestYAMLKeyZero(t *testing.T) {
	m := baseYAMLModel()
	m.yamlVisualCurCol = 15
	ret, _ := m.handleYAMLKey(runeKey('0'))
	result := ret.(Model)
	assert.Equal(t, yamlFoldPrefixLen, result.yamlVisualCurCol)
}

func TestYAMLKeyVEntersVisualMode(t *testing.T) {
	t.Run("V enters line visual", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlCursor = 3
		ret, _ := m.handleYAMLKey(runeKey('V'))
		result := ret.(Model)
		assert.True(t, result.yamlVisualMode)
		assert.Equal(t, rune('V'), result.yamlVisualType)
		assert.Equal(t, 3, result.yamlVisualStart)
	})

	t.Run("v enters char visual", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlCursor = 2
		ret, _ := m.handleYAMLKey(runeKey('v'))
		result := ret.(Model)
		assert.True(t, result.yamlVisualMode)
		assert.Equal(t, rune('v'), result.yamlVisualType)
		assert.Equal(t, 2, result.yamlVisualStart)
	})

	t.Run("ctrl+v enters block visual", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlCursor = 1
		ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlV})
		result := ret.(Model)
		assert.True(t, result.yamlVisualMode)
		assert.Equal(t, rune('B'), result.yamlVisualType)
		assert.Equal(t, 1, result.yamlVisualStart)
	})
}

func TestYAMLKeyNSearchNext(t *testing.T) {
	m := baseYAMLModel()
	m.yamlMatchLines = []int{5, 10, 20}
	m.yamlMatchIdx = 0
	ret, _ := m.handleYAMLKey(runeKey('n'))
	result := ret.(Model)
	assert.Equal(t, 1, result.yamlMatchIdx)
}

func TestYAMLKeyNSearchPrev(t *testing.T) {
	m := baseYAMLModel()
	m.yamlMatchLines = []int{5, 10, 20}
	m.yamlMatchIdx = 0
	ret, _ := m.handleYAMLKey(runeKey('N'))
	result := ret.(Model)
	assert.Equal(t, 2, result.yamlMatchIdx) // wraps to end
}

// --- handleYAMLKey: Search mode ---

func TestYAMLSearchModeEscCancels(t *testing.T) {
	m := baseYAMLModel()
	m.yamlSearchMode = true
	m.yamlSearchText = TextInput{Value: "test", Cursor: 4}
	m.yamlMatchLines = []int{1}
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.yamlSearchMode)
	assert.Equal(t, "", result.yamlSearchText.Value)
	assert.Nil(t, result.yamlMatchLines)
}

func TestYAMLSearchModeTyping(t *testing.T) {
	m := baseYAMLModel()
	m.yamlSearchMode = true
	m.yamlSearchText = TextInput{Value: "ab", Cursor: 2}
	ret, _ := m.handleYAMLKey(runeKey('c'))
	result := ret.(Model)
	assert.Equal(t, "abc", result.yamlSearchText.Value)
}

func TestYAMLSearchModeBackspace(t *testing.T) {
	m := baseYAMLModel()
	m.yamlSearchMode = true
	m.yamlSearchText = TextInput{Value: "abc", Cursor: 3}
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "ab", result.yamlSearchText.Value)
}

func TestYAMLSearchModeEnterActivatesSearch(t *testing.T) {
	m := baseYAMLModel()
	m.yamlSearchMode = true
	m.yamlSearchText = TextInput{Value: "key", Cursor: 3}
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.yamlSearchMode)
	// "key" should match in the yaml content "  key: value"
	assert.Greater(t, len(result.yamlMatchLines), 0)
}

func TestYAMLSearchModeCtrlW(t *testing.T) {
	m := baseYAMLModel()
	m.yamlSearchMode = true
	m.yamlSearchText = TextInput{Value: "hello world", Cursor: 11}
	ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	result := ret.(Model)
	assert.Equal(t, "hello ", result.yamlSearchText.Value)
}

// --- handleYAMLKey: Visual mode ---

func TestYAMLVisualModeEscCancels(t *testing.T) {
	m := baseYAMLModel()
	m.yamlVisualMode = true
	m.yamlVisualType = 'V'
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.yamlVisualMode)
}

func TestYAMLVisualModeVToggle(t *testing.T) {
	t.Run("v cancels char mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualMode = true
		m.yamlVisualType = 'v'
		ret, _ := m.handleYAMLKey(runeKey('v'))
		result := ret.(Model)
		assert.False(t, result.yamlVisualMode)
	})

	t.Run("v switches from line to char", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualMode = true
		m.yamlVisualType = 'V'
		ret, _ := m.handleYAMLKey(runeKey('v'))
		result := ret.(Model)
		assert.True(t, result.yamlVisualMode)
		assert.Equal(t, rune('v'), result.yamlVisualType)
	})
}

func TestYAMLVisualModeVVToggle(t *testing.T) {
	t.Run("V cancels line mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualMode = true
		m.yamlVisualType = 'V'
		ret, _ := m.handleYAMLKey(runeKey('V'))
		result := ret.(Model)
		assert.False(t, result.yamlVisualMode)
	})

	t.Run("V switches from char to line", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualMode = true
		m.yamlVisualType = 'v'
		ret, _ := m.handleYAMLKey(runeKey('V'))
		result := ret.(Model)
		assert.True(t, result.yamlVisualMode)
		assert.Equal(t, rune('V'), result.yamlVisualType)
	})
}

func TestYAMLVisualModeCtrlVToggle(t *testing.T) {
	t.Run("ctrl+v cancels block mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualMode = true
		m.yamlVisualType = 'B'
		ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlV})
		result := ret.(Model)
		assert.False(t, result.yamlVisualMode)
	})

	t.Run("ctrl+v switches from char to block", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualMode = true
		m.yamlVisualType = 'v'
		ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlV})
		result := ret.(Model)
		assert.True(t, result.yamlVisualMode)
		assert.Equal(t, rune('B'), result.yamlVisualType)
	})
}

func TestYAMLVisualModeJKNav(t *testing.T) {
	t.Run("j in visual mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualMode = true
		m.yamlVisualType = 'V'
		m.yamlCursor = 3
		ret, _ := m.handleYAMLKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 4, result.yamlCursor)
		assert.True(t, result.yamlVisualMode)
	})

	t.Run("k in visual mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualMode = true
		m.yamlVisualType = 'V'
		m.yamlCursor = 5
		ret, _ := m.handleYAMLKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 4, result.yamlCursor)
	})
}

func TestYAMLVisualModeHL(t *testing.T) {
	t.Run("h in visual mode moves column left", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualMode = true
		m.yamlVisualType = 'v'
		m.yamlVisualCurCol = 5
		ret, _ := m.handleYAMLKey(runeKey('h'))
		result := ret.(Model)
		assert.Equal(t, 4, result.yamlVisualCurCol)
	})

	t.Run("l in visual mode moves column right", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualMode = true
		m.yamlVisualType = 'v'
		m.yamlVisualCurCol = 5
		ret, _ := m.handleYAMLKey(runeKey('l'))
		result := ret.(Model)
		assert.Equal(t, 6, result.yamlVisualCurCol)
	})
}

func TestYAMLVisualModeGg(t *testing.T) {
	m := baseYAMLModel()
	m.yamlVisualMode = true
	m.yamlVisualType = 'V'
	m.yamlCursor = 20

	ret, _ := m.handleYAMLKey(runeKey('g'))
	result := ret.(Model)
	assert.True(t, result.pendingG)

	ret2, _ := result.handleYAMLKey(runeKey('g'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.yamlCursor)
	assert.Equal(t, 0, result2.yamlScroll)
}

func TestYAMLVisualModeG(t *testing.T) {
	m := baseYAMLModel()
	m.yamlVisualMode = true
	m.yamlVisualType = 'V'
	m.yamlCursor = 0
	ret, _ := m.handleYAMLKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 49, result.yamlCursor)
}

func TestYAMLVisualModeZero(t *testing.T) {
	m := baseYAMLModel()
	m.yamlVisualMode = true
	m.yamlVisualType = 'v'
	m.yamlVisualCurCol = 10
	ret, _ := m.handleYAMLKey(runeKey('0'))
	result := ret.(Model)
	assert.Equal(t, yamlFoldPrefixLen, result.yamlVisualCurCol)
}

func TestYAMLVisualModeCtrlDU(t *testing.T) {
	t.Run("ctrl+d in visual mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualMode = true
		m.yamlVisualType = 'V'
		m.yamlCursor = 0
		ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlD})
		result := ret.(Model)
		assert.Equal(t, 20, result.yamlCursor)
	})

	t.Run("ctrl+u in visual mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlVisualMode = true
		m.yamlVisualType = 'V'
		m.yamlCursor = 30
		ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlU})
		result := ret.(Model)
		assert.Equal(t, 10, result.yamlCursor)
	})
}

// --- handleYAMLKey: Fold operations ---

func TestYAMLKeyTabTogglesFold(t *testing.T) {
	// Without sections, tab is a no-op but should not crash.
	m := baseYAMLModel()
	m.yamlCursor = 0
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyTab))
	result := ret.(Model)
	assert.Equal(t, modeYAML, result.mode)
}

func TestYAMLKeyZTogglesFold(t *testing.T) {
	m := baseYAMLModel()
	m.yamlCursor = 0
	ret, _ := m.handleYAMLKey(runeKey('z'))
	result := ret.(Model)
	assert.Equal(t, modeYAML, result.mode)
}

func TestYAMLKeyZToggleAllFolds(t *testing.T) {
	// With sections, Z toggles all folds.
	m := baseYAMLModel()
	m.yamlSections = []yamlSection{
		{key: "metadata", startLine: 0, endLine: 5},
		{key: "spec", startLine: 6, endLine: 10},
	}
	ret, _ := m.handleYAMLKey(runeKey('Z'))
	result := ret.(Model)
	// All multi-line sections should be collapsed.
	assert.True(t, result.yamlCollapsed["metadata"])
	assert.True(t, result.yamlCollapsed["spec"])

	// Toggle again should expand all.
	ret2, _ := result.handleYAMLKey(runeKey('Z'))
	result2 := ret2.(Model)
	assert.False(t, result2.yamlCollapsed["metadata"])
	assert.False(t, result2.yamlCollapsed["spec"])
}

func TestYAMLKeyCtrlC(t *testing.T) {
	m := baseYAMLModel()
	m.yamlScroll = 10
	m.yamlCursor = 5
	ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
	assert.Equal(t, 0, result.yamlScroll)
	assert.Equal(t, 0, result.yamlCursor)
}

func TestYAMLVisualModeCtrlCExits(t *testing.T) {
	m := baseYAMLModel()
	m.yamlVisualMode = true
	m.yamlVisualType = 'V'
	m.yamlCursor = 5
	ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := ret.(Model)
	assert.False(t, result.yamlVisualMode)
	assert.Equal(t, modeExplorer, result.mode)
}
