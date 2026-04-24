package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- Update: tea.WindowSizeMsg ---

func TestUpdateWindowSizeMsg(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "pod-1"},
			{Name: "pod-2"},
		},
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		execMu:        &sync.Mutex{},
		requestGen:    0,
	}

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	result, cmd := m.Update(msg)
	mdl := result.(Model)
	assert.Equal(t, 120, mdl.width)
	assert.Equal(t, 40, mdl.height)
	assert.Nil(t, cmd)
}

func TestUpdateWindowSizeMsgSmall(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		middleItems:   []model.Item{{Name: "pod-1"}},
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		execMu:        &sync.Mutex{},
	}
	m.setCursor(0)

	msg := tea.WindowSizeMsg{Width: 20, Height: 10}
	result, _ := m.Update(msg)
	mdl := result.(Model)
	assert.Equal(t, 20, mdl.width)
	assert.Equal(t, 10, mdl.height)
}

// --- Update: statusMessageExpiredMsg ---

func TestUpdateStatusMessageExpired(t *testing.T) {
	m := Model{
		nav:              model.NavigationState{Level: model.LevelResources},
		tabs:             []TabState{{}},
		selectedItems:    make(map[string]bool),
		cursorMemory:     make(map[string]int),
		itemCache:        make(map[string][]model.Item),
		statusMessage:    "some message",
		statusMessageErr: false,
		width:            80,
		height:           40,
		execMu:           &sync.Mutex{},
	}

	result, _ := m.Update(statusMessageExpiredMsg{})
	mdl := result.(Model)
	assert.Empty(t, mdl.statusMessage)
}

// TestUpdateStatusMessageExpired_StaleTickPreservesNewerMessage asserts
// that a stale statusMessageExpiredMsg arriving while a newer (still
// unexpired) message is active does NOT clobber the newer message. This
// scenario happens when action A calls setStatusMessage+scheduleStatusClear
// and action B sets a new message before A's tick fires.
func TestUpdateStatusMessageExpired_StaleTickPreservesNewerMessage(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		width:         80,
		height:        40,
		execMu:        &sync.Mutex{},
	}
	// Simulate action B: setStatusMessage pushes expiration into the future.
	m.setStatusMessage("Removed bookmark: foo", false)

	// A stale tick from a prior scheduleStatusClear arrives now.
	result, _ := m.Update(statusMessageExpiredMsg{})
	mdl := result.(Model)

	// The newer message must still be present.
	assert.Equal(t, "Removed bookmark: foo", mdl.statusMessage)
	assert.True(t, mdl.hasStatusMessage(), "hasStatusMessage() should still return true")
}

// --- Update: resourceTypesMsg ---

func TestUpdateResourceTypesMsg(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResourceTypes,
			Context: "test",
		},
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		loading:       true,
		width:         80,
		height:        40,
		execMu:        &sync.Mutex{},
	}

	items := []model.Item{
		{Name: "Pods", Category: "Workloads"},
		{Name: "Deployments", Category: "Workloads"},
	}
	result, _ := m.Update(resourceTypesMsg{items: items})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Len(t, mdl.middleItems, 2)
}

func TestUpdateResourceTypesMsgAtClusterLevel(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level: model.LevelClusters,
		},
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		loading:       true,
		width:         80,
		height:        40,
		execMu:        &sync.Mutex{},
	}

	items := []model.Item{
		{Name: "Pods"},
		{Name: "Services"},
	}
	result, cmd := m.Update(resourceTypesMsg{items: items})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Len(t, mdl.rightItems, 2) // goes to rightItems at cluster level
	assert.Nil(t, cmd)
}

// At LevelClusters, a seeded resourceTypesMsg (uncached preview of the
// hovered context) must still populate the right pane so the user sees
// "Pods, Deployments, ..." instead of "No resource types found".
func TestUpdateResourceTypesMsg_AtClusterLevelShowsSeededItems(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelClusters},
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		width:         80,
		height:        40,
		execMu:        &sync.Mutex{},
	}
	items := []model.Item{{Name: "Pods"}, {Name: "Services"}}
	result, _ := m.Update(resourceTypesMsg{items: items, seeded: true})
	mdl := result.(Model)
	assert.Len(t, mdl.rightItems, 2,
		"right-pane preview must show seeded items at LevelClusters so the "+
			"user isn't greeted with 'No resource types found'")
}

// At LevelResourceTypes, a seeded resourceTypesMsg arriving while discovery
// is still in flight (m.loading=true) must NOT overwrite middleItems or
// clear the loader — otherwise every watch-tick flashes basic resource
// types and the discovery loader keeps disappearing (regression guard for
// TODO 866).
func TestUpdateResourceTypesMsg_AtResourceTypesSeededPreservesLoading(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResourceTypes},
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		loading:       true,
		width:         80,
		height:        40,
		execMu:        &sync.Mutex{},
	}
	seeded := []model.Item{{Name: "Pods"}, {Name: "Deployments"}}
	result, _ := m.Update(resourceTypesMsg{items: seeded, seeded: true})
	mdl := result.(Model)
	assert.True(t, mdl.loading,
		"seeded msg during discovery must preserve the middle-pane loader")
	assert.Empty(t, mdl.middleItems,
		"seeded msg during discovery must not clobber middleItems")
}

// Same level but real discovered items (seeded=false) — these come from
// apiResourceDiscoveryMsg's successful path and must update the middle pane
// and clear the loader.
func TestUpdateResourceTypesMsg_AtResourceTypesRealItemsUpdate(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResourceTypes},
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		loading:       true,
		width:         80,
		height:        40,
		execMu:        &sync.Mutex{},
	}
	items := []model.Item{{Name: "Pods"}, {Name: "ConfigMaps"}, {Name: "MyCRDs"}}
	result, _ := m.Update(resourceTypesMsg{items: items, seeded: false})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Len(t, mdl.middleItems, 3)
}

// --- Update: startupTipMsg ---

func TestUpdateStartupTipMsg(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		width:         80,
		height:        40,
		execMu:        &sync.Mutex{},
	}

	result, cmd := m.Update(startupTipMsg{tip: "Press ? for help"})
	mdl := result.(Model)
	assert.Contains(t, mdl.statusMessage, "Press ? for help")
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// --- Update: actionResultMsg ---

func TestUpdateActionResultSuccess(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		width:         80,
		height:        40,
		execMu:        &sync.Mutex{},
	}

	result, cmd := m.Update(actionResultMsg{message: "Resource deleted"})
	mdl := result.(Model)
	assert.Equal(t, "Resource deleted", mdl.statusMessage)
	assert.NotNil(t, cmd)
}

func TestUpdateActionResultError(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		width:         80,
		height:        40,
		execMu:        &sync.Mutex{},
	}

	result, _ := m.Update(actionResultMsg{err: assert.AnError})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
}

func TestPush2UpdateWindowSizeMsg(t *testing.T) {
	m := basePush80v2Model()
	result, cmd := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	rm := result.(Model)
	assert.Equal(t, 200, rm.width)
	assert.Equal(t, 60, rm.height)
	assert.Nil(t, cmd)
}

func TestPush2UpdateWindowSizeSmall(t *testing.T) {
	m := basePush80v2Model()
	result, cmd := m.Update(tea.WindowSizeMsg{Width: 20, Height: 10})
	rm := result.(Model)
	assert.Equal(t, 20, rm.width)
	assert.Nil(t, cmd)
}

func TestPush2UpdateSpinnerTickMsg(t *testing.T) {
	m := basePush80v2Model()
	m.spinner = spinner.New()
	result, _ := m.Update(m.spinner.Tick())
	_ = result.(Model)
}

func TestPush2UpdateContextsLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	m.loading = true
	msg := contextsLoadedMsg{
		items: []model.Item{
			{Name: "ctx-1"},
			{Name: "ctx-2"},
		},
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Len(t, rm.middleItems, 2)
}

func TestPush2UpdateContextsLoadedMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := contextsLoadedMsg{err: fmt.Errorf("fail")}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.NotNil(t, rm.err)
	assert.NotNil(t, cmd)
}

func TestPush2UpdateContextsLoadedMsgCanceled(t *testing.T) {
	m := basePush80v2Model()
	msg := contextsLoadedMsg{err: context.Canceled}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.Nil(t, rm.err)
	assert.Nil(t, cmd)
}

