package app

import (
	"regexp"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/ui"
)

// nakedSpaceAfterReset matches spaces directly after an SGR reset — cells
// painted with the terminal default background instead of the theme surface.
// See the matching guard in internal/ui/overlay_list_badge_bg_test.go.
var nakedSpaceAfterReset = regexp.MustCompile(`\x1b\[0m +`)

// TestRenderAutoSyncOverlay_NoBackgroundHoles regression-guards the reported
// bug: with a theme applied, the gap between the AutoSync / Self-Heal / Prune
// labels and the ON/OFF badges rendered with the terminal's default
// background instead of the overlay's surface background.
func TestRenderAutoSyncOverlay_NoBackgroundHoles(t *testing.T) {
	origProfile := lipgloss.DefaultRenderer().ColorProfile()
	origNoColor := ui.ConfigNoColor
	origContrast := ui.ConfigMinContrastRatio
	origTransparent := ui.ConfigTransparentBg
	origTheme := ui.ActiveTheme
	t.Cleanup(func() {
		ui.ConfigNoColor = origNoColor
		ui.ConfigMinContrastRatio = origContrast
		ui.ConfigTransparentBg = origTransparent
		ui.ApplyTheme(origTheme)
		lipgloss.DefaultRenderer().SetColorProfile(origProfile)
	})
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	ui.ConfigNoColor = false
	ui.ConfigMinContrastRatio = 0
	ui.ConfigTransparentBg = false
	ui.ApplyTheme(ui.DefaultTheme())

	m := basePush80Model()
	m.autoSyncEnabled = true
	m.autoSyncSelfHeal = false
	m.autoSyncPrune = true
	m.autoSyncCursor = 1

	out := renderAutoSyncOverlay(m)
	assert.NotRegexp(t, nakedSpaceAfterReset, out,
		"the AutoSync overlay must not contain unstyled spaces after an SGR "+
			"reset — label-to-badge gaps and badges must carry the surface bg")
}
