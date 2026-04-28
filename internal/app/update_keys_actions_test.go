package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
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

// --- handleExplorerActionKey: > cycles sort mode forward ---

func TestActionKeyGreaterCyclesSortNext(t *testing.T) {
	m := baseExplorerModel()
	m.sortColumnName = "Name"
	m.sortAscending = true
	ui.ActiveSortableColumns = []string{"Name", "Age", "Status"}
	ui.ActiveSortableColumnCount = 3

	ret, _, handled := m.handleExplorerActionKey(runeKey('>'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, "Age", result.sortColumnName)
}

// --- handleExplorerActionKey: < cycles sort mode backward ---

func TestActionKeyLessCyclesSortPrev(t *testing.T) {
	m := baseExplorerModel()
	m.sortColumnName = "Name"
	m.sortAscending = true
	ui.ActiveSortableColumns = []string{"Name", "Age", "Status"}
	ui.ActiveSortableColumnCount = 3

	ret, _, handled := m.handleExplorerActionKey(runeKey('<'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, "Status", result.sortColumnName)
}

// --- handleExplorerActionKey: sort keys are no-ops at picker levels ---
//
// At LevelClusters and LevelResourceTypes, sortMiddleItems() early-returns,
// so >, <, =, - mutating sort state and emitting "Sort: ..." status messages
// is misleading: the bar lies that sort changed when items are unmoved.
// These keys must short-circuit silently at those levels.

func TestActionKeySortNextNoOpAtResourceTypes(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResourceTypes
	m.sortColumnName = "Name"
	m.sortAscending = true
	oldCols := ui.ActiveSortableColumns
	oldCount := ui.ActiveSortableColumnCount
	t.Cleanup(func() {
		ui.ActiveSortableColumns = oldCols
		ui.ActiveSortableColumnCount = oldCount
	})
	ui.ActiveSortableColumns = []string{"Name", "Age", "Status"}
	ui.ActiveSortableColumnCount = 3

	ret, cmd, handled := m.handleExplorerActionKey(runeKey('>'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, "Name", result.sortColumnName, "sort column must not change at LevelResourceTypes")
	assert.True(t, result.sortAscending)
	assert.Empty(t, result.statusMessage, "no misleading status message")
	assert.Nil(t, cmd)
}

func TestActionKeySortPrevNoOpAtResourceTypes(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResourceTypes
	m.sortColumnName = "Name"
	oldCols := ui.ActiveSortableColumns
	oldCount := ui.ActiveSortableColumnCount
	t.Cleanup(func() {
		ui.ActiveSortableColumns = oldCols
		ui.ActiveSortableColumnCount = oldCount
	})
	ui.ActiveSortableColumns = []string{"Name", "Age", "Status"}
	ui.ActiveSortableColumnCount = 3

	ret, cmd, handled := m.handleExplorerActionKey(runeKey('<'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, "Name", result.sortColumnName)
	assert.Empty(t, result.statusMessage)
	assert.Nil(t, cmd)
}

func TestActionKeySortFlipNoOpAtClusters(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelClusters
	m.sortAscending = true

	ret, cmd, handled := m.handleExplorerActionKey(runeKey('='))
	assert.True(t, handled)
	result := ret.(Model)
	assert.True(t, result.sortAscending, "sortAscending must not toggle at LevelClusters")
	assert.Empty(t, result.statusMessage)
	assert.Nil(t, cmd)
}

func TestActionKeySortResetNoOpAtResourceTypes(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResourceTypes
	m.sortColumnName = "Status"
	m.sortAscending = false

	ret, cmd, handled := m.handleExplorerActionKey(runeKey('-'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, "Status", result.sortColumnName, "reset must not clobber column at LevelResourceTypes")
	assert.False(t, result.sortAscending, "reset must not flip ascending at LevelResourceTypes")
	assert.Empty(t, result.statusMessage)
	assert.Nil(t, cmd)
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

func TestPush2HandleExplorerActionKeyBackspace(t *testing.T) {
	m := basePush80v2Model()
	result, _, handled := m.handleExplorerActionKey(keyMsg("backspace"))
	if handled {
		_ = result.(Model)
	}
}

func TestPush2HandleExplorerActionKeyM(t *testing.T) {
	m := basePush80v2Model()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	result, _, handled := m.handleExplorerActionKey(keyMsg("m"))
	// 'm' is handled by handleKey, not handleExplorerActionKey.
	// It may not be handled here.
	_ = result
	_ = handled
}

func TestPush2HandleExplorerActionKeyEqualSign(t *testing.T) {
	m := basePush80v2Model()
	ui.ActiveSortableColumns = []string{"Name", "Status"}
	m.sortColumnName = "Name"
	m.sortAscending = true
	result, cmd, handled := m.handleExplorerActionKey(keyMsg("="))
	assert.True(t, handled)
	rm := result.(Model)
	assert.False(t, rm.sortAscending)
	assert.NotNil(t, cmd)
}

func TestPush2HandleExplorerActionKeyDash(t *testing.T) {
	m := basePush80v2Model()
	ui.ActiveSortableColumns = []string{"Name", "Status"}
	m.sortColumnName = "Status"
	result, cmd, handled := m.handleExplorerActionKey(keyMsg("-"))
	assert.True(t, handled)
	rm := result.(Model)
	// '-' resets sort -- sortColumnName becomes "Name" (default) or cleared.
	_ = rm
	assert.NotNil(t, cmd)
}

func TestP4ExplorerActionKeyF(t *testing.T) {
	m := bp4()
	result, _, handled := m.handleExplorerActionKey(keyMsg("f"))
	if handled {
		rm := result.(Model)
		assert.True(t, rm.filterActive)
	}
}

func TestP4ExplorerActionKeyQuote(t *testing.T) {
	m := bp4()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	result, _, handled := m.handleExplorerActionKey(keyMsg("'"))
	if handled {
		_ = result.(Model)
	}
}

func TestP4ExplorerActionKeyComma(t *testing.T) {
	m := bp4()
	ui.ActiveSortableColumns = []string{"Name", "Status", "Age"}
	result, _, handled := m.handleExplorerActionKey(keyMsg(","))
	if handled {
		_ = result.(Model)
	}
}

// --- handleExplorerActionKeyToggleRare ---

// rareResourceEntries returns a discovered resource set that contains one
// always-visible entry (Pod) and one curated Rare entry (CSIDriver). Used to
// exercise the rare-toggle rebuild in tests.
func rareResourceEntries() []model.ResourceTypeEntry {
	return []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
		{Kind: "CSIDriver", APIGroup: "storage.k8s.io", APIVersion: "v1", Resource: "csidrivers", Namespaced: false},
	}
}

// containsCSIDrivers reports whether the given sidebar item list contains
// the curated Rare "CSIDrivers" entry. Sidebar items use the display name
// as Name, so a substring match is sufficient for this regression guard.
func containsCSIDrivers(items []model.Item) bool {
	for _, it := range items {
		if it.Name == "CSIDrivers" {
			return true
		}
	}
	return false
}

// TestActionKeyToggleRareAtLevelResourceTypes verifies the in-place rebuild
// of middleItems when the user is on the resource types level.
func TestActionKeyToggleRareAtLevelResourceTypes(t *testing.T) {
	defer func(orig bool) { model.ShowRareResources = orig }(model.ShowRareResources)
	model.ShowRareResources = false

	m := baseExplorerModel()
	m.discoveredResources = map[string][]model.ResourceTypeEntry{}
	m.nav.Level = model.LevelResourceTypes
	m.nav.ResourceType = model.ResourceTypeEntry{}
	m.discoveredResources["test"] = rareResourceEntries()
	m.middleItems = model.BuildSidebarItems(rareResourceEntries())
	m.leftItems = nil

	require.False(t, containsCSIDrivers(m.middleItems),
		"CSIDrivers must be hidden by default")

	result, _, handled := m.handleExplorerActionKeyToggleRare()
	require.True(t, handled)
	rm := result.(Model)

	assert.True(t, rm.showRareResources)
	assert.True(t, containsCSIDrivers(rm.middleItems),
		"middleItems must include CSIDrivers after toggle ON")
}

// TestActionKeyToggleRareAtLevelResourcesRefreshesParent verifies that the
// parent column (leftItems) is refreshed when the user is deeper than the
// resource types level. This is the bug fix: previously the handler only
// updated middleItems and left the parent column stale.
func TestActionKeyToggleRareAtLevelResourcesRefreshesParent(t *testing.T) {
	defer func(orig bool) { model.ShowRareResources = orig }(model.ShowRareResources)
	model.ShowRareResources = false

	m := baseExplorerModel()
	m.discoveredResources = map[string][]model.ResourceTypeEntry{}
	m.nav.Level = model.LevelResources
	m.nav.Context = "test"
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
	}
	m.discoveredResources["test"] = rareResourceEntries()
	// The parent column starts with the default (no-rare) list.
	m.leftItems = model.BuildSidebarItems(rareResourceEntries())
	// The middle column has pods for the user's current view.
	m.middleItems = []model.Item{
		{Name: "pod-a", Kind: "Pod"},
		{Name: "pod-b", Kind: "Pod"},
	}

	require.False(t, containsCSIDrivers(m.leftItems),
		"CSIDrivers must be hidden in the parent column by default")

	result, _, handled := m.handleExplorerActionKeyToggleRare()
	require.True(t, handled)
	rm := result.(Model)

	assert.True(t, rm.showRareResources)
	assert.True(t, containsCSIDrivers(rm.leftItems),
		"leftItems (parent column) must include CSIDrivers after toggle ON")
	// The middle column (the pods list) must be untouched.
	assert.Len(t, rm.middleItems, 2, "middleItems (pods) must not be rebuilt")
	assert.Equal(t, "pod-a", rm.middleItems[0].Name)
}

// TestActionKeyToggleRareAtLevelResourcesUpdatesCursorMemory verifies that
// after rebuilding the parent column, the cursor memory for the resource
// types level points at the current resource type so navigating back with
// `h` lands on the correct row.
func TestActionKeyToggleRareAtLevelResourcesUpdatesCursorMemory(t *testing.T) {
	defer func(orig bool) { model.ShowRareResources = orig }(model.ShowRareResources)
	model.ShowRareResources = false

	m := baseExplorerModel()
	m.discoveredResources = map[string][]model.ResourceTypeEntry{}
	m.nav.Level = model.LevelResources
	m.nav.Context = "test"
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
	}
	m.discoveredResources["test"] = rareResourceEntries()
	m.leftItems = model.BuildSidebarItems(rareResourceEntries())
	m.cursorMemory = map[string]int{"test": 0}

	result, _, handled := m.handleExplorerActionKeyToggleRare()
	require.True(t, handled)
	rm := result.(Model)

	// Find Pod's index in the rebuilt leftItems.
	podIdx := -1
	for i, it := range rm.leftItems {
		if it.Extra == "/v1/pods" {
			podIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, podIdx, 0, "Pod must be present in rebuilt leftItems")
	assert.Equal(t, podIdx, rm.cursorMemory["test"],
		"cursorMemory[context] must point at the current resource type after rebuild")
}
