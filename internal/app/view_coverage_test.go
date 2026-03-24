package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
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
	output := m.View()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "Terminal not initialized")
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
	output := m.View()
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
	output := m.View()
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
		yamlContent:   "apiVersion: v1\nkind: Pod",
		yamlCollapsed: make(map[string]bool),
		tabs:          []TabState{{}},
		overlay:       overlayQuitConfirm,
	}
	output := m.View()
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
		middleItems:        []model.Item{{Name: "nginx"}},
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
		overlayErrorLog:    true,
	}
	output := m.View()
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
		middleItems:        []model.Item{{Name: "nginx"}},
		width:              120,
		height:             40,
		mode:               modeHelp,
		namespace:          "default",
		tabs:               []TabState{{}},
		selectedItems:      make(map[string]bool),
		cursorMemory:       make(map[string]int),
		itemCache:          make(map[string][]model.Item),
		yamlCollapsed:      make(map[string]bool),
		selectedNamespaces: make(map[string]bool),
		helpFilter:         TextInput{},
		helpSearchInput:    textinput.New(),
	}
	output := m.View()
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
		yamlCollapsed:       make(map[string]bool),
		selectedNamespaces:  make(map[string]bool),
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
		yamlCollapsed:       make(map[string]bool),
		selectedNamespaces:  make(map[string]bool),
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
		middleItems:        []model.Item{{Name: "nginx"}},
		fullscreenMiddle:   true,
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
		yamlCollapsed: make(map[string]bool),
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
		yamlCollapsed: make(map[string]bool),
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
		yamlCollapsed:  make(map[string]bool),
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
		yamlCollapsed: make(map[string]bool),
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
