package app

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// copyFieldTestModel returns a model at LevelResources with one node row.
func copyFieldTestModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Node"}
	m.middleItems = []model.Item{{Name: "worker-1", Status: "Ready"}}
	m.setCursor(0)
	return m
}

// copyFieldOpenPicker opens the picker via the ctrl+y handler (columns
// mode) and injects the manifest fetch result so fields mode is ready.
func copyFieldOpenPicker(t *testing.T, m Model) Model {
	t.Helper()
	docs := []any{nodeManifestDoc()}
	mdl, _, handled := m.handleExplorerActionKeyCopyField()
	require.True(t, handled)
	res, ok := mdl.(Model)
	require.True(t, ok)
	mdl, _ = res.updateCopyFieldManifests(copyFieldManifestsMsg{
		docs: docs, requested: len(docs), seq: res.copyFieldPicker.seq,
	})
	res, ok = mdl.(Model)
	require.True(t, ok)
	return res
}

// tabToFields switches the picker into fields mode.
func tabToFields(t *testing.T, m Model) Model {
	t.Helper()
	mdl, _ := m.handleCopyFieldPickerKey(tea.KeyMsg{Type: tea.KeyTab})
	res, ok := mdl.(Model)
	require.True(t, ok)
	require.Equal(t, copyFieldModeFields, res.copyFieldPicker.mode)
	return res
}

func TestCopyFieldPicker_OpensInstantlyInColumnsMode(t *testing.T) {
	m := copyFieldTestModel(t)
	mdl, cmd, handled := m.handleExplorerActionKeyCopyField()
	res := mdl.(Model)
	require.True(t, handled)
	assert.NotNil(t, cmd, "background manifest fetch dispatched")
	assert.Equal(t, overlayCopyField, res.overlay)
	require.True(t, res.copyFieldPicker.active)
	assert.Equal(t, copyFieldModeColumns, res.copyFieldPicker.mode)
	assert.False(t, res.copyFieldPicker.fieldsLoaded)

	// Columns are available immediately, before any fetch completes.
	vis := res.visibleCopyFieldEntries()
	require.NotEmpty(t, vis)
	name := findCopyFieldEntry(vis, "NAME")
	require.NotNil(t, name)
	assert.Equal(t, "worker-1", name.value)
}

func TestCopyFieldPicker_TabTogglesToSemanticFields(t *testing.T) {
	m := copyFieldOpenPicker(t, copyFieldTestModel(t))
	m = tabToFields(t, m)

	vis := m.visibleCopyFieldEntries()
	assert.NotNil(t, findCopyFieldEntry(vis, "status.addresses[ExternalIP].address"))

	// Tab back returns to columns.
	mdl, _ := m.handleCopyFieldPickerKey(tea.KeyMsg{Type: tea.KeyTab})
	m = mdl.(Model)
	assert.Equal(t, copyFieldModeColumns, m.copyFieldPicker.mode)
	assert.NotNil(t, findCopyFieldEntry(m.visibleCopyFieldEntries(), "NAME"))
}

func TestCopyFieldPicker_ExternalIPFilterFindsAddressRow(t *testing.T) {
	m := copyFieldOpenPicker(t, copyFieldTestModel(t))
	m = tabToFields(t, m)

	m.copyFieldPicker.filter = "externalip"
	m.recomputeCopyFieldVisible()
	vis := m.visibleCopyFieldEntries()
	require.NotEmpty(t, vis)
	assert.NotNil(t, findCopyFieldEntry(vis, "status.addresses[ExternalIP].address"),
		"the address row is reachable via the semantic label, not just the type row")
}

func TestUpdateCopyFieldManifests_DroppedWhenPickerClosed(t *testing.T) {
	m := copyFieldTestModel(t)
	mdl, _, _ := m.handleExplorerActionKeyCopyField()
	res := mdl.(Model)
	seq := res.copyFieldPicker.seq
	res.closeCopyFieldPicker()

	mdl, _ = res.updateCopyFieldManifests(copyFieldManifestsMsg{docs: []any{nodeManifestDoc()}, seq: seq})
	res = mdl.(Model)
	assert.False(t, res.copyFieldPicker.active, "stale fetch must not reopen the picker")
	assert.NotEqual(t, overlayCopyField, res.overlay)
}

func TestUpdateCopyFieldManifests_StaleSeqDropped(t *testing.T) {
	m := copyFieldTestModel(t)
	mdl, _, _ := m.handleExplorerActionKeyCopyField()
	res := mdl.(Model)
	staleSeq := res.copyFieldPicker.seq
	res.closeCopyFieldPicker()
	mdl, _, _ = res.handleExplorerActionKeyCopyField() // reopen — new seq
	res = mdl.(Model)

	mdl, _ = res.updateCopyFieldManifests(copyFieldManifestsMsg{docs: []any{nodeManifestDoc()}, seq: staleSeq})
	res = mdl.(Model)
	assert.False(t, res.copyFieldPicker.fieldsLoaded, "old fetch must not fill the new picker")
}

