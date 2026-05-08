package app

import (
	"fmt"
	"strings"
)

// formatCopiedLines returns the status message for an N-line yank.
// Singular for n=1, plural otherwise — preserves the existing
// "Copied 1 line" string for the unprefixed `y` path.
func formatCopiedLines(n int) string {
	if n == 1 {
		return "Copied 1 line"
	}
	return fmt.Sprintf("Copied %d lines", n)
}

// formatVisualYank returns the status message for a visual-mode yank.
// Multi-line selections (line-mode 'V' or any selection spanning newlines)
// report a line count; single-line char/block selections just say "Copied"
// — a character count after `viw` adds no useful information and saying
// "Copied 1 line" was misleading because only a word landed on the
// clipboard. lineCount is the number of source lines spanned by the
// selection.
func formatVisualYank(clipText string, mode rune, lineCount int) string {
	if mode != 'V' && !strings.Contains(clipText, "\n") {
		return "Copied"
	}
	return formatCopiedLines(lineCount)
}
