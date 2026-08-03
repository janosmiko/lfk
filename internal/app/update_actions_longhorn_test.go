package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

var longhornNodeResourceType = model.ResourceTypeEntry{
	APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "nodes", Kind: "Node", Namespaced: true,
}

func longhornNodeModel() Model {
	return Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: longhornNodeResourceType,
		},
		middleItems: []model.Item{
			{Name: "lh-node-1", Kind: "Node", Namespace: "longhorn-system"},
		},
		tabs:  []TabState{{}},
		width: 80, height: 40,
	}
}

// TestLonghornNode_ActionMenu_UsesDedicatedMenu verifies the action overlay for
// a longhorn.io node offers the Longhorn verbs, not the core-node kubectl verbs.
func TestLonghornNode_ActionMenu_UsesDedicatedMenu(t *testing.T) {
	m := longhornNodeModel()
	m = m.openResourceActionMenu()

	labels := make([]string, 0, len(m.overlayItems))
	for _, it := range m.overlayItems {
		labels = append(labels, it.Name)
	}
	assert.Contains(t, labels, "Force Delete")
	assert.Contains(t, labels, "Evict Replicas")
	assert.Contains(t, labels, "Cancel Eviction")
	assert.NotContains(t, labels, "Cordon")
	assert.NotContains(t, labels, "Drain")
}

// TestLonghornNode_ForceDelete_RoutesToLonghornPath verifies the global
// force-delete key is allowed for longhorn.io nodes (kind "Node" is not in
// IsForceDeleteableKind) and opens the type-to-confirm overlay.
func TestLonghornNode_ForceDelete_OpensTypeConfirm(t *testing.T) {
	m := longhornNodeModel()
	ret, _ := m.directActionForceDelete()
	result := ret.(Model)
	assert.Equal(t, overlayConfirmType, result.overlay)
	assert.Equal(t, "Force Delete", result.pendingAction)
	assert.Contains(t, result.confirmQuestion, "Force delete lh-node-1")
}

// TestCoreNode_ForceDelete_StillBlocked guards against the longhorn allowance
// leaking onto core Kubernetes nodes (same Kind "Node").
func TestCoreNode_ForceDelete_StillBlocked(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{APIGroup: "", APIVersion: "v1", Resource: "nodes", Kind: "Node"},
		},
		middleItems: []model.Item{{Name: "worker-1", Kind: "Node"}},
		tabs:        []TabState{{}},
		width:       80, height: 40,
	}
	ret, cmd := m.directActionForceDelete()
	result := ret.(Model)
	assert.NotEqual(t, overlayConfirmType, result.overlay)
	assert.Contains(t, result.statusMessage, "Force delete not available")
	require.NotNil(t, cmd) // status-clear timer
}

// TestLonghornNode_EvictReplicas_OpensConfirm verifies the Evict Replicas
// action opens the simple confirm with eviction-specific wording.
func TestLonghornNode_EvictReplicas_OpensConfirm(t *testing.T) {
	m := longhornNodeModel()
	m.actionCtx = m.buildActionCtx(&m.middleItems[0], "Node")
	ret, _ := m.executeActionEvictReplicas()
	result := ret.(Model)
	assert.Equal(t, overlayConfirm, result.overlay)
	assert.Equal(t, "Evict Replicas", result.pendingAction)
	assert.Equal(t, "Confirm Evict Replicas", result.confirmTitle)
	assert.Contains(t, result.confirmQuestion, "Evict all replicas from lh-node-1")
}

// TestLonghornNode_CancelEviction_OpensConfirm verifies the Cancel Eviction
// action opens the simple confirm with cancel wording.
func TestLonghornNode_CancelEviction_OpensConfirm(t *testing.T) {
	m := longhornNodeModel()
	m.actionCtx = m.buildActionCtx(&m.middleItems[0], "Node")
	ret, _ := m.executeActionCancelEviction()
	result := ret.(Model)
	assert.Equal(t, overlayConfirm, result.overlay)
	assert.Equal(t, "Cancel Eviction", result.pendingAction)
	assert.Equal(t, "Confirm Cancel Eviction", result.confirmTitle)
}

// TestConfirmOverlay_CancelClearsTitleOverride guards against the eviction
// confirm's title/question bleeding into a later plain-delete confirm: after
// cancelling Evict Replicas, the simple confirm must fall back to its default
// "Delete X?" wording.
func TestConfirmOverlay_CancelClearsTitleOverride(t *testing.T) {
	m := longhornNodeModel()
	m.actionCtx = m.buildActionCtx(&m.middleItems[0], "Node")
	ret, _ := m.executeActionEvictReplicas()
	m = ret.(Model)
	require.Equal(t, "Confirm Evict Replicas", m.confirmTitle)

	// Cancel the overlay.
	ret, _ = m.handleConfirmOverlayKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = ret.(Model)
	assert.Empty(t, m.confirmTitle, "title override must be cleared on cancel")
	assert.Empty(t, m.confirmQuestion, "question override must be cleared on cancel")
}

// TestEvictReplicas_BlockedByReadOnly verifies the eviction actions are gated
// by read-only mode (registered in mutatingActions).
func TestEvictReplicas_BlockedByReadOnly(t *testing.T) {
	assert.True(t, isMutatingAction("Evict Replicas"))
	assert.True(t, isMutatingAction("Cancel Eviction"))
}
