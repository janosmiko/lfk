package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

// --- AgeStyle ---

func TestAgeStyle(t *testing.T) {
	// Helper to extract a comparable foreground color key from a style.
	fgKey := func(s lipgloss.Style) string {
		fg := s.GetForeground()
		r, g, b, a := fg.RGBA()
		return fmt.Sprintf("%d:%d:%d:%d", r, g, b, a)
	}

	dimFg := fgKey(DimStyle)
	cyanFg := fgKey(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)))
	greenFg := fgKey(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)))
	borderFg := fgKey(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)))

	tests := []struct {
		name       string
		age        string
		expectedFg string
		desc       string
	}{
		// Empty returns DimStyle.
		{"empty string", "", dimFg, "dim"},

		// Seconds: very new -> cyan.
		{"5 seconds", "5s", cyanFg, "cyan"},
		{"30 seconds", "30s", cyanFg, "cyan"},

		// Minutes: very new -> cyan.
		{"1 minute", "1m", cyanFg, "cyan"},
		{"59 minutes", "59m", cyanFg, "cyan"},

		// Hours < 24: recent -> green.
		{"1 hour", "1h", greenFg, "green"},
		{"12 hours", "12h", greenFg, "green"},
		{"23 hours", "23h", greenFg, "green"},

		// Hours >= 24: dim.
		{"24 hours", "24h", dimFg, "dim"},
		{"48 hours", "48h", dimFg, "dim"},

		// Days <= 7: dim.
		{"1 day", "1d", dimFg, "dim"},
		{"7 days", "7d", dimFg, "dim"},

		// Days > 7: extra dim (border color).
		{"8 days", "8d", borderFg, "border"},
		{"30 days", "30d", borderFg, "border"},
		{"365 days", "365d", borderFg, "border"},

		// Years: old -> border.
		{"1 year", "1y", borderFg, "border"},

		// Parse error returns dim.
		{"invalid number", "xm", dimFg, "dim"},
		{"no number", "m", dimFg, "dim"},

		// Unknown unit returns dim.
		{"unknown unit", "5x", dimFg, "dim"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := AgeStyle(tt.age)
			got := fgKey(style)
			assert.Equal(t, tt.expectedFg, got, "age=%q expected %s style", tt.age, tt.desc)
		})
	}
}

// --- FillLinesBg ---

// TestFillLinesBgReestablishesBgAfterShortReset guards issue #293's recurrence
// in the dashboard events column. lipgloss/reflow emit the parameterless SGR
// reset (ESC[m) at word-wrap boundaries, not the full ESC[0m. FillLinesBg must
// re-apply the background after both, or column padding following a
// wrap-induced reset renders with the terminal default (a black "tear").
func TestFillLinesBgReestablishesBgAfterShortReset(t *testing.T) {
	origProfile := lipgloss.DefaultRenderer().ColorProfile()
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(origProfile) })

	// Use an explicit color (not the theme-dependent BaseBg) so the test does
	// not depend on global theme state another test may have mutated.
	bg := lipgloss.Color("#1a1a2e")

	// Derive the exact bg sequence FillLinesBg injects, and guard against a
	// vacuous pass: a downgraded color profile would emit no sequence.
	sample := lipgloss.NewStyle().Background(bg).Render("X")
	bgSeq, _, _ := strings.Cut(sample, "X")
	if bgSeq == "" {
		t.Fatal("color profile must emit a background sequence for this test to be meaningful")
	}

	// A styled span closed by the parameterless reset, followed by spaces, and
	// already at full width so no trailing fill is appended — the wrap boundary
	// lipgloss produces (interior column padding follows the reset). The old
	// code left these spaces un-backgrounded.
	content := "\x1b[38;2;1;2;3mhello\x1b[m     "
	got := FillLinesBg(content, 10, bg)

	assert.Contains(t, got, "\x1b[m"+bgSeq,
		"background must be re-established immediately after the parameterless reset")
	assert.NotContains(t, got, "\x1b[m ",
		"no un-backgrounded padding may follow a parameterless reset")
}
