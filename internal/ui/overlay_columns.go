package ui

import (
	"fmt"
	"strings"
)

// ColumnToggleEntry is the UI-facing column toggle entry.
type ColumnToggleEntry struct {
	Key     string
	Visible bool
}

// RenderColumnToggleOverlay renders the column toggle checklist overlay.
//
// Layout matches the namespace overlay so the filter bar feels the same
// across both:
//
//	Title
//	filter row (always present \u2014 placeholder when inactive)
//	(blank)
//	items...
//
// Anchoring the filter row under the title (instead of after the items)
// keeps it from "disappearing randomly" when the filter clears, and the
// row is counted toward the visible-item budget so the overlay never
// overflows its box.
func RenderColumnToggleOverlay(entries []ColumnToggleEntry, cursor int, filter string, filterActive bool, width, height int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Column Visibility"))
	b.WriteString("\n")

	// Filter bar \u2014 always renders one line so the layout is stable.
	switch {
	case filterActive:
		b.WriteString(OverlayFilterStyle.Render("/ " + filter + "\u2588"))
	case filter != "":
		b.WriteString(OverlayFilterStyle.Render("/ " + filter))
	default:
		b.WriteString(OverlayDimStyle.Render("/ to filter"))
	}
	b.WriteString("\n\n")

	if len(entries) == 0 {
		b.WriteString(OverlayDimStyle.Render("  No matching columns"))
		return b.String()
	}

	innerW := max(width-6, 20)

	// Reserve rows for title (1) + filter (1) + blank separator (1) +
	// overlay border/padding (~3) so the visible-item count is honest.
	maxVisible := max(height-6, 1)
	scrollOff := ConfigScrollOff
	if maxVisible < 8 {
		scrollOff = 0
	}
	scrollOffset := min(cursor-scrollOff, 0)
	if cursor+scrollOff >= scrollOffset+maxVisible {
		scrollOffset = cursor + scrollOff - maxVisible + 1
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	if scrollOffset+maxVisible > len(entries) {
		scrollOffset = max(len(entries)-maxVisible, 0)
	}
	endIdx := min(scrollOffset+maxVisible, len(entries))

	b.WriteString(RenderScrollAbove(scrollOffset, endIdx-scrollOffset, len(entries), innerW))
	b.WriteString("\n")

	for i := scrollOffset; i < endIdx; i++ {
		e := entries[i]
		prefix := "  "
		if e.Visible {
			prefix = "\u2713 "
		}
		line := fmt.Sprintf("%s%s", prefix, e.Key)
		if len(line) > innerW {
			line = line[:innerW]
		}

		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else if e.Visible {
			b.WriteString(OverlayFilterStyle.Render(line))
		} else {
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(RenderScrollBelow(scrollOffset, endIdx-scrollOffset, len(entries), innerW))
	return b.String()
}
