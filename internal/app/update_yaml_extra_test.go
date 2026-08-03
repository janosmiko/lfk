package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
		mode: modeYAML,
		yamlView: yamlViewState{
			content: makeYAMLContent(50),
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
}

// --- handleYAMLKey: Normal mode navigation ---

func TestYAMLKeyEscExitsToExplorer(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.scroll = 5
	m.yamlView.cursor = 3
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
	assert.Equal(t, 0, result.yamlView.scroll)
	assert.Equal(t, 0, result.yamlView.cursor)
}

func TestYAMLKeyEscClearsSearchFirst(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchText = TextInput{Value: "hello", Cursor: 5}
	m.yamlView.matchLines = []int{1, 2}
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, modeYAML, result.mode, "should stay in YAML mode when clearing search")
	assert.Equal(t, "", result.yamlView.searchText.Value)
	assert.Nil(t, result.yamlView.matchLines)
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
		m.yamlView.cursor = 0
		ret, _ := m.handleYAMLKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.yamlView.cursor)
	})

	t.Run("k moves cursor up", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.cursor = 5
		ret, _ := m.handleYAMLKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 4, result.yamlView.cursor)
	})

	t.Run("k at zero stays", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.cursor = 0
		ret, _ := m.handleYAMLKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 0, result.yamlView.cursor)
	})
}

func TestYAMLKeyGgScrollsToTop(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.cursor = 20
	m.yamlView.scroll = 10

	ret, _ := m.handleYAMLKey(runeKey('g'))
	result := ret.(Model)
	assert.True(t, result.pendingG)

	ret2, _ := result.handleYAMLKey(runeKey('g'))
	result2 := ret2.(Model)
	assert.False(t, result2.pendingG)
	assert.Equal(t, 0, result2.yamlView.cursor)
	assert.Equal(t, 0, result2.yamlView.scroll)
}

func TestYAMLKeyGScrollsToBottom(t *testing.T) {
	m := baseYAMLModel()
	ret, _ := m.handleYAMLKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 49, result.yamlView.cursor) // 50 lines, 0-indexed
}

func TestYAMLKeyCtrlDU(t *testing.T) {
	// Step is yamlViewportLines()/2 for ctrl+d/u: height 40 minus 5 rows of
	// overhead (title bar, yaml title, border*2, hint) gives a 35-line
	// viewport; half-page = 17.
	t.Run("ctrl+d moves down half page", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.cursor = 0
		ret, _ := m.handleYAMLKey(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
		result := ret.(Model)
		assert.Equal(t, m.yamlViewportLines()/2, result.yamlView.cursor)
		assert.Equal(t, 17, result.yamlView.cursor)
	})

	t.Run("ctrl+u moves up half page", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.cursor = 30
		ret, _ := m.handleYAMLKey(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
		result := ret.(Model)
		assert.Equal(t, 30-m.yamlViewportLines()/2, result.yamlView.cursor)
		assert.Equal(t, 13, result.yamlView.cursor)
	})

	t.Run("ctrl+u clamps at zero", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.cursor = 5
		ret, _ := m.handleYAMLKey(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
		result := ret.(Model)
		assert.Equal(t, 0, result.yamlView.cursor)
	})
}

func TestYAMLKeyCtrlFB(t *testing.T) {
	// Full-page step is yamlViewportLines() (35 with the base-model height of
	// 40 and a single tab), not raw m.height.
	t.Run("ctrl+f moves down full page", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.cursor = 0
		ret, _ := m.handleYAMLKey(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
		result := ret.(Model)
		assert.Equal(t, m.yamlViewportLines(), result.yamlView.cursor)
		assert.Equal(t, 35, result.yamlView.cursor)
	})

	t.Run("ctrl+b moves up full page", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.cursor = 45
		ret, _ := m.handleYAMLKey(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
		result := ret.(Model)
		assert.Equal(t, 45-m.yamlViewportLines(), result.yamlView.cursor)
		assert.Equal(t, 10, result.yamlView.cursor)
	})
}

func TestYAMLKeySlashEntersSearch(t *testing.T) {
	m := baseYAMLModel()
	ret, _ := m.handleYAMLKey(runeKey('/'))
	result := ret.(Model)
	assert.True(t, result.yamlView.searchMode)
	assert.Equal(t, "", result.yamlView.searchText.Value)
}

