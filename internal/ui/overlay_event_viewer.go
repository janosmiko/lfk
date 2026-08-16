package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

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
// with column tracking. The first sub-line uses the full contentW. Later
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
// width. Continuation lines are indented by hangingIndent so wrapped
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
// the whole row. Char-mode pre-computes the absolute column range on
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
	// Fullscreen suppresses the overlay-mode background style on plain
	// rows: in fullscreen mode the surrounding FullscreenBorderStyle
	// owns the body background, and applying OverlayNormalStyle here
	// would paint a different-colored rectangle behind text rows than
	// the empty padding rows around them.
	Fullscreen bool
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
// explicitly asked for the whole wrapped block to be highlighted. V
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
		if !styled && !opts.Fullscreen {
			// Apply overlay-normal styling so wrapped rows match the
			// surrounding (non-wrap) rows' fg/bg pair instead of
			// punching a transparent rectangle through the overlay.
			// Skipped in fullscreen: the FullscreenBorderStyle owns
			// the body background, and an explicit fg/bg here would
			// paint text rows a different color than empty padding
			// rows around them.
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
// range is clamped to the sub-line's bounds. The selection that
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
		// ResourceName is a cluster object name. The body lines (p.Lines)
		// are already sanitized upstream, but the title is built here from
		// the raw value.
		title += " - " + SanitizeTerminalText(p.ResourceName)
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

	// Resolve the start index so the cursor entry is visible under physical-line
	// pagination. The key handler tracks scroll in logical-entry units and is
	// wrap-unaware. Without this a wrapped entry above the cursor can push it
	// (and the footer's line number) off the viewport.
	cursor := max(min(p.Cursor, len(p.Lines)-1), 0)
	scroll := eventStartForCursor(p, evLineCtx, max(p.Scroll, 0), cursor, maxVisible)
	// Pagination is by physical lines, not logical events: in wrap
	// mode one event expands to multiple sub-lines, and counting by
	// logical events would emit far more physical rows than
	// maxVisible — pushing the footer and the overlay's bottom
	// border off the viewport. Stop emitting as soon as we fill the
	// reserved height. `end` tracks the last logical row actually
	// emitted (not scroll+maxVisible) so the footer's truncation
	// indicator is correct when wrapped rows fill the budget early.
	end := scroll
	var physicalLines []string
	for i := scroll; i < len(p.Lines) && len(physicalLines) < maxVisible; i++ {
		before := len(physicalLines)
		rendered := renderEventViewerLine(p, i, evLineCtx)
		for sub := range strings.SplitSeq(rendered, "\n") {
			if len(physicalLines) >= maxVisible {
				break
			}
			physicalLines = append(physicalLines, sub)
		}
		if len(physicalLines) > before {
			end = i + 1
		}
	}
	for len(physicalLines) < maxVisible {
		physicalLines = append(physicalLines, "")
	}
	b.WriteString(strings.Join(physicalLines, "\n"))
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

// eventStartForCursor returns the scroll (start entry index) that keeps the
// cursor entry's first sub-line within maxVisible physical lines. It reconciles
// the wrap-unaware logical-entry scroll the key handler supplies with the
// renderer's physical-line pagination, mirroring errorLogStartForCursor. The
// incoming scroll is honoured when it already shows the cursor. Otherwise it is
// pulled down just far enough.
func eventStartForCursor(p EventViewerParams, ctx eventLineContext, scroll, cursor, maxVisible int) int {
	minStart := cursor
	used := 1 // the cursor entry's first sub-line
	for s := cursor - 1; s >= 0; s-- {
		used += eventRowPhysicalLines(p, s, ctx)
		if used > maxVisible {
			break
		}
		minStart = s
	}
	return max(min(scroll, cursor), minStart)
}

// eventRowPhysicalLines reports how many physical lines event row i renders to,
// counted from the exact same renderEventViewerLine the pagination loop emits so
// the two never disagree.
func eventRowPhysicalLines(p EventViewerParams, i int, ctx eventLineContext) int {
	return strings.Count(renderEventViewerLine(p, i, ctx), "\n") + 1
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
// from anchorCol to end-of-line. Cursor-only line highlights from 0 to
// cursorCol+1. Middle lines highlight everything (caller can detect
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
