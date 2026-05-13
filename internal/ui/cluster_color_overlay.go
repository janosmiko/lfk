package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ClusterColorNoneLabel is the user-visible label for the picker row that
// clears any tint on the active context. Exposed so app-side helpers can
// render the same row when migrating ClusterColor to OverlayList.
const ClusterColorNoneLabel = "None  (clear)"

// ClusterColorSwatchN returns a width-cell coloured block for the given
// color name, suitable for use as an OverlayListItem.Badge. Empty or
// unknown names render as plain spaces so the "None" row stays aligned
// without a visible swatch.
func ClusterColorSwatchN(name string, width int) string {
	return clusterColorSwatchBgN(name, width)
}

// clusterColorSwatchBgN returns a swatchW-cell coloured block rendered
// as a background tint on whitespace. Empty / unknown name returns plain
// spaces so the "None" row stays aligned with the colour rows without
// adding a visible swatch. Routes through clusterColorBg so theme-mapped
// names (red/yellow/green/blue) follow the active colorscheme.
func clusterColorSwatchBgN(name string, swatchW int) string {
	bg := clusterColorBg(name)
	if bg == nil {
		return strings.Repeat(" ", swatchW)
	}
	return lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", swatchW))
}
