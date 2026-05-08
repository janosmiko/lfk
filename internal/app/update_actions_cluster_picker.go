// update_actions_cluster_picker carries the action-menu helpers used
// when the user is at LevelClusters. The branch lives in its own file
// so update_actions.go stays under the 800-line cap and the cluster-
// picker code path is easier to find without scrolling through the
// kind-based dispatcher.
package app

import (
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// openClusterPickerActionMenu builds the action menu shown at
// LevelClusters. Item.Status carries the live keybinding so the
// renderer's "[L] Set color - ..." hint stays in sync with user
// rebindings.
func (m Model) openClusterPickerActionMenu() Model {
	if m.selectedMiddleItem() == nil {
		return m
	}
	actions := model.ActionsForClusterPicker(model.ClusterPickerKeys{
		SetColor: ui.ActiveKeybindings.ClusterColorPicker,
	})
	items := make([]model.Item, 0, len(actions))
	for _, a := range actions {
		items = append(items, model.Item{Name: a.Label, Status: a.Key, Extra: a.Description})
	}
	m.bulkMode = false
	m.overlay = overlayAction
	m.overlayItems = items
	m.overlayCursor = 0
	return m
}
