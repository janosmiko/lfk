package app

import (
	"fmt"
	"strings"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// leftColumnHeader returns the header label for the left (parent) column.
func (m Model) leftColumnHeader() string {
	switch m.nav.Level {
	case model.LevelClusters:
		return "" // no parent at top level
	case model.LevelResourceTypes:
		return "KUBECONFIG"
	case model.LevelResources:
		return "RESOURCE TYPE"
	case model.LevelOwned:
		return strings.ToUpper(m.nav.ResourceType.DisplayName)
	case model.LevelContainers:
		return strings.ToUpper(m.nav.ResourceType.DisplayName)
	default:
		return ""
	}
}

// middleColumnHeader returns the header label for the middle column.
func (m Model) middleColumnHeader() string {
	switch m.nav.Level {
	case model.LevelClusters:
		return "KUBECONFIG"
	case model.LevelResourceTypes:
		return "RESOURCE TYPE"
	case model.LevelResources:
		return strings.ToUpper(m.nav.ResourceType.Kind)
	case model.LevelOwned:
		return strings.ToUpper(m.ownedItemKindLabel())
	case model.LevelContainers:
		return "CONTAINER"
	default:
		return ""
	}
}

// breadcrumb builds the "lfk > context > Type > Name > Owned" path rendered
// in the title bar. It mirrors labelForNav (see tabs.go) — both must use
// model.DisplayNameFor for the resource type because API-discovery-produced
// ResourceTypeEntry values do NOT populate DisplayName themselves. Reading
// nav.ResourceType.DisplayName directly silently drops the type for almost
// every real-world resource, leaving the title bar showing only the context.
func (m Model) breadcrumb() string {
	parts := []string{"lfk"}
	if m.nav.Context != "" {
		parts = append(parts, m.nav.Context)
	}
	if name := model.DisplayNameFor(m.nav.ResourceType); name != "" {
		parts = append(parts, name)
	}
	if m.nav.ResourceName != "" {
		parts = append(parts, m.nav.ResourceName)
	}
	// navigateChildResource sets both ResourceName and OwnedName to the same
	// value when entering a Pod (so the containers view knows its parent).
	// Skip the duplicate so the breadcrumb reads "lfk > ctx > Pods > my-pod"
	// instead of "lfk > ctx > Pods > my-pod > my-pod".
	if m.nav.OwnedName != "" && m.nav.OwnedName != m.nav.ResourceName {
		parts = append(parts, m.nav.OwnedName)
	}
	return strings.Join(parts, " > ")
}

func (m Model) statusBar() string {
	// StatusBarBgStyle has Padding(0, 1) which adds 2 chars of horizontal padding.
	// Use MaxWidth on the content to prevent overflow.
	innerWidth := m.width - 2
	if innerWidth < 10 {
		innerWidth = 10
	}

	// Show command bar when active.
	if m.commandBarActive {
		var prompt string
		if m.commandBarPreview != "" {
			// Ghost preview mode: show typed text + ghost completion (dimmed) + cursor at end.
			typed := m.commandBarInput.Value
			lastSpace := strings.LastIndex(typed, " ")
			partial := typed
			if lastSpace >= 0 {
				partial = typed[lastSpace+1:]
			}
			ghost := m.commandBarPreview
			lp := strings.ToLower(partial)
			lg := strings.ToLower(ghost)
			if strings.HasPrefix(lg, lp) {
				ghost = ghost[len(partial):]
			}
			prompt = ui.HelpKeyStyle.Render(":") +
				typed +
				ui.BarDimStyle.Render(ghost) +
				ui.BarDimStyle.Render("\u2588")
		} else {
			// Normal mode: overlay the cursor on the character at its position
			// (reverse-video) instead of inserting a block between characters.
			// Inserting would visually push the text after the cursor to the
			// right by one column and create the impression of a space.
			prompt = ui.HelpKeyStyle.Render(":") + renderInputWithCursor(m.commandBarInput.Value, m.commandBarInput.Cursor)
		}
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).Render(prompt)
	}

	// Show filter/search input in status bar when active.
	if m.filterActive {
		filterModeInd := ui.SearchModeIndicator(m.filterInput.Value)
		prompt := ui.HelpKeyStyle.Render("filter") + ui.BarDimStyle.Render(": ") + ui.BarDimStyle.Render(filterModeInd) + renderInputWithCursor(m.filterInput.Value, m.filterInput.Cursor)
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).Render(prompt)
	}
	if m.searchActive {
		searchModeInd := ui.SearchModeIndicator(m.searchInput.Value)
		prompt := ui.HelpKeyStyle.Render("search") + ui.BarDimStyle.Render(": ") + ui.BarDimStyle.Render(searchModeInd) + renderInputWithCursor(m.searchInput.Value, m.searchInput.Cursor)
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).Render(prompt)
	}
	// When a status message is active, show it exclusively (hide key hints).
	if m.hasStatusMessage() {
		msg := m.sanitizeMessage(m.statusMessage)
		var styled string
		if m.statusMessageErr {
			styled = ui.StatusMessageErrStyle.Render(msg)
		} else {
			styled = ui.StatusMessageOkStyle.Render(msg)
		}
		styled = ui.Truncate(styled, innerWidth)
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(styled)
	}

	// When an overlay is active, show overlay-specific hints instead of explorer hints.
	if hint := m.overlayHintBar(); hint != "" {
		content := ui.Truncate(hint, innerWidth)
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(content)
	}

	// When the help screen is active, show help-specific hints.
	if m.mode == modeHelp {
		var helpHint string
		switch {
		case m.helpSearchActive:
			helpHint = ui.HelpKeyStyle.Render("search") + ui.BarDimStyle.Render(": ") + m.helpSearchInput.View()
		case m.helpFilter.Value != "":
			helpHint = ui.BarDimStyle.Render("filter: ") +
				ui.HelpKeyStyle.Render(m.helpFilter.Value) + "  " +
				ui.HelpKeyStyle.Render("/") + ui.BarDimStyle.Render(" edit") + "  " +
				ui.HelpKeyStyle.Render("Esc") + ui.BarDimStyle.Render(" close")
		default:
			helpHint = m.renderHints([]ui.HintEntry{
				{Key: "j/k", Desc: "scroll"},
				{Key: "^d/^u", Desc: "half-page"},
				{Key: "/", Desc: "search"},
				{Key: "Esc/?/q", Desc: "close"},
			})
		}
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(helpHint)
	}

	// When the error log overlay is active, show error log hints.
	if m.overlayErrorLog {
		var entries []ui.HintEntry
		switch m.errorLogVisualMode {
		case 'v':
			entries = []ui.HintEntry{
				{Key: "h/l", Desc: "column"},
				{Key: "j/k", Desc: "extend"},
				{Key: "0/$", Desc: "start/end"},
				{Key: "y", Desc: "copy"},
				{Key: "esc", Desc: "cancel"},
			}
		case 'V':
			entries = []ui.HintEntry{
				{Key: "j/k", Desc: "extend"},
				{Key: "y", Desc: "copy"},
				{Key: "esc", Desc: "cancel"},
			}
		default:
			debugHint := "show debug"
			if m.showDebugLogs {
				debugHint = "hide debug"
			}
			fsHint := "fullscreen"
			if m.errorLogFullscreen {
				fsHint = "overlay"
			}
			entries = []ui.HintEntry{
				{Key: "j/k", Desc: "scroll"},
				{Key: "V", Desc: "select"},
				{Key: "y", Desc: "copy all"},
				{Key: "f", Desc: fsHint},
				{Key: "d", Desc: debugHint},
				{Key: "esc", Desc: "close"},
			}
		}
		hint := m.renderHints(entries)
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(hint)
	}

	var parts []string

	// Show selection count when items are selected.
	if m.hasSelection() {
		parts = append(parts, ui.SelectionCountStyle.Render(fmt.Sprintf(" %d selected ", len(m.selectedItems))))
	}

	// Show active filter preset indicator.
	if m.activeFilterPreset != nil {
		parts = append(parts, ui.HelpKeyStyle.Render("[filter: "+m.activeFilterPreset.Name+"]"))
	}

	// Show nyan mode indicator.
	if m.nyanMode {
		parts = append(parts, ui.HelpKeyStyle.Render("[NYAN]"))
	}

	visible := m.visibleMiddleItems()
	total := len(m.middleItems)
	cur := m.cursor() + 1

	if m.filterText != "" {
		parts = append(parts, ui.BarDimStyle.Render(fmt.Sprintf("[%d/%d filtered: %d/%d]", cur, len(visible), len(visible), total)))
	} else {
		parts = append(parts, ui.BarDimStyle.Render(fmt.Sprintf("[%d/%d]", cur, total)))
	}

	// Sort mode indicator.
	parts = append(parts, ui.BarDimStyle.Render("sort:"+m.sortModeName()))

	// Styled key hints -- show a reduced set for dashboard views.
	kb := ui.ActiveKeybindings
	var hintEntries []ui.HintEntry
	sel := m.selectedMiddleItem()
	isDashboard := sel != nil && m.nav.Level == model.LevelResourceTypes &&
		(sel.Extra == "__overview__" || sel.Extra == "__monitoring__")
	if isDashboard {
		hintEntries = []ui.HintEntry{
			{Key: kb.Down + "/" + kb.Up, Desc: "move"},
			{Key: kb.PageDown + "/" + kb.PageUp, Desc: "scroll"},
			{Key: kb.NamespaceSelector, Desc: "namespace"},
			{Key: kb.NewTab, Desc: "new tab"},
			{Key: kb.Help, Desc: "help"},
			{Key: "q", Desc: "quit"},
		}
	} else {
		hintEntries = []ui.HintEntry{
			{Key: kb.Left + "/" + kb.Right, Desc: "navigate"},
			{Key: kb.Down + "/" + kb.Up, Desc: "move"},
			{Key: kb.Enter, Desc: "view"},
			{Key: kb.NamespaceSelector, Desc: "namespace"},
			{Key: kb.AllNamespaces, Desc: "all-ns"},
			{Key: kb.ActionMenu, Desc: "actions"},
			{Key: kb.CreateTemplate, Desc: "create"},
			{Key: kb.SortNext + "/" + kb.SortPrev, Desc: "sort"},
			{Key: kb.Filter, Desc: "filter"},
			{Key: kb.SetMark + "/" + kb.OpenMarks, Desc: "marks"},
			{Key: kb.Help, Desc: "help"},
			{Key: "q", Desc: "quit"},
		}
		// Add context-specific hints for Events resource type.
		hintEntries = m.appendEventsHintEntries(hintEntries)
	}
	parts = append(parts, ui.FormatHintParts(hintEntries))

	content := strings.Join(parts, "  ")
	return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(content)
}

