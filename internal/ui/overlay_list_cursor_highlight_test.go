package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	anySGR     = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	trueBgCode = regexp.MustCompile(`48;2;\d+;\d+;\d+`)
)

// forceTrueColorTheme applies the default theme with a TrueColor profile so the
// Overlay* styles emit real SGR backgrounds. The profile is set AFTER
// ApplyTheme — applying the theme can reset the renderer profile, so ordering
// it last is what makes the color stick when other package tests have run.
func forceTrueColorTheme(t *testing.T) {
	t.Helper()
	origProfile := lipgloss.DefaultRenderer().ColorProfile()
	origNoColor := ConfigNoColor
	origTheme := ActiveTheme
	t.Cleanup(func() {
		ConfigNoColor = origNoColor
		ApplyTheme(origTheme)
		lipgloss.DefaultRenderer().SetColorProfile(origProfile)
	})
	ConfigNoColor = false
	ApplyTheme(DefaultTheme())
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
}

// selectedHighlightWidth returns the visible width of the leading run of cells
// carrying the selection background (the cursor-row highlight). lipgloss emits
// the highlighted text and its width-padding as separate SGR runs sharing the
// same background, so we sum consecutive reset-delimited segments until the
// background changes.
func selectedHighlightWidth(out string) int {
	bg := trueBgCode.FindString(OverlaySelectedStyle.Render("x"))
	if bg == "" {
		return -1
	}
	total := 0
	for seg := range strings.SplitSeq(out, "\x1b[0m") {
		if !strings.Contains(seg, bg) {
			break
		}
		total += lipgloss.Width(anySGR.ReplaceAllString(seg, ""))
	}
	return total
}

// TestRenderOverlayList_CursorHighlightWidthCapsHighlight verifies the cursor
// highlight is capped to CursorHighlightWidth instead of spanning the full item
// area, so trailing fields (e.g. the AutoSync ON/OFF switches) are not
// highlighted. Default (0) keeps the full-width behaviour.
func TestRenderOverlayList_CursorHighlightWidthCapsHighlight(t *testing.T) {
	forceTrueColorTheme(t)

	items := []OverlayListItem{{Name: "AB", Badge: "X"}}

	capped := RenderOverlayList(items, OverlayListConfig{
		Cursor:               0,
		BadgeWidth:           1,
		CursorHighlightWidth: 6,
	}, 42)
	assert.Equal(t, 6, selectedHighlightWidth(capped), "highlight should be capped to CursorHighlightWidth")

	full := RenderOverlayList(items, OverlayListConfig{
		Cursor:     0,
		BadgeWidth: 1,
	}, 42)
	// itemWidth = innerW(42) - badgeReserve(2) = 40 with no cap.
	require.Equal(t, 40, selectedHighlightWidth(full), "default highlight spans the full item area")
}
