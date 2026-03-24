package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/janosmiko/lfk/internal/model"
)

// Verb display order for the compact summary in the middle column.
var canIVerbs = []struct {
	verb  string
	label string
}{
	{"get", "GET"},
	{"list", "LIST"},
	{"watch", "WATCH"},
	{"create", "CREATE"},
	{"update", "UPDATE"},
	{"patch", "PATCH"},
	{"delete", "DELETE"},
}

// RenderCanIView renders the can-i browser with a two-column layout.
// The left column (API groups) is interactive; the right column (resources) is display-only.
func RenderCanIView(groups []string, resources []model.CanIResource, groupCursor, groupScroll int, subjectName string, namespaces []string, width, height int, hintBar string, resourceScroll int) string {
	// Title bar — truncate if it exceeds the available width.
	scopeLabel := "ns:" + strings.Join(namespaces, ",")
	titleText := TitleStyle.Render("RBAC Permissions ("+subjectName+")") + "  " + DimStyle.Render(scopeLabel)
	if lipgloss.Width(titleText) > width {
		// Drop scope label if too wide.
		titleText = TitleStyle.Render("RBAC Permissions (" + subjectName + ")")
	}
	if lipgloss.Width(titleText) > width {
		// Truncate subject name if still too wide.
		titleText = TitleStyle.Render("RBAC (" + subjectName + ")")
	}

	hint := hintBar

	// Column widths: left 25%, middle 75%.
	usable := width - 4
	leftW := max(10, usable*25/100)
	middleW := max(10, usable-leftW)

	contentHeight := max(height-4, 3)

	colPad := 2
	leftInner := max(5, leftW-colPad)
	middleInner := max(5, middleW-colPad)

	// Left column: API groups (always active/focused).
	leftHeader := DimStyle.Bold(true).Render("API Groups")
	leftLines := renderCanIGroups(groups, groupCursor, groupScroll, leftInner, contentHeight-1)
	leftContent := leftHeader + "\n" + strings.Join(leftLines, "\n")
	leftContent = padCanIToHeight(leftContent, contentHeight)

	left := ActiveColumnStyle.Width(leftW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(leftContent)

	// Middle column: resources with verb summary (display-only, no cursor).
	middleLines := renderCanIResources(resources, middleInner, contentHeight-1, resourceScroll)
	middleHeader := DimStyle.Bold(true).Render(renderCanIMiddleHeader(middleInner))
	middleContent := middleHeader + "\n" + strings.Join(middleLines, "\n")
	middleContent = padCanIToHeight(middleContent, contentHeight)
	middle := InactiveColumnStyle.Width(middleW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(middleContent)

	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, middle)

	return lipgloss.JoinVertical(lipgloss.Left, titleText, columns, hint)
}

// canIVerbColWidth returns the column width for a verb label (label length + 1 space padding).
func canIVerbColWidth(label string) int {
	return len(label) + 1
}

// canITotalVerbWidth returns the total width used by all verb columns.
func canITotalVerbWidth() int {
	total := 0
	for _, v := range canIVerbs {
		total += canIVerbColWidth(v.label)
	}
	return total
}

// renderCanIMiddleHeader builds the header line aligned with the resource columns.
func renderCanIMiddleHeader(width int) string {
	verbWidth := canITotalVerbWidth()
	nameWidth := width - verbWidth - 4
	if nameWidth < 8 {
		nameWidth = 8
	}

	// Build verb header with per-column widths matching the indicators.
	verbLabels := make([]string, len(canIVerbs))
	for i, v := range canIVerbs {
		verbLabels[i] = fmt.Sprintf("%-*s", canIVerbColWidth(v.label), v.label)
	}

	return fmt.Sprintf("  %-*s  %s", nameWidth, "RESOURCE", strings.Join(verbLabels, ""))
}

// renderCanIGroups renders the API group list for the left column.
func renderCanIGroups(groups []string, cursor, scroll, width, maxLines int) []string {
	if len(groups) == 0 {
		lines := make([]string, maxLines)
		lines[0] = DimStyle.Render("No groups found")
		for i := 1; i < maxLines; i++ {
			lines[i] = ""
		}
		return lines
	}

	maxScroll := max(len(groups)-maxLines, 0)
	scroll = max(min(scroll, maxScroll), 0)

	// Ensure cursor is within visible range.
	if cursor >= scroll+maxLines {
		scroll = cursor - maxLines + 1
	}
	if cursor < scroll {
		scroll = cursor
	}

	lines := make([]string, 0, maxLines)
	end := min(scroll+maxLines, len(groups))

	for i := scroll; i < end; i++ {
		display := groups[i]
		if len(display) > width-2 {
			display = display[:width-2]
		}

		if i == cursor {
			line := fmt.Sprintf("> %-*s", width-2, display)
			if len(line) > width {
				line = line[:width]
			}
			lines = append(lines, OverlaySelectedStyle.Render(line))
		} else {
			line := fmt.Sprintf("  %s", display)
			if len(line) > width {
				line = line[:width]
			}
			lines = append(lines, NormalStyle.Render(line))
		}
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return lines
}

// renderCanIResources renders the resource list with verb indicators (display-only, no cursor).
func renderCanIResources(resources []model.CanIResource, width, maxLines, scroll int) []string {
	if len(resources) == 0 {
		lines := make([]string, maxLines)
		lines[0] = DimStyle.Render("No resources in this group")
		for i := 1; i < maxLines; i++ {
			lines[i] = ""
		}
		return lines
	}

	maxScroll := max(len(resources)-maxLines, 0)
	scroll = max(min(scroll, maxScroll), 0)

	lines := make([]string, 0, maxLines)
	end := min(scroll+maxLines, len(resources))

	// Calculate name width: leave room for verb indicators + prefix (2) + gap (2).
	verbWidth := canITotalVerbWidth()
	nameWidth := width - verbWidth - 4
	if nameWidth < 8 {
		nameWidth = 8
	}

	for i := scroll; i < end; i++ {
		r := resources[i]
		name := r.Resource
		if len(name) > nameWidth {
			name = name[:nameWidth]
		}

		// Build verb indicator string with per-column widths.
		verbParts := make([]string, 0, len(canIVerbs))
		for _, v := range canIVerbs {
			colW := canIVerbColWidth(v.label)
			if r.Verbs[v.verb] {
				padded := "\u2713" + strings.Repeat(" ", colW-1)
				verbParts = append(verbParts, lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(padded))
			} else {
				padded := "\u00b7" + strings.Repeat(" ", colW-1)
				verbParts = append(verbParts, DimStyle.Render(padded))
			}
		}
		verbStr := strings.Join(verbParts, "")

		namePadded := fmt.Sprintf("%-*s", nameWidth, name)
		namePart := NormalStyle.Render("  " + namePadded + "  ")
		lines = append(lines, namePart+verbStr)
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return lines
}

// RenderCanISubjectOverlay renders the subject selector overlay for the can-i browser.
// Follows the same layout pattern as RenderNamespaceOverlay: title, filter bar, items, hint bar.
func RenderCanISubjectOverlay(items []model.Item, filter string, cursor int, filterMode bool) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Select Subject"))
	b.WriteString("\n")

	// Filter input (same 3 states as namespace overlay).
	switch {
	case filterMode:
		b.WriteString(OverlayFilterStyle.Render("/ " + filter + "\u2588"))
	case filter != "":
		b.WriteString(OverlayFilterStyle.Render("/ " + filter))
	default:
		b.WriteString(OverlayDimStyle.Render("/ to filter"))
	}
	b.WriteString("\n\n")

	if items == nil {
		b.WriteString(OverlayDimStyle.Render("Loading subjects..."))
		return b.String()
	}
	if len(items) == 0 {
		b.WriteString(OverlayDimStyle.Render("No matching subjects"))
		return b.String()
	}

	maxVisible := min(15, len(items))
	scrollOff := 3
	// Disable or reduce scrolloff when all items fit the visible area.
	if len(items) <= maxVisible {
		scrollOff = 0
	} else if maxSO := (maxVisible - 1) / 2; scrollOff > maxSO {
		scrollOff = maxSO
	}

	// Use VimScrollOff for stable viewport behavior.
	displayLines := func(from, to int) int { return to - from }
	start := VimScrollOff(overlayCanISubjectScroll, cursor, len(items), maxVisible, scrollOff, displayLines)
	overlayCanISubjectScroll = start

	end := start + maxVisible
	if end > len(items) {
		end = len(items)
	}

	for i := start; i < end; i++ {
		item := items[i]
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		line := prefix + item.Name
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// padCanIToHeight pads a rendered string to exactly the given height in lines.
func padCanIToHeight(s string, height int) string {
	lines := strings.Split(s, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:height], "\n")
}
