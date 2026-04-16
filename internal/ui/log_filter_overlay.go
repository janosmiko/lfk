package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// LogFilterOverlayState carries the data needed to render the filter modal.
type LogFilterOverlayState struct {
	Title       string // e.g. "Pod: api-gateway-7f4"
	IncludeMode string // "any" | "all"
	Rules       []LogFilterRowState
	ListCursor  int
	FocusInput  bool
	Input       string
	StatusMsg   string // ephemeral feedback
	StatusIsErr bool

	// Save-preset prompt state.
	SavePromptActive bool
	SavePromptInput  string

	// Load-preset picker state.
	LoadPickerActive bool
	LoadPickerItems  []string // formatted "name (N rules) [default]"
	LoadPickerCursor int
}

// LogFilterRowState is one row in the active rules list. The three fields
// are broken out so the UI layer can render them in distinct table
// columns with consistent widths.
type LogFilterRowState struct {
	Kind    string // "SEV" | "INC" | "EXC"
	Mode    string // "substr" | "regex" | "fuzzy" | "" for severity
	Pattern string // raw pattern text, or severity floor for SEV rows
}

// presetItem adapts a preset label to the bubbles/list.Item contract.
type presetItem struct{ label string }

func (p presetItem) FilterValue() string { return p.label }
func (p presetItem) Title() string       { return p.label }
func (p presetItem) Description() string { return "" }

// newPresetDelegate returns a list delegate that renders a single-line
// preset picker entry in the overlay, using the app theme's colors for
// the selected / unselected states.
func newPresetDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetSpacing(0)
	d.Styles.SelectedTitle = OverlaySelectedStyle.
		Padding(0, 1).
		Border(lipgloss.Border{Left: "▌"}, false, false, false, true).
		BorderForeground(lipgloss.Color(ColorPrimary))
	d.Styles.SelectedDesc = d.Styles.SelectedTitle
	d.Styles.NormalTitle = OverlayNormalStyle.Padding(0, 0, 0, 2)
	d.Styles.NormalDesc = d.Styles.NormalTitle
	d.Styles.DimmedTitle = OverlayDimStyle.Padding(0, 0, 0, 2)
	d.Styles.DimmedDesc = d.Styles.DimmedTitle
	return d
}

// renderIncludeModeBadge renders a small "ANY" or "ALL" chip that
// shows how the include rules combine. Sits in the top-right corner
// of the overlay to signal global filter state at a glance.
func renderIncludeModeBadge(mode string) string {
	label := "ANY"
	color := ColorSecondary
	if mode == "all" {
		label = "ALL"
		color = ColorPrimary
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorSelectedFg)).
		Background(lipgloss.Color(color)).
		Padding(0, 1).
		Render(label)
}

// kindColor returns the theme color for a rule kind badge.
func kindColor(kind string) string {
	switch kind {
	case "SEV":
		return ColorPrimary // severity = primary (blue)
	case "INC":
		return ColorSecondary // include = green/success
	case "EXC":
		return ColorWarning // exclude = warn/orange
	}
	return ColorDimmed
}

