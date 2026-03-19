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
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

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
			// Multiple pods; show inline pod selector overlay.
			m.overlayItems = pods
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
		m.overlay = overlayCanI
		m.canIGroupCursor = 0
		m.canIGroupScroll = 0
		return m, nil

	case canISAListMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatusMessage(fmt.Sprintf("Failed to list service accounts: %v", msg.err), true)
			return m, scheduleStatusClear()
		}
		m.canIServiceAccounts = msg.accounts
		// Build overlay items: "Current User" + ServiceAccounts.
		items := make([]model.Item, 0, len(msg.accounts)+1)
		items = append(items, model.Item{Name: "Current User", Extra: ""})
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
				Name:  ns + "/" + name,
				Extra: fmt.Sprintf("system:serviceaccount:%s:%s", ns, name),
			})
		}
		m.overlayItems = items
		m.overlayCursor = 0
		m.canISubjectScroll = 0
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
		m.describeScroll = 0
		m.describeTitle = msg.title
		return m, nil

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

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Dismiss startup tip on any keypress.
	if m.statusMessageTip {
		m.statusMessage = ""
		m.statusMessageTip = false
	}

	// Handle error log overlay first (independent of regular overlays).
	if m.overlayErrorLog {
		return m.handleErrorLogOverlayKey(msg)
	}

	// Handle overlays first.
	if m.overlay != overlayNone {
		return m.handleOverlayKey(msg)
	}

	// Handle command bar input mode.
	if m.commandBarActive {
		return m.handleCommandBarKey(msg)
	}

	// Handle filter input mode.
	if m.filterActive {
		return m.handleFilterKey(msg)
	}

	// Handle search input mode.
	if m.searchActive {
		return m.handleSearchKey(msg)
	}

	// Tab switching keys work in all fullscreen modes (YAML, Logs, Describe, Diff, Help)
	// as long as the user is not in a text input sub-mode (search, etc.).
	if m.mode != modeExplorer && m.mode != modeExec && !m.yamlSearchMode && !m.logSearchActive && !m.helpSearchActive && !m.explainSearchActive {
		switch msg.String() {
		case "]":
			if len(m.tabs) > 1 {
				m.saveCurrentTab() // save mode + log state BEFORE switching
				next := (m.activeTab + 1) % len(m.tabs)
				if cmd := m.loadTab(next); cmd != nil {
					return m, cmd
				}
				if m.mode == modeExplorer {
					return m, m.loadPreview()
				}
				if m.mode == modeLogs && m.logCh != nil {
					return m, m.waitForLogLine()
				}
				if m.mode == modeExec && m.execPTY != nil {
					return m, m.scheduleExecTick()
				}
				return m, nil
			}
		case "[":
			if len(m.tabs) > 1 {
				m.saveCurrentTab()
				prev := (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
				if cmd := m.loadTab(prev); cmd != nil {
					return m, cmd
				}
				if m.mode == modeExplorer {
					return m, m.loadPreview()
				}
				if m.mode == modeLogs && m.logCh != nil {
					return m, m.waitForLogLine()
				}
				if m.mode == modeExec && m.execPTY != nil {
					return m, m.scheduleExecTick()
				}
				return m, nil
			}
		case "t":
			if m.mode != modeHelp { // 't' is not used as a regular key in fullscreen modes
				if len(m.tabs) >= 9 {
					m.setStatusMessage("Max 9 tabs", true)
					return m, scheduleStatusClear()
				}
				m.saveCurrentTab()
				newTab := m.cloneCurrentTab()
				// New tab always starts in explorer mode.
				newTab.mode = modeExplorer
				newTab.logLines = nil
				newTab.logCancel = nil
				newTab.logCh = nil
				insertAt := m.activeTab + 1
				m.tabs = append(m.tabs[:insertAt], append([]TabState{newTab}, m.tabs[insertAt:]...)...)
				m.activeTab = insertAt
				m.loadTab(m.activeTab) // cloned tab is always fully loaded, ignore returned cmd
				m.setStatusMessage(fmt.Sprintf("Tab %d created", m.activeTab+1), false)
				return m, scheduleStatusClear()
			}
		}
	}

	// Embedded terminal mode: forward keys to PTY.
	if m.mode == modeExec {
		return m.handleExecKey(msg)
	}

	// In help view mode.
	if m.mode == modeHelp {
		return m.handleHelpKey(msg)
	}

	// In log viewer mode.
	if m.mode == modeLogs {
		return m.handleLogKey(msg)
	}

	// In diff view mode.
	if m.mode == modeDiff {
		return m.handleDiffKey(msg)
	}

	// In describe view mode.
	if m.mode == modeDescribe {
		return m.handleDescribeKey(msg)
	}

	// In explain view mode: handle search input before general keys.
	if m.mode == modeExplain && m.explainSearchActive {
		return m.handleExplainSearchKey(msg)
	}
	if m.mode == modeExplain {
		return m.handleExplainKey(msg)
	}

	// In YAML view mode.
	if m.mode == modeYAML {
		return m.handleYAMLKey(msg)
	}

	// Clear pending 'g' state if any other key is pressed (vim-style gg).
	if m.pendingG && msg.String() != "g" {
		m.pendingG = false
	}

	// Vim-style named marks: m<key> saves bookmark to slot, '<key> jumps to slot.
	if m.pendingMark {
		m.pendingMark = false
		key := msg.String()
		if len(key) == 1 && ((key[0] >= 'a' && key[0] <= 'z') || (key[0] >= '0' && key[0] <= '9')) {
			return m.bookmarkToSlot(key)
		}
		// Invalid slot key — ignore silently.
		return m, nil
	}

	// Bookmark overwrite confirmation: y accepts, anything else cancels.
	if m.pendingBookmark != nil {
		bm := m.pendingBookmark
		m.pendingBookmark = nil
		if msg.String() == "y" || msg.String() == "Y" {
			return m.saveBookmark(*bm)
		}
		m.setStatusMessage("Cancelled", false)
		return m, scheduleStatusClear()
	}

	switch msg.String() {
	case "q":
		m.overlay = overlayQuitConfirm
		return m, nil

	case "ctrl+c":
		return m.closeTabOrQuit()

	case "T":
		m.schemeEntries = ui.GroupedSchemeEntries()
		m.schemeCursor = 0
		m.schemeFilter.Clear()
		m.schemeOriginalName = ui.ActiveSchemeName
		// Position cursor on the currently active scheme.
		selectIdx := 0
		for _, e := range m.schemeEntries {
			if e.IsHeader {
				continue
			}
			if e.Name == ui.ActiveSchemeName {
				m.schemeCursor = selectIdx
				break
			}
			selectIdx++
		}
		m.overlay = overlayColorscheme
		return m, nil

	case "esc":
		if m.fullscreenDashboard {
			m.fullscreenDashboard = false
			m.previewScroll = 0
			m.setStatusMessage("Dashboard fullscreen OFF", false)
			return m, scheduleStatusClear()
		}
		if m.hasSelection() {
			m.clearSelection()
			return m, nil
		}
		if m.filterText != "" {
			m.filterText = ""
			m.setCursor(0)
			m.clampCursor()
			return m, m.loadPreview()
		}
		if m.nav.Level == model.LevelClusters {
			if len(m.tabs) > 1 {
				// Close current tab instead of quitting when multiple tabs are open.
				m.tabs = append(m.tabs[:m.activeTab], m.tabs[m.activeTab+1:]...)
				if m.activeTab >= len(m.tabs) {
					m.activeTab = len(m.tabs) - 1
				}
				if cmd := m.loadTab(m.activeTab); cmd != nil {
					return m, cmd
				}
				return m, m.loadPreview()
			}
			return m, tea.Quit
		}
		return m.navigateParent()

	case "j", "down":
		if m.fullscreenDashboard {
			m.previewScroll++
			m.clampPreviewScroll()
			return m, nil
		}
		return m.moveCursor(1)

	case "k", "up":
		if m.fullscreenDashboard {
			if m.previewScroll > 0 {
				m.previewScroll--
			}
			return m, nil
		}
		return m.moveCursor(-1)

	case "g":
		if m.fullscreenDashboard {
			if m.pendingG {
				m.pendingG = false
				m.previewScroll = 0
				return m, nil
			}
			m.pendingG = true
			return m, nil
		}
		if m.pendingG {
			m.pendingG = false
			m.setCursor(0)
			m.clampCursor()
			m.syncExpandedGroup()
			return m, m.loadPreview()
		}
		m.pendingG = true
		return m, nil

	case "G":
		if m.fullscreenDashboard {
			m.previewScroll = 99999 // will be clamped
			m.clampPreviewScroll()
			return m, nil
		}
		visible := m.visibleMiddleItems()
		if len(visible) > 0 {
			m.setCursor(len(visible) - 1)
		}
		m.syncExpandedGroup()
		return m, m.loadPreview()

	case "ctrl+@": // ctrl+space: region selection
		if m.nav.Level < model.LevelResources {
			return m, nil
		}
		items := m.visibleMiddleItems()
		if len(items) == 0 {
			return m, nil
		}
		cur := m.cursor()
		if m.selectionAnchor < 0 {
			// No anchor set — toggle current item and set anchor.
			if sel := m.selectedMiddleItem(); sel != nil {
				m.toggleSelection(*sel)
				m.selectionAnchor = cur
			}
			return m, nil
		}
		// Select range from anchor to cursor (inclusive).
		lo, hi := m.selectionAnchor, cur
		if lo > hi {
			lo, hi = hi, lo
		}
		for i := lo; i <= hi && i < len(items); i++ {
			m.selectedItems[selectionKey(items[i])] = true
		}
		return m, nil

	case " ":
		// Toggle selection on current item and move cursor down.
		if m.nav.Level >= model.LevelResources {
			sel := m.selectedMiddleItem()
			if sel != nil {
				m.toggleSelection(*sel)
				// Set anchor when selecting, reset when deselecting.
				if m.isSelected(*sel) {
					m.selectionAnchor = m.cursor()
				} else {
					m.selectionAnchor = -1
				}
			}
			// Move cursor down.
			visible := m.visibleMiddleItems()
			c := m.cursor() + 1
			if c >= len(visible) {
				c = len(visible) - 1
			}
			if c < 0 {
				c = 0
			}
			m.setCursor(c)
			return m, nil
		}
		return m, nil

	case "ctrl+a":
		// Select/deselect all visible items.
		if m.nav.Level >= model.LevelResources {
			visible := m.visibleMiddleItems()
			if m.hasSelection() {
				// If any are selected, deselect all.
				m.clearSelection()
			} else {
				// Select all visible items.
				for _, item := range visible {
					m.selectedItems[selectionKey(item)] = true
				}
			}
			m.selectionAnchor = -1
			return m, nil
		}
		return m, nil

	case "h", "left":
		if m.fullscreenDashboard {
			m.fullscreenDashboard = false
			m.previewScroll = 0
			m.setStatusMessage("Dashboard fullscreen OFF", false)
			return m, scheduleStatusClear()
		}
		return m.navigateParent()

	case "l", "right":
		return m.navigateChild()

	case "enter":
		return m.enterFullView()

	case "n":
		// Jump to next search match.
		if m.searchInput.Value != "" {
			m.jumpToSearchMatch(m.cursor() + 1)
			m.syncExpandedGroup()
			return m, m.loadPreview()
		}
		return m, nil

	case "N":
		// Jump to previous search match.
		if m.searchInput.Value != "" {
			m.jumpToPrevSearchMatch(m.cursor() - 1)
			m.syncExpandedGroup()
			return m, m.loadPreview()
		}
		return m, nil

	case "\\":
		// Open namespace selector immediately with loading state.
		m.overlay = overlayNamespace
		m.overlayFilter.Clear()
		m.overlayCursor = 0
		m.overlayItems = nil // will be populated when namespacesLoadedMsg arrives
		ui.ResetOverlayNsScroll()
		m.loading = true
		m.nsSelectionModified = false
		return m, m.loadNamespaces()

	case "x":
		return m.openActionMenu()

	case "w":
		m.watchMode = !m.watchMode
		if m.watchMode {
			m.setStatusMessage("Watch mode ON (refresh every 2s)", false)
			return m, tea.Batch(scheduleWatchTick(m.watchInterval), scheduleStatusClear())
		}
		m.setStatusMessage("Watch mode OFF", false)
		return m, scheduleStatusClear()

	case "z":
		// Toggle expand/collapse all groups at resource types level.
		if m.nav.Level == model.LevelResourceTypes {
			if m.allGroupsExpanded {
				// Collapsing: find current item's category BEFORE changing mode.
				visible := m.visibleMiddleItems()
				c := m.cursor()
				if c >= 0 && c < len(visible) {
					m.expandedGroup = visible[c].Category
				}
				m.allGroupsExpanded = false
				// Find the same item in the now-collapsed visible list.
				if c >= 0 && c < len(visible) {
					targetItem := visible[c]
					newVisible := m.visibleMiddleItems()
					for i, item := range newVisible {
						if item.Name == targetItem.Name && item.Kind == targetItem.Kind && item.Category == targetItem.Category {
							m.setCursor(i)
							break
						}
					}
				}
				m.clampCursor()
				m.setStatusMessage("Groups collapsed (accordion mode)", false)
			} else {
				// Expanding: find current item BEFORE changing mode.
				visible := m.visibleMiddleItems()
				c := m.cursor()
				var targetItem model.Item
				if c >= 0 && c < len(visible) {
					targetItem = visible[c]
				}
				m.allGroupsExpanded = true
				// Find the same item in the now-expanded visible list.
				if targetItem.Name != "" {
					newVisible := m.visibleMiddleItems()
					for i, item := range newVisible {
						if item.Name == targetItem.Name && item.Kind == targetItem.Kind && item.Category == targetItem.Category {
							m.setCursor(i)
							break
						}
					}
				}
				m.clampCursor()
				m.setStatusMessage("All groups expanded", false)
			}
			return m, tea.Batch(m.loadPreview(), scheduleStatusClear())
		}
		return m, nil

	case "p":
		// Pin/unpin CRD group at resource types level.
		if m.nav.Level == model.LevelResourceTypes {
			sel := m.selectedMiddleItem()
			if sel == nil || sel.Category == "" {
				return m, nil
			}
			// Don't allow pinning core categories.
			if model.IsCoreCategory(sel.Category) {
				m.setStatusMessage("Cannot pin built-in category", true)
				return m, scheduleStatusClear()
			}
			pinned := togglePinnedGroup(m.pinnedState, m.nav.Context, sel.Category)
			_ = savePinnedState(m.pinnedState)
			m.applyPinnedGroups()
			// Refresh the resource type list to reflect new ordering.
			if pinned {
				m.setStatusMessage(fmt.Sprintf("Pinned: %s", sel.Category), false)
			} else {
				m.setStatusMessage(fmt.Sprintf("Unpinned: %s", sel.Category), false)
			}
			// Reload resource types to apply the new pin order.
			return m, tea.Batch(m.loadResourceTypes(), scheduleStatusClear())
		}
		return m, nil

	case "'":
		// Open bookmarks overlay immediately; slot keys (a-z, 0-9) jump from within the overlay.
		m.overlay = overlayBookmarks
		m.overlayCursor = 0
		m.bookmarkFilter.Clear()
		return m, nil

	case "m":
		// Vim-style set mark: wait for slot key.
		m.pendingMark = true
		return m, nil

	case "?":
		m.helpPreviousMode = modeExplorer
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		// Set contextual help mode based on the current overlay/view.
		switch m.overlay {
		case overlayBookmarks:
			m.helpContextMode = "Bookmarks"
		default:
			m.helpContextMode = "Navigation"
		}
		return m, nil

	case "f":
		m.filterActive = true
		m.filterInput.Clear()
		m.filterText = ""
		m.setCursor(0)
		m.clampCursor()
		return m, nil

	case "/":
		m.searchActive = true
		m.searchInput.Clear()
		m.searchPrevCursor = m.cursor()
		return m, nil

	case ":":
		// Open kubectl command bar.
		m.commandBarActive = true
		m.commandBarInput.Clear()
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		m.commandHistory.reset()
		return m, nil

	case "M":
		// Toggle resource relationship map in the right column.
		if m.nav.Level >= model.LevelResources && (m.resourceTypeHasChildren() || m.nav.ResourceType.Kind == "Pod" || m.nav.ResourceType.Kind == "ReplicaSet") {
			m.mapView = !m.mapView
			if m.mapView {
				m.fullYAMLPreview = false
				m.previewScroll = 0
				m.setStatusMessage("Resource map", false)
				return m, tea.Batch(m.loadResourceTree(), scheduleStatusClear())
			}
			m.resourceTree = nil
			m.setStatusMessage("Details preview", false)
			return m, tea.Batch(m.loadPreview(), scheduleStatusClear())
		}
		return m, nil

	case "P":
		// No preview toggle on dashboard views.
		if sel := m.selectedMiddleItem(); sel != nil && m.nav.Level == model.LevelResourceTypes &&
			(sel.Extra == "__overview__" || sel.Extra == "__monitoring__") {
			return m, nil
		}
		// Toggle between split view (children + details) and full YAML preview.
		m.fullYAMLPreview = !m.fullYAMLPreview
		m.mapView = false
		m.resourceTree = nil
		if m.fullYAMLPreview {
			m.setStatusMessage("YAML preview", false)
		} else {
			m.previewYAML = ""
			m.setStatusMessage("Details preview", false)
		}
		return m, tea.Batch(m.loadPreview(), scheduleStatusClear())

	case "F":
		// If standing on Cluster Dashboard or Monitoring, toggle fullscreen dashboard instead.
		sel := m.selectedMiddleItem()
		if sel != nil && (sel.Extra == "__overview__" || sel.Extra == "__monitoring__") && m.nav.Level == model.LevelResourceTypes {
			m.fullscreenDashboard = !m.fullscreenDashboard
			m.previewScroll = 0
			if m.fullscreenDashboard {
				m.setStatusMessage("Dashboard fullscreen ON", false)
			} else {
				m.setStatusMessage("Dashboard fullscreen OFF", false)
			}
			return m, scheduleStatusClear()
		}
		// Toggle fullscreen middle column.
		m.fullscreenMiddle = !m.fullscreenMiddle
		if m.fullscreenMiddle {
			m.setStatusMessage("Fullscreen ON", false)
		} else {
			m.setStatusMessage("Fullscreen OFF", false)
		}
		return m, scheduleStatusClear()

	case "ctrl+s":
		// Toggle secret value visibility.
		m.showSecretValues = !m.showSecretValues
		if m.showSecretValues {
			m.setStatusMessage("Secret values VISIBLE", false)
		} else {
			m.setStatusMessage("Secret values HIDDEN", false)
		}
		return m, scheduleStatusClear()

	case "A":
		m.allNamespaces = !m.allNamespaces
		if m.allNamespaces {
			m.selectedNamespaces = nil
			m.setStatusMessage("All namespaces mode ON", false)
		} else {
			m.setStatusMessage("All namespaces mode OFF (ns: "+m.namespace+")", false)
		}
		m.cancelAndReset()
		m.requestGen++
		return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())

	case "Q":
		m.loading = true
		m.setStatusMessage("Loading quota data...", false)
		return m, m.loadQuotas()

	case "ctrl+d":
		if m.fullscreenDashboard {
			halfPage := (m.height - 4) / 2
			m.previewScroll += halfPage
			m.clampPreviewScroll()
			return m, nil
		}
		visible := m.visibleMiddleItems()
		halfPage := (m.height - 4) / 2
		c := m.cursor() + halfPage
		if c >= len(visible) {
			c = len(visible) - 1
		}
		if c < 0 {
			c = 0
		}
		m.setCursor(c)
		m.syncExpandedGroup()
		m.rightItems = nil
		m.previewYAML = ""
		m.previewScroll = 0
		m.loading = true
		return m, m.loadPreview()

	case "ctrl+u":
		if m.fullscreenDashboard {
			halfPage := (m.height - 4) / 2
			m.previewScroll -= halfPage
			if m.previewScroll < 0 {
				m.previewScroll = 0
			}
			return m, nil
		}
		visible := m.visibleMiddleItems()
		halfPage := (m.height - 4) / 2
		c := m.cursor() - halfPage
		if c < 0 {
			c = 0
		}
		if c >= len(visible) {
			c = len(visible) - 1
		}
		m.setCursor(c)
		m.syncExpandedGroup()
		m.rightItems = nil
		m.previewYAML = ""
		m.previewScroll = 0
		m.loading = true
		return m, m.loadPreview()

	case "ctrl+f":
		if m.fullscreenDashboard {
			fullPage := m.height - 4
			m.previewScroll += fullPage
			m.clampPreviewScroll()
			return m, nil
		}
		visible := m.visibleMiddleItems()
		fullPage := m.height - 4
		c := m.cursor() + fullPage
		if c >= len(visible) {
			c = len(visible) - 1
		}
		if c < 0 {
			c = 0
		}
		m.setCursor(c)
		m.syncExpandedGroup()
		m.rightItems = nil
		m.previewYAML = ""
		m.previewScroll = 0
		m.loading = true
		return m, m.loadPreview()

	case "ctrl+b":
		if m.fullscreenDashboard {
			fullPage := m.height - 4
			m.previewScroll -= fullPage
			if m.previewScroll < 0 {
				m.previewScroll = 0
			}
			return m, nil
		}
		visible := m.visibleMiddleItems()
		fullPage := m.height - 4
		c := m.cursor() - fullPage
		if c < 0 {
			c = 0
		}
		if c >= len(visible) {
			c = len(visible) - 1
		}
		m.setCursor(c)
		m.syncExpandedGroup()
		m.rightItems = nil
		m.previewYAML = ""
		m.previewScroll = 0
		m.loading = true
		return m, m.loadPreview()

	case "0":
		// Jump to clusters level.
		for m.nav.Level > model.LevelClusters {
			ret, _ := m.navigateParent()
			m = ret.(Model)
		}
		return m, m.loadPreview()

	case "1":
		// Jump to resource types level.
		if m.nav.Level < model.LevelResourceTypes {
			return m, nil // can't jump forward
		}
		for m.nav.Level > model.LevelResourceTypes {
			ret, _ := m.navigateParent()
			m = ret.(Model)
		}
		return m, m.loadPreview()

	case "2":
		// Jump to resources level.
		if m.nav.Level < model.LevelResources {
			return m, nil
		}
		for m.nav.Level > model.LevelResources {
			ret, _ := m.navigateParent()
			m = ret.(Model)
		}
		return m, m.loadPreview()

	case "W":
		if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "Event" {
			m.warningEventsOnly = !m.warningEventsOnly
			// Re-filter the current items.
			if cached, ok := m.itemCache[m.navKey()]; ok {
				if m.warningEventsOnly {
					var filtered []model.Item
					for _, item := range cached {
						if item.Status == "Warning" {
							filtered = append(filtered, item)
						}
					}
					m.middleItems = filtered
				} else {
					m.middleItems = cached
				}
				m.clampCursor()
			}
			if m.warningEventsOnly {
				m.setStatusMessage("Showing warnings only", false)
			} else {
				m.setStatusMessage("Showing all events", false)
			}
			return m, scheduleStatusClear()
		}
		// Save resource YAML to file (same as 'S').
		if m.nav.Level == model.LevelResources || m.nav.Level == model.LevelOwned || m.nav.Level == model.LevelContainers {
			sel := m.selectedMiddleItem()
			if sel != nil {
				m.setStatusMessage("Exporting...", false)
				return m, m.exportResourceToFile()
			}
		}
		return m, nil

	case "J":
		// Scroll preview pane down, clamped to content length.
		m.previewScroll++
		m.clampPreviewScroll()
		return m, nil

	case "K":
		// Scroll preview pane up.
		if m.previewScroll > 0 {
			m.previewScroll--
		}
		return m, nil

	case "o":
		// Navigate to owner/controller.
		sel := m.selectedMiddleItem()
		if sel == nil {
			return m, nil
		}
		type ownerRef struct {
			kind, name, apiVersion string
		}
		var owners []ownerRef
		for _, kv := range sel.Columns {
			if strings.HasPrefix(kv.Key, "owner:") {
				parts := strings.SplitN(kv.Value, "||", 3)
				if len(parts) == 3 {
					owners = append(owners, ownerRef{
						apiVersion: parts[0],
						kind:       parts[1],
						name:       parts[2],
					})
				}
			}
		}
		if len(owners) == 0 {
			m.setStatusMessage("No owner references found", true)
			return m, scheduleStatusClear()
		}
		// Use first owner (most resources have exactly one).
		owner := owners[0]
		return m.navigateToOwner(owner.kind, owner.name)

	case ",":
		m.sortBy = (m.sortBy + 1) % 3
		m.sortMiddleItems()
		m.clampCursor()
		m.setStatusMessage("Sort: "+m.sortModeName(), false)
		return m, tea.Batch(m.loadPreview(), scheduleStatusClear())

	case "ctrl+o":
		// Open ingress host in browser (works when viewing an Ingress resource).
		kind := m.selectedResourceKind()
		if kind != "Ingress" {
			m.setStatusMessage("Open in browser is only available for Ingress resources", true)
			return m, scheduleStatusClear()
		}
		sel := m.selectedMiddleItem()
		if sel == nil {
			return m, nil
		}
		m.actionCtx = m.buildActionCtx(sel, kind)
		return m.executeAction("Open in Browser")

	case "c", "y":
		// Copy resource name to clipboard.
		sel := m.selectedMiddleItem()
		if sel != nil {
			m.setStatusMessage("Copied: "+sel.Name, false)
			return m, tea.Batch(copyToSystemClipboard(sel.Name), scheduleStatusClear())
		}
		return m, nil

	case "C", "ctrl+y":
		// Copy resource YAML to clipboard.
		sel := m.selectedMiddleItem()
		if sel != nil {
			return m, m.copyYAMLToClipboard()
		}
		return m, nil

	case "ctrl+p":
		return m, m.applyFromClipboard()

	case "t":
		// Create new tab (clone current state, max 9).
		if len(m.tabs) >= 9 {
			m.setStatusMessage("Max 9 tabs", true)
			return m, scheduleStatusClear()
		}
		m.saveCurrentTab()
		insertAt := m.activeTab + 1
		newTab := m.cloneCurrentTab()
		m.tabs = append(m.tabs[:insertAt], append([]TabState{newTab}, m.tabs[insertAt:]...)...)
		m.activeTab = insertAt
		m.setStatusMessage(fmt.Sprintf("Tab %d created", m.activeTab+1), false)
		return m, scheduleStatusClear()

	case "]":
		// Next tab.
		if len(m.tabs) <= 1 {
			return m, nil
		}
		m.saveCurrentTab()
		next := (m.activeTab + 1) % len(m.tabs)
		if cmd := m.loadTab(next); cmd != nil {
			return m, cmd
		}
		if m.mode == modeExec && m.execPTY != nil {
			return m, m.scheduleExecTick()
		}
		return m, m.loadPreview()

	case "[":
		// Previous tab.
		if len(m.tabs) <= 1 {
			return m, nil
		}
		m.saveCurrentTab()
		prev := (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
		if cmd := m.loadTab(prev); cmd != nil {
			return m, cmd
		}
		if m.mode == modeExec && m.execPTY != nil {
			return m, m.scheduleExecTick()
		}
		return m, m.loadPreview()

	case "a":
		// Open template creation overlay.
		m.templateItems = model.BuiltinTemplates()
		m.templateCursor = 0
		m.overlay = overlayTemplates
		return m, nil

	case "e":
		// Open secret editor when a Secret resource is selected.
		if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "Secret" {
			sel := m.selectedMiddleItem()
			if sel != nil {
				return m, m.loadSecretData()
			}
		}
		// Open configmap editor when a ConfigMap resource is selected.
		if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "ConfigMap" {
			sel := m.selectedMiddleItem()
			if sel != nil {
				return m, m.loadConfigMapData()
			}
		}
		return m, nil

	case "I":
		// Open API explain browser (resource structure).
		return m.openExplainBrowser()

	case "U":
		// Open RBAC permissions browser (can-i).
		return m.openCanIBrowser()

	case "i":
		// Open label/annotation editor for any resource (not port forwards).
		if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind != "__port_forwards__" {
			sel := m.selectedMiddleItem()
			if sel != nil {
				m.labelResourceType = m.nav.ResourceType
				return m, m.loadLabelData()
			}
		} else if m.nav.Level == model.LevelOwned {
			sel := m.selectedMiddleItem()
			if sel != nil {
				rt, ok := m.resolveOwnedResourceType(sel)
				if ok {
					m.labelResourceType = rt
					return m, m.loadLabelData()
				}
			}
		}
		return m, nil

	case ".":
		// Quick filter presets: toggle or open overlay.
		if m.nav.Level < model.LevelResources {
			m.setStatusMessage("Quick filters are only available at resource level", true)
			return m, scheduleStatusClear()
		}
		if m.activeFilterPreset != nil {
			// Clear the active filter preset and restore the full list.
			name := m.activeFilterPreset.Name
			m.activeFilterPreset = nil
			m.middleItems = m.unfilteredMiddleItems
			m.unfilteredMiddleItems = nil
			m.setCursor(0)
			m.clampCursor()
			m.setStatusMessage("Filter cleared: "+name, false)
			return m, tea.Batch(scheduleStatusClear(), m.loadPreview())
		}
		// Open the filter preset overlay.
		m.filterPresets = builtinFilterPresets(m.nav.ResourceType.Kind)
		m.overlayCursor = 0
		m.overlay = overlayFilterPreset
		return m, nil

	case "S":
		// Export resource YAML to file.
		if m.nav.Level == model.LevelResources || m.nav.Level == model.LevelOwned || m.nav.Level == model.LevelContainers {
			sel := m.selectedMiddleItem()
			if sel != nil {
				m.setStatusMessage("Exporting...", false)
				return m, m.exportResourceToFile()
			}
		}
		return m, nil

	case "d":
		// Diff two selected resources side by side.
		if m.nav.Level < model.LevelResources {
			m.setStatusMessage("Diff is only available at resource level", true)
			return m, scheduleStatusClear()
		}
		selected := m.selectedItemsList()
		if len(selected) != 2 {
			m.setStatusMessage("Select exactly 2 resources to diff (use Space to select)", true)
			return m, scheduleStatusClear()
		}
		m.loading = true
		m.setStatusMessage("Loading diff...", false)
		return m, m.loadDiff(m.nav.ResourceType, selected[0], selected[1])

	case "!":
		// Open the error log overlay.
		m.overlayErrorLog = true
		m.errorLogScroll = 0
		return m, nil

	case "@":
		// Navigate to the Monitoring dashboard item.
		if m.nav.Level < model.LevelResourceTypes {
			m.setStatusMessage("Select a cluster first", true)
			return m, scheduleStatusClear()
		}
		// Find the Monitoring item in the middle column and select it.
		for i, item := range m.middleItems {
			if item.Extra == "__monitoring__" {
				m.setCursor(i)
				m.clampCursor()
				return m, m.loadPreview()
			}
		}
		return m, nil
	}

	// Configurable direct action keybindings.
	key := msg.String()
	kb := ui.ActiveKeybindings
	if key == kb.Logs {
		return m.directActionLogs()
	}
	if key == kb.Refresh {
		return m.directActionRefresh()
	}
	if key == kb.Restart {
		return m.directActionRestart()
	}
	if key == kb.Exec {
		return m.directActionExec()
	}
	if key == kb.Edit {
		return m.directActionEdit()
	}
	if key == kb.Describe {
		return m.directActionDescribe()
	}
	if key == kb.Delete {
		return m.directActionDelete()
	}
	if key == kb.ForceDelete {
		return m.directActionForceDelete()
	}
	if key == kb.Scale {
		return m.directActionScale()
	}

	return m, nil
}

