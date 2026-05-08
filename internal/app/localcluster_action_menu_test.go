package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// TestActionsForClusterPicker_IncludesManageLocal verifies the
// cluster-picker action constructor surfaces a "Manage local
// clusters..." entry alongside the existing Set color action so users
// can discover the manager overlay without knowing the Ctrl+N shortcut.
func TestActionsForClusterPicker_IncludesManageLocal(t *testing.T) {
	got := model.ActionsForClusterPicker(model.ClusterPickerKeys{SetColor: "L"})
	found := false
	for _, a := range got {
		if a.Label == "Local clusters" {
			found = true
			break
		}
	}
	assert.True(t, found, "missing 'Local clusters' in cluster-picker actions: %+v", got)
}

// TestOpenActionMenu_AtClusterPicker_ListsManageLocalClusters asserts
// the action overlay opened at LevelClusters now includes the new
// menu entry alongside Set color.
func TestOpenActionMenu_AtClusterPicker_ListsManageLocalClusters(t *testing.T) {
	m := newClusterPickerModel(t)
	result := m.openActionMenu()
	assert.Equal(t, overlayAction, result.overlay,
		"openActionMenu at the cluster picker must surface the action overlay")

	labels := make([]string, 0, len(result.overlayItems))
	for _, item := range result.overlayItems {
		labels = append(labels, item.Name)
	}
	assert.Contains(t, labels, "Local clusters",
		"the cluster-picker action menu must list 'Local clusters'")
	assert.Contains(t, labels, "Set color",
		"the existing Set color entry must remain present")
}

// TestExecuteAction_ManageLocalClustersOpensOverlay simulates the
// user hitting Enter on the new menu entry: the action handler must
// hand off to the local-cluster manager overlay (the same target as
// the Ctrl+N shortcut).
func TestExecuteAction_ManageLocalClustersOpensOverlay(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := newClusterPickerModel(t)
	m.overlay = overlayAction
	ret, cmd := m.executeAction("Local clusters")
	result := ret.(Model)
	assert.Equal(t, overlayLocalClusters, result.overlay,
		"selecting Local clusters from the action menu must hand off to the manager overlay")
	require.NotNil(t, cmd,
		"opening the manager must dispatch a Detect command so the cluster table loads")
	assert.Equal(t, localClusterScreenList, result.localClusterState.screen,
		"the manager opens on the list screen by default")
}

// TestActionsForClusterPicker_ManageLocalCarriesMenuKey verifies the
// menu entry advertises a single-letter in-menu shortcut (so the
// chip stays one-column-wide and the keypress is reachable inside
// the menu — chords like ctrl+n can't be the in-menu activator).
func TestActionsForClusterPicker_ManageLocalCarriesMenuKey(t *testing.T) {
	got := model.ActionsForClusterPicker(model.ClusterPickerKeys{SetColor: "L"})
	for _, a := range got {
		if a.Label == "Local clusters" {
			assert.Equal(t, "n", a.Key,
				"Local clusters entry must use the in-menu single-letter shortcut")
			return
		}
	}
	t.Fatal("Local clusters entry not found")
}

// TestOpenActionMenu_AtClusterPicker_ManageLocalChipIsSingleLetter
// checks the rendered menu item shows just `[n]` in the chip — not
// `[ctrl+n]` — so the row doesn't wrap. The global ctrl+n binding is
// independent of this chip.
func TestOpenActionMenu_AtClusterPicker_ManageLocalChipIsSingleLetter(t *testing.T) {
	m := newClusterPickerModel(t)
	result := m.openActionMenu()
	for _, item := range result.overlayItems {
		if item.Name == "Local clusters" {
			assert.Equal(t, "n", item.Status,
				"chip must be a single letter; chord activators wrap the action row")
			return
		}
	}
	t.Fatal("Local clusters entry not found in the rendered overlay")
}
