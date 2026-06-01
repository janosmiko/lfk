package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// threeTabModel returns a Model with three tabs, each having distinct nav state,
// namespace, and middleItems. The activeTab and model-level fields reflect the
// tab at the given index.
func threeTabModel(activeTab int) Model {
	tabs := []TabState{
		{
			nav: model.NavigationState{
				Context:      "prod",
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Pod", DisplayName: "Pods"},
			},
			namespace:   "prod-ns",
			middleItems: []model.Item{{Name: "pod-1"}},
			leftItems:   []model.Item{{Name: "prod-left"}},
		},
		{
			nav: model.NavigationState{
				Context:      "staging",
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Deployment", DisplayName: "Deployments"},
			},
			namespace:   "staging-ns",
			middleItems: []model.Item{{Name: "deploy-1"}},
			leftItems:   []model.Item{{Name: "staging-left"}},
		},
		{
			nav: model.NavigationState{
				Context:      "dev",
				Level:        model.LevelResources,
				ResourceType: model.ResourceTypeEntry{Kind: "Service", DisplayName: "Services"},
			},
			namespace:   "dev-ns",
			middleItems: []model.Item{{Name: "svc-1"}},
			leftItems:   []model.Item{{Name: "dev-left"}},
		},
	}

	active := tabs[activeTab]
	return Model{
		tabs:          tabs,
		activeTab:     activeTab,
		nav:           active.nav,
		namespace:     active.namespace,
		middleItems:   append([]model.Item(nil), active.middleItems...),
		leftItems:     append([]model.Item(nil), active.leftItems...),
		width:         120,
		height:        30,
		mode:          modeExplorer,
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
		selectionAnchor:    -1,
	}
}

// --- closeTabOrQuit: close middle tab preserves correct tab data ---

func TestCloseMiddleTabPreservesCorrectData(t *testing.T) {
	m := threeTabModel(1) // active = staging/Deployments

	ret, _ := m.closeTabOrQuit()
	result := ret.(Model)

	assert.Len(t, result.tabs, 2, "should have 2 tabs after close")
	assert.Equal(t, 0, result.activeTab, "activeTab should move to previous tab (index 0)")

	// After closing tab 1 (staging), model should load the previous tab (prod/Pods).
	assert.Equal(t, "prod", result.nav.Context, "model context should be prod (previous tab)")
	assert.Equal(t, "Pod", result.nav.ResourceType.Kind, "resource kind should be Pod")
	assert.Equal(t, "prod-ns", result.namespace, "namespace should be prod-ns")
	assert.Equal(t, "pod-1", result.middleItems[0].Name, "middleItems should be from prod tab")

	// Tab 1 (originally tab 2 = dev) should be unchanged.
	assert.Equal(t, "dev", result.tabs[1].nav.Context)
	assert.Equal(t, "dev-ns", result.tabs[1].namespace)
}

// --- closeTabOrQuit: close first tab preserves correct data ---

func TestCloseFirstTabPreservesCorrectData(t *testing.T) {
	m := threeTabModel(0) // active = prod/Pods

	ret, _ := m.closeTabOrQuit()
	result := ret.(Model)

	assert.Len(t, result.tabs, 2, "should have 2 tabs after close")
	assert.Equal(t, 0, result.activeTab, "activeTab should be 0")

	// The surviving tab at index 0 is what was originally tab 1 (staging/Deployments).
	assert.Equal(t, "staging", result.nav.Context, "model context should be staging (was tab 1)")
	assert.Equal(t, "Deployment", result.nav.ResourceType.Kind, "resource kind should be Deployment")
	assert.Equal(t, "staging-ns", result.namespace, "namespace should be staging-ns")
	assert.Equal(t, "deploy-1", result.middleItems[0].Name, "middleItems should be from staging tab")

	// Tab 1 (originally tab 2 = dev) should be unchanged.
	assert.Equal(t, "dev", result.tabs[1].nav.Context)
}

// --- closeTabOrQuit: close last tab adjusts activeTab ---

func TestCloseLastTabAdjustsActiveTab(t *testing.T) {
	m := threeTabModel(2) // active = dev/Services

	ret, _ := m.closeTabOrQuit()
	result := ret.(Model)

	assert.Len(t, result.tabs, 2, "should have 2 tabs after close")
	assert.Equal(t, 1, result.activeTab, "activeTab should be adjusted to len-1 = 1")

	// The surviving tab at index 1 is what was originally tab 1 (staging/Deployments).
	assert.Equal(t, "staging", result.nav.Context, "model context should be staging (was tab 1)")
	assert.Equal(t, "Deployment", result.nav.ResourceType.Kind, "resource kind should be Deployment")
	assert.Equal(t, "staging-ns", result.namespace, "namespace should be staging-ns")
	assert.Equal(t, "deploy-1", result.middleItems[0].Name, "middleItems should be from staging tab")
}

// --- closeTabOrQuit: single tab triggers quit or confirmation ---

