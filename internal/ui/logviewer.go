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
func RenderLogViewer(lines []string, scroll, width, height int, follow, wrap, lineNumbers, timestamps, previous bool, title, searchQuery, searchInput string, searchActive, canSwitchPod, canFilterContainers, hasMoreHistory, loadingHistory bool, statusMsg string, statusIsErr bool, cursor int, visualMode bool, visualStart int, visualType rune, visualCol, visualCurCol int) string {
	// Title bar with status indicators.
	var indicators []string
	if follow {
		indicators = append(indicators, HelpKeyStyle.Render("[FOLLOW]"))
	}
	if wrap {
		indicators = append(indicators, HelpKeyStyle.Render("[WRAP]"))
	}
	if lineNumbers {
		indicators = append(indicators, HelpKeyStyle.Render("[LINE#]"))
	}
	if timestamps {
		indicators = append(indicators, HelpKeyStyle.Render("[TIMESTAMPS]"))
	}
	if previous {
		indicators = append(indicators, HelpKeyStyle.Render("[PREVIOUS]"))
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
	if loadingHistory {
		indicators = append(indicators, HelpKeyStyle.Render("[LOADING HISTORY...]"))
	}
	if searchQuery != "" {
		indicators = append(indicators, HelpKeyStyle.Render("[/"+searchQuery+"]"))
	}

	titleText := " " + title + " "
	if len(indicators) > 0 {
		titleText += " " + strings.Join(indicators, " ")
	}
	lineInfo := BarDimStyle.Render(fmt.Sprintf(" [%d lines]", len(lines)))
	titleText += lineInfo

	// Constrain title to terminal width to prevent wrapping (which adds extra lines).
	maxTitleWidth := width - 2 // account for TitleStyle padding
	if maxTitleWidth < 10 {
		maxTitleWidth = 10
	}
	if ansi.StringWidth(titleText) > maxTitleWidth {
		titleText = ansi.Truncate(titleText, maxTitleWidth, "…")
	}
	titleBar := FillLinesBg(TitleStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(titleText), width, BarBg)

	// Footer: show status message, search input, or key hints.
	// Use BarDimStyle (not DimStyle) so hint text matches the bar background.
	var footer string
	if statusMsg != "" {
		style := HelpKeyStyle
		if statusIsErr {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true).Background(BarBg)
		}
		footer = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(style.Render(statusMsg))
	} else if searchActive {
		prompt := HelpKeyStyle.Render("/") + BarDimStyle.Render(": ") + searchInput + BarDimStyle.Render("\u2588") + BarDimStyle.Render("  (enter:apply  esc:cancel)")
		footer = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(prompt)
	} else if visualMode {
		footer = RenderHintBar([]HintEntry{
			{Key: "j/k", Desc: "extend"},
			{Key: "h/l", Desc: "column"},
			{Key: "y", Desc: "copy"},
			{Key: "v/V/ctrl+v", Desc: "switch mode"},
			{Key: "esc", Desc: "cancel"},
		}, width)
	} else {
		hints := []HintEntry{
			{Key: "q", Desc: "close"},
			{Key: "j/k", Desc: "move"},
			{Key: "ctrl+d/u", Desc: "half page"},
			{Key: "ctrl+f/b", Desc: "page"},
			{Key: "f", Desc: "follow"},
			{Key: "tab/z", Desc: "wrap"},
			{Key: "#", Desc: "line#"},
			{Key: "s", Desc: "timestamps"},
			{Key: "c", Desc: "previous"},
			{Key: "v/V/ctrl+v", Desc: "select"},
			{Key: "/", Desc: "search"},
			{Key: "n/N", Desc: "next/prev"},
			{Key: "123G", Desc: "goto"},
			{Key: "S", Desc: "save"},
			{Key: "ctrl+s", Desc: "save all"},
		}
		if canSwitchPod {
			hints = append(hints, HintEntry{"\\", "switch pod"})
		} else if canFilterContainers {
			hints = append(hints, HintEntry{"\\", "containers"})
		}
		footer = RenderHintBar(hints, width)
	}

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

	// Strip timestamps from visible lines if not showing them.
	// Timestamps are always streamed (--timestamps) but only displayed when toggled on.
	displayLines := lines
	if !timestamps && len(lines) > 0 {
		end := scroll + contentHeight
		if end > len(lines) {
			end = len(lines)
		}
		displayLines = make([]string, len(lines))
		copy(displayLines, lines)
		for i := scroll; i < end; i++ {
			displayLines[i] = stripTimestamp(lines[i])
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

	// Build visible lines, handling wrapping.
	var rendered []string
	if wrap {
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

// highlightSearchMatches highlights occurrences of query in each line.
func highlightSearchMatches(lines []string, query string) []string {
	queryLower := strings.ToLower(query)
	result := make([]string, len(lines))
	for i, line := range lines {
		lineLower := strings.ToLower(line)
		if !strings.Contains(lineLower, queryLower) {
			result[i] = line
			continue
		}
		// Highlight all occurrences (case-insensitive).
		var b strings.Builder
		pos := 0
		for pos < len(line) {
			idx := strings.Index(strings.ToLower(line[pos:]), queryLower)
			if idx < 0 {
				b.WriteString(line[pos:])
				break
			}
			b.WriteString(line[pos : pos+idx])
			b.WriteString(LogSearchHighlightStyle.Render(line[pos+idx : pos+idx+len(query)]))
			pos = pos + idx + len(query)
		}
		result[i] = b.String()
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
		wrapped := wrapLine(line, availWidth)
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

// stripTimestamp removes a leading kubectl timestamp (RFC3339Nano + space) from a log line.
// kubectl --timestamps format: "2024-01-15T10:30:00.000000000Z <log content>"
// With --prefix: "[pod/name container] 2024-01-15T10:30:00.000000000Z <log content>"
func stripTimestamp(line string) string {
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

// sanitizeLogLine replaces non-printable control characters (except tab) with
// the Unicode replacement character. Binary data from processes like MySQL
// handshakes contains bytes that break terminal width calculations and corrupt
// the log viewer layout.
func sanitizeLogLine(s string) string {
	// Fast path: check if any sanitization is needed.
	needsSanitize := false
	for _, r := range s {
		if r != '\t' && (r < 32 || r == 127) {
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
		if r != '\t' && (r < 32 || r == 127) {
			b.WriteRune('\ufffd') // Unicode replacement character
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
