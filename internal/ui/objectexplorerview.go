package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// RenderObjectExplorerView renders the Object Explorer: a three-column
// Miller layout (PARENT level | current NAME/VALUE list | YAML PREVIEW of the
// selected node). It mirrors the API Explorer's layout but shows live values:
// each row carries an inline value preview, and the right column renders the
// selected node's subtree as colorized YAML instead of a schema description.
// The drill path and resource identity live in the top breadcrumb, so this
// view has no title line of its own.
func RenderObjectExplorerView(fields []model.ObjectField, cursor, scroll int, parentFields []model.ObjectField, parentCursor int, previewYAML string, previewScroll int, filterBar, hintBar string, width, height int) string {
	// The bottom bar shows the filter input when filtering (like the API
	// Explorer's search), otherwise the key hints.
	bottomBar := hintBar
	if filterBar != "" {
		bottomBar = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(
			HelpKeyStyle.Render("/") + BarNormalStyle.Render(filterBar))
	}

	usable := width - 6
	leftW := max(10, usable*12/100)
	middleW := max(10, usable*51/100)
	rightW := max(10, usable-leftW-middleW)

	contentHeight := max(height-3, 3)

	colPad := 2
	leftInner := max(5, leftW-colPad)
	middleInner := max(5, middleW-colPad)
	rightInner := max(5, rightW-colPad)

	// Left column: the parent level's keys only (no values), with the
	// drilled-into item highlighted (Miller-columns parent pane). Empty at the
	// top level.
	leftHeader := DimStyle.Bold(true).Render("PARENT")
	leftBody := strings.Join(renderObjectKeyList(parentFields, parentCursor, leftInner, contentHeight-1), "\n")
	leftCol := leftHeader + "\n" + leftBody
	leftCol = PadToHeight(leftCol, contentHeight)
	leftCol = FillLinesBg(leftCol, leftInner, BaseBg)
	left := InactiveColumnStyle.Width(leftW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(leftCol)

	// Middle column: field list with inline values (active).
	fieldLines := renderResourceFieldList(fields, cursor, scroll, middleInner, contentHeight-1)
	nameWidth := resourceNameWidth(fields, middleInner)
	middleHeader := DimStyle.Bold(true).Render(fmt.Sprintf("  %-*s  %s", nameWidth, "NAME", "VALUE"))
	middleContent := middleHeader + "\n" + strings.Join(fieldLines, "\n")
	middleContent = PadToHeight(middleContent, contentHeight)
	middleContent = FillLinesBg(middleContent, middleInner, BaseBg)
	middle := ActiveColumnStyle.Width(middleW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(middleContent)

	// Right column: YAML preview of the selected node's subtree (inactive),
	// scrolled by previewScroll.
	var previewBody string
	if strings.TrimSpace(previewYAML) == "" {
		previewBody = DimStyle.Render("(empty)")
	} else {
		previewBody = RenderYAMLContent(scrollYAML(previewYAML, previewScroll), rightInner, contentHeight-1)
	}
	rightHeader := DimStyle.Bold(true).Render("PREVIEW")
	rightContent := rightHeader + "\n" + previewBody
	rightContent = PadToHeight(rightContent, contentHeight)
	rightContent = FillLinesBg(rightContent, rightInner, BaseBg)
	right := InactiveColumnStyle.Width(rightW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(rightContent)

	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, middle, right)
	return lipgloss.JoinVertical(lipgloss.Left, columns, bottomBar)
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
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		marker := ""
		if f.HasChildren {
			marker = " " + DimStyle.Render("›") // ›
		}

		valueWidth := max(width-nameWidth-6, 4)
		value := Truncate(f.Preview, valueWidth)

		if i == cursor {
			plain := fmt.Sprintf("%s%-*s  %s", prefix, nameWidth, name, Truncate(f.Preview, valueWidth))
			if f.HasChildren {
				plain += " ›"
			}
			lines = append(lines, OverlaySelectedStyle.Render(plain))
			continue
		}
		valueCell := styleYAMLValue(value)
		if f.HasChildren {
			valueCell = DimStyle.Render(value)
		}
		namePart := NormalStyle.Render(fmt.Sprintf("%s%-*s", prefix, nameWidth, name))
		lines = append(lines, namePart+"  "+valueCell+marker)
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return lines
}

// renderObjectKeyList renders a keys-only list (no inline values) for the
// parent pane, with the drilled-into row highlighted and a "›" marker on
// drillable keys. Scrolls to keep the cursor visible.
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
		f := fields[i]
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		line := prefix + f.Key
		if f.HasChildren {
			line += " ›"
		}
		line = Truncate(line, width)
		if i == cursor {
			line = OverlaySelectedStyle.Render(line)
		} else {
			line = NormalStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}
