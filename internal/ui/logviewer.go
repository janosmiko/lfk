package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// LogSearchHighlightStyle highlights matched substrings in log lines.
var LogSearchHighlightStyle = lipgloss.NewStyle().
	Background(lipgloss.Color(ColorWarning)).
	Foreground(lipgloss.Color(ColorBase)).
	Bold(true)

// RenderLogViewer renders the full-screen log viewer.
// ruleCount is the number of active filter rules; when greater than zero a
// `[FILTER: N]` chip is shown in the title bar.
// logSince is the currently applied --since window (e.g. "5m"); when
// non-empty a `[SINCE: ...]` chip is appended to the title bar.
// relativeTimestamps, when true together with timestamps, rewrites the
// leading RFC3339 timestamp on each visible line to a relative form
// ("5m ago"); a subtle `[REL]` chip is shown alongside `[TIMESTAMPS]`.
// jsonPretty, when true, renders JSON log lines in expanded multi-line
// form. The caller is responsible for producing the pretty-printed
// representation and passing it as prettyLines (a parallel slice to
// the post-projection lines; each element either equals the original
// line or contains embedded newlines for JSON lines). When
// prettyLines is nil, rendering falls back to the raw buffer even if
// jsonPretty is true (useful for tests that exercise just the chip).
// mergeJSONPrettyLines substitutes the expanded pretty-printed form
// into `lines` for each index where `prettyLines[i] != ""`. When pretty
// mode is off or the slice lengths don't match, returns the original
// `lines` unchanged. Kept as a helper so RenderLogViewer's cyclomatic
// complexity stays under the lint ceiling.
func mergeJSONPrettyLines(lines, prettyLines []string, jsonPretty bool) []string {
	if !jsonPretty || len(prettyLines) != len(lines) || len(lines) == 0 {
		return lines
	}
	merged := make([]string, len(lines))
	for i, line := range lines {
		if p := prettyLines[i]; p != "" {
			merged[i] = p
		} else {
			merged[i] = line
		}
	}
	return merged
}

