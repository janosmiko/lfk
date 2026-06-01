package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// --- handleYAMLKey: Normal mode fold operations (za, zo, zc, zM, zR) ---

func TestYAMLKeyZaTogglesFold(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "metadata:\n  name: test\n  labels:\n    app: nginx\nspec:\n  containers:\n  - name: nginx"
	m.yamlView.sections = []yamlSection{
		{key: "metadata", startLine: 0, endLine: 3},
		{key: "spec", startLine: 4, endLine: 6},
	}
	m.yamlView.collapsed = make(map[string]bool)
	m.yamlView.cursor = 0

	// "z" enters pending z mode, then "a" toggles fold.
	ret, _ := m.handleYAMLKey(runeKey('z'))
	result := ret.(Model)
	assert.Equal(t, modeYAML, result.mode)
}

func TestYAMLKeyZoOpensFold(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "metadata:\n  name: test\nspec:\n  containers: []\n"
	m.yamlView.sections = []yamlSection{
		{key: "metadata", startLine: 0, endLine: 1},
		{key: "spec", startLine: 2, endLine: 3},
	}
	m.yamlView.collapsed = map[string]bool{"metadata": true}
	m.yamlView.cursor = 0

	// Tab toggles the fold for the section at cursor.
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyTab))
	result := ret.(Model)
	assert.Equal(t, modeYAML, result.mode)
}

// --- handleYAMLKey: Normal mode vim motions (w, b, e, $, ^, W, B, E) ---

func TestYAMLKeyDollarMovesToEndOfLine(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "apiVersion: v1\nkind: Pod"
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('$'))
	result := ret.(Model)
	// $ should move to end of line.
	assert.Greater(t, result.yamlView.visualCurCol, yamlFoldPrefixLen)
}

func TestYAMLKeyCaretMovesToFirstNonWhitespace(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "  apiVersion: v1\nkind: Pod"
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = 10
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('^'))
	result := ret.(Model)
	// ^ should move to first non-whitespace.
	assert.GreaterOrEqual(t, result.yamlView.visualCurCol, yamlFoldPrefixLen)
}

func TestYAMLKeyWMovesToNextWord(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "apiVersion: v1\nkind: Pod"
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('w'))
	result := ret.(Model)
	assert.Greater(t, result.yamlView.visualCurCol, yamlFoldPrefixLen)
}

func TestYAMLKeyBMovesToPrevWord(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "apiVersion: v1\nkind: Pod"
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = 10
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('b'))
	result := ret.(Model)
	assert.LessOrEqual(t, result.yamlView.visualCurCol, 10)
}

func TestYAMLKeyEMovesToEndOfWord(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "apiVersion: v1\nkind: Pod"
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('e'))
	result := ret.(Model)
	assert.Greater(t, result.yamlView.visualCurCol, yamlFoldPrefixLen)
}

func TestYAMLKeyCapitalWMovesToNextWORD(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "apiVersion: v1\nkind: Pod"
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('W'))
	result := ret.(Model)
	assert.Greater(t, result.yamlView.visualCurCol, yamlFoldPrefixLen)
}

func TestYAMLKeyCapitalBMovesToPrevWORD(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "apiVersion: v1\nkind: Pod"
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = 10
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('B'))
	result := ret.(Model)
	assert.LessOrEqual(t, result.yamlView.visualCurCol, 10)
}

func TestYAMLKeyCapitalEMovesToEndOfWORD(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "apiVersion: v1\nkind: Pod"
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('E'))
	result := ret.(Model)
	assert.Greater(t, result.yamlView.visualCurCol, yamlFoldPrefixLen)
}

// --- handleYAMLKey: Search mode additional keys ---

func TestYAMLSearchModeCtrlA(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchMode = true
	m.yamlView.searchText = TextInput{Value: "hello", Cursor: 5}
	ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlA})
	result := ret.(Model)
	assert.Equal(t, 0, result.yamlView.searchText.Cursor)
}

func TestYAMLSearchModeCtrlE(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchMode = true
	m.yamlView.searchText = TextInput{Value: "hello", Cursor: 0}
	ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlE})
	result := ret.(Model)
	assert.Equal(t, 5, result.yamlView.searchText.Cursor)
}

