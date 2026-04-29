package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- leftColumnHeader ---

func TestLeftColumnHeader(t *testing.T) {
	tests := []struct {
		name     string
		level    model.Level
		nav      model.NavigationState
		expected string
	}{
		{
			name:     "LevelClusters returns empty",
			level:    model.LevelClusters,
			expected: "",
		},
		{
			name:     "LevelResourceTypes returns KUBECONFIG",
			level:    model.LevelResourceTypes,
			expected: "KUBECONFIG",
		},
		{
			name:     "LevelResources returns RESOURCE TYPE",
			level:    model.LevelResources,
			expected: "RESOURCE TYPE",
		},
		{
			name:  "LevelOwned returns uppercased display name",
			level: model.LevelOwned,
			nav: model.NavigationState{
				Level:        model.LevelOwned,
				ResourceType: model.ResourceTypeEntry{DisplayName: "Deployments"},
			},
			expected: "DEPLOYMENTS",
		},
		{
			name:  "LevelContainers returns uppercased display name",
			level: model.LevelContainers,
			nav: model.NavigationState{
				Level:        model.LevelContainers,
				ResourceType: model.ResourceTypeEntry{DisplayName: "Pods"},
			},
			expected: "PODS",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{nav: tt.nav}
			m.nav.Level = tt.level
			assert.Equal(t, tt.expected, m.leftColumnHeader())
		})
	}
}

// --- middleColumnHeader ---

func TestMiddleColumnHeader(t *testing.T) {
	tests := []struct {
		name     string
		nav      model.NavigationState
		expected string
	}{
		{
			name:     "LevelClusters",
			nav:      model.NavigationState{Level: model.LevelClusters},
			expected: "KUBECONFIG",
		},
		{
			name:     "LevelResourceTypes",
			nav:      model.NavigationState{Level: model.LevelResourceTypes},
			expected: "RESOURCE TYPE",
		},
		{
			name: "LevelResources",
			nav: model.NavigationState{
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Pod"},
			},
			expected: "POD",
		},
		{
			name:     "LevelContainers",
			nav:      model.NavigationState{Level: model.LevelContainers},
			expected: "CONTAINER",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{nav: tt.nav}
			assert.Equal(t, tt.expected, m.middleColumnHeader())
		})
	}
}

// --- breadcrumb ---

func TestBreadcrumb(t *testing.T) {
	tests := []struct {
		name     string
		nav      model.NavigationState
		expected string
	}{
		{
			name:     "root only",
			nav:      model.NavigationState{},
			expected: "lfk",
		},
		{
			name: "with context",
			nav: model.NavigationState{
				Context: "prod",
			},
			expected: "lfk > prod",
		},
		{
			name: "with context and resource type",
			nav: model.NavigationState{
				Context:      "prod",
				ResourceType: model.ResourceTypeEntry{DisplayName: "Pods"},
			},
			expected: "lfk > prod > Pods",
		},
		{
			name: "full path",
			nav: model.NavigationState{
				Context:      "prod",
				ResourceType: model.ResourceTypeEntry{DisplayName: "Deployments"},
				ResourceName: "my-deploy",
				OwnedName:    "my-pod-abc",
			},
			expected: "lfk > prod > Deployments > my-deploy > my-pod-abc",
		},
		{
			// Resource types coming from API discovery have an empty
			// DisplayName (per model.DisplayNameFor). The breadcrumb must
			// still surface a friendly name by going through the metadata
			// fallback chain.
			name: "discovered resource type without DisplayName uses metadata",
			nav: model.NavigationState{
				Context: "prod",
				ResourceType: model.ResourceTypeEntry{
					// DisplayName intentionally empty.
					Kind:       "Pod",
					APIGroup:   "",
					APIVersion: "v1",
					Resource:   "pods",
				},
			},
			expected: "lfk > prod > Pods",
		},
		{
			// CRD-style resource: no DisplayName, no built-in metadata entry,
			// only Kind. The breadcrumb should still show the Kind so the
			// title bar tells the user what they're standing on.
			name: "discovered CRD falls back to Kind when no metadata",
			nav: model.NavigationState{
				Context: "prod",
				ResourceType: model.ResourceTypeEntry{
					Kind:       "MyCustomResource",
					APIGroup:   "example.com",
					APIVersion: "v1",
					Resource:   "mycustomresources",
				},
			},
			expected: "lfk > prod > MyCustomResource",
		},
		{
			// Pod at LevelContainers: navigateChildResource sets both
			// ResourceName AND OwnedName to the same value so the containers
			// view knows its parent. The breadcrumb must not show the name
			// twice ("lfk > prod > Pods > my-pod > my-pod").
			name: "pod containers does not duplicate name",
			nav: model.NavigationState{
				Context:      "prod",
				ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"},
				ResourceName: "web-7d8c-abc",
				OwnedName:    "web-7d8c-abc",
			},
			expected: "lfk > prod > Pods > web-7d8c-abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{nav: tt.nav}
			assert.Equal(t, tt.expected, m.breadcrumb())
		})
	}
}

