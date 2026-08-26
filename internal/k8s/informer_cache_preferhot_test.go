package k8s

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clienttesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// TestGetResources_PreferCache_PromotesBelowThreshold is the regression
// guard for issue #646: a watch-tick refresh (PreferCache) must mark the
// GVR hot and route it through the informer cache even though the list is
// far below the size-based auto-promote threshold.
func TestGetResources_PreferCache_PromotesBelowThreshold(t *testing.T) {
	dc := newFakeDynClient(pod("api-1", "team-a"))
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAuto)
	t.Cleanup(c.Shutdown)
	withTunedThresholds(c, 1000, 500, 3)

	items, err := c.GetResources(t.Context(), "", "team-a", podRT, PreferCache())
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.True(t, c.informers.isPromoted("", podGVR()),
		"PreferCache must promote the GVR despite a 1-item list, promoteAt=1000")
}

// TestPreferCache_HotGVRNeverDemotes guards observeCachedListSize's hot
// exemption: a resource kept hot by repeated watch-tick refreshes must
// stay cache-backed even when every cached list is small enough to trip
// auto-demote after a single call.
func TestPreferCache_HotGVRNeverDemotes(t *testing.T) {
	dc := newFakeDynClient(pod("api-1", "team-a"))
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAuto)
	t.Cleanup(c.Shutdown)
	withTunedThresholds(c, 1, 500, 1)

	gvr := podGVR()
	// Promote by size first, as a large list would, then mark hot the way
	// a watch-tick refresh does.
	c.informers.observeDirectListSize("", gvr, 1)
	require.True(t, c.informers.isPromoted("", gvr))
	c.informers.markHot("", gvr)

	for i := range 5 {
		demoted := c.informers.observeCachedListSize("", gvr, 1) // always below demoteBelow=500
		assert.False(t, demoted, "hot GVR must never auto-demote, iteration %d", i)
	}
	assert.True(t, c.informers.isPromoted("", gvr))
}

// TestPreferCache_HotGVRDemotesAfterTTL guards the other half: once a hot
// GVR goes unused past hotTTL, the next markHot call (for any GVR, since
// the sweep is lazy) must clear its hot flag and close its watch.
func TestPreferCache_HotGVRDemotesAfterTTL(t *testing.T) {
	dc := newFakeDynClient(pod("api-1", "team-a"), namespaceObj("default"))
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAuto)
	t.Cleanup(c.Shutdown)
	withTunedThresholds(c, 1000, 500, 3)

	gvr := podGVR()
	require.True(t, c.informers.markHot("", gvr))
	require.True(t, c.informers.isPromoted("", gvr))

	c.informers.hotTTL = time.Millisecond
	c.informers.sweepInterval = 0
	time.Sleep(5 * time.Millisecond)

	// The sweep is lazy inside markHot. Trigger it via an unrelated GVR so
	// the pod GVR's own lastUse isn't refreshed by this call.
	nsGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	c.informers.markHot("", nsGVR)

	assert.False(t, c.informers.isPromoted("", gvr), "hot GVR unused past hotTTL must demote")
	_, ok := c.informers.entries[""][gvr]
	assert.False(t, ok, "the stale watch must be closed")
}

// TestInformerCache_WatchForbiddenFallsBackPermanentlyAndLogsOnce covers
// getOrStart's watch error handler: a Forbidden watch must permanently
// deny the GVR for the session, fall back to direct lists, and log
// exactly once despite the reflector's own retries.
func TestInformerCache_WatchForbiddenFallsBackPermanentlyAndLogsOnce(t *testing.T) {
	logger.ResetDedupForTest()
	drainUIChan()

	dc := newFakeDynClient(pod("api-1", "team-a"))
	dc.PrependWatchReactor("pods", func(clienttesting.Action) (bool, watch.Interface, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("watch denied"))
	})
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAlways)
	t.Cleanup(c.Shutdown)
	gvr := podGVR()

	deadline := time.Now().Add(2 * time.Second)
	var items []model.Item
	var err error
	for time.Now().Before(deadline) {
		items, err = c.GetResources(t.Context(), "", "team-a", podRT)
		require.NoError(t, err)
		if c.informers.getAutoState("", gvr).isDenied() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	require.True(t, c.informers.getAutoState("", gvr).isDenied(), "watch forbidden must permanently deny the GVR")
	require.False(t, c.informers.isPromoted("", gvr))
	require.Len(t, items, 1, "denied GVR must still return items via direct list")
	assert.Equal(t, "api-1", items[0].Name)

	warnings := 0
	for {
		select {
		case e := <-logger.UIChan():
			if e.Level == "WRN" {
				warnings++
			}
		case <-time.After(200 * time.Millisecond):
			assert.Equal(t, 1, warnings, "exactly one warn should surface for the denied watch")
			return
		}
	}
}

// drainUIChan clears any log entries left over from other tests sharing
// the process-wide logger.UIChan.
func drainUIChan() {
	for {
		select {
		case <-logger.UIChan():
		default:
			return
		}
	}
}

// TestObserveDirectListSize_IgnoresFieldSelectedList is the carry-over
// review fix: a field-selected direct list must not feed auto-promote
// stats for the unfiltered GVR, since it is not representative of that
// GVR's real size.
func TestObserveDirectListSize_IgnoresFieldSelectedList(t *testing.T) {
	dc := newFakeDynClient(
		pod("api-1", "team-a"),
		pod("api-2", "team-a"),
	)
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAuto)
	t.Cleanup(c.Shutdown)
	withTunedThresholds(c, 2, 1, 1)

	filteredRT := podRT
	filteredRT.FieldSelector = "status.phase=Running"

	_, err := c.GetResources(t.Context(), "", "", filteredRT)
	require.NoError(t, err)

	assert.False(t, c.informers.isPromoted("", podGVR()),
		"a field-selected list must not promote the unfiltered GVR")
}

// TestInformerAllowed_ExcludesEventsSecretsMetrics guards the exclusion
// list: Events, Secrets, metrics.k8s.io, and any resource whose Verbs omit
// "watch" must never route through the informer cache.
func TestInformerAllowed_ExcludesEventsSecretsMetrics(t *testing.T) {
	tests := []struct {
		name string
		rt   model.ResourceTypeEntry
		want bool
	}{
		{"core events excluded", model.ResourceTypeEntry{APIGroup: "", Resource: "events"}, false},
		{"events.k8s.io excluded", model.ResourceTypeEntry{APIGroup: "events.k8s.io", Resource: "events"}, false},
		{"secrets excluded", model.ResourceTypeEntry{APIGroup: "", Resource: "secrets"}, false},
		{"metrics.k8s.io excluded", model.ResourceTypeEntry{APIGroup: "metrics.k8s.io", Resource: "pods"}, false},
		{"verbs without watch excluded", model.ResourceTypeEntry{APIGroup: "example.com", Resource: "widgets", Verbs: []string{"get", "list"}}, false},
		{"verbs with watch allowed", model.ResourceTypeEntry{APIGroup: "example.com", Resource: "widgets", Verbs: []string{"get", "list", "watch"}}, true},
		{"empty verbs allowed (pseudo-resource)", model.ResourceTypeEntry{APIGroup: "_helm", Resource: "releases"}, true},
		{"pods allowed", podRT, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, informerAllowed(tc.rt))
		})
	}
}