func (m Model) handleYAMLKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Compute visible line count (accounting for collapsed sections).
	totalVisible := visibleLineCount(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	viewportLines := m.height - 4
	if viewportLines < 3 {
		viewportLines = 3
	}
	maxScroll := totalVisible - viewportLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	// When in search input mode, handle text input.
	if m.yamlSearchMode {
		switch msg.String() {
		case "enter":
			m.yamlSearchMode = false
			m.updateYAMLSearchMatches()
			if len(m.yamlMatchLines) > 0 {
				m.yamlMatchIdx = m.findYAMLMatchFromCursor()
				m.yamlScrollToMatchFolded(viewportLines)
			}
			return m, nil
		case "esc":
			m.yamlSearchMode = false
			m.yamlSearchText.Clear()
			m.yamlMatchLines = nil
			m.yamlMatchIdx = 0
			return m, nil
		case "backspace":
			if len(m.yamlSearchText.Value) > 0 {
				m.yamlSearchText.Backspace()
			}
			return m, nil
		case "ctrl+w":
			m.yamlSearchText.DeleteWord()
			return m, nil
		case "ctrl+a":
			m.yamlSearchText.Home()
			return m, nil
		case "ctrl+e":
			m.yamlSearchText.End()
			return m, nil
		case "left":
			m.yamlSearchText.Left()
			return m, nil
		case "right":
			m.yamlSearchText.Right()
			return m, nil
		case "ctrl+c":
			m.yamlSearchMode = false
			m.yamlSearchText.Clear()
			m.yamlMatchLines = nil
			return m, nil
		default:
			if len(msg.String()) == 1 || msg.String() == " " {
				m.yamlSearchText.Insert(msg.String())
			}
			return m, nil
		}
	}

	// In visual selection mode, restrict keys to selection/copy/cancel.
	if m.yamlVisualMode {
		switch msg.String() {
		case "esc":
			m.yamlVisualMode = false
			return m, nil
		case "V":
			// Toggle: if already in line mode, cancel; otherwise switch to line mode.
			if m.yamlVisualType == 'V' {
				m.yamlVisualMode = false
			} else {
				m.yamlVisualType = 'V'
			}
			return m, nil
		case "v":
			// Toggle: if already in char mode, cancel; otherwise switch to char mode.
			if m.yamlVisualType == 'v' {
				m.yamlVisualMode = false
			} else {
				m.yamlVisualType = 'v'
			}
			return m, nil
		case "ctrl+v":
			// Toggle: if already in block mode, cancel; otherwise switch to block mode.
			if m.yamlVisualType == 'B' {
				m.yamlVisualMode = false
			} else {
				m.yamlVisualType = 'B'
			}
			return m, nil
		case "y":
			// Copy selected content to clipboard using original content (no fold indicators).
			yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
			_, mapping := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
			selStart := min(m.yamlVisualStart, m.yamlCursor)
			selEnd := max(m.yamlVisualStart, m.yamlCursor)
			if selStart < 0 {
				selStart = 0
			}
			if selEnd >= len(mapping) {
				selEnd = len(mapping) - 1
			}
			origLines := strings.Split(yamlForDisplay, "\n")
			var clipText string
			switch m.yamlVisualType {
			case 'v': // Character mode: partial first/last lines.
				var parts []string
				colStart := m.yamlVisualCol
				colEnd := m.yamlVisualCurCol
				for i := selStart; i <= selEnd; i++ {
					if i >= len(mapping) || mapping[i] < 0 || mapping[i] >= len(origLines) {
						continue
					}
					line := origLines[mapping[i]]
					runes := []rune(line)
					if selStart == selEnd {
						// Single line: extract column range.
						cs := min(colStart, colEnd)
						ce := max(colStart, colEnd) + 1
						if cs > len(runes) {
							cs = len(runes)
						}
						if ce > len(runes) {
							ce = len(runes)
						}
						parts = append(parts, string(runes[cs:ce]))
					} else if i == selStart {
						cs := colStart
						if cs > len(runes) {
							cs = len(runes)
						}
						parts = append(parts, string(runes[cs:]))
					} else if i == selEnd {
						ce := colEnd + 1
						if ce > len(runes) {
							ce = len(runes)
						}
						parts = append(parts, string(runes[:ce]))
					} else {
						parts = append(parts, line)
					}
				}
				clipText = strings.Join(parts, "\n")
			case 'B': // Block mode: rectangular column range.
				colStart := min(m.yamlVisualCol, m.yamlVisualCurCol)
				colEnd := max(m.yamlVisualCol, m.yamlVisualCurCol) + 1
				var parts []string
				for i := selStart; i <= selEnd; i++ {
					if i >= len(mapping) || mapping[i] < 0 || mapping[i] >= len(origLines) {
						continue
					}
					line := origLines[mapping[i]]
					runes := []rune(line)
					cs := colStart
					ce := colEnd
					if cs > len(runes) {
						cs = len(runes)
					}
					if ce > len(runes) {
						ce = len(runes)
					}
					parts = append(parts, string(runes[cs:ce]))
				}
				clipText = strings.Join(parts, "\n")
			default: // Line mode: whole lines.
				var selected []string
				for i := selStart; i <= selEnd; i++ {
					if i < len(mapping) && mapping[i] >= 0 && mapping[i] < len(origLines) {
						selected = append(selected, origLines[mapping[i]])
					}
				}
				clipText = strings.Join(selected, "\n")
			}
			lineCount := selEnd - selStart + 1
			m.yamlVisualMode = false
			m.setStatusMessage(fmt.Sprintf("Copied %d lines", lineCount), false)
			return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
		case "h", "left":
			// Move cursor column left (for char and block modes).
			if m.yamlVisualType == 'v' || m.yamlVisualType == 'B' {
				if m.yamlVisualCurCol > 0 {
					m.yamlVisualCurCol--
				}
			}
			return m, nil
		case "l", "right":
			// Move cursor column right (for char and block modes).
			if m.yamlVisualType == 'v' || m.yamlVisualType == 'B' {
				m.yamlVisualCurCol++
			}
			return m, nil
		case "j", "down":
			if m.yamlCursor < totalVisible-1 {
				m.yamlCursor++
			}
			m.ensureYAMLCursorVisible()
			return m, nil
		case "k", "up":
			if m.yamlCursor > 0 {
				m.yamlCursor--
			}
			m.ensureYAMLCursorVisible()
			return m, nil
		case "g":
			if m.pendingG {
				m.pendingG = false
				m.yamlCursor = 0
				m.yamlScroll = 0
				return m, nil
			}
			m.pendingG = true
			return m, nil
		case "G":
			m.yamlCursor = totalVisible - 1
			if m.yamlCursor < 0 {
				m.yamlCursor = 0
			}
			m.yamlScroll = maxScroll
			return m, nil
		case "ctrl+d":
			m.yamlCursor += m.height / 2
			if m.yamlCursor >= totalVisible {
				m.yamlCursor = totalVisible - 1
			}
			m.ensureYAMLCursorVisible()
			return m, nil
		case "ctrl+u":
			m.yamlCursor -= m.height / 2
			if m.yamlCursor < 0 {
				m.yamlCursor = 0
			}
			m.ensureYAMLCursorVisible()
			return m, nil
		case "ctrl+c":
			m.yamlVisualMode = false
			m.mode = modeExplorer
			m.yamlScroll = 0
			m.yamlCursor = 0
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "?":
		m.helpPreviousMode = modeYAML
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "YAML View"
		return m, nil
	case "V":
		// Enter visual line selection mode.
		m.yamlVisualMode = true
		m.yamlVisualType = 'V'
		m.yamlVisualStart = m.yamlCursor
		m.yamlVisualCol = m.yamlVisualCurCol
		return m, nil
	case "v":
		// Enter character visual selection mode; anchor at current cursor column.
		m.yamlVisualMode = true
		m.yamlVisualType = 'v'
		m.yamlVisualStart = m.yamlCursor
		m.yamlVisualCol = m.yamlVisualCurCol
		return m, nil
	case "ctrl+v":
		// Enter block visual selection mode; anchor at current cursor column.
		m.yamlVisualMode = true
		m.yamlVisualType = 'B'
		m.yamlVisualStart = m.yamlCursor
		m.yamlVisualCol = m.yamlVisualCurCol
		return m, nil
	case "q", "esc":
		if m.yamlSearchText.Value != "" {
			// Clear search first.
			m.yamlSearchText.Clear()
			m.yamlMatchLines = nil
			m.yamlMatchIdx = 0
			return m, nil
		}
		m.mode = modeExplorer
		m.yamlScroll = 0
		m.yamlCursor = 0
		return m, nil
	case "ctrl+c":
		m.mode = modeExplorer
		m.yamlScroll = 0
		m.yamlCursor = 0
		m.yamlSearchText.Clear()
		m.yamlMatchLines = nil
		return m, nil
	case "/":
		m.yamlSearchMode = true
		m.yamlSearchText.Clear()
		m.yamlMatchLines = nil
		m.yamlMatchIdx = 0
		return m, nil
	case "n":
		// Next match.
		if len(m.yamlMatchLines) > 0 {
			m.yamlMatchIdx = (m.yamlMatchIdx + 1) % len(m.yamlMatchLines)
			m.yamlScrollToMatchFolded(viewportLines)
		}
		return m, nil
	case "N":
		// Previous match.
		if len(m.yamlMatchLines) > 0 {
			m.yamlMatchIdx--
			if m.yamlMatchIdx < 0 {
				m.yamlMatchIdx = len(m.yamlMatchLines) - 1
			}
			m.yamlScrollToMatchFolded(viewportLines)
		}
		return m, nil
	case "E":
		// Edit the resource in $EDITOR via kubectl edit.
		kind := m.selectedResourceKind()
		sel := m.selectedMiddleItem()
		if kind != "" && sel != nil {
			m.actionCtx = m.buildActionCtx(sel, kind)
			return m, m.execKubectlEdit()
		}
		return m, nil
	case "tab", "z":
		// Toggle fold on the section at the cursor position.
		_, mapping := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
		sec := sectionAtScrollPos(m.yamlCursor, mapping, m.yamlSections)
		if sec != "" {
			if m.yamlCollapsed == nil {
				m.yamlCollapsed = make(map[string]bool)
			}
			m.yamlCollapsed[sec] = !m.yamlCollapsed[sec]
			m.clampYAMLScroll()
		}
		return m, nil
	case "Z":
		// Toggle all folds: if any section is expanded, collapse all; otherwise expand all.
		if m.yamlCollapsed == nil {
			m.yamlCollapsed = make(map[string]bool)
		}
		anyExpanded := false
		for _, sec := range m.yamlSections {
			if isMultiLineSection(sec) && !m.yamlCollapsed[sec.key] {
				anyExpanded = true
				break
			}
		}
		if anyExpanded {
			for _, sec := range m.yamlSections {
				if isMultiLineSection(sec) {
					m.yamlCollapsed[sec.key] = true
				}
			}
		} else {
			m.yamlCollapsed = make(map[string]bool)
		}
		m.clampYAMLScroll()
		return m, nil
	case "h", "left":
		// Move cursor column left.
		if m.yamlVisualCurCol > 0 {
			m.yamlVisualCurCol--
		}
		return m, nil
	case "l", "right":
		// Move cursor column right.
		m.yamlVisualCurCol++
		return m, nil
	case "0":
		// Move cursor to beginning of line.
		m.yamlVisualCurCol = 0
		return m, nil
	case "$":
		// Move cursor to end of current line.
		yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
		visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
		if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
			lineLen := len([]rune(visLines[m.yamlCursor]))
			if lineLen > 0 {
				m.yamlVisualCurCol = lineLen - 1
			}
		}
		return m, nil
	case "w":
		// Move cursor to next word start.
		yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
		visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
		if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
			m.yamlVisualCurCol = nextWordStart(visLines[m.yamlCursor], m.yamlVisualCurCol)
		}
		return m, nil
	case "b":
		// Move cursor to previous word start.
		yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
		visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
		if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
			m.yamlVisualCurCol = prevWordStart(visLines[m.yamlCursor], m.yamlVisualCurCol)
		}
		return m, nil
	case "j", "down":
		if m.yamlCursor < totalVisible-1 {
			m.yamlCursor++
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "k", "up":
		if m.yamlCursor > 0 {
			m.yamlCursor--
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.yamlCursor = 0
			m.yamlScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		m.yamlCursor = totalVisible - 1
		if m.yamlCursor < 0 {
			m.yamlCursor = 0
		}
		m.yamlScroll = maxScroll
		return m, nil
	case "ctrl+d":
		m.yamlCursor += m.height / 2
		if m.yamlCursor >= totalVisible {
			m.yamlCursor = totalVisible - 1
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+u":
		m.yamlCursor -= m.height / 2
		if m.yamlCursor < 0 {
			m.yamlCursor = 0
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+f":
		m.yamlCursor += m.height
		if m.yamlCursor >= totalVisible {
			m.yamlCursor = totalVisible - 1
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+b":
		m.yamlCursor -= m.height
		if m.yamlCursor < 0 {
			m.yamlCursor = 0
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	}
	return m, nil
}

// ensureYAMLCursorVisible adjusts yamlScroll so the cursor is within the viewport.
func (m *Model) ensureYAMLCursorVisible() {
	maxLines := m.height - 4
	if maxLines < 3 {
		maxLines = 3
	}
	if m.yamlCursor < m.yamlScroll {
		m.yamlScroll = m.yamlCursor
	}
	if m.yamlCursor >= m.yamlScroll+maxLines {
		m.yamlScroll = m.yamlCursor - maxLines + 1
	}
}

// clampYAMLScroll ensures yamlScroll stays within bounds after fold changes.
func (m *Model) clampYAMLScroll() {
	totalVisible := visibleLineCount(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	viewportLines := m.height - 4
	if viewportLines < 3 {
		viewportLines = 3
	}
	maxScroll := totalVisible - viewportLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.yamlScroll > maxScroll {
		m.yamlScroll = maxScroll
	}
	if m.yamlScroll < 0 {
		m.yamlScroll = 0
	}
}

// yamlScrollToMatchFolded scrolls to show the current search match, expanding
// the containing section if it is collapsed, and using visible-line coordinates.
func (m *Model) yamlScrollToMatchFolded(viewportLines int) {
	if m.yamlMatchIdx < 0 || m.yamlMatchIdx >= len(m.yamlMatchLines) {
		return
	}
	targetOrig := m.yamlMatchLines[m.yamlMatchIdx]

	// If the match is inside a collapsed section, expand it.
	for _, sec := range m.yamlSections {
		if m.yamlCollapsed[sec.key] && targetOrig > sec.startLine && targetOrig <= sec.endLine {
			m.yamlCollapsed[sec.key] = false
		}
	}

	// Convert original line to visible line.
	_, mapping := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	visIdx := originalToVisible(targetOrig, mapping)
	if visIdx < 0 {
		return
	}

	totalVisible := len(mapping)
	maxScroll := totalVisible - viewportLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	// Move cursor to the match and center it in the viewport.
	m.yamlCursor = visIdx
	m.yamlScroll = visIdx - viewportLines/2
	if m.yamlScroll > maxScroll {
		m.yamlScroll = maxScroll
	}
	if m.yamlScroll < 0 {
		m.yamlScroll = 0
	}
}

// updateYAMLSearchMatches finds all lines matching the current search text.
func (m *Model) updateYAMLSearchMatches() {
	m.yamlMatchLines = nil
	if m.yamlSearchText.Value == "" {
		return
	}
	query := strings.ToLower(m.yamlSearchText.Value)
	for i, line := range strings.Split(m.yamlContent, "\n") {
		if strings.Contains(strings.ToLower(line), query) {
			m.yamlMatchLines = append(m.yamlMatchLines, i)
		}
	}
}

// findYAMLMatchFromCursor returns the index of the first match at or after the
// current cursor position. Wraps to 0 if no match is found after the cursor.
func (m *Model) findYAMLMatchFromCursor() int {
	_, mapping := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	origLine := 0
	if m.yamlCursor >= 0 && m.yamlCursor < len(mapping) {
		origLine = mapping[m.yamlCursor]
	}
	for i, matchLine := range m.yamlMatchLines {
		if matchLine >= origLine {
			return i
		}
	}
	return 0
}

func (m Model) handleDescribeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalLines := countLines(m.describeContent)
	visibleLines := m.height - 4
	if visibleLines < 3 {
		visibleLines = 3
	}
	maxScroll := totalLines - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch msg.String() {
	case "?":
		m.helpPreviousMode = modeDescribe
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "Navigation"
		return m, nil
	case "q", "esc":
		m.mode = modeExplorer
		m.describeScroll = 0
		return m, nil
	case "j", "down":
		m.describeScroll++
		if m.describeScroll > maxScroll {
			m.describeScroll = maxScroll
		}
		return m, nil
	case "k", "up":
		if m.describeScroll > 0 {
			m.describeScroll--
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.describeScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		m.describeScroll = maxScroll
		return m, nil
	case "ctrl+d":
		m.describeScroll += m.height / 2
		if m.describeScroll > maxScroll {
			m.describeScroll = maxScroll
		}
		return m, nil
	case "ctrl+u":
		m.describeScroll -= m.height / 2
		if m.describeScroll < 0 {
			m.describeScroll = 0
		}
		return m, nil
	case "ctrl+f":
		m.describeScroll += m.height
		if m.describeScroll > maxScroll {
			m.describeScroll = maxScroll
		}
		return m, nil
	case "ctrl+b":
		m.describeScroll -= m.height
		if m.describeScroll < 0 {
			m.describeScroll = 0
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleDiffKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalLines := ui.DiffViewTotalLines(m.diffLeft, m.diffRight)
	if m.diffUnified {
		totalLines = ui.UnifiedDiffViewTotalLines(m.diffLeft, m.diffRight)
	}
	visibleLines := m.height - 4
	if visibleLines < 3 {
		visibleLines = 3
	}
	maxScroll := totalLines - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch msg.String() {
	case "?":
		m.helpPreviousMode = modeDiff
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "Diff View"
		return m, nil
	case "q", "esc":
		m.mode = modeExplorer
		m.diffScroll = 0
		m.diffLineInput = ""
		return m, nil
	case "j", "down":
		m.diffLineInput = ""
		m.diffScroll++
		if m.diffScroll > maxScroll {
			m.diffScroll = maxScroll
		}
		return m, nil
	case "k", "up":
		m.diffLineInput = ""
		if m.diffScroll > 0 {
			m.diffScroll--
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.diffLineInput = ""
			m.diffScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		if m.diffLineInput != "" {
			lineNum, _ := strconv.Atoi(m.diffLineInput)
			m.diffLineInput = ""
			if lineNum > 0 {
				lineNum-- // 0-indexed
			}
			m.diffScroll = min(lineNum, maxScroll)
		} else {
			m.diffScroll = maxScroll
		}
		return m, nil
	case "ctrl+d":
		m.diffLineInput = ""
		m.diffScroll += m.height / 2
		if m.diffScroll > maxScroll {
			m.diffScroll = maxScroll
		}
		return m, nil
	case "ctrl+u":
		m.diffLineInput = ""
		m.diffScroll -= m.height / 2
		if m.diffScroll < 0 {
			m.diffScroll = 0
		}
		return m, nil
	case "ctrl+f":
		m.diffLineInput = ""
		m.diffScroll += m.height
		if m.diffScroll > maxScroll {
			m.diffScroll = maxScroll
		}
		return m, nil
	case "ctrl+b":
		m.diffLineInput = ""
		m.diffScroll -= m.height
		if m.diffScroll < 0 {
			m.diffScroll = 0
		}
		return m, nil
	case "u":
		m.diffLineInput = ""
		m.diffUnified = !m.diffUnified
		m.diffScroll = 0
		return m, nil
	case "#":
		m.diffLineInput = ""
		m.diffLineNumbers = !m.diffLineNumbers
		return m, nil
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.diffLineInput += msg.String()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		m.diffLineInput = ""
	}
	return m, nil
}

func (m Model) handleLogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle log search input mode.
	if m.logSearchActive {
		return m.handleLogSearchKey(msg)
	}

	// Handle visual select mode keys.
	if m.logVisualMode {
		return m.handleLogVisualKey(msg)
	}

	switch msg.String() {
	case "?":
		m.helpPreviousMode = modeLogs
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "Log Viewer"
		return m, nil
	case "q", "esc":
		if m.logCancel != nil {
			m.logCancel()
			m.logCancel = nil
		}
		if m.logHistoryCancel != nil {
			m.logHistoryCancel()
			m.logHistoryCancel = nil
		}
		m.logCh = nil
		m.mode = modeExplorer
		m.logLineInput = ""
		m.logSearchQuery = ""
		m.logSearchInput.Clear()
		m.logParentKind = ""
		m.logParentName = ""
		m.logVisualMode = false
		return m, nil
	case "j", "down":
		m.logFollow = false
		m.logLineInput = ""
		if m.logCursor < len(m.logLines)-1 {
			m.logCursor++
		}
		m.ensureLogCursorVisible()
		return m, nil
	case "k", "up":
		m.logFollow = false
		m.logLineInput = ""
		if m.logCursor > 0 {
			m.logCursor--
		}
		m.ensureLogCursorVisible()
		cmd := m.maybeLoadMoreHistory()
		return m, cmd
	case "ctrl+d":
		m.logFollow = false
		m.logLineInput = ""
		m.logCursor += m.logContentHeight() / 2
		if m.logCursor >= len(m.logLines) {
			m.logCursor = len(m.logLines) - 1
		}
		m.ensureLogCursorVisible()
		return m, nil
	case "ctrl+u":
		m.logFollow = false
		m.logLineInput = ""
		m.logCursor -= m.logContentHeight() / 2
		if m.logCursor < 0 {
			m.logCursor = 0
		}
		m.ensureLogCursorVisible()
		cmd := m.maybeLoadMoreHistory()
		return m, cmd
	case "ctrl+f":
		m.logFollow = false
		m.logLineInput = ""
		m.logCursor += m.logContentHeight()
		if m.logCursor >= len(m.logLines) {
			m.logCursor = len(m.logLines) - 1
		}
		m.ensureLogCursorVisible()
		return m, nil
	case "ctrl+b":
		m.logFollow = false
		m.logLineInput = ""
		m.logCursor -= m.logContentHeight()
		if m.logCursor < 0 {
			m.logCursor = 0
		}
		m.ensureLogCursorVisible()
		cmd := m.maybeLoadMoreHistory()
		return m, cmd
	case "G":
		if m.logLineInput != "" {
			lineNum, _ := strconv.Atoi(m.logLineInput)
			m.logLineInput = ""
			if lineNum > 0 {
				lineNum-- // 0-indexed
			}
			m.logCursor = min(lineNum, len(m.logLines)-1)
			m.logFollow = false
		} else {
			m.logCursor = len(m.logLines) - 1
			m.logFollow = true
		}
		m.ensureLogCursorVisible()
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.logFollow = false
			m.logLineInput = ""
			m.logCursor = 0
			m.ensureLogCursorVisible()
			cmd := m.maybeLoadMoreHistory()
			return m, cmd
		}
		m.pendingG = true
		return m, nil
	case "h", "left":
		// Move cursor column left.
		m.logLineInput = ""
		if m.logVisualCurCol > 0 {
			m.logVisualCurCol--
		}
		return m, nil
	case "l", "right":
		// Move cursor column right.
		m.logLineInput = ""
		m.logVisualCurCol++
		return m, nil
	case "$":
		// Move cursor to end of current line.
		m.logLineInput = ""
		if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
			lineLen := len([]rune(m.logLines[m.logCursor]))
			if lineLen > 0 {
				m.logVisualCurCol = lineLen - 1
			}
		}
		return m, nil
	case "e":
		// Move cursor to end of current/next word.
		m.logLineInput = ""
		if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
			m.logVisualCurCol = wordEnd(m.logLines[m.logCursor], m.logVisualCurCol)
		}
		return m, nil
	case "b":
		// Move cursor to previous word start.
		m.logLineInput = ""
		if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
			m.logVisualCurCol = prevWordStart(m.logLines[m.logCursor], m.logVisualCurCol)
		}
		return m, nil
	case "V":
		m.logLineInput = ""
		if m.logCursor < 0 {
			m.logCursor = m.logScroll
		}
		m.logVisualMode = true
		m.logVisualType = 'V'
		m.logVisualStart = m.logCursor
		m.logVisualCol = m.logVisualCurCol
		return m, nil
	case "v":
		m.logLineInput = ""
		if m.logCursor < 0 {
			m.logCursor = m.logScroll
		}
		m.logVisualMode = true
		m.logVisualType = 'v'
		m.logVisualStart = m.logCursor
		m.logVisualCol = m.logVisualCurCol
		return m, nil
	case "ctrl+v":
		m.logLineInput = ""
		if m.logCursor < 0 {
			m.logCursor = m.logScroll
		}
		m.logVisualMode = true
		m.logVisualType = 'B'
		m.logVisualStart = m.logCursor
		m.logVisualCol = m.logVisualCurCol
		return m, nil
	case "f":
		m.logLineInput = ""
		m.logFollow = !m.logFollow
		if m.logFollow {
			m.logCursor = len(m.logLines) - 1
			m.logScroll = m.logMaxScroll()
		}
		return m, nil
	case "w":
		m.logLineInput = ""
		m.logWrap = !m.logWrap
		m.clampLogScroll()
		return m, nil
	case "/":
		m.logLineInput = ""
		m.logSearchActive = true
		m.logSearchInput.Clear()
		return m, nil
	case "n":
		m.logLineInput = ""
		m.findNextLogMatch(true)
		return m, nil
	case "N", "p":
		m.logLineInput = ""
		m.findNextLogMatch(false)
		return m, nil
	case "#":
		m.logLineInput = ""
		m.logLineNumbers = !m.logLineNumbers
		return m, nil
	case "s":
		// Toggle timestamp visibility (no stream restart — timestamps are always streamed).
		m.logLineInput = ""
		m.logTimestamps = !m.logTimestamps
		return m, nil
	case "W":
		// Save loaded logs to file.
		m.logLineInput = ""
		path, err := m.saveLoadedLogs()
		if err != nil {
			m.setErrorFromErr("Log save failed: ", err)
			return m, scheduleStatusClear()
		}
		m.setStatusMessage("Loaded logs saved to "+path, false)
		return m, scheduleStatusClear()
	case "ctrl+s":
		// Save all logs (full kubectl logs without --tail) to file.
		m.logLineInput = ""
		m.setStatusMessage("Saving all logs...", false)
		return m, m.saveAllLogs()
	case "c":
		m.logLineInput = ""
		m.logPrevious = !m.logPrevious
		// --previous is incompatible with -f (follow).
		if m.logPrevious {
			m.logFollow = false
		}
		// Restart the log stream.
		if m.logCancel != nil {
			m.logCancel()
		}
		if m.logHistoryCancel != nil {
			m.logHistoryCancel()
			m.logHistoryCancel = nil
		}
		m.logLines = nil
		m.logScroll = 0
		m.logCursor = 0
		m.logVisualMode = false
		m.logTailLines = ui.ConfigLogTailLines
		m.logHasMoreHistory = !m.logPrevious && !m.logIsMulti
		m.logLoadingHistory = false
		if m.logIsMulti && len(m.logMultiItems) > 0 {
			var cmd tea.Cmd
			m, cmd = m.restartMultiLogStream()
			return m, cmd
		}
		return m, m.startLogStream()
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.logLineInput += msg.String()
		return m, nil
	case "\\":
		// Unified selector: container filter first (if on a Pod), pod switch second.
		m.logLineInput = ""
		if m.actionCtx.kind == "Pod" {
			// Show container selector overlay for filtering which containers' logs are shown.
			m.overlay = overlayLogContainerSelect
			m.overlayCursor = 0
			m.logContainerFilterText = ""
			m.logContainerFilterActive = false
			m.logContainerSelectionModified = false
			ui.ResetOverlayContainerScroll()
			return m, m.loadContainersForLogFilter()
		}
		if m.logParentKind != "" {
			// Switch pod: show inline pod selector overlay without leaving log mode.
			m.logSavedPodName = m.actionCtx.name
			if m.logCancel != nil {
				m.logCancel()
				m.logCancel = nil
			}
			if m.logHistoryCancel != nil {
				m.logHistoryCancel()
				m.logHistoryCancel = nil
			}
			m.logCh = nil
			m.actionCtx.kind = m.logParentKind
			m.actionCtx.name = m.logParentName
			m.actionCtx.containerName = ""
			m.pendingAction = "Logs"
			m.loading = true
			m.setStatusMessage("Loading pods...", false)
			return m, m.loadPodsForLogAction()
		}
		return m, nil
	case "ctrl+c":
		if m.logCancel != nil {
			m.logCancel()
		}
		if m.logHistoryCancel != nil {
			m.logHistoryCancel()
			m.logHistoryCancel = nil
		}
		return m.closeTabOrQuit()
	default:
		m.logLineInput = ""
	}
	return m, nil
}

func (m Model) handleLogVisualKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.logVisualMode = false
		return m, nil
	case "V":
		// Toggle: if already in line mode, cancel; otherwise switch to line mode.
		if m.logVisualType == 'V' {
			m.logVisualMode = false
		} else {
			m.logVisualType = 'V'
		}
		return m, nil
	case "v":
		// Toggle: if already in char mode, cancel; otherwise switch to char mode.
		if m.logVisualType == 'v' {
			m.logVisualMode = false
		} else {
			m.logVisualType = 'v'
		}
		return m, nil
	case "ctrl+v":
		// Toggle: if already in block mode, cancel; otherwise switch to block mode.
		if m.logVisualType == 'B' {
			m.logVisualMode = false
		} else {
			m.logVisualType = 'B'
		}
		return m, nil
	case "y":
		// Copy selected content to clipboard.
		selStart := min(m.logVisualStart, m.logCursor)
		selEnd := max(m.logVisualStart, m.logCursor)
		if selStart < 0 {
			selStart = 0
		}
		if selEnd >= len(m.logLines) {
			selEnd = len(m.logLines) - 1
		}
		var clipText string
		switch m.logVisualType {
		case 'v': // Character mode: partial first/last lines.
			var parts []string
			colStart := m.logVisualCol
			colEnd := m.logVisualCurCol
			for i := selStart; i <= selEnd; i++ {
				line := m.logLines[i]
				runes := []rune(line)
				if selStart == selEnd {
					// Single line: extract column range.
					cs := min(colStart, colEnd)
					ce := max(colStart, colEnd) + 1
					if cs > len(runes) {
						cs = len(runes)
					}
					if ce > len(runes) {
						ce = len(runes)
					}
					parts = append(parts, string(runes[cs:ce]))
				} else if i == selStart {
					cs := colStart
					if cs > len(runes) {
						cs = len(runes)
					}
					parts = append(parts, string(runes[cs:]))
				} else if i == selEnd {
					ce := colEnd + 1
					if ce > len(runes) {
						ce = len(runes)
					}
					parts = append(parts, string(runes[:ce]))
				} else {
					parts = append(parts, line)
				}
			}
			clipText = strings.Join(parts, "\n")
		case 'B': // Block mode: rectangular column range.
			colStart := min(m.logVisualCol, m.logVisualCurCol)
			colEnd := max(m.logVisualCol, m.logVisualCurCol) + 1
			var parts []string
			for i := selStart; i <= selEnd; i++ {
				line := m.logLines[i]
				runes := []rune(line)
				cs := colStart
				ce := colEnd
				if cs > len(runes) {
					cs = len(runes)
				}
				if ce > len(runes) {
					ce = len(runes)
				}
				parts = append(parts, string(runes[cs:ce]))
			}
			clipText = strings.Join(parts, "\n")
		default: // Line mode: whole lines.
			var selected []string
			for i := selStart; i <= selEnd; i++ {
				selected = append(selected, m.logLines[i])
			}
			clipText = strings.Join(selected, "\n")
		}
		lineCount := selEnd - selStart + 1
		m.logVisualMode = false
		m.setStatusMessage(fmt.Sprintf("Copied %d lines", lineCount), false)
		return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
	case "h", "left":
		// Move cursor column left (for char and block modes).
		if m.logVisualType == 'v' || m.logVisualType == 'B' {
			if m.logVisualCurCol > 0 {
				m.logVisualCurCol--
			}
		}
		return m, nil
	case "l", "right":
		// Move cursor column right (for char and block modes).
		if m.logVisualType == 'v' || m.logVisualType == 'B' {
			m.logVisualCurCol++
		}
		return m, nil
	case "j", "down":
		if m.logCursor < len(m.logLines)-1 {
			m.logCursor++
		}
		m.ensureLogCursorVisible()
		return m, nil
	case "k", "up":
		if m.logCursor > 0 {
			m.logCursor--
		}
		m.ensureLogCursorVisible()
		cmd := m.maybeLoadMoreHistory()
		return m, cmd
	case "G":
		m.logCursor = len(m.logLines) - 1
		m.ensureLogCursorVisible()
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.logCursor = 0
			m.ensureLogCursorVisible()
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "ctrl+d":
		m.logCursor += m.logContentHeight() / 2
		if m.logCursor >= len(m.logLines) {
			m.logCursor = len(m.logLines) - 1
		}
		m.ensureLogCursorVisible()
		return m, nil
	case "ctrl+u":
		m.logCursor -= m.logContentHeight() / 2
		if m.logCursor < 0 {
			m.logCursor = 0
		}
		m.ensureLogCursorVisible()
		return m, nil
	case "ctrl+c":
		m.logVisualMode = false
		return m.closeTabOrQuit()
	case "q":
		m.logVisualMode = false
		return m, nil
	}
	return m, nil
}

func (m Model) handleLogSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.logSearchActive = false
		m.logSearchQuery = m.logSearchInput.Value
		m.findNextLogMatch(true)
	case "esc":
		m.logSearchActive = false
		m.logSearchInput.Clear()
	case "backspace":
		if len(m.logSearchInput.Value) > 0 {
			m.logSearchInput.Backspace()
		}
	case "ctrl+w":
		m.logSearchInput.DeleteWord()
	case "ctrl+a":
		m.logSearchInput.Home()
	case "ctrl+e":
		m.logSearchInput.End()
	case "left":
		m.logSearchInput.Left()
	case "right":
		m.logSearchInput.Right()
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.logSearchInput.Insert(key)
		}
	}
	return m, nil
}

