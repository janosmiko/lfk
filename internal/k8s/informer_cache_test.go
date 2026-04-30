package k8s

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clienttesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/model"
)

// podRT is the ResourceTypeEntry used by every informer-cache test in this file.
// Pods are the resource that motivated issue #86 (~7k of them on the bug
// reporter's cluster) and exercise the namespaced + dynamic-client path
// GetResources cares about.
var podRT = model.ResourceTypeEntry{
	APIGroup:   "",
	APIVersion: "v1",
	Resource:   "pods",
	Kind:       "Pod",
	Namespaced: true,
}

// pod builds a minimal *unstructured.Unstructured for a pod with name+namespace.
func pod(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": "2026-04-01T00:00:00Z",
			},
		},
	}
}

// listActionCount returns how many list-pods actions the fake dynamic client
// has observed. The informer issues exactly one initial list per (context,
// GVR); every subsequent GetResources call after the cache is warm should
// leave this number unchanged.
func listActionCount(actions []clienttesting.Action) int {
	n := 0
	for _, a := range actions {
		if a.GetVerb() == "list" && a.GetResource().Resource == "pods" {
			n++
		}
	}
	return n
}

// TestGetResources_InformerCache_NamespaceSwitchHitsCache is the regression
// guard for issue #86. Switching the namespace filter must NOT issue a fresh
// LIST against the apiserver once the informer has synced — that was the
// 7k-pod lag the user was seeing.
func TestGetResources_InformerCache_NamespaceSwitchHitsCache(t *testing.T) {
	dc := newFakeDynClient(
		pod("api-1", "team-a"),
		pod("api-2", "team-a"),
		pod("worker-1", "team-b"),
	)
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAlways)
	t.Cleanup(c.Shutdown)

	// First list — primes the informer (the underlying watch fires one LIST).
	itemsAll, err := c.GetResources(context.Background(), "", "", podRT)
	require.NoError(t, err)
	require.Len(t, itemsAll, 3)

	listsAfterWarmup := listActionCount(dc.Actions())
	require.GreaterOrEqual(t, listsAfterWarmup, 1, "informer should have issued at least one LIST to populate the cache")

	// Switch to team-a, then team-b, then back to all-namespaces. Each call
	// is the moment the user pressed the namespace selector — under the
	// pre-#86 code these would have round-tripped to the apiserver.
	itemsA, err := c.GetResources(context.Background(), "", "team-a", podRT)
	require.NoError(t, err)
	itemsB, err := c.GetResources(context.Background(), "", "team-b", podRT)
	require.NoError(t, err)
	itemsAll2, err := c.GetResources(context.Background(), "", "", podRT)
	require.NoError(t, err)

	assert.Equal(t, []string{"api-1", "api-2"}, []string{itemsA[0].Name, itemsA[1].Name})
	assert.Equal(t, []string{"worker-1"}, []string{itemsB[0].Name})
	assert.Len(t, itemsAll2, 3)

	listsAfterSwitches := listActionCount(dc.Actions())
	assert.Equal(t, listsAfterWarmup, listsAfterSwitches,
		"namespace switching must not issue extra LIST calls — that is the whole point of issue #86")
}

// TestGetResources_InformerCache_OffModeHitsApiserverEveryTime verifies
// that explicit "off" mode always round-trips to the apiserver — useful
// when the operator has chosen to keep kubectl-equivalent semantics.
func TestGetResources_InformerCache_OffModeHitsApiserverEveryTime(t *testing.T) {
	dc := newFakeDynClient(
		pod("api-1", "team-a"),
	)
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheOff)

	for range 3 {
		_, err := c.GetResources(context.Background(), "", "team-a", podRT)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, listActionCount(dc.Actions()),
		"with informer_cache=off, each GetResources call should LIST the apiserver as before")
}

