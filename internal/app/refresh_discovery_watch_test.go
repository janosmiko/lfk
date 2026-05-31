package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
)

// TestRefreshCurrentLevel_WatchTickDoesNotRediscover is the fix for
// "Discover API Resources constantly running" on the resource-type list: a
// watch-tick refresh (suppressBgtasks) must not invalidate the discovery
// cache and re-fire discovery every interval. Discovery is session-cached by
// design and only an explicit refresh re-runs it.
func TestRefreshCurrentLevel_WatchTickDoesNotRediscover(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelResourceTypes
	// Pretend discovery already completed this session.
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{{Kind: "Pod", Resource: "pods"}}

	m.suppressBgtasks = true // watch-tick refresh
	_ = m.refreshCurrentLevel()

	assert.Empty(t, m.discoveringContexts,
		"watch-tick refresh must not start an API-resource discovery")
}

// TestRefreshCurrentLevel_ManualRefreshRediscovers verifies the explicit
// shift+r path still re-runs discovery so newly-installed CRDs surface.
func TestRefreshCurrentLevel_ManualRefreshRediscovers(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelResourceTypes
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{{Kind: "Pod", Resource: "pods"}}

	m.suppressBgtasks = false // explicit user refresh
	_ = m.refreshCurrentLevel()

	assert.True(t, m.discoveringContexts["test-ctx"],
		"manual refresh must re-run discovery to pick up new CRDs")
}
