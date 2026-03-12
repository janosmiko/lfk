package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// LogSearchHighlightStyle highlights matched substrings in log lines.
var LogSearchHighlightStyle = lipgloss.NewStyle().
	Background(lipgloss.Color(ColorWarning)).
	Foreground(lipgloss.Color(ColorBase)).
	Bold(true)

// RenderLogViewer renders the full-screen log viewer.
func RenderLogViewer(lines []string, scroll, width, height int, follow, wrap, lineNumbers bool, title, searchQuery, searchInput string, searchActive, canSwitchPod bool) string {
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
	if searchQuery != "" {
		indicators = append(indicators, HelpKeyStyle.Render("[/"+searchQuery+"]"))
	}

	titleText := " " + title + " "
	if len(indicators) > 0 {
		titleText += " " + strings.Join(indicators, " ")
	}
	lineInfo := DimStyle.Render(fmt.Sprintf(" [%d lines]", len(lines)))
	titleText += lineInfo

	titleBar := TitleStyle.Render(titleText)

	// Footer: show search input when active, otherwise key hints.
	var footer string
	if searchActive {
		prompt := HelpKeyStyle.Render("/") + DimStyle.Render(": ") + searchInput + DimStyle.Render("\u2588") + DimStyle.Render("  (enter:apply  esc:cancel)")
		footer = StatusBarBgStyle.Width(width).Render(prompt)
	} else {
		hintParts := []string{
			HelpKeyStyle.Render("q") + DimStyle.Render(":close"),
			HelpKeyStyle.Render("j/k") + DimStyle.Render(":scroll"),
			HelpKeyStyle.Render("ctrl+d/u") + DimStyle.Render(":half page"),
			HelpKeyStyle.Render("ctrl+f/b") + DimStyle.Render(":page"),
			HelpKeyStyle.Render("f") + DimStyle.Render(":follow"),
			HelpKeyStyle.Render("w") + DimStyle.Render(":wrap"),
			HelpKeyStyle.Render("l") + DimStyle.Render(":line#"),
			HelpKeyStyle.Render("/") + DimStyle.Render(":search"),
			HelpKeyStyle.Render("n/N") + DimStyle.Render(":next/prev"),
			HelpKeyStyle.Render("123G") + DimStyle.Render(":goto"),
		}
		if canSwitchPod {
			hintParts = append(hintParts, HelpKeyStyle.Render("P")+DimStyle.Render(":switch pod"))
		}
		footer = StatusBarBgStyle.Width(width).Render(strings.Join(hintParts, DimStyle.Render(" | ")))
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

	// Build visible lines, handling wrapping.
	var rendered []string
	if wrap {
		rendered = renderWrappedLines(lines, scroll, contentHeight, contentWidth, lineNumbers, lineNumWidth)
	} else {
		rendered = renderPlainLines(lines, scroll, contentHeight, contentWidth, lineNumbers, lineNumWidth)
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
		Height(contentHeight)
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
func renderPlainLines(lines []string, scroll, height, width int, lineNumbers bool, lineNumWidth int) []string {
	var result []string

	end := scroll + height
	if end > len(lines) {
		end = len(lines)
	}

	for i := scroll; i < end; i++ {
		line := lines[i]
		if lineNumbers {
			numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
			line = DimStyle.Render(numStr) + line
		}
		// Truncate to width if needed.
		if lipgloss.Width(line) > width {
			runes := []rune(line)
			if len(runes) > width {
				line = string(runes[:width])
			}
		}
		result = append(result, line)
	}
	return result
}

// renderWrappedLines renders lines with wrapping, accounting for scroll position.
func renderWrappedLines(lines []string, scroll, height, width int, lineNumbers bool, lineNumWidth int) []string {
	availWidth := width
	if lineNumbers {
		availWidth = width - lineNumWidth
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
		wrapped := wrapLine(line, availWidth)
		for j, wl := range wrapped {
			if len(result) >= height {
				break
			}
			if lineNumbers {
				if j == 0 {
					numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
					wl = DimStyle.Render(numStr) + wl
				} else {
					wl = strings.Repeat(" ", lineNumWidth) + wl
				}
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
