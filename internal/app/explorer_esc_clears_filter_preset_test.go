package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// Applying a quick filter preset (e.g. "Failing pods" via .) sets
// m.activeFilterPreset and stashes the pre-filter list in
// m.unfilteredMiddleItems. Esc must peel that layer off the same way it
// already peels selection / search / filterText, so users don't have to
// reach for . a second time just to undo a preset.
func TestExplorerEscClearsActiveFilterPreset(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.Context = "test-ctx"
	full := []model.Item{
		{Name: "pod-a", Kind: "Pod"},
		{Name: "pod-b", Kind: "Pod"},
		{Name: "pod-c", Kind: "Pod"},
	}
	filtered := []model.Item{{Name: "pod-b", Kind: "Pod"}}
	m.setMiddleItems(filtered)
	m.unfilteredMiddleItems = full
	m.activeFilterPreset = &FilterPreset{Name: "Failing"}

	r, _ := m.handleExplorerEsc()
	rm := r.(Model)

	assert.Nil(t, rm.activeFilterPreset,
		"Esc must clear the active filter preset")
	assert.Nil(t, rm.unfilteredMiddleItems,
		"Esc must drop the saved unfiltered snapshot once the preset is cleared")
	assert.Equal(t, full, rm.middleItems,
		"Esc must restore the pre-preset middle items")
	assert.Contains(t, rm.statusMessage, "Failing",
		"Esc clearing a preset should announce which preset was dropped")
}

// Esc cascade: filterText is the most recently-typed state, so it peels
// before activeFilterPreset. With both layers present, the first Esc
// clears the typed filter, leaving the preset intact for a second Esc.
func TestExplorerEscClearsFilterTextBeforeFilterPreset(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.Context = "test-ctx"
	m.filterText = "nginx"
	m.activeFilterPreset = &FilterPreset{Name: "Failing"}
	m.unfilteredMiddleItems = []model.Item{{Name: "pod-a"}}

	r, _ := m.handleExplorerEsc()
	rm := r.(Model)

	assert.Empty(t, rm.filterText, "first Esc clears the typed filter")
	assert.NotNil(t, rm.activeFilterPreset,
		"first Esc must NOT clear the preset while typed filter is present")
}

// Esc cascade priority: filter preset must peel before fullscreen,
// matching the documented selection → search → filter → fullscreen
// order so a user inspecting failing pods in fullscreen doesn't have
// to leave fullscreen just to drop the preset.
func TestExplorerEscClearsFilterPresetBeforeExitingFullscreen(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.Context = "test-ctx"
	m.activeFilterPreset = &FilterPreset{Name: "Failing"}
	m.unfilteredMiddleItems = []model.Item{{Name: "pod-a"}}
	m.fullscreenDashboard = true

	r, _ := m.handleExplorerEsc()
	rm := r.(Model)

	assert.Nil(t, rm.activeFilterPreset, "first Esc clears the preset")
	assert.True(t, rm.fullscreenDashboard,
		"first Esc must NOT exit fullscreen when a preset is still in place")
}
