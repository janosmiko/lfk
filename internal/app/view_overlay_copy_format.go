package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// renderOverlayCopyFormat paints the Y-key copy-as picker as a small
// OverlayList. Each row is a CopyFormat; the single-letter shortcut
// chip ([y]/[t]) is shown via ShowKey (JSON has no chip — its
// shortcut would collide with cursor-down). Hints live on the bottom
// hint bar via overlayHintBarSelector — no in-overlay footer. Returns
// empty content + zero dims when the picker isn't active so the
// view-dispatch fallback fires.
func (m Model) renderOverlayCopyFormat() (string, int, int) {
	if !m.copyFormatPicker.active {
		return "", 0, 0
	}
	formats := m.copyFormatPicker.formats
	items := make([]ui.OverlayListItem, len(formats))
	for i, f := range formats {
		items[i] = ui.OverlayListItem{
			Key:  f.ShortcutKey(),
			Name: f.Label(),
		}
	}
	cfg := ui.OverlayListConfig{
		Title:        "Copy as",
		Cursor:       m.copyFormatPicker.cursor,
		ShowKey:      true,
		EmptyMessage: "No formats available",
	}
	overlayW := ui.OverlayListWidth(items, cfg, max(m.width-10, 1))
	content := ui.RenderOverlayList(items, cfg, max(overlayW-4, 1))
	height := max(min(10, m.height-6), 1)
	return content, overlayW, height
}
