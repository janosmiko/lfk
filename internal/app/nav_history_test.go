package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

var podsType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}

// navHistoryModel sits on the resource-types level with Pods under the cursor.
func navHistoryModel() Model {
	m := jumpHistoryModel()
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{podsType}
	m.nav = model.NavigationState{Level: model.LevelResourceTypes, Context: "test-ctx"}
	m.leftItems = []model.Item{{Name: "test-ctx"}}
	m.leftItemsHistory = nil
	m.allGroupsExpanded = true
	m.setMiddleItems([]model.Item{{Name: "Pods", Kind: "Pod", Extra: podsType.ResourceRef()}})
	m.setCursor(0)
	return m
}

func pressNavKey(t *testing.T, m Model, key string) Model {
	t.Helper()
	out, _ := m.Update(keyMsg(key))
	rm, ok := out.(Model)
	require.True(t, ok)
	return rm
}

func TestRegularNavigationJumpBackWalksStack(t *testing.T) {
	kb := ui.ActiveKeybindings
	m := navHistoryModel()

	m = pressNavKey(t, m, kb.Right) // types -> pods
	require.Equal(t, model.LevelResources, m.nav.Level)
	m.setMiddleItems([]model.Item{{Name: "pod-1", Namespace: "default", Kind: "Pod"}})
	m.setCursor(0)

	m = pressNavKey(t, m, kb.Right) // pods -> pod-1 containers
	require.Equal(t, model.LevelContainers, m.nav.Level)
	require.Len(t, m.jumpBackStack, 2)

	m = pressNavKey(t, m, kb.JumpBack)
	assert.Equal(t, model.LevelResources, m.nav.Level)
	m = pressNavKey(t, m, kb.JumpBack)
	assert.Equal(t, model.LevelResourceTypes, m.nav.Level)
	assert.Empty(t, m.jumpBackStack, "jump back must not record itself")
}

func TestParentNavigationIsRecorded(t *testing.T) {
	kb := ui.ActiveKeybindings
	m := jumpHistoryModel()
	m.setCursor(0)

	m = pressNavKey(t, m, kb.Left) // pods -> resource types
	require.Equal(t, model.LevelResourceTypes, m.nav.Level)
	require.Len(t, m.jumpBackStack, 1)

	m = pressNavKey(t, m, kb.JumpBack)
	assert.Equal(t, model.LevelResources, m.nav.Level)
}

func TestCursorMoveIsNotRecorded(t *testing.T) {
	m := jumpHistoryModel()
	m = pressNavKey(t, m, ui.ActiveKeybindings.Down)
	assert.Empty(t, m.jumpBackStack)
}

func TestTeleportKeyRecordsOriginOnce(t *testing.T) {
	m := jumpHistoryModel()
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Deployment", APIVersion: "apps/v1", Resource: "deployments"},
	}
	m.setMiddleItems([]model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default"}})

	out, _ := m.trackNavHistory(func(m Model) (tea.Model, tea.Cmd) {
		return m.navigateToOwner("Deployment", "my-deploy", "apps/v1")
	})
	assert.Len(t, out.(Model).jumpBackStack, 1, "teleport already pushed its origin")
}

func TestTabSwitchIsNotRecorded(t *testing.T) {
	m := jumpHistoryModel()
	m.tabs = []TabState{{}, {nav: model.NavigationState{Level: model.LevelResourceTypes, Context: "test-ctx"}}}
	m.activeTab = 0

	m = pressNavKey(t, m, ui.ActiveKeybindings.NextTab)
	require.Equal(t, 1, m.activeTab)
	assert.Empty(t, m.jumpBackStack, "the other tab's location must not leak into this tab's stack")
}

func TestWhichKeyJumpBackDoesNotRecordItself(t *testing.T) {
	m := navHistoryModel()
	out, _ := m.Update(keyMsg(ui.ActiveKeybindings.Right))
	m = out.(Model)
	require.Len(t, m.jumpBackStack, 1)

	out, _ = m.Update(keymapsChordMsg{keyMsg(ui.ActiveKeybindings.JumpBack)})
	m = out.(Model)
	assert.Equal(t, model.LevelResourceTypes, m.nav.Level)
	assert.Empty(t, m.jumpBackStack)
}