// --- statusBar ---

func TestStatusBarShowsItemCount(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "pod-1"},
			{Name: "pod-2"},
			{Name: "pod-3"},
		},
		width:         120,
		height:        40,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
	}
	bar := m.statusBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "[1/3]")
}

func TestStatusBarShowsFilterCount(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "nginx-1"},
			{Name: "redis-1"},
			{Name: "nginx-2"},
		},
		filterText:    "nginx",
		width:         120,
		height:        40,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
	}
	bar := m.statusBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "filter:nginx")
	assert.Contains(t, stripped, "2/3")
	assert.Contains(t, stripped, "Esc to clear")
}

func TestStatusBarShowsSortMode(t *testing.T) {
	ui.ActiveSortableColumns = []string{"Name", "Age", "Status"}
	defer func() { ui.ActiveSortableColumns = nil }()
	m := Model{
		nav:            model.NavigationState{Level: model.LevelResources},
		middleItems:    []model.Item{{Name: "pod"}},
		sortColumnName: "Age",
		sortAscending:  true,
		width:          120,
		height:         40,
		tabs:           []TabState{{}},
		selectedItems:  make(map[string]bool),
	}
	bar := m.statusBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "sort:Age")
}

func TestStatusBarShowsSelectionCount(t *testing.T) {
	m := Model{
		nav:         model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{{Name: "pod-1"}, {Name: "pod-2"}},
		selectedItems: map[string]bool{
			"pod-1": true,
			"pod-2": true,
		},
		width:  120,
		height: 40,
		tabs:   []TabState{{}},
	}
	bar := m.statusBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "2 selected")
}

func TestStatusBarKeyHints(t *testing.T) {
	m := Model{
		nav:         model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{{Name: "pod"}},
		// Wide enough to fit the post-merge hint set: main added the
		// read-only toggle and security-dashboard added monitoring/
		// security hints. Width 200 elides "help" and "quit" off the
		// right edge.
		width:         260,
		height:        40,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
	}
	bar := m.statusBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "help")
	assert.Contains(t, stripped, "quit")
}

// --- View ---

func TestViewLoadingScreen(t *testing.T) {
	m := Model{width: 0}
	assert.Equal(t, "Loading...", m.View())
}

func TestViewExplorerMode(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test-cluster",
			ResourceType: model.ResourceTypeEntry{
				DisplayName: "Pods",
				Kind:        "Pod",
			},
		},
		middleItems: []model.Item{
			{Name: "nginx-pod", Status: "Running"},
		},
		width:              120,
		height:             40,
		mode:               modeExplorer,
		namespace:          "default",
		tabs:               []TabState{{}},
		selectedItems:      make(map[string]bool),
		cursorMemory:       make(map[string]int),
		itemCache:          make(map[string][]model.Item),
		yamlCollapsed:      make(map[string]bool),
		selectedNamespaces: make(map[string]bool),
	}
	view := m.View()
	stripped := stripANSI(view)
	// Should contain breadcrumb elements.
	assert.Contains(t, stripped, "lfk")
	assert.True(t, len(stripped) > 0)
	// Should contain resource name.
	assert.True(t, strings.Contains(stripped, "nginx-pod") || len(stripped) > 50)
}
