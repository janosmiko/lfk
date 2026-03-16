package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

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
		switch {
		case m.nsSelectionModified && len(m.selectedNamespaces) > 0:
			// User explicitly toggled selections with Space in this session.
			m.allNamespaces = false
			if len(m.selectedNamespaces) == 1 {
				for ns := range m.selectedNamespaces {
					m.namespace = ns
				}
			}
		case m.overlayCursor >= 0 && m.overlayCursor < len(items) && items[m.overlayCursor].Status != "all":
			// No Space toggling — apply the cursor position as single namespace.
			ns := items[m.overlayCursor].Name
			m.selectedNamespaces = map[string]bool{ns: true}
			m.namespace = ns
			m.allNamespaces = false
		default:
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
		switch {
		case m.pfPortCursor >= 0 && m.pfPortCursor < len(m.pfAvailablePorts):
			p := m.pfAvailablePorts[m.pfPortCursor]
			remotePort = p.Port
			if m.portForwardInput.Value != "" {
				// User typed a custom local port.
				localPort = m.portForwardInput.Value
			} else {
				// Empty input: let kubectl pick a random high port.
				localPort = "0"
			}
		case m.portForwardInput.Value != "":
			// Manual entry: parse as localPort:remotePort or just port.
			parts := strings.SplitN(m.portForwardInput.Value, ":", 2)
			localPort = parts[0]
			if len(parts) == 2 {
				remotePort = parts[1]
			} else {
				remotePort = localPort
			}
		default:
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
