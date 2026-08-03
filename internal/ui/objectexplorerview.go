package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/janosmiko/lfk/internal/model"
)

// RenderObjectExplorerView renders the Object Explorer: a three-column
// Miller layout (PARENT level | current NAME/VALUE list | YAML PREVIEW of the
// selected node). It mirrors the API Explorer's layout but shows live values:
// each row carries an inline value preview, and the right column renders the
// selected node's subtree as colorized YAML instead of a schema description.
// A title line sits below the breadcrumb (like the other fullscreen views) and
// an outer frame wraps the columns.
func RenderObjectExplorerView(fields []model.ObjectField, cursor, scroll int, title string, parentFields []model.ObjectField, parentCursor int, previewYAML string, previewScroll int, filterBar, hintBar string, width, height int) string {
	d := objectExplorerDims(width, height)
	fieldLines := renderResourceFieldList(fields, cursor, scroll, d.middleInner, d.contentHeight-1)
	nameWidth := resourceNameWidth(fields, d.middleInner)
	middleHeader := DimStyle.Bold(true).Render(fmt.Sprintf("  %-*s  %s", nameWidth, "NAME", "VALUE"))
	return renderObjectExplorerLayout(d, middleHeader, fieldLines, title, parentFields, parentCursor, previewYAML, previewScroll, filterBar, hintBar, width)
}

// objectExplorerColumnDims holds the column widths shared by the flat and tree
// variants of the Object Explorer layout.
type objectExplorerColumnDims struct {
	leftW, middleW, rightW             int
	leftInner, middleInner, rightInner int
	contentHeight                      int
}

// objectExplorerDims computes the Miller-column dimensions for the Object
// Explorer (12% | 51% | remainder, minus borders and the outer frame).
func objectExplorerDims(width, height int) objectExplorerColumnDims {
	// Reserve 2 cells/rows for the outer frame that wraps the columns (the
	// breadcrumb above and the hint bar below sit OUTSIDE it).
	usable := width - 8
	leftW := max(10, usable*12/100)
	middleW := max(10, usable*51/100)
	rightW := max(10, usable-leftW-middleW)
	colPad := 2
	return objectExplorerColumnDims{
		leftW: leftW, middleW: middleW, rightW: rightW,
		leftInner: max(5, leftW-colPad), middleInner: max(5, middleW-colPad), rightInner: max(5, rightW-colPad),
		contentHeight: max(height-6, 3), // title + hint + column borders + outer frame
	}
}

// renderObjectExplorerLayout assembles the three Object Explorer columns from a
// prepared middle column (header + body lines). Shared by the flat field list
// and the tree view.
func renderObjectExplorerLayout(d objectExplorerColumnDims, middleHeader string, fieldLines []string, title string, parentFields []model.ObjectField, parentCursor int, previewYAML string, previewScroll int, filterBar, hintBar string, width int) string {
	titleText := ViewTitle(width, title)

	// The bottom bar shows the filter input when filtering (like the API
	// Explorer's search), otherwise the key hints.
	bottomBar := hintBar
	if filterBar != "" {
		bottomBar = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(
			HelpKeyStyle.Render("/") + BarNormalStyle.Render(filterBar))
	}

	// Left column: the parent level's keys only (no values), with the
	// drilled-into item highlighted (Miller-columns parent pane). Empty at the
	// top level.
	leftHeader := DimStyle.Bold(true).Render("PARENT")
	leftBody := strings.Join(renderObjectKeyList(parentFields, parentCursor, d.leftInner, d.contentHeight-1), "\n")
	leftCol := leftHeader + "\n" + leftBody
	leftCol = PadToHeight(leftCol, d.contentHeight)
	leftCol = FillLinesBg(leftCol, d.leftInner, BaseBg)
	left := BoxHeight(BoxWidth(InactiveColumnStyle, d.leftW), d.contentHeight).MaxHeight(d.contentHeight + 2).Render(leftCol)

	// Middle column: field list with inline values (active).
	middleContent := middleHeader + "\n" + strings.Join(fieldLines, "\n")
	middleContent = PadToHeight(middleContent, d.contentHeight)
	middleContent = FillLinesBg(middleContent, d.middleInner, BaseBg)
	middle := BoxHeight(BoxWidth(ActiveColumnStyle, d.middleW), d.contentHeight).MaxHeight(d.contentHeight + 2).Render(middleContent)

	// Right column: YAML preview of the selected node's subtree (inactive),
	// scrolled by previewScroll.
	var previewBody string
	if strings.TrimSpace(previewYAML) == "" {
		previewBody = DimStyle.Render("(empty)")
	} else {
		previewBody = RenderYAMLContent(scrollYAML(previewYAML, previewScroll), d.rightInner, d.contentHeight-1)
	}
	rightHeader := DimStyle.Bold(true).Render("PREVIEW")
	rightContent := rightHeader + "\n" + previewBody
	rightContent = PadToHeight(rightContent, d.contentHeight)
	rightContent = FillLinesBg(rightContent, d.rightInner, BaseBg)
	right := BoxHeight(BoxWidth(InactiveColumnStyle, d.rightW), d.contentHeight).MaxHeight(d.contentHeight + 2).Render(rightContent)

	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, middle, right)
	framed := explorerFrameStyle().Render(columns)
	return lipgloss.JoinVertical(lipgloss.Left, titleText, framed, bottomBar)
}

