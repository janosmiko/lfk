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
	// Focus selects which panel gets the highlight border: 0 =
	// Presets, 1 = Start, 2 = End. Matches app.logTimeRangeFocus
	// ordinal values but the UI layer treats it as an opaque int.
	Focus int
	// Start / End hold the editor-panel state. When their Mode is
	// "Now", the inner spinner grid is omitted.
	Start LogTimeRangeEditorView
	End   LogTimeRangeEditorView
}

// LogTimeRangeEditorView is the UI-layer mirror of an editor panel.
// Mode is one of "Now", "Relative", "Absolute" — the renderer toggles
// radio dots and subsequently renders a spinner (Relative) or datetime
// (Absolute) strip based on it.
type LogTimeRangeEditorView struct {
	Mode string
	// RelDays / RelHrs / RelMin carry the three spinner values.
	RelDays, RelHrs, RelMin int
	// RelField is the ordinal of the focused spinner cell
	// (0=days, 1=hours, 2=minutes).
	RelField int
	// AbsYear / AbsMonth / ... hold the six absolute-datetime cells.
	AbsYear, AbsMonth, AbsDay, AbsHour, AbsMinute, AbsSecond int
	// AbsField is the ordinal of the focused absolute-datetime cell.
	AbsField int
}

// RenderLogTimeRangeOverlay renders the preset-list time-range picker.
// The overlay is a centered card with a preset column on the left and
// a Start / End editor column on the right. The focused panel gets a
// highlighted header so the user always knows where keystrokes land.
func RenderLogTimeRangeOverlay(state LogTimeRangeOverlayState, width, height int) string {
	presetCol := renderTimeRangePresetCol(state)
	editorCol := renderTimeRangeEditorCol(state)

	// Join the two columns side by side. Fallback to a single-column
	// layout when the viewport is too narrow.
	body := lipgloss.JoinHorizontal(lipgloss.Top, presetCol, "  ", editorCol)
	if width < 60 {
		body = presetCol + "\n\n" + editorCol
	}

	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Log time range"))
	b.WriteString("\n\n")
	b.WriteString(body)
	b.WriteString("\n\n")

	preview := state.ActiveRangeDisplay
	if preview == "" {
		preview = "(inactive)"
	}
	b.WriteString(OverlayDimStyle.Render("Preview: "))
	b.WriteString(OverlayNormalStyle.Render(preview))
	b.WriteString("\n")
	if state.StatusMsg != "" {
		b.WriteString("\n")
		b.WriteString(OverlayDimStyle.Render(state.StatusMsg))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(OverlayDimStyle.Render("tab:switch panel  enter:apply  c:clear  esc:cancel"))
	_ = height
	return b.String()
}

// renderTimeRangePresetCol renders the left column: the preset list.
// The column title gets the focused-panel highlight when state.Focus
// points at Presets (0).
func renderTimeRangePresetCol(state LogTimeRangeOverlayState) string {
	var b strings.Builder
	b.WriteString(panelHeader("Presets", state.Focus == 0))
	b.WriteString("\n")

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
	return b.String()
}

// renderTimeRangeEditorCol renders the right column: the Start and End
// editor panels stacked vertically. Each panel shows radio dots for
// the three modes (Now / Relative / Absolute) and — for Relative —
// the three d/h/m spinners.
func renderTimeRangeEditorCol(state LogTimeRangeOverlayState) string {
	var b strings.Builder
	b.WriteString(renderTimeRangeEditorPanel("Start:", state.Start, state.Focus == 1))
	b.WriteString("\n\n")
	b.WriteString(renderTimeRangeEditorPanel("End:", state.End, state.Focus == 2))
	return b.String()
}

// renderTimeRangeEditorPanel renders one Start/End panel: the header,
// the three radio buttons (Now / Relative / Absolute), and — for
// Relative — the three d/h/m spinners. Absolute mode's 6-field editor
// is rendered separately by renderTimeRangeAbsFields.
func renderTimeRangeEditorPanel(label string, ed LogTimeRangeEditorView, focused bool) string {
	var b strings.Builder
	b.WriteString(panelHeader(label, focused))
	b.WriteString("\n")
	b.WriteString(radio("Now", ed.Mode == "Now"))
	b.WriteString("\n")
	b.WriteString(radio("Relative", ed.Mode == "Relative"))
	b.WriteString("\n")
	b.WriteString(radio("Absolute", ed.Mode == "Absolute"))
	b.WriteString("\n")
	if ed.Mode == "Relative" {
		b.WriteString("\n")
		b.WriteString(renderTimeRangeRelFields(ed, focused))
	}
	if ed.Mode == "Absolute" {
		b.WriteString("\n")
		b.WriteString(renderTimeRangeAbsFields(ed, focused))
	}
	return b.String()
}

// renderTimeRangeRelFields renders the three d/h/m spinner cells for
// Relative mode. The focused cell (when the enclosing panel has focus)
// gets a primary-coloured border so the user can see which spinner
// will respond to j/k.
func renderTimeRangeRelFields(ed LogTimeRangeEditorView, focused bool) string {
	cells := []struct {
		value int
		label string
	}{
		{ed.RelDays, "days ago"},
		{ed.RelHrs, "hours ago"},
		{ed.RelMin, "mins ago"},
	}
	var row, labels strings.Builder
	for i, c := range cells {
		isFocused := focused && ed.RelField == i
		row.WriteString(renderSpinnerCell(c.value, isFocused))
		row.WriteString("  ")
		labels.WriteString(labelUnderCell(c.label, isFocused))
		labels.WriteString("  ")
	}
	return row.String() + "\n" + labels.String()
}

// renderSpinnerCell draws a single spinner value as "[ 24 ]" — three
// digits of room is enough for the 0..999 range the editor allows.
// When focused, the cell is rendered in the primary/selected style to
// mirror the filter overlay's selection highlight.
func renderSpinnerCell(v int, focused bool) string {
	text := "[ " + padInt(v, 3) + " ]"
	if focused {
		return OverlaySelectedStyle.Padding(0, 0).Render(text)
	}
	return OverlayNormalStyle.Render(text)
}

// renderTimeRangeAbsFields renders the 6-field absolute datetime
// editor. Phase 2 wires the struct through; Phase 3 adds nav/editing.
func renderTimeRangeAbsFields(ed LogTimeRangeEditorView, focused bool) string {
	cells := []struct {
		value, width int
	}{
		{ed.AbsYear, 4},
		{ed.AbsMonth, 2},
		{ed.AbsDay, 2},
		{ed.AbsHour, 2},
		{ed.AbsMinute, 2},
		{ed.AbsSecond, 2},
	}
	seps := []string{"-", "-", "  ", ":", ":"}
	var row strings.Builder
	for i, c := range cells {
		isFocused := focused && ed.AbsField == i
		row.WriteString(renderDatetimeCell(c.value, c.width, isFocused))
		if i < len(seps) {
			row.WriteString(seps[i])
		}
	}
	return row.String()
}

// renderDatetimeCell renders one zero-padded cell of the datetime
// editor with the same focus-styling rules as the spinner cells.
func renderDatetimeCell(v, width int, focused bool) string {
	text := "[" + padInt(v, width) + "]"
	if focused {
		return OverlaySelectedStyle.Padding(0, 0).Render(text)
	}
	return OverlayNormalStyle.Render(text)
}

// panelHeader renders a panel heading with optional focused styling.
func panelHeader(label string, focused bool) string {
	if focused {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorPrimary)).
			Render(label)
	}
	return OverlayNormalStyle.Render(label)
}

// radio renders "(•) Label" or "( ) Label" depending on `selected`.
func radio(label string, selected bool) string {
	dot := " "
	if selected {
		dot = "\u2022"
	}
	return OverlayNormalStyle.Render("  (" + dot + ") " + label)
}

// labelUnderCell renders the small caption printed beneath a spinner
// cell. Focused cells get a slightly bolder caption so the visual
// grouping stays obvious even when the spinner grid is narrow.
func labelUnderCell(label string, focused bool) string {
	if focused {
		return OverlayNormalStyle.Bold(true).Render(" " + label + " ")
	}
	return OverlayDimStyle.Render(" " + label + " ")
}

// padInt renders v with zero-padding to width. Values exceeding the
// width are truncated to the last `width` digits, which should never
// happen within the editor's clamps but keeps rendering safe.
func padInt(v, width int) string {
	s := strings.Repeat("0", width)
	if v < 0 {
		v = 0
	}
	digits := ""
	if v == 0 {
		digits = "0"
	} else {
		for v > 0 {
			digits = string(rune('0'+v%10)) + digits
			v /= 10
		}
	}
	if len(digits) >= width {
		return digits[len(digits)-width:]
	}
	return s[:width-len(digits)] + digits
}
