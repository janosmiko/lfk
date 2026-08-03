package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// shift+up / shift+down are aliases for ctrl+u / ctrl+d (half-page scroll),
// mirroring how pgup / pgdown alias ctrl+b / ctrl+f. These tests assert the
// alias produces the same observable movement as the real key across the
// representative scroll handlers, and that the alias does NOT leak into
// text-input contexts where ctrl+u means "delete to start of line".

func manyLines(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "x"
	}
	return strings.Join(lines, "\n")
}

func TestShiftDownAliasesCtrlDInExplorerList(t *testing.T) {
	items := make([]model.Item, 50)
	for i := range items {
		items[i] = model.Item{Name: "pod", Kind: "Pod"}
	}
	build := func() Model {
		m := baseExplorerModel()
		m.middleItems = items
		m.setCursor(0)
		return m
	}

	realRet, _, _ := build().handleExplorerActionKey(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	aliasRet, _, handled := build().handleExplorerActionKey(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
	assert.True(t, handled, "shift+down must be handled as half-page down")
	realM, aliasM := realRet.(Model), aliasRet.(Model)
	assert.Greater(t, aliasM.cursor(), 0)
	assert.Equal(t, realM.cursor(), aliasM.cursor())
}

func TestShiftUpAliasesCtrlUInExplorerList(t *testing.T) {
	items := make([]model.Item, 50)
	for i := range items {
		items[i] = model.Item{Name: "pod", Kind: "Pod"}
	}
	build := func() Model {
		m := baseExplorerModel()
		m.middleItems = items
		m.setCursor(40)
		return m
	}

	realRet, _, _ := build().handleExplorerActionKey(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	aliasRet, _, handled := build().handleExplorerActionKey(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	assert.True(t, handled, "shift+up must be handled as half-page up")
	realM, aliasM := realRet.(Model), aliasRet.(Model)
	assert.Less(t, aliasM.cursor(), 40)
	assert.Equal(t, realM.cursor(), aliasM.cursor())
}

func TestShiftArrowAliasesInYAMLView(t *testing.T) {
	build := func() Model {
		return Model{
			width: 80, height: 30, mode: modeYAML,
			yamlView: yamlViewState{
				content:   manyLines(100),
				collapsed: map[string]bool{},
				cursor:    50,
			},
			tabs: []TabState{{}},
		}
	}
	rd, _ := build().handleYAMLKey(keyMsg("ctrl+d"))
	ad, _ := build().handleYAMLKey(keyMsg("shift+down"))
	assert.Equal(t, rd.(Model).yamlView.cursor, ad.(Model).yamlView.cursor)
	assert.NotEqual(t, 50, ad.(Model).yamlView.cursor, "shift+down must move the cursor")

	ru, _ := build().handleYAMLKey(keyMsg("ctrl+u"))
	au, _ := build().handleYAMLKey(keyMsg("shift+up"))
	assert.Equal(t, ru.(Model).yamlView.cursor, au.(Model).yamlView.cursor)
	assert.NotEqual(t, 50, au.(Model).yamlView.cursor, "shift+up must move the cursor")
}

func TestShiftArrowAliasesInDiffView(t *testing.T) {
	content := manyLines(200)
	build := func() Model {
		return Model{
			width: 80, height: 40, mode: modeDiff,
			diffView: diffViewState{
				left: content, right: content,
				leftName: "before", rightName: "after",
				cursor: 50,
			},
			tabs: []TabState{{}},
		}
	}
	rd, _ := build().handleDiffKey(keyMsg("ctrl+d"))
	ad, _ := build().handleDiffKey(keyMsg("shift+down"))
	assert.Equal(t, rd.(Model).diffView.cursor, ad.(Model).diffView.cursor)
	assert.NotEqual(t, 50, ad.(Model).diffView.cursor)

	ru, _ := build().handleDiffKey(keyMsg("ctrl+u"))
	au, _ := build().handleDiffKey(keyMsg("shift+up"))
	assert.Equal(t, ru.(Model).diffView.cursor, au.(Model).diffView.cursor)
	assert.NotEqual(t, 50, au.(Model).diffView.cursor)
}

func TestShiftArrowAliasesInDescribeView(t *testing.T) {
	build := func() Model {
		m := baseModelDescribe()
		m.describeView.cursor = 5
		return m
	}
	rd, _ := build().handleDescribeKey(keyMsg("ctrl+d"))
	ad, _ := build().handleDescribeKey(keyMsg("shift+down"))
	assert.Equal(t, rd.(Model).describeView.cursor, ad.(Model).describeView.cursor)
	assert.Greater(t, ad.(Model).describeView.cursor, 5)

	ru, _ := build().handleDescribeKey(keyMsg("ctrl+u"))
	au, _ := build().handleDescribeKey(keyMsg("shift+up"))
	assert.Equal(t, ru.(Model).describeView.cursor, au.(Model).describeView.cursor)
	assert.Less(t, au.(Model).describeView.cursor, 5)
}

func TestShiftArrowAliasesInLogView(t *testing.T) {
	build := func() Model {
		m := basePush4Model()
		m.mode = modeLogs
		m.logView.lines = strings.Split(manyLines(100), "\n")
		m.logView.cursor = 50
		return m
	}
	rd, _ := build().handleLogKey(keyMsg("ctrl+d"))
	ad, _ := build().handleLogKey(keyMsg("shift+down"))
	assert.Equal(t, rd.(Model).logView.cursor, ad.(Model).logView.cursor)
	assert.NotEqual(t, 50, ad.(Model).logView.cursor)

	ru, _ := build().handleLogKey(keyMsg("ctrl+u"))
	au, _ := build().handleLogKey(keyMsg("shift+up"))
	assert.Equal(t, ru.(Model).logView.cursor, au.(Model).logView.cursor)
	assert.NotEqual(t, 50, au.(Model).logView.cursor)
}

// Regression: in a text input, ctrl+u clears the line. shift+up must NOT
// inherit that behavior — it should leave the input untouched.
func TestShiftUpDoesNotClearFilterInput(t *testing.T) {
	m := baseExplorerModel()
	m.filterInput.Value = "nginx"

	ret, _ := m.handleFilterKey(keyMsg("shift+up"))
	assert.Equal(t, "nginx", ret.(Model).filterInput.Value,
		"shift+up must not clear the filter input the way ctrl+u does")
}

func TestShiftUpDoesNotClearSearchInput(t *testing.T) {
	m := baseExplorerModel()
	m.searchInput.Value = "error"

	ret, _ := m.handleSearchKey(keyMsg("shift+up"))
	assert.Equal(t, "error", ret.(Model).searchInput.Value,
		"shift+up must not clear the search input the way ctrl+u does")
}
