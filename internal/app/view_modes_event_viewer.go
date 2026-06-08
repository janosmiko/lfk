package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) viewEventViewer() string {
	titleText := "Event Timeline"
	if m.actionCtx.name != "" {
		titleText += ": " + resourceTitleLabel(m.actionCtx.kind, m.actionNamespace(), m.actionCtx.name)
	}
	titleText += viewModeIndicators(m.eventTimelineWrap, rune(m.eventTimelineVisualMode), m.eventTimelineSearchQuery)
	title := ui.ViewTitle(m.width, titleText)

	hint := m.eventViewerHintBar()

	lines := m.eventTimelineLines
	maxLines := max(m.height-4, 3)
	contentWidth := max(m.width-4, 10)
	lineContentWidth := max(contentWidth-1, 10)

	scroll := m.eventTimelineScroll
	if scroll > len(lines) {
		scroll = len(lines) - 1
	}
	if scroll < 0 {
		scroll = 0
	}

	visible := m.renderEventViewerLines(lines, scroll, maxLines, lineContentWidth)

	for len(visible) < maxLines {
		visible = append(visible, "")
	}

	bodyContent := strings.Join(visible, "\n")
	borderStyle := ui.FullscreenBorderStyle(m.width, maxLines)
	body := borderStyle.Render(bodyContent)

	return lipgloss.JoinVertical(lipgloss.Left, title, body, hint)
}

func viewModeIndicators(wrap bool, visualMode rune, searchQuery string) string {
	var indicators []string
	if wrap {
		indicators = append(indicators, "WRAP")
	}
	switch visualMode {
	case 'v':
		indicators = append(indicators, "VISUAL")
	case 'V':
		indicators = append(indicators, "VISUAL LINE")
	case 'B':
		indicators = append(indicators, "VISUAL BLOCK")
	}
	if searchQuery != "" {
		indicators = append(indicators, "/"+searchQuery)
	}
	if len(indicators) > 0 {
		return " [" + strings.Join(indicators, " | ") + "]"
	}
	return ""
}

func (m Model) eventViewerHintBar() string {
	if m.hasStatusMessage() {
		return m.renderStatusHint()
	}
	if m.eventTimelineSearchActive {
		searchBar := ui.HelpKeyStyle.Render(ui.ActiveKeybindings.Search) + ui.BarNormalStyle.Render(m.eventTimelineSearchInput.CursorLeft()) + ui.BarDimStyle.Render("█") + ui.BarNormalStyle.Render(m.eventTimelineSearchInput.CursorRight())
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(searchBar)
	}
	if m.eventTimelineVisualMode != 0 {
		return ui.RenderHintBar([]ui.HintEntry{
			{Key: "j/k", Desc: "extend"},
			{Key: "h/l", Desc: "column"},
			{Key: "y", Desc: "copy"},
			{Key: "v/V", Desc: "switch mode"},
			{Key: "esc", Desc: "cancel"},
		}, m.width)
	}
	return ui.RenderHintBar([]ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: "h/l", Desc: "column"},
		{Key: "v/V", Desc: "visual"},
		{Key: "y", Desc: "copy"},
		{Key: ui.ActiveKeybindings.Search, Desc: "search"},
		{Key: ui.ActiveKeybindings.ToggleWrap, Desc: "wrap"},
		{Key: ui.ActiveKeybindings.Fullscreen, Desc: "minimize"},
		{Key: "q/esc", Desc: "back"},
	}, m.width)
}

func (m Model) renderEventViewerLines(lines []string, scroll, maxLines, lineContentWidth int) []string {
	selStart := min(m.eventTimelineVisualStart, m.eventTimelineCursor)
	selEnd := max(m.eventTimelineVisualStart, m.eventTimelineCursor)
	colStart := min(m.eventTimelineVisualCol, m.eventTimelineCursorCol)
	colEnd := max(m.eventTimelineVisualCol, m.eventTimelineCursorCol)
	lowerQuery := strings.ToLower(m.eventTimelineSearchQuery)

	if m.eventTimelineWrap {
		return m.renderEventViewerLinesWrapped(lines, scroll, maxLines, lineContentWidth, selStart, selEnd, colStart, colEnd)
	}

	var visible []string
	end := min(scroll+maxLines, len(lines))
	for i := scroll; i < end; i++ {
		line := lines[i]
		truncLine := line
		if len([]rune(truncLine)) > lineContentWidth {
			truncLine = string([]rune(truncLine)[:lineContentWidth])
		}
		isCursor := i == m.eventTimelineCursor
		inSel := m.eventTimelineVisualMode != 0 && i >= selStart && i <= selEnd

		if inSel {
			rendered := ui.RenderVisualSelection(truncLine, rune(m.eventTimelineVisualMode), i, selStart, selEnd,
				m.eventTimelineVisualStart, m.eventTimelineVisualCol, m.eventTimelineCursorCol, colStart, colEnd)
			if isCursor {
				visible = append(visible, ui.YamlCursorIndicatorStyle.Render("▎")+rendered)
			} else {
				visible = append(visible, " "+rendered)
			}
		} else if isCursor {
			displayLine := truncLine
			if lowerQuery != "" {
				displayLine = highlightDescribeSearchLine(displayLine, lowerQuery)
			}
			visible = append(visible, ui.YamlCursorIndicatorStyle.Render("▎")+ui.RenderCursorAtCol(displayLine, truncLine, m.eventTimelineCursorCol))
		} else {
			displayLine := truncLine
			if lowerQuery != "" {
				displayLine = highlightDescribeSearchLine(displayLine, lowerQuery)
			}
			visible = append(visible, " "+displayLine)
		}
	}
	return visible
}