// Regression: typing into the YAML search input now updates
// yamlMatchLines on every keystroke so the highlight overlay paints
// in real time. Previously yamlMatchLines stayed nil until Enter, so
// the user had no feedback on whether their query matched anything
// while typing.
func TestYAMLSearchTypingUpdatesMatchesLive(t *testing.T) {
	m := baseYAMLModel()
	// Content with a known matching pattern.
	m.yamlView.content = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: nginx\n  namespace: default\nspec:\n  containers:\n  - name: nginx\n    image: nginx:1.27\n"
	m.yamlView.searchMode = true

	// Type "n" → nginx + namespace + name lines should be matched.
	result, _ := m.handleYAMLKey(runeKey('n'))
	rm := result.(Model)
	assert.Equal(t, "n", rm.yamlView.searchText.Value)
	assert.NotEmpty(t, rm.yamlView.matchLines,
		"yamlMatchLines must populate on first keystroke so highlights paint live")

	// Type "g" → "n" + "g" filters down to nginx-bearing lines.
	result, _ = rm.handleYAMLKey(runeKey('g'))
	rm = result.(Model)
	assert.Equal(t, "ng", rm.yamlView.searchText.Value)
	require.NotEmpty(t, rm.yamlView.matchLines)

	// Backspace must keep the match set in sync, not leave stale state.
	result, _ = rm.handleYAMLKey(specialKey(tea.KeyBackspace))
	rm = result.(Model)
	assert.Equal(t, "n", rm.yamlView.searchText.Value)
	assert.NotEmpty(t, rm.yamlView.matchLines, "matches must recompute after backspace")
}

func TestYAMLKeyHLMoveCursorColumn(t *testing.T) {
	t.Run("h moves left", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualCurCol = 5
		ret, _ := m.handleYAMLKey(runeKey('h'))
		result := ret.(Model)
		assert.Equal(t, 4, result.yamlView.visualCurCol)
	})

	t.Run("h clamps at fold prefix len", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualCurCol = yamlFoldPrefixLen
		ret, _ := m.handleYAMLKey(runeKey('h'))
		result := ret.(Model)
		assert.Equal(t, yamlFoldPrefixLen, result.yamlView.visualCurCol)
	})

	t.Run("l moves right", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualCurCol = 5
		ret, _ := m.handleYAMLKey(runeKey('l'))
		result := ret.(Model)
		assert.Equal(t, 6, result.yamlView.visualCurCol)
	})
}

func TestYAMLKeyZero(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualCurCol = 15
	ret, _ := m.handleYAMLKey(runeKey('0'))
	result := ret.(Model)
	assert.Equal(t, yamlFoldPrefixLen, result.yamlView.visualCurCol)
}

func TestYAMLKeyVEntersVisualMode(t *testing.T) {
	t.Run("V enters line visual", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.cursor = 3
		ret, _ := m.handleYAMLKey(runeKey('V'))
		result := ret.(Model)
		assert.True(t, result.yamlView.visualMode)
		assert.Equal(t, rune('V'), result.yamlView.visualType)
		assert.Equal(t, 3, result.yamlView.visualStart)
	})

	t.Run("v enters char visual", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.cursor = 2
		ret, _ := m.handleYAMLKey(runeKey('v'))
		result := ret.(Model)
		assert.True(t, result.yamlView.visualMode)
		assert.Equal(t, rune('v'), result.yamlView.visualType)
		assert.Equal(t, 2, result.yamlView.visualStart)
	})

	t.Run("ctrl+v enters block visual", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.cursor = 1
		ret, _ := m.handleYAMLKey(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
		result := ret.(Model)
		assert.True(t, result.yamlView.visualMode)
		assert.Equal(t, rune('B'), result.yamlView.visualType)
		assert.Equal(t, 1, result.yamlView.visualStart)
	})
}

func TestYAMLKeyNSearchNext(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.matchLines = []int{5, 10, 20}
	m.yamlView.matchIdx = 0
	ret, _ := m.handleYAMLKey(runeKey('n'))
	result := ret.(Model)
	assert.Equal(t, 1, result.yamlView.matchIdx)
}

func TestYAMLKeyNSearchPrev(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.matchLines = []int{5, 10, 20}
	m.yamlView.matchIdx = 0
	ret, _ := m.handleYAMLKey(runeKey('N'))
	result := ret.(Model)
	assert.Equal(t, 2, result.yamlView.matchIdx) // wraps to end
}

