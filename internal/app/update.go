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
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

		cmds := []tea.Cmd{m.loadPreview()}
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
		}
		m.clampCursor()
		// For pod/node views, also trigger async metrics enrichment.
		var cmds []tea.Cmd
		cmds = append(cmds, m.loadPreview())
		if kind == "Pod" {
			cmds = append(cmds, m.loadPodMetricsForList())
		} else if kind == "Node" {
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
		m.middleItems = msg.items
		m.itemCache[m.navKey()] = m.middleItems
		m.clampCursor()
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
		return m, nil

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
		return m, nil

	case podLogSelectMsg:
		m.loading = false
		if msg.err != nil {
			m.setErrorFromErr("Error: ", msg.err)
			m.pendingAction = ""
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
			return m, scheduleStatusClear()
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
				os.Remove(msg.tmpFile)
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
			}
			return m, nil
		}
		m.logLines = append(m.logLines, msg.line)
		if m.logFollow {
			m.logScroll = m.logMaxScroll()
		}
		return m, m.waitForLogLine()
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	if m.mode != modeExplorer && m.mode != modeExec && !m.yamlSearchMode && !m.logSearchActive && !m.helpSearchActive {
		switch msg.String() {
		case "]":
			if len(m.tabs) > 1 {
				m.saveCurrentTab() // save mode + log state BEFORE switching
				next := (m.activeTab + 1) % len(m.tabs)
				m.loadTab(next)
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
				m.loadTab(prev)
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
				if len(m.tabs) >= 10 {
					m.setStatusMessage("Max 10 tabs", true)
					return m, scheduleStatusClear()
				}
				m.saveCurrentTab()
				newTab := m.cloneCurrentTab()
				// New tab always starts in explorer mode.
				newTab.mode = modeExplorer
				newTab.logLines = nil
				newTab.logCancel = nil
				newTab.logCh = nil
				m.tabs = append(m.tabs, newTab)
				m.activeTab = len(m.tabs) - 1
				m.loadTab(m.activeTab)
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

	// In YAML view mode.
	if m.mode == modeYAML {
		return m.handleYAMLKey(msg)
	}

	// Clear pending 'g' state if any other key is pressed (vim-style gg).
	if m.pendingG && msg.String() != "g" {
		m.pendingG = false
	}

	switch msg.String() {
	case "q":
		if m.portForwardMgr != nil {
			m.portForwardMgr.StopAll()
		}
		return m, tea.Quit

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
				m.loadTab(m.activeTab)
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

	case " ":
		// Toggle selection on current item and move cursor down.
		if m.nav.Level >= model.LevelResources {
			sel := m.selectedMiddleItem()
			if sel != nil {
				m.toggleSelection(*sel)
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

	case "b":
		// Open bookmarks overlay.
		m.overlay = overlayBookmarks
		m.overlayCursor = 0
		m.bookmarkFilter.Clear()
		return m, nil

	case "B":
		// Bookmark current location.
		return m.bookmarkCurrentLocation()

	case "?":
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

	case "m":
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
		// If standing on Overview or Monitoring, toggle fullscreen dashboard instead.
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
		return m.navigateToOwner(owner.kind, owner.name, owner.apiVersion)

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
		// Create new tab (clone current state, max 10).
		if len(m.tabs) >= 10 {
			m.setStatusMessage("Max 10 tabs", true)
			return m, scheduleStatusClear()
		}
		m.saveCurrentTab()
		m.tabs = append(m.tabs, m.cloneCurrentTab())
		m.activeTab = len(m.tabs) - 1
		m.setStatusMessage(fmt.Sprintf("Tab %d created", m.activeTab+1), false)
		return m, scheduleStatusClear()

	case "]":
		// Next tab.
		if len(m.tabs) <= 1 {
			return m, nil
		}
		m.saveCurrentTab()
		next := (m.activeTab + 1) % len(m.tabs)
		m.loadTab(next)
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
		m.loadTab(prev)
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
				m.yamlMatchIdx = 0
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

	switch msg.String() {
	case "?":
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "YAML View"
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
	case "e":
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

	// Center the match in the viewport.
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
	case "l":
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

	switch msg.String() {
	case "?":
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
		m.logCh = nil
		m.mode = modeExplorer
		m.logLineInput = ""
		m.logSearchQuery = ""
		m.logSearchInput.Clear()
		m.logParentKind = ""
		m.logParentName = ""
		return m, nil
	case "j", "down":
		m.logFollow = false
		m.logLineInput = ""
		m.logScroll++
		m.clampLogScroll()
		return m, nil
	case "k", "up":
		m.logFollow = false
		m.logLineInput = ""
		if m.logScroll > 0 {
			m.logScroll--
		}
		return m, nil
	case "ctrl+d":
		m.logFollow = false
		m.logLineInput = ""
		m.logScroll += m.logViewHeight() / 2
		m.clampLogScroll()
		return m, nil
	case "ctrl+u":
		m.logFollow = false
		m.logLineInput = ""
		m.logScroll -= m.logViewHeight() / 2
		if m.logScroll < 0 {
			m.logScroll = 0
		}
		return m, nil
	case "ctrl+f":
		m.logFollow = false
		m.logLineInput = ""
		m.logScroll += m.logViewHeight()
		m.clampLogScroll()
		return m, nil
	case "ctrl+b":
		m.logFollow = false
		m.logLineInput = ""
		m.logScroll -= m.logViewHeight()
		if m.logScroll < 0 {
			m.logScroll = 0
		}
		return m, nil
	case "G":
		if m.logLineInput != "" {
			lineNum, _ := strconv.Atoi(m.logLineInput)
			m.logLineInput = ""
			if lineNum > 0 {
				lineNum-- // 0-indexed
			}
			m.logScroll = min(lineNum, m.logMaxScroll())
			m.logFollow = false
		} else {
			m.logScroll = m.logMaxScroll()
			m.logFollow = true
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.logFollow = false
			m.logLineInput = ""
			m.logScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "f":
		m.logLineInput = ""
		m.logFollow = !m.logFollow
		if m.logFollow {
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
	case "l":
		m.logLineInput = ""
		m.logLineNumbers = !m.logLineNumbers
		return m, nil
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.logLineInput += msg.String()
		return m, nil
	case "P":
		// Switch pod: re-trigger pod selection for the parent resource.
		m.logLineInput = ""
		if m.logParentKind == "" {
			return m, nil
		}
		// Cancel current log stream.
		if m.logCancel != nil {
			m.logCancel()
			m.logCancel = nil
		}
		m.logCh = nil
		// Restore the parent resource context for pod selection.
		m.actionCtx.kind = m.logParentKind
		m.actionCtx.name = m.logParentName
		m.actionCtx.containerName = ""
		// Exit log mode so the pod selector overlay can render.
		m.mode = modeExplorer
		m.pendingAction = "Logs"
		m.loading = true
		m.setStatusMessage("Loading pods...", false)
		return m, m.loadPodsForLogAction()
	case "ctrl+c":
		if m.logCancel != nil {
			m.logCancel()
		}
		return m.closeTabOrQuit()
	default:
		m.logLineInput = ""
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
	start := m.logScroll
	if forward {
		for i := start + 1; i < len(m.logLines); i++ {
			if strings.Contains(strings.ToLower(m.logLines[i]), query) {
				m.logScroll = i
				m.logFollow = false
				return
			}
		}
		// Wrap around.
		for i := 0; i <= start; i++ {
			if strings.Contains(strings.ToLower(m.logLines[i]), query) {
				m.logScroll = i
				m.logFollow = false
				return
			}
		}
	} else {
		for i := start - 1; i >= 0; i-- {
			if strings.Contains(strings.ToLower(m.logLines[i]), query) {
				m.logScroll = i
				m.logFollow = false
				return
			}
		}
		// Wrap around.
		for i := len(m.logLines) - 1; i >= start; i-- {
			if strings.Contains(strings.ToLower(m.logLines[i]), query) {
				m.logScroll = i
				m.logFollow = false
				return
			}
		}
	}
}

func (m Model) handleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.overlay {
	case overlayNamespace:
		return m.handleNamespaceOverlayKey(msg)
	case overlayAction:
		return m.handleActionOverlayKey(msg)
	case overlayConfirm:
		return m.handleConfirmOverlayKey(msg)
	case overlayScaleInput:
		return m.handleScaleOverlayKey(msg)
	case overlayPortForward:
		return m.handlePortForwardOverlayKey(msg)
	case overlayContainerSelect:
		return m.handleContainerSelectOverlayKey(msg)
	case overlayPodSelect:
		return m.handlePodSelectOverlayKey(msg)
	case overlayBookmarks:
		return m.handleBookmarkOverlayKey(msg)
	case overlayTemplates:
		return m.handleTemplateOverlayKey(msg)
	case overlaySecretEditor:
		return m.handleSecretEditorKey(msg)
	case overlayConfigMapEditor:
		return m.handleConfigMapEditorKey(msg)
	case overlayRollback:
		return m.handleRollbackOverlayKey(msg)
	case overlayHelmRollback:
		return m.handleHelmRollbackOverlayKey(msg)
	case overlayLabelEditor:
		return m.handleLabelEditorKey(msg)
	case overlayColorscheme:
		return m.handleColorschemeOverlayKey(msg)
	case overlayFilterPreset:
		return m.handleFilterPresetOverlayKey(msg)
	case overlayRBAC:
		m.overlay = overlayNone
		return m, nil
	case overlayPodStartup:
		m.overlay = overlayNone
		return m, nil
	case overlayAlerts:
		return m.handleAlertsOverlayKey(msg)
	case overlayBatchLabel:
		return m.handleBatchLabelOverlayKey(msg)
	case overlayQuotaDashboard:
		if msg.String() == "esc" || msg.String() == "q" {
			m.overlay = overlayNone
		}
		return m, nil
	case overlayEventTimeline:
		return m.handleEventTimelineOverlayKey(msg)
	case overlayNetworkPolicy:
		return m.handleNetworkPolicyOverlayKey(msg)
	}
	return m, nil
}

// handleEventTimelineOverlayKey handles keyboard input for the event timeline overlay.
func (m Model) handleEventTimelineOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	// Calculate maximum scroll for clamping.
	overlayH := min(30, m.height-4)
	maxVisible := max(overlayH-6, 1) // reserve lines for header, footer, padding
	maxScroll := max(len(m.eventTimelineData)-maxVisible, 0)

	switch key {
	case "esc", "q":
		m.overlay = overlayNone
	case "j", "down":
		if m.eventTimelineScroll < maxScroll {
			m.eventTimelineScroll++
		}
	case "k", "up":
		if m.eventTimelineScroll > 0 {
			m.eventTimelineScroll--
		}
	case "g":
		m.eventTimelineScroll = 0
	case "G":
		m.eventTimelineScroll = maxScroll
	case "ctrl+d":
		m.eventTimelineScroll += 10
		if m.eventTimelineScroll > maxScroll {
			m.eventTimelineScroll = maxScroll
		}
	case "ctrl+u":
		m.eventTimelineScroll -= 10
		if m.eventTimelineScroll < 0 {
			m.eventTimelineScroll = 0
		}
	case "ctrl+f":
		m.eventTimelineScroll += 20
		if m.eventTimelineScroll > maxScroll {
			m.eventTimelineScroll = maxScroll
		}
	case "ctrl+b":
		m.eventTimelineScroll -= 20
		if m.eventTimelineScroll < 0 {
			m.eventTimelineScroll = 0
		}
	}
	return m, nil
}

// handleNetworkPolicyOverlayKey handles keyboard input in the network policy visualizer overlay.
func (m Model) handleNetworkPolicyOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.netpolData = nil
	case "j", "down":
		m.netpolScroll++
	case "k", "up":
		if m.netpolScroll > 0 {
			m.netpolScroll--
		}
	case "g":
		m.netpolScroll = 0
	case "G":
		// Jump to bottom: will be clamped during rendering.
		m.netpolScroll = 9999
	case "ctrl+d":
		m.netpolScroll += m.height / 2
	case "ctrl+u":
		m.netpolScroll -= m.height / 2
		if m.netpolScroll < 0 {
			m.netpolScroll = 0
		}
	case "ctrl+f":
		m.netpolScroll += m.height
	case "ctrl+b":
		m.netpolScroll -= m.height
		if m.netpolScroll < 0 {
			m.netpolScroll = 0
		}
	}
	return m, nil
}

// handleErrorLogOverlayKey handles keyboard input when the error log overlay is open.
func (m Model) handleErrorLogOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Count visible entries (respecting debug filter).
	visibleCount := 0
	for _, e := range m.errorLog {
		if e.Level == "DBG" && !m.showDebugLogs {
			continue
		}
		visibleCount++
	}
	// Calculate the max visible lines (same logic as RenderErrorLogOverlay).
	overlayH := min(30, m.height-4)
	maxVisible := overlayH - 4
	if maxVisible < 1 {
		maxVisible = 1
	}
	maxScroll := visibleCount - maxVisible
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch msg.String() {
	case "esc", "q":
		m.overlayErrorLog = false
		m.errorLogScroll = 0
		return m, nil
	case "d":
		m.showDebugLogs = !m.showDebugLogs
		m.errorLogScroll = 0
		return m, nil
	case "j", "down":
		if m.errorLogScroll < maxScroll {
			m.errorLogScroll++
		}
		return m, nil
	case "k", "up":
		if m.errorLogScroll > 0 {
			m.errorLogScroll--
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.errorLogScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		m.errorLogScroll = maxScroll
		return m, nil
	case "ctrl+d":
		halfPage := maxVisible / 2
		m.errorLogScroll += halfPage
		if m.errorLogScroll > maxScroll {
			m.errorLogScroll = maxScroll
		}
		return m, nil
	case "ctrl+u":
		halfPage := maxVisible / 2
		m.errorLogScroll -= halfPage
		if m.errorLogScroll < 0 {
			m.errorLogScroll = 0
		}
		return m, nil
	case "ctrl+f":
		m.errorLogScroll += maxVisible
		if m.errorLogScroll > maxScroll {
			m.errorLogScroll = maxScroll
		}
		return m, nil
	case "ctrl+b":
		m.errorLogScroll -= maxVisible
		if m.errorLogScroll < 0 {
			m.errorLogScroll = 0
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleNamespaceOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.nsFilterMode {
		return m.handleNamespaceFilterMode(msg)
	}
	return m.handleNamespaceNormalMode(msg)
}

func (m Model) handleNamespaceNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.filteredOverlayItems()

	switch msg.String() {
	case "esc", "q":
		if m.overlayFilter.Value != "" {
			m.overlayFilter.Clear()
			m.overlayCursor = 0
			return m, nil
		}
		m.overlay = overlayNone
		m.overlayFilter.Clear()
		return m, nil

	case "enter":
		// Apply selection and close.
		if m.nsSelectionModified && len(m.selectedNamespaces) > 0 {
			// User explicitly toggled selections with Space in this session.
			m.allNamespaces = false
			if len(m.selectedNamespaces) == 1 {
				for ns := range m.selectedNamespaces {
					m.namespace = ns
				}
			}
		} else if m.overlayCursor >= 0 && m.overlayCursor < len(items) && items[m.overlayCursor].Status != "all" {
			// No Space toggling — apply the cursor position as single namespace.
			ns := items[m.overlayCursor].Name
			m.selectedNamespaces = map[string]bool{ns: true}
			m.namespace = ns
			m.allNamespaces = false
		} else {
			// Cursor on "All Namespaces" or no specific item.
			m.selectedNamespaces = nil
			m.allNamespaces = true
		}
		m.overlay = overlayNone
		m.overlayFilter.Clear()
		m.nsFilterMode = false
		m.saveCurrentSession()
		m.cancelAndReset()
		m.requestGen++
		return m, m.refreshCurrentLevel()

	case " ":
		m.nsSelectionModified = true
		// Toggle selection on current item.
		if m.overlayCursor >= 0 && m.overlayCursor < len(items) {
			selected := items[m.overlayCursor]
			if selected.Status == "all" {
				// "All Namespaces" selected — clear individual selections.
				m.selectedNamespaces = nil
				m.allNamespaces = true
			} else {
				// Individual namespace — toggle it.
				if m.selectedNamespaces == nil {
					m.selectedNamespaces = make(map[string]bool)
				}
				if m.selectedNamespaces[selected.Name] {
					delete(m.selectedNamespaces, selected.Name)
					if len(m.selectedNamespaces) == 0 {
						m.selectedNamespaces = nil
						m.allNamespaces = true
					}
				} else {
					m.selectedNamespaces[selected.Name] = true
					m.allNamespaces = false
				}
			}
		}
		return m, nil

	case "c":
		m.nsSelectionModified = true
		// Clear all namespace selections (reset to all namespaces).
		m.selectedNamespaces = nil
		m.allNamespaces = true
		return m, nil

	case "/":
		m.nsFilterMode = true
		m.overlayFilter.Clear()
		return m, nil

	case "j", "down", "ctrl+n":
		if m.overlayCursor < len(items)-1 {
			m.overlayCursor++
		}
		return m, nil

	case "k", "up", "ctrl+p":
		if m.overlayCursor > 0 {
			m.overlayCursor--
		}
		return m, nil

	case "ctrl+d":
		m.overlayCursor += 10
		if m.overlayCursor >= len(items) {
			m.overlayCursor = len(items) - 1
		}
		return m, nil

	case "ctrl+u":
		m.overlayCursor -= 10
		if m.overlayCursor < 0 {
			m.overlayCursor = 0
		}
		return m, nil

	case "ctrl+f":
		m.overlayCursor += 20
		if m.overlayCursor >= len(items) {
			m.overlayCursor = len(items) - 1
		}
		return m, nil

	case "ctrl+b":
		m.overlayCursor -= 20
		if m.overlayCursor < 0 {
			m.overlayCursor = 0
		}
		return m, nil

	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleNamespaceFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.nsFilterMode = false
		m.overlayFilter.Clear()
		m.overlayCursor = 0
		return m, nil
	case "enter":
		m.nsFilterMode = false
		m.overlayCursor = 0
		return m, nil
	case "backspace":
		if len(m.overlayFilter.Value) > 0 {
			m.overlayFilter.Backspace()
			m.overlayCursor = 0
		}
		return m, nil
	case "ctrl+w":
		m.overlayFilter.DeleteWord()
		m.overlayCursor = 0
		return m, nil
	case "ctrl+a":
		m.overlayFilter.Home()
		return m, nil
	case "ctrl+e":
		m.overlayFilter.End()
		return m, nil
	case "left":
		m.overlayFilter.Left()
		return m, nil
	case "right":
		m.overlayFilter.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.overlayFilter.Insert(key)
			m.overlayCursor = 0
		}
		return m, nil
	}
}

func (m Model) handleFilterPresetOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "q":
		m.overlay = overlayNone
		return m, nil
	case "enter":
		if m.overlayCursor >= 0 && m.overlayCursor < len(m.filterPresets) {
			return m.applyFilterPreset(m.filterPresets[m.overlayCursor])
		}
		m.overlay = overlayNone
		return m, nil
	case "up", "k":
		if m.overlayCursor > 0 {
			m.overlayCursor--
		}
		return m, nil
	case "down", "j":
		if m.overlayCursor < len(m.filterPresets)-1 {
			m.overlayCursor++
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		// Shortcut key: match against preset hotkeys.
		for _, preset := range m.filterPresets {
			if preset.Key == key {
				return m.applyFilterPreset(preset)
			}
		}
	}
	return m, nil
}

// applyFilterPreset applies a filter preset to the middle items and closes the overlay.
func (m Model) applyFilterPreset(preset FilterPreset) (tea.Model, tea.Cmd) {
	m.overlay = overlayNone

	// Save the unfiltered list so we can restore it later.
	m.unfilteredMiddleItems = append([]model.Item(nil), m.middleItems...)

	// Filter middleItems.
	var filtered []model.Item
	for _, item := range m.middleItems {
		if preset.MatchFn(item) {
			filtered = append(filtered, item)
		}
	}
	m.middleItems = filtered
	m.activeFilterPreset = &preset
	m.setCursor(0)
	m.clampCursor()
	m.setStatusMessage(fmt.Sprintf("Filter: %s (%d matches)", preset.Name, len(filtered)), false)
	return m, tea.Batch(scheduleStatusClear(), m.loadPreview())
}

// handleBatchLabelOverlayKey handles keyboard input for the batch label/annotation editor overlay.
func (m Model) handleAlertsOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "q":
		m.overlay = overlayNone
		return m, nil
	case "j", "down":
		m.alertsScroll++
		return m, nil
	case "k", "up":
		if m.alertsScroll > 0 {
			m.alertsScroll--
		}
		return m, nil
	case "g":
		m.alertsScroll = 0
		return m, nil
	case "G":
		// Jump to bottom -- the render function will clamp.
		m.alertsScroll = len(m.alertsData)
		return m, nil
	case "ctrl+d":
		m.alertsScroll += 10
		return m, nil
	case "ctrl+u":
		m.alertsScroll -= 10
		if m.alertsScroll < 0 {
			m.alertsScroll = 0
		}
		return m, nil
	case "ctrl+f":
		m.alertsScroll += 20
		return m, nil
	case "ctrl+b":
		m.alertsScroll -= 20
		if m.alertsScroll < 0 {
			m.alertsScroll = 0
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleBatchLabelOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.overlay = overlayNone
		return m, nil
	case "tab":
		m.batchLabelRemove = !m.batchLabelRemove
		return m, nil
	case "enter":
		if m.batchLabelInput.Value == "" {
			return m, nil
		}
		isAnnotation := m.batchLabelMode == 1
		// Parse input: "key=value" for add, "key" for remove.
		var labelKey, labelValue string
		if m.batchLabelRemove {
			labelKey = m.batchLabelInput.Value
		} else {
			parts := strings.SplitN(m.batchLabelInput.Value, "=", 2)
			if len(parts) != 2 || parts[0] == "" {
				m.setStatusMessage("Format: key=value", true)
				return m, scheduleStatusClear()
			}
			labelKey = parts[0]
			labelValue = parts[1]
		}
		m.overlay = overlayNone
		m.loading = true
		action := "labels"
		if isAnnotation {
			action = "annotations"
		}
		mode := "Adding"
		if m.batchLabelRemove {
			mode = "Removing"
		}
		m.setStatusMessage(fmt.Sprintf("%s %s...", mode, action), false)
		m.clearSelection()
		return m, m.batchPatchLabels(labelKey, labelValue, m.batchLabelRemove, isAnnotation)
	case "backspace":
		if len(m.batchLabelInput.Value) > 0 {
			m.batchLabelInput.Backspace()
		}
		return m, nil
	case "ctrl+w":
		m.batchLabelInput.DeleteWord()
		return m, nil
	case "ctrl+a":
		m.batchLabelInput.Home()
		return m, nil
	case "ctrl+e":
		m.batchLabelInput.End()
		return m, nil
	case "left":
		m.batchLabelInput.Left()
		return m, nil
	case "right":
		m.batchLabelInput.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.batchLabelInput.Insert(key)
		}
		return m, nil
	}
}

func (m Model) handleActionOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "q":
		m.overlay = overlayNone
		return m, nil
	case "enter":
		if m.overlayCursor >= 0 && m.overlayCursor < len(m.overlayItems) {
			return m.executeAction(m.overlayItems[m.overlayCursor].Name)
		}
		m.overlay = overlayNone
		return m, nil
	case "up", "k", "ctrl+p":
		if m.overlayCursor > 0 {
			m.overlayCursor--
		}
		return m, nil
	case "down", "j", "ctrl+n":
		if m.overlayCursor < len(m.overlayItems)-1 {
			m.overlayCursor++
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		// Shortcut key: match against action hotkeys (stored in Status field).
		for _, item := range m.overlayItems {
			if item.Status == key {
				return m.executeAction(item.Name)
			}
		}
	}
	return m, nil
}

func (m Model) handleConfirmOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.overlay = overlayNone
		m.loading = true
		action := m.pendingAction
		m.pendingAction = ""
		m.confirmAction = ""

		ns := m.actionCtx.namespace
		name := m.actionCtx.name
		ctx := m.actionCtx.context
		rt := m.actionCtx.resourceType
		nsArg := ""
		if rt.Namespaced {
			nsArg = " -n " + ns
		}

		// Bulk mode.
		if m.bulkMode && len(m.bulkItems) > 0 {
			m.clearSelection()
			if action == "Force Delete" {
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete --force --grace-period=0 %s (%d items)%s --context %s", rt.Resource, len(m.bulkItems), nsArg, ctx))
				return m, m.bulkForceDeleteResources()
			}
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s (%d items)%s --context %s", rt.Resource, len(m.bulkItems), nsArg, ctx))
			return m, m.bulkDeleteResources()
		}

		if action == "Force Delete" {
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl patch %s %s --type merge -p '{\"metadata\":{\"finalizers\":null}}'%s --context %s && kubectl delete %s %s --grace-period=0 --force%s --context %s", rt.Resource, name, nsArg, ctx, rt.Resource, name, nsArg, ctx))
			return m, m.forceDeleteResource()
		}
		if action == "Drain" {
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl drain %s --ignore-daemonsets --delete-emptydir-data --context %s", name, ctx))
			return m, m.execKubectlDrain()
		}
		if rt.APIGroup == "_helm" {
			m.addLogEntry("DBG", fmt.Sprintf("$ helm uninstall %s -n %s --kube-context %s", name, ns, ctx))
		} else {
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s %s%s --context %s", rt.Resource, name, nsArg, ctx))
		}
		return m, m.deleteResource()
	case "n", "N", "esc", "q":
		m.overlay = overlayNone
		m.confirmAction = ""
		m.pendingAction = ""
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleScaleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.scaleInput.Clear()
		return m, nil
	case "enter":
		replicas, err := strconv.ParseInt(m.scaleInput.Value, 10, 32)
		if err != nil || replicas < 0 {
			m.setStatusMessage("Invalid replica count", true)
			m.overlay = overlayNone
			m.scaleInput.Clear()
			return m, scheduleStatusClear()
		}
		m.overlay = overlayNone
		m.loading = true
		m.scaleInput.Clear()

		// Bulk mode.
		if m.bulkMode && len(m.bulkItems) > 0 {
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl scale deployment --replicas=%d (%d items) -n %s --context %s", replicas, len(m.bulkItems), m.actionCtx.namespace, m.actionCtx.context))
			m.clearSelection()
			return m, m.bulkScaleResources(int32(replicas))
		}

		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl scale deployment %s --replicas=%d -n %s --context %s", m.actionCtx.name, replicas, m.actionCtx.namespace, m.actionCtx.context))
		return m, m.scaleDeployment(int32(replicas))
	case "backspace":
		if len(m.scaleInput.Value) > 0 {
			m.scaleInput.Backspace()
		}
		return m, nil
	case "ctrl+w":
		m.scaleInput.DeleteWord()
		return m, nil
	case "ctrl+a":
		m.scaleInput.Home()
		return m, nil
	case "ctrl+e":
		m.scaleInput.End()
		return m, nil
	case "left":
		m.scaleInput.Left()
		return m, nil
	case "right":
		m.scaleInput.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
			m.scaleInput.Insert(key)
		}
		return m, nil
	}
}

