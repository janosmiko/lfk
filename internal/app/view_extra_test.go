package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestViewExplorerDashboardTwoColFillsThemeBackground guards issue #293: the
// two-column fullscreen dashboard (cluster overview with a warning-events
// column) must re-apply the theme background after every ANSI reset in its
// content, otherwise styled spans leave the terminal's default background
// (black under non-black themes) "torn" into the panel.
//
// FillLinesBg rewrites every interior reset to "reset + bgSeq". The lipgloss
// column wrapper does not re-apply its background after resets inside the
// content it wraps, so the "reset + bgSeq" adjacency only appears when the
// two-col path itself ran FillLinesBg — which is exactly the fix.
func TestViewExplorerDashboardTwoColFillsThemeBackground(t *testing.T) {
	// lipgloss downgrades to the Ascii profile without a TTY (emits no ANSI),
	// which would make FillLinesBg a no-op. Force a color profile so the
	// background sequences are actually rendered.
	origProfile := lipgloss.DefaultRenderer().ColorProfile()
	origTheme := ui.ActiveTheme
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	ui.ApplyTheme(ui.DefaultTheme())
	t.Cleanup(func() {
		lipgloss.DefaultRenderer().SetColorProfile(origProfile)
		ui.ApplyTheme(origTheme)
	})

	// A styled span ends with an ANSI reset; the line is shorter than the
	// column so per-column padding follows it — the exact tear condition.
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555")).Render("GAUGE")
	m := Model{
		nav:                 model.NavigationState{Level: model.LevelResourceTypes, Context: "test"},
		middleItems:         []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}},
		width:               120,
		height:              40,
		mode:                modeExplorer,
		namespace:           "default",
		fullscreenDashboard: true,
		dashboardPreview:    styled + "\nNODES: 3 Ready\nPODS: 42 Running",
		dashboardEventsPreview: "RECENT WARNING EVENTS\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("#ffaa00")).Render("Warning") + " pod crashed",
		tabs:               []TabState{{}},
		selectedItems:      make(map[string]bool),
		cursorMemory:       make(map[string]int),
		itemCache:          make(map[string][]model.Item),
		yamlCollapsed:      make(map[string]bool),
		selectedNamespaces: make(map[string]bool),
	}

	out := m.View()

	// Guard against a vacuous pass: with the forced TrueColor profile the view
	// must contain ANSI styling, otherwise there are no resets to assert on.
	assert.Contains(t, out, "\x1b[", "forced color profile must emit ANSI sequences")
	// A reset immediately followed by a space is un-backgrounded padding — the
	// "black tear". FillLinesBg rewrites every reset to "reset + bgSeq", so no
	// bare "reset space" survives once the two-col path fills the background.
	assert.NotContains(t, out, "\x1b[0m ",
		"two-col dashboard must re-apply the theme background after interior ANSI resets")

	// Layout guard: rows must fit the wrapper's content area. If they overflow
	// (rows built to fullW instead of fullW-2), lipgloss wraps every row and
	// inserts a blank tail line between consecutive rows, so the left column's
	// adjacent lines stop being adjacent. Assert the two dashboard lines render
	// on consecutive output rows.
	strippedLines := strings.Split(stripANSI(out), "\n")
	nodesLine, podsLine := -1, -1
	for i, ln := range strippedLines {
		if strings.Contains(ln, "NODES: 3 Ready") {
			nodesLine = i
		}
		if strings.Contains(ln, "PODS: 42 Running") {
			podsLine = i
		}
	}
	assert.NotEqual(t, -1, nodesLine, "NODES line must render")
	assert.Equal(t, nodesLine+1, podsLine,
		"two-col rows must not overflow the content area and wrap (no blank tail line between rows)")
}

// --- View: fullscreen modes ---

func TestViewYAMLModeWithVisualMode(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		mode:   modeYAML,
		nav: model.NavigationState{
			Level: model.LevelResources,
		},
		namespace:      "default",
		middleItems:    []model.Item{{Name: "my-configmap"}},
		yamlContent:    "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: my-configmap",
		yamlCollapsed:  make(map[string]bool),
		yamlVisualMode: true,
		yamlVisualType: 'V',
		tabs:           []TabState{{}},
	}
	output := m.View()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "VISUAL LINE")
}

func TestViewYAMLModeCharVisual(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		mode:   modeYAML,
		nav: model.NavigationState{
			Level: model.LevelResources,
		},
		namespace:      "default",
		middleItems:    []model.Item{{Name: "my-configmap"}},
		yamlContent:    "apiVersion: v1\nkind: ConfigMap",
		yamlCollapsed:  make(map[string]bool),
		yamlVisualMode: true,
		yamlVisualType: 'v',
		tabs:           []TabState{{}},
	}
	output := m.View()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "VISUAL]")
}

