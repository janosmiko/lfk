package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
	"github.com/janosmiko/lfk/internal/ui"
)

// jumpHistoryModel returns a Model positioned at LevelResources in the
// "test-ctx" context, which the test client reports as a valid context so
// restore takes the happy path rather than the stale-fallback path.
func jumpHistoryModel() Model {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.Context = "test-ctx"
	m.leftItems = []model.Item{{Name: "Pods", Extra: "/v1/pods"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "test-ctx"}}}
	return m
}

func TestPushJumpHistoryCapturesState(t *testing.T) {
	m := jumpHistoryModel()
	m.setCursor(2)
	m.filterText = "web"
	m.expandedGroup = "Workloads"

	m.pushJumpHistory()

	require.Len(t, m.jumpBackStack, 1)
	snap := m.jumpBackStack[0]
	assert.Equal(t, model.LevelResources, snap.nav.Level)
	assert.Equal(t, "test-ctx", snap.nav.Context)
	assert.Equal(t, 2, snap.cursors[model.LevelResources])
	assert.Equal(t, "web", snap.filterText)
	assert.Equal(t, "Workloads", snap.expandedGroup)
	assert.Len(t, snap.middleItems, 3)
}

func TestJumpBackRestoresState(t *testing.T) {
	m := jumpHistoryModel()
	m.setCursor(2)
	m.filterText = "origin"

	m.pushJumpHistory()

	// Simulate a teleport mutating navigation away from the origin.
	m.nav.Level = model.LevelResourceTypes
	m.nav.ResourceName = "elsewhere"
	m.setCursor(0)
	m.filterText = ""
	m.setMiddleItems([]model.Item{{Name: "other"}})

	result, _ := m.jumpBack()
	rm := result.(Model)

	assert.Equal(t, model.LevelResources, rm.nav.Level)
	assert.Equal(t, "test-ctx", rm.nav.Context)
	assert.Equal(t, 2, rm.cursors[model.LevelResources])
	assert.Equal(t, "origin", rm.filterText)
	assert.Len(t, rm.middleItems, 3)
	assert.Empty(t, rm.jumpBackStack)
}

func TestTeleportThenJumpBackReturnsToOrigin(t *testing.T) {
	m := jumpHistoryModel()
	m.setCursor(1)
	// navigateToOwner calls pushJumpHistory before mutating nav state.
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Deployment", APIVersion: "apps/v1", Resource: "deployments"},
	}
	result, _ := m.navigateToOwner("Deployment", "my-deploy", "apps/v1")
	rm := result.(Model)
	require.Len(t, rm.jumpBackStack, 1, "navigateToOwner must record the origin")

	back, _ := rm.jumpBack()
	bm := back.(Model)
	assert.Equal(t, model.LevelResources, bm.nav.Level)
	assert.Equal(t, 1, bm.cursors[model.LevelResources])
}

func TestJumpBackEmptyStackIsNoOp(t *testing.T) {
	m := jumpHistoryModel()
	m.nav.Level = model.LevelOwned

	back, _ := m.jumpBack()
	bm := back.(Model)
	assert.Equal(t, model.LevelOwned, bm.nav.Level, "jumpBack on empty stack must not navigate")
	assert.Empty(t, bm.jumpBackStack)
}

func TestJumpHistoryStackCap(t *testing.T) {
	m := jumpHistoryModel()
	// Push one more than the cap; the oldest entry must be dropped.
	for i := range jumpHistoryCap + 10 {
		m.setCursor(i)
		m.pushJumpHistory()
	}
	assert.Len(t, m.jumpBackStack, jumpHistoryCap)
	// Oldest surviving entry should be cursor index 10, not 0.
	assert.Equal(t, 10, m.jumpBackStack[0].cursors[model.LevelResources])
}

func TestMultiLevelJumpHistory(t *testing.T) {
	m := jumpHistoryModel()
	m.setCursor(0)

	// First teleport: origin A -> B.
	m.pushJumpHistory()
	m.nav.Level = model.LevelResourceTypes
	m.setMiddleItems([]model.Item{{Name: "B"}})

	// Second teleport: B -> C.
	m.pushJumpHistory()
	m.nav.Level = model.LevelOwned
	m.setMiddleItems([]model.Item{{Name: "C"}})
	require.Len(t, m.jumpBackStack, 2)

	// First jumpBack: C -> B.
	back1, _ := m.jumpBack()
	m1 := back1.(Model)
	assert.Equal(t, model.LevelResourceTypes, m1.nav.Level)

	// Second jumpBack: B -> A.
	back2, _ := m1.jumpBack()
	m2 := back2.(Model)
	assert.Equal(t, model.LevelResources, m2.nav.Level)
	assert.Empty(t, m2.jumpBackStack)
}

func TestJumpBackStaleTargetGracefulFallback(t *testing.T) {
	m := jumpHistoryModel()
	m.setCursor(1)
	m.pushJumpHistory()
	// Corrupt the recorded snapshot's context so it no longer exists.
	m.jumpBackStack[0].nav.Context = "deleted-ctx"

	m.nav.Level = model.LevelResourceTypes
	// Live security/owned jump context from the pre-fallback view: the
	// fallback resets to the cluster picker, so none of it may survive.
	m.securityActiveGroup = "privileged"
	m.securityActiveSource = "heuristic"
	m.securityResourceFilter = []security.ResourceRef{{Namespace: "ns", Kind: "Pod", Name: "web"}}
	m.ownedParentStack = []ownedParentState{{resourceName: "api"}}

	result, _ := m.jumpBack()
	rm := result.(Model)
	assert.Equal(t, model.LevelClusters, rm.nav.Level,
		"stale snapshot context must fall back to the cluster picker, not crash")
	assert.Equal(t, "", rm.nav.Context)
	assert.True(t, rm.hasStatusMessage(), "stale fallback must surface a status message")
	assert.Empty(t, rm.securityActiveGroup, "fallback must clear the drilled finding group")
	assert.Empty(t, rm.securityActiveSource, "fallback must clear the drilled finding source")
	assert.Empty(t, rm.securityResourceFilter, "fallback must clear the per-resource findings filter")
	assert.Empty(t, rm.ownedParentStack, "fallback must clear the nested owned-drill ancestry")
}

