package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestUpdateContextsLoaded_AnnotatesPerContextColor(t *testing.T) {
	// updateContextsLoaded must walk the freshly loaded context list and
	// stamp each Item.ClusterColor from m.clusterColors so the cluster-
	// picker renderer can paint per-row swatches without taking the colors
	// map as a render argument.
	withClusterColorsStateDir(t)
	m := Model{
		nav:               model.NavigationState{Level: model.LevelClusters},
		tabs:              []TabState{{}},
		itemCache:         map[string][]model.Item{},
		cacheFingerprints: map[string]string{},
		clusterColors: map[string]string{
			"prod-eu":   "red",
			"dev-local": "green",
		},
		width: 80, height: 40,
	}
	msg := contextsLoadedMsg{
		items: []model.Item{
			{Name: "prod-eu"},
			{Name: "dev-local"},
			{Name: "scratch"},
		},
	}
	ret, _ := m.updateContextsLoaded(msg)
	result := ret.(Model)

	assert.Equal(t, "red", result.middleItems[0].ClusterColor, "prod-eu must inherit its red assignment")
	assert.Equal(t, "green", result.middleItems[1].ClusterColor, "dev-local must inherit its green assignment")
	assert.Equal(t, "", result.middleItems[2].ClusterColor, "scratch (no entry in clusterColors) must have empty ClusterColor")
}

func TestOpenActionMenu_AtClusterPickerListsSetColor(t *testing.T) {
	m := newClusterPickerModel(t)
	result := m.openActionMenu()
	assert.Equal(t, overlayAction, result.overlay, "x at Level=Clusters opens the action menu")
	require.GreaterOrEqual(t, len(result.overlayItems), 1, "the action menu must list at least the Set color entry")

	// Assert ordering, not just existence: Set color must be at index 0
	// so the cluster-picker action menu opens with the most-common
	// pick-cluster-color action selected. Searching the slice would
	// silently let a future regression demote it.
	assert.Equal(t, "Set color", result.overlayItems[0].Name,
		"Set color must remain the first entry in the cluster-picker action menu")
	assert.Equal(t, ui.ActiveKeybindings.ClusterColorPicker, result.overlayItems[0].Status,
		"Status carries the keybinding so the action menu renders [L] Set color and the in-menu shortcut matches the global one")
}

func TestExecuteAction_SetColorAtClusterPickerOpensColorOverlay(t *testing.T) {
	m := newClusterPickerModel(t)
	m.overlay = overlayAction
	ret, _ := m.executeAction("Set color")
	result := ret.(Model)
	assert.Equal(t, overlayClusterColor, result.overlay,
		"selecting Set color from the action menu hands off to the color picker overlay")
	assert.Equal(t, "prod-eu", result.clusterColorOverlayContext)
}

func TestApplyClusterColorSelection_RefreshesMiddleItemsRow(t *testing.T) {
	// Saving a color from the overlay must immediately stamp the row in
	// m.middleItems so the next render shows the swatch — without waiting
	// for the next loadContexts roundtrip. Mirrors the index-based update
	// in handleKeyReadOnlyToggle so a transient filtered-slice pointer
	// can't leave the row stale.
	withClusterColorsStateDir(t)
	m := Model{
		nav: model.NavigationState{Level: model.LevelClusters},
		middleItems: []model.Item{
			{Name: "prod-eu"},
			{Name: "dev-local"},
		},
		cursors:                    [5]int{0, 0, 0, 0, 0},
		tabs:                       []TabState{{}},
		itemCache:                  map[string][]model.Item{},
		cacheFingerprints:          map[string]string{},
		clusterColors:              map[string]string{},
		clusterColorOverlayContext: "prod-eu",
		clusterColorOverlayCursor:  0, // "red"
		overlay:                    overlayClusterColor,
	}
	result, _ := m.applyClusterColorSelection()

	assert.Equal(t, "red", result.middleItems[0].ClusterColor, "row in m.middleItems must reflect the new color immediately")
	assert.Equal(t, "", result.middleItems[1].ClusterColor, "non-targeted rows must be left untouched")
}
