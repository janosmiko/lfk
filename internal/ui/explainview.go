package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// RenderExplainView renders the API explain browser as a three-column Miller
// layout: parent level keys (left), current field list (middle), description
// (right). The drill path lives in the top breadcrumb, so there is no title
// line of its own.
func RenderExplainView(fields []model.ExplainField, cursor, scroll int, resourceDesc string, parentFields []model.ExplainField, parentCursor int, searchQuery, hintBar string, width, height int) string {
	// Calculate column widths matching the main explorer (12%, 51%, remainder).
	usable := width - 6 // 3 columns x 2 border chars
	leftW := max(10, usable*12/100)
	middleW := max(10, usable*51/100)
	rightW := max(10, usable-leftW-middleW)

	contentHeight := max(height-3, 3) // hint bar + borders

	// Column padding is 1 on each side, so inner content width is 2 less.
	colPad := 2
	leftInner := max(5, leftW-colPad)
	middleInner := max(5, middleW-colPad)
	rightInner := max(5, rightW-colPad)

	// Left column: the parent level's keys only, drilled-into row highlighted.
	leftHeader := DimStyle.Bold(true).Render("PARENT")
	leftCol := leftHeader + "\n" + strings.Join(renderExplainKeyList(parentFields, parentCursor, leftInner, contentHeight-1), "\n")
	leftCol = PadToHeight(leftCol, contentHeight)
	leftCol = FillLinesBg(leftCol, leftInner, BaseBg)
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
	middleContent = PadToHeight(middleContent, contentHeight)
	middleContent = FillLinesBg(middleContent, middleInner, BaseBg)
	middle := ActiveColumnStyle.Width(middleW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(middleContent)

	// Right column: description (inactive).
	descLines := renderFieldDescription(fields, cursor, resourceDesc, rightInner, contentHeight-1) // -1 for header
	rightHeader := DimStyle.Bold(true).Render("DESCRIPTION")
	rightContent := rightHeader + "\n" + strings.Join(descLines, "\n")
	rightContent = PadToHeight(rightContent, contentHeight)
	rightContent = FillLinesBg(rightContent, rightInner, BaseBg)
	right := InactiveColumnStyle.Width(rightW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(rightContent)

	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, middle, right)

	return lipgloss.JoinVertical(lipgloss.Left, columns, hintBar)
}

// renderExplainKeyList renders a keys-only (field names, no markers) list for
// the parent pane. The drilled-into row carries the main explorer's selection
// highlight. Scrolls to keep the cursor visible. Empty at the top level.
func renderExplainKeyList(fields []model.ExplainField, cursor, width, maxLines int) []string {
	if len(fields) == 0 {
		return []string{DimStyle.Render("(top level)")}
	}
	scroll := 0
	if cursor >= maxLines {
		scroll = cursor - maxLines + 1
	}
	end := min(scroll+maxLines, len(fields))
	lines := make([]string, 0, end-scroll)
	for i := scroll; i < end; i++ {
		lines = append(lines, renderKeyRow(fields[i].Name, i == cursor, width))
	}
	return lines
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

		// Selection is shown by the full-width active highlight, no cursor arrow.
		prefix := "  "

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
			if pad := width - lipgloss.Width(line); pad > 0 {
				line += strings.Repeat(" ", pad)
			}
			lines = append(lines, SelectedStyle.MaxWidth(width).Render(line))
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

// RenderExplainSearchOverlay renders the recursive field browser overlay using
// the shared OverlayList renderer, which provides the scrollbar, filter prompt,
// footer hint, and stable layout. Each field's type and path render as the dim
// secondary text. innerW is the content width (overlay box width minus chrome).
func RenderExplainSearchOverlay(results []model.ExplainField, cursor, scroll, maxVisible int, filterText string, filterActive bool, innerW int) string {
	items := make([]OverlayListItem, len(results))
	for i, r := range results {
		desc := r.Type
		if r.Path != "" {
			if desc != "" {
				desc += "  "
			}
			desc += r.Path
		}
		items[i] = OverlayListItem{Name: r.Name, Description: desc}
	}
	return RenderOverlayList(items, OverlayListConfig{
		Title:           "Recursive Field Browser",
		Subtitle:        fmt.Sprintf("%d fields", len(results)),
		Cursor:          cursor,
		Filterable:      true,
		Filter:          filterText,
		FilterActive:    filterActive,
		ShowDescription: true,
		Scroll:          scroll,
		MaxVisible:      maxVisible,
		EmptyMessage:    "No matching fields",
	}, innerW)
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
