package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// Issue: when the user has an active search highlight (m.searchInput.Value
// non-empty after `/foo` + Enter) and then navigates to a different level —
// either by opening a resource (Enter → child) or by going back to the
// parent (Esc-cascade past the search-clear step, or h/left) — the highlight
// must be cleared. Otherwise it bleeds onto the new level's items, which is
// confusing and not what the user asked for.
//
// The existing Esc cascade already clears the search as its own step
// (handleExplorerEsc), so a single Esc on a level with active search just
// clears the search and stays put — that's intentional. The bug is that
// programmatic navigation (Enter into a child, or any path that ends up in
// navigateChild / navigateParent) leaves searchInput.Value intact.

func TestNavigateChildClearsSearchHighlight(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{
		{Name: "nginx-pod", Kind: "Pod"},
	}
	m.searchInput.Value = "nginx"
	m.searchInput.Cursor = len(m.searchInput.Value)

	ret, _ := m.navigateChild()
	rm := ret.(Model)

	assert.Empty(t, rm.searchInput.Value,
		"navigateChild must clear searchInput so the highlight does not "+
			"bleed onto the child level")
}

func TestNavigateParentClearsSearchHighlight(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.searchInput.Value = "nginx"
	m.searchInput.Cursor = len(m.searchInput.Value)

	ret, _ := m.navigateParent()
	rm := ret.(Model)

	assert.Empty(t, rm.searchInput.Value,
		"navigateParent must clear searchInput so the highlight does not "+
			"bleed onto the parent level")
}