// appendEventsHintEntries injects Events-view toggle hints (warnings-only,
// grouping) just before the trailing "quit" entry. Returns the input slice
// unchanged when the current view isn't the Events resource list.
//
// Callers must keep "quit" as the final entry so it always renders on the
// right edge of the hint bar. If that invariant changes, this helper's
// splice (entries[:len(entries)-1]) will misplace the toggles.
func (m Model) appendEventsHintEntries(entries []ui.HintEntry) []ui.HintEntry {
	if m.nav.Level != model.LevelResources || m.nav.ResourceType.Kind != "Event" {
		return entries
	}
	kb := ui.ActiveKeybindings
	warnDesc := "warnings only"
	if m.warningEventsOnly {
		warnDesc = "all events"
	}
	groupDesc := "group"
	if m.eventGrouping {
		groupDesc = "ungroup"
	}
	extras := []ui.HintEntry{
		{Key: kb.SaveResource, Desc: warnDesc},
		{Key: kb.ExpandCollapse, Desc: groupDesc},
	}
	if len(entries) == 0 {
		return extras
	}
	// Insert before the trailing "quit" entry so the Events toggles sit next
	// to the other contextual actions.
	out := make([]ui.HintEntry, 0, len(entries)+len(extras))
	out = append(out, entries[:len(entries)-1]...)
	out = append(out, extras...)
	out = append(out, entries[len(entries)-1])
	return out
}

