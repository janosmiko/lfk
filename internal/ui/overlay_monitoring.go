package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// overlaySchemeScroll is the persistent scroll position for the colorscheme overlay.
var overlaySchemeScroll int

// ResetOverlaySchemeScroll resets the colorscheme overlay scroll to the top.
func ResetOverlaySchemeScroll() { overlaySchemeScroll = 0 }

// GetOverlaySchemeScroll returns the current colorscheme overlay scroll position.
func GetOverlaySchemeScroll() int { return overlaySchemeScroll }

// SchemeOverlayMaxVisible is the number of visible lines in the colorscheme overlay.
const SchemeOverlayMaxVisible = 20

// ErrorLogVisualParams holds visual selection state for the error log overlay.
type ErrorLogVisualParams struct {
	VisualMode     byte // 0 = off, 'v' = char, 'V' = line
	VisualStart    int  // anchor line index
	VisualStartCol int  // anchor column (for char mode)
	CursorLine     int  // current cursor line index
	CursorCol      int  // cursor column for char mode
}

// FilteredErrorLogEntries returns visible entries (respecting debug filter) in reverse chronological order.
func FilteredErrorLogEntries(entries []ErrorLogEntry, showDebug bool) []ErrorLogEntry {
	visible := make([]ErrorLogEntry, 0, len(entries))
	for _, e := range entries {
		if e.Level == "DBG" && !showDebug {
			continue
		}
		visible = append(visible, e)
	}
	reversed := make([]ErrorLogEntry, len(visible))
	for i, e := range visible {
		reversed[len(visible)-1-i] = e
	}
	return reversed
}

// ErrorLogEntryPlainText returns a plain text representation of a log entry for clipboard.
func ErrorLogEntryPlainText(e ErrorLogEntry) string {
	return fmt.Sprintf("%s [%s] %s", e.Time.Format("15:04:05"), e.Level, e.Message)
}

// errorLogLevelPalette returns the (foreground hex, bold) pair for a given
// log level. Unknown levels fall through to INF styling.
func errorLogLevelPalette(level string) (color, label string, bold bool) {
	switch level {
	case "ERR":
		return "#ff5555", "ERR", true
	case "WRN":
		return "#ffaa00", "WRN", true
	case "DBG":
		return "#6272a4", "DBG", false
	default:
		return "#888888", "INF", false
	}
}

// renderErrorLogLine formats a single error-log row: indicator + timestamp +
// styled level + message. When cursorBg is true, every segment inherits the
// cursor-line background so the level fg color stays visible while the row
// is highlighted. Non-cursor segments deliberately omit a background so the
// caller's FillLinesBg pass paints whichever bg fits the surrounding box
// (SurfaceBg in overlay mode, BaseBg in fullscreen).
func renderErrorLogLine(entry ErrorLogEntry, cursorBg bool) string {
	color, label, bold := errorLogLevelPalette(entry.Level)

	if cursorBg {
		base := lipgloss.NewStyle().Background(BaseBg).Bold(true)
		lvlStyle := lipgloss.NewStyle().Inherit(base).Foreground(ThemeColor(color))
		if bold {
			lvlStyle = lvlStyle.Bold(true)
		}
		ts := base.Render(entry.Time.Format("15:04:05"))
		lvl := lvlStyle.Render(label)
		msg := base.Render(entry.Message)
		sep := base.Render(" ")
		return base.Render(">") + sep + ts + sep + lvl + sep + msg
	}

	// Strip the surfaceBg the theme bakes into OverlayDimStyle and
	// OverlayNormalStyle so FillLinesBg can paint whichever bg fits
	// the surrounding box (SurfaceBg in the overlay form, BaseBg in
	// the fullscreen form). Otherwise the inner segments keep
	// SurfaceBg and clash with a BaseBg-filled fullscreen frame.
	ts := OverlayDimStyle.UnsetBackground().Render(entry.Time.Format("15:04:05"))
	lvlStyle := lipgloss.NewStyle().Foreground(ThemeColor(color))
	if bold {
		lvlStyle = lvlStyle.Bold(true)
	}
	lvl := lvlStyle.Render(label)
	return fmt.Sprintf("  %s %s %s", ts, lvl, OverlayNormalStyle.UnsetBackground().Render(entry.Message))
}

