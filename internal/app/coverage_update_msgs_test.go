package app

import (
	"context"
	"errors"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

func baseModelUpdate() Model {
	cs := fake.NewClientset()
	dyn := dynfake.NewSimpleDynamicClient(runtime.NewScheme())

	m := Model{
		nav:            model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:           []TabState{{}},
		selectedItems:  make(map[string]bool),
		cursorMemory:   make(map[string]int),
		itemCache:      make(map[string][]model.Item),
		discoveredCRDs: make(map[string][]model.ResourceTypeEntry),
		width:          120,
		height:         40,
		execMu:         &sync.Mutex{},
		client:         k8s.NewTestClient(cs, dyn),
		namespace:      "default",
		reqCtx:         context.Background(),
		portForwardMgr: k8s.NewPortForwardManager(),
	}
	m.middleItems = []model.Item{{Name: "pod-1", Namespace: "default", Kind: "Pod"}}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}

	return m
}

// =====================================================================
// Update -- WindowSizeMsg
// =====================================================================

func TestCovUpdateWindowSize(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	rm := result.(Model)
	assert.Equal(t, 200, rm.width)
	assert.Equal(t, 60, rm.height)
}

// =====================================================================
// Update -- contextsLoadedMsg
// =====================================================================

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

// =====================================================================
// Update -- resourceTypesMsg
// =====================================================================

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

// =====================================================================
// Update -- crdDiscoveryMsg
// =====================================================================

func TestCovUpdateCRDDiscovery(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "ctx-1"
	result, _ := m.Update(crdDiscoveryMsg{
		context: "ctx-1",
		entries: []model.ResourceTypeEntry{{Kind: "MyResource", Resource: "myresources"}},
	})
	rm := result.(Model)
	assert.NotNil(t, rm.discoveredCRDs["ctx-1"])
}

func TestCovUpdateCRDDiscoveryDeeper(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Level = model.LevelResources
	m.nav.Context = "ctx-1"
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1"}
	result, _ := m.Update(crdDiscoveryMsg{
		context: "ctx-1",
		entries: []model.ResourceTypeEntry{{Kind: "Pod", Resource: "pods"}},
	})
	rm := result.(Model)
	assert.NotNil(t, rm.discoveredCRDs["ctx-1"])
}

func TestCovUpdateCRDDiscoveryError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(crdDiscoveryMsg{
		context: "ctx-1",
		err:     errors.New("forbidden"),
	})
	_ = result
}

func TestCovUpdateCRDDiscoveryCanceled(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(crdDiscoveryMsg{
		err: context.Canceled,
	})
	assert.Nil(t, cmd)
	_ = result
}

func TestCovUpdateCRDDiscoveryDifferentContext(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Context = "ctx-1"
	result, _ := m.Update(crdDiscoveryMsg{
		context: "ctx-2",
		entries: []model.ResourceTypeEntry{{Kind: "MyResource", Resource: "myresources"}},
	})
	rm := result.(Model)
	assert.NotNil(t, rm.discoveredCRDs["ctx-2"])
}

// =====================================================================
// Update -- resourcesLoadedMsg
// =====================================================================

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

// =====================================================================
// Update -- ownedLoadedMsg
// =====================================================================

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

// =====================================================================
// Update -- resourceTreeLoadedMsg
// =====================================================================

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

// =====================================================================
// Update -- containersLoadedMsg
// =====================================================================

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

// =====================================================================
// Update -- yamlLoadedMsg
// =====================================================================

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

// =====================================================================
// Update -- actionResultMsg
// =====================================================================