// --- Overlay rendering ---

func (m Model) renderOverlay(background string) string {
	// Fullscreen overlays bypass the standard overlay rendering.
	switch m.overlay {
	case overlaySecretEditor, overlayConfigMapEditor, overlayRollback, overlayHelmRollback, overlayHelmHistory, overlayLabelEditor, overlayAutoSync:
		return m.renderOverlayFullscreen(background)
	case overlayCanI:
		return m.renderCanIOverlay(background)
	case overlayCanISubject:
		return m.renderOverlayCanISubject(background)
	case overlayNetworkPolicy:
		if result := m.renderOverlayNetworkPolicy(background); result != "" {
			return result
		}
	}

	content, overlayW, overlayH, ok := m.renderOverlayContent()
	if !ok {
		return background
	}

	if overlayW < 10 {
		overlayW = 10
	}
	if overlayH < 3 {
		overlayH = 3
	}

	content = ui.FillLinesBg(content, overlayW-4, ui.SurfaceBg)
	overlay := ui.OverlayStyle.Width(overlayW).Height(overlayH).Render(content)
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
}

// renderOverlayContent returns the overlay content and dimensions for standard (non-fullscreen) overlays.
func (m Model) renderOverlayContent() (string, int, int, bool) {
	switch m.overlay {
	case overlayNamespace:
		content := ui.RenderNamespaceOverlay(m.filteredOverlayItems(), m.overlayFilter.Value, m.overlayCursor, m.namespace, m.allNamespaces, m.selectedNamespaces, m.nsFilterMode)
		return content, min(60, m.width-10), min(20, m.height-6), true
	case overlayAction:
		w := min(70, m.width-10)
		return ui.RenderActionOverlay(m.overlayItems, m.overlayCursor, w), w, min(15, m.height-6), true
	case overlayQuitConfirm:
		return ui.RenderQuitConfirmOverlay(), min(40, m.width-10), min(7, m.height-6), true
	case overlayConfirm:
		return ui.RenderConfirmOverlay(m.confirmAction), min(50, m.width-10), min(8, m.height-6), true
	case overlayConfirmType:
		return ui.RenderConfirmTypeOverlay(m.confirmTitle, m.confirmQuestion, m.confirmTypeInput.Value), min(55, m.width-10), min(10, m.height-6), true
	case overlayScaleInput:
		return ui.RenderScaleOverlay(m.scaleInput.Value), min(45, m.width-10), min(8, m.height-6), true
	case overlayLogTimeRange:
		return ui.RenderLogTimeRangeOverlay(m.buildLogTimeRangeOverlayState(), min(60, m.width-10), min(24, m.height-6)), min(60, m.width-10), min(24, m.height-6), true
	case overlayPVCResize:
		return ui.RenderPVCResizeOverlay(m.scaleInput.Value, m.pvcCurrentSize), min(45, m.width-10), min(10, m.height-6), true
	case overlayPortForward:
		content := ui.RenderPortForwardOverlay(m.portForwardInput.Value, m.pfAvailablePorts, m.pfPortCursor, m.actionCtx.name)
		return content, min(55, m.width-10), min(5+len(m.pfAvailablePorts)+4, m.height-6), true
	case overlayContainerSelect:
		return ui.RenderContainerSelectOverlay(m.overlayItems, m.overlayCursor), min(50, m.width-10), min(15, m.height-6), true
	case overlayLogFilter:
		w := min(120, m.width-6)
		h := min(24, m.height-6)
		return ui.RenderLogFilterOverlay(m.logFilterOverlayState(), w, h), w, h, true
	case overlayPodSelect, overlayLogPodSelect:
		content := ui.RenderPodSelectOverlay(m.filteredLogPodItems(), m.overlayCursor, m.logPodFilterText, m.logPodFilterActive)
		return content, min(60, m.width-10), min(20, m.height-6), true
	case overlayLogContainerSelect:
		content := ui.RenderLogContainerSelectOverlay(m.filteredLogContainerItems(), m.overlayCursor, m.logSelectedContainers, m.logContainerFilterText, m.logContainerFilterActive, m.logParentKind != "")
		return content, min(60, m.width-10), min(len(m.filteredLogContainerItems())+9, m.height-6), true
	case overlayBookmarks:
		w, h := min(90, m.width-10), min(25, m.height-6)
		return ui.RenderBookmarkOverlay(m.bookmarks, m.bookmarkFilter.Value, m.overlayCursor, int(m.bookmarkSearchMode)), w, h, true
	case overlayTemplates:
		w, h := min(60, m.width-10), min(25, m.height-6)
		return ui.RenderTemplateOverlay(m.filteredTemplates(), m.templateFilter.Value, m.templateCursor, m.templateSearchMode, h), w, h, true
	case overlayColorscheme:
		content := ui.RenderColorschemeOverlay(m.schemeEntries, m.schemeFilter.Value, m.schemeCursor, m.schemeFilterMode)
		return content, min(50, m.width-10), min(22, m.height-6), true
	case overlayFilterPreset:
		c, w, h := m.renderOverlayFilterPreset()
		return c, w, h, true
	case overlayRBAC:
		c, w, h := m.renderOverlayRBAC()
		return c, w, h, true
	case overlayBatchLabel:
		content := ui.RenderBatchLabelOverlay(m.batchLabelMode, m.batchLabelInput.Value, m.batchLabelRemove)
		return content, min(50, m.width-10), min(12, m.height-6), true
	case overlayPodStartup:
		c, w, h := m.renderOverlayPodStartup()
		return c, w, h, true
	case overlayQuotaDashboard:
		c, w, h := m.renderOverlayQuotaDashboard()
		return c, w, h, true
	case overlayEventTimeline:
		c, w, h := m.renderOverlayEventTimeline()
		return c, w, h, true
	case overlayAlerts:
		c, w, h := m.renderOverlayAlerts()
		return c, w, h, true
	case overlayBackgroundTasks:
		c, w, h := m.renderOverlayBackgroundTasks()
		return c, w, h, true
	case overlayExplainSearch:
		c, w, h := m.renderOverlayExplainSearch()
		return c, w, h, true
	case overlayColumnToggle:
		c, w, h := m.renderOverlayColumnToggle()
		return c, w, h, true
	case overlayFinalizerSearch:
		c, w, h := m.renderOverlayFinalizerSearch()
		return c, w, h, true
	case overlayPasteConfirm:
		lineCount := strings.Count(strings.TrimRight(m.pendingPaste, "\n"), "\n") + 1
		return ui.RenderPasteConfirmOverlay(lineCount), min(45, m.width-10), min(8, m.height-6), true
	}
	return "", 0, 0, false
}

