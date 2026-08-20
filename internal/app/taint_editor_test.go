package app

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/janosmiko/lfk/internal/model"
)

// taintEditorTestModel returns a model with the action context set on a
// node, as if the action menu was opened on it.
func taintEditorTestModel() Model {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Node", Resource: "nodes"}
	m.actionCtx = actionContext{kind: "Node", name: "worker-1", context: "test-ctx", resourceType: m.nav.ResourceType}
	return m
}

func nodeTaints() []model.Taint {
	return []model.Taint{
		{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"},
		{Key: "maintenance", Effect: "NoExecute"},
	}
}

// openTaintEditorLoaded opens the editor and injects a loaded fetch.
func openTaintEditorLoaded(t *testing.T, m Model) Model {
	t.Helper()
	mdl, cmd := m.openTaintEditor()
	res, ok := mdl.(Model)
	require.True(t, ok)
	require.NotNil(t, cmd, "taint fetch dispatched")
	require.True(t, res.taintEditor.loading)
	mdl, _ = res.updateTaintsLoaded(taintsLoadedMsg{taints: nodeTaints(), seq: res.taintEditor.seq})
	res, ok = mdl.(Model)
	require.True(t, ok)
	require.False(t, res.taintEditor.loading)
	return res
}

func taintKey(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "space":
			msg = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEsc}
		case "tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab}
		case "right":
			msg = tea.KeyPressMsg{Code: tea.KeyRight}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		default:
			msg = keyPressText(k)
		}
		mdl, _ := m.handleTaintEditorKey(msg)
		var ok bool
		m, ok = mdl.(Model)
		require.True(t, ok)
	}
	return m
}

func TestOpenTaintEditor_LoadsAndShowsTaints(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	assert.Equal(t, overlayTaintEditor, m.overlay)
	require.Len(t, m.taintEditor.rows, 2)
	assert.Equal(t, "dedicated=gpu:NoSchedule", m.taintEditor.rows[0].taint.String())
}

func TestUpdateTaintsLoaded_StaleSeqDropped(t *testing.T) {
	m := taintEditorTestModel()
	mdl, _ := m.openTaintEditor()
	res := mdl.(Model)
	stale := res.taintEditor.seq
	res.closeTaintEditor()
	mdl, _ = res.openTaintEditor()
	res = mdl.(Model)

	mdl, _ = res.updateTaintsLoaded(taintsLoadedMsg{taints: nodeTaints(), seq: stale})
	res = mdl.(Model)
	assert.True(t, res.taintEditor.loading, "stale fetch must not fill the new editor")
}

func TestUpdateTaintsLoaded_ErrorClosesWithStatus(t *testing.T) {
	m := taintEditorTestModel()
	mdl, _ := m.openTaintEditor()
	res := mdl.(Model)
	mdl, _ = res.updateTaintsLoaded(taintsLoadedMsg{err: errors.New("rbac denied"), seq: res.taintEditor.seq})
	res = mdl.(Model)
	assert.False(t, res.taintEditor.active)
	assert.NotEqual(t, overlayTaintEditor, res.overlay)
	assert.True(t, res.statusMessageErr)
}

func TestTaintEditor_SpaceTogglesRemovalMarkAndAdvancesCursor(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	require.Equal(t, 0, m.taintEditor.cursor)
	m = taintKey(t, m, "space")
	assert.True(t, m.taintEditor.rows[0].remove)
	assert.Equal(t, 1, m.taintEditor.cursor, "cursor advances after toggle, like the explorer")

	// On the last row the cursor stays clamped.
	m = taintKey(t, m, "space")
	assert.True(t, m.taintEditor.rows[1].remove)
	assert.Equal(t, 1, m.taintEditor.cursor)

	m = taintKey(t, m, "space")
	assert.False(t, m.taintEditor.rows[1].remove, "space still toggles off")
}

