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
	return ui.RenderLogViewer(m.logLines, m.logScroll, m.width, viewH, m.logFollow, m.logWrap, m.logLineNumbers, m.logTimestamps, m.logPrevious, m.logHidePrefixes, m.logTitle, m.logSearchQuery, m.logSearchInput.Value, m.logSearchActive, canSwitchPod, canFilterContainers, m.logHasMoreHistory, m.logLoadingHistory, statusMsg, statusIsErr, m.logCursor, m.logVisualMode, m.logVisualStart, m.logVisualType, m.logVisualCol, m.logVisualCurCol)
}

func (m Model) viewDescribe() string {
	title := ui.TitleStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(m.describeTitle)
	hints := []ui.HintEntry{
		{Key: "j/k", Desc: "scroll"},
		{Key: "g/G", Desc: "top/bottom"},
		{Key: "ctrl+d/u", Desc: "half page"},
		{Key: "ctrl+f/b", Desc: "page"},
		{Key: "q/esc", Desc: "back"},
	}
	if m.describeAutoRefresh {
		hints = append(hints, ui.HintEntry{Key: "LIVE", Desc: "auto-refresh"})
	}
	hint := ui.RenderHintBar(hints, m.width)

	lines := strings.Split(m.describeContent, "\n")

	maxLines := m.height - 4
	if maxLines < 3 {
		maxLines = 3
	}

	scroll := m.describeScroll
	if scroll > len(lines) {
		scroll = len(lines) - 1
	}
	if scroll < 0 {
		scroll = 0
	}
	visible := lines[scroll:]
	if len(visible) > maxLines {
		visible = visible[:maxLines]
	}

	// Pad to fill available height so the hint bar stays at the bottom.
	for len(visible) < maxLines {
		visible = append(visible, "")
	}

	bodyContent := strings.Join(visible, "\n")
	borderStyle := ui.FullscreenBorderStyle(m.width, maxLines)
	body := borderStyle.Render(bodyContent)

	return lipgloss.JoinVertical(lipgloss.Left, title, body, hint)
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
	if m.diffUnified {
		return ui.RenderUnifiedDiffView(m.diffLeft, m.diffRight, m.diffLeftName, m.diffRightName, m.diffScroll, m.width, m.height, m.diffLineNumbers, m.diffSearchQuery, foldRegions, m.diffFoldState, m.diffSearchMode, searchInput, m.diffCursor)
	}
	return ui.RenderDiffView(m.diffLeft, m.diffRight, m.diffLeftName, m.diffRightName, m.diffScroll, m.width, m.height, m.diffLineNumbers, m.diffSearchQuery, foldRegions, m.diffFoldState, m.diffSearchMode, searchInput, m.diffCursor)
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

	var maxScroll int
	if m.logWrap {
		// When wrapping, each source line may produce multiple visual lines.
		// Walk backward from the end, accumulating visual lines until we
		// exceed viewH. The first source line that pushes us over is the
		// maximum scroll position.
		contentWidth := m.width - 4 // match logviewer.go contentWidth calculation
		if contentWidth < 10 {
			contentWidth = 10
		}
		// Account for cursor gutter (1) and line number gutter width.
		lineNumWidth := 0
		if m.logLineNumbers && len(m.logLines) > 0 {
			lineNumWidth = len(fmt.Sprintf("%d", len(m.logLines))) + 1
		}
		availWidth := contentWidth - 1 - lineNumWidth // -1 for cursor gutter
		if availWidth < 10 {
			availWidth = 10
		}

		visualLines := 0
		maxScroll = len(m.logLines) // default: can scroll to end
		for i := len(m.logLines) - 1; i >= 0; i-- {
			visualLines += wrappedLineCount(m.logLines[i], availWidth)
			if visualLines >= viewH {
				maxScroll = i
				break
			}
		}
	} else {
		maxScroll = len(m.logLines) - viewH
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

// ensureLogCursorVisible adjusts logScroll so the cursor is within the visible content area.
func (m *Model) ensureLogCursorVisible() {
	if m.logCursor < 0 {
		return
	}
	if m.logCursor < 0 {
		m.logCursor = 0
	}
	if len(m.logLines) > 0 && m.logCursor >= len(m.logLines) {
		m.logCursor = len(m.logLines) - 1
	}
	viewH := m.logContentHeight()
	if viewH < 1 {
		viewH = 1
	}
	// Scroll up if cursor is above viewport.
	if m.logCursor < m.logScroll {
		m.logScroll = m.logCursor
	}
	// Scroll down if cursor is below viewport.
	if m.logCursor >= m.logScroll+viewH {
		m.logScroll = m.logCursor - viewH + 1
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
