package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
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
// The left column (API groups) is interactive. The right column (resources) is display-only.
func RenderCanIView(groups []string, resources []model.CanIResource, groupCursor, groupScroll int, subjectName string, namespaces []string, width, height int, hintBar string, resourceScroll int, nsNegated bool) string {
	// Title at the left, scope label flushed to the far right with
	// baseBg-painted gap fill. Matches the WhoCan header layout so the
	// title bar reads consistently across both modes.
	//
	// Scope label collapses [""] (all-namespaces sentinel) and an empty
	// namespaces slice into "ns: all" so the user always sees what
	// scope is active — earlier behavior rendered "ns: " (no value),
	// which looked like a render bug.
	scopeLabel := CanIScopeLabel(namespaces, nsNegated)
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

	// Left 20%: group names are short and truncate cleanly, so the
	// middle pane keeps what it can against the fixed 42-col verb
	// block. Under a ~90-col terminal names still cut to "dep~".
	usable := width - 4
	leftW := max(10, usable*20/100)
	middleW := max(10, usable-leftW)

	contentHeight := max(height-4, 3)

	colPad := 2
	leftInner := max(5, leftW-colPad)
	middleInner := max(5, middleW-colPad)

	// Left column: API groups (always active/focused).
	// Narrow panes can't fit "API Groups" (10 cols) — it would wrap
	// and steal a body row. Fall back to "Groups" instead.
	leftHeaderText := "API Groups"
	if leftInner < lipgloss.Width(leftHeaderText) {
		leftHeaderText = "Groups"
	}
	leftHeader := DimStyle.Bold(true).Render(leftHeaderText)
	leftLines := renderCanIGroups(groups, groupCursor, groupScroll, leftInner, contentHeight-1)
	leftContent := leftHeader + "\n" + strings.Join(leftLines, "\n")
	leftContent = PadToHeight(leftContent, contentHeight)

	left := BoxHeight(BoxWidth(ActiveColumnStyle, leftW), contentHeight).MaxHeight(contentHeight + 2).Render(leftContent)

	// Middle column: resources with verb summary (display-only, no cursor).
	middleLines := renderCanIResources(resources, middleInner, contentHeight-1, resourceScroll)
	middleHeader := DimStyle.Bold(true).Render(renderCanIMiddleHeader(middleInner))
	middleContent := middleHeader + "\n" + strings.Join(middleLines, "\n")
	middleContent = PadToHeight(middleContent, contentHeight)
	middle := BoxHeight(BoxWidth(InactiveColumnStyle, middleW), contentHeight).MaxHeight(contentHeight + 2).Render(middleContent)

	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, middle)

	return lipgloss.JoinVertical(lipgloss.Left, titleText, columns, hint)
}

// joinTitleAndRightLabel composes a title row with the title segment
// at the left, baseBg-painted spaces filling the middle, and the
// label flushed to the right edge of `width`. Used by both Can-I and
// Who-Can title rows so the namespace/scope chip lands consistently
// at the right edge across both modes. If the combined widths exceed
// `width`, the right label is dropped (a half-shown label is more
// confusing than no label) and the title is cut to fit.
func joinTitleAndRightLabel(title, rightLabel string, width int) string {
	if lipgloss.Width(title)+1+lipgloss.Width(rightLabel) > width {
		// Who-Can's title plus its eight verb chips need 74 cols, so an
		// 80-col terminal wraps this row and pushes the columns down.
		return Truncate(title, width)
	}
	gap := max(width-lipgloss.Width(title)-lipgloss.Width(rightLabel), 1)
	return title + BarNormalStyle.Render(strings.Repeat(" ", gap)) + rightLabel
}

// CanIScopeLabel formats the namespace scope shown in the title row.
// Collapses the all-namespaces sentinels ([""] and the empty slice)
// to a literal "ns: all" so the label never reads as "ns: " — that
// looked like a render bug to users who picked "All Namespaces" in
// the namespace selector. When negated is true, each namespace is
// prefixed with "!" to indicate it is excluded.
func CanIScopeLabel(namespaces []string, negated bool) string {
	if len(namespaces) == 0 {
		return "ns: all"
	}
	if len(namespaces) == 1 && namespaces[0] == "" {
		return "ns: all"
	}
	if !negated {
		return "ns: " + strings.Join(namespaces, ",")
	}
	prefixed := make([]string, len(namespaces))
	for i, ns := range namespaces {
		prefixed[i] = "!" + ns
	}
	return "ns: " + strings.Join(prefixed, ",")
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
// nameWidth floors at 1 (not 8): on an 80-col terminal the middle pane is
// only ~50 cols wide against 42 cols of fixed verb indicators, so a floor
// of 8 overshoots the pane by 4 cols and lipgloss wraps every row.
func renderCanIMiddleHeader(width int) string {
	verbWidth := canITotalVerbWidth()
	nameWidth := max(width-verbWidth-4, 1)

	// Build verb header with per-column widths matching the indicators.
	verbLabels := make([]string, len(canIVerbs))
	for i, v := range canIVerbs {
		verbLabels[i] = fmt.Sprintf("%-*s", canIVerbColWidth(v.label), v.label)
	}

	// Truncate before padding: %-*s pads but never cuts, so a narrow
	// pane kept the full 8-col label and shifted every verb column
	// right of the indicator it labels.
	header := fmt.Sprintf("  %s  %s", padRight(Truncate("RESOURCE", nameWidth), nameWidth), strings.Join(verbLabels, ""))
	return Truncate(header, width)
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
		// Display-width aware: use lipgloss.Width / Truncate / padRight rather
		// than len()/byte-slicing so any non-ASCII group name aligns and never
		// gets cut mid-rune.
		display := groups[i]
		if lipgloss.Width(display) > width-2 {
			display = Truncate(display, width-2)
		}

		if i == cursor {
			line := Truncate(padRight("> "+display, width), width)
			lines = append(lines, OverlaySelectedStyle.Render(line))
		} else {
			line := Truncate("  "+display, width)
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
	// Floor at 1 (not 8) — see renderCanIMiddleHeader. The trailing
	// Truncate on each row guarantees the line never exceeds the pane
	// even when the fixed verb block alone is wider than a tiny pane.
	verbWidth := canITotalVerbWidth()
	nameWidth := max(width-verbWidth-4, 1)

	for i := scroll; i < end; i++ {
		r := resources[i]
		name := Truncate(r.Resource, nameWidth)

		// Build verb indicator string with per-column widths.
		verbParts := make([]string, 0, len(canIVerbs))
		for _, v := range canIVerbs {
			colW := canIVerbColWidth(v.label)
			switch r.VerbState(v.verb) {
			case model.CanIVerbAllowed:
				padded := "\u2713" + strings.Repeat(" ", colW-1)
				verbParts = append(verbParts, lipgloss.NewStyle().Foreground(ThemeColor("2")).Background(BaseBg).Render(padded))
			case model.CanIVerbMixed:
				padded := "?" + strings.Repeat(" ", colW-1)
				verbParts = append(verbParts, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning)).Background(BaseBg).Render(padded))
			default:
				padded := "\u00b7" + strings.Repeat(" ", colW-1)
				verbParts = append(verbParts, DimStyle.Render(padded))
			}
		}
		verbStr := strings.Join(verbParts, "")

		namePadded := padRight(name, nameWidth)
		namePart := NormalStyle.Render("  " + namePadded + "  ")
		lines = append(lines, Truncate(namePart+verbStr, width))
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return lines
}
