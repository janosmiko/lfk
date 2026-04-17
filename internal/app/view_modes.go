package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hinshun/vt10x"

	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) viewLogs() string {
	viewH := m.logViewHeight()
	canSwitchPod := m.logParentKind != ""
	canFilterContainers := m.actionCtx.kind == "Pod"
	var statusMsg string
	var statusIsErr bool
	if m.hasStatusMessage() {
		statusMsg = m.statusMessage
		statusIsErr = m.statusMessageErr
	}
	// Surface the active severity floor (if any) so the title bar can
	// render a distinct chip — users set via `>` / `<` often lose track
	// of whether a floor is active unless it's called out separately.
	severityFloor := ""
	for _, r := range m.logRules {
		if sev, ok := r.(SeverityRule); ok {
			severityFloor = sev.Floor.String()
			break
		}
	}
	return ui.RenderLogViewer(m.logLines, m.logVisibleIndices, m.logScroll, m.width, viewH, m.logFollow, m.logWrap, m.logLineNumbers, m.logTimestamps, m.logPrevious, m.logHidePrefixes, m.logTitle, m.logSearchQuery, m.logSearchInput.Value, m.logSearchActive, canSwitchPod, canFilterContainers, m.logHasMoreHistory, m.logLoadingHistory, statusMsg, statusIsErr, m.logCursor, m.logVisualMode, m.logVisualStart, m.logVisualType, m.logVisualCol, m.logVisualCurCol, len(m.logRules), severityFloor, m.logSinceDuration, m.logRelativeTimestamps)
}

func (m Model) viewDescribe() string {
	titleText := m.describeTitle + viewModeIndicators(m.describeWrap, rune(m.describeVisualMode), m.describeSearchQuery)
	title := ui.TitleStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(titleText)

	// Build hint bar or search bar.
	hints := []ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: "h/l", Desc: "column"},
		{Key: "v/V", Desc: "visual"},
		{Key: "y", Desc: "copy"},
		{Key: "/", Desc: "search"},
		{Key: "ctrl+w/>", Desc: "wrap"},
		{Key: "q/esc", Desc: "back"},
	}
	if m.describeAutoRefresh {
		hints = append(hints, ui.HintEntry{Key: "LIVE", Desc: "auto-refresh"})
	}
	var hint string
	if m.describeSearchActive {
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.describeSearchInput.CursorLeft()) + ui.BarDimStyle.Render("\u2588") + ui.BarNormalStyle.Render(m.describeSearchInput.CursorRight())
		hint = ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(searchBar)
	} else if m.describeSearchQuery != "" {
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.describeSearchQuery)
		hint = ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(searchBar)
	} else {
		hint = ui.RenderHintBar(hints, m.width)
	}

	lines := strings.Split(m.describeContent, "\n")

	maxLines := m.height - 4
	if maxLines < 3 {
		maxLines = 3
	}

	// Content width for wrapping/truncation.
	// Account for cursor gutter (1 char).
	contentWidth := m.width - 4 // border (2) + padding (2)
	if contentWidth < 10 {
		contentWidth = 10
	}
	lineContentWidth := contentWidth - 1 // -1 for cursor gutter
	if lineContentWidth < 10 {
		lineContentWidth = 10
	}

	scroll := m.describeScroll
	if scroll > len(lines) {
		scroll = len(lines) - 1
	}
	if scroll < 0 {
		scroll = 0
	}

	// Visual selection range.
	selStart := min(m.describeVisualStart, m.describeCursor)
	selEnd := max(m.describeVisualStart, m.describeCursor)
	colStart := min(m.describeVisualCol, m.describeCursorCol)
	colEnd := max(m.describeVisualCol, m.describeCursorCol)

	// Search query for highlighting.
	lowerQuery := strings.ToLower(m.describeSearchQuery)

	// When wrap is enabled, use a simplified rendering path (no column cursor).
	if m.describeWrap {
		visible := lines[scroll:]
		if len(visible) > maxLines {
			visible = visible[:maxLines]
		}
		var expanded []string
		for _, line := range visible {
			subLines := ui.WrapLine(line, lineContentWidth)
			for _, sub := range subLines {
				expanded = append(expanded, " "+sub)
				if len(expanded) >= maxLines {
					break
				}
			}
			if len(expanded) >= maxLines {
				break
			}
		}
		for len(expanded) < maxLines {
			expanded = append(expanded, "")
		}

		bodyContent := strings.Join(expanded, "\n")
		borderStyle := ui.FullscreenBorderStyle(m.width, maxLines)
		body := borderStyle.Render(bodyContent)
		return lipgloss.JoinVertical(lipgloss.Left, title, body, hint)
	}

	// Non-wrap rendering with cursor, visual selection, and search.
	end := min(scroll+maxLines, len(lines))
	var renderedLines []string

	for i := scroll; i < end; i++ {
		line := lines[i]
		inSelection := m.describeVisualMode != 0 && i >= selStart && i <= selEnd
		isCursorLine := i == m.describeCursor

		// Truncate to content width.
		plainLine := line
		if len([]rune(plainLine)) > lineContentWidth {
			plainLine = string([]rune(plainLine)[:lineContentWidth])
		}

		if inSelection {
			rendered := ui.RenderVisualSelection(
				plainLine, rune(m.describeVisualMode),
				i, selStart, selEnd,
				m.describeVisualStart, m.describeVisualCol, m.describeCursorCol,
				colStart, colEnd,
			)
			if isCursorLine {
				renderedLines = append(renderedLines, ui.YamlCursorIndicatorStyle.Render("\u258e")+rendered)
			} else {
				renderedLines = append(renderedLines, " "+rendered)
			}
		} else if isCursorLine {
			displayLine := plainLine
			if lowerQuery != "" {
				displayLine = highlightDescribeSearchLine(plainLine, lowerQuery)
			}
			cursorLine := ui.RenderCursorAtCol(displayLine, plainLine, m.describeCursorCol)
			renderedLines = append(renderedLines, ui.YamlCursorIndicatorStyle.Render("\u258e")+cursorLine)
		} else {
			displayLine := plainLine
			if lowerQuery != "" {
				displayLine = highlightDescribeSearchLine(plainLine, lowerQuery)
			}
			renderedLines = append(renderedLines, " "+displayLine)
		}
	}

	// Pad to fill available height.
	for len(renderedLines) < maxLines {
		renderedLines = append(renderedLines, "")
	}

	bodyContent := strings.Join(renderedLines, "\n")
	borderStyle := ui.FullscreenBorderStyle(m.width, maxLines)
	body := borderStyle.Render(bodyContent)

	return lipgloss.JoinVertical(lipgloss.Left, title, body, hint)
}

