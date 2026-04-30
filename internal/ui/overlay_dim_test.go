package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DimBackground fades the lines above an overlay so the user's eye is drawn
// to the overlay box and the bottom hint bar. The contract:
//
//   - keepLast bottom lines pass through verbatim — the hint bar must not be
//     dimmed.
//   - All other lines are wrapped with the SGR 2 (faint) attribute. The
//     line's existing foreground, background, and bold styling pass through
//     unchanged so the theme's colours, the BarBg/SurfaceBg fills, and the
//     selection highlight's bold weight all survive — only their intensity
//     drops.
//   - Faint is re-applied after every internal `\x1b[0m` so it survives the
//     mid-line resets that lipgloss emits between styled runs.
//   - Line count is preserved (callers feed this output back into
//     PlaceOverlay which is height-sensitive).
//   - ConfigNoColor short-circuits the whole function — NoColor mode must
//     stay free of colour-escape sequences.
//   - Empty input and edge values for keepLast must not panic.
func TestDimBackground(t *testing.T) {
	t.Run("empty input returns empty", func(t *testing.T) {
		assert.Equal(t, "", DimBackground("", 1))
	})

	t.Run("keepLast greater than line count leaves input untouched", func(t *testing.T) {
		in := "line one\nline two"
		assert.Equal(t, in, DimBackground(in, 5))
	})

	t.Run("keepLast equal to line count leaves input untouched", func(t *testing.T) {
		in := "line one\nline two"
		assert.Equal(t, in, DimBackground(in, 2))
	})

	t.Run("preserves line count", func(t *testing.T) {
		in := "a\nb\nc\nd\ne"
		out := DimBackground(in, 1)
		assert.Equal(t, strings.Count(in, "\n"), strings.Count(out, "\n"),
			"newline count must match so PlaceOverlay alignment stays correct")
	})

	t.Run("bottom keepLast lines pass through verbatim with ANSI intact", func(t *testing.T) {
		styled := "\x1b[31mhint bar\x1b[0m"
		in := "first line\nmiddle\n" + styled
		out := DimBackground(in, 1)
		lines := strings.Split(out, "\n")
		require.Len(t, lines, 3)
		assert.Equal(t, styled, lines[2], "kept line must keep its original ANSI sequences")
	})

	// Dimmed lines must keep their original SGR codes intact (just
	// wrapped with faint) so theme colours and bold weights survive.
	t.Run("dimmed lines keep their original SGR styling", func(t *testing.T) {
		styled := "\x1b[31;1mred bold\x1b[0m"
		in := styled + "\n" + styled + "\nkeep me"
		out := DimBackground(in, 1)
		lines := strings.Split(out, "\n")
		require.Len(t, lines, 3)
		assert.Contains(t, lines[0], "\x1b[31;1m", "original red+bold SGR must survive the dim wrap")
		assert.Contains(t, lines[1], "\x1b[31;1m", "original red+bold SGR must survive the dim wrap")
	})

	// Bold (SGR 1) on selection highlights must survive the dim. The
	// cursor row in the middle column, the highlighted parent in the
	// left pane, and the active tab all use lipgloss `Bold(true)` plus
	// fg/bg, which lipgloss combines into a single SGR like
	// `\x1b[1;97;104m` (bold + bright-white fg + cyan bg). Stripping
	// the bold parameter would change the cursor row's font weight when
	// an overlay opens, and a font-weight shift behind the dim is
	// jarring — so bold passes through verbatim.
	t.Run("preserves bold weight on selection highlights", func(t *testing.T) {
		// Canonical lipgloss output for SelectedStyle in the ANSI 16
		// colour profile (Bold(true) + bright-white fg + bright-blue bg).
		styled := "\x1b[1;97;104mselected row\x1b[0m"
		in := styled + "\nstatus"
		out := DimBackground(in, 1)
		lines := strings.Split(out, "\n")
		require.Len(t, lines, 2)
		assert.Contains(t, lines[0], "\x1b[1;97;104m", "bold + colour SGR must survive the dim wrap")
	})

	// Background fills (e.g. lipgloss BarBg / SurfaceBg) must persist so
	// the breadcrumb tint behind the dim doesn't revert to the terminal's
	// default background.
	t.Run("preserves background SGR runs", func(t *testing.T) {
		styled := "\x1b[48;5;235m\x1b[37mlfk > context > Pods\x1b[0m"
		in := styled + "\nstatus bar"
		out := DimBackground(in, 1)
		lines := strings.Split(out, "\n")
		require.Len(t, lines, 2)
		assert.Contains(t, lines[0], "\x1b[48;5;235m", "background SGR must survive the dim wrap")
		assert.Contains(t, lines[0], "\x1b[37m", "foreground SGR must survive the dim wrap")
	})

	t.Run("dimmed lines keep their plain-text content", func(t *testing.T) {
		in := "\x1b[34mexplorer row\x1b[0m\nsecond row\nstatus bar"
		out := DimBackground(in, 1)
		lines := strings.Split(out, "\n")
		require.Len(t, lines, 3)
		assert.Equal(t, "explorer row", ansi.Strip(lines[0]))
		assert.Equal(t, "second row", ansi.Strip(lines[1]))
	})

	t.Run("keepLast zero dims every line", func(t *testing.T) {
		styled := "\x1b[32mall green\x1b[0m"
		in := styled + "\n" + styled
		out := DimBackground(in, 0)
		lines := strings.Split(out, "\n")
		require.Len(t, lines, 2)
		assert.Contains(t, lines[0], "\x1b[2m", "row 0 must carry faint SGR")
		assert.Contains(t, lines[0], "\x1b[32m", "row 0 must keep its green SGR")
		assert.Contains(t, lines[1], "\x1b[2m", "row 1 must carry faint SGR")
		assert.Contains(t, lines[1], "\x1b[32m", "row 1 must keep its green SGR")
	})

	t.Run("negative keepLast clamps to zero (dims everything)", func(t *testing.T) {
		in := "alpha\nbeta\ngamma"
		out := DimBackground(in, -1)
		lines := strings.Split(out, "\n")
		require.Len(t, lines, 3)
		assert.Equal(t, "alpha", ansi.Strip(lines[0]))
		assert.Equal(t, "beta", ansi.Strip(lines[1]))
		assert.Equal(t, "gamma", ansi.Strip(lines[2]))
	})

	t.Run("empty dimmed lines stay empty", func(t *testing.T) {
		in := "\n\nkeep"
		out := DimBackground(in, 1)
		lines := strings.Split(out, "\n")
		require.Len(t, lines, 3)
		assert.Equal(t, "", lines[0])
		assert.Equal(t, "", lines[1])
		assert.Equal(t, "keep", lines[2])
	})

	t.Run("emits raw SGR faint on every dimmed row", func(t *testing.T) {
		in := "explorer\nrow two\nstatus bar"
		out := DimBackground(in, 1)
		lines := strings.Split(out, "\n")
		require.Len(t, lines, 3)
		for i := range 2 {
			assert.True(t, strings.HasPrefix(lines[i], "\x1b[2m"),
				"dimmed row %d must start with faint SGR, got %q", i, lines[i])
			assert.True(t, strings.HasSuffix(lines[i], "\x1b[0m"),
				"dimmed row %d must end with reset, got %q", i, lines[i])
		}
		assert.Equal(t, "status bar", lines[2], "kept line stays plain")
	})

	t.Run("re-applies faint after every internal reset", func(t *testing.T) {
		in := "\x1b[34mfoo\x1b[0m bar \x1b[31mbaz\x1b[0m\nkeep"
		out := DimBackground(in, 1)
		lines := strings.Split(out, "\n")
		require.Len(t, lines, 2)
		assert.Equal(t, 3, strings.Count(lines[0], "\x1b[2m"),
			"faint must be re-applied after each of the two internal resets plus the leading wrap")
	})

	t.Run("respects ConfigNoColor", func(t *testing.T) {
		prev := ConfigNoColor
		t.Cleanup(func() { ConfigNoColor = prev })
		ConfigNoColor = true

		in := "\x1b[34mexplorer\x1b[0m\nstatus"
		out := DimBackground(in, 1)
		assert.Equal(t, in, out, "no-color mode must not inject any SGR escapes")
	})

	// `strings.Split("row1\nrow2\nrow3\n", "\n")` produces a 4-element
	// slice with an empty trailing entry. Without normalising that, a
	// `keepLast=1` would "preserve" the empty trailer and still dim
	// row3 — the actual last visible row. We normalise the trailing
	// newline before split and restore it on the way out so keepLast
	// always counts visible rows.
	t.Run("normalises trailing newline before counting keepLast", func(t *testing.T) {
		in := "row1\nrow2\nrow3\n"
		out := DimBackground(in, 1)
		assert.True(t, strings.HasSuffix(out, "\n"),
			"trailing newline on input must be preserved on output")
		// Split on the trimmed output so the trailing-newline split
		// element doesn't bleed into the visible-row indexing.
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		require.Len(t, lines, 3, "three visible rows must remain after dim")
		assert.Contains(t, lines[0], "\x1b[2m", "row1 must be dimmed")
		assert.Contains(t, lines[1], "\x1b[2m", "row2 must be dimmed")
		assert.NotContains(t, lines[2], "\x1b[2m",
			"row3 (the actual last visible row) must remain bright")
	})
}
