package ui

import (
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

// Viewer columns are terminal cells: a CJK glyph covers two, a combining mark
// none, a tab the distance to the next tabStopWidth column - so rune-indexed
// code converts here instead of assuming one rune is one column.

// tabStopWidth matches internal/tainted's log-line tab expansion, so a
// tab-bearing line sizes the same way in both viewers.
const tabStopWidth = 8

// tabWidthAt returns how many columns a tab starting at col fills.
func tabWidthAt(col int) int {
	return tabStopWidth - col%tabStopWidth
}

// clusterWidth is ansi.StringWidth, except a tab's width runs to the next
// tab stop instead of the zero cells ansi.StringWidth reports for it.
func clusterWidth(cluster string, col int) int {
	if cluster == "\t" {
		return tabWidthAt(col)
	}
	return ansi.StringWidth(cluster)
}

// LineWidth returns how many cells line fills on screen.
func LineWidth(line string) int {
	if !strings.ContainsRune(line, '\t') {
		return ansi.StringWidth(line)
	}
	cols := CharCols(line)
	return cols[len(cols)-1]
}

// CharCols returns the column each grapheme cluster starts at, ascending, with
// the line's width last. A flag or a ZWJ emoji family is several runes but one
// glyph, so runes list columns the line does not have, and out of order.
func CharCols(line string) []int {
	_, cols := ClusterCols(line)
	return cols
}

// ClusterCols splits line into its grapheme clusters alongside the column
// each one starts at: clusters[i] starts at cols[i]. cols also carries the
// line's width as its last, unpaired entry, matching CharCols.
func ClusterCols(line string) (clusters []string, cols []int) {
	return clusterCols(line, true)
}

// AllClusterCols is ClusterCols without dropping zero-width clusters, so an
// unexpanded tab or another control character still counts as its own
// character for classification, even though it draws nothing on screen.
func AllClusterCols(line string) (clusters []string, cols []int) {
	return clusterCols(line, false)
}

func clusterCols(line string, dropZeroWidth bool) (clusters []string, cols []int) {
	clusters = make([]string, 0, len(line))
	cols = make([]int, 0, len(line)+1)
	col, state := 0, -1
	for rest := line; rest != ""; {
		var cluster string
		// uniseg finds the cluster, ansi measures it: the two disagree on a
		// keycap, and ansi is what the renderer draws with.
		cluster, rest, _, state = uniseg.FirstGraphemeClusterInString(rest, state)
		w := clusterWidth(cluster, col)
		if w == 0 && dropZeroWidth {
			continue
		}
		clusters = append(clusters, cluster)
		cols = append(cols, col)
		col += w
	}
	return clusters, append(cols, col)
}

// ColumnOf returns the cell column where the rune at runeIdx starts. An index
// past the last rune returns the width of the whole line.
func ColumnOf(line string, runeIdx int) int {
	if runeIdx <= 0 {
		return 0
	}
	runes := []rune(line)
	if runeIdx > len(runes) {
		runeIdx = len(runes)
	}
	return colOfRuneIdx(runes, runeIdx)
}

// colOfRuneIdx measures each run between tabs as a whole with
// ansi.StringWidth, so a multi-rune grapheme cluster (a flag, a ZWJ emoji)
// keeps its clustered width instead of an inflated per-rune sum.
func colOfRuneIdx(runes []rune, idx int) int {
	col, start := 0, 0
	for i := range idx {
		if runes[i] != '\t' {
			continue
		}
		col += ansi.StringWidth(string(runes[start:i]))
		col += tabWidthAt(col)
		start = i + 1
	}
	return col + ansi.StringWidth(string(runes[start:idx]))
}

// RuneIndexAt returns the index of the rune drawn at cell column col. A column
// that falls inside a wide rune or a tab reports that rune, and a column past
// the end of the line reports the rune count.
func RuneIndexAt(line string, col int) int {
	if col <= 0 {
		return 0
	}
	if col >= LineWidth(line) {
		return len([]rune(line))
	}
	return runeIndexAtCol([]rune(line), col)
}

// runeIndexAtCol walks the same tab-split runs as colOfRuneIdx, resolving the
// index within a run with ansi.Truncate. A col inside a tab's span reports
// the tab's own index, the rule SnapColStart applies to a wide glyph too.
func runeIndexAtCol(runes []rune, col int) int {
	pos, start := 0, 0
	for i := 0; i <= len(runes); i++ {
		if i < len(runes) && runes[i] != '\t' {
			continue
		}
		seg := string(runes[start:i])
		segWidth := ansi.StringWidth(seg)
		if pos+segWidth > col {
			return start + len([]rune(ansi.Truncate(seg, col-pos, "")))
		}
		pos += segWidth
		if i == len(runes) {
			break
		}
		tw := tabWidthAt(pos)
		if pos+tw > col {
			return i
		}
		pos += tw
		start = i + 1
	}
	return len(runes)
}

// SnapColStart moves col back to the first column of the character drawn
// there. A tab-free line goes through ansi.Cut, which is ANSI-escape-aware
// unlike a uniseg walk - callers pass already-styled lines carrying SGR codes.
func SnapColStart(line string, col int) int {
	if !strings.ContainsRune(line, '\t') {
		for col > 0 && isCharBoundaryPlain(line, col) != col {
			col--
		}
		return max(col, 0)
	}
	if col <= 0 {
		return 0
	}
	cols := CharCols(line)
	w := cols[len(cols)-1]
	if col >= w {
		return w
	}
	i, exact := slices.BinarySearch(cols, col)
	if exact {
		return col
	}
	if i == 0 {
		return 0
	}
	return cols[i-1]
}

// SnapColEnd is the forward half of SnapColStart.
func SnapColEnd(line string, col int) int {
	if !strings.ContainsRune(line, '\t') {
		w := ansi.StringWidth(line)
		for col < w && isCharBoundaryPlain(line, col) != col {
			col++
		}
		return min(max(col, 0), w)
	}
	cols := CharCols(line)
	w := cols[len(cols)-1]
	if col <= 0 {
		return 0
	}
	if col >= w {
		return w
	}
	i, _ := slices.BinarySearch(cols, col)
	return cols[i]
}

// isCharBoundaryPlain returns col when col starts a character, or the width
// of the cut short of it otherwise. ansi.Cut drops a glyph the cut only half
// covers, so a mid-glyph column yields a short prefix.
func isCharBoundaryPlain(line string, col int) int {
	return ansi.StringWidth(ansi.Cut(line, 0, col))
}

// CutCols returns the text between cell columns start and end, end exclusive.
// Bounds are clamped to the line and widened to whole characters, so a range
// that stops inside a wide glyph or a tab still carries it whole, unexpanded.
func CutCols(line string, start, end int) string {
	w := LineWidth(line)
	// Clamp start to the line too: a cursor column carried over from a longer
	// line would otherwise walk SnapColStart back one cell at a time.
	start = SnapColStart(line, min(max(start, 0), w))
	end = SnapColEnd(line, min(end, w))
	if end <= start {
		return ""
	}
	if !strings.ContainsRune(line, '\t') {
		return ansi.Cut(line, start, end)
	}
	return cutColsTabAware(line, start, end)
}

// cutColsTabAware is CutCols for a line with a literal tab: ansi.Cut can't
// compute a tab's terminal width, so this splits on the tab byte (safe, a
// tab is never a UTF-8 continuation byte) and cuts only within each run.
func cutColsTabAware(line string, start, end int) string {
	segs := strings.Split(line, "\t")
	var b strings.Builder
	col := 0
	for i, seg := range segs {
		segWidth := ansi.StringWidth(seg)
		if segEnd := col + segWidth; segEnd > start && col < end {
			cutFrom := max(start-col, 0)
			cutTo := min(end-col, segWidth)
			if cutTo > cutFrom {
				b.WriteString(ansi.Cut(seg, cutFrom, cutTo))
			}
		}
		col += segWidth
		if i == len(segs)-1 {
			break
		}
		tw := tabWidthAt(col)
		if col >= start && col < end {
			b.WriteByte('\t')
		}
		col += tw
	}
	return b.String()
}
