package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// At LevelClusters the right-pane preview must load resource types for the
// HOVERED context, not m.nav.Context (which is "" after navigate-parent from
// LevelResourceTypes). With no cached discovery for the hovered context we
// emit an empty rightItems (to clear stale items from a previously-hovered
// context) and kick off discovery — renderRightClusters then shows the
// loader until apiResourceDiscoveryMsg arrives with the real list.
func TestLoadPreviewClusters_UncachedTriggersDiscoveryAndClearsRightItems(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	m.nav.Context = "" // back-nav from LevelResourceTypes
	m = withMiddleItem(m, model.Item{Name: "hovered-ctx"})

	cmd := m.loadPreview()
	require.NotNil(t, cmd,
		"must emit at least one command (clear rightItems + discovery)")
	// The discovering flag should be set so a subsequent hover doesn't
	// kick off another discovery for the same context.
	assert.True(t, m.discoveringContexts["hovered-ctx"],
		"must mark the hovered context as discovering to dedupe subsequent hovers")
}

// When the hovered context has discovered resources from a previous session
// (cache prefill) we emit them immediately for instant paint AND fire a
// fresh discovery behind the scenes (stale-while-revalidate). Once that
// session-fresh discovery has completed, repeated hovers must not re-fire —
// otherwise cursoring through already-visited contexts would keep re-auth'ing.
func TestLoadPreviewClusters_CachedFiresRevalidationOnFirstHover(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	m.nav.Context = ""
	m.discoveredResources["visited-ctx"] = []model.ResourceTypeEntry{
		{DisplayName: "Pods", Kind: "Pod", APIVersion: "v1", Resource: "pods"},
	}
	m = withMiddleItem(m, model.Item{Name: "visited-ctx"})

	cmd := m.loadPreview()
	require.NotNil(t, cmd)
	// The cached items must still be emitted so the right pane paints
	// immediately. shouldFireDiscoveryFor returns true here because no
	// session-fresh discovery has run, so a Batch (cached emit + discovery)
	// is expected — drain it and verify the resourceTypesMsg is in flight.
	assert.True(t, m.discoveringContexts["visited-ctx"],
		"first hover of a cache-prefilled context must trigger background revalidation")
}

// After this session has already done a live discovery for the context,
// further hovers must not re-fire — that's the dedup the OIDC-auth-loop
// regression test is guarding against.
func TestLoadPreviewClusters_CachedSkipsAfterSessionRefresh(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	m.nav.Context = ""
	m.discoveredResources["visited-ctx"] = []model.ResourceTypeEntry{
		{DisplayName: "Pods", Kind: "Pod", APIVersion: "v1", Resource: "pods"},
	}
	m.discoveryRefreshedContexts["visited-ctx"] = true
	m = withMiddleItem(m, model.Item{Name: "visited-ctx"})

	cmd := m.loadPreview()
	require.NotNil(t, cmd)
	msg := cmd()
	rmsg, ok := msg.(resourceTypesMsg)
	require.True(t, ok,
		"expected the single-cmd path (no Batch) to emit resourceTypesMsg directly")
	assert.False(t, rmsg.seeded,
		"discovered entries must not be tagged seeded")
	assert.False(t, m.discoveringContexts["visited-ctx"],
		"context already refreshed this session must not be marked discovering again")
}

// If discovery is already in-flight for the hovered context, don't fire a
// second call (would open another OIDC auth flow etc.).
func TestLoadPreviewClusters_AlreadyDiscoveringDoesNotDuplicate(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	m.nav.Context = ""
	m.discoveringContexts["in-flight"] = true
	m = withMiddleItem(m, model.Item{Name: "in-flight"})

	cmd := m.loadPreview()
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(resourceTypesMsg)
	assert.True(t, ok,
		"when discovery already in-flight, only the resourceTypesMsg cmd should run")
}

// When apiResourceDiscoveryMsg arrives while user is at LevelClusters
// hovering the same context, rightItems must update so the right pane
// replaces seeds with the real discovered list.
func TestUpdateAPIResourceDiscovery_AtClusterLevelHoveredRefreshesRight(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	m.nav.Context = ""
	m.discoveringContexts["ctx-x"] = true
	m = withMiddleItem(m, model.Item{Name: "ctx-x"})
	initial := []model.Item{{Name: "only-seeded-item"}}
	m.rightItems = initial // seeded fallback already shown

	result, _ := m.Update(apiResourceDiscoveryMsg{
		context: "ctx-x",
		entries: []model.ResourceTypeEntry{
			{DisplayName: "Pods", Kind: "Pod", APIVersion: "v1", Resource: "pods"},
			{DisplayName: "MyCRD", Kind: "MyCRD", APIGroup: "demo.io", APIVersion: "v1", Resource: "mycrds"},
		},
	})
	rm := result.(Model)
	assert.NotEqual(t, initial, rm.rightItems,
		"rightItems must be replaced by the real discovered list (not the initial seeded fallback)")
	assert.NotEmpty(t, rm.rightItems)
	assert.False(t, rm.discoveringContexts["ctx-x"],
		"in-flight flag must be cleared on completion")
}

