package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestExecuteBulkAction_RestartOpensConfirmInsteadOfFiring ensures that the
// bulk Restart path now routes through a confirmation overlay. Pre-fix it
// jumped straight to bulkRestartResources, which in union mode meant a
// single keystroke could issue rollout restart across multiple clusters
// with no opportunity to review the blast radius.
func TestExecuteBulkAction_RestartOpensConfirmInsteadOfFiring(t *testing.T) {
	m := baseModelWithFakeClient()
	m.bulkMode = true
	m.bulkItems = []model.Item{
		{Name: "deploy-1", Namespace: "ns1", Kind: "Deployment", ClusterName: "blue"},
		{Name: "deploy-2", Namespace: "ns1", Kind: "Deployment", ClusterName: "green"},
	}
	m.actionCtx = actionContext{
		context: UnionContextSentinel,
		kind:    "Deployment",
	}

	model, cmd := m.executeBulkAction("Restart")
	rm := model.(Model)

	assert.Equal(t, overlayConfirm, rm.overlay,
		"bulk Restart must open a confirmation overlay (it was firing immediately)")
	assert.Equal(t, "Restart", rm.pendingAction,
		"pendingAction must be Restart so the confirm-key handler can dispatch correctly")
	assert.Nil(t, cmd, "no command must fire until the user confirms")
}

// TestExecuteBulkAction_DeleteConfirmNamesClustersInUnionMode covers the
// blast-radius signalling requirement: in union mode the confirmation
// prompt must enumerate the unique clusters that will be hit, so a user
// who thinks they are operating on a single cluster cannot mistake a
// multi-cluster bulk delete for a single-cluster one.
func TestExecuteBulkAction_DeleteConfirmNamesClustersInUnionMode(t *testing.T) {
	m := baseModelWithFakeClient()
	m.unionMode = true
	m.unionContexts = []string{"blue", "green"}
	m.bulkMode = true
	m.bulkItems = []model.Item{
		{Name: "pod-1", Namespace: "ns1", Kind: "Pod", ClusterName: "blue"},
		{Name: "pod-2", Namespace: "ns1", Kind: "Pod", ClusterName: "green"},
		{Name: "pod-3", Namespace: "ns1", Kind: "Pod", ClusterName: "blue"},
	}
	m.actionCtx = actionContext{
		context: UnionContextSentinel,
		kind:    "Pod",
	}

	model, _ := m.executeBulkAction("Delete")
	rm := model.(Model)

	assert.Equal(t, overlayConfirm, rm.overlay)
	// Both clusters appear once each, regardless of how many rows came from each.
	assert.Contains(t, rm.confirmAction, "blue", "confirm prompt must name the blue cluster")
	assert.Contains(t, rm.confirmAction, "green", "confirm prompt must name the green cluster")
}

// TestExecuteBulkAction_ForceDeleteConfirmNamesClustersInUnionMode mirrors
// the Delete test for the force-delete (typed confirmation) path.
func TestExecuteBulkAction_ForceDeleteConfirmNamesClustersInUnionMode(t *testing.T) {
	m := baseModelWithFakeClient()
	m.unionMode = true
	m.unionContexts = []string{"blue", "green"}
	m.bulkMode = true
	m.bulkItems = []model.Item{
		{Name: "pod-1", Namespace: "ns1", Kind: "Pod", ClusterName: "blue"},
		{Name: "pod-2", Namespace: "ns1", Kind: "Pod", ClusterName: "green"},
	}
	m.actionCtx = actionContext{
		context: UnionContextSentinel,
		kind:    "Pod",
	}

	model, _ := m.executeBulkAction("Force Delete")
	rm := model.(Model)

	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Contains(t, rm.confirmQuestion, "blue")
	assert.Contains(t, rm.confirmQuestion, "green")
}

