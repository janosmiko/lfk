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
	// Title at the left, scope label flushed to the far right with
	// baseBg-painted gap fill. Matches the WhoCan header layout so the
	// title bar reads consistently across both modes.
	//
	// Scope label collapses [""] (all-namespaces sentinel) and an empty
	// namespaces slice into "ns: all" so the user always sees what
	// scope is active — earlier behavior rendered "ns: " (no value),
	// which looked like a render bug.
	scopeLabel := CanIScopeLabel(namespaces)
	// Subject chip mirrors the Who-Can verb chip styling: dim "Subject:"
	// label + light value, both on baseBg so they sit flush in the title
	// row's barBg band.
	subjectChip := BarDimStyle.Render(" Subject: ") + BarNormalStyle.Render(subjectName)
	title := TitleStyle.Render("RBAC Explorer: Can-I?") + subjectChip
	if lipgloss.Width(title)+1+lipgloss.Width(scopeLabel) > width {
		// Shorter fallback for narrow terminals.
		title = TitleStyle.Render("Can-I?") + subjectChip
	}
	titleText := joinTitleAndRightLabel(title, BarDimStyle.Render(scopeLabel), width)

	hint := hintBar

	// Column widths: left 20% (API group names rarely exceed 25 cols even
	// for long group names like "apiextensions.k8s.io" — 25% wasted space),
	// middle 80%.
	usable := width - 4
	leftW := max(10, usable*20/100)
	middleW := max(10, usable-leftW)

	contentHeight := max(height-4, 3)

	colPad := 2
	leftInner := max(5, leftW-colPad)
	middleInner := max(5, middleW-colPad)

	// Left column: API groups (always active/focused).
	leftHeader := DimStyle.Bold(true).Render("API Groups")
	leftLines := renderCanIGroups(groups, groupCursor, groupScroll, leftInner, contentHeight-1)
	leftContent := leftHeader + "\n" + strings.Join(leftLines, "\n")
	leftContent = PadToHeight(leftContent, contentHeight)

	left := ActiveColumnStyle.Width(leftW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(leftContent)

	// Middle column: resources with verb summary (display-only, no cursor).
	middleLines := renderCanIResources(resources, middleInner, contentHeight-1, resourceScroll)
	middleHeader := DimStyle.Bold(true).Render(renderCanIMiddleHeader(middleInner))
	middleContent := middleHeader + "\n" + strings.Join(middleLines, "\n")
	middleContent = PadToHeight(middleContent, contentHeight)
	middle := InactiveColumnStyle.Width(middleW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(middleContent)

	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, middle)

	return lipgloss.JoinVertical(lipgloss.Left, titleText, columns, hint)
}

// joinTitleAndRightLabel composes a title row with the title segment
// at the left, baseBg-painted spaces filling the middle, and the
// label flushed to the right edge of `width`. Used by both Can-I and
// Who-Can title rows so the namespace/scope chip lands consistently
// at the right edge across both modes. If the combined widths exceed
// `width`, the right label is dropped (a half-shown label is more
// confusing than no label).
func joinTitleAndRightLabel(title, rightLabel string, width int) string {
	if lipgloss.Width(title)+1+lipgloss.Width(rightLabel) > width {
		// No room for the right label; just return the title.
		return title
	}
	gap := max(width-lipgloss.Width(title)-lipgloss.Width(rightLabel), 1)
	return title + BarNormalStyle.Render(strings.Repeat(" ", gap)) + rightLabel
}

// CanIScopeLabel formats the namespace scope shown in the title row.
// Collapses the all-namespaces sentinels ([""] and the empty slice)
// to a literal "ns: all" so the label never reads as "ns: " — that
// looked like a render bug to users who picked "All Namespaces" in
// the namespace selector.
func CanIScopeLabel(namespaces []string) string {
	if len(namespaces) == 0 {
		return "ns: all"
	}
	if len(namespaces) == 1 && namespaces[0] == "" {
		return "ns: all"
	}
	return "ns: " + strings.Join(namespaces, ",")
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
	nameWidth := max(width-verbWidth-4, 8)

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
	nameWidth := max(width-verbWidth-4, 8)

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
				verbParts = append(verbParts, lipgloss.NewStyle().Foreground(ThemeColor("2")).Background(BaseBg).Render(padded))
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
	scrollOff := ConfigScrollOff
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

	end := min(start+maxVisible, len(items))

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
