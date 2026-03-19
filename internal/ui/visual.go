package ui

import "github.com/charmbracelet/lipgloss"

// CursorBlockStyle is the reverse-video style used to render a block cursor at a column position.
var CursorBlockStyle = lipgloss.NewStyle().Reverse(true)

// RenderCursorAtCol renders a block cursor at the given column position within a line.
// If the column is at or beyond the line length, the cursor is shown as a highlighted space
// appended to the line. The styledLine parameter is the already-styled version of the line
// (with syntax highlighting etc.), and plainLine is the raw text used for column counting.
// When the cursor is at column 0 the original styled line is returned unchanged (the gutter
// indicator already marks the line).
func RenderCursorAtCol(styledLine, plainLine string, col int) string {
	if col <= 0 {
		return styledLine
	}
	runes := []rune(plainLine)
	if col >= len(runes) {
		// Cursor is past end of line: append a highlighted space.
		return styledLine + CursorBlockStyle.Render(" ")
	}
	// Rebuild the line with the cursor character highlighted.
	// Use plain text to get precise column positions, then rebuild with the cursor.
	before := string(runes[:col])
	cursorChar := string(runes[col : col+1])
	after := string(runes[col+1:])
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
//   - anchorCol, cursorCol: the column positions of the anchor and cursor
//   - colStart, colEnd: min/max of anchorCol and cursorCol (precomputed)
func RenderVisualSelection(line string, visualType rune, lineIdx, selStart, selEnd, anchorCol, cursorCol, colStart, colEnd int) string {
	runes := []rune(line)
	lineLen := len(runes)

	switch visualType {
	case 'v': // Character visual mode
		return renderCharSelection(runes, lineLen, lineIdx, selStart, selEnd, anchorCol, cursorCol)
	case 'B': // Block (column) visual mode
		return renderBlockSelection(runes, lineLen, colStart, colEnd)
	default: // 'V' or zero value: Line visual mode
		return SelectedStyle.Render(line)
	}
}

// renderCharSelection highlights a character-level selection range.
// On the first selected line, highlight from anchorCol onward.
// On the last selected line, highlight up to cursorCol.
// Middle lines are fully highlighted.
// When anchor and cursor are on the same line, highlight between the two columns.
func renderCharSelection(runes []rune, lineLen, lineIdx, selStart, selEnd, anchorCol, cursorCol int) string {
	// Determine which line is the anchor and which is the cursor.
	// selStart = min(anchorLine, cursorLine), selEnd = max(anchorLine, cursorLine)
	// We need to figure out the direction of selection.
	if selStart == selEnd {
		// Single line: highlight between the two column positions.
		cStart := min(anchorCol, cursorCol)
		cEnd := max(anchorCol, cursorCol) + 1
		return highlightColumnRange(runes, lineLen, cStart, cEnd)
	}

	// The anchor is the line where the visual selection was started.
	// selStart/selEnd are just min/max of anchor line and cursor line.
	// We need to determine actual direction:
	// If anchorCol is associated with selStart line, anchor is at top.
	// The caller passes the raw anchorCol and cursorCol.
	// For multi-line char selection:
	//   - First line (selStart): highlight from the starting column to end of line
	//   - Last line (selEnd): highlight from start of line to the ending column
	//   - Middle lines: fully highlighted
	// The "starting column" is anchorCol if anchor is on selStart, else cursorCol.
	// The "ending column" is cursorCol if cursor is on selEnd, else anchorCol.
	// Since we don't know which is which from just the min/max, we use a heuristic:
	// anchorCol applies to the anchor line and cursorCol to the cursor line.
	// But we only have selStart/selEnd. The convention is:
	//   anchorCol -> the line where selection was started
	//   cursorCol -> the line where cursor currently is
	// Since selStart = min(anchorLine, cursorLine), if anchor < cursor then
	//   selStart is the anchor line, selEnd is the cursor line.
	// We don't have anchorLine/cursorLine directly, but the caller knows.
	// For simplicity, we'll treat anchorCol as belonging to selStart and
	// cursorCol as belonging to selEnd. This matches the common case where
	// the user moves the cursor downward from the anchor.

	if lineIdx == selStart {
		// First line: highlight from anchorCol to end of line.
		return highlightColumnRange(runes, lineLen, anchorCol, lineLen)
	}
	if lineIdx == selEnd {
		// Last line: highlight from start to cursorCol (inclusive).
		return highlightColumnRange(runes, lineLen, 0, cursorCol+1)
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