func TestPush2UpdateResourceTypesMsg(t *testing.T) {
	m := basePush80v2Model()
	m.nav.Level = model.LevelResourceTypes
	msg := resourceTypesMsg{
		items: []model.Item{{Name: "Pods"}, {Name: "Services"}},
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Len(t, rm.middleItems, 2)
}

func TestPush2UpdateResourcesLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	m.requestGen = 5
	msg := resourcesLoadedMsg{
		items: []model.Item{{Name: "pod-new"}},
		gen:   5,
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Len(t, rm.middleItems, 1)
}

func TestPush2UpdateResourcesLoadedMsgStalegen(t *testing.T) {
	m := basePush80v2Model()
	m.requestGen = 5
	msg := resourcesLoadedMsg{
		items: []model.Item{{Name: "pod-new"}},
		gen:   3, // stale
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	// Items should not change for stale gen.
	assert.Len(t, rm.middleItems, 3)
}

func TestPush2UpdateResourcesLoadedMsgErr(t *testing.T) {
	m := basePush80v2Model()
	m.requestGen = 5
	msg := resourcesLoadedMsg{
		err: fmt.Errorf("fail"),
		gen: 5,
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
}

func TestPush2UpdateResourcesLoadedMsgCanceled(t *testing.T) {
	m := basePush80v2Model()
	m.requestGen = 5
	msg := resourcesLoadedMsg{
		err: context.Canceled,
		gen: 5,
	}
	result, cmd := m.Update(msg)
	_ = result
	assert.Nil(t, cmd)
}

func TestPush2UpdateResourcesLoadedMsgForPreview(t *testing.T) {
	m := basePush80v2Model()
	m.requestGen = 5
	msg := resourcesLoadedMsg{
		items:      []model.Item{{Name: "preview-item"}},
		gen:        5,
		forPreview: true,
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Len(t, rm.rightItems, 1)
}

func TestPush2UpdateOwnedLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	m.nav.Level = model.LevelOwned
	m.requestGen = 5
	msg := ownedLoadedMsg{
		items: []model.Item{{Name: "rs-1", Kind: "ReplicaSet"}},
		gen:   5,
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Len(t, rm.middleItems, 1)
}

func TestPush2UpdateOwnedLoadedMsgForPreview(t *testing.T) {
	m := basePush80v2Model()
	m.requestGen = 5
	msg := ownedLoadedMsg{
		items:      []model.Item{{Name: "owned-prev"}},
		gen:        5,
		forPreview: true,
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Len(t, rm.rightItems, 1)
}

func TestPush2UpdateContainersLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	m.nav.Level = model.LevelContainers
	m.requestGen = 5
	msg := containersLoadedMsg{
		items: []model.Item{{Name: "container-1", Kind: "Container"}},
		gen:   5,
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Len(t, rm.middleItems, 1)
}

func TestPush2UpdateContainersLoadedMsgForPreview(t *testing.T) {
	m := basePush80v2Model()
	m.requestGen = 5
	msg := containersLoadedMsg{
		items:      []model.Item{{Name: "container-1"}},
		gen:        5,
		forPreview: true,
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Len(t, rm.rightItems, 1)
}

func TestPush2UpdateYamlLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	m.mode = modeYAML
	msg := yamlLoadedMsg{content: "apiVersion: v1\nkind: Pod\n"}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Contains(t, rm.yamlContent, "apiVersion")
}

func TestPush2UpdateYamlLoadedMsgErr(t *testing.T) {
	m := basePush80v2Model()
	m.mode = modeYAML
	msg := yamlLoadedMsg{err: fmt.Errorf("fail")}
	result, _ := m.Update(msg)
	rm := result.(Model)
	// Exercises the error branch of yamlLoadedMsg handling.
	_ = rm
}

func TestPush2UpdateNamespacesLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	m.overlay = overlayNamespace
	msg := namespacesLoadedMsg{
		items: []model.Item{{Name: "default"}, {Name: "kube-system"}, {Name: "prod"}},
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.NotEmpty(t, rm.overlayItems)
}

func TestPush2UpdateNamespacesLoadedMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := namespacesLoadedMsg{err: fmt.Errorf("fail")}
	result, _ := m.Update(msg)
	rm := result.(Model)
	_ = rm
}

func TestPush2UpdateWatchTickMsg(t *testing.T) {
	m := basePush80v2Model()
	m.watchMode = true
	result, cmd := m.Update(watchTickMsg{})
	rm := result.(Model)
	_ = rm
	assert.NotNil(t, cmd)
}

func TestPush2UpdateWatchTickMsgNotActive(t *testing.T) {
	m := basePush80v2Model()
	m.watchMode = false
	result, cmd := m.Update(watchTickMsg{})
	_ = result
	assert.Nil(t, cmd)
}

func TestPush2UpdateAPIResourceDiscoveryMsg(t *testing.T) {
	m := basePush80v2Model()
	m.nav.Level = model.LevelResourceTypes
	msg := apiResourceDiscoveryMsg{
		context: "test-ctx",
		entries: []model.ResourceTypeEntry{
			{Kind: "MyCRD", Resource: "mycrds"},
		},
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.NotEmpty(t, rm.discoveredResources["test-ctx"])
}

func TestPush2UpdateAPIResourceDiscoveryMsgDiffContext(t *testing.T) {
	m := basePush80v2Model()
	msg := apiResourceDiscoveryMsg{
		context: "other-ctx",
		entries: []model.ResourceTypeEntry{{Kind: "X"}},
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.NotEmpty(t, rm.discoveredResources["other-ctx"])
}

func TestPush2UpdateActionResultMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := actionResultMsg{message: "Deleted pod-1"}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Deleted")
	assert.NotNil(t, cmd)
}

func TestPush2UpdateActionResultMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := actionResultMsg{err: fmt.Errorf("fail")}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush2UpdateBulkActionResultMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := bulkActionResultMsg{succeeded: 3, failed: 1, errors: []string{"pod-4: not found"}}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "3")
	assert.NotNil(t, cmd)
}

func TestPush2UpdateBulkActionResultMsgNoErrors(t *testing.T) {
	m := basePush80v2Model()
	msg := bulkActionResultMsg{succeeded: 5}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "5")
	assert.NotNil(t, cmd)
}

func TestPush2UpdateDashboardLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := dashboardLoadedMsg{content: "dashboard content", events: "events", context: "test-ctx"}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Equal(t, "dashboard content", rm.dashboardPreview)
	assert.Equal(t, "events", rm.dashboardEventsPreview)
}

func TestPush2UpdateDashboardLoadedMsgWrongContext(t *testing.T) {
	m := basePush80v2Model()
	msg := dashboardLoadedMsg{content: "data", context: "other-ctx"}
	result, _ := m.Update(msg)
	rm := result.(Model)
	// Should be ignored since context doesn't match.
	assert.Empty(t, rm.dashboardPreview)
}

func TestPush2UpdateMonitoringDashboardMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := monitoringDashboardMsg{content: "monitoring data", context: "test-ctx"}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Equal(t, "monitoring data", rm.monitoringPreview)
}

func TestPush2UpdateMonitoringDashboardMsgWrongContext(t *testing.T) {
	m := basePush80v2Model()
	msg := monitoringDashboardMsg{content: "data", context: "other"}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Empty(t, rm.monitoringPreview)
}

func TestPush2UpdateStartupTipMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := startupTipMsg{tip: "Press ? for help"}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Press ? for help")
	assert.NotNil(t, cmd)
}

func TestPush2UpdateExportDoneMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := exportDoneMsg{path: "/tmp/test.yaml"}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "/tmp/test.yaml")
	assert.NotNil(t, cmd)
}

func TestPush2UpdateExportDoneMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := exportDoneMsg{err: fmt.Errorf("write failed")}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush2UpdateYamlClipboardMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := yamlClipboardMsg{content: "apiVersion: v1"}
	result, cmd := m.Update(msg)
	_ = result.(Model)
	assert.NotNil(t, cmd)
}

func TestPush2UpdateYamlClipboardMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := yamlClipboardMsg{err: fmt.Errorf("fail")}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush2UpdateMetricsLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	m.requestGen = 5
	msg := metricsLoadedMsg{cpuUsed: 100, memUsed: 512, gen: 5}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.NotEmpty(t, rm.metricsContent)
}

func TestPush2UpdateMetricsLoadedMsgStaleGen(t *testing.T) {
	m := basePush80v2Model()
	m.requestGen = 5
	msg := metricsLoadedMsg{cpuUsed: 100, gen: 3}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Empty(t, rm.metricsContent)
}

func TestPush2UpdateDescribeRefreshTickMsg(t *testing.T) {
	m := basePush80v2Model()
	m.mode = modeDescribe
	result, _ := m.Update(describeRefreshTickMsg{})
	_ = result
}

func TestPush2UpdateDescribeRefreshTickMsgNotDescribe(t *testing.T) {
	m := basePush80v2Model()
	m.mode = modeExplorer
	result, cmd := m.Update(describeRefreshTickMsg{})
	_ = result
	assert.Nil(t, cmd)
}

