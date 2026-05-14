package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// findFormatIndex returns the index of f in formats, or -1 if not present.
// Tests use this to avoid hard-coding cursor positions, so a future reorder
// of availableCopyFormats can't silently dispatch the wrong format.
func findFormatIndex(formats []CopyFormat, f CopyFormat) int {
	for i, x := range formats {
		if x == f {
			return i
		}
	}
	return -1
}

func TestOpenCopyFormatPicker_AtResources_HasThreeRows(t *testing.T) {
	m := Model{}
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	assert.True(t, m.copyFormatPicker.active)
	assert.Equal(t, []CopyFormat{CopyFormatYAML, CopyFormatJSON, CopyFormatTable}, m.copyFormatPicker.formats)
	assert.Equal(t, 0, m.copyFormatPicker.cursor)
	assert.Len(t, m.copyFormatPicker.scope, 1)
	assert.Equal(t, "a", m.copyFormatPicker.scope[0].Name)
}

func TestOpenCopyFormatPicker_AtClusters_TableOnly(t *testing.T) {
	m := Model{}
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "kind-1"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	assert.True(t, m.copyFormatPicker.active)
	assert.Equal(t, []CopyFormat{CopyFormatTable}, m.copyFormatPicker.formats)
}

func TestCopyFormatPicker_CycleCursor(t *testing.T) {
	m := Model{}
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	m.copyFormatPickerStep(1)
	assert.Equal(t, 1, m.copyFormatPicker.cursor)
	m.copyFormatPickerStep(1)
	assert.Equal(t, 2, m.copyFormatPicker.cursor)
	m.copyFormatPickerStep(1)
	assert.Equal(t, 0, m.copyFormatPicker.cursor, "wraps")
	m.copyFormatPickerStep(-1)
	assert.Equal(t, 2, m.copyFormatPicker.cursor, "wraps backward")
}

func TestCopyFormatPickerCancel(t *testing.T) {
	m := Model{}
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	m.closeCopyFormatPicker()
	assert.False(t, m.copyFormatPicker.active)
	assert.Nil(t, m.copyFormatPicker.scope)
	assert.Equal(t, overlayNone, m.overlay, "overlay restored to pre-open value")
	assert.Equal(t, overlayNone, m.previousOverlay, "previousOverlay cleared")
}

func TestCopyFormatPickerStep_LargeNegativeDelta(t *testing.T) {
	m := Model{}
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	// 3 formats: cursor at 0. delta -4 ≡ -1 mod 3 → cursor 2.
	m.copyFormatPickerStep(-4)
	assert.Equal(t, 2, m.copyFormatPicker.cursor, "large negative delta wraps correctly")
	// delta +7 ≡ +1 mod 3 → cursor 0.
	m.copyFormatPickerStep(7)
	assert.Equal(t, 0, m.copyFormatPicker.cursor, "large positive delta wraps correctly")
}

func TestCopyFormatPicker_ScopeSnapshotIsStableUnderMutation(t *testing.T) {
	m := Model{}
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}, {Name: "b"}}
	m.selectedItems = map[string]bool{
		selectionKey(m.middleItems[0]): true,
		selectionKey(m.middleItems[1]): true,
	}
	m.setCursor(0)
	m.openCopyFormatPicker()
	// Simulate a watch refresh dropping one item from the underlying list.
	m.middleItems = m.middleItems[:1]
	m.selectedItems = nil
	assert.Len(t, m.copyFormatPicker.scope, 2, "snapshot survives later mutation")
}

func TestOpenCopyFormatPicker_NoItemsIsNoOp(t *testing.T) {
	m := Model{}
	m.nav.Level = model.LevelResources
	m.middleItems = nil
	m.openCopyFormatPicker()
	assert.False(t, m.copyFormatPicker.active, "open with empty scope is a no-op")
}

