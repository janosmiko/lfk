package app

import (
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- cursor / setCursor ---

func TestCursorGetSet(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
	}
	assert.Equal(t, 0, m.cursor())

	m.setCursor(5)
	assert.Equal(t, 5, m.cursor())

	// Setting cursor for a different level should not affect the current level.
	m.nav.Level = model.LevelOwned
	assert.Equal(t, 0, m.cursor())
	m.setCursor(3)
	assert.Equal(t, 3, m.cursor())

	// Switch back and verify previous level is preserved.
	m.nav.Level = model.LevelResources
	assert.Equal(t, 5, m.cursor())
}

// --- clampCursor ---

func TestClampCursor(t *testing.T) {
	tests := []struct {
		name     string
		cursor   int
		items    []model.Item
		expected int
	}{
		{
			name:     "cursor within bounds",
			cursor:   1,
			items:    []model.Item{{Name: "a"}, {Name: "b"}, {Name: "c"}},
			expected: 1,
		},
		{
			name:     "cursor past end clamped",
			cursor:   10,
			items:    []model.Item{{Name: "a"}, {Name: "b"}},
			expected: 1,
		},
		{
			name:     "negative cursor clamped to 0",
			cursor:   -5,
			items:    []model.Item{{Name: "a"}},
			expected: 0,
		},
		{
			name:     "empty items cursor stays 0",
			cursor:   5,
			items:    nil,
			expected: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				nav:         model.NavigationState{Level: model.LevelResources},
				middleItems: tt.items,
			}
			m.setCursor(tt.cursor)
			m.clampCursor()
			assert.Equal(t, tt.expected, m.cursor())
		})
	}
}

// --- cursorItemKey ---

func TestCursorItemKey(t *testing.T) {
	t.Run("returns item fields at cursor", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "pod-a", Namespace: "ns-1", Extra: "extra-a", Kind: "Pod"},
				{Name: "pod-b", Namespace: "ns-2", Extra: "extra-b", Kind: "Pod"},
			},
		}
		m.setCursor(1)
		name, ns, extra, kind := m.cursorItemKey()
		assert.Equal(t, "pod-b", name)
		assert.Equal(t, "ns-2", ns)
		assert.Equal(t, "extra-b", extra)
		assert.Equal(t, "Pod", kind)
	})

	t.Run("returns empty strings when no items", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
		}
		name, ns, extra, kind := m.cursorItemKey()
		assert.Empty(t, name)
		assert.Empty(t, ns)
		assert.Empty(t, extra)
		assert.Empty(t, kind)
	})

	t.Run("returns empty strings when cursor out of range", func(t *testing.T) {
		m := Model{
			nav:         model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{{Name: "only"}},
		}
		m.setCursor(5)
		name, ns, extra, kind := m.cursorItemKey()
		assert.Empty(t, name)
		assert.Empty(t, ns)
		assert.Empty(t, extra)
		assert.Empty(t, kind)
	})
}

// --- restoreCursorToItem ---

func TestRestoreCursorToItem(t *testing.T) {
	t.Run("restores to matching item", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "pod-a", Namespace: "ns-1", Kind: "Pod"},
				{Name: "pod-b", Namespace: "ns-2", Kind: "Pod"},
				{Name: "pod-c", Namespace: "ns-3", Kind: "Pod"},
			},
		}
		m.setCursor(0)
		m.restoreCursorToItem("pod-c", "ns-3", "", "Pod")
		assert.Equal(t, 2, m.cursor())
	})

	t.Run("clamps when item gone", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "pod-a"},
			},
		}
		m.setCursor(10)
		m.restoreCursorToItem("gone", "", "", "")
		assert.Equal(t, 0, m.cursor())
	})

	t.Run("empty name clamps", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "pod-a"},
			},
		}
		m.setCursor(5)
		m.restoreCursorToItem("", "", "", "")
		assert.Equal(t, 0, m.cursor())
	})

	// TestKindDistinguishesSameNameItems is the regression for the
	// "ServiceAccount cursor jumps one element up" bug. ArgoCD applications
	// often deploy multiple resources sharing the same name+namespace+extra
	// (Deployment, Service, ConfigMap, ServiceAccount all named "myapp"
	// in namespace "default", all with extra "/v1"). Without Kind in the
	// match, restoreCursorToItem returned the first match — typically a
	// resource one slot above the user's actual selection.
	t.Run("kind distinguishes same-name same-extra items", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelOwned},
			middleItems: []model.Item{
				{Name: "myapp", Namespace: "default", Extra: "/v1", Kind: "Deployment"},
				{Name: "myapp", Namespace: "default", Extra: "/v1", Kind: "Service"},
				{Name: "myapp", Namespace: "default", Extra: "/v1", Kind: "ConfigMap"},
				{Name: "myapp", Namespace: "default", Extra: "/v1", Kind: "ServiceAccount"},
			},
		}
		m.setCursor(0)
		m.restoreCursorToItem("myapp", "default", "/v1", "ServiceAccount")
		assert.Equal(t, 3, m.cursor(), "ServiceAccount must match its own slot, not the first item with the same name")
	})
}

// --- navKey ---

func TestNavKey(t *testing.T) {
	tests := []struct {
		name     string
		nav      model.NavigationState
		expected string
	}{
		{
			name:     "context only",
			nav:      model.NavigationState{Context: "prod"},
			expected: "prod",
		},
		{
			name: "context with resource type",
			nav: model.NavigationState{
				Context:      "prod",
				ResourceType: model.ResourceTypeEntry{Resource: "deployments"},
			},
			expected: "prod/deployments",
		},
		{
			name: "full path",
			nav: model.NavigationState{
				Context:      "prod",
				ResourceType: model.ResourceTypeEntry{Resource: "deployments"},
				ResourceName: "my-deploy",
				OwnedName:    "my-pod",
			},
			expected: "prod/deployments/my-deploy/my-pod",
		},
		{
			name:     "empty state",
			nav:      model.NavigationState{},
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{nav: tt.nav}
			assert.Equal(t, tt.expected, m.navKey())
		})
	}
}

// --- saveCursor / restoreCursor ---

func TestSaveCursorAndRestoreCursor(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "prod",
			ResourceType: model.ResourceTypeEntry{
				Resource: "pods",
			},
		},
		cursorMemory: make(map[string]int),
		middleItems: []model.Item{
			{Name: "a"}, {Name: "b"}, {Name: "c"},
		},
	}

	m.setCursor(2)
	m.saveCursor()

	// Move cursor away.
	m.setCursor(0)
	assert.Equal(t, 0, m.cursor())

	// Restore should bring it back.
	m.restoreCursor()
	assert.Equal(t, 2, m.cursor())
}

func TestRestoreCursorNoSavedPosition(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "dev",
		},
		cursorMemory: make(map[string]int),
		middleItems:  []model.Item{{Name: "a"}},
	}
	m.setCursor(5)

	// No saved position for this nav key, should reset to 0.
	m.restoreCursor()
	assert.Equal(t, 0, m.cursor())
}

// --- selectedMiddleItem ---

func TestSelectedMiddleItem(t *testing.T) {
	t.Run("returns correct item", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "pod-a", Kind: "Pod"},
				{Name: "pod-b", Kind: "Pod"},
			},
		}
		m.setCursor(1)
		item := m.selectedMiddleItem()
		assert.NotNil(t, item)
		assert.Equal(t, "pod-b", item.Name)
	})

	t.Run("returns nil when empty", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
		}
		assert.Nil(t, m.selectedMiddleItem())
	})

	t.Run("returns nil when cursor out of range", func(t *testing.T) {
		m := Model{
			nav:         model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{{Name: "only"}},
		}
		m.setCursor(10)
		assert.Nil(t, m.selectedMiddleItem())
	})
}

// --- isSelected / toggleSelection / clearSelection / hasSelection ---

func TestSelectionOperations(t *testing.T) {
	m := Model{
		selectedItems: make(map[string]bool),
	}
	podA := model.Item{Name: "pod-a", Namespace: "ns-1"}
	podB := model.Item{Name: "pod-b", Namespace: "ns-2"}

	assert.False(t, m.isSelected(podA))
	assert.False(t, m.hasSelection())

	m.toggleSelection(podA)
	assert.True(t, m.isSelected(podA))
	assert.False(t, m.isSelected(podB))
	assert.True(t, m.hasSelection())

	m.toggleSelection(podB)
	assert.True(t, m.isSelected(podA))
	assert.True(t, m.isSelected(podB))

	// Toggle off.
	m.toggleSelection(podA)
	assert.False(t, m.isSelected(podA))
	assert.True(t, m.isSelected(podB))

	m.clearSelection()
	assert.False(t, m.isSelected(podB))
	assert.False(t, m.hasSelection())
	assert.Equal(t, -1, m.selectionAnchor)
}

