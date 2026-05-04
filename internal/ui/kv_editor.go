package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/ansi"
)

// kv_editor.go holds rendering primitives shared by the three K/V
// editor overlays (secret, configmap, label). Each editor used to
// hand-roll its own ASCII table with `|` / `-` separators and manual
// padding; centralising the rendering on lipgloss/table here keeps
// the three editors visually consistent and removes ~60 lines of
// near-identical code per editor.

// RenderKVEditorEditPane paints the focused edit view that REPLACES
// the compact key/value table while the user is editing a single
// row. Layout: bordered Key and Value field boxes stacked
// vertically. Active field's border picks up ColorPrimary so the
// user sees which one Tab will swap into.
//
// editKeyCursor / editValueCursor are byte offsets where the cursor
// "block" lands. The cursor renders as inverse-video on the
// CHARACTER at the offset (not as an inserted "█") so moving the
// cursor doesn't shift the rest of the text by a column — the user
// reported the previous insert-style cursor felt like the text
// jumped around as they typed/navigated.
//
// editColumn picks the active field: 0 = key, 1 = value. No inline
// footer hint — the keymap lives in the global status bar
// (overlay_hintbar) so the pane gets the full height for content.
func RenderKVEditorEditPane(
	editKey string, editKeyCursor int,
	editValue string, editValueCursor int,
	editColumn, width, height int,
) string {
	const (
		labelKey = "  Key  "
		labelVal = "  Value  "
	)

	// Field-box dimensions: two fields share the available height.
	// Key gets one content row; Value gets the rest.
	fieldOuterW := max(width, 12)
	keyContentH := 1
	valContentH := max(height-keyContentH-4, 1) // -4: 2 borders for each box's top+bottom = 4 rows total chrome

	keyActive := editColumn == 0
	valActive := editColumn == 1

	keyContent := overlayCursor(editKey, editKeyCursor, keyActive, fieldOuterW-4)
	valContent := overlayCursorMultiline(editValue, editValueCursor, valActive, fieldOuterW-4, valContentH)

	keyBox := kvFieldBox(labelKey, keyContent, keyActive, fieldOuterW, keyContentH)
	valBox := kvFieldBox(labelVal, valContent, valActive, fieldOuterW, valContentH)

	return keyBox + "\n" + valBox
}

// kvFieldBox wraps `content` in a labelled bordered box. Active
// fields get an accent border color; idle fields use the standard
// border color. The box's bg matches the editor's baseBg so it
// doesn't paint a different shade against the surrounding pane.
func kvFieldBox(label, content string, active bool, outerW, contentH int) string {
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderBackground(BaseBg).
		Background(BaseBg).
		Padding(0, 1).
		Width(outerW - 2). // -2 for left/right borders
		Height(contentH)

	if active {
		border = border.BorderForeground(lipgloss.Color(ColorPrimary))
	} else {
		border = border.BorderForeground(lipgloss.Color(ColorBorder))
	}

	// Render the field with its content, then splice the label over
	// the top-border row so it reads as "╭ Key ───╮" / "╭ Value ──╮".
	rendered := border.Render(content)
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}
	top := lines[0]
	labelStyle := BarDimStyle
	if active {
		labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorPrimary)).
			Background(BaseBg).
			Bold(true)
	}
	styledLabel := labelStyle.Render(label)
	labelW := lipgloss.Width(styledLabel)
	topW := lipgloss.Width(top)
	if 1+labelW <= topW {
		// Splice the label into the styled top border. The original
		// `top` is ANSI-styled (border fg + bg SGR sequences around
		// every border char), so naive `[]rune(top)` slicing counts
		// the escape bytes as runes and lands inside an SGR — leaving
		// the tail visible as raw text (the user reported
		// "Value  ;162;247;48;2;36;40;59m╭───" symptom). ansi.Cut
		// is grapheme- and ANSI-aware: it returns the slice between
		// visual columns [left, right) with escape sequences preserved.
		prefix := ansi.Cut(top, 0, 1)           // styled "╭"
		suffix := ansi.Cut(top, 1+labelW, topW) // styled "──...─╮"
		lines[0] = prefix + styledLabel + suffix
	}
	return strings.Join(lines, "\n")
}