func (m Model) renderEventViewerLinesWrapped(lines []string, scroll, maxLines, lineContentWidth, selStart, selEnd, colStart, colEnd int) []string {
	lowerQuery := strings.ToLower(m.eventTimelineSearchQuery)

	var visible []string
	for i := scroll; i < len(lines) && len(visible) < maxLines; i++ {
		isCursor := i == m.eventTimelineCursor
		inSel := m.eventTimelineVisualMode != 0 && i >= selStart && i <= selEnd
		gutter := " "
		if isCursor {
			gutter = ui.YamlCursorIndicatorStyle.Render("▎")
		}

		// Block-mode (B) selection has no meaningful geometry over
		// wrapped sub-lines (rectangular column ranges across multiple
		// physical rows don't represent what the user picked). Fall
		// back to the single-line truncate path for that case only.
		if inSel && m.eventTimelineVisualMode == 'B' {
			truncLine := lines[i]
			if len([]rune(truncLine)) > lineContentWidth {
				truncLine = string([]rune(truncLine)[:lineContentWidth])
			}
			rendered := ui.RenderVisualSelection(truncLine, rune(m.eventTimelineVisualMode), i, selStart, selEnd,
				m.eventTimelineVisualStart, m.eventTimelineVisualCol, m.eventTimelineCursorCol, colStart, colEnd)
			visible = append(visible, gutter+rendered)
			continue
		}

		// Unified wrap rendering: per-sub-line gutter, V-mode full
		// highlight, v-mode char highlight mapped across sub-lines,
		// block cursor placed on the sub-line containing CursorCol so
		// it stays visible while navigating.
		opts := ui.WrappedEventRowOpts{
			Line:          lines[i],
			Gutter:        gutter,
			ContentW:      lineContentWidth,
			HangingIndent: eventTimelineMessageColumn,
			IsCursor:      isCursor,
			CursorCol:     m.eventTimelineCursorCol,
			Fullscreen:    true,
		}
		switch {
		case inSel && m.eventTimelineVisualMode == 'V':
			opts.SelectionLine = true
		case inSel && m.eventTimelineVisualMode == 'v':
			opts.SelStart, opts.SelEnd = m.charSelectionRangeForLine(lines[i], i, selStart, selEnd)
		default:
			opts.LowerSearch = lowerQuery
		}
		block := ui.RenderWrappedEventRow(opts)
		for sub := range strings.SplitSeq(block, "\n") {
			if len(visible) >= maxLines {
				break
			}
			visible = append(visible, sub)
		}
	}
	return visible
}

// charSelectionRangeForLine mirrors the per-line logic of
// ui.renderCharSelection for the wrap renderer: anchor-only line
// highlights from anchorCol to end; cursor-only line highlights from
// 0 to cursorCol+1; middle lines highlight everything.
func (m Model) charSelectionRangeForLine(line string, i, selStart, selEnd int) (int, int) {
	lineWidth := len([]rune(line))
	if selStart == selEnd {
		return min(m.eventTimelineVisualCol, m.eventTimelineCursorCol),
			max(m.eventTimelineVisualCol, m.eventTimelineCursorCol) + 1
	}
	var startCol, endCol int
	if m.eventTimelineVisualStart <= m.eventTimelineCursor {
		startCol, endCol = m.eventTimelineVisualCol, m.eventTimelineCursorCol
	} else {
		startCol, endCol = m.eventTimelineCursorCol, m.eventTimelineVisualCol
	}
	switch i {
	case selStart:
		return startCol, lineWidth
	case selEnd:
		return 0, endCol + 1
	default:
		return 0, lineWidth
	}
}
