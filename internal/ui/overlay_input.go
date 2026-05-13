package ui

import (
	"strings"
)

// OverlayInputRow is one Label+Input pair inside an OverlayInput. Multiple
// rows are typically only needed for forms that read more than one value
// in the same screen (e.g. PortForward's remote + local pair); single-input
// overlays pass a one-element slice.
type OverlayInputRow struct {
	Label       string
	Input       string
	Placeholder string // shown dim when Input is empty
	ShowCursor  bool   // append a dim █ block after Input
	ReadOnly    bool   // render the value but suppress cursor
}

// OverlayInputConfig is the rendering contract for the unified input
// overlay. Covers Scale, PVCResize, BatchLabel, and PortForward's hybrid
// list+input shape (set Candidates to render a selectable list above the
// input rows).
type OverlayInputConfig struct {
	Title    string
	Subtitle string // dim line under title (e.g. resource name)
	Hint     string // dim line under subtitle (e.g. "Current: 10Gi")

	// Optional candidate list. Rendered above the input rows when
	// Candidates is non-empty. CandidateCursor selects which row is
	// highlighted (-1 means "no candidate selected — cursor lives in the
	// input area").
	CandidateTitle  string
	Candidates      []OverlayListItem
	CandidateCursor int

	// One or more input rows. Each row renders "<Label><Input or Placeholder>"
	// with optional trailing cursor.
	Rows []OverlayInputRow
}

// RenderOverlayInput renders an input overlay per cfg.
func RenderOverlayInput(cfg OverlayInputConfig) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render(cfg.Title))
	if cfg.Subtitle != "" {
		b.WriteString(OverlayDimStyle.Render("  " + cfg.Subtitle))
	}
	b.WriteString("\n\n")

	if cfg.Hint != "" {
		b.WriteString(OverlayDimStyle.Render(cfg.Hint))
		b.WriteString("\n")
	}

	if len(cfg.Candidates) > 0 {
		if cfg.CandidateTitle != "" {
			b.WriteString(OverlayNormalStyle.Render(cfg.CandidateTitle))
			b.WriteString("\n")
		}
		for i, c := range cfg.Candidates {
			label := "  " + c.Name
			if i == cfg.CandidateCursor {
				b.WriteString(OverlaySelectedStyle.Render(label))
			} else {
				b.WriteString(OverlayNormalStyle.Render(label))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	for i, row := range cfg.Rows {
		b.WriteString(OverlayNormalStyle.Render(row.Label))
		switch {
		case row.Input == "" && row.Placeholder != "":
			b.WriteString(OverlayDimStyle.Render(row.Placeholder))
		case row.Input == "":
			b.WriteString(OverlayDimStyle.Render("_"))
		default:
			b.WriteString(OverlayInputStyle.Render(row.Input))
		}
		if row.ShowCursor && !row.ReadOnly {
			b.WriteString(OverlayDimStyle.Render("█"))
		}
		if i < len(cfg.Rows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
