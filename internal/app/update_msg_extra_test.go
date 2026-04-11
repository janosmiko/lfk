package app

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- helpers ---

func baseModel() Model {
	return Model{
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		sortAscending:       true,
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
}

// --- yamlLoadedMsg ---

func TestUpdateYamlLoadedMsgSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(yamlLoadedMsg{content: "apiVersion: v1\nkind: Pod\n"})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Contains(t, mdl.yamlContent, "apiVersion")
	assert.Nil(t, mdl.err)
	assert.Nil(t, cmd)
}

func TestUpdateYamlLoadedMsgError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(yamlLoadedMsg{err: errors.New("forbidden")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.NotNil(t, mdl.err)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// --- previewYAMLLoadedMsg ---

func TestUpdatePreviewYAMLLoadedSuccess(t *testing.T) {
	m := baseModel()
	m.requestGen = 5

	result, cmd := m.Update(previewYAMLLoadedMsg{content: "kind: Service\n", gen: 5})
	mdl := result.(Model)
	assert.Contains(t, mdl.previewYAML, "Service")
	assert.Nil(t, cmd)
}

func TestUpdatePreviewYAMLLoadedError(t *testing.T) {
	m := baseModel()
	m.requestGen = 5
	m.previewYAML = "old content"

	result, cmd := m.Update(previewYAMLLoadedMsg{err: errors.New("not found"), gen: 5})
	mdl := result.(Model)
	assert.Empty(t, mdl.previewYAML)
	assert.Nil(t, cmd)
}

func TestUpdatePreviewYAMLLoadedStaleGen(t *testing.T) {
	m := baseModel()
	m.requestGen = 10
	m.previewYAML = "current"

	result, cmd := m.Update(previewYAMLLoadedMsg{content: "stale", gen: 5})
	mdl := result.(Model)
	assert.Equal(t, "current", mdl.previewYAML) // unchanged
	assert.Nil(t, cmd)
}

// --- namespacesLoadedMsg ---

func TestUpdateNamespacesLoadedSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true
	m.namespace = "kube-system"

	items := []model.Item{
		{Name: "default"},
		{Name: "kube-system"},
		{Name: "production"},
	}
	result, cmd := m.Update(namespacesLoadedMsg{items: items})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Len(t, mdl.overlayItems, 4) // "All Namespaces" + 3
	assert.Equal(t, "All Namespaces", mdl.overlayItems[0].Name)
	assert.Equal(t, 2, mdl.overlayCursor) // index of "kube-system" (1-indexed due to "All Namespaces")
	assert.Nil(t, cmd)
}

func TestUpdateNamespacesLoadedAllNamespaces(t *testing.T) {
	m := baseModel()
	m.allNamespaces = true

	items := []model.Item{{Name: "default"}}
	result, cmd := m.Update(namespacesLoadedMsg{items: items})
	mdl := result.(Model)
	assert.Equal(t, 0, mdl.overlayCursor) // "All Namespaces" selected
	assert.Nil(t, cmd)
}

func TestUpdateNamespacesLoadedError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(namespacesLoadedMsg{err: errors.New("forbidden")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.NotNil(t, mdl.err)
	assert.Equal(t, overlayNone, mdl.overlay)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// --- resourcesLoadedMsg ---

func TestUpdateResourcesLoadedForPreview(t *testing.T) {
	m := baseModel()
	m.loading = true

	items := []model.Item{{Name: "child-1"}, {Name: "child-2"}}
	result, cmd := m.Update(resourcesLoadedMsg{items: items, forPreview: true, gen: 0})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Len(t, mdl.rightItems, 2)
	assert.Nil(t, cmd)
}

func TestUpdateResourcesLoadedStaleGen(t *testing.T) {
	m := baseModel()
	m.requestGen = 10

	result, cmd := m.Update(resourcesLoadedMsg{items: []model.Item{{Name: "stale"}}, gen: 5})
	mdl := result.(Model)
	assert.Empty(t, mdl.middleItems) // not applied
	assert.Nil(t, cmd)
}

func TestUpdateResourcesLoadedError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(resourcesLoadedMsg{err: errors.New("server error"), gen: 0})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.NotNil(t, mdl.err)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// TestUpdateResourcesLoadedAlwaysSortsOnLoad is a regression guard for
// the Helm-releases flicker: updateResourcesLoadedMain previously only
// ran sortMiddleItems when the user had changed sort away from the
// default (Name, ascending), relying on the k8s layer to pre-sort by
// Name. But the k8s layer uses a non-stable sort with a single-key
// comparator, so rows with equal Name would shuffle between watch
// refreshes. The fix unconditionally calls sortMiddleItems so the
// app-level tiebreaker always applies.
//
// The test feeds in two items with identical Name and different
// Namespace in "wrong" order (prod before dev). If sortMiddleItems runs,
// the ascending Namespace tiebreaker puts dev first. If the old skip is
// back, the input order survives and prod comes out first.
func TestUpdateResourcesLoadedAlwaysSortsOnLoad(t *testing.T) {
	ui.ActiveSortableColumns = []string{"Name", "Age", "Namespace", "Status"}
	defer func() { ui.ActiveSortableColumns = nil }()

	m := baseModel()
	m.sortColumnName = "Name"
	m.sortAscending = true

	items := []model.Item{
		{Name: "traefik", Namespace: "prod", Kind: "HelmRelease"},
		{Name: "traefik", Namespace: "dev", Kind: "HelmRelease"},
	}
	result, _ := m.Update(resourcesLoadedMsg{items: items, gen: 0})
	mdl := result.(Model)

	assert.Len(t, mdl.middleItems, 2)
	assert.Equal(t, "dev", mdl.middleItems[0].Namespace,
		"tiebreaker must put dev before prod even on default Name/asc sort")
	assert.Equal(t, "prod", mdl.middleItems[1].Namespace)
}

func TestUpdateResourcesLoadedWithWarningEventsFilter(t *testing.T) {
	m := baseModel()
	m.warningEventsOnly = true
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Event"}

	items := []model.Item{
		{Name: "event-1", Status: "Warning"},
		{Name: "event-2", Status: "Normal"},
		{Name: "event-3", Status: "Warning"},
	}
	result, _ := m.Update(resourcesLoadedMsg{items: items, gen: 0})
	mdl := result.(Model)
	assert.Len(t, mdl.middleItems, 2) // only Warning events
}

func TestUpdateResourcesLoadedWithPendingTarget(t *testing.T) {
	m := baseModel()
	m.pendingTarget = "target-pod"

	items := []model.Item{
		{Name: "other-pod"},
		{Name: "target-pod"},
		{Name: "another-pod"},
	}
	result, _ := m.Update(resourcesLoadedMsg{items: items, gen: 0})
	mdl := result.(Model)
	assert.Equal(t, 1, mdl.cursor()) // cursor on target-pod
	assert.Empty(t, mdl.pendingTarget)
}

// --- ownedLoadedMsg ---

func TestUpdateOwnedLoadedForPreview(t *testing.T) {
	m := baseModel()
	m.loading = true

	items := []model.Item{{Name: "owned-1"}}
	result, cmd := m.Update(ownedLoadedMsg{items: items, forPreview: true, gen: 0})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Len(t, mdl.rightItems, 1)
	assert.Nil(t, cmd)
}

func TestUpdateOwnedLoadedStaleGen(t *testing.T) {
	m := baseModel()
	m.requestGen = 10

	result, cmd := m.Update(ownedLoadedMsg{items: []model.Item{{Name: "stale"}}, gen: 5})
	mdl := result.(Model)
	assert.Empty(t, mdl.middleItems) // not applied
	assert.Nil(t, cmd)
}

func TestUpdateOwnedLoadedError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(ownedLoadedMsg{err: errors.New("not found"), gen: 0})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.NotNil(t, mdl.err)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// --- containersLoadedMsg ---

func TestUpdateContainersLoadedForPreview(t *testing.T) {
	m := baseModel()
	m.loading = true

	items := []model.Item{{Name: "container-1"}, {Name: "container-2"}}
	result, cmd := m.Update(containersLoadedMsg{items: items, forPreview: true, gen: 0})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Len(t, mdl.rightItems, 2)
	assert.Nil(t, cmd)
}

func TestUpdateContainersLoadedStaleGen(t *testing.T) {
	m := baseModel()
	m.requestGen = 10

	result, cmd := m.Update(containersLoadedMsg{items: []model.Item{{Name: "stale"}}, gen: 5})
	mdl := result.(Model)
	assert.Empty(t, mdl.middleItems)
	assert.Nil(t, cmd)
}

func TestUpdateContainersLoadedError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(containersLoadedMsg{err: errors.New("timeout"), gen: 0})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.NotNil(t, mdl.err)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// --- resourceTreeLoadedMsg ---

func TestUpdateResourceTreeLoadedStaleGen(t *testing.T) {
	m := baseModel()
	m.requestGen = 10

	result, cmd := m.Update(resourceTreeLoadedMsg{gen: 5})
	mdl := result.(Model)
	assert.Nil(t, mdl.resourceTree) // not applied
	assert.Nil(t, cmd)
}

func TestUpdateResourceTreeLoadedError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(resourceTreeLoadedMsg{err: errors.New("tree err"), gen: 0})
	mdl := result.(Model)
	assert.Nil(t, mdl.resourceTree)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestUpdateResourceTreeLoadedSuccess(t *testing.T) {
	m := baseModel()

	tree := &model.ResourceNode{Name: "deploy-1"}
	result, cmd := m.Update(resourceTreeLoadedMsg{tree: tree, gen: 0})
	mdl := result.(Model)
	assert.NotNil(t, mdl.resourceTree)
	assert.Equal(t, "deploy-1", mdl.resourceTree.Name)
	assert.Nil(t, cmd)
}

// --- commandBarResultMsg ---

func TestUpdateCommandBarResultSuccess(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(commandBarResultMsg{output: "NAME\tSTATUS\npod-1\tRunning"})
	mdl := result.(Model)
	assert.Equal(t, modeDescribe, mdl.mode)
	assert.Contains(t, mdl.describeContent, "pod-1")
	assert.Equal(t, "Command Output", mdl.describeTitle)
	assert.Nil(t, cmd)
}

func TestUpdateCommandBarResultEmpty(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(commandBarResultMsg{output: ""})
	mdl := result.(Model)
	assert.Contains(t, mdl.statusMessage, "no output")
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestUpdateCommandBarResultErrorWithOutput(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(commandBarResultMsg{
		err:    errors.New("exit 1"),
		output: "error: the server could not find...",
	})
	mdl := result.(Model)
	assert.Equal(t, modeDescribe, mdl.mode)
	assert.Equal(t, "Command Output (error)", mdl.describeTitle)
	assert.Nil(t, cmd)
}

func TestUpdateCommandBarResultErrorNoOutput(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(commandBarResultMsg{err: errors.New("connection refused")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// --- triggerCronJobMsg ---

func TestUpdateTriggerCronJobSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(triggerCronJobMsg{jobName: "my-job-12345"})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Contains(t, mdl.statusMessage, "my-job-12345")
	assert.NotNil(t, cmd) // refreshCurrentLevel + scheduleStatusClear batch
}

func TestUpdateTriggerCronJobError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(triggerCronJobMsg{err: errors.New("forbidden")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

// --- bulkActionResultMsg ---

func TestUpdateBulkActionResultAllSucceeded(t *testing.T) {
	m := baseModel()
	m.loading = true
	m.bulkMode = true
	m.selectedItems = map[string]bool{"a": true, "b": true}

	result, cmd := m.Update(bulkActionResultMsg{succeeded: 3, failed: 0})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.False(t, mdl.bulkMode)
	assert.False(t, mdl.hasSelection())
	assert.Contains(t, mdl.statusMessage, "3 resources processed")
	assert.NotNil(t, cmd)
}

func TestUpdateBulkActionResultWithFailures(t *testing.T) {
	m := baseModel()
	m.loading = true
	m.bulkMode = true

	result, cmd := m.Update(bulkActionResultMsg{
		succeeded: 2,
		failed:    1,
		errors:    []string{"pod/broken: forbidden"},
	})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.Contains(t, mdl.statusMessage, "2 succeeded")
	assert.Contains(t, mdl.statusMessage, "1 failed")
	assert.NotNil(t, cmd)
}

// --- stderrCapturedMsg ---

func TestUpdateStderrCaptured(t *testing.T) {
	m := baseModel()
	m.stderrChan = make(chan string, 1)

	result, cmd := m.Update(stderrCapturedMsg{message: "AWS SSO session expired"})
	mdl := result.(Model)
	assert.Contains(t, mdl.statusMessage, "stderr:")
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd) // scheduleStatusClear + waitForStderr batch
}

// --- portForwardUpdateMsg ---

func TestUpdatePortForwardUpdateMsgWithError(t *testing.T) {
	m := baseModel()
	m.portForwardMgr = k8s.NewPortForwardManager()

	result, _ := m.Update(portForwardUpdateMsg{err: fmt.Errorf("restore port forward svc/my-svc: connection refused")})
	mdl := result.(Model)

	// The error should appear in the error log.
	found := false
	for _, entry := range mdl.errorLog {
		if entry.Level == "ERR" && strings.Contains(entry.Message, "connection refused") {
			found = true
			break
		}
	}
	assert.True(t, found, "port forward error should appear in error log")
}

// --- watchTickMsg ---

func TestUpdateWatchTickWatchModeOff(t *testing.T) {
	m := baseModel()
	m.watchMode = false

	result, cmd := m.Update(watchTickMsg{})
	mdl := result.(Model)
	assert.False(t, mdl.watchMode)
	assert.Nil(t, cmd)
}

// --- describeLoadedMsg ---

func TestUpdateDescribeLoadedSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(describeLoadedMsg{
		content: "Name: my-pod\nNamespace: default",
		title:   "Pod: my-pod",
	})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Equal(t, modeDescribe, mdl.mode)
	assert.Contains(t, mdl.describeContent, "my-pod")
	assert.Equal(t, "Pod: my-pod", mdl.describeTitle)
	assert.Equal(t, 0, mdl.describeScroll)
	assert.Nil(t, cmd)
}

func TestUpdateDescribeLoadedError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(describeLoadedMsg{err: errors.New("not found")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// --- helmValuesLoadedMsg ---

func TestUpdateHelmValuesLoadedSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(helmValuesLoadedMsg{
		content: "replicaCount: 3",
		title:   "Values: my-release",
	})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Equal(t, modeDescribe, mdl.mode)
	assert.Contains(t, mdl.describeContent, "replicaCount")
	assert.Equal(t, "Values: my-release", mdl.describeTitle)
	assert.Nil(t, cmd)
}

func TestUpdateHelmValuesLoadedError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(helmValuesLoadedMsg{err: errors.New("release not found")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// --- diffLoadedMsg ---

func TestUpdateDiffLoadedSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(diffLoadedMsg{
		left:      "apiVersion: v1\nkind: Pod",
		right:     "apiVersion: v1\nkind: Service",
		leftName:  "pod.yaml",
		rightName: "svc.yaml",
	})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Equal(t, modeDiff, mdl.mode)
	assert.Contains(t, mdl.diffLeft, "Pod")
	assert.Contains(t, mdl.diffRight, "Service")
	assert.Equal(t, "pod.yaml", mdl.diffLeftName)
	assert.Equal(t, "svc.yaml", mdl.diffRightName)
	assert.Equal(t, 0, mdl.diffScroll)
	assert.False(t, mdl.diffUnified)
	assert.Nil(t, cmd)
}

func TestUpdateDiffLoadedError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(diffLoadedMsg{err: errors.New("diff error")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// --- explainLoadedMsg ---

func TestUpdateExplainLoadedSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true

	fields := []model.ExplainField{
		{Name: "apiVersion", Type: "string"},
		{Name: "kind", Type: "string"},
	}
	result, cmd := m.Update(explainLoadedMsg{
		fields:      fields,
		description: "Pod is a collection of containers",
		title:       "pods.v1",
		path:        "pods",
	})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Equal(t, modeExplain, mdl.mode)
	assert.Len(t, mdl.explainFields, 2)
	assert.Equal(t, "Pod is a collection of containers", mdl.explainDesc)
	assert.Equal(t, "pods.v1", mdl.explainTitle)
	assert.Equal(t, "pods", mdl.explainPath)
	assert.Equal(t, 0, mdl.explainCursor)
	assert.Equal(t, 0, mdl.explainScroll)
	assert.False(t, mdl.explainSearchActive)
	assert.Nil(t, cmd)
}

func TestUpdateExplainLoadedError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(explainLoadedMsg{err: errors.New("not supported")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// --- explainRecursiveMsg ---

func TestUpdateExplainRecursiveSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true

	matches := []model.ExplainField{
		{Name: "containers", Type: "[]Object", Path: "spec.containers"},
		{Name: "volumes", Type: "[]Object", Path: "spec.volumes"},
	}
	result, cmd := m.Update(explainRecursiveMsg{matches: matches})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Len(t, mdl.explainRecursiveResults, 2)
	assert.Equal(t, 0, mdl.explainRecursiveCursor)
	assert.Equal(t, overlayExplainSearch, mdl.overlay)
	assert.Nil(t, cmd)
}

func TestUpdateExplainRecursiveNoMatches(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(explainRecursiveMsg{matches: nil})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.Contains(t, mdl.statusMessage, "No fields found")
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestUpdateExplainRecursiveError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(explainRecursiveMsg{err: errors.New("parse error")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// --- metricsLoadedMsg ---

func TestUpdateMetricsLoadedStaleGen(t *testing.T) {
	m := baseModel()
	m.requestGen = 10
	m.metricsContent = "old"

	result, cmd := m.Update(metricsLoadedMsg{cpuUsed: 100, gen: 5})
	mdl := result.(Model)
	assert.Equal(t, "old", mdl.metricsContent) // unchanged
	assert.Nil(t, cmd)
}

func TestUpdateMetricsLoadedZero(t *testing.T) {
	m := baseModel()
	m.metricsContent = "old metrics"

	result, cmd := m.Update(metricsLoadedMsg{cpuUsed: 0, memUsed: 0, gen: 0})
	mdl := result.(Model)
	assert.Empty(t, mdl.metricsContent) // cleared
	assert.Nil(t, cmd)
}

// --- apiResourceDiscoveryMsg ---

func TestUpdateAPIResourceDiscoveryError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(apiResourceDiscoveryMsg{
		context: "test",
		err:     errors.New("forbidden"),
	})
	mdl := result.(Model)
	assert.Nil(t, mdl.discoveredResources["test"])
	assert.Nil(t, cmd)
}

func TestUpdateAPIResourceDiscoveryDifferentContext(t *testing.T) {
	m := baseModel()
	m.nav.Context = "prod"

	entries := []model.ResourceTypeEntry{
		{Kind: "MyResource", Resource: "myresources", DisplayName: "MyResource"},
	}
	result, cmd := m.Update(apiResourceDiscoveryMsg{context: "staging", entries: entries})
	mdl := result.(Model)
	// Discovery prepends PseudoResources() (helm releases + port forwards)
	// so the stored slice contains the 2 pseudo entries plus the real one.
	expected := len(model.PseudoResources()) + 1
	assert.Len(t, mdl.discoveredResources["staging"], expected)
	assert.Empty(t, mdl.middleItems) // not in same context, no middle update
	assert.Nil(t, cmd)
}

// --- podSelectMsg ---

func TestUpdatePodSelectError(t *testing.T) {
	m := baseModel()
	m.pendingAction = "exec"

	result, cmd := m.Update(podSelectMsg{err: errors.New("forbidden")})
	mdl := result.(Model)
	assert.Empty(t, mdl.pendingAction)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestUpdatePodSelectNoPods(t *testing.T) {
	m := baseModel()
	m.pendingAction = "exec"

	result, cmd := m.Update(podSelectMsg{items: []model.Item{
		{Name: "svc-1", Kind: "Service"}, // not a Pod
	}})
	mdl := result.(Model)
	assert.Empty(t, mdl.pendingAction)
	assert.True(t, mdl.statusMessageErr)
	assert.Contains(t, mdl.statusMessage, "No pods found")
	assert.NotNil(t, cmd)
}

func TestUpdatePodSelectMultiplePods(t *testing.T) {
	m := baseModel()
	m.pendingAction = "exec"

	pods := []model.Item{
		{Name: "pod-1", Kind: "Pod"},
		{Name: "pod-2", Kind: "Pod"},
	}
	result, cmd := m.Update(podSelectMsg{items: pods})
	mdl := result.(Model)
	assert.Equal(t, overlayPodSelect, mdl.overlay)
	assert.Len(t, mdl.overlayItems, 2)
	assert.Equal(t, 0, mdl.overlayCursor)
	assert.Nil(t, cmd)
}

// --- logContainersLoadedMsg ---

func TestUpdateLogContainersLoadedSingleContainer(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(logContainersLoadedMsg{containers: []string{"app"}})
	mdl := result.(Model)
	assert.Equal(t, overlayNone, mdl.overlay)
	assert.Contains(t, mdl.statusMessage, "app")
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestUpdateLogContainersLoadedMultiple(t *testing.T) {
	m := baseModel()
	m.overlay = overlayLogContainerSelect

	result, cmd := m.Update(logContainersLoadedMsg{
		containers: []string{"app", "sidecar", "init"},
	})
	mdl := result.(Model)
	assert.Equal(t, "app", mdl.logContainers[0])
	assert.Len(t, mdl.overlayItems, 4) // "All Containers" + 3
	assert.Equal(t, "All Containers", mdl.overlayItems[0].Name)
	assert.Nil(t, cmd)
}

func TestUpdateLogContainersLoadedError(t *testing.T) {
	m := baseModel()
	m.overlay = overlayLogContainerSelect

	result, cmd := m.Update(logContainersLoadedMsg{err: errors.New("failed")})
	mdl := result.(Model)
	assert.Equal(t, overlayNone, mdl.overlay)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// --- secretSavedMsg ---

func TestUpdateSecretSavedSuccess(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(secretSavedMsg{})
	mdl := result.(Model)
	assert.Contains(t, mdl.statusMessage, "Secret saved")
	assert.NotNil(t, cmd)
}

func TestUpdateSecretSavedError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(secretSavedMsg{err: errors.New("denied")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

// --- configMapSavedMsg ---

func TestUpdateConfigMapSavedSuccess(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(configMapSavedMsg{})
	mdl := result.(Model)
	assert.Contains(t, mdl.statusMessage, "ConfigMap saved")
	assert.NotNil(t, cmd)
}

func TestUpdateConfigMapSavedError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(configMapSavedMsg{err: errors.New("forbidden")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

// --- labelSavedMsg ---

func TestUpdateLabelSavedSuccess(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(labelSavedMsg{})
	mdl := result.(Model)
	assert.Contains(t, mdl.statusMessage, "Labels/annotations saved")
	assert.Equal(t, overlayNone, mdl.overlay)
	assert.NotNil(t, cmd)
}

func TestUpdateLabelSavedError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(labelSavedMsg{err: errors.New("conflict")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

// --- rollbackDoneMsg ---

func TestUpdateRollbackDoneSuccess(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(rollbackDoneMsg{})
	mdl := result.(Model)
	assert.Contains(t, mdl.statusMessage, "Rollback successful")
	assert.Equal(t, overlayNone, mdl.overlay)
	assert.NotNil(t, cmd)
}

func TestUpdateRollbackDoneError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(rollbackDoneMsg{err: errors.New("fail")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

// --- helmRollbackDoneMsg ---

func TestUpdateHelmRollbackDoneSuccess(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(helmRollbackDoneMsg{})
	mdl := result.(Model)
	assert.Contains(t, mdl.statusMessage, "Helm rollback successful")
	assert.Equal(t, overlayNone, mdl.overlay)
	assert.NotNil(t, cmd)
}

func TestUpdateHelmRollbackDoneError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(helmRollbackDoneMsg{err: errors.New("fail")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

// --- exportDoneMsg ---

func TestUpdateExportDoneSuccess(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(exportDoneMsg{path: "/tmp/pod.yaml"})
	mdl := result.(Model)
	assert.Contains(t, mdl.statusMessage, "/tmp/pod.yaml")
	assert.NotNil(t, cmd)
}

func TestUpdateExportDoneError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(exportDoneMsg{err: errors.New("write failed")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

// --- containerSelectMsg ---

func TestUpdateContainerSelectError(t *testing.T) {
	m := baseModel()
	m.pendingAction = "exec"

	result, cmd := m.Update(containerSelectMsg{err: errors.New("timeout")})
	mdl := result.(Model)
	assert.Empty(t, mdl.pendingAction)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdateContainerSelectNoContainers(t *testing.T) {
	m := baseModel()
	m.pendingAction = "exec"

	result, cmd := m.Update(containerSelectMsg{items: nil})
	mdl := result.(Model)
	assert.Empty(t, mdl.pendingAction)
	assert.Contains(t, mdl.statusMessage, "No containers found")
	assert.NotNil(t, cmd)
}

func TestUpdateContainerSelectMultiple(t *testing.T) {
	m := baseModel()
	m.pendingAction = "exec"

	items := []model.Item{{Name: "app"}, {Name: "sidecar"}}
	result, cmd := m.Update(containerSelectMsg{items: items})
	mdl := result.(Model)
	assert.Equal(t, overlayContainerSelect, mdl.overlay)
	assert.Len(t, mdl.overlayItems, 2)
	assert.Equal(t, 0, mdl.overlayCursor)
	assert.Nil(t, cmd)
}

// --- yamlClipboardMsg ---

func TestUpdateYamlClipboardError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(yamlClipboardMsg{err: errors.New("fetch failed")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdateYamlClipboardSuccess(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(yamlClipboardMsg{content: "apiVersion: v1"})
	mdl := result.(Model)
	assert.Contains(t, mdl.statusMessage, "YAML copied")
	assert.NotNil(t, cmd) // copyToSystemClipboard + scheduleStatusClear
}

// --- eventTimelineMsg ---

func TestUpdateEventTimelineError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(eventTimelineMsg{err: errors.New("forbidden")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdateEventTimelineEmpty(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(eventTimelineMsg{events: nil})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Contains(t, mdl.statusMessage, "No events found")
	assert.NotNil(t, cmd)
}

func TestUpdateEventTimelineSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true

	events := []k8s.EventInfo{
		{Reason: "Pulled", Message: "Container image pulled"},
	}
	result, cmd := m.Update(eventTimelineMsg{events: events})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Equal(t, overlayEventTimeline, mdl.overlay)
	assert.Len(t, mdl.eventTimelineData, 1)
	assert.Equal(t, 0, mdl.eventTimelineScroll)
	assert.Nil(t, cmd)
}

// --- rbacCheckMsg ---

func TestUpdateRBACCheckError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(rbacCheckMsg{err: errors.New("forbidden")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdateRBACCheckSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true

	results := []k8s.RBACCheck{
		{Verb: "get", Allowed: true},
		{Verb: "delete", Allowed: false},
	}
	result, cmd := m.Update(rbacCheckMsg{results: results, kind: "Pod", resource: "pods"})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Equal(t, overlayRBAC, mdl.overlay)
	assert.Len(t, mdl.rbacResults, 2)
	assert.Equal(t, "Pod", mdl.rbacKind)
	assert.Nil(t, cmd)
}

// --- podStartupMsg ---

func TestUpdatePodStartupError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(podStartupMsg{err: errors.New("not found")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdatePodStartupSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true

	info := &k8s.PodStartupInfo{PodName: "test-pod"}
	result, _ := m.Update(podStartupMsg{info: info})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Equal(t, overlayPodStartup, mdl.overlay)
	assert.NotNil(t, mdl.podStartupData)
}

// --- quotaLoadedMsg ---

func TestUpdateQuotaLoadedError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(quotaLoadedMsg{err: errors.New("forbidden")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdateQuotaLoadedEmpty(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(quotaLoadedMsg{quotas: nil})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Contains(t, mdl.statusMessage, "No resource quotas")
	assert.NotNil(t, cmd)
}

func TestUpdateQuotaLoadedSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true

	quotas := []k8s.QuotaInfo{{Name: "default-quota"}}
	result, _ := m.Update(quotaLoadedMsg{quotas: quotas})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Equal(t, overlayQuotaDashboard, mdl.overlay)
	assert.Len(t, mdl.quotaData, 1)
}

// --- alertsLoadedMsg ---

func TestUpdateAlertsLoadedError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(alertsLoadedMsg{err: errors.New("timeout")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdateAlertsLoadedSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true

	alerts := []k8s.AlertInfo{{Name: "HighCPU"}}
	result, _ := m.Update(alertsLoadedMsg{alerts: alerts})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Equal(t, overlayAlerts, mdl.overlay)
	assert.Len(t, mdl.alertsData, 1)
	assert.Equal(t, 0, mdl.alertsScroll)
}

// --- netpolLoadedMsg ---

func TestUpdateNetpolLoadedError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(netpolLoadedMsg{err: errors.New("not found")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdateNetpolLoadedSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true

	info := &k8s.NetworkPolicyInfo{Name: "default-deny"}
	result, cmd := m.Update(netpolLoadedMsg{info: info})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Equal(t, overlayNetworkPolicy, mdl.overlay)
	assert.NotNil(t, mdl.netpolData)
	assert.Equal(t, 0, mdl.netpolScroll)
	assert.Nil(t, cmd)
}

// --- revisionListMsg ---

func TestUpdateRevisionListError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(revisionListMsg{err: errors.New("forbidden")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdateRevisionListSuccess(t *testing.T) {
	m := baseModel()

	revisions := []k8s.DeploymentRevision{{Revision: 1, Name: "rs-1"}, {Revision: 2, Name: "rs-2"}}
	result, cmd := m.Update(revisionListMsg{revisions: revisions})
	mdl := result.(Model)
	assert.Equal(t, overlayRollback, mdl.overlay)
	assert.Len(t, mdl.rollbackRevisions, 2)
	assert.Equal(t, 0, mdl.rollbackCursor)
	assert.Nil(t, cmd)
}

// --- helmRevisionListMsg ---

func TestUpdateHelmRevisionListError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(helmRevisionListMsg{err: errors.New("not found")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdateHelmRevisionListSuccess(t *testing.T) {
	m := baseModel()

	revisions := []ui.HelmRevision{{Revision: 1}, {Revision: 2}}
	result, cmd := m.Update(helmRevisionListMsg{revisions: revisions})
	mdl := result.(Model)
	assert.Equal(t, overlayHelmRollback, mdl.overlay)
	assert.Len(t, mdl.helmRollbackRevisions, 2)
	assert.Equal(t, 0, mdl.helmRollbackCursor)
	assert.Nil(t, cmd)
}

// --- helmHistoryListMsg ---

func TestUpdateHelmHistoryListError(t *testing.T) {
	m := baseModel()
	m.overlay = overlayHelmHistory

	result, cmd := m.Update(helmHistoryListMsg{err: errors.New("not found")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.Equal(t, overlayNone, mdl.overlay)
	assert.NotNil(t, cmd)
}

func TestUpdateHelmHistoryListSuccess(t *testing.T) {
	m := baseModel()

	revisions := []ui.HelmRevision{{Revision: 1}, {Revision: 2}}
	result, cmd := m.Update(helmHistoryListMsg{revisions: revisions})
	mdl := result.(Model)
	assert.Equal(t, overlayHelmHistory, mdl.overlay)
	assert.Len(t, mdl.helmHistoryRevisions, 2)
	assert.Equal(t, 0, mdl.helmHistoryCursor)
	assert.Nil(t, cmd)
}

// --- dashboardLoadedMsg ---

func TestUpdateDashboardLoadedSameContext(t *testing.T) {
	m := baseModel()
	m.nav.Context = "prod"

	result, cmd := m.Update(dashboardLoadedMsg{content: "dashboard content", context: "prod"})
	mdl := result.(Model)
	assert.Equal(t, "dashboard content", mdl.dashboardPreview)
	assert.Nil(t, cmd)
}

func TestUpdateDashboardLoadedDifferentContext(t *testing.T) {
	m := baseModel()
	m.nav.Context = "prod"
	m.dashboardPreview = "old"

	result, cmd := m.Update(dashboardLoadedMsg{content: "new content", context: "staging"})
	mdl := result.(Model)
	assert.Equal(t, "old", mdl.dashboardPreview) // unchanged
	assert.Nil(t, cmd)
}

// --- monitoringDashboardMsg ---

func TestUpdateMonitoringDashboardSameContext(t *testing.T) {
	m := baseModel()
	m.nav.Context = "prod"

	result, cmd := m.Update(monitoringDashboardMsg{content: "monitoring data", context: "prod"})
	mdl := result.(Model)
	assert.Equal(t, "monitoring data", mdl.monitoringPreview)
	assert.Nil(t, cmd)
}

func TestUpdateMonitoringDashboardDifferentContext(t *testing.T) {
	m := baseModel()
	m.nav.Context = "prod"
	m.monitoringPreview = "old"

	result, cmd := m.Update(monitoringDashboardMsg{content: "new", context: "staging"})
	mdl := result.(Model)
	assert.Equal(t, "old", mdl.monitoringPreview) // unchanged
	assert.Nil(t, cmd)
}

// --- configMapDataLoadedMsg ---

func TestUpdateConfigMapDataLoadedError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(configMapDataLoadedMsg{err: errors.New("not found")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdateConfigMapDataLoadedSuccess(t *testing.T) {
	m := baseModel()

	data := &model.ConfigMapData{Keys: []string{"key1"}, Data: map[string]string{"key1": "val1"}}
	result, cmd := m.Update(configMapDataLoadedMsg{data: data})
	mdl := result.(Model)
	assert.Equal(t, overlayConfigMapEditor, mdl.overlay)
	assert.NotNil(t, mdl.configMapData)
	assert.Equal(t, 0, mdl.configMapCursor)
	assert.False(t, mdl.configMapEditing)
	assert.Nil(t, cmd)
}

// --- secretDataLoadedMsg ---

func TestUpdateSecretDataLoadedError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(secretDataLoadedMsg{err: errors.New("forbidden")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdateSecretDataLoadedSuccess(t *testing.T) {
	m := baseModel()

	data := &model.SecretData{Keys: []string{"password"}, Data: map[string]string{"password": "secret"}}
	result, cmd := m.Update(secretDataLoadedMsg{data: data})
	mdl := result.(Model)
	assert.Equal(t, overlaySecretEditor, mdl.overlay)
	assert.NotNil(t, mdl.secretData)
	assert.Equal(t, 0, mdl.secretCursor)
	assert.False(t, mdl.secretEditing)
	assert.Nil(t, cmd)
}

// --- labelDataLoadedMsg ---

func TestUpdateLabelDataLoadedError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(labelDataLoadedMsg{err: errors.New("timeout")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdateLabelDataLoadedSuccess(t *testing.T) {
	m := baseModel()

	data := &model.LabelAnnotationData{
		Labels:    map[string]string{"app": "test"},
		LabelKeys: []string{"app"},
	}
	result, cmd := m.Update(labelDataLoadedMsg{data: data})
	mdl := result.(Model)
	assert.Equal(t, overlayLabelEditor, mdl.overlay)
	assert.NotNil(t, mdl.labelData)
	assert.Equal(t, 0, mdl.labelCursor)
	assert.Equal(t, 0, mdl.labelTab)
	assert.False(t, mdl.labelEditing)
	assert.Nil(t, cmd)
}

// --- applyPinnedGroups ---

func TestApplyPinnedGroupsConfigOnly(t *testing.T) {
	orig := ui.ConfigPinnedGroups
	defer func() { ui.ConfigPinnedGroups = orig }()

	ui.ConfigPinnedGroups = []string{"Workloads", "Storage"}

	m := baseModel()
	m.applyPinnedGroups()

	assert.Equal(t, []string{"Workloads", "Storage"}, model.PinnedGroups)
}

func TestApplyPinnedGroupsWithPerContextPins(t *testing.T) {
	orig := ui.ConfigPinnedGroups
	defer func() { ui.ConfigPinnedGroups = orig }()

	ui.ConfigPinnedGroups = []string{"Workloads"}

	m := baseModel()
	m.nav.Context = "prod"
	m.pinnedState = &PinnedState{
		Contexts: map[string][]string{
			"prod": {"Networking", "Security"},
		},
	}
	m.applyPinnedGroups()

	assert.Equal(t, []string{"Workloads", "Networking", "Security"}, model.PinnedGroups)
}

func TestApplyPinnedGroupsDeduplicates(t *testing.T) {
	orig := ui.ConfigPinnedGroups
	defer func() { ui.ConfigPinnedGroups = orig }()

	ui.ConfigPinnedGroups = []string{"Workloads", "Storage"}

	m := baseModel()
	m.nav.Context = "prod"
	m.pinnedState = &PinnedState{
		Contexts: map[string][]string{
			"prod": {"Workloads", "Networking"}, // Workloads is duplicate
		},
	}
	m.applyPinnedGroups()

	assert.Equal(t, []string{"Workloads", "Storage", "Networking"}, model.PinnedGroups)
}

// --- contextsLoadedMsg ---

func TestUpdateContextsLoadedSuccess(t *testing.T) {
	m := baseModel()
	m.nav.Level = model.LevelClusters
	m.loading = true

	items := []model.Item{
		{Name: "minikube"},
		{Name: "prod-cluster"},
	}
	result, _ := m.Update(contextsLoadedMsg{items: items})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Nil(t, mdl.err)
	assert.Len(t, mdl.middleItems, 2)
	assert.Nil(t, mdl.leftItems)
}

func TestUpdateContextsLoadedError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(contextsLoadedMsg{err: errors.New("kube config not found")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.NotNil(t, mdl.err)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// --- metricsLoadedMsg with actual values ---

func TestUpdateMetricsLoadedWithValues(t *testing.T) {
	m := baseModel()
	m.requestGen = 0

	result, cmd := m.Update(metricsLoadedMsg{
		cpuUsed: 500,
		cpuReq:  1000,
		cpuLim:  2000,
		memUsed: 256 * 1024 * 1024,
		memReq:  512 * 1024 * 1024,
		memLim:  1024 * 1024 * 1024,
		gen:     0,
	})
	mdl := result.(Model)
	assert.NotEmpty(t, mdl.metricsContent)
	assert.Nil(t, cmd)
}

// --- previewEventsLoadedMsg ---

func TestUpdatePreviewEventsLoadedStaleGen(t *testing.T) {
	m := baseModel()
	m.requestGen = 10
	m.previewEventsContent = "old"

	result, cmd := m.Update(previewEventsLoadedMsg{gen: 5})
	mdl := result.(Model)
	assert.Equal(t, "old", mdl.previewEventsContent) // unchanged
	assert.Nil(t, cmd)
}

func TestUpdatePreviewEventsLoadedEmpty(t *testing.T) {
	m := baseModel()
	m.previewEventsContent = "old"

	result, cmd := m.Update(previewEventsLoadedMsg{events: nil, gen: 0})
	mdl := result.(Model)
	assert.Empty(t, mdl.previewEventsContent) // cleared
	assert.Nil(t, cmd)
}

// --- podMetricsEnrichedMsg ---

func TestUpdatePodMetricsEnrichedStaleGen(t *testing.T) {
	m := baseModel()
	m.requestGen = 10

	result, cmd := m.Update(podMetricsEnrichedMsg{gen: 5})
	mdl := result.(Model)
	assert.Empty(t, mdl.middleItems) // unchanged
	assert.Nil(t, cmd)
}

func TestUpdatePodMetricsEnrichedEmpty(t *testing.T) {
	m := baseModel()

	_, cmd := m.Update(podMetricsEnrichedMsg{metrics: nil, gen: 0})
	assert.Nil(t, cmd)
}

func TestUpdatePodMetricsEnrichedWithData(t *testing.T) {
	m := baseModel()
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default"},
		{Name: "pod-2", Namespace: "default"},
	}

	metrics := map[string]model.PodMetrics{
		"default/pod-1": {Name: "pod-1", CPU: 100, Memory: 256},
	}
	result, cmd := m.Update(podMetricsEnrichedMsg{metrics: metrics, gen: 0})
	mdl := result.(Model)
	// pod-1 should have metrics columns enriched
	assert.Len(t, mdl.middleItems, 2)
	assert.Nil(t, cmd)
}

// --- nodeMetricsEnrichedMsg ---

func TestUpdateNodeMetricsEnrichedStaleGen(t *testing.T) {
	m := baseModel()
	m.requestGen = 10

	_, cmd := m.Update(nodeMetricsEnrichedMsg{gen: 5})
	assert.Nil(t, cmd)
}

// --- podLogSelectMsg ---

func TestUpdatePodLogSelectError(t *testing.T) {
	m := baseModel()
	m.pendingAction = "logs"

	result, cmd := m.Update(podLogSelectMsg{err: errors.New("forbidden")})
	mdl := result.(Model)
	assert.Empty(t, mdl.pendingAction)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdatePodLogSelectNoPods(t *testing.T) {
	m := baseModel()
	m.pendingAction = "logs"

	result, cmd := m.Update(podLogSelectMsg{items: []model.Item{}})
	mdl := result.(Model)
	assert.Empty(t, mdl.pendingAction)
	assert.Contains(t, mdl.statusMessage, "No pods found")
	assert.NotNil(t, cmd)
}

// --- canISAListMsg ---

func TestUpdateCanISAListError(t *testing.T) {
	m := baseModel()
	m.loading = true

	result, cmd := m.Update(canISAListMsg{err: errors.New("forbidden")})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestUpdateCanISAListSuccess(t *testing.T) {
	m := baseModel()
	m.loading = true
	m.namespace = "default"

	accounts := []string{"default/my-sa"}
	subjects := []k8s.RBACSubject{
		{Kind: "User", Name: "admin"},
		{Kind: "Group", Name: "devs"},
	}
	result, cmd := m.Update(canISAListMsg{accounts: accounts, subjects: subjects})
	mdl := result.(Model)
	assert.False(t, mdl.loading)
	assert.Equal(t, overlayCanISubject, mdl.overlay)
	// Items: "Current User" + 1 User + 1 Group + 1 SA = 4
	assert.Len(t, mdl.overlayItems, 4)
	assert.Equal(t, "Current User", mdl.overlayItems[0].Name)
	assert.Contains(t, mdl.overlayItems[1].Name, "admin")
	assert.Contains(t, mdl.overlayItems[2].Name, "devs")
	assert.Contains(t, mdl.overlayItems[3].Name, "my-sa")
	assert.Nil(t, cmd)
}

// --- logHistoryMsg ---

func TestUpdateLogHistoryError(t *testing.T) {
	m := baseModel()
	m.logLoadingHistory = true

	result, cmd := m.Update(logHistoryMsg{err: errors.New("fetch failed")})
	mdl := result.(Model)
	assert.False(t, mdl.logLoadingHistory)
	assert.False(t, mdl.logHasMoreHistory)
	assert.Nil(t, cmd)
}

func TestUpdateLogHistoryNotInLogMode(t *testing.T) {
	m := baseModel()
	m.mode = modeExplorer
	m.logLoadingHistory = true

	result, cmd := m.Update(logHistoryMsg{lines: []string{"line1", "line2"}})
	mdl := result.(Model)
	assert.False(t, mdl.logLoadingHistory)
	assert.Nil(t, cmd) // not in log mode, skip
}

// --- logSaveAllMsg ---

func TestUpdateLogSaveAllSuccess(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(logSaveAllMsg{path: "/tmp/logs.txt"})
	mdl := result.(Model)
	assert.Contains(t, mdl.statusMessage, "/tmp/logs.txt")
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestUpdateLogSaveAllError(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(logSaveAllMsg{err: errors.New("write failed")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd)
}

// --- tea.WindowSizeMsg at different tab counts ---

func TestUpdateWindowSizeMsgWithMultipleTabs(t *testing.T) {
	m := baseModel()
	m.tabs = []TabState{{}, {}} // two tabs

	result, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	mdl := result.(Model)
	assert.Equal(t, 100, mdl.width)
	assert.Equal(t, 30, mdl.height)
	assert.Nil(t, cmd)
}
