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

	logWidth := m.width
	previewWidth := 0
	if m.logPreviewVisible {
		logWidth, previewWidth = splitLogPreviewWidth(m.width)
	}

	logView := ui.RenderLogViewer(m.logLines, m.logScroll, logWidth, viewH, m.logFollow, m.logWrap, m.logLineNumbers, m.logTimestamps, m.logPrevious, m.logHidePrefixes, m.logTitle, m.logSearchQuery, m.logSearchInput.Value, m.logSearchActive, canSwitchPod, canFilterContainers, m.logHasMoreHistory, m.logLoadingHistory, statusMsg, statusIsErr, m.logCursor, m.logVisualMode, m.logVisualStart, m.logVisualType, m.logVisualCol, m.logVisualCurCol, m.logWrapTopSkip)

	if previewWidth > 0 {
		preview := ui.RenderLogPreviewPane(m.logPreviewLine(), previewWidth, viewH)
		return lipgloss.JoinHorizontal(lipgloss.Top, logView, preview)
	}
	return logView
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

	maxLines := max(m.height-4, 3)

	// Content width for wrapping/truncation.
	// Account for cursor gutter (1 char).
	contentWidth := max(
		// border (2) + padding (2)
		m.width-4, 10)
	lineContentWidth := max(
		// -1 for cursor gutter
		contentWidth-1, 10)

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
		matchEnd := min(pos+idx+len(lowerQuery), len(line))
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
	h := max(
		// title + footer (border is handled inside RenderLogViewer)
		m.height-2, 3)
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
	maxScroll := m.logMaxScroll()
	if m.logScroll > maxScroll {
		m.logScroll = maxScroll
	}
	if m.logScroll < 0 {
		m.logScroll = 0
	}
}

// ensureLogCursorVisible adjusts logScroll so the cursor is within the visible
// content area. In wrap mode the math accounts for visual rows (each source
// line may produce multiple wrapped sub-lines) so the cursor doesn't drift
// off-screen when intermediate lines wrap heavily; outside wrap mode it's
// classic source-line scrolloff. logFollow always wins: it snaps to the
// (maxScroll, topSkip) pair that pins the most recent sub-line to the bottom.
func (m *Model) ensureLogCursorVisible() {
	if m.logCursor < 0 {
		return
	}
	if len(m.logLines) > 0 && m.logCursor >= len(m.logLines) {
		m.logCursor = len(m.logLines) - 1
	}
	if m.logFollow {
		m.logScroll, m.logWrapTopSkip = m.logMaxScrollAndSkip()
		return
	}
	viewH := max(m.logContentHeight(), 1)
	so := min(ui.ConfigScrollOff, viewH/2)

	if m.logWrap {
		m.adjustLogScrollForCursorWrap(viewH)
		return
	}

	if m.logCursor < m.logScroll+so {
		m.logScroll = m.logCursor - so
	}
	if m.logCursor >= m.logScroll+viewH-so {
		m.logScroll = m.logCursor - viewH + so + 1
	}
	m.logWrapTopSkip = 0
	m.clampLogScroll()
}

// adjustLogScrollForCursorWrap positions logScroll/logWrapTopSkip so that
// the cursor's source line (and ideally all of its wrapped sub-lines) is
// inside the visible viewport, in visual-row terms. Pins cursor's bottom
// sub-line to the viewport's bottom row when scrolling down; cursor's first
// sub-line to row 0 when scrolling up. Drops scrolloff for simplicity.
func (m *Model) adjustLogScrollForCursorWrap(viewH int) {
	availWidth := m.logWrapAvailWidth()

	// Cursor above scroll → snap top.
	if m.logCursor < m.logScroll {
		m.logScroll = m.logCursor
		m.logWrapTopSkip = 0
		m.clampLogScroll()
		return
	}

	// Compute the visual row where cursor's first wrapped sub-line will
	// land given the current (logScroll, logWrapTopSkip).
	cursorTopRow := -m.logWrapTopSkip
	for i := m.logScroll; i < m.logCursor && i < len(m.logLines); i++ {
		cursorTopRow += wrappedLineCount(m.logDisplayLine(i), availWidth)
	}
	cursorWrap := wrappedLineCount(m.logDisplayLine(m.logCursor), availWidth)
	cursorBottomRow := cursorTopRow + cursorWrap - 1

	if cursorBottomRow < viewH {
		return // already visible
	}

	// Cursor's last sub-line is off the bottom. Place it at viewH-1 by
	// walking back from cursor and accumulating wrap counts until the
	// rows above sum to (viewH - cursorWrap).
	target := viewH - cursorWrap
	if target <= 0 {
		// Cursor itself wraps to more than viewH rows: show its first
		// viewH sub-lines so the cursor indicator (always on sub-line 0)
		// stays visible at the top.
		m.logScroll = m.logCursor
		m.logWrapTopSkip = 0
		m.clampLogScroll()
		return
	}
	accumulated := 0
	for i := m.logCursor - 1; i >= 0; i-- {
		wc := wrappedLineCount(m.logDisplayLine(i), availWidth)
		if accumulated+wc >= target {
			m.logScroll = i
			m.logWrapTopSkip = accumulated + wc - target
			m.clampLogScroll()
			return
		}
		accumulated += wc
	}
	m.logScroll = 0
	m.logWrapTopSkip = 0
}

