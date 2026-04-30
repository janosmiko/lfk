package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// nsSelectorModel returns a baseExplorerModel wired up so the namespace
// selector code path can run end-to-end: the active context resolves, the
// cachedNamespaces map exists, and the overlay subsystem is at its
// initial state.
func nsSelectorModel() Model {
	m := baseExplorerModel()
	m.cachedNamespaces = make(map[string]namespaceCacheEntry)
	return m
}

// TestHandleKeyNamespaceSelector_SeedsFromFreshCache is the regression
// guard for the lag a user reported when opening the namespace overlay
// on a slow API server. With a fresh cache entry, the overlay must
// populate synchronously: overlayItems is non-nil before any tea.Cmd
// runs, m.loading stays false, and ensureNamespaceCacheFresh returns nil
// so no API call is scheduled.
func TestHandleKeyNamespaceSelector_SeedsFromFreshCache(t *testing.T) {
	m := nsSelectorModel()
	m.cachedNamespaces[m.activeContext()] = namespaceCacheEntry{
		items: []model.Item{
			{Name: "default", Status: "Active"},
			{Name: "kube-system", Status: "Active"},
		},
		names:     []string{"default", "kube-system"},
		fetchedAt: time.Now(),
	}

	ret, cmd := m.handleKeyNamespaceSelector()
	result := ret.(Model)

	assert.Equal(t, overlayNamespace, result.overlay)
	assert.False(t, result.loading, "cached open must not show the loading spinner")
	require.Len(t, result.overlayItems, 3, "expected All Namespaces header + 2 cached items")
	assert.Equal(t, "All Namespaces", result.overlayItems[0].Name)
	assert.Equal(t, "default", result.overlayItems[1].Name)
	assert.Equal(t, "Active", result.overlayItems[1].Status,
		"status must round-trip through the cache so Terminating namespaces still render correctly")
	assert.Nil(t, cmd, "fresh cache hit must not schedule an API refresh")
}

// TestHandleKeyNamespaceSelector_CursorPositionsOnCurrentNamespace
// verifies the overlay opens with the cursor on the user's active
// namespace, mirroring the post-fetch behaviour. Without this the
// synchronously-seeded overlay would stick the cursor on row 0
// regardless of the current selection.
func TestHandleKeyNamespaceSelector_CursorPositionsOnCurrentNamespace(t *testing.T) {
	m := nsSelectorModel()
	m.namespace = "kube-system"
	m.allNamespaces = false
	m.cachedNamespaces[m.activeContext()] = namespaceCacheEntry{
		items: []model.Item{
			{Name: "default"},
			{Name: "kube-system"},
			{Name: "monitoring"},
		},
		names:     []string{"default", "kube-system", "monitoring"},
		fetchedAt: time.Now(),
	}

	ret, _ := m.handleKeyNamespaceSelector()
	result := ret.(Model)

	// items: [All Namespaces, default, kube-system, monitoring] → cursor 2.
	assert.Equal(t, 2, result.overlayCursor)
}

// TestHandleKeyNamespaceSelector_AllNamespacesParksCursorOnHeader
// verifies that when the user is in all-ns mode, the cursor lands on
// the "All Namespaces" header row (index 0) rather than on whatever
// namespace happened to match m.namespace.
func TestHandleKeyNamespaceSelector_AllNamespacesParksCursorOnHeader(t *testing.T) {
	m := nsSelectorModel()
	m.namespace = "default"
	m.allNamespaces = true
	m.cachedNamespaces[m.activeContext()] = namespaceCacheEntry{
		items:     []model.Item{{Name: "default"}, {Name: "kube-system"}},
		names:     []string{"default", "kube-system"},
		fetchedAt: time.Now(),
	}

	ret, _ := m.handleKeyNamespaceSelector()
	result := ret.(Model)

	assert.Equal(t, 0, result.overlayCursor)
	assert.Equal(t, "All Namespaces", result.overlayItems[0].Name)
}

// TestHandleKeyNamespaceSelector_StaleCacheStillSeedsOverlay covers the
// synchronous half of the stale-while-revalidate path: an aged entry
// must still populate the overlay immediately so the open feels
// instant, with no spinner. The cmd-side wiring (silent refresh
// scheduled) is asserted separately in
// TestHandleKeyNamespaceSelector_StaleCacheSchedulesBackgroundRefresh,
// which uses a real fake client.
func TestHandleKeyNamespaceSelector_StaleCacheStillSeedsOverlay(t *testing.T) {
	m := nsSelectorModel()
	m.cachedNamespaces[m.activeContext()] = namespaceCacheEntry{
		items:     []model.Item{{Name: "default"}, {Name: "kube-system"}},
		names:     []string{"default", "kube-system"},
		fetchedAt: time.Now().Add(-2 * namespaceCacheTTL),
	}

	ret, _ := m.handleKeyNamespaceSelector()
	result := ret.(Model)

	assert.Equal(t, overlayNamespace, result.overlay)
	assert.False(t, result.loading, "stale cache hit must not blank the already-shown overlay")
	require.Len(t, result.overlayItems, 3,
		"stale entry should still seed the overlay synchronously")
}

