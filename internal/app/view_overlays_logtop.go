package app

import (
	"github.com/janosmiko/lfk/internal/logagg"
	"github.com/janosmiko/lfk/internal/ui"
)

// overlayHintBarLogTop returns hint bar text for all Log Top overlays.
func (m Model) overlayHintBarLogTop() string {
	switch m.overlay {
	case overlayLogTopGroupBy:
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "navigate"},
			{Key: "space", Desc: "toggle"},
			{Key: "enter", Desc: "apply"},
			{Key: "esc", Desc: "cancel"},
		})
	case overlayLogTopProfile:
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "navigate"},
			{Key: "enter", Desc: "select"},
			{Key: "esc", Desc: "cancel"},
		})
	default: // overlayLogTopColumns
		return m.renderHints([]ui.HintEntry{
			{Key: "space", Desc: "toggle"},
			{Key: "J/K", Desc: "reorder"},
			{Key: "enter", Desc: "apply"},
			{Key: "esc", Desc: "cancel"},
		})
	}
}

func (m Model) renderLogTopGroupByOverlay() (string, int, int) {
	cands := m.logTopGroupByCandidates()
	items := make([]ui.OverlayListItem, len(cands))
	for i, c := range cands {
		items[i] = ui.OverlayListItem{Name: c, Selected: m.logTop.pendingGroup[c]}
	}
	w := min(m.width-10, 50)
	cfg := ui.OverlayListConfig{
		Title:       "Group by",
		Cursor:      m.overlayCursor,
		MultiSelect: true,
		FooterHint:  "space toggle · enter apply · esc cancel",
		Height:      min(len(items)+4, m.height-6),
	}
	return ui.RenderOverlayList(items, cfg, w-4), w, cfg.Height + 2
}

func (m Model) renderLogTopProfileOverlay() (string, int, int) {
	kinds := logagg.AllKinds()
	items := make([]ui.OverlayListItem, len(kinds))
	for i, k := range kinds {
		items[i] = ui.OverlayListItem{Name: string(k), Active: k == m.logTop.profile}
	}
	w := min(m.width-10, 40)
	cfg := ui.OverlayListConfig{
		Title:            "Log format profile",
		Cursor:           m.overlayCursor,
		ShowActiveMarker: true,
		FooterHint:       "enter select · esc cancel",
		Height:           min(len(items)+4, m.height-6),
	}
	return ui.RenderOverlayList(items, cfg, w-4), w, cfg.Height + 2
}

func (m Model) renderLogTopColumnsOverlay() (string, int, int) {
	dims := m.logTop.colOrder
	mets := m.logTopAllMetrics()
	items := make([]ui.OverlayListItem, 0, len(dims)+1+len(mets))
	for _, d := range dims {
		items = append(items, ui.OverlayListItem{Name: d, Selected: !m.logTop.colHidden[d]})
	}
	// Divider between dims and metrics sections.
	if len(dims) > 0 && len(mets) > 0 {
		items = append(items, ui.OverlayListItem{Name: "--- metrics ---", Header: true})
	}
	for _, met := range mets {
		items = append(items, ui.OverlayListItem{Name: met, Selected: !m.logTop.colHidden[met]})
	}
	w := min(m.width-10, 44)
	cfg := ui.OverlayListConfig{
		Title:       "Columns",
		Cursor:      m.overlayCursor,
		MultiSelect: true,
		FooterHint:  "space toggle · J/K reorder dims · enter apply · esc cancel",
		Height:      min(len(items)+4, m.height-6),
	}
	return ui.RenderOverlayList(items, cfg, w-4), w, cfg.Height + 2
}
