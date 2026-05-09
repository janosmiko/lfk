package app

// nextWordStart returns the column of the next word start (vim 'w' motion).
// Returns n (past end) when no next word exists on this line, signaling cross-line needed.
func nextWordStart(line string, col int) int {
	runes := []rune(line)
	n := len(runes)
	if n == 0 || col >= n-1 {
		return n
	}
	i := col
	// Skip current word characters.
	for i < n && !isWordBoundary(runes[i]) {
		i++
	}
	// Skip whitespace/punctuation.
	for i < n && isWordBoundary(runes[i]) {
		i++
	}
	if i >= n {
		return n
	}
	return i
}

// wordEnd returns the column of the current/next word end (vim 'e' motion).
// Returns n (past end) when no next word end exists on this line, signaling cross-line needed.
func wordEnd(line string, col int) int {
	runes := []rune(line)
	n := len(runes)
	if n == 0 || col >= n-1 {
		return n
	}
	i := col + 1
	// Skip whitespace/punctuation.
	for i < n && isWordBoundary(runes[i]) {
		i++
	}
	if i >= n {
		return n
	}
	// Move to end of word.
	for i < n-1 && !isWordBoundary(runes[i+1]) {
		i++
	}
	return i
}

// prevWordStart returns the column of the previous word start (vim 'b' motion).
// Returns -1 when no previous word exists on this line, signaling cross-line needed.
func prevWordStart(line string, col int) int {
	runes := []rune(line)
	n := len(runes)
	if n == 0 || col <= 0 {
		return -1
	}
	if col >= n {
		col = n
	}
	i := col - 1
	// Skip whitespace/punctuation.
	for i > 0 && isWordBoundary(runes[i]) {
		i--
	}
	// Move to start of word.
	for i > 0 && !isWordBoundary(runes[i-1]) {
		i--
	}
	return i
}

// isWordBoundary returns true if the rune is whitespace or punctuation (non-word character).
func isWordBoundary(r rune) bool {
	return r == ' ' || r == '\t' || r == '.' || r == ':' || r == ',' || r == ';' ||
		r == '/' || r == '-' || r == '_' || r == '"' || r == '\'' || r == '(' || r == ')' ||
		r == '[' || r == ']' || r == '{' || r == '}'
}

// nextWORDStart returns the column of the next WORD start (vim 'W' motion).
// WORDs are whitespace-delimited (only spaces and tabs are boundaries).
// Returns n (past end) when no next WORD exists on this line, signaling cross-line needed.
func nextWORDStart(line string, col int) int {
	runes := []rune(line)
	n := len(runes)
	if n == 0 || col >= n-1 {
		return n
	}
	i := col
	// Skip current WORD characters (non-whitespace).
	for i < n && runes[i] != ' ' && runes[i] != '\t' {
		i++
	}
	// Skip whitespace.
	for i < n && (runes[i] == ' ' || runes[i] == '\t') {
		i++
	}
	if i >= n {
		return n
	}
	return i
}

// prevWORDStart returns the column of the previous WORD start (vim 'B' motion).
// WORDs are whitespace-delimited (only spaces and tabs are boundaries).
// Returns -1 when no previous WORD exists on this line, signaling cross-line needed.
func prevWORDStart(line string, col int) int {
	runes := []rune(line)
	n := len(runes)
	if n == 0 || col <= 0 {
		return -1
	}
	if col >= n {
		col = n
	}
	i := col - 1
	// Skip whitespace.
	for i > 0 && (runes[i] == ' ' || runes[i] == '\t') {
		i--
	}
	// Move to start of WORD (non-whitespace).
	for i > 0 && runes[i-1] != ' ' && runes[i-1] != '\t' {
		i--
	}
	return i
}

// WORDEnd returns the column of the current/next WORD end (vim 'E' motion).
// WORDs are whitespace-delimited (only spaces and tabs are boundaries).
// Returns n (past end) when no next WORD end exists on this line, signaling cross-line needed.
func WORDEnd(line string, col int) int {
	runes := []rune(line)
	n := len(runes)
	if n == 0 || col >= n-1 {
		return n
	}
	i := col + 1
	// Skip whitespace.
	for i < n && (runes[i] == ' ' || runes[i] == '\t') {
		i++
	}
	if i >= n {
		return n
	}
	// Move to end of WORD (non-whitespace).
	for i < n-1 && runes[i+1] != ' ' && runes[i+1] != '\t' {
		i++
	}
	return i
}

// firstNonWhitespace returns the column of the first non-space/tab character (vim '^' motion).
func firstNonWhitespace(line string) int {
	for i, r := range []rune(line) {
		if r != ' ' && r != '\t' {
			return i
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
	runes := []rune(line)
	n := len(runes)
	if n == 0 {
		return -1, -1
	}
	if col < 0 {
		col = 0
	}
	if col >= n {
		col = n - 1
	}
	onBoundary := isBoundary(runes[col])
	start := col
	for start > 0 && isBoundary(runes[start-1]) == onBoundary {
		start--
	}
	end := col
	for end < n-1 && isBoundary(runes[end+1]) == onBoundary {
		end++
	}
	return start, end
}

// aroundRangeWith extends innerRangeWith to the "around" form: a cursor on a
// word swallows the trailing boundary run (or the leading run when no
// trailing exists); a cursor on a boundary run swallows the following word.
// Shared by the word and WORD around-range variants. Returns (-1, -1) on
// empty input.
func aroundRangeWith(line string, col int, isBoundary func(rune) bool) (int, int) {
	runes := []rune(line)
	n := len(runes)
	if n == 0 {
		return -1, -1
	}
	start, end := innerRangeWith(line, col, isBoundary)
	if start < 0 {
		return -1, -1
	}
	onBoundary := isBoundary(runes[start])
	if onBoundary {
		// Cursor on a boundary run: extend forward to swallow the next word.
		for end < n-1 && !isBoundary(runes[end+1]) {
			end++
		}
		return start, end
	}
	// Cursor on a word: prefer trailing boundary; fall back to leading.
	if end < n-1 && isBoundary(runes[end+1]) {
		for end < n-1 && isBoundary(runes[end+1]) {
			end++
		}
		return start, end
	}
	for start > 0 && isBoundary(runes[start-1]) {
		start--
	}
	return start, end
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
