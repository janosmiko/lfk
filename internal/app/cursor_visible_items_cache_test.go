package app

import (
	"testing"
	"unsafe"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func visibleItemsTestModel() Model {
	m := newTestModel()
	m.setMiddleItems([]model.Item{
		{Name: "alpha", Namespace: "ns1", Kind: "Pod"},
		{Name: "beta", Namespace: "ns1", Kind: "Pod"},
		{Name: "gamma", Namespace: "ns2", Kind: "Pod"},
	})
	return m
}

func sameBackingArray(a, b []model.Item) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	return unsafe.SliceData(a) == unsafe.SliceData(b)
}

func TestVisibleMiddleItems_MemoReusesSliceWhenInputsUnchanged(t *testing.T) {
	m := visibleItemsTestModel()

	first := m.visibleMiddleItems()
	second := m.visibleMiddleItems()

	assert.True(t, sameBackingArray(first, second),
		"repeated calls with unchanged filter/items/nav state must reuse the memoized slice")
}

func TestVisibleMiddleItems_InvalidatesOnItemsRevChange(t *testing.T) {
	m := visibleItemsTestModel()
	first := m.visibleMiddleItems()

	m.setMiddleItems([]model.Item{{Name: "delta", Namespace: "ns3", Kind: "Pod"}})
	second := m.visibleMiddleItems()

	assert.False(t, sameBackingArray(first, second), "setMiddleItems must invalidate the memo")
	require.Len(t, second, 1)
	assert.Equal(t, "delta", second[0].Name)
}

func TestVisibleMiddleItems_InvalidatesOnFilterTextChange(t *testing.T) {
	m := visibleItemsTestModel()
	unfiltered := m.visibleMiddleItems()
	require.Len(t, unfiltered, 3)

	m.filterText = "beta"
	filtered := m.visibleMiddleItems()

	require.Len(t, filtered, 1)
	assert.Equal(t, "beta", filtered[0].Name)
}

func TestVisibleMiddleItems_InvalidatesOnFilterBroadModeChange(t *testing.T) {
	m := visibleItemsTestModel()
	m.middleItems[0].Columns = []model.KeyValue{{Key: "label", Value: "special-tag"}}
	m.setMiddleItems(m.middleItems)
	m.filterText = "special-tag"

	m.filterBroadMode = false
	narrow := m.visibleMiddleItems()
	assert.Empty(t, narrow, "name-only filter must not match a column value")

	m.filterBroadMode = true
	broad := m.visibleMiddleItems()
	require.Len(t, broad, 1, "broad mode must invalidate the memo and rescan columns")
}

func TestVisibleMiddleItems_InvalidatesOnExpandedGroupChange(t *testing.T) {
	m := newTestModel()
	m.nav.Level = model.LevelResourceTypes
	m.setMiddleItems([]model.Item{
		{Name: "pods", Category: "Workloads"},
		{Name: "deployments", Category: "Workloads"},
		{Name: "services", Category: "Networking"},
	})
	m.expandedGroup = "Workloads"

	withWorkloadsExpanded := m.visibleMiddleItems()
	names := itemNames(withWorkloadsExpanded)
	assert.Contains(t, names, "deployments")

	m.expandedGroup = "Networking"
	withNetworkingExpanded := m.visibleMiddleItems()
	names = itemNames(withNetworkingExpanded)
	assert.Contains(t, names, "services")
	assert.NotContains(t, names, "deployments",
		"changing expandedGroup must invalidate the memo and recollapse")
}

func TestVisibleMiddleItems_InvalidatesOnAllGroupsExpandedChange(t *testing.T) {
	m := newTestModel()
	m.nav.Level = model.LevelResourceTypes
	m.setMiddleItems([]model.Item{
		{Name: "pods", Category: "Workloads"},
		{Name: "deployments", Category: "Workloads"},
	})
	m.expandedGroup = "Networking" // Workloads stays collapsed

	collapsed := m.visibleMiddleItems()
	assert.NotContains(t, itemNames(collapsed), "deployments")

	m.allGroupsExpanded = true
	expanded := m.visibleMiddleItems()
	assert.Contains(t, itemNames(expanded), "deployments")
}

func TestVisibleMiddleItems_InvalidatesOnNavLevelChange(t *testing.T) {
	m := newTestModel()
	m.setMiddleItems([]model.Item{
		{Name: "pods", Category: "Workloads"},
		{Name: "deployments", Category: "Workloads"},
	})
	m.expandedGroup = "Networking"

	m.nav.Level = model.LevelResources
	uncollapsed := m.visibleMiddleItems()
	assert.Len(t, uncollapsed, 2, "collapse logic only applies at LevelResourceTypes")

	m.nav.Level = model.LevelResourceTypes
	collapsed := m.visibleMiddleItems()
	assert.NotContains(t, itemNames(collapsed), "deployments")
}

// These two sites mutate m.middleItems[i] by index instead of reassigning
// the slice, so they must bump middleItemsRev themselves or the memo below
// keeps a stale ClusterColor/ReadOnly value.
func TestClusterColorOverlay_EnterApply_BumpsMiddleItemsRev(t *testing.T) {
	m := newClusterPickerModel(t)
	m.setMiddleItems(m.middleItems) // establish a non-zero baseline rev
	before := m.middleItemsRev

	ret, _ := m.handleKeyClusterColorPicker()
	m = ret.(Model)
	m.clusterColorOverlayCursor = 0

	ret, _ = m.handleClusterColorOverlayKey(keyMsg("enter"))
	result := ret.(Model)

	assert.Greater(t, result.middleItemsRev, before,
		"applying a cluster color must bump middleItemsRev so the visibleMiddleItems memo invalidates")
}

func TestHandleKeyReadOnlyToggle_AtClusterPicker_BumpsMiddleItemsRev(t *testing.T) {
	m := Model{
		nav:               model.NavigationState{Level: model.LevelClusters},
		middleItems:       []model.Item{{Name: "prod"}, {Name: "dev"}},
		cursors:           [5]int{1, 0, 0, 0, 0},
		tabs:              []TabState{{}},
		itemCache:         map[string][]model.Item{},
		cacheFingerprints: map[string]string{},
		width:             80, height: 40,
	}
	m.setMiddleItems(m.middleItems) // establish a non-zero baseline rev
	before := m.middleItemsRev

	ret, _ := m.handleKeyReadOnlyToggle()
	result := ret.(Model)

	assert.Greater(t, result.middleItemsRev, before,
		"toggling read-only must bump middleItemsRev so the visibleMiddleItems memo invalidates")
}
