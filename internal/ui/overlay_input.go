package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// OverlayInputRow is one Label+Input pair inside an OverlayInput. Multiple
// rows are typically only needed for forms that read more than one value
// in the same screen (e.g. PortForward's remote + local pair); single-input
// overlays pass a one-element slice.
type OverlayInputRow struct {
	Label       string
	Input       string
	Placeholder string // shown dim when Input is empty
	ShowCursor  bool   // render a reverse-video cursor at Cursor
	Cursor      int    // byte offset of the cursor within Input (used when ShowCursor)
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

	// Notes are one-line facts about what the entered value costs, rendered
	// under the input rows. Same shape as the confirm overlay, so a caller
	// can word a blast radius once and show it in either.
	Notes []ConfirmNote

	// Optional candidate list. Rendered above the input rows when
	// Candidates is non-empty. CandidateCursor selects which row is
	// highlighted (-1 means "no candidate selected — cursor lives in the
	// input area").
	CandidateTitle  string
	Candidates      []OverlayListItem
	CandidateCursor int

	// Width is the inner content width. When > 0, the active (ShowCursor)
	// row is highlighted full-width — label and value on the selection
	// background, matching the selected row in list overlays.
	Width int

	// One or more input rows. Each row renders "<Label><Input or Placeholder>"
	// with optional trailing cursor.
	Rows []OverlayInputRow
}

// RenderOverlayInput renders an input overlay per cfg.
func RenderOverlayInput(cfg OverlayInputConfig) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render(cfg.Title))
	b.WriteString("\n")
	if cfg.Subtitle != "" {
		b.WriteString(OverlayDimStyle.Render(cfg.Subtitle))
		b.WriteString("\n")
	}
	b.WriteString("\n")

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
		switch {
		case row.ShowCursor && !row.ReadOnly && cfg.Width > 0:
			// Highlight the whole active row (label + value) full-width,
			// like the selected row in list overlays, with a visible cursor
			// cell inside the highlight.
			b.WriteString(renderActiveInputRow(row.Label, row.Input, row.Cursor, cfg.Width))
		case row.ShowCursor && !row.ReadOnly:
			// No width available: fall back to a reverse-video cursor cell
			// at the cursor offset (e.g. the batch label overlay).
			b.WriteString(OverlayNormalStyle.Render(row.Label))
			b.WriteString(overlayInputCursor(row.Input, row.Cursor))
		case row.Input == "" && row.Placeholder != "":
			b.WriteString(OverlayNormalStyle.Render(row.Label))
			b.WriteString(OverlayDimStyle.Render(row.Placeholder))
		case row.Input == "":
			b.WriteString(OverlayNormalStyle.Render(row.Label))
			b.WriteString(OverlayDimStyle.Render("_"))
		default:
			b.WriteString(OverlayNormalStyle.Render(row.Label))
			b.WriteString(OverlayNormalStyle.Render(row.Input))
		}
		if i < len(cfg.Rows)-1 {
			b.WriteString("\n")
		}
	}
	if len(cfg.Notes) > 0 {
		labelWidth := confirmNoteLabelWidth(cfg.Notes)
		b.WriteString("\n")
		for _, note := range cfg.Notes {
			b.WriteString("\n")
			b.WriteString(renderConfirmNote(note, labelWidth, cfg.Width))
		}
	}
	return b.String()
}

// renderActiveInputRow renders the active row (label + value) on the selection
// background, padded to width, with a visible cursor cell at the cursor offset.
// The cursor cell reverses the selection colors so it stands out within the
// highlighted row. A cursor at the end of the value yields a trailing cell.
func renderActiveInputRow(label, value string, cursor, width int) string {
	sel := OverlaySelectedStyle
	cur := OverlaySelectedStyle.Reverse(true)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(value) {
		cursor = len(value)
	}

	var b strings.Builder
	b.WriteString(sel.Render(label))
	used := lipgloss.Width(label) + lipgloss.Width(value)
	if cursor == len(value) {
		b.WriteString(sel.Render(value))
		b.WriteString(cur.Render(" "))
		used++ // trailing cursor cell
	} else {
		end := nextRuneEnd(value, cursor)
		b.WriteString(sel.Render(value[:cursor]))
		b.WriteString(cur.Render(value[cursor:end]))
		b.WriteString(sel.Render(value[end:]))
	}
	if pad := width - used; pad > 0 {
		b.WriteString(sel.Render(strings.Repeat(" ", pad)))
	}
	return b.String()
}

// overlayInputCursor renders s with a reverse-video cursor at the byte offset
// cursor, for overlays that don't supply a Width (no full-row highlight).
// A cursor at len(s) (or an empty input) yields a trailing cursor cell.
func overlayInputCursor(s string, cursor int) string {
	cursorStyle := OverlayNormalStyle.Reverse(true).Background(SurfaceBg)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(s) {
		cursor = len(s)
	}
	if cursor == len(s) {
		return OverlayNormalStyle.Render(s) + cursorStyle.Render(" ")
	}
	end := nextRuneEnd(s, cursor)
	return OverlayNormalStyle.Render(s[:cursor]) + cursorStyle.Render(s[cursor:end]) + OverlayNormalStyle.Render(s[end:])
}