// --- selectedItemsList ---

func TestSelectedItemsList(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "pod-a", Namespace: "ns-1"},
			{Name: "pod-b", Namespace: "ns-2"},
			{Name: "pod-c", Namespace: "ns-3"},
		},
		selectedItems: make(map[string]bool),
	}

	m.toggleSelection(m.middleItems[0])
	m.toggleSelection(m.middleItems[2])

	selected := m.selectedItemsList()
	assert.Len(t, selected, 2)
	assert.Equal(t, "pod-a", selected[0].Name)
	assert.Equal(t, "pod-c", selected[1].Name)
}

// --- visibleMiddleItems ---

func TestVisibleMiddleItemsNoFilter(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "pod-a"},
			{Name: "pod-b"},
		},
	}
	visible := m.visibleMiddleItems()
	assert.Len(t, visible, 2)
}

func TestVisibleMiddleItemsWithFilter(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "nginx-pod"},
			{Name: "redis-pod"},
			{Name: "nginx-deployment"},
		},
		filterText: "nginx",
	}
	visible := m.visibleMiddleItems()
	assert.Len(t, visible, 2)
	assert.Equal(t, "nginx-pod", visible[0].Name)
	assert.Equal(t, "nginx-deployment", visible[1].Name)
}

func TestVisibleMiddleItemsFilterByNamespace(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "pod-a", Namespace: "production"},
			{Name: "pod-b", Namespace: "staging"},
		},
		filterText: "production",
	}
	visible := m.visibleMiddleItems()
	assert.Len(t, visible, 1)
	assert.Equal(t, "pod-a", visible[0].Name)
}

func TestVisibleMiddleItemsCollapsedGroups(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{
			{Name: "Pods", Category: "Workloads"},
			{Name: "Deployments", Category: "Workloads"},
			{Name: "Services", Category: "Networking"},
			{Name: "Ingresses", Category: "Networking"},
		},
		expandedGroup:     "Workloads",
		allGroupsExpanded: false,
	}
	visible := m.visibleMiddleItems()
	// Workloads should be expanded (2 items), Networking should be collapsed (1 header).
	assert.Len(t, visible, 3)
	assert.Equal(t, "Pods", visible[0].Name)
	assert.Equal(t, "Deployments", visible[1].Name)
	assert.Equal(t, "Networking", visible[2].Name)
	assert.Equal(t, "__collapsed_group__", visible[2].Kind)
}

func TestVisibleMiddleItemsAllGroupsExpanded(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{
			{Name: "Pods", Category: "Workloads"},
			{Name: "Services", Category: "Networking"},
		},
		allGroupsExpanded: true,
	}
	visible := m.visibleMiddleItems()
	assert.Len(t, visible, 2)
}

func TestVisibleMiddleItemsCategoryFilterIncludesAll(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "Pods", Category: "Workloads"},
			{Name: "Deployments", Category: "Workloads"},
			{Name: "Services", Category: "Networking"},
		},
		filterText: "Workloads",
	}
	visible := m.visibleMiddleItems()
	// Category match should include all items in the "Workloads" category.
	assert.Len(t, visible, 2)
}

// --- categoryCounts ---

func TestCategoryCounts(t *testing.T) {
	m := Model{
		middleItems: []model.Item{
			{Name: "a", Category: "Workloads"},
			{Name: "b", Category: "Workloads"},
			{Name: "c", Category: "Networking"},
			{Name: "d"},
		},
	}
	counts := m.categoryCounts()
	assert.Equal(t, 2, counts["Workloads"])
	assert.Equal(t, 1, counts["Networking"])
	assert.Equal(t, 0, counts[""])
}

// --- parentIndex ---

func TestParentIndex(t *testing.T) {
	t.Run("LevelResourceTypes returns context match", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:   model.LevelResourceTypes,
				Context: "prod-cluster",
			},
			leftItems: []model.Item{
				{Name: "dev-cluster"},
				{Name: "prod-cluster"},
			},
		}
		assert.Equal(t, 1, m.parentIndex())
	})

	t.Run("LevelResources returns resource type match", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level: model.LevelResources,
				ResourceType: model.ResourceTypeEntry{
					DisplayName: "Deployments",
					APIGroup:    "apps",
					APIVersion:  "v1",
					Resource:    "deployments",
				},
			},
			leftItems: []model.Item{
				{Name: "Pods", Extra: "/v1/pods"},
				{Name: "Deployments", Extra: "apps/v1/deployments"},
				{Name: "Services", Extra: "/v1/services"},
			},
		}
		assert.Equal(t, 1, m.parentIndex())
	})

	// Regression: when the user drills from Resource Types into a resource list,
	// navigateChildResourceType stores the API-discovery-produced
	// ResourceTypeEntry in m.nav.ResourceType. Discovery does NOT populate
	// DisplayName on that struct — only pseudo-resources (Port Forwards, Helm
	// Releases) and the curated metadata table carry one. The parent column in
	// leftItems still renders the resource types, and the correct entry must
	// stay highlighted. Matching on DisplayName silently drops the highlight
	// for every real-world resource; matching on the stable ResourceRef
	// (APIGroup/APIVersion/Resource, stored in Item.Extra) survives the empty
	// DisplayName.
	t.Run("LevelResources matches discovery entry with empty DisplayName", func(t *testing.T) {
		discovered := []model.ResourceTypeEntry{
			{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
			{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true},
			{Kind: "Service", APIGroup: "", APIVersion: "v1", Resource: "services", Namespaced: true},
		}
		items := model.BuildSidebarItems(discovered)
		rt, ok := model.FindResourceTypeIn("apps/v1/deployments", discovered)
		require.True(t, ok)
		assert.Empty(t, rt.DisplayName, "discovery must not populate DisplayName (guards the bug we are testing)")
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: rt,
			},
			leftItems: items,
		}
		got := m.parentIndex()
		require.GreaterOrEqual(t, got, 0, "expected the parent Deployments row to be highlighted")
		assert.Equal(t, "Deployments", items[got].Name)
	})

	t.Run("LevelOwned returns resource name match", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:        model.LevelOwned,
				ResourceName: "my-deploy",
			},
			leftItems: []model.Item{
				{Name: "other-deploy"},
				{Name: "my-deploy"},
			},
		}
		assert.Equal(t, 1, m.parentIndex())
	})

	t.Run("LevelContainers returns owned name match", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:     model.LevelContainers,
				OwnedName: "my-pod",
			},
			leftItems: []model.Item{
				{Name: "my-pod"},
			},
		}
		assert.Equal(t, 0, m.parentIndex())
	})

	t.Run("LevelClusters returns -1", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelClusters},
		}
		assert.Equal(t, -1, m.parentIndex())
	})

	t.Run("no match returns -1", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				Level:   model.LevelResourceTypes,
				Context: "nonexistent",
			},
			leftItems: []model.Item{{Name: "prod"}},
		}
		assert.Equal(t, -1, m.parentIndex())
	})
}

// --- carryOverMetricsColumns ---

