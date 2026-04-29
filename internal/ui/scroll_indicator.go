package ui

import "fmt"

// RenderScrollAbove returns a single dimmed row reading "↑ N above" when
// the viewport hides items above. RenderScrollBelow does the same for
// items below ("↓ N below"). When there's no overflow in the requested
// direction the function returns a blank row of the same width — the
// row is always present so the surrounding overlay layout doesn't shift
// when filtering removes the overflow.
//
// Arguments are the same windowing values the caller already has from
// its pagination math: scrollOffset (start index of the visible window),
// visibleCount (how many rows fit), totalCount (full list length),
// width (overlay inner content width — 0 means no truncation).
//
// Used by paginated overlays (namespace, column toggle, pod selector,
// log container filter, bookmark, template, colorscheme).

// RenderScrollAbove renders the "↑ N above" line that goes above the
// visible items list.
func RenderScrollAbove(scrollOffset, visibleCount, totalCount, width int) string {
	above := computeAbove(scrollOffset, visibleCount, totalCount)
	if above <= 0 {
		return placeholderRow(width)
	}
	return formatScrollLine(fmt.Sprintf("↑ %d above", above), width)
}

// RenderScrollBelow renders the "↓ N below" line that goes below the
// visible items list.
func RenderScrollBelow(scrollOffset, visibleCount, totalCount, width int) string {
	below := computeBelow(scrollOffset, visibleCount, totalCount)
	if below <= 0 {
		return placeholderRow(width)
	}
	return formatScrollLine(fmt.Sprintf("↓ %d below", below), width)
}

func computeAbove(scrollOffset, visibleCount, totalCount int) int {
	if scrollOffset < 0 || visibleCount <= 0 || totalCount <= 0 {
		return 0
	}
	if scrollOffset > totalCount {
		return 0
	}
	return scrollOffset
}

func computeBelow(scrollOffset, visibleCount, totalCount int) int {
	if scrollOffset < 0 || visibleCount <= 0 || totalCount <= 0 {
		return 0
	}
	if visibleCount > totalCount {
		visibleCount = totalCount
	}
	below := totalCount - scrollOffset - visibleCount
	if below < 0 {
		return 0
	}
	return below
}

func formatScrollLine(text string, width int) string {
	if width > 0 {
		text = Truncate(text, width)
	}
	return OverlayDimStyle.Render(text)
}

// placeholderRow returns a single space rendered with the dim style so
// the row occupies one line of the layout but is visually empty. A bare
// "" would collapse the row and shift everything below by one line —
// the bug the indicator pair is designed to avoid.
func placeholderRow(width int) string {
	_ = width
	return OverlayDimStyle.Render(" ")
}