func TestViewYAMLModeBlockVisual(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		mode:   modeYAML,
		nav: model.NavigationState{
			Level: model.LevelResources,
		},
		namespace:      "default",
		middleItems:    []model.Item{{Name: "my-configmap"}},
		yamlContent:    "apiVersion: v1\nkind: ConfigMap",
		yamlCollapsed:  make(map[string]bool),
		yamlVisualMode: true,
		yamlVisualType: 'B',
		tabs:           []TabState{{}},
	}
	output := m.View()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "VISUAL BLOCK")
}

func TestViewYAMLModeSearchActive(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		mode:   modeYAML,
		nav: model.NavigationState{
			Level: model.LevelResources,
		},
		namespace:      "default",
		middleItems:    []model.Item{{Name: "test"}},
		yamlContent:    "apiVersion: v1\nkind: ConfigMap",
		yamlCollapsed:  make(map[string]bool),
		yamlSearchMode: true,
		yamlSearchText: TextInput{Value: "api"},
		tabs:           []TabState{{}},
	}
	output := m.View()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "/")
}

func TestViewYAMLModeSearchResultsShown(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		mode:   modeYAML,
		nav: model.NavigationState{
			Level: model.LevelResources,
		},
		namespace:      "default",
		middleItems:    []model.Item{{Name: "test"}},
		yamlContent:    "apiVersion: v1\nkind: ConfigMap\napiVersion: apps/v1",
		yamlCollapsed:  make(map[string]bool),
		yamlSearchText: TextInput{Value: "apiVersion"},
		yamlMatchLines: []int{0, 2},
		yamlMatchIdx:   0,
		tabs:           []TabState{{}},
	}
	output := m.View()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "1/2")
}

func TestViewYAMLModeSearchNoMatches(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		mode:   modeYAML,
		nav: model.NavigationState{
			Level: model.LevelResources,
		},
		namespace:      "default",
		middleItems:    []model.Item{{Name: "test"}},
		yamlContent:    "apiVersion: v1\nkind: ConfigMap",
		yamlCollapsed:  make(map[string]bool),
		yamlSearchText: TextInput{Value: "nonexistent"},
		yamlMatchLines: nil,
		tabs:           []TabState{{}},
	}
	output := m.View()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "no matches")
	// n/N has no matches to navigate, so the nav suffix must not appear.
	assert.NotContains(t, stripped, "n/N")
	assert.NotContains(t, stripped, "next/prev")
}

func TestViewYAMLModeDefaultHintsListGotoAndOmitSearchNav(t *testing.T) {
	// No search has been committed and we are not in search-input mode, so
	// the default hint bar is rendered. It must advertise the goto chord
	// (123G) but must not advertise n/N — there are no matches to step
	// through yet.
	m := Model{
		width:         200, // wide enough so hints don't truncate
		height:        30,
		mode:          modeYAML,
		nav:           model.NavigationState{Level: model.LevelResources},
		namespace:     "default",
		middleItems:   []model.Item{{Name: "test"}},
		yamlContent:   "apiVersion: v1\nkind: ConfigMap",
		yamlCollapsed: make(map[string]bool),
		tabs:          []TabState{{}},
	}
	output := m.View()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "123G")
	assert.Contains(t, stripped, "goto")
	assert.NotContains(t, stripped, "n/N")
	assert.NotContains(t, stripped, "next/prev")
}

func TestViewYAMLModeSearchResultsShowNavHint(t *testing.T) {
	// A committed search with at least one match should expose the n/N
	// next/prev navigation hint alongside the [hit/total] indicator.
	m := Model{
		width:          200,
		height:         30,
		mode:           modeYAML,
		nav:            model.NavigationState{Level: model.LevelResources},
		namespace:      "default",
		middleItems:    []model.Item{{Name: "test"}},
		yamlContent:    "apiVersion: v1\nkind: ConfigMap\napiVersion: apps/v1",
		yamlCollapsed:  make(map[string]bool),
		yamlSearchText: TextInput{Value: "apiVersion"},
		yamlMatchLines: []int{0, 2},
		yamlMatchIdx:   0,
		tabs:           []TabState{{}},
	}
	output := m.View()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "1/2")
	assert.Contains(t, stripped, "n/N")
	assert.Contains(t, stripped, "next/prev")
}

func TestViewYAMLModeSmallHeight(t *testing.T) {
	m := Model{
		width:         80,
		height:        5,
		mode:          modeYAML,
		nav:           model.NavigationState{Level: model.LevelResources},
		namespace:     "default",
		middleItems:   []model.Item{{Name: "test"}},
		yamlContent:   "a: 1\nb: 2\nc: 3\nd: 4\ne: 5\nf: 6",
		yamlCollapsed: make(map[string]bool),
		tabs:          []TabState{{}},
	}
	output := m.View()
	assert.NotEmpty(t, output)
}

