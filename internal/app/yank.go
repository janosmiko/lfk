package app

import "fmt"

// formatCopiedLines returns the status message for an N-line yank.
// Singular for n=1, plural otherwise — preserves the existing
// "Copied 1 line" string for the unprefixed `y` path.
func formatCopiedLines(n int) string {
	if n == 1 {
		return "Copied 1 line"
	}
	return fmt.Sprintf("Copied %d lines", n)
}