func (m Model) renderOverlayFilterPreset() (string, int, int) {
	var activePresetName string
	if m.activeFilterPreset != nil {
		activePresetName = m.activeFilterPreset.Name
	}
	entries := make([]ui.FilterPresetEntry, len(m.filterPresets))
	for i, p := range m.filterPresets {
		entries[i] = ui.FilterPresetEntry{Name: p.Name, Description: p.Description, Key: p.Key}
	}
	return ui.RenderFilterPresetOverlay(entries, m.overlayCursor, activePresetName), min(55, m.width-10), min(15, m.height-6)
}

func (m Model) renderOverlayRBAC() (string, int, int) {
	entries := make([]ui.RBACCheckEntry, len(m.rbacResults))
	for i, r := range m.rbacResults {
		entries[i] = ui.RBACCheckEntry{Verb: r.Verb, Allowed: r.Allowed}
	}
	return ui.RenderRBACOverlay(entries, m.rbacKind), min(45, m.width-10), min(15, m.height-6)
}

func (m Model) renderOverlayPodStartup() (string, int, int) {
	w, h := min(70, m.width-10), min(25, m.height-6)
	if m.podStartupData == nil {
		return "", w, h
	}
	entry := ui.PodStartupEntry{
		PodName: m.podStartupData.PodName, Namespace: m.podStartupData.Namespace, TotalTime: m.podStartupData.TotalTime,
	}
	for _, p := range m.podStartupData.Phases {
		entry.Phases = append(entry.Phases, ui.StartupPhaseEntry{Name: p.Name, Duration: p.Duration, Status: p.Status})
	}
	return ui.RenderPodStartupOverlay(entry), w, h
}

