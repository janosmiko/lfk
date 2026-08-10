package app

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	assert.Contains(t, mdl.yamlView.content, "apiVersion")
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

// TestUpdateNamespacesLoadedSilentPreservesLoading verifies that a
// silent namespace-cache refresh (fired by ensureNamespaceCacheFresh
// during session restore) does not clear m.loading. The loading flag
// belongs to the resource-types list; clearing it while API discovery
// is still in flight produces a "No items" flash between the loader
// and the populated sidebar.
func TestUpdateNamespacesLoadedSilentPreservesLoading(t *testing.T) {
	m := baseModel()
	m.loading = true

	items := []model.Item{{Name: "default"}, {Name: "kube-system"}}
	result, _ := m.Update(namespacesLoadedMsg{items: items, silent: true})
	mdl := result.(Model)

	assert.True(t, mdl.loading, "silent namespace refresh must leave the loading flag alone so discovery can own it")
	// The cache is still updated even in silent mode — that's the whole
	// point of the background refresh.
	assert.NotEmpty(t, mdl.cachedNamespaces)
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
	assert.Contains(t, mdl.describeView.content, "pod-1")
	assert.Equal(t, "Command Output", mdl.describeView.title)
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
	assert.Equal(t, "Command Output (error)", mdl.describeView.title)
	assert.Nil(t, cmd)
}

