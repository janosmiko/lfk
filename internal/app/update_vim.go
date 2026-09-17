package app

import (
	"slices"
	"unicode/utf8"

	"github.com/janosmiko/lfk/internal/ui"
)

// Motion columns are cell columns, matching what the renderer draws. Each
// motion converts to a grapheme-cluster index to classify characters and
// converts the answer back, so a combining mark or a ZWJ emoji counts as one.

// nextWordStart returns the column of the next word start (vim 'w' motion).
func nextWordStart(line string, col int) int { return nextStart(line, col, isWordBoundary) }

// wordEnd returns the column of the current/next word end (vim 'e' motion).
func wordEnd(line string, col int) int { return wordEndWith(line, col, isWordBoundary) }

// prevWordStart returns the column of the previous word start (vim 'b' motion).
func prevWordStart(line string, col int) int { return prevStart(line, col, isWordBoundary) }

// isWordBoundary returns true if the rune is whitespace or punctuation (non-word character).
func isWordBoundary(r rune) bool {
	return r == ' ' || r == '\t' || r == '.' || r == ':' || r == ',' || r == ';' ||
		r == '/' || r == '-' || r == '_' || r == '"' || r == '\'' || r == '(' || r == ')' ||
		r == '[' || r == ']' || r == '{' || r == '}'
}

// nextWORDStart returns the column of the next WORD start (vim 'W' motion).
// WORDs are whitespace-delimited (only spaces and tabs are boundaries).
func nextWORDStart(line string, col int) int { return nextStart(line, col, isWORDBoundary) }

// prevWORDStart returns the column of the previous WORD start (vim 'B' motion).
// WORDs are whitespace-delimited (only spaces and tabs are boundaries).
func prevWORDStart(line string, col int) int { return prevStart(line, col, isWORDBoundary) }

// WORDEnd returns the column of the current/next WORD end (vim 'E' motion).
// WORDs are whitespace-delimited (only spaces and tabs are boundaries).
func WORDEnd(line string, col int) int { return wordEndWith(line, col, isWORDBoundary) }

// nextStart is the shared body of nextWordStart/nextWORDStart. Returns the
// line width (past end) when no next word exists, signaling cross-line needed.
func nextStart(line string, col int, isBoundary func(rune) bool) int {
	clusters, cols := ui.AllClusterCols(line)
	n := len(clusters)
	i := colIndex(cols, col)
	if n == 0 || i >= n-1 {
		return ui.LineWidth(line)
	}
	for i < n && !clusterIsBoundary(clusters[i], isBoundary) {
		i++
	}
	for i < n && clusterIsBoundary(clusters[i], isBoundary) {
		i++
	}
	return cols[i]
}

// wordEndWith is the shared body of wordEnd/WORDEnd. Returns the line width
// (past end) when no next word end exists, signaling cross-line needed.
func wordEndWith(line string, col int, isBoundary func(rune) bool) int {
	clusters, cols := ui.AllClusterCols(line)
	n := len(clusters)
	i := colIndex(cols, col)
	if n == 0 || i >= n-1 {
		return ui.LineWidth(line)
	}
	i++
	for i < n && clusterIsBoundary(clusters[i], isBoundary) {
		i++
	}
	if i >= n {
		return ui.LineWidth(line)
	}
	for i < n-1 && !clusterIsBoundary(clusters[i+1], isBoundary) {
		i++
	}
	return cols[i]
}

// prevStart is the shared body of prevWordStart/prevWORDStart. Returns -1
// when no previous word exists, signaling cross-line needed.
func prevStart(line string, col int, isBoundary func(rune) bool) int {
	clusters, cols := ui.AllClusterCols(line)
	if len(clusters) == 0 || col <= 0 {
		return -1
	}
	i := colIndex(cols, col) - 1
	if i < 0 {
		return -1
	}
	for i > 0 && clusterIsBoundary(clusters[i], isBoundary) {
		i--
	}
	for i > 0 && !clusterIsBoundary(clusters[i-1], isBoundary) {
		i--
	}
	return cols[i]
}

