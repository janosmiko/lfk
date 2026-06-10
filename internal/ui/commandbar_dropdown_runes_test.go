package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Suggestion text can be a kubeconfig context name — arbitrary UTF-8. The
// dropdown's fit-to-width must use display width and a rune-safe truncate, not
// byte len()/slicing, or a wide context name misaligns rows and a mid-rune cut
// emits invalid UTF-8.
func TestFormatSuggestionLineRuneSafe(t *testing.T) {
	s := Suggestion{Category: "context", Text: "日本語クラスタ-本番-very-long-context-name"}

	// Selected path exercises the content truncation.
	out := formatSuggestionLine(s, 7, 20, true, NormalStyle, NormalStyle, NormalStyle)

	require.True(t, utf8.ValidString(out), "truncation must not split a multibyte rune")
	assert.LessOrEqual(t, lipgloss.Width(out), 20, "must not overflow the target width")
}
