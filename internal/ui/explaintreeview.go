package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// RenderExplainTreeView renders the API Explorer's tree mode: the same
// three-column Miller layout, but the middle column shows the recursive field
// tree of the current schema path with ASCII-art guides (issue #417). depths
// holds each field's nesting depth relative to the tree root; folded marks
// rows whose subtree is folded away (rendered with a trailing "›"). Recursive
// kubectl explain output carries no per-field descriptions, so the right
// column shows the selected field's name/type only.
func RenderExplainTreeView(fields []model.ExplainField, depths []int, folded []bool, cursor, scroll int, resourceDesc, title string, parentFields []model.ExplainField, parentCursor int, searchQuery, hintBar string, width, height int) string {
	d := objectExplorerDims(width, height)
	if len(depths) != len(fields) {
		// Defensive: a fields/depths mismatch must not panic the renderer;
		// degrade to a flat (all depth-0) tree.
		depths = make([]int, len(fields))
	}
	if len(folded) != len(fields) {
		folded = make([]bool, len(fields))
	}
	prefixes := TreeGuidePrefixes(depths)
	nameWidth := 0
	for i, f := range fields {
		nameWidth = max(nameWidth, lipgloss.Width(prefixes[i]+f.Name))
	}
	nameWidth = min(nameWidth, d.middleInner/2)
	fieldLines := renderExplainTreeFieldList(fields, prefixes, folded, nameWidth, cursor, scroll, d.middleInner, d.contentHeight-1, searchQuery)
	middleHeader := DimStyle.Bold(true).Render(fmt.Sprintf("  %-*s  %s", nameWidth, "NAME", "TYPE"))
	return renderExplainLayout(d, middleHeader, fieldLines, fields, cursor, resourceDesc, title, parentFields, parentCursor, hintBar, width)
}

// renderExplainTreeFieldList renders the middle-column rows of the API
// Explorer tree view: dim guide prefix, field name, type, and a "›" marker on
// folded rows.
func renderExplainTreeFieldList(fields []model.ExplainField, prefixes []string, folded []bool, nameWidth, cursor, scroll, width, maxLines int, searchQuery string) []string {
	if len(fields) == 0 {
		lines := make([]string, maxLines)
		lines[0] = DimStyle.Render("No fields found")
		return lines
	}

	maxScroll := max(len(fields)-maxLines, 0)
	scroll = max(min(scroll, maxScroll), 0)

	lines := make([]string, 0, maxLines)
	end := min(scroll+maxLines, len(fields))
	for i := scroll; i < end; i++ {
		f := fields[i]
		prefixW := lipgloss.Width(prefixes[i])
		// Deeply nested rows whose guide prefix eats the whole name column
		// keep a minimum of label space and simply run wider.
		name := Truncate(f.Name, max(nameWidth-prefixW, minTreeLabelWidth))
		pad := max(nameWidth-prefixW-lipgloss.Width(name), 0)

		typeStr := Truncate(f.Type, max(width-nameWidth-6, 4))

		if i == cursor {
			plain := "  " + prefixes[i] + fmt.Sprintf("%-*s", lipgloss.Width(name)+pad, name) + "  " + typeStr
			if folded[i] {
				plain += " ›"
			}
			if p := width - lipgloss.Width(plain); p > 0 {
				plain += strings.Repeat(" ", p)
			}
			lines = append(lines, SelectedStyle.MaxWidth(width).Render(plain))
			continue
		}
		marker := ""
		if folded[i] {
			marker = " " + DimStyle.Render("›")
		}
		namePart := DimStyle.Render(prefixes[i]) + highlightName(name, searchQuery)
		lines = append(lines, "  "+namePart+strings.Repeat(" ", pad)+DimStyle.Render("  "+typeStr)+marker)
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return lines
}
