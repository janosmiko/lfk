package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// renderPortForwardOverlay maps the PortForward overlay state onto the
// unified OverlayInput component. The list of available ports becomes
// the candidate list; whether the cursor sits on a candidate determines
// the input rows: either two pinned rows (Remote read-only + Local) or
// a single free-form "Port mapping" row when the cursor is in manual mode.
func renderPortForwardOverlay(m Model) string {
	cfg := ui.OverlayInputConfig{
		Title:           "Port Forward",
		Subtitle:        m.actionCtx.name,
		CandidateTitle:  "Available ports:",
		CandidateCursor: m.pfPortCursor,
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
	case m.pfPortCursor >= 0 && m.pfPortCursor < len(m.pfAvailablePorts):
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
