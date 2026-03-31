package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// yamlFoldPrefixLen is the number of characters prepended by buildVisibleLines
// for fold indicators (always "  ", 2 chars). Cursor columns operate on these
// prefixed lines, so the first content character is at index yamlFoldPrefixLen.
const yamlFoldPrefixLen = 2

// isContextCanceled returns true if the error is due to an intentional context cancellation.
// It checks both Go context errors and string-based "context canceled" messages from kubectl.
func isContextCanceled(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "context canceled") || strings.Contains(msg, "context deadline exceeded")
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampAllCursors()
		// Resize the embedded PTY terminal if active.
		if m.mode == modeExec && m.execTerm != nil && m.execPTY != nil {
			cols := m.width - 4
			rows := m.height - 6
			if cols < 20 {
				cols = 20
			}
			if rows < 5 {
				rows = 5
			}
			m.execMu.Lock()
			m.execTerm.Resize(cols, rows)
			m.execMu.Unlock()
			_ = pty.Setsize(m.execPTY, &pty.Winsize{
				Rows: uint16(rows),
				Cols: uint16(cols),
			})
		}
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case contextsLoadedMsg:
		m.loading = false
		if isContextCanceled(msg.err) {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			m.setErrorFromErr("Warning: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.err = nil
		m.middleItems = msg.items
		m.itemCache[m.navKey()] = m.middleItems
		m.leftItems = nil
		m.clearRight()
		m.clampCursor()

		// Restore saved port forwards in the background.
		var pfCmds []tea.Cmd
		if m.pendingPortForwards != nil && len(m.pendingPortForwards.PortForwards) > 0 {
			pfCmds = m.restorePortForwards()
			m.pendingPortForwards = nil
		}

		// Restore session: navigate to the saved context/namespace/resource type.
		if m.pendingSession != nil && !m.sessionRestored {
			mdl, cmd := m.restoreSession(msg.items)
			if len(pfCmds) > 0 {
				pfCmds = append(pfCmds, cmd)
				return mdl, tea.Batch(pfCmds...)
			}
			return mdl, cmd
		}

		cmds := make([]tea.Cmd, 0, 1+len(pfCmds))
		cmds = append(cmds, m.loadPreview())
		cmds = append(cmds, pfCmds...)
		return m, tea.Batch(cmds...)

	case resourceTypesMsg:
		m.loading = false
		m.err = nil
		if m.nav.Level == model.LevelClusters {
			m.rightItems = msg.items
			return m, nil
		}
		m.middleItems = msg.items
		m.itemCache[m.navKey()] = m.middleItems
		m.clampCursor()
		return m, m.loadPreview()

	case crdDiscoveryMsg:
		if isContextCanceled(msg.err) {
			return m, nil
		}
		if msg.err != nil {
			// CRD discovery failed (permissions, etc.) -- silently ignore.
			logger.Info("CRD discovery failed", "context", msg.context, "error", msg.err.Error())
			return m, nil
		}
		m.discoveredCRDs[msg.context] = msg.entries
		if m.nav.Context == msg.context {
			merged := model.MergeWithCRDs(msg.entries)
			// Update the item cache for the resource types level.
			rtCacheKey := msg.context
			m.itemCache[rtCacheKey] = merged
			if m.nav.Level == model.LevelResourceTypes {
				// User is on resource types level: update the visible list.
				m.middleItems = merged
				m.clampCursor()
			} else {
				// User is deeper: update leftItems so back-navigation shows CRDs.
				m.leftItems = merged
				// Update cursor memory for the resource types level so
				// navigating back lands on the correct resource type.
				if m.nav.ResourceType.Resource != "" {
					rtRef := m.nav.ResourceType.ResourceRef()
					for i, item := range merged {
						if item.Extra == rtRef {
							m.cursorMemory[msg.context] = i
							break
						}
					}
				}
			}
		}
		return m, nil

	case resourcesLoadedMsg:
		if msg.gen != m.requestGen {
			return m, nil // stale response, discard
		}
		m.loading = false
		if isContextCanceled(msg.err) {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			m.setErrorFromErr("Warning: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.err = nil
		if msg.forPreview {
			m.rightItems = msg.items
			if len(msg.items) == 0 {
				logger.Info("No child resources found", "resourceType", m.nav.ResourceType.Kind, "resource", m.nav.ResourceName)
			}
			return m, nil
		}
		// Filter by selected namespaces when multi-select is active.
		if len(m.selectedNamespaces) > 1 {
			filtered := make([]model.Item, 0, len(msg.items))
			for _, item := range msg.items {
				if m.selectedNamespaces[item.Namespace] {
					filtered = append(filtered, item)
				}
			}
			msg.items = filtered
		}
		if len(msg.items) == 0 {
			logger.Info("No resources found", "resourceType", m.nav.ResourceType.Kind, "namespace", m.namespace)
		}
		// Remember which item is currently selected so the cursor can be
		// restored after the list is replaced (watch mode may reorder items).
		prevName, prevNs, prevExtra := m.cursorItemKey()

		// For pod/node views, carry over existing metrics columns to avoid blinking
		// when watch mode refreshes (metrics enrichment arrives async).
		kind := m.nav.ResourceType.Kind
		if (kind == "Pod" || kind == "Node") && len(m.middleItems) > 0 {
			m.carryOverMetricsColumns(msg.items)
		}
		m.middleItems = msg.items
		m.itemCache[m.navKey()] = m.middleItems
		if m.sortBy != sortByName {
			m.sortMiddleItems()
		}
		// Auto-filter events to warnings-only when enabled.
		if m.warningEventsOnly && m.nav.ResourceType.Kind == "Event" {
			var filtered []model.Item
			for _, item := range m.middleItems {
				if item.Status == "Warning" {
					filtered = append(filtered, item)
				}
			}
			m.middleItems = filtered
		}
		// Re-apply active filter preset on refresh.
		if m.activeFilterPreset != nil {
			m.unfilteredMiddleItems = append([]model.Item(nil), m.middleItems...)
			var filtered []model.Item
			for _, item := range m.middleItems {
				if m.activeFilterPreset.MatchFn(item) {
					filtered = append(filtered, item)
				}
			}
			m.middleItems = filtered
		}
		// If there's a pending target (from owner navigation), select it.
		if m.pendingTarget != "" {
			for i, item := range m.middleItems {
				if item.Name == m.pendingTarget {
					m.setCursor(i)
					break
				}
			}
			m.pendingTarget = ""
		} else {
			// Restore cursor to the previously selected item.
			m.restoreCursorToItem(prevName, prevNs, prevExtra)
		}
		// For pod/node views, also trigger async metrics enrichment.
		var cmds []tea.Cmd
		cmds = append(cmds, m.loadPreview())
		switch kind {
		case "Pod":
			cmds = append(cmds, m.loadPodMetricsForList())
		case "Node":
			cmds = append(cmds, m.loadNodeMetricsForList())
		}
		return m, tea.Batch(cmds...)

	case ownedLoadedMsg:
		if msg.gen != m.requestGen {
			return m, nil // stale response, discard
		}
		m.loading = false
		if isContextCanceled(msg.err) {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			m.setErrorFromErr("Warning: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.err = nil
		// Filter by selected namespaces when multi-select is active.
		if len(m.selectedNamespaces) > 1 {
			filtered := make([]model.Item, 0, len(msg.items))
			for _, item := range msg.items {
				if m.selectedNamespaces[item.Namespace] {
					filtered = append(filtered, item)
				}
			}
			msg.items = filtered
		}
		if msg.forPreview {
			m.rightItems = msg.items
			return m, nil
		}
		prevName, prevNs, prevExtra := m.cursorItemKey()
		m.middleItems = msg.items
		m.itemCache[m.navKey()] = m.middleItems
		// Re-apply active filter preset on owned refresh (same as resourcesLoadedMsg).
		if m.activeFilterPreset != nil {
			m.unfilteredMiddleItems = append([]model.Item(nil), m.middleItems...)
			var filtered []model.Item
			for _, item := range m.middleItems {
				if m.activeFilterPreset.MatchFn(item) {
					filtered = append(filtered, item)
				}
			}
			m.middleItems = filtered
			m.itemCache[m.navKey()] = m.middleItems
		}
		m.restoreCursorToItem(prevName, prevNs, prevExtra)
		return m, m.loadPreview()

	case resourceTreeLoadedMsg:
		if msg.gen != m.requestGen {
			return m, nil
		}
		if isContextCanceled(msg.err) {
			return m, nil
		}
		if msg.err != nil {
			m.setErrorFromErr("Resource tree: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.resourceTree = msg.tree
		return m, nil

	case containersLoadedMsg:
		if msg.gen != m.requestGen {
			return m, nil // stale response, discard
		}
		m.loading = false
		if isContextCanceled(msg.err) {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			m.setErrorFromErr("Warning: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.err = nil
		if msg.forPreview {
			m.rightItems = msg.items
			return m, nil
		}
		m.middleItems = msg.items
		m.itemCache[m.navKey()] = m.middleItems
		m.clampCursor()
		return m, m.loadPreview()

	case namespacesLoadedMsg:
		m.loading = false
		if isContextCanceled(msg.err) {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			m.overlay = overlayNone
			m.setErrorFromErr("Warning: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.err = nil
		allNsItem := model.Item{Name: "All Namespaces", Status: "all"}
		m.overlayItems = append([]model.Item{allNsItem}, msg.items...)
		// Cache namespace names for command bar autocompletion.
		m.cachedNamespaces = make([]string, 0, len(msg.items))
		for _, item := range msg.items {
			m.cachedNamespaces = append(m.cachedNamespaces, item.Name)
		}
		if m.allNamespaces {
			m.overlayCursor = 0
		} else {
			for i, item := range m.overlayItems {
				if item.Name == m.namespace {
					m.overlayCursor = i
					break
				}
			}
		}
		return m, nil

	case yamlLoadedMsg:
		m.loading = false
		if isContextCanceled(msg.err) {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			m.setErrorFromErr("Warning: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.err = nil
		m.yamlContent = indentYAMLListItems(msg.content)
		m.yamlSections = parseYAMLSections(m.yamlContent)
		return m, nil

	case previewYAMLLoadedMsg:
		if msg.gen != m.requestGen {
			return m, nil // stale response, discard
		}
		if msg.err != nil {
			m.previewYAML = ""
			return m, nil
		}
		m.previewYAML = indentYAMLListItems(msg.content)
		return m, nil

	case actionResultMsg:
		m.loading = false
		m.bulkMode = false
		if msg.err != nil {
			m.setErrorFromErr("Error: ", msg.err)
		} else if msg.message != "" {
			logger.Info("Action completed", "message", msg.message)
			m.setStatusMessage(msg.message, false)
		}
		return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())

	case containerPortsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			// No ports discovered, still open the overlay for manual entry.
			m.addLogEntry("WRN", fmt.Sprintf("Could not discover ports: %v", msg.err))
			m.pfAvailablePorts = nil
			m.pfPortCursor = -1
		} else {
			m.pfAvailablePorts = nil
			for _, p := range msg.ports {
				m.pfAvailablePorts = append(m.pfAvailablePorts, ui.PortInfo{
					Port:     strconv.Itoa(int(p.ContainerPort)),
					Name:     p.Name,
					Protocol: p.Protocol,
				})
			}
			if len(m.pfAvailablePorts) > 0 {
				m.pfPortCursor = 0
			} else {
				m.pfPortCursor = -1
			}
		}
		m.portForwardInput.Clear()
		m.overlay = overlayPortForward
		return m, nil

	case portForwardStartedMsg:
		if msg.err != nil {
			m.setErrorFromErr("Port forward failed: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.pfLastCreatedID = msg.id
		if msg.localPort == "0" {
			m.setStatusMessage(fmt.Sprintf("Port forward started (resolving local port...) -> %s", msg.remotePort), false)
			m.addLogEntry("INF", fmt.Sprintf("Port forward %d started: localhost:? -> %s (random port)", msg.id, msg.remotePort))
		} else {
			m.setStatusMessage(fmt.Sprintf("Port forward started on localhost:%s -> %s", msg.localPort, msg.remotePort), false)
			m.addLogEntry("INF", fmt.Sprintf("Port forward %d started: localhost:%s -> %s", msg.id, msg.localPort, msg.remotePort))
		}
		// Navigate to the Port Forwards list.
		m.navigateToPortForwards()
		m.saveCurrentPortForwards()
		cmds := []tea.Cmd{m.waitForPortForwardUpdate()}
		return m, tea.Batch(cmds...)

	case portForwardStoppedMsg:
		if msg.err != nil {
			m.setErrorFromErr("Error stopping port forward: ", msg.err)
		} else {
			m.setStatusMessage("Port forward stopped", false)
		}
		m.saveCurrentPortForwards()
		// Refresh the port forwards list if viewing it.
		cmds := []tea.Cmd{scheduleStatusClear()}
		if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "__port_forwards__" {
			cmds = append(cmds, m.refreshCurrentLevel())
		}
		return m, tea.Batch(cmds...)

	case portForwardUpdateMsg:
		// Port forward state changed (e.g., port resolved, process exited).
		cmds := []tea.Cmd{m.waitForPortForwardUpdate()}
		if msg.err != nil {
			m.addLogEntry("ERR", msg.err.Error())
		}
		// Log newly failed port forwards.
		for _, e := range m.portForwardMgr.Entries() {
			if e.Status == k8s.PortForwardFailed && e.Error != "" {
				if _, seen := m.pfLoggedErrors[e.ID]; !seen {
					m.addLogEntry("ERR", fmt.Sprintf("Port forward %s/%s failed: %s", e.ResourceKind, e.ResourceName, e.Error))
					if m.pfLoggedErrors == nil {
						m.pfLoggedErrors = make(map[int]struct{})
					}
					m.pfLoggedErrors[e.ID] = struct{}{}
				}
			}
		}
		// Show resolved port notification for recently created port forward.
		if m.pfLastCreatedID > 0 {
			for _, e := range m.portForwardMgr.Entries() {
				if e.ID == m.pfLastCreatedID && e.LocalPort != "0" && e.LocalPort != "" {
					m.setStatusMessage(fmt.Sprintf("Port forward active on localhost:%s -> %s", e.LocalPort, e.RemotePort), false)
					m.addLogEntry("INF", fmt.Sprintf("Port forward %d resolved: localhost:%s -> %s", e.ID, e.LocalPort, e.RemotePort))
					m.pfLastCreatedID = 0
					m.saveCurrentPortForwards()
					cmds = append(cmds, scheduleStatusClear())
					break
				}
			}
		}
		if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "__port_forwards__" {
			m.middleItems = m.portForwardItems()
			m.clampCursor()
		}
		return m, tea.Batch(cmds...)

	case commandBarResultMsg:
		if msg.err != nil {
			errMsg := msg.err.Error()
			if msg.output != "" {
				errMsg = strings.TrimSpace(msg.output)
			}
			m.addLogEntry("ERR", errMsg)
			// Show error output in describe view if there's content, otherwise status bar.
			if msg.output != "" {
				m.mode = modeDescribe
				m.describeContent = strings.TrimSpace(msg.output)
				m.describeScroll = 0
				m.describeTitle = "Command Output (error)"
				return m, nil
			}
			m.setErrorFromErr("Command failed: ", fmt.Errorf("%s", errMsg))
			return m, scheduleStatusClear()
		}
		output := strings.TrimSpace(msg.output)
		if output != "" {
			m.addLogEntry("INF", output)
			// Open output in the describe viewer (scrollable, searchable, wrappable).
			m.mode = modeDescribe
			m.describeContent = output
			m.describeScroll = 0
			m.describeTitle = "Command Output"
			return m, nil
		}
		m.setStatusMessage("Command completed (no output)", false)
		return m, scheduleStatusClear()

	case triggerCronJobMsg:
		if msg.err != nil {
			m.setErrorFromErr("Trigger failed: ", msg.err)
		} else {
			m.setStatusMessage("Job created: "+msg.jobName, false)
		}
		m.loading = false
		return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())

	case bulkActionResultMsg:
		m.loading = false
		m.bulkMode = false
		m.clearSelection()
		if msg.failed > 0 {
			errSummary := fmt.Sprintf("Bulk: %d succeeded, %d failed", msg.succeeded, msg.failed)
			if len(msg.errors) > 0 {
				errSummary += ": " + msg.errors[0]
			}
			m.setStatusMessage(errSummary, true)
		} else {
			m.setStatusMessage(fmt.Sprintf("Bulk: %d resources processed", msg.succeeded), false)
		}
		return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())

	case stderrCapturedMsg:
		// Show captured stderr output (e.g., AWS SSO errors) in the status bar
		// and error log. Continue listening for more messages.
		m.setStatusMessage("stderr: "+msg.message, true)
		return m, tea.Batch(scheduleStatusClear(), m.waitForStderr())

	case statusMessageExpiredMsg:
		m.statusMessage = ""
		m.statusMessageTip = false
		return m, nil

	case startupTipMsg:
		m.setStatusMessage("Tip: "+msg.tip, false)
		m.statusMessageTip = true
		return m, scheduleStatusClear()

	case watchTickMsg:
		if !m.watchMode {
			return m, nil
		}
		return m, tea.Batch(m.refreshCurrentLevel(), scheduleWatchTick(m.watchInterval))

	case podSelectMsg:
		if msg.err != nil {
			m.setErrorFromErr("Error: ", msg.err)
			m.pendingAction = ""
			return m, scheduleStatusClear()
		}
		// Filter to only pods.
		var pods []model.Item
		for _, item := range msg.items {
			if item.Kind == "Pod" {
				pods = append(pods, item)
			}
		}
		if len(pods) == 0 {
			m.setStatusMessage("No pods found", true)
			m.pendingAction = ""
			return m, scheduleStatusClear()
		}
		if len(pods) == 1 {
			// Only one pod, proceed to container selection.
			m.actionCtx.name = pods[0].Name
			m.actionCtx.kind = "Pod"
			if pods[0].Namespace != "" {
				m.actionCtx.namespace = pods[0].Namespace
			}
			return m, m.loadContainersForAction()
		}
		// Multiple pods, show picker.
		m.overlayItems = pods
		m.overlay = overlayPodSelect
		m.overlayCursor = 0
		m.logPodFilterText = ""
		m.logPodFilterActive = false
		ui.ResetOverlayPodScroll()
		return m, nil

	case podLogSelectMsg:
		m.loading = false
		if msg.err != nil {
			m.setErrorFromErr("Error: ", msg.err)
			m.pendingAction = ""
			// If in log mode, restart the previous log stream on error.
			if m.mode == modeLogs && m.logSavedPodName != "" {
				m.actionCtx.name = m.logSavedPodName
				m.actionCtx.kind = "Pod"
				m.logSavedPodName = ""
				return m, m.startLogStream()
			}
			return m, scheduleStatusClear()
		}
		var pods []model.Item
		for _, item := range msg.items {
			if item.Kind == "Pod" {
				pods = append(pods, item)
			}
		}
		if len(pods) == 0 {
			m.setStatusMessage("No pods found", true)
			m.pendingAction = ""
			// If in log mode, restart the previous log stream when no pods found.
			if m.mode == modeLogs && m.logSavedPodName != "" {
				m.actionCtx.name = m.logSavedPodName
				m.actionCtx.kind = "Pod"
				m.logSavedPodName = ""
				return m, m.startLogStream()
			}
			return m, scheduleStatusClear()
		}

		// If we're in log mode (P was pressed from the log viewer), handle inline.
		if m.mode == modeLogs {
			if len(pods) == 1 {
				// Only one pod; switch directly to its logs.
				m.actionCtx.name = pods[0].Name
				m.actionCtx.kind = "Pod"
				if pods[0].Namespace != "" {
					m.actionCtx.namespace = pods[0].Namespace
				}
				m.pendingAction = ""
				m.logSavedPodName = ""
				m.logLines = nil
				m.logScroll = 0
				m.logFollow = true
				m.logTailLines = ui.ConfigLogTailLines
				m.logHasMoreHistory = true
				m.logLoadingHistory = false
				m.logCursor = 0
				m.logVisualMode = false
				m.logTitle = fmt.Sprintf("Logs: %s/%s", m.actionNamespace(), m.actionCtx.name)
				return m, m.startLogStream()
			}
			// Multiple pods; show inline pod selector overlay with "All Pods" at top.
			allItem := model.Item{Name: "All Pods", Status: "all"}
			m.overlayItems = append([]model.Item{allItem}, pods...)
			m.overlay = overlayLogPodSelect
			m.overlayCursor = 0
			m.logPodFilterText = ""
			m.logPodFilterActive = false
			ui.ResetOverlayPodScroll()
			return m, nil
		}

		if len(pods) == 1 {
			// Only one pod, start log streaming directly.
			m.actionCtx.name = pods[0].Name
			m.actionCtx.kind = "Pod"
			if pods[0].Namespace != "" {
				m.actionCtx.namespace = pods[0].Namespace
			}
			m.pendingAction = ""
			return m.executeAction("Logs")
		}
		// Multiple pods, show picker.
		m.overlayItems = pods
		m.overlay = overlayPodSelect
		m.overlayCursor = 0
		m.logPodFilterText = ""
		m.logPodFilterActive = false
		ui.ResetOverlayPodScroll()
		return m, nil

	case containerSelectMsg:
		if msg.err != nil {
			m.setErrorFromErr("Error: ", msg.err)
			m.pendingAction = ""
			return m, scheduleStatusClear()
		}
		if len(msg.items) == 1 {
			// Only one container; proceed directly.
			m.actionCtx.containerName = msg.items[0].Name
			action := m.pendingAction
			m.pendingAction = ""
			return m.executeAction(action)
		}
		if len(msg.items) == 0 {
			m.setStatusMessage("No containers found", true)
			m.pendingAction = ""
			return m, scheduleStatusClear()
		}
		// Multiple containers; show selection overlay.
		m.overlayItems = msg.items
		m.overlay = overlayContainerSelect
		m.overlayCursor = 0
		return m, nil

	case yamlClipboardMsg:
		if msg.err != nil {
			m.setErrorFromErr("Error: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.setStatusMessage("YAML copied to clipboard", false)
		return m, tea.Batch(copyToSystemClipboard(msg.content), scheduleStatusClear())

	case eventTimelineMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatusMessage(fmt.Sprintf("Event load failed: %v", msg.err), true)
			return m, scheduleStatusClear()
		}
		if len(msg.events) == 0 {
			m.setStatusMessage("No events found", false)
			return m, scheduleStatusClear()
		}
		m.eventTimelineData = msg.events
		m.eventTimelineScroll = 0
		m.overlay = overlayEventTimeline
		return m, nil

	case rbacCheckMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatusMessage(fmt.Sprintf("RBAC check failed: %v", msg.err), true)
			return m, scheduleStatusClear()
		}
		m.rbacResults = msg.results
		m.rbacKind = msg.kind
		m.overlay = overlayRBAC
		return m, nil

	case canILoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatusMessage(fmt.Sprintf("RBAC rules check failed: %v", msg.err), true)
			return m, scheduleStatusClear()
		}
		m.processCanIRules(msg.rules)
		m.canINamespaces = msg.namespaces
		m.overlay = overlayCanI
		m.canIGroupCursor = 0
		m.canIGroupScroll = 0
		return m, nil

	case canISAListMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatusMessage(fmt.Sprintf("Failed to list subjects: %v", msg.err), true)
			return m, scheduleStatusClear()
		}
		m.canIServiceAccounts = msg.accounts
		// Build overlay items: "Current User", then Users, Groups, ServiceAccounts.
		items := make([]model.Item, 0, len(msg.subjects)+len(msg.accounts)+1)
		items = append(items, model.Item{Name: "Current User", Extra: ""})

		// Add Users from RBAC bindings.
		for _, subj := range msg.subjects {
			if subj.Kind == "User" {
				items = append(items, model.Item{
					Name:  "[User] " + subj.Name,
					Kind:  "User",
					Extra: subj.Name,
				})
			}
		}
		// Add Groups from RBAC bindings.
		for _, subj := range msg.subjects {
			if subj.Kind == "Group" {
				items = append(items, model.Item{
					Name:  "[Group] " + subj.Name,
					Kind:  "Group",
					Extra: "group:" + subj.Name,
				})
			}
		}
		// Add ServiceAccounts.
		for _, sa := range msg.accounts {
			var name, ns string
			if parts := strings.SplitN(sa, "/", 2); len(parts) == 2 {
				ns = parts[0]
				name = parts[1]
			} else {
				ns = m.namespace
				name = sa
			}
			items = append(items, model.Item{
				Name:  "[SA] " + ns + "/" + name,
				Kind:  "ServiceAccount",
				Extra: fmt.Sprintf("system:serviceaccount:%s:%s", ns, name),
			})
		}
		m.overlayItems = items
		m.overlayCursor = 0
		m.overlayFilter.Clear()
		m.canISubjectFilterMode = false
		ui.ResetOverlayCanISubjectScroll()
		m.overlay = overlayCanISubject
		return m, nil

	case podStartupMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatusMessage(fmt.Sprintf("Startup analysis failed: %v", msg.err), true)
			return m, scheduleStatusClear()
		}
		m.podStartupData = msg.info
		m.overlay = overlayPodStartup

	case quotaLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatusMessage(fmt.Sprintf("Quota load failed: %v", msg.err), true)
			return m, scheduleStatusClear()
		}
		if len(msg.quotas) == 0 {
			m.setStatusMessage("No resource quotas found in namespace", false)
			return m, scheduleStatusClear()
		}
		m.quotaData = msg.quotas
		m.overlay = overlayQuotaDashboard

	case alertsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatusMessage(fmt.Sprintf("Alerts: %v", msg.err), true)
			return m, scheduleStatusClear()
		}
		m.alertsData = msg.alerts
		m.alertsScroll = 0
		m.overlay = overlayAlerts

	case netpolLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatusMessage(fmt.Sprintf("Failed to load network policy: %v", msg.err), true)
			return m, scheduleStatusClear()
		}
		m.netpolData = msg.info
		m.netpolScroll = 0
		m.overlay = overlayNetworkPolicy
		return m, nil

	case describeLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.setErrorFromErr("Error: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.mode = modeDescribe
		m.describeContent = msg.content
		// Preserve scroll on auto-refresh, reset on first load.
		if !m.describeAutoRefresh {
			m.describeScroll = 0
		}
		m.describeTitle = msg.title
		if m.describeAutoRefresh {
			return m, scheduleDescribeRefresh()
		}
		return m, nil

	case describeRefreshTickMsg:
		if m.mode != modeDescribe || !m.describeAutoRefresh || m.describeRefreshFunc == nil {
			return m, nil
		}
		return m, m.describeRefreshFunc()

	case helmValuesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.setErrorFromErr("Error loading Helm values: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.mode = modeDescribe
		m.describeContent = msg.content
		m.describeScroll = 0
		m.describeTitle = msg.title
		return m, nil

	case diffLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.setErrorFromErr("Diff failed: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.mode = modeDiff
		m.diffLeft = msg.left
		m.diffRight = msg.right
		m.diffLeftName = msg.leftName
		m.diffRightName = msg.rightName
		m.diffScroll = 0
		m.diffUnified = false
		return m, nil

	case explainLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.setErrorFromErr("Explain failed: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.mode = modeExplain
		m.explainFields = msg.fields
		m.explainDesc = msg.description
		m.explainPath = msg.path
		m.explainTitle = msg.title
		m.explainCursor = 0
		m.explainScroll = 0
		m.explainSearchActive = false
		return m, nil

	case explainRecursiveMsg:
		m.loading = false
		if msg.err != nil {
			m.setErrorFromErr("Recursive search failed: ", msg.err)
			return m, scheduleStatusClear()
		}
		if len(msg.matches) == 0 {
			m.setStatusMessage("No fields found", true)
			return m, scheduleStatusClear()
		}
		m.explainRecursiveResults = msg.matches
		m.explainRecursiveCursor = 0
		m.explainRecursiveScroll = 0
		m.explainRecursiveFilter.Clear()
		m.explainRecursiveFilterActive = false
		m.overlay = overlayExplainSearch
		return m, nil

	case logContainersLoadedMsg:
		if msg.err != nil {
			m.setErrorFromErr("Failed to load containers: ", msg.err)
			m.overlay = overlayNone
			return m, scheduleStatusClear()
		}
		if len(msg.containers) <= 1 {
			// Only one container (or none), no need for a selector.
			m.overlay = overlayNone
			name := "this pod"
			if len(msg.containers) == 1 {
				name = msg.containers[0]
			}
			m.setStatusMessage(fmt.Sprintf("Only one container: %s", name), false)
			return m, scheduleStatusClear()
		}
		m.logContainers = msg.containers
		// Build overlay items with "All Containers" virtual item at the top.
		items := []model.Item{{Name: "All Containers", Status: "all"}}
		for _, c := range msg.containers {
			items = append(items, model.Item{Name: c})
		}
		m.overlayItems = items
		return m, nil

	case metricsLoadedMsg:
		if msg.gen != m.requestGen {
			return m, nil // stale response
		}
		if msg.cpuUsed == 0 && msg.memUsed == 0 {
			m.metricsContent = ""
			return m, nil
		}
		// Calculate available width for the metrics bar.
		usable := m.width - 6
		rightW := max(10, usable-max(10, usable*12/100)-max(10, usable*51/100))
		innerW := rightW - 4 // column padding + border
		if innerW < 20 {
			innerW = 20
		}
		m.metricsContent = ui.RenderResourceUsage(
			msg.cpuUsed, msg.cpuReq, msg.cpuLim,
			msg.memUsed, msg.memReq, msg.memLim,
			innerW,
		)
		return m, nil

	case previewEventsLoadedMsg:
		if msg.gen != m.requestGen {
			return m, nil // stale response
		}
		if len(msg.events) == 0 {
			m.previewEventsContent = ""
			return m, nil
		}
		// Calculate available width for the events section.
		usable := m.width - 6
		rightW := max(10, usable-max(10, usable*12/100)-max(10, usable*51/100))
		innerW := rightW - 4
		if innerW < 20 {
			innerW = 20
		}
		entries := make([]ui.EventTimelineEntry, len(msg.events))
		for i, e := range msg.events {
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
		m.previewEventsContent = ui.RenderPreviewEvents(entries, innerW)
		return m, nil

	case podMetricsEnrichedMsg:
		if msg.gen != m.requestGen {
			return m, nil // stale response
		}
		if len(msg.metrics) == 0 {
			return m, nil
		}
		// Enrich middle items with CPU/Memory usage + percentage columns.
		for i := range m.middleItems {
			item := &m.middleItems[i]
			key := item.Name
			if item.Namespace != "" {
				key = item.Namespace + "/" + item.Name
			}
			pm, ok := msg.metrics[key]
			if !ok {
				continue
			}

			// Look up existing request/limit values from item columns.
			var cpuReqStr, cpuLimStr, memReqStr, memLimStr string
			for _, kv := range item.Columns {
				switch kv.Key {
				case "CPU Req":
					cpuReqStr = kv.Value
				case "CPU Lim":
					cpuLimStr = kv.Value
				case "Mem Req":
					memReqStr = kv.Value
				case "Mem Lim":
					memLimStr = kv.Value
				}
			}

			cpuUse := ui.FormatCPU(pm.CPU)
			memUse := ui.FormatMemory(pm.Memory)

			// Detect significant usage trends (arrows before value).
			if m.prevPodMetrics != nil {
				if prev, ok := m.prevPodMetrics[key]; ok {
					cpuDiff := pm.CPU - prev.CPU
					memDiff := pm.Memory - prev.Memory
					// CPU: significant if >10% change AND >20m absolute change.
					if prev.CPU > 0 {
						pctChange := float64(cpuDiff) / float64(prev.CPU)
						if pctChange > 0.10 && cpuDiff > 20 {
							cpuUse = "↑ " + cpuUse
						} else if pctChange < -0.10 && cpuDiff < -20 {
							cpuUse = "↓ " + cpuUse
						}
					}
					// Memory: significant if >10% change AND >20Mi absolute change.
					if prev.Memory > 0 {
						pctChange := float64(memDiff) / float64(prev.Memory)
						if pctChange > 0.10 && memDiff > 20*1024*1024 {
							memUse = "↑ " + memUse
						} else if pctChange < -0.10 && memDiff < -20*1024*1024 {
							memUse = "↓ " + memUse
						}
					}
				}
			}

			cpuReqPct := ui.ComputePctStr(pm.CPU, cpuReqStr, true)
			cpuLimPct := ui.ComputePctStr(pm.CPU, cpuLimStr, true)
			memReqPct := ui.ComputePctStr(pm.Memory, memReqStr, false)
			memLimPct := ui.ComputePctStr(pm.Memory, memLimStr, false)

			// Rebuild columns: replace old CPU/Mem columns with new compact format.
			removeCols := map[string]bool{
				"CPU Use": true, "CPU Req": true, "CPU Lim": true,
				"Mem Use": true, "Mem Req": true, "Mem Lim": true,
				"CPU/R": true, "CPU/L": true, "MEM/R": true, "MEM/L": true,
			}
			var newCols []model.KeyValue
			newCols = append(newCols,
				model.KeyValue{Key: "CPU", Value: cpuUse},
				model.KeyValue{Key: "CPU/R", Value: cpuReqPct},
				model.KeyValue{Key: "CPU/L", Value: cpuLimPct},
				model.KeyValue{Key: "MEM", Value: memUse},
				model.KeyValue{Key: "MEM/R", Value: memReqPct},
				model.KeyValue{Key: "MEM/L", Value: memLimPct},
			)
			for _, kv := range item.Columns {
				if !removeCols[kv.Key] {
					newCols = append(newCols, kv)
				}
			}
			item.Columns = newCols
		}
		// Only update the baseline every 60s so trend arrows persist longer.
		if m.prevPodMetrics == nil || time.Since(m.prevPodMetricsTime) > 60*time.Second {
			m.prevPodMetrics = msg.metrics
			m.prevPodMetricsTime = time.Now()
		}
		// Update cache.
		m.itemCache[m.navKey()] = m.middleItems
		return m, nil

	case nodeMetricsEnrichedMsg:
		if msg.gen != m.requestGen {
			return m, nil
		}
		if len(msg.metrics) == 0 {
			return m, nil
		}
		for i := range m.middleItems {
			item := &m.middleItems[i]
			nm, ok := msg.metrics[item.Name]
			if !ok {
				continue
			}

			// Look up allocatable values from item columns.
			var cpuAllocStr, memAllocStr string
			for _, kv := range item.Columns {
				switch kv.Key {
				case "CPU Alloc":
					cpuAllocStr = kv.Value
				case "Mem Alloc":
					memAllocStr = kv.Value
				}
			}

			cpuUse := ui.FormatCPU(nm.CPU)
			memUse := ui.FormatMemory(nm.Memory)

			// Detect significant usage trends (arrows before value).
			if m.prevNodeMetrics != nil {
				if prev, ok := m.prevNodeMetrics[item.Name]; ok {
					cpuDiff := nm.CPU - prev.CPU
					memDiff := nm.Memory - prev.Memory
					// CPU: significant if >10% change AND >20m absolute change.
					if prev.CPU > 0 {
						pctChange := float64(cpuDiff) / float64(prev.CPU)
						if pctChange > 0.10 && cpuDiff > 20 {
							cpuUse = "↑ " + cpuUse
						} else if pctChange < -0.10 && cpuDiff < -20 {
							cpuUse = "↓ " + cpuUse
						}
					}
					// Memory: significant if >10% change AND >20Mi absolute change.
					if prev.Memory > 0 {
						pctChange := float64(memDiff) / float64(prev.Memory)
						if pctChange > 0.10 && memDiff > 20*1024*1024 {
							memUse = "↑ " + memUse
						} else if pctChange < -0.10 && memDiff < -20*1024*1024 {
							memUse = "↓ " + memUse
						}
					}
				}
			}

			cpuPct := ui.ComputePctStr(nm.CPU, cpuAllocStr, true)
			memPct := ui.ComputePctStr(nm.Memory, memAllocStr, false)

			removeCols := map[string]bool{
				"CPU": true, "CPU%": true, "MEM": true, "MEM%": true,
				"CPU Alloc": true, "Mem Alloc": true,
			}
			var newCols []model.KeyValue
			newCols = append(newCols,
				model.KeyValue{Key: "CPU", Value: cpuUse},
				model.KeyValue{Key: "CPU%", Value: cpuPct},
				model.KeyValue{Key: "MEM", Value: memUse},
				model.KeyValue{Key: "MEM%", Value: memPct},
			)
			for _, kv := range item.Columns {
				if !removeCols[kv.Key] {
					newCols = append(newCols, kv)
				}
			}
			item.Columns = newCols
		}
		// Only update the baseline every 60s so trend arrows persist longer.
		if m.prevNodeMetrics == nil || time.Since(m.prevNodeMetricsTime) > 60*time.Second {
			m.prevNodeMetrics = msg.metrics
			m.prevNodeMetricsTime = time.Now()
		}
		m.itemCache[m.navKey()] = m.middleItems
		return m, nil

	case secretDataLoadedMsg:
		if msg.err != nil {
			m.setErrorFromErr("Error loading secret: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.secretData = msg.data
		m.secretCursor = 0
		m.secretRevealed = make(map[string]bool)
		m.secretAllRevealed = false
		m.secretEditing = false
		m.secretEditColumn = -1
		m.overlay = overlaySecretEditor
		return m, nil

	case dashboardLoadedMsg:
		if msg.context == m.nav.Context {
			m.dashboardPreview = msg.content
		}
		return m, nil

	case monitoringDashboardMsg:
		if msg.context == m.nav.Context {
			m.monitoringPreview = msg.content
		}
		return m, nil

	case secretSavedMsg:
		if msg.err != nil {
			m.setErrorFromErr("Error saving secret: ", msg.err)
		} else {
			m.setStatusMessage("Secret saved", false)
			m.overlay = overlayNone
		}
		return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())

	case configMapDataLoadedMsg:
		if msg.err != nil {
			m.setErrorFromErr("Error loading configmap: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.configMapData = msg.data
		m.configMapCursor = 0
		m.configMapEditing = false
		m.configMapEditColumn = -1
		m.overlay = overlayConfigMapEditor
		return m, nil

	case configMapSavedMsg:
		if msg.err != nil {
			m.setErrorFromErr("Error saving configmap: ", msg.err)
		} else {
			m.setStatusMessage("ConfigMap saved", false)
			m.overlay = overlayNone
		}
		return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())

	case labelDataLoadedMsg:
		if msg.err != nil {
			m.setErrorFromErr("Error loading labels: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.labelData = msg.data
		m.labelCursor = 0
		m.labelTab = 0
		m.labelEditing = false
		m.labelEditColumn = -1
		m.overlay = overlayLabelEditor
		return m, nil

	case labelSavedMsg:
		if msg.err != nil {
			m.setErrorFromErr("Error saving labels: ", msg.err)
		} else {
			m.setStatusMessage("Labels/annotations saved", false)
			m.overlay = overlayNone
		}
		return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())

	case autoSyncLoadedMsg:
		if msg.err != nil {
			m.setErrorFromErr("Loading autosync config: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.autoSyncEnabled = msg.enabled
		m.autoSyncSelfHeal = msg.selfHeal
		m.autoSyncPrune = msg.prune
		m.autoSyncCursor = 0
		m.overlay = overlayAutoSync
		return m, nil

	case autoSyncSavedMsg:
		if msg.err != nil {
			m.setErrorFromErr("Saving autosync config: ", msg.err)
		} else {
			m.setStatusMessage("AutoSync configuration updated", false)
			m.overlay = overlayNone
		}
		return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())

	case exportDoneMsg:
		if msg.err != nil {
			m.setErrorFromErr("Export failed: ", msg.err)
		} else {
			m.setStatusMessage("Exported to "+msg.path, false)
		}
		return m, scheduleStatusClear()

	case revisionListMsg:
		if msg.err != nil {
			m.setErrorFromErr("Error loading revisions: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.rollbackRevisions = msg.revisions
		m.rollbackCursor = 0
		m.overlay = overlayRollback
		return m, nil

	case rollbackDoneMsg:
		if msg.err != nil {
			m.setErrorFromErr("Rollback failed: ", msg.err)
		} else {
			m.setStatusMessage("Rollback successful", false)
			m.overlay = overlayNone
		}
		return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())

	case helmRevisionListMsg:
		if msg.err != nil {
			m.setErrorFromErr("Error loading Helm revisions: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.helmRollbackRevisions = msg.revisions
		m.helmRollbackCursor = 0
		m.overlay = overlayHelmRollback
		return m, nil

	case helmRollbackDoneMsg:
		if msg.err != nil {
			m.setErrorFromErr("Helm rollback failed: ", msg.err)
		} else {
			m.setStatusMessage("Helm rollback successful", false)
			m.overlay = overlayNone
		}
		return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())

	case templateApplyMsg:
		// Editor closed; only apply if the file was actually saved (modified).
		if !msg.origModTime.IsZero() {
			if fi, err := os.Stat(msg.tmpFile); err == nil && fi.ModTime().Equal(msg.origModTime) {
				_ = os.Remove(msg.tmpFile)
				m.setStatusMessage("Template not saved — apply skipped", false)
				return m, scheduleStatusClear()
			}
		}
		return m, m.applyTemplateFile(msg.tmpFile, msg.context, msg.ns)

	case execPTYTickMsg:
		// Only tick if this belongs to the current tab's PTY.
		if msg.ptmx != m.execPTY || m.mode != modeExec {
			// Check if a background tab owns this PTY — keep ticking for it.
			for i := range m.tabs {
				if i != m.activeTab && m.tabs[i].execPTY == msg.ptmx && m.tabs[i].execPTY != nil {
					ptmx := msg.ptmx
					return m, tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
						return execPTYTickMsg{ptmx: ptmx}
					})
				}
			}
			return m, nil
		}
		// Continue ticking to refresh the terminal view.
		return m, m.scheduleExecTick()

	case execPTYExitMsg:
		if msg.ptmx == m.execPTY {
			m.execDone.Store(true)
		} else {
			// Mark the background tab's exec as done.
			for i := range m.tabs {
				if m.tabs[i].execPTY == msg.ptmx {
					m.tabs[i].execDone.Store(true)
					break
				}
			}
		}
		return m, nil

	case execPTYStartMsg:
		m.execPTY = msg.ptmx
		m.execTerm = msg.term
		m.execTitle = msg.title
		m.execDone = &atomic.Bool{}
		m.execMu = &sync.Mutex{}
		m.mode = modeExec

		// Start background reader goroutine.
		startExecPTYReader(msg.ptmx, msg.term, msg.cmd, m.execMu, m.execDone)

		return m, m.scheduleExecTick()

	case logLineMsg:
		// Check if this message belongs to the current tab's log channel.
		if msg.ch != m.logCh {
			// Message from a background tab's log stream — buffer it into that tab's state.
			for i := range m.tabs {
				if m.tabs[i].logCh == msg.ch {
					if msg.done {
						m.tabs[i].logLines = append(m.tabs[i].logLines, "--- stream ended ---")
					} else {
						m.tabs[i].logLines = append(m.tabs[i].logLines, msg.line)
						// Continue draining: re-issue waitForLogLine for that channel.
						ch := msg.ch
						return m, func() tea.Msg {
							line, ok := <-ch
							if !ok {
								return logLineMsg{done: true, ch: ch}
							}
							return logLineMsg{line: line, ch: ch}
						}
					}
					break
				}
			}
			return m, nil
		}
		if msg.done {
			m.logLines = append(m.logLines, "--- stream ended ---")
			if m.logFollow {
				m.logScroll = m.logMaxScroll()
				m.logCursor = len(m.logLines) - 1
			}
			return m, nil
		}
		m.logLines = append(m.logLines, msg.line)
		if m.logFollow {
			m.logScroll = m.logMaxScroll()
			m.logCursor = len(m.logLines) - 1
		}
		return m, m.waitForLogLine()

	case logHistoryMsg:
		m.logLoadingHistory = false
		if msg.err != nil {
			m.logHasMoreHistory = false
			return m, nil
		}
		if m.mode != modeLogs {
			return m, nil
		}

		// Find overlap: search for the first 3 current lines in the fetched history.
		overlapIdx := -1
		if len(m.logLines) >= 3 && len(msg.lines) > 3 {
			first3 := m.logLines[:3]
			for i := len(msg.lines) - 3; i >= 0; i-- {
				if msg.lines[i] == first3[0] && msg.lines[i+1] == first3[1] && msg.lines[i+2] == first3[2] {
					overlapIdx = i
					break
				}
			}
		} else if len(m.logLines) > 0 && len(msg.lines) > 0 {
			// Single-line fallback.
			for i := len(msg.lines) - 1; i >= 0; i-- {
				if msg.lines[i] == m.logLines[0] {
					overlapIdx = i
					break
				}
			}
		}

		var newOlderLines []string
		if overlapIdx > 0 {
			newOlderLines = msg.lines[:overlapIdx]
		} else if overlapIdx == -1 && len(msg.lines) > 0 {
			// No overlap found; prepend all (logs may have rotated).
			newOlderLines = msg.lines
		}

		if len(newOlderLines) == 0 {
			m.logHasMoreHistory = false
			return m, nil
		}

		// Prepend and adjust scroll to maintain view position.
		prepended := len(newOlderLines)
		m.logLines = append(newOlderLines, m.logLines...)
		m.logScroll += prepended
		if m.logCursor >= 0 {
			m.logCursor += prepended
		}
		m.logTailLines += ui.ConfigLogTailLines

		// Cap total to prevent unbounded growth.
		if m.logTailLines > 100000 {
			m.logHasMoreHistory = false
		}

		return m, nil

	case logSaveAllMsg:
		if msg.err != nil {
			m.setErrorFromErr("Log save failed: ", msg.err)
			return m, scheduleStatusClear()
		}
		m.setStatusMessage("All logs saved to "+msg.path, false)
		return m, scheduleStatusClear()
	}

	return m, nil
}
