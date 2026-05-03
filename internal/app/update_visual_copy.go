package app

import "strings"

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

// visualCopyChar extracts character-mode visual selection text.
func visualCopyChar(lines []string, selStart, selEnd, anchorCol, cursorCol int, reversed bool) string {
	var parts []string
	startCol, endCol := anchorCol, cursorCol
	if reversed {
		startCol, endCol = cursorCol, anchorCol
	}
	for i := selStart; i <= selEnd; i++ {
		line := lines[i]
		runes := []rune(line)
		if selStart == selEnd {
			cs := min(anchorCol, cursorCol)
			ce := max(anchorCol, cursorCol) + 1
			if cs > len(runes) {
				cs = len(runes)
			}
			if ce > len(runes) {
				ce = len(runes)
			}
			parts = append(parts, string(runes[cs:ce]))
		} else if i == selStart {
			cs := min(startCol, len(runes))
			parts = append(parts, string(runes[cs:]))
		} else if i == selEnd {
			ce := min(endCol+1, len(runes))
			parts = append(parts, string(runes[:ce]))
		} else {
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
		line := lines[i]
		runes := []rune(line)
		cs := colStart
		ce := colEnd
		if cs > len(runes) {
			cs = len(runes)
		}
		if ce > len(runes) {
			ce = len(runes)
		}
		parts = append(parts, string(runes[cs:ce]))
	}
	return strings.Join(parts, "\n")
}