// TestGetResources_InformerCache_PicksUpDeletes verifies that the cache
// stays fresh under the watch — a deletion observed by the informer must
// drop out of subsequent list results without forcing a refresh round trip.
// This is the property that lets us serve cached results without falling
// back to "stale data" semantics: deleted pods don't linger.
func TestGetResources_InformerCache_PicksUpDeletes(t *testing.T) {
	dc := newFakeDynClient(
		pod("api-1", "team-a"),
		pod("api-2", "team-a"),
	)
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAlways)
	t.Cleanup(c.Shutdown)

	// Warm the cache.
	items, err := c.GetResources(context.Background(), "", "team-a", podRT)
	require.NoError(t, err)
	require.Len(t, items, 2)

	// Delete api-2 via the dynamic client. The informer's watch sees this
	// asynchronously, so we poll the cached list until it converges instead
	// of asserting on the very next call.
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	require.NoError(t, dc.Resource(gvr).Namespace("team-a").Delete(context.Background(), "api-2", metav1.DeleteOptions{}))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		items, err = c.GetResources(context.Background(), "", "team-a", podRT)
		require.NoError(t, err)
		if len(items) == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	require.Len(t, items, 1, "watch should have removed api-2 from the cache")
	assert.Equal(t, "api-1", items[0].Name)
}

// TestSetInformerCacheMode_NormalizesInput guards the API contract that
// callers passing mode strings with stray casing or whitespace resolve
// to the right mode rather than silently falling back to auto. Without
// this, a programmatic caller like an integration test or a third-party
// embedder would get the auto fallback on a typo or trim mismatch.
func TestSetInformerCacheMode_NormalizesInput(t *testing.T) {
	tests := []struct {
		name string
		in   InformerCacheMode
		want InformerCacheMode
	}{
		{"lowercase off matches", "off", InformerCacheOff},
		{"uppercase ALWAYS lowered", "ALWAYS", InformerCacheAlways},
		{"mixed case Auto lowered", "Auto", InformerCacheAuto},
		{"leading whitespace trimmed", "  always", InformerCacheAlways},
		{"trailing whitespace trimmed", "off  ", InformerCacheOff},
		{"unknown value falls back to auto", "maybe", InformerCacheAuto},
		{"empty string falls back to auto", "", InformerCacheAuto},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewTestClient(nil, nil)
			c.SetInformerCacheMode(tc.in)
			assert.Equal(t, tc.want, c.informerMode)
			c.Shutdown()
		})
	}
}

// TestInformerCache_StopIsIdempotent guards against a regression where
// Shutdown() panics if called twice (e.g. once from a deferred client.Shutdown
// and once explicitly in a test cleanup). Cheap to enforce, easy to break.
func TestInformerCache_StopIsIdempotent(t *testing.T) {
	dc := newFakeDynClient()
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAlways)

	c.Shutdown()
	c.Shutdown() // must not panic
}

// TestInformerCache_ConcurrentStopAndDemoteNoPanic exercises the race
// between Stop (called from main.go's defer client.Shutdown()) and
// stopOne (called when auto-demote fires inside a still-in-flight
// GetResources). Both paths close the same stopCh; without coordination
// the second one panics with "close of closed channel". The
// ic.stopped early-out in stopOne is what prevents that, and this test
// is the regression guard — run with -race for full effect.
func TestInformerCache_ConcurrentStopAndDemoteNoPanic(t *testing.T) {
	// One pod per call so cached lists trivially fall under any
	// reasonable demote threshold.
	dc := newFakeDynClient(pod("a", "ns"))
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAuto)
	withTunedThresholds(c, 1, 5, 1)

	// Warm up: promotes on first list (1 item >= promoteAt=1).
	_, err := c.GetResources(context.Background(), "", "", podRT)
	require.NoError(t, err)
	require.True(t, c.informers.isPromoted("", podGVR()))

	// Race: one goroutine triggers a cached list (which auto-demotes,
	// trying to stopOne), another calls Shutdown (which Stop()s every
	// remaining entry). Both want to close the same stopCh.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = c.GetResources(context.Background(), "", "", podRT)
	}()
	go func() {
		defer wg.Done()
		c.Shutdown()
	}()
	wg.Wait()
	// Reaching this line without a panic is the assertion.
}

