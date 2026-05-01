package app

import (
	"fmt"
	"strconv"
)

// parseYankCount parses the digit-prefix buffer that powers count-prefixed
// motions (e.g. `123y`, `123G`) and returns the count to apply. An empty or
// invalid buffer falls back to 1 so plain `y` keeps yanking a single line.
func parseYankCount(buf string) int {
	if buf == "" {
		return 1
	}
	n, err := strconv.Atoi(buf)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// formatCopiedLines returns the status message for an N-line yank.
// Singular for n=1, plural otherwise — preserves the existing
// "Copied 1 line" string for the unprefixed `y` path.
func formatCopiedLines(n int) string {
	if n == 1 {
		return "Copied 1 line"
	}
	return fmt.Sprintf("Copied %d lines", n)
}