func TestCloseTabSingleTabQuitConfirm(t *testing.T) {
	t.Run("with confirm on exit enabled", func(t *testing.T) {
		orig := ui.ConfigConfirmOnExit
		ui.ConfigConfirmOnExit = true
		t.Cleanup(func() { ui.ConfigConfirmOnExit = orig })

		m := Model{
			tabs: []TabState{{
				nav: model.NavigationState{Context: "prod", Level: model.LevelResources},
			}},
			activeTab:     0,
			nav:           model.NavigationState{Context: "prod", Level: model.LevelResources},
			width:         120,
			height:        30,
			selectedItems: make(map[string]bool),
			cursorMemory:  make(map[string]int),
			itemCache:     make(map[string][]model.Item),
			yamlView: yamlViewState{
				collapsed: make(map[string]bool),
			},
			selectedNamespaces: make(map[string]bool),
		}

		ret, cmd := m.closeTabOrQuit()
		result := ret.(Model)

		assert.Equal(t, overlayQuitConfirm, result.overlay, "should show quit confirmation overlay")
		assert.Nil(t, cmd, "should not return a command")
		assert.Len(t, result.tabs, 1, "tab count should remain 1")
	})

	t.Run("with confirm on exit disabled", func(t *testing.T) {
		orig := ui.ConfigConfirmOnExit
		ui.ConfigConfirmOnExit = false
		t.Cleanup(func() { ui.ConfigConfirmOnExit = orig })

		m := Model{
			tabs: []TabState{{
				nav: model.NavigationState{Context: "prod", Level: model.LevelResources},
			}},
			activeTab:     0,
			nav:           model.NavigationState{Context: "prod", Level: model.LevelResources},
			width:         120,
			height:        30,
			selectedItems: make(map[string]bool),
			cursorMemory:  make(map[string]int),
			itemCache:     make(map[string][]model.Item),
			yamlView: yamlViewState{
				collapsed: make(map[string]bool),
			},
			selectedNamespaces: make(map[string]bool),
		}

		_, cmd := m.closeTabOrQuit()

		// With confirmation disabled and no port forward manager, should return tea.Quit.
		assert.NotNil(t, cmd, "should return a quit command")
	})
}

// --- esc at LevelClusters with multiple tabs closes tab ---

func TestEscAtClusterLevelClosesTab(t *testing.T) {
	m := Model{
		tabs: []TabState{
			{
				nav: model.NavigationState{
					Context: "prod",
					Level:   model.LevelResources,
				},
				namespace:   "prod-ns",
				middleItems: []model.Item{{Name: "pod-1"}},
				leftItems:   []model.Item{{Name: "prod-left"}},
			},
			{
				nav: model.NavigationState{
					Context: "staging",
					Level:   model.LevelClusters,
				},
				namespace:   "staging-ns",
				middleItems: []model.Item{{Name: "cluster-item"}},
				leftItems:   []model.Item{{Name: "staging-left"}},
			},
		},
		activeTab: 1,
		nav: model.NavigationState{
			Context: "staging",
			Level:   model.LevelClusters,
		},
		namespace:     "staging-ns",
		middleItems:   []model.Item{{Name: "cluster-item"}},
		leftItems:     []model.Item{{Name: "staging-left"}},
		width:         120,
		height:        30,
		mode:          modeExplorer,
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
		selectionAnchor:    -1,
	}

	ret, _ := m.handleKey(specialKey(tea.KeyEsc))
	result := ret.(Model)

	assert.Len(t, result.tabs, 1, "esc at cluster level should close the tab")
	assert.Equal(t, 0, result.activeTab, "activeTab should be 0 after closing")

	// The surviving tab is prod/Resources.
	assert.Equal(t, "prod", result.nav.Context, "should load surviving tab (prod)")
	assert.Equal(t, "prod-ns", result.namespace, "namespace should be from surviving tab")
	assert.Equal(t, "pod-1", result.middleItems[0].Name, "middleItems should be from surviving tab")
}

// --- tab data isolation after close ---

func TestTabDataIsolationAfterClose(t *testing.T) {
	m := threeTabModel(1) // active = staging/Deployments

	// Verify model initially reflects the active tab (staging).
	assert.Equal(t, "staging", m.nav.Context)
	assert.Equal(t, "staging-ns", m.namespace)
	assert.Equal(t, "deploy-1", m.middleItems[0].Name)
	assert.Equal(t, "staging-left", m.leftItems[0].Name)

	ret, _ := m.closeTabOrQuit()
	result := ret.(Model)

	// After closing tab 1 (staging), model should NOT retain staging data.
	assert.NotEqual(t, "staging", result.nav.Context, "closed tab context must not persist")
	assert.NotEqual(t, "staging-ns", result.namespace, "closed tab namespace must not persist")

	// Model should reflect the previous tab (prod/Pods at index 0).
	assert.Equal(t, "prod", result.nav.Context, "model context should be prod (previous tab)")
	assert.Equal(t, "prod-ns", result.namespace, "model namespace should be prod-ns")
	assert.Equal(t, "pod-1", result.middleItems[0].Name, "model middleItems should reflect loaded tab")
	assert.Equal(t, "prod-left", result.leftItems[0].Name, "model leftItems should reflect loaded tab")

	// Verify the remaining tab at index 1 still has dev data.
	assert.Equal(t, "dev", result.tabs[1].nav.Context)
	assert.Equal(t, "dev-ns", result.tabs[1].namespace)
	assert.Equal(t, "svc-1", result.tabs[1].middleItems[0].Name)
}

