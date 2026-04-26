package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// Default search/filter scope is name + namespace + category. Tab inside
// the / or f input toggles "broad" mode that also scans every visible
// column value (annotations, labels, finalizers, CRD printer columns,
// custom user columns) — internal-prefix columns stay excluded.

// --- searchMatchesItem with broad mode ---

func TestSearchMatchesItem_BroadModeMatchesColumns(t *testing.T) {
	m := Model{nav: model.NavigationState{Level: model.LevelResources}}
	item := model.Item{
		Name: "boring-pod",
		Columns: []model.KeyValue{
			{Key: "Annotations", Value: "owner=alice team=platform"},
			{Key: "Node", Value: "worker-12"},
		},
	}

	// Default: name-only — annotation value should NOT match.
	assert.False(t, m.searchMatchesItem(item, []string{"alice"}),
		"default scope must not reach annotations")

	// Broad mode: annotation value should match.
	m.searchBroadMode = true
	assert.True(t, m.searchMatchesItem(item, []string{"alice"}),
		"broad mode must scan column values")
	assert.True(t, m.searchMatchesItem(item, []string{"worker-12"}),
		"broad mode covers Node column too")
}

func TestSearchMatchesItem_BroadModeSkipsInternalColumns(t *testing.T) {
	m := Model{
		nav:             model.NavigationState{Level: model.LevelResources},
		searchBroadMode: true,
	}
	item := model.Item{
		Name: "boring-pod",
		Columns: []model.KeyValue{
			{Key: "__internal", Value: "secret-marker"},
			{Key: "secret:db-password", Value: "secret-marker"},
			{Key: "data:body", Value: "secret-marker"},
			{Key: "owner:Deployment", Value: "secret-marker"},
			{Key: "condition:Ready", Value: "secret-marker"},
			{Key: "step:1", Value: "secret-marker"},
			{Key: "cond:Available", Value: "secret-marker"},
		},
	}

	assert.False(t, m.searchMatchesItem(item, []string{"secret-marker"}),
		"internal-prefix columns must remain excluded even in broad mode")
}

// --- visibleMiddleItems with broad-mode filter ---

func TestVisibleMiddleItems_BroadModeMatchesColumns(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "alpha", Columns: []model.KeyValue{{Key: "Labels", Value: "tier=frontend"}}},
			{Name: "beta", Columns: []model.KeyValue{{Key: "Labels", Value: "tier=backend"}}},
		},
		filterText:        "frontend",
		allGroupsExpanded: true,
	}

	// Default: name-only — "frontend" matches neither name.
	got := m.visibleMiddleItems()
	assert.Empty(t, got, "default filter must not match labels")

	// Broad mode: alpha's Labels column matches.
	m.filterBroadMode = true
	got = m.visibleMiddleItems()
	if assert.Len(t, got, 1) {
		assert.Equal(t, "alpha", got[0].Name)
	}
}

// --- Tab toggle in filter input ---

func TestHandleFilterKeyTabTogglesBroadMode(t *testing.T) {
	m := Model{filterActive: true}
	r, _ := m.handleFilterKey(tea.KeyMsg{Type: tea.KeyTab})
	rm := r.(Model)
	assert.True(t, rm.filterBroadMode, "Tab inside filter must enter broad mode")

	// Toggle back.
	r2, _ := rm.handleFilterKey(tea.KeyMsg{Type: tea.KeyTab})
	rm2 := r2.(Model)
	assert.False(t, rm2.filterBroadMode, "second Tab must return to name-only")
}

func TestHandleFilterKeyEnterPreservesBroadMode(t *testing.T) {
	// Enter commits the filter — the mode must persist so the visible
	// list keeps matching column values after the input closes.
	// Otherwise typing "frontend" with Tab on, then Enter, would silently
	// drop back to name-only and the list would empty out.
	m := baseModelCov()
	m.filterActive = true
	m.filterBroadMode = true
	r, _ := m.handleFilterKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := r.(Model)
	assert.True(t, rm.filterBroadMode,
		"Enter must keep broad mode on so the applied filterText keeps matching column values")
}

func TestHandleKeyFilterStartsInNameOnly(t *testing.T) {
	// Opening a fresh / or f input starts in name-only — broad mode does
	// not leak across input sessions.
	m := baseModelCov()
	m.filterBroadMode = true // simulates leftover from a previous filter
	rm := m.handleKeyFilter()
	assert.False(t, rm.filterBroadMode,
		"opening a new filter must reset broad mode to default name-only")
}

func TestHandleFilterKeyEscResetsBroadMode(t *testing.T) {
	m := baseModelCov()
	m.filterActive = true
	m.filterBroadMode = true
	r, _ := m.handleFilterKey(tea.KeyMsg{Type: tea.KeyEsc})
	rm := r.(Model)
	assert.False(t, rm.filterBroadMode, "Esc must reset broad mode for the next session")
}

// --- Tab toggle in search input ---

func TestHandleSearchKeyTabTogglesBroadMode(t *testing.T) {
	m := Model{searchActive: true}
	r, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyTab})
	rm := r.(Model)
	assert.True(t, rm.searchBroadMode, "Tab inside search must enter broad mode")

	r2, _ := rm.handleSearchKey(tea.KeyMsg{Type: tea.KeyTab})
	rm2 := r2.(Model)
	assert.False(t, rm2.searchBroadMode)
}

// Confirming a search must NOT clear searchInput.Value — the highlight
// query in viewExplorer reads from there, and n/N navigation re-runs
// jumpToSearchMatch which also depends on it. Past iterations
// effectively cleared highlights on Enter by gating
// ActiveHighlightQuery on m.searchActive; the value is now derived
// from the input value so it survives commit.
func TestHandleSearchKeyEnterPreservesQueryForHighlight(t *testing.T) {
	m := baseModelCov()
	m.searchActive = true
	m.searchInput.Value = "nginx"
	r, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := r.(Model)
	assert.False(t, rm.searchActive, "Enter exits the input")
	assert.Equal(t, "nginx", rm.searchInput.Value,
		"Enter must keep searchInput.Value so highlights and n/N stay armed")
}

func TestHandleSearchKeyEnterPreservesBroadMode(t *testing.T) {
	// User reproduction: search with Tab on, press Enter, then n/N.
	// jumpToSearchMatch reads m.searchBroadMode, so resetting on Enter
	// breaks next/previous-match navigation for the just-confirmed
	// query. Mode must persist past Enter; only Esc clears.
	m := baseModelCov()
	m.searchActive = true
	m.searchBroadMode = true
	r, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := r.(Model)
	assert.True(t, rm.searchBroadMode,
		"Enter must keep broad mode on so n/N keep matching column values")
}

func TestHandleKeySearchStartsInNameOnly(t *testing.T) {
	m := baseModelCov()
	m.searchBroadMode = true // simulates leftover from a previous search
	rm := m.handleKeySearch()
	assert.False(t, rm.searchBroadMode,
		"opening a new search must reset broad mode to default name-only")
}

func TestHandleSearchKeyEscResetsBroadMode(t *testing.T) {
	m := baseModelCov()
	m.searchActive = true
	m.searchBroadMode = true
	r, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEsc})
	rm := r.(Model)
	assert.False(t, rm.searchBroadMode)
}