func (m Model) renderOverlayQuotaDashboard() (string, int, int) {
	entries := make([]ui.QuotaEntry, len(m.quotaData))
	for i, q := range m.quotaData {
		resources := make([]ui.QuotaResourceEntry, len(q.Resources))
		for j, r := range q.Resources {
			resources[j] = ui.QuotaResourceEntry{Name: r.Name, Hard: r.Hard, Used: r.Used, Percent: r.Percent}
		}
		entries[i] = ui.QuotaEntry{Name: q.Name, Namespace: q.Namespace, Resources: resources}
	}
	w, h := min(80, m.width-10), min(30, m.height-6)
	return ui.RenderQuotaDashboardOverlay(entries, w, h), w, h
}

func (m Model) renderOverlayEventTimeline() (string, int, int) {
	w, h := min(100, m.width-6), min(30, m.height-4)
	params := ui.EventViewerParams{
		Lines: m.eventTimelineLines, ResourceName: m.actionCtx.name,
		Scroll: m.eventTimelineScroll, Cursor: m.eventTimelineCursor, CursorCol: m.eventTimelineCursorCol,
		Width: w, Height: h, Wrap: m.eventTimelineWrap, Fullscreen: false,
		VisualMode: m.eventTimelineVisualMode, VisualStart: m.eventTimelineVisualStart, VisualCol: m.eventTimelineVisualCol,
		SearchQuery: m.eventTimelineSearchQuery, SearchActive: m.eventTimelineSearchActive, SearchInput: m.eventTimelineSearchInput.Value,
	}
	return ui.RenderEventViewer(params), w, h
}

