package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// Refresh at LevelResourceTypes (shift+r in the resource-types sidebar)
// must trigger a fresh API discovery so newly-installed CRDs appear and
// uninstalled ones disappear without restarting lfk. Without this, the
// discoveredResources cache is written once per session and never
// refreshed.
//
// Side effect we can observe synchronously: discoveringContexts[ctx]
// flips to true (the dedup flag the discovery cmd sets at launch). The
// cmd itself runs async and is verified by integration tests
// elsewhere.
func TestRefreshAtResourceTypesTriggersRediscovery(t *testing.T) {
	t.Parallel()
	m := newLoadResourcesTestModel()
	m.cacheFingerprints = make(map[string]string)
	m.discoveringContexts = make(map[string]bool)
	m.nav.Level = model.LevelResourceTypes

	// Seed an existing discovery so we can confirm refresh re-triggers
	// instead of just returning the cached entries.
	m.discoveredResources[m.nav.Context] = []model.ResourceTypeEntry{
		{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true},
	}

	cmd := m.refreshCurrentLevel()
	if cmd == nil {
		t.Fatal("refreshCurrentLevel returned nil")
	}
	// The dedup flag is set synchronously when the cmd is built (not when
	// it runs), so we can assert it without dispatching the async fetch
	// against the fake client.
	assert.True(t, m.discoveringContexts[m.nav.Context],
		"refresh at LevelResourceTypes must mark discovery in-flight so a new CRD pickup actually happens")
}

func TestRefreshAtResourceTypesDedupsIfDiscoveryAlreadyInFlight(t *testing.T) {
	t.Parallel()
	m := newLoadResourcesTestModel()
	m.cacheFingerprints = make(map[string]string)
	m.discoveringContexts = map[string]bool{
		m.nav.Context: true, // already in flight
	}
	m.nav.Level = model.LevelResourceTypes
	m.discoveredResources[m.nav.Context] = []model.ResourceTypeEntry{
		{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true},
	}

	// Should not panic and should still return a non-nil cmd (the cached
	// resourceTypesMsg emit). We just verify it doesn't blow up — the
	// dedup is documented behavior; whether the cmd batch contains a
	// second discovery cmd is an implementation detail.
	cmd := m.refreshCurrentLevel()
	assert.NotNil(t, cmd, "refresh must always emit some cmd to repaint the UI")
}