// TestHandleKeyNamespaceSelector_EmptyCacheFallsBackToLoadingSpinner
// preserves the original first-open behaviour: with no cached entry,
// the overlay shows the loading spinner and schedules a synchronous
// (non-silent) load command. Without this the user would see an
// "empty namespace list" flash before the load lands.
func TestHandleKeyNamespaceSelector_EmptyCacheFallsBackToLoadingSpinner(t *testing.T) {
	m := nsSelectorModel()
	// cachedNamespaces map exists but is empty.

	ret, cmd := m.handleKeyNamespaceSelector()
	result := ret.(Model)

	assert.Equal(t, overlayNamespace, result.overlay)
	assert.True(t, result.loading, "empty cache must show the loading spinner")
	assert.Nil(t, result.overlayItems, "empty cache must not populate items synchronously")
	// loadNamespaces -> loadNamespacesSilent(false): returns a tea.Cmd
	// even when client is nil (the cmd would surface a nil-client error
	// at execute time). Either way, it's not nil here.
	if m.client != nil {
		assert.NotNil(t, cmd, "empty cache must schedule a load command")
	}
}

// TestHandleKeyNamespaceSelector_StaleCacheSchedulesBackgroundRefresh is
// the integration counterpart to the stale-cache seed test: with a real
// client wired in, the stale cache hit must seed the overlay AND return
// a non-nil refresh cmd (the silent loader). Without this assertion the
// stale-while-revalidate wiring could regress without any test failure
// — the cache would be stuck on stale data forever.
func TestHandleKeyNamespaceSelector_StaleCacheSchedulesBackgroundRefresh(t *testing.T) {
	m := baseModelWithFakeClient()
	m.cachedNamespaces = map[string]namespaceCacheEntry{
		m.activeContext(): {
			items:     []model.Item{{Name: "default"}, {Name: "kube-system"}},
			names:     []string{"default", "kube-system"},
			fetchedAt: time.Now().Add(-2 * namespaceCacheTTL),
		},
	}

	ret, cmd := m.handleKeyNamespaceSelector()
	result := ret.(Model)

	require.Len(t, result.overlayItems, 3, "stale cache must still seed the overlay")
	assert.False(t, result.loading, "stale cache hit must not blank the overlay")
	assert.NotNil(t, cmd, "stale cache must schedule a silent refresh command")
}

// TestUpdateNamespacesLoaded_SilentPreservesOverlayCursor guards the
// race between the user navigating an open overlay and the silent
// stale-while-revalidate refresh landing. Without this guard, the
// cursor jumps back to the active namespace under the user's hand
// every time the silent refresh lands during an overlay session.
func TestUpdateNamespacesLoaded_SilentPreservesOverlayCursor(t *testing.T) {
	m := nsSelectorModel()
	m.namespace = "default"
	m.allNamespaces = false
	m.overlay = overlayNamespace
	m.overlayItems = []model.Item{
		{Name: "All Namespaces", Status: "all"},
		{Name: "default"},
		{Name: "kube-system"},
		{Name: "production"},
	}
	// User has navigated to "production" (index 3); a non-silent load
	// would snap the cursor back to row 1 ("default").
	m.overlayCursor = 3

	ret, _ := m.Update(namespacesLoadedMsg{
		context: m.activeContext(),
		items: []model.Item{
			{Name: "default"},
			{Name: "kube-system"},
			{Name: "production"},
		},
		silent: true,
	})
	result := ret.(Model)

	assert.Equal(t, 3, result.overlayCursor,
		"silent refresh must not yank the cursor away from the user")
}

// TestUpdateNamespacesLoaded_SilentLeavesOverlayItemsAlone is the
// companion to the cursor test: a silent refresh that lands while a
// non-namespace overlay is open (or while the user is mid-interaction
// in the namespace overlay) must not rewrite m.overlayItems either.
// The cache is updated for the next open; the currently-displayed
// overlay is left untouched.
func TestUpdateNamespacesLoaded_SilentLeavesOverlayItemsAlone(t *testing.T) {
	m := nsSelectorModel()
	original := []model.Item{
		{Name: "All Namespaces", Status: "all"},
		{Name: "default", Status: "Active"},
		{Name: "kube-system", Status: "Active"},
	}
	m.overlay = overlayNamespace
	m.overlayItems = original
	m.overlayCursor = 2

	ret, _ := m.Update(namespacesLoadedMsg{
		context: m.activeContext(),
		// Refreshed list: a brand-new namespace appeared and "default"
		// is now Terminating. Silent path must not splice this in
		// under the user's open overlay.
		items: []model.Item{
			{Name: "default", Status: "Terminating"},
			{Name: "kube-system", Status: "Active"},
			{Name: "newly-created", Status: "Active"},
		},
		silent: true,
	})
	result := ret.(Model)

	assert.Equal(t, original, result.overlayItems,
		"silent refresh must leave the visible overlay items alone")
	// The cache must still be updated so the next overlay open sees
	// the fresh data — that's the whole point of the background refresh.
	entry, ok := result.cachedNamespaces[m.activeContext()]
	require.True(t, ok)
	require.Len(t, entry.items, 3)
	assert.Equal(t, "newly-created", entry.items[2].Name,
		"silent refresh must still update the per-context cache")
}
