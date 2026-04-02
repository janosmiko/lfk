package app

import (
	"fmt"
	"strings"

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

func (m Model) breadcrumb() string {
	parts := []string{"lfk"}
	if m.nav.Context != "" {
		parts = append(parts, m.nav.Context)
	}
	if m.nav.ResourceType.DisplayName != "" {
		parts = append(parts, m.nav.ResourceType.DisplayName)
	}
	if m.nav.ResourceName != "" {
		parts = append(parts, m.nav.ResourceName)
	}
	if m.nav.OwnedName != "" {
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
		prompt := ui.HelpKeyStyle.Render(":") + m.commandBarInput.CursorLeft() + ui.BarDimStyle.Render("\u2588") + m.commandBarInput.CursorRight()
		if len(m.commandBarSuggestions) > 0 {
			prompt += "  "
			for i, s := range m.commandBarSuggestions {
				if i == m.commandBarSelectedSuggestion {
					prompt += ui.OverlaySelectedStyle.Render(" "+s+" ") + " "
				} else {
					prompt += ui.BarDimStyle.Render(s) + " "
				}
			}
		}
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).Render(prompt)
	}

	// Show filter/search input in status bar when active.
	if m.filterActive {
		prompt := ui.HelpKeyStyle.Render("filter") + ui.BarDimStyle.Render(": ") + m.filterInput.CursorLeft() + ui.BarDimStyle.Render("\u2588") + m.filterInput.CursorRight()
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).Render(prompt)
	}
	if m.searchActive {
		prompt := ui.HelpKeyStyle.Render("search") + ui.BarDimStyle.Render(": ") + m.searchInput.CursorLeft() + ui.BarDimStyle.Render("\u2588") + m.searchInput.CursorRight()
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
	}
	parts = append(parts, ui.FormatHintParts(hintEntries))

	content := strings.Join(parts, "  ")
	return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(content)
}

// --- Overlay rendering ---