// RenderErrorLogOverlay renders the application log overlay showing timestamped
// log entries with level indicators. The scroll parameter controls which portion is visible.
// When showDebug is false, DBG entries are filtered out.
func RenderErrorLogOverlay(entries []ErrorLogEntry, scroll int, height int, showDebug bool, vp ErrorLogVisualParams) string {
	// Use bg-stripped variants of the overlay styles so the caller's
	// FillLinesBg pass paints whichever bg fits the surrounding box —
	// SurfaceBg for the bordered overlay form, BaseBg when this same
	// content is rendered as a fullscreen viewExplorer column.
	titleStyle := OverlayTitleStyle.UnsetBackground()
	dimStyle := OverlayDimStyle.UnsetBackground()

	var b strings.Builder
	b.WriteString(titleStyle.Render("Application Log"))
	b.WriteString("\n")

	reversed := FilteredErrorLogEntries(entries, showDebug)

	if len(reversed) == 0 {
		if len(entries) > 0 && !showDebug {
			b.WriteString(dimStyle.Render("No entries (debug logs hidden, press d to show)"))
		} else {
			b.WriteString(dimStyle.Render("No log entries"))
		}
		return b.String()
	}

	// Reserve lines for the title (1), blank line before footer (1), footer (1), and border padding.
	maxVisible := max(height-4, 1)

	// Clamp scroll.
	maxScroll := max(len(reversed)-maxVisible, 0)
	scroll = max(min(scroll, maxScroll), 0)

	end := min(scroll+maxVisible, len(reversed))

	// Visual selection range.
	selStart := min(vp.VisualStart, vp.CursorLine)
	selEnd := max(vp.VisualStart, vp.CursorLine)
	colStart := min(vp.VisualStartCol, vp.CursorCol)
	colEnd := max(vp.VisualStartCol, vp.CursorCol)

	for i := scroll; i < end; i++ {
		entry := reversed[i]
		plainText := ErrorLogEntryPlainText(entry)

		// Check if this line is in visual selection.
		inSelection := vp.VisualMode != 0 && i >= selStart && i <= selEnd
		isCursorLine := i == vp.CursorLine

		if inSelection {
			// Render with visual selection highlighting.
			rendered := RenderVisualSelection(
				plainText, rune(vp.VisualMode),
				i, selStart, selEnd,
				vp.VisualStart, vp.VisualStartCol, vp.CursorCol,
				colStart, colEnd,
			)
			b.WriteString("  " + rendered)
		} else if isCursorLine && vp.VisualMode == 0 {
			// Cursor line indicator (outside visual mode). Preserve the
			// level fg color by composing per-segment styles that inherit
			// the cursor-line bg, so red/orange ERR/WRN markers stay
			// visible when the user navigates through the overlay.
			b.WriteString(renderErrorLogLine(entry, true))
		} else {
			b.WriteString(renderErrorLogLine(entry, false))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")

	// Filter count for footer.
	visibleCount := len(reversed)
	scrollInfo := fmt.Sprintf("%d entries", visibleCount)
	if visibleCount != len(entries) {
		scrollInfo += fmt.Sprintf(" (%d hidden)", len(entries)-visibleCount)
	}
	if maxScroll > 0 {
		scrollInfo += fmt.Sprintf(" | scroll %d/%d", scroll+1, maxScroll+1)
	}
	if vp.VisualMode != 0 {
		modeLabel := "VISUAL LINE"
		if vp.VisualMode == 'v' {
			modeLabel = "VISUAL"
		}
		scrollInfo += " | " + modeLabel
	}
	b.WriteString(dimStyle.Render(scrollInfo))

	return b.String()
}

// RenderColorschemeOverlay renders the color scheme selector overlay content.
// entries is a list of SchemeEntry (with headers). cursor indexes only selectable entries.
func RenderColorschemeOverlay(entries []SchemeEntry, filter string, cursor int, filterMode bool) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Select Color Scheme"))
	b.WriteString("\n")

	// Filter input.
	switch {
	case filterMode:
		b.WriteString(OverlayFilterStyle.Render("/ " + filter + "█"))
	case filter != "":
		b.WriteString(OverlayFilterStyle.Render("/ " + filter))
	default:
		b.WriteString(OverlayDimStyle.Render("/ to filter"))
	}
	b.WriteString("\n\n")

	// Build display list: when filtering, skip headers and filter selectable entries.
	type displayItem struct {
		label     string
		isHeader  bool
		selectIdx int // index among selectable items (-1 for headers)
	}

	var items []displayItem
	selectIdx := 0
	if filter == "" {
		for _, e := range entries {
			if e.IsHeader {
				items = append(items, displayItem{label: e.Name, isHeader: true, selectIdx: -1})
			} else {
				items = append(items, displayItem{label: e.Name, isHeader: false, selectIdx: selectIdx})
				selectIdx++
			}
		}
	} else {
		lowerFilter := strings.ToLower(filter)
		for _, e := range entries {
			if e.IsHeader {
				continue
			}
			if strings.Contains(e.Name, lowerFilter) {
				items = append(items, displayItem{label: e.Name, isHeader: false, selectIdx: selectIdx})
				selectIdx++
			}
		}
	}

	selectableCount := selectIdx
	if selectableCount == 0 {
		b.WriteString(OverlayDimStyle.Render("No matching schemes"))
		return b.String()
	}

	// Scrolling window with vim-style scrolloff for stable viewport.
	maxVisible := SchemeOverlayMaxVisible
	scrollOff := ConfigScrollOff
	if len(items) <= maxVisible {
		scrollOff = 0
	} else if maxSO := (maxVisible - 1) / 2; scrollOff > maxSO {
		scrollOff = maxSO
	}

	// Find the display index of the cursor item.
	cursorDisplayIdx := 0
	for i, it := range items {
		if !it.isHeader && it.selectIdx == cursor {
			cursorDisplayIdx = i
			break
		}
	}

	displayLines := func(from, to int) int { return to - from }
	start := VimScrollOff(overlaySchemeScroll, cursorDisplayIdx, len(items), maxVisible, scrollOff, displayLines)
	overlaySchemeScroll = start

	end := min(start+maxVisible, len(items))

	b.WriteString(RenderScrollAbove(start, end-start, len(items), 0))
	b.WriteString("\n")

	var lines []string
	for i := start; i < end; i++ {
		it := items[i]
		if it.isHeader {
			lines = append(lines, "") // separator line
			lines = append(lines, CategoryStyle.Render("── "+it.label+" ──"))
		} else {
			prefix := "  "
			if it.label == ActiveSchemeName {
				prefix = "* "
			}
			line := prefix + it.label
			if it.selectIdx == cursor {
				lines = append(lines, OverlaySelectedStyle.Render(line))
			} else {
				lines = append(lines, OverlayNormalStyle.Render(line))
			}
		}
	}

	// Pad or truncate to fixed height so the overlay doesn't resize.
	for len(lines) < maxVisible {
		lines = append(lines, "")
	}
	if len(lines) > maxVisible {
		lines = lines[:maxVisible]
	}
	b.WriteString(strings.Join(lines, "\n"))

	b.WriteString("\n")
	b.WriteString(RenderScrollBelow(start, end-start, len(items), 0))

	return b.String()
}

// RelativeTime returns a human-readable relative time string (e.g., "2m ago", "1h ago", "3d ago").
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", max(int(d.Seconds()), 1))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

// formatRelativeTime returns a human-readable relative time string.
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m > 0 {
			return fmt.Sprintf("%dh%dm ago", h, m)
		}
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}