func (m *Model) findNextLogMatch(forward bool) {
	if m.logSearchQuery == "" {
		return
	}
	query := strings.ToLower(m.logSearchQuery)
	start := m.logCursor
	if start < 0 {
		start = m.logScroll
	}
	if forward {
		for i := start + 1; i < len(m.logLines); i++ {
			if strings.Contains(strings.ToLower(m.logLines[i]), query) {
				m.logCursor = i
				m.logFollow = false
				m.ensureLogCursorVisible()
				return
			}
		}
		// Wrap around.
		for i := 0; i <= start; i++ {
			if strings.Contains(strings.ToLower(m.logLines[i]), query) {
				m.logCursor = i
				m.logFollow = false
				m.ensureLogCursorVisible()
				return
			}
		}
	} else {
		for i := start - 1; i >= 0; i-- {
			if strings.Contains(strings.ToLower(m.logLines[i]), query) {
				m.logCursor = i
				m.logFollow = false
				m.ensureLogCursorVisible()
				return
			}
		}
		// Wrap around.
		for i := len(m.logLines) - 1; i >= start; i-- {
			if strings.Contains(strings.ToLower(m.logLines[i]), query) {
				m.logCursor = i
				m.logFollow = false
				m.ensureLogCursorVisible()
				return
			}
		}
	}
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle mouse scroll in log viewer mode.
	if m.mode == modeLogs {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.logFollow = false
			if m.logScroll > 0 {
				m.logScroll -= 3
				if m.logScroll < 0 {
					m.logScroll = 0
				}
			}
			cmd := m.maybeLoadMoreHistory()
			return m, cmd
		case tea.MouseButtonWheelDown:
			m.logFollow = false
			m.logScroll += 3
			m.clampLogScroll()
		}
		return m, nil
	}

	// Don't handle mouse in overlay/help/yaml modes.
	if m.overlay != overlayNone || m.mode != modeExplorer {
		return m, nil
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.moveCursor(-3)
	case tea.MouseButtonWheelDown:
		return m.moveCursor(3)
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		return m.handleMouseClick(msg.X, msg.Y)
	}
	return m, nil
}