func TestApplyCopyFormatPicker_TableDispatchesImmediately(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}, {Name: "b"}}
	m.selectedItems = map[string]bool{
		selectionKey(m.middleItems[0]): true,
		selectionKey(m.middleItems[1]): true,
	}
	m.setCursor(0)
	m.openCopyFormatPicker()
	tableIdx := findFormatIndex(m.copyFormatPicker.formats, CopyFormatTable)
	require.NotEqual(t, -1, tableIdx)
	m.copyFormatPicker.cursor = tableIdx
	_, cmd := m.applyCopyFormatPicker()
	assert.NotNil(t, cmd, "table dispatch must return a tea.Cmd")
	msg := cmd()
	yc, ok := msg.(yamlClipboardMsg)
	assert.True(t, ok, "table dispatch must produce a yamlClipboardMsg")
	assert.Equal(t, "table", yc.format)
	assert.Equal(t, 2, yc.count, "table count equals number of rows in scope")
	assert.Contains(t, yc.content, "NAME")
}

func TestApplyCopyFormatPicker_YAMLUsesExistingDispatcher(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a", Kind: "Pod"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	m.copyFormatPicker.cursor = 0 // YAML is documented as the first row.
	mdl, cmd := m.applyCopyFormatPicker()
	assert.NotNil(t, cmd, "yaml dispatch must return a tea.Cmd")
	r := mdl.(Model)
	assert.False(t, r.copyFormatPicker.active, "picker closes after apply (yaml path)")
}

func TestApplyCopyFormatPicker_ClosesAfterApply(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	tableIdx := findFormatIndex(m.copyFormatPicker.formats, CopyFormatTable)
	require.NotEqual(t, -1, tableIdx)
	m.copyFormatPicker.cursor = tableIdx // Table — pure, no network
	mdl, _ := m.applyCopyFormatPicker()
	m = mdl.(Model)
	assert.False(t, m.copyFormatPicker.active, "picker closes after apply")
}

func TestApplyCopyFormatPicker_JSONWrapsExistingDispatcher(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a", Kind: "Pod"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	jsonIdx := findFormatIndex(m.copyFormatPicker.formats, CopyFormatJSON)
	require.NotEqual(t, -1, jsonIdx)
	m.copyFormatPicker.cursor = jsonIdx
	_, cmd := m.applyCopyFormatPicker()
	// Cmd should exist when there's a valid item to copy.
	assert.NotNil(t, cmd, "JSON dispatch must return a tea.Cmd")
}

func TestAnyNonEmpty(t *testing.T) {
	items := []model.Item{{Name: ""}, {Name: ""}, {Name: "x"}}
	assert.True(t, anyNonEmpty(items, func(it model.Item) string { return it.Name }))

	empty := []model.Item{{Name: ""}, {Name: ""}}
	assert.False(t, anyNonEmpty(empty, func(it model.Item) string { return it.Name }))

	assert.False(t, anyNonEmpty(nil, func(it model.Item) string { return it.Name }))
}

func TestCopyTableColumnsForLevel_OnlyNonEmptyBuiltins(t *testing.T) {
	// Items have Namespace and Status populated, others empty.
	items := []model.Item{
		{Name: "a", Namespace: "default", Status: "Running"},
		{Name: "b", Namespace: "default", Status: "Pending"},
	}
	got := copyTableColumnsForLevel(model.LevelResources, items)
	assert.Equal(t, []string{"Name", "Namespace", "Status"}, got,
		"only Name plus populated built-ins; Ready/Restarts/Age skipped because empty")
}

func TestCopyTableColumnsForLevel_BuiltinOrder(t *testing.T) {
	// All built-ins populated → spec-defined order: Name, Namespace, Ready, Status, Restarts, Age.
	items := []model.Item{
		{Name: "a", Namespace: "n", Ready: "1/1", Status: "Running", Restarts: "0", Age: "1d"},
	}
	got := copyTableColumnsForLevel(model.LevelResources, items)
	assert.Equal(t, []string{"Name", "Namespace", "Ready", "Status", "Restarts", "Age"}, got)
}