func (m Model) renderOverlayAlerts() (string, int, int) {
	entries := make([]ui.AlertEntry, len(m.alertsData))
	for i, a := range m.alertsData {
		entries[i] = ui.AlertEntry{
			Name: a.Name, State: a.State, Severity: a.Severity, Summary: a.Summary,
			Description: a.Description, Since: a.Since, GrafanaURL: a.GrafanaURL,
		}
	}
	w, h := min(80, m.width-10), min(25, m.height-6)
	return ui.RenderAlertsOverlay(entries, m.alertsScroll, w, h), w, h
}

func (m Model) renderOverlayBackgroundTasks() (string, int, int) {
	var rows []ui.BackgroundTaskRow
	mode := ui.ModeRunning
	if m.tasksOverlayShowCompleted {
		mode = ui.ModeCompleted
		// Collapse identical (Kind, Name, Target) entries into a single
		// row with "×N" appended to Name. Without this, a watch-mode
		// session fills the 50-entry history with twelve consecutive
		// "List Pods / dev-envs" refreshes and evicts genuinely
		// interesting one-off tasks.
		rows = groupCompletedTasks(m.bgtasks.SnapshotCompleted())
	} else {
		snap := m.bgtasks.Snapshot()
		rows = make([]ui.BackgroundTaskRow, len(snap))
		for i, t := range snap {
			rows[i] = ui.BackgroundTaskRow{
				Kind:      t.Kind.String(),
				Name:      t.Name,
				Target:    t.Target,
				StartedAt: t.StartedAt,
			}
		}
	}
	w, h := min(120, m.width-10), min(20, m.height-6)
	return ui.RenderBackgroundTasksOverlay(rows, mode, m.tasksOverlayScroll, w, h), w, h
}

func (m Model) renderOverlayCanISubject(background string) string {
	canIBg := m.renderCanIOverlay(background)
	w, h := min(80, m.width-10), min(20, m.height-6)
	content := ui.RenderCanISubjectOverlay(m.filteredOverlayItems(), m.overlayFilter.Value, m.overlayCursor, m.canISubjectFilterMode)
	content = ui.FillLinesBg(content, w-4, ui.SurfaceBg)
	overlay := ui.OverlayStyle.Width(w).Height(h).Render(content)
	return ui.PlaceOverlay(m.width, m.height, overlay, canIBg)
}

func (m Model) renderOverlayExplainSearch() (string, int, int) {
	w := min(m.width-6, m.width*70/100)
	h := min(m.height-4, m.height*70/100)
	maxVisible := max(h-6, 1)
	filtered := m.filteredExplainRecursiveResults()
	return ui.RenderExplainSearchOverlay(filtered, m.explainRecursiveCursor, m.explainRecursiveScroll, maxVisible, m.explainRecursiveFilter.Value, m.explainRecursiveFilterActive), w, h
}

func (m Model) renderOverlayNetworkPolicy(background string) string {
	if m.netpolData == nil {
		return ""
	}
	entry := ui.NetworkPolicyEntry{
		Name: m.netpolData.Name, Namespace: m.netpolData.Namespace,
		PodSelector: m.netpolData.PodSelector, PolicyTypes: m.netpolData.PolicyTypes,
		AffectedPods: m.netpolData.AffectedPods,
	}
	for _, r := range m.netpolData.IngressRules {
		entry.IngressRules = append(entry.IngressRules, convertNetpolRule(r))
	}
	for _, r := range m.netpolData.EgressRules {
		entry.EgressRules = append(entry.EgressRules, convertNetpolRule(r))
	}
	w, h := min(100, m.width-6), min(35, m.height-4)
	innerW, innerH := w-4, h-2
	netpolContent := ui.RenderNetworkPolicyOverlay(entry, m.netpolScroll, innerW, innerH)
	netpolContent = ui.FillLinesBg(netpolContent, innerW, ui.SurfaceBg)
	overlay := ui.OverlayStyle.Width(w).Render(netpolContent)
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
}