func (m Model) handlePortForwardOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.portForwardInput.Clear()
		m.pfAvailablePorts = nil
		m.pfPortCursor = -1
		return m, nil
	case "j", "down":
		if len(m.pfAvailablePorts) > 0 && m.pfPortCursor < len(m.pfAvailablePorts)-1 {
			m.pfPortCursor++
		}
		return m, nil
	case "k", "up":
		if m.pfPortCursor > 0 {
			m.pfPortCursor--
		}
		return m, nil
	case "enter":
		var localPort, remotePort string
		if m.pfPortCursor >= 0 && m.pfPortCursor < len(m.pfAvailablePorts) {
			p := m.pfAvailablePorts[m.pfPortCursor]
			remotePort = p.Port
			if m.portForwardInput.Value != "" {
				// User typed a custom local port.
				localPort = m.portForwardInput.Value
			} else {
				// Empty input: let kubectl pick a random high port.
				localPort = "0"
			}
		} else if m.portForwardInput.Value != "" {
			// Manual entry: parse as localPort:remotePort or just port.
			parts := strings.SplitN(m.portForwardInput.Value, ":", 2)
			localPort = parts[0]
			if len(parts) == 2 {
				remotePort = parts[1]
			} else {
				remotePort = localPort
			}
		} else {
			m.setStatusMessage("Port mapping required (e.g., 8080:80)", true)
			m.overlay = overlayNone
			return m, scheduleStatusClear()
		}
		portMapping := localPort + ":" + remotePort
		m.overlay = overlayNone
		m.portForwardInput.Clear()
		m.pfAvailablePorts = nil
		m.pfPortCursor = -1
		resourcePrefix := "pod/"
		if m.actionCtx.kind == "Service" {
			resourcePrefix = "svc/"
		}
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl port-forward %s%s %s -n %s --context %s", resourcePrefix, m.actionCtx.name, portMapping, m.actionCtx.namespace, m.actionCtx.context))
		return m, m.execKubectlPortForward(portMapping)
	case "backspace":
		if len(m.portForwardInput.Value) > 0 {
			m.portForwardInput.Backspace()
		}
		return m, nil
	case "ctrl+w":
		m.portForwardInput.DeleteWord()
		return m, nil
	case "ctrl+a":
		m.portForwardInput.Home()
		return m, nil
	case "ctrl+e":
		m.portForwardInput.End()
		return m, nil
	case "left":
		m.portForwardInput.Left()
		return m, nil
	case "right":
		m.portForwardInput.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && ((key[0] >= '0' && key[0] <= '9') || key[0] == ':') {
			m.portForwardInput.Insert(key)
			// When user types ':', they want manual local:remote mode — deselect from port list.
			if key[0] == ':' {
				m.pfPortCursor = -1
			}
		}
		return m, nil
	}
}

func (m Model) moveCursor(delta int) (tea.Model, tea.Cmd) {
	visible := m.visibleMiddleItems()
	c := m.cursor() + delta
	if c < 0 {
		c = 0
	}
	if c >= len(visible) {
		c = len(visible) - 1
	}
	if c < 0 {
		c = 0
	}
	m.setCursor(c)

	// Accordion behavior: auto-expand the group the cursor just entered.
	if m.nav.Level == model.LevelResourceTypes && !m.allGroupsExpanded {
		visible = m.visibleMiddleItems()
		if c >= 0 && c < len(visible) {
			newCat := visible[c].Category
			if newCat != "" && newCat != m.expandedGroup {
				m.expandedGroup = newCat
				// Recompute visible items after expansion change.
				newVisible := m.visibleMiddleItems()

				if delta < 0 {
					// Moving UP into a group: land on the LAST item of that group.
					for i := len(newVisible) - 1; i >= 0; i-- {
						if newVisible[i].Category == newCat && newVisible[i].Kind != "__collapsed_group__" {
							m.setCursor(i)
							break
						}
					}
				} else {
					// Moving DOWN into a group: land on the FIRST real item of that group.
					for i, item := range newVisible {
						if item.Category == newCat && item.Kind != "__collapsed_group__" {
							m.setCursor(i)
							break
						}
					}
				}
			}
		}
	}

	// Clear right column to show loading state while new preview loads.
	m.rightItems = nil
	m.previewYAML = ""
	m.previewScroll = 0
	m.loading = true
	return m, m.loadPreview()
}

