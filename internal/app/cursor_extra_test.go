package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- syncExpandedGroup: cursor past visible items ---

func TestSyncExpandedGroupCursorPastEnd(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{
			{Name: "Pods", Category: "Workloads"},
			{Name: "Deployments", Category: "Workloads"},
			{Name: "Services", Category: "Networking"},
		},
		expandedGroup:     "Workloads",
		allGroupsExpanded: false,
	}
	// Set cursor past visible items.
	m.setCursor(100)
	m.syncExpandedGroup()
	// Cursor should be clamped and group should be updated.
	assert.LessOrEqual(t, m.cursor(), 2)
}

func TestSyncExpandedGroupDashboardCategoryAlwaysExpanded(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{
			{Name: "Dashboard", Category: "Dashboards"},
			{Name: "Pods", Category: "Workloads"},
		},
		expandedGroup:     "Workloads",
		allGroupsExpanded: false,
	}
	// Dashboard items have no category collapse.
	visible := m.visibleMiddleItems()
	hasDashboard := false
	for _, item := range visible {
		if item.Name == "Dashboard" {
			hasDashboard = true
		}
	}
	assert.True(t, hasDashboard)
}

// --- visibleMiddleItems: filter matches name only ---

func TestVisibleMiddleItemsFilterByName(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "pod-a", Kind: "Pod"},
			{Name: "deploy-b", Kind: "Deployment"},
		},
		filterText: "pod",
	}
	visible := m.visibleMiddleItems()
	assert.Len(t, visible, 1)
	assert.Equal(t, "pod-a", visible[0].Name)
}

// --- visibleMiddleItems: filter does NOT match by Extra/Kind/Status ---

func TestVisibleMiddleItemsFilterIgnoresExtra(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "svc-a", Extra: "ClusterIP"},
			{Name: "svc-b", Extra: "LoadBalancer"},
		},
		filterText: "LoadBalancer",
	}
	// Filter only matches names, not Extra fields.
	visible := m.visibleMiddleItems()
	assert.Empty(t, visible)
}

func TestVisibleMiddleItemsFilterByNamespaceName(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "pod-a", Namespace: "kube-system"},
			{Name: "pod-b", Namespace: "default"},
		},
		filterText: "kube-system",
	}
	visible := m.visibleMiddleItems()
	assert.Len(t, visible, 1)
	assert.Equal(t, "pod-a", visible[0].Name)
}

// --- selectedMiddleItem: with filter ---

func TestSelectedMiddleItemWithFilter(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "nginx", Kind: "Pod"},
			{Name: "redis", Kind: "Pod"},
			{Name: "nginx-deploy", Kind: "Pod"},
		},
		filterText: "nginx",
	}
	m.setCursor(1)
	item := m.selectedMiddleItem()
	assert.NotNil(t, item)
	assert.Equal(t, "nginx-deploy", item.Name)
}

// --- selectionKey: edge cases ---

func TestSelectionKeyEmptyNamespace(t *testing.T) {
	item := model.Item{Name: "node-1"}
	assert.Equal(t, "node-1", selectionKey(item))
}

func TestSelectionKeyWithNamespace(t *testing.T) {
	item := model.Item{Name: "pod-1", Namespace: "kube-system"}
	assert.Equal(t, "kube-system/pod-1", selectionKey(item))
}

// --- carryOverMetricsColumns: CPU% and MEM% keys ---

func TestCarryOverMetricsColumnsPercentKeys(t *testing.T) {
	m := Model{
		middleItems: []model.Item{
			{
				Name: "pod-a", Namespace: "ns-1",
				Columns: []model.KeyValue{
					{Key: "CPU", Value: "100m"},
					{Key: "MEM", Value: "256Mi"},
					{Key: "CPU%", Value: "50%"},
					{Key: "MEM%", Value: "60%"},
				},
			},
		},
	}
	newItems := []model.Item{
		{
			Name: "pod-a", Namespace: "ns-1",
			Columns: []model.KeyValue{
				{Key: "Status", Value: "Running"},
			},
		},
	}
	m.carryOverMetricsColumns(newItems)
	assert.Greater(t, len(newItems[0].Columns), 1)
}

// --- selectedItemsList: with filter ---

func TestSelectedItemsListWithFilter(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "nginx-1"},
			{Name: "redis-1"},
			{Name: "nginx-2"},
		},
		filterText:    "nginx",
		selectedItems: map[string]bool{"nginx-1": true, "redis-1": true, "nginx-2": true},
	}
	// selectedItemsList only returns selected items from visible items.
	selected := m.selectedItemsList()
	assert.Len(t, selected, 2) // only nginx-1 and nginx-2 are visible
}

// --- restoreCursorToItem: with extra field ---

func TestRestoreCursorToItemWithExtra(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "svc-a", Extra: "ClusterIP"},
			{Name: "svc-a", Extra: "LoadBalancer"},
		},
	}
	m.setCursor(0)
	m.restoreCursorToItem("svc-a", "", "LoadBalancer")
	assert.Equal(t, 1, m.cursor())
}

// --- visibleMiddleItems: collapsed group with Dashboards ---

func TestVisibleMiddleItemsCollapsedWithDashboards(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{
			{Name: "Overview", Category: "Dashboards"},
			{Name: "Pods", Category: "Workloads"},
			{Name: "Services", Category: "Networking"},
		},
		expandedGroup:     "Workloads",
		allGroupsExpanded: false,
	}
	visible := m.visibleMiddleItems()
	// Dashboard items should be visible regardless of collapsed state.
	foundOverview := false
	for _, item := range visible {
		if item.Name == "Overview" {
			foundOverview = true
		}
	}
	assert.True(t, foundOverview)
}

// --- categoryCounts: no categories ---

func TestCategoryCountsNone(t *testing.T) {
	m := Model{
		middleItems: []model.Item{
			{Name: "a"},
			{Name: "b"},
		},
	}
	counts := m.categoryCounts()
	assert.Empty(t, counts)
}