func TestCaptureNavSnapshotDeepCopiesSlices(t *testing.T) {
	m := jumpHistoryModel()
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:           "Widget",
		Verbs:          []string{"get", "list"},
		PrinterColumns: []model.PrinterColumn{{Name: "Status", JSONPath: ".status.phase"}},
	}
	snap := m.captureNavSnapshot()

	// Mutating the live model must not corrupt the snapshot.
	m.middleItems[0].Name = "mutated"
	m.leftItems[0].Name = "mutated"
	m.nav.ResourceType.Verbs[0] = "mutated"
	m.nav.ResourceType.PrinterColumns[0].Name = "mutated"

	assert.NotEqual(t, "mutated", snap.middleItems[0].Name)
	assert.NotEqual(t, "mutated", snap.leftItems[0].Name)
	assert.Equal(t, "get", snap.nav.ResourceType.Verbs[0],
		"snapshot Verbs must not alias the live model")
	assert.Equal(t, "Status", snap.nav.ResourceType.PrinterColumns[0].Name,
		"snapshot PrinterColumns must not alias the live model")
}

func TestJumpHistoryKeybindingsHaveDefaults(t *testing.T) {
	kb := ui.DefaultKeybindings()
	assert.Equal(t, "backspace", kb.JumpBack)
}

// TestSaveLoadTabRoundTripsJumpStack verifies the per-tab jump-back history
// survives a saveCurrentTab/loadTab cycle and is decoupled from the Model.
func TestSaveLoadTabRoundTripsJumpStack(t *testing.T) {
	m := jumpHistoryModel()
	m.tabs = []TabState{{}}
	m.activeTab = 0
	m.setCursor(2)
	m.filterText = "origin"
	m.pushJumpHistory()
	require.Len(t, m.jumpBackStack, 1)

	m.saveCurrentTab()
	require.Len(t, m.tabs[0].jumpBackStack, 1)

	// Clear the live stack, then restore from the tab.
	m.jumpBackStack = nil
	m.loadTab(0)
	require.Len(t, m.jumpBackStack, 1)
	assert.Equal(t, "origin", m.jumpBackStack[0].filterText)

	// Mutating the restored stack must not corrupt the saved tab state.
	m.jumpBackStack = m.jumpBackStack[:0]
	assert.Len(t, m.tabs[0].jumpBackStack, 1)
}

// TestJumpHistoryIsIndependentPerTab verifies that a teleport+jumpBack in one
// tab does not reach into another tab's jump history.
func TestJumpHistoryIsIndependentPerTab(t *testing.T) {
	m := jumpHistoryModel()
	m.tabs = []TabState{{}, {}}
	m.activeTab = 0

	// Tab A: record a jump.
	m.setCursor(1)
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Deployment", APIVersion: "apps/v1", Resource: "deployments"},
	}
	resA, _ := m.navigateToOwner("Deployment", "deploy-a", "apps/v1")
	m = resA.(Model)
	require.Len(t, m.jumpBackStack, 1, "tab A must record its own jump")
	m.saveCurrentTab()

	// Switch to tab B — it must start with an empty jump history.
	m.loadTab(1)
	assert.Empty(t, m.jumpBackStack, "tab B must not see tab A's jump history")

	// A jumpBack in tab B is a no-op and cannot reach tab A's origin.
	resB, _ := m.jumpBack()
	m = resB.(Model)
	assert.Empty(t, m.jumpBackStack)

	// Switch back to tab A — its jump history is intact.
	m.saveCurrentTab()
	m.loadTab(0)
	require.Len(t, m.jumpBackStack, 1, "tab A's jump history must survive the round-trip")
}

// TestJumpBackKeyIgnoredDuringTextEntry verifies that the JumpBack key
// (Backspace) is left unhandled by handleExplorerJumpKey while a filter or
// search query is being edited, so the input handlers receive it instead.
func TestJumpBackKeyIgnoredDuringTextEntry(t *testing.T) {
	// The non-editing case reaches jumpBack(), which persists session state;
	// isolate the state dir so it doesn't leak into other tests.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	kb := ui.ActiveKeybindings
	jumpBackMsg := keyPressText(kb.JumpBack)
	if kb.JumpBack == "backspace" {
		jumpBackMsg = tea.KeyPressMsg{Code: tea.KeyBackspace}
	}

	t.Run("not handled while filter active", func(t *testing.T) {
		m := jumpHistoryModel()
		m.pushJumpHistory()
		m.filterActive = true
		_, _, handled := m.handleExplorerJumpKey(jumpBackMsg)
		assert.False(t, handled,
			"JumpBack must be left for the filter input handler while editing")
	})

	t.Run("not handled while search active", func(t *testing.T) {
		m := jumpHistoryModel()
		m.pushJumpHistory()
		m.searchActive = true
		_, _, handled := m.handleExplorerJumpKey(jumpBackMsg)
		assert.False(t, handled,
			"JumpBack must be left for the search input handler while editing")
	})

	t.Run("handled when not editing", func(t *testing.T) {
		m := jumpHistoryModel()
		m.pushJumpHistory()
		_, _, handled := m.handleExplorerJumpKey(jumpBackMsg)
		assert.True(t, handled,
			"JumpBack must trigger jump-back when no text entry is active")
	})
}