// clusterIsBoundary classifies a grapheme cluster by its base rune, matching
// isWordBoundary/isWORDBoundary's rune-level rules.
func clusterIsBoundary(cluster string, isBoundary func(rune) bool) bool {
	r, _ := utf8.DecodeRuneInString(cluster)
	return isBoundary(r)
}

// lastCol returns the column the final character of line starts at (vim '$'
// motion). The last rune is not the last character: a trailing combining mark
// draws no cell of its own, so a rune index lands past the visible text.
func lastCol(line string) int {
	w := ui.LineWidth(line)
	if w == 0 {
		return 0
	}
	return ui.SnapColStart(line, w-1)
}

// stepCol moves col by n characters. Stepping by runes gets stuck on a
// combining mark, which shares its column with the letter it sits on. A line
// with no text, such as the empty side of a diff, keeps the column.
func stepCol(line string, col, n int) int {
	if line == "" {
		return max(col+n, 0)
	}
	cols := ui.CharCols(line)
	i := colIndex(cols, col)
	return cols[min(max(i+n, 0), len(cols)-1)]
}

// colIndex reports the index of the character occupying column col. A column
// landing inside a wide character reports the character it starts.
func colIndex(cols []int, col int) int {
	i, exact := slices.BinarySearch(cols, col)
	switch {
	case i >= len(cols):
		i = len(cols) - 1
	case exact:
		// Skip past duplicate offsets from zero-width clusters.
		for i+1 < len(cols)-1 && cols[i+1] == col {
			i++
		}
	case !exact && i > 0:
		i--
	}
	return i
}

// firstNonWhitespace returns the column of the first non-space/tab character (vim '^' motion).
func firstNonWhitespace(line string) int {
	for i, r := range []rune(line) {
		if r != ' ' && r != '\t' {
			return ui.ColumnOf(line, i)
		}
	}
	return 0
}

// isWORDBoundary returns true if the rune separates WORDs (whitespace only).
// Mirrors the binary split used by nextWORDStart/prevWORDStart/WORDEnd.
func isWORDBoundary(r rune) bool {
	return r == ' ' || r == '\t'
}

// innerWordRange returns the inclusive [start, end] column range of the inner
// word text object (vim 'iw') at col. If col sits on a word character the
// range covers the contiguous run of word characters; if col sits on a word
// boundary (whitespace/punctuation) the range covers the contiguous run of
// boundary characters. Returns (-1, -1) for an empty line.
//
// Boundary classification follows isWordBoundary, so results are consistent
// with the w/b/e motions in this codebase.
func innerWordRange(line string, col int) (int, int) {
	return innerRangeWith(line, col, isWordBoundary)
}

// innerWORDRange is the WORD variant of innerWordRange (vim 'iW'); only space
// and tab are treated as boundaries.
func innerWORDRange(line string, col int) (int, int) {
	return innerRangeWith(line, col, isWORDBoundary)
}

// aroundWordRange returns the inclusive [start, end] column range of the
// "around word" text object (vim 'aw') at col. If col is on a word, the range
// covers the word plus trailing whitespace, or leading whitespace when no
// trailing exists. If col is on whitespace/punctuation, the range covers that
// run plus the following word. Returns (-1, -1) for an empty line.
func aroundWordRange(line string, col int) (int, int) {
	return aroundRangeWith(line, col, isWordBoundary)
}

// aroundWORDRange is the WORD variant of aroundWordRange (vim 'aW'); only
// space and tab are treated as boundaries.
func aroundWORDRange(line string, col int) (int, int) {
	return aroundRangeWith(line, col, isWORDBoundary)
}

// innerRangeWith returns the inclusive [start, end] column range covering the
// contiguous run at col whose runes share isBoundary's classification (all
// boundary or all non-boundary). Shared by the word and WORD inner-range
// variants. Returns (-1, -1) on empty input.
func innerRangeWith(line string, col int, isBoundary func(rune) bool) (int, int) {
	start, end, ok := innerRuneRange(line, col, isBoundary)
	if !ok {
		return -1, -1
	}
	return ui.ColumnOf(line, start), ui.ColumnOf(line, end)
}