func (m Model) navigateParent() (tea.Model, tea.Cmd) {
	m.cancelAndReset()
	m.requestGen++
	m.clearSelection()
	m.activeFilterPreset = nil
	m.unfilteredMiddleItems = nil
	switch m.nav.Level {
	case model.LevelClusters:
		return m, nil

	case model.LevelResourceTypes:
		m.saveCursor()
		m.nav.Level = model.LevelClusters
		m.nav.Context = ""
		m.middleItems = m.leftItems
		m.popLeft()
		m.clearRight()
		m.restoreCursor()
		return m, m.loadPreview()

	case model.LevelResources:
		m.saveCursor()
		m.nav.Level = model.LevelResourceTypes
		m.nav.ResourceType = model.ResourceTypeEntry{}
		m.middleItems = m.leftItems
		m.popLeft()
		m.clearRight()
		m.restoreCursor()
		m.syncExpandedGroup()
		return m, m.loadPreview()

	case model.LevelOwned:
		m.saveCursor()
		// If we drilled into a nested owned level (e.g., ArgoCD → Deployment),
		// pop back to the parent owned level instead of jumping to LevelResources.
		if n := len(m.ownedParentStack); n > 0 {
			parent := m.ownedParentStack[n-1]
			m.ownedParentStack = m.ownedParentStack[:n-1]
			m.nav.ResourceType = parent.resourceType
			m.nav.ResourceName = parent.resourceName
			m.nav.Namespace = parent.namespace
			// Stay at LevelOwned — we're returning to the parent's owned view.
			if cached, ok := m.itemCache[m.navKey()]; ok {
				m.middleItems = cached
			} else {
				m.middleItems = m.leftItems
			}
			m.popLeft()
			m.clearRight()
			m.restoreCursor()
			return m, m.loadPreview()
		}
		m.nav.Level = model.LevelResources
		m.nav.ResourceName = ""
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.middleItems = cached
		} else {
			m.middleItems = m.leftItems
		}
		m.popLeft()
		m.clearRight()
		m.restoreCursor()
		return m, m.loadPreview()

	case model.LevelContainers:
		m.saveCursor()
		// If we came directly from Pods (skipping LevelOwned), go back to LevelResources.
		if m.nav.ResourceType.Kind == "Pod" {
			m.nav.Level = model.LevelResources
			m.nav.ResourceName = ""
			m.nav.OwnedName = ""
		} else {
			m.nav.Level = model.LevelOwned
			m.nav.OwnedName = ""
		}
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.middleItems = cached
		} else {
			m.middleItems = m.leftItems
		}
		m.popLeft()
		m.clearRight()
		m.restoreCursor()
		return m, m.loadPreview()
	}
	return m, nil
}

func (m Model) navigateToOwner(kind, name, apiVersion string) (tea.Model, tea.Cmd) {
	crds := m.discoveredCRDs[m.nav.Context]
	rt, ok := model.FindResourceTypeByKind(kind, crds)
	if !ok {
		m.setStatusMessage(fmt.Sprintf("Unknown resource type: %s", kind), true)
		return m, scheduleStatusClear()
	}

	// Navigate back to resource types level.
	for m.nav.Level > model.LevelResourceTypes {
		ret, _ := m.navigateParent()
		m = ret.(Model)
	}

	// Find and select the target resource type in middle items.
	for i, item := range m.middleItems {
		if item.Extra == rt.ResourceRef() {
			m.setCursor(i)
			break
		}
	}

	// Set pending target to auto-select the owner resource after load.
	m.pendingTarget = name

	// Navigate into the resource type.
	return m.navigateChild()
}

func (m Model) navigateChild() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}

	m.cancelAndReset()
	m.requestGen++
	m.clearSelection()

	// Clear filter when navigating into a child.
	m.filterText = ""
	m.filterInput.Clear()
	m.filterActive = false
	m.activeFilterPreset = nil
	m.unfilteredMiddleItems = nil

	switch m.nav.Level {
	case model.LevelClusters:
		logger.Info("Context selected", "context", sel.Name)
		m.saveCursor()
		m.nav.Context = sel.Name
		m.applyPinnedGroups()
		m.nav.Level = model.LevelResourceTypes
		m.pushLeft()
		m.clearRight()
		// Use CRD-merged list if already discovered, otherwise start with built-in types.
		if crds, ok := m.discoveredCRDs[sel.Name]; ok && len(crds) > 0 {
			m.middleItems = model.MergeWithCRDs(crds)
		} else {
			m.middleItems = model.FlattenedResourceTypes()
		}
		m.itemCache[m.navKey()] = m.middleItems
		m.restoreCursor()
		m.syncExpandedGroup()
		m.saveCurrentSession()
		// Trigger async CRD discovery if not already cached.
		cmds := []tea.Cmd{m.loadPreview()}
		if _, ok := m.discoveredCRDs[sel.Name]; !ok {
			cmds = append(cmds, m.discoverCRDs(sel.Name))
		}
		return m, tea.Batch(cmds...)

	case model.LevelResourceTypes:
		// Overview or Monitoring item: enter fullscreen dashboard view.
		if sel.Extra == "__overview__" || sel.Extra == "__monitoring__" {
			m.fullscreenDashboard = true
			m.previewScroll = 0
			m.setStatusMessage("Dashboard fullscreen ON", false)
			return m, scheduleStatusClear()
		}
		// Port Forwards: show active port forwards.
		if sel.Kind == "__port_forwards__" {
			m.saveCursor()
			m.nav.ResourceType = model.ResourceTypeEntry{
				DisplayName: "Port Forwards",
				Kind:        "__port_forwards__",
				APIGroup:    "_portforward",
				APIVersion:  "v1",
				Resource:    "portforwards",
				Namespaced:  false,
			}
			m.nav.Level = model.LevelResources
			m.pushLeft()
			m.clearRight()
			m.middleItems = m.portForwardItems()
			m.setCursor(0)
			m.clampCursor()
			m.saveCurrentSession()
			return m, m.waitForPortForwardUpdate()
		}
		// Collapsed group placeholder: expand the group instead of navigating.
		if sel.Kind == "__collapsed_group__" {
			m.expandedGroup = sel.Category
			visible := m.visibleMiddleItems()
			// Move cursor to the first item in the newly expanded group.
			for i, item := range visible {
				if item.Category == sel.Category && item.Kind != "__collapsed_group__" {
					m.setCursor(i)
					break
				}
			}
			m.rightItems = nil
			m.previewYAML = ""
			m.loading = true
			return m, m.loadPreview()
		}
		rt, ok := model.FindResourceTypeIn(sel.Extra, m.discoveredCRDs[m.nav.Context])
		if !ok {
			return m, nil
		}
		m.saveCursor()
		m.nav.ResourceType = rt
		m.nav.Level = model.LevelResources
		m.pushLeft()
		m.clearRight()
		m.saveCurrentSession()
		// Use cached items if available for instant display.
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.middleItems = cached
			m.restoreCursor()
		} else {
			m.middleItems = nil
			m.setCursor(0)
		}
		m.loading = true
		return m, m.loadResources(false)

	case model.LevelResources:
		// Resources without children: don't navigate further, preview is already shown in right column.
		if !m.resourceTypeHasChildren() && m.nav.ResourceType.Kind != "Pod" {
			return m, nil
		}

		m.saveCursor()
		m.nav.ResourceName = sel.Name
		if sel.Namespace != "" {
			m.nav.Namespace = sel.Namespace
		} else if !m.allNamespaces {
			m.nav.Namespace = m.namespace
		}
		m.saveCurrentSession()

		// Pods have no owned resources; navigate directly to containers.
		if m.nav.ResourceType.Kind == "Pod" {
			m.nav.OwnedName = sel.Name
			m.nav.Level = model.LevelContainers
			m.pushLeft()
			m.clearRight()
			if cached, ok := m.itemCache[m.navKey()]; ok {
				m.middleItems = cached
				m.restoreCursor()
			} else {
				m.middleItems = nil
				m.setCursor(0)
			}
			m.loading = true
			return m, m.loadContainers(false)
		}

		m.nav.Level = model.LevelOwned
		m.pushLeft()
		m.clearRight()
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.middleItems = cached
			m.restoreCursor()
		} else {
			m.middleItems = nil
			m.setCursor(0)
		}
		m.loading = true
		return m, m.loadOwned(false)

	case model.LevelOwned:
		if sel.Kind == "Pod" {
			m.saveCursor()
			m.nav.OwnedName = sel.Name
			m.nav.Level = model.LevelContainers
			m.pushLeft()
			m.clearRight()
			if cached, ok := m.itemCache[m.navKey()]; ok {
				m.middleItems = cached
				m.restoreCursor()
			} else {
				m.middleItems = nil
				m.setCursor(0)
			}
			m.loading = true
			return m, m.loadContainers(false)
		}
		// Allow drilling into owned resources that themselves have children
		// (e.g., ArgoCD Application → Deployment → Pods).
		if kindHasOwnedChildren(sel.Kind) {
			m.saveCursor()
			// Push current parent state so navigateParent can restore it.
			m.ownedParentStack = append(m.ownedParentStack, ownedParentState{
				resourceType: m.nav.ResourceType,
				resourceName: m.nav.ResourceName,
				namespace:    m.nav.Namespace,
			})
			m.nav.ResourceType.Kind = sel.Kind
			m.nav.ResourceName = sel.Name
			if sel.Namespace != "" {
				m.nav.Namespace = sel.Namespace
			}
			m.pushLeft()
			m.clearRight()
			if cached, ok := m.itemCache[m.navKey()]; ok {
				m.middleItems = cached
				m.restoreCursor()
			} else {
				m.middleItems = nil
				m.setCursor(0)
			}
			m.loading = true
			return m, m.loadOwned(false)
		}
		// Non-drillable items at LevelOwned: do nothing on right arrow.
		// Users can still press 'y' to view YAML explicitly.
		return m, nil

	case model.LevelContainers:
		// Containers are the deepest level — don't navigate further.
		return m, nil
	}
	return m, nil
}

func (m Model) enterFullView() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}

	if m.nav.Level == model.LevelClusters || m.nav.Level == model.LevelResourceTypes {
		return m.navigateChild()
	}

	// Port forward entries are virtual — no YAML to display.
	if m.nav.ResourceType.Kind == "__port_forwards__" {
		return m, nil
	}

	m.mode = modeYAML
	m.yamlScroll = 0
	m.yamlContent = "Loading..."
	m.yamlSections = nil
	return m, m.loadYAML()
}

func (m Model) openActionMenu() (tea.Model, tea.Cmd) {
	// Bulk mode: when items are selected, show bulk action menu.
	if m.hasSelection() {
		selectedList := m.selectedItemsList()
		if len(selectedList) == 0 {
			return m, nil
		}
		m.bulkMode = true
		m.bulkItems = selectedList

		// Build action context from first selected item (for resource type info).
		kind := m.selectedResourceKind()
		if kind == "" {
			return m, nil
		}
		m.actionCtx = m.buildActionCtx(&selectedList[0], kind)

		actions := model.ActionsForBulk()
		// Filter out actions that don't apply to the selected resource kind.
		if !model.IsScaleableKind(kind) || !model.IsRestartableKind(kind) {
			filtered := actions[:0]
			for _, a := range actions {
				if a.Label == "Scale" && !model.IsScaleableKind(kind) {
					continue
				}
				if a.Label == "Restart" && !model.IsRestartableKind(kind) {
					continue
				}
				filtered = append(filtered, a)
			}
			actions = filtered
		}
		var items []model.Item
		for _, a := range actions {
			items = append(items, model.Item{
				Name:   a.Label,
				Extra:  fmt.Sprintf("%s (%d items)", a.Description, len(selectedList)),
				Status: a.Key,
			})
		}

		m.overlay = overlayAction
		m.overlayItems = items
		m.overlayCursor = 0
		return m, nil
	}

	kind := m.selectedResourceKind()
	if kind == "" {
		return m, nil
	}

	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}

	m.bulkMode = false
	m.actionCtx = m.buildActionCtx(sel, kind)

	var actions []model.ActionMenuItem
	if kind == "__port_forwards__" || kind == "__port_forward_entry__" {
		actions = model.ActionsForPortForward()
	} else if m.nav.Level == model.LevelContainers {
		actions = model.ActionsForContainer()
	} else {
		actions = model.ActionsForKind(kind)
	}

	// Append user-defined custom actions for this resource kind.
	if customActions, ok := ui.ConfigCustomActions[kind]; ok {
		for _, ca := range customActions {
			actions = append(actions, model.ActionMenuItem{
				Label:       ca.Label,
				Description: ca.Description,
				Key:         ca.Key,
			})
		}
	}

	var items []model.Item
	for _, a := range actions {
		items = append(items, model.Item{
			Name:   a.Label,
			Extra:  a.Description,
			Status: a.Key,
		})
	}

	m.overlay = overlayAction
	m.overlayItems = items
	m.overlayCursor = 0
	return m, nil
}

// buildActionCtx creates an actionContext from the current selection, extracting
// the common logic shared between openActionMenu and direct action keybindings.
func (m *Model) buildActionCtx(sel *model.Item, kind string) actionContext {
	ctx := actionContext{
		kind:    kind,
		name:    sel.Name,
		context: m.nav.Context,
	}

	// Capture the namespace of the target resource.
	if sel.Namespace != "" {
		ctx.namespace = sel.Namespace
	} else if m.allNamespaces && m.nav.Namespace != "" {
		ctx.namespace = m.nav.Namespace
	} else {
		ctx.namespace = m.namespace
	}

	switch m.nav.Level {
	case model.LevelResources:
		ctx.resourceType = m.nav.ResourceType
	case model.LevelOwned:
		if rt, ok := m.resolveOwnedResourceType(sel); ok {
			ctx.resourceType = rt
		}
	case model.LevelContainers:
		ctx.containerName = sel.Name
		ctx.name = m.nav.OwnedName
		ctx.kind = "Pod"
		ctx.resourceType = model.ResourceTypeEntry{APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true}
	}

	// Store item columns for custom action template variable substitution.
	ctx.columns = sel.Columns

	return ctx
}

