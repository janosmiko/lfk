package ui

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

// Viewer columns are terminal cells, the unit the renderer draws in. A CJK
// glyph covers two, a combining mark none, an ANSI escape none, so rune-indexed
// code converts here instead of assuming one rune is one column.

// LineWidth returns how many cells line fills on screen.
func LineWidth(line string) int { return ansi.StringWidth(line) }

// CharCols returns the column each grapheme cluster starts at, ascending, with
// the line's width last. A flag or a ZWJ emoji family is several runes but one
// glyph, so runes list columns the line does not have, and out of order.
func CharCols(line string) []int {
	cols := make([]int, 0, len(line)+1)
	col, state := 0, -1
	for rest := line; rest != ""; {
		var cluster string
		// uniseg finds the cluster, ansi measures it: the two disagree on a
		// keycap, and ansi is what the renderer draws with.
		cluster, rest, _, state = uniseg.FirstGraphemeClusterInString(rest, state)
		w := ansi.StringWidth(cluster)
		if w == 0 {
			continue
		}
		cols = append(cols, col)
		col += w
	}
	return append(cols, col)
}

// ColumnOf returns the cell column where the rune at runeIdx starts. An index
// past the last rune returns the width of the whole line.
func ColumnOf(line string, runeIdx int) int {
	if runeIdx <= 0 {
		return 0
	}
	runes := []rune(line)
	if runeIdx >= len(runes) {
		return ansi.StringWidth(line)
	}
	return ansi.StringWidth(string(runes[:runeIdx]))
}

// RuneIndexAt returns the index of the rune drawn at cell column col. A column
// that falls inside a wide rune reports that rune, and a column past the end
// of the line reports the rune count.
func RuneIndexAt(line string, col int) int {
	if col <= 0 {
		return 0
	}
	if col >= ansi.StringWidth(line) {
		return len([]rune(line))
	}
	return len([]rune(ansi.Truncate(line, col, "")))
}

// SnapColStart moves col back to the first column of the character drawn
// there. Cutting a double-width glyph in half yields an empty cell and loses
// the glyph: there is no half a character to draw.
func SnapColStart(line string, col int) int {
	for col > 0 && !isCharBoundary(line, col) {
		col--
	}
	return max(col, 0)
}

// SnapColEnd is the forward half of SnapColStart.
func SnapColEnd(line string, col int) int {
	w := ansi.StringWidth(line)
	for col < w && !isCharBoundary(line, col) {
		col++
	}
	return min(col, w)
}

// isCharBoundary reports whether col starts a character. ansi.Cut drops a glyph
// the cut only half covers, so a mid-glyph column yields a short prefix.
func isCharBoundary(line string, col int) bool {
	return ansi.StringWidth(ansi.Cut(line, 0, col)) == col
}

// CutCols returns the text between cell columns start and end, end exclusive.
// Bounds are clamped to the line and widened to whole characters, so a range
// that stops inside a wide glyph still carries it.
func CutCols(line string, start, end int) string {
	w := ansi.StringWidth(line)
	// Clamp start to the line too: a cursor column carried over from a longer
	// line would otherwise walk SnapColStart back one cell at a time.
	start = SnapColStart(line, min(max(start, 0), w))
	end = SnapColEnd(line, min(end, w))
	if end <= start {
		return ""
	}
	return ansi.Cut(line, start, end)
}