func (m Model) renderOverlayFullscreen(background string) string {
	var overlay string
	switch m.overlay {
	case overlaySecretEditor:
		overlay = ui.RenderSecretEditorOverlay(
			m.secretData, m.secretCursor, m.secretRevealed, m.secretAllRevealed,
			m.secretEditing, m.secretEditKey.Value, m.secretEditValue.Value, m.secretEditColumn,
			m.width, m.height,
		)
	case overlayConfigMapEditor:
		overlay = ui.RenderConfigMapEditorOverlay(
			m.configMapData, m.configMapCursor,
			m.configMapEditing, m.configMapEditKey.Value, m.configMapEditValue.Value, m.configMapEditColumn,
			m.width, m.height,
		)
	case overlayRollback:
		overlay = ui.RenderRollbackOverlay(m.rollbackRevisions, m.rollbackCursor, m.width, m.height)
	case overlayHelmRollback:
		overlay = ui.RenderHelmRollbackOverlay(m.helmRollbackRevisions, m.helmRollbackCursor, m.width, m.height, m.helmRevisionsLoading)
	case overlayHelmHistory:
		overlay = ui.RenderHelmHistoryOverlay(m.helmHistoryRevisions, m.helmHistoryCursor, m.width, m.height, m.helmRevisionsLoading)
	case overlayLabelEditor:
		overlay = ui.RenderLabelEditorOverlay(
			m.labelData, m.labelCursor, m.labelTab,
			m.labelEditing, m.labelEditKey.Value, m.labelEditValue.Value, m.labelEditColumn,
			m.width, m.height,
		)
	case overlayAutoSync:
		overlay = ui.RenderAutoSyncOverlay(
			m.autoSyncEnabled, m.autoSyncSelfHeal, m.autoSyncPrune,
			m.autoSyncCursor, m.width, m.height,
		)
	default:
		return background
	}
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
}

func (m Model) renderOverlayColumnToggle() (string, int, int) {
	filtered := m.filteredColumnToggleItems()
	entries := make([]ui.ColumnToggleEntry, len(filtered))
	for i, e := range filtered {
		entries[i] = ui.ColumnToggleEntry{Key: e.key, Visible: e.visible}
	}
	return ui.RenderColumnToggleOverlay(entries, m.columnToggleCursor, m.columnToggleFilter, m.columnToggleFilterActive, m.width, m.height),
		min(50, m.width-10), min(20, m.height-6)
}

func (m Model) renderOverlayFinalizerSearch() (string, int, int) {
	filtered := m.filteredFinalizerResults()
	entries := make([]ui.FinalizerMatchEntry, len(filtered))
	for i, r := range filtered {
		entries[i] = ui.FinalizerMatchEntry{
			Name: r.Name, Namespace: r.Namespace, Kind: r.Kind, Matched: r.Matched, Age: r.Age,
		}
	}
	w := min(m.width-6, m.width*80/100)
	if w < 60 {
		w = min(60, m.width-4)
	}
	h := min(m.height-4, m.height*70/100)
	return ui.RenderFinalizerSearchOverlay(
		entries, m.finalizerSearchCursor, m.finalizerSearchSelected,
		m.finalizerSearchPattern, m.finalizerSearchFilter, m.finalizerSearchFilterActive,
		m.finalizerSearchLoading, w, h,
	), w, h
}

// convertNetpolRule converts a k8s.NetpolRule to a ui.NetpolRuleEntry.
func convertNetpolRule(r k8s.NetpolRule) ui.NetpolRuleEntry {
	re := ui.NetpolRuleEntry{}
	for _, p := range r.Ports {
		re.Ports = append(re.Ports, ui.NetpolPortEntry{Protocol: p.Protocol, Port: p.Port})
	}
	for _, p := range r.Peers {
		re.Peers = append(re.Peers, ui.NetpolPeerEntry{
			Type: p.Type, Selector: p.Selector,
			CIDR: p.CIDR, Except: p.Except, Namespace: p.Namespace,
		})
	}
	return re
}

