package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// restoringModel is a model mid-restore: the explorer has already navigated to
// the saved resource type, but the list has not come back yet.
func restoringModel() Model {
	m := basePush80Model()
	m.restoringSession = true
	m.middleItems = nil
	m.loading = true
	m.pendingTarget = "pod-2"
	m.pendingTargetNamespace = "ns-2"
	return m
}

func loadThreePods(m Model) Model {
	mdl, _ := m.updateResourcesLoaded(resourcesLoadedMsg{
		gen: m.requestGen,
		items: []model.Item{
			{Name: "pod-1", Namespace: "default", Kind: "Pod"},
			{Name: "pod-2", Namespace: "ns-2", Kind: "Pod"},
			{Name: "pod-3", Namespace: "default", Kind: "Pod"},
		},
	})
	return mdl.(Model)
}

func TestRestoreGuard_NoOverlayIsDrawn(t *testing.T) {
	m := restoringModel()

	view := stripANSI(m.renderView())

	assert.NotContains(t, view, "Restoring session")
	assert.NotContains(t, view, "press any key")
}

func TestRestoreGuard_UntouchedRestoreLandsOnTheSavedRow(t *testing.T) {
	got := loadThreePods(restoringModel())

	require.Len(t, got.visibleMiddleItems(), 3)
	assert.Equal(t, 1, got.cursor(), "pod-2 in ns-2 is the saved row")
	assert.False(t, got.restoringSession)
}

func TestRestoreGuard_AKeystrokeDropsTheSavedCursor(t *testing.T) {
	m := restoringModel()

	mdl, _ := m.updateImpl(tea.KeyPressMsg{Code: 'j', Text: "j"})
	got := mdl.(Model)

	assert.False(t, got.restoringSession)
	assert.Empty(t, got.pendingTarget)
	assert.Empty(t, got.pendingTargetNamespace)
}

func TestRestoreGuard_KeystrokeSurvivesTheLateLoad(t *testing.T) {
	m := restoringModel()

	mdl, _ := m.updateImpl(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = mdl.(Model)
	m.middleItems = []model.Item{{Name: "pod-9", Namespace: "default", Kind: "Pod"}}
	m.setCursor(0)

	got := loadThreePods(m)

	assert.NotEqual(t, 1, got.cursor(), "the restore must not steal the cursor after a keystroke")
}

func TestRestoreGuard_TheKeyStillDoesItsJob(t *testing.T) {
	m := restoringModel()
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod"},
		{Name: "pod-2", Namespace: "ns-2", Kind: "Pod"},
	}
	m.loading = false
	m.setCursor(0)

	mdl, _ := m.updateImpl(tea.KeyPressMsg{Code: 'j', Text: "j"})
	got := mdl.(Model)

	assert.Equal(t, 1, got.cursor(), "the guard consumes nothing; j still moves down")
}

func TestRestoreGuard_MouseClickAlsoDropsTheSavedCursor(t *testing.T) {
	m := restoringModel()

	mdl, _ := m.updateImpl(tea.MouseClickMsg{Button: tea.MouseLeft})

	assert.Empty(t, mdl.(Model).pendingTarget)
}

func TestRestoreGuard_LaterKeystrokesLeaveOtherJumpsAlone(t *testing.T) {
	// pendingTarget also carries bookmark jumps and orphan drill-downs. Once
	// the restore is over, a keystroke must not clear it.
	m := basePush80Model()
	m.restoringSession = false
	m.pendingTarget = "bookmarked-pod"

	mdl, _ := m.updateImpl(tea.KeyPressMsg{Code: 'j', Text: "j"})

	assert.Equal(t, "bookmarked-pod", mdl.(Model).pendingTarget)
}

func TestRestoreGuard_ContextMissingFromKubeconfigEndsTheRestore(t *testing.T) {
	m := basePush80Model()
	m.restoringSession = true
	m.nav.Level = model.LevelClusters

	mdl, _ := m.restoreSingleTabSession(
		&SessionState{Context: "gone", ResourceType: "pods"},
		[]model.Item{{Name: "other", IsContext: true}},
	)

	assert.False(t, mdl.(Model).restoringSession)
}

func TestRestoreGuard_MultiTabContextMissingEndsTheRestore(t *testing.T) {
	m := basePush80Model()
	m.restoringSession = true
	m.nav.Level = model.LevelClusters

	mdl, _ := m.restoreMultiTabSession(
		&SessionState{Tabs: []SessionTab{{Context: "gone", ResourceType: "pods"}}},
		[]model.Item{{Name: "other", IsContext: true}},
	)

	assert.False(t, mdl.(Model).restoringSession)
}

func TestRestoreGuard_UnresolvableResourceTypeEndsTheRestore(t *testing.T) {
	m := basePush80Model()
	m.restoringSession = true
	m.nav.Level = model.LevelResourceTypes
	m.sessionResourceTypeAwaitingDiscovery = "widgets.example.com"

	got, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: m.nav.Context,
		entries: []model.ResourceTypeEntry{
			{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true},
		},
	})

	assert.False(t, got.restoringSession)
}

func TestRestoreGuard_OffWhenThereIsNoSavedSession(t *testing.T) {
	assert.False(t, basePush80Model().restoringSession)
}