func RenderLogViewer(lines []string, visibleIndices []int, scroll, width, height int, follow, wrap, lineNumbers, timestamps, previous, hidePrefixes bool, title, searchQuery, searchInput string, searchActive, canSwitchPod, canFilterContainers, hasMoreHistory, loadingHistory bool, statusMsg string, statusIsErr bool, cursor int, visualMode bool, visualStart int, visualType rune, visualCol, visualCurCol, ruleCount int, severityFloor, logSince string, relativeTimestamps, jsonPretty bool, prettyLines []string) string {
	// Record the total number of lines before any filter projection so the
	// title bar can render "[X of Y lines]" when filtering is active.
	totalLines := lines
	// If visibleIndices is non-nil, project lines through it so the renderer
	// shows only the visible subset.
	if visibleIndices != nil {
		projected := make([]string, len(visibleIndices))
		for i, idx := range visibleIndices {
			if idx >= 0 && idx < len(lines) {
				projected[i] = lines[idx]
			}
		}
		lines = projected
	}
	// When filtering is active, pass the visible count; otherwise pass the
	// full count so the "[X of Y]" branch is skipped.
	visibleCount := len(totalLines)
	if visibleIndices != nil {
		visibleCount = len(visibleIndices)
	}
	titleBar := renderLogTitleBar(title, totalLines, visibleCount, width, follow, wrap, lineNumbers, timestamps, previous, hidePrefixes, visualMode, visualType, loadingHistory, searchQuery, ruleCount, severityFloor, logSince, relativeTimestamps, jsonPretty)
	footer := renderLogFooter(width, statusMsg, statusIsErr, searchActive, searchInput, visualMode, canSwitchPod, canFilterContainers)

	// Content area: subtract border top + bottom (2 lines).
	contentHeight := height - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Content width accounting for border (2) + padding (2).
	contentWidth := width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	// Calculate line number width if needed.
	lineNumWidth := 0
	if lineNumbers && len(lines) > 0 {
		lineNumWidth = len(fmt.Sprintf("%d", len(lines))) + 1 // +1 for separator space
	}

	// Clamp scroll.
	if scroll < 0 {
		scroll = 0
	}

	lines = mergeJSONPrettyLines(lines, prettyLines, jsonPretty)

	// Strip timestamps and/or pod prefixes from visible lines, or rewrite
	// RFC3339 timestamps to their relative form when the caller asked for
	// the "5m ago" view. The rewrite only runs when the absolute form is
	// on (timestamps=true) — stripping wins otherwise, and it would be
	// surprising to see relative timestamps appear after the user pressed
	// `s` to hide them.
	rewriteRelative := timestamps && relativeTimestamps
	displayLines := lines
	if (!timestamps || hidePrefixes || rewriteRelative) && len(lines) > 0 {
		end := scroll + contentHeight
		if end > len(lines) {
			end = len(lines)
		}
		displayLines = make([]string, len(lines))
		copy(displayLines, lines)
		// Snapshot `now` once so every rewritten line in this frame uses
		// the same reference point. Without this, the top of the window
		// might say "5m ago" while the bottom says "5m 1s ago" for lines
		// one scroll tick apart.
		now := nowFunc()
		for i := scroll; i < end; i++ {
			if !timestamps {
				displayLines[i] = StripTimestamp(lines[i])
			} else if rewriteRelative {
				displayLines[i] = RewriteLogLineTimestamp(lines[i], now)
			}
			if hidePrefixes {
				displayLines[i] = StripPodPrefix(displayLines[i])
			}
		}
	}

	// Sanitize visible lines: replace non-printable characters (except common
	// whitespace) that can break terminal width calculations and corrupt the layout.
	{
		end := scroll + contentHeight
		if end > len(displayLines) {
			end = len(displayLines)
		}
		for i := scroll; i < end; i++ {
			displayLines[i] = sanitizeLogLine(displayLines[i])
		}
	}

	// Compute visual selection range.
	selStart, selEnd := -1, -1
	if visualMode {
		selStart = min(visualStart, cursor)
		selEnd = max(visualStart, cursor)
	}

	// Build visible lines, handling wrapping. JSON pretty-print forces
	// the wrap-style renderer because pretty lines contain embedded
	// newlines: the wrap renderer is the only path that handles one
	// source line producing multiple visual rows.
	var rendered []string
	if wrap || jsonPretty {
		rendered = renderWrappedLines(displayLines, scroll, contentHeight, contentWidth, lineNumbers, lineNumWidth, cursor, selStart, selEnd, visualStart, visualType, visualCol, visualCurCol)
	} else {
		rendered = renderPlainLines(displayLines, scroll, contentHeight, contentWidth, lineNumbers, lineNumWidth, cursor, selStart, selEnd, visualStart, visualType, visualCol, visualCurCol)
	}

	// Highlight search matches in rendered lines.
	if searchQuery != "" {
		rendered = highlightSearchMatches(rendered, searchQuery)
	}

	// Pad to fill content height.
	for len(rendered) < contentHeight {
		rendered = append(rendered, "")
	}

	// Ensure no rendered line exceeds contentWidth visual cells. Wrapped lines
	// or FillLinesBg padding can occasionally produce lines wider than the border
	// container expects, causing lipgloss to re-wrap them internally and push the
	// bottom border out of view.
	for i, line := range rendered {
		if lipgloss.Width(line) > contentWidth {
			rendered[i] = ansi.Truncate(line, contentWidth, "")
		}
	}

	bodyContent := strings.Join(rendered, "\n")
	// Fill each line's background so ANSI resets from styled segments
	// (line numbers, search highlights) don't leave gaps in the theme bg.
	bodyContent = FillLinesBg(bodyContent, contentWidth, BaseBg)
	borderStyle := FullscreenBorderStyle(width, contentHeight)
	body := borderStyle.Render(bodyContent)

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body, footer)
}

