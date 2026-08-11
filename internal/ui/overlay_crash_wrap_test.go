package ui

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWrapCrashText_ColoredLineMatchesPlainColumn asserts that an SGR-colored
// line wraps at the same column as the equivalent plain line. Before the
// fix, wrapCrashText compared lipgloss.Width (display columns) against
// strings.Builder.Len (bytes), so a colored word's escape bytes counted
// against the column budget and pushed the wrap point earlier.
func TestWrapCrashText_ColoredLineMatchesPlainColumn(t *testing.T) {
	plain := "alpha bravo charlie delta echo foxtrot golf"
	colored := "\x1b[31malpha\x1b[0m bravo \x1b[32mcharlie\x1b[0m delta echo foxtrot golf"

	plainLines := wrapCrashText(plain, 20)
	coloredLines := wrapCrashText(colored, 20)

	require.Equal(t, len(plainLines), len(coloredLines))
	for i := range plainLines {
		assert.Equal(t, ansi.Strip(plainLines[i]), ansi.Strip(coloredLines[i]))
		assert.LessOrEqual(t, lipgloss.Width(coloredLines[i]), 20)
	}
}

// TestWrapCrashText_MultibyteWithinBounds asserts multibyte runes are
// measured by display width, not byte length, when a run of them has to
// be hard-broken (a single unbroken token with no whitespace to wrap
// on). Byte-slicing a 3-byte-per-rune, width-2 rune at a budget that
// isn't a multiple of 3 cuts inside the rune's encoding and yields an
// invalid UTF-8 fragment — a defect a plain width<=N bound can't see,
// since a byte cut can only ever undershoot the display-width budget.
func TestWrapCrashText_MultibyteWithinBounds(t *testing.T) {
	word := strings.Repeat("あ", 10) // 3 bytes / 2 cols each, no spaces to wrap on

	for _, l := range wrapCrashText(word, 4) {
		assert.True(t, utf8.ValidString(l), "line %q is not valid UTF-8 (rune split across a wrap boundary)", l)
		assert.LessOrEqual(t, lipgloss.Width(l), 4, "line %q exceeds width budget", l)
	}
}

// incompleteEscapeAtEnd matches an SGR escape sequence that was cut off
// before its terminating letter: a trailing ESC, "ESC[", or
// "ESC[<digits/semicolons>" with nothing after it. A complete sequence
// always ends in a letter (e.g. "m"), which this pattern excludes.
var incompleteEscapeAtEnd = regexp.MustCompile(`\x1b(\[[0-9;]*)?$`)

// TestWrapCrashText_HardBreakNeverSplitsSGR asserts a single word longer
// than the column width, that carries SGR sequences, is hard-broken
// without ever cutting inside the escape sequence itself. Every lipgloss
// color emits ESC, so checking for ESC's mere presence proves nothing;
// this checks each wrapped line does not end mid-sequence.
func TestWrapCrashText_HardBreakNeverSplitsSGR(t *testing.T) {
	huge := "\x1b[35m" + strings.Repeat("x", 40) + "\x1b[0m"

	for _, l := range wrapCrashText(huge, 3) {
		assert.LessOrEqual(t, lipgloss.Width(l), 3, "line %q exceeds width budget", l)
		assert.False(t, incompleteEscapeAtEnd.MatchString(l), "line %q ends mid-escape-sequence", l)
	}
}