func (m Model) directActionLogs() (tea.Model, tea.Cmd) {
	if m.hasSelection() {
		return m.openBulkActionDirect("Logs")
	}
	kind := m.selectedResourceKind()
	if kind == "" || kind == "__port_forwards__" {
		return m, nil
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	m.actionCtx = m.buildActionCtx(sel, kind)
	return m.executeAction("Logs")
}

func (m Model) directActionRefresh() (tea.Model, tea.Cmd) {
	m.cancelAndReset()
	m.requestGen++
	m.setStatusMessage("Refreshing...", false)
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) directActionRestart() (tea.Model, tea.Cmd) {
	if m.hasSelection() {
		return m.openBulkActionDirect("Restart")
	}
	kind := m.selectedResourceKind()
	if !model.IsRestartableKind(kind) {
		m.setStatusMessage("Restart not available for "+kind, true)
		return m, scheduleStatusClear()
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	m.actionCtx = m.buildActionCtx(sel, kind)
	return m.executeAction("Restart")
}

func (m Model) directActionExec() (tea.Model, tea.Cmd) {
	kind := m.selectedResourceKind()
	if kind == "" || kind == "__port_forwards__" {
		return m, nil
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	m.actionCtx = m.buildActionCtx(sel, kind)
	return m.executeAction("Exec")
}

func (m Model) directActionDescribe() (tea.Model, tea.Cmd) {
	kind := m.selectedResourceKind()
	if kind == "" || kind == "__port_forwards__" {
		return m, nil
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	m.actionCtx = m.buildActionCtx(sel, kind)
	return m.executeAction("Describe")
}

func (m Model) directActionDelete() (tea.Model, tea.Cmd) {
	if m.hasSelection() {
		return m.openBulkActionDirect("Delete")
	}
	kind := m.selectedResourceKind()
	if kind == "" || kind == "__port_forwards__" {
		return m, nil
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	m.actionCtx = m.buildActionCtx(sel, kind)
	return m.executeAction("Delete")
}

func (m Model) directActionForceDelete() (tea.Model, tea.Cmd) {
	if m.hasSelection() {
		return m.openBulkActionDirect("Force Delete")
	}
	kind := m.selectedResourceKind()
	if kind == "" || kind == "__port_forwards__" {
		return m, nil
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	m.actionCtx = m.buildActionCtx(sel, kind)
	m.confirmAction = sel.Name + " (FORCE)"
	m.overlay = overlayConfirm
	m.pendingAction = "Force Delete"
	return m, nil
}

func (m Model) directActionScale() (tea.Model, tea.Cmd) {
	if m.hasSelection() {
		return m.openBulkActionDirect("Scale")
	}
	kind := m.selectedResourceKind()
	if !model.IsScaleableKind(kind) {
		m.setStatusMessage("Scale not available for "+kind, true)
		return m, scheduleStatusClear()
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	m.actionCtx = m.buildActionCtx(sel, kind)
	return m.executeAction("Scale")
}

func (m Model) executeAction(actionLabel string) (tea.Model, tea.Cmd) {
	m.overlay = overlayNone

	// Handle bulk actions.
	if m.bulkMode && len(m.bulkItems) > 0 {
		return m.executeBulkAction(actionLabel)
	}

	logger.Info("Executing action",
		"action", actionLabel,
		"kind", m.actionCtx.kind,
		"name", m.actionCtx.name,
		"namespace", m.actionCtx.namespace,
		"context", m.actionCtx.context,
	)
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	rt := m.actionCtx.resourceType

	switch actionLabel {
	case "Logs":
		kind := m.actionCtx.kind
		isGroupResource := kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" ||
			kind == "Job" || kind == "CronJob" || kind == "Service"

		if isGroupResource && m.actionCtx.containerName == "" {
			// Save parent resource context for pod re-selection from the log viewer.
			m.logParentKind = m.actionCtx.kind
			m.logParentName = m.actionCtx.name
			// Show pod selector for group resources.
			m.pendingAction = actionLabel
			m.loading = true
			m.setStatusMessage("Loading pods...", false)
			return m, m.loadPodsForLogAction()
		}

		if kind != "Pod" && !isGroupResource && m.actionCtx.containerName == "" {
			m.pendingAction = actionLabel
			return m, m.loadContainersForAction()
		}

		// Direct log streaming for pods or when container is already selected.
		if m.actionCtx.containerName != "" {
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl logs -f %s -c %s -n %s --context %s", name, m.actionCtx.containerName, ns, ctx))
		} else {
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl logs -f %s --all-containers --prefix -n %s --context %s", name, ns, ctx))
		}
		// Initialize log viewer state.
		m.mode = modeLogs
		m.logLines = nil
		m.logScroll = 0
		m.logFollow = true
		m.logWrap = false
		m.logLineNumbers = true
		if m.actionCtx.containerName != "" {
			m.logTitle = fmt.Sprintf("Logs: %s/%s [%s]", m.actionNamespace(), m.actionCtx.name, m.actionCtx.containerName)
		} else {
			m.logTitle = fmt.Sprintf("Logs: %s/%s", m.actionNamespace(), m.actionCtx.name)
		}
		return m, m.startLogStream()
	case "Exec":
		kind := m.actionCtx.kind
		isParentExec := kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" ||
			kind == "Job" || kind == "CronJob" || kind == "Service"
		if isParentExec {
			m.pendingAction = actionLabel
			m.loading = true
			m.setStatusMessage("Loading pods...", false)
			return m, m.loadPodsForAction()
		}
		if m.actionCtx.containerName == "" {
			m.pendingAction = actionLabel
			m.loading = true
			m.setStatusMessage("Loading containers...", false)
			return m, m.loadContainersForAction()
		}
		cArg := ""
		if m.actionCtx.containerName != "" {
			cArg = " -c " + m.actionCtx.containerName
		}
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl exec -it %s%s -n %s --context %s -- /bin/sh -c 'clear; (bash || ash || sh)'", name, cArg, ns, ctx))
		return m, m.execKubectlExec()
	case "Attach":
		kind := m.actionCtx.kind
		isParentAttach := kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" ||
			kind == "Job" || kind == "CronJob" || kind == "Service"
		if isParentAttach {
			m.pendingAction = actionLabel
			m.loading = true
			m.setStatusMessage("Loading pods...", false)
			return m, m.loadPodsForAction()
		}
		if m.actionCtx.containerName == "" {
			m.pendingAction = actionLabel
			m.loading = true
			m.setStatusMessage("Loading containers...", false)
			return m, m.loadContainersForAction()
		}
		cArg := ""
		if m.actionCtx.containerName != "" {
			cArg = " -c " + m.actionCtx.containerName
		}
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl attach -it %s%s -n %s --context %s", name, cArg, ns, ctx))
		return m, m.execKubectlAttach()
	case "Describe":
		nsArg := ""
		if rt.Namespaced {
			nsArg = " -n " + ns
		}
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl describe %s %s%s --context %s", rt.Resource, name, nsArg, ctx))
		return m, m.execKubectlDescribe()
	case "Edit":
		nsArg := ""
		if rt.Namespaced {
			nsArg = " -n " + ns
		}
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl edit %s %s%s --context %s", rt.Resource, name, nsArg, ctx))
		return m, m.execKubectlEdit()
	case "Delete":
		m.confirmAction = m.actionCtx.name
		m.overlay = overlayConfirm
		return m, nil
	case "Scale":
		m.scaleInput.Clear()
		m.overlay = overlayScaleInput
		return m, nil
	case "Restart":
		// Restart a stopped/failed port forward entry.
		if m.actionCtx.kind == "__port_forward_entry__" || m.actionCtx.kind == "__port_forwards__" {
			pfID := m.getPortForwardID(m.actionCtx.columns)
			if pfID > 0 {
				m.setStatusMessage("Restarting port forward...", false)
				return m, m.restartPortForward(pfID)
			}
			return m, nil
		}
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl rollout restart deployment %s -n %s --context %s", name, ns, ctx))
		m.loading = true
		return m, m.restartDeployment()
	case "Rollback":
		if m.actionCtx.kind == "HelmRelease" {
			m.addLogEntry("DBG", fmt.Sprintf("$ helm history %s -n %s --kube-context %s -o json", name, ns, ctx))
			return m, m.loadHelmRevisions()
		}
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl rollout undo deployment %s -n %s --context %s", name, ns, ctx))
		return m, m.loadRevisions()
	case "Port Forward":
		m.portForwardInput.Clear()
		m.pfAvailablePorts = nil
		m.pfPortCursor = -1
		m.loading = true
		m.setStatusMessage("Loading ports...", false)
		return m, m.loadContainerPorts()
	case "Debug":
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl debug %s -it --image=busybox -n %s --context %s", name, ns, ctx))
		return m, m.execKubectlDebug()
	case "Events":
		m.loading = true
		m.setStatusMessage("Loading events...", false)
		m.addLogEntry("DBG", fmt.Sprintf("Loading event timeline for %s/%s in %s", m.actionCtx.kind, name, ns))
		return m, m.loadEventTimeline()
	case "Diff":
		m.addLogEntry("DBG", fmt.Sprintf("$ argocd app diff %s", name))
		m.loading = true
		return m, m.diffArgoApp()
	case "Sync":
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl patch app %s --type merge -p '{\"operation\":{\"sync\":{\"syncStrategy\":{\"hook\":{}}}}}' -n %s --context %s", name, ns, ctx))
		m.loading = true
		return m, m.syncArgoApp()
	case "Refresh":
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl patch app %s --type merge -p '{\"metadata\":{\"annotations\":{\"argocd.argoproj.io/refresh\":\"hard\"}}}' -n %s --context %s", name, ns, ctx))
		m.loading = true
		return m, m.refreshArgoApp()
	case "Reconcile":
		m.addLogEntry("DBG", fmt.Sprintf("Reconciling %s/%s in %s", m.actionCtx.kind, name, ns))
		m.loading = true
		return m, m.reconcileFluxResource()
	case "Suspend":
		m.addLogEntry("DBG", fmt.Sprintf("Suspending %s/%s in %s", m.actionCtx.kind, name, ns))
		m.loading = true
		return m, m.suspendFluxResource()
	case "Resume":
		m.addLogEntry("DBG", fmt.Sprintf("Resuming %s/%s in %s", m.actionCtx.kind, name, ns))
		m.loading = true
		return m, m.resumeFluxResource()
	case "Force Delete":
		nsArg := ""
		if rt.Namespaced {
			nsArg = " -n " + ns
		}
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl patch %s %s --type merge -p '{\"metadata\":{\"finalizers\":null}}'%s --context %s && kubectl delete %s %s --grace-period=0 --force%s --context %s", rt.Resource, name, nsArg, ctx, rt.Resource, name, nsArg, ctx))
		m.loading = true
		return m, m.forceDeleteResource()
	case "Cordon":
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl cordon %s --context %s", name, ctx))
		m.loading = true
		return m, m.execKubectlCordon()
	case "Uncordon":
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl uncordon %s --context %s", name, ctx))
		m.loading = true
		return m, m.execKubectlUncordon()
	case "Drain":
		m.confirmAction = m.actionCtx.name + " (drain)"
		m.pendingAction = "Drain"
		m.overlay = overlayConfirm
		return m, nil
	case "Taint":
		m.commandBarActive = true
		m.commandBarInput.Clear()
		m.commandBarInput.Insert("taint node " + name + " ")
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		return m, nil
	case "Untaint":
		// Pre-fill with existing taint keys for convenient removal.
		prefill := "taint node " + name + " "
		for _, col := range m.actionCtx.columns {
			if col.Key == "Taints" && col.Value != "" {
				// Parse taint strings and append removal syntax (key-).
				parts := strings.Split(col.Value, ", ")
				for i, p := range parts {
					// Extract just the key from key=value:effect or key:effect.
					taintKey := strings.SplitN(p, "=", 2)[0]
					taintKey = strings.SplitN(taintKey, ":", 2)[0]
					if i > 0 {
						prefill += " "
					}
					prefill += taintKey + "-"
				}
				break
			}
		}
		m.commandBarActive = true
		m.commandBarInput.Clear()
		m.commandBarInput.Insert(prefill)
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		return m, nil
	case "Trigger":
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl create job --from=cronjob/%s manual-trigger -n %s --context %s", name, ns, ctx))
		m.loading = true
		return m, m.triggerCronJob()
	case "Shell":
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl debug node/%s -it --image=busybox --context %s -- chroot /host /bin/sh", name, ctx))
		return m, m.execKubectlNodeShell()
	case "Debug Pod":
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl run debug-pod --image=alpine -it --rm --restart=Never -n %s --context %s -- sh", ns, ctx))
		return m, m.runDebugPod()
	case "Debug Mount":
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl run debug-pvc --image=alpine -it --rm --restart=Never --overrides='{...pvc:%s...}' -n %s --context %s -- sh", name, ns, ctx))
		return m, m.runDebugPodWithPVC()
	case "Open in Browser":
		if m.actionCtx.kind == "__port_forward_entry__" || m.actionCtx.kind == "__port_forwards__" {
			var localPort string
			for _, kv := range m.actionCtx.columns {
				if kv.Key == "Local" {
					localPort = kv.Value
					break
				}
			}
			if localPort != "" {
				url := "http://localhost:" + localPort
				m.setStatusMessage("Opening "+url, false)
				return m, tea.Batch(openInBrowser(url), scheduleStatusClear())
			}
			m.setStatusMessage("No local port found", true)
			return m, scheduleStatusClear()
		}
		return m.openIngressInBrowser()
	case "Values":
		m.addLogEntry("DBG", fmt.Sprintf("$ helm get values %s -n %s --kube-context %s -o yaml", name, ns, ctx))
		m.loading = true
		return m, m.loadHelmValues(false)
	case "All Values":
		m.addLogEntry("DBG", fmt.Sprintf("$ helm get values %s -n %s --kube-context %s -o yaml --all", name, ns, ctx))
		m.loading = true
		return m, m.loadHelmValues(true)
	case "Edit Values":
		m.addLogEntry("DBG", fmt.Sprintf("$ helm get values %s -n %s --kube-context %s -o yaml → $EDITOR → helm upgrade --reuse-values", name, ns, ctx))
		return m, m.editHelmValues()
	case "Permissions":
		m.loading = true
		m.setStatusMessage("Checking RBAC permissions...", false)
		return m, m.checkRBAC()
	case "Startup Analysis":
		m.loading = true
		m.setStatusMessage("Analyzing pod startup...", false)
		return m, m.loadPodStartup()
	case "Alerts":
		m.loading = true
		m.setStatusMessage("Loading Prometheus alerts...", false)
		return m, m.loadAlerts()
	case "Visualize":
		m.loading = true
		m.setStatusMessage("Loading network policy...", false)
		return m, m.loadNetworkPolicy()
	case "Labels / Annotations":
		m.labelResourceType = rt
		return m, m.loadLabelData()
	case "Stop":
		// Stop a port forward entry.
		if m.actionCtx.kind == "__port_forward_entry__" || m.actionCtx.kind == "__port_forwards__" {
			pfID := m.getPortForwardID(m.actionCtx.columns)
			if pfID > 0 {
				return m, m.stopPortForward(pfID)
			}
		}
		return m, nil
	case "Remove":
		// Remove a port forward entry.
		if m.actionCtx.kind == "__port_forward_entry__" || m.actionCtx.kind == "__port_forwards__" {
			pfID := m.getPortForwardID(m.actionCtx.columns)
			if pfID > 0 {
				m.portForwardMgr.Remove(pfID)
				m.middleItems = m.portForwardItems()
				m.clampCursor()
				m.saveCurrentPortForwards()
				m.setStatusMessage("Port forward removed", false)
				return m, scheduleStatusClear()
			}
		}
		return m, nil
	default:
		// Check if this is a user-defined custom action.
		if ca, ok := findCustomAction(m.actionCtx.kind, actionLabel); ok {
			expandedCmd := expandCustomActionTemplate(ca.Command, m.actionCtx)
			m.addLogEntry("DBG", fmt.Sprintf("$ sh -c %q", expandedCmd))
			return m, m.execCustomAction(expandedCmd)
		}
	}

	return m, nil
}

// openIngressInBrowser extracts the pre-computed URL from the selected Ingress
// resource's hidden __ingress_url column and opens it in the default browser.
func (m Model) openIngressInBrowser() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		m.setStatusMessage("No resource selected", true)
		return m, scheduleStatusClear()
	}
	// Find the pre-computed URL in the item's columns.
	var url string
	for _, kv := range sel.Columns {
		if kv.Key == "__ingress_url" {
			url = kv.Value
			break
		}
	}
	if url == "" {
		m.setStatusMessage("No host found for this ingress", true)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage("Opening "+url, false)
	return m, tea.Batch(openInBrowser(url), scheduleStatusClear())
}

// openBulkActionDirect sets up bulk mode and executes a bulk action directly
// (bypassing the action menu overlay).
func (m Model) openBulkActionDirect(actionLabel string) (tea.Model, tea.Cmd) {
	selectedList := m.selectedItemsList()
	if len(selectedList) == 0 {
		return m, nil
	}
	m.bulkMode = true
	m.bulkItems = selectedList

	kind := m.selectedResourceKind()
	if kind == "" {
		return m, nil
	}
	m.actionCtx = m.buildActionCtx(&selectedList[0], kind)

	return m.executeBulkAction(actionLabel)
}

func (m Model) executeBulkAction(actionLabel string) (tea.Model, tea.Cmd) {
	logger.Info("Executing bulk action",
		"action", actionLabel,
		"count", len(m.bulkItems),
	)
	m.addLogEntry("DBG", fmt.Sprintf("Bulk action: %s (%d items)", actionLabel, len(m.bulkItems)))

	switch actionLabel {
	case "Logs":
		m.overlay = 0
		m.bulkMode = false
		return m.startMultiLogStream(m.bulkItems)
	case "Delete":
		m.confirmAction = fmt.Sprintf("%d resources", len(m.bulkItems))
		m.overlay = overlayConfirm
		m.pendingAction = "Delete"
		return m, nil
	case "Force Delete":
		m.confirmAction = fmt.Sprintf("%d resources (FORCE)", len(m.bulkItems))
		m.overlay = overlayConfirm
		m.pendingAction = "Force Delete"
		return m, nil
	case "Scale":
		m.scaleInput.Clear()
		m.overlay = overlayScaleInput
		return m, nil
	case "Restart":
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl rollout restart deployment (%d items) -n %s --context %s", len(m.bulkItems), m.actionCtx.namespace, m.actionCtx.context))
		m.loading = true
		m.clearSelection()
		return m, m.bulkRestartResources()
	case "Labels / Annotations":
		m.batchLabelMode = 0
		m.batchLabelInput.Clear()
		m.batchLabelRemove = false
		m.overlay = overlayBatchLabel
		return m, nil
	case "Diff":
		if len(m.bulkItems) != 2 {
			m.setStatusMessage("Select exactly 2 resources to diff", true)
			return m, scheduleStatusClear()
		}
		m.loading = true
		m.setStatusMessage("Loading diff...", false)
		return m, m.loadDiff(m.actionCtx.resourceType, m.bulkItems[0], m.bulkItems[1])
	}

	return m, nil
}

func (m Model) refreshCurrentLevel() tea.Cmd {
	switch m.nav.Level {
	case model.LevelClusters:
		return m.loadContexts
	case model.LevelResourceTypes:
		return m.loadResourceTypes()
	case model.LevelResources:
		// Port forwards are virtual - refresh from the manager directly.
		if m.nav.ResourceType.Kind == "__port_forwards__" {
			return func() tea.Msg {
				return resourcesLoadedMsg{items: m.portForwardItems()}
			}
		}
		return m.loadResources(false)
	case model.LevelOwned:
		return m.loadOwned(false)
	case model.LevelContainers:
		return m.loadContainers(false)
	}
	return nil
}

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.helpSearchActive {
		switch msg.String() {
		case "esc":
			m.helpSearchActive = false
			m.helpFilter.Clear()
			m.helpSearchInput.Blur()
			return m, nil
		case "enter":
			m.helpSearchActive = false
			m.helpSearchInput.Blur()
			// Keep the filter value active.
			return m, nil
		case "ctrl+c":
			return m.closeTabOrQuit()
		default:
			var cmd tea.Cmd
			m.helpSearchInput, cmd = m.helpSearchInput.Update(msg)
			m.helpFilter.Value = m.helpSearchInput.Value()
			m.helpScroll = 0
			return m, cmd
		}
	}

	switch msg.String() {
	case "q", "esc", "?":
		m.mode = modeExplorer
		return m, nil
	case "j", "down":
		m.helpScroll++
		return m, nil
	case "k", "up":
		if m.helpScroll > 0 {
			m.helpScroll--
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.helpScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		m.helpScroll = 9999
		return m, nil
	case "ctrl+d":
		m.helpScroll += m.height / 2
		return m, nil
	case "ctrl+u":
		m.helpScroll -= m.height / 2
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
		return m, nil
	case "ctrl+f":
		m.helpScroll += m.height
		return m, nil
	case "ctrl+b":
		m.helpScroll -= m.height
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
		return m, nil
	case "/":
		m.helpSearchActive = true
		m.helpSearchInput.SetValue(m.helpFilter.Value)
		m.helpSearchInput.Focus()
		return m, textinput.Blink
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.filterText = m.filterInput.Value
		m.filterActive = false
		m.setCursor(0)
		m.clampCursor()
		return m, m.loadPreview()
	case "esc":
		m.filterActive = false
		m.filterInput.Clear()
		m.filterText = ""
		m.setCursor(0)
		m.clampCursor()
		return m, m.loadPreview()
	case "backspace":
		if len(m.filterInput.Value) > 0 {
			m.filterInput.Backspace()
			m.filterText = m.filterInput.Value
			m.setCursor(0)
			m.clampCursor()
		}
		return m, nil
	case "ctrl+w":
		m.filterInput.DeleteWord()
		m.filterText = m.filterInput.Value
		m.setCursor(0)
		m.clampCursor()
		return m, nil
	case "ctrl+a":
		m.filterInput.Home()
		return m, nil
	case "ctrl+e":
		m.filterInput.End()
		return m, nil
	case "left":
		m.filterInput.Left()
		return m, nil
	case "right":
		m.filterInput.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.filterInput.Insert(key)
			m.filterText = m.filterInput.Value
			m.setCursor(0)
			m.clampCursor()
		}
		return m, nil
	}
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.searchActive = false
		m.syncExpandedGroup()
		return m, m.loadPreview()
	case "esc":
		m.searchActive = false
		m.searchInput.Clear()
		m.setCursor(m.searchPrevCursor)
		m.clampCursor()
		m.syncExpandedGroup()
		return m, m.loadPreview()
	case "backspace":
		if len(m.searchInput.Value) > 0 {
			m.searchInput.Backspace()
			m.jumpToSearchMatch(0)
		}
		return m, nil
	case "ctrl+w":
		m.searchInput.DeleteWord()
		m.jumpToSearchMatch(0)
		return m, nil
	case "ctrl+a":
		m.searchInput.Home()
		return m, nil
	case "ctrl+e":
		m.searchInput.End()
		return m, nil
	case "left":
		m.searchInput.Left()
		return m, nil
	case "right":
		m.searchInput.Right()
		return m, nil
	case "ctrl+n":
		m.jumpToSearchMatch(m.cursor() + 1)
		return m, nil
	case "ctrl+p":
		m.jumpToPrevSearchMatch(m.cursor() - 1)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.searchInput.Insert(key)
			m.jumpToSearchMatch(0)
		}
		return m, nil
	}
}

// expandSearchQuery returns the query and its abbreviation expansion (if any).
func expandSearchQuery(query string) []string {
	q := strings.ToLower(query)
	queries := []string{q}
	if expanded, ok := ui.SearchAbbreviations[q]; ok {
		queries = append(queries, expanded)
	}
	return queries
}

func (m *Model) searchMatches(name string, queries []string) bool {
	lower := strings.ToLower(name)
	for _, q := range queries {
		if strings.Contains(lower, q) {
			return true
		}
	}
	return false
}

// searchMatchesItem checks if an item matches the search query by name or category.
func (m *Model) searchMatchesItem(item model.Item, queries []string) bool {
	if m.searchMatches(item.Name, queries) {
		return true
	}
	if item.Category != "" && m.searchMatches(item.Category, queries) {
		return true
	}
	return false
}

func (m *Model) jumpToSearchMatch(startIdx int) {
	if m.searchInput.Value == "" {
		return
	}
	queries := expandSearchQuery(m.searchInput.Value)

	// At LevelResourceTypes with collapsed groups, search ALL items (not just visible).
	if m.nav.Level == model.LevelResourceTypes && !m.allGroupsExpanded {
		m.searchAllItems(queries, startIdx, true)
		return
	}

	visible := m.visibleMiddleItems()
	for i := startIdx; i < len(visible); i++ {
		if m.searchMatchesItem(visible[i], queries) {
			m.setCursor(i)
			return
		}
	}
	for i := 0; i < startIdx && i < len(visible); i++ {
		if m.searchMatchesItem(visible[i], queries) {
			m.setCursor(i)
			return
		}
	}
}

func (m *Model) jumpToPrevSearchMatch(startIdx int) {
	if m.searchInput.Value == "" {
		return
	}
	queries := expandSearchQuery(m.searchInput.Value)

	// At LevelResourceTypes with collapsed groups, search ALL items (not just visible).
	if m.nav.Level == model.LevelResourceTypes && !m.allGroupsExpanded {
		m.searchAllItems(queries, startIdx, false)
		return
	}

	visible := m.visibleMiddleItems()
	// Search backwards from startIdx.
	for i := startIdx; i >= 0; i-- {
		if m.searchMatchesItem(visible[i], queries) {
			m.setCursor(i)
			return
		}
	}
	// Wrap around from the end.
	for i := len(visible) - 1; i > startIdx; i-- {
		if m.searchMatchesItem(visible[i], queries) {
			m.setCursor(i)
			return
		}
	}
}

// searchAllItems searches through ALL middleItems (including collapsed groups)
// and expands the matching group if needed. Used for search at LevelResourceTypes.
func (m *Model) searchAllItems(queries []string, startIdx int, forward bool) {
	// Map startIdx (visible cursor) to the corresponding item in middleItems.
	visible := m.visibleMiddleItems()
	var currentItem model.Item
	if startIdx >= 0 && startIdx < len(visible) {
		currentItem = visible[startIdx]
	}

	// Find the current item's index in the full middleItems list.
	allItems := m.middleItems
	fullStart := 0
	for i, item := range allItems {
		if item.Name == currentItem.Name && item.Kind == currentItem.Kind &&
			item.Category == currentItem.Category && item.Extra == currentItem.Extra {
			fullStart = i
			break
		}
	}

	// Search through all items in the specified direction.
	var matchIdx int = -1
	if forward {
		for i := fullStart; i < len(allItems); i++ {
			if allItems[i].Kind != "__collapsed_group__" && m.searchMatchesItem(allItems[i], queries) {
				matchIdx = i
				break
			}
		}
		if matchIdx < 0 {
			for i := 0; i < fullStart && i < len(allItems); i++ {
				if allItems[i].Kind != "__collapsed_group__" && m.searchMatchesItem(allItems[i], queries) {
					matchIdx = i
					break
				}
			}
		}
	} else {
		for i := fullStart; i >= 0; i-- {
			if allItems[i].Kind != "__collapsed_group__" && m.searchMatchesItem(allItems[i], queries) {
				matchIdx = i
				break
			}
		}
		if matchIdx < 0 {
			for i := len(allItems) - 1; i > fullStart; i-- {
				if allItems[i].Kind != "__collapsed_group__" && m.searchMatchesItem(allItems[i], queries) {
					matchIdx = i
					break
				}
			}
		}
	}

	if matchIdx < 0 {
		return
	}

	// Expand the matched item's group if it's currently collapsed.
	matchedItem := allItems[matchIdx]
	if matchedItem.Category != "" && matchedItem.Category != m.expandedGroup {
		m.expandedGroup = matchedItem.Category
	}

	// Find the matched item in the now-visible list and set cursor.
	newVisible := m.visibleMiddleItems()
	for i, item := range newVisible {
		if item.Name == matchedItem.Name && item.Kind == matchedItem.Kind &&
			item.Category == matchedItem.Category && item.Extra == matchedItem.Extra {
			m.setCursor(i)
			return
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

	if x < leftEnd {
		// Left column click: navigate parent.
		return m.navigateParent()
	} else if x < middleEnd {
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
	} else {
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

// applySortMode sets the sort mode, re-sorts items, and shows a status message.
func (m Model) applySortMode(mode sortMode) (tea.Model, tea.Cmd) {
	m.sortBy = mode
	m.sortMiddleItems()
	m.clampCursor()
	m.setStatusMessage("Sort: "+m.sortModeName(), false)
	return m, tea.Batch(m.loadPreview(), scheduleStatusClear())
}

// itemIndexFromDisplayLine maps a display line number to the actual item index,
// accounting for category headers and separators in the middle column.
func (m *Model) itemIndexFromDisplayLine(displayLine int) int {
	visible := m.visibleMiddleItems()
	line := 0
	lastCategory := ""
	for i, item := range visible {
		if item.Category != "" && item.Category != lastCategory {
			lastCategory = item.Category
			if i > 0 {
				line++ // separator
			}
			line++ // category header
		}
		if line == displayLine {
			return i
		}
		line++
	}
	return -1
}

func (m Model) handleContainerSelectOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.pendingAction = ""
		return m, nil
	case "enter":
		if m.overlayCursor >= 0 && m.overlayCursor < len(m.overlayItems) {
			m.actionCtx.containerName = m.overlayItems[m.overlayCursor].Name
			m.overlay = overlayNone
			action := m.pendingAction
			m.pendingAction = ""
			return m.executeAction(action)
		}
		m.overlay = overlayNone
		return m, nil
	case "up", "k", "ctrl+p":
		if m.overlayCursor > 0 {
			m.overlayCursor--
		}
		return m, nil
	case "down", "j", "ctrl+n":
		if m.overlayCursor < len(m.overlayItems)-1 {
			m.overlayCursor++
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handlePodSelectOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.pendingAction = ""
		return m, nil
	case "enter":
		if m.overlayCursor >= 0 && m.overlayCursor < len(m.overlayItems) {
			sel := m.overlayItems[m.overlayCursor]
			m.actionCtx.name = sel.Name
			m.actionCtx.kind = "Pod"
			if sel.Namespace != "" {
				m.actionCtx.namespace = sel.Namespace
			}
			m.overlay = overlayNone
			if m.pendingAction == "Logs" {
				m.pendingAction = ""
				return m.executeAction("Logs")
			}
			return m, m.loadContainersForAction()
		}
		m.overlay = overlayNone
		return m, nil
	case "j", "down", "ctrl+n":
		if m.overlayCursor < len(m.overlayItems)-1 {
			m.overlayCursor++
		}
		return m, nil
	case "k", "up", "ctrl+p":
		if m.overlayCursor > 0 {
			m.overlayCursor--
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// --- Bookmark handlers ---

// bookmarkCurrentLocation saves the current navigation state as a bookmark.
func (m Model) bookmarkCurrentLocation() (tea.Model, tea.Cmd) {
	if m.nav.Level < model.LevelResourceTypes {
		m.setStatusMessage("Navigate to a resource type first", true)
		return m, scheduleStatusClear()
	}

	// Build a readable name for the bookmark.
	parts := []string{m.nav.Context}
	if m.nav.ResourceType.DisplayName != "" {
		parts = append(parts, m.nav.ResourceType.DisplayName)
	}
	if m.nav.ResourceName != "" {
		parts = append(parts, m.nav.ResourceName)
	}
	name := strings.Join(parts, " > ")

	ns := m.namespace
	if m.allNamespaces {
		ns = ""
	}

	bm := model.Bookmark{
		Name:         name,
		Context:      m.nav.Context,
		Namespace:    ns,
		ResourceType: m.nav.ResourceType.ResourceRef(),
		ResourceName: m.nav.ResourceName,
	}

	m.bookmarks = addBookmark(m.bookmarks, bm)
	if err := saveBookmarks(m.bookmarks); err != nil {
		m.setStatusMessage("Failed to save bookmark: "+err.Error(), true)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage("Bookmarked: "+name, false)
	return m, scheduleStatusClear()
}

// filteredBookmarks returns bookmarks matching the current bookmark filter.
func (m *Model) filteredBookmarks() []model.Bookmark {
	if m.bookmarkFilter.Value == "" {
		return m.bookmarks
	}
	filter := strings.ToLower(m.bookmarkFilter.Value)
	var filtered []model.Bookmark
	for _, bm := range m.bookmarks {
		if strings.Contains(strings.ToLower(bm.Name), filter) {
			filtered = append(filtered, bm)
		}
	}
	return filtered
}

// bookmarkDeleteCurrent removes the bookmark at the current cursor position.
func (m *Model) bookmarkDeleteCurrent() tea.Cmd {
	filtered := m.filteredBookmarks()
	if len(filtered) == 0 || m.overlayCursor < 0 || m.overlayCursor >= len(filtered) {
		return nil
	}
	target := filtered[m.overlayCursor]
	for i, bm := range m.bookmarks {
		if bm.Name == target.Name && bm.Context == target.Context &&
			bm.ResourceType == target.ResourceType && bm.ResourceName == target.ResourceName {
			m.bookmarks = removeBookmark(m.bookmarks, i)
			break
		}
	}
	_ = saveBookmarks(m.bookmarks)
	newFiltered := m.filteredBookmarks()
	if m.overlayCursor >= len(newFiltered) && m.overlayCursor > 0 {
		m.overlayCursor--
	}
	m.setStatusMessage("Removed bookmark: "+target.Name, false)
	if len(m.bookmarks) == 0 {
		m.overlay = overlayNone
	}
	return scheduleStatusClear()
}

// bookmarkDeleteAll removes all bookmarks (or all filtered bookmarks if a filter is active).
func (m *Model) bookmarkDeleteAll() tea.Cmd {
	filtered := m.filteredBookmarks()
	if len(filtered) == 0 {
		return nil
	}
	if m.bookmarkFilter.Value == "" {
		// Delete all bookmarks.
		m.bookmarks = nil
	} else {
		// Delete only the filtered bookmarks.
		filterSet := make(map[string]bool)
		for _, bm := range filtered {
			filterSet[bm.Name+"\x00"+bm.Context+"\x00"+bm.ResourceType+"\x00"+bm.ResourceName] = true
		}
		var remaining []model.Bookmark
		for _, bm := range m.bookmarks {
			key := bm.Name + "\x00" + bm.Context + "\x00" + bm.ResourceType + "\x00" + bm.ResourceName
			if !filterSet[key] {
				remaining = append(remaining, bm)
			}
		}
		m.bookmarks = remaining
	}
	_ = saveBookmarks(m.bookmarks)
	m.overlayCursor = 0
	count := len(filtered)
	m.setStatusMessage(fmt.Sprintf("Removed %d bookmark(s)", count), false)
	if len(m.bookmarks) == 0 {
		m.overlay = overlayNone
	}
	return scheduleStatusClear()
}

// handleBookmarkOverlayKey handles key events in the bookmark overlay.
func (m Model) handleBookmarkOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := m.filteredBookmarks()

	switch m.bookmarkSearchMode {
	case bookmarkModeFilter:
		return m.handleBookmarkFilterMode(msg)
	default:
		return m.handleBookmarkNormalMode(msg, filtered)
	}
}

// handleBookmarkNormalMode handles keys when the bookmark overlay is in normal navigation mode.
func (m Model) handleBookmarkNormalMode(msg tea.KeyMsg, filtered []model.Bookmark) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.overlay = overlayNone
		m.bookmarkFilter.Clear()
		m.bookmarkSearchMode = bookmarkModeNormal
		return m, nil
	case "enter":
		if len(filtered) > 0 && m.overlayCursor >= 0 && m.overlayCursor < len(filtered) {
			return m.navigateToBookmark(filtered[m.overlayCursor])
		}
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0]-'0') - 1
		if idx < len(filtered) {
			return m.navigateToBookmark(filtered[idx])
		}
		return m, nil
	case "j", "down", "ctrl+n":
		if m.overlayCursor < len(filtered)-1 {
			m.overlayCursor++
		}
		return m, nil
	case "k", "up", "ctrl+p":
		if m.overlayCursor > 0 {
			m.overlayCursor--
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.overlayCursor = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		if len(filtered) > 0 {
			m.overlayCursor = len(filtered) - 1
		}
		return m, nil
	case "ctrl+d":
		m.overlayCursor += 10
		if m.overlayCursor >= len(filtered) {
			m.overlayCursor = len(filtered) - 1
		}
		return m, nil
	case "ctrl+u":
		m.overlayCursor -= 10
		if m.overlayCursor < 0 {
			m.overlayCursor = 0
		}
		return m, nil
	case "ctrl+f":
		m.overlayCursor += 20
		if m.overlayCursor >= len(filtered) {
			m.overlayCursor = len(filtered) - 1
		}
		return m, nil
	case "ctrl+b":
		m.overlayCursor -= 20
		if m.overlayCursor < 0 {
			m.overlayCursor = 0
		}
		return m, nil
	case "/":
		m.bookmarkSearchMode = bookmarkModeFilter
		m.bookmarkFilter.Clear()
		return m, nil
	case "d":
		cmd := m.bookmarkDeleteCurrent()
		return m, cmd
	case "D":
		cmd := m.bookmarkDeleteAll()
		return m, cmd
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleBookmarkFilterMode handles keys when the bookmark overlay is in filter input mode.
func (m Model) handleBookmarkFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.bookmarkSearchMode = bookmarkModeNormal
		m.bookmarkFilter.Clear()
		m.overlayCursor = 0
		return m, nil
	case "enter":
		m.bookmarkSearchMode = bookmarkModeNormal
		return m, nil
	case "backspace":
		if len(m.bookmarkFilter.Value) > 0 {
			m.bookmarkFilter.Backspace()
			m.overlayCursor = 0
		}
		return m, nil
	case "ctrl+w":
		m.bookmarkFilter.DeleteWord()
		m.overlayCursor = 0
		return m, nil
	case "ctrl+a":
		m.bookmarkFilter.Home()
		return m, nil
	case "ctrl+e":
		m.bookmarkFilter.End()
		return m, nil
	case "left":
		m.bookmarkFilter.Left()
		return m, nil
	case "right":
		m.bookmarkFilter.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.bookmarkFilter.Insert(key)
			m.overlayCursor = 0
		}
		return m, nil
	}
}

func (m Model) handleTemplateOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		return m, nil
	case "enter":
		if len(m.templateItems) > 0 && m.templateCursor >= 0 && m.templateCursor < len(m.templateItems) {
			tmpl := m.templateItems[m.templateCursor]
			m.overlay = overlayNone
			return m, m.applyTemplate(tmpl)
		}
		return m, nil
	case "up", "k", "ctrl+p":
		if m.templateCursor > 0 {
			m.templateCursor--
		}
		return m, nil
	case "down", "j", "ctrl+n":
		if m.templateCursor < len(m.templateItems)-1 {
			m.templateCursor++
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleSecretEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.secretData == nil {
		m.overlay = overlayNone
		return m, nil
	}

	// Handle editing mode.
	if m.secretEditing {
		switch msg.String() {
		case "esc":
			m.secretEditing = false
			m.secretEditColumn = -1
			return m, nil
		case "ctrl+s":
			// Save the edit.
			if m.secretEditColumn == 0 {
				// Renaming key.
				oldKey := m.secretData.Keys[m.secretCursor]
				newKey := m.secretEditKey.Value
				if newKey != "" && newKey != oldKey {
					val := m.secretData.Data[oldKey]
					delete(m.secretData.Data, oldKey)
					m.secretData.Data[newKey] = val
					m.secretData.Keys[m.secretCursor] = newKey
				}
			} else {
				// Editing value.
				key := m.secretData.Keys[m.secretCursor]
				m.secretData.Data[key] = m.secretEditValue.Value
			}
			m.secretEditing = false
			m.secretEditColumn = -1
			return m, nil
		case "enter":
			// Insert newline into current value.
			if m.secretEditColumn == 1 {
				m.secretEditValue.Insert("\n")
			}
			return m, nil
		case "tab":
			// Switch between key and value columns.
			if m.secretEditColumn == 0 {
				m.secretEditColumn = 1
			} else {
				m.secretEditColumn = 0
			}
			return m, nil
		case "backspace":
			if m.secretEditColumn == 0 && len(m.secretEditKey.Value) > 0 {
				m.secretEditKey.Backspace()
			} else if m.secretEditColumn == 1 && len(m.secretEditValue.Value) > 0 {
				m.secretEditValue.Backspace()
			}
			return m, nil
		case "ctrl+w":
			if m.secretEditColumn == 0 {
				m.secretEditKey.DeleteWord()
			} else {
				m.secretEditValue.DeleteWord()
			}
			return m, nil
		case "ctrl+a":
			if m.secretEditColumn == 0 {
				m.secretEditKey.Home()
			} else {
				m.secretEditValue.Home()
			}
			return m, nil
		case "ctrl+e":
			if m.secretEditColumn == 0 {
				m.secretEditKey.End()
			} else {
				m.secretEditValue.End()
			}
			return m, nil
		case "left":
			if m.secretEditColumn == 0 {
				m.secretEditKey.Left()
			} else {
				m.secretEditValue.Left()
			}
			return m, nil
		case "right":
			if m.secretEditColumn == 0 {
				m.secretEditKey.Right()
			} else {
				m.secretEditValue.Right()
			}
			return m, nil
		default:
			key := msg.String()
			if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
				if m.secretEditColumn == 0 {
					m.secretEditKey.Insert(key)
				} else {
					m.secretEditValue.Insert(key)
				}
			}
			return m, nil
		}
	}

	// Normal mode.
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.secretData = nil
		return m, nil
	case "j", "down":
		if m.secretCursor < len(m.secretData.Keys)-1 {
			m.secretCursor++
		}
		return m, nil
	case "k", "up":
		if m.secretCursor > 0 {
			m.secretCursor--
		}
		return m, nil
	case "v":
		// Toggle visibility for selected row.
		if m.secretCursor >= 0 && m.secretCursor < len(m.secretData.Keys) {
			key := m.secretData.Keys[m.secretCursor]
			m.secretRevealed[key] = !m.secretRevealed[key]
		}
		return m, nil
	case "V":
		// Toggle all values visibility.
		m.secretAllRevealed = !m.secretAllRevealed
		return m, nil
	case "e":
		// Edit selected value.
		if m.secretCursor >= 0 && m.secretCursor < len(m.secretData.Keys) {
			key := m.secretData.Keys[m.secretCursor]
			m.secretEditing = true
			m.secretEditColumn = 1
			m.secretEditKey.Set(key)
			m.secretEditValue.Set(m.secretData.Data[key])
		}
		return m, nil
	case "a":
		// Add new key-value pair.
		newKey := fmt.Sprintf("new-key-%d", len(m.secretData.Keys)+1)
		m.secretData.Keys = append(m.secretData.Keys, newKey)
		m.secretData.Data[newKey] = ""
		m.secretCursor = len(m.secretData.Keys) - 1
		m.secretEditing = true
		m.secretEditColumn = 0
		m.secretEditKey.Set(newKey)
		m.secretEditValue.Clear()
		return m, nil
	case "D":
		// Delete selected row.
		if m.secretCursor >= 0 && m.secretCursor < len(m.secretData.Keys) {
			key := m.secretData.Keys[m.secretCursor]
			delete(m.secretData.Data, key)
			m.secretData.Keys = append(m.secretData.Keys[:m.secretCursor], m.secretData.Keys[m.secretCursor+1:]...)
			if m.secretCursor >= len(m.secretData.Keys) && m.secretCursor > 0 {
				m.secretCursor--
			}
		}
		return m, nil
	case "y":
		// Copy current value to clipboard.
		if m.secretCursor >= 0 && m.secretCursor < len(m.secretData.Keys) {
			key := m.secretData.Keys[m.secretCursor]
			val := m.secretData.Data[key]
			m.setStatusMessage("Copied value of "+key, false)
			return m, tea.Batch(copyToSystemClipboard(val), scheduleStatusClear())
		}
		return m, nil
	case "s":
		// Save the secret.
		return m, m.saveSecretData()
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleConfigMapEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.configMapData == nil {
		m.overlay = overlayNone
		return m, nil
	}

	// Handle editing mode.
	if m.configMapEditing {
		switch msg.String() {
		case "esc":
			m.configMapEditing = false
			m.configMapEditColumn = -1
			return m, nil
		case "ctrl+s":
			// Save the edit.
			if m.configMapEditColumn == 0 {
				// Renaming key.
				oldKey := m.configMapData.Keys[m.configMapCursor]
				newKey := m.configMapEditKey.Value
				if newKey != "" && newKey != oldKey {
					val := m.configMapData.Data[oldKey]
					delete(m.configMapData.Data, oldKey)
					m.configMapData.Data[newKey] = val
					m.configMapData.Keys[m.configMapCursor] = newKey
				}
			} else {
				// Editing value.
				key := m.configMapData.Keys[m.configMapCursor]
				m.configMapData.Data[key] = m.configMapEditValue.Value
			}
			m.configMapEditing = false
			m.configMapEditColumn = -1
			return m, nil
		case "enter":
			// Insert newline into current value.
			if m.configMapEditColumn == 1 {
				m.configMapEditValue.Insert("\n")
			}
			return m, nil
		case "tab":
			// Switch between key and value columns.
			if m.configMapEditColumn == 0 {
				m.configMapEditColumn = 1
			} else {
				m.configMapEditColumn = 0
			}
			return m, nil
		case "backspace":
			if m.configMapEditColumn == 0 && len(m.configMapEditKey.Value) > 0 {
				m.configMapEditKey.Backspace()
			} else if m.configMapEditColumn == 1 && len(m.configMapEditValue.Value) > 0 {
				m.configMapEditValue.Backspace()
			}
			return m, nil
		case "ctrl+w":
			if m.configMapEditColumn == 0 {
				m.configMapEditKey.DeleteWord()
			} else {
				m.configMapEditValue.DeleteWord()
			}
			return m, nil
		case "ctrl+a":
			if m.configMapEditColumn == 0 {
				m.configMapEditKey.Home()
			} else {
				m.configMapEditValue.Home()
			}
			return m, nil
		case "ctrl+e":
			if m.configMapEditColumn == 0 {
				m.configMapEditKey.End()
			} else {
				m.configMapEditValue.End()
			}
			return m, nil
		case "left":
			if m.configMapEditColumn == 0 {
				m.configMapEditKey.Left()
			} else {
				m.configMapEditValue.Left()
			}
			return m, nil
		case "right":
			if m.configMapEditColumn == 0 {
				m.configMapEditKey.Right()
			} else {
				m.configMapEditValue.Right()
			}
			return m, nil
		default:
			key := msg.String()
			if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
				if m.configMapEditColumn == 0 {
					m.configMapEditKey.Insert(key)
				} else {
					m.configMapEditValue.Insert(key)
				}
			}
			return m, nil
		}
	}

	// Normal mode.
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.configMapData = nil
		return m, nil
	case "j", "down":
		if m.configMapCursor < len(m.configMapData.Keys)-1 {
			m.configMapCursor++
		}
		return m, nil
	case "k", "up":
		if m.configMapCursor > 0 {
			m.configMapCursor--
		}
		return m, nil
	case "e":
		// Edit selected value.
		if m.configMapCursor >= 0 && m.configMapCursor < len(m.configMapData.Keys) {
			key := m.configMapData.Keys[m.configMapCursor]
			m.configMapEditing = true
			m.configMapEditColumn = 1
			m.configMapEditKey.Set(key)
			m.configMapEditValue.Set(m.configMapData.Data[key])
		}
		return m, nil
	case "a":
		// Add new key-value pair.
		newKey := fmt.Sprintf("new-key-%d", len(m.configMapData.Keys)+1)
		m.configMapData.Keys = append(m.configMapData.Keys, newKey)
		m.configMapData.Data[newKey] = ""
		m.configMapCursor = len(m.configMapData.Keys) - 1
		m.configMapEditing = true
		m.configMapEditColumn = 0
		m.configMapEditKey.Set(newKey)
		m.configMapEditValue.Clear()
		return m, nil
	case "D":
		// Delete selected row.
		if m.configMapCursor >= 0 && m.configMapCursor < len(m.configMapData.Keys) {
			key := m.configMapData.Keys[m.configMapCursor]
			delete(m.configMapData.Data, key)
			m.configMapData.Keys = append(m.configMapData.Keys[:m.configMapCursor], m.configMapData.Keys[m.configMapCursor+1:]...)
			if m.configMapCursor >= len(m.configMapData.Keys) && m.configMapCursor > 0 {
				m.configMapCursor--
			}
		}
		return m, nil
	case "y":
		// Copy current value to clipboard.
		if m.configMapCursor >= 0 && m.configMapCursor < len(m.configMapData.Keys) {
			key := m.configMapData.Keys[m.configMapCursor]
			val := m.configMapData.Data[key]
			m.setStatusMessage("Copied value of "+key, false)
			return m, tea.Batch(copyToSystemClipboard(val), scheduleStatusClear())
		}
		return m, nil
	case "s":
		// Save the configmap.
		return m, m.saveConfigMapData()
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// navigateToBookmark jumps to the location described by a bookmark.
func (m Model) navigateToBookmark(bm model.Bookmark) (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	m.bookmarkFilter.Clear()

	rt, ok := model.FindResourceTypeIn(bm.ResourceType, m.discoveredCRDs[bm.Context])
	if !ok {
		m.setStatusMessage("Unknown resource type in bookmark", true)
		return m, scheduleStatusClear()
	}

	// Switch context.
	m.nav.Context = bm.Context
	m.applyPinnedGroups()

	// Set namespace.
	if bm.Namespace == "" {
		m.allNamespaces = true
	} else {
		m.allNamespaces = false
		m.namespace = bm.Namespace
	}

	// Navigate to resource type level first, then optionally deeper.
	m.nav.ResourceType = rt
	m.nav.ResourceName = bm.ResourceName

	if bm.ResourceName != "" {
		// Navigate to resources level with the specific resource selected.
		m.nav.Level = model.LevelResources
	} else {
		// Navigate to resources level (listing all resources of this type).
		m.nav.Level = model.LevelResources
	}

	// Reset navigation state that doesn't apply.
	m.nav.OwnedName = ""
	m.nav.Namespace = ""

	// Reset column data and history; we'll rebuild from the target level.
	m.leftItems = nil
	m.leftItemsHistory = nil
	m.middleItems = nil
	m.rightItems = nil
	m.clearRight()

	// Rebuild left items history: clusters -> resource types.
	// Load contexts as the base left column.
	contexts, _ := m.client.GetContexts()
	var resourceTypes []model.Item
	if crds := m.discoveredCRDs[bm.Context]; len(crds) > 0 {
		resourceTypes = model.MergeWithCRDs(crds)
	} else {
		resourceTypes = model.FlattenedResourceTypes()
	}

	// Set up history: at LevelResources, leftItemsHistory has [contexts], leftItems = resourceTypes.
	m.leftItemsHistory = [][]model.Item{contexts}
	m.leftItems = resourceTypes

	// Reset cursors.
	m.cursors = [5]int{}
	m.cursorMemory = make(map[string]int)
	m.itemCache = make(map[string][]model.Item)
	m.setCursor(0)

	m.cancelAndReset()
	m.requestGen++
	m.loading = true
	m.filterText = ""
	m.filterActive = false
	m.searchActive = false

	m.setStatusMessage("Jumped to: "+bm.Name, false)
	return m, tea.Batch(m.loadResources(false), scheduleStatusClear())
}

// restoreSession applies the pending session state after contexts have been loaded.
// It navigates to the saved context and optionally to the resource type level,
// similar to how bookmark navigation works.
func (m Model) restoreSession(contexts []model.Item) (tea.Model, tea.Cmd) {
	sess := m.pendingSession
	m.pendingSession = nil
	m.sessionRestored = true

	// Verify the saved context still exists in the loaded context list.
	contextExists := false
	for _, item := range contexts {
		if item.Name == sess.Context {
			contextExists = true
			break
		}
	}
	if !contextExists {
		// Context no longer exists; fall back to normal startup.
		return m, m.loadPreview()
	}

	// Navigate into the saved context (same as navigateChild at LevelClusters).
	m.nav.Context = sess.Context
	m.applyPinnedGroups()
	m.nav.Level = model.LevelResourceTypes

	// Set up left column history: contexts list becomes the breadcrumb.
	m.leftItemsHistory = nil
	m.leftItems = contexts

	// Load resource types for the middle column.
	m.middleItems = model.FlattenedResourceTypes()
	m.itemCache[m.navKey()] = m.middleItems
	m.clearRight()

	// Restore namespace settings from session.
	if sess.AllNamespaces {
		m.allNamespaces = true
		m.selectedNamespaces = nil
	} else if sess.Namespace != "" {
		m.namespace = sess.Namespace
		m.allNamespaces = false
		if len(sess.SelectedNamespaces) > 0 {
			m.selectedNamespaces = make(map[string]bool, len(sess.SelectedNamespaces))
			for _, ns := range sess.SelectedNamespaces {
				m.selectedNamespaces[ns] = true
			}
		} else {
			m.selectedNamespaces = map[string]bool{sess.Namespace: true}
		}
	}

	cmds := []tea.Cmd{m.discoverCRDs(sess.Context)}

	// If a resource type was saved, navigate deeper.
	if sess.ResourceType != "" {
		rt, ok := model.FindResourceType(sess.ResourceType)
		if ok {
			// Push resource types into history and navigate to resources level.
			m.leftItemsHistory = [][]model.Item{contexts}
			m.leftItems = m.middleItems
			m.nav.ResourceType = rt
			m.nav.Level = model.LevelResources
			m.middleItems = nil
			m.clearRight()
			m.setCursor(0)
			m.loading = true

			if sess.ResourceName != "" {
				m.pendingTarget = sess.ResourceName
			}

			cmds = append(cmds, m.loadResources(false))
			return m, tea.Batch(cmds...)
		}
	}

	// No resource type or not found: stay at resource types level.
	m.clampCursor()
	cmds = append(cmds, m.loadPreview())
	return m, tea.Batch(cmds...)
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

func (m Model) handleLabelEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.labelData == nil {
		m.overlay = overlayNone
		return m, nil
	}

	currentKeys := m.labelData.LabelKeys
	currentData := m.labelData.Labels
	if m.labelTab == 1 {
		currentKeys = m.labelData.AnnotKeys
		currentData = m.labelData.Annotations
	}

	if m.labelEditing {
		switch msg.String() {
		case "esc":
			m.labelEditing = false
			m.labelEditColumn = -1
			return m, nil
		case "ctrl+s":
			// Save the edit.
			if m.labelEditColumn == 0 {
				oldKey := currentKeys[m.labelCursor]
				newKey := m.labelEditKey.Value
				if newKey != "" && newKey != oldKey {
					val := currentData[oldKey]
					delete(currentData, oldKey)
					currentData[newKey] = val
					currentKeys[m.labelCursor] = newKey
				}
			} else {
				key := currentKeys[m.labelCursor]
				currentData[key] = m.labelEditValue.Value
			}
			if m.labelTab == 0 {
				m.labelData.LabelKeys = currentKeys
				m.labelData.Labels = currentData
			} else {
				m.labelData.AnnotKeys = currentKeys
				m.labelData.Annotations = currentData
			}
			m.labelEditing = false
			m.labelEditColumn = -1
			return m, nil
		case "tab":
			// Switch between key and value columns.
			if m.labelEditColumn == 0 {
				m.labelEditColumn = 1
			} else {
				m.labelEditColumn = 0
			}
			return m, nil
		case "backspace":
			if m.labelEditColumn == 0 && len(m.labelEditKey.Value) > 0 {
				m.labelEditKey.Backspace()
			} else if m.labelEditColumn == 1 && len(m.labelEditValue.Value) > 0 {
				m.labelEditValue.Backspace()
			}
			return m, nil
		case "ctrl+w":
			if m.labelEditColumn == 0 {
				m.labelEditKey.DeleteWord()
			} else {
				m.labelEditValue.DeleteWord()
			}
			return m, nil
		case "ctrl+a":
			if m.labelEditColumn == 0 {
				m.labelEditKey.Home()
			} else {
				m.labelEditValue.Home()
			}
			return m, nil
		case "ctrl+e":
			if m.labelEditColumn == 0 {
				m.labelEditKey.End()
			} else {
				m.labelEditValue.End()
			}
			return m, nil
		case "left":
			if m.labelEditColumn == 0 {
				m.labelEditKey.Left()
			} else {
				m.labelEditValue.Left()
			}
			return m, nil
		case "right":
			if m.labelEditColumn == 0 {
				m.labelEditKey.Right()
			} else {
				m.labelEditValue.Right()
			}
			return m, nil
		default:
			key := msg.String()
			if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
				if m.labelEditColumn == 0 {
					m.labelEditKey.Insert(key)
				} else {
					m.labelEditValue.Insert(key)
				}
			}
			return m, nil
		}
	}

	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.labelData = nil
		return m, nil
	case "tab":
		// Switch between labels and annotations tabs.
		m.labelTab = (m.labelTab + 1) % 2
		m.labelCursor = 0
		return m, nil
	case "j", "down":
		if m.labelCursor < len(currentKeys)-1 {
			m.labelCursor++
		}
		return m, nil
	case "k", "up":
		if m.labelCursor > 0 {
			m.labelCursor--
		}
		return m, nil
	case "e":
		if m.labelCursor >= 0 && m.labelCursor < len(currentKeys) {
			key := currentKeys[m.labelCursor]
			m.labelEditing = true
			m.labelEditColumn = 1
			m.labelEditKey.Set(key)
			m.labelEditValue.Set(currentData[key])
		}
		return m, nil
	case "a":
		newKey := fmt.Sprintf("new-key-%d", len(currentKeys)+1)
		currentKeys = append(currentKeys, newKey)
		currentData[newKey] = ""
		if m.labelTab == 0 {
			m.labelData.LabelKeys = currentKeys
		} else {
			m.labelData.AnnotKeys = currentKeys
		}
		m.labelCursor = len(currentKeys) - 1
		m.labelEditing = true
		m.labelEditColumn = 0
		m.labelEditKey.Set(newKey)
		m.labelEditValue.Clear()
		return m, nil
	case "D":
		if m.labelCursor >= 0 && m.labelCursor < len(currentKeys) {
			key := currentKeys[m.labelCursor]
			delete(currentData, key)
			currentKeys = append(currentKeys[:m.labelCursor], currentKeys[m.labelCursor+1:]...)
			if m.labelTab == 0 {
				m.labelData.LabelKeys = currentKeys
			} else {
				m.labelData.AnnotKeys = currentKeys
			}
			if m.labelCursor >= len(currentKeys) && m.labelCursor > 0 {
				m.labelCursor--
			}
		}
		return m, nil
	case "y":
		if m.labelCursor >= 0 && m.labelCursor < len(currentKeys) {
			key := currentKeys[m.labelCursor]
			val := currentData[key]
			m.setStatusMessage("Copied value of "+key, false)
			return m, tea.Batch(copyToSystemClipboard(val), scheduleStatusClear())
		}
		return m, nil
	case "s":
		return m, m.saveLabelData()
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleRollbackOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.rollbackRevisions = nil
		return m, nil
	case "j", "down":
		if m.rollbackCursor < len(m.rollbackRevisions)-1 {
			m.rollbackCursor++
		}
		return m, nil
	case "k", "up":
		if m.rollbackCursor > 0 {
			m.rollbackCursor--
		}
		return m, nil
	case "ctrl+d":
		m.rollbackCursor += 10
		if m.rollbackCursor >= len(m.rollbackRevisions) {
			m.rollbackCursor = len(m.rollbackRevisions) - 1
		}
		return m, nil
	case "ctrl+u":
		m.rollbackCursor -= 10
		if m.rollbackCursor < 0 {
			m.rollbackCursor = 0
		}
		return m, nil
	case "ctrl+f":
		m.rollbackCursor += 20
		if m.rollbackCursor >= len(m.rollbackRevisions) {
			m.rollbackCursor = len(m.rollbackRevisions) - 1
		}
		return m, nil
	case "ctrl+b":
		m.rollbackCursor -= 20
		if m.rollbackCursor < 0 {
			m.rollbackCursor = 0
		}
		return m, nil
	case "enter":
		if m.rollbackCursor >= 0 && m.rollbackCursor < len(m.rollbackRevisions) {
			rev := m.rollbackRevisions[m.rollbackCursor]
			m.addLogEntry("DBG", fmt.Sprintf("Rolling back to revision %d (RS: %s)", rev.Revision, rev.Name))
			m.loading = true
			return m, m.rollbackDeployment(rev.Revision)
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleHelmRollbackOverlayKey handles keyboard input for the Helm rollback overlay.
func (m Model) handleHelmRollbackOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.helmRollbackRevisions = nil
		return m, nil
	case "j", "down":
		if m.helmRollbackCursor < len(m.helmRollbackRevisions)-1 {
			m.helmRollbackCursor++
		}
		return m, nil
	case "k", "up":
		if m.helmRollbackCursor > 0 {
			m.helmRollbackCursor--
		}
		return m, nil
	case "ctrl+d":
		m.helmRollbackCursor += 10
		if m.helmRollbackCursor >= len(m.helmRollbackRevisions) {
			m.helmRollbackCursor = len(m.helmRollbackRevisions) - 1
		}
		return m, nil
	case "ctrl+u":
		m.helmRollbackCursor -= 10
		if m.helmRollbackCursor < 0 {
			m.helmRollbackCursor = 0
		}
		return m, nil
	case "ctrl+f":
		m.helmRollbackCursor += 20
		if m.helmRollbackCursor >= len(m.helmRollbackRevisions) {
			m.helmRollbackCursor = len(m.helmRollbackRevisions) - 1
		}
		return m, nil
	case "ctrl+b":
		m.helmRollbackCursor -= 20
		if m.helmRollbackCursor < 0 {
			m.helmRollbackCursor = 0
		}
		return m, nil
	case "enter":
		if m.helmRollbackCursor >= 0 && m.helmRollbackCursor < len(m.helmRollbackRevisions) {
			rev := m.helmRollbackRevisions[m.helmRollbackCursor]
			m.addLogEntry("DBG", fmt.Sprintf("Rolling back Helm release to revision %d", rev.Revision))
			m.loading = true
			return m, m.rollbackHelmRelease(rev.Revision)
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleColorschemeOverlayKey handles keyboard input for the color scheme selector overlay.
func (m Model) handleColorschemeOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.schemeFilterMode {
		return m.handleColorschemeFilterMode(msg)
	}
	return m.handleColorschemeNormalMode(msg)
}

func (m Model) handleColorschemeNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := m.filteredSchemeNames()
	selectableCount := len(filtered)

	switch msg.String() {
	case "esc", "q":
		if m.schemeFilter.Value != "" {
			m.schemeFilter.Clear()
			m.schemeCursor = 0
			m.previewSchemeAtCursor(m.filteredSchemeNames())
			return m, nil
		}
		// Restore original theme on cancel.
		schemes := ui.BuiltinSchemes()
		if theme, ok := schemes[m.schemeOriginalName]; ok {
			ui.ApplyTheme(theme)
			ui.ActiveSchemeName = m.schemeOriginalName
		}
		m.overlay = overlayNone
		m.schemeFilter.Clear()
		return m, nil

	case "enter":
		if selectableCount > 0 && m.schemeCursor >= 0 && m.schemeCursor < selectableCount {
			name := filtered[m.schemeCursor]
			schemes := ui.BuiltinSchemes()
			if theme, ok := schemes[name]; ok {
				ui.ApplyTheme(theme)
				ui.ActiveSchemeName = name
				m.setStatusMessage("Color scheme: "+name, false)
			}
			m.overlay = overlayNone
			m.schemeFilter.Clear()
			return m, scheduleStatusClear()
		}
		return m, nil

	case "/":
		m.schemeFilterMode = true
		m.schemeFilter.Clear()
		return m, nil

	case "j", "down", "ctrl+n":
		if m.schemeCursor < selectableCount-1 {
			m.schemeCursor++
		}
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "k", "up", "ctrl+p":
		if m.schemeCursor > 0 {
			m.schemeCursor--
		}
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "ctrl+d":
		m.schemeCursor += 10
		if m.schemeCursor >= selectableCount {
			m.schemeCursor = selectableCount - 1
		}
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "ctrl+u":
		m.schemeCursor -= 10
		if m.schemeCursor < 0 {
			m.schemeCursor = 0
		}
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "ctrl+f":
		m.schemeCursor += 20
		if m.schemeCursor >= selectableCount {
			m.schemeCursor = selectableCount - 1
		}
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "ctrl+b":
		m.schemeCursor -= 20
		if m.schemeCursor < 0 {
			m.schemeCursor = 0
		}
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleColorschemeFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.schemeFilterMode = false
		m.schemeFilter.Clear()
		m.schemeCursor = 0
		m.previewSchemeAtCursor(m.filteredSchemeNames())
		return m, nil
	case "enter":
		m.schemeFilterMode = false
		m.schemeCursor = 0
		m.previewSchemeAtCursor(m.filteredSchemeNames())
		return m, nil
	case "backspace":
		if len(m.schemeFilter.Value) > 0 {
			m.schemeFilter.Backspace()
			m.schemeCursor = 0
			m.previewSchemeAtCursor(m.filteredSchemeNames())
		}
		return m, nil
	case "ctrl+w":
		m.schemeFilter.DeleteWord()
		m.schemeCursor = 0
		m.previewSchemeAtCursor(m.filteredSchemeNames())
		return m, nil
	case "ctrl+a":
		m.schemeFilter.Home()
		return m, nil
	case "ctrl+e":
		m.schemeFilter.End()
		return m, nil
	case "left":
		m.schemeFilter.Left()
		return m, nil
	case "right":
		m.schemeFilter.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.schemeFilter.Insert(key)
			m.schemeCursor = 0
			m.previewSchemeAtCursor(m.filteredSchemeNames())
		}
		return m, nil
	}
}

// previewSchemeAtCursor applies the scheme under the cursor as a live preview.
func (m *Model) previewSchemeAtCursor(filtered []string) {
	if m.schemeCursor >= 0 && m.schemeCursor < len(filtered) {
		name := filtered[m.schemeCursor]
		schemes := ui.BuiltinSchemes()
		if theme, ok := schemes[name]; ok {
			ui.ApplyTheme(theme)
			ui.ActiveSchemeName = name
		}
	}
}

// filteredSchemeNames returns the selectable scheme names filtered by the current filter text.
func (m *Model) filteredSchemeNames() []string {
	var result []string
	if m.schemeFilter.Value == "" {
		for _, e := range m.schemeEntries {
			if !e.IsHeader {
				result = append(result, e.Name)
			}
		}
		return result
	}
	lower := strings.ToLower(m.schemeFilter.Value)
	for _, e := range m.schemeEntries {
		if e.IsHeader {
			continue
		}
		if strings.Contains(e.Name, lower) {
			result = append(result, e.Name)
		}
	}
	return result
}

// handleCommandBarKey processes key events when the command bar is active.
func (m Model) handleCommandBarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.commandBarActive = false
		m.commandBarInput.Clear()
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		return m, nil
	case "enter":
		m.commandBarActive = false
		input := m.commandBarInput.Value
		m.commandBarInput.Clear()
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		trimmed := strings.TrimSpace(input)
		if trimmed == "" {
			return m, nil
		}
		// Record command in persistent history.
		m.commandHistory.add(trimmed)
		m.commandHistory.save()
		// Handle built-in commands.
		if trimmed == "q" || trimmed == "q!" || trimmed == "quit" {
			return m, tea.Quit
		}
		return m, m.executeCommandBar(input)
	case "up":
		m.commandBarInput.Set(m.commandHistory.up(m.commandBarInput.Value))
		return m, nil
	case "down":
		m.commandBarInput.Set(m.commandHistory.down())
		return m, nil
	case "tab":
		if len(m.commandBarSuggestions) > 0 {
			// Accept the currently selected suggestion into the input.
			suggestion := m.commandBarSuggestions[m.commandBarSelectedSuggestion]
			m.commandBarInput.Set(m.commandBarApplySuggestion(suggestion))
			m.commandBarSuggestions = nil
			m.commandBarSelectedSuggestion = 0
			// Regenerate suggestions for the new input state.
			m.commandBarSuggestions = m.commandBarGenerateSuggestions()
		}
		return m, nil
	case "shift+tab":
		if len(m.commandBarSuggestions) > 0 {
			m.commandBarSelectedSuggestion--
			if m.commandBarSelectedSuggestion < 0 {
				m.commandBarSelectedSuggestion = len(m.commandBarSuggestions) - 1
			}
		}
		return m, nil
	case "right":
		// Cycle forward through suggestions, or move cursor if no suggestions.
		if len(m.commandBarSuggestions) > 0 {
			m.commandBarSelectedSuggestion = (m.commandBarSelectedSuggestion + 1) % len(m.commandBarSuggestions)
		} else {
			m.commandBarInput.Right()
		}
		return m, nil
	case "left":
		// Cycle backward through suggestions, or move cursor if no suggestions.
		if len(m.commandBarSuggestions) > 0 {
			m.commandBarSelectedSuggestion--
			if m.commandBarSelectedSuggestion < 0 {
				m.commandBarSelectedSuggestion = len(m.commandBarSuggestions) - 1
			}
		} else {
			m.commandBarInput.Left()
		}
		return m, nil
	case "ctrl+a":
		m.commandBarInput.Home()
		return m, nil
	case "ctrl+e":
		m.commandBarInput.End()
		return m, nil
	case "backspace":
		if len(m.commandBarInput.Value) > 0 {
			m.commandBarInput.Backspace()
		}
		m.commandBarSuggestions = m.commandBarGenerateSuggestions()
		m.commandBarSelectedSuggestion = 0
		return m, nil
	case "ctrl+w":
		// Delete word backwards.
		m.commandBarInput.DeleteWord()
		m.commandBarSuggestions = m.commandBarGenerateSuggestions()
		m.commandBarSelectedSuggestion = 0
		return m, nil
	case "ctrl+c":
		m.commandBarActive = false
		m.commandBarInput.Clear()
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		return m, nil
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.commandBarInput.Insert(key)
		} else if key == " " {
			m.commandBarInput.Insert(" ")
		}
		m.commandBarSuggestions = m.commandBarGenerateSuggestions()
		m.commandBarSelectedSuggestion = 0
		return m, nil
	}
}

