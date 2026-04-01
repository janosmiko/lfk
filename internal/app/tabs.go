package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// pushLeft saves the current leftItems and promotes middleItems to become the new leftItems.
func (m *Model) pushLeft() {
	m.leftItemsHistory = append(m.leftItemsHistory, m.leftItems)
	m.leftItems = m.middleItems
}

// popLeft restores leftItems from the history stack.
func (m *Model) popLeft() {
	n := len(m.leftItemsHistory)
	if n > 0 {
		m.leftItems = m.leftItemsHistory[n-1]
		m.leftItemsHistory = m.leftItemsHistory[:n-1]
	} else {
		m.leftItems = nil
	}
}

// clearRight resets the right column and YAML preview so stale data doesn't linger.
func (m *Model) clearRight() {
	m.rightItems = nil
	m.yamlContent = ""
	m.yamlSections = nil
	m.previewYAML = ""
	m.metricsContent = ""
	m.previewEventsContent = ""
	m.resourceTree = nil
	m.mapView = false
}

// selectedResourceKind returns the Kind of the currently selected resource,
// which is context-dependent on the navigation level.
func (m *Model) selectedResourceKind() string {
	switch m.nav.Level {
	case model.LevelResources:
		return m.nav.ResourceType.Kind
	case model.LevelOwned:
		sel := m.selectedMiddleItem()
		if sel != nil {
			return sel.Kind
		}
	case model.LevelContainers:
		return "Container"
	}
	return ""
}

// effectiveNamespace returns the namespace to use for API calls.
// Returns empty string when allNamespaces is true or multiple namespaces are
// selected (fetches all, filters client-side).
func (m *Model) effectiveNamespace() string {
	if m.allNamespaces || len(m.selectedNamespaces) > 1 {
		return "" // fetch all, filter client-side
	}
	if len(m.selectedNamespaces) == 1 {
		for ns := range m.selectedNamespaces {
			return ns
		}
	}
	return m.namespace
}

// sortMiddleItems sorts middleItems based on the current sort column and direction.
// At LevelResourceTypes and LevelClusters, items keep their original ordering.
func (m *Model) sortMiddleItems() {
	if m.nav.Level == model.LevelResourceTypes || m.nav.Level == model.LevelClusters {
		return
	}

	cols := ui.ActiveSortableColumns
	if len(cols) == 0 {
		return
	}

	colName := m.sortColumnName
	asc := m.sortAscending

	sort.SliceStable(m.middleItems, func(i, j int) bool {
		a, b := m.middleItems[i], m.middleItems[j]
		var less bool
		switch colName {
		case "Name":
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case "Namespace":
			less = strings.ToLower(a.Namespace) < strings.ToLower(b.Namespace)
		case "Ready":
			less = compareReady(a.Ready, b.Ready)
		case "Restarts":
			less = compareNumeric(a.Restarts, b.Restarts)
		case "Status":
			pa, pb := statusPriority(a.Status), statusPriority(b.Status)
			if pa != pb {
				less = pa < pb
			} else {
				less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
			}
		case "Age":
			if a.CreatedAt.IsZero() && b.CreatedAt.IsZero() {
				less = a.Name < b.Name
			} else if a.CreatedAt.IsZero() {
				less = false
			} else if b.CreatedAt.IsZero() {
				less = true
			} else {
				less = a.CreatedAt.After(b.CreatedAt) // newest first is "ascending" for age
			}
		default:
			// Extra column: compare values as strings, with numeric awareness for CPU/MEM.
			va := getColumnValue(a, colName)
			vb := getColumnValue(b, colName)
			if colName == "CPU" || colName == "MEM" || colName == "CPU/R" || colName == "CPU/L" || colName == "MEM/R" || colName == "MEM/L" {
				less = compareResourceValues(va, vb, colName)
			} else {
				less = strings.ToLower(va) < strings.ToLower(vb)
			}
		}
		if !asc {
			return !less
		}
		return less
	})
}

