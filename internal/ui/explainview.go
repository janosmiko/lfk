package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// RenderExplainView renders the API explain browser view with a three-column layout
// matching the main explorer style: path breadcrumb (left), field list (middle),
// description (right).
func RenderExplainView(fields []model.ExplainField, cursor, scroll int, resourceDesc, title, path, searchQuery, hintBar string, width, height int) string {
	// Title bar.
	titleText := TitleStyle.Width(width).MaxWidth(width).MaxHeight(1).Render("API Explorer: " + title)

	// Calculate column widths matching the main explorer (12%, 51%, remainder).
	usable := width - 6 // 3 columns x 2 border chars
	leftW := max(10, usable*12/100)
	middleW := max(10, usable*51/100)
	rightW := max(10, usable-leftW-middleW)

	contentHeight := max(height-4, 3) // title + hint bar + borders

	// Column padding is 1 on each side, so inner content width is 2 less.
	colPad := 2
	leftInner := max(5, leftW-colPad)
	middleInner := max(5, middleW-colPad)
	rightInner := max(5, rightW-colPad)

	// Left column: path breadcrumb.
	leftCol := renderExplainPath(path, title, leftInner, contentHeight)
	leftCol = padExplainToHeight(leftCol, contentHeight)
	left := InactiveColumnStyle.Width(leftW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(leftCol)

	// Middle column: field list (active).
	fieldLines := renderFieldList(fields, cursor, scroll, middleInner, contentHeight-1, searchQuery) // -1 for header
	// Build a table header row with NAME and TYPE columns, using the same nameWidth as the field rows.
	nameWidth := 0
	for _, f := range fields {
		nameWidth = max(nameWidth, len(f.Name))
	}
	nameWidth = min(nameWidth, middleInner/2)
	middleHeader := DimStyle.Bold(true).Render(fmt.Sprintf("  %-*s  %-4s  %s", nameWidth, "NAME", "REQ", "TYPE"))
	middleContent := middleHeader + "\n" + strings.Join(fieldLines, "\n")
	middleContent = padExplainToHeight(middleContent, contentHeight)
	middle := ActiveColumnStyle.Width(middleW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(middleContent)

	// Right column: description (inactive).
	descLines := renderFieldDescription(fields, cursor, resourceDesc, rightInner, contentHeight-1) // -1 for header
	rightHeader := DimStyle.Bold(true).Render("DESCRIPTION")
	rightContent := rightHeader + "\n" + strings.Join(descLines, "\n")
	rightContent = padExplainToHeight(rightContent, contentHeight)
	right := InactiveColumnStyle.Width(rightW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(rightContent)

	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, middle, right)

	return lipgloss.JoinVertical(lipgloss.Left, titleText, columns, hintBar)
}

// renderExplainPath renders the left column content showing the drill-down path
// as a vertical breadcrumb. At root level it shows just the resource name highlighted.
// When drilled into a nested path, each segment is shown vertically with the last
// segment highlighted as the current level.
func renderExplainPath(path, resourceName string, width, _ int) string {
	var b strings.Builder

	// Header.
	b.WriteString(DimStyle.Bold(true).Render("PATH"))
	b.WriteString("\n")

	// Resource name at top (always shown).
	resDisplay := resourceName
	if len(resDisplay) > width {
		resDisplay = resDisplay[:width]
	}
	if path == "" {
		// At root level - resource name is the active breadcrumb.
		b.WriteString(HeaderStyle.Render("> " + resDisplay))
	} else {
		b.WriteString(DimStyle.Render("  " + resDisplay))
	}

	// Show path segments when drilled in.
	if path != "" {
		segments := strings.Split(path, ".")
		for i, seg := range segments {
			b.WriteString("\n")
			display := seg
			if len(display) > width-2 {
				display = display[:width-2]
			}
			if i == len(segments)-1 {
				// Current level - highlighted.
				b.WriteString(HeaderStyle.Render("> " + display))
			} else {
				b.WriteString(DimStyle.Render("  " + display))
			}
		}
	}

	return b.String()
}

// padExplainToHeight pads a rendered string to exactly the given height in lines.
func padExplainToHeight(s string, height int) string {
	lines := strings.Split(s, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:height], "\n")
}

// renderFieldList renders the scrollable field list for the middle column.
func renderFieldList(fields []model.ExplainField, cursor, scroll, width, maxLines int, searchQuery string) []string {
	if len(fields) == 0 {
		lines := make([]string, maxLines)
		lines[0] = DimStyle.Render("No fields found")
		for i := 1; i < maxLines; i++ {
			lines[i] = ""
		}
		return lines
	}

	// Clamp scroll.
	maxScroll := max(len(fields)-maxLines, 0)
	scroll = max(min(scroll, maxScroll), 0)

	lines := make([]string, 0, maxLines)
	end := min(scroll+maxLines, len(fields))

	// Calculate the maximum name width for alignment.
	nameWidth := 0
	for _, f := range fields {
		nameWidth = max(nameWidth, len(f.Name))
	}
	nameWidth = min(nameWidth, width/2)

	for i := scroll; i < end; i++ {
		f := fields[i]
		name := f.Name
		if len(name) > nameWidth {
			name = name[:nameWidth]
		}

		// Format: "> name     REQ   <type>" or "  name     REQ   <type>"
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}

		// Build required column (4 chars wide).
		reqStr := "    "
		if f.Required {
			reqStr = " yes"
		}

		typeStr := f.Type
		maxTypeLen := width - nameWidth - 12 // prefix(2) + padding(2) + req(4) + padding(2)
		if maxTypeLen > 0 && len(typeStr) > maxTypeLen {
			typeStr = typeStr[:maxTypeLen]
		}

		if i == cursor {
			// Selected line: render with highlight on search matches.
			highlightedName := highlightNameSelected(fmt.Sprintf("%-*s", nameWidth, name), searchQuery)
			line := prefix + highlightedName + "  " + reqStr + "  " + typeStr
			lines = append(lines, OverlaySelectedStyle.Render(line))
		} else {
			// Normal line: highlight search matches in field name.
			highlightedName := highlightName(fmt.Sprintf("%-*s", nameWidth, name), searchQuery)
			styledReq := "    "
			if f.Required {
				styledReq = StatusProgressing.Render(" yes")
			}
			namePart := prefix + highlightedName
			reqPart := "  " + styledReq
			typePart := DimStyle.Render("  " + typeStr)
			lines = append(lines, NormalStyle.Render(namePart)+reqPart+typePart)
		}
	}

	// Pad remaining lines.
	for len(lines) < maxLines {
		lines = append(lines, "")
	}

	return lines
}

