package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	// HangingIndent is the column at which the message field starts in
	// Lines. Continuation lines under wrap are indented by this many
	// spaces so the wrapped message text stays under the original
	// message column (table-style continuation). Zero means flush-left.
	HangingIndent int
}

// wrappedEventChunk describes one physical sub-line of a wrapped event
// row, tracking how it maps back to the original logical line so the
// renderer can place a block cursor or per-character selection
// highlight on the correct physical sub-line + column.
type wrappedEventChunk struct {
	text       string // full sub-line text including any leading indent
	indentCols int    // leading pad width within text
	origStart  int    // index of first original-line char in this chunk
	origLen    int    // count of original-line chars in this chunk
}

// wrappedEventChunks splits a logical event line into physical sub-lines
// with column tracking. The first sub-line uses the full contentW; later
// sub-lines are indented by hangingIndent so the wrapped message stays
// under the original message column.
//
// Falls back to flush-left wrap if contentW is too narrow to leave any
// useful room for continuation text after the indent.
func wrappedEventChunks(line string, contentW, hangingIndent int) []wrappedEventChunk {
	runes := []rune(line)
	if contentW <= 0 || len(runes) <= contentW {
		return []wrappedEventChunk{{text: line, origLen: len(runes)}}
	}
	// Clamp the indent: leave at least 8 chars per continuation line.
	if hangingIndent < 0 || hangingIndent >= contentW-8 {
		hangingIndent = 0
	}
	out := []wrappedEventChunk{{
		text:      string(runes[:contentW]),
		origStart: 0,
		origLen:   contentW,
	}}
	pad := strings.Repeat(" ", hangingIndent)
	chunkSize := contentW - hangingIndent
	pos := contentW
	for pos < len(runes) {
		n := min(chunkSize, len(runes)-pos)
		out = append(out, wrappedEventChunk{
			text:       pad + string(runes[pos:pos+n]),
			indentCols: hangingIndent,
			origStart:  pos,
			origLen:    n,
		})
		pos += n
	}
	return out
}

// WrapEventLine wraps a single event timeline line into physical lines
// that fit within contentW. The first physical line uses the full
// width; continuation lines are indented by hangingIndent so wrapped
// message text aligns under the original message column instead of
// re-flowing flush to the left margin.
func WrapEventLine(line string, contentW, hangingIndent int) []string {
	chunks := wrappedEventChunks(line, contentW, hangingIndent)
	out := make([]string, len(chunks))
	for i, c := range chunks {
		out[i] = c.text
	}
	return out
}

// WrappedEventRowOpts bundles inputs for RenderWrappedEventRow. The
// caller computes which selection range applies (line-mode highlights
// the whole row; char-mode pre-computes the absolute column range on
// the logical line). Cursor block rendering is opt-in via IsCursor.
type WrappedEventRowOpts struct {
	Line          string
	Gutter        string // prefix prepended to every physical sub-line
	ContentW      int
	HangingIndent int
	// Cursor block:
	IsCursor  bool
	CursorCol int
	// Selection:
	SelectionLine bool // full-row highlight (V mode)
	SelStart      int  // absolute col on the logical line (inclusive)
	SelEnd        int  // absolute col on the logical line (exclusive)
	// Search highlight (applied only when no selection is active):
	LowerSearch string
}

// RenderWrappedEventRow renders one logical event line as a multi-line
// wrapped block. Mirrors the convention used by the YAML and describe
// viewers: the cursor block and per-line selection highlight sit on
// the FIRST physical sub-line, continuation sub-lines are plain
// (gutter + indented text). RenderCursorAtCol's "append a highlighted
// space when col >= width" branch keeps the cursor visible when
// CursorCol drifted past the visible chunk after navigating from a
// long event to a short one — same behaviour YAML relies on.
//
// V-mode (line) selection still spans every sub-line because the user
// explicitly asked for the whole wrapped block to be highlighted; v
// and B modes stick to the simpler single-line model.
func RenderWrappedEventRow(opts WrappedEventRowOpts) string {
	chunks := wrappedEventChunks(opts.Line, opts.ContentW, opts.HangingIndent)
	if len(chunks) == 0 {
		return opts.Gutter
	}

	var sb strings.Builder
	for i, ch := range chunks {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(opts.Gutter)
		text := ch.text

		styled := false
		if opts.SelectionLine {
			// V-mode: full highlight on every sub-line.
			text = SelectedStyle.Render(ansi.Strip(text))
			styled = true
		} else if i == 0 {
			// First sub-line carries selection and search styling.
			switch {
			case opts.SelEnd > opts.SelStart:
				text = applyCharSelectionToFirstSubLine(text, opts.SelStart, opts.SelEnd)
				styled = true
			case opts.LowerSearch != "":
				text = highlightEventSearchLine(text, opts.LowerSearch)
				styled = true
			}
		} else if opts.LowerSearch != "" {
			// Highlight matches on continuation sub-lines too.
			text = highlightEventSearchLine(text, opts.LowerSearch)
			styled = true
		}
		if !styled {
			// Apply overlay-normal styling so wrapped rows match the
			// surrounding (non-wrap) rows' fg/bg pair instead of
			// punching a transparent rectangle through the overlay.
			text = OverlayNormalStyle.Render(text)
		}

		if opts.IsCursor && i == 0 {
			text = RenderCursorAtCol(text, "", opts.CursorCol)
		}
		sb.WriteString(text)
	}
	return sb.String()
}

