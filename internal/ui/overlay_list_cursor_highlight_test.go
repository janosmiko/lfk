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

// TestRenderOverlayList_BadgeInHighlight verifies that BadgeInHighlight extends
// the cursor-row selection highlight across the badge column (the whole line is
// highlighted), while the default keeps the badge outside the highlight.
func TestRenderOverlayList_BadgeInHighlight(t *testing.T) {
	forceTrueColorTheme(t)

	// Badge painted on the selection background (as the AutoSync overlay does
	// for its selected row) so it blends with the highlight.
	selBadge := OverlaySelectedStyle.Render(" ON")
	items := []OverlayListItem{{Name: "AB", Badge: selBadge}}

	withFlag := RenderOverlayList(items, OverlayListConfig{
		Cursor:           0,
		BadgeWidth:       3,
		BadgeInHighlight: true,
	}, 42)
	// Highlight spans the full inner width: label area + separator + badge.
	assert.Equal(t, 42, selectedHighlightWidth(withFlag), "highlight should span the whole row including the badge")

	without := RenderOverlayList(items, OverlayListConfig{
		Cursor:     0,
		BadgeWidth: 3,
	}, 42)
	// itemWidth = innerW(42) - badgeReserve(3+1) = 38; the surface-bg
	// separator breaks the selection run before the badge.
	require.Equal(t, 38, selectedHighlightWidth(without), "default keeps the badge outside the highlight")
}