// TestInformerCache_StopWaitsForGoroutines verifies the docstring promise
// that Stop blocks until informer goroutines have exited. We can't observe
// goroutine ids directly, but we can sample runtime.NumGoroutine before and
// after to confirm the count returned to baseline rather than leaking the
// inf.Run / WaitForCacheSync goroutines past Shutdown.
func TestInformerCache_StopWaitsForGoroutines(t *testing.T) {
	baseline := runtime.NumGoroutine()

	dc := newFakeDynClient(pod("a", "ns"))
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAlways)

	// Force the informer to start by issuing a list.
	_, err := c.GetResources(context.Background(), "", "", podRT)
	require.NoError(t, err)
	require.Greater(t, runtime.NumGoroutine(), baseline,
		"expected new goroutines for inf.Run + sync watcher")

	c.Shutdown()

	// Goroutines exit asynchronously even after their stop channel
	// closes; give the scheduler a brief window to run their defers.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= baseline+1 { // +1 tolerance for runtime jitter
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Shutdown did not drain informer goroutines: baseline=%d, after=%d",
		baseline, runtime.NumGoroutine())
}

// withTunedThresholds dials the auto promote/demote thresholds down so a
// test can drive transitions with handful-sized fixtures rather than 1000+
// pods. Using small numbers keeps fixtures readable and the test fast,
// while still exercising the same state-machine paths the production
// thresholds drive.
func withTunedThresholds(c *Client, promoteAt, demoteBelow, demoteAfterN int) {
	c.informers.promoteAt = promoteAt
	c.informers.demoteBelow = demoteBelow
	c.informers.demoteAfterN = demoteAfterN
}

// TestGetResources_InformerCache_AutoPromote drives the auto-promote arc
// of the state machine: a direct list returning ≥ promoteAt items must
// flip the (context, GVR) into informer mode, so the next call goes to
// the cache instead of the apiserver. This is the path that gives big
// clusters the issue #86 win without forcing users to find a config knob.
func TestGetResources_InformerCache_AutoPromote(t *testing.T) {
	pods := []*unstructured.Unstructured{
		pod("api-1", "team-a"),
		pod("api-2", "team-a"),
		pod("api-3", "team-a"),
		pod("worker-1", "team-b"),
	}
	dc := newFakeDynClient(toRuntimeObjects(pods)...)
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAuto)
	t.Cleanup(c.Shutdown)
	// Promote at 4 items so the fixture above triggers it; demote thresholds
	// don't matter for this test but must satisfy demoteBelow < promoteAt.
	withTunedThresholds(c, 4, 2, 3)

	// First call: direct LIST against the apiserver (cache not yet warm).
	// Result has 4 items, which crosses promoteAt.
	_, err := c.GetResources(context.Background(), "", "", podRT)
	require.NoError(t, err)
	listsAfterFirst := listActionCount(dc.Actions())
	require.Equal(t, 1, listsAfterFirst, "first auto-mode call should LIST the apiserver directly")
	require.True(t, c.informers.isPromoted("", podGVR()),
		"4 items >= promoteAt=4 must flip the GVR to informer mode")

	// Second call: routed through the informer. The informer's own initial
	// LIST counts as one apiserver call; subsequent namespace switches do
	// not. Wait briefly for the watch to populate before asserting items.
	_, err = c.GetResources(context.Background(), "", "team-a", podRT)
	require.NoError(t, err)

	// Third call (different namespace) — must not add another apiserver list.
	listsBefore := listActionCount(dc.Actions())
	_, err = c.GetResources(context.Background(), "", "team-b", podRT)
	require.NoError(t, err)
	assert.Equal(t, listsBefore, listActionCount(dc.Actions()),
		"after promotion, namespace switching should be served from the cache")
}

