package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// LogTimeRangeOverlayState carries the data needed to render the
// time-range picker overlay. The app layer owns the full LogTimeRange
// type; this struct is the layer-crossing adapter that the renderer
// reads without touching internal/app.
type LogTimeRangeOverlayState struct {
	// Presets is the list of preset labels shown in the picker, in
	// display order. Empty → no presets shown.
	Presets []string
	// Cursor indexes the preset currently highlighted.
	Cursor int
	// ActiveRangeDisplay is the pre-rendered chip body for the
	// range the user is about to commit (or has selected). Empty →
	// "(inactive)" is shown as a placeholder so the preview strip
	// never vanishes entirely.
	ActiveRangeDisplay string
	// StatusMsg is an ephemeral line shown beneath the preset list
	// (e.g. to explain that Custom… is coming in Phase 2).
	StatusMsg string
}

// RenderLogTimeRangeOverlay renders the preset-list time-range picker.
// The overlay is a centered card with a list of presets (Last 5m,
// Today, …), a preview strip showing the preset that's currently
// highlighted, and a hint line. Full editor panels land in Phase 2/3.
func RenderLogTimeRangeOverlay(state LogTimeRangeOverlayState, width, height int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Log time range"))
	b.WriteString("\n\n")
	b.WriteString(OverlayNormalStyle.Render("Presets"))
	b.WriteString("\n")

	// Render the preset list. Selected row is highlighted with the
	// same style used by the filter preset picker for consistency.
	// We don't rely on bubbles/list here because the list is short
	// (10 entries) and always static; hand-rolled rendering keeps the
	// overlay rendering path free of the bubbles state machine.
	selectedStyle := OverlaySelectedStyle.
		Padding(0, 1).
		Border(lipgloss.Border{Left: "\u258c"}, false, false, false, true).
		BorderForeground(lipgloss.Color(ColorPrimary))
	normalStyle := OverlayNormalStyle.Padding(0, 0, 0, 2)
	for i, label := range state.Presets {
		if i == state.Cursor {
			b.WriteString(selectedStyle.Render(label))
		} else {
			b.WriteString(normalStyle.Render(label))
		}
		b.WriteString("\n")
	}

	// Preview strip: the active/pending range chip body. A placeholder
	// is shown when the range is empty so the strip doesn't blink in
	// and out as the user cursors over the Custom… entry.
	preview := state.ActiveRangeDisplay
	if preview == "" {
		preview = "(inactive)"
	}
	b.WriteString("\n")
	b.WriteString(OverlayDimStyle.Render("Preview: "))
	b.WriteString(OverlayNormalStyle.Render(preview))
	b.WriteString("\n")

	if state.StatusMsg != "" {
		b.WriteString("\n")
		b.WriteString(OverlayDimStyle.Render(state.StatusMsg))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(OverlayDimStyle.Render("enter:apply  c:clear  esc:cancel"))
	_ = width
	_ = height
	return b.String()
}