func (m Model) handleMouseClick(x, y int) (tea.Model, tea.Cmd) {
	// Calculate column boundaries (must match viewExplorer).
	var leftEnd, middleEnd int
	if m.fullscreenMiddle || m.fullscreenDashboard {
		// Fullscreen: only middle column exists.
		leftEnd = 0
		middleEnd = m.width
	} else {
		usable := m.width - 6
		leftW := max(10, usable*12/100)
		middleW := max(10, usable*51/100)
		// Each column has border(2) + padding(2) = 4 extra chars width.
		leftEnd = leftW + 4
		middleEnd = leftEnd + middleW + 4
	}

	switch {
	case x < leftEnd:
		// Left column click: navigate parent.
		return m.navigateParent()
	case x < middleEnd:
		// Middle column click: select item.
		// y offset depends on whether column has a header line.
		// Title bar (1) + top border (1) = 2 base offset.
		baseOffset := 2 // title bar (1) + top border (1)
		if len(m.tabs) > 1 {
			baseOffset = 3 // title bar (1) + tab bar (1) + top border (1)
		}
		itemY := y - baseOffset
		switch m.nav.Level {
		case model.LevelResources, model.LevelOwned, model.LevelContainers:
			// Table view has a header row for column names.
			itemY--
			if itemY >= 0 {
				visible := m.visibleMiddleItems()
				contentHeight := m.height - 4
				if contentHeight < 3 {
					contentHeight = 3
				}
				tableHeight := contentHeight - 1 // minus table header
				startIdx := 0
				if m.cursor() >= tableHeight {
					startIdx = m.cursor() - tableHeight + 1
				}
				targetIdx := startIdx + itemY
				if targetIdx >= 0 && targetIdx < len(visible) {
					m.setCursor(targetIdx)
					return m, m.loadPreview()
				}
			} else {
				// Header row click — sort by the clicked column.
				relX := x - 2 // border + padding
				if !m.fullscreenMiddle && !m.fullscreenDashboard {
					relX = x - leftEnd - 2
				}
				return m.handleHeaderClick(relX)
			}
		default:
			// Column view has a header line (rendered by RenderColumn).
			itemY--
			if itemY >= 0 {
				visible := m.visibleMiddleItems()
				targetIdx := m.itemIndexFromDisplayLine(itemY)
				if targetIdx >= 0 && targetIdx < len(visible) {
					m.setCursor(targetIdx)
					m.syncExpandedGroup()
					return m, m.loadPreview()
				}
			}
		}
		return m, nil
	default:
		// Right column click: navigate child.
		return m.navigateChild()
	}
}

