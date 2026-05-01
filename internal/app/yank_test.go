package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseYankCount(t *testing.T) {
	cases := []struct {
		name string
		buf  string
		want int
	}{
		{"empty buffer falls back to 1", "", 1},
		{"single digit", "5", 5},
		{"multi digit", "42", 42},
		{"non-numeric falls back to 1", "abc", 1},
		{"zero falls back to 1", "0", 1},
		{"negative-looking string falls back to 1", "-3", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseYankCount(tc.buf))
		})
	}
}

func TestFormatCopiedLines(t *testing.T) {
	assert.Equal(t, "Copied 1 line", formatCopiedLines(1))
	assert.Equal(t, "Copied 2 lines", formatCopiedLines(2))
	assert.Equal(t, "Copied 100 lines", formatCopiedLines(100))
}

// consumeYankCount must both parse the buffer and clear it in one step —
// every caller relies on the buffer being empty after the yank so the
// digits don't leak into the next command.
func TestConsumeYankCount(t *testing.T) {
	cases := []struct {
		name      string
		buf       string
		wantCount int
	}{
		{"empty buffer falls back to 1", "", 1},
		{"valid count is parsed", "5", 5},
		{"multi digit", "42", 42},
		{"non-numeric falls back to 1", "abc", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := tc.buf
			n := consumeYankCount(&buf)
			assert.Equal(t, tc.wantCount, n)
			assert.Empty(t, buf, "buffer must be cleared after consume")
		})
	}
}