// TestGetResources_InformerCache_AutoDemote drives the auto-demote arc:
// once a (context, GVR) has been promoted, sustained cached lists below
// the demote threshold must close the watch and return to direct lists,
// so a one-off large list does not leave a permanent watch behind.
func TestGetResources_InformerCache_AutoDemote(t *testing.T) {
	dc := newFakeDynClient(
		pod("api-1", "team-a"),
		pod("api-2", "team-a"),
	)
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAuto)
	t.Cleanup(c.Shutdown)
	withTunedThresholds(c, 1, 5, 3)
	// promoteAt=1 means even tiny lists promote; demoteBelow=5 means cached
	// lists in this fixture (2 items) always count as "small"; demoteAfterN=3
	// is the minimum to exercise the consecutive-call counter.

	// Warm up: first list promotes the GVR (2 items >= promoteAt=1).
	_, err := c.GetResources(context.Background(), "", "", podRT)
	require.NoError(t, err)
	require.True(t, c.informers.isPromoted("", podGVR()), "expected promotion after first list")

	// Three more cached lists, each below demoteBelow. The third must trip
	// the demote: state flips back to direct and the watch is torn down.
	for i := range 3 {
		_, err := c.GetResources(context.Background(), "", "", podRT)
		require.NoError(t, err, "cached list iteration %d", i)
	}
	assert.False(t, c.informers.isPromoted("", podGVR()),
		"3 consecutive cached lists below demoteBelow should auto-demote back to direct")

	// Verify the watch was actually closed by re-promoting and observing
	// that a fresh informer was started — i.e. demote is not a no-op.
	listsBefore := listActionCount(dc.Actions())
	_, err = c.GetResources(context.Background(), "", "", podRT)
	require.NoError(t, err)
	listsAfter := listActionCount(dc.Actions())
	assert.Greater(t, listsAfter, listsBefore,
		"after demote, the next call should issue a fresh apiserver LIST")
}

// podGVR is the GroupVersionResource for core/v1 pods, used by the auto
// tests when poking at internal cache state. Defined as a function (not a
// var) so the value is unambiguous at the call site without an init order
// dance.
func podGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
}

// TestListItems_MemoizesPerItem is the regression guard for the
// per-call DeepCopy + buildResourceItem cost on a 6k-pod list. After
// the warmup, a follow-up listItems call against an unchanged indexer
// must reuse every cached row — build is invoked zero times.
func TestListItems_MemoizesPerItem(t *testing.T) {
	dc := newFakeDynClient(
		pod("api-1", "team-a"),
		pod("api-2", "team-a"),
	)
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAlways)
	t.Cleanup(c.Shutdown)

	gvr := podGVR()
	buildCalls := 0
	build := func(obj *unstructured.Unstructured) model.Item {
		buildCalls++
		return model.Item{Name: obj.GetName(), Namespace: obj.GetNamespace()}
	}

	// Warmup: starts the informer, walks indexer, runs build per item,
	// stores memo. GetResources is deliberately not used here so we
	// don't pre-populate the memo with the production build.
	first, builds, err := c.informers.listItems(context.Background(), "", gvr, "", build)
	require.NoError(t, err)
	require.Len(t, first, 2)
	assert.Equal(t, 2, builds, "warmup builds every item once")
	assert.Equal(t, 2, buildCalls)

	second, builds, err := c.informers.listItems(context.Background(), "", gvr, "", build)
	require.NoError(t, err)
	require.Len(t, second, 2)
	assert.Equal(t, 0, builds,
		"unchanged indexer must reuse every cached row — that's the optimization")
	assert.Equal(t, 2, buildCalls, "build must not run again when nothing changed")
}

// TestListItems_RebuildsOnlyChangedItems is the per-item invalidation
// guard. When the informer observes a watch event for one pod, only
// that pod's row should rebuild — the rest stay cached. This is the
// property that makes the memo useful on a busy cluster: pod-A churn
// doesn't force pod-B through buildResourceItem.
func TestListItems_RebuildsOnlyChangedItems(t *testing.T) {
	dc := newFakeDynClient(
		pod("api-1", "team-a"),
		pod("api-2", "team-a"),
		pod("api-3", "team-a"),
	)
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAlways)
	t.Cleanup(c.Shutdown)

	gvr := podGVR()
	build := func(obj *unstructured.Unstructured) model.Item {
		return model.Item{Name: obj.GetName(), Namespace: obj.GetNamespace()}
	}

	// Warmup: builds all 3.
	_, builds, err := c.informers.listItems(context.Background(), "", gvr, "", build)
	require.NoError(t, err)
	require.Equal(t, 3, builds)

	// Mutate exactly one pod: the watch bumps its individual RV; the
	// other two pods' resourceVersion stays unchanged.
	require.NoError(t, dc.Resource(gvr).Namespace("team-a").Delete(context.Background(), "api-2", metav1.DeleteOptions{}))

	// Poll until the indexer reflects the delete (watch is async).
	// Once it has, the next listItems should return 2 items and have
	// rebuilt zero of them — both api-1 and api-3 still match their
	// memo entries.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		items, builds, err := c.informers.listItems(context.Background(), "", gvr, "", build)
		require.NoError(t, err)
		if len(items) == 2 {
			assert.Equal(t, 0, builds,
				"only api-2 changed (deleted) — api-1 and api-3 should reuse their memo entries")
			names := []string{items[0].Name, items[1].Name}
			assert.Contains(t, names, "api-1")
			assert.Contains(t, names, "api-3")
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("indexer never converged on the post-delete state")
}