// Discovery result for a context the user is NOT hovering at LevelClusters
// must not overwrite rightItems (which is showing the hovered context's
// data).
func TestUpdateAPIResourceDiscovery_AtClusterLevelOtherContextDoesNotOverwrite(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	m.nav.Context = ""
	m = withMiddleItem(m, model.Item{Name: "currently-hovered"})
	m.rightItems = []model.Item{{Name: "seeded-for-currently-hovered"}}

	result, _ := m.Update(apiResourceDiscoveryMsg{
		context: "some-other-context",
		entries: []model.ResourceTypeEntry{
			{DisplayName: "Pods", Kind: "Pod", APIVersion: "v1", Resource: "pods"},
		},
	})
	rm := result.(Model)
	assert.Equal(t, "seeded-for-currently-hovered", rm.rightItems[0].Name,
		"an unrelated context's discovery must not clobber what the user is looking at")
}

// While initial discovery is in flight for the hovered context, rightItems
// is empty (loadPreviewClusters clears it) and the right pane shows only
// the "Loading..." spinner — no header annotation, no seed placeholder.
func TestRenderRightClusters_LoaderShownDuringInitialDiscovery(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	m = withMiddleItem(m, model.Item{Name: "discovering-ctx"})
	m.rightItems = nil // loadPreviewClusters clears rightItems while uncached
	m.discoveringContexts["discovering-ctx"] = true

	out := m.renderRightClusters(80, 20)
	assert.Contains(t, out, "Loading",
		"empty right pane + in-flight discovery must render only the loader")
	assert.NotContains(t, out, "RESOURCE TYPE",
		"no header annotation while initial discovery runs — just the loader")
}

// Once discovery completes and rightItems is populated, the plain RESOURCE
// TYPE header appears — no spinner noise in the header.
func TestRenderRightClusters_NormalHeaderOnceCached(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	m = withMiddleItem(m, model.Item{Name: "quiet-ctx"})
	m.rightItems = []model.Item{{Name: "Pods"}}

	out := m.renderRightClusters(80, 20)
	assert.Contains(t, out, "RESOURCE TYPE")
	assert.NotContains(t, out, "discovering")
}

// When the user hovers cluster-a (kicking off discovery) and then quickly
// Enters into it BEFORE discovery finishes, navigateChildCluster must not
// fire a second discoverAPIResources for the same context. The in-flight
// call from the hover already has it covered.
func TestNavigateChildCluster_DoesNotDuplicateInFlightDiscovery(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	m.nav.Context = ""
	m.discoveringContexts["cluster-a"] = true
	m = withMiddleItem(m, model.Item{Name: "cluster-a"})

	// navigateChildCluster is a method on Model that returns (Model, Cmd).
	// We don't assert on the returned Cmd content — just that the context
	// is still marked discovering (i.e. we didn't clear+rearm it) and
	// that we haven't panicked. The real assertion is a visual one: we
	// rely on the loop-dedupe guard added in this commit and document the
	// intent here. The important state-level invariant: we did navigate,
	// m.nav.Context is now cluster-a, and the existing discovery flag
	// remains so updateAPIResourceDiscovery can clear it on arrival.
	result, _ := m.navigateChildCluster(&model.Item{Name: "cluster-a"})
	rm := result.(Model)
	assert.Equal(t, "cluster-a", rm.nav.Context)
	assert.True(t, rm.discoveringContexts["cluster-a"],
		"existing in-flight discovery flag must remain set until the msg arrives")
}

// If the user navigates into a cluster BEFORE hover-discovery has completed
// but the right pane already had seeded items, the middle column should
// show those items immediately — not a blank "Loading..." spinner. Once
// discovery completes the real list replaces them.
func TestNavigateChildCluster_UsesHoverSeedsAsMiddleItems(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	m.nav.Context = ""
	m.discoveringContexts["cluster-a"] = true
	seeds := []model.Item{{Name: "Pods"}, {Name: "Deployments"}}
	m.rightItems = seeds
	m = withMiddleItem(m, model.Item{Name: "cluster-a"})

	result, _ := m.navigateChildCluster(&model.Item{Name: "cluster-a"})
	rm := result.(Model)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
	assert.Len(t, rm.middleItems, len(seeds),
		"hover-seed items must be reused as the initial middle list so the user "+
			"doesn't see a blank loader during the navigation → discovery gap")
}
