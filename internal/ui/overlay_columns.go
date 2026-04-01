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
func RenderColumnToggleOverlay(entries []ColumnToggleEntry, cursor int, filter string, filterActive bool, width, height int) string {
	title := OverlayTitleStyle.Render("Column Visibility")

	if len(entries) == 0 {
		return title + "\n\n" + OverlayDimStyle.Render("  No columns available.")
	}

	innerW := width - 6
	if innerW < 20 {
		innerW = 20
	}

	// Scroll with scrolloff.
	maxVisible := height - 6
	if maxVisible < 1 {
		maxVisible = 1
	}
	scrollOff := 3
	if maxVisible < 8 {
		scrollOff = 0
	}
	scrollOffset := 0
	if cursor-scrollOff < scrollOffset {
		scrollOffset = cursor - scrollOff
	}
	if cursor+scrollOff >= scrollOffset+maxVisible {
		scrollOffset = cursor + scrollOff - maxVisible + 1
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	if scrollOffset+maxVisible > len(entries) {
		scrollOffset = len(entries) - maxVisible
		if scrollOffset < 0 {
			scrollOffset = 0
		}
	}
	endIdx := scrollOffset + maxVisible
	if endIdx > len(entries) {
		endIdx = len(entries)
	}

	var lines []string
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
			lines = append(lines, OverlaySelectedStyle.Render(line))
		} else if e.Visible {
			lines = append(lines, OverlayFilterStyle.Render(line))
		} else {
			lines = append(lines, OverlayNormalStyle.Render(line))
		}
	}

	content := strings.Join(lines, "\n")

	// Filter bar.
	var footer string
	if filterActive {
		footer = "\n" + HelpKeyStyle.Render("/") + BarDimStyle.Render(": ") +
			OverlayNormalStyle.Render(filter) +
			OverlayDimStyle.Render("\u2588")
	} else if filter != "" {
		footer = "\n" + OverlayDimStyle.Render("filter: ") + OverlayFilterStyle.Render(filter)
	}

	return title + "\n" + content + footer
}
