package app

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// BuildCopyTable renders items as a kubectl-style aligned plain-text
// table. The supplied columns are emitted in order; widths are sized
// to the widest value actually present (no terminal-width truncation).
// Empty built-in values render as "<none>"; empty extras render as an
// empty cell. Sort arrows (the leading "↑ " / "↓ " status decorations
// produced by the on-screen renderer) are stripped so the copied
// output is data, not display chrome.
func BuildCopyTable(items []model.Item, columns []string) string {
	if len(columns) == 0 {
		return ""
	}

	// Pre-compute each row's per-column cell value, plus a per-column
	// width across header + every row.
	headers := make([]string, len(columns))
	widths := make([]int, len(columns))
	for i, key := range columns {
		headers[i] = ui.ColumnHeaderLabel(key)
		widths[i] = lipgloss.Width(headers[i])
	}

	rows := make([][]string, len(items))
	for ri, item := range items {
		row := make([]string, len(columns))
		for ci, key := range columns {
			val := copyTableCellValue(&item, key)
			row[ci] = val
			if w := lipgloss.Width(val); w > widths[ci] {
				widths[ci] = w
			}
		}
		rows[ri] = row
	}

	var b strings.Builder
	writeCopyTableRow(&b, headers, widths)
	for _, r := range rows {
		writeCopyTableRow(&b, r, widths)
	}
	return b.String()
}

// copyTableCellValue resolves a column's value for one item. Built-in
// columns read fields directly; extras fall back to the Columns
// key/value list. Empty built-ins render as "<none>" so the copied
// output stays self-explanatory.
func copyTableCellValue(item *model.Item, key string) string {
	var val string
	switch key {
	case "Name":
		val = item.Name
	case "Namespace":
		val = item.Namespace
	case "Ready":
		val = item.Ready
	case "Restarts":
		val = item.Restarts
	case "Status":
		val = item.Status
	case "Age":
		val = item.Age
	default:
		val = ui.GetExtraColumnValue(item, key)
	}
	val = stripCopyTableOrnaments(val)
	if val == "" && isBuiltinCopyColumn(key) {
		return "<none>"
	}
	return val
}

// stripCopyTableOrnaments removes the leading sort-arrow decoration
// ("↑ " / "↓ ") the explorer prepends to certain status values when
// the column is the current sort key. The copied table is data, not
// a screenshot of the display.
func stripCopyTableOrnaments(val string) string {
	if strings.HasPrefix(val, "↑ ") || strings.HasPrefix(val, "↓ ") {
		return val[len("↑ "):]
	}
	return val
}

// isBuiltinCopyColumn reports whether the column is one of the
// non-Name built-ins whose empty value should render as "<none>".
// Extras stay empty so they don't pollute their column with "<none>"
// repeats when the underlying KV is just missing.
func isBuiltinCopyColumn(key string) bool {
	switch key {
	case "Namespace", "Ready", "Restarts", "Status", "Age":
		return true
	}
	return false
}

// writeCopyTableRow writes one space-padded row terminated with "\n".
// Two-space separator between cells; the trailing cell is padded only
// implicitly (its trailing spaces are trimmed by the newline).
func writeCopyTableRow(b *strings.Builder, cells []string, widths []int) {
	for i, cell := range cells {
		b.WriteString(cell)
		if i < len(cells)-1 {
			pad := widths[i] - lipgloss.Width(cell) + 2
			b.WriteString(strings.Repeat(" ", pad))
		}
	}
	b.WriteByte('\n')
}