// highlightDescribeSearchLine highlights search matches in a single line of
// the describe view. The query should be pre-lowered for case-insensitive matching.
func highlightDescribeSearchLine(line, lowerQuery string) string {
	if lowerQuery == "" {
		return line
	}
	lowerLine := strings.ToLower(line)
	matchStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ui.ColorSelectedFg)).
		Background(lipgloss.Color(ui.ColorWarning)).
		Bold(true)

	var result strings.Builder
	pos := 0
	for pos < len(line) {
		idx := strings.Index(lowerLine[pos:], lowerQuery)
		if idx < 0 {
			result.WriteString(line[pos:])
			break
		}
		if idx > 0 {
			result.WriteString(line[pos : pos+idx])
		}
		matchEnd := pos + idx + len(lowerQuery)
		if matchEnd > len(line) {
			matchEnd = len(line)
		}
		result.WriteString(matchStyle.Render(line[pos+idx : matchEnd]))
		pos = matchEnd
	}
	return result.String()
}

func (m Model) viewExplain() string {
	searchQuery := m.explainSearchQuery
	if m.explainSearchActive {
		searchQuery = m.explainSearchInput.Value
	}

	// Build hint bar (default key hints).
	hint := ui.RenderHintBar([]ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: "l/Enter", Desc: "drill in"},
		{Key: "h/Backspace", Desc: "back"},
		{Key: "/", Desc: "search"},
		{Key: "n/N", Desc: "next/prev match"},
		{Key: "q", Desc: "close"},
		{Key: "Esc", Desc: "back/close"},
	}, m.width)

	// If search is active, show search bar instead of hints.
	if m.explainSearchActive {
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.explainSearchInput.CursorLeft()) + ui.BarDimStyle.Render("\u2588") + ui.BarNormalStyle.Render(m.explainSearchInput.CursorRight())
		hint = ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(searchBar)
	} else if m.explainSearchQuery != "" {
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.explainSearchQuery)
		hint = ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(searchBar)
	}

	return ui.RenderExplainView(
		m.explainFields,
		m.explainCursor,
		m.explainScroll,
		m.explainDesc,
		m.explainTitle,
		m.explainPath,
		searchQuery,
		hint,
		m.width,
		m.height,
	)
}

