package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// clusterColorPickerNoneRow is the user-visible label for the picker row
// that clears any previously assigned color.
const clusterColorPickerNoneRow = "None  (clear)"

// RenderClusterColorOverlay renders the inner content of the colour
// picker for the cluster highlighted in the cluster picker. Returns
// raw, unwrapped content; the caller is expected to wrap it in
// OverlayStyle (renderOverlay does this for every standard overlay).
//
// Layout:
//
//	Set color for prod-eu
//
//	▶ red          █████
//	  yellow       █████
//	  ...
//	  None  (clear)
//
//	↑↓ navigate | enter: apply | esc/c: cancel
//
// The cursor uses a "▶" prefix marker rather than a row-wide background
// highlight so the swatch's own colour stays visible without fighting
// an outer style. The swatch is rendered with the named colour as a
// *background* on five spaces so it shows up even when the terminal
// renders foregrounds as muted.
func RenderClusterColorOverlay(contextName string, cursor int) string {
	titleText := "Set color for " + contextName
	if contextName == "" {
		titleText = "Set cluster color"
	}
	title := OverlayTitleStyle.Render(titleText)

	const (
		labelW   = 14 // "None  (clear)" is the longest entry; magenta etc fit comfortably
		swatchW  = 5  // 5-cell coloured block — wide enough to identify colours at a glance
		markerOn = "▶ "
		markerNo = "  "
	)

	rows := make([]string, 0, len(ClusterColorNames)+1)
	for i, name := range ClusterColorNames {
		rows = append(rows, formatClusterColorRow(name, name, labelW, swatchW, i == cursor, markerOn, markerNo))
	}
	rows = append(rows, formatClusterColorRow("", clusterColorPickerNoneRow, labelW, swatchW, cursor == len(ClusterColorNames), markerOn, markerNo))

	hints := OverlayDimStyle.Render("↑↓ navigate | enter: apply | esc/c: cancel")
	return title + "\n" + strings.Join(rows, "\n") + "\n\n" + hints
}

// formatClusterColorRow assembles one picker row as marker + label +
// swatch. The swatch uses background-on-spaces so its colour survives
// any outer style; the label and marker are styled per-segment so the
// cursor highlight is unambiguous without needing to span the whole row.
func formatClusterColorRow(colorName, label string, labelW, swatchW int, selected bool, markerOn, markerNo string) string {
	marker := markerNo
	labelStyle := OverlayNormalStyle
	if selected {
		marker = markerOn
		labelStyle = OverlaySelectedStyle
	}
	swatch := clusterColorSwatchBgN(colorName, swatchW)
	return labelStyle.Render(marker+fmt.Sprintf("%-*s", labelW, label)) + " " + swatch
}

// clusterColorSwatchBgN returns a swatchW-cell coloured block rendered
// as a background tint on whitespace. Empty / unknown name returns plain
// spaces so the "None" row stays aligned with the colour rows without
// adding a visible swatch.
func clusterColorSwatchBgN(name string, swatchW int) string {
	code, ok := ansiCodeForClusterColor[name]
	if !ok {
		return strings.Repeat(" ", swatchW)
	}
	return lipgloss.NewStyle().Background(lipgloss.Color(code)).Render(strings.Repeat(" ", swatchW))
}
