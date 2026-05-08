package app

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	clientfake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// TestPostTabSwitch_RefreshesMiddleColumnAtLevelResources locks in the fix
// for the cross-tab stale-data bug: a mutation on Tab A (e.g., deleting a
// pod that had 10 restarts) used to leave Tab B's saved middleItems
// untouched until manual refresh. Now switching tabs at LevelResources
// fires a background refresh so cached data is replaced when the fresh
// fetch returns.
//
// middleItems is left empty so loadPreview's per-selection sub-cmds
// (containers / metrics / events) short-circuit; the test then proves
// that postTabSwitchCmd still returns a cmd whose result includes a
// non-preview resourcesLoadedMsg — i.e. refreshCurrentLevel was added.
func TestPostTabSwitch_RefreshesMiddleColumnAtLevelResources(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}
	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs)

	m := newTabSwitchTestModel(t, model.LevelResources)
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.client = k8s.NewTestClient(clientfake.NewClientset(), dyn)

	cmd := m.postTabSwitchCmd()
	if cmd == nil {
		t.Fatal("postTabSwitchCmd returned nil at LevelResources/modeExplorer")
	}

	sawRefresh := false
	for _, c := range flattenBatch(cmd) {
		if loaded, ok := c().(resourcesLoadedMsg); ok && !loaded.forPreview {
			sawRefresh = true
		}
	}
	assert.True(t, sawRefresh,
		"postTabSwitchCmd at LevelResources must fire a non-preview refresh fetch")
}

// TestPostTabSwitch_RefreshesAtLevelOwned covers the LevelOwned variant.
func TestPostTabSwitch_RefreshesAtLevelOwned(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:            "PodList",
		{Group: "apps", Version: "v1", Resource: "replicasets"}: "ReplicaSetList",
	}
	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs)

	m := newTabSwitchTestModel(t, model.LevelOwned)
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", APIVersion: "apps/v1", Namespaced: true}
	m.nav.ResourceName = "my-deploy"
	m.client = k8s.NewTestClient(clientfake.NewClientset(), dyn)

	cmd := m.postTabSwitchCmd()
	if cmd == nil {
		t.Fatal("postTabSwitchCmd returned nil at LevelOwned/modeExplorer")
	}

	sawRefresh := false
	for _, c := range flattenBatch(cmd) {
		if loaded, ok := c().(ownedLoadedMsg); ok && !loaded.forPreview {
			sawRefresh = true
		}
	}
	assert.True(t, sawRefresh,
		"postTabSwitchCmd at LevelOwned must fire a non-preview owned-refresh fetch")
}

// TestPostTabSwitch_RefreshesAtLevelContainers covers the LevelContainers
// variant — the parent pod's containers may have changed (e.g., pod
// restarted with a different image after a deployment edit on another tab).
func TestPostTabSwitch_RefreshesAtLevelContainers(t *testing.T) {
	t.Parallel()

	m := newTabSwitchTestModel(t, model.LevelContainers)
	m.nav.OwnedName = "my-pod"
	m.client = k8s.NewTestClient(clientfake.NewClientset(), nil)

	cmd := m.postTabSwitchCmd()
	if cmd == nil {
		t.Fatal("postTabSwitchCmd returned nil at LevelContainers/modeExplorer")
	}

	sawRefresh := false
	for _, c := range flattenBatch(cmd) {
		if loaded, ok := c().(containersLoadedMsg); ok && !loaded.forPreview {
			sawRefresh = true
		}
	}
	assert.True(t, sawRefresh,
		"postTabSwitchCmd at LevelContainers must fire a non-preview containers-refresh fetch")
}

// TestPostTabSwitch_DoesNotRefreshAtLevelClusters keeps the cluster picker
// view passive on tab switch — the cluster list itself doesn't go stale
// within a session and discovery is heavy.
func TestPostTabSwitch_DoesNotRefreshAtLevelClusters(t *testing.T) {
	t.Parallel()

	m := newTabSwitchTestModel(t, model.LevelClusters)
	m.client = k8s.NewTestClient(clientfake.NewClientset(), nil)

	cmd := m.postTabSwitchCmd()
	for _, c := range flattenBatch(cmd) {
		msg := c()
		if _, ok := msg.(resourcesLoadedMsg); ok {
			t.Errorf("LevelClusters tab switch must not fire resourcesLoadedMsg")
		}
		if _, ok := msg.(ownedLoadedMsg); ok {
			t.Errorf("LevelClusters tab switch must not fire ownedLoadedMsg")
		}
		if _, ok := msg.(containersLoadedMsg); ok {
			t.Errorf("LevelClusters tab switch must not fire containersLoadedMsg")
		}
	}
}

// newTabSwitchTestModel builds a Model wired for postTabSwitchCmd tests.
// Empty middleItems means loadPreview's selection-gated sub-cmds short-
// circuit, so each test can focus on whether the refresh-on-switch was
// added without false signals from preview fetches.
//
// Workers are started so scheduleK8sCall futures resolve; t.Cleanup
// registers StopWorkers so workers exit when the test ends.
func newTabSwitchTestModel(t *testing.T, level model.Level) Model {
	t.Helper()
	m := Model{
		nav: model.NavigationState{
			Level:   level,
			Context: "test-ctx",
		},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		execMu:              &sync.Mutex{},
		namespace:           "default",
		scheduler:           scheduler.New(0),
		reqCtx:              context.Background(),
		mode:                modeExplorer,
	}
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)
	return m
}
