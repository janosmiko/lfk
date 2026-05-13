package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// renderActionOverlay maps the action-menu items onto OverlayList. The
// verb code (model.Item.Status) renders as the "[s]" status badge; the
// long-form description (Extra) renders dim after the action name.
// Adaptive width replaces the old ActionOverlayWidth helper so long
// Karpenter / Knative descriptions still grow the box without wrapping.
func renderActionOverlay(m Model) (string, int) {
	items := make([]ui.OverlayListItem, len(m.overlayItems))
	for i, it := range m.overlayItems {
		items[i] = ui.OverlayListItem{Name: it.Name, Description: it.Extra, Status: it.Status}
	}
	cfg := ui.OverlayListConfig{
		Title:           "Actions",
		Cursor:          m.overlayCursor,
		ShowStatus:      true,
		ShowDescription: true,
	}
	w := ui.OverlayListWidth(items, cfg, m.width-10)
	return ui.RenderOverlayList(items, cfg, w-4), w
}

// renderContainerSelectOverlay maps the container-picker items onto
// OverlayList. Category and Status collapse into a single dim Description
// segment so the original "<name>  (category)  status" composition is
// preserved.
func renderContainerSelectOverlay(m Model) string {
	items := make([]ui.OverlayListItem, len(m.overlayItems))
	for i, it := range m.overlayItems {
		desc := ""
		if it.Category != "" && it.Category != "Containers" {
			desc = "(" + it.Category + ")"
		}
		if it.Status != "" {
			if desc != "" {
				desc += "  "
			}
			desc += it.Status
		}
		items[i] = ui.OverlayListItem{Name: it.Name, Description: desc}
	}
	return ui.RenderOverlayList(items, ui.OverlayListConfig{
		Title:           "Select Container",
		Cursor:          m.overlayCursor,
		ShowDescription: true,
	}, min(50, m.width-10)-4)
}
