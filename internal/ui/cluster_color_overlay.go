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
// Hints live on the bottom-of-screen status bar (overlayHintBarSelector
// in internal/app/overlay_hintbar.go) — there is no inline hint row
// here so the overlay matches every other selector in lfk.
//
// Layout:
//
//	Set color for prod-eu
//	/ red█                     ← filter input (visible only when typing or non-empty)
//
//	▶ red          █████
//	  yellow       █████
//	  ...
//	  None  (clear)
//
// names is the post-filter list of colour names, cursor is the row
// index within (names + the trailing "None" row), filter is the
// current filter buffer (visible regardless of mode so the user can
// see what's narrowing the list), filterMode toggles the cursor glyph
// after the filter so they know typing is active.
func RenderClusterColorOverlay(contextName string, names []string, cursor int, filter string, filterMode bool) string {
	titleText := "Set color for " + contextName
	if contextName == "" {
		titleText = "Set cluster color"
	}
	title := OverlayTitleStyle.Render(titleText)

	// Filter line: matches the OverlayFilterStyle / OverlayDimStyle
	// pattern used by RenderColorschemeOverlay so the picker's filter
	// row looks identical to every other overlay's.
	var filterLine string
	switch {
	case filterMode:
		filterLine = OverlayFilterStyle.Render("/ " + filter + "█")
	case filter != "":
		filterLine = OverlayFilterStyle.Render("/ " + filter)
	default:
		filterLine = OverlayDimStyle.Render("/ to filter")
	}

	const (
		labelW   = 14 // "None  (clear)" is the longest entry; magenta etc fit comfortably
		swatchW  = 5  // 5-cell coloured block — wide enough to identify colours at a glance
		markerOn = "▶ "
		markerNo = "  "
	)

	rows := make([]string, 0, len(names)+1)
	if len(names) == 0 {
		rows = append(rows, OverlayDimStyle.Render("  No matching colors"))
	} else {
		for i, name := range names {
			rows = append(rows, formatClusterColorRow(name, name, labelW, swatchW, i == cursor, markerOn, markerNo))
		}
	}
	rows = append(rows, formatClusterColorRow("", clusterColorPickerNoneRow, labelW, swatchW, cursor == len(names), markerOn, markerNo))

	return title + "\n" + filterLine + "\n\n" + strings.Join(rows, "\n")
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
// adding a visible swatch. Routes through clusterColorBg so theme-mapped
// names (red/yellow/green/blue) follow the active colorscheme.
func clusterColorSwatchBgN(name string, swatchW int) string {
	bg := clusterColorBg(name)
	if bg == nil {
		return strings.Repeat(" ", swatchW)
	}
	return lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", swatchW))
}