// handleHeaderClick sorts the table by the column that was clicked in the header row.
// relX is the click position relative to the start of the middle column content area.
func (m Model) handleHeaderClick(relX int) (tea.Model, tea.Cmd) {
	items := m.visibleMiddleItems()
	if len(items) == 0 {
		return m, nil
	}

	// Replicate column width calculation from RenderTable.
	// The table receives middleInner = middleW - colPad as its width.
	usable := m.width - 6
	middleW := max(10, usable*51/100)
	if m.fullscreenMiddle {
		middleW = m.width - 2
	}
	width := middleW - 2 // subtract colPad to match middleInner passed to RenderTable

	// Detect which detail columns have data.
	hasNs, hasReady, hasRestarts, hasAge, hasStatus := false, false, false, false, false
	for _, item := range items {
		if item.Namespace != "" {
			hasNs = true
		}
		if item.Ready != "" {
			hasReady = true
		}
		if item.Restarts != "" {
			hasRestarts = true
		}
		if item.Age != "" {
			hasAge = true
		}
		if item.Status != "" {
			hasStatus = true
		}
	}

	// Calculate column widths (must match RenderTable logic).
	nsW, readyW, restartsW, ageW, statusW := 0, 0, 0, 0, 0
	if hasNs {
		nsW = len("NAMESPACE")
		for _, item := range items {
			if w := len(item.Namespace); w > nsW {
				nsW = w
			}
		}
		nsW++
		if nsW > 30 {
			nsW = 30
		}
	}
	if hasReady {
		readyW = len("READY")
		for _, item := range items {
			if w := len(item.Ready); w > readyW {
				readyW = w
			}
		}
		readyW++
	}
	if hasRestarts {
		restartsW = len("RS") + 1
		for _, item := range items {
			if w := len(item.Restarts); w >= restartsW {
				restartsW = w + 1
			}
		}
	}
	if hasAge {
		ageW = len("AGE") + 1
		for _, item := range items {
			if w := len(item.Age); w >= ageW {
				ageW = w + 1
			}
		}
		if ageW > 10 {
			ageW = 10
		}
	}
	if hasStatus {
		statusW = len("STATUS")
		for _, item := range items {
			if w := len(item.Status); w > statusW {
				statusW = w
			}
		}
		statusW++
		if statusW > 20 {
			statusW = 20
		}
	}

	markerColW := 2

	nameW := width - nsW - readyW - restartsW - ageW - statusW - markerColW
	if nameW < 10 {
		nameW = 10
	}

	// Determine which column was clicked based on cumulative position.
	// Column order: marker | namespace | NAME | READY | RS | STATUS | extra columns... | AGE
	pos := markerColW
	if hasNs {
		if relX < pos+nsW {
			// Clicked NAMESPACE — sort by name.
			return m.applySortMode(sortByName)
		}
		pos += nsW
	}
	pos += nameW
	if relX < pos {
		return m.applySortMode(sortByName)
	}
	if hasReady {
		pos += readyW
		if relX < pos {
			return m.applySortMode(sortByStatus)
		}
	}
	if hasRestarts {
		pos += restartsW
		if relX < pos {
			return m.applySortMode(sortByStatus)
		}
	}
	if hasStatus {
		pos += statusW
		if relX < pos {
			return m.applySortMode(sortByStatus)
		}
	}
	// Remaining space is AGE (or extra columns, mapped to age sort).
	return m.applySortMode(sortByAge)
}

