package app

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- View: fullscreen modes ---

func TestViewExecMode(t *testing.T) {
	m := Model{
		width:     80,
		height:    30,
		mode:      modeExec,
		execTitle: "Exec: my-pod",
		tabs:      []TabState{{}},
	}
	output := m.View().Content
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "Terminal not initialized")
}

// TestViewExecTerminalNoSideBorders verifies issue #81 fix: the PTY pane is
// framed with horizontal rules only, so a host-terminal shift+drag does not
// pick up vertical box-drawing chars (`│`) around each output line.
func TestViewExecTerminalNoSideBorders(t *testing.T) {
	m := Model{
		width:     40,
		height:    10,
		mode:      modeExec,
		execTitle: "Exec: ns/pod",
		tabs:      []TabState{{}},
	}
	stripped := stripANSI(m.viewExecTerminal())

	assert.NotContains(t, stripped, "│", "PTY pane must not render side bars")
	// Top + bottom rules use NormalBorder, which uses ─ for the horizontal rule.
	assert.Contains(t, stripped, "─", "PTY pane must keep horizontal rules")
}

// TestViewExecTerminalScrolledShowsScrollback verifies the scrollback
// path: when m.execScrollOffset > 0, the rendered pane must contain
// captured lines, not blank padding.
func TestViewExecTerminalScrolledShowsScrollback(t *testing.T) {
	sb := newScrollback(200)
	for i := range 100 {
		_, _ = sb.Write(fmt.Appendf(nil, "row-%d\r\n", i))
	}
	m := Model{
		width:            40,
		height:           24,
		mode:             modeExec,
		execTitle:        "Exec: ns/pod",
		execScrollback:   sb,
		execScrollOffset: 1,
		tabs:             []TabState{{}},
	}
	out := m.viewExecTerminal()
	stripped := stripANSI(out)

	// Offset 1 means "one line back from the tail" — the latest committed
	// line ("row-99") should be hidden, but the lines below it must show.
	assert.Contains(t, stripped, "row-98", "scrolled pane must include captured lines, not just blank padding")
	assert.NotContains(t, stripped, "row-99", "offset 1 should hide the most recent committed line")
}

// TestViewExecTerminalScrolledSmallScrollback covers the user-reported
// regression: "scroll up only one line and the whole pane goes blank".
// With only a handful of captured lines, a single wheel tick on a fresh
// shell must not erase the visible content.
func TestViewExecTerminalScrolledSmallScrollback(t *testing.T) {
	sb := newScrollback(200)
	// Only 30 lines captured — typical for a shell that just printed a
	// kubectl get output and a couple of prompts.
	for i := range 30 {
		_, _ = sb.Write(fmt.Appendf(nil, "row-%d\r\n", i))
	}
	m := Model{
		width:            40,
		height:           24,
		mode:             modeExec,
		execTitle:        "Exec",
		execScrollback:   sb,
		execScrollOffset: 1,
		tabs:             []TabState{{}},
	}
	out := m.viewExecTerminal()
	stripped := stripANSI(out)

	// At least one captured row should be visible — the pane must not
	// render blank when scrolled by one line over a 30-line scrollback.
	hasAny := false
	for i := range 30 {
		if strings.Contains(stripped, fmt.Sprintf("row-%d", i)) {
			hasAny = true
			break
		}
	}
	assert.True(t, hasAny, "scrolled pane with 30 captured lines must show some content; got:\n%s", stripped)
}

func TestViewExecModeWithTabs(t *testing.T) {
	m := Model{
		width:     80,
		height:    30,
		mode:      modeExec,
		execTitle: "Exec: my-pod",
		tabs:      []TabState{{}, {}},
		nav:       model.NavigationState{Context: "ctx-1"},
	}
	output := m.View().Content
	assert.NotEmpty(t, output)
}

func TestViewExplainModeSearchActive(t *testing.T) {
	m := Model{
		width:               120,
		height:              30,
		mode:                modeExplain,
		explainTitle:        "Explain: Pod",
		explainDesc:         "A Pod is the smallest deployable unit.",
		explainSearchActive: true,
		explainSearchInput:  TextInput{Value: "spec"},
		tabs:                []TabState{{}},
	}
	output := m.View().Content
	assert.NotEmpty(t, output)
}

// --- View: overlay on top of fullscreen modes ---

func TestViewYAMLModeWithOverlay(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		mode:   modeYAML,
		nav: model.NavigationState{
			Level: model.LevelResources,
		},
		yamlView: yamlViewState{
			content:   "apiVersion: v1\nkind: Pod",
			collapsed: make(map[string]bool),
		},
		tabs:    []TabState{{}},
		overlay: overlayQuitConfirm,
	}
	output := m.View().Content
	assert.NotEmpty(t, output)
}

// --- View: explorer with overlays ---