// innerRuneRange is innerRangeWith in rune indices, so the around variant can
// keep walking the same slice instead of converting back and forth.
func innerRuneRange(line string, col int, isBoundary func(rune) bool) (int, int, bool) {
	runes := []rune(line)
	n := len(runes)
	if n == 0 {
		return 0, 0, false
	}
	i := min(max(ui.RuneIndexAt(line, col), 0), n-1)
	onBoundary := isBoundary(runes[i])
	start := i
	for start > 0 && isBoundary(runes[start-1]) == onBoundary {
		start--
	}
	end := i
	for end < n-1 && isBoundary(runes[end+1]) == onBoundary {
		end++
	}
	return start, end, true
}

// aroundRangeWith extends innerRangeWith to the "around" form: a cursor on a
// word swallows the trailing boundary run (or the leading run when no
// trailing exists); a cursor on a boundary run swallows the following word.
// Shared by the word and WORD around-range variants. Returns (-1, -1) on
// empty input.
func aroundRangeWith(line string, col int, isBoundary func(rune) bool) (int, int) {
	runes := []rune(line)
	n := len(runes)
	start, end, ok := innerRuneRange(line, col, isBoundary)
	if !ok {
		return -1, -1
	}
	if isBoundary(runes[start]) {
		// Cursor on a boundary run: extend forward to swallow the next word.
		for end < n-1 && !isBoundary(runes[end+1]) {
			end++
		}
		return ui.ColumnOf(line, start), ui.ColumnOf(line, end)
	}
	// Cursor on a word: prefer trailing boundary; fall back to leading.
	if end < n-1 && isBoundary(runes[end+1]) {
		for end < n-1 && isBoundary(runes[end+1]) {
			end++
		}
		return ui.ColumnOf(line, start), ui.ColumnOf(line, end)
	}
	for start > 0 && isBoundary(runes[start-1]) {
		start--
	}
	return ui.ColumnOf(line, start), ui.ColumnOf(line, end)
}

// consumeTextObjectPrelude is called at the top of every visual-mode key
// handler. If a text-object operator (`i` or `a`) is pending and `key` is a
// supported motion (`w` or `W`), it returns (op, motion, true) and clears
// the pending state so the caller can apply the resolution. If pending is
// set but `key` is anything else, the operator is dropped (vim semantics:
// any unrelated key cancels a half-typed operator) and (0, "", false) is
// returned. Also clears any stale `pendingG` so a half-typed `gg` doesn't
// survive across an unrelated operator sequence.
func (m *Model) consumeTextObjectPrelude(key string) (byte, string, bool) {
	if m.pendingTextObject == 0 {
		return 0, "", false
	}
	op := m.pendingTextObject
	m.pendingTextObject = 0
	m.pendingG = false
	if key == "w" || key == "W" {
		return op, key, true
	}
	return 0, "", false
}

// textObjectRange resolves a vim text-object operator (`i` or `a`) plus a
// motion key (`w` or `W`) into an inclusive [start, end] column range on
// `line`, evaluated at column `col`. Returns ok=false when the line is empty
// or the inputs are not a recognised text-object combination, leaving the
// caller's selection unchanged.
//
// Callers always commit the resulting range as a character-wise selection
// (visualType='v') even if the user was previously in line ('V') or block
// ('B') mode. This deviates from real vim, which keeps the original mode,
// but matches the "select a word to copy" intent that drives this feature
// in read-only viewers.
func textObjectRange(line string, col int, op byte, motion string) (int, int, bool) {
	switch motion {
	case "w":
		switch op {
		case 'i':
			s, e := innerWordRange(line, col)
			return s, e, s >= 0
		case 'a':
			s, e := aroundWordRange(line, col)
			return s, e, s >= 0
		}
	case "W":
		switch op {
		case 'i':
			s, e := innerWORDRange(line, col)
			return s, e, s >= 0
		case 'a':
			s, e := aroundWORDRange(line, col)
			return s, e, s >= 0
		}
	}
	return 0, 0, false
}
