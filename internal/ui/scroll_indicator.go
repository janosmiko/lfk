package ui

import "fmt"

// RenderScrollIndicator returns a single dimmed line that tells the
// user how many list items are hidden above and/or below the current
// viewport. Returns "" when nothing is hidden so callers can append
// unconditionally without a gating check.
//
// Format: "↑ N above   ↓ M below" — both arrows when the viewport is
// in the middle, single arrow at the edges. The string is truncated
// to width so the row never overflows the overlay's column budget.
//
// Used by paginated overlays (namespace, column toggle, action menu,
// bookmark, template, colorscheme, container/pod selectors) where the
// existing list rendering provides scrollOffset, visibleCount, and
// totalCount values straight from their windowing math.
func RenderScrollIndicator(scrollOffset, visibleCount, totalCount, width int) string {
	if scrollOffset < 0 || visibleCount <= 0 || totalCount <= 0 {
		return ""
	}
	if visibleCount > totalCount {
		visibleCount = totalCount
	}
	above := scrollOffset
	below := totalCount - scrollOffset - visibleCount
	if below < 0 {
		below = 0
	}
	if above == 0 && below == 0 {
		return ""
	}

	var text string
	switch {
	case above > 0 && below > 0:
		text = fmt.Sprintf("↑ %d above   ↓ %d below", above, below)
	case above > 0:
		text = fmt.Sprintf("↑ %d above", above)
	default:
		text = fmt.Sprintf("↓ %d below", below)
	}

	if width > 0 {
		text = Truncate(text, width)
	}
	return OverlayDimStyle.Render(text)
}
