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

func TestHandleFilterKeyEnterResetsBroadMode(t *testing.T) {
	m := baseModelCov()
	m.filterActive = true
	m.filterBroadMode = true
	r, _ := m.handleFilterKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := r.(Model)
	assert.False(t, rm.filterBroadMode, "Enter must reset broad mode for the next session")
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

func TestHandleSearchKeyEnterResetsBroadMode(t *testing.T) {
	m := baseModelCov()
	m.searchActive = true
	m.searchBroadMode = true
	r, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := r.(Model)
	assert.False(t, rm.searchBroadMode)
}

func TestHandleSearchKeyEscResetsBroadMode(t *testing.T) {
	m := baseModelCov()
	m.searchActive = true
	m.searchBroadMode = true
	r, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEsc})
	rm := r.(Model)
	assert.False(t, rm.searchBroadMode)
}