func TestUpdateCopyFieldManifests_FetchErrorSurfacesOnFieldsMode(t *testing.T) {
	m := copyFieldTestModel(t)
	mdl, _, _ := m.handleExplorerActionKeyCopyField()
	res := mdl.(Model)
	mdl, _ = res.updateCopyFieldManifests(copyFieldManifestsMsg{
		err: errors.New("rbac denied"), seq: res.copyFieldPicker.seq,
	})
	res = mdl.(Model)
	assert.True(t, res.copyFieldPicker.active, "columns mode stays usable")
	assert.True(t, res.copyFieldPicker.fieldsLoaded)
	assert.Contains(t, res.copyFieldPicker.fieldsErr, "rbac denied")
}

func TestApplyCopyFieldPicker_ColumnValue(t *testing.T) {
	m := copyFieldTestModel(t)
	m.middleItems = []model.Item{
		{Name: "worker-1", Status: "Ready"},
		{Name: "worker-2", Status: "NotReady"},
	}
	m.selectedItems = map[string]bool{
		selectionKey(m.middleItems[0]): true,
		selectionKey(m.middleItems[1]): true,
	}
	mdl, _, _ := m.handleExplorerActionKeyCopyField()
	res := mdl.(Model)

	res.copyFieldPicker.filter = "status"
	res.recomputeCopyFieldVisible()
	res.copyFieldPicker.cursor = 0
	mdl, cmd := res.applyCopyFieldPicker()
	res = mdl.(Model)

	assert.NotNil(t, cmd)
	assert.Contains(t, res.statusMessage, "2 values")
	assert.Equal(t, copyFieldMemory{mode: copyFieldModeColumns, display: "STATUS"},
		res.lastCopyFieldByKind["Node"])
}

func TestApplyCopyFieldPicker_ColumnValuePartialMissIsCounted(t *testing.T) {
	m := copyFieldTestModel(t)
	m.middleItems = []model.Item{
		{Name: "worker-1", Status: "Ready"},
		{Name: "worker-2"}, // no Status — must be skipped and counted
	}
	m.selectedItems = map[string]bool{
		selectionKey(m.middleItems[0]): true,
		selectionKey(m.middleItems[1]): true,
	}
	mdl, _, _ := m.handleExplorerActionKeyCopyField()
	res := mdl.(Model)

	res.copyFieldPicker.filter = "status"
	res.recomputeCopyFieldVisible()
	res.copyFieldPicker.cursor = 0
	mdl, cmd := res.applyCopyFieldPicker()
	res = mdl.(Model)

	assert.NotNil(t, cmd)
	assert.Contains(t, res.statusMessage, "1 values")
	assert.Contains(t, res.statusMessage, "1 missing")
}

func TestApplyCopyFieldPicker_FieldValueRemembersPerKind(t *testing.T) {
	m := copyFieldOpenPicker(t, copyFieldTestModel(t))
	m = tabToFields(t, m)

	m.copyFieldPicker.filter = "34.1.2.3"
	m.recomputeCopyFieldVisible()
	m.copyFieldPicker.cursor = 0
	mdl, cmd := m.applyCopyFieldPicker()
	res := mdl.(Model)

	assert.NotNil(t, cmd)
	assert.False(t, res.copyFieldPicker.active)
	assert.Equal(t, overlayNone, res.overlay)
	assert.Contains(t, res.statusMessage, "status.addresses[ExternalIP].address")
	assert.Equal(t,
		copyFieldMemory{mode: copyFieldModeFields, display: "status.addresses[ExternalIP].address"},
		res.lastCopyFieldByKind["Node"])
}

func TestCopyFieldPicker_PreselectsRememberedFieldAfterTab(t *testing.T) {
	m := copyFieldTestModel(t)
	m.lastCopyFieldByKind = map[string]copyFieldMemory{
		"Node": {mode: copyFieldModeFields, display: "status.addresses[ExternalIP].address"},
	}
	m = copyFieldOpenPicker(t, m)
	assert.Equal(t, copyFieldModeColumns, m.copyFieldPicker.mode, "always opens on columns")
	m = tabToFields(t, m)

	vis := m.visibleCopyFieldEntries()
	require.NotEmpty(t, vis)
	require.Less(t, m.copyFieldPicker.cursor, len(vis))
	assert.Equal(t, "status.addresses[ExternalIP].address", vis[m.copyFieldPicker.cursor].display)
}

