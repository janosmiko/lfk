package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// overlaySchemeScroll is the persistent scroll position for the colorscheme overlay.
var overlaySchemeScroll int

// ResetOverlaySchemeScroll resets the colorscheme overlay scroll to the top.
func ResetOverlaySchemeScroll() { overlaySchemeScroll = 0 }

// GetOverlaySchemeScroll returns the current colorscheme overlay scroll position.
func GetOverlaySchemeScroll() int { return overlaySchemeScroll }

// SetOverlaySchemeScroll updates the colorscheme-overlay scroll state. Called
// by the renderColorschemeOverlay helper on every render so mouse-click row
// resolution in update_overlays_selectors.go keeps resolving rows correctly.
func SetOverlaySchemeScroll(s int) { overlaySchemeScroll = s }

// overlaySchemeVisible mirrors the viewport size last used by the
// colorscheme overlay renderer so the mouse-click / page-scroll
// handlers can resolve hit rows against the correct viewport. The
// renderer writes this on every render via SetOverlaySchemeVisible;
// the default 20 matches the legacy constant for the brief window
// before the first render.
var overlaySchemeVisible = 20

// GetOverlaySchemeVisible returns the current colorscheme overlay
// viewport size (number of display lines, including any interleaved
// section headers).
func GetOverlaySchemeVisible() int { return overlaySchemeVisible }

// SetOverlaySchemeVisible updates the viewport size to match what the
// renderer is actually painting. Called from renderColorschemeOverlay
// on every render.
func SetOverlaySchemeVisible(n int) { overlaySchemeVisible = n }

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

