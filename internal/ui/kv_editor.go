package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// kv_editor.go holds rendering primitives shared by the three K/V
// editor overlays (secret, configmap, label). Each editor used to
// hand-roll its own ASCII table with `|` / `-` separators and manual
// padding; centralising the rendering on lipgloss/table here keeps
// the three editors visually consistent and removes ~60 lines of
// near-identical code per editor.

// FilterKVKeys narrows `keys` to entries that contain `query` as a
// case-insensitive substring. Empty query returns the input unchanged.
// Used by the K/V editor renderers to apply the / search filter
// without forcing the editor to mutate its source data structure.
func FilterKVKeys(keys []string, query string) []string {
	if query == "" {
		return keys
	}
	q := strings.ToLower(query)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if strings.Contains(strings.ToLower(k), q) {
			out = append(out, k)
		}
	}
	return out
}

// RenderKVEditorSearchBar paints the / search bar shown above the
// editor table when search is active or the query is non-empty.
// Layout: "/ <query>█" while typing, "/ <query>" otherwise. Returns
// "" when there's nothing to render so the caller can omit the row.
func RenderKVEditorSearchBar(query string, active bool) string {
	if !active && query == "" {
		return ""
	}
	prefix := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSecondary)).
		Bold(true).
		Background(BaseBg).
		Render("/")
	body := query
	if active {
		body += "█"
	}
	return prefix + " " + lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorFile)).
		Background(BaseBg).
		Render(body)
}

// computeKeyColumnWidth picks a key-column width that fits the longest
// key in `keys`, clamped to a fraction of the table's total width
// (`totalWidth / divisor`). Floor of 10 ensures the column stays usable
// when all keys are very short.
func computeKeyColumnWidth(keys []string, totalWidth, divisor int) int {
	w := 0
	for _, k := range keys {
		if len(k) > w {
			w = len(k)
		}
	}
	if w < 10 {
		w = 10
	}
	if w > totalWidth/divisor {
		w = totalWidth / divisor
	}
	return w
}

// scrollWindowStart returns the first visible row index so `cursor`
// stays in view inside a window of `windowH` rows. Vim-like contract:
// if the cursor is already inside [0, windowH), no scroll; otherwise
// pull just enough so the cursor lands on the last visible row.
func scrollWindowStart(cursor, windowH, total int) int {
	if total <= windowH {
		return 0
	}
	if cursor < windowH {
		return 0
	}
	start := cursor - windowH + 1
	start = min(start, total-windowH)
	return start
}

// newKVEditorTable builds a lipgloss/table configured for the K/V
// editor look: vertical column divider only, header underline, theme-
// aware bg, and per-row styling that highlights the cursor row.
//
// cursorRow is the body-row index (0-based; lipgloss/table's HeaderRow
// is the constant -1) of the cursor. Pass -1 when no row is the cursor
// (e.g. an empty table that's only showing headers + a placeholder).
func newKVEditorTable(keyColW, valColW, cursorRow int) *table.Table {
	return table.New().
		Border(lipgloss.NormalBorder()).
		BorderRow(false).
		BorderColumn(true).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderStyle(lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBorder)).
			Background(BaseBg)).
		Headers("KEY", "VALUE").
		StyleFunc(func(row, col int) lipgloss.Style {
			// Padding eats text space; widen the cell by the padding
			// budget (1 + 1 = 2) so the row-data Truncate's full
			// keyColW / valColW characters can land inside without
			// wrapping into a second visual line.
			base := lipgloss.NewStyle().Padding(0, 1).Background(BaseBg)
			cellW := valColW + 2
			if col == 0 {
				cellW = keyColW + 2
			}
			base = base.Width(cellW)
			switch {
			case row == table.HeaderRow:
				return base.
					Foreground(lipgloss.Color(ColorPrimary)).
					Bold(true).
					Underline(true)
			case row == cursorRow:
				return base.
					Foreground(lipgloss.Color(ColorSelectedFg)).
					Background(lipgloss.Color(ColorSelectedBg)).
					Bold(true)
			case col == 0:
				return base.Foreground(lipgloss.Color(ColorSecondary)).Bold(true)
			default:
				return base.Foreground(lipgloss.Color(ColorDimmed))
			}
		})
}