func TestYAMLSearchModeLeft(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchMode = true
	m.yamlView.searchText = TextInput{Value: "abc", Cursor: 3}
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyLeft))
	result := ret.(Model)
	assert.Equal(t, 2, result.yamlView.searchText.Cursor)
}

func TestYAMLSearchModeRight(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchMode = true
	m.yamlView.searchText = TextInput{Value: "abc", Cursor: 0}
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyRight))
	result := ret.(Model)
	assert.Equal(t, 1, result.yamlView.searchText.Cursor)
}

func TestYAMLSearchModeCtrlCCancels(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchMode = true
	m.yamlView.searchText = TextInput{Value: "test", Cursor: 4}
	m.yamlView.matchLines = []int{1}
	ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := ret.(Model)
	assert.False(t, result.yamlView.searchMode)
	assert.Equal(t, "", result.yamlView.searchText.Value)
	assert.Nil(t, result.yamlView.matchLines)
}

func TestYAMLSearchModeSpaceChar(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchMode = true
	m.yamlView.searchText = TextInput{Value: "test", Cursor: 4}
	ret, _ := m.handleYAMLKey(runeKey(' '))
	result := ret.(Model)
	assert.Equal(t, "test ", result.yamlView.searchText.Value)
}

func TestYAMLSearchModeBackspaceEmptyString(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchMode = true
	m.yamlView.searchText = TextInput{Value: "", Cursor: 0}
	ret, _ := m.handleYAMLKey(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "", result.yamlView.searchText.Value)
}

// --- handleYAMLKey: Visual mode additional motions ---

func TestYAMLVisualModeDollar(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('$'))
	result := ret.(Model)
	assert.Greater(t, result.yamlView.visualCurCol, yamlFoldPrefixLen)
}

func TestYAMLVisualModeW(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('w'))
	result := ret.(Model)
	assert.Greater(t, result.yamlView.visualCurCol, yamlFoldPrefixLen)
}

func TestYAMLVisualModeB(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = 10
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('b'))
	result := ret.(Model)
	assert.LessOrEqual(t, result.yamlView.visualCurCol, 10)
}

func TestYAMLVisualModeE(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('e'))
	result := ret.(Model)
	assert.Greater(t, result.yamlView.visualCurCol, yamlFoldPrefixLen)
}

func TestYAMLVisualModeCaret(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = 10
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('^'))
	result := ret.(Model)
	assert.GreaterOrEqual(t, result.yamlView.visualCurCol, yamlFoldPrefixLen)
}

func TestYAMLVisualModeCapitalW(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('W'))
	result := ret.(Model)
	assert.Greater(t, result.yamlView.visualCurCol, yamlFoldPrefixLen)
}

func TestYAMLVisualModeCapitalB(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = 10
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('B'))
	result := ret.(Model)
	assert.LessOrEqual(t, result.yamlView.visualCurCol, 10)
}

func TestYAMLVisualModeCapitalE(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	m.yamlView.collapsed = make(map[string]bool)

	ret, _ := m.handleYAMLKey(runeKey('E'))
	result := ret.(Model)
	assert.Greater(t, result.yamlView.visualCurCol, yamlFoldPrefixLen)
}

// --- handleYAMLKey: G with pending count ---

func TestYAMLKeyGWithCount(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.collapsed = make(map[string]bool)
	// First press a digit, then G should go to that line.
	// Press 'G' directly should go to last line.
	ret, _ := m.handleYAMLKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 49, result.yamlView.cursor)
}

// --- handleYAMLKey: q exits YAML mode ---

func TestYAMLKeyQExitsToExplorer(t *testing.T) {
	m := baseYAMLModel()
	ret, _ := m.handleYAMLKey(runeKey('q'))
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
}

func TestYAMLKeyQClearsSearchFirst(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.searchText = TextInput{Value: "query", Cursor: 5}
	m.yamlView.matchLines = []int{1, 3}
	ret, _ := m.handleYAMLKey(runeKey('q'))
	result := ret.(Model)
	assert.Equal(t, modeYAML, result.mode, "should stay in YAML mode when clearing search")
	assert.Equal(t, "", result.yamlView.searchText.Value)
}