func (m Model) viewDiff() string {
	foldRegions := ui.ComputeDiffFoldRegions(m.diffLeft, m.diffRight)
	searchInput := m.diffSearchText.Value
	vp := ui.DiffVisualParams{
		CursorSide:  m.diffCursorSide,
		CursorCol:   m.diffVisualCurCol,
		VisualMode:  m.diffVisualMode,
		VisualType:  m.diffVisualType,
		VisualStart: m.diffVisualStart,
		VisualCol:   m.diffVisualCol,
	}
	if m.diffUnified {
		return ui.RenderUnifiedDiffView(m.diffLeft, m.diffRight, m.diffLeftName, m.diffRightName, m.diffScroll, m.width, m.height, m.diffLineNumbers, m.diffWrap, m.diffSearchQuery, foldRegions, m.diffFoldState, m.diffSearchMode, searchInput, m.diffCursor, vp)
	}
	return ui.RenderDiffView(m.diffLeft, m.diffRight, m.diffLeftName, m.diffRightName, m.diffScroll, m.width, m.height, m.diffLineNumbers, m.diffWrap, m.diffSearchQuery, foldRegions, m.diffFoldState, m.diffSearchMode, searchInput, m.diffCursor, vp)
}

func (m Model) logViewHeight() int {
	h := m.height - 2 // title + footer (border is handled inside RenderLogViewer)
	if h < 3 {
		h = 3
	}
	return h
}

// logContentHeight returns the number of visible log content lines accounting
// for ALL overhead: app title bar (1), optional tab bar (1), log viewer
// title (1), border top+bottom (2), and footer (1). This is used by scroll
// calculations in Update() where m.height is the original terminal height.
func (m *Model) logContentHeight() int {
	h := m.height - 5 // app title(1) + log title(1) + border(2) + footer(1)
	if len(m.tabs) > 1 {
		h-- // tab bar
	}
	if h < 1 {
		h = 1
	}
	return h
}

func (m *Model) clampLogScroll() {
	viewH := m.logContentHeight()
	if viewH < 1 {
		viewH = 1
	}

	// When filter is active, logScroll is a visible-coordinate — it indexes
	// into logVisibleIndices, NOT logLines. The buffer walked for maxScroll
	// must be the projected slice so logScroll can't exceed the visible set.
	filterActive := m.logFilterChain != nil && m.logFilterChain.Active()
	bufLen := len(m.logLines)
	if filterActive {
		bufLen = len(m.logVisibleIndices)
	}

	var maxScroll int
	if m.logWrap {
		// When wrapping, each source line may produce multiple visual lines.
		// Walk backward from the end (of the projected buffer when filter is
		// active), accumulating visual lines until we exceed viewH.
		contentWidth := m.width - 4 // match logviewer.go contentWidth calculation
		if contentWidth < 10 {
			contentWidth = 10
		}
		// Account for cursor gutter (1) and line number gutter width.
		lineNumWidth := 0
		if m.logLineNumbers && bufLen > 0 {
			lineNumWidth = len(fmt.Sprintf("%d", bufLen)) + 1
		}
		availWidth := contentWidth - 1 - lineNumWidth // -1 for cursor gutter
		if availWidth < 10 {
			availWidth = 10
		}

		visualLines := 0
		maxScroll = bufLen // default: can scroll to end
		for i := bufLen - 1; i >= 0; i-- {
			line := m.lineAtBufferIndex(i, filterActive)
			visualLines += wrappedLineCount(line, availWidth)
			if visualLines >= viewH {
				maxScroll = i
				break
			}
		}
	} else {
		maxScroll = bufLen - viewH
	}
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.logScroll > maxScroll {
		m.logScroll = maxScroll
	}
	if m.logScroll < 0 {
		m.logScroll = 0
	}
}

