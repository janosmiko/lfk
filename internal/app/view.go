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

	// Sync nyan mode state to UI globals for rendering.
	ui.NyanMode = m.nyanMode
	ui.NyanTick = m.nyanTick

	// Render fullscreen modes (YAML, Logs, Describe, Diff, Exec, Explain) with title bar and tab bar.
	// Each view renders its own hint bar, so the main status bar is not shown.
	// Also render the fullscreen view as background when help is open from a fullscreen mode.
	renderMode := m.mode
	if m.mode == modeHelp {
		renderMode = m.helpPreviousMode
	}
	if renderMode == modeYAML || renderMode == modeLogs || renderMode == modeDescribe || renderMode == modeDiff || renderMode == modeExec || renderMode == modeExplain || renderMode == modeEventViewer {
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
		case modeEventViewer:
			content = m.viewEventViewer()
		}

		var parts []string
		parts = append(parts, title)
		if tabBar != "" {
			parts = append(parts, tabBar)
		}
		parts = append(parts, content)
		view := lipgloss.JoinVertical(lipgloss.Left, parts...)

		// Render overlay on top if active (e.g. Can-I subject selector).
		// Use fullHeight so PlaceOverlay doesn't trim the view (which
		// includes title bar + tab bar above the content).
		if m.overlay != overlayNone {
			m.height = fullHeight
			view = m.renderOverlay(view)
			// Replace the last line with the overlay hint bar.
			hintBar := m.overlayHintBar()
			if hintBar != "" {
				viewLines := strings.Split(view, "\n")
				if len(viewLines) > 0 {
					viewLines[len(viewLines)-1] = ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(hintBar)
				}
				view = strings.Join(viewLines, "\n")
			}
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

			overlay := ui.RenderHelpScreen(m.width, fullHeight-1, m.helpScroll, m.helpFilter.Value, m.helpContextMode)
			view = ui.PlaceOverlay(m.width, fullHeight, overlay, view)
		}

		return view
	}

	// Credits mode: fullscreen scrolling credits, any key exits.
	if m.mode == modeCredits {
		return m.viewCredits()
	}

	// Kubetris mode: fullscreen falling blocks game.
	if m.mode == modeKubetris {
		return m.viewKubetris()
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
		overlay := ui.RenderHelpScreen(m.width, m.height-1, m.helpScroll, m.helpFilter.Value, m.helpContextMode)
		view = ui.PlaceOverlay(m.width, m.height, overlay, view)
	}

	return view
}

// applySessionColumnsForKind sets the ui package vars that drive column
// visibility to the configuration stored for the given kind. Pass an empty
// kind to clear all overrides. The vars are globals consumed by RenderTable
// during the next render, so the caller must call this immediately before
// rendering and use withSessionColumnsForKind to restore if the same frame
// needs to render multiple kinds.
func (m Model) applySessionColumnsForKind(kind string) {
	if kind == "" {
		ui.ActiveSessionColumns = nil
		ui.ActiveHiddenBuiltinColumns = nil
		ui.ActiveColumnOrder = nil
		return
	}
	// Session extras: nil vs non-nil-empty distinguishes "auto-detect" from
	// "user explicitly configured no extras".
	if sessionCols, ok := m.sessionColumns[kind]; ok {
		if sessionCols == nil {
			sessionCols = []string{}
		}
		ui.ActiveSessionColumns = sessionCols
	} else {
		ui.ActiveSessionColumns = nil
	}
	// Hidden built-in columns for this kind.
	if hiddenBi, ok := m.hiddenBuiltinColumns[kind]; ok && len(hiddenBi) > 0 {
		set := make(map[string]bool, len(hiddenBi))
		for _, k := range hiddenBi {
			set[k] = true
		}
		ui.ActiveHiddenBuiltinColumns = set
	} else {
		ui.ActiveHiddenBuiltinColumns = nil
	}
	// Explicit column order (excluding Name).
	if order, ok := m.columnOrder[kind]; ok && len(order) > 0 {
		ui.ActiveColumnOrder = order
	} else {
		ui.ActiveColumnOrder = nil
	}
}