func compareReady(a, b string) bool {
	ra := parseReadyRatio(a)
	rb := parseReadyRatio(b)
	return ra < rb
}

func parseReadyRatio(s string) float64 {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return 0
	}
	num, _ := strconv.ParseFloat(parts[0], 64)
	den, _ := strconv.ParseFloat(parts[1], 64)
	if den == 0 {
		return 0
	}
	return num / den
}

func compareNumeric(a, b string) bool {
	na, _ := strconv.Atoi(strings.TrimSpace(a))
	nb, _ := strconv.Atoi(strings.TrimSpace(b))
	return na < nb
}

func compareResourceValues(a, b, col string) bool {
	isCPU := strings.HasPrefix(col, "CPU")
	va := ui.ParseResourceValue(a, isCPU)
	vb := ui.ParseResourceValue(b, isCPU)
	return va < vb
}

func getColumnValue(item model.Item, key string) string {
	for _, kv := range item.Columns {
		if kv.Key == key {
			return kv.Value
		}
	}
	return ""
}

// statusPriority returns a sort priority for a status string.
func statusPriority(status string) int {
	switch status {
	case "Running", "Active", "Bound", "Available", "Ready", "Healthy", "Healthy/Synced", "Deployed":
		return 0
	case "Pending", "ContainerCreating", "Waiting", "Init", "Progressing", "Progressing/Synced", "Suspended",
		"Pending-install", "Pending-upgrade", "Pending-rollback", "Uninstalling":
		return 1
	case "Failed", "CrashLoopBackOff", "Error", "ImagePullBackOff", "Degraded", "Degraded/OutOfSync":
		return 2
	default:
		return 3
	}
}

// sortModeName returns a display name for the current sort column with direction indicator.
func (m *Model) sortModeName() string {
	if m.sortColumnName != "" {
		dir := "\u2191" // ↑
		if !m.sortAscending {
			dir = "\u2193" // ↓
		}
		return m.sortColumnName + " " + dir
	}
	return "Name \u2191"
}

