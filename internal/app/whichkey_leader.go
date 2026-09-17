package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/ui"
)

// whichKeyState holds the g-prefix goto popup's state.
type whichKeyState struct {
	shown  bool // delay elapsed, panel drawn
	scroll int
	// cells caches one render's entries. Only primeWhichKeyCells writes it,
	// from renderView's throwaway Model copy, so nothing persists into a tab
	// snapshot or the session.
	cells []whichKeyCell
}

// scrollWhichKey moves the panel viewport by half a page. Re-clamps against
// the CURRENT maxScroll so a resize while scrolled to the bottom can't leave
// a stale offset behind.
func (m Model) scrollWhichKey(cells []whichKeyCell, down bool) (Model, bool) {
	lay, ok := m.whichKeyLayoutFor(cells)
	if !ok || lay.maxScroll == 0 {
		return m, false
	}
	cur := min(max(m.whichKey.scroll, 0), lay.maxScroll)
	step := max(lay.viewRows/2, 1)
	if down {
		m.whichKey.scroll = min(cur+step, lay.maxScroll)
	} else {
		m.whichKey.scroll = max(cur-step, 0)
	}
	return m, true
}

// handleWhichKeyScrollKey consumes the half-page scroll keys. Callers MUST
// gate this on whichKey.shown.
func (m Model) handleWhichKeyScrollKey(msg tea.KeyPressMsg, cells []whichKeyCell) (Model, bool) {
	kb := ui.ActiveKeybindings
	switch key := msg.String(); {
	case kb.PageDown != "" && key == kb.PageDown:
		return m.scrollWhichKey(cells, true)
	case kb.PageUp != "" && key == kb.PageUp:
		return m.scrollWhichKey(cells, false)
	}
	return m, false
}

// primeWhichKeyCells fills the one-frame cell cache when the g-prefix goto
// popup is actually on screen. Call once per render, ahead of anything that
// reads it.
func (m Model) primeWhichKeyCells() Model {
	if !ui.ConfigWhichKeyEnabled || !m.whichKey.shown {
		return m
	}
	if m.pendingG {
		m.whichKey.cells = m.whichKeyCells()
	}
	return m
}

// frameWhichKeyCells returns the frame cache when primeWhichKeyCells filled it,
// and builds via build otherwise — so every path outside a render (key
// handlers, tests) behaves exactly as before.
func (m Model) frameWhichKeyCells(build func() []whichKeyCell) []whichKeyCell {
	if m.whichKey.cells != nil {
		return m.whichKey.cells
	}
	return build()
}
