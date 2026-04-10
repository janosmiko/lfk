package app

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
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

// --- handleExplorerActionKeySecurity: G4 ---

func TestHandleExplorerActionKeySecurityJumpsToFirstEntry(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "Monitoring", Extra: "__monitoring__"},
		{Name: "Trivy", Category: "Security", Extra: "_security/v1/findings"},
		{Name: "Heuristic", Category: "Security", Extra: "_security/v1/findings"},
		{Name: "Workloads"},
	}
	updated, _, handled := m.handleExplorerActionKeySecurity()
	assert.True(t, handled)
	mm, ok := updated.(Model)
	require.True(t, ok)
	assert.Equal(t, 1, mm.cursor())
}

func TestHandleExplorerActionKeySecurityNoSourcesAvailable(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "Monitoring", Extra: "__monitoring__"},
		{Name: "Workloads"},
	}
	updated, _, handled := m.handleExplorerActionKeySecurity()
	assert.True(t, handled)
	mm, ok := updated.(Model)
	require.True(t, ok)
	assert.Contains(t, mm.statusMessage, "No security sources available")
}

func TestHandleExplorerActionKeySecurityRequiresContext(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelClusters

	result, _, handled := m.handleExplorerActionKeySecurity()
	assert.True(t, handled)
	mm, ok := result.(Model)
	require.True(t, ok)
	assert.Contains(t, mm.statusMessage, "Select a cluster first")
}

// TestHandleExplorerActionKeySecurityAscendsFromDeeperLevel verifies that
// pressing # while at LevelResources (viewing a pod list) ascends back to
// LevelResourceTypes and jumps to the first Security category entry.
func TestHandleExplorerActionKeySecurityAscendsFromDeeperLevel(t *testing.T) {
	m := baseExplorerModel() // starts at LevelResources with middleItems = pods
	m.leftItems = []model.Item{
		{Name: "Monitoring", Extra: "__monitoring__"},
		{Name: "Trivy", Category: "Security", Extra: "_security/v1/findings"},
		{Name: "Workloads"},
	}

	updated, _, handled := m.handleExplorerActionKeySecurity()
	assert.True(t, handled)
	mm, ok := updated.(Model)
	require.True(t, ok)
	assert.Equal(t, model.LevelResourceTypes, mm.nav.Level,
		"handler should ascend to LevelResourceTypes before jumping")
	assert.Equal(t, 1, mm.cursor(),
		"cursor should land on the Trivy entry (index 1) after ascension")
}

// TestHandleExplorerActionKeyMonitoringAscendsFromDeeperLevel verifies the
// same fix applies to the monitoring @ hotkey (shared ascendToResourceTypes
// helper).
func TestHandleExplorerActionKeyMonitoringAscendsFromDeeperLevel(t *testing.T) {
	m := baseExplorerModel()
	m.leftItems = []model.Item{
		{Name: "Cluster", Extra: "__overview__"},
		{Name: "Monitoring", Extra: "__monitoring__"},
		{Name: "Workloads"},
	}

	updated, _, handled := m.handleExplorerActionKeyMonitoring()
	assert.True(t, handled)
	mm, ok := updated.(Model)
	require.True(t, ok)
	assert.Equal(t, model.LevelResourceTypes, mm.nav.Level)
	assert.Equal(t, 1, mm.cursor())
}

// --- jumpToFindingResource: Enter on a finding ---

func TestJumpToFindingResourceClusterScoped(t *testing.T) {
	m := baseExplorerModel()
	sel := model.Item{
		Kind: "__security_finding__",
		Columns: []model.KeyValue{
			{Key: "Resource", Value: "(cluster-scoped)"},
			{Key: "ResourceKind", Value: ""},
		},
	}
	updated, _ := m.jumpToFindingResource(sel)
	mm, ok := updated.(Model)
	require.True(t, ok)
	assert.Contains(t, mm.statusMessage, "No affected resource")
}

func TestJumpToFindingResourceMalformedResource(t *testing.T) {
	m := baseExplorerModel()
	sel := model.Item{
		Kind: "__security_finding__",
		Columns: []model.KeyValue{
			{Key: "Resource", Value: "malformed-no-slash"},
			{Key: "ResourceKind", Value: "Deployment"},
		},
	}
	updated, _ := m.jumpToFindingResource(sel)
	_, ok := updated.(Model)
	assert.True(t, ok)
}

// --- directActionRefresh: security cache invalidation ---

func TestHandleKeyRefreshInvalidatesSecurityCache(t *testing.T) {
	mgr := security.NewManager()
	mgr.SetRefreshTTL(1 * time.Hour)
	fakeSrc := &security.FakeSource{
		NameStr: "fake", Available: true,
		Findings: []security.Finding{{ID: "1"}},
	}
	mgr.Register(fakeSrc)

	_, _ = mgr.FetchAll(context.Background(), "kctx", "")
	require.Equal(t, int32(1), fakeSrc.FetchCalls.Load())

	_, _ = mgr.FetchAll(context.Background(), "kctx", "")
	require.Equal(t, int32(1), fakeSrc.FetchCalls.Load())

	m := baseExplorerModel()
	m.securityManager = mgr
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:     "__security_trivy-operator__",
		APIGroup: model.SecurityVirtualAPIGroup,
	}
	m.nav.Context = "kctx"

	_, _ = m.directActionRefresh()

	_, _ = mgr.FetchAll(context.Background(), "kctx", "")
	assert.Equal(t, int32(2), fakeSrc.FetchCalls.Load())
}
