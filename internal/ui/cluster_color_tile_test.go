package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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

// TestClusterColorTileBgOver guards the cursor-row variant: the colored tile
// ends with an SGR reset, and on the cursor row that reset would cancel the
// selection highlight for every cell after the tile. ClusterColorTileBgOver
// re-emits the outer (highlight) style's open codes right after the tile so
// the highlight survives the rest of the row.
func TestClusterColorTileBgOver(t *testing.T) {
	// Force a color-capable profile so the tile emits real escape codes;
	// in a no-TTY test context lipgloss otherwise downgrades to no-color.
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(originalProfile) })
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI256)

	origNoColor := ConfigNoColor
	t.Cleanup(func() { ConfigNoColor = origNoColor })
	ConfigNoColor = false

	outer := lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15"))
	outerOpen := styleOpenCodes(outer)

	t.Run("colored tile re-asserts the outer style after its reset", func(t *testing.T) {
		// cyan is palette-relative (not theme-mapped), so it resolves to a
		// concrete background regardless of ActiveTheme state in the test.
		got := ClusterColorTileBgOver("cyan", outer)
		const reset = "\x1b[0m"
		resetIdx := strings.LastIndex(got, reset)
		if !assert.GreaterOrEqualf(t, resetIdx, 0, "a colored tile must contain an SGR reset; got %q", got) {
			return
		}
		tail := got[resetIdx+len(reset):]
		assert.Equalf(t, outerOpen, tail,
			"the outer style's open codes must immediately follow the tile's reset "+
				"so a cursor-row highlight is not cancelled for the rest of the row; got %q", got)
	})

	t.Run("uncolored tile is a plain space with no restore appended", func(t *testing.T) {
		assert.Equal(t, " ", ClusterColorTileBgOver("", outer),
			"an uncolored tile emits no reset, so no outer-style re-assertion is needed")
	})

	t.Run("colored tile is still exactly one cell wide", func(t *testing.T) {
		assert.Equal(t, 1, lipgloss.Width(ClusterColorTileBgOver("cyan", outer)),
			"re-asserted SGR codes have zero display width; the tile stays one cell")
	})
}
