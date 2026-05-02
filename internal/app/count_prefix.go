package app

import "strconv"

// parseCountPrefix parses the digit-prefix buffer that powers count-prefixed
// commands (e.g. `123y`, `123G`, `123j`, `123k`) and returns the count to
// apply. An empty or invalid buffer falls back to 1 so plain `j`/`k`/`y` keep
// their single-step behaviour.
func parseCountPrefix(buf string) int {
	if buf == "" {
		return 1
	}
	n, err := strconv.Atoi(buf)
	if err != nil || n < 1 {
		return 1
	}
	return n
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