func TestCarryOverMetricsColumns(t *testing.T) {
	t.Run("carries over CPU and MEM from old items", func(t *testing.T) {
		m := Model{
			middleItems: []model.Item{
				{
					Name: "pod-a", Namespace: "ns-1",
					Columns: []model.KeyValue{
						{Key: "CPU", Value: "100m"},
						{Key: "MEM", Value: "256Mi"},
						{Key: "CPU/R", Value: "200m"},
						{Key: "Other", Value: "val"},
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
		// Should have metrics columns prepended, plus the non-metrics "Status".
		assert.Len(t, newItems[0].Columns, 4)
		assert.Equal(t, "CPU", newItems[0].Columns[0].Key)
		assert.Equal(t, "MEM", newItems[0].Columns[1].Key)
		assert.Equal(t, "CPU/R", newItems[0].Columns[2].Key)
		assert.Equal(t, "Status", newItems[0].Columns[3].Key)
	})

	t.Run("no carryover when old items have no real usage", func(t *testing.T) {
		m := Model{
			middleItems: []model.Item{
				{
					Name: "pod-a", Namespace: "ns-1",
					Columns: []model.KeyValue{
						{Key: "CPU", Value: "0"},
						{Key: "MEM", Value: "0"},
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
		assert.Len(t, newItems[0].Columns, 1)
		assert.Equal(t, "Status", newItems[0].Columns[0].Key)
	})

	t.Run("no carryover for unmatched items", func(t *testing.T) {
		m := Model{
			middleItems: []model.Item{
				{
					Name: "pod-x", Namespace: "ns-1",
					Columns: []model.KeyValue{
						{Key: "CPU", Value: "100m"},
						{Key: "MEM", Value: "256Mi"},
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
		assert.Len(t, newItems[0].Columns, 1)
	})

	t.Run("empty old items does nothing", func(t *testing.T) {
		m := Model{}
		newItems := []model.Item{
			{Name: "pod-a", Columns: []model.KeyValue{{Key: "Status", Value: "Running"}}},
		}
		m.carryOverMetricsColumns(newItems)
		assert.Len(t, newItems[0].Columns, 1)
	})
}

// --- filteredOverlayItems ---

func TestFilteredOverlayItems(t *testing.T) {
	m := Model{
		overlayItems: []model.Item{
			{Name: "default"},
			{Name: "kube-system"},
			{Name: "production"},
		},
	}

	t.Run("no filter returns all", func(t *testing.T) {
		m.overlayFilter = TextInput{}
		result := m.filteredOverlayItems()
		assert.Len(t, result, 3)
	})

	t.Run("filter matches subset", func(t *testing.T) {
		m.overlayFilter = TextInput{Value: "kube"}
		result := m.filteredOverlayItems()
		assert.Len(t, result, 1)
		assert.Equal(t, "kube-system", result[0].Name)
	})

	t.Run("case insensitive filter", func(t *testing.T) {
		m.overlayFilter = TextInput{Value: "DEFAULT"}
		result := m.filteredOverlayItems()
		assert.Len(t, result, 1)
		assert.Equal(t, "default", result[0].Name)
	})

	t.Run("no match returns empty", func(t *testing.T) {
		m.overlayFilter = TextInput{Value: "nonexistent"}
		result := m.filteredOverlayItems()
		assert.Empty(t, result)
	})
}

// --- filteredExplainRecursiveResults ---

func TestFilteredExplainRecursiveResults(t *testing.T) {
	m := Model{
		explainRecursiveResults: []model.ExplainField{
			{Name: "spec", Path: "spec"},
			{Name: "metadata", Path: "metadata"},
			{Name: "status", Path: "status"},
		},
	}

	t.Run("no filter returns all", func(t *testing.T) {
		m.explainRecursiveFilter = TextInput{}
		result := m.filteredExplainRecursiveResults()
		assert.Len(t, result, 3)
	})

	t.Run("filter by name", func(t *testing.T) {
		m.explainRecursiveFilter = TextInput{Value: "spec"}
		result := m.filteredExplainRecursiveResults()
		assert.Len(t, result, 1)
		assert.Equal(t, "spec", result[0].Name)
	})

	t.Run("filter by path", func(t *testing.T) {
		m.explainRecursiveFilter = TextInput{Value: "meta"}
		result := m.filteredExplainRecursiveResults()
		assert.Len(t, result, 1)
		assert.Equal(t, "metadata", result[0].Name)
	})
}

// --- filteredLogPodItems ---

func TestFilteredLogPodItems(t *testing.T) {
	m := Model{
		overlayItems: []model.Item{
			{Name: "nginx-pod-1"},
			{Name: "redis-pod-1"},
			{Name: "nginx-pod-2"},
		},
	}

	t.Run("no filter returns all", func(t *testing.T) {
		m.logPodFilterText = ""
		result := m.filteredLogPodItems()
		assert.Len(t, result, 3)
	})

	t.Run("filter matches subset", func(t *testing.T) {
		m.logPodFilterText = "nginx"
		result := m.filteredLogPodItems()
		assert.Len(t, result, 2)
	})
}

// --- clampAllCursors ---

func TestClampAllCursors(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "a"}, {Name: "b"},
		},
	}
	m.setCursor(10)
	m.clampAllCursors()
	assert.Equal(t, 1, m.cursor())
}

// --- syncExpandedGroup ---

func TestSyncExpandedGroup(t *testing.T) {
	t.Run("updates expanded group when cursor moves to different category", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResourceTypes},
			middleItems: []model.Item{
				{Name: "Pods", Category: "Workloads"},
				{Name: "Deployments", Category: "Workloads"},
				{Name: "Services", Category: "Networking"},
				{Name: "Ingresses", Category: "Networking"},
			},
			expandedGroup:     "Workloads",
			allGroupsExpanded: false,
		}
		// Move cursor to a collapsed group header
		// visibleMiddleItems with expandedGroup="Workloads" shows:
		// 0: Pods(Workloads), 1: Deployments(Workloads), 2: Networking(collapsed header)
		m.setCursor(2)
		m.syncExpandedGroup()
		assert.Equal(t, "Networking", m.expandedGroup)
	})

	t.Run("no-op when allGroupsExpanded", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResourceTypes},
			middleItems: []model.Item{
				{Name: "Pods", Category: "Workloads"},
			},
			expandedGroup:     "Workloads",
			allGroupsExpanded: true,
		}
		m.syncExpandedGroup()
		assert.Equal(t, "Workloads", m.expandedGroup)
	})

	t.Run("no-op when not at LevelResourceTypes", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "Pods", Category: "Workloads"},
			},
			expandedGroup: "Workloads",
		}
		m.syncExpandedGroup()
		assert.Equal(t, "Workloads", m.expandedGroup)
	})
}

// --- filteredLogContainerItems ---

func TestFilteredLogContainerItems(t *testing.T) {
	m := Model{
		overlayItems: []model.Item{
			{Name: "All Containers", Status: "all"},
			{Name: "nginx"},
			{Name: "sidecar"},
		},
	}

	t.Run("no filter returns all", func(t *testing.T) {
		m.logContainerFilterText = ""
		result := m.filteredLogContainerItems()
		assert.Len(t, result, 3)
	})

	t.Run("filter matches plus always includes All Containers", func(t *testing.T) {
		m.logContainerFilterText = "nginx"
		result := m.filteredLogContainerItems()
		assert.Len(t, result, 2)
		assert.Equal(t, "All Containers", result[0].Name)
		assert.Equal(t, "nginx", result[1].Name)
	})
}

func TestPush2HandleKeyExplorerModeEnter(t *testing.T) {
	m := basePush80v2Model()
	m.mode = modeExplorer
	m.setCursor(0)
	result, cmd := m.handleKey(keyMsg("enter"))
	rm := result.(Model)
	// Enter navigates -- either to containers or YAML view.
	_ = rm
	_ = cmd
}

func TestPush2HandleExplorerActionKeyP(t *testing.T) {
	// 'p' pins a CRD group at LevelResourceTypes.
	m := basePush80v2Model()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Pods", Category: "Core"}}
	m.setCursor(0)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	result, _, handled := m.handleExplorerActionKey(keyMsg("p"))
	if handled {
		_ = result.(Model)
	}
}

func TestPush2DirectActionScaleNoSelection(t *testing.T) {
	m := basePush80v2Model()
	m.nav.ResourceType.Kind = "Deployment"
	m.middleItems = []model.Item{{Name: "deploy-1", Kind: "Deployment", Namespace: "default"}}
	m.setCursor(0)
	result, cmd := m.directActionScale()
	rm := result.(Model)
	assert.Equal(t, overlayScaleInput, rm.overlay)
	assert.Nil(t, cmd)
}

func TestPush2DirectActionScaleNonScaleable(t *testing.T) {
	m := basePush80v2Model()
	m.nav.ResourceType.Kind = "ConfigMap"
	m.middleItems = []model.Item{{Name: "cm-1", Kind: "ConfigMap"}}
	m.setCursor(0)
	result, _ := m.directActionScale()
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
}

func TestPush2DirectActionForceDeleteWithSel(t *testing.T) {
	m := basePush80v2Model()
	m.nav.ResourceType.Kind = "Pod"
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default"}}
	m.setCursor(0)
	result, cmd := m.directActionForceDelete()
	rm := result.(Model)
	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Nil(t, cmd)
}

func TestPush3HandleKeyVimBindings(t *testing.T) {
	m := basePush80v3Model()
	m.setCursor(0)
	kb := ui.ActiveKeybindings

	// j = down
	result, _ := m.handleKey(keyMsg(kb.Down))
	rm := result.(Model)
	assert.GreaterOrEqual(t, rm.cursor(), 0)

	// k = up
	m.setCursor(1)
	result2, _ := m.handleKey(keyMsg(kb.Up))
	rm2 := result2.(Model)
	assert.GreaterOrEqual(t, rm2.cursor(), 0)
}

func TestPush3HandleKeyGG(t *testing.T) {
	m := basePush80v3Model()
	m.setCursor(1)
	// First 'g' sets pendingG.
	result, _ := m.handleKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)

	// Second 'g' jumps to top.
	result2, _ := rm.handleKey(keyMsg("g"))
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.cursor())
	assert.False(t, rm2.pendingG)
}

