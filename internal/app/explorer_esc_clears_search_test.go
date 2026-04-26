package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// After / + text + Enter, m.searchActive is false but searchInput.Value
// stays populated so highlights persist and n/N navigation still works.
// Esc must peel that state off as a distinct step before falling
// through to the navigate-parent default — otherwise Esc jumps the
// user out of the current level when they only meant to dismiss the
// search highlights.
func TestExplorerEscClearsSearchHighlightsBeforeNavigatingParent(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources // not at root → parent navigation possible
	m.nav.Context = "test-ctx"
	m.searchInput.Value = "nginx"

	r, _ := m.handleExplorerEsc()
	rm := r.(Model)

	assert.Empty(t, rm.searchInput.Value,
		"first Esc must clear the persisted search query (and its highlights)")
	assert.Equal(t, model.LevelResources, rm.nav.Level,
		"first Esc must NOT navigate to parent when there's a search to clear first")
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

// Once highlights are gone, a second Esc falls through to the existing
// navigate-parent behavior — preserves the cascade so users can still
// back out of nested levels with repeated Esc presses.
func TestExplorerEscNavigatesParentWhenNoSearchHighlights(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.Context = "test-ctx"
	m.searchInput.Value = "" // no search to clear
	m.leftItems = []model.Item{{Name: "test-ctx"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "root"}}}

	r, _ := m.handleExplorerEsc()
	rm := r.(Model)

	assert.NotEqual(t, model.LevelResources, rm.nav.Level,
		"with no search highlights, Esc must navigate to parent")
}
