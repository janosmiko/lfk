package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noSurfaceBg pins these fixtures to the package's zero-value SurfaceBg
// (color.Color = lipgloss.NoColor{} at init) instead of the mutable
// SurfaceBg var, which other tests permanently repoint via ApplyTheme.
var noSurfaceBg = lipgloss.NoColor{}

func goldenCompare(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "overlay_golden", name+".golden")
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}

// styledBgLine returns a themed, styled background line of the given cell
// width (CJK-safe: build from single-width runs plus wide runes to hit an
// exact width).
func styledBgLine(width int) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile)).Background(noSurfaceBg)
	return style.Render(strings.Repeat(".", width))
}

func themedOverlayBox(lines ...string) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorPrimary)).
		Foreground(lipgloss.Color(ColorError)).
		Background(noSurfaceBg).
		Padding(0, 1)
	return style.Render(strings.Join(lines, "\n"))
}

func TestPlaceOverlay_Golden_StyledThemeBackground(t *testing.T) {
	const width, height = 30, 6
	bg := strings.Join([]string{
		styledBgLine(width), styledBgLine(width), styledBgLine(width),
		styledBgLine(width), styledBgLine(width), styledBgLine(width),
	}, "\n")
	overlay := themedOverlayBox("Confirm", "yes / no")
	got := PlaceOverlay(width, height, overlay, bg)
	goldenCompare(t, "place_overlay_styled_theme", got)
}

func TestPlaceOverlay_Golden_OverlayPartlyOffScreen(t *testing.T) {
	// Overlay wider than the background forces startCol to clamp to 0 and
	// the right-hand background slice to run past the line's own width.
	const width, height = 10, 4
	bgLine := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile)).Background(noSurfaceBg).Render(strings.Repeat("b", width))
	bg := strings.Join([]string{bgLine, bgLine, bgLine, bgLine}, "\n")
	overlay := themedOverlayBox("wider than bg")
	got := PlaceOverlay(width, height, overlay, bg)
	goldenCompare(t, "place_overlay_partly_offscreen", got)
}

func TestPlaceOverlayBottom_Golden_StyledThemeBackground(t *testing.T) {
	const width, height, margin = 24, 8, 1
	bg := strings.Join([]string{
		styledBgLine(width), styledBgLine(width), styledBgLine(width), styledBgLine(width),
		styledBgLine(width), styledBgLine(width), styledBgLine(width), styledBgLine(width),
	}, "\n")
	overlay := themedOverlayBox("j/k", "esc: close")
	got := PlaceOverlayBottom(width, height, margin, overlay, bg)
	goldenCompare(t, "place_overlay_bottom_styled_theme", got)
}

func TestPlaceOverlayBottom_Golden_OverlayPartlyOffScreen(t *testing.T) {
	// marginBottom exceeds height so startRow clamps to 0, and the overlay
	// is also wider than the background line, exercising both clamps at once.
	const width, height, margin = 10, 4, 100
	bgLine := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile)).Background(noSurfaceBg).Render(strings.Repeat("c", width))
	bg := strings.Join([]string{bgLine, bgLine, bgLine, bgLine}, "\n")
	overlay := themedOverlayBox("too wide here")
	got := PlaceOverlayBottom(width, height, margin, overlay, bg)
	goldenCompare(t, "place_overlay_bottom_partly_offscreen", got)
}

// overlayColumn returns the cell column where marker first appears in a
// stripped, rendered line, measuring by display width rather than byte or
// rune index so a wide rune earlier on the line doesn't skew the result.
func overlayColumn(t *testing.T, line, marker string) int {
	t.Helper()
	before, _, found := strings.Cut(ansi.Strip(line), marker)
	require.True(t, found, "marker %q not found in line %q", marker, line)
	return ansi.StringWidth(before)
}

func TestPlaceOverlay_KeepsFullRowWidthWhenWideRuneStraddlesOverlayEdge(t *testing.T) {
	const width, height = 20, 3
	const marker = "XXXXXX"
	bgLine := lipgloss.NewStyle().Background(noSurfaceBg).Render(strings.Repeat("A", 6) + "网" + strings.Repeat("A", 12))
	bg := strings.Join([]string{bgLine, bgLine, bgLine}, "\n")
	overlay := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Render(marker)

	got := PlaceOverlay(width, height, overlay, bg)
	lines := strings.Split(got, "\n")
	require.Len(t, lines, height)
	for i, line := range lines {
		assert.Equal(t, width, lipgloss.Width(line), "row %d is not the full requested width", i)
	}

	wantCol := (width - lipgloss.Width(marker)) / 2
	assert.Equal(t, wantCol, overlayColumn(t, lines[1], marker), "overlay must start at the centered column")
}

func TestPlaceOverlayBottom_KeepsFullRowWidthWhenWideRuneStraddlesOverlayEdge(t *testing.T) {
	const width, height, margin = 20, 6, 0
	const marker = "YYYYYY"
	bgLine := lipgloss.NewStyle().Background(noSurfaceBg).Render(strings.Repeat("A", 6) + "世界" + strings.Repeat("A", 10))
	bg := strings.Join([]string{bgLine, bgLine, bgLine, bgLine, bgLine, bgLine}, "\n")
	overlay := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Render(marker)

	got := PlaceOverlayBottom(width, height, margin, overlay, bg)
	lines := strings.Split(got, "\n")
	require.Len(t, lines, height)
	for i, line := range lines {
		assert.Equal(t, width, lipgloss.Width(line), "row %d is not the full requested width", i)
	}

	wantCol := (width - lipgloss.Width(marker)) / 2
	assert.Equal(t, wantCol, overlayColumn(t, lines[height-1], marker), "overlay must start at the centered column")
}
