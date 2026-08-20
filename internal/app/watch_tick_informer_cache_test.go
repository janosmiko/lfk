package app

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clienttesting "k8s.io/client-go/testing"

	dynfake "k8s.io/client-go/dynamic/fake"
	clientfake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// podListActionCount counts "list pods" actions the fake dynamic client has
// observed, mirroring the k8s package's own helper. Used to prove a watch
// tick was served from the informer cache rather than a fresh LIST.
func podListActionCount(actions []clienttesting.Action) int {
	n := 0
	for _, a := range actions {
		// Group must be empty: metrics.k8s.io PodMetrics lists share the
		// "pods" resource name and would inflate the count.
		if a.GetVerb() == "list" && a.GetResource().Resource == "pods" && a.GetResource().Group == "" {
			n++
		}
	}
	return n
}

func informerCacheTestPod(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
			},
			"status": map[string]any{"phase": "Running"},
		},
	}
}

// TestWatchTickDropsDeletedPodViaInformerCache is the app-level regression
// guard for issue #646: a watch tick's PreferCache-driven refresh must
// serve from the informer cache once warm, and a deletion observed by the
// watch must drop the pod from middleItems without a fresh LIST.
func TestWatchTickDropsDeletedPodViaInformerCache(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:                     "PodList",
		{Group: "", Version: "v1", Resource: "events"}:                   "EventList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:  "PodMetricsList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}: "NodeMetricsList",
	}
	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, informerCacheTestPod("doomed-pod", "default"))

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
		reqCtx:              t.Context(),
		watchMode:           true,
	}
	m.client = k8s.NewTestClient(clientfake.NewClientset(), dyn)
	m.client.SetInformerCacheMode(k8s.InformerCacheAlways)
	t.Cleanup(m.client.Shutdown)
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)

	runTick := func() {
		mAny, cmd := m.updateWatchTick(watchTickMsg{})
		m = mAny.(Model)
		for _, c := range flattenBatch(cmd) {
			if loaded, ok := c().(resourcesLoadedMsg); ok {
				modelAny, _ := m.updateResourcesLoaded(loaded)
				m = modelAny.(Model)
			}
		}
	}

	runTick()
	if !assertPodPresent(t, m.middleItems, "doomed-pod", "first tick must observe the live pod") {
		return
	}
	// markHot's own informer issues one internal LIST asynchronously.
	// Wait for it to land so it isn't mistaken for a spurious extra call.
	listsAfterWarmup := waitForListActionCountToSettle(t, dyn)
	// The fake watch only delivers events sent after it registers. Delete
	// before that and the informer never sees it.
	waitForPodWatchAction(t, dyn)

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	require.NoError(t, dyn.Resource(gvr).Namespace("default").Delete(t.Context(), "doomed-pod", metav1.DeleteOptions{}))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runTick()
		if len(m.middleItems) == 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	assert.Empty(t, m.middleItems, "second tick must drop the deleted pod")
	assert.Equal(t, listsAfterWarmup, podListActionCount(dyn.Actions()),
		"a warm, always-mode cache must serve watch ticks without an extra LIST")
}

// waitForListActionCountToSettle polls until the pod-list count stays
// unchanged across several consecutive checks, bounded by a deadline. A
// stricter bar than "unchanged once" is needed under -race / full-suite
// CPU contention, where the informer's own internal LIST can lag well
// behind a single 20ms poll.
func waitForPodWatchAction(t *testing.T, dyn *dynfake.FakeDynamicClient) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, a := range dyn.Actions() {
			if a.GetVerb() == "watch" && a.GetResource().Resource == "pods" && a.GetResource().Group == "" {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("informer pod watch never registered")
}

func waitForListActionCountToSettle(t *testing.T, dyn *dynfake.FakeDynamicClient) int {
	t.Helper()
	const stableReadsRequired = 5
	deadline := time.Now().Add(5 * time.Second)
	stable := 0
	last := podListActionCount(dyn.Actions())
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		current := podListActionCount(dyn.Actions())
		if current == last {
			stable++
			if stable >= stableReadsRequired {
				return current
			}
			continue
		}
		stable = 0
		last = current
	}
	return last
}