func TestPush2UpdateSecretDataLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	m.overlay = overlaySecretEditor
	msg := secretDataLoadedMsg{data: &model.SecretData{Data: map[string]string{"key": "val"}}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.NotNil(t, rm.secretData)
}

func TestPush2UpdateSecretSavedMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := secretSavedMsg{}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Secret")
	assert.NotNil(t, cmd)
}

func TestPush2UpdateSecretSavedMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := secretSavedMsg{err: fmt.Errorf("fail")}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush2UpdateLabelDataLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	m.overlay = overlayLabelEditor
	msg := labelDataLoadedMsg{data: &model.LabelAnnotationData{
		Labels:      map[string]string{"a": "b"},
		Annotations: map[string]string{"c": "d"},
	}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.NotNil(t, rm.labelData)
}

func TestPush2UpdateLabelSavedMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := labelSavedMsg{}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "saved")
	assert.NotNil(t, cmd)
}

func TestPush2UpdateLabelSavedMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := labelSavedMsg{err: fmt.Errorf("fail")}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush2UpdateConfigMapDataLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := configMapDataLoadedMsg{data: &model.ConfigMapData{Data: map[string]string{"key": "val"}}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.NotNil(t, rm.configMapData)
}

func TestPush2UpdateConfigMapSavedMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := configMapSavedMsg{}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "ConfigMap")
	assert.NotNil(t, cmd)
}

func TestPush2UpdateRevisionListMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := revisionListMsg{revisions: []k8s.DeploymentRevision{
		{Revision: 1, Name: "deploy-1-abc"},
	}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.NotEqual(t, overlayNone, rm.overlay)
}

func TestPush2UpdateRevisionListMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := revisionListMsg{err: fmt.Errorf("fail")}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush2UpdateHelmRevisionListMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := helmRevisionListMsg{revisions: []ui.HelmRevision{
		{Revision: 1, Status: "deployed"},
	}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.NotEqual(t, overlayNone, rm.overlay)
}

func TestPush2UpdateHelmRevisionListMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := helmRevisionListMsg{err: fmt.Errorf("fail")}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush2UpdateResourceTreeLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	m.requestGen = 5
	msg := resourceTreeLoadedMsg{
		tree: &model.ResourceNode{Name: "deploy-1", Kind: "Deployment"},
		gen:  5,
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.NotNil(t, rm.resourceTree)
}

func TestPush2UpdateQuotaLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := quotaLoadedMsg{quotas: []k8s.QuotaInfo{
		{Name: "q1", Namespace: "default", Resources: []k8s.QuotaResource{
			{Name: "pods", Hard: "10", Used: "5", Percent: 50},
		}},
	}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.NotEqual(t, overlayNone, rm.overlay)
}

func TestPush2UpdateQuotaLoadedMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := quotaLoadedMsg{err: fmt.Errorf("fail")}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush2UpdateCommandBarResultMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := commandBarResultMsg{output: "output here"}
	result, _ := m.Update(msg)
	_ = result.(Model)
	// Exercises the success path of commandBarResultMsg.
}

func TestPush2UpdateCommandBarResultMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := commandBarResultMsg{output: "error output", err: fmt.Errorf("fail")}
	result, _ := m.Update(msg)
	_ = result.(Model)
	// Exercises the error path of commandBarResultMsg handling.
}

func TestPush2UpdatePodSelectMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := podSelectMsg{items: []model.Item{
		{Name: "pod-a"},
		{Name: "pod-b"},
	}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	_ = rm // exercises the branch
}

func TestPush2UpdatePodSelectMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := podSelectMsg{err: fmt.Errorf("fail")}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush2UpdateContainerSelectMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := containerSelectMsg{items: []model.Item{
		{Name: "container-a"},
	}}
	result, _ := m.Update(msg)
	rm := result.(Model)
	_ = rm
}