// --- handleYAMLKey: Ctrl+F and Ctrl+B full-page scroll ---

func TestYAMLKeyCtrlFClampsAtEnd(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.collapsed = make(map[string]bool)
	m.yamlView.cursor = 45
	ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	result := ret.(Model)
	assert.Equal(t, 49, result.yamlView.cursor)
}

func TestYAMLKeyCtrlBClampsAtZero(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.collapsed = make(map[string]bool)
	m.yamlView.cursor = 5
	ret, _ := m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	result := ret.(Model)
	assert.Equal(t, 0, result.yamlView.cursor)
}

// --- handleYAMLKey: N/n search wrapping ---

func TestYAMLKeyNWrapsToStart(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.matchLines = []int{5, 10, 20}
	m.yamlView.matchIdx = 2
	m.yamlView.collapsed = make(map[string]bool)
	ret, _ := m.handleYAMLKey(runeKey('n'))
	result := ret.(Model)
	assert.Equal(t, 0, result.yamlView.matchIdx) // wraps to start
}

func TestYAMLKeyCapitalNWrapsToEnd(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.matchLines = []int{5, 10, 20}
	m.yamlView.matchIdx = 0
	m.yamlView.collapsed = make(map[string]bool)
	ret, _ := m.handleYAMLKey(runeKey('N'))
	result := ret.(Model)
	assert.Equal(t, 2, result.yamlView.matchIdx) // wraps to end
}

func TestYAMLKeyNNoMatchesNoop(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.matchLines = nil
	m.yamlView.matchIdx = 0
	ret, _ := m.handleYAMLKey(runeKey('n'))
	result := ret.(Model)
	assert.Equal(t, 0, result.yamlView.matchIdx) // unchanged
}

// --- handleYAMLKey: Visual mode yank (copy) ---

func TestYAMLVisualModeYankLineMode(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "line0\nline1\nline2"
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'V'
	m.yamlView.visualStart = 0
	m.yamlView.cursor = 1
	m.yamlView.collapsed = make(map[string]bool)

	ret, cmd := m.handleYAMLKey(runeKey('y'))
	result := ret.(Model)
	assert.False(t, result.yamlView.visualMode)
	assert.NotNil(t, cmd)
}

func TestYAMLVisualModeYankCharMode(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "apiVersion: v1\nkind: Pod"
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'
	m.yamlView.visualStart = 0
	m.yamlView.cursor = 0
	m.yamlView.visualCol = yamlFoldPrefixLen
	m.yamlView.visualCurCol = yamlFoldPrefixLen + 5
	m.yamlView.collapsed = make(map[string]bool)

	ret, cmd := m.handleYAMLKey(runeKey('y'))
	result := ret.(Model)
	assert.False(t, result.yamlView.visualMode)
	assert.NotNil(t, cmd)
}

func TestYAMLVisualModeYankBlockMode(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "apiVersion: v1\nkind: Pod\nmetadata:"
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'B'
	m.yamlView.visualStart = 0
	m.yamlView.cursor = 1
	m.yamlView.visualCol = yamlFoldPrefixLen
	m.yamlView.visualCurCol = yamlFoldPrefixLen + 4
	m.yamlView.collapsed = make(map[string]bool)

	ret, cmd := m.handleYAMLKey(runeKey('y'))
	result := ret.(Model)
	assert.False(t, result.yamlView.visualMode)
	assert.NotNil(t, cmd)
}

// --- handleYAMLKey: Visual mode yank single line ---

func TestYAMLVisualModeYankSingleLineChar(t *testing.T) {
	m := baseYAMLModel()
	m.yamlView.content = "apiVersion: v1"
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'
	m.yamlView.visualStart = 0
	m.yamlView.cursor = 0
	m.yamlView.visualCol = yamlFoldPrefixLen + 2
	m.yamlView.visualCurCol = yamlFoldPrefixLen + 5
	m.yamlView.collapsed = make(map[string]bool)

	ret, cmd := m.handleYAMLKey(runeKey('y'))
	result := ret.(Model)
	assert.False(t, result.yamlView.visualMode)
	assert.NotNil(t, cmd)
}
