package ui

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// CursorBlockStyle is the reverse-video style used to render a block cursor at a column position.
var CursorBlockStyle = lipgloss.NewStyle().Reverse(true)

// RenderCursorAtCol renders a block cursor at the given visual column of
// styledLine, which carries any already-applied ANSI styling (YAML/diff/log
// colors) that must survive around the cursor. Negative col returns the
// line as-is; col past the visual width appends a highlighted space.
//
// The split is ANSI-aware: rune-indexing across an SGR sequence would land
// the cursor on the ESC byte or a payload digit, and lipgloss strips bare
// ESC bytes when wrapping content with reverse-video codes.
func RenderCursorAtCol(styledLine string, col int) string {
	if col < 0 {
		return styledLine
	}
	visualWidth := ansi.StringWidth(styledLine)
	if col >= visualWidth {
		// Cursor is past end of line: append a highlighted space.
		return styledLine + CursorBlockStyle.Render(" ")
	}
	// A double-width glyph needs both its columns, or the cut yields an empty
	// cell and the glyph disappears.
	start := SnapColStart(styledLine, col)
	end := SnapColEnd(styledLine, start+1)
	before := ansi.Truncate(styledLine, start, "")
	cursorChar := ansi.Strip(ansi.Cut(styledLine, start, end))
	after := ansi.TruncateLeft(styledLine, end, "")
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
// are visual columns. Embedded ANSI escape sequences in line are treated as
// zero-width so cursor and selection address the same positions.
//
// On the anchor line, highlight from anchorCol to end of line (downward) or
// start of line (upward). On the cursor line, the opposite. Middle lines are
// fully highlighted. When anchor and cursor are on the same line, highlight
// between the two columns.
func renderCharSelection(line string, lineWidth, lineIdx, selStart, selEnd, anchorLine, anchorCol, cursorCol int) string {
	start, end, whole := charSelectionRange(lineWidth, lineIdx, selStart, selEnd, anchorLine, anchorCol, cursorCol)
	if whole {
		// Strip producer ANSI so the selection style owns the visual
		// presentation (same rule as line-mode V).
		return SelectedStyle.Render(ansi.Strip(line))
	}
	return highlightColumnRange(line, lineWidth, start, end)
}

// charSelectionRange returns the half-open column range selected on lineIdx in
// character visual mode. whole reports a line lying between anchor and cursor,
// which is selected end to end.
func charSelectionRange(lineWidth, lineIdx, selStart, selEnd, anchorLine, anchorCol, cursorCol int) (start, end int, whole bool) {
	if selStart == selEnd {
		return min(anchorCol, cursorCol), max(anchorCol, cursorCol) + 1, false
	}

	// Anchor sits at selStart when the selection grew downward, at selEnd when
	// it grew upward.
	startCol, endCol := anchorCol, cursorCol
	if anchorLine > selStart {
		startCol, endCol = cursorCol, anchorCol
	}

	switch lineIdx {
	case selStart:
		return startCol, lineWidth, false
	case selEnd:
		return 0, endCol + 1, false
	default:
		return 0, 0, true
	}
}

// SubLine locates one wrapped sub-line inside its source line, so a renderer
// can translate selection and cursor columns into the slice of the line this
// sub-line actually shows.
type SubLine struct {
	Text string
	// Base is the source column where Text starts.
	Base int
	// SrcWidth is the width of the whole source line.
	SrcWidth int
	// WrapWidth is how many columns the rendered row can hold.
	WrapWidth int
}

// Width returns the visual width of the sub-line.
func (s SubLine) Width() int { return ansi.StringWidth(s.Text) }

// OwnsCursor reports whether the block cursor at source column col belongs on
// this sub-line. A cursor past the end of the source line goes on the last one.
func (s SubLine) OwnsCursor(col int) bool {
	w := s.Width()
	if col >= s.SrcWidth {
		return s.Base+w >= s.SrcWidth
	}
	return col >= s.Base && col < s.Base+w
}

// ClampCol converts source column col into this sub-line's column space. A
// marker parked past a sub-line that already fills the row widens it by one,
// and the viewer's width guard then truncates the marker away.
func (s SubLine) ClampCol(col int) int {
	local := col - s.Base
	w := s.Width()
	if local >= w && w >= s.WrapWidth && w > 0 {
		return w - 1
	}
	return local
}

// RenderVisualSelectionSub renders one wrapped sub-line. Columns arrive in
// source-line space, so a sub-line past the first must shift them or the
// highlight lands on the wrong text. Other arguments match RenderVisualSelection.
func RenderVisualSelectionSub(s SubLine, visualType rune, lineIdx, selStart, selEnd, anchorLine, anchorCol, cursorCol int) string {
	switch visualType {
	case 'v':
		start, end, whole := charSelectionRange(s.SrcWidth, lineIdx, selStart, selEnd, anchorLine, anchorCol, cursorCol)
		if whole {
			return SelectedStyle.Render(ansi.Strip(s.Text))
		}
		return s.highlightRange(start, end)
	case 'B':
		return s.highlightRange(min(anchorCol, cursorCol), max(anchorCol, cursorCol)+1)
	default: // 'V' or zero value: line mode selects every sub-line whole.
		return SelectedStyle.Render(ansi.Strip(s.Text))
	}
}

// highlightRange highlights the part of source range [start, end) inside this
// sub-line. A sub-line the range misses stays untouched: highlightColumnRange's
// past-the-line fallback would park a cell on every sub-line of a wrapped line.
func (s SubLine) highlightRange(start, end int) string {
	w := s.Width()
	lo := max(start-s.Base, 0)
	hi := min(end-s.Base, w)
	if hi > lo {
		return highlightColumnRange(s.Text, w, lo, hi)
	}
	if end <= start || start < s.SrcWidth || s.Base+w < s.SrcWidth {
		return s.Text
	}
	// The selection is parked past the end of the source line: an empty line,
	// or a cursor that ran off the last column. Show it on the final sub-line.
	c := s.ClampCol(start)
	if c < 0 || c >= w {
		return s.Text + SelectedStyle.Render(" ")
	}
	return highlightColumnRange(s.Text, w, c, c+1)
}

// renderBlockSelection highlights a rectangular visual-column range on the line.
func renderBlockSelection(line string, lineWidth, colStart, colEnd int) string {
	return highlightColumnRange(line, lineWidth, colStart, colEnd+1)
}

// highlightColumnRange highlights visible characters from colStart (inclusive)
// to colEnd (exclusive). The line may carry producer SGR sequences. The
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
		// Selection is beyond the line. Render the line with a padded
		// selection cell so the user sees where their cursor parked.
		return line + SelectedStyle.Render(" ")
	}
	if colEnd <= colStart {
		return line
	}

	// Widen to whole characters: a range ending mid-glyph would drop it from
	// both the highlight and the tail after it.
	colStart = SnapColStart(line, colStart)
	colEnd = SnapColEnd(line, colEnd)
	before := ansi.Truncate(line, colStart, "")
	selected := ansi.Strip(ansi.Cut(line, colStart, colEnd))
	after := ansi.TruncateLeft(line, colEnd, "")
	return before + SelectedStyle.Render(selected) + after
}