// renderFieldDescription renders the description panel for the selected field.
func renderFieldDescription(fields []model.ExplainField, cursor int, resourceDesc string, width, maxLines int) []string {
	lines := make([]string, 0, maxLines)

	if len(fields) == 0 {
		// Show resource description when no fields.
		if resourceDesc != "" {
			wrapped := wrapText(resourceDesc, width)
			for _, line := range wrapped {
				lines = append(lines, NormalStyle.Render(line))
			}
		} else {
			lines = append(lines, DimStyle.Render("No description available"))
		}
		for len(lines) < maxLines {
			lines = append(lines, "")
		}
		return lines
	}

	if cursor < 0 || cursor >= len(fields) {
		for range maxLines {
			lines = append(lines, "")
		}
		return lines
	}

	f := fields[cursor]

	// Field name and type header.
	lines = append(lines, HeaderStyle.Render(f.Name))
	if f.Type != "" {
		lines = append(lines, DimStyle.Render("TYPE: "+f.Type))
	}
	lines = append(lines, "")

	// Field description.
	if f.Description != "" {
		wrapped := wrapText(f.Description, width)
		for _, w := range wrapped {
			lines = append(lines, NormalStyle.Render(w))
		}
	} else {
		lines = append(lines, DimStyle.Render("No description available"))
	}

	// If the field has an Object or array type, show drill-in hint.
	if IsDrillableType(f.Type) {
		lines = append(lines, "")
		lines = append(lines, HelpKeyStyle.Render("Press l or Enter to drill into this field"))
	}

	// Pad remaining lines.
	for len(lines) < maxLines {
		lines = append(lines, "")
	}

	// Truncate if too many lines.
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	return lines
}