// renderLogTitleBar builds the title bar with status indicators for the log viewer.
// visibleCount indicates how many of the `lines` are currently visible after
// filtering. When it is less than len(lines), the title renders `[X of Y lines]`
// instead of `[Y lines]`. ruleCount is the number of active filter rules; when
// greater than zero a `[FILTER: N]` chip is appended to the indicators.
// logSince is the currently applied --since window; when non-empty a
// `[SINCE: ...]` chip is shown so the user can tell at a glance that the
// view is time-bounded. relativeTimestamps only surfaces (as a subtle
// `[REL]` chip) when timestamps is also on — mirroring the runtime
// precedence in the per-line rendering loop.
func renderLogTitleBar(title string, lines []string, visibleCount, width int, follow, wrap, lineNumbers, timestamps, previous, hidePrefixes, visualMode bool, visualType rune, loadingHistory bool, searchQuery string, ruleCount int, severityFloor, logSince string, relativeTimestamps, jsonPretty bool) string {
	type indicatorFlag struct {
		enabled bool
		label   string
	}
	flags := []indicatorFlag{
		{follow, "FOLLOW"},
		{wrap, "WRAP"},
		{lineNumbers, "LINE#"},
		{timestamps, "TIMESTAMPS"},
		{timestamps && relativeTimestamps, "REL"},
		{hidePrefixes, "NO PREFIX"},
		{previous, "PREVIOUS"},
		{jsonPretty, "JSON"},
		{loadingHistory, "LOADING HISTORY..."},
	}
	var indicators []string
	for _, f := range flags {
		if f.enabled {
			indicators = append(indicators, HelpKeyStyle.Render("["+f.label+"]"))
		}
	}
	if visualMode {
		switch visualType {
		case 'v':
			indicators = append(indicators, HelpKeyStyle.Render("[VISUAL]"))
		case 'B':
			indicators = append(indicators, HelpKeyStyle.Render("[VISUAL BLOCK]"))
		default:
			indicators = append(indicators, HelpKeyStyle.Render("[VISUAL LINE]"))
		}
	}
	if searchQuery != "" {
		indicators = append(indicators, HelpKeyStyle.Render("[/"+searchQuery+"]"))
	}
	// Severity floor is surfaced as its own chip so the user can tell at a
	// glance whether severity filtering is narrowing the view, independent
	// of the total rule count.
	if severityFloor != "" {
		indicators = append(indicators, HelpKeyStyle.Render("[≥ "+severityFloor+"]"))
	}
	if ruleCount > 0 {
		indicators = append(indicators, HelpKeyStyle.Render(fmt.Sprintf("[FILTER: %d]", ruleCount)))
	}
	// --since window chip: makes it obvious when the view is
	// time-bounded, independent of any filter rules.
	if logSince != "" {
		indicators = append(indicators, HelpKeyStyle.Render("[SINCE: "+logSince+"]"))
	}

	titleText := " " + title + " "
	if len(indicators) > 0 {
		titleText += " " + strings.Join(indicators, " ")
	}
	if visibleCount > 0 && visibleCount < len(lines) {
		titleText += BarDimStyle.Render(fmt.Sprintf(" [%d of %d lines]", visibleCount, len(lines)))
	} else {
		titleText += BarDimStyle.Render(fmt.Sprintf(" [%d lines]", len(lines)))
	}

	maxTitleWidth := max(width-2, 10)
	if ansi.StringWidth(titleText) > maxTitleWidth {
		titleText = ansi.Truncate(titleText, maxTitleWidth, "...")
	}
	return FillLinesBg(TitleStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(titleText), width, BarBg)
}