// applyCharSelectionToFirstSubLine highlights the [selStart, selEnd)
// column range on the first physical sub-line of a wrapped event. The
// range is clamped to the sub-line's bounds; the selection that
// extends past contentW is not shown on continuation lines (matches
// the YAML viewer's behaviour, where v-mode selection only paints the
// first sub-line).
func applyCharSelectionToFirstSubLine(text string, selStart, selEnd int) string {
	runes := []rune(text)
	if selStart < 0 {
		selStart = 0
	}
	if selEnd > len(runes) {
		selEnd = len(runes)
	}
	if selEnd <= selStart {
		return text
	}
	return string(runes[:selStart]) +
		SelectedStyle.Render(string(runes[selStart:selEnd])) +
		string(runes[selEnd:])
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

	evLineCtx := eventLineContext{
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

	gutter := " "
	if isCursorLine {
		gutter = YamlCursorIndicatorStyle.Render("▎")
	}

	// Wrap-mode rendering is unified via RenderWrappedEventRow: it
	// handles the gutter, block-cursor placement on the physical
	// sub-line containing CursorCol, V-mode full-row highlight, and
	// v-mode char selection across sub-lines. Block-mode (B) keeps the
	// single-line truncate path below because rectangular column
	// selection over wrapped sub-lines has no meaningful geometry.
	if p.Wrap && (!inSelection || p.VisualMode == 'V' || p.VisualMode == 'v') {
		opts := WrappedEventRowOpts{
			Line:          line,
			Gutter:        gutter,
			ContentW:      ctx.contentW,
			HangingIndent: p.HangingIndent,
			IsCursor:      isCursorLine,
			CursorCol:     p.CursorCol,
		}
		switch {
		case inSelection && p.VisualMode == 'V':
			opts.SelectionLine = true
		case inSelection && p.VisualMode == 'v':
			opts.SelStart, opts.SelEnd = charSelectionRangeForLine(p, ctx, i)
		default:
			opts.LowerSearch = ctx.lowerQuery
		}
		return RenderWrappedEventRow(opts)
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
		return gutter + rendered
	}

	// Non-wrap, non-selection path.
	fitLine := line
	if len([]rune(fitLine)) > ctx.contentW {
		fitLine = string([]rune(fitLine)[:ctx.contentW])
	}
	if isCursorLine {
		return renderEventCursorLine(p, fitLine, ctx, gutter)
	}
	return renderEventNormalLine(p, fitLine, ctx, gutter)
}

// charSelectionRangeForLine returns the [start, end) column range to
// highlight on the given logical line under v-mode selection. Mirrors
// renderCharSelection's per-line logic: anchor-only line highlights
// from anchorCol to end-of-line; cursor-only line highlights from 0 to
// cursorCol+1; middle lines highlight everything (caller can detect
// "everything" via end == len(line)).
func charSelectionRangeForLine(p EventViewerParams, ctx eventLineContext, i int) (start, end int) {
	lineWidth := len([]rune(p.Lines[i]))
	if ctx.selStart == ctx.selEnd {
		return min(p.VisualCol, p.CursorCol), max(p.VisualCol, p.CursorCol) + 1
	}
	var startCol, endCol int
	if p.VisualStart <= p.Cursor {
		startCol, endCol = p.VisualCol, p.CursorCol
	} else {
		startCol, endCol = p.CursorCol, p.VisualCol
	}
	switch i {
	case ctx.selStart:
		return startCol, lineWidth
	case ctx.selEnd:
		return 0, endCol + 1
	default:
		return 0, lineWidth // middle line — highlight everything
	}
}

// renderEventCursorLine renders the non-wrap cursor line with gutter
// indicator and block cursor at the configured column.
func renderEventCursorLine(p EventViewerParams, fitLine string, ctx eventLineContext, gutter string) string {
	displayLine := fitLine
	if p.SearchQuery != "" {
		displayLine = highlightEventSearchLine(displayLine, ctx.lowerQuery)
	}
	return gutter + RenderCursorAtCol(displayLine, fitLine, p.CursorCol)
}

// renderEventNormalLine renders a non-wrap, non-cursor, non-selected line.
func renderEventNormalLine(p EventViewerParams, fitLine string, ctx eventLineContext, gutter string) string {
	displayLine := fitLine
	if p.SearchQuery != "" {
		displayLine = highlightEventSearchLine(displayLine, ctx.lowerQuery)
	} else {
		displayLine = OverlayNormalStyle.Render(displayLine)
	}
	return gutter + displayLine
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
