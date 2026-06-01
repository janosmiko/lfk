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
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
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

// TestViewExplorerDashboardTwoColWrappedEventsFillBackground guards the issue
// #293 follow-up: the events column wraps long messages, and lipgloss emits the
// parameterless reset (ESC[m) at each wrap boundary. FillLinesBg must re-apply
// the theme background after that reset too, or the per-column padding that
// follows a wrapped sub-line renders with the terminal default (a black "tear")
// — the artifact reported in the cluster events preview.
func TestViewExplorerDashboardTwoColWrappedEventsFillBackground(t *testing.T) {
	origProfile := lipgloss.DefaultRenderer().ColorProfile()
	origTheme := ui.ActiveTheme
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	ui.ApplyTheme(ui.DefaultTheme())
	t.Cleanup(func() {
		lipgloss.DefaultRenderer().SetColorProfile(origProfile)
		ui.ApplyTheme(origTheme)
	})

	// A long, styled message that must wrap across several sub-lines within the
	// right column — each wrap boundary closes with the parameterless reset.
	longMsg := ui.DimStyle.Render("Readiness probe failed: Get http://172.18.12.93:8080/healthz: " +
		"dial tcp 172.18.12.93:8080: connect: connection refused after several retries")
	m := Model{
		nav:                 model.NavigationState{Level: model.LevelResourceTypes, Context: "test"},
		middleItems:         []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}},
		width:               120,
		height:              40,
		mode:                modeExplorer,
		namespace:           "default",
		fullscreenDashboard: true,
		dashboardPreview:    "NODES: 3 Ready\nPODS: 42 Running",
		dashboardEventsPreview: ui.DimStyle.Bold(true).Render("  RECENT EVENTS") + "\n\n" +
			"  " + ui.StatusProgressing.Render("⚠") + " 2m   " +
			ui.StatusFailed.Render("Unhealthy:") + " " + ui.NormalStyle.Render("Pod/argocd-server") + "\n" +
			"       " + longMsg,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
	}

	out := m.View()

	assert.Contains(t, out, "\x1b[", "forced color profile must emit ANSI sequences")
	// Guard against a vacuous pass: confirm a wrap boundary actually emitted the
	// parameterless reset. The fix rewrites it to "reset + bgSeq" (keeping the
	// reset present), so its absence would mean no wrap occurred — making the
	// NotContains check below meaningless.
	assert.Contains(t, out, "\x1b[m",
		"a wrapped event sub-line must emit the parameterless reset for this test to be meaningful")
	// A parameterless reset immediately followed by a space is un-backgrounded
	// padding after a wrap boundary — the black tear. FillLinesBg must rewrite
	// it to "reset + bgSeq".
	assert.NotContains(t, out, "\x1b[m ",
		"wrapped event sub-lines must re-apply the theme background after the parameterless reset")
}

