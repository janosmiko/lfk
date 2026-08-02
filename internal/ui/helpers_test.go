package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestPadToHeight(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		height   int
		expected int // expected number of lines
	}{
		{"shorter content", "line1\nline2", 5, 5},
		{"exact height", "a\nb\nc", 3, 3},
		{"taller content", "a\nb\nc\nd\ne", 3, 3},
		{"empty content", "", 3, 3},
		{"single line", "hello", 1, 1},
		{"height zero", "hello", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PadToHeight(tt.content, tt.height)
			lines := strings.Split(result, "\n")
			if tt.height == 0 {
				// PadToHeight truncates to 0 lines but Split always gives at least 1
				assert.LessOrEqual(t, len(lines), 1)
			} else {
				assert.Equal(t, tt.expected, len(lines))
			}
		})
	}

	t.Run("padding is empty lines", func(t *testing.T) {
		result := PadToHeight("line1", 3)
		lines := strings.Split(result, "\n")
		assert.Equal(t, "line1", lines[0])
		assert.Equal(t, "", lines[1])
		assert.Equal(t, "", lines[2])
	})

	t.Run("truncation preserves order", func(t *testing.T) {
		result := PadToHeight("a\nb\nc\nd", 2)
		lines := strings.Split(result, "\n")
		assert.Equal(t, "a", lines[0])
		assert.Equal(t, "b", lines[1])
	})
}

func TestFullscreenBorderStyle(t *testing.T) {
	t.Run("returns style with expected dimensions", func(t *testing.T) {
		s := FullscreenBorderStyle(100, 30)
		// lipgloss v2 counts the border inside Width/Height, so the style
		// carries the full outer box: 100 wide, and 30 content rows plus the
		// two border rows. See TestFullscreenBorderStyleFillsRequestedBox for
		// the rendered-output check.
		assert.Equal(t, 100, s.GetWidth())
		assert.Equal(t, 32, s.GetHeight())
		assert.Equal(t, 32, s.GetMaxHeight())
	})

	t.Run("has rounded border", func(t *testing.T) {
		s := FullscreenBorderStyle(80, 20)
		assert.True(t, s.GetBorderStyle() == lipgloss.RoundedBorder())
	})

	t.Run("has padding", func(t *testing.T) {
		s := FullscreenBorderStyle(80, 20)
		assert.Equal(t, 0, s.GetPaddingTop())
		assert.Equal(t, 1, s.GetPaddingRight())
		assert.Equal(t, 0, s.GetPaddingBottom())
		assert.Equal(t, 1, s.GetPaddingLeft())
	})
}