// renderLogFooter builds the footer bar for the log viewer.
func renderLogFooter(width int, statusMsg string, statusIsErr, searchActive bool, searchInput string, visualMode, canSwitchPod, canFilterContainers bool) string {
	if statusMsg != "" {
		style := HelpKeyStyle
		if statusIsErr {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true).Background(BarBg)
		}
		return StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(style.Render(statusMsg))
	}
	if searchActive {
		modeInd := SearchModeIndicator(searchInput)
		prompt := HelpKeyStyle.Render("/") + BarDimStyle.Render(": ") + BarDimStyle.Render(modeInd) + searchInput + BarDimStyle.Render("\u2588") + BarDimStyle.Render("  (enter:apply  esc:cancel)")
		return StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(prompt)
	}
	if visualMode {
		return RenderHintBar([]HintEntry{
			{Key: "j/k", Desc: "extend"},
			{Key: "h/l", Desc: "column"},
			{Key: "y", Desc: "copy"},
			{Key: "v/V/ctrl+v", Desc: "switch mode"},
			{Key: "esc", Desc: "cancel"},
		}, width)
	}
	hints := []HintEntry{
		{Key: "q", Desc: "close"},
		{Key: "j/k", Desc: "move"},
		{Key: "ctrl+d/u", Desc: "half page"},
		{Key: "ctrl+f/b", Desc: "page"},
		{Key: "f", Desc: "filter"},
		{Key: "F", Desc: "follow"},
		{Key: ">/<", Desc: "severity"},
		{Key: "tab/z", Desc: "wrap"},
		{Key: "#", Desc: "line#"},
		{Key: "s", Desc: "timestamps"},
		{Key: "R", Desc: "rel-ts"},
		{Key: "J", Desc: "json"},
		{Key: "p", Desc: "prefixes"},
		{Key: "c", Desc: "previous"},
		{Key: "t", Desc: "since"},
		{Key: "v/V/ctrl+v", Desc: "select"},
		{Key: "/", Desc: "search"},
		{Key: "n/N", Desc: "next/prev"},
		{Key: "]e/]w", Desc: "next err/warn"},
		{Key: "123G", Desc: "goto"},
		{Key: "S", Desc: "save"},
		{Key: "ctrl+s", Desc: "save all"},
	}
	if canSwitchPod {
		hints = append(hints, HintEntry{"\\", "switch pod"})
	} else if canFilterContainers {
		hints = append(hints, HintEntry{"\\", "containers"})
	}
	return RenderHintBar(hints, width)
}

// highlightSearchMatches highlights occurrences of query in each line.
// Supports substring, regex, and fuzzy search modes via HighlightMatch.
func highlightSearchMatches(lines []string, query string) []string {
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = HighlightMatch(line, query)
	}
	return result
}

// renderPlainLines renders lines without wrapping.
func renderPlainLines(lines []string, scroll, height, width int, lineNumbers bool, lineNumWidth int, cursor int, selStart, selEnd, visualStart int, visualType rune, visualCol, visualCurCol int) []string {
	var result []string

	end := scroll + height
	if end > len(lines) {
		end = len(lines)
	}

	// Reserve 1 column for cursor gutter.
	effectiveWidth := width - 1
	if effectiveWidth < 10 {
		effectiveWidth = 10
	}

	for i := scroll; i < end; i++ {
		line := lines[i]

		// Check selection state before applying styles.
		isSelected := selStart >= 0 && i >= selStart && i <= selEnd

		if isSelected {
			// Build plain text for selection highlight on raw content (no line numbers).
			// Line numbers are prepended after highlighting to avoid column offset mismatch.
			plainLine := lines[i]
			selEffWidth := effectiveWidth
			if lineNumbers {
				selEffWidth -= lineNumWidth
			}
			if len([]rune(plainLine)) > selEffWidth {
				plainLine = string([]rune(plainLine)[:selEffWidth])
			}
			line = RenderVisualSelection(plainLine, visualType, i, selStart, selEnd, visualStart, visualCol, visualCurCol, min(visualCol, visualCurCol), max(visualCol, visualCurCol))
			if lineNumbers {
				numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
				line = DimStyle.Render(numStr) + line
			}
		} else {
			// Colorize pod prefix for non-selected, non-cursor lines.
			if i != cursor {
				line = colorizePodPrefix(line)
			}
			if lineNumbers && i != cursor {
				// Non-cursor lines get line numbers here.
				// Cursor line's number is added after RenderCursorAtCol to avoid it being stripped.
				numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
				line = DimStyle.Render(numStr) + line
			}
			// Truncate to width if needed (ANSI-aware to handle styled line numbers).
			if lipgloss.Width(line) > effectiveWidth {
				line = ansi.Truncate(line, effectiveWidth, "")
			}
		}

		// Cursor gutter indicator and block cursor at column.
		if i == cursor {
			if isSelected {
				// In visual mode, don't overlay block cursor on visual selection styling.
				line = YamlCursorIndicatorStyle.Render("\u258e") + line
			} else {
				cursorLine := RenderCursorAtCol(line, lines[i], visualCurCol)
				if lineNumbers {
					numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
					cursorLine = YamlCursorIndicatorStyle.Render(numStr) + cursorLine
				}
				line = YamlCursorIndicatorStyle.Render("\u258e") + colorizePodPrefix(cursorLine)
			}
		} else {
			line = " " + line
		}

		result = append(result, line)
	}
	return result
}

