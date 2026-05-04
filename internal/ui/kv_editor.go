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

// RenderKVEditorEditPane paints the focused edit view that REPLACES
// the compact key/value table while the user is editing a single
// row. The pane shows the key and value as full-width labelled
// regions; the value region renders embedded newlines as actual
// vertical lines so the user can see and edit multi-line content
// (PEM certs, kubeconfigs, anything the SingleLineCell collapse
// would otherwise hide behind a "↵" glyph).
//
// editKeyCursor / editValueCursor are byte offsets into editKey /
// editValue where the "█" cursor block lands. Without them the
// cursor was always pinned to the end of the input, which made
// ←/→ navigation feel broken.
//
// editColumn picks which region carries the cursor block: 0 = key,
// 1 = value. No inline footer hint — the keymap lives in the global
// status bar (overlay_hintbar) so the pane gets the full height.
func RenderKVEditorEditPane(
	editKey string, editKeyCursor int,
	editValue string, editValueCursor int,
	editColumn, width, height int,
) string {
	keyLabel := BarDimStyle.Bold(true).Render("Key:   ")
	valLabel := BarDimStyle.Bold(true).Render("Value: ")

	keyText := insertCursorBlock(editKey, editKeyCursor, editColumn == 0)
	keyText = Truncate(keyText, max(width-len("Key:   "), 4))
	keyRow := lipgloss.NewStyle().Background(BaseBg).Render(keyLabel + BarNormalStyle.Render(keyText))

	// Value region: width-aware wrap to the cell's available height.
	// Reserve 1 row for the key row; the remainder is the value's
	// vertical budget. (Was -2 to reserve a footer hint row, but the
	// hint moved to the global status bar — we use the row for value
	// content instead.)
	valHeight := max(height-1, 1)
	valWidth := max(width-len("Value: "), 4)

	valText := insertCursorBlock(editValue, editValueCursor, editColumn == 1)
	valBody := wrapAndClip(valText, valWidth, valHeight)
	// Pad each value line so the bg fills the row width — without
	// padding, lines shorter than valWidth show terminal-default bg
	// to the right of the text and break the editor's uniform shade.
	valBodyStyled := stylePerLine(valBody, valWidth, BarNormalStyle)
	valRow := lipgloss.NewStyle().Background(BaseBg).Render(valLabel) + valBodyStyled

	return keyRow + "\n" + valRow
}

// insertCursorBlock returns s with a "█" cursor glyph inserted at
// the given byte offset, but only when active is true. Clamps the
// offset to s's bounds so a stale cursor position can't panic.
func insertCursorBlock(s string, cursor int, active bool) string {
	if !active {
		return s
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(s) {
		cursor = len(s)
	}
	return s[:cursor] + "█" + s[cursor:]
}

// wrapAndClip soft-wraps `s` so each visual line is at most maxW
// columns, then clips to at most maxH lines. Returns the wrapped
// content joined with "\n". Doesn't break inside ANSI sequences
// (the editor passes plain text so the simple rune split is safe).
func wrapAndClip(s string, maxW, maxH int) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			out = append(out, "")
			continue
		}
		runes := []rune(line)
		for i := 0; i < len(runes); i += maxW {
			end := min(i+maxW, len(runes))
			out = append(out, string(runes[i:end]))
		}
	}
	if len(out) > maxH {
		out = out[:maxH]
		if maxH > 0 {
			out[maxH-1] = Truncate(out[maxH-1], maxW-1) + "…"
		}
	}
	return strings.Join(out, "\n")
}

// stylePerLine renders each line of `body` through `style.Width(w)`
// so every line ends up exactly w visible columns wide and the bg
// extends across the row. Used for the editor edit pane so cells
// don't fade to terminal-default bg at the right edge.
func stylePerLine(body string, w int, style lipgloss.Style) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = style.Width(w).Render(line)
	}
	return strings.Join(lines, "\n")
}

// SingleLineCell collapses a value to a single visual line that fits
// inside `maxW` columns. Embedded newlines, carriage returns, and tabs
// are replaced with a faint "↵" glyph so the user still sees that the
// raw value was multi-line — without letting lipgloss/table wrap the
// cell vertically (which would expand the row, the table, and break
// the editor's outer dimensions).
//
// Used by every K/V editor renderer for both the key and value cells:
// passing raw multi-line content to lipgloss/table makes the entire
// editor window resize to fit the tallest cell, which the user sees
// as the editor "growing past the screen" instead of truncating.
func SingleLineCell(s string, maxW int) string {
	if s == "" || maxW <= 0 {
		return Truncate(s, maxW)
	}
	// strings.NewReplacer is allocation-friendly here; the inputs are
	// small and we hit this once per visible cell per render.
	flat := strings.NewReplacer(
		"\r\n", " ↵ ",
		"\n", " ↵ ",
		"\r", " ↵ ",
		"\t", "    ",
	).Replace(s)
	return Truncate(flat, maxW)
}

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
