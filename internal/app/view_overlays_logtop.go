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
			{Key: "/", Desc: "filter"},
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

// overlayLogTopColScrollPos is the scroll state for the Log Top column-visibility overlay.
var overlayLogTopColScrollPos int

func (m Model) renderLogTopColumnsOverlay() (string, int, int) {
	cols := m.logTopFilteredColumns()
	items := make([]ui.OverlayListItem, len(cols))
	for i, c := range cols {
		items[i] = ui.OverlayListItem{Name: c, Active: !m.logTop.colHidden[c]}
	}
	w := min(m.width-10, 60)
	maxH := max(min(len(items)+4+overlayListChromeFilterable(), m.height-6), 3)
	contentH := max(maxH, 1)
	maxVisible := max(contentH-overlayListChromeFilterable(), 1)
	cfg := ui.OverlayListConfig{
		Title:            "Column Visibility",
		Cursor:           m.overlayCursor,
		Filterable:       true,
		Filter:           m.logTop.colFilter,
		FilterActive:     m.logTop.colFilterActive,
		ShowActiveMarker: true,
		Scroll:           overlayListScroll(&overlayLogTopColScrollPos, m.overlayCursor, len(items), maxVisible),
		MaxVisible:       maxVisible,
		EmptyMessage:     "No matching columns",
		Height:           contentH,
	}
	return ui.RenderOverlayList(items, cfg, w-6), w, contentH + 2
}