// sanitizeError strips newlines and truncates an error message for status bar display.
func (m *Model) sanitizeError(err error) string {
	s := strings.ReplaceAll(err.Error(), "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	// Collapse multiple spaces.
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	maxLen := m.width - 20
	if maxLen < 40 {
		maxLen = 40
	}
	if len(s) > maxLen {
		s = s[:maxLen-3] + "..."
	}
	return s
}

// fullErrorMessage returns the full error message with newlines collapsed, for logging.
func fullErrorMessage(err error) string {
	s := strings.ReplaceAll(err.Error(), "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

// sanitizeMessage strips newlines and truncates a string for status bar display.
func (m *Model) sanitizeMessage(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	maxLen := m.width - 6 // account for status bar padding
	if maxLen < 40 {
		maxLen = 40
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		s = string(runes[:maxLen-3]) + "..."
	}
	return s
}

// setStatusMessage sets a temporary status bar message.
// All messages are appended to the application log buffer with appropriate level.
func (m *Model) setStatusMessage(msg string, isErr bool) {
	m.statusMessage = msg
	m.statusMessageErr = isErr
	m.statusMessageExp = time.Now().Add(5 * time.Second)

	level := "INF"
	if isErr {
		level = "ERR"
		logger.Error("Application error", "message", msg)
	} else {
		logger.Info("Status message", "message", msg)
	}
	m.errorLog = append(m.errorLog, ui.ErrorLogEntry{
		Time:    time.Now(),
		Message: msg,
		Level:   level,
	})
	// Keep at most 200 entries (drop oldest).
	if len(m.errorLog) > 200 {
		m.errorLog = m.errorLog[len(m.errorLog)-200:]
	}
}

// setErrorFromErr shows a sanitized error in the status bar and logs the
// full untruncated error to the error log overlay.
func (m *Model) setErrorFromErr(prefix string, err error) {
	// Show truncated version in status bar.
	m.statusMessage = prefix + m.sanitizeError(err)
	m.statusMessageErr = true
	m.statusMessageExp = time.Now().Add(5 * time.Second)

	// Log the full untruncated error to the error log.
	full := fullErrorMessage(err)
	logger.Error("Application error", "message", full)
	m.errorLog = append(m.errorLog, ui.ErrorLogEntry{
		Time:    time.Now(),
		Message: prefix + full,
		Level:   "ERR",
	})
	if len(m.errorLog) > 200 {
		m.errorLog = m.errorLog[len(m.errorLog)-200:]
	}
}

// hasStatusMessage checks whether there's a non-expired status message.
func (m *Model) hasStatusMessage() bool {
	return m.statusMessage != "" && time.Now().Before(m.statusMessageExp)
}

// addLogEntry appends an entry to the in-app error log at the given level.
func (m *Model) addLogEntry(level, msg string) {
	m.errorLog = append(m.errorLog, ui.ErrorLogEntry{
		Time:    time.Now(),
		Message: msg,
		Level:   level,
	})
	if len(m.errorLog) > 500 {
		m.errorLog = m.errorLog[len(m.errorLog)-500:]
	}
}

// portForwardItems returns the list of active port forwards as model.Items for display.
func (m *Model) portForwardItems() []model.Item {
	entries := m.portForwardMgr.Entries()
	items := make([]model.Item, 0, len(entries))
	for _, e := range entries {
		displayLocalPort := e.LocalPort
		if displayLocalPort == "0" {
			displayLocalPort = "..."
		}
		name := fmt.Sprintf("%s/%s  %s:%s", e.ResourceKind, e.ResourceName, displayLocalPort, e.RemotePort)
		extra := fmt.Sprintf("%s/%s", e.Namespace, e.Context)
		status := string(e.Status)
		age := time.Since(e.StartedAt).Truncate(time.Second).String()

		items = append(items, model.Item{
			Name:      name,
			Namespace: e.Namespace,
			Status:    status,
			Kind:      "__port_forward_entry__",
			Extra:     extra,
			Age:       age,
			CreatedAt: e.StartedAt,
			Columns: []model.KeyValue{
				{Key: "ID", Value: fmt.Sprintf("%d", e.ID)},
				{Key: "Local", Value: displayLocalPort},
				{Key: "Remote", Value: e.RemotePort},
				{Key: "Resource", Value: e.ResourceKind + "/" + e.ResourceName},
				{Key: "Status", Value: status},
			},
		})
	}
	return items
}

// navigateToPortForwards switches the view to the Port Forwards resource list.
// If pfLastCreatedID is set, the cursor is placed on the matching entry.
func (m *Model) navigateToPortForwards() {
	// Build the correct left column state for LevelResources.
	contexts, _ := m.client.GetContexts()
	var resourceTypes []model.Item
	if crds := m.discoveredCRDs[m.nav.Context]; len(crds) > 0 {
		resourceTypes = model.MergeWithCRDs(crds)
	} else {
		resourceTypes = model.FlattenedResourceTypes()
	}

	m.nav.ResourceType = model.ResourceTypeEntry{
		DisplayName: "Port Forwards",
		Kind:        "__port_forwards__",
		APIGroup:    "_portforward",
		APIVersion:  "v1",
		Resource:    "portforwards",
		Namespaced:  false,
	}
	m.nav.Level = model.LevelResources
	m.leftItemsHistory = [][]model.Item{contexts}
	m.leftItems = resourceTypes
	m.clearRight()
	m.middleItems = m.portForwardItems()
	m.setCursor(0)
	// Try to position cursor on the newly created port forward.
	if m.pfLastCreatedID > 0 {
		for i, item := range m.middleItems {
			if m.getPortForwardID(item.Columns) == m.pfLastCreatedID {
				m.setCursor(i)
				break
			}
		}
	}
	m.clampCursor()
	m.saveCurrentSession()
}

// getPortForwardID extracts the port forward ID from item columns.
func (m *Model) getPortForwardID(columns []model.KeyValue) int {
	for _, kv := range columns {
		if kv.Key == "ID" {
			id, err := strconv.Atoi(kv.Value)
			if err == nil {
				return id
			}
		}
	}
	return 0
}

// tabLabels builds a display label for each tab.
func (m Model) tabLabels() []string {
	labels := make([]string, len(m.tabs))
	for i, t := range m.tabs {
		if t.nav.Context != "" {
			label := t.nav.Context
			if t.nav.ResourceType.DisplayName != "" {
				label += "/" + t.nav.ResourceType.DisplayName
			}
			labels[i] = label
		} else {
			labels[i] = "clusters"
		}
	}
	// Update current tab label from live model state.
	if m.nav.Context != "" {
		label := m.nav.Context
		if m.nav.ResourceType.DisplayName != "" {
			label += "/" + m.nav.ResourceType.DisplayName
		}
		labels[m.activeTab] = label
	} else {
		labels[m.activeTab] = "clusters"
	}
	return labels
}

// saveCurrentTab persists Model fields into the current TabState.
func (m *Model) saveCurrentTab() {
	t := &m.tabs[m.activeTab]
	t.nav = m.nav
	t.leftItems = append([]model.Item(nil), m.leftItems...)
	t.middleItems = append([]model.Item(nil), m.middleItems...)
	t.rightItems = append([]model.Item(nil), m.rightItems...)
	// Deep copy leftItemsHistory.
	t.leftItemsHistory = make([][]model.Item, len(m.leftItemsHistory))
	for i, hist := range m.leftItemsHistory {
		t.leftItemsHistory[i] = append([]model.Item(nil), hist...)
	}
	t.cursors = m.cursors
	t.middleScroll = ui.ActiveMiddleScroll
	t.leftScroll = ui.ActiveLeftScroll
	t.cursorMemory = copyMapStringInt(m.cursorMemory)
	t.itemCache = copyItemCache(m.itemCache)
	t.yamlContent = m.yamlContent
	t.yamlScroll = m.yamlScroll
	t.yamlCursor = m.yamlCursor
	t.yamlSearchText = m.yamlSearchText
	t.yamlMatchLines = m.yamlMatchLines
	t.yamlMatchIdx = m.yamlMatchIdx
	t.yamlCollapsed = copyMapStringBool(m.yamlCollapsed)
	t.splitPreview = m.splitPreview
	t.fullYAMLPreview = m.fullYAMLPreview
	t.previewYAML = m.previewYAML
	t.namespace = m.namespace
	t.allNamespaces = m.allNamespaces
	t.selectedNamespaces = copyMapStringBool(m.selectedNamespaces)
	t.sortColumnName = m.sortColumnName
	t.sortAscending = m.sortAscending
	t.filterText = m.filterText
	t.watchMode = m.watchMode
	t.requestGen = m.requestGen
	t.selectedItems = copyMapStringBool(m.selectedItems)
	t.selectionAnchor = m.selectionAnchor
	t.fullscreenMiddle = m.fullscreenMiddle
	t.fullscreenDashboard = m.fullscreenDashboard
	t.dashboardPreview = m.dashboardPreview
	t.monitoringPreview = m.monitoringPreview
	t.warningEventsOnly = m.warningEventsOnly
	t.expandedGroup = m.expandedGroup
	t.allGroupsExpanded = m.allGroupsExpanded
	t.mode = m.mode
	t.logLines = append([]string(nil), m.logLines...)
	t.logScroll = m.logScroll
	t.logFollow = m.logFollow
	t.logWrap = m.logWrap
	t.logLineNumbers = m.logLineNumbers
	t.logTimestamps = m.logTimestamps
	t.logPrevious = m.logPrevious
	t.logIsMulti = m.logIsMulti
	t.logTitle = m.logTitle
	t.logCancel = m.logCancel
	t.logCh = m.logCh
	t.logTailLines = m.logTailLines
	t.logHasMoreHistory = m.logHasMoreHistory
	t.logLoadingHistory = m.logLoadingHistory
	t.logCursor = m.logCursor
	t.logVisualMode = m.logVisualMode
	t.logVisualStart = m.logVisualStart
	t.logVisualType = m.logVisualType
	t.logVisualCol = m.logVisualCol
	t.logVisualCurCol = m.logVisualCurCol
	t.logParentKind = m.logParentKind
	t.logParentName = m.logParentName
	t.logSavedPodName = m.logSavedPodName
	t.logContainers = append([]string(nil), m.logContainers...)
	t.logSelectedContainers = append([]string(nil), m.logSelectedContainers...)
	t.describeContent = m.describeContent
	t.describeScroll = m.describeScroll
	t.describeTitle = m.describeTitle
	t.diffLeft = m.diffLeft
	t.diffRight = m.diffRight
	t.diffLeftName = m.diffLeftName
	t.diffRightName = m.diffRightName
	t.diffScroll = m.diffScroll
	t.diffUnified = m.diffUnified
	t.execPTY = m.execPTY
	t.execTerm = m.execTerm
	t.execTitle = m.execTitle
	t.execDone = m.execDone
	t.execMu = m.execMu
	t.explainFields = append([]model.ExplainField(nil), m.explainFields...)
	t.explainDesc = m.explainDesc
	t.explainPath = m.explainPath
	t.explainResource = m.explainResource
	t.explainAPIVersion = m.explainAPIVersion
	t.explainTitle = m.explainTitle
	t.explainCursor = m.explainCursor
	t.explainScroll = m.explainScroll
	t.explainSearchQuery = m.explainSearchQuery
}

// loadTab restores Model fields from the given tab index.
// If the tab was restored from a session and has not been loaded yet (needsLoad),
// it returns a tea.Cmd that fetches the tab's data; otherwise it returns nil.
func (m *Model) loadTab(idx int) tea.Cmd {
	t := m.tabs[idx]
	needsLoad := t.needsLoad
	m.activeTab = idx
	m.nav = t.nav
	m.leftItems = append([]model.Item(nil), t.leftItems...)
	m.middleItems = append([]model.Item(nil), t.middleItems...)
	m.rightItems = append([]model.Item(nil), t.rightItems...)
	m.leftItemsHistory = make([][]model.Item, len(t.leftItemsHistory))
	for i, hist := range t.leftItemsHistory {
		m.leftItemsHistory[i] = append([]model.Item(nil), hist...)
	}
	m.cursors = t.cursors
	ui.ActiveMiddleScroll = t.middleScroll
	ui.ActiveLeftScroll = t.leftScroll
	m.cursorMemory = copyMapStringInt(t.cursorMemory)
	m.itemCache = copyItemCache(t.itemCache)
	m.yamlContent = t.yamlContent
	m.yamlScroll = t.yamlScroll
	m.yamlCursor = t.yamlCursor
	m.yamlSearchText = t.yamlSearchText
	m.yamlMatchLines = t.yamlMatchLines
	m.yamlMatchIdx = t.yamlMatchIdx
	m.yamlCollapsed = copyMapStringBool(t.yamlCollapsed)
	m.splitPreview = t.splitPreview
	m.fullYAMLPreview = t.fullYAMLPreview
	m.previewYAML = t.previewYAML
	m.namespace = t.namespace
	m.allNamespaces = t.allNamespaces
	m.selectedNamespaces = copyMapStringBool(t.selectedNamespaces)
	m.sortColumnName = t.sortColumnName
	m.sortAscending = t.sortAscending
	m.filterText = t.filterText
	m.watchMode = t.watchMode
	m.requestGen = t.requestGen
	m.selectedItems = copyMapStringBool(t.selectedItems)
	m.selectionAnchor = t.selectionAnchor
	m.fullscreenMiddle = t.fullscreenMiddle
	m.fullscreenDashboard = t.fullscreenDashboard
	m.dashboardPreview = t.dashboardPreview
	m.monitoringPreview = t.monitoringPreview
	m.warningEventsOnly = t.warningEventsOnly
	m.expandedGroup = t.expandedGroup
	m.allGroupsExpanded = t.allGroupsExpanded

	// Restore per-tab view mode and log state.
	m.mode = t.mode
	m.logLines = append([]string(nil), t.logLines...)
	m.logScroll = t.logScroll
	m.logFollow = t.logFollow
	m.logWrap = t.logWrap
	m.logLineNumbers = t.logLineNumbers
	m.logTimestamps = t.logTimestamps
	m.logPrevious = t.logPrevious
	m.logIsMulti = t.logIsMulti
	m.logTitle = t.logTitle
	m.logCancel = t.logCancel
	m.logCh = t.logCh
	m.logTailLines = t.logTailLines
	m.logHasMoreHistory = t.logHasMoreHistory
	m.logLoadingHistory = t.logLoadingHistory
	m.logCursor = t.logCursor
	m.logVisualMode = t.logVisualMode
	m.logVisualStart = t.logVisualStart
	m.logVisualType = t.logVisualType
	m.logVisualCol = t.logVisualCol
	m.logVisualCurCol = t.logVisualCurCol
	m.logParentKind = t.logParentKind
	m.logParentName = t.logParentName
	m.logSavedPodName = t.logSavedPodName
	m.logContainers = append([]string(nil), t.logContainers...)
	m.logSelectedContainers = append([]string(nil), t.logSelectedContainers...)
	m.describeContent = t.describeContent
	m.describeScroll = t.describeScroll
	m.describeTitle = t.describeTitle
	m.diffLeft = t.diffLeft
	m.diffRight = t.diffRight
	m.diffLeftName = t.diffLeftName
	m.diffRightName = t.diffRightName
	m.diffScroll = t.diffScroll
	m.diffUnified = t.diffUnified
	m.execPTY = t.execPTY
	m.execTerm = t.execTerm
	m.execTitle = t.execTitle
	m.execDone = t.execDone
	m.execMu = t.execMu
	m.explainFields = append([]model.ExplainField(nil), t.explainFields...)
	m.explainDesc = t.explainDesc
	m.explainPath = t.explainPath
	m.explainResource = t.explainResource
	m.explainAPIVersion = t.explainAPIVersion
	m.explainTitle = t.explainTitle
	m.explainCursor = t.explainCursor
	m.explainScroll = t.explainScroll
	m.explainSearchQuery = t.explainSearchQuery

	// Close overlays and reset transient state.
	m.overlay = overlayNone
	m.filterActive = false
	m.searchActive = false
	m.err = nil

	// If this tab was restored from a session but never loaded, clear the
	// flag, set up the navigation column structure, and return a command
	// that fetches the tab's data.
	if needsLoad {
		m.tabs[idx].needsLoad = false
		m.applyPinnedGroups()

		// Load contexts for the left column breadcrumb.
		contexts, _ := m.client.GetContexts()
		resourceTypes := model.FlattenedResourceTypes()
		if crds := m.discoveredCRDs[m.nav.Context]; len(crds) > 0 {
			resourceTypes = model.MergeWithCRDs(crds)
		}

		switch m.nav.Level {
		case model.LevelResources:
			// At resources level: left = resource types, history = [contexts].
			m.leftItemsHistory = [][]model.Item{contexts}
			m.leftItems = resourceTypes
			m.middleItems = nil
			m.clearRight()
			m.setCursor(0)
			m.loading = true
			return m.loadResources(false)
		case model.LevelResourceTypes:
			// At resource types level: left = contexts, middle = resource types.
			m.leftItemsHistory = nil
			m.leftItems = contexts
			m.middleItems = resourceTypes
			m.itemCache[m.navKey()] = m.middleItems
			m.clearRight()
			m.clampCursor()
			return m.loadPreview()
		default:
			// Clusters level or unknown: just load contexts.
			m.loading = true
			return m.refreshCurrentLevel()
		}
	}
	return nil
}

// cloneCurrentTab creates a deep copy of the current model state as a new TabState.
func (m *Model) cloneCurrentTab() TabState {
	newTab := TabState{
		nav:                 m.nav,
		leftItems:           append([]model.Item(nil), m.leftItems...),
		middleItems:         append([]model.Item(nil), m.middleItems...),
		rightItems:          append([]model.Item(nil), m.rightItems...),
		cursors:             m.cursors,
		middleScroll:        ui.ActiveMiddleScroll,
		leftScroll:          ui.ActiveLeftScroll,
		cursorMemory:        copyMapStringInt(m.cursorMemory),
		itemCache:           copyItemCache(m.itemCache),
		yamlContent:         m.yamlContent,
		yamlCollapsed:       copyMapStringBool(m.yamlCollapsed),
		splitPreview:        m.splitPreview,
		fullYAMLPreview:     m.fullYAMLPreview,
		previewYAML:         m.previewYAML,
		namespace:           m.namespace,
		allNamespaces:       m.allNamespaces,
		selectedNamespaces:  copyMapStringBool(m.selectedNamespaces),
		sortColumnName:      m.sortColumnName,
		sortAscending:       m.sortAscending,
		filterText:          m.filterText,
		watchMode:           m.watchMode,
		requestGen:          m.requestGen,
		selectedItems:       copyMapStringBool(m.selectedItems),
		selectionAnchor:     m.selectionAnchor,
		fullscreenMiddle:    m.fullscreenMiddle,
		fullscreenDashboard: m.fullscreenDashboard,
		dashboardPreview:    m.dashboardPreview,
		monitoringPreview:   m.monitoringPreview,
		warningEventsOnly:   m.warningEventsOnly,
		expandedGroup:       m.expandedGroup,
		allGroupsExpanded:   m.allGroupsExpanded,
		logCursor:           m.logCursor,
		logVisualMode:       false, // don't clone visual mode into new tabs
		logVisualStart:      0,
		logVisualType:       'V',
		logVisualCol:        0,
		logVisualCurCol:     0,
	}
	// Deep copy leftItemsHistory.
	newTab.leftItemsHistory = make([][]model.Item, len(m.leftItemsHistory))
	for i, hist := range m.leftItemsHistory {
		newTab.leftItemsHistory[i] = append([]model.Item(nil), hist...)
	}
	return newTab
}

// copyMapStringInt deep copies a map[string]int.
func copyMapStringInt(m map[string]int) map[string]int {
	if m == nil {
		return make(map[string]int)
	}
	c := make(map[string]int, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// copyMapStringBool deep copies a map[string]bool.
func copyMapStringBool(m map[string]bool) map[string]bool {
	if m == nil {
		return make(map[string]bool)
	}
	c := make(map[string]bool, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// copyItemCache deep copies the item cache.
func copyItemCache(m map[string][]model.Item) map[string][]model.Item {
	if m == nil {
		return make(map[string][]model.Item)
	}
	c := make(map[string][]model.Item, len(m))
	for k, v := range m {
		c[k] = append([]model.Item(nil), v...)
	}
	return c
}

// actionNamespace returns the namespace to use for action commands.
// It prefers the namespace captured when the action menu was opened.
func (m Model) actionNamespace() string {
	if m.actionCtx.namespace != "" {
		return m.actionCtx.namespace
	}
	return m.resolveNamespace()
}