// --- close two tabs sequentially ---

func TestCloseTabsSequentially(t *testing.T) {
	m := threeTabModel(1) // active = staging

	// Close tab 1 (staging). Now tabs = [prod, dev], activeTab = 0 (prod, previous tab).
	ret, _ := m.closeTabOrQuit()
	m = ret.(Model)

	assert.Len(t, m.tabs, 2)
	assert.Equal(t, 0, m.activeTab, "should move to previous tab")
	assert.Equal(t, "prod", m.nav.Context, "should load prod (previous tab)")

	// Close tab 0 (prod). Now tabs = [dev], activeTab = 0 (dev, no previous).
	ret, _ = m.closeTabOrQuit()
	m = ret.(Model)

	assert.Len(t, m.tabs, 1)
	assert.Equal(t, 0, m.activeTab)
	assert.Equal(t, "dev", m.nav.Context)
	assert.Equal(t, "dev-ns", m.namespace)
}

// --- saveCurrentSession writes surviving tab data, not closed tab data ---

func TestCloseTabSavesCorrectSessionData(t *testing.T) {
	m := threeTabModel(1) // active = staging/Deployments

	ret, _ := m.closeTabOrQuit()
	result := ret.(Model)

	// After closeTabOrQuit, saveCurrentSession was called internally.
	// The active tab slot in the tabs array should contain the previous tab's
	// data (prod), not the closed tab's data (staging).
	activeTabState := result.tabs[result.activeTab]
	assert.Equal(t, "prod", activeTabState.nav.Context,
		"saved tab state should reflect prod (previous tab), not staging (closed tab)")
	assert.Equal(t, "prod-ns", activeTabState.namespace,
		"saved tab namespace should be prod-ns")
}

// --- close tab with two tabs: close first ---

func TestCloseTwoTabsCloseFirst(t *testing.T) {
	m := Model{
		tabs: []TabState{
			{
				nav:         model.NavigationState{Context: "alpha", Level: model.LevelResources},
				namespace:   "alpha-ns",
				middleItems: []model.Item{{Name: "alpha-item"}},
			},
			{
				nav:         model.NavigationState{Context: "beta", Level: model.LevelResources},
				namespace:   "beta-ns",
				middleItems: []model.Item{{Name: "beta-item"}},
			},
		},
		activeTab:     0,
		nav:           model.NavigationState{Context: "alpha", Level: model.LevelResources},
		namespace:     "alpha-ns",
		middleItems:   []model.Item{{Name: "alpha-item"}},
		width:         120,
		height:        30,
		mode:          modeExplorer,
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
	}

	ret, _ := m.closeTabOrQuit()
	result := ret.(Model)

	assert.Len(t, result.tabs, 1)
	assert.Equal(t, 0, result.activeTab)
	assert.Equal(t, "beta", result.nav.Context, "model should reflect surviving tab (beta)")
	assert.Equal(t, "beta-ns", result.namespace)
	assert.Equal(t, "beta-item", result.middleItems[0].Name)
}

// --- close tab with two tabs: close second ---

func TestCloseTwoTabsCloseSecond(t *testing.T) {
	m := Model{
		tabs: []TabState{
			{
				nav:         model.NavigationState{Context: "alpha", Level: model.LevelResources},
				namespace:   "alpha-ns",
				middleItems: []model.Item{{Name: "alpha-item"}},
			},
			{
				nav:         model.NavigationState{Context: "beta", Level: model.LevelResources},
				namespace:   "beta-ns",
				middleItems: []model.Item{{Name: "beta-item"}},
			},
		},
		activeTab:     1,
		nav:           model.NavigationState{Context: "beta", Level: model.LevelResources},
		namespace:     "beta-ns",
		middleItems:   []model.Item{{Name: "beta-item"}},
		width:         120,
		height:        30,
		mode:          modeExplorer,
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
	}

	ret, _ := m.closeTabOrQuit()
	result := ret.(Model)

	assert.Len(t, result.tabs, 1)
	assert.Equal(t, 0, result.activeTab, "activeTab should clamp to 0")
	assert.Equal(t, "alpha", result.nav.Context, "model should reflect surviving tab (alpha)")
	assert.Equal(t, "alpha-ns", result.namespace)
	assert.Equal(t, "alpha-item", result.middleItems[0].Name)
}