// overlayCursor renders s with the character at `cursor` shown in
// inverse video (active) or returns s unchanged (inactive). When
// cursor is at len(s) a single space gets the inverse style so the
// indicator stays visible at the end of the input.
//
// Reverse-video instead of an inserted "█" block: inserting shifts
// every character to the right of the cursor by one visual column
// every time the cursor moves, which the user reported as the text
// "jumping around" while typing / navigating.
func overlayCursor(s string, cursor int, active bool, maxW int) string {
	if !active {
		return Truncate(s, maxW)
	}
	cursor = clampInt(cursor, 0, len(s))
	cursorStyle := lipgloss.NewStyle().Reverse(true).Background(BaseBg)
	var head, ch, tail string
	if cursor == len(s) {
		head = s
		ch = " "
	} else {
		end := nextRuneEnd(s, cursor)
		head = s[:cursor]
		ch = s[cursor:end]
		tail = s[end:]
	}
	out := head + cursorStyle.Render(ch) + tail
	return Truncate(out, maxW)
}

// overlayCursorMultiline soft-wraps s to maxW columns and clips to
// maxH lines, optionally overlaying a reverse-video cursor at the
// byte offset. The wrap is performed on the plain source so the
// ANSI sequences from the cursor styling don't break the column
// math (a previous wrapAndClip-then-style approach miscounted runes
// when escape sequences landed inside the wrap window).
//
// When `active` is true the visible window scrolls to keep the
// cursor in view: if the cursor's visual line lies past the last
// visible row, the slice shifts down so the cursor lands on the
// bottom row. Without this the user couldn't reach the end of long
// secret values — the trailing lines were silently truncated to
// maxH and the cursor placed on a line that never made it on screen.
func overlayCursorMultiline(s string, cursor int, active bool, maxW, maxH int) string {
	if maxW <= 0 || maxH <= 0 {
		return ""
	}
	cursorStyle := lipgloss.NewStyle().Reverse(true).Background(BaseBg)
	cursor = clampInt(cursor, 0, len(s))

	var lines []string
	var cur []byte
	visualCol := 0
	cursorPlaced := false
	cursorLine := -1 // visual line index where the cursor was placed

	flush := func() {
		lines = append(lines, string(cur))
		cur = cur[:0]
		visualCol = 0
	}
	emitCursor := func(text string) {
		cur = append(cur, []byte(cursorStyle.Render(text))...)
		visualCol++             // text is exactly one rune (or " ") so always 1 visual col
		cursorLine = len(lines) // index of the line currently being built
	}

	i := 0
	for i < len(s) {
		// Place cursor at this position before consuming the next char.
		if active && !cursorPlaced && i == cursor {
			if s[i] == '\n' {
				// Cursor on a newline — show " " mark at end of the
				// current line, then process the newline as normal.
				emitCursor(" ")
				flush()
				cursorPlaced = true
				i++
				continue
			}
			end := nextRuneEnd(s, i)
			emitCursor(s[i:end])
			cursorPlaced = true
			i = end
			if visualCol >= maxW {
				flush()
			}
			continue
		}
		if s[i] == '\n' {
			flush()
			i++
			continue
		}
		end := nextRuneEnd(s, i)
		cur = append(cur, s[i:end]...)
		visualCol++
		i = end
		if visualCol >= maxW {
			flush()
		}
	}
	if active && !cursorPlaced && cursor == len(s) {
		emitCursor(" ")
	}
	if len(cur) > 0 {
		lines = append(lines, string(cur))
	}

	// Scroll-to-cursor: when active and the cursor sits below the
	// visible window, slide the window down so the cursor lands on
	// the bottom row. When inactive (or no cursor placed) keep the
	// historical behaviour of clipping from the top.
	scroll := 0
	if active && cursorLine >= maxH {
		scroll = cursorLine - maxH + 1
	}
	end := min(scroll+maxH, len(lines))
	lines = lines[scroll:end]
	return stylePerLine(strings.Join(lines, "\n"), maxW, BarNormalStyle)
}

// nextRuneEnd returns the byte offset of the next rune after the
// rune starting at i. Handles multi-byte UTF-8 by skipping
// continuation bytes (0b10xxxxxx).
func nextRuneEnd(s string, i int) int {
	end := i + 1
	for end < len(s) && (s[end]&0xC0) == 0x80 {
		end++
	}
	return end
}

// clampInt restricts v to [lo, hi] inclusive.
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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