func TestPush2UpdateContainerSelectMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := containerSelectMsg{err: fmt.Errorf("fail")}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush2UpdateDiffLoadedMsg(t *testing.T) {
	m := basePush80v2Model()
	msg := diffLoadedMsg{
		left:      "left content",
		right:     "right content",
		leftName:  "pod-1",
		rightName: "pod-2",
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Equal(t, modeDiff, rm.mode)
}

func TestPush2UpdateDiffLoadedMsgErr(t *testing.T) {
	m := basePush80v2Model()
	msg := diffLoadedMsg{err: fmt.Errorf("fail")}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush3UpdateLogLineMsg(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeLogs
	ch := make(chan string, 1)
	m.logCh = ch
	ch <- "next line"
	msg := logLineMsg{line: "test line", ch: ch}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Contains(t, rm.logLines, "test line")
}

func TestPush3UpdateLogLineMsgDone(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeLogs
	msg := logLineMsg{done: true}
	result, _ := m.Update(msg)
	rm := result.(Model)
	// Done is handled silently — no sentinel marker appended.
	_ = rm
}

func TestPush3UpdatePortForwardUpdateMsg(t *testing.T) {
	m := basePush80v3Model()
	m.portForwardMgr = k8s.NewPortForwardManager()
	msg := portForwardUpdateMsg{}
	result, _ := m.Update(msg)
	_ = result.(Model)
}

func TestPush3UpdatePortForwardUpdateMsgErr(t *testing.T) {
	m := basePush80v3Model()
	m.portForwardMgr = k8s.NewPortForwardManager()
	msg := portForwardUpdateMsg{err: assert.AnError}
	result, _ := m.Update(msg)
	rm := result.(Model)
	// Exercises the error path.
	_ = rm
}

func TestPush3UpdatePreviewEventsLoadedMsg(t *testing.T) {
	m := basePush80v3Model()
	m.requestGen = 5
	msg := previewEventsLoadedMsg{
		events: []k8s.EventInfo{
			{Type: "Normal", Reason: "Scheduled", Message: "pod assigned", Source: "scheduler"},
		},
		gen: 5,
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.NotEmpty(t, rm.previewEventsContent)
}

func TestPush3UpdatePreviewEventsLoadedMsgStaleGen(t *testing.T) {
	m := basePush80v3Model()
	m.requestGen = 5
	msg := previewEventsLoadedMsg{gen: 3}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.Empty(t, rm.previewEventsContent)
}

func TestPush3UpdateRollbackDoneMsg(t *testing.T) {
	m := basePush80v3Model()
	msg := rollbackDoneMsg{}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Rollback")
	assert.NotNil(t, cmd)
}

func TestPush3UpdateRollbackDoneMsgErr(t *testing.T) {
	m := basePush80v3Model()
	msg := rollbackDoneMsg{err: assert.AnError}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush3UpdateHelmRollbackDoneMsg(t *testing.T) {
	m := basePush80v3Model()
	msg := helmRollbackDoneMsg{}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.NotEmpty(t, rm.statusMessage)
	assert.NotNil(t, cmd)
}

func TestPush3UpdateHelmRollbackDoneMsgErr(t *testing.T) {
	m := basePush80v3Model()
	msg := helmRollbackDoneMsg{err: assert.AnError}
	result, cmd := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestP4UpdatePodMetricsEnrichedMsg(t *testing.T) {
	m := bp4()
	m.requestGen = 5
	m.nav.ResourceType.Kind = "Pod"
	msg := podMetricsEnrichedMsg{
		metrics: map[string]model.PodMetrics{
			"pod-1": {CPU: 100, Memory: 256},
		},
		gen: 5,
	}
	result, _ := m.Update(msg)
	_ = result.(Model)
}

func TestP4UpdateNodeMetricsEnrichedMsg(t *testing.T) {
	m := bp4()
	m.requestGen = 5
	m.nav.ResourceType.Kind = "Node"
	msg := nodeMetricsEnrichedMsg{
		metrics: map[string]model.PodMetrics{
			"node-1": {CPU: 1000, Memory: 4096},
		},
		gen: 5,
	}
	result, _ := m.Update(msg)
	_ = result.(Model)
}

func TestP4UpdateFinalizerSearchMsg(t *testing.T) {
	m := bp4()
	msg := finalizerSearchResultMsg{
		results: []k8s.FinalizerMatch{
			{Name: "pod-1", Namespace: "default", Kind: "Pod", Finalizers: []string{"kubernetes"}},
		},
	}
	result, _ := m.Update(msg)
	rm := result.(Model)
	_ = rm
}

func TestP4UpdateFinalizerSearchMsgErr(t *testing.T) {
	m := bp4()
	msg := finalizerSearchResultMsg{err: assert.AnError}
	result, _ := m.Update(msg)
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
}

func TestP4UpdateFinalizerSearchMsgEmpty(t *testing.T) {
	m := bp4()
	msg := finalizerSearchResultMsg{}
	result, _ := m.Update(msg)
	rm := result.(Model)
	_ = rm
}

func TestCovUpdateCommandBarResult(t *testing.T) {
	m := baseModelCov()

	r, _ := m.Update(commandBarResultMsg{output: "pod deleted"})
	assert.Equal(t, modeDescribe, r.(Model).mode)

	r, _ = m.Update(commandBarResultMsg{output: "forbidden", err: assert.AnError})
	assert.Equal(t, modeDescribe, r.(Model).mode)

	r, cmd := m.Update(commandBarResultMsg{err: assert.AnError})
	assert.True(t, r.(Model).statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestCovUpdateContainersLoadedPreview(t *testing.T) {
	m := baseModelCov()
	m.requestGen = 1
	r, cmd := m.Update(containersLoadedMsg{items: []model.Item{{Name: "c1"}}, forPreview: true, gen: 1})
	assert.Len(t, r.(Model).rightItems, 1)
	assert.Nil(t, cmd)
}

func TestCovUpdateContainersLoadedStale(t *testing.T) {
	m := baseModelCov()
	m.requestGen = 2
	_, cmd := m.Update(containersLoadedMsg{gen: 1})
	assert.Nil(t, cmd)
}

func TestCovUpdateNamespacesLoaded(t *testing.T) {
	m := baseModelCov()
	r, _ := m.Update(namespacesLoadedMsg{items: []model.Item{{Name: "default"}, {Name: "kube-system"}}})
	assert.Len(t, r.(Model).overlayItems, 3) // "All Namespaces" + 2

	m.allNamespaces = true
	r, _ = m.Update(namespacesLoadedMsg{items: []model.Item{{Name: "default"}}})
	assert.Equal(t, 0, r.(Model).overlayCursor)
}

func TestCovUpdatePreviewYAMLLoaded(t *testing.T) {
	m := baseModelCov()
	m.requestGen = 5
	r, _ := m.Update(previewYAMLLoadedMsg{content: "apiVersion: v1", gen: 5})
	assert.Contains(t, r.(Model).previewYAML, "apiVersion")

	r, _ = m.Update(previewYAMLLoadedMsg{content: "stale", gen: 3})
	assert.Empty(t, r.(Model).previewYAML)

	r, _ = m.Update(previewYAMLLoadedMsg{err: assert.AnError, gen: 5})
	assert.Empty(t, r.(Model).previewYAML)
}

func TestCovUpdateOwnedLoadedPreview(t *testing.T) {
	m := baseModelCov()
	m.requestGen = 1
	r, cmd := m.Update(ownedLoadedMsg{items: []model.Item{{Name: "pod-1"}}, forPreview: true, gen: 1})
	assert.Len(t, r.(Model).rightItems, 1)
	assert.Nil(t, cmd)

	_, cmd = m.Update(ownedLoadedMsg{gen: 0})
	assert.Nil(t, cmd)
}

func TestCovUpdateResourceTreeLoadedStale(t *testing.T) {
	m := baseModelCov()
	m.requestGen = 5
	_, cmd := m.Update(resourceTreeLoadedMsg{gen: 3})
	assert.Nil(t, cmd)
}

func TestFinal2UpdateResourcesLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	m.requestGen = 1
	items := []model.Item{{Name: "pod-1", Namespace: "default"}}
	result, cmd := m.Update(resourcesLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, items, rm.middleItems)
	assert.NotNil(t, cmd)
}

// TestFinal2UpdateResourcesLoadedMsgPreviewLoadingArmed locks in the fix for
// the navigation flicker: when the main resource list arrives, the right
// pane's previewLoading flag must be set so the spinner keeps showing during
// the gap between the main list arriving and the preview load completing.
// Without this the right pane briefly renders "No resources found".
func TestFinal2UpdateResourcesLoadedMsgPreviewLoadingArmed(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	m.previewLoading = false
	m.requestGen = 1
	items := []model.Item{{Name: "pod-1", Namespace: "default"}}
	result, _ := m.Update(resourcesLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.True(t, rm.previewLoading, "previewLoading must be armed when main list arrives so the right pane keeps showing the spinner")
}

func TestFinal2UpdateResourcesLoadedMsgStale(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 2
	result, cmd := m.Update(resourcesLoadedMsg{items: nil, gen: 1})
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinal2UpdateResourcesLoadedMsgForPreview(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	items := []model.Item{{Name: "child-1"}}
	result, cmd := m.Update(resourcesLoadedMsg{items: items, gen: 1, forPreview: true})
	rm := result.(Model)
	assert.Equal(t, items, rm.rightItems)
	assert.Nil(t, cmd)
}

func TestFinal2UpdateResourcesLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, cmd := m.Update(resourcesLoadedMsg{err: assert.AnError, gen: 1})
	rm := result.(Model)
	assert.NotNil(t, rm.err)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateResourcesLoadedMsgCanceled(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, cmd := m.Update(resourcesLoadedMsg{err: context.Canceled, gen: 1})
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinal2UpdateResourcesLoadedWithPendingTarget(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	m.pendingTarget = "pod-2"
	items := []model.Item{{Name: "pod-1"}, {Name: "pod-2"}, {Name: "pod-3"}}
	result, _ := m.Update(resourcesLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.Empty(t, rm.pendingTarget)
}

func TestFinal2UpdateOwnedLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	items := []model.Item{{Name: "rs-1"}}
	result, cmd := m.Update(ownedLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, items, rm.middleItems)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateOwnedLoadedMsgForPreview(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	items := []model.Item{{Name: "rs-1"}}
	result, cmd := m.Update(ownedLoadedMsg{items: items, gen: 1, forPreview: true})
	rm := result.(Model)
	assert.Equal(t, items, rm.rightItems)
	assert.Nil(t, cmd)
}

func TestFinal2UpdateOwnedLoadedMsgStale(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 2
	result, cmd := m.Update(ownedLoadedMsg{gen: 1})
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinal2UpdateOwnedLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, cmd := m.Update(ownedLoadedMsg{err: assert.AnError, gen: 1})
	rm := result.(Model)
	assert.NotNil(t, rm.err)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateContainersLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	items := []model.Item{{Name: "nginx"}}
	result, cmd := m.Update(containersLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, items, rm.middleItems)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateContainersLoadedMsgForPreview(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	items := []model.Item{{Name: "nginx"}}
	result, cmd := m.Update(containersLoadedMsg{items: items, gen: 1, forPreview: true})
	rm := result.(Model)
	assert.Equal(t, items, rm.rightItems)
	assert.Nil(t, cmd)
}

func TestFinal2UpdateContainersLoadedMsgStale(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 2
	result, cmd := m.Update(containersLoadedMsg{gen: 1})
	assert.Nil(t, cmd)
	_ = result.(Model)
}

// TestUpdateContainersLoadedClearsPreviewLoadingAtLevelContainers locks in
// the fix for the stuck "Loading..." bug when drilling into a pod with
// several containers. At LevelContainers, loadPreview() always returns nil
// (containers have no right-pane preview load of their own), so
// previewLoading — armed by clearRight() on navigation — must be cleared
// here. Without this, the right pane renders "Loading..." forever because
// nothing downstream ever clears the flag.
func TestUpdateContainersLoadedClearsPreviewLoadingAtLevelContainers(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelContainers
	m.nav.ResourceName = "pod-1"
	m.nav.OwnedName = "pod-1"
	m.requestGen = 1
	m.previewLoading = true
	m.loading = true

	items := []model.Item{
		{Name: "app", Kind: "Container"},
		{Name: "sidecar", Kind: "Container"},
	}
	result, _ := m.Update(containersLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.False(t, rm.loading, "loading must be cleared after containers load")
	assert.False(t, rm.previewLoading, "previewLoading must be cleared at LevelContainers because loadPreview returns nil there")
	assert.Equal(t, items, rm.middleItems)
}

// TestUpdateYamlLoadedErrorClearsLoadingPlaceholder locks in the fix for
// issue #34: pressing Enter on a resource kicks off the full-screen YAML
// view with m.yamlContent="Loading..." as an initial placeholder.
// updateYamlLoaded previously bailed out on any error (including
// context.Canceled) without touching yamlContent, so the viewer stayed
// stuck on "Loading..." forever. The status-bar error flashed briefly
// and was lost. The fix: replace the placeholder with an error message
// so the user sees what happened instead of an eternal loader.
func TestUpdateYamlLoadedErrorClearsLoadingPlaceholder(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeYAML
	m.yamlContent = "Loading..."
	result, _ := m.Update(yamlLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.NotEqual(t, "Loading...", rm.yamlContent, "yamlContent must not stay on the Loading placeholder when the fetch errors")
	assert.Contains(t, rm.yamlContent, assert.AnError.Error(), "yamlContent should surface the error message so users can see what went wrong")
}

// TestUpdateYamlLoadedContextCanceledClearsPlaceholder covers the
// context.Canceled branch specifically — a cancellation that races the
// viewer transition must still clear the "Loading..." placeholder,
// otherwise every mid-load navigation leaves a stuck YAML view behind.
func TestUpdateYamlLoadedContextCanceledClearsPlaceholder(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeYAML
	m.yamlContent = "Loading..."
	result, _ := m.Update(yamlLoadedMsg{err: context.Canceled})
	rm := result.(Model)
	assert.NotEqual(t, "Loading...", rm.yamlContent, "yamlContent must not stay on the Loading placeholder when the request is canceled")
}

func TestFinal2UpdateNamespacesLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	m.allNamespaces = true
	items := []model.Item{{Name: "default"}, {Name: "kube-system"}}
	result, cmd := m.Update(namespacesLoadedMsg{items: items})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, 3, len(rm.overlayItems)) // "All Namespaces" + 2 items
	assert.Nil(t, cmd)
}

func TestFinal2UpdateNamespacesLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(namespacesLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.NotNil(t, rm.err)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateYAMLLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, _ := m.Update(yamlLoadedMsg{content: "apiVersion: v1\nkind: Pod"})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.NotEmpty(t, rm.yamlContent)
}

func TestFinal2UpdateYAMLLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(yamlLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.NotNil(t, rm.err)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdatePreviewYAMLLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, _ := m.Update(previewYAMLLoadedMsg{content: "key: value", gen: 1})
	rm := result.(Model)
	assert.NotEmpty(t, rm.previewYAML)
}

func TestFinal2UpdatePreviewYAMLLoadedMsgStale(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 2
	result, cmd := m.Update(previewYAMLLoadedMsg{content: "key: value", gen: 1})
	rm := result.(Model)
	assert.Empty(t, rm.previewYAML)
	assert.Nil(t, cmd)
}

func TestFinal2UpdatePreviewYAMLLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, _ := m.Update(previewYAMLLoadedMsg{err: assert.AnError, gen: 1})
	rm := result.(Model)
	assert.Empty(t, rm.previewYAML)
}

func TestFinal2UpdateContainerPortsLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(containerPortsLoadedMsg{
		ports: []k8s.ContainerPort{{ContainerPort: 8080, Name: "http", Protocol: "TCP"}},
	})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, overlayPortForward, rm.overlay)
	assert.Equal(t, 1, len(rm.pfAvailablePorts))
	assert.Nil(t, cmd)
}

func TestFinal2UpdateContainerPortsLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(containerPortsLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, overlayPortForward, rm.overlay)
	assert.Nil(t, rm.pfAvailablePorts)
	assert.Nil(t, cmd)
}

func TestFinal2UpdateContainerPortsLoadedMsgEmpty(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(containerPortsLoadedMsg{ports: nil})
	rm := result.(Model)
	assert.Equal(t, overlayPortForward, rm.overlay)
	assert.Equal(t, -1, rm.pfPortCursor)
	assert.Nil(t, cmd)
}

func TestFinal2UpdateResourceTreeLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	tree := &model.ResourceNode{Name: "root"}
	result, _ := m.Update(resourceTreeLoadedMsg{tree: tree, gen: 1})
	rm := result.(Model)
	assert.Equal(t, tree, rm.resourceTree)
}

func TestFinal2UpdateResourceTreeLoadedMsgStale(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 2
	result, cmd := m.Update(resourceTreeLoadedMsg{gen: 1})
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinal2UpdateResourceTreeLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, cmd := m.Update(resourceTreeLoadedMsg{err: assert.AnError, gen: 1})
	_ = result.(Model)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateResourcesLoadedWarningEventsOnly(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	m.warningEventsOnly = true
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Event", Resource: "events"}
	items := []model.Item{
		{Name: "evt-1", Status: "Warning"},
		{Name: "evt-2", Status: "Normal"},
		{Name: "evt-3", Status: "Warning"},
	}
	result, _ := m.Update(resourcesLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.Equal(t, 2, len(rm.middleItems))
}

func TestFinal2UpdateResourcesLoadedPreviewWarningFilter(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	m.warningEventsOnly = true
	items := []model.Item{
		{Name: "evt-1", Kind: "Event", Status: "Warning"},
		{Name: "evt-2", Kind: "Event", Status: "Normal"},
	}
	result, _ := m.Update(resourcesLoadedMsg{items: items, gen: 1, forPreview: true})
	rm := result.(Model)
	assert.Equal(t, 1, len(rm.rightItems))
}

func TestFinal2UpdateResourcesLoadedMultiNSFilter(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	m.selectedNamespaces = map[string]bool{"ns1": true, "ns2": true}
	items := []model.Item{
		{Name: "pod-1", Namespace: "ns1"},
		{Name: "pod-2", Namespace: "ns2"},
		{Name: "pod-3", Namespace: "ns3"},
	}
	result, _ := m.Update(resourcesLoadedMsg{items: items, gen: 1})
	rm := result.(Model)
	assert.Equal(t, 2, len(rm.middleItems))
}

func TestFinal2UpdateExplainRecursiveMsg(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeExplain
	matches := []model.ExplainField{{Name: "containers", Type: "[]Container"}}
	result, _ := m.Update(explainRecursiveMsg{matches: matches, query: "container"})
	rm := result.(Model)
	assert.Equal(t, 1, len(rm.explainRecursiveResults))
}

func TestFinal2UpdateExplainRecursiveMsgError(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeExplain
	result, cmd := m.Update(explainRecursiveMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateContainerSelectMsg(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	items := []model.Item{{Name: "nginx"}, {Name: "sidecar"}}
	result, _ := m.Update(containerSelectMsg{items: items})
	rm := result.(Model)
	assert.Equal(t, overlayContainerSelect, rm.overlay)
}

func TestFinal2UpdateContainerSelectMsgSingle(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	items := []model.Item{{Name: "nginx"}}
	result, _ := m.Update(containerSelectMsg{items: items})
	rm := result.(Model)
	assert.Equal(t, "nginx", rm.actionCtx.containerName)
}

func TestFinal2UpdateContainerSelectMsgError(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	result, cmd := m.Update(containerSelectMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.Empty(t, rm.pendingAction)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdatePodSelectMsg(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	items := []model.Item{{Name: "pod-1", Kind: "Pod"}, {Name: "pod-2", Kind: "Pod"}}
	result, _ := m.Update(podSelectMsg{items: items})
	rm := result.(Model)
	assert.Equal(t, overlayPodSelect, rm.overlay)
}

func TestFinal2UpdatePodSelectMsgSingle(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	items := []model.Item{{Name: "pod-1", Kind: "Pod"}}
	result, cmd := m.Update(podSelectMsg{items: items})
	rm := result.(Model)
	assert.Equal(t, "pod-1", rm.actionCtx.name)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdatePodSelectMsgNoPods(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	items := []model.Item{{Name: "not-a-pod", Kind: "Service"}}
	result, cmd := m.Update(podSelectMsg{items: items})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "No pods")
	assert.NotNil(t, cmd)
}

func TestFinal2UpdatePodSelectMsgError(t *testing.T) {
	m := baseFinalModel()
	m.pendingAction = "Exec"
	result, cmd := m.Update(podSelectMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal2UpdateCRDDiscoveryMsgDeeperLevel(t *testing.T) {
	m := baseFinalModel()
	m.nav.Context = "test-ctx"
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1"}
	entries := []model.ResourceTypeEntry{
		{Kind: "Pod", Resource: "pods", APIVersion: "v1"},
		{Kind: "CustomResource", Resource: "crs", APIGroup: "example.com", APIVersion: "v1"},
	}
	result, _ := m.Update(apiResourceDiscoveryMsg{context: "test-ctx", entries: entries})
	rm := result.(Model)
	assert.Contains(t, rm.discoveredResources, "test-ctx")
}

func TestFinal2UpdateSpinnerTick(t *testing.T) {
	m := baseFinalModel()
	m.spinner = spinner.New()
	// Send a spinner tick message directly.
	result, _ := m.Update(spinner.TickMsg{})
	_ = result.(Model)
}

func TestFinal3UpdateContextsLoadedWithSession(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	m.pendingSession = &SessionState{Context: "test-ctx", Namespace: "default"}
	m.sessionRestored = false
	items := []model.Item{{Name: "test-ctx"}}
	result, cmd := m.Update(contextsLoadedMsg{items: items})
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.sessionRestored)
}

func TestFinal3UpdateYAMLClipboardMsg(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(yamlClipboardMsg{})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "YAML copied")
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateYAMLClipboardMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(yamlClipboardMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateSecretDataLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(secretDataLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateConfigMapDataLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(configMapDataLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateLabelDataLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(labelDataLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateCanILoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(canILoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateAlertsLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(alertsLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateQuotaLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(quotaLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateNetpolLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(netpolLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateLogContainersLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	result, _ := m.Update(logContainersLoadedMsg{err: assert.AnError})
	_ = result.(Model)
}

func TestFinal3UpdatePreviewEventsLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, _ := m.Update(previewEventsLoadedMsg{gen: 1})
	_ = result.(Model)
}

func TestFinal3UpdatePreviewEventsLoadedMsgStale(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 2
	result, cmd := m.Update(previewEventsLoadedMsg{gen: 1})
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestPush4UpdateWatchTickMsg(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	m.watchMode = true
	result, _ := m.Update(watchTickMsg{})
	_ = result.(Model)
}

func TestPush4UpdateWatchTickMsgNotWatching(t *testing.T) {
	m := basePush4Model()
	m.watchMode = false
	result, _ := m.Update(watchTickMsg{})
	_ = result.(Model)
}

func TestPush4UpdateSecretSavedMsg(t *testing.T) {
	m := basePush4Model()
	result, cmd := m.Update(secretSavedMsg{})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "saved")
	assert.NotNil(t, cmd)
}

func TestPush4UpdateSecretSavedMsgError(t *testing.T) {
	m := basePush4Model()
	result, cmd := m.Update(secretSavedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush4UpdateConfigMapSavedMsg(t *testing.T) {
	m := basePush4Model()
	result, cmd := m.Update(configMapSavedMsg{})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "saved")
	assert.NotNil(t, cmd)
}

func TestPush4UpdateConfigMapSavedMsgError(t *testing.T) {
	m := basePush4Model()
	result, cmd := m.Update(configMapSavedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestPush4UpdateLabelSavedMsg(t *testing.T) {
	m := basePush4Model()
	result, cmd := m.Update(labelSavedMsg{})
	rm := result.(Model)
	_ = rm
	assert.NotNil(t, cmd)
}

func TestPush4UpdateLabelSavedMsgError(t *testing.T) {
	m := basePush4Model()
	result, cmd := m.Update(labelSavedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinalUpdateWindowSizeMsg(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	rm := result.(Model)
	assert.Equal(t, 200, rm.width)
	assert.Equal(t, 50, rm.height)
	assert.Nil(t, cmd)
}

func TestFinalUpdateResourceTypesMsg(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelClusters
	items := []model.Item{{Name: "Pods"}, {Name: "Deployments"}}
	result, _ := m.Update(resourceTypesMsg{items: items})
	rm := result.(Model)
	assert.Equal(t, items, rm.rightItems)
}

func TestFinalUpdateResourceTypesMsgAtResourceTypes(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	items := []model.Item{{Name: "Pods"}}
	result, _ := m.Update(resourceTypesMsg{items: items})
	rm := result.(Model)
	assert.Equal(t, items, rm.middleItems)
}

func TestFinalUpdateContextsLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	items := []model.Item{{Name: "cluster-1"}}
	result, cmd := m.Update(contextsLoadedMsg{items: items})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, items, rm.middleItems)
	assert.NotNil(t, cmd)
}

func TestFinalUpdateContextsLoadedMsgWithError(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, _ := m.Update(contextsLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.NotNil(t, rm.err)
}

func TestFinalUpdateContextsLoadedMsgCanceled(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, _ := m.Update(contextsLoadedMsg{err: context.Canceled})
	rm := result.(Model)
	assert.Nil(t, rm.err)
}

func TestFinalUpdateCRDDiscoveryMsg(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	entries := []model.ResourceTypeEntry{{Kind: "CustomResource", Resource: "crs"}}
	result, _ := m.Update(apiResourceDiscoveryMsg{context: "test-ctx", entries: entries})
	rm := result.(Model)
	assert.Contains(t, rm.discoveredResources, "test-ctx")
}

func TestFinalUpdateCRDDiscoveryMsgError(t *testing.T) {
	m := baseFinalModel()
	result, _ := m.Update(apiResourceDiscoveryMsg{context: "test-ctx", err: assert.AnError})
	_ = result.(Model)
}

func TestFinalUpdateCRDDiscoveryMsgCanceled(t *testing.T) {
	m := baseFinalModel()
	result, _ := m.Update(apiResourceDiscoveryMsg{err: context.Canceled})
	_ = result.(Model)
}

func TestFinalUpdateDashboardLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.nav.Context = "test-ctx"
	result, _ := m.Update(dashboardLoadedMsg{content: "dashboard content", context: "test-ctx"})
	rm := result.(Model)
	assert.Equal(t, "dashboard content", rm.dashboardPreview)
}

func TestFinalUpdateDashboardLoadedMsgDifferentCtx(t *testing.T) {
	m := baseFinalModel()
	m.nav.Context = "test-ctx"
	result, _ := m.Update(dashboardLoadedMsg{content: "other", context: "other-ctx"})
	rm := result.(Model)
	assert.NotEqual(t, "other", rm.dashboardPreview)
}

func TestFinalUpdateMonitoringDashboardMsg(t *testing.T) {
	m := baseFinalModel()
	m.nav.Context = "test-ctx"
	result, _ := m.Update(monitoringDashboardMsg{content: "monitoring content", context: "test-ctx"})
	rm := result.(Model)
	assert.Equal(t, "monitoring content", rm.monitoringPreview)
}

func TestFinalUpdateActionResultMsgSuccess(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(actionResultMsg{message: "Done"})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Contains(t, rm.statusMessage, "Done")
	assert.NotNil(t, cmd)
}

func TestFinalUpdateActionResultMsgError(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, _ := m.Update(actionResultMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.loading)
}

func TestFinalUpdateDescribeLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeDescribe
	result, _ := m.Update(describeLoadedMsg{content: "desc content", title: "Title"})
	rm := result.(Model)
	assert.Equal(t, "desc content", rm.describeContent)
	assert.Equal(t, "Title", rm.describeTitle)
}

func TestFinalUpdateDescribeLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeDescribe
	result, cmd := m.Update(describeLoadedMsg{err: assert.AnError, title: "Title"})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinalUpdateLogLineMsg(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	ch := make(chan string, 1)
	m.logCh = ch
	m.logFollow = true
	result, cmd := m.Update(logLineMsg{line: "log line", ch: ch})
	rm := result.(Model)
	assert.Contains(t, rm.logLines, "log line")
	assert.NotNil(t, cmd)
}

func TestFinalUpdateLogLineMsgDone(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	ch := make(chan string)
	m.logCh = ch
	result, _ := m.Update(logLineMsg{done: true, ch: ch})
	_ = result.(Model)
}

func TestFinalUpdateLogLineMsgStaleChannel(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	ch1 := make(chan string, 1)
	ch2 := make(chan string, 1)
	m.logCh = ch2 // current channel
	result, _ := m.Update(logLineMsg{line: "stale", ch: ch1})
	rm := result.(Model)
	// Should discard the stale line.
	assert.NotContains(t, rm.logLines, "stale")
}

func TestFinalUpdateTriggerCronJobMsg(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(triggerCronJobMsg{jobName: "manual-trigger-1"})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.NotNil(t, cmd)
}

func TestFinalUpdateTriggerCronJobMsgError(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, _ := m.Update(triggerCronJobMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.loading)
}

func TestFinalUpdateRollbackDoneMsg(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(rollbackDoneMsg{})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Rollback successful")
	assert.NotNil(t, cmd)
}

func TestFinalUpdateRollbackDoneMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(rollbackDoneMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinalUpdateLogHistoryMsg(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	m.logLoadingHistory = true
	m.logLines = []string{"existing"}
	result, _ := m.Update(logHistoryMsg{
		lines:     []string{"older1", "older2", "existing"},
		prevTotal: 1,
	})
	rm := result.(Model)
	assert.False(t, rm.logLoadingHistory)
}

func TestFinalUpdateLogHistoryMsgError(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	m.logLoadingHistory = true
	result, _ := m.Update(logHistoryMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.logLoadingHistory)
}

func TestFinalUpdateDiffLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	result, _ := m.Update(diffLoadedMsg{left: "left", right: "right", leftName: "L", rightName: "R"})
	rm := result.(Model)
	assert.Equal(t, modeDiff, rm.mode)
}

func TestFinalUpdateDiffLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, _ := m.Update(diffLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	_ = rm
}

func TestFinalUpdateExplainLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	result, _ := m.Update(explainLoadedMsg{
		title:       "pods",
		description: "A pod is...",
		fields:      []model.ExplainField{{Name: "metadata", Type: "Object"}},
	})
	rm := result.(Model)
	assert.Equal(t, modeExplain, rm.mode)
}

func TestFinalUpdateLogSaveAllMsg(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	result, _ := m.Update(logSaveAllMsg{path: "/tmp/logs.log"})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "/tmp/logs.log")
}

func TestFinalUpdateLogSaveAllMsgError(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	result, _ := m.Update(logSaveAllMsg{err: assert.AnError})
	rm := result.(Model)
	_ = rm
}

func TestFinalUpdateHelmRollbackDoneMsg(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(helmRollbackDoneMsg{})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "rollback successful")
	assert.NotNil(t, cmd)
}

func TestFinalUpdateHelmRollbackDoneMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(helmRollbackDoneMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestCovUpdateWindowSizeMsg(t *testing.T) {
	m := baseModelHandlers2()
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	rm := result.(Model)
	assert.Equal(t, 120, rm.width)
	assert.Equal(t, 50, rm.height)
}

func TestCovUpdateKeyMsg(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeExplorer
	m.middleItems = []model.Item{{Name: "item1"}, {Name: "item2"}}
	result, _ := m.Update(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.cursor())
}

func TestCovUpdateSpinnerMsg(t *testing.T) {
	m := baseModelNav()
	m.loading = true
	result, _ := m.Update(m.spinner.Tick())
	_ = result.(Model)
}

func TestCovUpdateWindowSize(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	rm := result.(Model)
	assert.Equal(t, 200, rm.width)
	assert.Equal(t, 60, rm.height)
}

func TestCovUpdateContextsLoaded(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Level = model.LevelClusters
	m.loading = true
	result, _ := m.Update(contextsLoadedMsg{
		items: []model.Item{{Name: "ctx-1"}, {Name: "ctx-2"}},
	})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Len(t, rm.middleItems, 2)
}

func TestCovUpdateContextsLoadedError(t *testing.T) {
	m := baseModelUpdate()
	m.loading = true
	result, cmd := m.Update(contextsLoadedMsg{
		err: errors.New("connection refused"),
	})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.NotNil(t, rm.err)
	assert.NotNil(t, cmd)
}

func TestCovUpdateContextsLoadedCanceled(t *testing.T) {
	m := baseModelUpdate()
	m.loading = true
	result, cmd := m.Update(contextsLoadedMsg{
		err: context.Canceled,
	})
	_ = result
	assert.Nil(t, cmd)
}

func TestCovUpdateResourceTypesMsg(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Level = model.LevelResourceTypes
	result, _ := m.Update(resourceTypesMsg{
		items: []model.Item{{Name: "Pods", Extra: "v1/pods"}},
	})
	rm := result.(Model)
	assert.Len(t, rm.middleItems, 1)
}

func TestCovUpdateResourceTypesMsgAtClusters(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Level = model.LevelClusters
	result, _ := m.Update(resourceTypesMsg{
		items: []model.Item{{Name: "Pods"}},
	})
	rm := result.(Model)
	assert.Len(t, rm.rightItems, 1)
}

func TestCovUpdateCRDDiscovery(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "ctx-1"
	result, _ := m.Update(apiResourceDiscoveryMsg{
		context: "ctx-1",
		entries: []model.ResourceTypeEntry{{Kind: "MyResource", Resource: "myresources"}},
	})
	rm := result.(Model)
	assert.NotNil(t, rm.discoveredResources["ctx-1"])
}

func TestCovUpdateCRDDiscoveryDeeper(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Level = model.LevelResources
	m.nav.Context = "ctx-1"
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1"}
	result, _ := m.Update(apiResourceDiscoveryMsg{
		context: "ctx-1",
		entries: []model.ResourceTypeEntry{{Kind: "Pod", Resource: "pods"}},
	})
	rm := result.(Model)
	assert.NotNil(t, rm.discoveredResources["ctx-1"])
}

func TestCovUpdateCRDDiscoveryError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(apiResourceDiscoveryMsg{
		context: "ctx-1",
		err:     errors.New("forbidden"),
	})
	_ = result
}

func TestCovUpdateCRDDiscoveryCanceled(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(apiResourceDiscoveryMsg{
		err: context.Canceled,
	})
	assert.Nil(t, cmd)
	_ = result
}

func TestCovUpdateCRDDiscoveryDifferentContext(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Context = "ctx-1"
	result, _ := m.Update(apiResourceDiscoveryMsg{
		context: "ctx-2",
		entries: []model.ResourceTypeEntry{{Kind: "MyResource", Resource: "myresources"}},
	})
	rm := result.(Model)
	assert.NotNil(t, rm.discoveredResources["ctx-2"])
}

func TestCovUpdateResourcesLoaded(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(resourcesLoadedMsg{
		items: []model.Item{{Name: "pod-1"}, {Name: "pod-2"}},
	})
	rm := result.(Model)
	assert.Len(t, rm.middleItems, 2)
}

func TestCovUpdateResourcesLoadedForPreview(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(resourcesLoadedMsg{
		items:      []model.Item{{Name: "owned-1"}},
		forPreview: true,
	})
	rm := result.(Model)
	assert.Len(t, rm.rightItems, 1)
}

func TestCovUpdateResourcesLoadedError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(resourcesLoadedMsg{
		err: errors.New("bad request"),
	})
	rm := result.(Model)
	assert.NotNil(t, rm.err)
	assert.NotNil(t, cmd)
}

func TestCovUpdateResourcesLoadedStale(t *testing.T) {
	m := baseModelUpdate()
	m.requestGen = 5
	result, cmd := m.Update(resourcesLoadedMsg{
		gen:   4,
		items: []model.Item{{Name: "stale"}},
	})
	rm := result.(Model)
	assert.NotEqual(t, "stale", rm.middleItems[0].Name)
	assert.Nil(t, cmd)
}

func TestCovUpdateResourcesLoadedWarningFilter(t *testing.T) {
	m := baseModelUpdate()
	m.warningEventsOnly = true
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Event", Resource: "events"}
	result, _ := m.Update(resourcesLoadedMsg{
		items: []model.Item{
			{Name: "evt-1", Status: "Warning"},
			{Name: "evt-2", Status: "Normal"},
		},
	})
	rm := result.(Model)
	assert.Len(t, rm.middleItems, 1)
}

func TestCovUpdateResourcesLoadedPendingTarget(t *testing.T) {
	m := baseModelUpdate()
	m.pendingTarget = "pod-2"
	result, _ := m.Update(resourcesLoadedMsg{
		items: []model.Item{{Name: "pod-1"}, {Name: "pod-2"}, {Name: "pod-3"}},
	})
	rm := result.(Model)
	assert.Empty(t, rm.pendingTarget)
}

func TestCovUpdateOwnedLoaded(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Level = model.LevelOwned
	result, _ := m.Update(ownedLoadedMsg{
		items: []model.Item{{Name: "rs-1"}, {Name: "rs-2"}},
	})
	rm := result.(Model)
	assert.Len(t, rm.middleItems, 2)
}

func TestCovUpdateOwnedLoadedForPreview(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(ownedLoadedMsg{
		items:      []model.Item{{Name: "pod-a"}},
		forPreview: true,
	})
	rm := result.(Model)
	assert.Len(t, rm.rightItems, 1)
}

func TestCovUpdateOwnedLoadedError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(ownedLoadedMsg{
		err: errors.New("not found"),
	})
	rm := result.(Model)
	assert.NotNil(t, rm.err)
	assert.NotNil(t, cmd)
}

func TestCovUpdateOwnedLoadedStale(t *testing.T) {
	m := baseModelUpdate()
	m.requestGen = 5
	result, cmd := m.Update(ownedLoadedMsg{gen: 3})
	assert.Nil(t, cmd)
	_ = result
}

func TestCovUpdateResourceTreeLoaded(t *testing.T) {
	m := baseModelUpdate()
	tree := &model.ResourceNode{Name: "root"}
	result, _ := m.Update(resourceTreeLoadedMsg{
		tree: tree,
	})
	rm := result.(Model)
	assert.Equal(t, tree, rm.resourceTree)
}

func TestCovUpdateResourceTreeError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(resourceTreeLoadedMsg{
		err: errors.New("timeout"),
	})
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovUpdateResourceTreeStale(t *testing.T) {
	m := baseModelUpdate()
	m.requestGen = 5
	result, cmd := m.Update(resourceTreeLoadedMsg{gen: 3})
	assert.Nil(t, cmd)
	_ = result
}

func TestCovUpdateContainersLoaded(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Level = model.LevelContainers
	result, _ := m.Update(containersLoadedMsg{
		items: []model.Item{{Name: "main"}, {Name: "sidecar"}},
	})
	rm := result.(Model)
	assert.Len(t, rm.middleItems, 2)
}

func TestCovUpdateContainersLoadedForPreview(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(containersLoadedMsg{
		items:      []model.Item{{Name: "main"}},
		forPreview: true,
	})
	rm := result.(Model)
	assert.Len(t, rm.rightItems, 1)
}

func TestCovUpdateContainersLoadedError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(containersLoadedMsg{
		err: errors.New("forbidden"),
	})
	rm := result.(Model)
	assert.NotNil(t, rm.err)
}

func TestCovUpdateYAMLLoaded(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(yamlLoadedMsg{
		content: "apiVersion: v1\nkind: Pod",
	})
	_ = result
}

func TestCovUpdateYAMLLoadedError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(yamlLoadedMsg{
		err: errors.New("not found"),
	})
	_ = result
}

func TestCovUpdateDescribeLoaded(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(describeLoadedMsg{
		content: "Name: my-pod\nNamespace: default",
		title:   "Describe: pods/my-pod",
	})
	rm := result.(Model)
	assert.Equal(t, modeDescribe, rm.mode)
	assert.Contains(t, rm.describeContent, "Name: my-pod")
}

func TestCovUpdateDescribeLoadedError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(describeLoadedMsg{
		err: errors.New("not found"),
	})
	_ = result
}

func TestCovUpdateDiffLoaded(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(diffLoadedMsg{
		left:      "old values",
		right:     "new values",
		leftName:  "Defaults",
		rightName: "User",
	})
	rm := result.(Model)
	assert.Equal(t, modeDiff, rm.mode)
}

func TestCovUpdateLogLine(t *testing.T) {
	m := baseModelUpdate()
	m.mode = modeLogs
	m.logFollow = true
	result, _ := m.Update(logLineMsg{line: "2024-01-01 log entry"})
	rm := result.(Model)
	assert.Contains(t, rm.logLines, "2024-01-01 log entry")
}

func TestCovUpdatePodsForAction(t *testing.T) {
	m := baseModelUpdate()
	m.pendingAction = "Exec"
	result, _ := m.Update(podSelectMsg{
		items: []model.Item{{Name: "pod-1"}, {Name: "pod-2"}},
	})
	_ = result
}

func TestCovUpdatePodsForActionSingle(t *testing.T) {
	m := baseModelUpdate()
	m.pendingAction = "Exec"
	result, _ := m.Update(podSelectMsg{
		items: []model.Item{{Name: "pod-1"}},
	})
	_ = result
}

func TestCovUpdateContainersForAction(t *testing.T) {
	m := baseModelUpdate()
	m.pendingAction = "Exec"
	result, _ := m.Update(containerSelectMsg{
		items: []model.Item{{Name: "main"}, {Name: "sidecar"}},
	})
	rm := result.(Model)
	assert.Equal(t, overlayContainerSelect, rm.overlay)
}

func TestCovUpdateContainersForActionSingle(t *testing.T) {
	m := baseModelUpdate()
	m.pendingAction = "Describe"
	m.actionCtx = actionContext{
		context:      "test-ctx",
		name:         "pod-1",
		namespace:    "default",
		kind:         "Pod",
		resourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
	}
	result, cmd := m.Update(containerSelectMsg{
		items: []model.Item{{Name: "main"}},
	})
	rm := result.(Model)
	assert.Equal(t, "main", rm.actionCtx.containerName)
	assert.NotNil(t, cmd)
}

func TestCovUpdateContainerPortsLoaded(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(containerPortsLoadedMsg{
		ports: []k8s.ContainerPort{{ContainerPort: 80, Protocol: "TCP"}},
	})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, overlayPortForward, rm.overlay)
}

func TestCovUpdateContainerPortsLoadedEmpty(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(containerPortsLoadedMsg{})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, overlayPortForward, rm.overlay)
}

func TestCovUpdateContainerPortsLoadedError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(containerPortsLoadedMsg{
		err: errors.New("not found"),
	})
	_ = result
}

func TestCovUpdateExplainLoaded(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(explainLoadedMsg{
		title:       "pods",
		description: "A Pod is a group of containers.",
		fields:      []model.ExplainField{{Name: "spec", Type: "Object"}},
	})
	rm := result.(Model)
	assert.Equal(t, modeExplain, rm.mode)
}

func TestCovUpdateExplainRecursive(t *testing.T) {
	m := baseModelUpdate()
	m.mode = modeExplain
	result, _ := m.Update(explainRecursiveMsg{
		matches: []model.ExplainField{{Name: "spec.containers"}, {Name: "spec.volumes"}},
		query:   "spec",
	})
	rm := result.(Model)
	assert.Len(t, rm.explainRecursiveResults, 2)
}

func TestCovUpdateFinalizerSearch(t *testing.T) {
	m := baseModelUpdate()
	m.overlay = overlayFinalizerSearch
	result, _ := m.Update(finalizerSearchResultMsg{
		results: []k8s.FinalizerMatch{{Namespace: "default", Kind: "Pod", Name: "stuck-pod"}},
	})
	rm := result.(Model)
	assert.Len(t, rm.finalizerSearchResults, 1)
}

func TestCovUpdateEventTimeline(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(eventTimelineMsg{
		events: []k8s.EventInfo{{Message: "event 1"}, {Message: "event 2"}},
	})
	rm := result.(Model)
	assert.Equal(t, overlayEventTimeline, rm.overlay)
}

func TestCovUpdatePreviewEvents(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(previewEventsLoadedMsg{
		events: []k8s.EventInfo{{Message: "evt-1"}},
	})
	_ = result
}

func TestCovUpdateMetricsLoaded(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(metricsLoadedMsg{
		cpuUsed: 100,
		memUsed: 256,
	})
	_ = result
}

func TestCovUpdatePortForwardUpdate(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__"}
	result, _ := m.Update(portForwardUpdateMsg{})
	_ = result
}

// TestUpdateResourcesLoadedPreviewPrimesItemCache verifies that a preview
// load for a drillable resource type also primes m.itemCache under the
// future drill-in navKey (context/resource) so navigateChildResourceType
// can serve the cached list without issuing a second API fetch. The
// previewSatisfiedNavKey + fingerprint pair act as a one-shot marker
// consumed on drill-in; the fingerprint is a digest of the fetch-affecting
// state (namespace, allNamespaces, selectedNamespaces) so later changes
// to any of those invalidate the shortcut without relying on requestGen
// (which navigateChild always bumps before the child handler runs).
func TestUpdateResourcesLoadedPreviewPrimesItemCache(t *testing.T) {
	m := baseFinalModel()
	m.nav = model.NavigationState{Level: model.LevelResourceTypes, Context: "test-ctx"}
	m.namespace = "default"
	m.requestGen = 5
	items := []model.Item{{Name: "pvc-1", Kind: "PersistentVolumeClaim", Namespace: "default"}}
	pvcRT := model.ResourceTypeEntry{
		Kind:       "PersistentVolumeClaim",
		Resource:   "persistentvolumeclaims",
		APIVersion: "v1",
		Namespaced: true,
	}
	result, _ := m.Update(resourcesLoadedMsg{
		items:      items,
		rt:         pvcRT,
		forPreview: true,
		gen:        5,
	})
	rm := result.(Model)

	expectedKey := "test-ctx/persistentvolumeclaims"
	cached, ok := rm.itemCache[expectedKey]
	assert.True(t, ok, "itemCache must be primed at drill-in navKey %q", expectedKey)
	assert.Equal(t, items, cached, "cached items must match preview payload")
	assert.Equal(t, rm.fetchFingerprint(), rm.cacheFingerprints[expectedKey],
		"cacheFingerprints must snapshot the fetch-affecting state at prime time for this key")
	// The existing rightItems behavior still runs — preview pane shows the
	// filtered items.
	assert.Equal(t, items, rm.rightItems)
}

// TestUpdateResourcesLoadedPreviewNoPrimeWhenMissingRT verifies that the
// cache-prime logic is skipped when the preview msg does not carry an rt
// (e.g., synthetic port-forward previews). Without this guard the handler
// would write an empty resource key like "test-ctx/".
func TestUpdateResourcesLoadedPreviewNoPrimeWhenMissingRT(t *testing.T) {
	m := baseFinalModel()
	m.nav = model.NavigationState{Level: model.LevelResourceTypes, Context: "test-ctx"}
	m.requestGen = 3
	items := []model.Item{{Name: "pf-1", Kind: "PortForward"}}
	result, _ := m.Update(resourcesLoadedMsg{
		items:      items,
		forPreview: true,
		gen:        3,
		// rt left zero-valued on purpose
	})
	rm := result.(Model)

	_, hasEmptyKey := rm.itemCache["test-ctx/"]
	assert.False(t, hasEmptyKey, "must not prime cache under an empty-resource key")
	assert.Empty(t, rm.cacheFingerprints, "must not record a fingerprint without an rt")
}
