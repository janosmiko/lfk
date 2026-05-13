package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// OverlayConfirmConfig is the rendering contract for the unified confirm
// overlay. It covers four historical shapes:
//
//   - Simple confirm: Title + Warning.
//   - Paste-style: Title + Body lines (no warning).
//   - Type-to-confirm: Title + Warning + Type<token> input row.
//   - Centered single-line: Title centered in a fixed-size box (Quit).
//
// Keymap hints are deliberately NOT rendered here — they live in the
// shared hint bar so two confirm dialogs never disagree on their footer.
type OverlayConfirmConfig struct {
	Title   string
	Warning string   // warning-styled question line (e.g. "Delete my-pod?")
	Body    []string // optional plain-style body lines

	// TypeToken triggers the type-to-confirm row. When non-empty, the
	// overlay prompts the user to type the token verbatim; Input is the
	// current accumulator. An empty Input renders a dim placeholder.
	TypeToken string
	Input     string

	// Centered mode renders Title alone, centered both axes inside a
	// fixed-size box. Used by the Quit overlay; ignores Warning, Body,
	// and TypeToken when true.
	Centered    bool
	InnerWidth  int
	InnerHeight int
}

// RenderOverlayConfirm renders a confirm overlay per cfg.
func RenderOverlayConfirm(cfg OverlayConfirmConfig) string {
	if cfg.Centered {
		return OverlayTitleStyle.
			Padding(0).
			Width(cfg.InnerWidth).
			Height(cfg.InnerHeight).
			Align(lipgloss.Center, lipgloss.Center).
			Render(cfg.Title)
	}

	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render(cfg.Title))
	b.WriteString("\n\n")
	if cfg.Warning != "" {
		b.WriteString(OverlayWarningStyle.Render(cfg.Warning))
		b.WriteString("\n\n")
	}
	for i, line := range cfg.Body {
		b.WriteString(OverlayNormalStyle.Render(line))
		if i < len(cfg.Body)-1 {
			b.WriteString("\n")
		}
	}
	if cfg.TypeToken != "" {
		b.WriteString(OverlayNormalStyle.Render("Type "))
		b.WriteString(OverlayFilterStyle.Render(cfg.TypeToken))
		b.WriteString(OverlayNormalStyle.Render(" to confirm: "))
		if cfg.Input == "" {
			b.WriteString(OverlayDimStyle.Render("_"))
		} else {
			b.WriteString(OverlayFilterStyle.Render(cfg.Input))
		}
	}
	return b.String()
}
