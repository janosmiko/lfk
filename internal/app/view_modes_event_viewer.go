package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) viewEventViewer() string {
	titleText := "Event Timeline"
	if m.actionCtx.name != "" {
		titleText += " - " + m.actionCtx.name
	}
	titleText += viewModeIndicators(m.eventTimelineWrap, rune(m.eventTimelineVisualMode), m.eventTimelineSearchQuery)
	title := ui.TitleStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(titleText)

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
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.eventTimelineSearchInput.CursorLeft()) + ui.BarDimStyle.Render("█") + ui.BarNormalStyle.Render(m.eventTimelineSearchInput.CursorRight())
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
		{Key: "/", Desc: "search"},
		{Key: ">", Desc: "wrap"},
		{Key: "f", Desc: "minimize"},
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
		return m.renderEventViewerLinesWrapped(lines, scroll, maxLines, lineContentWidth)
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

func (m Model) renderEventViewerLinesWrapped(lines []string, scroll, maxLines, lineContentWidth int) []string {
	wrapStyle := lipgloss.NewStyle().Width(lineContentWidth)
	var visible []string
	for i := scroll; i < len(lines) && len(visible) < maxLines; i++ {
		isCursor := i == m.eventTimelineCursor
		wrapped := wrapStyle.Render(lines[i])
		subLines := strings.Split(wrapped, "\n")
		for si, sub := range subLines {
			if len(visible) >= maxLines {
				break
			}
			if isCursor && si == 0 {
				visible = append(visible, ui.YamlCursorIndicatorStyle.Render("▎")+sub)
			} else {
				visible = append(visible, " "+sub)
			}
		}
	}
	return visible
}
