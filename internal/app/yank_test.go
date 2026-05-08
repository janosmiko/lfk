package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatCopiedLines(t *testing.T) {
	assert.Equal(t, "Copied 1 line", formatCopiedLines(1))
	assert.Equal(t, "Copied 2 lines", formatCopiedLines(2))
	assert.Equal(t, "Copied 100 lines", formatCopiedLines(100))
}

func TestFormatVisualYank(t *testing.T) {
	tests := []struct {
		name      string
		clipText  string
		mode      rune
		lineCount int
		want      string
	}{
		{"line mode single line", "hello", 'V', 1, "Copied 1 line"},
		{"line mode multi-line", "a\nb\nc", 'V', 3, "Copied 3 lines"},
		{"char mode single-line word", "beta", 'v', 1, "Copied"},
		{"char mode single-line one rune", "x", 'v', 1, "Copied"},
		{"char mode multi-line", "end\nstart", 'v', 2, "Copied 2 lines"},
		{"block mode single row", "abc", 'B', 1, "Copied"},
		{"block mode multi-row", "ab\ncd\nef", 'B', 3, "Copied 3 lines"},
		{"unicode char mode", "héllo", 'v', 1, "Copied"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatVisualYank(tt.clipText, tt.mode, tt.lineCount))
		})
	}
}
