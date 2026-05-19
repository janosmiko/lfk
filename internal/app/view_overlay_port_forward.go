package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/ui"
)

// renderPortForwardOverlay maps the PortForward overlay state onto the
// unified OverlayInput component. The list of available ports becomes the
// candidate list. A ':' in the input puts the overlay in manual local:remote
// mode (single free-form "Port mapping" row, no candidate highlight);
// otherwise, with a port selected, it shows two pinned rows (Remote
// read-only + Local).
func renderPortForwardOverlay(m Model) string {
	// A ':' in the input switches to manual local:remote entry. The list
	// selection is set aside (no highlight, no Remote/Local rows) but kept
	// in pfPortCursor, so clearing the ':' restores it.
	manualMapping := strings.Contains(m.portForwardInput.Value, ":")
	candidateCursor := m.pfPortCursor
	if manualMapping {
		candidateCursor = -1
	}
	cfg := ui.OverlayInputConfig{
		Title:           "Port Forward",
		Subtitle:        m.actionCtx.name,
		CandidateTitle:  "Available ports:",
		CandidateCursor: candidateCursor,
	}
	if len(m.pfAvailablePorts) > 0 {
		cfg.Candidates = make([]ui.OverlayListItem, len(m.pfAvailablePorts))
		for i, p := range m.pfAvailablePorts {
			label := p.Port
			if p.Name != "" {
				label += " (" + p.Name + ")"
			}
			if p.Protocol != "" && p.Protocol != "TCP" {
				label += " [" + p.Protocol + "]"
			}
			cfg.Candidates[i] = ui.OverlayListItem{Name: label}
		}
	}
	switch {
	case !manualMapping && m.pfPortCursor >= 0 && m.pfPortCursor < len(m.pfAvailablePorts):
		cfg.Rows = []ui.OverlayInputRow{
			{Label: "Remote port: ", Input: m.pfAvailablePorts[m.pfPortCursor].Port, ReadOnly: true},
			{Label: "Local port:  ", Input: m.portForwardInput.Value, Placeholder: "(random)"},
		}
	default:
		cfg.Rows = []ui.OverlayInputRow{
			{Label: "Port mapping: ", Input: m.portForwardInput.Value, Placeholder: "local:remote"},
		}
	}
	return ui.RenderOverlayInput(cfg)
}