// commandBarApplySuggestion replaces the current partial word in the input
// with the accepted suggestion, followed by a trailing space.
func (m Model) commandBarApplySuggestion(suggestion string) string {
	input := m.commandBarInput.Value
	// If input ends with a space, append the suggestion as a new word.
	if strings.HasSuffix(input, " ") || input == "" {
		return input + suggestion + " "
	}
	// Otherwise replace the last partial word.
	if idx := strings.LastIndex(input, " "); idx >= 0 {
		return input[:idx+1] + suggestion + " "
	}
	return suggestion + " "
}

// commandBarGenerateSuggestions returns contextual suggestions based on the
// current command bar input. It considers the position (which word is being
// typed) and the preceding words to provide relevant completions.
//
// Suggestions are only provided when the input looks like a kubectl command
// (starts with "kubectl " or the first word matches a known subcommand).
// For arbitrary shell commands, no suggestions are returned.
func (m Model) commandBarGenerateSuggestions() []string {
	input := m.commandBarInput.Value

	// Strip optional leading "kubectl " prefix for analysis.
	stripped := input
	hasKubectlPrefix := strings.HasPrefix(strings.ToLower(stripped), "kubectl ")
	if hasKubectlPrefix {
		stripped = stripped[8:]
	}

	// Split into words, keeping track of whether we're mid-word or starting a new one.
	words := strings.Fields(stripped)
	midWord := !strings.HasSuffix(input, " ") && len(input) > 0

	// If the user has typed at least one complete word (or is mid-word on a
	// non-first word) and it doesn't look like a kubectl command, skip
	// suggestions — shell autocompletion is too complex to handle inline.
	if !hasKubectlPrefix && len(words) > 0 {
		firstWord := strings.ToLower(words[0])
		isKubectl := false
		for _, sub := range kubectlSubcommands() {
			if firstWord == sub {
				isKubectl = true
				break
			}
		}
		// If we're mid-word on the very first word, we still want to offer
		// kubectl subcommand completions so the user can discover them.
		// But if the first completed word isn't a kubectl subcommand, bail out.
		if !isKubectl && (!midWord || len(words) > 1) {
			return nil
		}
	}

	// Determine position: what word index are we completing?
	pos := len(words)
	if midWord && pos > 0 {
		pos-- // We're still typing the current word, so position is the last word's index.
	}

	var prefix string
	if midWord && len(words) > 0 {
		prefix = strings.ToLower(words[len(words)-1])
	}

	// --- Flag-aware completion ---
	// Check if the previous word is a flag that expects a value.
	if len(words) >= 1 {
		prevWord := ""
		if midWord && len(words) >= 2 {
			// Currently typing a word; the flag is the word before it.
			prevWord = words[len(words)-2]
		} else if !midWord {
			// Cursor is after a space; the flag is the last completed word.
			prevWord = words[len(words)-1]
		}

		prevWordLower := strings.ToLower(prevWord)
		switch prevWordLower {
		case "-n", "--namespace":
			return m.filterSuggestions(m.cachedNamespaces, prefix)
		case "-o", "--output":
			return m.filterSuggestions(outputFormatSuggestions(), prefix)
		}
	}

	// If the current word starts with "-", suggest common kubectl flags.
	if midWord && strings.HasPrefix(prefix, "-") {
		return m.filterSuggestions(kubectlFlagSuggestions(), prefix)
	}

	var candidates []string
	switch pos {
	case 0:
		// First word: kubectl subcommands.
		candidates = kubectlSubcommands()
	case 1:
		// Second word: depends on the subcommand.
		subCmd := ""
		if len(words) > 0 {
			subCmd = strings.ToLower(words[0])
		}
		switch subCmd {
		case "get", "describe", "delete", "edit", "patch", "label", "annotate", "scale", "autoscale":
			candidates = m.resourceTypeSuggestions()
		case "logs", "exec", "port-forward", "attach", "cp":
			// These operate on pods - suggest resource names if we have pods loaded.
			candidates = m.resourceNameSuggestions()
		case "apply", "create":
			candidates = []string{"-f", "-k", "--dry-run=client", "--dry-run=server"}
		case "rollout":
			candidates = []string{"status", "history", "undo", "restart", "pause", "resume"}
		case "config":
			candidates = []string{"view", "use-context", "get-contexts", "current-context", "set-context"}
		case "top":
			candidates = []string{"pods", "nodes"}
		case "cordon", "uncordon", "drain", "taint":
			candidates = m.resourceNameSuggestions()
		default:
			candidates = m.resourceTypeSuggestions()
		}
	default:
		// Third+ word: suggest resource names from the current view context.
		subCmd := ""
		if len(words) > 0 {
			subCmd = strings.ToLower(words[0])
		}
		switch subCmd {
		case "rollout":
			if pos == 2 {
				// After "rollout status/restart/etc", suggest resource types.
				candidates = m.resourceTypeSuggestions()
			} else {
				candidates = m.resourceNameSuggestions()
			}
		default:
			candidates = m.resourceNameSuggestions()
		}
	}

	if prefix == "" {
		// Limit visible suggestions when there's no prefix filter.
		const maxVisible = 8
		if len(candidates) > maxVisible {
			candidates = candidates[:maxVisible]
		}
		return candidates
	}

	// Filter candidates by prefix.
	var filtered []string
	for _, c := range candidates {
		if strings.HasPrefix(strings.ToLower(c), prefix) && strings.ToLower(c) != prefix {
			filtered = append(filtered, c)
		}
	}

	const maxFiltered = 8
	if len(filtered) > maxFiltered {
		filtered = filtered[:maxFiltered]
	}

	return filtered
}

