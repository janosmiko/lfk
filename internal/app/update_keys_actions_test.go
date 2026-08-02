package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/scheduler"
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

func TestActionKeyAllNamespacesRestoresPreviousSelection(t *testing.T) {
	m := baseExplorerModel()
	m.selectedNamespaces = map[string]bool{"alpha": true, "beta": true}
	m.allNamespaces = false

	// Toggle ON: selection stashed, all-namespaces enabled.
	ret, _, handled := m.handleExplorerActionKey(runeKey('A'))
	require.True(t, handled)
	on := ret.(Model)
	assert.True(t, on.allNamespaces)
	assert.Empty(t, on.selectedNamespaces, "selection cleared while in all-namespaces mode")

	// Toggle OFF: previous multi-select restored, not reset to default.
	ret, _, handled = on.handleExplorerActionKey(runeKey('A'))
	require.True(t, handled)
	off := ret.(Model)
	assert.False(t, off.allNamespaces)
	assert.Equal(t, map[string]bool{"alpha": true, "beta": true}, off.selectedNamespaces)
}

func TestActionKeyAllNamespacesRestoresNegatedSelection(t *testing.T) {
	m := baseExplorerModel()
	m.selectedNamespaces = map[string]bool{"kube-system": true}
	m.nsSelectionNegated = true
	m.allNamespaces = false

	ret, _, _ := m.handleExplorerActionKey(runeKey('A'))
	on := ret.(Model)
	assert.True(t, on.allNamespaces)
	assert.False(t, on.nsSelectionNegated, "negation flag cleared while in all-namespaces mode")

	ret, _, _ = on.handleExplorerActionKey(runeKey('A'))
	off := ret.(Model)
	assert.True(t, off.nsSelectionNegated, "negation flag restored on toggle off")
	assert.Equal(t, map[string]bool{"kube-system": true}, off.selectedNamespaces)
}

// TestActionKeyAllNamespacesSavedSelectionSurvivesTabSwitch proves the
// stashed selection is per-tab: toggling all-namespaces ON, switching tabs
// away and back, then toggling OFF must restore that tab's own selection.
func TestActionKeyAllNamespacesSavedSelectionSurvivesTabSwitch(t *testing.T) {
	m := baseExplorerModel()
	m.tabs = []TabState{{}, {}}
	m.activeTab = 0

	// Tab 1: select two namespaces, then toggle all-namespaces ON.
	m.selectedNamespaces = map[string]bool{"alpha": true, "beta": true}
	ret, _, _ := m.handleExplorerActionKey(runeKey('A'))
	m = ret.(Model)
	require.True(t, m.allNamespaces)
	require.Empty(t, m.selectedNamespaces)

	// Switch to tab 2 and back to tab 1.
	ret, _, _ = m.handleExplorerActionKeyNextTab()
	m = ret.(Model)
	require.Equal(t, 1, m.activeTab)
	ret, _, _ = m.handleExplorerActionKeyPrevTab()
	m = ret.(Model)
	require.Equal(t, 0, m.activeTab)

	// Tab 1 is still in all-namespaces mode; toggling OFF restores the
	// selection that was stashed before the tab round-trip.
	require.True(t, m.allNamespaces)
	ret, _, _ = m.handleExplorerActionKey(runeKey('A'))
	m = ret.(Model)
	assert.False(t, m.allNamespaces)
	assert.Equal(t, map[string]bool{"alpha": true, "beta": true}, m.selectedNamespaces)
}

// TestActionKeyAllNamespacesOffFallsBackToDefault covers the repro where the
// app is in all-namespaces mode with no concrete namespace to return to
// (m.namespace == "" and no saved selection): toggling OFF must scope to the
// default namespace instead of silently staying on every namespace.
func TestActionKeyAllNamespacesOffFallsBackToDefault(t *testing.T) {
	m := baseExplorerModel()
	m.allNamespaces = true
	m.namespace = ""
	m.selectedNamespaces = nil
	m.savedSelectedNamespaces = nil

	ret, _, handled := m.handleExplorerActionKey(runeKey('A'))
	require.True(t, handled)
	off := ret.(Model)

	assert.False(t, off.allNamespaces)
	assert.Equal(t, "default", off.namespace)
	assert.Equal(t, "default", off.effectiveNamespace(), "must scope the fetch, not return empty (all namespaces)")
	assert.Contains(t, off.statusMessage, "ns: default")
}