// --- handleYAMLKey: Search mode ---

func TestYAMLSearchModeEscCancels(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchMode = true
	m.yamlView.searchText = TextInput{Value: "test", Cursor: 4}
	m.yamlView.matchLines = []int{1}
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.yamlView.searchMode)
	assert.Equal(t, "", result.yamlView.searchText.Value)
	assert.Nil(t, result.yamlView.matchLines)
}

func TestYAMLSearchModeTyping(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchMode = true
	m.yamlView.searchText = TextInput{Value: "ab", Cursor: 2}
	ret, _ := m.handleYAMLKey(runeKey('c'))
	result := ret.(Model)
	assert.Equal(t, "abc", result.yamlView.searchText.Value)
}

func TestYAMLSearchModeBackspace(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchMode = true
	m.yamlView.searchText = TextInput{Value: "abc", Cursor: 3}
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "ab", result.yamlView.searchText.Value)
}

func TestYAMLSearchModeEnterActivatesSearch(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchMode = true
	m.yamlView.searchText = TextInput{Value: "key", Cursor: 3}
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.yamlView.searchMode)
	// "key" should match in the yaml content "  key: value"
	assert.Greater(t, len(result.yamlView.matchLines), 0)
}

func TestYAMLSearchModeCtrlW(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchMode = true
	m.yamlView.searchText = TextInput{Value: "hello world", Cursor: 11}
	ret, _ := m.handleYAMLKey(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	result := ret.(Model)
	assert.Equal(t, "hello ", result.yamlView.searchText.Value)
}

// --- handleYAMLKey: Visual mode ---

func TestYAMLVisualModeEscCancels(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'V'
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.yamlView.visualMode)
}

func TestYAMLVisualModeVToggle(t *testing.T) {
	t.Run("v cancels char mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualMode = true
		m.yamlView.visualType = 'v'
		ret, _ := m.handleYAMLKey(runeKey('v'))
		result := ret.(Model)
		assert.False(t, result.yamlView.visualMode)
	})

	t.Run("v switches from line to char", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualMode = true
		m.yamlView.visualType = 'V'
		ret, _ := m.handleYAMLKey(runeKey('v'))
		result := ret.(Model)
		assert.True(t, result.yamlView.visualMode)
		assert.Equal(t, rune('v'), result.yamlView.visualType)
	})
}

func TestYAMLVisualModeVVToggle(t *testing.T) {
	t.Run("V cancels line mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualMode = true
		m.yamlView.visualType = 'V'
		ret, _ := m.handleYAMLKey(runeKey('V'))
		result := ret.(Model)
		assert.False(t, result.yamlView.visualMode)
	})

	t.Run("V switches from char to line", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualMode = true
		m.yamlView.visualType = 'v'
		ret, _ := m.handleYAMLKey(runeKey('V'))
		result := ret.(Model)
		assert.True(t, result.yamlView.visualMode)
		assert.Equal(t, rune('V'), result.yamlView.visualType)
	})
}

func TestYAMLVisualModeCtrlVToggle(t *testing.T) {
	t.Run("ctrl+v cancels block mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualMode = true
		m.yamlView.visualType = 'B'
		ret, _ := m.handleYAMLKey(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
		result := ret.(Model)
		assert.False(t, result.yamlView.visualMode)
	})

	t.Run("ctrl+v switches from char to block", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualMode = true
		m.yamlView.visualType = 'v'
		ret, _ := m.handleYAMLKey(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
		result := ret.(Model)
		assert.True(t, result.yamlView.visualMode)
		assert.Equal(t, rune('B'), result.yamlView.visualType)
	})
}

func TestYAMLVisualModeJKNav(t *testing.T) {
	t.Run("j in visual mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualMode = true
		m.yamlView.visualType = 'V'
		m.yamlView.cursor = 3
		ret, _ := m.handleYAMLKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 4, result.yamlView.cursor)
		assert.True(t, result.yamlView.visualMode)
	})

	t.Run("k in visual mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualMode = true
		m.yamlView.visualType = 'V'
		m.yamlView.cursor = 5
		ret, _ := m.handleYAMLKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 4, result.yamlView.cursor)
	})
}