// kubectlSubcommands returns common kubectl subcommands for autocompletion.
func kubectlSubcommands() []string {
	return []string{
		"get", "describe", "logs", "exec", "delete", "apply", "create",
		"edit", "patch", "scale", "rollout", "top", "label", "annotate",
		"port-forward", "cp", "cordon", "uncordon", "drain", "taint",
		"config", "auth", "api-resources", "explain", "diff",
	}
}

// resourceTypeSuggestions returns resource type names available for completion.
// It combines the built-in types with any CRDs that were discovered.
func (m Model) resourceTypeSuggestions() []string {
	seen := make(map[string]bool)
	var result []string

	for _, cat := range model.TopLevelResourceTypes() {
		for _, t := range cat.Types {
			name := strings.ToLower(t.Resource)
			if !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	}

	// Also include CRD resource types from the left column items if available.
	for _, item := range m.leftItems {
		if item.Extra != "" && item.Extra != "__overview__" && item.Extra != "__monitoring__" && item.Kind != "__port_forwards__" {
			name := strings.ToLower(item.Name)
			if !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	}

	return result
}

// resourceNameSuggestions returns resource names from the current middle column,
// which represents the resources currently visible to the user.
func (m Model) resourceNameSuggestions() []string {
	var names []string
	seen := make(map[string]bool)
	for _, item := range m.middleItems {
		if item.Name != "" && !seen[item.Name] {
			seen[item.Name] = true
			names = append(names, item.Name)
		}
	}
	return names
}

// kubectlFlagSuggestions returns common kubectl flags for autocompletion.
func kubectlFlagSuggestions() []string {
	return []string{
		"-n", "--namespace",
		"-o", "--output",
		"-l", "--selector",
		"-A", "--all-namespaces",
		"--sort-by",
		"--field-selector",
		"--show-labels",
		"-w", "--watch",
		"--no-headers",
		"-f", "--filename",
		"--dry-run=client", "--dry-run=server",
	}
}

// outputFormatSuggestions returns kubectl output format values.
func outputFormatSuggestions() []string {
	return []string{
		"json", "yaml", "wide", "name",
		"jsonpath=", "custom-columns=",
	}
}

// filterSuggestions filters candidates by prefix and returns a limited result set.
// This is used by flag-aware completion paths that bypass the main switch logic.
func (m Model) filterSuggestions(candidates []string, prefix string) []string {
	const maxResults = 8
	if prefix == "" {
		if len(candidates) > maxResults {
			candidates = candidates[:maxResults]
		}
		return candidates
	}
	var filtered []string
	for _, c := range candidates {
		if strings.HasPrefix(strings.ToLower(c), prefix) && strings.ToLower(c) != prefix {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) > maxResults {
		filtered = filtered[:maxResults]
	}
	return filtered
}

// cleanupFullscreenMode resets the current fullscreen mode to modeExplorer
// and cleans up any resources (e.g., active log streams) tied to the mode.
func (m *Model) cleanupFullscreenMode() {
	// Don't cancel log streams — they are preserved per-tab.
	// saveCurrentTab() will store logCancel/logCh so they can be resumed.
	if m.mode == modeExec {
		m.cleanupExecPTY()
	}
	m.mode = modeExplorer
}

// closeTabOrQuit closes the current tab if multiple tabs are open,
// otherwise quits the application.
func (m Model) closeTabOrQuit() (tea.Model, tea.Cmd) {
	if len(m.tabs) > 1 {
		m.tabs = append(m.tabs[:m.activeTab], m.tabs[m.activeTab+1:]...)
		if m.activeTab >= len(m.tabs) {
			m.activeTab = len(m.tabs) - 1
		}
		m.loadTab(m.activeTab)
		return m, m.loadPreview()
	}
	// Stop all active port forwards before quitting.
	if m.portForwardMgr != nil {
		m.portForwardMgr.StopAll()
	}
	return m, tea.Quit
}