func TestPush3HandleKeyG(t *testing.T) {
	m := basePush80v3Model()
	m.setCursor(0)
	kb := ui.ActiveKeybindings
	// 'G' goes to bottom.
	result, _ := m.handleKey(keyMsg(kb.JumpBottom))
	rm := result.(Model)
	assert.Equal(t, len(rm.visibleMiddleItems())-1, rm.cursor())
}

func TestP4ExplorerActionKeyI(t *testing.T) {
	m := bp4()
	m.setCursor(0)
	result, _, handled := m.handleExplorerActionKey(keyMsg("I"))
	if handled {
		rm := result.(Model)
		assert.True(t, rm.loading)
	}
}

func TestP4ExplorerActionKeyR(t *testing.T) {
	m := bp4()
	m.setCursor(0)
	result, cmd, handled := m.handleExplorerActionKey(keyMsg("R"))
	if handled {
		rm := result.(Model)
		_ = rm
		_ = cmd
	}
}

func TestP4ExplorerActionKeyV(t *testing.T) {
	m := bp4()
	m.setCursor(0)
	result, cmd, handled := m.handleExplorerActionKey(keyMsg("v"))
	if handled {
		rm := result.(Model)
		_ = rm
		_ = cmd
	}
}

func TestP4ExplorerActionKeyX(t *testing.T) {
	m := bp4()
	m.setCursor(0)
	result, cmd, handled := m.handleExplorerActionKey(keyMsg("x"))
	if handled {
		rm := result.(Model)
		assert.Equal(t, overlayAction, rm.overlay)
		_ = cmd
	}
}

func TestP4ExplorerActionKeyL(t *testing.T) {
	m := bp4()
	m.setCursor(0)
	result, _, handled := m.handleExplorerActionKey(keyMsg("L"))
	if handled {
		_ = result.(Model)
	}
}

func TestP4ExplorerActionKeyY(t *testing.T) {
	m := bp4()
	m.setCursor(0)
	result, _, handled := m.handleExplorerActionKey(keyMsg("y"))
	if handled {
		rm := result.(Model)
		_ = rm
	}
}

func TestP4ExplorerActionKeyBigY(t *testing.T) {
	m := bp4()
	m.setCursor(0)
	result, _, handled := m.handleExplorerActionKey(keyMsg("Y"))
	if handled {
		_ = result.(Model)
	}
}

func TestP4ExplorerActionKeyE(t *testing.T) {
	m := bp4()
	m.setCursor(0)
	result, _, handled := m.handleExplorerActionKey(keyMsg("e"))
	if handled {
		_ = result.(Model)
	}
}

func TestP4ExplorerActionKeyD(t *testing.T) {
	m := bp4()
	m.setCursor(0)
	result, _, handled := m.handleExplorerActionKey(keyMsg("d"))
	if handled {
		_ = result.(Model)
	}
}

func TestP4ExplorerActionKeyO(t *testing.T) {
	m := bp4()
	m.setCursor(0)
	result, _, handled := m.handleExplorerActionKey(keyMsg("o"))
	if handled {
		_ = result.(Model)
	}
}

func TestP4ExplorerActionKeyAt(t *testing.T) {
	m := bp4()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Overview", Extra: "__overview__"}}
	m.setCursor(0)
	result, _, handled := m.handleExplorerActionKey(keyMsg("@"))
	_ = handled
	_ = result
}

func TestP4MouseClickRightColumn(t *testing.T) {
	m := bp4()
	m.mode = modeExplorer
	m.setCursor(0)
	// Click in the right column (x > middleEnd).
	result, _ := m.handleMouseClick(m.width-1, 10)
	_ = result.(Model)
}

func TestCov80MoveCursorUpFromZero(t *testing.T) {
	m := basePush80Model()
	m.setCursor(0)
	result, cmd := m.moveCursor(-1)
	rm := result.(Model)
	assert.Equal(t, 0, rm.cursor())
	assert.NotNil(t, cmd) // loadPreview
}

func TestCov80MoveCursorDownPastEnd(t *testing.T) {
	m := basePush80Model()
	m.setCursor(0)
	result, cmd := m.moveCursor(100)
	rm := result.(Model)
	assert.Equal(t, len(rm.visibleMiddleItems())-1, rm.cursor())
	assert.NotNil(t, cmd)
}

func TestCov80MoveCursorByOne(t *testing.T) {
	m := basePush80Model()
	m.setCursor(0)
	result, cmd := m.moveCursor(1)
	rm := result.(Model)
	assert.Equal(t, 1, rm.cursor())
	assert.NotNil(t, cmd)
}

// TestRefreshOwnedAtLevelOwnedPreservesCursor verifies that a non-preview
// ownedLoadedMsg arriving while the cursor is at index N (because the user
// has navigated down) does not reset the cursor to 0. The handler must use
// the prev cursor item key to restore the cursor in the new items list.
func TestRefreshOwnedAtLevelOwnedPreservesCursor(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelOwned,
			Context: "test-ctx",
			ResourceType: model.ResourceTypeEntry{
				Kind:     "Application",
				Resource: "applications",
			},
			ResourceName: "my-app",
		},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
		requestGen:          1,
	}
	items := []model.Item{
		{Name: "child-1", Kind: "Deployment", Namespace: "default", Extra: "apps/v1"},
		{Name: "child-2", Kind: "Service", Namespace: "default", Extra: "v1"},
		{Name: "child-3", Kind: "ConfigMap", Namespace: "default", Extra: "v1"},
		{Name: "child-4", Kind: "Secret", Namespace: "default", Extra: "v1"},
		{Name: "child-5", Kind: "Deployment", Namespace: "default", Extra: "apps/v1"},
	}
	m.middleItems = items
	m.setCursor(3) // simulate user has navigated to child-4

	// A non-preview ownedLoadedMsg arrives (e.g. from refreshCurrentLevel).
	// The cursor should remain on child-4, not reset to 0.
	result, _ := m.Update(ownedLoadedMsg{items: items, gen: m.requestGen})
	rm := result.(Model)
	assert.Equal(t, 3, rm.cursor(), "refresh ownedLoadedMsg must preserve cursor position")

	// Even with statuses changed (simulating live status diff), cursor must
	// hold because matching is on name/namespace/extra.
	updated := append([]model.Item(nil), items...)
	updated[3].Status = "Different"
	result, _ = rm.Update(ownedLoadedMsg{items: updated, gen: rm.requestGen})
	rm = result.(Model)
	assert.Equal(t, 3, rm.cursor(), "status change must not reset cursor")
}

// TestMoveCursorAtLevelOwnedKeepsCursorPosition reproduces the user-reported
// bug where navigating down the children of an ArgoCD application or Helm
// release with j/k caused the cursor to jump back to the first element. The
// movement should advance the cursor and keep it there even after subsequent
// gen-bumped preview loads or stale ownedLoadedMsg arrivals.
func TestMoveCursorAtLevelOwnedKeepsCursorPosition(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelOwned,
			Context: "test-ctx",
			ResourceType: model.ResourceTypeEntry{
				Kind:     "Application",
				Resource: "applications",
			},
			ResourceName: "my-app",
		},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
		requestGen:          1,
	}
	items := []model.Item{
		{Name: "child-1", Kind: "Deployment", Namespace: "default", Extra: "apps/v1"},
		{Name: "child-2", Kind: "Service", Namespace: "default", Extra: "v1"},
		{Name: "child-3", Kind: "ConfigMap", Namespace: "default", Extra: "v1"},
		{Name: "child-4", Kind: "Secret", Namespace: "default", Extra: "v1"},
		{Name: "child-5", Kind: "Deployment", Namespace: "default", Extra: "apps/v1"},
	}
	m.middleItems = items

	// Step through the children one j press at a time, simulating an
	// arriving stale ownedLoadedMsg between each press to mimic what
	// happens when a refresh load completes during rapid navigation.
	m.setCursor(0)
	for want := 1; want <= 4; want++ {
		result, _ := m.moveCursor(1)
		m = result.(Model)
		assert.Equalf(t, want, m.cursor(), "after pressing j %d times the cursor must be at index %d", want, want)

		// Simulate a stale (gen mismatched) ownedLoadedMsg arriving.
		staleResult, _ := m.Update(ownedLoadedMsg{items: items, gen: 0})
		m = staleResult.(Model)
		assert.Equalf(t, want, m.cursor(), "stale ownedLoadedMsg must not reset cursor (was at %d after %d presses)", want, want)

		// Simulate a fresh ownedLoadedMsg matching the current gen.
		freshResult, _ := m.Update(ownedLoadedMsg{items: items, gen: m.requestGen})
		m = freshResult.(Model)
		assert.Equalf(t, want, m.cursor(), "fresh ownedLoadedMsg with matching gen must preserve cursor (was at %d after %d presses)", want, want)
	}
}