// nextWordStart returns the column of the next word start (vim 'w' motion).
func nextWordStart(line string, col int) int {
	runes := []rune(line)
	n := len(runes)
	if n == 0 {
		return 0
	}
	if col >= n-1 {
		return n - 1
	}
	i := col
	// Skip current word characters.
	for i < n && !isWordBoundary(runes[i]) {
		i++
	}
	// Skip whitespace/punctuation.
	for i < n && isWordBoundary(runes[i]) {
		i++
	}
	if i >= n {
		return n - 1
	}
	return i
}

// wordEnd returns the column of the current/next word end (vim 'e' motion).
func wordEnd(line string, col int) int {
	runes := []rune(line)
	n := len(runes)
	if n == 0 {
		return 0
	}
	if col >= n-1 {
		return n - 1
	}
	i := col + 1
	// Skip whitespace/punctuation.
	for i < n && isWordBoundary(runes[i]) {
		i++
	}
	// Move to end of word.
	for i < n-1 && !isWordBoundary(runes[i+1]) {
		i++
	}
	if i >= n {
		return n - 1
	}
	return i
}

// prevWordStart returns the column of the previous word start (vim 'b' motion).
func prevWordStart(line string, col int) int {
	runes := []rune(line)
	n := len(runes)
	if n == 0 || col <= 0 {
		return 0
	}
	if col >= n {
		col = n
	}
	i := col - 1
	// Skip whitespace/punctuation.
	for i > 0 && isWordBoundary(runes[i]) {
		i--
	}
	// Move to start of word.
	for i > 0 && !isWordBoundary(runes[i-1]) {
		i--
	}
	return i
}

// isWordBoundary returns true if the rune is whitespace or punctuation (non-word character).
func isWordBoundary(r rune) bool {
	return r == ' ' || r == '\t' || r == '.' || r == ':' || r == ',' || r == ';' ||
		r == '/' || r == '-' || r == '_' || r == '"' || r == '\'' || r == '(' || r == ')' ||
		r == '[' || r == ']' || r == '{' || r == '}'
}

// countLines counts newline characters.
func countLines(s string) int {
	n := 0
	for _, c := range s {
		if c == '\n' {
			n++
		}
	}
	return n
}