func TestTaintEditor_AddFlowStagesTaint(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "a")
	assert.Equal(t, taintFocusKey, m.taintEditor.focus)

	m = taintKey(t, m, "team", "tab", "ml", "tab", "right", "enter")
	require.Len(t, m.taintEditor.rows, 3)
	staged := m.taintEditor.rows[2]
	assert.True(t, staged.staged)
	assert.Equal(t, "team=ml:PreferNoSchedule", staged.taint.String())
	assert.Equal(t, taintFocusList, m.taintEditor.focus, "focus returns to list after staging")

	// Space on a staged row unstages (removes) it.
	m.taintEditor.cursor = 2
	m = taintKey(t, m, "space")
	assert.Len(t, m.taintEditor.rows, 2)
}

func TestTaintEditor_AddValidationRejectsBadKeyAndDuplicate(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())

	m = taintKey(t, m, "a", "bad key", "enter")
	assert.True(t, m.statusMessageErr, "invalid key rejected at staging")
	assert.Len(t, m.taintEditor.rows, 2)
	assert.NotEqual(t, taintFocusList, m.taintEditor.focus, "input stays open for correction")

	// Clear and retype a duplicate of an existing taint (key+effect).
	m = taintKey(t, m, "esc", "a")
	m = taintKey(t, m, "dedicated", "enter") // NoSchedule is the default effect
	assert.True(t, m.statusMessageErr, "duplicate key+effect rejected")
	assert.Len(t, m.taintEditor.rows, 2)
}

func TestTaintEditor_EnterNoChangesCloses(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "enter")
	assert.False(t, m.taintEditor.active)
	assert.Equal(t, overlayNone, m.overlay)
	assert.Contains(t, m.statusMessage, "No changes")
}

func TestTaintEditor_EnterWithChangesOpensConfirm(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "space", "enter") // mark first taint for removal
	assert.Equal(t, overlayConfirm, m.overlay)
	assert.Equal(t, "Apply Taints", m.pendingAction)
	assert.Contains(t, m.confirmQuestion, "1 removed")
	assert.True(t, m.taintEditor.active, "editor state survives while confirming")
}

func TestTaintEditor_ConfirmMentionsNoExecuteEviction(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "a", "evict-me", "tab", "tab", "right", "right", "enter") // effect -> NoExecute
	m = taintKey(t, m, "enter")
	assert.Equal(t, overlayConfirm, m.overlay)
	assert.Contains(t, m.confirmQuestion, "NoExecute")
	assert.Contains(t, m.confirmQuestion, "evict")
}

func TestTaintEditor_ConfirmCancelReturnsToEditor(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "space", "enter")
	require.Equal(t, overlayConfirm, m.overlay)

	mdl, _ := m.handleConfirmOverlayKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	res := mdl.(Model)
	assert.Equal(t, overlayTaintEditor, res.overlay, "cancel returns to the editor")
	assert.True(t, res.taintEditor.active)
	assert.True(t, res.taintEditor.rows[0].remove, "marks survive the round-trip")
}

func TestTaintEditor_EscDiscards(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "space", "esc")
	assert.False(t, m.taintEditor.active)
	assert.Equal(t, overlayNone, m.overlay)
}

func TestRenderOverlayTaintEditor_RowsAndMarks(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "space")
	m = taintKey(t, m, "a", "team", "tab", "ml", "tab", "enter")

	content, w, h := m.renderOverlayTaintEditor()
	require.NotEmpty(t, content)
	assert.Positive(t, w)
	assert.Positive(t, h)
	plain := stripANSI(content)
	assert.Contains(t, plain, "Taints")
	assert.Contains(t, plain, "worker-1")
	assert.Contains(t, plain, "dedicated=gpu:NoSchedule")
	assert.Contains(t, plain, "team=ml:NoSchedule")
}

func TestRenderOverlayTaintEditor_LoadingState(t *testing.T) {
	m := taintEditorTestModel()
	mdl, _ := m.openTaintEditor()
	res := mdl.(Model)
	content, _, _ := res.renderOverlayTaintEditor()
	assert.Contains(t, stripANSI(content), "Loading")
}

func TestExecuteAction_TaintsOpensEditor(t *testing.T) {
	m := taintEditorTestModel()
	mdl, _ := m.executeAction("Taints")
	res := mdl.(Model)
	assert.Equal(t, overlayTaintEditor, res.overlay)
	assert.True(t, res.taintEditor.active)
}