// renderRuleTable renders the active-rules list as a manually-formatted
// table. We avoid bubbles/table here because its nested per-cell Render
// calls emit ANSI resets that break the Selected row's uniform
// background highlight. A single-string-per-row approach (same pattern
// as secretview.go / labelview.go) gives us full control.
// maxBodyRows is the target number of body rows the table renders
// (header + underline are in addition to this). The table's total line
// count is always exactly maxBodyRows + 2 regardless of whether it's
// scrolling — this keeps the layout perfectly stable as the cursor
// moves (no rows shifting up/down when the scroll chrome appears or
// disappears). When rows exceed maxBodyRows, the table scrolls around
// the cursor. Zero or negative means "no cap / fit all".
func renderRuleTable(rows []LogFilterRowState, cursor, width, maxBodyRows int, focused bool) string {
	// Column widths. First column is a 1-char cursor indicator (">" on
	// the selected row, blank otherwise), matching secretview/explainview.
	// Remaining columns share the space with a bounded pattern column.
	const (
		markerW = 2 // "> " or "  "
		idxW    = 4
		kindW   = 5 // "SEV" / "INC" / "EXC" + gutter
		modeW   = 8
		sep     = 2 // "  " between columns
	)
	patternW := width - markerW - idxW - kindW - modeW - sep*4
	if patternW < 10 {
		patternW = 10
	}

	// Selected-row styling is stable regardless of focus state: the
	// highlighted row always renders as bold + selected colors. The
	// focused/unfocused distinction lives elsewhere (input row, cursor
	// marker, hint bar) — keeping the selected row itself consistent
	// avoids a distracting flip between bold / faint on every
	// context-switch keystroke. `focused` is intentionally unused.
	_ = focused
	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorSelectedFg)).
		Background(lipgloss.Color(ColorSelectedBg))
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorPrimary)).
		Background(SurfaceBg)
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorFile)).
		Background(SurfaceBg)
	dimStyle := OverlayDimStyle

	var b strings.Builder

	// Header row (always rendered — even when there are no rules yet —
	// so the table layout is obvious to the user). Thin underline for
	// visual separation.
	header := fmt.Sprintf(
		"%-*s%-*s%-*s%-*s%-*s",
		markerW, "",
		idxW, "#",
		kindW, "Kind",
		modeW, "Mode",
		patternW, "Pattern",
	)
	// Always render to a fixed body-line budget so the overall table
	// height stays constant regardless of scrolling / chrome toggling.
	bodyBudget := maxBodyRows
	if bodyBudget <= 0 {
		bodyBudget = len(rows)
		if bodyBudget < 1 {
			bodyBudget = 1
		}
	}

	// Collect body lines into a slice so we can pad to a fixed height
	// at the end.
	var bodyLines []string
	emit := func(s string) { bodyLines = append(bodyLines, s) }

	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", width)))

	// Empty state: the table renders with a placeholder row under the
	// header so the user sees the layout and a hint. Pad to the body
	// budget so the table height matches the populated case.
	if len(rows) == 0 {
		emit(dimStyle.Render("  (no rules yet — press `a` to add)"))
		return finalizeTable(&b, bodyLines, bodyBudget)
	}

	// Severity is always the top rule when present. Rendered with a
	// star marker + "!" index + bold primary tint so it reads as a
	// pinned high-priority row.
	pinnedSeverity := len(rows) > 0 && rows[0].Kind == "SEV"
	renderSeverityRow := func(r LogFilterRowState, isCursor bool) string {
		marker := "★ "
		pat := Truncate(r.Pattern, patternW)
		line := fmt.Sprintf(
			"%-*s%-*s%-*s%-*s%-*s",
			markerW, marker,
			idxW, "!",
			kindW, r.Kind,
			modeW, r.Mode,
			patternW, pat,
		)
		if isCursor {
			return selectedStyle.Bold(true).Render(line)
		}
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorPrimary)).
			Background(SurfaceBg).
			Render(line)
	}

	// When severity is pinned, non-severity rows are numbered starting
	// at 1 (severity itself is labelled "!" and doesn't consume an
	// index). Otherwise row i+1 matches the cursor index.
	displayIdx := func(i int) int {
		if pinnedSeverity {
			return i // since i >= 1 when pinnedSeverity, and we want 1-based numbering starting after the pin
		}
		return i + 1
	}

	renderNormalRow := func(i int, r LogFilterRowState) string {
		marker := "  "
		if i == cursor {
			marker = "> "
		}
		patternText := Truncate(r.Pattern, patternW)
		idx := displayIdx(i)
		if i == cursor {
			line := fmt.Sprintf(
				"%-*s%-*d%-*s%-*s%-*s",
				markerW, marker,
				idxW, idx,
				kindW, r.Kind,
				modeW, r.Mode,
				patternW, patternText,
			)
			return selectedStyle.Render(line)
		}
		markerPart := normalStyle.Render(fmt.Sprintf("%-*s", markerW, marker))
		idxPart := normalStyle.Render(fmt.Sprintf("%-*d", idxW, idx))
		kindPart := lipgloss.NewStyle().
			Foreground(lipgloss.Color(kindColor(r.Kind))).
			Background(SurfaceBg).
			Bold(true).
			Render(fmt.Sprintf("%-*s", kindW, r.Kind))
		restPart := normalStyle.Render(fmt.Sprintf("%-*s%-*s", modeW, r.Mode, patternW, patternText))
		return markerPart + idxPart + kindPart + restPart
	}

	// Severity pinned layout: the severity row ALWAYS sits on the first
	// line of the body area (standalone). Directly below it comes a
	// single "context" line which carries either the scroll-up chrome
	// ("↑ N more above") when the body is scrolled down past the top,
	// or stays blank — this one line doubles as the visual spacer
	// between severity and the first regular rule. It's always present
	// so the layout is consistent whether we're scrolling or not.
	if pinnedSeverity {
		emit(renderSeverityRow(rows[0], cursor == 0))
	}

	// Scroll window is computed over the scrollable (non-severity) part
	// of the rules list.
	scrollableOffset := 0
	if pinnedSeverity {
		scrollableOffset = 1
	}
	scrollableCount := len(rows) - scrollableOffset
	scrollableCursor := cursor - scrollableOffset
	if scrollableCursor < 0 {
		scrollableCursor = 0
	}

	// Compute the body window directly with the known line budget so the
	// total overlay height is exactly bodyBudget in every case.
	//
	// Layout accounting:
	//   pinnedSeverity, scrolling   → 1 severity + 1 combined-up + W body + 1 down = bodyBudget
	//   pinnedSeverity, not scroll  → 1 severity + 1 spacer      + N body (+pad)
	//   no severity,   scrolling   → 1 up  + W body + 1 down = bodyBudget
	//   no severity,   not scroll  → N body (+pad)
	reserved := 0
	if pinnedSeverity {
		reserved++ // severity line itself
		reserved++ // combined spacer / "↑ more above" line
	}
	// First decide whether we need to scroll: do all remaining rows fit
	// with just a spacer (no bottom chrome)?
	scrolling := scrollableCount > (bodyBudget - reserved)
	if scrolling {
		if !pinnedSeverity {
			reserved++ // "↑ more above" line
		}
		reserved++ // "↓ more below" line
	}
	// Ensure at least one row slot.
	windowSize := bodyBudget - reserved
	if windowSize < 1 {
		windowSize = 1
	}
	windowStart, windowEnd := slidingWindow(scrollableCount, scrollableCursor, windowSize, scrolling)

	// Combined spacer / "↑ more above" line.
	//
	// - When severity is pinned: always emit one line below it so there
	//   is a visual gap between the pinned row and the first regular
	//   rule. When scrolling and not at top, that line carries the
	//   "↑ N more above" chrome; otherwise it stays blank.
	// - When severity is NOT pinned: emit the chrome only when there
	//   is actually something above the visible window. We accept a
	//   single-line layout shift on that one transition (off→on) in
	//   exchange for a flush top-edge the rest of the time.
	switch {
	case scrolling && windowStart > 0:
		emit(dimStyle.Render(fmt.Sprintf("  ↑ %d more above", windowStart)))
	case pinnedSeverity:
		emit("")
	}

	// Body rows — iterate over the scrollable window and map back to
	// the original rules index via scrollableOffset.
	for si := windowStart; si < windowEnd && si < scrollableCount; si++ {
		i := si + scrollableOffset
		emit(renderNormalRow(i, rows[i]))
	}

	// "↓ more below" chrome.
	if scrolling {
		if windowEnd < scrollableCount {
			emit(dimStyle.Render(fmt.Sprintf("  ↓ %d more below", scrollableCount-windowEnd)))
		} else {
			emit("")
		}
	}

	return finalizeTable(&b, bodyLines, bodyBudget)
}

