package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// renderOverlayContentTail is the last hop in the overlay render chain.
// renderOverlayContentExtended sits at the gocyclo ceiling, so new overlays
// render from here rather than growing its switch past the limit.
func (m Model) renderOverlayContentTail() (string, int, int, bool) {
	if m.overlay == overlayUndeliverable {
		c, w, h := m.renderUndeliverableOverlay()
		return c, w, h, true
	}
	return "", 0, 0, false
}

// overlayHintBarUndeliverable lists the keys that do something right now.
// Filter-input mode drops the navigation hints: those keys are being typed
// into the query, not dispatched.
func (m Model) overlayHintBarUndeliverable() string {
	if m.undeliverable.filterActive {
		return m.renderHints([]ui.HintEntry{
			{Key: "type", Desc: "filter"},
			{Key: "enter", Desc: "apply"},
			{Key: "esc", Desc: "clear"},
		})
	}
	return m.renderHints([]ui.HintEntry{
		{Key: "j/k", Desc: "move"},
		{Key: "g/G", Desc: "top/bottom"},
		{Key: "ctrl+d/u", Desc: "half page"},
		{Key: "ctrl+f/b", Desc: "page"},
		{Key: "/", Desc: "filter"},
		{Key: "enter", Desc: "jump"},
		{Key: "R", Desc: "refresh"},
		{Key: "q/esc", Desc: "close"},
	})
}

func (m Model) renderUndeliverableOverlay() (string, int, int) {
	partial := ""
	if m.undeliverable.partial != nil {
		partial = m.undeliverable.partial.Error()
	}
	w := m.undeliverableOverlayW()
	h := m.undeliverableOverlayH()
	body := ui.RenderUndeliverableOverlay(
		m.undeliverable.visibleRows(),
		m.undeliverable.cursor, m.undeliverable.scroll,
		w, h,
		m.undeliverable.filter.Value, m.undeliverable.filterActive,
		m.undeliverable.loading, partial,
	)
	return body, w, h
}