func TestUpdateCommandBarResultErrorNoOutput(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(commandBarResultMsg{err: errors.New("connection refused")})
	mdl := result.(Model)
	assert.True(t, mdl.statusMessageErr)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// TestUpdateCommandBarResultSanitizesOutput guards the describe view's
// command-bar producer sink: shell/kubectl output is not lfk-controlled.
func TestUpdateCommandBarResultSanitizesOutput(t *testing.T) {
	m := baseModel()

	result, _ := m.Update(commandBarResultMsg{output: "before\x1b[2Jgone\x9b31mHACKED"})
	mdl := result.(Model)
	assert.NotContains(t, mdl.describeView.content, "\x1b[2J")
	assert.NotContains(t, mdl.describeView.content, "\x9b")
}

// TestUpdateCommandBarResultErrorSanitizesOutput mirrors
// TestUpdateCommandBarResultSanitizesOutput for the error-with-output path.
func TestUpdateCommandBarResultErrorSanitizesOutput(t *testing.T) {
	m := baseModel()

	result, _ := m.Update(commandBarResultMsg{
		err:    errors.New("exit 1"),
		output: "before\x1b[2Jgone\x9b31mHACKED",
	})
	mdl := result.(Model)
	assert.NotContains(t, mdl.describeView.content, "\x1b[2J")
	assert.NotContains(t, mdl.describeView.content, "\x9b")
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
	assert.Contains(t, mdl.describeView.content, "my-pod")
	assert.Equal(t, "Pod: my-pod", mdl.describeView.title)
	assert.Equal(t, 0, mdl.describeView.scroll)
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

// TestUpdateDescribeLoadedSanitizesContent guards the describe view's
// kubectl-describe producer sink: describe output (annotation values,
// field-manager names, etc.) is not lfk-controlled.
func TestUpdateDescribeLoadedSanitizesContent(t *testing.T) {
	m := baseModel()

	result, _ := m.Update(describeLoadedMsg{
		content: "Name: my-pod\x1b[2Jgone\x9b31mHACKED",
		title:   "Pod: my-pod",
	})
	mdl := result.(Model)
	assert.NotContains(t, mdl.describeView.content, "\x1b[2J")
	assert.NotContains(t, mdl.describeView.content, "\x9b")
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
	assert.Contains(t, mdl.describeView.content, "replicaCount")
	assert.Equal(t, "Values: my-release", mdl.describeView.title)
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

// TestUpdateHelmValuesLoadedSanitizesContent guards the describe view's helm
// values producer sink: a values.yaml key can carry attacker-influenced
// strings (e.g. a chart pulled from an untrusted repo).
func TestUpdateHelmValuesLoadedSanitizesContent(t *testing.T) {
	m := baseModel()

	result, _ := m.Update(helmValuesLoadedMsg{
		content: "replicaCount: 3\x1b[2Jgone\x9b31mHACKED",
		title:   "Values: my-release",
	})
	mdl := result.(Model)
	assert.NotContains(t, mdl.describeView.content, "\x1b[2J")
	assert.NotContains(t, mdl.describeView.content, "\x9b")
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
	assert.Contains(t, mdl.diffView.left, "Pod")
	assert.Contains(t, mdl.diffView.right, "Service")
	assert.Equal(t, "pod.yaml", mdl.diffView.leftName)
	assert.Equal(t, "svc.yaml", mdl.diffView.rightName)
	assert.Equal(t, 0, mdl.diffView.scroll)
	assert.False(t, mdl.diffView.unified)
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

// When only one container exists, the handler must skip the picker AND start
// the log stream — earlier behavior stalled with just a status message,
// requiring the user to manually retry the action.
func TestUpdateLogContainersLoadedSingleContainer(t *testing.T) {
	m := newTestModelWithClient(t)
	m.actionCtx = actionContext{kind: "Pod", name: "my-pod", namespace: "default", context: "test-ctx"}

	result, cmd := m.Update(logContainersLoadedMsg{containers: []string{"app"}})
	mdl := result.(Model)
	assert.Equal(t, overlayNone, mdl.overlay)
	assert.Equal(t, "app", mdl.actionCtx.containerName, "containerName must be set so the log stream targets the right container")
	assert.NotNil(t, cmd, "log stream command must be returned, not just a status message")
}

// Zero-container case: clear the container filter and still kick off the
// stream so kubectl surfaces the empty-pod error to the user.
func TestUpdateLogContainersLoadedNoContainers(t *testing.T) {
	m := newTestModelWithClient(t)
	m.actionCtx = actionContext{kind: "Pod", name: "my-pod", namespace: "default", context: "test-ctx", containerName: "stale"}

	result, cmd := m.Update(logContainersLoadedMsg{containers: nil})
	mdl := result.(Model)
	assert.Equal(t, overlayNone, mdl.overlay)
	assert.Equal(t, "", mdl.actionCtx.containerName, "stale containerName must be cleared")
	assert.NotNil(t, cmd)
}

// The handler is the one that opens the overlay so the user never sees
// an empty/loading flicker — handleLogKeyOther dispatches the load
// without setting m.overlay, and the overlay is set here only once the
// container items are ready to render.
func TestUpdateLogContainersLoadedMultiple(t *testing.T) {
	m := baseModel()
	// Overlay starts closed; the handler opens it.

	result, cmd := m.Update(logContainersLoadedMsg{
		containers: []string{"app", "sidecar", "init"},
	})
	mdl := result.(Model)
	assert.Equal(t, overlayLogContainerSelect, mdl.overlay,
		"handler must open the overlay once containers have loaded")
	assert.Equal(t, "app", mdl.logView.containers[0])
	assert.Len(t, mdl.overlayItems, 4) // "All Containers" + 3
	assert.Equal(t, "All Containers", mdl.overlayItems[0].Name)
	assert.Equal(t, 0, mdl.overlayCursor, "cursor must reset to top of new overlay")
	assert.Empty(t, mdl.logView.containerFilterText, "filter text must be clear when overlay opens")
	assert.False(t, mdl.logView.containerFilterActive, "filter must be inactive when overlay opens")
	assert.False(t, mdl.logView.containerSelectionModified, "modified flag must be false on a fresh open")
	assert.False(t, mdl.loading, "loading flag must be cleared once data is ready")
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
	// Wrap defaults to on so long messages (FailedScheduling reasons,
	// Helm hook output) don't right-truncate in the default overlay
	// view. User can press `>` to flip off.
	assert.True(t, mdl.eventTimelineWrap, "events overlay must open with wrap enabled")
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

// TestUpdatePodMetricsEnrichedDoesNotDropRawRequestsOnRepeatedTicks reproduces
// a bug where opening the Pods view showed CPU/R, CPU/L, MEM/R, MEM/L with
// real percentages for a second and then all of them flipped to "n/a".
//
// Root cause: the first metrics tick reads CPU Req / CPU Lim / Mem Req /
// Mem Lim from item.Columns to compute the percentages, then destructively
// REMOVES those raw columns when rebuilding (they were in the removeCols
// set). On the next metrics tick (watch refresh, ~2s later) the handler
// finds those columns empty, so ComputePctStr returns "n/a" for every
// percentage cell — the values the user was seeing vanish.
//
// The fix is to preserve the raw request/limit columns across metrics
// ticks. They are always blocked from auto-detected table display
// (internal/ui/explorer_format.go:209), so leaving them on the item has
// no visible side effects, but keeps the source data available for
// subsequent recomputations until the list refresh next repopulates them.
func TestUpdatePodMetricsEnrichedDoesNotDropRawRequestsOnRepeatedTicks(t *testing.T) {
	m := baseModel()
	m.middleItems = []model.Item{
		{
			Name:      "pod-1",
			Namespace: "default",
			Columns: []model.KeyValue{
				{Key: "CPU Req", Value: "100m"},
				{Key: "CPU Lim", Value: "500m"},
				{Key: "Mem Req", Value: "128Mi"},
				{Key: "Mem Lim", Value: "512Mi"},
			},
		},
	}

	metrics := map[string]model.PodMetrics{
		"default/pod-1": {Name: "pod-1", CPU: 50, Memory: 64 * 1024 * 1024},
	}

	// First tick — produces real percentages.
	result, _ := m.Update(podMetricsEnrichedMsg{metrics: metrics, gen: 0})
	m1 := result.(Model)

	pctAfterFirst := func(key string) string {
		for _, kv := range m1.middleItems[0].Columns {
			if kv.Key == key {
				return kv.Value
			}
		}
		return ""
	}

	// Sanity: first tick should have computed real percentages, not n/a.
	require.NotEqual(t, "n/a", pctAfterFirst("CPU/R"), "first tick must compute a real CPU/R; baseline for the regression assertion below")
	require.NotEqual(t, "n/a", pctAfterFirst("MEM/L"), "first tick must compute a real MEM/L; baseline for the regression assertion below")

	// Second tick — must not collapse to n/a because the raw req/lim columns
	// should still be present. This is the actual regression guard.
	result2, _ := m1.Update(podMetricsEnrichedMsg{metrics: metrics, gen: 0})
	m2 := result2.(Model)

	pct := func(key string) string {
		for _, kv := range m2.middleItems[0].Columns {
			if kv.Key == key {
				return kv.Value
			}
		}
		return "<missing>"
	}

	assert.NotEqual(t, "n/a", pct("CPU/R"), "CPU/R must remain computable on the 2nd tick; raw CPU Req was dropped after 1st tick")
	assert.NotEqual(t, "n/a", pct("CPU/L"), "CPU/L must remain computable on the 2nd tick")
	assert.NotEqual(t, "n/a", pct("MEM/R"), "MEM/R must remain computable on the 2nd tick")
	assert.NotEqual(t, "n/a", pct("MEM/L"), "MEM/L must remain computable on the 2nd tick")
}

// TestUpdatePodMetricsEnrichedSingleNamespaceKeyFormat reproduces a bug
// where CPU/R, CPU/L, MEM/R, MEM/L columns never appeared in the Pods
// table (not even in the `,` column toggle overlay) when viewing a single
// namespace rather than all-namespaces. The metrics lookup key must have
// the same shape on both sides of the map — "namespace/name" — or every
// lookup misses and every pod is skipped in enrichment.
func TestUpdatePodMetricsEnrichedSingleNamespaceKeyFormat(t *testing.T) {
	m := baseModel()
	m.middleItems = []model.Item{
		{
			Name:      "pod-a",
			Namespace: "default",
			Columns: []model.KeyValue{
				{Key: "CPU Req", Value: "100m"},
				{Key: "CPU Lim", Value: "500m"},
				{Key: "Mem Req", Value: "128Mi"},
				{Key: "Mem Lim", Value: "512Mi"},
			},
		},
	}

	// Map keyed by "namespace/name" — the canonical format the k8s
	// layer's GetAllPodMetrics now always uses, independent of query
	// scope.
	metrics := map[string]model.PodMetrics{
		"default/pod-a": {Name: "pod-a", Namespace: "default", CPU: 25, Memory: 32 * 1024 * 1024},
	}

	result, _ := m.Update(podMetricsEnrichedMsg{metrics: metrics, gen: 0})
	mdl := result.(Model)

	keys := map[string]string{}
	for _, kv := range mdl.middleItems[0].Columns {
		keys[kv.Key] = kv.Value
	}
	for _, want := range []string{"CPU", "CPU/R", "CPU/L", "MEM", "MEM/R", "MEM/L"} {
		_, ok := keys[want]
		assert.True(t, ok, "enrichment must add %q to the item so the column toggle can surface it; had keys=%v", want, keys)
	}
}

// When a pod was present in the prior tick but is missing from the new
// metrics payload, the handler must clear the prior CPU/MEM column values
// instead of leaving them visible as if they were current.
func TestUpdatePodMetricsEnrichedClearsStaleRowOnMissingPod(t *testing.T) {
	m := baseModel()
	m.middleItems = []model.Item{
		{
			Name:      "pod-x",
			Namespace: "default",
			Columns: []model.KeyValue{
				{Key: "CPU Req", Value: "100m"},
				{Key: "CPU", Value: "42m"},
				{Key: "MEM", Value: "100Mi"},
				{Key: "CPU/R", Value: "50%"},
			},
		},
	}

	// Empty for pod-x but non-empty overall, so the early return for an empty
	// payload does not short-circuit the enrichment loop.
	result, _ := m.Update(podMetricsEnrichedMsg{
		metrics: map[string]model.PodMetrics{
			"default/other-pod": {Name: "other-pod", CPU: 1, Memory: 1},
		},
		gen: 0,
	})
	mdl := result.(Model)

	col := func(key string) string {
		for _, kv := range mdl.middleItems[0].Columns {
			if kv.Key == key {
				return kv.Value
			}
		}
		return "<missing>"
	}
	assert.Equal(t, "n/a", col("CPU"), "stale CPU must reset to n/a when the pod drops out of the payload")
	assert.Equal(t, "n/a", col("MEM"), "stale MEM must reset to n/a when the pod drops out of the payload")
	assert.Equal(t, "n/a", col("CPU/R"), "stale CPU/R must reset to n/a")
	assert.Equal(t, "100m", col("CPU Req"), "raw request column must survive so the next successful tick can recompute")
}

// Regression: PodInitializing/CrashLoopBackOff pods saw the Reason column
// hop between two positions every watch tick because the three paths that
// rebuild item.Columns disagreed on order. carryOverMetricsColumns and
// updatePodMetricsEnriched put CPU/MEM first then everything else;
// clearStalePodMetricsColumns used to put everything else first then CPU/MEM,
// so each tick flipped the column order and the user saw a ~1 Hz layout blink.
// Pin the order: CPU/MEM block first, all other columns after, identical
// across all three paths.
func TestPodMetricsColumnOrderIsStableAcrossAllPaths(t *testing.T) {
	metricsKeys := []string{"CPU", "CPU/R", "CPU/L", "MEM", "MEM/R", "MEM/L"}
	keysOf := func(cols []model.KeyValue) []string {
		ks := make([]string, len(cols))
		for i, kv := range cols {
			ks[i] = kv.Key
		}
		return ks
	}
	assertCPUMemFirst := func(t *testing.T, label string, cols []model.KeyValue) {
		t.Helper()
		require.GreaterOrEqual(t, len(cols), len(metricsKeys), "%s: too few columns", label)
		for i, want := range metricsKeys {
			assert.Equal(t, want, cols[i].Key, "%s: column %d must be %q (CPU/MEM block before other columns); full order=%v", label, i, want, keysOf(cols))
		}
	}

	// Path 1: clearStalePodMetricsColumns (no metrics for this pod).
	t.Run("clearStalePodMetricsColumns", func(t *testing.T) {
		m := baseModel()
		m.middleItems = []model.Item{{
			Name: "pod-x", Namespace: "default",
			Columns: []model.KeyValue{
				{Key: "Reason", Value: "PodInitializing"},
				{Key: "QoS", Value: "BestEffort"},
				{Key: "CPU Req", Value: "100m"},
				{Key: "CPU", Value: "42m"}, // stale prior-tick value
			},
		}}
		result, _ := m.Update(podMetricsEnrichedMsg{
			metrics: map[string]model.PodMetrics{"default/other": {Name: "other"}},
			gen:     0,
		})
		assertCPUMemFirst(t, "clearStale", result.(Model).middleItems[0].Columns)
	})

	// Path 2: updatePodMetricsEnriched with real metrics for this pod.
	t.Run("updatePodMetricsEnriched", func(t *testing.T) {
		m := baseModel()
		m.middleItems = []model.Item{{
			Name: "pod-x", Namespace: "default",
			Columns: []model.KeyValue{
				{Key: "Reason", Value: "Running"},
				{Key: "QoS", Value: "BestEffort"},
				{Key: "CPU Req", Value: "100m"},
			},
		}}
		result, _ := m.Update(podMetricsEnrichedMsg{
			metrics: map[string]model.PodMetrics{
				"default/pod-x": {Name: "pod-x", CPU: 50, Memory: 100 * 1024 * 1024},
			},
			gen: 0,
		})
		assertCPUMemFirst(t, "enriched", result.(Model).middleItems[0].Columns)
	})

	// Path 3: carryOverMetricsColumns (watch tick replacing items).
	t.Run("carryOverMetricsColumns", func(t *testing.T) {
		m := baseModel()
		m.middleItems = []model.Item{{
			Name: "pod-x", Namespace: "default",
			Columns: []model.KeyValue{
				{Key: "CPU", Value: "n/a"},
				{Key: "CPU/R", Value: "n/a"},
				{Key: "CPU/L", Value: "n/a"},
				{Key: "MEM", Value: "n/a"},
				{Key: "MEM/R", Value: "n/a"},
				{Key: "MEM/L", Value: "n/a"},
				{Key: "Reason", Value: "PodInitializing"},
				{Key: "QoS", Value: "BestEffort"},
			},
		}}
		newItems := []model.Item{{
			Name: "pod-x", Namespace: "default",
			Columns: []model.KeyValue{
				{Key: "Reason", Value: "PodInitializing"},
				{Key: "QoS", Value: "BestEffort"},
				{Key: "CPU Req", Value: "100m"},
			},
		}}
		m.carryOverMetricsColumns(newItems)
		assertCPUMemFirst(t, "carryOver", newItems[0].Columns)
	})
}

// ensureNodeMetricsColumnsPlaceholder used to append the CPU/CPU%/MEM/MEM%
// columns while the enriched and carry-over paths prepend them, so on every
// watch tick a metrics-less node's column order flipped between the two
// layouts and the user saw a ~1 Hz layout blink. Pin the order: CPU/MEM
// block first, all other columns after, identical across all three paths.
func TestNodeMetricsColumnOrderIsStableAcrossAllPaths(t *testing.T) {
	metricsKeys := []string{"CPU", "CPU%", "MEM", "MEM%"}
	keysOf := func(cols []model.KeyValue) []string {
		ks := make([]string, len(cols))
		for i, kv := range cols {
			ks[i] = kv.Key
		}
		return ks
	}
	assertCPUMemFirst := func(t *testing.T, label string, cols []model.KeyValue) {
		t.Helper()
		require.GreaterOrEqual(t, len(cols), len(metricsKeys), "%s: too few columns", label)
		for i, want := range metricsKeys {
			assert.Equal(t, want, cols[i].Key, "%s: column %d must be %q (CPU/MEM block before other columns); full order=%v", label, i, want, keysOf(cols))
		}
	}

	// Path 1: ensureNodeMetricsColumnsPlaceholder (no metrics for this node).
	t.Run("placeholder", func(t *testing.T) {
		m := baseModel()
		m.middleItems = []model.Item{{
			Name: "node-x",
			Columns: []model.KeyValue{
				{Key: "INTERNALDNS", Value: "node-x"},
				{Key: "VERSION", Value: "v1.31"},
				{Key: "CPU", Value: "500m"}, // stale prior-tick value
			},
		}}
		result, _ := m.Update(nodeMetricsEnrichedMsg{
			metrics: map[string]model.PodMetrics{"other": {Name: "other"}},
			gen:     0,
		})
		assertCPUMemFirst(t, "placeholder", result.(Model).middleItems[0].Columns)
	})

	// Path 2: updateNodeMetricsEnriched with real metrics for this node.
	t.Run("enriched", func(t *testing.T) {
		m := baseModel()
		m.middleItems = []model.Item{{
			Name: "node-x",
			Columns: []model.KeyValue{
				{Key: "INTERNALDNS", Value: "node-x"},
				{Key: "VERSION", Value: "v1.31"},
			},
		}}
		result, _ := m.Update(nodeMetricsEnrichedMsg{
			metrics: map[string]model.PodMetrics{
				"node-x": {Name: "node-x", CPU: 50, Memory: 100 * 1024 * 1024},
			},
			gen: 0,
		})
		assertCPUMemFirst(t, "enriched", result.(Model).middleItems[0].Columns)
	})

	// Path 3: carryOverMetricsColumns (watch tick replacing items).
	t.Run("carryOver", func(t *testing.T) {
		m := baseModel()
		m.middleItems = []model.Item{{
			Name: "node-x",
			Columns: []model.KeyValue{
				{Key: "CPU", Value: "n/a"},
				{Key: "CPU%", Value: "n/a"},
				{Key: "MEM", Value: "n/a"},
				{Key: "MEM%", Value: "n/a"},
				{Key: "INTERNALDNS", Value: "node-x"},
				{Key: "VERSION", Value: "v1.31"},
			},
		}}
		newItems := []model.Item{{
			Name: "node-x",
			Columns: []model.KeyValue{
				{Key: "INTERNALDNS", Value: "node-x"},
				{Key: "VERSION", Value: "v1.31"},
			},
		}}
		m.carryOverMetricsColumns(newItems)
		assertCPUMemFirst(t, "carryOver", newItems[0].Columns)
	})
}

// --- nodeMetricsEnrichedMsg ---

func TestUpdateNodeMetricsEnrichedStaleGen(t *testing.T) {
	m := baseModel()
	m.requestGen = 10

	_, cmd := m.Update(nodeMetricsEnrichedMsg{gen: 5})
	assert.Nil(t, cmd)
}

// --- lookupNodeUptime ---

func TestLookupNodeUptime(t *testing.T) {
	uptimes := k8s.NodeUptimes{
		ByName: map[string]time.Duration{"node-a": 5 * time.Minute},
		ByAddr: map[string]time.Duration{
			"10.0.1.5":     10 * time.Minute,
			"node-c.local": 15 * time.Minute,
		},
	}

	t.Run("hit by name", func(t *testing.T) {
		item := model.Item{Name: "node-a"}
		d, ok := lookupNodeUptime(item, uptimes)
		assert.True(t, ok)
		assert.Equal(t, 5*time.Minute, d)
	})

	t.Run("miss on name but hit on InternalIP", func(t *testing.T) {
		item := model.Item{
			Name:    "node-b",
			Columns: []model.KeyValue{{Key: "InternalIP", Value: "10.0.1.5"}},
		}
		d, ok := lookupNodeUptime(item, uptimes)
		assert.True(t, ok)
		assert.Equal(t, 10*time.Minute, d)
	})

	t.Run("miss on name and InternalIP but hit on Hostname", func(t *testing.T) {
		item := model.Item{
			Name: "node-c",
			Columns: []model.KeyValue{
				{Key: "InternalIP", Value: "10.0.9.9"},
				{Key: "Hostname", Value: "node-c.local"},
			},
		}
		d, ok := lookupNodeUptime(item, uptimes)
		assert.True(t, ok)
		assert.Equal(t, 15*time.Minute, d)
	})

	t.Run("total miss returns ok=false", func(t *testing.T) {
		item := model.Item{
			Name:    "node-z",
			Columns: []model.KeyValue{{Key: "InternalIP", Value: "10.0.0.0"}},
		}
		_, ok := lookupNodeUptime(item, uptimes)
		assert.False(t, ok)
	})

	t.Run("empty result returns ok=false", func(t *testing.T) {
		item := model.Item{Name: "node-a"}
		_, ok := lookupNodeUptime(item, k8s.NodeUptimes{})
		assert.False(t, ok)
	})
}

// TestLookupNodeUptimeNameAndAddrCollisionResolvesIndependently is the
// regression guard for defect #4: a flat single-map keyspace let a node
// literally named the same string as another node's InternalIP silently
// collide, so one of the two would show the wrong uptime. Splitting into
// ByName/ByAddr must resolve each node to its own value.
func TestLookupNodeUptimeNameAndAddrCollisionResolvesIndependently(t *testing.T) {
	uptimes := k8s.NodeUptimes{
		ByName: map[string]time.Duration{"10.0.1.5": 100 * time.Minute}, // a node literally named "10.0.1.5"
		ByAddr: map[string]time.Duration{"10.0.1.5": 5 * time.Minute},   // a different node whose InternalIP is 10.0.1.5
	}

	nodeNamedByIP := model.Item{Name: "10.0.1.5"}
	d, ok := lookupNodeUptime(nodeNamedByIP, uptimes)
	require.True(t, ok)
	assert.Equal(t, 100*time.Minute, d, "must resolve via ByName, not the colliding ByAddr entry")

	otherNode := model.Item{
		Name:    "node-b",
		Columns: []model.KeyValue{{Key: "InternalIP", Value: "10.0.1.5"}},
	}
	d, ok = lookupNodeUptime(otherNode, uptimes)
	require.True(t, ok)
	assert.Equal(t, 5*time.Minute, d, "must resolve via ByAddr for the node whose IP equals another node's name")
}

// --- nodeUptimeEnrichedMsg ---

func TestUpdateNodeUptimeEnrichedStaleGen(t *testing.T) {
	m := baseModel()
	m.requestGen = 10
	m.middleItems = []model.Item{{Name: "node-a"}}

	result, _ := m.Update(nodeUptimeEnrichedMsg{
		uptimes: k8s.NodeUptimes{ByName: map[string]time.Duration{"node-a": 5 * time.Minute}},
		gen:     5,
	})
	rm := result.(Model)
	for _, kv := range rm.middleItems[0].Columns {
		assert.NotEqual(t, "Uptime", kv.Key, "stale gen must not mutate columns")
	}
}

// TestUpdateNodeUptimeEnrichedEmptyMapAddsNoColumn is the key regression
// guard: an empty uptimes map means Prometheus isn't configured (or
// node_exporter isn't installed, or the query transiently failed) -- not
// that every node genuinely has zero uptime data. Writing "n/a" placeholders
// here would permanently pin an UPTIME column onto every nodes list on every
// cluster that doesn't run node_exporter.
func TestUpdateNodeUptimeEnrichedEmptyMapAddsNoColumn(t *testing.T) {
	m := baseModel()
	m.middleItems = []model.Item{{Name: "node-a"}, {Name: "node-b"}}

	result, _ := m.Update(nodeUptimeEnrichedMsg{gen: 0})
	rm := result.(Model)
	for _, item := range rm.middleItems {
		for _, kv := range item.Columns {
			assert.NotEqual(t, "Uptime", kv.Key, "empty uptimes map must not add an Uptime column")
		}
	}
}

func TestUpdateNodeUptimeEnrichedMatchedAndUnmatched(t *testing.T) {
	m := baseModel()
	m.middleItems = []model.Item{{Name: "node-a"}, {Name: "node-b"}}

	result, _ := m.Update(nodeUptimeEnrichedMsg{
		uptimes: k8s.NodeUptimes{ByName: map[string]time.Duration{"node-a": 90 * time.Minute}},
		gen:     0,
	})
	rm := result.(Model)

	got := func(item model.Item) (string, bool) {
		for _, kv := range item.Columns {
			if kv.Key == "Uptime" {
				return kv.Value, true
			}
		}
		return "", false
	}

	valA, okA := got(rm.middleItems[0])
	require.True(t, okA)
	assert.Equal(t, k8s.FormatAge(90*time.Minute), valA)

	valB, okB := got(rm.middleItems[1])
	require.True(t, okB)
	assert.Equal(t, "n/a", valB)
}

func TestUpdateNodeUptimeEnrichedMatchedByInternalIP(t *testing.T) {
	m := baseModel()
	m.middleItems = []model.Item{{
		Name:    "node-a",
		Columns: []model.KeyValue{{Key: "InternalIP", Value: "10.0.1.5"}},
	}}

	result, _ := m.Update(nodeUptimeEnrichedMsg{
		uptimes: k8s.NodeUptimes{ByAddr: map[string]time.Duration{"10.0.1.5": 45 * time.Minute}},
		gen:     0,
	})
	rm := result.(Model)

	var val string
	for _, kv := range rm.middleItems[0].Columns {
		if kv.Key == "Uptime" {
			val = kv.Value
		}
	}
	assert.Equal(t, k8s.FormatAge(45*time.Minute), val)
}

func TestUpdateNodeUptimeEnrichedTwiceDoesNotDuplicate(t *testing.T) {
	m := baseModel()
	m.middleItems = []model.Item{{Name: "node-a"}}

	msg := nodeUptimeEnrichedMsg{
		uptimes: k8s.NodeUptimes{ByName: map[string]time.Duration{"node-a": 5 * time.Minute}},
		gen:     0,
	}
	result, _ := m.Update(msg)
	result, _ = result.(Model).Update(msg)
	rm := result.(Model)

	count := 0
	for _, kv := range rm.middleItems[0].Columns {
		if kv.Key == "Uptime" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

// TestUpdateNodeUptimeEnrichedPreservesExistingIndex is the regression guard
// for defect #1 (row-flapping): an existing "Uptime" column must be updated
// in place, never moved, or its position in the detail pane oscillates
// between carryOverMetricsColumnsFrom's prepend and this handler's position.
func TestUpdateNodeUptimeEnrichedPreservesExistingIndex(t *testing.T) {
	m := baseModel()
	m.middleItems = []model.Item{{
		Name: "node-a",
		Columns: []model.KeyValue{
			{Key: "Foo", Value: "x"},
			{Key: "Uptime", Value: "1h0m0s"},
			{Key: "Bar", Value: "y"},
		},
	}}

	result, _ := m.Update(nodeUptimeEnrichedMsg{
		uptimes: k8s.NodeUptimes{ByName: map[string]time.Duration{"node-a": 2 * time.Hour}},
		gen:     0,
	})
	rm := result.(Model)

	cols := rm.middleItems[0].Columns
	require.Len(t, cols, 3)
	assert.Equal(t, "Foo", cols[0].Key)
	assert.Equal(t, "Uptime", cols[1].Key, "Uptime must keep its original index")
	assert.Equal(t, k8s.FormatAge(2*time.Hour), cols[1].Value)
	assert.Equal(t, "Bar", cols[2].Key)
}

// TestUpdateNodeUptimeEnrichedStableAcrossCarryOverTicks simulates two watch
// ticks (carryOverMetricsColumnsFrom then the enrichment handler, twice) and
// asserts the column key order never changes. The flap this defect fixes
// alternated the UPTIME row between its carried-over (prepended) position
// and a moved (appended) position on every tick.
func TestUpdateNodeUptimeEnrichedStableAcrossCarryOverTicks(t *testing.T) {
	m := baseModel()
	// Seed a steady state where "Uptime" is already established (from a
	// prior successful tick), matching the reported symptom: the flap was
	// observed on an already-populated nodes list, not on the very first
	// tick before an Uptime column ever existed.
	m.middleItems = []model.Item{{
		Name:    "node-a",
		Columns: []model.KeyValue{{Key: "Uptime", Value: "1h0m0s"}, {Key: "Other", Value: "x"}},
	}}

	keysOf := func(cols []model.KeyValue) []string {
		keys := make([]string, len(cols))
		for i, kv := range cols {
			keys[i] = kv.Key
		}
		return keys
	}

	var afterCarryOver, afterHandler [2][]string
	for tick := range 2 {
		fresh := []model.Item{{Name: "node-a", Columns: []model.KeyValue{{Key: "Other", Value: "x"}}}}
		carryOverMetricsColumnsFrom(m.middleItems, fresh)
		afterCarryOver[tick] = keysOf(fresh[0].Columns)

		m.middleItems = fresh
		result, _ := m.Update(nodeUptimeEnrichedMsg{
			uptimes: k8s.NodeUptimes{ByName: map[string]time.Duration{"node-a": time.Duration(tick+1) * time.Hour}},
			gen:     0,
		})
		m = result.(Model)
		afterHandler[tick] = keysOf(m.middleItems[0].Columns)
	}

	want := []string{"Uptime", "Other"}
	assert.Equal(t, want, afterCarryOver[0], "carryOver must place Uptime at the same index every tick")
	assert.Equal(t, want, afterHandler[0], "handler must not move the column carryOver just placed")
	assert.Equal(t, want, afterCarryOver[1], "carryOver position must be stable across ticks")
	assert.Equal(t, want, afterHandler[1], "handler position must be stable across ticks")
}

// TestUpdateNodeUptimeEnrichedOrderIndependentFromMetrics asserts running
// updateNodeMetricsEnriched then updateNodeUptimeEnriched produces the same
// column key order as running them in the reverse order -- the two async
// messages can land in either order and must not disagree about where
// "Uptime" sits relative to the CPU/MEM block.
func TestUpdateNodeUptimeEnrichedOrderIndependentFromMetrics(t *testing.T) {
	established := []model.KeyValue{
		{Key: "CPU", Value: "100m"},
		{Key: "CPU%", Value: "10%"},
		{Key: "MEM", Value: "1Gi"},
		{Key: "MEM%", Value: "20%"},
		{Key: "Uptime", Value: "1h0m0s"},
		{Key: "Other", Value: "x"},
	}
	newCols := func() []model.KeyValue {
		cols := make([]model.KeyValue, len(established))
		copy(cols, established)
		return cols
	}
	keysOf := func(cols []model.KeyValue) []string {
		keys := make([]string, len(cols))
		for i, kv := range cols {
			keys[i] = kv.Key
		}
		return keys
	}

	metricsMsg := nodeMetricsEnrichedMsg{
		metrics: map[string]model.PodMetrics{"node-a": {CPU: 500, Memory: 2 * 1024 * 1024 * 1024}},
		gen:     0,
	}
	uptimeMsg := nodeUptimeEnrichedMsg{
		uptimes: k8s.NodeUptimes{ByName: map[string]time.Duration{"node-a": 3 * time.Hour}},
		gen:     0,
	}

	mA := baseModel()
	mA.middleItems = []model.Item{{Name: "node-a", Columns: newCols()}}
	rA, _ := mA.Update(metricsMsg)
	rA, _ = rA.(Model).Update(uptimeMsg)
	orderA := keysOf(rA.(Model).middleItems[0].Columns)

	mB := baseModel()
	mB.middleItems = []model.Item{{Name: "node-a", Columns: newCols()}}
	rB, _ := mB.Update(uptimeMsg)
	rB, _ = rB.(Model).Update(metricsMsg)
	orderB := keysOf(rB.(Model).middleItems[0].Columns)

	wantOrder := keysOf(established)
	assert.Equal(t, wantOrder, orderA, "metrics-then-uptime must not reorder columns")
	assert.Equal(t, wantOrder, orderB, "uptime-then-metrics must not reorder columns")
	assert.Equal(t, orderA, orderB, "arrival order must not affect final column order")
}

// TestUpdateNodeUptimeEnrichedEmptyMapDegradesExistingColumnToNA is the
// regression guard for defect #2: once Prometheus stops answering (an empty
// uptimes map), a node that already carries a live "Uptime" value must have
// it degrade to "n/a" rather than freeze forever. A node that never had an
// Uptime column must still not get one invented (see
// TestUpdateNodeUptimeEnrichedEmptyMapAddsNoColumn).
func TestUpdateNodeUptimeEnrichedEmptyMapDegradesExistingColumnToNA(t *testing.T) {
	m := baseModel()
	m.middleItems = []model.Item{
		{Name: "node-a", Columns: []model.KeyValue{{Key: "Uptime", Value: "2h0m0s"}}},
		{Name: "node-b", Columns: []model.KeyValue{{Key: "Foo", Value: "bar"}}},
	}

	result, _ := m.Update(nodeUptimeEnrichedMsg{gen: 0})
	rm := result.(Model)

	require.Len(t, rm.middleItems[0].Columns, 1)
	assert.Equal(t, "Uptime", rm.middleItems[0].Columns[0].Key, "position must not move")
	assert.Equal(t, "n/a", rm.middleItems[0].Columns[0].Value, "stale value must degrade to n/a")

	for _, kv := range rm.middleItems[1].Columns {
		assert.NotEqual(t, "Uptime", kv.Key, "a node that never had Uptime must not get one invented")
	}
}

// TestUpdateNodeUptimeEnrichedFreezePreventedAcrossTicks runs a realistic
// tick sequence -- a successful fetch, a watch-tick carryOver, then a failed
// (empty-map) fetch -- and asserts the column reads "n/a" afterward instead
// of freezing on the last live value forever.
func TestUpdateNodeUptimeEnrichedFreezePreventedAcrossTicks(t *testing.T) {
	m := baseModel()
	m.middleItems = []model.Item{{Name: "node-a"}}

	// Tick 1: Prometheus answers.
	result, _ := m.Update(nodeUptimeEnrichedMsg{
		uptimes: k8s.NodeUptimes{ByName: map[string]time.Duration{"node-a": 2 * time.Hour}},
		gen:     0,
	})
	m = result.(Model)
	liveValue := m.middleItems[0].Columns[0].Value
	require.Equal(t, k8s.FormatAge(2*time.Hour), liveValue)

	// Tick 2: list refresh carries the live value onto the fresh items,
	// then the fetch fails (empty map).
	fresh := []model.Item{{Name: "node-a"}}
	carryOverMetricsColumnsFrom(m.middleItems, fresh)
	m.middleItems = fresh
	require.Equal(t, liveValue, m.middleItems[0].Columns[0].Value, "carryOver should have preserved the live value going into the failed tick")

	result, _ = m.Update(nodeUptimeEnrichedMsg{gen: 0})
	rm := result.(Model)

	assert.Equal(t, "n/a", rm.middleItems[0].Columns[0].Value, "must degrade to n/a, not freeze on the stale live value")
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
	m.logView.loadingHistory = true

	result, cmd := m.Update(logHistoryMsg{err: errors.New("fetch failed")})
	mdl := result.(Model)
	assert.False(t, mdl.logView.loadingHistory)
	assert.False(t, mdl.logView.hasMoreHistory)
	assert.Nil(t, cmd)
}

func TestUpdateLogHistoryNotInLogMode(t *testing.T) {
	m := baseModel()
	m.mode = modeExplorer
	m.logView.loadingHistory = true

	result, cmd := m.Update(logHistoryMsg{lines: []string{"line1", "line2"}})
	mdl := result.(Model)
	assert.False(t, mdl.logView.loadingHistory)
	assert.Nil(t, cmd) // not in log mode, skip
}

// When the user has navigated to the absolute top (e.g. pressed `gg`),
// the cursor and scroll must stay at 0 after older history is prepended
// so the newly revealed lines come into view. Regression for the
// "gg jumps back down ~1000 lines a second later" bug.
func TestUpdateLogHistoryAtTopKeepsCursor(t *testing.T) {
	m := baseModel()
	m.mode = modeLogs
	m.logView.loadingHistory = true
	m.logView.lines = []string{"existing-1", "existing-2", "existing-3"}
	m.logView.cursor = 0
	m.logView.scroll = 0

	result, _ := m.Update(logHistoryMsg{
		lines:     []string{"older1", "older2", "existing-1", "existing-2", "existing-3"},
		prevTotal: 3,
	})
	mdl := result.(Model)
	assert.False(t, mdl.logView.loadingHistory)
	assert.Equal(t, 0, mdl.logView.cursor, "cursor must remain at top to reveal older lines")
	assert.Equal(t, 0, mdl.logView.scroll, "scroll must remain at top to reveal older lines")
	assert.Equal(t, 5, len(mdl.logView.lines))
	assert.Equal(t, "older1", mdl.logView.lines[0])
}

// When the user has scrolled away from the top while the async history
// fetch was in flight, prepending older lines must shift the cursor and
// scroll by the prepended count to preserve visual position.
func TestUpdateLogHistoryMidScrollPreservesPosition(t *testing.T) {
	m := baseModel()
	m.mode = modeLogs
	m.logView.loadingHistory = true
	m.logView.rawLines = []string{"existing-1", "existing-2", "existing-3"}
	m.logView.lines = m.logView.rawLines
	m.logView.cursor = 2
	m.logView.scroll = 1

	result, _ := m.Update(logHistoryMsg{
		lines:     []string{"older1", "older2", "existing-1", "existing-2", "existing-3"},
		prevTotal: 3,
	})
	mdl := result.(Model)
	assert.Equal(t, 4, mdl.logView.cursor, "cursor shifted by 2 prepended lines")
	assert.Equal(t, 3, mdl.logView.scroll, "scroll shifted by 2 prepended lines")
}

// Same at-top guard applies on the no-overlap fallback path (logs may
// have rotated between fetches, in which case all fetched lines are
// prepended).
func TestUpdateLogHistoryNoOverlapAtTopKeepsCursor(t *testing.T) {
	m := baseModel()
	m.mode = modeLogs
	m.logView.loadingHistory = true
	m.logView.rawLines = []string{"current-1", "current-2", "current-3"}
	m.logView.lines = m.logView.rawLines
	m.logView.cursor = 0
	m.logView.scroll = 0

	result, _ := m.Update(logHistoryMsg{
		lines:     []string{"rotated-1", "rotated-2"}, // no overlap with current
		prevTotal: 3,
	})
	mdl := result.(Model)
	assert.Equal(t, 0, mdl.logView.cursor)
	assert.Equal(t, 0, mdl.logView.scroll)
	assert.Equal(t, 5, len(mdl.logView.lines))
	assert.Equal(t, "rotated-1", mdl.logView.lines[0])
}

// --- logSaveAllMsg ---

func TestUpdateLogSaveAllSuccess(t *testing.T) {
	m := baseModel()

	result, cmd := m.Update(logSaveAllMsg{path: "/tmp/logs.txt"})
	mdl := result.(Model)
	assert.Contains(t, mdl.statusMessage, "/tmp/logs.txt")
	// Issue #61: status should announce the clipboard copy so the user
	// knows the path is recoverable after the 5s status-clear.
	assert.Contains(t, mdl.statusMessage, "(copied to clipboard)")
	assert.NotNil(t, cmd) // tea.Batch(copyToSystemClipboard, scheduleStatusClear)
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

// setUptimeColumn must replace the Columns slice, not write through it:
// cloneCurrentTab shallow-copies Item values, so a tab snapshot shares the
// backing array and would otherwise see another tab's uptime writes.
func TestSetUptimeColumnDoesNotMutateSharedBackingArray(t *testing.T) {
	item := model.Item{Name: "node-1", Columns: []model.KeyValue{
		{Key: "Uptime", Value: "5d"},
		{Key: "InternalIP", Value: "10.0.1.5"},
	}}
	snapshot := item // shallow copy, shares the Columns backing array

	setUptimeColumn(&item, "n/a")

	assert.Equal(t, "n/a", item.Columns[0].Value, "target item updated")
	assert.Equal(t, "5d", snapshot.Columns[0].Value, "snapshot must not be mutated")
	assert.Equal(t, "Uptime", item.Columns[0].Key, "position preserved")
}