// slidingWindow computes [start, end) over n elements such that cursor
// stays visible with a scrolloff margin when scrolling is active.
// When scrolling is false, the full range [0, n) is returned.
func slidingWindow(n, cursor, windowSize int, scrolling bool) (int, int) {
	if !scrolling || windowSize >= n {
		return 0, n
	}
	scrollOff := 1
	if windowSize < 4 {
		scrollOff = 0
	}
	start := cursor - windowSize/2
	if start < 0 {
		start = 0
	}
	if start > n-windowSize {
		start = n - windowSize
	}
	if cursor < start+scrollOff {
		start = cursor - scrollOff
		if start < 0 {
			start = 0
		}
	}
	if cursor >= start+windowSize-scrollOff {
		start = cursor - windowSize + scrollOff + 1
		if start > n-windowSize {
			start = n - windowSize
		}
	}
	return start, start + windowSize
}

// finalizeTable writes the body lines joined by newlines and pads with
// blank lines up to the bodyBudget so the table always occupies the
// same vertical space — the main render relies on this for stable
// outer layout.
func finalizeTable(b *strings.Builder, bodyLines []string, bodyBudget int) string {
	if len(bodyLines) < bodyBudget {
		pad := bodyBudget - len(bodyLines)
		for range pad {
			bodyLines = append(bodyLines, "")
		}
	} else if len(bodyLines) > bodyBudget {
		bodyLines = bodyLines[:bodyBudget]
	}
	for _, line := range bodyLines {
		b.WriteString("\n")
		b.WriteString(line)
	}
	return b.String()
}

