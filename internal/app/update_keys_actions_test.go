package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// --- handleExplorerActionKey: A toggles all namespaces ---

func TestActionKeyATogglesAllNamespaces(t *testing.T) {
	m := baseExplorerModel()

	ret, _, handled := m.handleExplorerActionKey(runeKey('A'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.True(t, result.allNamespaces)

	ret, _, handled = result.handleExplorerActionKey(runeKey('A'))
	assert.True(t, handled)
	result = ret.(Model)
	assert.False(t, result.allNamespaces)
}

// --- handleExplorerActionKey: ctrl+d half page down ---

func TestActionKeyCtrlDHalfPageDown(t *testing.T) {
	items := make([]model.Item, 50)
	for i := range items {
		items[i] = model.Item{Name: "pod", Kind: "Pod"}
	}
	m := baseExplorerModel()
	m.middleItems = items
	m.setCursor(0)

	ret, _, handled := m.handleExplorerActionKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.True(t, handled)
	result := ret.(Model)
	assert.Greater(t, result.cursor(), 0)
}

// --- handleExplorerActionKey: ctrl+u half page up ---

func TestActionKeyCtrlUHalfPageUp(t *testing.T) {
	items := make([]model.Item, 50)
	for i := range items {
		items[i] = model.Item{Name: "pod", Kind: "Pod"}
	}
	m := baseExplorerModel()
	m.middleItems = items
	m.setCursor(30)

	ret, _, handled := m.handleExplorerActionKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.True(t, handled)
	result := ret.(Model)
	assert.Less(t, result.cursor(), 30)
}

// --- handleExplorerActionKey: ctrl+f full page down ---

func TestActionKeyCtrlFFullPageDown(t *testing.T) {
	items := make([]model.Item, 100)
	for i := range items {
		items[i] = model.Item{Name: "pod", Kind: "Pod"}
	}
	m := baseExplorerModel()
	m.middleItems = items
	m.setCursor(0)

	ret, _, handled := m.handleExplorerActionKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	assert.True(t, handled)
	result := ret.(Model)
	assert.Greater(t, result.cursor(), 0)
}

// --- handleExplorerActionKey: ctrl+b full page up ---

func TestActionKeyCtrlBFullPageUp(t *testing.T) {
	items := make([]model.Item, 100)
	for i := range items {
		items[i] = model.Item{Name: "pod", Kind: "Pod"}
	}
	m := baseExplorerModel()
	m.middleItems = items
	m.setCursor(50)

	ret, _, handled := m.handleExplorerActionKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	assert.True(t, handled)
	result := ret.(Model)
	assert.Less(t, result.cursor(), 50)
}

// --- handleExplorerActionKey: J/K scroll preview ---

func TestActionKeyJScrollsPreviewDown(t *testing.T) {
	m := baseExplorerModel()
	m.previewScroll = 0
	// Provide enough preview content so clamp does not reset to 0.
	m.previewYAML = strings.Repeat("line\n", 200)
	m.fullYAMLPreview = true

	ret, _, handled := m.handleExplorerActionKey(runeKey('J'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, 1, result.previewScroll)
}

func TestActionKeyKScrollsPreviewUp(t *testing.T) {
	m := baseExplorerModel()
	m.previewScroll = 5

	ret, _, handled := m.handleExplorerActionKey(runeKey('K'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, 4, result.previewScroll)
}

func TestActionKeyKDoesNotGoBelowZero(t *testing.T) {
	m := baseExplorerModel()
	m.previewScroll = 0

	ret, _, handled := m.handleExplorerActionKey(runeKey('K'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, 0, result.previewScroll)
}

// --- handleExplorerActionKey: 0 jumps to clusters ---

func TestActionKey0JumpsToClusters(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources

	ret, _, handled := m.handleExplorerActionKey(runeKey('0'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, model.LevelClusters, result.nav.Level)
}

// --- handleExplorerActionKey: 1 jumps to resource types ---

func TestActionKey1JumpsToResourceTypes(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources

	ret, _, handled := m.handleExplorerActionKey(runeKey('1'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, model.LevelResourceTypes, result.nav.Level)
}

func TestActionKey1NoopWhenBelowResourceTypes(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelClusters

	_, _, handled := m.handleExplorerActionKey(runeKey('1'))
	assert.True(t, handled)
}

// --- handleExplorerActionKey: 2 jumps to resources ---

func TestActionKey2JumpsToResources(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelOwned

	ret, _, handled := m.handleExplorerActionKey(runeKey('2'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, model.LevelResources, result.nav.Level)
}

// --- handleExplorerActionKey: , cycles sort mode ---

func TestActionKeyCommaCyclesSort(t *testing.T) {
	m := baseExplorerModel()
	m.sortBy = sortByName

	ret, _, handled := m.handleExplorerActionKey(runeKey(','))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, sortByAge, result.sortBy)
}

// --- handleExplorerActionKey: y copies resource name ---

func TestActionKeyYCopiesResourceName(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(0)

	_, cmd, handled := m.handleExplorerActionKey(runeKey('y'))
	assert.True(t, handled)
	assert.NotNil(t, cmd)
}

// --- handleExplorerActionKey: t creates new tab ---

func TestActionKeyTCreatesTab(t *testing.T) {
	m := baseExplorerModel()
	m.tabs = []TabState{{nav: m.nav}}
	m.activeTab = 0

	ret, _, handled := m.handleExplorerActionKey(runeKey('t'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Len(t, result.tabs, 2)
	assert.Equal(t, 1, result.activeTab)
}

func TestActionKeyTMaxTabs(t *testing.T) {
	m := baseExplorerModel()
	m.tabs = make([]TabState, 9)
	m.activeTab = 0

	ret, _, handled := m.handleExplorerActionKey(runeKey('t'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Len(t, result.tabs, 9)
}

// --- handleExplorerActionKey: ] next tab ---

func TestActionKeyBracketNextTab(t *testing.T) {
	m := baseExplorerModel()
	m.tabs = []TabState{
		{nav: model.NavigationState{Context: "ctx-1"}},
		{nav: model.NavigationState{Context: "ctx-2"}},
	}
	m.activeTab = 0

	ret, _, handled := m.handleExplorerActionKey(runeKey(']'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, 1, result.activeTab)
}

func TestActionKeyBracketNextTabSingleTab(t *testing.T) {
	m := baseExplorerModel()
	m.tabs = []TabState{{}}
	m.activeTab = 0

	_, _, handled := m.handleExplorerActionKey(runeKey(']'))
	assert.True(t, handled)
}

// --- handleExplorerActionKey: [ prev tab ---

func TestActionKeyBracketPrevTab(t *testing.T) {
	m := baseExplorerModel()
	m.tabs = []TabState{
		{nav: model.NavigationState{Context: "ctx-1"}},
		{nav: model.NavigationState{Context: "ctx-2"}},
	}
	m.activeTab = 1

	ret, _, handled := m.handleExplorerActionKey(runeKey('['))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, 0, result.activeTab)
}

// --- handleExplorerActionKey: a opens template overlay ---

func TestActionKeyAOpensTemplates(t *testing.T) {
	m := baseExplorerModel()

	ret, _, handled := m.handleExplorerActionKey(runeKey('a'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, overlayTemplates, result.overlay)
}

func TestActionKeyATemplateMatchesCurrentKind(t *testing.T) {
	m := baseExplorerModel()
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:        "ConfigMap",
		DisplayName: "ConfigMaps",
		Resource:    "configmaps",
		APIVersion:  "v1",
	}
	m.nav.Level = model.LevelResources

	ret, _, handled := m.handleExplorerActionKey(runeKey('a'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, overlayTemplates, result.overlay)
	require.NotEmpty(t, result.templateItems)
	assert.Equal(t, "ConfigMap", result.templateItems[0].Name,
		"template matching current resource kind should be first in the list")
}

func TestActionKeyATemplateNoMatchKeepsOriginalOrder(t *testing.T) {
	m := baseExplorerModel()
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:        "CustomWidget",
		DisplayName: "CustomWidgets",
		Resource:    "customwidgets",
	}
	m.nav.Level = model.LevelResources

	ret, _, _ := m.handleExplorerActionKey(runeKey('a'))
	result := ret.(Model)
	// First template should be the default first one (Pod) when no match.
	require.NotEmpty(t, result.templateItems)
	assert.Equal(t, "Pod", result.templateItems[0].Name,
		"when no template matches current kind, original order should be preserved")
}

// --- handleExplorerActionKey: . opens filter presets ---

func TestActionKeyDotOpensFilterPresets(t *testing.T) {
	m := baseExplorerModel()

	ret, _, handled := m.handleExplorerActionKey(runeKey('.'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, overlayFilterPreset, result.overlay)
}

func TestActionKeyDotClearsActiveFilter(t *testing.T) {
	m := baseExplorerModel()
	m.activeFilterPreset = &FilterPreset{Name: "Failing"}
	m.unfilteredMiddleItems = m.middleItems

	ret, _, handled := m.handleExplorerActionKey(runeKey('.'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Nil(t, result.activeFilterPreset)
}

func TestActionKeyDotBelowResourceLevel(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelClusters

	ret, _, handled := m.handleExplorerActionKey(runeKey('.'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Contains(t, result.statusMessage, "only available at resource level")
}

// --- handleExplorerActionKey: d requires 2 selected ---

func TestActionKeyDDiffRequires2Selected(t *testing.T) {
	m := baseExplorerModel()

	ret, _, handled := m.handleExplorerActionKey(runeKey('d'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Contains(t, result.statusMessage, "Select exactly 2")
}

// --- handleExplorerActionKey: ! opens error log ---

func TestActionKeyBangOpensErrorLog(t *testing.T) {
	m := baseExplorerModel()

	ret, _, handled := m.handleExplorerActionKey(runeKey('!'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.True(t, result.overlayErrorLog)
}

// --- handleExplorerActionKey: unhandled key returns false ---

func TestActionKeyUnhandledReturnsFalse(t *testing.T) {
	m := baseExplorerModel()

	_, _, handled := m.handleExplorerActionKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'^'}})
	assert.False(t, handled)
}

// --- handleExplorerActionKey: ctrl+d/u in fullscreen dashboard ---

func TestActionKeyCtrlDInFullscreenDashboard(t *testing.T) {
	m := baseExplorerModel()
	m.fullscreenDashboard = true
	m.previewScroll = 0
	// Set up as dashboard at LevelResourceTypes with enough content for scroll.
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}}
	m.dashboardPreview = strings.Repeat("dashboard line\n", 200)

	ret, _, handled := m.handleExplorerActionKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.True(t, handled)
	result := ret.(Model)
	assert.Greater(t, result.previewScroll, 0)
}

func TestActionKeyCtrlUInFullscreenDashboard(t *testing.T) {
	m := baseExplorerModel()
	m.fullscreenDashboard = true
	m.previewScroll = 50
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}}
	m.dashboardPreview = strings.Repeat("dashboard line\n", 200)

	ret, _, handled := m.handleExplorerActionKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.True(t, handled)
	result := ret.(Model)
	assert.Less(t, result.previewScroll, 50)
}

func TestActionKeyCtrlFInFullscreenDashboard(t *testing.T) {
	m := baseExplorerModel()
	m.fullscreenDashboard = true
	m.previewScroll = 0
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}}
	m.dashboardPreview = strings.Repeat("dashboard line\n", 200)

	ret, _, handled := m.handleExplorerActionKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	assert.True(t, handled)
	result := ret.(Model)
	assert.Greater(t, result.previewScroll, 0)
}

func TestActionKeyCtrlBInFullscreenDashboard(t *testing.T) {
	m := baseExplorerModel()
	m.fullscreenDashboard = true
	m.previewScroll = 50
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}}
	m.dashboardPreview = strings.Repeat("dashboard line\n", 200)

	ret, _, handled := m.handleExplorerActionKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	assert.True(t, handled)
	result := ret.(Model)
	assert.Less(t, result.previewScroll, 50)
}

// --- handleExplorerActionKey: Q loads quotas ---

func TestActionKeyQLoadsQuotas(t *testing.T) {
	m := baseExplorerModel()

	ret, cmd, handled := m.handleExplorerActionKey(runeKey('Q'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.True(t, result.loading)
	assert.NotNil(t, cmd)
}