func TestViewExplorerErrorLogOverlayVisible(t *testing.T) {
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
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
		overlayErrorLog:    true,
	}
	output := m.View().Content
	assert.NotEmpty(t, output)
}

func TestViewExplorerWithHelpOverlay(t *testing.T) {
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
		mode:          modeHelp,
		namespace:     "default",
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
		helpFilter:         TextInput{},
		helpSearchInput:    textinput.New(),
	}
	output := m.View().Content
	assert.NotEmpty(t, output)
}

// --- viewExplorer: fullscreen dashboard mode ---

func TestViewExplorerFullscreenDashboardContent(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResourceTypes,
			Context: "test",
		},
		middleItems: []model.Item{
			{Name: "Cluster Dashboard", Extra: "__overview__"},
		},
		dashboardPreview:    "Node Count: 3",
		fullscreenDashboard: true,
		width:               120,
		height:              40,
		mode:                modeExplorer,
		namespace:           "default",
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
	}
	output := m.viewExplorer()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "Node Count: 3")
}

func TestViewExplorerFullscreenMonitoring(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResourceTypes,
			Context: "test",
		},
		middleItems: []model.Item{
			{Name: "Monitoring", Extra: "__monitoring__"},
		},
		monitoringPreview:   "CPU: 45%",
		fullscreenDashboard: true,
		width:               120,
		height:              40,
		mode:                modeExplorer,
		namespace:           "default",
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
	}
	output := m.viewExplorer()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "CPU: 45%")
}

func TestViewExplorerFullscreenMiddleMode(t *testing.T) {
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
		fullscreenMiddle: true,
		width:            120,
		height:           40,
		mode:             modeExplorer,
		namespace:        "default",
		tabs:             []TabState{{}},
		selectedItems:    make(map[string]bool),
		cursorMemory:     make(map[string]int),
		itemCache:        make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
	}
	output := m.viewExplorer()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "nginx")
}

// --- renderTitleBar ---

func TestRenderTitleBarWithWatch(t *testing.T) {
	m := Model{
		width:     120,
		height:    40,
		watchMode: true,
		namespace: "default",
		nav: model.NavigationState{
			Context: "prod",
		},
		selectedNamespaces: make(map[string]bool),
	}
	bar := m.renderTitleBar()
	assert.NotEmpty(t, bar)
}

func TestRenderTitleBarAllNamespaces(t *testing.T) {
	m := Model{
		width:              120,
		height:             40,
		allNamespaces:      true,
		namespace:          "default",
		nav:                model.NavigationState{Context: "prod"},
		selectedNamespaces: make(map[string]bool),
	}
	bar := m.renderTitleBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "all")
}

func TestRenderTitleBarMultipleNamespaces(t *testing.T) {
	m := Model{
		width:     120,
		height:    40,
		namespace: "default",
		nav:       model.NavigationState{Context: "prod"},
		selectedNamespaces: map[string]bool{
			"default":     true,
			"kube-system": true,
			"monitoring":  true,
			"logging":     true,
			"tracing":     true,
		},
	}
	bar := m.renderTitleBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "+")
}

func TestRenderTitleBarSingleSelectedNamespace(t *testing.T) {
	m := Model{
		width:     120,
		height:    40,
		namespace: "default",
		nav:       model.NavigationState{Context: "prod"},
		selectedNamespaces: map[string]bool{
			"kube-system": true,
		},
	}
	bar := m.renderTitleBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "kube-system")
}

func TestRenderTitleBarWithVersion(t *testing.T) {
	m := Model{
		width:              120,
		height:             40,
		namespace:          "default",
		version:            "v1.2.3",
		nav:                model.NavigationState{Context: "prod"},
		selectedNamespaces: make(map[string]bool),
	}
	bar := m.renderTitleBar()
	stripped := stripANSI(bar)
	assert.Contains(t, stripped, "v1.2.3")
}

func TestRenderTitleBarLongBreadcrumb(t *testing.T) {
	m := Model{
		width:     40,
		height:    40,
		namespace: "default",
		nav: model.NavigationState{
			Context:      "very-long-cluster-name-that-overflows-width",
			ResourceType: model.ResourceTypeEntry{DisplayName: "Deployments"},
			ResourceName: "my-very-long-deployment-name",
		},
		selectedNamespaces: make(map[string]bool),
	}
	bar := m.renderTitleBar()
	assert.NotEmpty(t, bar)
}

// --- viewExplorer: with error ---

func TestViewExplorerWithErrorMsg(t *testing.T) {
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
	output := m.viewExplorer()
	assert.NotEmpty(t, output)
}

// --- clampPreviewScroll ---

func TestClampPreviewScrollBasic(t *testing.T) {
	m := Model{
		width:         120,
		height:        40,
		tabs:          []TabState{{}},
		previewScroll: 0,
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
		},
	}
	m.clampPreviewScroll()
	assert.GreaterOrEqual(t, m.previewScroll, 0)
}