// renderWrappedLines renders lines with wrapping, accounting for scroll position.
func renderWrappedLines(lines []string, scroll, height, width int, lineNumbers bool, lineNumWidth int, cursor int, selStart, selEnd, visualStart int, visualType rune, visualCol, visualCurCol int) []string {
	// Reserve 1 column for cursor gutter.
	gutterWidth := 1
	availWidth := width - gutterWidth
	if lineNumbers {
		availWidth = width - gutterWidth - lineNumWidth
	}
	if availWidth < 10 {
		availWidth = 10
	}

	// We need to figure out which source lines and which wrapped sub-lines
	// correspond to the scroll offset. For wrapping, scroll refers to source lines.
	var result []string

	end := len(lines)
	for i := scroll; i < end && len(result) < height; i++ {
		line := lines[i]
		isSelected := selStart >= 0 && i >= selStart && i <= selEnd
		// splitAndWrapLine handles JSON pretty-printed content by
		// splitting on embedded "\n" before width-wrapping. For lines
		// without embedded newlines it is identical to wrapLine.
		wrapped := splitAndWrapLine(line, availWidth)
		for j, wl := range wrapped {
			if len(result) >= height {
				break
			}

			if isSelected {
				// Highlight raw content first, then prepend line numbers
				// to avoid column offset mismatch.
				wl = RenderVisualSelection(wl, visualType, i, selStart, selEnd, visualStart, visualCol, visualCurCol, min(visualCol, visualCurCol), max(visualCol, visualCurCol))
				if lineNumbers {
					if j == 0 {
						numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
						wl = DimStyle.Render(numStr) + wl
					} else {
						wl = strings.Repeat(" ", lineNumWidth) + wl
					}
				}
			} else {
				// Colorize pod prefix for non-selected, non-cursor first sub-lines.
				if j == 0 && i != cursor {
					wl = colorizePodPrefix(wl)
				}
				if lineNumbers && (i != cursor || j != 0) {
					// Non-cursor lines get line numbers here.
					// Cursor line's number is added after RenderCursorAtCol.
					if j == 0 {
						numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
						wl = DimStyle.Render(numStr) + wl
					} else {
						wl = strings.Repeat(" ", lineNumWidth) + wl
					}
				}
			}

			// Cursor gutter indicator and block cursor at column (only on first wrapped sub-line).
			if i == cursor && j == 0 {
				if isSelected {
					wl = YamlCursorIndicatorStyle.Render("\u258e") + wl
				} else {
					cursorLine := RenderCursorAtCol(wl, lines[i], visualCurCol)
					if lineNumbers {
						numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
						cursorLine = YamlCursorIndicatorStyle.Render(numStr) + cursorLine
					}
					wl = YamlCursorIndicatorStyle.Render("\u258e") + colorizePodPrefix(cursorLine)
				}
			} else {
				wl = " " + wl
			}

			result = append(result, wl)
		}
	}
	return result
}

// WrapLine splits a line into chunks of at most width runes.
// Exported for reuse in YAML, describe, and diff view wrapping.
func WrapLine(line string, width int) []string {
	return wrapLine(line, width)
}

// splitAndWrapLine first splits line on "\n" (to expand pretty-printed
// JSON multi-line blocks) and then width-wraps each resulting fragment.
// When the line contains no embedded newlines the behaviour is
// identical to wrapLine. Used by the pretty-print render path to
// reuse the wrap renderer's visual-row machinery without reinventing
// variable-height scroll math.
func splitAndWrapLine(line string, width int) []string {
	if !strings.Contains(line, "\n") {
		return wrapLine(line, width)
	}
	parts := strings.Split(line, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, wrapLine(p, width)...)
	}
	return out
}

// wrapLine splits a line into chunks of at most width runes.
func wrapLine(line string, width int) []string {
	if width <= 0 {
		return []string{line}
	}
	runes := []rune(line)
	if len(runes) == 0 {
		return []string{""}
	}
	var parts []string
	for len(runes) > width {
		parts = append(parts, string(runes[:width]))
		runes = runes[width:]
	}
	parts = append(parts, string(runes))
	return parts
}

