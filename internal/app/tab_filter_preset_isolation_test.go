package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// A quick filter preset applied on one tab must stay on that tab. Before the
// fix activeFilterPreset / unfilteredMiddleItems were loose Model fields not
// captured per-tab, so switching to a sibling tab left the preset in place and
// the next data load re-applied it (reapplyFilterPreset), silently hiding rows
// that don't match — e.g. every Job vanishing under a "Failing pods" preset.
func TestTabSwitchDoesNotLeakActiveFilterPreset(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.Context = "ctx"
	// Two tabs: tab 0 is the current (Pods) tab, tab 1 is an untouched sibling.
	m.tabs = []TabState{{}, {}}
	m.activeTab = 0

	full := []model.Item{
		{Name: "pod-a", Kind: "Pod"},
		{Name: "pod-b", Kind: "Pod"},
	}
	filtered := []model.Item{{Name: "pod-b", Kind: "Pod"}}
	m.setMiddleItems(filtered)
	m.unfilteredMiddleItems = full
	m.activeFilterPreset = &FilterPreset{Name: "Failing"}

	// Switch to the sibling tab, which never had a preset applied.
	m.saveCurrentTab()
	m.loadTab(1)

	assert.Nil(t, m.activeFilterPreset,
		"switching to a tab with no preset must clear the active preset")
	assert.Nil(t, m.unfilteredMiddleItems,
		"switching tabs must drop the previous tab's unfiltered snapshot")
}

// The preset must survive a round-trip: switching away and back restores the
// tab's own active preset and its pre-filter snapshot.
func TestTabSwitchRestoresActiveFilterPreset(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.Context = "ctx"
	m.tabs = []TabState{{}, {}}
	m.activeTab = 0

	full := []model.Item{
		{Name: "pod-a", Kind: "Pod"},
		{Name: "pod-b", Kind: "Pod"},
	}
	filtered := []model.Item{{Name: "pod-b", Kind: "Pod"}}
	m.setMiddleItems(filtered)
	m.unfilteredMiddleItems = full
	m.activeFilterPreset = &FilterPreset{Name: "Failing"}

	m.saveCurrentTab()
	m.loadTab(1)
	// Back to tab 0.
	m.saveCurrentTab()
	m.loadTab(0)

	if assert.NotNil(t, m.activeFilterPreset,
		"returning to a tab with a preset must restore it") {
		assert.Equal(t, "Failing", m.activeFilterPreset.Name)
	}
	assert.Equal(t, full, m.unfilteredMiddleItems,
		"returning to a filtered tab must restore its unfiltered snapshot")
}

// A newly opened tab starts with no active preset even when cloned from a tab
// that has one applied.
func TestCloneCurrentTabStartsWithNoFilterPreset(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.Context = "ctx"
	m.activeFilterPreset = &FilterPreset{Name: "Failing"}
	m.unfilteredMiddleItems = []model.Item{{Name: "pod-a", Kind: "Pod"}}

	clone := m.cloneCurrentTab()

	assert.Nil(t, clone.activeFilterPreset,
		"a cloned tab must start with no active filter preset")
	assert.Nil(t, clone.unfilteredMiddleItems,
		"a cloned tab must start with no unfiltered snapshot")
}