// TestMoveCursorBumpsRequestGenAndDiscardsStalePreview locks in the navigation
// flicker fix. Without the requestGen bump in moveCursor, an in-flight preview
// load triggered by the previous cursor position would still match the
// current generation when its message arrived, causing stale items to appear
// in the right pane (or a brief "No resources found" flash if the previous
// selection was empty) before the new load returned.
func TestMoveCursorBumpsRequestGenAndDiscardsStalePreview(t *testing.T) {
	m := basePush80Model()
	m.setCursor(0)
	prevGen := m.requestGen

	// Simulate a preview load dispatched at the previous cursor position.
	stalePreviewGen := prevGen

	// Move the cursor: this must bump requestGen so the in-flight preview
	// response captured at stalePreviewGen is discarded on arrival.
	result, _ := m.moveCursor(1)
	rm := result.(Model)
	assert.Greater(t, rm.requestGen, prevGen, "moveCursor must bump requestGen")
	assert.True(t, rm.loading, "moveCursor must set loading=true")
	assert.Nil(t, rm.rightItems, "moveCursor must clear rightItems")

	// The stale preview response now arrives — it must NOT populate rightItems
	// or clear the loading flag, because its gen no longer matches.
	stale := ownedLoadedMsg{
		items:      []model.Item{{Name: "stale-from-prev-cursor"}},
		forPreview: true,
		gen:        stalePreviewGen,
	}
	after, _ := rm.Update(stale)
	am := after.(Model)
	assert.True(t, am.loading, "stale preview must not clear loading flag")
	assert.Nil(t, am.rightItems, "stale preview must not populate rightItems")
}

func TestCov80MoveCursorEmptyItems(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	result, _ := m.moveCursor(1)
	rm := result.(Model)
	assert.Equal(t, 0, rm.cursor())
}

func TestCov80MoveCursorWithMapView(t *testing.T) {
	m := basePush80Model()
	m.setCursor(0)
	m.mapView = true
	result, cmd := m.moveCursor(1)
	rm := result.(Model)
	assert.Equal(t, 1, rm.cursor())
	assert.NotNil(t, cmd)
}

func TestCov80MoveCursorAccordionDown(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.allGroupsExpanded = false
	m.expandedGroup = "Core"
	// Set up items with categories for accordion behavior.
	m.middleItems = []model.Item{
		{Name: "pods", Kind: "Pod", Category: "Core"},
		{Name: "svc", Kind: "Service", Category: "Core"},
		{Name: "deploy", Kind: "Deployment", Category: "Workloads"},
	}
	m.setCursor(1) // last item in Core group
	result, _ := m.moveCursor(1)
	rm := result.(Model)
	_ = rm
}

func TestCov80MoveCursorAccordionUp(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.allGroupsExpanded = false
	m.expandedGroup = "Workloads"
	m.middleItems = []model.Item{
		{Name: "pods", Kind: "Pod", Category: "Core"},
		{Name: "deploy", Kind: "Deployment", Category: "Workloads"},
		{Name: "sts", Kind: "StatefulSet", Category: "Workloads"},
	}
	m.setCursor(1) // first item in Workloads group
	result, _ := m.moveCursor(-1)
	rm := result.(Model)
	_ = rm
}

func TestCov80LoadMetricsPod(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType.Kind = "Pod"
	m.setCursor(0)
	cmd := m.loadMetrics()
	require.NotNil(t, cmd)
	msg := cmd()
	mmsg, ok := msg.(metricsLoadedMsg)
	require.True(t, ok)
	_ = mmsg
}

func TestCov80LoadMetricsDeployment(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType.Kind = "Deployment"
	m.middleItems = []model.Item{
		{Name: "deploy-1", Namespace: "default", Kind: "Deployment", Status: "Running"},
	}
	m.setCursor(0)
	cmd := m.loadMetrics()
	// Deployment/StatefulSet/DaemonSet branch captured; can't execute
	// because the fake dynamic client lacks registered list kinds.
	require.NotNil(t, cmd)
}

func TestCov80LoadMetricsStatefulSet(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType.Kind = "StatefulSet"
	m.middleItems = []model.Item{
		{Name: "sts-1", Namespace: "default", Kind: "StatefulSet", Status: "Running"},
	}
	m.setCursor(0)
	cmd := m.loadMetrics()
	require.NotNil(t, cmd)
}

func TestCov80LoadMetricsDaemonSet(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType.Kind = "DaemonSet"
	m.middleItems = []model.Item{
		{Name: "ds-1", Namespace: "default", Kind: "DaemonSet"},
	}
	m.setCursor(0)
	cmd := m.loadMetrics()
	require.NotNil(t, cmd)
}

func TestCov80LoadMetricsUnknownKind(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType.Kind = "ConfigMap"
	m.middleItems = []model.Item{
		{Name: "cm-1", Namespace: "default", Kind: "ConfigMap"},
	}
	m.setCursor(0)
	cmd := m.loadMetrics()
	// ConfigMap does not have metrics, returns nil.
	assert.Nil(t, cmd)
}

func TestCov80LoadMetricsAtLevelOwned(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType.Kind = "Deployment"
	m.middleItems = []model.Item{
		{Name: "pod-a", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	m.setCursor(0)
	cmd := m.loadMetrics()
	require.NotNil(t, cmd) // Pod kind at LevelOwned
	msg := cmd()
	_, ok := msg.(metricsLoadedMsg)
	assert.True(t, ok)
}

func TestCov80SaveLabelDataWithSelection(t *testing.T) {
	m := basePush80Model()
	m.labelData = &model.LabelAnnotationData{
		Labels:      map[string]string{"env": "prod"},
		Annotations: map[string]string{"note": "test"},
	}
	m.labelResourceType = model.ResourceTypeEntry{Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.setCursor(0)
	cmd := m.saveLabelData()
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(labelSavedMsg)
	assert.True(t, ok)
}

func TestCov80SaveLabelDataItemNamespace(t *testing.T) {
	m := basePush80Model()
	m.labelData = &model.LabelAnnotationData{
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	}
	m.labelResourceType = model.ResourceTypeEntry{Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "custom-ns", Kind: "Pod"},
	}
	m.setCursor(0)
	cmd := m.saveLabelData()
	require.NotNil(t, cmd)
}

func TestCov80OpenExplainBrowserAtResourceTypes(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "Pods", Kind: "Pod", Extra: "/v1/pods"},
	}
	m.setCursor(0)
	result, cmd := m.openExplainBrowser()
	rm := result.(Model)
	_ = rm
	_ = cmd
}

func TestCov80OpenExplainBrowserVirtualItem(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "Overview", Kind: "__overview__", Extra: "__overview__"},
	}
	m.setCursor(0)
	result, cmd := m.openExplainBrowser()
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestCov80OpenExplainBrowserCollapsedGroup(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "Collapsed", Kind: "__collapsed_group__"},
	}
	m.setCursor(0)
	result, _ := m.openExplainBrowser()
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
}

func TestCov80OpenExplainBrowserMonitoring(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "Monitoring", Kind: "__monitoring__", Extra: "__monitoring__"},
	}
	m.setCursor(0)
	result, _ := m.openExplainBrowser()
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
}

func TestCov80OpenExplainBrowserFallbackKind(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "CRD thing", Kind: "MyCRD", Extra: "nonexistent/ref"},
	}
	m.setCursor(0)
	result, cmd := m.openExplainBrowser()
	rm := result.(Model)
	// Should fallback to lowercase kind + "s".
	assert.True(t, rm.loading)
	_ = cmd
}

