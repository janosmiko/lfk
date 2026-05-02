package app

import "strconv"

// maxCountPrefix caps the parsed count to keep `cursor + n` arithmetic in the
// call sites from overflowing on a pathologically large prefix. One million
// lines is comfortably larger than any real viewer fixture (log buffers cap
// well below this) so the cap is invisible to actual users; what it prevents
// is a 19-digit count silently wrapping into a negative cursor and panicking
// on slice indexing downstream.
const maxCountPrefix = 1_000_000

// parseCountPrefix parses the digit-prefix buffer that powers count-prefixed
// commands (e.g. `123y`, `123G`, `123j`, `123k`) and returns the count to
// apply. An empty or invalid buffer falls back to 1 so plain `j`/`k`/`y` keep
// their single-step behaviour. Counts above maxCountPrefix saturate at the
// cap.
func parseCountPrefix(buf string) int {
	if buf == "" {
		return 1
	}
	n, err := strconv.Atoi(buf)
	if err != nil || n < 1 {
		return 1
	}
	return min(n, maxCountPrefix)
}

// consumeCountPrefix parses the digit-prefix buffer and clears it in one step.
// Every count-prefixed handler must consume the buffer (so the digits don't
// leak into the next command), so doing parse + clear together keeps the call
// sites symmetric and prevents anyone forgetting the second half.
func consumeCountPrefix(buf *string) int {
	n := parseCountPrefix(*buf)
	*buf = ""
	return n
}