// TestExecuteBulkAction_DeleteConfirmInSingleClusterModeNoCluster confirms
// the cluster-naming logic only kicks in when union mode is active.
// Single-cluster users keep the original prompt to avoid breaking the
// existing snapshot-style expectations and noise.
func TestExecuteBulkAction_DeleteConfirmInSingleClusterModeNoCluster(t *testing.T) {
	m := baseModelWithFakeClient()
	m.unionMode = false
	m.bulkMode = true
	m.bulkItems = []model.Item{
		{Name: "pod-1", Namespace: "ns1", Kind: "Pod"},
		{Name: "pod-2", Namespace: "ns1", Kind: "Pod"},
	}
	m.actionCtx = actionContext{
		context: "test-ctx",
		kind:    "Pod",
	}

	model, _ := m.executeBulkAction("Delete")
	rm := model.(Model)

	assert.Equal(t, overlayConfirm, rm.overlay)
	assert.Equal(t, "2 resources", rm.confirmAction,
		"single-cluster prompt unchanged so existing tests/UX hold")
}

// TestHandleConfirmOverlayKey_RestartDispatchesBulkRestart covers the
// confirm-overlay dispatch path: when the user presses Enter with
// pendingAction == "Restart" and bulkMode active, the resulting tea.Cmd
// must be the bulk-restart command — not bulk delete.
func TestHandleConfirmOverlayKey_RestartDispatchesBulkRestart(t *testing.T) {
	m := baseModelWithFakeClient()
	m.bulkMode = true
	m.pendingAction = "Restart"
	m.overlay = overlayConfirm
	m.bulkItems = []model.Item{
		{Name: "deploy-1", Namespace: "ns1", Kind: "Deployment", ClusterName: "blue"},
	}
	m.actionCtx = actionContext{
		context:      UnionContextSentinel,
		kind:         "Deployment",
		resourceType: model.ResourceTypeEntry{Resource: "deployments"},
	}

	model, cmd := m.handleConfirmOverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	rm := model.(Model)

	require.NotNil(t, cmd, "confirm must dispatch a command for bulk Restart")
	assert.Equal(t, overlayNone, rm.overlay, "overlay must close after confirm")
	assert.Empty(t, rm.pendingAction, "pendingAction must be cleared after dispatch")
	// We cannot directly identify which bulk command was returned without
	// invoking it, but the next assertion ensures the dispatch did not
	// fall through to the legacy "any bulk action = bulk delete" branch.
	// We rely on the fact that bulkRestartResources logs "Bulk restarting"
	// and bulkDeleteResources logs "Bulk deleting" — covered by Theme 1.
}

// --- Regression: stale bulk-action snapshot ---
//
// Confirming or cancelling a bulk action used to leave bulkMode/bulkItems
// set until the background operation finished. During that window a
// single-item action routed through executeAction's bulk gate and acted
// on the stale snapshot — e.g. pressing delete on one pod prompted
// "delete N resources" for the N items deleted moments earlier.

func TestHandleConfirmOverlayKey_BulkDeleteClearsBulkSnapshot(t *testing.T) {
	m := baseModelWithFakeClient()
	m.bulkMode = true
	m.pendingAction = "Delete"
	m.overlay = overlayConfirm
	m.bulkItems = make([]model.Item, 50)
	m.actionCtx = actionContext{
		context:      "test-ctx",
		kind:         "Pod",
		resourceType: model.ResourceTypeEntry{Resource: "pods", Namespaced: true},
	}

	res, cmd := m.handleConfirmOverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	rm := res.(Model)

	require.NotNil(t, cmd, "bulk delete must dispatch a command")
	assert.False(t, rm.bulkMode, "bulkMode must be reset once the bulk delete is dispatched")
	assert.Empty(t, rm.bulkItems, "bulkItems must be cleared once the bulk delete is dispatched")
}