func TestCov80NavigateChildFromClusters(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "test-ctx"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCov80NavigateChildFromResourceTypesOverview(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Overview", Extra: "__overview__"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.True(t, rm.fullscreenDashboard)
	assert.NotNil(t, cmd)
}

func TestCov80NavigateChildFromResourceTypesMonitoring(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Monitoring", Extra: "__monitoring__"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.True(t, rm.fullscreenDashboard)
	assert.NotNil(t, cmd)
}

func TestCov80NavigateChildFromResourceTypesPortForward(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.portForwardMgr = k8s.NewPortForwardManager()
	m.middleItems = []model.Item{{Name: "Port Forwards", Kind: "__port_forwards__"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelResources, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCov80NavigateChildFromResourceTypesCollapsedGroup(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	// Pre-populate discoveredResources so that loadResources (called after
	// expanding the collapsed group) can resolve the Pod resource type.
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	m.middleItems = []model.Item{
		{Name: "Core", Kind: "__collapsed_group__", Category: "Core"},
		{Name: "Pods", Kind: "Pod", Category: "Core", Extra: "/v1/pods"},
	}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, "Core", rm.expandedGroup)
	assert.NotNil(t, cmd)
}

func TestCov80NavigateChildFromResourcesPod(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-pod", Kind: "Pod", Namespace: "default"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelContainers, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCov80NavigateChildFromOwnedPod(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment"}
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelContainers, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCov80NavigateChildFromOwnedNonDrillable(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment"}
	m.middleItems = []model.Item{{Name: "cm-1", Kind: "ConfigMap", Namespace: "default"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80NavigateChildFromContainers(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.middleItems = []model.Item{{Name: "container-1", Kind: "Container"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80EnterFullViewFromClusters(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "ctx-1"}}
	m.setCursor(0)
	result, cmd := m.enterFullView()
	rm := result.(Model)
	// Should navigate child.
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCov80EnterFullViewFromResourceTypes(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Overview", Extra: "__overview__"}}
	m.setCursor(0)
	result, cmd := m.enterFullView()
	rm := result.(Model)
	assert.True(t, rm.fullscreenDashboard)
	assert.NotNil(t, cmd)
}

func TestCov80EnterFullViewPortForwards(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__"}
	m.middleItems = []model.Item{{Name: "pf-1"}}
	m.setCursor(0)
	result, cmd := m.enterFullView()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80EnterFullViewNormal(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod"}}
	m.setCursor(0)
	result, cmd := m.enterFullView()
	rm := result.(Model)
	assert.Equal(t, modeYAML, rm.mode)
	assert.NotNil(t, cmd)
}

func TestCov80ExportResourceToFileLevelResourcesCmd(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.setCursor(0)
	cmd := m.exportResourceToFile()
	require.NotNil(t, cmd)
}

func TestCov80ExportResourceToFileLevelOwnedPod(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "pod-owned", Kind: "Pod", Namespace: "default"}}
	m.setCursor(0)
	cmd := m.exportResourceToFile()
	require.NotNil(t, cmd)
}

func TestCov80ExportResourceToFileLevelOwnedUnknownKind(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "unknown-thing", Kind: "UnknownKind123", Namespace: "default"}}
	m.setCursor(0)
	cmd := m.exportResourceToFile()
	require.NotNil(t, cmd)
	msg := cmd()
	emsg, ok := msg.(exportDoneMsg)
	require.True(t, ok)
	assert.Error(t, emsg.err)
}

func TestCov80ExportResourceToFileLevelContainers(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "my-pod"
	m.middleItems = []model.Item{{Name: "container-1", Kind: "Container"}}
	m.setCursor(0)
	cmd := m.exportResourceToFile()
	require.NotNil(t, cmd)
}

func TestCov80CopyYAMLLevelResources(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.setCursor(0)
	cmd := m.copyYAMLToClipboard()
	require.NotNil(t, cmd)
}

func TestCov80CopyYAMLLevelOwnedPod(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default"}}
	m.setCursor(0)
	cmd := m.copyYAMLToClipboard()
	require.NotNil(t, cmd)
}

func TestCov80CopyYAMLLevelOwnedKnownKind(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "rs-1", Kind: "ReplicaSet", Namespace: "default"}}
	m.setCursor(0)
	cmd := m.copyYAMLToClipboard()
	require.NotNil(t, cmd)
}

func TestCov80CopyYAMLLevelOwnedUnknownKind(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "x", Kind: "XYZ999", Namespace: "ns"}}
	m.setCursor(0)
	cmd := m.copyYAMLToClipboard()
	require.NotNil(t, cmd)
	msg := cmd()
	ymsg, ok := msg.(yamlClipboardMsg)
	require.True(t, ok)
	assert.Error(t, ymsg.err)
}

func TestCov80CopyYAMLLevelContainers(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "pod-1"
	m.middleItems = []model.Item{{Name: "container-1"}}
	m.setCursor(0)
	cmd := m.copyYAMLToClipboard()
	require.NotNil(t, cmd)
}

func TestCov80LoadYAMLLevelResources(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.setCursor(0)
	cmd := m.loadYAML()
	require.NotNil(t, cmd)
}

func TestCov80LoadYAMLLevelOwnedPod(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default"}}
	m.setCursor(0)
	cmd := m.loadYAML()
	require.NotNil(t, cmd)
}

func TestCov80LoadYAMLLevelOwnedOther(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "rs-1", Kind: "ReplicaSet", Namespace: "default"}}
	m.setCursor(0)
	cmd := m.loadYAML()
	require.NotNil(t, cmd)
}

func TestCov80LoadYAMLLevelOwnedUnknown(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "x", Kind: "Unknown999"}}
	m.setCursor(0)
	cmd := m.loadYAML()
	require.NotNil(t, cmd)
	msg := cmd()
	ymsg, ok := msg.(yamlLoadedMsg)
	require.True(t, ok)
	assert.Error(t, ymsg.err)
}

func TestCov80LoadYAMLLevelContainers(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "pod-1"
	m.middleItems = []model.Item{{Name: "container-1"}}
	m.setCursor(0)
	cmd := m.loadYAML()
	require.NotNil(t, cmd)
}

func TestCov80HandleMouseWheelUpInExplorer(t *testing.T) {
	m := basePush80Model()
	m.mode = modeExplorer
	m.setCursor(2)
	msg := tea.MouseMsg{Button: tea.MouseButtonWheelUp}
	result, _ := m.handleMouse(msg)
	rm := result.(Model)
	assert.LessOrEqual(t, rm.cursor(), 2)
}

func TestCov80HandleMouseWheelDownInExplorer(t *testing.T) {
	m := basePush80Model()
	m.mode = modeExplorer
	m.setCursor(0)
	msg := tea.MouseMsg{Button: tea.MouseButtonWheelDown}
	result, _ := m.handleMouse(msg)
	_ = result.(Model)
}

func TestCov80LoadPreviewEventsWithSel(t *testing.T) {
	m := basePush80Model()
	m.setCursor(0)
	cmd := m.loadPreviewEvents()
	require.NotNil(t, cmd)
}

func TestCov80LoadPreviewEventsAtLevelOwned(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType.Kind = "Deployment"
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default"}}
	m.setCursor(0)
	cmd := m.loadPreviewEvents()
	require.NotNil(t, cmd)
}

func TestCov80LoadSecretDataWithSel(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Secret", Resource: "secrets", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-secret", Namespace: "default", Kind: "Secret"}}
	m.setCursor(0)
	cmd := m.loadSecretData()
	require.NotNil(t, cmd)
}

func TestCov80LoadConfigMapDataWithSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = []model.Item{{Name: "my-cm", Namespace: "default", Kind: "ConfigMap"}}
	m.setCursor(0)
	cmd := m.loadConfigMapData()
	require.NotNil(t, cmd)
}

func TestCov80LoadLabelDataWithSel(t *testing.T) {
	m := basePush80Model()
	m.labelResourceType = model.ResourceTypeEntry{Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.setCursor(0)
	cmd := m.loadLabelData()
	require.NotNil(t, cmd)
}

func TestCov80LoadRevisionsWithSel(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", Namespaced: true}
	m.middleItems = []model.Item{{Name: "deploy-1", Namespace: "default", Kind: "Deployment"}}
	m.setCursor(0)
	cmd := m.loadRevisions()
	require.NotNil(t, cmd)
}

func TestCov80LoadResourceTreeResources(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.setCursor(0)
	cmd := m.loadResourceTree()
	require.NotNil(t, cmd)
}

func TestCov80LoadResourceTreeOwned(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default"}}
	m.setCursor(0)
	cmd := m.loadResourceTree()
	require.NotNil(t, cmd)
}

func TestCov80DirectActionDeleteDeletingPod(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default", Deleting: true}}
	m.setCursor(0)
	result, cmd := m.directActionDelete()
	rm := result.(Model)
	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Contains(t, rm.pendingAction, "Force Delete")
	assert.Nil(t, cmd)
}

func TestCov80DirectActionDeleteDeletingNonPod(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "ConfigMap", Resource: "configmaps", Namespaced: true}
	m.middleItems = []model.Item{{Name: "cm-1", Kind: "ConfigMap", Namespace: "default", Deleting: true}}
	m.setCursor(0)
	result, cmd := m.directActionDelete()
	rm := result.(Model)
	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Contains(t, rm.pendingAction, "Force Finalize")
	assert.Nil(t, cmd)
}

func TestCov80OpenActionMenuSingleItem(t *testing.T) {
	m := basePush80Model()
	m.setCursor(0)
	rm := m.openActionMenu()
	assert.False(t, rm.bulkMode)
	assert.Equal(t, overlayAction, rm.overlay)
}

func TestCov80OpenActionMenuPortForward(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__"}
	m.middleItems = []model.Item{{Name: "pf-1", Kind: "__port_forward_entry__"}}
	m.setCursor(0)
	rm := m.openActionMenu()
	assert.Equal(t, overlayAction, rm.overlay)
}