func TestCovUpdateActionResult(t *testing.T) {
	m := baseModelUpdate()
	m.loading = true
	result, cmd := m.Update(actionResultMsg{
		message: "Deleted pod/test-pod",
	})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateActionResultError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(actionResultMsg{
		err: errors.New("delete failed"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

// =====================================================================
// Update -- describeLoadedMsg
// =====================================================================

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

// =====================================================================
// Update -- statusMessageExpiredMsg
// =====================================================================

func TestCovUpdateStatusClear(t *testing.T) {
	m := baseModelUpdate()
	m.setStatusMessage("test message", false)
	result, _ := m.Update(statusMessageExpiredMsg{})
	rm := result.(Model)
	assert.False(t, rm.hasStatusMessage())
}

// =====================================================================
// Update -- diffLoadedMsg
// =====================================================================

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

func TestCovUpdateDiffLoadedError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(diffLoadedMsg{
		err: errors.New("helm not found"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
}

// =====================================================================
// Update -- logLineMsg
// =====================================================================

func TestCovUpdateLogLine(t *testing.T) {
	m := baseModelUpdate()
	m.mode = modeLogs
	m.logFollow = true
	result, _ := m.Update(logLineMsg{line: "2024-01-01 log entry"})
	rm := result.(Model)
	assert.Contains(t, rm.logLines, "2024-01-01 log entry")
}

// =====================================================================
// Update -- podSelectMsg
// =====================================================================

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

func TestCovUpdatePodsForActionError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(podSelectMsg{
		err: errors.New("not found"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

// =====================================================================
// Update -- containerSelectMsg
// =====================================================================

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

func TestCovUpdateContainersForActionError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(containerSelectMsg{
		err: errors.New("forbidden"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

// =====================================================================
// Update -- rollbackDoneMsg
// =====================================================================

func TestCovUpdateRollbackDone(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(rollbackDoneMsg{})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateRollbackDoneError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(rollbackDoneMsg{err: errors.New("failed")})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

// =====================================================================
// Update -- helmRollbackDoneMsg
// =====================================================================

func TestCovUpdateHelmRollbackDone(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(helmRollbackDoneMsg{})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateHelmRollbackDoneError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(helmRollbackDoneMsg{err: errors.New("failed")})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

// =====================================================================
// Update -- triggerCronJobMsg
// =====================================================================

func TestCovUpdateTriggerCronJob(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(triggerCronJobMsg{jobName: "manual-job-1"})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateTriggerCronJobError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(triggerCronJobMsg{err: errors.New("quota exceeded")})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

// =====================================================================
// Update -- portForwardStartedMsg
// =====================================================================

func TestCovUpdatePortForwardStarted(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(portForwardStartedMsg{localPort: "9090", remotePort: "80"})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdatePortForwardStartedError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(portForwardStartedMsg{err: errors.New("bind failed")})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

// =====================================================================
// Update -- containerPortsLoadedMsg
// =====================================================================

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

// =====================================================================
// Update -- logSaveAllMsg
// =====================================================================

func TestCovUpdateLogSaveAll(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(logSaveAllMsg{path: "/tmp/test.log"})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovUpdateLogSaveAllError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(logSaveAllMsg{err: errors.New("disk full")})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

// =====================================================================
// Update -- explainLoadedMsg
// =====================================================================

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

func TestCovUpdateExplainLoadedError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(explainLoadedMsg{
		err: errors.New("not found"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
}

// =====================================================================
// Update -- explainRecursiveMsg
// =====================================================================

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

func TestCovUpdateExplainRecursiveError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(explainRecursiveMsg{
		err: errors.New("kubectl not found"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
}

// =====================================================================
// Update -- finalizerSearchMsg
// =====================================================================

func TestCovUpdateFinalizerSearch(t *testing.T) {
	m := baseModelUpdate()
	m.overlay = overlayFinalizerSearch
	result, _ := m.Update(finalizerSearchResultMsg{
		results: []k8s.FinalizerMatch{{Namespace: "default", Kind: "Pod", Name: "stuck-pod"}},
	})
	rm := result.(Model)
	assert.Len(t, rm.finalizerSearchResults, 1)
}

func TestCovUpdateFinalizerSearchError(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(finalizerSearchResultMsg{
		err: errors.New("timeout"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
}

// =====================================================================
// Update -- eventTimelineMsg
// =====================================================================

func TestCovUpdateEventTimeline(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(eventTimelineMsg{
		events: []k8s.EventInfo{{Message: "event 1"}, {Message: "event 2"}},
	})
	rm := result.(Model)
	assert.Equal(t, overlayEventTimeline, rm.overlay)
}

func TestCovUpdateEventTimelineError(t *testing.T) {
	m := baseModelUpdate()
	result, cmd := m.Update(eventTimelineMsg{
		err: errors.New("forbidden"),
	})
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd)
}

// =====================================================================
// Update -- previewEventsMsg
// =====================================================================

func TestCovUpdatePreviewEvents(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(previewEventsLoadedMsg{
		events: []k8s.EventInfo{{Message: "evt-1"}},
	})
	_ = result
}

// =====================================================================
// Update -- metricsLoadedMsg
// =====================================================================

func TestCovUpdateMetricsLoaded(t *testing.T) {
	m := baseModelUpdate()
	result, _ := m.Update(metricsLoadedMsg{
		cpuUsed: 100,
		memUsed: 256,
	})
	_ = result
}

// =====================================================================
// Update -- portForwardUpdateMsg
// =====================================================================

func TestCovUpdatePortForwardUpdate(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__"}
	result, _ := m.Update(portForwardUpdateMsg{})
	_ = result
}
