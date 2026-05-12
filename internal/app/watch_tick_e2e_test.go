package app

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	clientfake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// End-to-end repro of the user's "deleted pod still shown" bug.
//
// Setup: fake K8s with one pod, model viewing the pods list, watch on.
// First tick caches the pod. Then we remove it from K8s (simulating the
// pod finishing termination). Second tick must produce middleItems
// without the gone pod — proving the cache invalidation actually
// surfaces fresh API state and not the stale snapshot.
func TestWatchTickRemovesDeletedPodFromList(t *testing.T) {
	t.Parallel()

	// Fake k8s with one pod present.
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:                     "PodList",
		{Group: "", Version: "v1", Resource: "events"}:                   "EventList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:  "PodMetricsList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}: "NodeMetricsList",
	}
	pod := &unstructured.Unstructured{}
	pod.SetUnstructuredContent(map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":              "doomed-pod",
			"namespace":         "default",
			"creationTimestamp": time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
		},
		"status": map[string]any{"phase": "Running"},
	})
	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, pod)

	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			Context:      "test-ctx",
			ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true},
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
		watchMode:           true,
	}
	m.client = k8s.NewTestClient(clientfake.NewClientset(), dyn)
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)

	// First tick: prime middleItems via the full message pipeline so
	// the model state mirrors what a real first-load looks like.
	mAny, cmd := m.updateWatchTick(watchTickMsg{})
	m = mAny.(Model)
	cmds := flattenBatch(cmd)
	for _, c := range cmds {
		msg := c()
		if loaded, ok := msg.(resourcesLoadedMsg); ok {
			modelAny, _ := m.updateResourcesLoaded(loaded)
			m = modelAny.(Model)
		}
	}
	if !assertPodPresent(t, m.middleItems, "doomed-pod", "first tick must observe the live pod") {
		return
	}

	// Pod terminates: remove it from the fake API.
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if err := dyn.Resource(gvr).Namespace("default").Delete(context.Background(), "doomed-pod", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("fake delete failed: %v", err)
	}

	// Second tick: must refetch and reflect that the pod is gone.
	mAny, cmd = m.updateWatchTick(watchTickMsg{})
	m = mAny.(Model)
	for _, c := range flattenBatch(cmd) {
		msg := c()
		if loaded, ok := msg.(resourcesLoadedMsg); ok {
			modelAny, _ := m.updateResourcesLoaded(loaded)
			m = modelAny.(Model)
		}
	}

	for _, item := range m.middleItems {
		assert.NotEqual(t, "doomed-pod", item.Name,
			"second watch tick must drop the deleted pod, not serve the cached snapshot")
	}
	assert.Empty(t, m.middleItems,
		"fake API has no pods after delete; middleItems must be empty")
}