// explorerFrameStyle is the outer rounded border that frames the Object/API
// Explorer columns (excluding the breadcrumb and hint bar).
func explorerFrameStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorPrimary)).
		BorderBackground(BaseBg)
}

// scrollYAML drops the first n lines of a YAML block for the preview pane.
func scrollYAML(yamlText string, n int) string {
	if n <= 0 {
		return yamlText
	}
	lines := strings.Split(yamlText, "\n")
	if n >= len(lines) {
		return ""
	}
	return strings.Join(lines[n:], "\n")
}

// resourceNameWidth computes the key-column width, capped at half the pane.
func resourceNameWidth(fields []model.ObjectField, paneInner int) int {
	w := 0
	for _, f := range fields {
		w = max(w, len(f.Key))
	}
	return min(w, paneInner/2)
}

// renderResourceFieldList renders the middle column: each row shows the key,
// an inline value (scalar value colored by type, or a count summary for
// containers), and a trailing "›" marker on drillable rows.
func renderResourceFieldList(fields []model.ObjectField, cursor, scroll, width, maxLines int) []string {
	if len(fields) == 0 {
		lines := make([]string, maxLines)
		lines[0] = DimStyle.Render("(no fields)")
		return lines
	}

	maxScroll := max(len(fields)-maxLines, 0)
	scroll = max(min(scroll, maxScroll), 0)
	nameWidth := resourceNameWidth(fields, width)

	lines := make([]string, 0, maxLines)
	end := min(scroll+maxLines, len(fields))
	for i := scroll; i < end; i++ {
		f := fields[i]
		name := f.Key
		if len(name) > nameWidth {
			name = name[:nameWidth]
		}
		marker := ""
		if f.HasChildren {
			marker = " " + DimStyle.Render("›") // ›
		}

		valueWidth := max(width-nameWidth-6, 4)
		value := Truncate(f.Preview, valueWidth)

		if i == cursor {
			// Selection shown by the full-width active highlight, no cursor arrow.
			plain := fmt.Sprintf("  %-*s  %s", nameWidth, name, value)
			if f.HasChildren {
				plain += " ›"
			}
			if pad := width - lipgloss.Width(plain); pad > 0 {
				plain += strings.Repeat(" ", pad)
			}
			lines = append(lines, SelectedStyle.MaxWidth(width).Render(plain))
			continue
		}
		valueCell := styleYAMLValue(value)
		if f.HasChildren {
			valueCell = DimStyle.Render(value)
		}
		namePart := NormalStyle.Render(fmt.Sprintf("  %-*s", nameWidth, name))
		lines = append(lines, namePart+"  "+valueCell+marker)
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return lines
}

// renderObjectKeyList renders a keys-only list (no values, no markers) for the
// parent pane. The drilled-into row carries the same full-width background
// highlight as the main explorer's selected row. Scrolls to keep it visible.
func renderObjectKeyList(fields []model.ObjectField, cursor, width, maxLines int) []string {
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
		lines = append(lines, renderKeyRow(fields[i].Key, i == cursor, width))
	}
	return lines
}

// renderKeyRow renders one parent-pane key, highlighting the selected row with
// the main explorer's INACTIVE-column style (ParentHighlightStyle, greyish)
// spanning the full column width.
func renderKeyRow(key string, selected bool, width int) string {
	line := Truncate(" "+key, width)
	if !selected {
		return NormalStyle.Render(line)
	}
	if pad := width - lipgloss.Width(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	return ParentHighlightStyle.MaxWidth(width).Render(line)
}
