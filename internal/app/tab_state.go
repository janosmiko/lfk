package app

import (
	"strings"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// Tab lifecycle: saving/restoring/cloning a TabState snapshot and the tab
// identity used to attribute background work. Split out of tabs.go to keep
// that file under the length cap.

// tabUIDSeq issues tab UIDs. Process-global so every tab of every Model
// gets a distinct value.
var tabUIDSeq atomic.Uint64

// tabLabels builds a display label for each tab. Inactive tabs render from
// their saved TabState; the active tab is overridden with the live model
// state so navigation within a tab updates its label immediately.
func (m Model) tabLabels() []string {
	labels := make([]string, len(m.tabs))
	for i, t := range m.tabs {
		labels[i] = labelForNav(t.nav)
	}
	labels[m.activeTab] = labelForNav(m.nav)
	return labels
}

// labelForNav builds a "context/Type/Name/Owned" label that grows as the user
// drills deeper into the resource hierarchy. RenderTabBar truncates long
// labels by chopping the prefix and keeping the suffix, so the most-specific
// (and most useful) part of the path always wins for screen space.
//
// The resource type label goes through model.DisplayNameFor because
// API-discovery-produced ResourceTypeEntry values do NOT populate
// DisplayName themselves — only the curated metadata table does. Reading
// nav.ResourceType.DisplayName directly silently drops the type for almost
// every real-world resource.
func labelForNav(nav model.NavigationState) string {
	if nav.Context == "" {
		return "clusters"
	}
	parts := []string{nav.Context}
	if name := model.DisplayNameFor(nav.ResourceType); name != "" {
		parts = append(parts, name)
	}
	if nav.ResourceName != "" {
		parts = append(parts, nav.ResourceName)
	}
	// navigateChildResource sets both ResourceName and OwnedName to the same
	// value when entering a Pod (so the containers view knows its parent).
	// Skip the duplicate so the label reads "ctx/Pods/my-pod" instead of
	// "ctx/Pods/my-pod/my-pod".
	if nav.OwnedName != "" && nav.OwnedName != nav.ResourceName {
		parts = append(parts, nav.OwnedName)
	}
	return strings.Join(parts, "/")
}

// saveCurrentTab persists Model fields into the current TabState.
func (m *Model) saveCurrentTab() {
	t := &m.tabs[m.activeTab]
	t.nav = m.nav
	t.leftItems = append([]model.Item(nil), m.leftItems...)
	t.middleItems = append([]model.Item(nil), m.middleItems...)
	t.rightItems = append([]model.Item(nil), m.rightItems...)
	t.leftItemsHistory = make([][]model.Item, len(m.leftItemsHistory))
	for i, hist := range m.leftItemsHistory {
		t.leftItemsHistory[i] = append([]model.Item(nil), hist...)
	}
	t.jumpBackStack = cloneNavSnapshots(m.jumpBackStack)
	t.cursors = m.cursors
	t.middleScroll = ui.ActiveMiddleScroll
	t.leftScroll = ui.ActiveLeftScroll
	t.cursorMemory = copyMapStringInt(m.cursorMemory)
	t.filterMemory = copyMapStringSavedFilter(m.filterMemory)
	t.sortMemory = copyMapStringSortPref(m.sortMemory)
	t.itemCache = copyItemCache(m.itemCache)
	t.cacheFingerprints = copyMapStringString(m.cacheFingerprints)
	t.yamlContent = m.yamlView.content
	t.yamlScroll = m.yamlView.scroll
	t.yamlCursor = m.yamlView.cursor
	t.yamlScrollOption = m.yamlView.scrollOption
	t.yamlSearchText = m.yamlView.searchText
	t.yamlMatchLines = m.yamlView.matchLines
	t.yamlMatchIdx = m.yamlView.matchIdx
	t.yamlCollapsed = copyMapStringBool(m.yamlView.collapsed)
	t.splitPreview = m.splitPreview
	t.fullYAMLPreview = m.fullYAMLPreview
	t.fullLogPreview = m.fullLogPreview
	t.previewYAML = m.previewYAML
	t.namespace = m.namespace
	t.allNamespaces = m.allNamespaces
	t.selectedNamespaces = copyMapStringBool(m.selectedNamespaces)
	t.nsSelectionNegated = m.nsSelectionNegated
	t.savedSelectedNamespaces = copyMapStringBool(m.savedSelectedNamespaces)
	t.savedNsSelectionNegated = m.savedNsSelectionNegated
	t.previousNsScope = m.previousNsScope.clone()
	t.sortColumnName = m.sortColumnName
	t.sortAscending = m.sortAscending
	t.filterText = m.filterText
	t.filterBroadMode = m.filterBroadMode
	t.activeFilterPreset = m.activeFilterPreset
	t.unfilteredMiddleItems = append([]model.Item(nil), m.unfilteredMiddleItems...)
	t.cursorName, t.cursorNamespace, _, _ = m.cursorItemKey()
	t.watchMode = m.watchMode
	t.objectExplorerLive = m.objectExplorerLive
	t.objectExplorerTree = m.objectExplorerTree
	t.readOnly = m.readOnly
	t.requestGen = m.requestGen
	t.selectedItems = copyMapStringBool(m.selectedItems)
	t.selectionAnchor = m.selectionAnchor
	t.fullscreenMiddle = m.fullscreenMiddle
	t.fullscreenDashboard = m.fullscreenDashboard
	t.hideLeftPane = m.hideLeftPane
	t.dashboardPreview = m.dashboardPreview
	t.dashboardEventsPreview = m.dashboardEventsPreview
	t.monitoringPreview = m.monitoringPreview
	t.metricsContent = m.metricsContent
	t.previewEventsContent = m.previewEventsContent
	t.metricsData = m.metricsData
	t.metricsLoading = m.metricsLoading
	t.previewEventsData = m.previewEventsData
	t.warningEventsOnly = m.warningEventsOnly
	t.eventGrouping = m.eventGrouping
	t.expandedGroup = m.expandedGroup
	t.allGroupsExpanded = m.allGroupsExpanded
	t.mode = m.mode
	t.logLines = append([]string(nil), m.logView.rawLines...)
	t.logFilterQuery = m.logView.filterQuery
	t.logSevThreshold = m.logView.sevThreshold
	t.logScroll = m.logView.scroll
	t.logWrapTopSkip = m.logView.wrapTopSkip
	t.logFollow = m.logView.follow
	t.logWrap = m.logView.wrap
	t.logLineNumbers = m.logView.lineNumbers
	t.logTimestamps = m.logView.timestamps
	t.logPrevious = m.logView.previous
	t.logIsMulti = m.logView.isMulti
	t.logTitle = m.logView.title
	t.logCancel = m.logView.cancel
	t.logCh = m.logView.ch
	t.logTailLines = m.logView.tailLines
	t.logHasMoreHistory = m.logView.hasMoreHistory
	t.logLoadingHistory = m.logView.loadingHistory
	t.logCursor = m.logView.cursor
	t.logVisualMode = m.logView.visualMode
	t.logVisualStart = m.logView.visualStart
	t.logVisualType = m.logView.visualType
	t.logVisualCol = m.logView.visualCol
	t.logVisualCurCol = m.logView.visualCurCol
	t.logScrollOption = m.logView.scrollOption
	t.logParentKind = m.logView.parentKind
	t.logParentName = m.logView.parentName
	t.logSavedPodName = m.logView.savedPodName
	t.logContainers = append([]string(nil), m.logView.containers...)
	t.logSelectedContainers = append([]string(nil), m.logView.selectedContainers...)
	t.describeContent = m.describeView.content
	t.describeScroll = m.describeView.scroll
	t.describeTitle = m.describeView.title
	t.diffLeft = m.diffView.left
	t.diffRight = m.diffView.right
	t.diffLeftName = m.diffView.leftName
	t.diffRightName = m.diffView.rightName
	t.diffScroll = m.diffView.scroll
	t.diffUnified = m.diffView.unified
	t.execPTY = m.execPTY
	t.execTerm = m.execTerm
	t.execTitle = m.execTitle
	t.execDone = m.execDone
	t.execMu = m.execMu
	t.execScrollback = m.execScrollback
	t.execScrollOffset = m.execScrollOffset
	t.explainFields = append([]model.ExplainField(nil), m.explainFields...)
	t.explainDesc = m.explainDesc
	t.explainPath = m.explainPath
	t.explainResource = m.explainResource
	t.explainAPIVersion = m.explainAPIVersion
	t.explainTitle = m.explainTitle
	t.explainPending = m.explainPending
	t.explainCursor = m.explainCursor
	t.explainScroll = m.explainScroll
	t.explainSearchQuery = m.explainSearchQuery
	m.saveLogTopToTab(t)
	m.saveSecurityStateToTab(t)
	// Cache the current preview buffer then cancel the stream so it does not
	// outlive the tab. fullLogPreview is persisted above, so the next loadTab
	// knows to restart (and can restore from cache) if needed.
	m.cachePreviewLog()
	m.cancelPreviewLogStream()
}

// loadTab restores Model fields from the given tab index.
// It returns a tea.Cmd that fetches the tab's data when the tab was restored
// from a session (needsLoad), and one that resumes an unfinished API Explorer.
func (m *Model) loadTab(idx int) tea.Cmd {
	m.wheel.dead = true // switching/reloading a tab empties the wheel momentum queue (#524)
	// Stop the explain fetches of the tab being left, and their spinner too.
	m.cancelExplainSession()
	m.loading = false
	t := m.tabs[idx]
	needsLoad := t.needsLoad
	m.activeTab = idx
	m.nav = t.nav
	m.leftItems = append([]model.Item(nil), t.leftItems...)
	m.setMiddleItems(append([]model.Item(nil), t.middleItems...))
	m.rightItems = append([]model.Item(nil), t.rightItems...)
	m.leftItemsHistory = make([][]model.Item, len(t.leftItemsHistory))
	for i, hist := range t.leftItemsHistory {
		m.leftItemsHistory[i] = append([]model.Item(nil), hist...)
	}
	m.jumpBackStack = cloneNavSnapshots(t.jumpBackStack)
	m.cursors = t.cursors
	ui.ActiveMiddleScroll = t.middleScroll
	ui.ActiveLeftScroll = t.leftScroll
	m.cursorMemory = copyMapStringInt(t.cursorMemory)
	m.filterMemory = copyMapStringSavedFilter(t.filterMemory)
	m.sortMemory = copyMapStringSortPref(t.sortMemory)
	m.itemCache = copyItemCache(t.itemCache)
	m.cacheFingerprints = copyMapStringString(t.cacheFingerprints)
	m.yamlView.content = t.yamlContent
	m.yamlView.scroll = t.yamlScroll
	m.yamlView.cursor = t.yamlCursor
	m.yamlView.scrollOption = t.yamlScrollOption
	m.yamlView.searchText = t.yamlSearchText
	m.yamlView.matchLines = t.yamlMatchLines
	m.yamlView.matchIdx = t.yamlMatchIdx
	m.yamlView.collapsed = copyMapStringBool(t.yamlCollapsed)
	m.splitPreview = t.splitPreview
	m.fullYAMLPreview = t.fullYAMLPreview
	m.fullLogPreview = t.fullLogPreview
	m.previewYAML = t.previewYAML
	m.namespace = t.namespace
	m.allNamespaces = t.allNamespaces
	m.selectedNamespaces = copyMapStringBool(t.selectedNamespaces)
	m.nsSelectionNegated = t.nsSelectionNegated
	m.savedSelectedNamespaces = copyMapStringBool(t.savedSelectedNamespaces)
	m.savedNsSelectionNegated = t.savedNsSelectionNegated
	m.previousNsScope = t.previousNsScope.clone()
	m.sortColumnName = t.sortColumnName
	m.sortAscending = t.sortAscending
	m.filterText = t.filterText
	m.filterBroadMode = t.filterBroadMode
	m.filterInput.Set(t.filterText)
	m.activeFilterPreset = t.activeFilterPreset
	m.unfilteredMiddleItems = append([]model.Item(nil), t.unfilteredMiddleItems...)
	m.watchMode = t.watchMode
	m.objectExplorerLive = t.objectExplorerLive
	m.objectExplorerTree = t.objectExplorerTree
	m.readOnly = t.readOnly
	m.requestGen = t.requestGen
	m.selectedItems = copyMapStringBool(t.selectedItems)
	m.selectionAnchor = t.selectionAnchor
	m.fullscreenMiddle = t.fullscreenMiddle
	m.fullscreenDashboard = t.fullscreenDashboard
	m.hideLeftPane = t.hideLeftPane
	m.dashboardPreview = t.dashboardPreview
	m.dashboardEventsPreview = t.dashboardEventsPreview
	m.monitoringPreview = t.monitoringPreview
	// Restore the right-pane footers so the new tab paints its own metrics/events.
	m.metricsContent = t.metricsContent
	m.previewEventsContent = t.previewEventsContent
	m.metricsData = t.metricsData
	m.metricsLoading = t.metricsLoading
	m.previewEventsData = t.previewEventsData
	m.warningEventsOnly = t.warningEventsOnly
	m.eventGrouping = t.eventGrouping
	m.expandedGroup = t.expandedGroup
	m.allGroupsExpanded = t.allGroupsExpanded

	// Restore per-tab view mode and log state.
	m.mode = t.mode
	// A tab restored with the API Explorer open needs its own session, or
	// closing the view could not stop the fetches it starts next.
	if m.mode == modeExplain {
		m.beginExplainSession()
	}
	m.logView.rawLines = append([]string(nil), t.logLines...)
	m.logView.rawSev = nil
	m.logView.filterQuery = t.logFilterQuery
	m.logView.sevThreshold = t.logSevThreshold
	m.logView.scroll = t.logScroll
	m.logView.wrapTopSkip = t.logWrapTopSkip
	m.logView.follow = t.logFollow
	m.logView.wrap = ui.ConfigLogWrap
	m.logView.lineNumbers = t.logLineNumbers
	m.logView.timestamps = t.logTimestamps
	m.logView.previous = t.logPrevious
	m.logView.isMulti = t.logIsMulti
	m.logView.title = t.logTitle
	m.logView.cancel = t.logCancel
	m.logView.ch = t.logCh
	m.logView.tailLines = t.logTailLines
	m.logView.hasMoreHistory = t.logHasMoreHistory
	m.logView.loadingHistory = t.logLoadingHistory
	m.logView.cursor = t.logCursor
	m.logView.visualMode = t.logVisualMode
	m.logView.visualStart = t.logVisualStart
	m.logView.visualType = t.logVisualType
	m.logView.visualCol = t.logVisualCol
	m.logView.visualCurCol = t.logVisualCurCol
	m.logView.scrollOption = t.logScrollOption
	// Rebuild after offset/follow restore so clampLogOffsets sees correct state.
	m.rebuildLogView()
	m.logView.parentKind = t.logParentKind
	m.logView.parentName = t.logParentName
	m.logView.savedPodName = t.logSavedPodName
	m.logView.containers = append([]string(nil), t.logContainers...)
	m.logView.selectedContainers = append([]string(nil), t.logSelectedContainers...)
	m.describeView.content = t.describeContent
	m.describeView.scroll = t.describeScroll
	m.describeView.title = t.describeTitle
	m.diffView.left = t.diffLeft
	m.diffView.right = t.diffRight
	m.diffView.leftName = t.diffLeftName
	m.diffView.rightName = t.diffRightName
	m.diffView.scroll = t.diffScroll
	m.diffView.unified = t.diffUnified
	m.execPTY = t.execPTY
	m.execTerm = t.execTerm
	m.execTitle = t.execTitle
	m.execDone = t.execDone
	m.execMu = t.execMu
	m.execScrollback = t.execScrollback
	m.execScrollOffset = t.execScrollOffset
	m.explainFields = append([]model.ExplainField(nil), t.explainFields...)
	m.explainDesc = t.explainDesc
	m.explainPath = t.explainPath
	m.explainResource = t.explainResource
	m.explainAPIVersion = t.explainAPIVersion
	m.explainTitle = t.explainTitle
	m.explainPending = t.explainPending
	m.explainCursor = t.explainCursor
	m.explainScroll = t.explainScroll
	m.explainSearchQuery = t.explainSearchQuery
	m.loadSecurityStateFromTab(&t)
	m.loadLogTopFromTab(t)
	// Close overlays and reset transient state.
	m.overlay = overlayNone
	// The blast radius belongs to the overlay being closed here, and it is
	// not part of the per-tab snapshot, so it must not follow the user into
	// the next tab.
	m.blast.reset()
	m.deps.reset()
	m.filterActive = false
	m.searchActive = false
	m.err = nil
	m.pendingTextObject = 0

	// Re-annotate cluster picker rows with the current effective read-only
	// state. The override map is per-Model (shared across tabs), but middleItems
	// was captured per-tab — so a Ctrl+R toggle in another tab can leave this
	// tab's [RO] markers stale until the next context reload. Sync them now.
	m.refreshContextReadOnlyMarkers()

	// If this tab was restored from a session but never loaded, clear the flag,
	// set up the navigation column structure, and return a fetch command.
	if needsLoad {
		m.tabs[idx].needsLoad = false
		m.applyPinnedTypes()

		// Rebuild security state for the restored context so the Security sidebar
		// seeds from the on-disk cache. Live availability probing is lazy
		// (maybeProbeSecurityOnFocus): on Security focus, not on every restore.
		securitySeedCmd := m.refreshSecuritySources()

		// Load contexts for the left column breadcrumb. A failure is
		// non-fatal here (the breadcrumb just degrades to empty), but
		// surface it so the read error is not swallowed silently.
		contexts, err := m.client.GetContexts()
		if err != nil {
			m.setErrorFromErr("Failed to load contexts: ", err)
		}
		resourceTypes := model.BuildSidebarItems(model.SeedResources())
		discoveryCtx := m.nav.Context
		if m.unionMode && m.nav.Context == UnionContextSentinel && len(m.unionContexts) > 0 {
			discoveryCtx = m.unionContexts[0]
		}
		if discovered := m.discoveredResources[discoveryCtx]; len(discovered) > 0 {
			resourceTypes = model.BuildSidebarItems(discovered)
		}

		switch m.nav.Level {
		case model.LevelResources:
			// Resources level: left = resource types, history = [contexts].
			m.leftItemsHistory = [][]model.Item{contexts}
			m.leftItems = resourceTypes
			m.setMiddleItems(nil)
			m.clearRight()
			m.setCursor(0)
			m.loading = true
			m.pendingTarget, m.pendingTargetNamespace = t.cursorName, t.cursorNamespace // quit-time row
			return tea.Batch(securitySeedCmd, m.resumeExplainFetch(), m.loadResources(false))
		case model.LevelResourceTypes:
			// At resource types level: left = contexts, middle = resource types.
			m.leftItemsHistory = nil
			m.leftItems = contexts
			m.setMiddleItems(resourceTypes)
			m.itemCache[m.navKey()] = m.middleItems
			m.clearRight()
			m.clampCursor()
			return tea.Batch(securitySeedCmd, m.resumeExplainFetch(), m.loadPreview())
		default:
			// Clusters level or unknown: just load contexts.
			m.loading = true
			return tea.Batch(securitySeedCmd, m.resumeExplainFetch(), m.refreshCurrentLevel())
		}
	}
	return m.resumeExplainFetch()
}

// moveActiveTab reorders the active tab one slot in the given direction
// (-1 = left, +1 = right) and keeps it active at its new index. It is a
// no-op (returns false) when there are fewer than two tabs or the tab is
// already at the edge — the order does not wrap.
//
// This is a pure reorder that does NOT switch tabs: the active tab's live
// state stays in the Model, only its slot and m.activeTab move. So it must
// not call saveCurrentTab — that cancels the live preview-log stream (for a
// real switch loadTab restarts it, but a move never reloads), which would
// silently kill the tail. The active tab's snapshot in m.tabs is left stale
// and is refreshed by the next real switch's saveCurrentTab before it is
// ever loaded; the neighbor's snapshot is already current and just changes
// index.
func (m *Model) moveActiveTab(direction int) bool {
	if len(m.tabs) <= 1 {
		return false
	}
	target := m.activeTab + direction
	if target < 0 || target >= len(m.tabs) {
		return false
	}
	m.tabs[m.activeTab], m.tabs[target] = m.tabs[target], m.tabs[m.activeTab]
	m.activeTab = target
	return true
}

// nextTabUID hands out the stable per-tab identity used to attribute
// background work to the tab that started it. Tab index cannot serve as
// that identity: closing or reordering tabs shifts it.
func nextTabUID() uint64 {
	return tabUIDSeq.Add(1)
}

// currentTabUID returns the active tab's UID, or 0 when there is no tab
// (minimal test fixtures). 0 means "unowned" to the scheduler.
func (m *Model) currentTabUID() uint64 {
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return 0
	}
	if m.tabs[m.activeTab].uid == 0 {
		m.tabs[m.activeTab].uid = nextTabUID()
	}
	return m.tabs[m.activeTab].uid
}

