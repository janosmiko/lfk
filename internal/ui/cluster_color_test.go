package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestClusterColorNames_PaletteIsStable(t *testing.T) {
	// The on-disk schema and the picker overlay both depend on this set —
	// dropping or renaming an entry is a breaking change. Adding a new one
	// is fine but must come with a schema bump in cluster_colors.go.
	expected := []string{"red", "yellow", "green", "blue", "magenta", "cyan", "white", "gray"}
	assert.Equal(t, expected, ClusterColorNames)
}

func TestIsValidClusterColor(t *testing.T) {
	for _, name := range ClusterColorNames {
		assert.True(t, IsValidClusterColor(name), "expected %q to be valid", name)
	}
	assert.False(t, IsValidClusterColor(""), "empty string is the unset sentinel, not a valid color")
	assert.False(t, IsValidClusterColor("chartreuse"), "unknown color name must be rejected")
	assert.False(t, IsValidClusterColor("RED"), "names are case-sensitive (lowercase only)")
}

func TestClusterColorTitleBarStyle_KnownColorSetsBg(t *testing.T) {
	// Inspect style attributes directly — lipgloss strips ANSI escapes
	// when stdout isn't a TTY (e.g. during `go test`), so we cannot
	// rely on the rendered output here.
	style := ClusterColorTitleBarStyle("red")
	bg, ok := style.GetBackground().(lipgloss.Color)
	assert.True(t, ok, "known color must set a concrete lipgloss.Color background, not NoColor")
	assert.NotEmpty(t, string(bg), "background color must be non-empty")
	assert.True(t, style.GetBold(), "title bar tint should be bold so badges stay legible against bright backgrounds")
}

func TestClusterColorBg_ThemeMappedColorsFollowTheme(t *testing.T) {
	// Theme-mapped colour names must resolve through ActiveTheme so a
	// colorscheme switch propagates. Smoke test by mutating
	// ActiveTheme.Error and observing clusterColorBg("red") change.
	prev := ActiveTheme.Error
	t.Cleanup(func() { ActiveTheme.Error = prev })

	ActiveTheme.Error = "#abcdef"
	bg, ok := clusterColorBg("red").(lipgloss.Color)
	if assert.True(t, ok, "red must resolve to a concrete lipgloss.Color") {
		assert.Equal(t, "#abcdef", string(bg),
			"red must read the *current* ActiveTheme.Error so theme switches propagate")
	}
}

func TestClusterColorBg_AnsiMappedColorsAreFixed(t *testing.T) {
	// magenta/cyan/white/gray are deliberately NOT theme-mapped — they
	// give the user a stable palette-relative escape hatch when none of
	// the theme accent colours fit. Mutating theme tokens must not
	// touch them.
	prev := ActiveTheme.Error
	t.Cleanup(func() { ActiveTheme.Error = prev })

	ActiveTheme.Error = "#abcdef"
	for _, name := range []string{"magenta", "cyan", "white", "gray"} {
		bg, ok := clusterColorBg(name).(lipgloss.Color)
		if assert.True(t, ok, "%s must resolve to a concrete lipgloss.Color", name) {
			assert.NotEqual(t, "#abcdef", string(bg),
				"%s must NOT pick up theme.Error — palette-relative colours stay independent", name)
		}
	}
}

func TestClusterColorBg_UnknownNameReturnsNil(t *testing.T) {
	assert.Nil(t, clusterColorBg("chartreuse"), "unknown colour name resolves to nil so callers can no-op")
	assert.Nil(t, clusterColorBg(""), "unset (empty) name resolves to nil — same as no tint")
}

func TestClusterColorTitleBarStyle_UnknownColorIsZeroStyle(t *testing.T) {
	style := ClusterColorTitleBarStyle("chartreuse")
	// Zero style: GetBackground() returns NoColor{} for an unset background,
	// not a lipgloss.Color — that distinction is what tells the renderer to
	// pass through unchanged.
	_, isColor := style.GetBackground().(lipgloss.Color)
	assert.False(t, isColor, "unknown color must yield the zero style (NoColor background) so callers can compose unconditionally")
}

func TestClusterColorSwatch_KnownColorRendersBlock(t *testing.T) {
	out := ClusterColorSwatch("red")
	assert.Contains(t, out, "██", "known color renders the full block character so the swatch is visible at a glance")
}

func TestClusterColorSwatch_UnknownColorRendersDimDots(t *testing.T) {
	out := ClusterColorSwatch("")
	assert.Contains(t, out, "··", "unset/unknown color renders dim dots so rows stay aligned with coloured rows")
	// We can't reliably assert ANSI escapes here — see the title-bar test
	// above. The character choice (dots vs. blocks) is the user-visible
	// marker that this row has no color set.
	_ = strings.Contains(out, "")
}