// TestListItems_RebuildsUpdatedItem verifies the targeted-rebuild path
// when an item's resourceVersion changes (a Modified watch event):
// only that one item should run through build, neighbours stay cached.
func TestListItems_RebuildsUpdatedItem(t *testing.T) {
	dc := newFakeDynClient(
		pod("api-1", "team-a"),
		pod("api-2", "team-a"),
	)
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAlways)
	t.Cleanup(c.Shutdown)

	gvr := podGVR()
	buildNames := []string{}
	build := func(obj *unstructured.Unstructured) model.Item {
		buildNames = append(buildNames, obj.GetName())
		return model.Item{Name: obj.GetName(), Namespace: obj.GetNamespace()}
	}

	_, _, err := c.informers.listItems(context.Background(), "", gvr, "", build)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"api-1", "api-2"}, buildNames)
	buildNames = buildNames[:0]

	// Update api-2 with an explicit resourceVersion bump. The dynamic
	// fake doesn't auto-stamp resourceVersion on Update, so we have to
	// supply one ourselves — the watch event then carries the new RV
	// to the informer, which in turn invalidates the memo entry.
	updated := pod("api-2", "team-a")
	meta := updated.Object["metadata"].(map[string]any)
	meta["labels"] = map[string]any{"updated": "true"}
	meta["resourceVersion"] = "2"
	_, err = dc.Resource(gvr).Namespace("team-a").Update(context.Background(), updated, metav1.UpdateOptions{})
	require.NoError(t, err)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		buildNames = buildNames[:0]
		_, builds, err := c.informers.listItems(context.Background(), "", gvr, "", build)
		require.NoError(t, err)
		if builds == 1 && len(buildNames) == 1 && buildNames[0] == "api-2" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("update never triggered exactly one rebuild for api-2: last buildNames=%v", buildNames)
}

// toRuntimeObjects converts a slice of *unstructured to []runtime.Object so
// it can be passed to newFakeDynClient's variadic parameter. Saves a
// for-loop in every fixture-heavy test below.
func toRuntimeObjects(in []*unstructured.Unstructured) []apiruntime.Object {
	out := make([]apiruntime.Object, 0, len(in))
	for _, u := range in {
		out = append(out, u)
	}
	return out
}

// namespaceObj builds an unstructured Namespace resource. Used by the
// cluster-scoped tests below — namespaces are themselves cluster-scoped
// (Namespaced: false), so they exercise the memoKey "/<name>" branch
// that doesn't fire for namespaced resources like pods.
func namespaceObj(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name":              name,
				"creationTimestamp": "2026-04-01T00:00:00Z",
			},
		},
	}
}

// TestListItems_ClusterScopedResource verifies the cluster-scoped path
// through applyMemo. memoKey produces "/<name>" for objects with no
// namespace; the test confirms two calls (a) return correct items and
// (b) hit the memo on the second call, just like namespaced resources.
// Without this we'd be silently regressing on Namespaces, Nodes, CRDs,
// etc., which all flow through the same listItems API.
func TestListItems_ClusterScopedResource(t *testing.T) {
	dc := newFakeDynClient(
		namespaceObj("default"),
		namespaceObj("kube-system"),
	)
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAlways)
	t.Cleanup(c.Shutdown)

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	build := func(obj *unstructured.Unstructured) model.Item {
		return model.Item{Name: obj.GetName(), Kind: "Namespace"}
	}

	first, builds, err := c.informers.listItems(context.Background(), "", gvr, "", build)
	require.NoError(t, err)
	require.Len(t, first, 2)
	assert.Equal(t, 2, builds, "warmup builds every cluster-scoped item once")

	second, builds, err := c.informers.listItems(context.Background(), "", gvr, "", build)
	require.NoError(t, err)
	require.Len(t, second, 2)
	assert.Equal(t, 0, builds,
		"cluster-scoped items must memoize the same way namespaced ones do")

	// Sanity check: the cache key for a cluster-scoped object starts with
	// "/" — that's what keeps "/foo" from colliding with "ns/foo".
	entry, ok := c.informers.entries[""][gvr]
	require.True(t, ok, "informer entry must exist after listItems")
	require.Contains(t, entry.memo, "/default")
	require.Contains(t, entry.memo, "/kube-system")
}