// IsDrillableType returns true if the type indicates the field can be drilled into.
func IsDrillableType(typ string) bool {
	if typ == "" {
		return false
	}
	lower := strings.ToLower(typ)
	// Object types: <Object>, <ObjectMeta>, <PodSpec>, etc.
	// Array of objects: <[]Object>, <[]Container>, etc.
	// Map types: <map[string]string>, etc.
	if strings.Contains(lower, "object") {
		return true
	}
	if strings.Contains(lower, "[]") {
		return true
	}
	if strings.Contains(lower, "map[") {
		return true
	}
	// Types that are likely objects (capitalized and not primitive).
	inner := strings.Trim(typ, "<>[]")
	if len(inner) > 0 && inner[0] >= 'A' && inner[0] <= 'Z' {
		// Capitalized types are usually objects (e.g., <PodSpec>, <Container>).
		// Exclude known primitives.
		switch inner {
		case "string", "integer", "boolean", "number", "int32", "int64",
			"Time", "Duration", "Quantity":
			return false
		}
		return true
	}
	return false
}

// RenderExplainSearchOverlay renders the recursive field browser overlay with filter support.
func RenderExplainSearchOverlay(results []model.ExplainField, cursor, scroll, maxVisible int, filterText string, filterActive bool) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Recursive Field Browser"))
	b.WriteString("\n")

	// Filter bar (like namespace selector).
	switch {
	case filterActive:
		b.WriteString(OverlayFilterStyle.Render("/ " + filterText + "\u2588"))
	case filterText != "":
		b.WriteString(OverlayFilterStyle.Render("/ " + filterText))
	default:
		b.WriteString(OverlayDimStyle.Render("/ to filter"))
	}
	b.WriteString("\n")

	b.WriteString(DimStyle.Render(fmt.Sprintf("  %d fields", len(results))))
	b.WriteString("\n")

	if len(results) == 0 {
		if filterText != "" {
			b.WriteString("\n")
			b.WriteString(OverlayDimStyle.Render("  No matching fields"))
		}
		return b.String()
	}

	// Clamp scroll.
	maxScroll := max(len(results)-maxVisible, 0)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	end := min(scroll+maxVisible, len(results))

	// Calculate column widths based on visible data.
	nameWidth := 10
	typeWidth := 8
	for i := scroll; i < end; i++ {
		r := results[i]
		if len(r.Name) > nameWidth {
			nameWidth = len(r.Name)
		}
		if len(r.Type) > typeWidth {
			typeWidth = len(r.Type)
		}
	}
	nameWidth = min(nameWidth, 30)
	typeWidth = min(typeWidth, 20)

	// Show scroll-up indicator.
	if scroll > 0 {
		b.WriteString(DimStyle.Render(fmt.Sprintf("  (%d more above)", scroll)))
		b.WriteString("\n")
	}

	for i := scroll; i < end; i++ {
		r := results[i]
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}

		name := r.Name
		if len(name) > nameWidth {
			name = name[:nameWidth]
		}
		typ := r.Type
		if len(typ) > typeWidth {
			typ = typ[:typeWidth]
		}

		line := fmt.Sprintf("%s%-*s  %-*s  %s", prefix, nameWidth, name, typeWidth, typ, r.Path)

		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	// Show scroll-down indicator.
	if end < len(results) {
		b.WriteString(DimStyle.Render(fmt.Sprintf("  (%d more below)", len(results)-end)))
		b.WriteString("\n")
	}

	return b.String()
}

// wrapText wraps a text string to the given width, breaking on word boundaries.
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	currentLine := words[0]

	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}