// --- View: explorer mode variants ---

func TestViewExplorerFullscreenMiddle(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test",
			ResourceType: model.ResourceTypeEntry{
				DisplayName: "Pods",
				Kind:        "Pod",
			},
		},
		middleItems:        []model.Item{{Name: "nginx"}},
		width:              120,
		height:             40,
		mode:               modeExplorer,
		namespace:          "default",
		fullscreenMiddle:   true,
		tabs:               []TabState{{}},
		selectedItems:      make(map[string]bool),
		cursorMemory:       make(map[string]int),
		itemCache:          make(map[string][]model.Item),
		yamlCollapsed:      make(map[string]bool),
		selectedNamespaces: make(map[string]bool),
	}
	view := m.View()
	assert.NotEmpty(t, view)
}

func TestViewExplorerWithError(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test",
			ResourceType: model.ResourceTypeEntry{
				DisplayName: "Pods",
				Kind:        "Pod",
			},
		},
		middleItems:        nil,
		err:                assert.AnError,
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
	assert.NotEmpty(t, view)
}

func TestViewExplorerWithOverlay(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test",
			ResourceType: model.ResourceTypeEntry{
				DisplayName: "Pods",
				Kind:        "Pod",
			},
		},
		middleItems:        []model.Item{{Name: "nginx"}},
		width:              120,
		height:             40,
		mode:               modeExplorer,
		namespace:          "default",
		overlay:            overlayQuitConfirm,
		tabs:               []TabState{{}},
		selectedItems:      make(map[string]bool),
		cursorMemory:       make(map[string]int),
		itemCache:          make(map[string][]model.Item),
		yamlCollapsed:      make(map[string]bool),
		selectedNamespaces: make(map[string]bool),
	}
	view := m.View()
	assert.NotEmpty(t, view)
}

func TestViewExplorerWithErrorLogOverlay(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test",
			ResourceType: model.ResourceTypeEntry{
				DisplayName: "Pods",
				Kind:        "Pod",
			},
		},
		middleItems:        []model.Item{{Name: "nginx"}},
		width:              120,
		height:             40,
		mode:               modeExplorer,
		namespace:          "default",
		overlayErrorLog:    true,
		tabs:               []TabState{{}},
		selectedItems:      make(map[string]bool),
		cursorMemory:       make(map[string]int),
		itemCache:          make(map[string][]model.Item),
		yamlCollapsed:      make(map[string]bool),
		selectedNamespaces: make(map[string]bool),
	}
	view := m.View()
	assert.NotEmpty(t, view)
}

// --- renderTitleBar ---

func TestRenderTitleBarVariants(t *testing.T) {
	t.Run("single namespace", func(t *testing.T) {
		m := Model{
			width:     120,
			namespace: "production",
		}
		bar := m.renderTitleBar()
		stripped := stripANSI(bar)
		assert.Contains(t, stripped, "ns: production")
	})

	t.Run("all namespaces", func(t *testing.T) {
		m := Model{
			width:         120,
			namespace:     "default",
			allNamespaces: true,
		}
		bar := m.renderTitleBar()
		stripped := stripANSI(bar)
		assert.Contains(t, stripped, "ns: all")
	})

	t.Run("multiple selected namespaces", func(t *testing.T) {
		m := Model{
			width:              120,
			namespace:          "default",
			selectedNamespaces: map[string]bool{"ns-1": true, "ns-2": true},
		}
		bar := m.renderTitleBar()
		stripped := stripANSI(bar)
		assert.Contains(t, stripped, "ns-1")
		assert.Contains(t, stripped, "ns-2")
	})

	t.Run("more than 3 selected namespaces", func(t *testing.T) {
		m := Model{
			width:     120,
			namespace: "default",
			selectedNamespaces: map[string]bool{
				"a-ns": true, "b-ns": true, "c-ns": true, "d-ns": true,
			},
		}
		bar := m.renderTitleBar()
		stripped := stripANSI(bar)
		assert.Contains(t, stripped, "+1 more")
	})

	t.Run("single selected namespace", func(t *testing.T) {
		m := Model{
			width:              120,
			namespace:          "default",
			selectedNamespaces: map[string]bool{"chosen-ns": true},
		}
		bar := m.renderTitleBar()
		stripped := stripANSI(bar)
		assert.Contains(t, stripped, "ns: chosen-ns")
	})

	t.Run("with watch mode", func(t *testing.T) {
		m := Model{
			width:     120,
			namespace: "default",
			watchMode: true,
		}
		bar := m.renderTitleBar()
		assert.NotEmpty(t, bar)
	})

	t.Run("with version", func(t *testing.T) {
		m := Model{
			width:     120,
			namespace: "default",
			version:   "v1.2.3",
		}
		bar := m.renderTitleBar()
		stripped := stripANSI(bar)
		assert.Contains(t, stripped, "v1.2.3")
	})

	t.Run("small width", func(t *testing.T) {
		m := Model{
			width:     20,
			namespace: "default",
			nav:       model.NavigationState{Context: "very-long-context-name-here"},
		}
		bar := m.renderTitleBar()
		assert.NotEmpty(t, bar)
	})
}

