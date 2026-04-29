package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// Reproduces the bug where deleting a pod (or anything that removes a
// row from k8s) didn't update the visible list because refreshCurrentLevel
// hit the in-process itemCache shortcut in loadResources and returned the
// stale entry instead of fetching fresh from the API.
//
// After the fix, refreshCurrentLevel must invalidate the current
// resource's cache entry so the underlying loadResources call performs
// an actual API fetch — letting the user see the updated list after a
// watch tick or shift+r.
func TestRefreshCurrentLevelBypassesItemCache(t *testing.T) {
	t.Parallel()
	m := newLoadResourcesTestModel()
	m.cacheFingerprints = make(map[string]string)

	// Prime the cache with a stale entry: fake API has no pods, but the
	// cache claims one is there.
	cacheKey := m.nav.Context + "/pods"
	m.itemCache[cacheKey] = []model.Item{
		{Name: "stale-pod", Namespace: "default", Age: "2h", CreatedAt: time.Now().Add(-2 * time.Hour)},
	}
	m.cacheFingerprints[cacheKey] = m.fetchFingerprint()

	cmd := m.refreshCurrentLevel()
	if cmd == nil {
		t.Fatal("refreshCurrentLevel returned nil")
	}
	msg := cmd().(resourcesLoadedMsg)

	for _, item := range msg.items {
		assert.NotEqual(t, "stale-pod", item.Name,
			"refresh must not return cached items removed since last load")
	}
	assert.Empty(t, msg.items,
		"fake API has no pods; refresh must return empty, not the cached stale-pod")
}
