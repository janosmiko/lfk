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
	lineWidth := ansi.StringWidth(line)

	switch visualType {
	case 'v': // Character visual mode
		return renderCharSelection(line, lineWidth, lineIdx, selStart, selEnd, anchorLine, anchorCol, cursorCol)
	case 'B': // Block (column) visual mode
		return renderBlockSelection(line, lineWidth, colStart, colEnd)
	default: // 'V' or zero value: Line visual mode
		// Strip producer-emitted ANSI before rendering: when the user is
		// in selection mode the selection style owns the visual, and
		// preserving the producer's colors creates collisions (kyverno's
		// blue level on selection's blue bg becomes invisible blue-on-blue,
		// dim grey timestamps on selection bg drop legibility, etc.).
		return SelectedStyle.Render(ansi.Strip(line))
	}
}

// renderCharSelection highlights a character-level selection range. Columns
// are visual columns; embedded ANSI escape sequences in line are treated as
// zero-width so cursor and selection address the same positions.
//
// On the anchor line, highlight from anchorCol to end of line (downward) or
// start of line (upward). On the cursor line, the opposite. Middle lines are
// fully highlighted. When anchor and cursor are on the same line, highlight
// between the two columns.
func renderCharSelection(line string, lineWidth, lineIdx, selStart, selEnd, anchorLine, anchorCol, cursorCol int) string {
	if selStart == selEnd {
		// Single line: highlight between the two column positions.
		cStart := min(anchorCol, cursorCol)
		cEnd := max(anchorCol, cursorCol) + 1
		return highlightColumnRange(line, lineWidth, cStart, cEnd)
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
		return highlightColumnRange(line, lineWidth, startCol, lineWidth)
	}
	if lineIdx == selEnd {
		return highlightColumnRange(line, lineWidth, 0, endCol+1)
	}
	// Middle line: fully highlighted. Strip producer ANSI so the selection
	// style owns the visual presentation (same rule as line-mode V).
	return SelectedStyle.Render(ansi.Strip(line))
}

// renderBlockSelection highlights a rectangular visual-column range on the line.
func renderBlockSelection(line string, lineWidth, colStart, colEnd int) string {
	return highlightColumnRange(line, lineWidth, colStart, colEnd+1)
}

// highlightColumnRange highlights visible characters from colStart (inclusive)
// to colEnd (exclusive). The line may carry producer SGR sequences; the
// before/after segments keep their original styling (ansi.Truncate /
// TruncateLeft preserve embedded ANSI), while the selected slice is rendered
// through SelectedStyle on stripped text so the selection's fg/bg pair
// applies cleanly without colliding with producer colors.
func highlightColumnRange(line string, lineWidth, colStart, colEnd int) string {
	if colStart < 0 {
		colStart = 0
	}
	if colEnd > lineWidth {
		colEnd = lineWidth
	}
	if colStart >= lineWidth {
		// Selection is beyond the line; render the line with a padded
		// selection cell so the user sees where their cursor parked.
		return line + SelectedStyle.Render(" ")
	}
	if colEnd <= colStart {
		return line
	}

	before := ansi.Truncate(line, colStart, "")
	selected := ansi.Strip(ansi.Cut(line, colStart, colEnd))
	after := ansi.TruncateLeft(line, colEnd, "")
	return before + SelectedStyle.Render(selected) + after
}
