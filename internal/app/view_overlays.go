package app

import (
	"fmt"
	"strings"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- Overlay rendering ---
//
// All overlay rendering helpers live in this file. They were extracted
// from view_status.go to keep that file focused on the status-bar /
// breadcrumb / column-header concerns and to bring it back under the
// 800-line guideline. The dispatch entry point is renderOverlay, which
// either delegates to a fullscreen-overlay renderer or composes a
// centered overlay box via renderOverlayContent.

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
		// Width: outer 32, inner = 32 − 2(border) − 4(left+right padding) = 26.
		//
		// Height is trickier than the comment used to claim. OverlayStyle
		// renders with Height(qh) and a Border, but `Height` in lipgloss
		// counts the inner area + padding (NOT the border) — so the visible
		// outer height is qh+2. To land "Quit lfk?" on the visual middle
		// row we ship a content slice that exactly fills the inner area
		// (qh − 2 rows after the 1+1 padding), letting the renderer's own
		// `Align(Center, Center)` do the vertical centering. Setting qh=3
		// gives a 5-row outer box: border / padding / Quit lfk? / padding
		// / border, with the text on the middle row.
		qw := min(32, m.width-10)
		qh := min(3, m.height-6)
		return ui.RenderQuitConfirmOverlay(qw-6, qh-2), qw, qh, true
	case overlayConfirm:
		return ui.RenderConfirmOverlay(m.confirmAction), min(50, m.width-10), min(8, m.height-6), true
	case overlayConfirmType:
		return ui.RenderConfirmTypeOverlay(m.confirmTitle, m.confirmQuestion, m.confirmTypeInput.Value), min(55, m.width-10), min(10, m.height-6), true
	case overlayScaleInput:
		return ui.RenderScaleOverlay(m.scaleInput.Value), min(45, m.width-10), min(8, m.height-6), true
	case overlayPVCResize:
		return ui.RenderPVCResizeOverlay(m.scaleInput.Value, m.pvcCurrentSize), min(45, m.width-10), min(10, m.height-6), true
	case overlayPortForward:
		content := ui.RenderPortForwardOverlay(m.portForwardInput.Value, m.pfAvailablePorts, m.pfPortCursor, m.actionCtx.name)
		return content, min(55, m.width-10), min(5+len(m.pfAvailablePorts)+4, m.height-6), true
	case overlayContainerSelect:
		return ui.RenderContainerSelectOverlay(m.overlayItems, m.overlayCursor), min(50, m.width-10), min(15, m.height-6), true
	case overlayPodSelect, overlayLogPodSelect:
		content := ui.RenderPodSelectOverlay(m.filteredLogPodItems(), m.overlayCursor, m.logPodFilterText, m.logPodFilterActive)
		return content, min(60, m.width-10), min(20, m.height-6), true
	case overlayLogContainerSelect:
		content := ui.RenderLogContainerSelectOverlay(m.filteredLogContainerItems(), m.overlayCursor, m.logSelectedContainers, m.logContainerFilterText, m.logContainerFilterActive, m.logParentKind != "")
		return content, min(60, m.width-10), min(len(m.filteredLogContainerItems())+9, m.height-6), true
	case overlayBookmarks:
		w, h := min(90, m.width-10), min(25, m.height-6)
		return ui.RenderBookmarkOverlay(m.bookmarks, m.bookmarkFilter.Value, m.overlayCursor, int(m.bookmarkSearchMode), m.bookmarkLoadNamespace), w, h, true
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
	case overlayClusterColor:
		content := ui.RenderClusterColorOverlay(
			m.clusterColorOverlayContext,
			m.filteredClusterColorNames(),
			m.clusterColorOverlayCursor,
			m.clusterColorFilter.Value,
			m.clusterColorFilterMode,
		)
		return content, min(40, m.width-10), min(15, m.height-6), true
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
	overlayW := min(72, m.width-10)
	// OverlayStyle reserves 4 cells horizontally for Padding(1, 2).
	contentW := max(overlayW-4, 0)
	return ui.RenderFilterPresetOverlay(entries, m.overlayCursor, activePresetName, contentW), overlayW, min(15, m.height-6)
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
	// Pass the overlay box dimensions (not the full screen) so the
	// renderer's maxVisible cap matches what fits inside the box.
	// Otherwise on a tall terminal the renderer emits ~34 lines into a
	// 20-tall box; the box visibly grew on overflow and "shrank" back
	// as the filter narrowed results — looked like the window was
	// resizing.
	overlayW := min(50, m.width-10)
	overlayH := min(20, m.height-6)
	return ui.RenderColumnToggleOverlay(entries, m.columnToggleCursor, m.columnToggleFilter, m.columnToggleFilterActive, overlayW, overlayH),
		overlayW, overlayH
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

// renderCanIOverlay renders the Can-I browser overlay on top of the
// given background. In Who-Can mode the same overlay frame hosts the
// reverse-RBAC view via renderWhoCanInner — same dimensions, same wrap,
// just different content.
func (m Model) renderCanIOverlay(background string) string {
	if m.canIMode == canIModeWhoCan {
		return m.renderWhoCanOverlay(background)
	}
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
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.canISearchInput.CursorLeft()) + ui.BarDimStyle.Render("█") + ui.BarNormalStyle.Render(m.canISearchInput.CursorRight())
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
		// Fullscreen rendering is handled by viewExplorer via the
		// viewErrorLogFullscreen helper (same pattern as the dashboard
		// fullscreen). The background passed in here is already that
		// composed view, so just return it unchanged.
		return background
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