// renderCanIOverlay renders the Can-I browser overlay on top of the given background.
func (m Model) renderCanIOverlay(background string) string {
	visibleGroupIdxs := m.canIVisibleGroups()
	groupNames := make([]string, len(visibleGroupIdxs))
	for i, idx := range visibleGroupIdxs {
		name := m.canIGroups[idx].Name
		if name == "" {
			name = "core"
		}
		count := len(m.canIGroups[idx].Resources)
		if m.canIAllowedOnly {
			count = countAllowedResources(m.canIGroups[idx].Resources)
		}
		groupNames[i] = fmt.Sprintf("%s (%d)", name, count)
	}
	var resources []model.CanIResource
	if m.canIGroupCursor >= 0 && m.canIGroupCursor < len(visibleGroupIdxs) {
		resources = m.canIGroups[visibleGroupIdxs[m.canIGroupCursor]].Resources
		if m.canIAllowedOnly {
			resources = filterAllowedResources(resources)
		}
	}
	subjectName := m.canISubjectName
	if subjectName == "" {
		subjectName = "Current User"
	}
	overlayW := min(m.width-4, m.width*90/100)
	overlayH := min(m.height-4, m.height*80/100)
	innerW := overlayW - 4
	innerH := overlayH - 2

	// Search bar shown inside the overlay; normal hints moved to the main status bar.
	var hintBar string
	if m.canISearchActive {
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.canISearchInput.CursorLeft()) + ui.BarDimStyle.Render("\u2588") + ui.BarNormalStyle.Render(m.canISearchInput.CursorRight())
		hintBar = ui.StatusBarBgStyle.Width(innerW).Render(searchBar)
	} else if m.canISearchQuery != "" {
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.canISearchQuery)
		hintBar = ui.StatusBarBgStyle.Width(innerW).Render(searchBar)
	}

	canIContent := ui.RenderCanIView(
		groupNames, resources,
		m.canIGroupCursor, m.canIGroupScroll,
		subjectName, m.canINamespaces,
		innerW, innerH,
		hintBar,
		m.canIResourceScroll,
	)
	canIContent = ui.FillLinesBg(canIContent, overlayW-4, ui.SurfaceBg)
	overlay := ui.OverlayStyle.Width(overlayW).Height(overlayH).Render(canIContent)
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
}

// renderErrorLogOverlay renders the error log overlay on top of the given background.
// In fullscreen mode it replaces the background entirely; in overlay mode it centers on top.
func (m Model) renderErrorLogOverlay(background string) string {
	vp := ui.ErrorLogVisualParams{
		VisualMode:     m.errorLogVisualMode,
		VisualStart:    m.errorLogVisualStart,
		VisualStartCol: m.errorLogVisualStartCol,
		CursorLine:     m.errorLogCursorLine,
		CursorCol:      m.errorLogCursorCol,
	}

	if m.errorLogFullscreen {
		// Fullscreen: use full terminal dimensions, no overlay border.
		content := ui.RenderErrorLogOverlay(m.errorLog, m.errorLogScroll, m.height-1, m.showDebugLogs, vp)
		// Truncate each line to terminal width so long messages do not wrap
		// and push content off the bottom of the screen.
		content = clampErrorLogLines(content, m.width, m.height-1)
		return content
	}

	overlayW := min(140, m.width-4)
	overlayH := min(30, m.height-4)
	if overlayW < 10 {
		overlayW = 10
	}
	if overlayH < 3 {
		overlayH = 3
	}

	// OverlayStyle adds 2 border + 2*2 horizontal padding + 2*1 vertical padding,
	// so the inner content area is overlayW-6 wide and overlayH-4 tall. Render
	// only that many lines so lipgloss does not expand the overlay to fit
	// overflowing content.
	innerW := overlayW - 6
	innerH := overlayH - 4
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	content := ui.RenderErrorLogOverlay(m.errorLog, m.errorLogScroll, innerH, m.showDebugLogs, vp)
	content = clampErrorLogLines(content, innerW, innerH)
	content = ui.FillLinesBg(content, innerW, ui.SurfaceBg)
	overlay := ui.OverlayStyle.Width(overlayW).Height(overlayH).Render(content)
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
}

// clampErrorLogLines truncates each line of content to maxW visual columns
// and caps the total line count at maxH. Lines that exceed maxW are cut with
// a trailing "~" marker via ui.Truncate; extra lines beyond maxH are dropped.
// This prevents long error messages from wrapping and pushing the overlay
// past its allocated height.
func clampErrorLogLines(content string, maxW, maxH int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > maxH {
		lines = lines[:maxH]
	}
	for i, line := range lines {
		lines[i] = ui.Truncate(line, maxW)
	}
	return strings.Join(lines, "\n")
}

// renderInputWithCursor renders a text input value with a reverse-video
// cursor overlaid on the character at the given byte position. When the
// cursor is past the end of the value, a highlighted space is appended.
//
// Unlike the previous "insert block between characters" approach, this
// overlays the cursor on the character it points at, so moving left or
// right does not push the surrounding text around and create the
// appearance of an inserted space.
func renderInputWithCursor(value string, cursor int) string {
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(value) {
		return value + ui.CursorBlockStyle.Render(" ")
	}
	return value[:cursor] + ui.CursorBlockStyle.Render(string(value[cursor])) + value[cursor+1:]
}
