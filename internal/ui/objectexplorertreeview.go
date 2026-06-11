package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// RenderObjectExplorerTreeView renders the Object Explorer's tree mode: the
// same three-column Miller layout, but the middle column shows the expanded
// subtree at the current path with ASCII-art guides (issue #417). When
// filtered is true, rows render their full relative path instead of guides
// (the filter matches keys anywhere in the subtree, so guides would dangle).
func RenderObjectExplorerTreeView(rows []model.ObjectTreeRow, filtered bool, cursor, scroll int, title string, parentFields []model.ObjectField, parentCursor int, previewYAML string, previewScroll int, filterBar, hintBar string, width, height int) string {
	d := objectExplorerDims(width, height)
	labels, prefixes := objectTreeLabels(rows, filtered)
	nameWidth := objectTreeNameWidth(labels, prefixes, d.middleInner)
	fieldLines := renderObjectTreeFieldList(rows, labels, prefixes, nameWidth, cursor, scroll, d.middleInner, d.contentHeight-1)
	middleHeader := DimStyle.Bold(true).Render(fmt.Sprintf("  %-*s  %s", nameWidth, "NAME", "VALUE"))
	return renderObjectExplorerLayout(d, middleHeader, fieldLines, title, parentFields, parentCursor, previewYAML, previewScroll, filterBar, hintBar, width)
}

// objectTreeLabels computes each row's display label and guide prefix. In
// filtered mode the label is the row's relative dotted path and the prefix is
// empty.
func objectTreeLabels(rows []model.ObjectTreeRow, filtered bool) (labels, prefixes []string) {
	labels = make([]string, len(rows))
	prefixes = make([]string, len(rows))
	if filtered {
		for i, r := range rows {
			labels[i] = model.FormatObjectPath(r.Segs)
		}
		return labels, prefixes
	}
	depths := make([]int, len(rows))
	for i, r := range rows {
		depths[i] = r.Depth
		labels[i] = r.Field.Key
	}
	return labels, TreeGuidePrefixes(depths)
}

// objectTreeNameWidth computes the name-column width (guide prefix + label),
// capped at half the pane.
func objectTreeNameWidth(labels, prefixes []string, paneInner int) int {
	w := 0
	for i := range labels {
		w = max(w, lipgloss.Width(prefixes[i]+labels[i]))
	}
	return min(w, paneInner/2)
}

// renderObjectTreeFieldList renders the middle-column rows of the tree view:
// dim guide prefix, key, inline value preview. No drill marker — children are
// already expanded inline.
func renderObjectTreeFieldList(rows []model.ObjectTreeRow, labels, prefixes []string, nameWidth, cursor, scroll, width, maxLines int) []string {
	if len(rows) == 0 {
		lines := make([]string, maxLines)
		lines[0] = DimStyle.Render("(no fields)")
		return lines
	}

	maxScroll := max(len(rows)-maxLines, 0)
	scroll = max(min(scroll, maxScroll), 0)

	lines := make([]string, 0, maxLines)
	end := min(scroll+maxLines, len(rows))
	for i := scroll; i < end; i++ {
		f := rows[i].Field
		prefixW := lipgloss.Width(prefixes[i])
		// Deeply nested rows whose guide prefix eats the whole name column
		// keep a minimum of label space and simply run wider.
		label := Truncate(labels[i], max(nameWidth-prefixW, minTreeLabelWidth))
		pad := max(nameWidth-prefixW-lipgloss.Width(label), 0)

		valueWidth := max(width-nameWidth-6, 4)
		value := Truncate(f.Preview, valueWidth)

		// A container row not followed by a deeper row has folded children
		// (in pre-order a parent's children render right below it). Mark it
		// like the flat view's drillable rows. Guides are off while filtering.
		folded := prefixes[i] != "" && f.HasChildren &&
			(i+1 >= len(rows) || rows[i+1].Depth <= rows[i].Depth)

		if i == cursor {
			// Selection shown by the full-width active highlight, no cursor arrow.
			plain := "  " + prefixes[i] + label + strings.Repeat(" ", pad) + "  " + value
			if folded {
				plain += " ›"
			}
			if p := width - lipgloss.Width(plain); p > 0 {
				plain += strings.Repeat(" ", p)
			}
			lines = append(lines, SelectedStyle.MaxWidth(width).Render(plain))
			continue
		}
		valueCell := styleYAMLValue(value)
		if f.HasChildren {
			valueCell = DimStyle.Render(value)
		}
		marker := ""
		if folded {
			marker = " " + DimStyle.Render("›")
		}
		namePart := DimStyle.Render(prefixes[i]) + NormalStyle.Render(label)
		lines = append(lines, "  "+namePart+strings.Repeat(" ", pad)+"  "+valueCell+marker)
	}

	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return lines
}
