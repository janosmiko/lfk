package app

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// forceANSIRenderer flips the lipgloss renderer into ANSI mode and
// re-applies the theme so styles like SearchHighlightStyle emit real
// escape sequences. Tests run without a TTY so by default lipgloss
// returns plain text and our highlight assertions would be vacuous.
// Restored on test cleanup. Same trick the ui package's help tests use.
func forceANSIRenderer(t *testing.T) {
	t.Helper()
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	originalNoColor := ui.ConfigNoColor
	t.Cleanup(func() {
		lipgloss.DefaultRenderer().SetColorProfile(originalProfile)
		ui.ConfigNoColor = originalNoColor
		ui.ApplyTheme(ui.DefaultTheme())
	})
	ui.ConfigNoColor = false
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)
	ui.ApplyTheme(ui.DefaultTheme())
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)
}

// TestSearchHighlightDoesNotBleedIntoLeftColumn is the regression test
// for the search highlight bleeding into the parent (left) column. The
// search/filter highlight is meant for the middle column only — the
// left column shows the parent context (resource type categories like
// "Workloads"/"Networking", contexts, kubeconfigs, …) and matching
// text there must NOT be styled with SearchHighlightStyle just because
// the user typed `/workload` in the middle.
//
// Concrete trigger: the category-header path in RenderColumn calls
// highlightName(headerText, ActiveHighlightQuery) when the query is
// active. If viewExplorerThreeCol renders the left column without
// first clearing ActiveHighlightQuery, every category bar whose label
// contains the substring lights up.
//
// We pick a search query that matches a LEFT category header but not
// any MIDDLE item, render the explorer, and assert the highlight ANSI
// for that substring does not appear anywhere in the output.
func TestSearchHighlightDoesNotBleedIntoLeftColumn(t *testing.T) {
	forceANSIRenderer(t)

	const query = "Workloads" // matches the left category header
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test-ctx",
			ResourceType: model.ResourceTypeEntry{
				DisplayName: "Pods",
				Kind:        "Pod",
			},
		},
		// Left column shows category-grouped resource types — the bar
		// for the "Workloads" category is what the highlight bleeds into.
		leftItems: []model.Item{
			{Name: "Pods", Category: "Workloads"},
			{Name: "Deployments", Category: "Workloads"},
		},
		// Middle column items do NOT contain the substring, so any
		// highlight ANSI in the rendered output can only come from the
		// left column leaking.
		middleItems: []model.Item{
			{Name: "nginx-pod", Status: "Running", Ready: "1/1", Age: "3d"},
			{Name: "redis-pod", Status: "Running", Ready: "1/1", Age: "1d"},
		},
		searchInput: TextInput{
			Value:  query,
			Cursor: len(query),
		},
		width:              160,
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

	rendered := m.View()
	highlighted := ui.SearchHighlightStyle.Render(query)

	assert.NotContains(t, rendered, highlighted,
		"search highlight ANSI must not appear in the rendered view "+
			"when the query only matches a LEFT category header — the "+
			"highlight is for the middle (search target) column only")
}

// TestSearchHighlightStillAppliesToMiddleColumn guards against an
// over-correction: clearing the highlight too aggressively would also
// drop it from the middle column, defeating the search UX. With a
// query that matches the MIDDLE items, the highlight ANSI must still
// be present somewhere in the rendered view.
func TestSearchHighlightStillAppliesToMiddleColumn(t *testing.T) {
	forceANSIRenderer(t)

	const query = "nginx"
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test-ctx",
			ResourceType: model.ResourceTypeEntry{
				DisplayName: "Pods",
				Kind:        "Pod",
			},
		},
		leftItems: []model.Item{
			{Name: "Workloads", Category: "Workloads"},
		},
		// nginx-pod is the SECOND item so it falls on a non-cursor row,
		// where the highlight uses SearchHighlightStyle as a single ANSI
		// span. The cursor row's SelectedSearchHighlightStyle renders
		// per-character (Bold+Underline), splitting the substring across
		// ANSI codes and making a substring assertion vacuously hard.
		middleItems: []model.Item{
			{Name: "redis-pod", Status: "Running", Ready: "1/1", Age: "1d"},
			{Name: "nginx-pod", Status: "Running", Ready: "1/1", Age: "3d"},
		},
		searchInput: TextInput{
			Value:  query,
			Cursor: len(query),
		},
		width:              160,
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

	rendered := m.View()
	highlighted := ui.SearchHighlightStyle.Render(query)

	assert.Contains(t, rendered, highlighted,
		"middle-column matches on non-cursor rows must still be wrapped in "+
			"SearchHighlightStyle — clearing ActiveHighlightQuery for the side "+
			"columns must not also strip the middle column")
}
