package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestWrapConfirmText(t *testing.T) {
	t.Run("keeps words and hyphenated names intact", func(t *testing.T) {
		lines := wrapConfirmText("Evict all replicas from dev-envs-autoscaled-cx43-2496bed88dc5524c?", 46)
		for _, ln := range lines {
			assert.LessOrEqual(t, lipgloss.Width(ln), 46, "line exceeds width: %q", ln)
		}
		// The long name fits within 46 cols, so it must stay on one line
		// (no hyphen shattering).
		assert.Contains(t, lines, "dev-envs-autoscaled-cx43-2496bed88dc5524c?")
		// No word is split mid-character.
		assert.NotContains(t, lines, "r")
	})

	t.Run("hard-splits a token longer than width", func(t *testing.T) {
		long := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
		lines := wrapConfirmText(long, 10)
		for _, ln := range lines {
			assert.LessOrEqual(t, lipgloss.Width(ln), 10)
		}
		assert.Equal(t, []string{"aaaaaaaaaa", "aaaaaaaaaa", "aaaaaaaaaa"}, lines)
	})

	t.Run("zero width returns text unchanged", func(t *testing.T) {
		assert.Equal(t, []string{"hello world"}, wrapConfirmText("hello world", 0))
	})

	t.Run("hard-splits multibyte token on rune boundaries", func(t *testing.T) {
		token := strings.Repeat("α", 9) // 9 width-1 Greek runes
		lines := wrapConfirmText(token, 4)
		for _, ln := range lines {
			assert.True(t, utf8.ValidString(ln), "split produced invalid UTF-8: %q", ln)
			assert.LessOrEqual(t, lipgloss.Width(ln), 4)
		}
		assert.Equal(t, token, strings.Join(lines, ""), "split must not lose or corrupt runes")
	})
}
