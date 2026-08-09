package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
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
// ConfirmNote is one line of consequence shown in a confirm overlay, such as
// the disruption budget headroom an action would spend.
type ConfirmNote struct {
	Text string
	Warn bool
}

type OverlayConfirmConfig struct {
	Title   string
	Warning string   // warning-styled question line (e.g. "Delete my-pod?")
	Body    []string // optional plain-style body lines

	// ChoiceLabel/ChoiceValue render a labeled, cycleable value row below the
	// warning (e.g. "Cascade: Background"). An empty ChoiceValue omits the
	// row; the hotkey that cycles it lives in the hint bar. ChoiceWarn styles
	// the value as a warning so a riskier selection is not visually
	// interchangeable with a safe one.
	ChoiceLabel string
	ChoiceValue string
	ChoiceWarn  bool

	// Notes are one-line facts about what the action costs, rendered under
	// the choice row. Warn styles a note as a warning, so a budget the
	// action would breach does not look like an ordinary line.
	Notes []ConfirmNote

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

// WrappedLineCount reports how many rows text takes at this width, so a
// caller can size a box around content the overlay will wrap.
func WrappedLineCount(text string, width int) int {
	if width <= 0 {
		return 1
	}
	return len(wrapConfirmText(text, width))
}

// wrapConfirmText word-wraps text to width display columns on whitespace
// boundaries so words (and hyphenated resource names) stay intact. A single
// token wider than width is hard-split on rune boundaries so no line exceeds
// width and lipgloss never re-wraps with its hyphen-breaking default. Widths
// use lipgloss.Width so multibyte/wide characters are measured correctly.
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
		for lipgloss.Width(word) > width {
			flush()
			head, rest := splitAtWidth(word, width)
			lines = append(lines, head)
			word = rest
		}
		switch {
		case cur == "":
			cur = word
		case lipgloss.Width(cur)+1+lipgloss.Width(word) <= width:
			cur += " " + word
		default:
			flush()
			cur = word
		}
	}
	flush()
	return lines
}

// splitAtWidth returns the longest prefix of s whose display width is <= width
// and the remainder, splitting on rune boundaries so multibyte characters are
// never cut in half.
func splitAtWidth(s string, width int) (string, string) {
	w := 0
	for i, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > width {
			return s[:i], s[i:]
		}
		w += rw
	}
	return s, ""
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
	if cfg.ChoiceValue != "" {
		// An empty label drops the prefix entirely — callers shed it to fit a
		// narrow box, and ": value" would read as a rendering bug.
		if cfg.ChoiceLabel != "" {
			b.WriteString(OverlayNormalStyle.Render(cfg.ChoiceLabel + ": "))
		}
		valueStyle := OverlayFilterStyle
		if cfg.ChoiceWarn {
			valueStyle = OverlayWarningStyle
		}
		b.WriteString(valueStyle.Render(cfg.ChoiceValue))
		b.WriteString("\n\n")
	}
	for _, note := range cfg.Notes {
		style := OverlayNormalStyle
		if note.Warn {
			style = OverlayWarningStyle
		}
		text := note.Text
		if cfg.WrapWidth > 0 {
			text = strings.Join(wrapConfirmText(text, cfg.WrapWidth), "\n")
		}
		b.WriteString(style.Render(text))
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
