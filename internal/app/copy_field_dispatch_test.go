package app

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestDefaultKeybindings_CopyFieldIsCtrlY(t *testing.T) {
	assert.Equal(t, "ctrl+y", ui.DefaultKeybindings().CopyField)
}

func TestHandleExplorerActionKeyCopyField_NoItemsIsNoOp(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.middleItems = nil
	mdl, cmd, handled := m.handleExplorerActionKeyCopyField()
	res := mdl.(Model)
	assert.True(t, handled)
	assert.Nil(t, cmd)
	assert.False(t, res.copyFieldPicker.active)
}

func TestHandleExplorerActionKeyCopyField_BulkCapDegradesToColumnsOnly(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.middleItems = make([]model.Item, maxBulkYAMLCopy+1)
	m.selectedItems = map[string]bool{}
	for i := range m.middleItems {
		m.middleItems[i].Name = string(rune('a' + i%26))
		m.middleItems[i].Namespace = "ns" + string(rune('0'+i/26))
		m.selectedItems[selectionKey(m.middleItems[i])] = true
	}
	mdl, cmd, handled := m.handleExplorerActionKeyCopyField()
	res := mdl.(Model)
	assert.True(t, handled)
	assert.Nil(t, cmd, "no manifest fetch above the cap")
	require.True(t, res.copyFieldPicker.active, "columns mode still works for any count")
	assert.True(t, res.copyFieldPicker.fieldsLoaded)
	assert.Contains(t, res.copyFieldPicker.fieldsErr, "Max")
}

func TestHandleExplorerActionKeyCopyField_DispatchesBackgroundFetch(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Node"}
	m.middleItems = []model.Item{{Name: "worker-1"}}
	m.setCursor(0)
	mdl, cmd, handled := m.handleExplorerActionKeyCopyField()
	res := mdl.(Model)
	assert.True(t, handled)
	require.NotNil(t, cmd, "a fetch command is dispatched")
	assert.True(t, res.copyFieldPicker.active, "picker opens without waiting for the fetch")
	assert.Equal(t, "Node", res.copyFieldPicker.kind)
}

func TestWrapYAMLCmdForCopyField_ParsesDocs(t *testing.T) {
	inner := func() tea.Msg {
		return yamlClipboardMsg{content: "kind: Node\nmetadata:\n  name: a\n---\nkind: Node\nmetadata:\n  name: b\n", count: 2}
	}
	msg := wrapYAMLCmdForCopyField(inner, 2, 7)()
	fm, ok := msg.(copyFieldManifestsMsg)
	require.True(t, ok)
	assert.Len(t, fm.docs, 2)
	assert.Equal(t, 2, fm.requested)
	assert.Equal(t, 7, fm.seq)
	assert.NoError(t, fm.err)
}

func TestWrapYAMLCmdForCopyField_PassesThroughErrors(t *testing.T) {
	inner := func() tea.Msg {
		return yamlClipboardMsg{err: errors.New("rbac denied")}
	}
	msg := wrapYAMLCmdForCopyField(inner, 1, 1)()
	fm, ok := msg.(copyFieldManifestsMsg)
	require.True(t, ok)
	assert.Empty(t, fm.docs)
	assert.Error(t, fm.err)
}

func TestWrapYAMLCmdForCopyField_NonClipboardMsgPassesThrough(t *testing.T) {
	inner := func() tea.Msg { return nil }
	msg := wrapYAMLCmdForCopyField(inner, 1, 1)()
	assert.Nil(t, msg)
}
