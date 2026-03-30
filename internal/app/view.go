package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// View renders the UI.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Render fullscreen modes (YAML, Logs, Describe, Diff, Exec, Explain) with title bar and tab bar.
	// Each view renders its own hint bar, so the main status bar is not shown.
	// Also render the fullscreen view as background when help is open from a fullscreen mode.
	renderMode := m.mode
	if m.mode == modeHelp {
		renderMode = m.helpPreviousMode
	}
	if renderMode == modeYAML || renderMode == modeLogs || renderMode == modeDescribe || renderMode == modeDiff || renderMode == modeExec || renderMode == modeExplain {
		// Save original height before reducing for title/tab bar — overlays
		// need the full terminal dimensions for correct sizing and placement.
		fullHeight := m.height

		title := ui.FillLinesBg(m.renderTitleBar(), m.width, ui.BarBg)
		m.height -= 1 // title bar

		var tabBar string
		if len(m.tabs) > 1 {
			tabBar = ui.RenderTabBar(m.tabLabels(), m.activeTab, m.width)
			m.height-- // tab bar takes one line
		}

		var content string
		switch renderMode {
		case modeYAML:
			content = m.viewYAML()
		case modeLogs:
			content = m.viewLogs()
		case modeDescribe:
			content = m.viewDescribe()
		case modeDiff:
			content = m.viewDiff()
		case modeExec:
			content = m.viewExecTerminal()
		case modeExplain:
			content = m.viewExplain()
		}

		var parts []string
		parts = append(parts, title)
		if tabBar != "" {
			parts = append(parts, tabBar)
		}
		parts = append(parts, content)
		view := lipgloss.JoinVertical(lipgloss.Left, parts...)

		// Render overlay on top if active (e.g. Can-I subject selector).
		if m.overlay != overlayNone {
			view = m.renderOverlay(view)
		}

		// Render help screen as overlay on top of the fullscreen view.
		// Use fullHeight-1 for the overlay so the bottom status bar line
		// remains visible below the overlay with the help search prompt.
		if m.mode == modeHelp {
			// Replace the last line of the view with the help status bar
			// so it's always visible below the overlay.
			helpStatus := m.statusBar()
			viewLines := strings.Split(view, "\n")
			if len(viewLines) > 0 {
				viewLines[len(viewLines)-1] = helpStatus
			}
			view = strings.Join(viewLines, "\n")

			overlay := ui.RenderHelpScreen(m.width, fullHeight-1, m.helpScroll, m.helpFilter.Value)
			view = ui.PlaceOverlay(m.width, fullHeight, overlay, view)
		}

		return view
	}

	view := m.viewExplorer()

	// Render overlay on top if active.
	if m.overlay != overlayNone {
		view = m.renderOverlay(view)
	}

	// Render error log overlay on top if active (independent of regular overlays).
	if m.overlayErrorLog {
		view = m.renderErrorLogOverlay(view)
	}

	// Render help screen as an overlay on top of the explorer view.
	// The status bar (bottom line) already renders the help search prompt,
	// so size the overlay to leave the bottom line uncovered.
	if m.mode == modeHelp {
		overlay := ui.RenderHelpScreen(m.width, m.height-1, m.helpScroll, m.helpFilter.Value)
		view = ui.PlaceOverlay(m.width, m.height, overlay, view)
	}

	return view
}

