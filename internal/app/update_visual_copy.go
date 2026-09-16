package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/ui"
)

// visualCopyText extracts text from lines based on visual selection mode.
// This is shared between describe and diff visual copy.
func visualCopyText(lines []string, selStart, selEnd int, mode rune, anchorCol, cursorCol int, reversed bool) string {
	switch mode {
	case 'v':
		return visualCopyChar(lines, selStart, selEnd, anchorCol, cursorCol, reversed)
	case 'B':
		return visualCopyBlock(lines, selStart, selEnd, anchorCol, cursorCol)
	default:
		var parts []string
		for i := selStart; i <= selEnd; i++ {
			parts = append(parts, lines[i])
		}
		return strings.Join(parts, "\n")
	}
}

// visualCopyChar extracts character-mode visual selection text. Columns are
// cell columns, the same unit the selection highlight is drawn in, so the
// clipboard holds exactly the text the user saw highlighted.
func visualCopyChar(lines []string, selStart, selEnd, anchorCol, cursorCol int, reversed bool) string {
	var parts []string
	startCol, endCol := anchorCol, cursorCol
	if reversed {
		startCol, endCol = cursorCol, anchorCol
	}
	for i := selStart; i <= selEnd; i++ {
		line := lines[i]
		w := ui.LineWidth(line)
		switch {
		case selStart == selEnd:
			parts = append(parts, ui.CutCols(line, min(anchorCol, cursorCol), max(anchorCol, cursorCol)+1))
		case i == selStart:
			parts = append(parts, ui.CutCols(line, startCol, w))
		case i == selEnd:
			parts = append(parts, ui.CutCols(line, 0, endCol+1))
		default:
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, "\n")
}

// visualCopyBlock extracts block-mode visual selection text.
func visualCopyBlock(lines []string, selStart, selEnd, col1, col2 int) string {
	colStart := min(col1, col2)
	colEnd := max(col1, col2) + 1
	var parts []string
	for i := selStart; i <= selEnd; i++ {
		parts = append(parts, ui.CutCols(lines[i], colStart, colEnd))
	}
	return strings.Join(parts, "\n")
}