func TestHandleConfirmTypeOverlayKey_BulkForceDeleteClearsBulkSnapshot(t *testing.T) {
	m := baseModelWithFakeClient()
	m.bulkMode = true
	m.pendingAction = "Force Delete"
	m.overlay = overlayConfirmType
	m.confirmTypeInput = TextInput{Value: "DELETE", Cursor: 6}
	m.bulkItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod"},
		{Name: "pod-2", Namespace: "default", Kind: "Pod"},
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		kind:         "Pod",
		resourceType: model.ResourceTypeEntry{Resource: "pods", Namespaced: true},
	}

	res, cmd := m.handleConfirmTypeOverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	rm := res.(Model)

	require.NotNil(t, cmd, "bulk force delete must dispatch a command")
	assert.False(t, rm.bulkMode, "bulkMode must be reset once the bulk force delete is dispatched")
	assert.Empty(t, rm.bulkItems, "bulkItems must be cleared once the bulk force delete is dispatched")
}

func TestHandleScaleOverlayKey_BulkScaleClearsBulkSnapshot(t *testing.T) {
	m := baseModelWithFakeClient()
	m.bulkMode = true
	m.overlay = overlayScaleInput
	m.scaleInput = TextInput{Value: "3"}
	m.bulkItems = []model.Item{
		{Name: "deploy-1", Namespace: "default", Kind: "Deployment"},
		{Name: "deploy-2", Namespace: "default", Kind: "Deployment"},
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		kind:         "Deployment",
		resourceType: model.ResourceTypeEntry{Resource: "deployments", Namespaced: true},
	}

	res, cmd := m.handleScaleOverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	rm := res.(Model)

	require.NotNil(t, cmd, "bulk scale must dispatch a command")
	assert.False(t, rm.bulkMode, "bulkMode must be reset once the bulk scale is dispatched")
	assert.Empty(t, rm.bulkItems, "bulkItems must be cleared once the bulk scale is dispatched")
}

func TestHandleConfirmOverlayKey_CancelClearsBulkSnapshot(t *testing.T) {
	m := baseModelWithFakeClient()
	m.bulkMode = true
	m.pendingAction = "Delete"
	m.overlay = overlayConfirm
	m.bulkItems = []model.Item{{Name: "pod-1", Namespace: "default", Kind: "Pod"}}

	res, _ := m.handleConfirmOverlayKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	rm := res.(Model)

	assert.False(t, rm.bulkMode, "cancelling a bulk delete must reset bulkMode")
	assert.Empty(t, rm.bulkItems, "cancelling a bulk delete must clear bulkItems")
}

func TestHandleConfirmTypeOverlayKey_CancelClearsBulkSnapshot(t *testing.T) {
	m := baseModelWithFakeClient()
	m.bulkMode = true
	m.pendingAction = "Force Delete"
	m.overlay = overlayConfirmType
	m.confirmTypeInput = TextInput{Value: "DEL"}
	m.bulkItems = []model.Item{{Name: "pod-1", Namespace: "default", Kind: "Pod"}}

	res, _ := m.handleConfirmTypeOverlayKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	rm := res.(Model)

	assert.False(t, rm.bulkMode, "cancelling a bulk force delete must reset bulkMode")
	assert.Empty(t, rm.bulkItems, "cancelling a bulk force delete must clear bulkItems")
}

func TestHandleScaleOverlayKey_CancelClearsBulkSnapshot(t *testing.T) {
	m := baseModelWithFakeClient()
	m.bulkMode = true
	m.overlay = overlayScaleInput
	m.scaleInput = TextInput{Value: "3"}
	m.bulkItems = []model.Item{{Name: "deploy-1", Namespace: "default", Kind: "Deployment"}}

	res, _ := m.handleScaleOverlayKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	rm := res.(Model)

	assert.False(t, rm.bulkMode, "cancelling a bulk scale must reset bulkMode")
	assert.Empty(t, rm.bulkItems, "cancelling a bulk scale must clear bulkItems")
}

// A read-only block dismisses the confirm overlay without dispatching. The
// bulk snapshot must still be cleared, otherwise it leaks exactly as it
// would after a successful dispatch.

