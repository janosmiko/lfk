package ui

import "strings"

// RenderLogSinceOverlay renders the --since duration prompt.  It mirrors
// the look of the scale / pvc-resize overlays: a small centered card
// with a single text input plus a one-line hint under the field telling
// the user how to clear the filter.
func RenderLogSinceOverlay(input, active string) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Set Since Duration"))
	b.WriteString("\n\n")
	if active != "" {
		b.WriteString(OverlayDimStyle.Render("Current: " + active))
		b.WriteString("\n")
	}
	b.WriteString(OverlayNormalStyle.Render("Duration: "))
	if input == "" {
		b.WriteString(OverlayDimStyle.Render("e.g. 5m, 1h30m, 2d"))
	} else {
		b.WriteString(OverlayInputStyle.Render(input))
	}
	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("Enter to apply - empty to clear - Esc to cancel"))
	return b.String()
}
