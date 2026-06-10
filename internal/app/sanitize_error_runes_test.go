package app

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sanitizeError truncates status-bar error text. Error messages from webhooks /
// operators can contain arbitrary UTF-8; truncating on a byte boundary splits a
// multibyte rune and emits a replacement char. It must rune-slice, like its
// sibling sanitizeMessage.
func TestSanitizeErrorRuneSafe(t *testing.T) {
	m := baseModelCov()
	m.width = 60
	long := strings.Repeat("日", 200) // 3-byte runes; byte slicing would split one

	out := m.sanitizeError(errors.New(long))

	require.True(t, utf8.ValidString(out), "truncation must not split a multibyte rune")
	assert.True(t, strings.HasSuffix(out, "..."), "truncated message keeps the ellipsis")
}