func (m Model) viewExplorer() string {
	// Set highlight query for search/filter term highlighting.
	ui.ActiveHighlightQuery = m.filterText
	if m.searchActive {
		ui.ActiveHighlightQuery = m.searchInput.Value
	}
	defer func() { ui.ActiveHighlightQuery = "" }()

	// Set secret values visibility for rendering.
	ui.ActiveShowSecretValues = m.showSecretValues

	// Set fullscreen mode for column visibility.
	ui.ActiveFullscreenMode = m.fullscreenMiddle

	// Set selection state for rendering.
	ui.ActiveSelectedItems = m.selectedItems
	defer func() { ui.ActiveSelectedItems = nil }()

	// Calculate column widths: left=12%, middle=51%, right=remainder (~37%).
	usable := m.width - 6 // 3 columns x 2 border chars
	var leftW, middleW, rightW int
	if m.fullscreenDashboard || m.fullscreenMiddle {
		leftW = 0
		rightW = 0
		middleW = m.width - 2 // single column with border
	} else {
		leftW = max(10, usable*12/100)
		middleW = max(10, usable*51/100)
		rightW = max(10, usable-leftW-middleW)
	}

	contentHeight := m.height - 5 // room for title(1) + column borders(2) + status(1) + border rounding(1)
	if contentHeight < 3 {
		contentHeight = 3
	}

	// Tab bar (only shown with 2+ tabs).
	var tabBar string
	if len(m.tabs) > 1 {
		tabBar = ui.RenderTabBar(m.tabLabels(), m.activeTab, m.width)
		contentHeight-- // tab bar takes one line
	}

	// Column padding is 1 on each side, so inner content width is 2 less.
	colPad := 2
	leftInner := leftW - colPad
	middleInner := middleW - colPad
	rightInner := rightW - colPad
	if leftInner < 5 {
		leftInner = 5
	}
	if middleInner < 5 {
		middleInner = 5
	}
	if rightInner < 5 {
		rightInner = 5
	}

	// Only show error in the middle column when there are no items (first load failure).
	// Otherwise errors are displayed in the status bar.
	var middleErrMsg string
	if m.err != nil && len(m.middleItems) == 0 {
		middleErrMsg = m.err.Error()
	}

	// Set collapsed state for rendering resource type categories.
	if m.nav.Level == model.LevelResourceTypes && !m.allGroupsExpanded {
		collapsed := make(map[string]bool)
		for _, item := range m.middleItems {
			if item.Category != "" && item.Category != m.expandedGroup {
				collapsed[item.Category] = true
			}
		}
		ui.ActiveCollapsedCategories = collapsed
		ui.ActiveCategoryCounts = m.categoryCounts()
	} else {
		ui.ActiveCollapsedCategories = nil
		ui.ActiveCategoryCounts = nil
	}

	// Build columns.
	middleHeader := m.middleColumnHeader()
	var middleCol string
	switch m.nav.Level {
	case model.LevelResources, model.LevelOwned, model.LevelContainers:
		middleCol = ui.RenderTable(middleHeader, m.visibleMiddleItems(), m.cursor(), middleInner, contentHeight, m.loading, m.spinner.View(), middleErrMsg)
	default:
		middleCol = ui.RenderColumn(middleHeader, m.visibleMiddleItems(), m.cursor(), middleInner, contentHeight, true, m.loading, m.spinner.View(), middleErrMsg)
	}
	middleCol = ui.PadToHeight(middleCol, contentHeight)
	middleCol = ui.FillLinesBg(middleCol, middleInner, ui.BaseBg)
	middle := ui.ActiveColumnStyle.Width(middleW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(middleCol)

	var columns string
	switch {
	case m.fullscreenDashboard:
		// Fullscreen dashboard: render cluster/monitoring dashboard using full width.
		var dashContent string
		sel := m.selectedMiddleItem()
		if sel != nil && sel.Extra == "__monitoring__" {
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
		// Apply preview scroll.
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
		columns = ui.ActiveColumnStyle.Width(m.width - 2).Height(contentHeight).MaxHeight(contentHeight + 2).Render(dashCol)
	case m.fullscreenMiddle:
		columns = middle
	default:
		leftCol := ui.RenderColumn(m.leftColumnHeader(), m.leftItems, m.parentIndex(), leftInner, contentHeight, false, m.loading, m.spinner.View(), "")
		// Clear highlight query for preview/right column — search only applies to the focus pane.
		savedHighlight := ui.ActiveHighlightQuery
		ui.ActiveHighlightQuery = ""
		// Disable vim-style scroll for right column (it has its own preview scroll).
		savedMiddleScroll := ui.ActiveMiddleScroll
		savedLeftScroll := ui.ActiveLeftScroll
		ui.ActiveMiddleScroll = -1
		ui.ActiveLeftScroll = -1
		rightCol := m.renderRightColumn(rightInner, contentHeight)
		ui.ActiveMiddleScroll = savedMiddleScroll
		ui.ActiveLeftScroll = savedLeftScroll
		ui.ActiveHighlightQuery = savedHighlight
		leftCol = ui.PadToHeight(leftCol, contentHeight)
		rightCol = ui.PadToHeight(rightCol, contentHeight)
		leftCol = ui.FillLinesBg(leftCol, leftInner, ui.BaseBg)
		rightCol = ui.FillLinesBg(rightCol, rightInner, ui.BaseBg)
		left := ui.InactiveColumnStyle.Width(leftW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(leftCol)
		right := ui.InactiveColumnStyle.Width(rightW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(rightCol)
		columns = lipgloss.JoinHorizontal(lipgloss.Top, left, middle, right)
	}

	// Title bar with namespace indicator on the right.
	title := ui.FillLinesBg(m.renderTitleBar(), m.width, ui.BarBg)

	// Status bar.
	status := ui.FillLinesBg(m.statusBar(), m.width, ui.BarBg)

	var parts []string
	parts = append(parts, title)
	if tabBar != "" {
		parts = append(parts, tabBar)
	}
	parts = append(parts, columns, status)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) renderTitleBar() string {
	// TitleBarStyle has Padding(0, 1) which adds 2 chars of horizontal padding.
	// The inner content area is m.width - 2.
	innerWidth := m.width - 2
	if innerWidth < 10 {
		innerWidth = 10
	}

	var watchIndicator string
	if m.watchMode {
		watchIndicator = ui.HelpKeyStyle.Render(" \u27f3 ")
	}

	nsText := m.namespace
	switch {
	case m.allNamespaces:
		nsText = "all"
	case len(m.selectedNamespaces) > 1:
		names := make([]string, 0, len(m.selectedNamespaces))
		for ns := range m.selectedNamespaces {
			names = append(names, ns)
		}
		sort.Strings(names)
		if len(names) > 3 {
			nsText = fmt.Sprintf("%s +%d more", strings.Join(names[:3], ","), len(names)-3)
		} else {
			nsText = strings.Join(names, ",")
		}
	case len(m.selectedNamespaces) == 1:
		for ns := range m.selectedNamespaces {
			nsText = ns
		}
	}
	nsLabel := ui.NamespaceBadgeStyle.Render(" ns: " + nsText + " ")

	var versionLabel string
	if m.version != "" {
		versionLabel = ui.BarDimStyle.Render(" " + m.version)
	}

	// Calculate available width for breadcrumb.
	fixedWidth := lipgloss.Width(watchIndicator) + lipgloss.Width(nsLabel) + lipgloss.Width(versionLabel)
	maxBcWidth := innerWidth - fixedWidth - 1 // -1 for minimum gap
	if maxBcWidth < 10 {
		maxBcWidth = 10
	}

	bcText := " " + m.breadcrumb() + " "
	if lipgloss.Width(bcText) > maxBcWidth {
		runes := []rune(bcText)
		if len(runes) > maxBcWidth-1 {
			bcText = string(runes[:maxBcWidth-2]) + "~ "
		}
	}
	bc := ui.TitleBreadcrumbStyle.Render(bcText)

	contentWidth := lipgloss.Width(bc) + lipgloss.Width(watchIndicator) + lipgloss.Width(nsLabel) + lipgloss.Width(versionLabel)
	gap := innerWidth - contentWidth
	if gap < 0 {
		gap = 0
	}

	barContent := bc + watchIndicator + ui.BarDimStyle.Render(strings.Repeat(" ", gap)) + nsLabel + versionLabel
	return ui.TitleBarStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(barContent)
}
