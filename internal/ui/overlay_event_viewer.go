package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderEventTimelineOverlay renders the event timeline overlay content.
// Events are displayed with relative timestamps, type indicators, and scrolling support.
func RenderEventTimelineOverlay(events []EventTimelineEntry, resourceName string, scroll, width, height int) string {
	var b strings.Builder

	title := fmt.Sprintf("Event Timeline - %s", resourceName)
	b.WriteString(OverlayTitleStyle.Render(title))
	b.WriteString("\n")

	if len(events) == 0 {
		b.WriteString(OverlayDimStyle.Render("No events found"))
		return b.String()
	}

	// Reserve lines for header, blank line before footer, footer.
	maxLines := max(height-4, 1)

	// Content width inside OverlayStyle Padding(1,2) = 2 left + 2 right.
	contentWidth := width - 4

	// Calculate available width for message wrapping.
	msgIndent := "           "
	msgMaxWidth := max(contentWidth-len(msgIndent), 20)
	msgContIndent := msgIndent + "  "
	msgContWidth := max(msgMaxWidth-2, 10)

	// Calculate visual lines per event for scroll/viewport calculations.
	msgLineCount := func(idx int) int {
		msgLen := len([]rune(events[idx].Message))
		if msgLen <= msgMaxWidth {
			return 1
		}
		remaining := msgLen - msgMaxWidth
		return 1 + (remaining+msgContWidth-1)/msgContWidth
	}
	eventLines := func(idx int) int {
		return 1 + msgLineCount(idx) // 1 header line + message lines
	}

	// Clamp scroll: find max scroll where remaining events fill the viewport.
	if scroll < 0 {
		scroll = 0
	}
	if scroll >= len(events) {
		scroll = max(len(events)-1, 0)
	}
	// Shrink scroll if there's empty space at the bottom.
	for scroll > 0 {
		lines := 0
		for i := scroll; i < len(events); i++ {
			lines += eventLines(i)
		}
		if lines >= maxLines {
			break
		}
		scroll--
	}

	// Compute end index based on available visual lines.
	// Separators between events just terminate the previous line (already
	// counted in eventLines), they don't add extra visual lines.
	usedLines := 0
	end := scroll
	for end < len(events) {
		el := eventLines(end)
		if usedLines+el > maxLines {
			break
		}
		usedLines += el
		end++
	}
	if end == scroll && end < len(events) {
		usedLines += eventLines(end)
		end++
	}

	// Styles for event type indicators.
	normalDot := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Background(SurfaceBg).Render("●") // green filled circle
	warningDot := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Background(SurfaceBg).Render("●")    // red filled circle
	reasonStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorFile)).Background(SurfaceBg)
	sourceStyle := OverlayDimStyle
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning)).Background(SurfaceBg)

	for i := scroll; i < end; i++ {
		event := events[i]

		// Relative timestamp.
		ts := RelativeTime(event.Timestamp)
		tsStr := OverlayDimStyle.Render(fmt.Sprintf("%-8s", ts))

		// Type indicator.
		dot := normalDot
		if event.Type == "Warning" {
			dot = warningDot
		}

		// Reason.
		reason := reasonStyle.Render(event.Reason)

		// Source.
		src := ""
		if event.Source != "" {
			src = " " + sourceStyle.Render("["+event.Source+"]")
		}

		// Involved object info (show if different from the main resource).
		involved := ""
		if event.InvolvedName != resourceName {
			involved = " " + OverlayDimStyle.Render(event.InvolvedKind+"/"+event.InvolvedName)
		}

		// Count.
		countStr := ""
		if event.Count > 1 {
			countStr = " " + countStyle.Render(fmt.Sprintf("(x%d)", event.Count))
		}

		// First line: timestamp, dot, reason, source, involved, count.
		line := fmt.Sprintf("  %s %s %s%s%s%s", tsStr, dot, reason, src, involved, countStr)
		b.WriteString(line)
		b.WriteString("\n")

		// Message lines: wrap long messages instead of truncating.
		// Continuation lines get extra indentation to distinguish them.
		msg := event.Message
		msgRunes := []rune(msg)
		firstChunkEnd := min(msgMaxWidth, len(msgRunes))
		fmt.Fprintf(&b, "%s%s", msgIndent, OverlayNormalStyle.Render(string(msgRunes[:firstChunkEnd])))
		for start := firstChunkEnd; start < len(msgRunes); start += msgContWidth {
			chunkEnd := min(start+msgContWidth, len(msgRunes))
			chunk := string(msgRunes[start:chunkEnd])
			b.WriteString("\n")
			fmt.Fprintf(&b, "%s%s", msgContIndent, OverlayDimStyle.Render(chunk))
		}

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Pad to fixed height so the footer stays in place.
	for usedLines < maxLines {
		b.WriteString("\n")
		usedLines++
	}
	b.WriteString("\n")

	// Scroll info (hints moved to main status bar).
	scrollInfo := fmt.Sprintf("%d events", len(events))
	if scroll > 0 || end < len(events) {
		scrollInfo += fmt.Sprintf(" | showing %d-%d", scroll+1, end)
	}
	b.WriteString(OverlayDimStyle.Render(scrollInfo))

	return b.String()
}