// buildFilterInput builds a textinput.Model pre-populated with the current
// input value. The textinput only renders; the underlying key handling
// keeps using the model's lightweight TextInput. This keeps the existing
// key dispatch (handleFilterInputKey, handleSavePresetPromptKey) unchanged
// — we only borrow bubbles/textinput for consistent rendering (prompt,
// cursor, underline, etc.).
func buildFilterInput(value string, focused bool, width int, prompt string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = prompt
	ti.CharLimit = 256
	if width > 0 {
		ti.Width = width
	}
	ti.SetValue(value)
	ti.SetCursor(len(value))
	ti.PromptStyle = OverlayDimStyle
	ti.TextStyle = OverlayInputStyle
	ti.PlaceholderStyle = OverlayDimStyle
	if focused {
		ti.Focus()
	} else {
		ti.Blur()
		// When blurred, render the text dimly so the user knows the
		// table has focus. TextStyle applies to unfocused text too.
		ti.TextStyle = OverlayDimStyle
	}
	return ti
}

// buildPresetList constructs a bubbles/list.Model for the load-preset
// picker. Height is capped to fit inside the overlay.
func buildPresetList(items []string, cursor, width, height int) list.Model {
	lis := make([]list.Item, 0, len(items))
	for _, it := range items {
		lis = append(lis, presetItem{label: it})
	}
	l := list.New(lis, newPresetDelegate(), width, height)
	l.Title = "Presets"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetShowFilter(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = OverlayTitleStyle
	if cursor >= 0 && cursor < len(items) {
		l.Select(cursor)
	}
	return l
}

// RenderLogFilterOverlay renders the filter modal as a single string
// suitable for absolute-positioned overlay rendering by the caller.
// width is the total overlay box width (including border + padding from
// OverlayStyle, which is border 1 + padding 2 = 3 on each side).
// height is the total overlay box height; the renderer uses it to pin
// the input bar to the bottom of the content area, padding the middle
// with blank lines.
func RenderLogFilterOverlay(s LogFilterOverlayState, width, height int) string {
	// Account for overlay border (1 each side) + padding (1 each side
	// vertical) = 4 vertical, 6 horizontal.
	innerW := width - 6
	if innerW < 20 {
		innerW = 20
	}
	innerH := height - 4
	if innerH < 5 {
		innerH = 5
	}

	// Body section — everything that sits ABOVE the input bar.
	var body []string

	// Header row: "Filters — <title>" on the left, ANY/ALL mode badge on the right.
	title := OverlayTitleStyle.Render("Filters — " + s.Title)
	modeBadge := renderIncludeModeBadge(s.IncludeMode)
	gap := innerW - lipgloss.Width(title) - lipgloss.Width(modeBadge)
	if gap < 1 {
		gap = 1
	}
	body = append(body, title+strings.Repeat(" ", gap)+modeBadge)

	// Thin separator below the header.
	body = append(body, OverlayDimStyle.Render(strings.Repeat("─", innerW)))

	// Available height for the rules-table body: total inner height minus
	// the fixed chrome (header line, separator line, table header + its
	// underline = 2, blank pad above input ≥1, input line = 1, optional
	// picker / save prompt / status message when active). Floor at 3 so
	// the table always shows at least a few rows.
	reserved := 2 /* header + separator */ +
		2 /* table header + underline */ +
		1 /* pad */ +
		1 /* input */
	if s.SavePromptActive {
		reserved++
	}
	if s.StatusMsg != "" {
		reserved++
	}
	if s.LoadPickerActive {
		// Picker is preceded by a blank line and itself takes up to 10.
		pickerH := len(s.LoadPickerItems) + 2
		if pickerH < 4 {
			pickerH = 4
		}
		if pickerH > 10 {
			pickerH = 10
		}
		reserved += 1 + pickerH
	}
	tableBody := innerH - reserved
	if tableBody < 3 {
		tableBody = 3
	}

	// Active rules table (headers always rendered; empty-state
	// placeholder sits under the header row inside the table itself).
	body = append(body, renderRuleTable(s.Rules, s.ListCursor, innerW, tableBody, !s.FocusInput))

	// Load-preset picker is part of the body when active.
	if s.LoadPickerActive {
		pickerHeight := len(s.LoadPickerItems) + 2
		if pickerHeight < 4 {
			pickerHeight = 4
		}
		if pickerHeight > 10 {
			pickerHeight = 10
		}
		body = append(body, "")
		if len(s.LoadPickerItems) == 0 {
			body = append(body, OverlayTitleStyle.Render("Presets"))
			body = append(body, OverlayDimStyle.Render("  (none saved yet)"))
		} else {
			l := buildPresetList(s.LoadPickerItems, s.LoadPickerCursor, innerW, pickerHeight)
			body = append(body, l.View())
		}
	}

	// Status / feedback message — body element, just above the footer.
	if s.StatusMsg != "" {
		style := OverlayNormalStyle
		if s.StatusIsErr {
			style = OverlayWarningStyle
		}
		body = append(body, style.Render(" "+s.StatusMsg))
	}

	// Footer section — input row(s) pinned to the bottom.
	var footer []string
	inputPrompt := " > "
	ti := buildFilterInput(s.Input, s.FocusInput && !s.SavePromptActive, innerW-len(inputPrompt)-1, inputPrompt)
	footer = append(footer, ti.View())
	if s.SavePromptActive {
		saveTI := buildFilterInput(s.SavePromptInput, true, innerW-len(" Save preset as: ")-1, " Save preset as: ")
		footer = append(footer, saveTI.View())
	}

	// Combine: body at top, input at bottom, blank lines in between so
	// the input sits exactly on the last content row. Count rendered
	// line heights (accounting for ANSI escapes) via lipgloss.Height.
	bodyStr := strings.Join(body, "\n")
	footerStr := strings.Join(footer, "\n")
	bodyH := lipgloss.Height(bodyStr)
	footerH := lipgloss.Height(footerStr)
	padH := innerH - bodyH - footerH
	if padH < 1 {
		padH = 1 // at minimum, one blank line separates body from input
	}
	return bodyStr + "\n" + strings.Repeat("\n", padH) + footerStr
}