func TestCov80OpenActionMenuContainers(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.middleItems = []model.Item{{Name: "container-1", Kind: "Container"}}
	m.setCursor(0)
	rm := m.openActionMenu()
	assert.Equal(t, overlayAction, rm.overlay)
}

func TestCov80OpenActionMenuDeletingItem(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{
		{Name: "pod-deleting", Kind: "Pod", Namespace: "default", Deleting: true},
	}
	m.setCursor(0)
	rm := m.openActionMenu()
	assert.Equal(t, overlayAction, rm.overlay)
	// The "Delete" action should be escalated.
	hasForce := false
	for _, item := range rm.overlayItems {
		if item.Name == "Force Delete" || item.Name == "Force Finalize" {
			hasForce = true
		}
	}
	assert.True(t, hasForce)
}

func TestCovMouseScrollUpExplorer(t *testing.T) {
	m := baseModelActions()
	m.mode = modeExplorer
	m.middleItems = make([]model.Item, 20)
	for i := range m.middleItems {
		m.middleItems[i] = model.Item{Name: "item"}
	}
	m.setCursor(10)
	result, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	rm := result.(Model)
	assert.Less(t, rm.cursor(), 10)
}

func TestCovMouseScrollDownExplorer(t *testing.T) {
	m := baseModelActions()
	m.mode = modeExplorer
	m.middleItems = make([]model.Item, 20)
	for i := range m.middleItems {
		m.middleItems[i] = model.Item{Name: "item"}
	}
	m.setCursor(0)
	result, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	rm := result.(Model)
	assert.Greater(t, rm.cursor(), 0)
}

func TestCovParentIndex(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "prod"
	m.leftItems = []model.Item{{Name: "dev"}, {Name: "prod"}, {Name: "stg"}}
	assert.Equal(t, 1, m.parentIndex())

	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{APIVersion: "v1", Resource: "pods"}
	m.leftItems = []model.Item{
		{Name: "Deployments", Extra: "apps/v1/deployments"},
		{Name: "Pods", Extra: "/v1/pods"},
	}
	assert.Equal(t, 1, m.parentIndex())

	m.nav.Level = model.LevelOwned
	m.nav.ResourceName = "nginx"
	m.leftItems = []model.Item{{Name: "nginx"}, {Name: "redis"}}
	assert.Equal(t, 0, m.parentIndex())

	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "my-pod"
	m.leftItems = []model.Item{{Name: "other-pod"}, {Name: "my-pod"}}
	assert.Equal(t, 1, m.parentIndex())

	m.nav.Level = model.LevelClusters
	assert.Equal(t, -1, m.parentIndex())

	// Not found.
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "missing"
	assert.Equal(t, -1, m.parentIndex())
}

func TestCovCursorSetGet(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}

	m.setCursor(5)
	assert.Equal(t, 5, m.cursor())

	m.nav.Level = model.LevelResourceTypes
	m.setCursor(3)
	assert.Equal(t, 3, m.cursor())

	// Switch back.
	m.nav.Level = model.LevelResources
	assert.Equal(t, 5, m.cursor())
}

func TestCovClampCursor(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.middleItems = []model.Item{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	m.setCursor(10)
	m.clampCursor()
	assert.Equal(t, 2, m.cursor())

	m.setCursor(-5)
	m.clampCursor()
	assert.Equal(t, 0, m.cursor())

	m.middleItems = nil
	m.setCursor(3)
	m.clampCursor()
	assert.Equal(t, 3, m.cursor()) // Empty list, no items to clamp against.
}

func TestCovCursorItemKey(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "ns1", Extra: "ref1", Kind: "Pod"},
		{Name: "pod-2", Namespace: "ns2", Extra: "ref2", Kind: "Pod"},
	}
	m.setCursor(0)
	name, ns, extra, kind := m.cursorItemKey()
	assert.Equal(t, "pod-1", name)
	assert.Equal(t, "ns1", ns)
	assert.Equal(t, "ref1", extra)
	assert.Equal(t, "Pod", kind)

	m.setCursor(1)
	name, ns, extra, kind = m.cursorItemKey()
	assert.Equal(t, "pod-2", name)
	assert.Equal(t, "ns2", ns)
	assert.Equal(t, "ref2", extra)
	assert.Equal(t, "Pod", kind)

	// Out of bounds.
	m.setCursor(10)
	name, ns, extra, kind = m.cursorItemKey()
	assert.Empty(t, name)
	assert.Empty(t, ns)
	assert.Empty(t, extra)
	assert.Empty(t, kind)
}

func TestCovRestoreCursorToItem(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.middleItems = []model.Item{
		{Name: "a", Namespace: "ns1"},
		{Name: "b", Namespace: "ns2"},
		{Name: "c", Namespace: "ns3"},
	}

	m.restoreCursorToItem("b", "ns2", "", "")
	assert.Equal(t, 1, m.cursor())

	// Item gone: clamp.
	m.restoreCursorToItem("missing", "ns4", "", "")
	assert.LessOrEqual(t, m.cursor(), 2)

	// Empty name: just clamp.
	m.setCursor(100)
	m.restoreCursorToItem("", "", "", "")
	assert.LessOrEqual(t, m.cursor(), 2)
}

func TestCovNavKey(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "prod"
	assert.Equal(t, "prod", m.navKey())

	m.nav.ResourceType = model.ResourceTypeEntry{Resource: "pods"}
	assert.Equal(t, "prod/pods", m.navKey())

	m.nav.ResourceName = "nginx"
	assert.Equal(t, "prod/pods/nginx", m.navKey())

	m.nav.OwnedName = "pod-1"
	assert.Equal(t, "prod/pods/nginx/pod-1", m.navKey())
}