// EventViewerParams holds state for the rich event viewer rendering.
type EventViewerParams struct {
	Lines        []string // flat text lines (one per event)
	ResourceName string
	Scroll       int
	Cursor       int
	CursorCol    int
	Width        int
	Height       int
	Wrap         bool
	Fullscreen   bool
	VisualMode   byte // 0=off, 'v'=char, 'V'=line, 'B'=block
	VisualStart  int
	VisualCol    int
	SearchQuery  string
	SearchActive bool
	SearchInput  string
}

// RenderEventViewer renders the event viewer with cursor, visual selection,
// search highlighting, and fullscreen support.
func RenderEventViewer(p EventViewerParams) string {
	var b strings.Builder

	// Title with mode indicators.
	title := "Event Timeline"
	if p.ResourceName != "" {
		title += " - " + p.ResourceName
	}
	var indicators []string
	if p.Fullscreen {
		indicators = append(indicators, "FULLSCREEN")
	}
	if p.Wrap {
		indicators = append(indicators, "WRAP")
	}
	if p.VisualMode != 0 {
		switch p.VisualMode {
		case 'v':
			indicators = append(indicators, "VISUAL")
		case 'V':
			indicators = append(indicators, "VISUAL LINE")
		case 'B':
			indicators = append(indicators, "VISUAL BLOCK")
		}
	}
	if p.SearchQuery != "" {
		indicators = append(indicators, "/"+p.SearchQuery)
	}
	if len(indicators) > 0 {
		title += " [" + strings.Join(indicators, " | ") + "]"
	}
	b.WriteString(OverlayTitleStyle.Render(title))
	b.WriteString("\n")

	if len(p.Lines) == 0 {
		b.WriteString(OverlayDimStyle.Render("No events found"))
		return b.String()
	}

	// Calculate visible area.
	maxVisible := max(p.Height-4, 1) // reserve for title, blank, footer, padding

	// Clamp scroll.
	maxScroll := max(len(p.Lines)-maxVisible, 0)
	scroll := max(min(p.Scroll, maxScroll), 0)

	end := min(scroll+maxVisible, len(p.Lines))

	// Visual selection range.
	selStart := min(p.VisualStart, p.Cursor)
	selEnd := max(p.VisualStart, p.Cursor)
	colStart := min(p.VisualCol, p.CursorCol)
	colEnd := max(p.VisualCol, p.CursorCol)

	// Search query for highlighting.
	lowerQuery := strings.ToLower(p.SearchQuery)

	// Available content width.
	// Overlay mode: OverlayStyle adds border(2) + padding(4) = 6, plus 1 for gutter.
	// Fullscreen mode: no border/padding, just gutter + margin.
	contentW := p.Width - 7
	if p.Fullscreen {
		contentW = p.Width - 2
	}
	if contentW < 10 {
		contentW = 10
	}

	wrapStyle := lipgloss.NewStyle().Width(contentW)

	evLineCtx := eventLineContext{
		wrapStyle:  wrapStyle,
		contentW:   contentW,
		lowerQuery: lowerQuery,
		selStart:   selStart,
		selEnd:     selEnd,
		colStart:   colStart,
		colEnd:     colEnd,
	}
	for i := scroll; i < end; i++ {
		b.WriteString(renderEventViewerLine(p, i, evLineCtx))
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Pad to fixed height.
	rendered := end - scroll
	for rendered < maxVisible {
		b.WriteString("\n")
		rendered++
	}
	b.WriteString("\n")

	// Search input / footer.
	if p.SearchActive {
		b.WriteString(OverlayFilterStyle.Render("/ " + p.SearchInput + "█"))
	} else {
		// Footer info.
		info := fmt.Sprintf("%d events", len(p.Lines))
		if scroll > 0 || end < len(p.Lines) {
			info += fmt.Sprintf(" | line %d/%d", p.Cursor+1, len(p.Lines))
		} else {
			info += fmt.Sprintf(" | line %d", p.Cursor+1)
		}
		if p.VisualMode != 0 {
			lineCount := selEnd - selStart + 1
			info += fmt.Sprintf(" | %d selected", lineCount)
		}
		b.WriteString(OverlayDimStyle.Render(info))
	}

	return b.String()
}

// eventLineContext holds shared state for rendering individual event viewer lines.
type eventLineContext struct {
	wrapStyle  lipgloss.Style
	contentW   int
	lowerQuery string
	selStart   int
	selEnd     int
	colStart   int
	colEnd     int
}

// renderEventViewerLine renders a single line in the event viewer.
func renderEventViewerLine(p EventViewerParams, i int, ctx eventLineContext) string {
	line := p.Lines[i]
	inSelection := p.VisualMode != 0 && i >= ctx.selStart && i <= ctx.selEnd
	isCursorLine := i == p.Cursor

	fitLine := line
	if p.Wrap {
		fitLine = ctx.wrapStyle.Render(line)
	} else if len([]rune(fitLine)) > ctx.contentW {
		fitLine = string([]rune(fitLine)[:ctx.contentW])
	}

	if inSelection {
		selLine := line
		if len([]rune(selLine)) > ctx.contentW {
			selLine = string([]rune(selLine)[:ctx.contentW])
		}
		rendered := RenderVisualSelection(
			selLine, rune(p.VisualMode),
			i, ctx.selStart, ctx.selEnd,
			p.VisualStart, p.VisualCol, p.CursorCol,
			ctx.colStart, ctx.colEnd,
		)
		if isCursorLine {
			return YamlCursorIndicatorStyle.Render("▎") + rendered
		}
		return " " + rendered
	}

	if isCursorLine {
		return renderEventCursorLine(p, line, fitLine, ctx)
	}

	return renderEventNormalLine(p, line, fitLine, ctx)
}

// renderEventCursorLine renders the cursor line with gutter indicator and block cursor.
func renderEventCursorLine(p EventViewerParams, line, fitLine string, ctx eventLineContext) string {
	gutter := YamlCursorIndicatorStyle.Render("▎")
	if p.Wrap {
		displayLine := fitLine
		if p.SearchQuery != "" {
			displayLine = ctx.wrapStyle.Render(highlightEventSearchLine(line, ctx.lowerQuery))
		}
		return gutter + displayLine
	}
	displayLine := fitLine
	if p.SearchQuery != "" {
		displayLine = highlightEventSearchLine(displayLine, ctx.lowerQuery)
	}
	return gutter + RenderCursorAtCol(displayLine, fitLine, p.CursorCol)
}

// renderEventNormalLine renders a non-cursor, non-selected line.
func renderEventNormalLine(p EventViewerParams, line, fitLine string, ctx eventLineContext) string {
	if p.Wrap {
		displayLine := fitLine
		if p.SearchQuery != "" {
			displayLine = ctx.wrapStyle.Render(highlightEventSearchLine(line, ctx.lowerQuery))
		}
		return " " + displayLine
	}
	displayLine := fitLine
	if p.SearchQuery != "" {
		displayLine = highlightEventSearchLine(displayLine, ctx.lowerQuery)
	} else {
		displayLine = OverlayNormalStyle.Render(displayLine)
	}
	return " " + displayLine
}

// highlightEventSearchLine highlights search matches in a single line using
// the overlay styles. The query should be pre-lowered for case-insensitive matching.
func highlightEventSearchLine(line, lowerQuery string) string {
	if lowerQuery == "" {
		return OverlayNormalStyle.Render(line)
	}
	lowerLine := strings.ToLower(line)
	matchStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSelectedFg)).
		Background(lipgloss.Color(ColorWarning)).
		Bold(true)

	var result strings.Builder
	pos := 0
	for pos < len(line) {
		idx := strings.Index(lowerLine[pos:], lowerQuery)
		if idx < 0 {
			result.WriteString(OverlayNormalStyle.Render(line[pos:]))
			break
		}
		if idx > 0 {
			result.WriteString(OverlayNormalStyle.Render(line[pos : pos+idx]))
		}
		matchEnd := pos + idx + len(lowerQuery)
		result.WriteString(matchStyle.Render(line[pos+idx : matchEnd]))
		pos = matchEnd
	}
	return result.String()
}