// logMaxScroll returns the maximum valid scroll offset for the log viewer.
// It is wrap-aware when logWrap is enabled.
func (m *Model) logMaxScroll() int {
	maxScroll, _ := m.logMaxScrollAndSkip()
	return maxScroll
}

// logMaxScrollAndSkip returns the (maxScroll, topSkip) pair that pins the
// most recent log content to the bottom of the viewport in wrap mode. The
// sub-line skip is non-zero only when the chosen source line wraps to more
// rows than the renderer can show — in that case the renderer drops the
// first topSkip wrapped sub-lines so the tail of the line lands at the
// bottom row instead of falling off-screen. Outside wrap mode, topSkip is
// always 0 and the math collapses to the classic source-line offset.
func (m *Model) logMaxScrollAndSkip() (int, int) {
	viewH := max(m.logContentHeight(), 1)

	if m.logWrap {
		availWidth := m.logWrapAvailWidth()
		visualLines := 0
		// Default: everything fits, scroll stays at the top with no skip.
		maxScroll, topSkip := 0, 0
		for i := len(m.logLines) - 1; i >= 0; i-- {
			// Use the displayed line (timestamps and pod prefixes
			// stripped to match the renderer) so the wrap count reflects
			// what the viewer actually paints — otherwise the raw line
			// is longer than the rendered one and we'd overestimate
			// wraps, shrinking maxScroll and pushing the tail off the
			// bottom when following.
			visualLines += wrappedLineCount(m.logDisplayLine(i), availWidth)
			if visualLines >= viewH {
				maxScroll = i
				topSkip = visualLines - viewH
				break
			}
		}
		return maxScroll, topSkip
	}

	ms := len(m.logLines) - viewH
	if ms < 0 {
		return 0, 0
	}
	return ms, 0
}

// logWrapAvailWidth returns the per-line content width used when computing
// wrap counts. Mirrors the logviewer's renderWrappedLines availWidth math
// (border + padding + cursor gutter + line-number gutter). Uses the
// effective viewer width so wrap math stays correct when the preview pane
// is open and the log column has been narrowed.
func (m *Model) logWrapAvailWidth() int {
	contentWidth := max(m.logEffectiveWidth()-4, 10)
	lineNumWidth := 0
	if m.logLineNumbers && len(m.logLines) > 0 {
		lineNumWidth = len(fmt.Sprintf("%d", len(m.logLines))) + 1
	}
	return max(contentWidth-1-lineNumWidth, 10)
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
	viewH := max(m.height-4, 3)
	viewW := max(
		// border left/right + padding
		m.width-4, 10)

	var termContent string
	if m.execTerm != nil {
		m.execMu.Lock()
		cols, rows := m.execTerm.Size()
		cursor := m.execTerm.Cursor()
		cursorVisible := m.execTerm.CursorVisible()

		// Calculate vertical scroll offset to keep the cursor visible.
		// When the terminal buffer is taller than the viewport, show the
		// portion that contains the cursor row.
		startY := 0
		renderH := rows
		if rows > viewH {
			renderH = viewH
			// Ensure cursor row is within the visible portion.
			startY = max(cursor.Y-viewH+1, 0)
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
				// Cursor block: invert the cell at the cursor position
				// (flipping any existing reverse so the block stands out).
				if cursorVisible && x == cursor.X && y == cursor.Y {
					style = style.Reverse(g.Mode&1 == 0)
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
