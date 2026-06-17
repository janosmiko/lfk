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

	// WrapWidth, when > 0, word-wraps the Warning to this many columns
	// before rendering. Without it, lipgloss wraps the styled line at the
	// box edge and breaks on hyphens / mid-word, which shatters long
	// resource names (e.g. dev-envs-autoscaled-cx43-...). Callers pass the
	// box inner width.
	WrapWidth int
}

// wrapConfirmText word-wraps text to width on whitespace boundaries so words
// (and hyphenated resource names) stay intact. A single token longer than
// width is hard-split into width-sized chunks so no line exceeds width and
// lipgloss never re-wraps with its hyphen-breaking default. Confirm text is
// ASCII, so byte length is a safe column proxy.
func wrapConfirmText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	var lines []string
	cur := ""
	flush := func() {
		if cur != "" {
			lines = append(lines, cur)
			cur = ""
		}
	}
	for word := range strings.FieldsSeq(text) {
		for len(word) > width {
			flush()
			lines = append(lines, word[:width])
			word = word[width:]
		}
		switch {
		case cur == "":
			cur = word
		case len(cur)+1+len(word) <= width:
			cur += " " + word
		default:
			flush()
			cur = word
		}
	}
	flush()
	return lines
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
		warning := cfg.Warning
		if cfg.WrapWidth > 0 {
			warning = strings.Join(wrapConfirmText(warning, cfg.WrapWidth), "\n")
		}
		b.WriteString(OverlayWarningStyle.Render(warning))
		b.WriteString("\n\n")
	}
	for i, line := range cfg.Body {
		b.WriteString(OverlayNormalStyle.Render(line))
		if i < len(cfg.Body)-1 {
			b.WriteString("\n")
		}
	}
	if cfg.TypeToken != "" {
		// Insert a blank line between body and the type-confirm row
		// so they don't read as a single merged line.
		if len(cfg.Body) > 0 {
			b.WriteString("\n\n")
		}
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
