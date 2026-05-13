package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// scrollOffsetFromCursor returns the smallest scroll value that keeps the
// cursor inside the visible window. Used by helpers that delegate scroll
// computation to the caller (OverlayList does not auto-scroll on cursor).
func scrollOffsetFromCursor(cursor, maxVisible int) int {
	if cursor < maxVisible {
		return 0
	}
	return cursor - maxVisible + 1
}

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

// renderPodSelectOverlay maps the pod picker (used by both the standard
// log-pod selector and the embedded view's pod-switcher) onto OverlayList.
// Pod status moves into the dim Description segment; the per-status color
// styling from the bespoke renderer is not preserved (acceptable for the
// log viewer's "pick a pod" UX).
func renderPodSelectOverlay(m Model) string {
	src := m.filteredLogPodItems()
	items := make([]ui.OverlayListItem, len(src))
	for i, it := range src {
		items[i] = ui.OverlayListItem{Name: it.Name, Description: it.Status}
	}
	const maxVisible = 15
	return ui.RenderOverlayList(items, ui.OverlayListConfig{
		Title:           "Select Pod",
		Cursor:          m.overlayCursor,
		Filter:          m.logPodFilterText,
		FilterActive:    m.logPodFilterActive,
		ShowDescription: true,
		Scroll:          scrollOffsetFromCursor(m.overlayCursor, maxVisible),
		MaxVisible:      maxVisible,
		EmptyMessage:    "No matching pods",
	}, min(60, m.width-10)-4)
}

// renderCanISubjectOverlay maps the CanI subject selector (a flat list of
// ServiceAccount / User / Group items) onto OverlayList. Status moves to
// Description so the subject kind reads alongside the name.
func renderCanISubjectOverlay(m Model) string {
	src := m.filteredOverlayItems()
	items := make([]ui.OverlayListItem, len(src))
	for i, it := range src {
		items[i] = ui.OverlayListItem{Name: it.Name, Description: it.Status}
	}
	const maxVisible = 15
	return ui.RenderOverlayList(items, ui.OverlayListConfig{
		Title:           "Select Subject",
		Cursor:          m.overlayCursor,
		Filter:          m.overlayFilter.Value,
		FilterActive:    m.canISubjectFilterMode,
		ShowDescription: true,
		Scroll:          scrollOffsetFromCursor(m.overlayCursor, maxVisible),
		MaxVisible:      maxVisible,
		EmptyMessage:    "No matching subjects",
	}, min(60, m.width-10)-4)
}

// renderBookmarkOverlay maps the bookmark picker onto OverlayList. The
// "[LOAD NAMESPACE]" chip embeds in the title as raw styled text; the
// per-row "<key>: <name>" slot prefix collapses into Status="[k]" + Name.
func renderBookmarkOverlay(m Model) string {
	const w = 90
	title := "Bookmarks"
	if m.bookmarkLoadNamespace {
		title += "   " + ui.HelpKeyStyle.Render("[LOAD NAMESPACE]")
	}
	var bookmarks []ui.OverlayListItem
	for _, bm := range m.bookmarks {
		if m.bookmarkFilter.Value != "" && !ui.MatchLine(bm.Name, m.bookmarkFilter.Value) {
			continue
		}
		bookmarks = append(bookmarks, ui.OverlayListItem{Key: bm.Slot, Name: bm.Name})
	}
	return ui.RenderOverlayList(bookmarks, ui.OverlayListConfig{
		Title:        title,
		Cursor:       m.overlayCursor,
		Filter:       m.bookmarkFilter.Value,
		FilterActive: m.bookmarkSearchMode == bookmarkModeFilter,
		ShowKey:      true,
		EmptyMessage: "No bookmarks yet — press m<key> in the explorer to set a mark",
	}, min(w, m.width-10)-4)
}

// renderTemplateOverlay maps the resource-template picker onto OverlayList.
// The "[Category] Name" composition collapses into Status (the category) +
// Name; the bespoke "> " cursor indicator is replaced by OverlayList's
// uniform highlight background.
func renderTemplateOverlay(m Model) (string, int) {
	src := m.filteredTemplates()
	items := make([]ui.OverlayListItem, len(src))
	for i, t := range src {
		items[i] = ui.OverlayListItem{Name: t.Name, Status: t.Category}
	}
	overlayW := min(60, m.width-10)
	overlayH := min(25, m.height-6)
	maxVisible := max(overlayH-5, 1)
	cfg := ui.OverlayListConfig{
		Title:        "Create from Template",
		Cursor:       m.templateCursor,
		Filter:       m.templateFilter.Value,
		FilterActive: m.templateSearchMode,
		ShowStatus:   true,
		MaxVisible:   maxVisible,
		EmptyMessage: "No templates available",
	}
	return ui.RenderOverlayList(items, cfg, overlayW-4), overlayH
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
