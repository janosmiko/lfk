package app

import (
	"github.com/janosmiko/lfk/internal/logagg"
	"github.com/janosmiko/lfk/internal/ui"
)

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