// TestClampPreviewScrollDashboard guards against scrolling the fullscreen
// dashboard past its last line. clampPreviewScroll previously only knew the
// right-column preview's content, so in dashboard mode it let previewScroll
// run into blank space below the rendered nodes/overview content.
func TestClampPreviewScrollDashboard(t *testing.T) {
	newModel := func(lines, height int) Model {
		body := make([]string, lines)
		for i := range body {
			// Wide lines: they fit on one row at the full dashboard width but
			// wrap when the old clamp measured them at the narrow right-column
			// width, which is exactly what inflated maxScroll and allowed the
			// over-scroll.
			body[i] = strings.Repeat("x", 80)
		}
		return Model{
			nav:                 model.NavigationState{Level: model.LevelResourceTypes, Context: "test"},
			middleItems:         []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}},
			width:               120,
			height:              height,
			fullscreenDashboard: true,
			dashboardPreview:    strings.Join(body, "\n"),
			// Right-pane preview state that the old clamp folded into its
			// total — irrelevant to the dashboard, and the source of the
			// over-scroll. The dashboard clamp must ignore it.
			previewEventsContent: strings.Repeat("event\n", 50),
			tabs:                 []TabState{{}},
			selectedItems:        make(map[string]bool),
			cursorMemory:         make(map[string]int),
		}
	}

	t.Run("content shorter than viewport cannot scroll", func(t *testing.T) {
		m := newModel(5, 40) // 5 lines, viewport ~36
		m.previewScroll = 999
		m.clampPreviewScroll()
		assert.Equal(t, 0, m.previewScroll, "no scroll when content fits the viewport")
	})

	t.Run("content taller than viewport clamps to last page", func(t *testing.T) {
		m := newModel(100, 20) // viewport = height-4 = 16
		m.previewScroll = 999
		m.clampPreviewScroll()
		assert.Equal(t, 100-16, m.previewScroll,
			"max scroll must leave the last viewport of content on screen")
	})
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
		namespace:   "default",
		middleItems: []model.Item{{Name: "my-configmap"}},
		yamlView: yamlViewState{
			content:    "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: my-configmap",
			collapsed:  make(map[string]bool),
			visualMode: true,
			visualType: 'V',
		},
		tabs: []TabState{{}},
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
		namespace:   "default",
		middleItems: []model.Item{{Name: "my-configmap"}},
		yamlView: yamlViewState{
			content:    "apiVersion: v1\nkind: ConfigMap",
			collapsed:  make(map[string]bool),
			visualMode: true,
			visualType: 'v',
		},
		tabs: []TabState{{}},
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
		namespace:   "default",
		middleItems: []model.Item{{Name: "my-configmap"}},
		yamlView: yamlViewState{
			content:    "apiVersion: v1\nkind: ConfigMap",
			collapsed:  make(map[string]bool),
			visualMode: true,
			visualType: 'B',
		},
		tabs: []TabState{{}},
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
		namespace:   "default",
		middleItems: []model.Item{{Name: "test"}},
		yamlView: yamlViewState{
			content:    "apiVersion: v1\nkind: ConfigMap",
			collapsed:  make(map[string]bool),
			searchMode: true,
			searchText: TextInput{Value: "api"},
		},
		tabs: []TabState{{}},
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
		namespace:   "default",
		middleItems: []model.Item{{Name: "test"}},
		yamlView: yamlViewState{
			content:    "apiVersion: v1\nkind: ConfigMap\napiVersion: apps/v1",
			collapsed:  make(map[string]bool),
			searchText: TextInput{Value: "apiVersion"},
			matchLines: []int{0, 2},
			matchIdx:   0,
		},
		tabs: []TabState{{}},
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
		namespace:   "default",
		middleItems: []model.Item{{Name: "test"}},
		yamlView: yamlViewState{
			content:    "apiVersion: v1\nkind: ConfigMap",
			collapsed:  make(map[string]bool),
			searchText: TextInput{Value: "nonexistent"},
			matchLines: nil,
		},
		tabs: []TabState{{}},
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
		width:       200, // wide enough so hints don't truncate
		height:      30,
		mode:        modeYAML,
		nav:         model.NavigationState{Level: model.LevelResources},
		namespace:   "default",
		middleItems: []model.Item{{Name: "test"}},
		yamlView: yamlViewState{
			content:   "apiVersion: v1\nkind: ConfigMap",
			collapsed: make(map[string]bool),
		},
		tabs: []TabState{{}},
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
		width:       200,
		height:      30,
		mode:        modeYAML,
		nav:         model.NavigationState{Level: model.LevelResources},
		namespace:   "default",
		middleItems: []model.Item{{Name: "test"}},
		yamlView: yamlViewState{
			content:    "apiVersion: v1\nkind: ConfigMap\napiVersion: apps/v1",
			collapsed:  make(map[string]bool),
			searchText: TextInput{Value: "apiVersion"},
			matchLines: []int{0, 2},
			matchIdx:   0,
		},
		tabs: []TabState{{}},
	}
	output := m.View()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "1/2")
	assert.Contains(t, stripped, "n/N")
	assert.Contains(t, stripped, "next/prev")
}

func TestViewYAMLModeSmallHeight(t *testing.T) {
	m := Model{
		width:       80,
		height:      5,
		mode:        modeYAML,
		nav:         model.NavigationState{Level: model.LevelResources},
		namespace:   "default",
		middleItems: []model.Item{{Name: "test"}},
		yamlView: yamlViewState{
			content:   "a: 1\nb: 2\nc: 3\nd: 4\ne: 5\nf: 6",
			collapsed: make(map[string]bool),
		},
		tabs: []TabState{{}},
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
		middleItems:      []model.Item{{Name: "nginx"}},
		width:            120,
		height:           40,
		mode:             modeExplorer,
		namespace:        "default",
		fullscreenMiddle: true,
		tabs:             []TabState{{}},
		selectedItems:    make(map[string]bool),
		cursorMemory:     make(map[string]int),
		itemCache:        make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
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
		middleItems:   nil,
		err:           assert.AnError,
		width:         120,
		height:        40,
		mode:          modeExplorer,
		namespace:     "default",
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
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
		middleItems:   []model.Item{{Name: "nginx"}},
		width:         120,
		height:        40,
		mode:          modeExplorer,
		namespace:     "default",
		overlay:       overlayQuitConfirm,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
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
		middleItems:     []model.Item{{Name: "nginx"}},
		width:           120,
		height:          40,
		mode:            modeExplorer,
		namespace:       "default",
		overlayErrorLog: true,
		tabs:            []TabState{{}},
		selectedItems:   make(map[string]bool),
		cursorMemory:    make(map[string]int),
		itemCache:       make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
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
		width:         120,
		height:        40,
		mode:          modeExplorer,
		namespace:     "default",
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
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
		width:         120,
		height:        40,
		mode:          modeExplorer,
		namespace:     "default",
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
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
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
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
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
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
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
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
		activeTab:     0,
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
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
		width:             120,
		height:            40,
		mode:              modeExplorer,
		namespace:         "default",
		expandedGroup:     "Workloads",
		allGroupsExpanded: false,
		tabs:              []TabState{{}},
		selectedItems:     make(map[string]bool),
		cursorMemory:      make(map[string]int),
		itemCache:         make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
	}
	view := m.View()
	assert.NotEmpty(t, view)
}
