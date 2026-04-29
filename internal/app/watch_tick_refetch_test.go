package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// updateWatchTick must dispatch a refresh whose underlying loadResources
// goes to the API rather than serving cached items — otherwise watch
// mode keeps showing terminated pods that are gone from the cluster.
//
// Real-world repro: user deletes a pod → first refresh shows
// "Terminating" → pod finishes terminating and disappears from k8s →
// subsequent watch ticks must NOT re-serve the terminating row from the
// in-process itemCache.
func TestWatchTickRefreshGoesToAPI(t *testing.T) {
	t.Parallel()
	m := newLoadResourcesTestModel()
	m.cacheFingerprints = make(map[string]string)
	m.watchMode = true

	// Stale cache claims a (since-terminated) pod still exists.
	cacheKey := m.nav.Context + "/pods"
	m.itemCache[cacheKey] = []model.Item{
		{
			Name: "terminated-pod", Namespace: "default", Status: "Terminating",
			CreatedAt: time.Now().Add(-2 * time.Hour),
		},
	}
	m.cacheFingerprints[cacheKey] = m.fetchFingerprint()

	_, cmd := m.updateWatchTick(watchTickMsg{})
	if cmd == nil {
		t.Fatal("watch tick returned nil cmd")
	}

	// Drain the batch and inspect the resourcesLoadedMsg the load cmd
	// produces. It must reflect the real (empty) API state, not the
	// cached terminated-pod entry.
	for _, c := range flattenBatch(cmd) {
		msg := c()
		loaded, ok := msg.(resourcesLoadedMsg)
		if !ok {
			continue
		}
		for _, item := range loaded.items {
			assert.NotEqual(t, "terminated-pod", item.Name,
				"watch tick refresh must hit the API, not return the stale cache")
		}
		assert.Empty(t, loaded.items,
			"fake API has no pods; watch tick load must produce empty items")
		return
	}
	t.Fatal("expected a resourcesLoadedMsg in the watch tick batch; got none")
}