func TestClampPreviewScrollHighScroll(t *testing.T) {
	m := Model{
		width:         120,
		height:        40,
		tabs:          []TabState{{}},
		previewScroll: 10000,
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
		},
	}
	m.clampPreviewScroll()
	// previewScroll should be clamped down.
	assert.Less(t, m.previewScroll, 10000)
}

func TestClampPreviewScrollWithMetrics(t *testing.T) {
	m := Model{
		width:          120,
		height:         40,
		tabs:           []TabState{{}},
		previewScroll:  0,
		metricsContent: "CPU: 100m\nMEM: 256Mi",
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
		},
	}
	m.clampPreviewScroll()
	assert.GreaterOrEqual(t, m.previewScroll, 0)
}

func TestClampPreviewScrollSplitMode(t *testing.T) {
	m := Model{
		width:         120,
		height:        40,
		tabs:          []TabState{{}},
		previewScroll: 100,
		rightItems:    []model.Item{{Name: "pod-1"}},
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
		},
	}
	m.clampPreviewScroll()
	assert.Less(t, m.previewScroll, 100)
}

// --- renderRightColumn: with preview scroll ---

func TestRenderRightColumnWithScroll(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "content-line"
	}
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
		},
		middleItems: []model.Item{{
			Name:    "cm-1",
			Columns: []model.KeyValue{{Key: "Data", Value: "50"}},
		}},
		previewYAML:   strings.Join(lines, "\n"),
		previewScroll: 5,
	}
	result := m.renderRightColumn(80, 30)
	assert.NotEmpty(t, result)
}

func TestRenderRightColumnSplitPreview(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
		},
		middleItems: []model.Item{{
			Name:    "deploy-1",
			Columns: []model.KeyValue{{Key: "Ready", Value: "3/3"}},
		}},
		rightItems: []model.Item{
			{Name: "pod-1", Status: "Running"},
			{Name: "pod-2", Status: "Running"},
		},
		previewYAML: "apiVersion: apps/v1\nkind: Deployment",
	}
	result := m.renderRightColumn(80, 30)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "deploy-1", "split preview summary should include resource name as NAME row")
}

func TestRenderRightColumnSplitPreviewWithEvents(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Deployment"},
		},
		middleItems: []model.Item{{
			Name:    "deploy-1",
			Columns: []model.KeyValue{{Key: "Ready", Value: "3/3"}},
		}},
		rightItems: []model.Item{
			{Name: "pod-1"},
		},
		previewEventsContent: "Normal  Created  container started",
	}
	result := m.renderRightColumn(80, 30)
	assert.NotEmpty(t, result)
}

// The exec pane must fill the terminal exactly: the hint bar is the last line,
// with no blank row under it. lipgloss v2 counts the border rows inside
// Height(), so the pre-v2 arithmetic left the pane two rows short.
func TestViewExecFillsTerminalHeight(t *testing.T) {
	for _, tc := range []struct{ w, h, tabs int }{
		{80, 30, 1}, {120, 40, 1}, {100, 24, 1}, {80, 30, 2},
	} {
		t.Run(fmt.Sprintf("%dx%d_%dtabs", tc.w, tc.h, tc.tabs), func(t *testing.T) {
			m := Model{
				width: tc.w, height: tc.h, mode: modeExec,
				execTitle: "Exec: my-pod", tabs: make([]TabState, tc.tabs),
			}
			out := m.View().Content
			assert.Equal(t, tc.h, lipgloss.Height(out),
				"exec view must fill the terminal exactly")

			lines := strings.Split(out, "\n")
			assert.NotEmpty(t, strings.TrimSpace(stripANSI(lines[len(lines)-1])),
				"the last line must be the hint bar, not a blank row")
		})
	}
}

// Every fullscreen mode must fill the terminal exactly. This guards the whole
// family against the lipgloss v2 Width/Height semantics change, where the
// border is counted inside the requested size rather than added around it.
func TestFullscreenModesFillTerminalHeight(t *testing.T) {
	modes := map[string]viewMode{
		"exec": modeExec, "yaml": modeYAML, "logs": modeLogs,
		"describe": modeDescribe, "diff": modeDiff, "explain": modeExplain,
		"eventViewer": modeEventViewer, "objectExplorer": modeObjectExplorer,
		"logTop": modeLogTop,
	}
	for name, mode := range modes {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct{ w, h int }{{100, 30}, {80, 24}} {
				m := Model{
					width: tc.w, height: tc.h, mode: mode,
					execTitle: "x", tabs: []TabState{{}},
				}
				assert.Equal(t, tc.h, lipgloss.Height(m.View().Content),
					"%s at %dx%d must fill the terminal height", name, tc.w, tc.h)
			}
		})
	}
}
