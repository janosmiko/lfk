package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/scheduler"
)

// ctrlC builds the key message the explorer nav handler sees for Ctrl+C.
func ctrlC() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
}

// TestCtrlCOnOtherTabClosesTabInsteadOfCancelling covers the reported bug: a
// bulk mutation started on tab 0 swallowed Ctrl+C on every other tab, so the
// user could not close the tab they were actually looking at.
func TestCtrlCOnOtherTabClosesTabInsteadOfCancelling(t *testing.T) {
	m := threeTabModel(0)
	m.scheduler = scheduler.New(0)
	owner := m.currentTabUID()
	cancelled := false
	m.scheduler.StartCancellable(owner, scheduler.KindMutation, "Sync ArgoApps (12)", "prod", func() { cancelled = true })

	// Move to a different tab and press Ctrl+C.
	m.activeTab = 1
	ret, _, handled := m.handleExplorerNavKey(ctrlC())
	assert.True(t, handled, "ctrl+c must be handled")
	result := ret.(Model)

	assert.False(t, cancelled, "mutation owned by another tab must not be cancelled")
	assert.Len(t, result.tabs, 2, "ctrl+c on a foreign tab must close that tab")
}

// TestCtrlCOnOwningTabCancelsMutation keeps the original behaviour: on the tab
// that started the mutation, Ctrl+C cancels instead of closing the tab.
func TestCtrlCOnOwningTabCancelsMutation(t *testing.T) {
	m := threeTabModel(0)
	m.scheduler = scheduler.New(0)
	owner := m.currentTabUID()
	cancelled := false
	m.scheduler.StartCancellable(owner, scheduler.KindMutation, "Sync ArgoApps (12)", "prod", func() { cancelled = true })

	ret, _, handled := m.handleExplorerNavKey(ctrlC())
	assert.True(t, handled, "ctrl+c must be handled")
	result := ret.(Model)

	assert.True(t, cancelled, "mutation owned by the active tab must be cancelled")
	assert.Len(t, result.tabs, 3, "the owning tab must stay open")
}

// TestClosingOwningTabDisownsItsMutations covers the orphan case: tab UIDs are
// never reused, so a mutation left owned by a closed tab could never be
// cancelled again. Closing the tab hands its work back to every tab.
func TestClosingOwningTabDisownsItsMutations(t *testing.T) {
	m := threeTabModel(0)
	m.scheduler = scheduler.New(0)
	owner := m.currentTabUID()
	cancelled := false
	m.scheduler.StartCancellable(owner, scheduler.KindMutation, "Sync ArgoApps (12)", "prod", func() { cancelled = true })

	ret, _ := m.closeTabOrQuit()
	result := ret.(Model)
	require.Len(t, result.tabs, 2, "owning tab should be closed")

	// A surviving tab must still be able to cancel the orphaned mutation.
	assert.True(t, result.scheduler.HasActiveMutationsOwnedBy(result.currentTabUID()))
	result.scheduler.CancelMutationsOwnedBy(result.currentTabUID())
	assert.True(t, cancelled, "orphaned mutation must stay cancellable")
}

// TestTabUIDsAreUnique guards the identity the ownership check relies on:
// cloned tabs must not share the source tab's UID.
func TestTabUIDsAreUnique(t *testing.T) {
	m := threeTabModel(0)
	first := m.currentTabUID()
	clone := m.cloneCurrentTab()

	assert.NotZero(t, first, "active tab must have a UID")
	assert.NotZero(t, clone.uid, "cloned tab must get its own UID")
	assert.NotEqual(t, first, clone.uid, "cloned tab must not inherit the source tab's UID")
}
