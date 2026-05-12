package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// ActionOverlayWidth returns the overlay box width needed to render
// items without wrapping. Sized to fit the longest
// "  [k] Label - Description" line plus border/padding (6 cells),
// floored at 70 so short menus keep the historical width, and capped
// at maxWidth so the overlay never overflows the terminal. The caller
// passes m.width-10 as maxWidth so the box leaves 5 cells of margin on
// each side, matching the prior min(70, m.width-10) behaviour.
func ActionOverlayWidth(items []model.Item, maxWidth int) int {
	const (
		floor    = 70
		overhead = 6 // OverlayStyle border (1+1) + padding (2+2)
	)
	contentW := 0
	for _, item := range items {
		prefix := ""
		if item.Status != "" {
			prefix = "[" + item.Status + "] "
		}
		line := "  " + prefix + item.Name + " - " + item.Extra
		if w := lipgloss.Width(line); w > contentW {
			contentW = w
		}
	}
	w := max(contentW+overhead, floor)
	if maxWidth > 0 && w > maxWidth {
		w = maxWidth
	}
	return w
}

// RenderActionOverlay renders the action menu overlay content.
func RenderActionOverlay(items []model.Item, cursor int, width int) string {
	// Account for overlay border (1 each side) + padding (2 each side) = 6 total.
	innerW := max(width-6, 20)

	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Actions"))
	b.WriteString("\n")

	for i, item := range items {
		keyHint := ""
		if item.Status != "" {
			keyHint = "[" + item.Status + "] "
		}
		label := fmt.Sprintf("  %s%s - %s", keyHint, item.Name, item.Extra)
		// Pad label with spaces to fill the inner width.
		if len(label) < innerW {
			label += strings.Repeat(" ", innerW-len(label))
		}
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(label))
		} else {
			b.WriteString(OverlayNormalStyle.Render(label))
		}
		if i < len(items)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}