func TestCopyTableColumnsForLevel_ExtraColumnsDeduped(t *testing.T) {
	items := []model.Item{
		{Name: "a", Columns: []model.KeyValue{{Key: "Image", Value: "x:1"}}},
		{Name: "b", Columns: []model.KeyValue{{Key: "Image", Value: "x:2"}, {Key: "Node", Value: "n1"}}},
	}
	got := copyTableColumnsForLevel(model.LevelResources, items)
	assert.Equal(t, []string{"Name", "Image", "Node"}, got, "extra columns deduped, original order preserved")
}

func TestCopyTableColumnsForLevel_LevelParamReserved(t *testing.T) {
	// Same items at different levels should produce identical output today —
	// `level` is reserved for future per-level customization.
	items := []model.Item{{Name: "a", Status: "Running"}}
	resources := copyTableColumnsForLevel(model.LevelResources, items)
	clusters := copyTableColumnsForLevel(model.LevelClusters, items)
	assert.Equal(t, resources, clusters, "level parameter is currently inert")
}

// withActiveColumnState sets the ui.Active* column-state globals for a test
// and restores them on cleanup. The picker reads these to mirror what the
// renderer last drew.
func withActiveColumnState(t *testing.T, order []string, hidden map[string]bool, session []string) {
	t.Helper()
	prevOrder := ui.ActiveColumnOrder
	prevHidden := ui.ActiveHiddenBuiltinColumns
	prevSession := ui.ActiveSessionColumns
	ui.ActiveColumnOrder = order
	ui.ActiveHiddenBuiltinColumns = hidden
	ui.ActiveSessionColumns = session
	t.Cleanup(func() {
		ui.ActiveColumnOrder = prevOrder
		ui.ActiveHiddenBuiltinColumns = prevHidden
		ui.ActiveSessionColumns = prevSession
	})
}

func TestCopyTableColumnsForLevel_HiddenBuiltinExcluded(t *testing.T) {
	withActiveColumnState(t, nil, map[string]bool{"Namespace": true, "Age": true}, nil)
	items := []model.Item{
		{Name: "a", Namespace: "n", Ready: "1/1", Status: "Running", Restarts: "0", Age: "1d"},
	}
	got := copyTableColumnsForLevel(model.LevelResources, items)
	assert.Equal(t, []string{"Name", "Ready", "Status", "Restarts"}, got,
		"hidden built-ins (Namespace, Age) must not appear in the copied table")
}

func TestCopyTableColumnsForLevel_SessionExtrasRestrictsToVisible(t *testing.T) {
	// User has hidden "Node" via the column-toggle overlay; only "Image" is visible.
	withActiveColumnState(t, nil, nil, []string{"Image"})
	items := []model.Item{
		{Name: "a", Columns: []model.KeyValue{
			{Key: "Image", Value: "x:1"},
			{Key: "Node", Value: "n1"},
		}},
	}
	got := copyTableColumnsForLevel(model.LevelResources, items)
	assert.Equal(t, []string{"Name", "Image"}, got,
		"only extras in ActiveSessionColumns appear; Node hidden by user is excluded")
}

func TestCopyTableColumnsForLevel_OrderRespected(t *testing.T) {
	// User moved Status to the front (after Name) and Namespace to the end.
	withActiveColumnState(t, []string{"Status", "Ready", "Restarts", "Age", "Namespace"}, nil, nil)
	items := []model.Item{
		{Name: "a", Namespace: "n", Ready: "1/1", Status: "Running", Restarts: "0", Age: "1d"},
	}
	got := copyTableColumnsForLevel(model.LevelResources, items)
	assert.Equal(t, []string{"Name", "Status", "Ready", "Restarts", "Age", "Namespace"}, got,
		"column order must follow ui.ActiveColumnOrder")
}

