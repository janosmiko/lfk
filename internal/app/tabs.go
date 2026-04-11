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
// Every caller of clearRight is a navigation transition that will dispatch a
// new preview load, so we arm previewLoading here to keep the right pane's
// spinner visible during the gap. Without this, navigateParent/navigateChild
// and other transitions briefly render "No resources found".
func (m *Model) clearRight() {
	m.rightItems = nil
	m.yamlContent = ""
	m.yamlSections = nil
	m.previewYAML = ""
	m.metricsContent = ""
	m.previewEventsContent = ""
	m.resourceTree = nil
	m.mapView = false
	m.previewLoading = true
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
	if colName == "" {
		// Production always seeds sortColumnName with sortColDefault in
		// NewModel; an empty value here means a test fixture that built
		// a bare Model{} literal. Skip sorting in that case — otherwise
		// the tiebreaker below would impose a deterministic order on
		// items the caller may want to keep in their original sequence.
		return
	}
	asc := m.sortAscending

	sort.SliceStable(m.middleItems, func(i, j int) bool {
		a, b := m.middleItems[i], m.middleItems[j]

		// Primary comparison on the selected column.
		primary := comparePrimaryColumn(a, b, colName)
		if primary != 0 {
			if asc {
				return primary < 0
			}
			return primary > 0
		}

		// Tiebreaker: items with identical primary keys fall through to a
		// stable chain that is always ascending, regardless of the
		// primary's asc/desc flag. The chain is primary-aware: the
		// identity triple (Name, Namespace, Age) forms the main fallback
		// in that order, with whichever of those three is already the
		// primary column skipped so the tiebreaker doesn't redo work
		// the primary already did. Kind and Extra are appended as
		// absolute final discriminators.
		//
		// Without this, watch-mode refreshes would reshuffle rows with
		// identical primary keys (e.g. a Helm release "traefik" deployed
		// to multiple namespaces), because k8s API list calls can return
		// items in different orders and sort.SliceStable would then
		// preserve that shifting order.
		return itemTiebreakerLess(a, b, colName)
	})
}

// itemTiebreakerLess defines a total order on model.Item used as a sort
// tiebreaker. Always ascending — independent of the primary sort's asc
// flag — so identical primary keys land in a deterministic order across
// refreshes whether the user is sorting ascending or descending.
//
// The chain is primary-aware: (Name, Namespace, Age) participates in
// that order, with whichever of those three is the current primary
// column excluded. Kind and Extra act as final fallbacks so rows with
// truly identical identity still have a stable order.
//
//	primary=Name      → (Namespace, Age,   Kind, Extra)
//	primary=Namespace → (Name,      Age,   Kind, Extra)
//	primary=Age       → (Name,      Namespace, Kind, Extra)
//	primary=anything  → (Name, Namespace, Age, Kind, Extra)
func itemTiebreakerLess(a, b model.Item, primaryCol string) bool {
	if primaryCol != "Name" {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c < 0
		}
	}
	if primaryCol != "Namespace" {
		if c := strings.Compare(strings.ToLower(a.Namespace), strings.ToLower(b.Namespace)); c != 0 {
			return c < 0
		}
	}
	if primaryCol != "Age" {
		if c := compareAgeCmp(a, b); c != 0 {
			return c < 0
		}
	}
	if c := strings.Compare(a.Kind, b.Kind); c != 0 {
		return c < 0
	}
	return a.Extra < b.Extra
}

