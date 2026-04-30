package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Quit overlay must render as a 5-row box with "Quit lfk?" on the
// middle row. lipgloss `Height(N)` counts inner+padding (not border), so the
// arithmetic in renderOverlayContent has to compensate (qh=3 produces a
// 5-row outer box). PRs #80, #97 highlighted that contributors notice when
// "Quit lfk?" floats above empty space — keep this test green to catch the
// regression directly instead of waiting for the next PR.
func TestQuitOverlayCentersTextOnMiddleRow(t *testing.T) {
	m := Model{
		overlay: overlayQuitConfirm,
		width:   80,
		height:  24,
	}
	bg := strings.Repeat("background row\n", 24)
	out := m.renderOverlay(bg)
	lines := strings.Split(out, "\n")

	topBorder, bottomBorder, textRow := -1, -1, -1
	for i, line := range lines {
		stripped := ansi.Strip(line)
		switch {
		case strings.Contains(stripped, "╭"):
			topBorder = i
		case strings.Contains(stripped, "╰"):
			bottomBorder = i
		case strings.Contains(stripped, "Quit lfk?"):
			textRow = i
		}
	}

	require.NotEqual(t, -1, topBorder, "top border row not found")
	require.NotEqual(t, -1, bottomBorder, "bottom border row not found")
	require.NotEqual(t, -1, textRow, "text row not found")

	dialogHeight := bottomBorder - topBorder + 1
	assert.Equal(t, 5, dialogHeight, "dialog should be exactly 5 rows tall (border / padding / text / padding / border)")

	// Middle row of 5 is offset 2 from the top border (0-indexed).
	expectedTextRow := topBorder + 2
	assert.Equal(t, expectedTextRow, textRow,
		"text must sit on the middle row (border@%d, text@%d, expected@%d)", topBorder, textRow, expectedTextRow)
}