func (m Model) renderOverlay(background string) string {
	var content string
	var overlayW, overlayH int

	switch m.overlay {
	case overlayNamespace:
		content = ui.RenderNamespaceOverlay(m.filteredOverlayItems(), m.overlayFilter.Value, m.overlayCursor, m.namespace, m.allNamespaces, m.selectedNamespaces, m.nsFilterMode)
		overlayW = min(60, m.width-10)
		overlayH = min(20, m.height-6)
	case overlayAction:
		overlayW = min(70, m.width-10)
		content = ui.RenderActionOverlay(m.overlayItems, m.overlayCursor, overlayW)
		overlayH = min(15, m.height-6)
	case overlayQuitConfirm:
		content = ui.RenderQuitConfirmOverlay()
		overlayW = min(40, m.width-10)
		overlayH = min(7, m.height-6)
	case overlayConfirm:
		content = ui.RenderConfirmOverlay(m.confirmAction)
		overlayW = min(50, m.width-10)
		overlayH = min(8, m.height-6)
	case overlayConfirmType:
		content = ui.RenderConfirmTypeOverlay(m.confirmTitle, m.confirmQuestion, m.confirmTypeInput.Value)
		overlayW = min(55, m.width-10)
		overlayH = min(10, m.height-6)
	case overlayScaleInput:
		content = ui.RenderScaleOverlay(m.scaleInput.Value)
		overlayW = min(45, m.width-10)
		overlayH = min(8, m.height-6)
	case overlayPVCResize:
		content = ui.RenderPVCResizeOverlay(m.scaleInput.Value, m.pvcCurrentSize)
		overlayW = min(45, m.width-10)
		overlayH = min(10, m.height-6)
	case overlayPortForward:
		content = ui.RenderPortForwardOverlay(m.portForwardInput.Value, m.pfAvailablePorts, m.pfPortCursor, m.actionCtx.name)
		overlayW = min(55, m.width-10)
		overlayH = min(5+len(m.pfAvailablePorts)+4, m.height-6)
	case overlayContainerSelect:
		content = ui.RenderContainerSelectOverlay(m.overlayItems, m.overlayCursor)
		overlayW = min(50, m.width-10)
		overlayH = min(15, m.height-6)
	case overlayPodSelect, overlayLogPodSelect:
		content = ui.RenderPodSelectOverlay(m.filteredLogPodItems(), m.overlayCursor, m.logPodFilterText, m.logPodFilterActive)
		overlayW = min(60, m.width-10)
		overlayH = min(20, m.height-6)
	case overlayLogContainerSelect:
		canSwitchPod := m.logParentKind != ""
		content = ui.RenderLogContainerSelectOverlay(m.filteredLogContainerItems(), m.overlayCursor, m.logSelectedContainers, m.logContainerFilterText, m.logContainerFilterActive, canSwitchPod)
		overlayW = min(60, m.width-10)
		overlayH = min(len(m.filteredLogContainerItems())+9, m.height-6)
	case overlayBookmarks:
		overlayW = min(90, m.width-10)
		overlayH = min(25, m.height-6)
		content = ui.RenderBookmarkOverlay(m.bookmarks, m.bookmarkFilter.Value, m.overlayCursor, int(m.bookmarkSearchMode))
	case overlayTemplates:
		filtered := m.filteredTemplates()
		overlayW = min(60, m.width-10)
		overlayH = min(25, m.height-6)
		content = ui.RenderTemplateOverlay(filtered, m.templateFilter.Value, m.templateCursor, m.templateSearchMode, overlayH)
	case overlayColorscheme:
		content = ui.RenderColorschemeOverlay(m.schemeEntries, m.schemeFilter.Value, m.schemeCursor, m.schemeFilterMode)
		overlayW = min(50, m.width-10)
		overlayH = min(22, m.height-6)
	case overlayFilterPreset:
		var activePresetName string
		if m.activeFilterPreset != nil {
			activePresetName = m.activeFilterPreset.Name
		}
		entries := make([]ui.FilterPresetEntry, len(m.filterPresets))
		for i, p := range m.filterPresets {
			entries[i] = ui.FilterPresetEntry{Name: p.Name, Description: p.Description, Key: p.Key}
		}
		content = ui.RenderFilterPresetOverlay(entries, m.overlayCursor, activePresetName)
		overlayW = min(55, m.width-10)
		overlayH = min(15, m.height-6)
	case overlayRBAC:
		entries := make([]ui.RBACCheckEntry, len(m.rbacResults))
		for i, r := range m.rbacResults {
			entries[i] = ui.RBACCheckEntry{Verb: r.Verb, Allowed: r.Allowed}
		}
		content = ui.RenderRBACOverlay(entries, m.rbacKind)
		overlayW = min(45, m.width-10)
		overlayH = min(15, m.height-6)
	case overlayBatchLabel:
		content = ui.RenderBatchLabelOverlay(m.batchLabelMode, m.batchLabelInput.Value, m.batchLabelRemove)
		overlayW = min(50, m.width-10)
		overlayH = min(12, m.height-6)
	case overlayPodStartup:
		if m.podStartupData != nil {
			entry := ui.PodStartupEntry{
				PodName:   m.podStartupData.PodName,
				Namespace: m.podStartupData.Namespace,
				TotalTime: m.podStartupData.TotalTime,
			}
			for _, p := range m.podStartupData.Phases {
				entry.Phases = append(entry.Phases, ui.StartupPhaseEntry{
					Name:     p.Name,
					Duration: p.Duration,
					Status:   p.Status,
				})
			}
			content = ui.RenderPodStartupOverlay(entry)
		}
		overlayW = min(70, m.width-10)
		overlayH = min(25, m.height-6)
	case overlayQuotaDashboard:
		entries := make([]ui.QuotaEntry, len(m.quotaData))
		for i, q := range m.quotaData {
			resources := make([]ui.QuotaResourceEntry, len(q.Resources))
			for j, r := range q.Resources {
				resources[j] = ui.QuotaResourceEntry{
					Name:    r.Name,
					Hard:    r.Hard,
					Used:    r.Used,
					Percent: r.Percent,
				}
			}
			entries[i] = ui.QuotaEntry{
				Name:      q.Name,
				Namespace: q.Namespace,
				Resources: resources,
			}
		}
		content = ui.RenderQuotaDashboardOverlay(entries, min(80, m.width-10), min(30, m.height-6))
		overlayW = min(80, m.width-10)
		overlayH = min(30, m.height-6)
	case overlayEventTimeline:
		entries := make([]ui.EventTimelineEntry, len(m.eventTimelineData))
		for i, e := range m.eventTimelineData {
			entries[i] = ui.EventTimelineEntry{
				Timestamp:    e.Timestamp,
				Type:         e.Type,
				Reason:       e.Reason,
				Message:      e.Message,
				Source:       e.Source,
				Count:        e.Count,
				InvolvedName: e.InvolvedName,
				InvolvedKind: e.InvolvedKind,
			}
		}
		overlayW = min(100, m.width-6)
		overlayH = min(30, m.height-4)
		content = ui.RenderEventTimelineOverlay(entries, m.actionCtx.name, m.eventTimelineScroll, overlayW, overlayH)
	case overlayAlerts:
		entries := make([]ui.AlertEntry, len(m.alertsData))
		for i, a := range m.alertsData {
			entries[i] = ui.AlertEntry{
				Name:        a.Name,
				State:       a.State,
				Severity:    a.Severity,
				Summary:     a.Summary,
				Description: a.Description,
				Since:       a.Since,
				GrafanaURL:  a.GrafanaURL,
			}
		}
		content = ui.RenderAlertsOverlay(entries, m.alertsScroll, m.width-10, m.height-6)
		overlayW = min(80, m.width-10)
		overlayH = min(25, m.height-6)
	case overlayCanI:
		return m.renderCanIOverlay(background)
	case overlayCanISubject:
		// Render the Can-I browser as the background, then overlay the subject selector on top.
		canIBg := m.renderCanIOverlay(background)
		overlayW = min(80, m.width-10)
		overlayH = min(20, m.height-6)
		content = ui.RenderCanISubjectOverlay(m.filteredOverlayItems(), m.overlayFilter.Value, m.overlayCursor, m.canISubjectFilterMode)
		content = ui.FillLinesBg(content, overlayW-4, ui.SurfaceBg)
		overlay := ui.OverlayStyle.Width(overlayW).Height(overlayH).Render(content)
		return ui.PlaceOverlay(m.width, m.height, overlay, canIBg)
	case overlayExplainSearch:
		overlayW = min(m.width-6, m.width*70/100)
		overlayH = min(m.height-4, m.height*70/100)
		maxVisible := max(overlayH-6, 1) // header + filter + footer
		filtered := m.filteredExplainRecursiveResults()
		content = ui.RenderExplainSearchOverlay(filtered, m.explainRecursiveCursor, m.explainRecursiveScroll, maxVisible, m.explainRecursiveFilter.Value, m.explainRecursiveFilterActive)
	case overlayNetworkPolicy:
		if m.netpolData != nil {
			entry := ui.NetworkPolicyEntry{
				Name:         m.netpolData.Name,
				Namespace:    m.netpolData.Namespace,
				PodSelector:  m.netpolData.PodSelector,
				PolicyTypes:  m.netpolData.PolicyTypes,
				AffectedPods: m.netpolData.AffectedPods,
			}
			for _, r := range m.netpolData.IngressRules {
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
				entry.IngressRules = append(entry.IngressRules, re)
			}
			for _, r := range m.netpolData.EgressRules {
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
				entry.EgressRules = append(entry.EgressRules, re)
			}
			overlayW = min(100, m.width-6)
			overlayH = min(35, m.height-4)
			// Build the bordered box without fixed Height to avoid clipping.
			innerW := overlayW - 4 // account for OverlayStyle Padding(1,2) = 2 chars each side
			innerH := overlayH - 2 // account for OverlayStyle Padding(1,2) = 1 line top + 1 line bottom
			netpolContent := ui.RenderNetworkPolicyOverlay(entry, m.netpolScroll, innerW, innerH)
			netpolContent = ui.FillLinesBg(netpolContent, innerW, ui.SurfaceBg)
			overlay := ui.OverlayStyle.Width(overlayW).Render(netpolContent)
			bg := ui.PadToHeight(background, m.height)
			return ui.PlaceOverlay(m.width, m.height, overlay, bg)
		}
	case overlaySecretEditor:
		overlay := ui.RenderSecretEditorOverlay(
			m.secretData, m.secretCursor, m.secretRevealed, m.secretAllRevealed,
			m.secretEditing, m.secretEditKey.Value, m.secretEditValue.Value, m.secretEditColumn,
			m.width, m.height,
		)
		bg := ui.PadToHeight(background, m.height)
		return ui.PlaceOverlay(m.width, m.height, overlay, bg)
	case overlayConfigMapEditor:
		overlay := ui.RenderConfigMapEditorOverlay(
			m.configMapData, m.configMapCursor,
			m.configMapEditing, m.configMapEditKey.Value, m.configMapEditValue.Value, m.configMapEditColumn,
			m.width, m.height,
		)
		bg := ui.PadToHeight(background, m.height)
		return ui.PlaceOverlay(m.width, m.height, overlay, bg)
	case overlayRollback:
		overlay := ui.RenderRollbackOverlay(m.rollbackRevisions, m.rollbackCursor, m.width, m.height)
		bg := ui.PadToHeight(background, m.height)
		return ui.PlaceOverlay(m.width, m.height, overlay, bg)
	case overlayHelmRollback:
		overlay := ui.RenderHelmRollbackOverlay(m.helmRollbackRevisions, m.helmRollbackCursor, m.width, m.height)
		bg := ui.PadToHeight(background, m.height)
		return ui.PlaceOverlay(m.width, m.height, overlay, bg)
	case overlayLabelEditor:
		overlay := ui.RenderLabelEditorOverlay(
			m.labelData, m.labelCursor, m.labelTab,
			m.labelEditing, m.labelEditKey.Value, m.labelEditValue.Value, m.labelEditColumn,
			m.width, m.height,
		)
		bg := ui.PadToHeight(background, m.height)
		return ui.PlaceOverlay(m.width, m.height, overlay, bg)
	case overlayAutoSync:
		overlay := ui.RenderAutoSyncOverlay(
			m.autoSyncEnabled, m.autoSyncSelfHeal, m.autoSyncPrune,
			m.autoSyncCursor, m.width, m.height,
		)
		bg := ui.PadToHeight(background, m.height)
		return ui.PlaceOverlay(m.width, m.height, overlay, bg)
	case overlayColumnToggle:
		filtered := m.filteredColumnToggleItems()
		entries := make([]ui.ColumnToggleEntry, len(filtered))
		for i, e := range filtered {
			entries[i] = ui.ColumnToggleEntry{Key: e.key, Visible: e.visible}
		}
		content = ui.RenderColumnToggleOverlay(entries, m.columnToggleCursor, m.columnToggleFilter, m.columnToggleFilterActive, m.width, m.height)
		overlayW = min(50, m.width-10)
		overlayH = min(20, m.height-6)
	case overlayFinalizerSearch:
		filtered := m.filteredFinalizerResults()
		entries := make([]ui.FinalizerMatchEntry, len(filtered))
		for i, r := range filtered {
			entries[i] = ui.FinalizerMatchEntry{
				Name:      r.Name,
				Namespace: r.Namespace,
				Kind:      r.Kind,
				Matched:   r.Matched,
				Age:       r.Age,
			}
		}
		overlayW = min(m.width-6, m.width*80/100)
		if overlayW < 60 {
			overlayW = min(60, m.width-4)
		}
		overlayH = min(m.height-4, m.height*70/100)
		content = ui.RenderFinalizerSearchOverlay(
			entries,
			m.finalizerSearchCursor,
			m.finalizerSearchSelected,
			m.finalizerSearchPattern,
			m.finalizerSearchFilter,
			m.finalizerSearchFilterActive,
			m.finalizerSearchLoading,
			overlayW, overlayH,
		)
	default:
		return background
	}

	if overlayW < 10 {
		overlayW = 10
	}
	if overlayH < 3 {
		overlayH = 3
	}

	content = ui.FillLinesBg(content, overlayW-4, ui.SurfaceBg) // -4 for OverlayStyle padding(1,2)
	overlay := ui.OverlayStyle.Width(overlayW).Height(overlayH).Render(content)

	// Ensure background has exactly m.height lines for correct overlay placement.
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
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
		// Pad to full height so there's no background bleed.
		lines := strings.Split(content, "\n")
		for len(lines) < m.height-1 {
			lines = append(lines, "")
		}
		return strings.Join(lines[:m.height-1], "\n")
	}

	overlayW := min(140, m.width-4)
	overlayH := min(30, m.height-4)
	if overlayW < 10 {
		overlayW = 10
	}
	if overlayH < 3 {
		overlayH = 3
	}

	content := ui.RenderErrorLogOverlay(m.errorLog, m.errorLogScroll, overlayH, m.showDebugLogs, vp)
	content = ui.FillLinesBg(content, overlayW-4, ui.SurfaceBg)
	overlay := ui.OverlayStyle.Width(overlayW).Height(overlayH).Render(content)
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
}
