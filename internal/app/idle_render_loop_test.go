package app

import (
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// A previewLoading flag that clearRight() arms and no handler clears leaves the
// 10 FPS spinner loop running for the life of the process, and every tick
// re-renders the whole screen (issue #646).

func newSpinnerStuckModel() Model {
	return Model{
		nav:                 model.NavigationState{Context: "test-ctx"},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		dashboardAcc:        make(map[string]*dashboardAccumulator),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		previewLoading:      true,
	}
}

// Discovery writes the resource-type list straight into middleItems, so the
// resourceTypesMsg that would otherwise disarm the flag never arrives. This is
// the path the app takes on launch.
func TestDiscoveryAtResourceTypesClearsPreviewLoading(t *testing.T) {
	m := newSpinnerStuckModel()
	m.nav.Level = model.LevelResourceTypes
	m.discoveringContexts = make(map[string]bool)
	m.discoveryRefreshedContexts = make(map[string]bool)

	got, cmd := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "test-ctx",
		entries: []model.ResourceTypeEntry{{
			Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true,
		}},
	})

	assert.Nil(t, cmd, "test setup: discovery must dispatch no preview load here")
	assert.False(t, got.previewLoading,
		"previewLoading must clear when discovery finishes the list without a preview load")
	assert.False(t, got.spinnerNeeded(),
		"the spinner loop must not stay armed once the app is quiescent")
}

// TestResourceTypesAtClusterLevelClearsPreviewLoading covers the startup
// screen. At LevelClusters the right pane previews the highlighted context's
// resource types, so the arriving list is the awaited preview result.
func TestResourceTypesAtClusterLevelClearsPreviewLoading(t *testing.T) {
	m := newSpinnerStuckModel()
	m.nav.Level = model.LevelClusters

	got, _ := m.updateResourceTypes(resourceTypesMsg{items: []model.Item{{Name: "pods"}}})

	assert.False(t, got.(Model).previewLoading,
		"previewLoading must clear when the cluster-level preview arrives")
}

// TestResourceTypesWithoutPreviewCmdClearsPreviewLoading covers
// LevelResourceTypes with nothing under the cursor: loadPreview dispatches no
// command, so no later message can clear the flag.
func TestResourceTypesWithoutPreviewCmdClearsPreviewLoading(t *testing.T) {
	m := newSpinnerStuckModel()
	m.nav.Level = model.LevelResourceTypes

	got, cmd := m.updateResourceTypes(resourceTypesMsg{items: nil})

	assert.Nil(t, cmd, "test setup: an empty list must dispatch no preview load")
	assert.False(t, got.(Model).previewLoading,
		"previewLoading must clear when no preview load follows")
}

// TestDashboardLoadedClearsPreviewLoading covers the cluster dashboard, the
// page issue #646 reports. The dashboard fan-out is the preview load, so its
// completion is what disarms the spinner.
func TestDashboardLoadedClearsPreviewLoading(t *testing.T) {
	m := newSpinnerStuckModel()
	m.nav.Level = model.LevelResourceTypes

	got := m.updateDashboardLoaded(dashboardLoadedMsg{
		context: "test-ctx",
		data:    dashboardData{nodeCount: 2, readyNodes: 2},
	})

	assert.False(t, got.previewLoading,
		"previewLoading must clear when the dashboard preview lands")
}

// TestDashboardStaticOverrideClearsPreviewLoading covers the "Cluster dashboard
// disabled" path, which returns early and would otherwise skip the reset.
func TestDashboardStaticOverrideClearsPreviewLoading(t *testing.T) {
	m := newSpinnerStuckModel()
	m.nav.Level = model.LevelResourceTypes

	got := m.updateDashboardLoaded(dashboardLoadedMsg{
		context: "test-ctx",
		content: "Cluster dashboard disabled",
	})

	assert.False(t, got.previewLoading,
		"previewLoading must clear when a static dashboard override lands")
}

// TestMonitoringDashboardClearsPreviewLoading covers the monitoring dashboard,
// which reaches the right pane through its own message.
func TestMonitoringDashboardClearsPreviewLoading(t *testing.T) {
	m := newSpinnerStuckModel()
	m.nav.Level = model.LevelResourceTypes

	got := m.updateMonitoringDashboard(monitoringDashboardMsg{
		context: "test-ctx",
		content: "MONITORING OVERVIEW",
	})

	assert.False(t, got.previewLoading,
		"previewLoading must clear when the monitoring preview lands")
}

// Watch mode re-runs the preview load every interval. A background refresh that
// arms previewLoading puts the 10 FPS spinner loop back on for most of every
// interval, which is the steady-state half of issue #646.
func TestSilentRefreshDoesNotArmPreviewLoading(t *testing.T) {
	got, cmd := runResourceTypesRefresh(t, true)

	require.NotNil(t, cmd, "test setup: the selection must dispatch a preview load")
	assert.False(t, got.previewLoading, "a silent watch-tick refresh must not arm the spinner")
}

// A user-driven load still animates.
func TestUserRefreshArmsPreviewLoading(t *testing.T) {
	got, cmd := runResourceTypesRefresh(t, false)

	require.NotNil(t, cmd, "test setup: the selection must dispatch a preview load")
	assert.True(t, got.previewLoading, "a user-driven load must still show the spinner")
}

// runResourceTypesRefresh drives one resource-types refresh with the cursor on
// the cluster dashboard, the page issue #646 reports.
func runResourceTypesRefresh(t *testing.T, silent bool) (Model, tea.Cmd) {
	t.Helper()
	m := newTestModelWithScheduler()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.dashboardAcc = make(map[string]*dashboardAccumulator)
	m.previewLoading = false
	m.scheduler.StopWorkers()

	items := []model.Item{{Name: "Cluster", Extra: "__overview__"}}
	got, cmd := m.updateResourceTypes(resourceTypesMsg{items: items, silent: silent})
	if cmd != nil {
		drainBatch(t, cmd, m.scheduler.Close)
	}
	return got.(Model), cmd
}

// drainBatch runs a fan-out's sub-commands so they Submit, then releases them
// through closeScheduler so no goroutine stays parked on an unresolved future.
func drainBatch(t *testing.T, cmd tea.Cmd, closeScheduler func()) {
	t.Helper()
	batchMsg, ok := cmd().(tea.BatchMsg)
	if !ok {
		return
	}
	var wg sync.WaitGroup
	for _, subCmd := range batchMsg {
		wg.Add(1)
		go func(c tea.Cmd) {
			defer wg.Done()
			_ = c()
		}(subCmd)
	}
	closeScheduler()
	wg.Wait()
}
