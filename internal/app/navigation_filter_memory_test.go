package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// saveLevelFilter/restoreLevelFilter are the per-level filter memory primitives
// that back issue #303: a resource list must remember its filter when the user
// drills into a subview and navigates back.
func TestLevelFilterMemory_RoundTrip(t *testing.T) {
	m := basePush80Model()
	m.filterMemory = make(map[string]savedFilter)

	// At LevelResources (context test-ctx, pods) with a committed broad filter.
	m.filterText = "web"
	m.filterInput.Set("web")
	m.filterBroadMode = true
	key := m.navKey()

	m.saveLevelFilter()
	assert.Equal(t, savedFilter{text: "web", broad: true}, m.filterMemory[key])

	// Simulate a level change wiping the live filter.
	m.filterText = ""
	m.filterInput.Clear()
	m.filterBroadMode = false

	// Returning to the same navKey restores the saved filter, broad mode included.
	m.restoreLevelFilter()
	assert.Equal(t, "web", m.filterText)
	assert.Equal(t, "web", m.filterInput.Value)
	assert.True(t, m.filterBroadMode, "broad mode must be restored with the filter")
	assert.False(t, m.filterActive)
}

// An empty filter must not leave a stale entry behind, so a later visit to the
// same level starts clean rather than restoring a phantom filter.
func TestLevelFilterMemory_EmptyFilterClearsEntry(t *testing.T) {
	m := basePush80Model()
	m.filterMemory = map[string]savedFilter{m.navKey(): {text: "stale"}}

	m.filterText = ""
	m.filterInput.Clear()
	m.saveLevelFilter()

	_, ok := m.filterMemory[m.navKey()]
	assert.False(t, ok, "saving an empty filter must delete the level's entry")

	m.restoreLevelFilter()
	assert.Empty(t, m.filterText)
	assert.Empty(t, m.filterInput.Value)
}

// restoreLevelFilter on a level with no saved filter must clear the live filter
// (a different sibling list should not inherit the previous list's filter).
func TestLevelFilterMemory_RestoreNoEntryClears(t *testing.T) {
	m := basePush80Model()
	m.filterMemory = make(map[string]savedFilter)
	m.filterText = "leftover"
	m.filterInput.Set("leftover")
	m.filterBroadMode = true

	m.restoreLevelFilter()
	assert.Empty(t, m.filterText)
	assert.Empty(t, m.filterInput.Value)
	assert.False(t, m.filterBroadMode, "a level with no saved filter must reset broad mode")
	assert.False(t, m.filterActive)
}

// Integration: a filter committed on the Pods list (LevelResources) is restored
// when the user backs out from a subview to that exact list. Pre-seeds the
// parent level's saved filter and asserts navigateParent restores it.
func TestNavigateParent_RestoresSavedFilter(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1"}
	m.nav.ResourceName = "pod-1" // containers always carry the pod name
	m.leftItems = []model.Item{{Name: "container-1"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "cluster"}}, {{Name: "Pods"}}, {{Name: "pod-1"}}}

	// The Pods list (parent) had a broad filter "web" saved when the user
	// drilled in. Parent navKey after navigateParent: context + resource.
	m.filterMemory = map[string]savedFilter{"test-ctx/pods": {text: "web", broad: true}}

	result, _ := m.navigateParent()
	rm := result.(Model)

	require.Equal(t, model.LevelResources, rm.nav.Level)
	assert.Equal(t, "web", rm.filterText, "parent list filter must be restored on back-navigation")
	assert.Equal(t, "web", rm.filterInput.Value)
	assert.True(t, rm.filterBroadMode, "broad mode must be restored on back-navigation")
	assert.False(t, rm.filterActive)
}

// Full round trip: filter the Pods list, drill into a pod, back out — the
// filter must reappear. This is the literal issue #303 reproduction.
func TestNavigateChildThenParent_PreservesPodListFilter(t *testing.T) {
	m := basePush80Model() // LevelResources, Pods, context test-ctx
	m.leftItems = []model.Item{{Name: "Pods"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "test-ctx"}}}
	m.filterText = "pod-1"
	m.filterInput.Set("pod-1")
	m.setCursor(0) // pod-1 is the only visible match

	// Drill into the selected pod.
	child, _ := m.navigateChild()
	cm := child.(Model)
	require.NotEqual(t, model.LevelResources, cm.nav.Level, "should descend below the pod list")
	assert.Empty(t, cm.filterText, "child level starts unfiltered")

	// Back out to the Pods list.
	parent, _ := cm.navigateParent()
	pm := parent.(Model)
	require.Equal(t, model.LevelResources, pm.nav.Level)
	assert.Equal(t, "pod-1", pm.filterText, "pod list filter must survive the round trip")
	assert.Equal(t, "pod-1", pm.filterInput.Value)
}
