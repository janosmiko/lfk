package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/ui"
)

// dashboardScrollableLines returns the number of content lines the fullscreen
// dashboard can scroll through for the active mode, before viewport padding.
// Mirrors what viewExplorerDashboard actually renders so the scroll clamp
// can't run past the last drawn line.
func (m Model) dashboardScrollableLines(fullW int) int {
	sel := m.selectedMiddleItem()
	if sel != nil && sel.Extra == "__monitoring__" {
		if m.monitoringPreview == "" {
			return 0
		}
		return strings.Count(m.monitoringPreview, "\n") + 1
	}
	if m.dashboardPreview == "" {
		return 0
	}
	if m.dashboardEventsPreview == "" {
		return strings.Count(m.dashboardPreview, "\n") + 1
	}
	// Two-column: the scroll bound is the taller of the two columns, since
	// both scroll together by the same offset.
	leftContent := m.dashboardPreview
	if idx := strings.Index(leftContent, "RECENT WARNING EVENTS"); idx > 0 {
		if lineStart := strings.LastIndex(leftContent[:idx], "\n"); lineStart > 0 {
			leftContent = leftContent[:lineStart]
		}
	}
	leftN := strings.Count(leftContent, "\n") + 1
	_, rightW := dashboardColumnWidths(m.dashboardPreview, fullW)
	rightN := len(wrapEventsColumn(strings.Split(m.dashboardEventsPreview, "\n"), rightW))
	return max(leftN, rightN)
}

// clampDashboardScroll bounds previewScroll to the fullscreen dashboard's
// content so the user can't scroll into blank space past the last line. The
// viewport height mirrors the contentHeight computed in viewExplorer (the
// command-bar dropdown is never open while scrolling, so it isn't subtracted).
func (m *Model) clampDashboardScroll() {
	total := m.dashboardScrollableLines(m.width - 2)
	vp := max(m.height-4, 3)
	if len(m.tabs) > 1 {
		vp-- // tab bar
	}
	maxScroll := max(total-vp, 0)
	if m.previewScroll > maxScroll {
		m.previewScroll = maxScroll
	}
	if m.previewScroll < 0 {
		m.previewScroll = 0
	}
}

// viewExplorerDashboard renders the fullscreen dashboard view.
func (m Model) viewExplorerDashboard(contentHeight int) string {
	sel := m.selectedMiddleItem()
	isMonitoring := sel != nil && sel.Extra == "__monitoring__"

	var dashContent string
	if isMonitoring {
		dashContent = m.monitoringPreview
		if dashContent == "" {
			dashContent = ui.DimStyle.Render(m.spinner.View() + " Loading monitoring dashboard...")
		}
	} else {
		dashContent = m.dashboardPreview
		if dashContent == "" {
			dashContent = ui.DimStyle.Render(m.spinner.View() + " Loading cluster dashboard...")
		}
	}

	fullW := m.width - 2
	if !isMonitoring && m.dashboardEventsPreview != "" {
		return m.viewExplorerDashboardTwoCol(dashContent, fullW, contentHeight)
	}
	return m.viewExplorerDashboardSingleCol(dashContent, fullW, contentHeight)
}

// viewExplorerColumns picks which view fills the explorer's columns slot.
// Fullscreen variants (error log, dashboard, single middle column) take
// precedence over the three-column layout. Extracted so viewExplorer
// itself stays under the gocyclo cap.
func (m Model) viewExplorerColumns(middle string, leftW, leftInner, rightW, rightInner, contentHeight int) string {
	switch {
	case m.overlayErrorLog && m.errorLogFullscreen:
		return m.viewErrorLogFullscreen(contentHeight)
	case m.fullscreenDashboard:
		return m.viewExplorerDashboard(contentHeight)
	case m.fullscreenMiddle:
		return middle
	default:
		return m.viewExplorerThreeCol(middle, leftW, leftInner, rightW, rightInner, contentHeight)
	}
}

// viewErrorLogFullscreen renders the in-app error log as the columns slot
// of viewExplorer when the user has fullscreened it. This reuses the same
// fullscreen pattern as viewExplorerDashboard so the surrounding title
// bar, tab bar, and status bar (with the overlayErrorLog-specific hints)
// stay consistent — instead of doing custom slice-and-rebuild that would
// drop background fills and break global keys like the theme selector.
func (m Model) viewErrorLogFullscreen(contentHeight int) string {
	vp := ui.ErrorLogVisualParams{
		VisualMode:     m.errorLogVisualMode,
		VisualStart:    m.errorLogVisualStart,
		VisualStartCol: m.errorLogVisualStartCol,
		CursorLine:     m.errorLogCursorLine,
		CursorCol:      m.errorLogCursorCol,
	}
	fullW := m.width - 2
	innerW := max(fullW-2, 1) // minus column padding
	content := ui.RenderErrorLogOverlay(m.errorLog, m.errorLogScroll, innerW, contentHeight, m.showDebugLogs, vp)
	content = clampErrorLogLines(content, innerW, contentHeight)
	content = ui.PadToHeight(content, contentHeight)
	content = ui.FillLinesBg(content, innerW, ui.BaseBg)
	// Apply BaseBg to the column wrapper too so the 1-char padding lipgloss
	// adds inside the rounded border doesn't render with the terminal's
	// default background — that's the "background looks different" gap
	// users see between the inner BaseBg-filled content and the border.
	style := ui.ActiveColumnStyle.
		Width(fullW).
		Height(contentHeight).
		MaxHeight(contentHeight + 2).
		Background(ui.BaseBg).
		BorderBackground(ui.BaseBg)
	return style.Render(content)
}