// ErrorLogEntryPlainText returns a plain text representation of a log entry,
// used for clipboard yank, char-selection column math, and visual-mode
// rendering. The layout (no brackets around the level) matches the styled
// non-visual row so selecting a log does not shift the text or wrap the
// severity in brackets — what is shown selected is what gets copied.
func ErrorLogEntryPlainText(e ErrorLogEntry) string {
	return fmt.Sprintf("%s %s %s", e.Time.Format("15:04:05"), e.Level, e.Message)
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

// errorLogGutterWidth is the width of the leading line marker prepended to
// every row: a "▎" bar on the cursor line, a space otherwise.
const errorLogGutterWidth = 1

// errorLogMsgColumn is the column where the message starts WITHIN a row's
// content (after the gutter): "15:04:05" (8) + " " + "LVL" (3) + " " = 13.
// Continuation lines produced by wrapping are indented this far so wrapped
// text stays under the message column. The on-screen message column is
// errorLogGutterWidth + errorLogMsgColumn (the gutter sits to its left).
const errorLogMsgColumn = 13

// errorLogGutter returns the 1-column line marker prepended to every physical
// sub-line: a primary-colored "▎" on the cursor line (matching the event
// viewer's YamlCursorIndicatorStyle gutter), a plain space otherwise. The
// marker is independent of selection state, so it stays visible in visual mode.
func errorLogGutter(isCursor bool) string {
	if isCursor {
		return YamlCursorIndicatorStyle.Render("▎")
	}
	return " "
}

// errorLogIndent returns an indent string of col spaces, clamped so it never
// consumes the whole content width (leaving at least one column for text at
// pathologically narrow widths). Mirrors wrappedEventChunks' indent clamp.
func errorLogIndent(col, contentW int) string {
	if col > contentW-1 {
		col = max(contentW-1, 0)
	}
	return strings.Repeat(" ", col)
}

// renderErrorLogContent builds the colored content sub-lines for an entry —
// timestamp + styled level + wrapped message — WITHOUT the leading gutter or
// any cursor decoration. The caller prepends the gutter (and a block cursor on
// the cursor line). contentW is the width available after the gutter;
// continuation lines are indented to errorLogMsgColumn so wrapped text stays
// under the message column. Reuses WrapEventLine (the "other overlay" wrap
// primitive) with a zero hanging indent.
//
// The inner segment styles drop their baked-in background so the caller's
// FillLinesBg pass paints whichever bg fits the surrounding box (SurfaceBg in
// the overlay form, BaseBg in fullscreen).
func renderErrorLogContent(entry ErrorLogEntry, contentW int) []string {
	color, label, bold := errorLogLevelPalette(entry.Level)
	chunks := WrapEventLine(entry.Message, max(contentW-errorLogMsgColumn, 1), 0)
	indent := errorLogIndent(errorLogMsgColumn, contentW)

	ts := OverlayDimStyle.UnsetBackground().Render(entry.Time.Format("15:04:05"))
	lvlStyle := lipgloss.NewStyle().Foreground(ThemeColor(color))
	if bold {
		lvlStyle = lvlStyle.Bold(true)
	}
	lvl := lvlStyle.Render(label)
	msgStyle := OverlayNormalStyle.UnsetBackground()

	lines := make([]string, 0, len(chunks))
	lines = append(lines, fmt.Sprintf("%s %s %s", ts, lvl, msgStyle.Render(chunks[0])))
	for _, c := range chunks[1:] {
		lines = append(lines, indent+msgStyle.Render(c))
	}
	return lines
}

// errorLogVisualWrap splits the plain text used for visual-mode rendering into
// the first physical sub-line and message continuation chunks aligned under the
// message column. contentW is the width available after the gutter. Returns
// (plainText, nil) when the entry fits on a single line, so short entries
// render identically to the non-wrap behaviour.
func errorLogVisualWrap(plainText string, contentW int) (first string, cont []string) {
	firstWidth := max(contentW, 1)
	runes := []rune(plainText)
	if len(runes) <= firstWidth {
		return plainText, nil
	}
	first = string(runes[:firstWidth])
	contWidth := max(contentW-errorLogMsgColumn, 1)
	for start := firstWidth; start < len(runes); start += contWidth {
		end := min(start+contWidth, len(runes))
		cont = append(cont, string(runes[start:end]))
	}
	return first, cont
}

// renderErrorLogSelectionContent builds the selection-highlighted content
// sub-lines for an entry (no gutter). Line ('V') mode highlights every
// sub-line. Char ('v') mode applies the column-range highlight to the first
// sub-line only and renders continuation sub-lines as plain wrapped text —
// matching the event viewer convention (RenderWrappedEventRow), where char
// selection does not span wrapped lines.
func renderErrorLogSelectionContent(plainText string, contentW int, vp ErrorLogVisualParams, lineIdx, selStart, selEnd, colStart, colEnd int) []string {
	first, cont := errorLogVisualWrap(plainText, contentW)
	indent := errorLogIndent(errorLogMsgColumn, contentW)

	lines := make([]string, 0, 1+len(cont))
	lines = append(lines, RenderVisualSelection(
		first, rune(vp.VisualMode),
		lineIdx, selStart, selEnd,
		vp.VisualStart, vp.VisualStartCol, vp.CursorCol,
		colStart, colEnd,
	))
	for _, c := range cont {
		if vp.VisualMode == 'V' {
			lines = append(lines, indent+SelectedStyle.Render(c))
		} else {
			lines = append(lines, indent+OverlayNormalStyle.UnsetBackground().Render(c))
		}
	}
	return lines
}

// errorLogEntryLines returns the number of physical sub-lines an entry renders
// to, where contentW is the width available after the gutter. Used to
// reconcile the key handler's logical-entry scroll with the renderer's
// physical-line pagination.
func errorLogEntryLines(entry ErrorLogEntry, contentW int, inSelection bool) int {
	if inSelection {
		_, cont := errorLogVisualWrap(ErrorLogEntryPlainText(entry), contentW)
		return 1 + len(cont)
	}
	return len(WrapEventLine(entry.Message, max(contentW-errorLogMsgColumn, 1), 0))
}

// errorLogStartForCursor returns the scroll (start entry index) that keeps the
// cursor entry's first sub-line within maxVisible physical lines. The incoming
// scroll is honoured when it already shows the cursor. Otherwise it is clamped
// into the visible range. This reconciles the key handler's logical-entry
// scroll model with the renderer's physical-line pagination, so the cursor is
// never drawn off-screen whether or not entries wrap.
func errorLogStartForCursor(reversed []ErrorLogEntry, scroll, cursor, maxVisible, contentW int, vp ErrorLogVisualParams, selStart, selEnd int) int {
	inSel := func(i int) bool { return vp.VisualMode != 0 && i >= selStart && i <= selEnd }
	// Walk back from the cursor accumulating physical lines until the next
	// entry above would no longer fit. minStart is the earliest start that
	// still keeps the cursor's first sub-line on screen.
	minStart := cursor
	used := 1 // the cursor entry's first sub-line
	for s := cursor - 1; s >= 0; s-- {
		used += errorLogEntryLines(reversed[s], contentW, inSel(s))
		if used > maxVisible {
			break
		}
		minStart = s
	}
	return max(min(scroll, cursor), minStart)
}

// RenderErrorLogOverlay renders the application log overlay showing timestamped
// log entries with level indicators. Long messages are wrapped to width so they
// stay readable instead of being truncated. The scroll parameter (in logical
// entry units, matching the key handler) is honoured when it keeps the cursor
// visible. Pagination is by physical line and the start index is adjusted so a
// wrapped entry never pushes the cursor (or the footer) off the viewport. When
// showDebug is false, DBG entries are filtered out.
func RenderErrorLogOverlay(entries []ErrorLogEntry, scroll, width, height int, showDebug bool, vp ErrorLogVisualParams) string {
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
	// Content width is the line width minus the 1-column gutter (line marker).
	contentW := max(width-errorLogGutterWidth, 1)

	// Visual selection range.
	selStart := min(vp.VisualStart, vp.CursorLine)
	selEnd := max(vp.VisualStart, vp.CursorLine)
	colStart := min(vp.VisualStartCol, vp.CursorCol)
	colEnd := max(vp.VisualStartCol, vp.CursorCol)

	// Resolve the start index so the cursor entry is always visible. The key
	// handler tracks scroll in logical-entry units and is wrap-unaware. Here
	// we paginate by physical line, so a literal entry-based scroll could
	// leave the cursor (and the footer) off-screen when entries wrap.
	cursor := max(min(vp.CursorLine, len(reversed)-1), 0)
	scroll = errorLogStartForCursor(reversed, max(scroll, 0), cursor, maxVisible, contentW, vp, selStart, selEnd)

	// Paginate by physical line: a wrapped entry expands to several sub-lines,
	// so counting logical entries would emit more rows than maxVisible and
	// push the footer (and the overlay border) out of view.
	physical := make([]string, 0, maxVisible)
	end := scroll
	for i := scroll; i < len(reversed) && len(physical) < maxVisible; i++ {
		for sub := range strings.SplitSeq(renderErrorLogEntry(reversed[i], contentW, vp, i, selStart, selEnd, colStart, colEnd), "\n") {
			if len(physical) >= maxVisible {
				break
			}
			physical = append(physical, sub)
		}
		end = i + 1
	}
	b.WriteString(strings.Join(physical, "\n"))

	b.WriteString("\n\n")

	// Filter count + cursor position for footer.
	visibleCount := len(reversed)
	scrollInfo := fmt.Sprintf("%d entries", visibleCount)
	if visibleCount != len(entries) {
		scrollInfo += fmt.Sprintf(" (%d hidden)", len(entries)-visibleCount)
	}
	if scroll > 0 || end < visibleCount {
		scrollInfo += fmt.Sprintf(" | line %d/%d", cursor+1, visibleCount)
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

// renderErrorLogEntry renders one entry into its (possibly multi-line) block.
// Every physical sub-line is prefixed with the gutter (line marker) so the
// cursor line is flagged consistently in all modes — matching the event
// viewer. The cursor line also carries a reverse-video block cursor on its
// first sub-line when not selecting. During selection the highlight stands in
// for it. contentW is the width available after the gutter.
func renderErrorLogEntry(entry ErrorLogEntry, contentW int, vp ErrorLogVisualParams, i, selStart, selEnd, colStart, colEnd int) string {
	isCursor := i == vp.CursorLine
	inSelection := vp.VisualMode != 0 && i >= selStart && i <= selEnd

	var lines []string
	if inSelection {
		lines = renderErrorLogSelectionContent(ErrorLogEntryPlainText(entry), contentW, vp, i, selStart, selEnd, colStart, colEnd)
	} else {
		lines = renderErrorLogContent(entry, contentW)
		if isCursor && len(lines) > 0 {
			// Clamp the block cursor to the last real character so a column
			// left over from a vertical move onto a shorter line lands on the
			// text rather than parking in the padding past end-of-line.
			col := min(vp.CursorCol, max(ansi.StringWidth(lines[0])-1, 0))
			lines[0] = RenderCursorAtCol(lines[0], "", col)
		}
	}

	gutter := errorLogGutter(isCursor)
	var b strings.Builder
	for k, ln := range lines {
		if k > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(gutter + ln)
	}
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
