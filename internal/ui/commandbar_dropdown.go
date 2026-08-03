package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Suggestion represents a single completion item in the command bar dropdown.
type Suggestion struct {
	Text     string // The completion text (e.g., "pods", "kube-system").
	Category string // Category label (e.g., "resource", "namespace", "flag", "command").
}

// RenderCommandDropdown renders a vertical dropdown of suggestions.
// Returns an empty string if there are no suggestions.
//
// Each line shows: [category]  text -- with the category dimmed on the left.
// The selected item is highlighted with OverlaySelectedStyle.
// The viewport scrolls to keep the selected item visible, centered when possible.
// Lines are padded to width.
func RenderCommandDropdown(suggestions []Suggestion, selected, maxHeight, width int) string {
	if len(suggestions) == 0 {
		return ""
	}

	// Clamp selected index to valid range.
	if selected < 0 {
		selected = 0
	}

	if selected >= len(suggestions) {
		selected = len(suggestions) - 1
	}

	// Determine visible height.
	visibleCount := min(len(suggestions), maxHeight)

	// Calculate viewport start to keep selected item visible (centered).
	start := computeViewportStart(selected, visibleCount, len(suggestions))

	// Pre-compute styles.
	categoryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDimmed))
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile))
	bgStyle := lipgloss.NewStyle().Background(lipgloss.Color(ColorSurface))

	// Find max category width for alignment within the visible slice.
	maxCatWidth := maxCategoryWidth(suggestions[start : start+visibleCount])

	var lines []string

	for i := start; i < start+visibleCount; i++ {
		s := suggestions[i]
		line := formatSuggestionLine(s, maxCatWidth, width, i == selected, categoryStyle, textStyle, bgStyle)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// computeViewportStart calculates the first visible index so that the selected
// item stays centered (or as close to centered as possible) within the viewport.
func computeViewportStart(selected, visibleCount, total int) int {
	if total <= visibleCount {
		return 0
	}

	// Try to center the selected item.
	half := visibleCount / 2
	start := selected - half

	// Clamp to valid range.
	start = max(start, 0)
	start = min(start, total-visibleCount)

	return start
}

// maxCategoryWidth returns the length of the longest category string
// in the given slice.
func maxCategoryWidth(suggestions []Suggestion) int {
	maxW := 0

	for _, s := range suggestions {
		if len(s.Category) > maxW {
			maxW = len(s.Category)
		}
	}

	return maxW
}

// formatSuggestionLine renders a single suggestion line with category label,
// text, and padding to the given width.
func formatSuggestionLine(
	s Suggestion,
	maxCatWidth, width int,
	isSelected bool,
	categoryStyle, textStyle, bgStyle lipgloss.Style,
) string {
	// Format: " [category]  text " padded to width.
	catLabel := fmt.Sprintf(" %-*s", maxCatWidth, s.Category)
	textLabel := fmt.Sprintf("  %s", s.Text)
	content := catLabel + textLabel

	// Pad or truncate to fit width using display width — suggestion text can be
	// a UTF-8 context name, so len()/byte-slicing would misalign and split runes.
	if cw := lipgloss.Width(content); cw < width {
		content += strings.Repeat(" ", width-cw)
	} else if cw > width {
		content = Truncate(content, width)
	}

	if isSelected {
		return OverlaySelectedStyle.Render(content)
	}

	// Apply category + text styling for non-selected lines.
	styledCat := categoryStyle.Render(catLabel)
	styledText := textStyle.Render(textLabel)
	padding := ""

	plainLen := lipgloss.Width(catLabel) + lipgloss.Width(textLabel)
	if plainLen < width {
		padding = strings.Repeat(" ", width-plainLen)
	}

	return bgStyle.Render(styledCat + styledText + padding)
}