// TestWatchTickRefreshesPreviewAtLevelResourceTypes covers the extension
// of the per-tab cache fix: the right-pane preview at LevelResourceTypes
// must refresh on every watch tick, not just on tab switch. Without this,
// any external cluster mutation (kubectl delete, an operator, another
// process) leaves the hovered resource type's preview stuck on the old
// snapshot until the user tab-switches or drills in.
//
// Setup: model at LevelResourceTypes hovering Pods; cache pre-populated
// with a "stale-pod" snapshot + a matching fingerprint, so the
// hover-cycle shortcut in loadResources(forPreview=true) would normally
// serve the stale items synchronously. Fake K8s has a single "live-pod".
// One watch tick must drive the cascade through resourceTypesMsg →
// loadPreview → real fetch and land "live-pod" in rightItems.
func TestWatchTickRefreshesPreviewAtLevelResourceTypes(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:                     "PodList",
		{Group: "", Version: "v1", Resource: "events"}:                   "EventList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:  "PodMetricsList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}: "NodeMetricsList",
	}
	livePod := &unstructured.Unstructured{}
	livePod.SetUnstructuredContent(map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":              "live-pod",
			"namespace":         "default",
			"creationTimestamp": time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
		},
		"status": map[string]any{"phase": "Running"},
	})
	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, livePod)

	podsRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResourceTypes,
			Context: "test-ctx",
		},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: map[string][]model.ResourceTypeEntry{"test-ctx": {podsRT}},
		// Tell refreshCurrentLevel that CRD discovery is already in flight
		// so it doesn't fire a second discoverAPIResources call — the fake
		// dynamic client only knows about Pod/Event/Metrics GVRs (the test
		// is focused on the preview fetch, not CRD discovery).
		discoveringContexts: map[string]bool{"test-ctx": true},
		execMu:              &sync.Mutex{},
		namespace:           "default",
		scheduler:           scheduler.New(0),
		reqCtx:              context.Background(),
		watchMode:           true,
	}
	// middleItems must mirror BuildSidebarItems' output so updateResourceTypes
	// (which overwrites middleItems with that result) doesn't move our cursor
	// onto a synthetic dashboard / group-header row. The cursor indexes into
	// visibleMiddleItems (which injects group headers and may collapse
	// groups), not middleItems directly.
	m.middleItems = model.BuildSidebarItems([]model.ResourceTypeEntry{podsRT})
	m.allGroupsExpanded = true // keep groups expanded so visibleMiddleItems contains Pods
	visible := m.visibleMiddleItems()
	cursorSet := false
	for i, item := range visible {
		if item.Extra == podsRT.ResourceRef() {
			m.setCursor(i)
			cursorSet = true
			break
		}
	}
	if !cursorSet {
		t.Fatalf("test setup: Pods row not found in visibleMiddleItems; visible=%+v", visible)
	}
	m.client = k8s.NewTestClient(clientfake.NewClientset(), dyn)
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)

	// Mimic the state of a tab where the user previously hovered Pods
	// and the per-tab cache locked in a since-stale snapshot.
	cacheKey := "test-ctx/pods"
	m.itemCache[cacheKey] = []model.Item{{Name: "stale-pod"}}
	m.cacheFingerprints[cacheKey] = m.fetchFingerprint()
	m.rightItems = []model.Item{{Name: "stale-pod"}}

	// Run one watch tick through the full message pipeline. The cascade
	// is resourceTypesMsg → updateResourceTypes → loadPreview → fetch →
	// resourcesLoadedMsg{forPreview} → updateResourcesLoadedPreview, so
	// the test drains both message kinds.
	mAny, cmd := m.updateWatchTick(watchTickMsg{})
	m = mAny.(Model)
	for _, c := range flattenBatch(cmd) {
		switch typed := c().(type) {
		case resourceTypesMsg:
			modelAny, follow := m.updateResourceTypes(typed)
			m = modelAny.(Model)
			for _, fc := range flattenBatch(follow) {
				if loaded, ok := fc().(resourcesLoadedMsg); ok {
					modelAny, _ = m.updateResourcesLoaded(loaded)
					m = modelAny.(Model)
				}
			}
		case resourcesLoadedMsg:
			modelAny, _ := m.updateResourcesLoaded(typed)
			m = modelAny.(Model)
		}
	}

	for _, item := range m.rightItems {
		assert.NotEqual(t, "stale-pod", item.Name,
			"watch tick at LevelResourceTypes must replace cached stale preview items, not serve them")
	}
	sawLive := false
	for _, item := range m.rightItems {
		if item.Name == "live-pod" {
			sawLive = true
			break
		}
	}
	assert.True(t, sawLive,
		"watch tick at LevelResourceTypes must surface live pods into the preview (rightItems=%+v)", m.rightItems)
}

// flattenBatch unwraps a tea.Cmd that may be a tea.Batch into a slice of
// individual commands. Bubble Tea's BatchMsg holds a list of cmds; we
// invoke each to drive the test pipeline manually.
func flattenBatch(cmd tea.Cmd) []tea.Cmd {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		return []tea.Cmd(batch)
	}
	// Single cmd whose msg isn't a batch — wrap it so the test loop can
	// inspect the result uniformly.
	return []tea.Cmd{func() tea.Msg { return msg }}
}

func assertPodPresent(t *testing.T, items []model.Item, name, why string) bool {
	t.Helper()
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	t.Errorf("%s — pod %q not in middleItems (have %d items)", why, name, len(items))
	return false
}

// Suppress unused import warning for corev1 if K8s API surface shifts.
var _ = corev1.Pod{}