func TestCopyTableColumnsForLevel_OrderReorderExtras(t *testing.T) {
	// User reordered extras: Node before Image.
	withActiveColumnState(t, []string{"Namespace", "Node", "Image"}, nil, []string{"Image", "Node"})
	items := []model.Item{
		{Name: "a", Namespace: "n", Columns: []model.KeyValue{
			{Key: "Image", Value: "x:1"},
			{Key: "Node", Value: "n1"},
		}},
	}
	got := copyTableColumnsForLevel(model.LevelResources, items)
	assert.Equal(t, []string{"Name", "Namespace", "Node", "Image"}, got,
		"extras follow user reorder, not item-discovery order")
}

func TestCopyTableColumnsForLevel_InternalKeysFiltered(t *testing.T) {
	items := []model.Item{
		{Name: "a", Columns: []model.KeyValue{
			{Key: "Image", Value: "x:1"},
			{Key: "__internal", Value: "junk"},
			{Key: "secret:tls", Value: "skip"},
			{Key: "owner:rs", Value: "skip"},
			{Key: "data:foo", Value: "skip"},
			{Key: "condition:Ready", Value: "skip"},
			{Key: "step:1", Value: "skip"},
			{Key: "cond:Ready", Value: "skip"},
		}},
	}
	got := copyTableColumnsForLevel(model.LevelResources, items)
	assert.Equal(t, []string{"Name", "Image"}, got,
		"internal-prefixed Columns keys must be filtered out of the copy")
}

func TestCopyFormatPicker_EscClosesViaRouter(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	mdl, _ := m.handleCopyFormatPickerKey(tea.KeyMsg{Type: tea.KeyEsc})
	r := mdl.(Model)
	assert.False(t, r.copyFormatPicker.active, "esc cancels picker")
}

func TestCopyFormatPicker_DownKeyAdvancesCursor(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	mdl, _ := m.handleCopyFormatPickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	r := mdl.(Model)
	assert.Equal(t, 1, r.copyFormatPicker.cursor, "j moves cursor down")
}

func TestCopyFormatPicker_UpKeyMovesCursor(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	m.copyFormatPicker.cursor = 2
	mdl, _ := m.handleCopyFormatPickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	r := mdl.(Model)
	assert.Equal(t, 1, r.copyFormatPicker.cursor, "k moves cursor up")
}

func TestCopyFormatPicker_TShortcutAppliesTable(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	mdl, cmd := m.handleCopyFormatPickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	r := mdl.(Model)
	assert.False(t, r.copyFormatPicker.active, "t applies Table and closes picker")
	require.NotNil(t, cmd, "table dispatch returns non-nil cmd")
	msg := cmd().(yamlClipboardMsg)
	assert.Equal(t, "table", msg.format)
}

func TestCopyFormatPicker_YShortcutAppliesYAML(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a", Kind: "Pod"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	mdl, cmd := m.handleCopyFormatPickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	r := mdl.(Model)
	assert.False(t, r.copyFormatPicker.active, "y applies YAML and closes picker")
	require.NotNil(t, cmd, "yaml dispatch returns non-nil cmd")
}

func TestCopyFormatPicker_EnterAppliesCursorRow(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	// Move cursor to Table (last row)
	for i, f := range m.copyFormatPicker.formats {
		if f == CopyFormatTable {
			m.copyFormatPicker.cursor = i
			break
		}
	}
	mdl, cmd := m.handleCopyFormatPickerKey(tea.KeyMsg{Type: tea.KeyEnter})
	r := mdl.(Model)
	assert.False(t, r.copyFormatPicker.active, "enter applies and closes picker")
	require.NotNil(t, cmd)
	msg := cmd().(yamlClipboardMsg)
	assert.Equal(t, "table", msg.format)
}

func TestCopyFormatPicker_UnhandledKeyIsNoOp(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a"}}
	m.setCursor(0)
	m.openCopyFormatPicker()
	cursorBefore := m.copyFormatPicker.cursor
	mdl, cmd := m.handleCopyFormatPickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})
	r := mdl.(Model)
	assert.True(t, r.copyFormatPicker.active, "unhandled key leaves picker open")
	assert.Equal(t, cursorBefore, r.copyFormatPicker.cursor, "unhandled key doesn't move cursor")
	assert.Nil(t, cmd)
}