// comparePrimaryColumn returns -1, 0, or +1 for a < b, a == b, a > b
// according to the selected sort column. Returning 0 lets the caller run
// a tiebreaker chain instead of relying on sort.SliceStable's input-order
// preservation.
func comparePrimaryColumn(a, b model.Item, colName string) int {
	switch colName {
	case "Name":
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	case "Namespace":
		return strings.Compare(strings.ToLower(a.Namespace), strings.ToLower(b.Namespace))
	case "Ready":
		return compareReadyCmp(a.Ready, b.Ready)
	case "Restarts":
		return compareNumericCmp(a.Restarts, b.Restarts)
	case "Status":
		if c := cmpInt(statusPriority(a.Status), statusPriority(b.Status)); c != 0 {
			return c
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	case "Age":
		return compareAgeCmp(a, b)
	default:
		return compareColumnValuesCmp(getColumnValue(a, colName), getColumnValue(b, colName), colName)
	}
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareReady(a, b string) bool {
	return compareReadyCmp(a, b) < 0
}

func compareReadyCmp(a, b string) int {
	return cmpFloat(parseReadyRatio(a), parseReadyRatio(b))
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
	return compareNumericCmp(a, b) < 0
}

func compareNumericCmp(a, b string) int {
	na, _ := strconv.Atoi(strings.TrimSpace(a))
	nb, _ := strconv.Atoi(strings.TrimSpace(b))
	return cmpInt(na, nb)
}

func compareResourceValues(a, b, col string) bool {
	return compareResourceValuesCmp(a, b, col) < 0
}

func compareResourceValuesCmp(a, b, col string) int {
	isCPU := strings.HasPrefix(col, "CPU")
	return cmpInt64(ui.ParseResourceValue(a, isCPU), ui.ParseResourceValue(b, isCPU))
}

// compareAgeCmp returns the three-way age comparison with zero-time
// values sorted last and newer timestamps sorted first ("ascending" age
// means newest-first in the UI).
func compareAgeCmp(a, b model.Item) int {
	aZero := a.CreatedAt.IsZero()
	bZero := b.CreatedAt.IsZero()
	switch {
	case aZero && bZero:
		return strings.Compare(a.Name, b.Name)
	case aZero:
		return 1 // zero sorts after any real time
	case bZero:
		return -1
	}
	// Newer timestamps are "less" (render higher in ascending view).
	switch {
	case a.CreatedAt.After(b.CreatedAt):
		return -1
	case a.CreatedAt.Before(b.CreatedAt):
		return 1
	default:
		return 0
	}
}

// compareColumnValuesCmp compares two column values with automatic detection
// of resource quantities (10Gi, 500Mi, 100m), plain numbers, and strings.
// Returns -1, 0, or +1 so sort.SliceStable callers can detect equality
// and fall through to the row-identity tiebreaker chain.
func compareColumnValuesCmp(a, b, colName string) int {
	// Known CPU/MEM columns: use resource value parser directly.
	if colName == "CPU" || colName == "MEM" || colName == "CPU/R" || colName == "CPU/L" || colName == "MEM/R" || colName == "MEM/L" {
		return compareResourceValuesCmp(a, b, colName)
	}

	// Try parsing as resource quantities (Gi, Mi, Ki, B suffixes or millicores).
	if looksLikeResourceQuantity(a) || looksLikeResourceQuantity(b) {
		va := ui.ParseResourceValue(a, false)
		vb := ui.ParseResourceValue(b, false)
		if va != 0 || vb != 0 {
			return cmpInt64(va, vb)
		}
	}

	// Try parsing as plain numbers.
	na, errA := strconv.ParseFloat(strings.TrimSpace(a), 64)
	nb, errB := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if errA == nil && errB == nil {
		return cmpFloat(na, nb)
	}

	// Fall back to lexicographic comparison.
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// looksLikeResourceQuantity returns true if the value has a Kubernetes resource
// quantity suffix (Gi, Mi, Ki, B, m for millicores).
func looksLikeResourceQuantity(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasSuffix(s, "Gi") ||
		strings.HasSuffix(s, "Mi") ||
		strings.HasSuffix(s, "Ki") ||
		strings.HasSuffix(s, "Ti") ||
		(strings.HasSuffix(s, "m") && len(s) > 1 && s[len(s)-2] >= '0' && s[len(s)-2] <= '9')
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
	if discovered := m.discoveredResources[m.nav.Context]; len(discovered) > 0 {
		resourceTypes = model.BuildSidebarItems(discovered)
	} else {
		resourceTypes = model.BuildSidebarItems(model.SeedResources())
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
	t.dashboardEventsPreview = m.dashboardEventsPreview
	t.monitoringPreview = m.monitoringPreview
	t.warningEventsOnly = m.warningEventsOnly
	t.eventGrouping = m.eventGrouping
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
	m.dashboardEventsPreview = t.dashboardEventsPreview
	m.monitoringPreview = t.monitoringPreview
	m.warningEventsOnly = t.warningEventsOnly
	m.eventGrouping = t.eventGrouping
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
		resourceTypes := model.BuildSidebarItems(model.SeedResources())
		if discovered := m.discoveredResources[m.nav.Context]; len(discovered) > 0 {
			resourceTypes = model.BuildSidebarItems(discovered)
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
		nav:                    m.nav,
		leftItems:              append([]model.Item(nil), m.leftItems...),
		middleItems:            append([]model.Item(nil), m.middleItems...),
		rightItems:             append([]model.Item(nil), m.rightItems...),
		cursors:                m.cursors,
		middleScroll:           ui.ActiveMiddleScroll,
		leftScroll:             ui.ActiveLeftScroll,
		cursorMemory:           copyMapStringInt(m.cursorMemory),
		itemCache:              copyItemCache(m.itemCache),
		yamlContent:            m.yamlContent,
		yamlCollapsed:          copyMapStringBool(m.yamlCollapsed),
		splitPreview:           m.splitPreview,
		fullYAMLPreview:        m.fullYAMLPreview,
		previewYAML:            m.previewYAML,
		namespace:              m.namespace,
		allNamespaces:          m.allNamespaces,
		selectedNamespaces:     copyMapStringBool(m.selectedNamespaces),
		sortColumnName:         m.sortColumnName,
		sortAscending:          m.sortAscending,
		filterText:             m.filterText,
		watchMode:              m.watchMode,
		requestGen:             m.requestGen,
		selectedItems:          copyMapStringBool(m.selectedItems),
		selectionAnchor:        m.selectionAnchor,
		fullscreenMiddle:       m.fullscreenMiddle,
		fullscreenDashboard:    m.fullscreenDashboard,
		dashboardPreview:       m.dashboardPreview,
		dashboardEventsPreview: m.dashboardEventsPreview,
		monitoringPreview:      m.monitoringPreview,
		warningEventsOnly:      m.warningEventsOnly,
		eventGrouping:          m.eventGrouping,
		expandedGroup:          m.expandedGroup,
		allGroupsExpanded:      m.allGroupsExpanded,
		logCursor:              m.logCursor,
		logVisualMode:          false, // don't clone visual mode into new tabs
		logVisualStart:         0,
		logVisualType:          'V',
		logVisualCol:           0,
		logVisualCurCol:        0,
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