func TestCopyFieldPickerKeys_FilterTypingAndEscape(t *testing.T) {
	m := copyFieldOpenPicker(t, copyFieldTestModel(t))
	m = tabToFields(t, m)

	mdl, _ := m.handleCopyFieldPickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = mdl.(Model)
	assert.True(t, m.copyFieldPicker.filterActive)

	mdl, _ = m.handleCopyFieldPickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("kubelet")})
	m = mdl.(Model)
	assert.Equal(t, "kubelet", m.copyFieldPicker.filter)
	for _, e := range m.visibleCopyFieldEntries() {
		assert.True(t, strings.Contains(strings.ToLower(e.display), "kubelet") ||
			strings.Contains(strings.ToLower(e.value), "kubelet"))
	}

	// Enter leaves filter-typing mode but keeps the filter.
	mdl, _ = m.handleCopyFieldPickerKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = mdl.(Model)
	assert.False(t, m.copyFieldPicker.filterActive)
	assert.Equal(t, "kubelet", m.copyFieldPicker.filter)

	// Esc with a filter set clears it; second esc closes.
	mdl, _ = m.handleCopyFieldPickerKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mdl.(Model)
	assert.Equal(t, "", m.copyFieldPicker.filter)
	assert.True(t, m.copyFieldPicker.active)

	mdl, _ = m.handleCopyFieldPickerKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mdl.(Model)
	assert.False(t, m.copyFieldPicker.active)
	assert.Equal(t, overlayNone, m.overlay)
}

func TestCopyFieldPicker_TabResetsFilter(t *testing.T) {
	m := copyFieldOpenPicker(t, copyFieldTestModel(t))
	m.copyFieldPicker.filter = "name"
	m.recomputeCopyFieldVisible()
	m = tabToFields(t, m)
	assert.Equal(t, "", m.copyFieldPicker.filter, "filter does not carry across modes")
}

func TestCopyFieldPickerFilter_BackspaceIsRuneSafe(t *testing.T) {
	m := copyFieldOpenPicker(t, copyFieldTestModel(t))
	mdl, _ := m.handleCopyFieldPickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = mdl.(Model)
	mdl, _ = m.handleCopyFieldPickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("naïve")})
	m = mdl.(Model)
	mdl, _ = m.handleCopyFieldPickerKey(tea.KeyMsg{Type: tea.KeyBackspace})
	m = mdl.(Model)
	assert.Equal(t, "naïv", m.copyFieldPicker.filter, "backspace removes one rune, not one byte")
	mdl, _ = m.handleCopyFieldPickerKey(tea.KeyMsg{Type: tea.KeyBackspace})
	m = mdl.(Model)
	assert.Equal(t, "naï", m.copyFieldPicker.filter, "multibyte rune removed intact")
}

func TestRenderOverlayCopyField_LongRowsStayWithinWidth(t *testing.T) {
	m := copyFieldTestModel(t)
	doc := map[string]any{
		"kind": "Node",
		"metadata": map[string]any{
			"annotations": map[string]any{
				strings.Repeat("k", 200): strings.Repeat("v", 500),
			},
		},
	}
	mdl, _, _ := m.handleExplorerActionKeyCopyField()
	res := mdl.(Model)
	mdl, _ = res.updateCopyFieldManifests(copyFieldManifestsMsg{docs: []any{doc}, requested: 1, seq: res.copyFieldPicker.seq})
	res = mdl.(Model)
	res = tabToFields(t, res)

	content, w, _ := res.renderOverlayCopyField()
	innerW := w - 4
	for line := range strings.SplitSeq(stripANSI(content), "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), innerW+2,
			"no row may exceed the overlay content width")
	}
}

func TestRenderOverlayCopyField_ColumnsAndFieldsSubtitles(t *testing.T) {
	m := copyFieldOpenPicker(t, copyFieldTestModel(t))
	content, w, h := m.renderOverlayCopyField()
	require.NotEmpty(t, content)
	assert.Positive(t, w)
	assert.Positive(t, h)
	plain := stripANSI(content)
	assert.Contains(t, plain, "Copy field")
	assert.Contains(t, plain, "columns (tab: all fields)")
	assert.Contains(t, plain, "worker-1")

	m = tabToFields(t, m)
	plain = stripANSI(mustRenderCopyField(t, m))
	assert.Contains(t, plain, "all fields (tab: columns)")
	assert.Contains(t, plain, "metadata.name")
}

func TestRenderOverlayCopyField_LoadingState(t *testing.T) {
	m := copyFieldTestModel(t)
	mdl, _, _ := m.handleExplorerActionKeyCopyField()
	res := mdl.(Model)
	res = tabToFields(t, res) // fetch has not landed yet
	plain := stripANSI(mustRenderCopyField(t, res))
	assert.Contains(t, plain, "Loading fields...")
}

func mustRenderCopyField(t *testing.T, m Model) string {
	t.Helper()
	content, _, _ := m.renderOverlayCopyField()
	require.NotEmpty(t, content)
	return content
}