// TestActionKeyAllNamespacesOffKeepsRestoredSelection ensures the default
// fallback does not clobber a restored single-namespace selection.
func TestActionKeyAllNamespacesOffKeepsRestoredSelection(t *testing.T) {
	m := baseExplorerModel()
	m.selectedNamespaces = map[string]bool{"team-a": true}
	m.namespace = ""
	m.allNamespaces = false

	// Toggle ON (stash), then OFF (restore).
	ret, _, _ := m.handleExplorerActionKey(runeKey('A'))
	on := ret.(Model)
	ret, _, _ = on.handleExplorerActionKey(runeKey('A'))
	off := ret.(Model)

	assert.False(t, off.allNamespaces)
	assert.Equal(t, map[string]bool{"team-a": true}, off.selectedNamespaces)
	assert.Empty(t, off.namespace, "single-namespace selection takes precedence; no default fallback needed")
	// Status hint must reflect the restored selection, not the empty
	// m.namespace (would otherwise read "(ns: )").
	assert.Contains(t, off.statusMessage, "ns: team-a")
}

func TestActionKeyAllNamespacesNoOpAtClusters(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelClusters
	m.allNamespaces = false

	ret, cmd, handled := m.handleExplorerActionKey(runeKey('A'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.False(t, result.allNamespaces, "all-namespaces must not toggle without a selected context")
	assert.NotEmpty(t, result.statusMessage)
	assert.NotNil(t, cmd)
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

	ret, _, handled := m.handleExplorerActionKey(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
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

	ret, _, handled := m.handleExplorerActionKey(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
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

	ret, _, handled := m.handleExplorerActionKey(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
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

	ret, _, handled := m.handleExplorerActionKey(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
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

// --- handleExplorerActionKey: cycling from a hidden sort column (issue #339) ---
//
// When the active sort column is not in the current layout's sortable set
// (e.g. a wide-only column after leaving fullscreen), cycling must enter the
// visible set at a predictable boundary rather than jumping past the first
// column. Previously sortColumnIndex returned 0 for a missing column, so
// `>` skipped Name and landed on the second column.

func TestActionKeySortNextFromHiddenColumnLandsOnFirst(t *testing.T) {
	m := baseExplorerModel()
	m.sortColumnName = "Node" // wide-only, hidden in narrow mode
	m.sortAscending = true
	oldCols := ui.ActiveSortableColumns
	oldCount := ui.ActiveSortableColumnCount
	t.Cleanup(func() {
		ui.ActiveSortableColumns = oldCols
		ui.ActiveSortableColumnCount = oldCount
	})
	ui.ActiveSortableColumns = []string{"Name", "Age", "Status"}
	ui.ActiveSortableColumnCount = 3

	ret, _, handled := m.handleExplorerActionKey(runeKey('>'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, "Name", result.sortColumnName,
		"next from a hidden sort column must land on the first visible column")
}

func TestActionKeySortPrevFromHiddenColumnLandsOnLast(t *testing.T) {
	m := baseExplorerModel()
	m.sortColumnName = "Node" // wide-only, hidden in narrow mode
	m.sortAscending = true
	oldCols := ui.ActiveSortableColumns
	oldCount := ui.ActiveSortableColumnCount
	t.Cleanup(func() {
		ui.ActiveSortableColumns = oldCols
		ui.ActiveSortableColumnCount = oldCount
	})
	ui.ActiveSortableColumns = []string{"Name", "Age", "Status"}
	ui.ActiveSortableColumnCount = 3

	ret, _, handled := m.handleExplorerActionKey(runeKey('<'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, "Status", result.sortColumnName,
		"prev from a hidden sort column must land on the last visible column")
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

func TestActionKeyANoOpAtClusters(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelClusters

	ret, cmd, handled := m.handleExplorerActionKey(runeKey('a'))
	assert.True(t, handled)
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay,
		"template overlay must not open without a selected context")
	assert.NotEmpty(t, result.statusMessage)
	assert.NotNil(t, cmd)
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

	_, _, handled := m.handleExplorerActionKey(tea.KeyPressMsg{Code: '^', Text: "^"})
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

	ret, _, handled := m.handleExplorerActionKey(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
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

	ret, _, handled := m.handleExplorerActionKey(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
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

	ret, _, handled := m.handleExplorerActionKey(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
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

	ret, _, handled := m.handleExplorerActionKey(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
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

// --- y / Y bulk copy: selection wins over cursor (mirrors directActionDelete) ---

// TestCopyNameBulkUsesSelectionOverCursor verifies pressing `y` with N items
// multi-selected joins their names, copies them, and reports the count —
// rather than copying just the cursor row. Mirrors the precedence used by
// directActionDelete.
func TestCopyNameBulkUsesSelectionOverCursor(t *testing.T) {
	m := basePush80Model()
	m.toggleSelection(m.middleItems[0]) // pod-1
	m.toggleSelection(m.middleItems[2]) // pod-3
	m.setCursor(1)                      // cursor on pod-2 — must be ignored

	ret, cmd, handled := m.handleExplorerActionKeyCopyName()
	require.True(t, handled)
	rm := ret.(Model)

	assert.Equal(t, "Copied 2 names", rm.statusMessage)
	assert.NotNil(t, cmd, "must dispatch a clipboard cmd")
}

// TestCopyNameNoSelectionFallsBackToCursor guards the single-item fallback —
// without a selection, `y` still copies just the cursor row.
func TestCopyNameNoSelectionFallsBackToCursor(t *testing.T) {
	m := basePush80Model()
	m.setCursor(0)

	ret, cmd, handled := m.handleExplorerActionKeyCopyName()
	require.True(t, handled)
	rm := ret.(Model)

	assert.Equal(t, "Copied: pod-1", rm.statusMessage)
	assert.NotNil(t, cmd)
}

// TestCopyNameSelectionFilteredOutFallsBack guards the n==0 edge case: a
// selection survives in the raw map but every selected row is currently
// filtered out of view, so selectedItemsList returns empty. `y` must fall
// through to copy the cursor row rather than emit an empty clipboard write.
func TestCopyNameSelectionFilteredOutFallsBack(t *testing.T) {
	m := basePush80Model()
	// Forge a "ghost" selection — a key not present in middleItems, so
	// hasSelection() == true but selectedItemsList() == [].
	m.selectedItems["ghost-ns/ghost-pod"] = true
	m.setCursor(0)

	ret, cmd, handled := m.handleExplorerActionKeyCopyName()
	require.True(t, handled)
	rm := ret.(Model)

	assert.Equal(t, "Copied: pod-1", rm.statusMessage,
		"must fall through to cursor when selection has no visible items")
	assert.NotNil(t, cmd)
}

// TestCopyYAMLSelectionFilteredOutFallsBack mirrors the above for `Y`.
// With a ghost selection (hasSelection() true but selectedItemsList() empty),
// the picker still opens — its scope falls through to the cursor row, matching
// the precedence today's Y already follows. No "Fetching..." or "Max..." status
// is set at open time; status only updates once the user picks a format and
// the fetch resolves.
func TestCopyYAMLSelectionFilteredOutFallsBack(t *testing.T) {
	m := basePush80Model()
	m.selectedItems["ghost-ns/ghost-pod"] = true
	m.setCursor(0)

	ret, _, handled := m.handleExplorerActionKeyCopyYAML()
	require.True(t, handled)
	rm := ret.(Model)

	assert.True(t, rm.copyFormatPicker.active, "Y opens the picker")
	assert.Len(t, rm.copyFormatPicker.scope, 1,
		"scope falls back to cursor row when selection has no visible items")
	assert.Equal(t, "pod-1", rm.copyFormatPicker.scope[0].Name)
	assert.Empty(t, rm.statusMessage,
		"opening the picker does not set a status; that happens after apply")
}

// TestCopyYAMLBulkErrorWrapsNamespaceName checks that the bulk fetch path
// surfaces fetch errors tagged with the offending ns/name — the marker that
// proves the bulk branch ran (vs the single-item path, which would emit an
// unwrapped error). The successful join + count is exercised by
// TestUpdateYamlClipboardCountAwareStatus/bulk against the receiver directly.
func TestCopyYAMLBulkErrorWrapsNamespaceName(t *testing.T) {
	m := basePush80Model()
	m.scheduler = scheduler.New(0)
	m.toggleSelection(m.middleItems[0]) // default/pod-1
	m.toggleSelection(m.middleItems[1]) // ns-2/pod-2

	cmd := m.copyYAMLToClipboard()
	require.NotNil(t, cmd)

	ymsg, ok := execScheduled(t, m, cmd).(yamlClipboardMsg)
	require.True(t, ok)
	require.Error(t, ymsg.err, "fake client has no pods seeded; first fetch must fail")
	assert.Contains(t, ymsg.err.Error(), "default/pod-1",
		"bulk path must wrap errors with ns/name")
	assert.Equal(t, 0, ymsg.count, "early-return on error keeps count at zero")
}

// TestCopyYAMLBulkSetsFetchingStatus checks the wrapper surfaces a
// "Fetching N manifests..." hint before dispatching, so the user has
// feedback during the sequential fetch (client-go's default rate limiter
// serializes the per-item Gets — see maxBulkYAMLCopy comment).
//
// After Task 10, Y opens the picker first; applying the YAML row from the
// picker is what runs dispatchYAMLClipboardCopy and sets the status — so
// the test now drives open-then-apply rather than direct dispatch.
func TestCopyYAMLBulkSetsFetchingStatus(t *testing.T) {
	m := basePush80Model()
	m.toggleSelection(m.middleItems[0])
	m.toggleSelection(m.middleItems[1])

	mdl, _, handled := m.handleExplorerActionKeyCopyYAML()
	require.True(t, handled)
	r := mdl.(Model)
	require.True(t, r.copyFormatPicker.active, "Y must open the picker")

	// Apply the YAML row (cursor=0 at LevelResources → YAML).
	mdl2, cmd := r.applyCopyFormatPicker()
	rm := mdl2.(Model)

	assert.Equal(t, "Fetching 2 manifests...", rm.statusMessage)
	assert.NotNil(t, cmd)
}

// TestCopyYAMLBulkRejectsOverCap verifies selections larger than
// maxBulkYAMLCopy bail out with an error toast at open time, before the
// user even picks a format. The picker stays closed so the cap rejection
// is the only feedback they see — no half-open overlay.
func TestCopyYAMLBulkRejectsOverCap(t *testing.T) {
	m := basePush80Model()
	m.middleItems = make([]model.Item, maxBulkYAMLCopy+1)
	for i := range m.middleItems {
		m.middleItems[i] = model.Item{
			Name:      fmt.Sprintf("pod-%d", i),
			Namespace: "default",
			Kind:      "Pod",
		}
		m.toggleSelection(m.middleItems[i])
	}

	ret, cmd, handled := m.handleExplorerActionKeyCopyYAML()
	require.True(t, handled)
	rm := ret.(Model)

	assert.Equal(t, fmt.Sprintf("Max %d exceeded for bulk YAML/JSON copy", maxBulkYAMLCopy), rm.statusMessage)
	assert.True(t, rm.statusMessageErr, "must surface as error toast")
	assert.False(t, rm.copyFormatPicker.active, "picker must not open when cap is exceeded")
	// Cmd is the auto-clear timer, not a fetch.
	assert.NotNil(t, cmd)
}

// TestCopyYAMLBulkAtCapDispatches confirms the boundary case (exactly N=cap)
// is allowed through: Y opens the picker, and applying YAML kicks off the
// bulk fetch with the "Fetching..." status.
func TestCopyYAMLBulkAtCapDispatches(t *testing.T) {
	m := basePush80Model()
	m.middleItems = make([]model.Item, maxBulkYAMLCopy)
	for i := range m.middleItems {
		m.middleItems[i] = model.Item{
			Name:      fmt.Sprintf("pod-%d", i),
			Namespace: "default",
			Kind:      "Pod",
		}
		m.toggleSelection(m.middleItems[i])
	}

	mdl, _, handled := m.handleExplorerActionKeyCopyYAML()
	require.True(t, handled)
	r := mdl.(Model)
	require.True(t, r.copyFormatPicker.active, "exactly N=cap must open the picker")

	// Apply YAML (cursor=0 at LevelResources → YAML).
	mdl2, cmd := r.applyCopyFormatPicker()
	rm := mdl2.(Model)

	assert.Contains(t, rm.statusMessage, "Fetching")
	assert.False(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

// TestUpdateYamlClipboardCountAwareStatus verifies the receiver picks the
// "Copied N manifests" message when count > 1, and the legacy single-item
// message otherwise.
func TestUpdateYamlClipboardCountAwareStatus(t *testing.T) {
	t.Run("bulk", func(t *testing.T) {
		m := basePush80Model()
		ret, cmd := m.updateYamlClipboard(yamlClipboardMsg{
			content: "a\n---\nb\n",
			count:   3,
		})
		rm := ret.(Model)
		assert.Equal(t, "Copied 3 manifests as YAML", rm.statusMessage)
		assert.NotNil(t, cmd)
	})
	t.Run("single", func(t *testing.T) {
		m := basePush80Model()
		ret, cmd := m.updateYamlClipboard(yamlClipboardMsg{
			content: "a\n",
			count:   1,
		})
		rm := ret.(Model)
		assert.Equal(t, "YAML copied to clipboard", rm.statusMessage)
		assert.NotNil(t, cmd)
	})
}

// TestCopyYAMLBulkLevelOwnedWrapsErrorWithNamespaceName guards bulk Y at
// LevelOwned. Multi-select is allowed at LevelOwned (>= LevelResources) and
// bulk delete works there, so bulk YAML must too — otherwise the dispatcher
// shows "Fetching N manifests..." but the cmd silently returns just the
// cursor row's YAML (count=1) and the user gets one manifest when the toast
// promised N. The bulk branch wraps fetch errors with ns/name and tags
// count=0 on early-return; the single-item path does neither, so this
// assertion pins down which branch ran.
func TestCopyYAMLBulkLevelOwnedWrapsErrorWithNamespaceName(t *testing.T) {
	m := basePush80Model()
	m.scheduler = scheduler.New(0)
	m.nav.Level = model.LevelOwned
	m.toggleSelection(m.middleItems[0]) // default/pod-1
	m.toggleSelection(m.middleItems[1]) // ns-2/pod-2

	cmd := m.copyYAMLToClipboard()
	require.NotNil(t, cmd)

	ymsg, ok := execScheduled(t, m, cmd).(yamlClipboardMsg)
	require.True(t, ok)
	require.Error(t, ymsg.err, "fake client has no pods seeded; first fetch must fail")
	assert.Contains(t, ymsg.err.Error(), "default/pod-1",
		"bulk path must wrap errors with ns/name (single-item path does not)")
	assert.Equal(t, 0, ymsg.count, "early-return on error keeps count at zero")
}

// TestCopyYAMLBulkLevelOwnedDispatcherShowsFetchingStatus confirms the
// dispatcher's "Fetching N manifests..." pre-fetch status applies at
// LevelOwned the same as LevelResources, once the user picks YAML from
// the picker that Y opens.
func TestCopyYAMLBulkLevelOwnedDispatcherShowsFetchingStatus(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.toggleSelection(m.middleItems[0])
	m.toggleSelection(m.middleItems[1])

	mdl, _, handled := m.handleExplorerActionKeyCopyYAML()
	require.True(t, handled)
	r := mdl.(Model)
	require.True(t, r.copyFormatPicker.active, "Y must open the picker at LevelOwned")

	// Apply YAML (cursor=0 → YAML at LevelOwned).
	mdl2, cmd := r.applyCopyFormatPicker()
	rm := mdl2.(Model)

	assert.Equal(t, "Fetching 2 manifests...", rm.statusMessage)
	assert.NotNil(t, cmd)
}

// TestCopyYAMLLevelContainersBulkSelection asserts the new bulk behavior at
// LevelContainers: when N containers are selected and the user picks YAML
// from the picker that Y opens, the dispatcher fires the "Fetching N
// manifests..." status, fetches the parent Pod once, and the command
// extracts the matching container spec blocks. Without seeded Pod YAML in
// the fake client the fetch errors, but error wrapping (`ns/name:`)
// confirms the bulk path was taken rather than the single-pod fallthrough.
func TestCopyYAMLLevelContainersBulkSelection(t *testing.T) {
	m := basePush80Model()
	m.scheduler = scheduler.New(0)
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "pod-1"
	m.middleItems = []model.Item{
		{Name: "container-1", Kind: "Container", Namespace: "default"},
		{Name: "container-2", Kind: "Container", Namespace: "default"},
	}
	m.toggleSelection(m.middleItems[0])
	m.toggleSelection(m.middleItems[1])

	mdl, _, handled := m.handleExplorerActionKeyCopyYAML()
	require.True(t, handled)
	r := mdl.(Model)
	require.True(t, r.copyFormatPicker.active, "Y must open the picker at LevelContainers")

	// Apply YAML (cursor=0 → YAML).
	mdl2, cmd := r.applyCopyFormatPicker()
	rm := mdl2.(Model)

	assert.Equal(t, "Fetching 2 manifests...", rm.statusMessage,
		"LevelContainers now supports bulk; dispatcher must show the fetching toast")
	require.NotNil(t, cmd, "must still dispatch a bulk fetch")

	ymsg, ok := execScheduled(t, r, cmd).(yamlClipboardMsg)
	require.True(t, ok)
	require.Error(t, ymsg.err, "fake client has no pod seeded; Pod GET must fail")
	assert.Contains(t, ymsg.err.Error(), "default/pod-1",
		"bulk path wraps the Pod fetch error with ns/name")
	assert.Equal(t, 0, ymsg.count, "early-return on error keeps count at zero")
}

// TestYKeyOpensPicker_AtResources confirms `Y` at LevelResources opens the
// copy-as picker instead of dispatching a YAML copy directly. This is the
// observable contract change for Task 10: Y is no longer a direct YAML
// dispatcher — it gates on format choice via the picker.
func TestYKeyOpensPicker_AtResources(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{{Name: "a", Kind: "Pod"}}
	m.setCursor(0)
	mdl, _, handled := m.handleExplorerActionKeyCopyYAML()
	assert.True(t, handled)
	r := mdl.(Model)
	assert.True(t, r.copyFormatPicker.active, "Y must open the picker, not copy directly")
}

// TestYKeyOpensPicker_AtClusters confirms `Y` at LevelClusters opens the
// picker with the Table-only format set — there is no manifest behind a
// cluster row, so YAML/JSON are deliberately omitted.
func TestYKeyOpensPicker_AtClusters(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "kind-1"}}
	m.setCursor(0)
	mdl, _, handled := m.handleExplorerActionKeyCopyYAML()
	assert.True(t, handled)
	r := mdl.(Model)
	assert.True(t, r.copyFormatPicker.active)
	assert.Equal(t, []CopyFormat{CopyFormatTable}, r.copyFormatPicker.formats)
}

// TestYKeyBulkCapExceeded verifies the cap check fires at open time: with
// more than maxBulkYAMLCopy items selected at a bulk-capable level, the
// picker must NOT open and an error toast is shown immediately. This gives
// the user feedback before they pick a format rather than after.
func TestYKeyBulkCapExceeded(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	items := make([]model.Item, 51)
	for i := range items {
		items[i] = model.Item{Name: fmt.Sprintf("p-%d", i), Kind: "Pod"}
	}
	m.middleItems = items
	m.selectedItems = map[string]bool{}
	for i := range items {
		m.selectedItems[selectionKey(items[i])] = true
	}
	m.setCursor(0)
	mdl, _, handled := m.handleExplorerActionKeyCopyYAML()
	assert.True(t, handled)
	r := mdl.(Model)
	assert.False(t, r.copyFormatPicker.active, "picker must NOT open when cap is exceeded")
	assert.Contains(t, r.statusMessage, "Max 50 exceeded")
}

// TestLoadDiffSecurityNoFetch guards against the Diff key dispatching a
// kubectl-shaped GetResourceYAML for a synthetic security ResourceTypeEntry —
// the cmd must short-circuit with a clear error instead of producing a kubectl
// failure tagged with "fetching <name>".
func TestLoadDiffSecurityNoFetch(t *testing.T) {
	m := basePush80Model()
	rt := model.ResourceTypeEntry{Kind: "__security_falco__", APIGroup: "_security"}
	itemA := model.Item{Name: "pod/a", Kind: "__security_affected_resource__", Namespace: "default"}
	itemB := model.Item{Name: "pod/b", Kind: "__security_affected_resource__", Namespace: "default"}
	cmd := m.loadDiff(rt, itemA, itemB)
	require.NotNil(t, cmd)
	msg, ok := cmd().(diffLoadedMsg)
	require.True(t, ok)
	require.Error(t, msg.err)
	assert.Contains(t, msg.err.Error(), "__security_falco__", "error must identify the synthetic kind")
}

// --- handleExplorerActionKeyMonitoring: cycle Cluster <-> Monitoring ---

// dashboardTypesModel builds a resource-types view with a normal resource type
// followed by the two dashboard pseudo-items.
func dashboardTypesModel(cursor int) Model {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResourceTypes
	m.nav.ResourceType = model.ResourceTypeEntry{}
	m.middleItems = []model.Item{
		{Name: "Pods", Kind: "Pod"},
		{Name: "Cluster", Kind: "__overview__", Extra: "__overview__", Category: "Dashboards"},
		{Name: "Monitoring", Kind: "__monitoring__", Extra: "__monitoring__", Category: "Dashboards"},
	}
	m.setCursor(cursor)
	return m
}

func TestActionKeyMonitoringStartsWithCluster(t *testing.T) {
	m := dashboardTypesModel(0) // cursor on a normal resource type
	ret, _, handled := m.handleExplorerActionKeyMonitoring()
	assert.True(t, handled)
	rm := ret.(Model)
	sel := rm.selectedMiddleItem()
	require.NotNil(t, sel)
	assert.Equal(t, "__overview__", sel.Extra, "first press opens the Cluster dashboard")
}

func TestActionKeyMonitoringClusterToMonitoring(t *testing.T) {
	m := dashboardTypesModel(1) // cursor already on Cluster dashboard
	ret, _, handled := m.handleExplorerActionKeyMonitoring()
	assert.True(t, handled)
	rm := ret.(Model)
	sel := rm.selectedMiddleItem()
	require.NotNil(t, sel)
	assert.Equal(t, "__monitoring__", sel.Extra, "from Cluster the key switches to Monitoring")
}

func TestActionKeyMonitoringMonitoringBackToCluster(t *testing.T) {
	m := dashboardTypesModel(2) // cursor on Monitoring dashboard
	ret, _, handled := m.handleExplorerActionKeyMonitoring()
	assert.True(t, handled)
	rm := ret.(Model)
	sel := rm.selectedMiddleItem()
	require.NotNil(t, sel)
	assert.Equal(t, "__overview__", sel.Extra, "from Monitoring the key cycles back to Cluster")
}

func TestActionKeyMonitoringNeedsCluster(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelClusters
	ret, _, handled := m.handleExplorerActionKeyMonitoring()
	assert.True(t, handled)
	rm := ret.(Model)
	assert.Equal(t, "Select a cluster first", rm.statusMessage)
}
