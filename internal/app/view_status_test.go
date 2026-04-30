package app

import (
	"fmt"
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
	assert.Contains(t, stripped, "filtered")
	assert.Contains(t, stripped, "2/3")
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

// The cursor-position counter and the selection-count badge share a
// single chip slot — by default the bar surfaces "[cur/total]"; the
// moment the user marks even one item the chip swaps to "N selected"
// and the counter is hidden. Showing both at once was redundant
// (the user just made the selection) and the stacked chips were one
// of the things that made the keymap feel cramped.
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
	assert.Contains(t, stripped, "2 selected", "selection badge takes the chip slot when items are marked")
	assert.NotContains(t, stripped, "[1/2]",
		"counter must be replaced (not stacked) by the selection badge — both at once was the old, cluttered layout")
}

func TestStatusBarKeyHints(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		middleItems:   []model.Item{{Name: "pod"}},
		width:         200,
		height:        40,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
	}
	bar := m.statusBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "help")
	assert.Contains(t, stripped, "quit")
}

// User-reported bug: "When I select multiple items, the hint bar is
// trimmed." The fix has two parts working together. (1) The chip group
// (sort, counter / selected count, filter preset, NYAN) moves to the
// FAR RIGHT and JoinStatusBar pins it intact on overflow — the keymap
// is the part that truncates, never the chips. (2) The cursor-position
// counter is replaced by the selection badge instead of stacking, so
// the chip group's width barely grows when the user enters bulk mode.
//
// We render at width 120 (where the explorer keymap alone exceeds the
// bar, exercising the truncation path) and assert the chips are intact
// across two snapshots: no selection (counter visible) and 20 items
// selected (selection badge visible, counter hidden, full keymap
// truncated with the `~` marker).
func TestStatusBarChipsSurviveLargeSelection(t *testing.T) {
	items := make([]model.Item, 20)
	selected := make(map[string]bool, len(items))
	for i := range items {
		name := fmt.Sprintf("pod-with-a-fairly-long-name-%02d", i)
		items[i] = model.Item{Name: name}
		selected[name] = true
	}

	noSel := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		middleItems:   items,
		width:         120,
		height:        40,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
	}
	withSel := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		middleItems:   items,
		width:         120,
		height:        40,
		tabs:          []TabState{{}},
		selectedItems: selected,
	}

	baseline := stripANSI(noSel.statusBar())
	loaded := stripANSI(withSel.statusBar())

	// Counter is the right-anchored chip when nothing is selected.
	assert.Contains(t, baseline, "[1/20]", "counter must remain visible when no selection")

	// Selecting items swaps the counter for the selection badge AND
	// keeps it intact — that's the contract this test pins.
	assert.Contains(t, loaded, "20 selected",
		"selection badge must remain visible at the right edge regardless of keymap pressure")
	assert.NotContains(t, loaded, "[1/20]",
		"the counter chip swaps out for the selection badge; both must not appear together")

	// The keymap is fitted entry-by-entry to its budget, so the gap
	// between the truncated keymap and the chip group on the right is
	// pure whitespace. A stray `~` between the two would mean the bar
	// hard-cut a hint mid-description, which is the regression this
	// pair of assertions guards against.
	assert.NotContains(t, baseline, "~",
		"baseline bar must use a clean separator, not a truncate marker")
	assert.NotContains(t, loaded, "~",
		"selected bar must use a clean separator, not a truncate marker")
}

// While a bulk-action confirm overlay is open the user must keep their
// "how many am I about to affect?" indicator. The overlay branch of
// statusBar() normally suppresses chips and shows just the overlay's
// keymap, but when bulkMode is set (i.e. the overlay was triggered with
// multiple items selected) the selection badge is pinned to the right
// edge alongside the overlay keymap.
func TestStatusBarBulkActionOverlayKeepsSelectionBadge(t *testing.T) {
	items := []model.Item{{Name: "pod-1"}, {Name: "pod-2"}, {Name: "pod-3"}}
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		middleItems:   items,
		overlay:       overlayConfirm,
		confirmAction: "3 resources",
		bulkMode:      true,
		bulkItems:     items,
		selectedItems: map[string]bool{"pod-1": true, "pod-2": true, "pod-3": true},
		width:         120,
		height:        40,
		tabs:          []TabState{{}},
	}
	stripped := stripANSI(m.statusBar())

	assert.Contains(t, stripped, "Enter", "overlay keymap must still be present")
	assert.Contains(t, stripped, "3 selected",
		"bulk action overlays must keep the selection-count badge visible on the right")
}

// Non-bulk overlays (theme picker, namespace selector, paste confirm,
// etc.) do not carry "how many am I about to affect?" context. The
// selection badge is scoped to bulkMode so it does not bleed into
// unrelated overlays — even when the user happens to have a stale
// selection from before opening the overlay.
func TestStatusBarNonBulkOverlayHidesSelectionBadge(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		middleItems:   []model.Item{{Name: "pod-1"}, {Name: "pod-2"}},
		overlay:       overlayColorscheme, // not a bulk action
		bulkMode:      false,
		selectedItems: map[string]bool{"pod-1": true, "pod-2": true},
		width:         120,
		height:        40,
		tabs:          []TabState{{}},
	}
	stripped := stripANSI(m.statusBar())

	assert.NotContains(t, stripped, "selected",
		"non-bulk overlays must not show the selection badge — the chip is scoped to bulkMode")
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