// lineAtBufferIndex returns the text of the line at position i in the
// active buffer (projected through logVisibleIndices when filter is active,
// raw logLines otherwise). Out-of-range returns "".
func (m *Model) lineAtBufferIndex(i int, filterActive bool) string {
	if filterActive {
		if i < 0 || i >= len(m.logVisibleIndices) {
			return ""
		}
		src := m.logVisibleIndices[i]
		if src < 0 || src >= len(m.logLines) {
			return ""
		}
		return m.logLines[src]
	}
	if i < 0 || i >= len(m.logLines) {
		return ""
	}
	return m.logLines[i]
}

// ensureLogCursorVisible adjusts logScroll so the cursor is within the visible
// content area with scrolloff margin.
func (m *Model) ensureLogCursorVisible() {
	if m.logCursor < 0 {
		return
	}
	// Clamp cursor against the visible buffer when filter is active
	// (logCursor is a visible-coordinate in that case).
	if max := m.logCursorMax(); max >= 0 && m.logCursor > max {
		m.logCursor = max
	}
	viewH := m.logContentHeight()
	if viewH < 1 {
		viewH = 1
	}
	so := ui.ConfigScrollOff
	if so > viewH/2 {
		so = viewH / 2
	}
	// Scroll up if cursor is above viewport (with scrolloff).
	if m.logCursor < m.logScroll+so {
		m.logScroll = m.logCursor - so
	}
	// Scroll down if cursor is below viewport (with scrolloff).
	if m.logCursor >= m.logScroll+viewH-so {
		m.logScroll = m.logCursor - viewH + so + 1
	}
	m.clampLogScroll()
}

// logMaxScroll returns the maximum valid scroll offset for the log viewer.
// It is wrap-aware when logWrap is enabled.
func (m *Model) logMaxScroll() int {
	viewH := m.logContentHeight()

	if m.logWrap {
		contentWidth := m.width - 4
		if contentWidth < 10 {
			contentWidth = 10
		}
		lineNumWidth := 0
		if m.logLineNumbers && len(m.logLines) > 0 {
			lineNumWidth = len(fmt.Sprintf("%d", len(m.logLines))) + 1
		}
		availWidth := contentWidth - 1 - lineNumWidth // -1 for cursor gutter
		if availWidth < 10 {
			availWidth = 10
		}

		visualLines := 0
		maxScroll := len(m.logLines)
		for i := len(m.logLines) - 1; i >= 0; i-- {
			visualLines += wrappedLineCount(m.logLines[i], availWidth)
			if visualLines >= viewH {
				maxScroll = i
				break
			}
		}
		if maxScroll < 0 {
			return 0
		}
		return maxScroll
	}

	ms := len(m.logLines) - viewH
	if ms < 0 {
		return 0
	}
	return ms
}

// wrappedLineCount returns how many visual lines a source line produces
// when wrapped at the given width.
func wrappedLineCount(line string, width int) int {
	if width <= 0 {
		return 1
	}
	n := len([]rune(line))
	if n == 0 {
		return 1
	}
	return (n + width - 1) / width
}