// TestApplyMemo_PrunesDeletedKeysOnAllNamespacesList verifies the memo's
// growth-bounding sweep. After a pod has been removed from the indexer,
// the next all-namespaces list must drop its memo entry — otherwise
// long sessions on busy clusters leak entries linearly with churn.
func TestApplyMemo_PrunesDeletedKeysOnAllNamespacesList(t *testing.T) {
	dc := newFakeDynClient(
		pod("api-1", "team-a"),
		pod("api-2", "team-a"),
	)
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAlways)
	t.Cleanup(c.Shutdown)

	gvr := podGVR()
	build := func(obj *unstructured.Unstructured) model.Item {
		return model.Item{Name: obj.GetName(), Namespace: obj.GetNamespace()}
	}

	// Warmup: memo gets both keys.
	_, _, err := c.informers.listItems(context.Background(), "", gvr, "", build)
	require.NoError(t, err)
	entry := c.informers.entries[""][gvr]
	require.Contains(t, entry.memo, "team-a/api-1")
	require.Contains(t, entry.memo, "team-a/api-2")

	// Delete api-2 and wait for the watch to apply.
	require.NoError(t, dc.Resource(gvr).Namespace("team-a").Delete(context.Background(), "api-2", metav1.DeleteOptions{}))
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		items, _, err := c.informers.listItems(context.Background(), "", gvr, "", build)
		require.NoError(t, err)
		if len(items) == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// The next listItems must have pruned api-2's memo entry. Without
	// the prune, this assertion fails — the entry would linger forever.
	assert.NotContains(t, entry.memo, "team-a/api-2",
		"deleted pod's memo entry must be pruned by the all-ns sweep")
	assert.Contains(t, entry.memo, "team-a/api-1",
		"surviving pod's memo entry must remain")
}

// TestApplyMemo_PerNamespaceListDoesNotPruneOtherNamespaces guards the
// scope of the prune sweep. A namespaced list is only authoritative for
// its own namespace — pruning entries from other namespaces would erase
// useful memo state for any prior all-namespaces or other-namespace
// list, defeating the purpose of caching across views.
func TestApplyMemo_PerNamespaceListDoesNotPruneOtherNamespaces(t *testing.T) {
	dc := newFakeDynClient(
		pod("api-1", "team-a"),
		pod("api-2", "team-a"),
		pod("worker-1", "team-b"),
	)
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAlways)
	t.Cleanup(c.Shutdown)

	gvr := podGVR()
	build := func(obj *unstructured.Unstructured) model.Item {
		return model.Item{Name: obj.GetName(), Namespace: obj.GetNamespace()}
	}

	// Warmup: all-namespaces list seeds memo with all three pods.
	_, _, err := c.informers.listItems(context.Background(), "", gvr, "", build)
	require.NoError(t, err)
	entry := c.informers.entries[""][gvr]
	require.Contains(t, entry.memo, "team-a/api-1")
	require.Contains(t, entry.memo, "team-a/api-2")
	require.Contains(t, entry.memo, "team-b/worker-1")

	// Now list only team-a. Even though team-b/worker-1 is not in the
	// returned objs, the prune logic must leave its memo entry alone —
	// the team-a list isn't authoritative for team-b.
	_, _, err = c.informers.listItems(context.Background(), "", gvr, "team-a", build)
	require.NoError(t, err)
	assert.Contains(t, entry.memo, "team-a/api-1")
	assert.Contains(t, entry.memo, "team-a/api-2")
	assert.Contains(t, entry.memo, "team-b/worker-1",
		"per-namespace list must not prune entries from other namespaces")
}