func TestYAMLVisualModeHL(t *testing.T) {
	t.Run("h in visual mode moves column left", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualMode = true
		m.yamlView.visualType = 'v'
		m.yamlView.visualCurCol = 5
		ret, _ := m.handleYAMLKey(runeKey('h'))
		result := ret.(Model)
		assert.Equal(t, 4, result.yamlView.visualCurCol)
	})

	t.Run("l in visual mode moves column right", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualMode = true
		m.yamlView.visualType = 'v'
		m.yamlView.visualCurCol = 5
		ret, _ := m.handleYAMLKey(runeKey('l'))
		result := ret.(Model)
		assert.Equal(t, 6, result.yamlView.visualCurCol)
	})
}

func TestYAMLVisualModeGg(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'V'
	m.yamlView.cursor = 20

	ret, _ := m.handleYAMLKey(runeKey('g'))
	result := ret.(Model)
	assert.True(t, result.pendingG)

	ret2, _ := result.handleYAMLKey(runeKey('g'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.yamlView.cursor)
	assert.Equal(t, 0, result2.yamlView.scroll)
}

func TestYAMLVisualModeG(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'V'
	m.yamlView.cursor = 0
	ret, _ := m.handleYAMLKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 49, result.yamlView.cursor)
}

func TestYAMLVisualModeZero(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'
	m.yamlView.visualCurCol = 10
	ret, _ := m.handleYAMLKey(runeKey('0'))
	result := ret.(Model)
	assert.Equal(t, yamlFoldPrefixLen, result.yamlView.visualCurCol)
}

func TestYAMLVisualModeCtrlDU(t *testing.T) {
	// Visual-mode <C-d>/<C-u> share the sticky 'scroll' option but don't
	// consume counts themselves. With no prior counted press, fall back to
	// vim's default (yamlViewportLines/2).
	t.Run("ctrl+d in visual mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualMode = true
		m.yamlView.visualType = 'V'
		m.yamlView.cursor = 0
		ret, _ := m.handleYAMLKey(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
		result := ret.(Model)
		assert.Equal(t, m.yamlViewportLines()/2, result.yamlView.cursor)
	})

	t.Run("ctrl+u in visual mode", func(t *testing.T) {
		m := baseYAMLModel()
		m.yamlView.visualMode = true
		m.yamlView.visualType = 'V'
		m.yamlView.cursor = 30
		ret, _ := m.handleYAMLKey(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
		result := ret.(Model)
		assert.Equal(t, 30-m.yamlViewportLines()/2, result.yamlView.cursor)
	})
}

// --- handleYAMLKey: Fold operations ---

func TestYAMLKeyTabTogglesFold(t *testing.T) {
	// Without sections, tab is a no-op but should not crash.
	m := baseYAMLModel()
	m.yamlView.cursor = 0
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyTab))
	result := ret.(Model)
	assert.Equal(t, modeYAML, result.mode)
}

func TestYAMLKeyZTogglesFold(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.cursor = 0
	ret, _ := m.handleYAMLKey(runeKey('z'))
	result := ret.(Model)
	assert.Equal(t, modeYAML, result.mode)
}

func TestYAMLKeyZToggleAllFolds(t *testing.T) {
	// With sections, Z toggles all folds.
	m := baseYAMLModel()
	m.yamlView.sections = []yamlSection{
		{key: "metadata", startLine: 0, endLine: 5},
		{key: "spec", startLine: 6, endLine: 10},
	}
	ret, _ := m.handleYAMLKey(runeKey('Z'))
	result := ret.(Model)
	// All multi-line sections should be collapsed.
	assert.True(t, result.yamlView.collapsed["metadata"])
	assert.True(t, result.yamlView.collapsed["spec"])

	// Toggle again should expand all.
	ret2, _ := result.handleYAMLKey(runeKey('Z'))
	result2 := ret2.(Model)
	assert.False(t, result2.yamlView.collapsed["metadata"])
	assert.False(t, result2.yamlView.collapsed["spec"])
}

func TestYAMLKeyCtrlC(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.scroll = 10
	m.yamlView.cursor = 5
	ret, _ := m.handleYAMLKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
	assert.Equal(t, 0, result.yamlView.scroll)
	assert.Equal(t, 0, result.yamlView.cursor)
}

func TestYAMLVisualModeCtrlCExits(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'V'
	m.yamlView.cursor = 5
	ret, _ := m.handleYAMLKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	result := ret.(Model)
	assert.False(t, result.yamlView.visualMode)
	assert.Equal(t, modeExplorer, result.mode)
}