// viewExecTerminal renders the embedded PTY terminal view.
func (m Model) viewExecTerminal() string {
	title := ui.TitleStyle.Render(m.execTitle)

	// Render hints.
	hints := []struct{ key, desc string }{
		{"ctrl+] ctrl+]", "exit"},
		{"ctrl+] ]/[", "switch tab"},
		{"ctrl+] t", "new tab"},
	}
	hintParts := make([]string, 0, len(hints))
	for _, h := range hints {
		hintParts = append(hintParts, ui.HelpKeyStyle.Render(h.key)+" "+ui.BarDimStyle.Render(h.desc))
	}
	hintLine := "  " + strings.Join(hintParts, "  ")

	// Render terminal content. Overhead: exec title (1) + border top/bottom (2) + hint line (1) = 4.
	// The outer View() already subtracted the main title bar and tab bar from m.height.
	viewH := m.height - 4
	if viewH < 3 {
		viewH = 3
	}
	viewW := m.width - 4 // border left/right + padding
	if viewW < 10 {
		viewW = 10
	}

	var termContent string
	if m.execTerm != nil {
		m.execMu.Lock()
		cols, rows := m.execTerm.Size()
		cursor := m.execTerm.Cursor()

		// Calculate vertical scroll offset to keep the cursor visible.
		// When the terminal buffer is taller than the viewport, show the
		// portion that contains the cursor row.
		startY := 0
		renderH := rows
		if rows > viewH {
			renderH = viewH
			// Ensure cursor row is within the visible portion.
			startY = cursor.Y - viewH + 1
			if startY < 0 {
				startY = 0
			}
			if startY+viewH > rows {
				startY = rows - viewH
			}
		}

		var lines []string
		for y := startY; y < startY+renderH && y < rows; y++ {
			var line strings.Builder
			for x := 0; x < cols && x < viewW; x++ {
				g := m.execTerm.Cell(x, y)
				ch := g.Char
				if ch == 0 {
					ch = ' '
				}
				// Apply FG/BG colors.
				style := lipgloss.NewStyle()
				if g.FG != vt10x.DefaultFG {
					style = style.Foreground(vt10xColorToLipgloss(g.FG))
				}
				if g.BG != vt10x.DefaultBG {
					style = style.Background(vt10xColorToLipgloss(g.BG))
				}
				// Apply text attributes from glyph mode.
				if g.Mode&(1<<2) != 0 { // bold (attrBold = 4)
					style = style.Bold(true)
				}
				if g.Mode&(1<<1) != 0 { // underline (attrUnderline = 2)
					style = style.Underline(true)
				}
				if g.Mode&1 != 0 { // reverse (attrReverse = 1)
					style = style.Reverse(true)
				}
				line.WriteString(style.Render(string(ch)))
			}
			lines = append(lines, line.String())
		}
		m.execMu.Unlock()
		termContent = strings.Join(lines, "\n")
	} else {
		termContent = ui.DimStyle.Render("Terminal not initialized")
	}

	if m.execDone != nil && m.execDone.Load() {
		termContent += "\n\n" + ui.DimStyle.Render("  Process exited. Press any key to return.")
	}

	// Wrap terminal content in a rounded border.
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorPrimary)).
		Padding(0, 0).
		Width(m.width - 2).
		Height(viewH)
	bordered := borderStyle.Render(termContent)

	return lipgloss.JoinVertical(lipgloss.Left, title, bordered, hintLine)
}

// vt10xColorToLipgloss converts a vt10x color to a lipgloss terminal color.
func vt10xColorToLipgloss(c vt10x.Color) lipgloss.TerminalColor {
	return lipgloss.Color(fmt.Sprintf("%d", int(c)))
}

// viewEventViewer renders the fullscreen event viewer mode.
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

// viewModeIndicators builds the bracket-enclosed mode indicators string.
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

// eventViewerHintBar returns the hint bar for the event viewer.
func (m Model) eventViewerHintBar() string {
	if m.eventTimelineSearchActive {
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.eventTimelineSearchInput.CursorLeft()) + ui.BarDimStyle.Render("\u2588") + ui.BarNormalStyle.Render(m.eventTimelineSearchInput.CursorRight())
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

// renderEventViewerLines renders the visible event lines.
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
				visible = append(visible, ui.YamlCursorIndicatorStyle.Render("\u258e")+rendered)
			} else {
				visible = append(visible, " "+rendered)
			}
		} else if isCursor {
			displayLine := truncLine
			if lowerQuery != "" {
				displayLine = highlightDescribeSearchLine(displayLine, lowerQuery)
			}
			visible = append(visible, ui.YamlCursorIndicatorStyle.Render("\u258e")+ui.RenderCursorAtCol(displayLine, truncLine, m.eventTimelineCursorCol))
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

// renderEventViewerLinesWrapped renders wrapped event viewer lines.
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
				visible = append(visible, ui.YamlCursorIndicatorStyle.Render("\u258e")+sub)
			} else {
				visible = append(visible, " "+sub)
			}
		}
	}
	return visible
}
