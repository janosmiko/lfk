package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// CursorBlockStyle is the reverse-video style used to render a block cursor at a column position.
var CursorBlockStyle = lipgloss.NewStyle().Reverse(true)

// RenderCursorAtCol renders a block cursor at the given visual column within a
// line. If the column is at or beyond the line's visual width, the cursor is
// shown as a highlighted space appended to the line. styledLine is the already-
// styled version (line numbers, syntax highlighting); plainLine is the source
// of truth for column counting and may contain ANSI SGR sequences from the
// underlying log producer (kyverno, klog, etc. when ConfigLogRenderAnsi is on).
// When the cursor is at a negative column the styled line is returned as-is.
//
// The split is ANSI-aware on purpose: rune-indexing across an SGR sequence
// would land the cursor on the ESC byte or a payload digit, and lipgloss
// strips bare ESC bytes when wrapping content with reverse-video codes. The
// remaining "[NNm" payload then leaks as literal text in front of the line.
func RenderCursorAtCol(styledLine, plainLine string, col int) string {
	if col < 0 {
		return styledLine
	}
	visualWidth := ansi.StringWidth(plainLine)
	if col >= visualWidth {
		// Cursor is past end of line: append a highlighted space.
		return styledLine + CursorBlockStyle.Render(" ")
	}
	before := ansi.Truncate(plainLine, col, "")
	cursorChar := ansi.Strip(ansi.Cut(plainLine, col, col+1))
	after := ansi.TruncateLeft(plainLine, col+1, "")
	return before + CursorBlockStyle.Render(cursorChar) + after
}

// RenderVisualSelection renders a line with visual selection highlighting
// based on the visual mode type ('V' = line, 'v' = char, 'B' = block).
//
// Parameters:
//   - line: the plain text line to render
//   - visualType: 'V' for line, 'v' for char, 'B' for block
//   - lineIdx: the index of this line in the document
//   - selStart, selEnd: the range of selected line indices (min/max of anchor and cursor)
//   - anchorLine: the original anchor line (before min/max), needed for direction detection
//   - anchorCol, cursorCol: the column positions of the anchor and cursor
//   - colStart, colEnd: min/max of anchorCol and cursorCol (precomputed)
func RenderVisualSelection(line string, visualType rune, lineIdx, selStart, selEnd, anchorLine, anchorCol, cursorCol, colStart, colEnd int) string {
	runes := []rune(line)
	lineLen := len(runes)

	switch visualType {
	case 'v': // Character visual mode
		return renderCharSelection(runes, lineLen, lineIdx, selStart, selEnd, anchorLine, anchorCol, cursorCol)
	case 'B': // Block (column) visual mode
		return renderBlockSelection(runes, lineLen, colStart, colEnd)
	default: // 'V' or zero value: Line visual mode
		// Strip producer-emitted ANSI before rendering: when the user is
		// in selection mode the selection style owns the visual, and
		// preserving the producer's colors creates collisions (kyverno's
		// blue level on selection's blue bg becomes invisible blue-on-blue,
		// dim grey timestamps on selection bg drop legibility, etc.).
		// Restoring producer color across embedded resets — the previous
		// approach — kept the bg alive but couldn't avoid these
		// foreground/background collisions.
		return SelectedStyle.Render(ansi.Strip(line))
	}
}

// renderCharSelection highlights a character-level selection range.
// anchorLine is the original (non-min/maxed) anchor line for direction detection.
// On the anchor line, highlight from anchorCol to end of line (downward) or start of line (upward).
// On the cursor line, the opposite. Middle lines are fully highlighted.
// When anchor and cursor are on the same line, highlight between the two columns.
func renderCharSelection(runes []rune, lineLen, lineIdx, selStart, selEnd, anchorLine, anchorCol, cursorCol int) string {
	if selStart == selEnd {
		// Single line: highlight between the two column positions.
		cStart := min(anchorCol, cursorCol)
		cEnd := max(anchorCol, cursorCol) + 1
		return highlightColumnRange(runes, lineLen, cStart, cEnd)
	}

	// Determine direction: anchor is at selStart (downward) or selEnd (upward).
	// startCol/endCol are the columns for selStart/selEnd lines respectively.
	var startCol, endCol int
	if anchorLine <= selStart {
		// Downward: anchor is at top, cursor at bottom.
		startCol = anchorCol
		endCol = cursorCol
	} else {
		// Upward: cursor is at top, anchor at bottom.
		startCol = cursorCol
		endCol = anchorCol
	}

	if lineIdx == selStart {
		return highlightColumnRange(runes, lineLen, startCol, lineLen)
	}
	if lineIdx == selEnd {
		return highlightColumnRange(runes, lineLen, 0, endCol+1)
	}
	// Middle line: fully highlighted.
	return SelectedStyle.Render(string(runes))
}

// renderBlockSelection highlights a rectangular column range on the line.
func renderBlockSelection(runes []rune, lineLen, colStart, colEnd int) string {
	return highlightColumnRange(runes, lineLen, colStart, colEnd+1)
}

// highlightColumnRange highlights characters from colStart (inclusive) to colEnd (exclusive).
func highlightColumnRange(runes []rune, lineLen, colStart, colEnd int) string {
	if colStart < 0 {
		colStart = 0
	}
	if colEnd > lineLen {
		colEnd = lineLen
	}
	if colStart >= lineLen {
		// Selection is beyond the line; just render the line with padding.
		return string(runes) + SelectedStyle.Render(" ")
	}
	if colEnd <= colStart {
		// No selection on this line.
		return string(runes)
	}

	var result string
	if colStart > 0 {
		result += string(runes[:colStart])
	}
	result += SelectedStyle.Render(string(runes[colStart:colEnd]))
	if colEnd < lineLen {
		result += string(runes[colEnd:])
	}
	return result
}
