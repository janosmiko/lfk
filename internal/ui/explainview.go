package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// RenderExplainView renders the API explain browser as a three-column Miller
// layout: parent level keys (left), current field list (middle), description
// (right). A title line sits below the breadcrumb (like the other fullscreen
// views) and an outer frame wraps the columns.
func RenderExplainView(fields []model.ExplainField, cursor, scroll int, resourceDesc, title string, parentFields []model.ExplainField, parentCursor int, searchQuery, hintBar string, width, height int) string {
	d := objectExplorerDims(width, height)
	fieldLines := renderFieldList(fields, cursor, scroll, d.middleInner, d.contentHeight-1, searchQuery) // -1 for header
	// Build a table header row with NAME and TYPE columns, using the same nameWidth as the field rows.
	nameWidth := 0
	for _, f := range fields {
		nameWidth = max(nameWidth, len(f.Name))
	}
	nameWidth = min(nameWidth, d.middleInner/2)
	middleHeader := DimStyle.Bold(true).Render(fmt.Sprintf("  %-*s  %-4s  %s", nameWidth, "NAME", "REQ", "TYPE"))
	return renderExplainLayout(d, middleHeader, fieldLines, fields, cursor, resourceDesc, title, parentFields, parentCursor, hintBar, width)
}

// renderExplainLayout assembles the three API Explorer columns from a prepared
// middle column (header + body lines). Shared by the flat field list and the
// tree view. Column dimensions match the Object Explorer's.
func renderExplainLayout(d objectExplorerColumnDims, middleHeader string, fieldLines []string, fields []model.ExplainField, cursor int, resourceDesc, title string, parentFields []model.ExplainField, parentCursor int, hintBar string, width int) string {
	titleText := ViewTitle(width, title)

	// Left column: the parent level's keys only, drilled-into row highlighted.
	leftHeader := DimStyle.Bold(true).Render("PARENT")
	leftCol := leftHeader + "\n" + strings.Join(renderExplainKeyList(parentFields, parentCursor, d.leftInner, d.contentHeight-1), "\n")
	leftCol = PadToHeight(leftCol, d.contentHeight)
	leftCol = FillLinesBg(leftCol, d.leftInner, BaseBg)
	left := InactiveColumnStyle.Width(d.leftW).Height(d.contentHeight).MaxHeight(d.contentHeight + 2).Render(leftCol)

	// Middle column: field list (active).
	middleContent := middleHeader + "\n" + strings.Join(fieldLines, "\n")
	middleContent = PadToHeight(middleContent, d.contentHeight)
	middleContent = FillLinesBg(middleContent, d.middleInner, BaseBg)
	middle := ActiveColumnStyle.Width(d.middleW).Height(d.contentHeight).MaxHeight(d.contentHeight + 2).Render(middleContent)

	// Right column: description (inactive).
	descLines := renderFieldDescription(fields, cursor, resourceDesc, d.rightInner, d.contentHeight-1) // -1 for header
	rightHeader := DimStyle.Bold(true).Render("DESCRIPTION")
	rightContent := rightHeader + "\n" + strings.Join(descLines, "\n")
	rightContent = PadToHeight(rightContent, d.contentHeight)
	rightContent = FillLinesBg(rightContent, d.rightInner, BaseBg)
	right := InactiveColumnStyle.Width(d.rightW).Height(d.contentHeight).MaxHeight(d.contentHeight + 2).Render(rightContent)

	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, middle, right)
	framed := explorerFrameStyle().Render(columns)
	return lipgloss.JoinVertical(lipgloss.Left, titleText, framed, hintBar)
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
