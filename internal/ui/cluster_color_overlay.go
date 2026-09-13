package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// ClusterColorNoneLabel is the user-visible label for the picker row that
// clears any tint on the active context. Exposed so app-side helpers can
// render the same row when migrating ClusterColor to OverlayList.
const ClusterColorNoneLabel = "None  (clear)"

// ClusterColorSwatchN returns a width-cell coloured block for the given
// color name, suitable for use as an OverlayListItem.Badge. Empty or
// unknown names render as plain spaces so the "None" row stays aligned.
func ClusterColorSwatchN(name string, swatchW int) string {
	bg := clusterColorBg(name)
	if bg == nil {
		return strings.Repeat(" ", swatchW)
	}
	return lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", swatchW))
}