func TestExecuteAction_TaintsBlockedInReadOnly(t *testing.T) {
	m := taintEditorTestModel()
	m.readOnly = true
	mdl, _ := m.executeAction("Taints")
	res := mdl.(Model)
	assert.NotEqual(t, overlayTaintEditor, res.overlay)
	assert.False(t, res.taintEditor.active)
	assert.True(t, res.statusMessageErr)
}

func TestTaintEditor_ReadOnlyToggleDuringConfirmBlocksApply(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "space", "enter")
	require.Equal(t, overlayConfirm, m.overlay)

	// RO toggled on while the confirm dialog is already showing: the
	// confirm-time safety net must refuse to commit the mutation.
	m.readOnly = true
	mdl, _ := m.handleConfirmOverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	res := mdl.(Model)
	assert.True(t, res.statusMessageErr, "read-only block message shown")
	assert.Contains(t, res.statusMessage, "Read-only")
	assert.Empty(t, res.pendingAction, "pending mutation discarded")
}

func TestTaintEditor_RemoveThenReAddSameIdentity(t *testing.T) {
	// The only way to change a taint's value: mark the old one for
	// removal, stage a replacement with the same key+effect.
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m.taintEditor.cursor = 0 // dedicated=gpu:NoSchedule
	m = taintKey(t, m, "space")
	m = taintKey(t, m, "a", "dedicated", "tab", "cpu", "tab", "enter")
	require.Len(t, m.taintEditor.rows, 3, "replacement stages once the original is marked for removal")
	assert.Equal(t, "dedicated=cpu:NoSchedule", m.taintEditor.rows[2].taint.String())
}

func TestCloseTaintEditor_ClearsPreviousOverlay(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m.overlay = overlayNone // simulate the confirm handler having closed the overlay first
	m.closeTaintEditor()
	assert.Equal(t, overlayNone, m.previousOverlay)
}

func TestTaintEditor_EndToEndApply(t *testing.T) {
	node := &corev1.Node{
		Name: "worker-1",
		Spec: corev1.NodeSpec{Taints: []corev1.Taint{
			{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
		}},
	}
	m := baseModelWithFakeClientAndScheduler(t, node)
	m.width, m.height = 120, 40
	m.nav.Level = model.LevelResources
	m.actionCtx = actionContext{kind: "Node", name: "worker-1", context: "test-ctx"}

	// Open and run the fetch through the real scheduler.
	mdl, cmd := m.openTaintEditor()
	res := mdl.(Model)
	require.NotNil(t, cmd)
	loaded, ok := cmd().(taintsLoadedMsg)
	require.True(t, ok)
	require.NoError(t, loaded.err)
	mdl, _ = res.updateTaintsLoaded(loaded)
	res = mdl.(Model)
	require.Len(t, res.taintEditor.rows, 1)

	// Mark removal, stage an addition, apply through the confirm overlay.
	res = taintKey(t, res, "space")
	res = taintKey(t, res, "a", "team", "tab", "ml", "tab", "enter")
	res = taintKey(t, res, "enter")
	require.Equal(t, overlayConfirm, res.overlay)
	mdl, applyCmd := res.handleConfirmOverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	res = mdl.(Model)
	require.NotNil(t, applyCmd)
	assert.False(t, res.taintEditor.active, "editor closed on confirmed apply")

	applied, ok := applyCmd().(taintsAppliedMsg)
	require.True(t, ok)
	require.NoError(t, applied.err)
	mdl, _ = res.updateTaintsApplied(applied)
	res = mdl.(Model)
	assert.Contains(t, res.statusMessage, "Taints updated on worker-1")

	got, err := res.client.GetNodeTaints(t.Context(), "test-ctx", "worker-1")
	require.NoError(t, err)
	assert.Equal(t, []model.Taint{{Key: "team", Value: "ml", Effect: "NoSchedule"}}, got,
		"removal applied, addition landed")
}

func TestNodeActionMenu_SingleTaintsEntry(t *testing.T) {
	actions := model.ActionsForKind("Node")
	labels := make([]string, 0, len(actions))
	for _, a := range actions {
		labels = append(labels, a.Label)
	}
	assert.Contains(t, labels, "Taints")
	assert.NotContains(t, labels, "Taint")
	assert.NotContains(t, labels, "Untaint")
}
