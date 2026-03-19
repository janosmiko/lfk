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
	lineInfo := DimStyle.Render(fmt.Sprintf(" [%d lines]", len(lines)))
	titleText += lineInfo

	// Constrain title to terminal width to prevent wrapping (which adds extra lines).
	maxTitleWidth := width - 2 // account for TitleStyle padding
	if maxTitleWidth < 10 {
		maxTitleWidth = 10
	}
	if ansi.StringWidth(titleText) > maxTitleWidth {
		titleText = ansi.Truncate(titleText, maxTitleWidth, "…")
	}
	titleBar := TitleStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(titleText)

	// Footer: show status message, search input, or key hints.
	var footer string
	if statusMsg != "" {
		style := HelpKeyStyle
		if statusIsErr {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true)
		}
		footer = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(style.Render(statusMsg))
	} else if searchActive {
		prompt := HelpKeyStyle.Render("/") + DimStyle.Render(": ") + searchInput + DimStyle.Render("\u2588") + DimStyle.Render("  (enter:apply  esc:cancel)")
		footer = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(prompt)
	} else if visualMode {
		hintParts := []string{
			HelpKeyStyle.Render("j/k") + DimStyle.Render(":extend"),
			HelpKeyStyle.Render("h/l") + DimStyle.Render(":column"),
			HelpKeyStyle.Render("y") + DimStyle.Render(":copy"),
			HelpKeyStyle.Render("v/V/ctrl+v") + DimStyle.Render(":switch mode"),
			HelpKeyStyle.Render("esc") + DimStyle.Render(":cancel"),
		}
		footer = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(strings.Join(hintParts, DimStyle.Render(" | ")))
	} else {
		hintParts := []string{
			HelpKeyStyle.Render("q") + DimStyle.Render(":close"),
			HelpKeyStyle.Render("j/k") + DimStyle.Render(":move"),
			HelpKeyStyle.Render("ctrl+d/u") + DimStyle.Render(":half page"),
			HelpKeyStyle.Render("ctrl+f/b") + DimStyle.Render(":page"),
			HelpKeyStyle.Render("f") + DimStyle.Render(":follow"),
			HelpKeyStyle.Render("w") + DimStyle.Render(":wrap"),
			HelpKeyStyle.Render("#") + DimStyle.Render(":line#"),
			HelpKeyStyle.Render("s") + DimStyle.Render(":timestamps"),
			HelpKeyStyle.Render("c") + DimStyle.Render(":previous"),
			HelpKeyStyle.Render("v/V/ctrl+v") + DimStyle.Render(":select"),
			HelpKeyStyle.Render("/") + DimStyle.Render(":search"),
			HelpKeyStyle.Render("n/N") + DimStyle.Render(":next/prev"),
			HelpKeyStyle.Render("123G") + DimStyle.Render(":goto"),
			HelpKeyStyle.Render("W") + DimStyle.Render(":save"),
			HelpKeyStyle.Render("ctrl+s") + DimStyle.Render(":save all"),
		}
		if canSwitchPod {
			hintParts = append(hintParts, HelpKeyStyle.Render("\\")+DimStyle.Render(":switch pod"))
		} else if canFilterContainers {
			hintParts = append(hintParts, HelpKeyStyle.Render("\\")+DimStyle.Render(":containers"))
		}
		footer = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(strings.Join(hintParts, DimStyle.Render(" | ")))
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

	// Compute visual selection range.
	selStart, selEnd := -1, -1
	if visualMode {
		selStart = min(visualStart, cursor)
		selEnd = max(visualStart, cursor)
	}

	// Build visible lines, handling wrapping.
	var rendered []string
	if wrap {
		rendered = renderWrappedLines(displayLines, scroll, contentHeight, contentWidth, lineNumbers, lineNumWidth, cursor, selStart, selEnd, visualType, visualCol, visualCurCol)
	} else {
		rendered = renderPlainLines(displayLines, scroll, contentHeight, contentWidth, lineNumbers, lineNumWidth, cursor, selStart, selEnd, visualType, visualCol, visualCurCol)
	}

	// Highlight search matches in rendered lines.
	if searchQuery != "" {
		rendered = highlightSearchMatches(rendered, searchQuery)
	}

	// Pad to fill content height.
	for len(rendered) < contentHeight {
		rendered = append(rendered, "")
	}

	bodyContent := strings.Join(rendered, "\n")
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorPrimary)).
		Padding(0, 1).
		Width(width - 2).
		Height(contentHeight).
		MaxHeight(contentHeight + 2)
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
func renderPlainLines(lines []string, scroll, height, width int, lineNumbers bool, lineNumWidth int, cursor int, selStart, selEnd int, visualType rune, visualCol, visualCurCol int) []string {
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
			line = RenderVisualSelection(plainLine, visualType, i, selStart, selEnd, visualCol, visualCurCol, min(visualCol, visualCurCol), max(visualCol, visualCurCol))
			if lineNumbers {
				numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
				line = DimStyle.Render(numStr) + line
			}
		} else {
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
				line = YamlCursorIndicatorStyle.Render("\u258e") + cursorLine
			}
		} else {
			line = " " + line
		}

		result = append(result, line)
	}
	return result
}

// renderWrappedLines renders lines with wrapping, accounting for scroll position.
func renderWrappedLines(lines []string, scroll, height, width int, lineNumbers bool, lineNumWidth int, cursor int, selStart, selEnd int, visualType rune, visualCol, visualCurCol int) []string {
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
				wl = RenderVisualSelection(wl, visualType, i, selStart, selEnd, visualCol, visualCurCol, min(visualCol, visualCurCol), max(visualCol, visualCurCol))
				if lineNumbers {
					if j == 0 {
						numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
						wl = DimStyle.Render(numStr) + wl
					} else {
						wl = strings.Repeat(" ", lineNumWidth) + wl
					}
				}
			} else {
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
					wl = YamlCursorIndicatorStyle.Render("\u258e") + cursorLine
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