// withSessionColumnsForKind applies the session configuration for the
// given kind around fn. The previous ui.Active* values are captured before
// fn runs and restored afterwards, so this can be used to render a single
// table (e.g., the right-column children) with a different kind's config
// without leaking the swap back into subsequent renders in the same frame.
// fn's return value is passed through untouched.
func (m Model) withSessionColumnsForKind(kind string, fn func() string) string {
	prevSession := ui.ActiveSessionColumns
	prevHidden := ui.ActiveHiddenBuiltinColumns
	prevOrder := ui.ActiveColumnOrder
	m.applySessionColumnsForKind(kind)
	defer func() {
		ui.ActiveSessionColumns = prevSession
		ui.ActiveHiddenBuiltinColumns = prevHidden
		ui.ActiveColumnOrder = prevOrder
	}()
	return fn()
}

// rightColumnKind returns the lowercased kind that identifies the items
// currently rendered in the right column (children pane). Derived from
// the first rightItem when available; falls back to the empty string,
// which applySessionColumnsForKind treats as "no overrides".
func (m Model) rightColumnKind() string {
	if len(m.rightItems) > 0 && m.rightItems[0].Kind != "" {
		return strings.ToLower(m.rightItems[0].Kind)
	}
	return ""
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

	// Set fullscreen mode and context for column visibility.
	ui.ActiveFullscreenMode = m.fullscreenMiddle
	ui.ActiveContext = m.nav.Context

	// Set sort state for column header indicators.
	// Mirror the Event override so the arrow appears on "Last Seen"
	// instead of "Name" when Events use their default sort.
	ui.ActiveSortColumnName = m.sortColumnName
	ui.ActiveSortAscending = m.sortAscending
	if m.sortColumnName == sortColDefault && m.nav.ResourceType.Kind == "Event" {
		ui.ActiveSortColumnName = "Last Seen"
	}

	// Apply session column config for the middle column's kind.
	// middleColumnKind() reflects the kind of items actually rendered in
	// the middle column, which differs from nav.ResourceType.Kind at
	// LevelOwned/LevelContainers (e.g., containers under a pod must not
	// share column config with the parent pod list).
	m.applySessionColumnsForKind(m.middleColumnKind())

	// Set selection state for rendering.
	ui.ActiveSelectedItems = m.selectedItems
	defer func() { ui.ActiveSelectedItems = nil }()

	// Set security badge state so RenderTable can decorate eligible rows.
	// The badge is gated on any available security source so clusters without
	// one see the same output they had before security was introduced.
	ui.ActiveSecurityAvailable = m.securityAvailableAny()
	if ui.ActiveSecurityAvailable && m.securityManager != nil {
		ui.ActiveSecurityIndex = m.securityManager.Index()
	} else {
		ui.ActiveSecurityIndex = nil
	}
	defer func() {
		ui.ActiveSecurityAvailable = false
		ui.ActiveSecurityIndex = nil
	}()

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

	contentHeight := m.height - 4 // room for title(1) + column borders(2) + status(1)
	if contentHeight < 3 {
		contentHeight = 3
	}

	// Tab bar (only shown with 2+ tabs).
	var tabBar string
	if len(m.tabs) > 1 {
		tabBar = ui.RenderTabBar(m.tabLabels(), m.activeTab, m.width)
		contentHeight-- // tab bar takes one line
	}

	// Command bar dropdown (rendered above the status bar).
	dropdown := m.commandBarDropdown()
	dropdownHeight := 0
	if dropdown != "" {
		dropdownHeight = strings.Count(dropdown, "\n") + 1
		contentHeight -= dropdownHeight
		if contentHeight < 3 {
			contentHeight = 3
		}
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
	if m.filterText != "" {
		middleHeader += " (filtered: " + m.filterText + ")"
	}
	var middleCol string
	switch m.nav.Level {
	case model.LevelResources, model.LevelOwned, model.LevelContainers:
		middleCol = ui.RenderTable(middleHeader, m.visibleMiddleItems(), m.cursor(), middleInner, contentHeight, m.loading, m.spinner.View(), middleErrMsg)
	default:
		middleCol = ui.RenderColumn(middleHeader, m.visibleMiddleItems(), m.cursor(), middleInner, contentHeight, true, m.loading, m.spinner.View(), middleErrMsg)
	}
	// Clear sort indicator so it doesn't appear in right column (children) tables.
	ui.ActiveSortColumnName = ""
	middleCol = ui.PadToHeight(middleCol, contentHeight)
	middleCol = ui.FillLinesBg(middleCol, middleInner, ui.BaseBg)
	middle := ui.ActiveColumnStyle.Width(middleW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(middleCol)

	var columns string
	switch {
	case m.fullscreenDashboard:
		columns = m.viewExplorerDashboard(contentHeight)
	case m.fullscreenMiddle:
		columns = middle
	default:
		columns = m.viewExplorerThreeCol(middle, leftW, leftInner, rightW, rightInner, contentHeight)
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
	parts = append(parts, columns)
	if dropdown != "" {
		parts = append(parts, dropdown)
	}
	parts = append(parts, status)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// commandBarDropdown renders the vertical suggestion dropdown for the command bar.
// Returns an empty string when the command bar is inactive or has no suggestions.
func (m Model) commandBarDropdown() string {
	if !m.commandBarActive || len(m.commandBarSuggestions) == 0 {
		return ""
	}

	maxHeight := m.height / 2
	if maxHeight > 10 {
		maxHeight = 10
	}

	if maxHeight < 1 {
		maxHeight = 1
	}

	return ui.RenderCommandDropdown(
		m.commandBarSuggestions,
		m.commandBarSelectedSuggestion,
		maxHeight,
		m.width,
	)
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

	var mutationProgress, tasksIndicator string
	if m.bgtasks != nil && m.bgtasks.Len() > 0 {
		snap := m.bgtasks.Snapshot()
		mutationProgress = renderMutationProgress(m.spinner.View(), snap)
		tasksIndicator = renderTasksIndicator(m.spinner.View(), snap)
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
	fixedWidth := lipgloss.Width(watchIndicator) + lipgloss.Width(mutationProgress) + lipgloss.Width(tasksIndicator) + lipgloss.Width(nsLabel) + lipgloss.Width(versionLabel)
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

	contentWidth := lipgloss.Width(bc) + lipgloss.Width(watchIndicator) + lipgloss.Width(mutationProgress) + lipgloss.Width(tasksIndicator) + lipgloss.Width(nsLabel) + lipgloss.Width(versionLabel)
	gap := innerWidth - contentWidth
	if gap < 0 {
		gap = 0
	}

	barContent := bc + watchIndicator + ui.BarDimStyle.Render(strings.Repeat(" ", gap)) + mutationProgress + tasksIndicator + nsLabel + versionLabel
	return ui.TitleBarStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(barContent)
}

// viewExplorerDashboard renders the fullscreen dashboard view.
func (m Model) viewExplorerDashboard(contentHeight int) string {
	sel := m.selectedMiddleItem()
	isMonitoring := sel != nil && sel.Extra == "__monitoring__"

	fullW := m.width - 2

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

	if !isMonitoring && m.dashboardEventsPreview != "" {
		return m.viewExplorerDashboardTwoCol(dashContent, fullW, contentHeight)
	}
	return m.viewExplorerDashboardSingleCol(dashContent, fullW, contentHeight)
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

// viewExplorerDashboardTwoCol renders a two-column fullscreen dashboard.
func (m Model) viewExplorerDashboardTwoCol(dashContent string, fullW, contentHeight int) string {
	leftW := fullW / 2
	for _, line := range strings.Split(dashContent, "\n") {
		if w := lipgloss.Width(line); w+2 > leftW {
			leftW = w + 2
		}
	}
	maxLeft := fullW * 60 / 100
	if leftW > maxLeft {
		leftW = maxLeft
	}
	rightW := fullW - leftW - 1

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
	maxContentW := rightW - 4
	if maxContentW < 10 {
		maxContentW = 10
	}
	wrapStyle := lipgloss.NewStyle().Width(maxContentW)
	result := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		if lipgloss.Width(line) == 0 {
			result = append(result, "")
		} else if lipgloss.Width(line) <= maxContentW {
			result = append(result, pad+line)
		} else {
			wrapped := wrapStyle.Render(line)
			for _, wl := range strings.Split(wrapped, "\n") {
				result = append(result, pad+wl)
			}
		}
	}
	return result
}

// viewExplorerThreeCol renders the standard three-column explorer layout.
func (m Model) viewExplorerThreeCol(middle string, leftW, leftInner, rightW, rightInner, contentHeight int) string {
	leftCol := ui.RenderColumn(m.leftColumnHeader(), m.leftItems, m.parentIndex(), leftInner, contentHeight, false, m.loading, m.spinner.View(), "")
	savedHighlight := ui.ActiveHighlightQuery
	ui.ActiveHighlightQuery = ""
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
	return lipgloss.JoinHorizontal(lipgloss.Top, left, middle, right)
}
