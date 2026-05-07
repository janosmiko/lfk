package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

// TestClusterColorTileBg asserts the contract the explorer_table renderer
// relies on: the tile is exactly one terminal cell wide regardless of
// whether the color is set, so the column-width calc can budget a fixed
// tileW=1 in union mode without the tile leaking into the Name column when
// a row's source cluster has no configured color.
func TestClusterColorTileBg(t *testing.T) {
	t.Run("known color renders one cell", func(t *testing.T) {
		// In a no-TTY test context lipgloss downgrades to no-color output
		// (no ANSI escapes), so the only contract we can assert here is
		// the cell width — which is the shape the table renderer's
		// width-budget actually depends on. The visual ANSI is exercised
		// by the integration / TTY rendering paths.
		got := ClusterColorTileBg("blue")
		assert.Equal(t, 1, lipgloss.Width(got),
			"colored tile must be exactly one cell wide; got %q (%d cells)", got, lipgloss.Width(got))
	})
	t.Run("empty name renders one blank cell", func(t *testing.T) {
		got := ClusterColorTileBg("")
		assert.Equal(t, " ", got, "no-color rows must reserve the cell with a plain space; otherwise the union table's column boundaries would jitter row-by-row")
		assert.Equal(t, 1, lipgloss.Width(got))
	})
	t.Run("unknown color name renders one blank cell", func(t *testing.T) {
		// Defensive: if a stale cluster_colors entry references a color
		// that's been removed from ClusterColorNames, fall back to blank
		// rather than emit malformed ANSI.
		got := ClusterColorTileBg("magenta-fluorescent")
		assert.Equal(t, " ", got)
		assert.Equal(t, 1, lipgloss.Width(got))
	})
}