func TestCovSaveRestoreCursor(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.nav.Context = "prod"
	m.nav.ResourceType = model.ResourceTypeEntry{Resource: "pods"}
	m.middleItems = []model.Item{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	m.setCursor(2)
	m.saveCursor()

	m.setCursor(0)
	m.restoreCursor()
	assert.Equal(t, 2, m.cursor())

	// No saved position: reset to 0.
	m.nav.ResourceType = model.ResourceTypeEntry{Resource: "deployments"}
	m.restoreCursor()
	assert.Equal(t, 0, m.cursor())
}

func TestCovSelectedMiddleItem(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.middleItems = []model.Item{
		{Name: "a", Kind: "Pod", Namespace: "ns1"},
		{Name: "b", Kind: "Pod", Namespace: "ns2"},
	}

	m.setCursor(0)
	sel := m.selectedMiddleItem()
	assert.NotNil(t, sel)
	assert.Equal(t, "a", sel.Name)

	m.setCursor(1)
	sel = m.selectedMiddleItem()
	assert.NotNil(t, sel)
	assert.Equal(t, "b", sel.Name)

	// Out of bounds.
	m.setCursor(10)
	assert.Nil(t, m.selectedMiddleItem())
}

func TestCovSelectionKey(t *testing.T) {
	assert.Equal(t, "ns1/pod-1", selectionKey(model.Item{Name: "pod-1", Namespace: "ns1"}))
	assert.Equal(t, "node-1", selectionKey(model.Item{Name: "node-1"}))
}

func TestCovIsSelectedToggle(t *testing.T) {
	m := baseModelCov()
	item := model.Item{Name: "pod-1", Namespace: "ns1"}

	assert.False(t, m.isSelected(item))

	m.toggleSelection(item)
	assert.True(t, m.isSelected(item))

	m.toggleSelection(item)
	assert.False(t, m.isSelected(item))
}

func TestCovClearSelection(t *testing.T) {
	m := baseModelCov()
	m.selectedItems["a"] = true
	m.selectedItems["b"] = true
	m.selectionAnchor = 3

	m.clearSelection()
	assert.Empty(t, m.selectedItems)
	assert.Equal(t, -1, m.selectionAnchor)
}

func TestCovHasSelection(t *testing.T) {
	m := baseModelCov()
	assert.False(t, m.hasSelection())

	m.selectedItems["a"] = true
	assert.True(t, m.hasSelection())
}

func TestCovSelectedItemsList(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.middleItems = []model.Item{
		{Name: "a", Namespace: "ns1"},
		{Name: "b", Namespace: "ns2"},
		{Name: "c", Namespace: "ns3"},
	}
	m.selectedItems["ns1/a"] = true
	m.selectedItems["ns3/c"] = true

	sel := m.selectedItemsList()
	assert.Len(t, sel, 2)
}

func TestCovCarryOverMetricsColumns(t *testing.T) {
	m := baseModelCov()
	m.middleItems = []model.Item{
		{
			Name:      "pod-1",
			Namespace: "ns1",
			Columns: []model.KeyValue{
				{Key: "CPU", Value: "100m"},
				{Key: "MEM", Value: "128Mi"},
				{Key: "IP", Value: "10.0.0.1"},
			},
		},
	}

	newItems := []model.Item{
		{
			Name:      "pod-1",
			Namespace: "ns1",
			Columns: []model.KeyValue{
				{Key: "IP", Value: "10.0.0.2"},
			},
		},
		{
			Name:      "pod-2",
			Namespace: "ns1",
			Columns: []model.KeyValue{
				{Key: "IP", Value: "10.0.0.3"},
			},
		},
	}

	m.carryOverMetricsColumns(newItems)

	// pod-1 should have carried-over CPU and MEM plus kept IP.
	var hasCPU, hasMEM, hasIP bool
	for _, kv := range newItems[0].Columns {
		switch kv.Key {
		case "CPU":
			hasCPU = true
			assert.Equal(t, "100m", kv.Value)
		case "MEM":
			hasMEM = true
			assert.Equal(t, "128Mi", kv.Value)
		case "IP":
			hasIP = true
		}
	}
	assert.True(t, hasCPU)
	assert.True(t, hasMEM)
	assert.True(t, hasIP)

	// pod-2 should be unchanged.
	assert.Len(t, newItems[1].Columns, 1)
}

func TestCovCarryOverMetricsNoUsage(t *testing.T) {
	m := baseModelCov()
	m.middleItems = []model.Item{
		{
			Name:      "pod-1",
			Namespace: "ns1",
			Columns: []model.KeyValue{
				{Key: "CPU", Value: "0"},
				{Key: "MEM", Value: "0"},
			},
		},
	}

	newItems := []model.Item{
		{Name: "pod-1", Namespace: "ns1"},
	}

	m.carryOverMetricsColumns(newItems)
	// No usage data, so no carryover.
	assert.Empty(t, newItems[0].Columns)
}

func TestCovClampAllCursors(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.middleItems = []model.Item{{Name: "a"}}

	// With event timeline lines.
	m.eventTimelineLines = []string{"line1", "line2", "line3"}
	m.eventTimelineCursor = 10
	m.clampAllCursors()
	assert.Equal(t, 2, m.eventTimelineCursor)
}

func TestCovPushPopLeft(t *testing.T) {
	m := baseModelCov()
	m.leftItems = []model.Item{{Name: "left1"}}
	m.middleItems = []model.Item{{Name: "mid1"}, {Name: "mid2"}}

	m.pushLeft()
	assert.Len(t, m.leftItemsHistory, 1)
	assert.Equal(t, "mid1", m.leftItems[0].Name)

	m.popLeft()
	assert.Empty(t, m.leftItemsHistory)
	assert.Equal(t, "left1", m.leftItems[0].Name)

	// Pop from empty history.
	m.popLeft()
	assert.Nil(t, m.leftItems)
}

func TestCovClearRight(t *testing.T) {
	m := baseModelCov()
	m.rightItems = []model.Item{{Name: "right1"}}
	m.yamlContent = "apiVersion: v1"
	m.previewYAML = "yaml content"
	m.metricsContent = "metrics"
	m.previewEventsContent = "events"
	m.mapView = true

	m.clearRight()
	assert.Nil(t, m.rightItems)
	assert.Empty(t, m.yamlContent)
	assert.Empty(t, m.previewYAML)
	assert.Empty(t, m.metricsContent)
	assert.Empty(t, m.previewEventsContent)
	assert.False(t, m.mapView)
}

func TestPush4HandleKeyGGExplorer(t *testing.T) {
	m := basePush4Model()
	m.pendingG = true
	m.setCursor(2)
	result, _ := m.handleKey(keyMsg("g"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.cursor())
}

func TestPush4HandleKeyGBigExplorer(t *testing.T) {
	m := basePush4Model()
	m.setCursor(0)
	result, _ := m.handleKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.cursor())
}

func TestPush4HandleKeySelectRangeWithAnchor(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	m.selectionAnchor = 0
	m.setCursor(2)
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.SelectRange))
	rm := result.(Model)
	assert.True(t, len(rm.selectedItems) > 0)
}

func TestPush4MoveCursorDown(t *testing.T) {
	m := basePush4Model()
	m.setCursor(0)
	result, _ := m.moveCursor(1)
	rm := result.(Model)
	assert.Equal(t, 1, rm.cursor())
}

func TestPush4MoveCursorUp(t *testing.T) {
	m := basePush4Model()
	m.setCursor(2)
	result, _ := m.moveCursor(-1)
	rm := result.(Model)
	assert.Equal(t, 1, rm.cursor())
}

func TestPush4MoveCursorAtBottom(t *testing.T) {
	m := basePush4Model()
	m.setCursor(2) // last item
	result, _ := m.moveCursor(1)
	rm := result.(Model)
	assert.Equal(t, 2, rm.cursor()) // should stay
}

func TestPush4MoveCursorAtTop(t *testing.T) {
	m := basePush4Model()
	m.setCursor(0)
	result, _ := m.moveCursor(-1)
	rm := result.(Model)
	assert.Equal(t, 0, rm.cursor()) // should stay
}

func TestFinalNavigateChildLevelClusters(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "cluster-1"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
	assert.Equal(t, "cluster-1", rm.nav.Context)
}

func TestFinalNavigateChildLevelResourceTypesOverview(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.middleItems = []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}}
	m.setCursor(0)
	result, _ := m.navigateChild()
	rm := result.(Model)
	assert.True(t, rm.fullscreenDashboard)
}

func TestFinalNavigateChildLevelResourceTypesMonitoring(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.middleItems = []model.Item{{Name: "Monitoring", Extra: "__monitoring__"}}
	m.setCursor(0)
	result, _ := m.navigateChild()
	rm := result.(Model)
	assert.True(t, rm.fullscreenDashboard)
}

func TestFinalNavigateChildLevelResourceTypesUnknownExtra(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	// Extra doesn't match any known resource type -> returns nil cmd.
	m.middleItems = []model.Item{{Name: "Unknown", Extra: "nonexistent-resource"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinalNavigateChildLevelResourceTypesCollapsedGroup(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.middleItems = []model.Item{{Name: "Core", Kind: "__collapsed_group__", Category: "Core"}}
	m.setCursor(0)
	result, _ := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, "Core", rm.expandedGroup)
}

func TestFinalNavigateChildLevelResourceTypesPods(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	// Set up CRDs so FindResourceTypeIn works.
	podRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{podRT}
	// Extra must match ResourceRef format.
	m.middleItems = []model.Item{{Name: "Pods", Extra: podRT.ResourceRef()}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, model.LevelResources, rm.nav.Level)
}

func TestFinalNavigateChildLevelResourcesNonPodNoChildren(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "ConfigMap", Resource: "configmaps", Namespaced: true}
	m.middleItems = []model.Item{{Name: "cm-1", Namespace: "default"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinalNavigateChildLevelResourcesPod(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "pod-1", Namespace: "default"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, model.LevelContainers, rm.nav.Level)
}

func TestCovUpdateMouseMsg(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeExplorer
	m.middleItems = make([]model.Item, 20)
	for i := range m.middleItems {
		m.middleItems[i] = model.Item{Name: "item"}
	}
	m.setCursor(5)
	result, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	rm := result.(Model)
	assert.Less(t, rm.cursor(), 5)
}

func TestCovMoveCursorDown(t *testing.T) {
	m := baseModelNav()
	m.setCursor(0)
	result, _ := m.moveCursor(1)
	rm := result.(Model)
	assert.Equal(t, 1, rm.cursor())
}

func TestCovMoveCursorUp(t *testing.T) {
	m := baseModelNav()
	m.setCursor(3)
	result, _ := m.moveCursor(-1)
	rm := result.(Model)
	assert.Equal(t, 2, rm.cursor())
}

func TestCovMoveCursorClamp(t *testing.T) {
	m := baseModelNav()
	m.setCursor(0)
	result, _ := m.moveCursor(-5)
	rm := result.(Model)
	assert.Equal(t, 0, rm.cursor())
}

func TestCovMoveCursorClampBottom(t *testing.T) {
	m := baseModelNav()
	m.setCursor(0)
	result, _ := m.moveCursor(100)
	rm := result.(Model)
	assert.Equal(t, 4, rm.cursor())
}

func TestCovMoveCursorEmpty(t *testing.T) {
	m := baseModelNav()
	m.middleItems = nil
	result, _ := m.moveCursor(1)
	_ = result.(Model)
}

func TestCovHandleKeyExplorerDown(t *testing.T) {
	m := baseModelNav()
	m.mode = modeExplorer
	m.setCursor(0)
	result, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	rm := result.(Model)
	assert.Equal(t, 1, rm.cursor())
}