// StripTimestamp removes a leading kubectl timestamp (RFC3339Nano + space) from a log line.
// kubectl --timestamps format: "2024-01-15T10:30:00.000000000Z <log content>"
// With --prefix: "[pod/name container] 2024-01-15T10:30:00.000000000Z <log content>"
func StripTimestamp(line string) string {
	// Handle prefixed lines: "[pod/name container] timestamp rest"
	if len(line) > 0 && line[0] == '[' {
		closeBracket := strings.Index(line, "] ")
		if closeBracket > 0 {
			prefix := line[:closeBracket+2]
			rest := stripTimestampRaw(line[closeBracket+2:])
			return prefix + rest
		}
	}
	return stripTimestampRaw(line)
}

// stripTimestampRaw removes a leading RFC3339Nano timestamp from a string.
func stripTimestampRaw(s string) string {
	// Quick check: RFC3339Nano timestamps start with a digit and contain 'T'.
	// Minimum length: "2024-01-15T10:30:00Z " = 21 chars.
	if len(s) < 21 || s[4] != '-' {
		return s
	}
	// Find the space after the timestamp.
	spaceIdx := strings.IndexByte(s, ' ')
	if spaceIdx < 20 || spaceIdx > 35 {
		return s
	}
	// Verify it looks like a timestamp (contains 'T' between date and time).
	if s[10] != 'T' {
		return s
	}
	return s[spaceIdx+1:]
}

// podPrefixColors is a palette of distinct colors for pod/container log prefixes.
var podPrefixColors = []lipgloss.Color{
	"#7aa2f7", // blue
	"#9ece6a", // green
	"#bb9af7", // purple
	"#e0af68", // yellow
	"#f7768e", // red/pink
	"#73daca", // cyan
	"#ff9e64", // orange
	"#2ac3de", // light blue
	"#b4f9f8", // teal
	"#c0caf5", // light lavender
	"#ff007c", // magenta
	"#41a6b5", // dark cyan
}

// colorizePodPrefix replaces the "[pod/name/container] " prefix with a colorized
// version. The color is deterministically assigned based on the pod name so the
// same pod always gets the same color across the session.
func colorizePodPrefix(line string) string {
	if len(line) == 0 || line[0] != '[' {
		return line
	}
	closeBracket := strings.Index(line, "] ")
	if closeBracket < 0 {
		return line
	}
	prefix := line[1:closeBracket] // "pod/name/container"
	rest := line[closeBracket+2:]

	// Extract pod name for color hashing (between first and last slash).
	podName := prefix
	if firstSlash := strings.Index(prefix, "/"); firstSlash >= 0 {
		afterFirst := prefix[firstSlash+1:]
		if lastSlash := strings.LastIndex(afterFirst, "/"); lastSlash >= 0 {
			podName = afterFirst[:lastSlash]
		} else {
			podName = afterFirst
		}
	}

	// Simple hash to pick a deterministic color.
	var hash uint32
	for _, c := range podName {
		hash = hash*31 + uint32(c)
	}
	color := podPrefixColors[hash%uint32(len(podPrefixColors))]
	style := lipgloss.NewStyle().Foreground(color)

	return style.Render("["+prefix+"]") + " " + rest
}

// StripPodPrefix removes the leading "[pod/name/container] " prefix from a log line.
func StripPodPrefix(line string) string {
	if len(line) == 0 || line[0] != '[' {
		return line
	}
	closeBracket := strings.Index(line, "] ")
	if closeBracket < 0 {
		return line
	}
	return line[closeBracket+2:]
}

// sanitizeLogLine replaces non-printable control characters (except tab) with
// the Unicode replacement character. Binary data from processes like MySQL
// handshakes contains bytes that break terminal width calculations and corrupt
// the log viewer layout.
func sanitizeLogLine(s string) string {
	// Fast path: check if any sanitization is needed.
	needsSanitize := false
	for _, r := range s {
		if r != '\t' && r != '\n' && (r < 32 || r == 127) {
			needsSanitize = true
			break
		}
	}
	if !needsSanitize {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r != '\t' && r != '\n' && (r < 32 || r == 127) {
			b.WriteRune('\ufffd') // Unicode replacement character
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