// cloneCurrentTab creates a deep copy of the current model state as a new TabState.
func (m *Model) cloneCurrentTab() TabState {
	newTab := TabState{
		uid:                     nextTabUID(),
		nav:                     m.nav,
		leftItems:               append([]model.Item(nil), m.leftItems...),
		middleItems:             append([]model.Item(nil), m.middleItems...),
		rightItems:              append([]model.Item(nil), m.rightItems...),
		cursors:                 m.cursors,
		middleScroll:            ui.ActiveMiddleScroll,
		leftScroll:              ui.ActiveLeftScroll,
		cursorMemory:            copyMapStringInt(m.cursorMemory),
		filterMemory:            copyMapStringSavedFilter(m.filterMemory),
		sortMemory:              copyMapStringSortPref(m.sortMemory),
		itemCache:               copyItemCache(m.itemCache),
		cacheFingerprints:       copyMapStringString(m.cacheFingerprints),
		yamlContent:             m.yamlView.content,
		yamlCollapsed:           copyMapStringBool(m.yamlView.collapsed),
		splitPreview:            m.splitPreview,
		fullYAMLPreview:         m.fullYAMLPreview,
		fullLogPreview:          m.fullLogPreview,
		previewYAML:             m.previewYAML,
		namespace:               m.namespace,
		allNamespaces:           m.allNamespaces,
		selectedNamespaces:      copyMapStringBool(m.selectedNamespaces),
		nsSelectionNegated:      m.nsSelectionNegated,
		savedSelectedNamespaces: copyMapStringBool(m.savedSelectedNamespaces),
		savedNsSelectionNegated: m.savedNsSelectionNegated,
		previousNsScope:         m.previousNsScope.clone(),
		sortColumnName:          m.sortColumnName,
		sortAscending:           m.sortAscending,
		filterText:              m.filterText,
		watchMode:               m.watchMode,
		objectExplorerLive:      m.objectExplorerLive,
		objectExplorerTree:      m.objectExplorerTree,
		readOnly:                m.readOnly,
		requestGen:              m.requestGen,
		selectedItems:           copyMapStringBool(m.selectedItems),
		selectionAnchor:         m.selectionAnchor,
		fullscreenMiddle:        m.fullscreenMiddle,
		fullscreenDashboard:     m.fullscreenDashboard,
		dashboardPreview:        m.dashboardPreview,
		dashboardEventsPreview:  m.dashboardEventsPreview,
		monitoringPreview:       m.monitoringPreview,
		metricsContent:          m.metricsContent,
		previewEventsContent:    m.previewEventsContent,
		metricsData:             m.metricsData,
		metricsLoading:          m.metricsLoading,
		previewEventsData:       append([]ui.EventTimelineEntry(nil), m.previewEventsData...),
		warningEventsOnly:       m.warningEventsOnly,
		eventGrouping:           m.eventGrouping,
		expandedGroup:           m.expandedGroup,
		allGroupsExpanded:       m.allGroupsExpanded,

		// logFilterQuery/logSevThreshold are intentionally left zero: a cloned tab
		// starts with no active log filter.
		logCursor:       m.logView.cursor,
		logVisualMode:   false, // don't clone visual mode into new tabs
		logVisualStart:  0,
		logVisualType:   'V',
		logVisualCol:    0,
		logVisualCurCol: 0,
	}
	// New tabs inherit the active tab's security state because they
	// start on the same cluster; navigateChildCluster will rebuild
	// them via refreshSecuritySources when the user picks a different
	// context.
	m.saveLogTopToTab(&newTab)
	m.saveSecurityStateToTab(&newTab)
	newTab.leftItemsHistory = make([][]model.Item, len(m.leftItemsHistory))
	for i, hist := range m.leftItemsHistory {
		newTab.leftItemsHistory[i] = append([]model.Item(nil), hist...)
	}
	return newTab
}