// viewExplorerDashboardSingleCol renders a single-column fullscreen dashboard.
func (m Model) viewExplorerDashboardSingleCol(dashContent string, fullW, contentHeight int) string {
	if m.previewScroll > 0 {
		lines := strings.Split(dashContent, "\n")
		if m.previewScroll >= len(lines) {
			m.previewScroll = len(lines) - 1
		}
		if m.previewScroll > 0 {
			lines = lines[m.previewScroll:]
		}
		dashContent = strings.Join(lines, "\n")
	}
	dashCol := ui.PadToHeight(dashContent, contentHeight)
	dashCol = ui.FillLinesBg(dashCol, m.width-4, ui.BaseBg)
	return ui.ActiveColumnStyle.Width(fullW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(dashCol)
}

// dashboardColumnWidths computes the left/right column widths for the
// two-column fullscreen dashboard. Allocates within the wrapper's *content*
// area (fullW-2): ActiveColumnStyle adds Padding(0,1), so building rows to the
// full fullW overflows the content box by 2 columns and lipgloss wraps every
// row. Shared by the renderer and clampDashboardScroll so the scroll bound
// matches what's actually drawn.
func dashboardColumnWidths(dashContent string, fullW int) (leftW, rightW int) {
	innerW := fullW - 2
	leftW = innerW / 2
	for line := range strings.SplitSeq(dashContent, "\n") {
		if w := lipgloss.Width(line); w+2 > leftW {
			leftW = w + 2
		}
	}
	if maxLeft := innerW * 60 / 100; leftW > maxLeft {
		leftW = maxLeft
	}
	rightW = innerW - leftW - 1
	return leftW, rightW
}

// viewExplorerDashboardTwoCol renders a two-column fullscreen dashboard.
func (m Model) viewExplorerDashboardTwoCol(dashContent string, fullW, contentHeight int) string {
	leftW, rightW := dashboardColumnWidths(dashContent, fullW)

	leftContent := dashContent
	if idx := strings.Index(leftContent, "RECENT WARNING EVENTS"); idx > 0 {
		lineStart := strings.LastIndex(leftContent[:idx], "\n")
		if lineStart > 0 {
			leftContent = leftContent[:lineStart]
		}
	}
	rightContent := m.dashboardEventsPreview

	if m.previewScroll > 0 {
		leftContent = scrollContent(leftContent, m.previewScroll)
		rightContent = scrollContent(rightContent, m.previewScroll)
	}

	leftLines := strings.Split(leftContent, "\n")
	rightLines := wrapEventsColumn(strings.Split(rightContent, "\n"), rightW)

	for len(leftLines) < contentHeight {
		leftLines = append(leftLines, "")
	}
	for len(rightLines) < contentHeight {
		rightLines = append(rightLines, "")
	}

	leftStyle := lipgloss.NewStyle().Width(leftW).MaxWidth(leftW)
	rightStyle := lipgloss.NewStyle().Width(rightW).MaxWidth(rightW)
	sep := ui.DimStyle.Render("\u2502")
	rows := make([]string, contentHeight)
	for i := range contentHeight {
		l, r := "", ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}
		rows[i] = leftStyle.Render(l) + sep + rightStyle.Render(r)
	}
	dashCol := strings.Join(rows, "\n")
	// Re-apply the theme background after every ANSI reset and pad each row to
	// full width, mirroring the single-col path. Without this the per-column
	// lipgloss padding and the gaps after styled spans render with the
	// terminal's default background, which "tears" black rectangles into the
	// dashboard under themes whose base colour isn't black.
	dashCol = ui.FillLinesBg(dashCol, leftW+1+rightW, ui.BaseBg)
	return ui.ActiveColumnStyle.Width(fullW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(dashCol)
}

// scrollContent applies scroll offset to a newline-separated content string.
func scrollContent(content string, scroll int) string {
	lines := strings.Split(content, "\n")
	if scroll >= len(lines) {
		return ""
	}
	if scroll > 0 {
		lines = lines[scroll:]
	}
	return strings.Join(lines, "\n")
}

// wrapEventsColumn word-wraps event lines to fit the right column width.
func wrapEventsColumn(rawLines []string, rightW int) []string {
	pad := "  "
	maxContentW := max(rightW-4, 10)
	wrapStyle := lipgloss.NewStyle().Width(maxContentW)
	result := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		if lipgloss.Width(line) == 0 {
			result = append(result, "")
		} else if lipgloss.Width(line) <= maxContentW {
			result = append(result, pad+line)
		} else {
			wrapped := wrapStyle.Render(line)
			for wl := range strings.SplitSeq(wrapped, "\n") {
				result = append(result, pad+wl)
			}
		}
	}
	return result
}
