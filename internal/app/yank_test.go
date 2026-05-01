package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConsumeYankCount(t *testing.T) {
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
