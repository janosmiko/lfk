package app

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	dynfake "k8s.io/client-go/dynamic/fake"
	clientfake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// ageTokenRe matches the trailing "<n><unit>" age token in a rendered
// table row, e.g. "0s" or "3m".
var ageTokenRe = regexp.MustCompile(`(\d+)([smhdy])\b`)

func ageInRow(t *testing.T, view, needle string) int {
	t.Helper()
	for line := range strings.SplitSeq(view, "\n") {
		if !strings.Contains(line, needle) {
			continue
		}
		m := ageTokenRe.FindStringSubmatch(line)
		if m == nil {
			continue // breadcrumb/title also contains needle but has no age token
		}
		n, err := strconv.Atoi(m[1])
		require.NoError(t, err)
		return n
	}
	t.Fatalf("no row with an age token containing %q found in view:\n%s", needle, view)
	return 0
}

// ageMemoTestPod builds a minimal unstructured Pod with an explicit
// creation timestamp, for controlling the age the rendered view shows.
func ageMemoTestPod(name, namespace string, created time.Time) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": created.UTC().Format(time.RFC3339),
			},
			"status": map[string]any{"phase": "Running"},
		},
	}
}

// TestWatchTickAgeAdvancesOnMemoizedItems is the app-level regression guard
// for the informer cache's per-item memo: even when a watch tick's list is
// served entirely from cached model.Item rows (no rebuild, since nothing
// changed), the rendered AGE column must still advance -- LiveAge recomputes
// from CreatedAt at render time, so a memoized item must not freeze it.
func TestWatchTickAgeAdvancesOnMemoizedItems(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:                     "PodList",
		{Group: "", Version: "v1", Resource: "events"}:                   "EventList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:  "PodMetricsList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}: "NodeMetricsList",
	}
	created := time.Now().Add(-400 * time.Millisecond)
	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, ageMemoTestPod("steady-pod", "default", created))

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
		selectedNamespaces:  make(map[string]bool),
		yamlView:            yamlViewState{collapsed: make(map[string]bool)},
		execMu:              &sync.Mutex{},
		namespace:           "default",
		scheduler:           scheduler.New(0),
		reqCtx:              t.Context(),
		watchMode:           true,
		width:               120,
		height:              40,
		mode:                modeExplorer,
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

	// Tick 1 skips the cache branch (markHot's own informer start hasn't
	// synced yet). Tick 2 is the first read through the memo, populating
	// it. Only tick 3 is a true memo hit.
	runTick()
	if !assertPodPresent(t, m.middleItems, "steady-pod", "first tick must observe the live pod") {
		return
	}
	runTick()
	if !assertPodPresent(t, m.middleItems, "steady-pod", "second tick must still observe the live pod") {
		return
	}
	firstView := m.View().Content
	firstAge := ageInRow(t, stripANSI(firstView), "steady-pod")

	time.Sleep(1100 * time.Millisecond)
	runTick()

	secondView := m.View().Content
	secondAge := ageInRow(t, stripANSI(secondView), "steady-pod")

	assert.Greater(t, secondAge, firstAge,
		"AGE must advance between ticks even when the informer cache serves a memoized item")
}