// --- viewExplorer: different column views ---

func TestViewExplorerColumnView(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelClusters,
			Context: "",
		},
		middleItems: []model.Item{
			{Name: "cluster-1"},
			{Name: "cluster-2"},
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
	assert.Contains(t, stripped, "cluster-1")
}

func TestViewExplorerTableView(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test",
			ResourceType: model.ResourceTypeEntry{
				DisplayName: "Pods",
				Kind:        "Pod",
			},
		},
		middleItems: []model.Item{
			{Name: "nginx-pod", Status: "Running", Ready: "1/1", Age: "3d"},
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
	assert.NotEmpty(t, view)
}

// --- viewExplorer: fullscreen dashboard ---

func TestViewExplorerFullscreenDashboard(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResourceTypes,
			Context: "test",
		},
		middleItems: []model.Item{
			{Name: "Cluster Dashboard", Extra: "__overview__"},
		},
		width:               120,
		height:              40,
		mode:                modeExplorer,
		namespace:           "default",
		fullscreenDashboard: true,
		dashboardPreview:    "Node Count: 3\nPod Count: 42",
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		yamlCollapsed:       make(map[string]bool),
		selectedNamespaces:  make(map[string]bool),
	}
	view := m.View()
	assert.NotEmpty(t, view)
}

func TestViewExplorerFullscreenDashboardMonitoring(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResourceTypes,
			Context: "test",
		},
		middleItems: []model.Item{
			{Name: "Monitoring", Extra: "__monitoring__"},
		},
		width:               120,
		height:              40,
		mode:                modeExplorer,
		namespace:           "default",
		fullscreenDashboard: true,
		monitoringPreview:   "CPU: 45%\nMEM: 60%",
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		yamlCollapsed:       make(map[string]bool),
		selectedNamespaces:  make(map[string]bool),
	}
	view := m.View()
	assert.NotEmpty(t, view)
}

func TestViewExplorerFullscreenDashboardWithScroll(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = strings.Repeat("x", 10)
	}
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResourceTypes,
			Context: "test",
		},
		middleItems: []model.Item{
			{Name: "Cluster Dashboard", Extra: "__overview__"},
		},
		width:               120,
		height:              40,
		mode:                modeExplorer,
		namespace:           "default",
		fullscreenDashboard: true,
		dashboardPreview:    strings.Join(lines, "\n"),
		previewScroll:       10,
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		yamlCollapsed:       make(map[string]bool),
		selectedNamespaces:  make(map[string]bool),
	}
	view := m.View()
	assert.NotEmpty(t, view)
}

// --- viewExplorer: with tabs ---

func TestViewExplorerWithMultipleTabs(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test-cluster",
			ResourceType: model.ResourceTypeEntry{
				DisplayName: "Pods",
				Kind:        "Pod",
			},
		},
		middleItems: []model.Item{{Name: "nginx"}},
		width:       120,
		height:      40,
		mode:        modeExplorer,
		namespace:   "default",
		tabs: []TabState{
			{nav: model.NavigationState{Context: "test-cluster"}},
			{nav: model.NavigationState{Context: "prod-cluster"}},
		},
		activeTab:          0,
		selectedItems:      make(map[string]bool),
		cursorMemory:       make(map[string]int),
		itemCache:          make(map[string][]model.Item),
		yamlCollapsed:      make(map[string]bool),
		selectedNamespaces: make(map[string]bool),
	}
	view := m.View()
	assert.NotEmpty(t, view)
}

// --- viewExplorer: resource types with collapsed groups ---

func TestViewExplorerCollapsedGroups(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResourceTypes,
			Context: "test",
		},
		middleItems: []model.Item{
			{Name: "Pods", Category: "Workloads"},
			{Name: "Deployments", Category: "Workloads"},
			{Name: "Services", Category: "Networking"},
		},
		width:              120,
		height:             40,
		mode:               modeExplorer,
		namespace:          "default",
		expandedGroup:      "Workloads",
		allGroupsExpanded:  false,
		tabs:               []TabState{{}},
		selectedItems:      make(map[string]bool),
		cursorMemory:       make(map[string]int),
		itemCache:          make(map[string][]model.Item),
		yamlCollapsed:      make(map[string]bool),
		selectedNamespaces: make(map[string]bool),
	}
	view := m.View()
	assert.NotEmpty(t, view)
}
