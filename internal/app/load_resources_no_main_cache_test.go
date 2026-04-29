package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// loadResources(forPreview=false) is what every main-list load goes
// through (drill-in, navigate-back, watch tick, shift+r). The
// itemCache shortcut existed to skip API calls during sidebar
// hover-cycles between sibling resource types — a preview concern.
//
// When the shortcut also catches main loads, deleted pods linger:
// navigating away and back, hovering a sibling type, or any flow that
// doesn't pass through refreshCurrentLevel re-serves the stale list.
//
// Main loads must always go to the API. The cache stays useful for
// preview hover (forPreview=true) and as the immediate "show old
// items first" cache read in update_navigation.go.
func TestLoadResourcesMainPathDoesNotShortcutOnCache(t *testing.T) {
	t.Parallel()
	m := newLoadResourcesTestModel()
	m.cacheFingerprints = make(map[string]string)

	cacheKey := m.nav.Context + "/pods"
	m.itemCache[cacheKey] = []model.Item{
		{Name: "ghost-pod", Namespace: "default", CreatedAt: time.Now().Add(-2 * time.Hour)},
	}
	m.cacheFingerprints[cacheKey] = m.fetchFingerprint()

	cmd := m.loadResources(false)
	if cmd == nil {
		t.Fatal("loadResources returned nil")
	}
	msg := cmd().(resourcesLoadedMsg)

	// The fake K8s client has no pods; if we shortcut on cache, the
	// ghost-pod would come back instead.
	for _, item := range msg.items {
		assert.NotEqual(t, "ghost-pod", item.Name,
			"main load must not return cached items; the shortcut belongs to preview loads")
	}
	assert.Empty(t, msg.items)
}
