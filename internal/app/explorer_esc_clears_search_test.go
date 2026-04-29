package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// After / + text + Enter, m.searchActive is false but searchInput.Value
// stays populated so highlights persist and n/N navigation still works.
// Esc must peel that state off so the user can dismiss the search
// highlights without leaving the current level.
func TestExplorerEscClearsSearchHighlights(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.Context = "test-ctx"
	m.searchInput.Value = "nginx"

	r, _ := m.handleExplorerEsc()
	rm := r.(Model)

	assert.Empty(t, rm.searchInput.Value,
		"Esc must clear the persisted search query (and its highlights)")
	assert.Equal(t, model.LevelResources, rm.nav.Level,
		"Esc must stay on the current level after clearing search")
}

// Esc cascade: selection → search → filter → fullscreen → navigate.
// Content state peels off before the viewport toggle so a user with
// search highlights AND fullscreen on doesn't have to exit fullscreen
// just to clear the highlights they were inspecting.
func TestExplorerEscClearsSearchBeforeExitingFullscreen(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.Context = "test-ctx"
	m.searchInput.Value = "nginx"
	m.fullscreenDashboard = true

	r, _ := m.handleExplorerEsc()
	rm := r.(Model)

	assert.Empty(t, rm.searchInput.Value, "first Esc clears search highlights")
	assert.True(t, rm.fullscreenDashboard,
		"first Esc must NOT exit fullscreen when there's content state to peel first")
}

// With no transient state to peel and no fullscreen to exit, Esc is a
// no-op on a non-cluster level. Esc is reserved for cancel / dismiss
// semantics; navigation back is via h/Left.
func TestExplorerEscIsNoOpWhenNoStateToClear(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.Context = "test-ctx"
	m.searchInput.Value = ""
	m.leftItems = []model.Item{{Name: "test-ctx"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "root"}}}

	r, _ := m.handleExplorerEsc()
	rm := r.(Model)

	assert.Equal(t, model.LevelResources, rm.nav.Level,
		"Esc must NOT walk back to parent — that's h/Left's job")
}