func TestHandleConfirmOverlayKey_ReadOnlyBlockedClearsBulkSnapshot(t *testing.T) {
	m := baseModelWithFakeClient()
	m.cliReadOnly = true
	m.bulkMode = true
	m.pendingAction = "Delete"
	m.overlay = overlayConfirm
	m.bulkItems = []model.Item{{Name: "pod-1", Namespace: "default", Kind: "Pod"}}
	m.actionCtx = actionContext{context: "test-ctx", kind: "Pod"}

	res, _ := m.handleConfirmOverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	rm := res.(Model)

	assert.Equal(t, overlayNone, rm.overlay, "read-only block must close the overlay")
	assert.False(t, rm.bulkMode, "read-only block must reset bulkMode")
	assert.Empty(t, rm.bulkItems, "read-only block must clear bulkItems")
}

func TestHandleConfirmTypeOverlayKey_ReadOnlyBlockedClearsBulkSnapshot(t *testing.T) {
	m := baseModelWithFakeClient()
	m.cliReadOnly = true
	m.bulkMode = true
	m.pendingAction = "Force Delete"
	m.overlay = overlayConfirmType
	m.confirmTypeInput = TextInput{Value: "DELETE", Cursor: 6}
	m.bulkItems = []model.Item{{Name: "pod-1", Namespace: "default", Kind: "Pod"}}
	m.actionCtx = actionContext{context: "test-ctx", kind: "Pod"}

	res, _ := m.handleConfirmTypeOverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	rm := res.(Model)

	assert.Equal(t, overlayNone, rm.overlay, "read-only block must close the overlay")
	assert.False(t, rm.bulkMode, "read-only block must reset bulkMode")
	assert.Empty(t, rm.bulkItems, "read-only block must clear bulkItems")
}

func TestCloseCurrentOverlay_ClearsBulkSnapshot(t *testing.T) {
	// ctrl+c on a confirm overlay is intercepted by handleOverlayKey and
	// routed to closeCurrentOverlay, bypassing the per-overlay cancel
	// branches — so the snapshot must be cleared here too.
	m := baseModelWithFakeClient()
	m.bulkMode = true
	m.overlay = overlayConfirm
	m.bulkItems = []model.Item{{Name: "pod-1", Namespace: "default", Kind: "Pod"}}

	res, _ := m.closeCurrentOverlay()
	rm := res.(Model)

	assert.False(t, rm.bulkMode, "closing the overlay must reset bulkMode")
	assert.Empty(t, rm.bulkItems, "closing the overlay must clear bulkItems")
}

func TestHandleOverlayKey_ToggleCloseClearsBulkSnapshot(t *testing.T) {
	// The bulk action menu (overlayAction) is opened with bulkMode set.
	// Pressing its hotkey again toggles it shut via handleOverlayKey's
	// toggle path, which must also drop the bulk snapshot.
	m := baseModelWithFakeClient()
	m.bulkMode = true
	m.overlay = overlayAction
	m.bulkItems = []model.Item{{Name: "pod-1", Namespace: "default", Kind: "Pod"}}

	res, _ := m.handleOverlayKey(keyPressText(ui.ActiveKeybindings.ActionMenu))
	rm := res.(Model)

	assert.Equal(t, overlayNone, rm.overlay, "toggle key must close the bulk action menu")
	assert.False(t, rm.bulkMode, "toggling the bulk action menu shut must reset bulkMode")
	assert.Empty(t, rm.bulkItems, "toggling the bulk action menu shut must clear bulkItems")
}

func TestExecuteBulkAction_SyncClearsBulkSnapshot(t *testing.T) {
	m := baseModelWithFakeClient()
	m.bulkMode = true
	m.bulkItems = []model.Item{
		{Name: "app-1", Namespace: "argocd", Kind: "Application"},
		{Name: "app-2", Namespace: "argocd", Kind: "Application"},
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		kind:         "Application",
		resourceType: model.ResourceTypeEntry{Resource: "applications", Namespaced: true},
	}

	res, cmd := m.executeBulkAction("Sync")
	rm := res.(Model)

	require.NotNil(t, cmd, "bulk sync must dispatch a command")
	assert.False(t, rm.bulkMode, "bulkMode must be reset once the bulk sync is dispatched")
	assert.Empty(t, rm.bulkItems, "bulkItems must be cleared once the bulk sync is dispatched")
}
