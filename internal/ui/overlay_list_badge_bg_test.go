package ui

import (
	"regexp"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

// nakedSpaceAfterReset matches a run of spaces sitting directly after an SGR
// reset — i.e. cells painted with the terminal's default background instead
// of the theme's surface background. Any hit inside an overlay row is the
// "background hole" bug: with a themed surface bg, the gap between the item
// text and the badge (or scrollbar) flashes the terminal background.
var nakedSpaceAfterReset = regexp.MustCompile(`\x1b\[0m +`)

// applyBadgeBgTestTheme applies a theme with a known surface color so the
// Overlay* styles carry a real background, restoring everything on cleanup.
func applyBadgeBgTestTheme(t *testing.T) {
	t.Helper()
	origNoColor := ConfigNoColor
	origContrast := ConfigMinContrastRatio
	origTransparent := ConfigTransparentBg
	origTheme := ActiveTheme
	t.Cleanup(func() {
		ConfigNoColor = origNoColor
		ConfigMinContrastRatio = origContrast
		ConfigTransparentBg = origTransparent
		ApplyTheme(origTheme)
	})
	ConfigNoColor = false
	ConfigMinContrastRatio = 0
	ConfigTransparentBg = false
	ApplyTheme(DefaultTheme())
}

// TestRenderOverlayList_BadgeGapKeepsSurfaceBackground regression-guards the
// AutoSync overlay report: with a themed surface background, the padding
// between a row's text and its badge column rendered as raw spaces, punching
// through to the terminal's default background.
func TestRenderOverlayList_BadgeGapKeepsSurfaceBackground(t *testing.T) {
	applyBadgeBgTestTheme(t)

	badge := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Background(SurfaceBg).
		Render(" ON")
	items := []OverlayListItem{
		{Name: "AutoSync      ", Badge: badge},
		{Name: "Self-Heal     ", Badge: badge},
		{Name: "Prune         ", Badge: badge, Disabled: true},
	}
	out := RenderOverlayList(items, OverlayListConfig{
		Title:      "Configure AutoSync",
		Cursor:     0,
		BadgeWidth: 3,
		FooterHint: "space: toggle | enter: save | esc: cancel",
	}, 42)

	assert.NotRegexp(t, nakedSpaceAfterReset, out,
		"no overlay row may contain unstyled spaces after an SGR reset — "+
			"the badge-gap padding must carry the surface background")
}

// TestRenderOverlayList_ScrollbarGapKeepsSurfaceBackground covers the same
// hole for overflowing lists without badges: the padding between short rows
// and the scrollbar column must carry the surface background too.
func TestRenderOverlayList_ScrollbarGapKeepsSurfaceBackground(t *testing.T) {
	applyBadgeBgTestTheme(t)

	items := make([]OverlayListItem, 10)
	for i := range items {
		items[i] = OverlayListItem{Name: "item"}
	}
	out := RenderOverlayList(items, OverlayListConfig{
		Title:      "Scrolling",
		Cursor:     0,
		MaxVisible: 4,
	}, 42)

	assert.NotRegexp(t, nakedSpaceAfterReset, out,
		"scrollbar-gap padding must carry the surface background")
}
