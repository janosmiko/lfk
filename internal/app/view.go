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

	// Activate the SEC badge on resource rows when at least one security
	// source is available and findings have been indexed for the current
	// cluster. The renderer reads these globals during table layout.
	ui.ActiveSecurityIndex = m.securityIndex
	ui.ActiveSecurityAvailable = m.anySecurityAvailable()

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

			overlay := ui.RenderHelpScreen(m.width, fullHeight-1, m.helpScroll, m.helpFilter.Value, m.helpSearchQuery, m.helpContextMode, m.helpCurrentMatchLine())
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
		overlay := ui.RenderHelpScreen(m.width, m.height-1, m.helpScroll, m.helpFilter.Value, m.helpSearchQuery, m.helpContextMode, m.helpCurrentMatchLine())
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
		ui.ActivePrinterColumns = nil
		return
	}
	// CRD additionalPrinterColumns for the navigated resource type drive
	// priority-aware, first-class printer-column rendering. Only applies when
	// the rendered kind matches nav.ResourceType (the middle column at
	// LevelResources); owned children/containers have their own kinds.
	if rt := m.nav.ResourceType; len(rt.PrinterColumns) > 0 && rt.Kind != "" && strings.EqualFold(rt.Kind, kind) {
		pcs := make(map[string]int, len(rt.PrinterColumns))
		for _, pc := range rt.PrinterColumns {
			pcs[pc.Name] = pc.Priority
		}
		ui.ActivePrinterColumns = pcs
	} else {
		ui.ActivePrinterColumns = nil
	}
	key := m.columnMemoryKey(kind)
	// Session extras: nil vs non-nil-empty distinguishes "auto-detect" from
	// "user explicitly configured no extras".
	if sessionCols, ok := m.sessionColumns[key]; ok {
		if sessionCols == nil {
			sessionCols = []string{}
		}
		ui.ActiveSessionColumns = sessionCols
	} else {
		ui.ActiveSessionColumns = nil
	}
	// Hidden built-in columns for this kind. Session toggles (from the
	// column-toggle overlay) win over view-derived defaults; when neither is
	// set, fall back to the view's column list — any built-in not listed in
	// views.<kind>.columns is hidden so the table doesn't render columns the
	// user didn't ask for.
	if hiddenBi, ok := m.hiddenBuiltinColumns[key]; ok && len(hiddenBi) > 0 {
		set := make(map[string]bool, len(hiddenBi))
		for _, k := range hiddenBi {
			set[k] = true
		}
		ui.ActiveHiddenBuiltinColumns = set
	} else if viewHidden := ui.HiddenBuiltinsForView(m.viewRefForKind(kind), m.nav.Context); viewHidden != nil {
		ui.ActiveHiddenBuiltinColumns = viewHidden
	} else {
		ui.ActiveHiddenBuiltinColumns = nil
	}
	// Explicit column order (excluding Name).
	if order, ok := m.columnOrder[key]; ok && len(order) > 0 {
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
	prevPrinter := ui.ActivePrinterColumns
	m.applySessionColumnsForKind(kind)
	defer func() {
		ui.ActiveSessionColumns = prevSession
		ui.ActiveHiddenBuiltinColumns = prevHidden
		ui.ActiveColumnOrder = prevOrder
		ui.ActivePrinterColumns = prevPrinter
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
	// Set highlight query for search/filter term highlighting. Search
	// query takes precedence over filter when both are set, and it
	// persists past Enter (m.searchInput.Value stays populated until
	// Esc clears it) so highlights stay visible while n/N navigation
	// is meaningful.
	ui.ActiveHighlightQuery = m.filterText
	if m.searchInput.Value != "" {
		ui.ActiveHighlightQuery = m.searchInput.Value
	}
	defer func() { ui.ActiveHighlightQuery = "" }()

	// Category bars only light up when the user opted into category
	// matching via Tab — at LevelResourceTypes only, since that's
	// where the bars actually render. Plain `/foo` or `f foo` thus
	// stays a name-search both visually and behaviourally; Tab is
	// the explicit "include groups" toggle in both senses.
	ui.ActiveHighlightCategories = m.nav.Level == model.LevelResourceTypes &&
		((m.searchInput.Value != "" && m.searchBroadMode) ||
			(m.searchInput.Value == "" && m.filterText != "" && m.filterBroadMode))
	defer func() { ui.ActiveHighlightCategories = false }()

	// Set secret values visibility for rendering.
	ui.ActiveShowSecretValues = m.showSecretValues

	// Set fullscreen mode and context for column visibility.
	ui.ActiveFullscreenMode = m.fullscreenMiddle
	ui.ActiveContext = m.nav.Context
	// Carry the middle column's resource ref so view configs keyed by
	// GVR (e.g. "apps/v1/deployments") resolve inside collectExtraColumns.
	ui.ActiveResourceRef = m.middleColumnRef()

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

	contentHeight := max(
		// room for title(1) + column borders(2) + status(1)
		m.height-4, 3)

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
	var middleCol string
	switch m.nav.Level {
	case model.LevelResources, model.LevelOwned, model.LevelContainers:
		if m.middleTableRenderer != nil {
			middleCol = m.middleTableRenderer.Render(middleHeader, m.visibleMiddleItems(), m.cursor(), middleInner, contentHeight, m.loading, m.spinner.View(), middleErrMsg, m.middleItemsRev, m.selectionRev)
		} else {
			middleCol = ui.RenderTable(middleHeader, m.visibleMiddleItems(), m.cursor(), middleInner, contentHeight, m.loading, m.spinner.View(), middleErrMsg)
		}
	default:
		// At LevelClusters, surface column labels (NAME / DEF /
		// STATUS / COLOR) in the header so the trailing marker
		// block on each row reads as a real table.
		header := middleHeader
		if m.nav.Level == model.LevelClusters {
			header = ui.ClusterPickerHeader(middleInner)
		}
		middleCol = ui.RenderColumn(header, m.visibleMiddleItems(), m.cursor(), middleInner, contentHeight, true, m.loading, m.spinner.View(), middleErrMsg)
	}
	// Clear sort indicator so it doesn't appear in right column (children) tables.
	ui.ActiveSortColumnName = ""
	middleCol = ui.PadToHeight(middleCol, contentHeight)
	middleCol = ui.FillLinesBg(middleCol, middleInner, ui.BaseBg)
	middle := ui.ActiveColumnStyle.Width(middleW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(middleCol)

	columns := m.viewExplorerColumns(middle, leftW, leftInner, rightW, rightInner, contentHeight)

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

	maxHeight := max(min(m.height/2, 10), 1)

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
	innerWidth := max(m.width-2, 10)

	// Resolve cluster-tint up front: when set, every segment that
	// normally carries barBg needs to swap to the tint colour as its
	// background instead \u2014 otherwise the tint only fills cells without
	// an explicit bg (the wrapper's Padding cells), leaving badges /
	// breadcrumb / gap looking untinted.
	tint := m.clusterColorForActiveContext()
	tintStyle := ui.ClusterColorTitleBarStyle(tint)
	tintBg := tintStyle.GetBackground()

	var watchIndicator string
	if m.watchMode {
		st := ui.HelpKeyStyle
		if tint != "" {
			st = st.Background(tintBg)
		}
		watchIndicator = st.Render(" \u27f3 ")
	}

	var readOnlyIndicator string
	// At the cluster picker (LevelClusters) the user has not entered a
	// specific context yet, so a global "RO" header would be ambiguous —
	// per-row [RO] markers in the picker do that job. Once inside a
	// context, the header badge tracks the current tab's state.
	if m.readOnly && m.nav.Level != model.LevelClusters {
		readOnlyIndicator = ui.ReadOnlyBadgeStyle.Render("RO")
	}

	var mutationProgress, tasksIndicator string
	if m.scheduler != nil && m.scheduler.LenIndicator() > 0 {
		// Watch-mode auto-refresh marks its tasks Silent so the spinner
		// doesn't flicker every second. Filter them here too — they
		// stay in the :scheduler overlay history but don't crowd the
		// title-bar indicator.
		snap := nonSilentTasks(m.scheduler.Snapshot())
		if tint != "" {
			mutationProgress = renderMutationProgressOverrideBg(m.spinner.View(), snap, tintBg)
			tasksIndicator = renderTasksIndicatorOverrideBg(m.spinner.View(), snap, tintBg)
		} else {
			mutationProgress = renderMutationProgress(m.spinner.View(), snap)
			tasksIndicator = renderTasksIndicator(m.spinner.View(), snap)
		}
	}

	nsLabel := ui.NamespaceBadgeStyle.Render(" ns: " + m.buildNsLabelText() + " ")

	var versionLabel string
	if m.version != "" {
		if tint != "" {
			versionLabel = tintStyle.Render(" " + m.version)
		} else {
			versionLabel = ui.BarDimStyle.Render(" " + m.version)
		}
	}

	// Calculate available width for breadcrumb.
	fixedWidth := lipgloss.Width(watchIndicator) + lipgloss.Width(readOnlyIndicator) + lipgloss.Width(mutationProgress) + lipgloss.Width(tasksIndicator) + lipgloss.Width(nsLabel) + lipgloss.Width(versionLabel)
	maxBcWidth := max(
		// -1 for minimum gap
		innerWidth-fixedWidth-1, 10)

	bcText := " " + m.breadcrumb() + " "
	if lipgloss.Width(bcText) > maxBcWidth {
		runes := []rune(bcText)
		if len(runes) > maxBcWidth-1 {
			bcText = string(runes[:maxBcWidth-2]) + "~ "
		}
	}
	var bc string
	if tint != "" {
		// Bold black-on-bright matches ClusterColorTitleBarStyle and keeps
		// the breadcrumb path legible against every named tint.
		bc = tintStyle.Render(bcText)
	} else {
		bc = ui.TitleBreadcrumbStyle.Render(bcText)
	}

	contentWidth := lipgloss.Width(bc) + lipgloss.Width(watchIndicator) + lipgloss.Width(readOnlyIndicator) + lipgloss.Width(mutationProgress) + lipgloss.Width(tasksIndicator) + lipgloss.Width(nsLabel) + lipgloss.Width(versionLabel)
	gap := max(innerWidth-contentWidth, 0)

	var gapContent string
	if tint != "" {
		gapContent = tintStyle.Render(strings.Repeat(" ", gap))
	} else {
		gapContent = ui.BarDimStyle.Render(strings.Repeat(" ", gap))
	}

	// Record the namespace badge's screen-space x range so the mouse
	// handler can map clicks on row 0 to the namespace selector. The
	// outer style wraps barContent with Padding(0, 1) which adds a
	// 1-char left margin, so positions in barContent shift right by 1
	// to land on screen.
	nsStartX := 1 +
		lipgloss.Width(bc) +
		lipgloss.Width(watchIndicator) +
		lipgloss.Width(readOnlyIndicator) +
		lipgloss.Width(gapContent) +
		lipgloss.Width(mutationProgress) +
		lipgloss.Width(tasksIndicator)
	setTitleBarLayout(titleBarLayout{
		nsStartX: nsStartX,
		nsEndX:   nsStartX + lipgloss.Width(nsLabel),
	})

	barContent := bc + watchIndicator + readOnlyIndicator + gapContent + mutationProgress + tasksIndicator + nsLabel + versionLabel
	if tint != "" {
		// Outer wrap also uses the tint so the Padding(0, 1) cells share
		// the bar background — the inner segments each carry the tint
		// explicitly so it spans every column even with the [RO] / ns /
		// watch badges layered on top with their own backgrounds.
		return tintStyle.
			Width(m.width).MaxWidth(m.width).MaxHeight(1).
			Padding(0, 1).
			Render(barContent)
	}
	return ui.TitleBarStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(barContent)
}

// leftColumnLoading reports whether the left column should display a
// loading spinner. The left column represents the *parent* of the current
// middle list (kubeconfig -> contexts -> resource types -> resources ...).
// At LevelClusters there is no parent — the left column is empty by design
// — so a spinner there would be misleading. Everywhere else the spinner
// tracks m.loading so the parent header shows progress during discovery /
// context switches.
func (m Model) leftColumnLoading() bool {
	return m.loading && m.nav.Level != model.LevelClusters
}

// viewExplorerThreeCol renders the standard three-column explorer layout.
//
// The search/filter highlight is scoped to the middle column only —
// neither the left (parent context: resource-type categories,
// kubeconfigs, …) nor the right (child preview) should light up just
// because the user typed `/workload`. The middle was already rendered
// upstream with ActiveHighlightQuery active; we clear it here before
// touching either side column and restore at the end.
func (m Model) viewExplorerThreeCol(middle string, leftW, leftInner, rightW, rightInner, contentHeight int) string {
	savedHighlight := ui.ActiveHighlightQuery
	ui.ActiveHighlightQuery = ""
	leftCol := ui.RenderColumn(m.leftColumnHeader(), m.leftItems, m.parentIndex(), leftInner, contentHeight, false, m.leftColumnLoading(), m.spinner.View(), "")
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

// buildNsLabelText returns the text portion of the namespace scope chip shown
// in the title bar. When nsSelectionNegated is true, each selected namespace
// is prefixed with "!" to indicate it is excluded.
func (m Model) buildNsLabelText() string {
	if m.allNamespaces {
		return "all"
	}
	if len(m.selectedNamespaces) == 0 {
		return m.namespace
	}

	names := make([]string, 0, len(m.selectedNamespaces))
	for ns := range m.selectedNamespaces {
		names = append(names, ns)
	}
	sort.Strings(names)

	if m.nsSelectionNegated {
		for i, ns := range names {
			names[i] = "!" + ns
		}
	}

	if len(names) > 3 {
		more := "more"
		if m.nsSelectionNegated {
			more = "more excl"
		}
		return fmt.Sprintf("%s +%d %s", strings.Join(names[:3], ","), len(names)-3, more)
	}
	return strings.Join(names, ",")
}
