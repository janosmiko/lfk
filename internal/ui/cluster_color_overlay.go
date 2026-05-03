package ui

import (
	"fmt"
	"strings"
)

// clusterColorPickerNoneRow is the user-visible label for the picker row
// that clears any previously assigned color.
const clusterColorPickerNoneRow = "None  (clear)"

// RenderClusterColorOverlay renders the color picker for the cluster
// highlighted in the cluster picker. Rows are the named colours in
// ClusterColorNames declaration order, followed by a "None" row that
// clears the assignment. cursor selects which row is highlighted.
//
// Each row is composed of three independently-styled segments:
//
//  1. label half — rendered with OverlayNormalStyle / OverlaySelectedStyle
//     so cursor highlight covers the text consistently.
//  2. swatch — rendered with the colour as a *background* on two spaces
//     so the colour wins over any outer style (a foreground-only swatch
//     gets clobbered by OverlaySelectedStyle's foreground when the row
//     is highlighted).
//  3. trailing pad — keeps the box body to a stable width so the right
//     border lands in the same column on every row.
func RenderClusterColorOverlay(contextName string, cursor int) string {
	titleText := "Set color for " + contextName
	if contextName == "" {
		titleText = "Set cluster color"
	}
	title := OverlayTitleStyle.Render(titleText)

	const labelW = 14 // "magenta" + headroom; enough for "None  (clear)" too

	rows := make([]string, 0, len(ClusterColorNames)+1)
	for i, name := range ClusterColorNames {
		rows = append(rows, renderClusterColorRow(name, name, labelW, i == cursor))
	}
	rows = append(rows, renderClusterColorRow("", clusterColorPickerNoneRow, labelW, cursor == len(ClusterColorNames)))

	hints := OverlayDimStyle.Render("↑↓ navigate | enter: apply | esc: cancel")
	body := title + "\n\n" + strings.Join(rows, "\n") + "\n\n" + hints
	return OverlayStyle.Render(body)
}

// renderClusterColorRow composes one row of the picker: label + swatch
// (rendered as background-on-spaces so its colour survives the cursor
// highlight). selected toggles between the highlight and the dim row
// styles for the textual portion.
func renderClusterColorRow(colorName, label string, labelW int, selected bool) string {
	rowStyle := OverlayNormalStyle
	if selected {
		rowStyle = OverlaySelectedStyle
	}
	padded := fmt.Sprintf("  %-*s  ", labelW, label)
	swatch := ClusterColorSwatchBg(colorName)
	return rowStyle.Render(padded) + swatch + rowStyle.Render("  ")
}
