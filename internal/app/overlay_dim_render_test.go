package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

// When the dim option is enabled, rows above the dialog (and not in its
// horizontal range either) must carry the SGR 2 (faint) wrap so terminals
// render them dimmer than the un-dimmed rows the overlay box overwrites.
func TestRenderOverlayDimAppliesWhenEnabled(t *testing.T) {
	prev := ui.ConfigDimOverlay
	t.Cleanup(func() { ui.ConfigDimOverlay = prev })
	ui.ConfigDimOverlay = true

	m := Model{
		overlay: overlayQuitConfirm,
		width:   80,
		height:  10,
	}
	bg := strings.Repeat("explorer row\n", 10)
	out := m.renderOverlay(bg)
	// Trim a possible trailing newline before splitting so the bottom-row
	// assertion below indexes the actual hint-bar row rather than an
	// empty split element (which would trivially satisfy NotContains).
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	require.GreaterOrEqual(t, len(lines), m.height)

	assert.Contains(t, lines[0], "\x1b[2m", "top row must carry faint SGR")
	assert.NotContains(t, lines[m.height-1], "\x1b[2m",
		"bottom hint row (m.height-1) must remain bright")
}

func TestRenderOverlayDimSkippedWhenDisabled(t *testing.T) {
	prev := ui.ConfigDimOverlay
	t.Cleanup(func() { ui.ConfigDimOverlay = prev })
	ui.ConfigDimOverlay = false

	m := Model{
		overlay: overlayQuitConfirm,
		width:   80,
		height:  10,
	}
	bg := strings.Repeat("explorer row\n", 10)
	out := m.renderOverlay(bg)
	for i, line := range strings.Split(out, "\n") {
		assert.NotContains(t, line, "\x1b[2m", "row %d must not carry faint SGR when option is off", i)
	}
}

func TestRenderOverlayDimSkippedForColorschemePicker(t *testing.T) {
	prev := ui.ConfigDimOverlay
	t.Cleanup(func() { ui.ConfigDimOverlay = prev })
	ui.ConfigDimOverlay = true

	m := Model{
		overlay: overlayColorscheme,
		width:   80,
		height:  10,
	}
	bg := strings.Repeat("explorer row\n", 10)
	out := m.renderOverlay(bg)
	for i, line := range strings.Split(out, "\n") {
		assert.NotContains(t, line, "\x1b[2m",
			"row %d must not carry faint SGR while the colourscheme picker is up", i)
	}
}

func TestRenderOverlayDimSkippedWhenNoColor(t *testing.T) {
	prevDim := ui.ConfigDimOverlay
	prevNoColor := ui.ConfigNoColor
	t.Cleanup(func() {
		ui.ConfigDimOverlay = prevDim
		ui.ConfigNoColor = prevNoColor
	})
	ui.ConfigDimOverlay = true
	ui.ConfigNoColor = true

	m := Model{
		overlay: overlayQuitConfirm,
		width:   80,
		height:  10,
	}
	bg := strings.Repeat("explorer row\n", 10)
	out := m.renderOverlay(bg)
	for i, line := range strings.Split(out, "\n") {
		assert.NotContains(t, line, "\x1b[2m",
			"row %d must not carry faint SGR while no_color is on", i)
	}
}

// Theme colours and background fills in the explorer must survive the
// dim wrap. The previous strip-and-rebuild approach erased lipgloss's
// BarBg / SurfaceBg fills behind the breadcrumb. We verify the contract
// by feeding in a styled background with both fg and bg SGR runs and
// asserting both survive.
func TestRenderOverlayDimPreservesThemeColors(t *testing.T) {
	prev := ui.ConfigDimOverlay
	t.Cleanup(func() { ui.ConfigDimOverlay = prev })
	ui.ConfigDimOverlay = true

	styled := "\x1b[48;5;235m\x1b[37mlfk > context > Pods\x1b[0m"
	bg := styled + "\n" + strings.Repeat("explorer row\n", 9)

	m := Model{
		overlay: overlayQuitConfirm,
		width:   80,
		height:  10,
	}
	out := m.renderOverlay(bg)
	lines := strings.Split(out, "\n")
	require.GreaterOrEqual(t, len(lines), 1)

	assert.Contains(t, lines[0], "\x1b[2m", "breadcrumb row must carry faint SGR")
	assert.Contains(t, lines[0], "\x1b[48;5;235m", "breadcrumb bg fill must survive the dim wrap")
	assert.Contains(t, lines[0], "\x1b[37m", "breadcrumb fg colour must survive the dim wrap")
}
