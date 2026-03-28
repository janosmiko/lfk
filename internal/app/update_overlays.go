package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
)

func (m Model) handleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.overlay {
	case overlayNamespace:
		return m.handleNamespaceOverlayKey(msg)
	case overlayAction:
		return m.handleActionOverlayKey(msg)
	case overlayConfirm:
		return m.handleConfirmOverlayKey(msg)
	case overlayConfirmType:
		return m.handleConfirmTypeOverlayKey(msg)
	case overlayScaleInput:
		return m.handleScaleOverlayKey(msg)
	case overlayPVCResize:
		return m.handlePVCResizeOverlayKey(msg)
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
	case overlayCanI:
		return m.handleCanIKey(msg)
	case overlayCanISubject:
		return m.handleCanISubjectOverlayKey(msg)
	case overlayExplainSearch:
		return m.handleExplainSearchOverlayKey(msg)
	case overlayQuitConfirm:
		return m.handleQuitConfirmOverlayKey(msg)
	case overlayLogPodSelect:
		return m.handleLogPodSelectOverlayKey(msg)
	case overlayLogContainerSelect:
		return m.handleLogContainerSelectOverlayKey(msg)
	}
	return m, nil
}

// handleEventTimelineOverlayKey handles keyboard input for the event timeline overlay.
func (m Model) handleEventTimelineOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	// Max scroll is clamped by the renderer, so use event count as upper bound.
	maxScroll := max(len(m.eventTimelineData)-1, 0)

	switch key {
	case "esc", "q":
		m.overlay = overlayNone
	case "j", "down":
		m.eventTimelineScroll = clampOverlayCursor(m.eventTimelineScroll, 1, maxScroll)
	case "k", "up":
		m.eventTimelineScroll = clampOverlayCursor(m.eventTimelineScroll, -1, maxScroll)
	case "g":
		m.eventTimelineScroll = 0
	case "G":
		m.eventTimelineScroll = maxScroll
	case "ctrl+d":
		m.eventTimelineScroll = clampOverlayCursor(m.eventTimelineScroll, 10, maxScroll)
	case "ctrl+u":
		m.eventTimelineScroll = clampOverlayCursor(m.eventTimelineScroll, -10, maxScroll)
	case "ctrl+f":
		m.eventTimelineScroll = clampOverlayCursor(m.eventTimelineScroll, 20, maxScroll)
	case "ctrl+b":
		m.eventTimelineScroll = clampOverlayCursor(m.eventTimelineScroll, -20, maxScroll)
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
		m.errorLogScroll = clampOverlayCursor(m.errorLogScroll, 1, maxScroll)
		return m, nil
	case "k", "up":
		m.errorLogScroll = clampOverlayCursor(m.errorLogScroll, -1, maxScroll)
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
		m.errorLogScroll = clampOverlayCursor(m.errorLogScroll, halfPage, maxScroll)
		return m, nil
	case "ctrl+u":
		halfPage := maxVisible / 2
		m.errorLogScroll = clampOverlayCursor(m.errorLogScroll, -halfPage, maxScroll)
		return m, nil
	case "ctrl+f":
		m.errorLogScroll = clampOverlayCursor(m.errorLogScroll, maxVisible, maxScroll)
		return m, nil
	case "ctrl+b":
		m.errorLogScroll = clampOverlayCursor(m.errorLogScroll, -maxVisible, maxScroll)
		return m, nil
	}
	return m, nil
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
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(m.filterPresets)-1)
		return m, nil
	case "down", "j":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(m.filterPresets)-1)
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
		m.alertsScroll = max(m.alertsScroll-10, 0)
		return m, nil
	case "ctrl+f":
		m.alertsScroll += 20
		return m, nil
	case "ctrl+b":
		m.alertsScroll = max(m.alertsScroll-20, 0)
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
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(m.overlayItems)-1)
		return m, nil
	case "down", "j", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(m.overlayItems)-1)
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

		// Bulk delete.
		if m.bulkMode && len(m.bulkItems) > 0 {
			m.clearSelection()
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s (%d items)%s --context %s", rt.Resource, len(m.bulkItems), nsArg, ctx))
			return m, m.bulkDeleteResources()
		}

		if action == "Drain" {
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl drain %s --ignore-daemonsets --delete-emptydir-data --context %s", name, ctx))
			return m, m.execKubectlDrain()
		}

		// Regular delete.
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

func (m Model) handleConfirmTypeOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.confirmAction = ""
		m.confirmTitle = ""
		m.confirmQuestion = ""
		m.pendingAction = ""
		m.confirmTypeInput.Clear()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	case "enter":
		if m.confirmTypeInput.Value == "DELETE" {
			m.overlay = overlayNone
			m.loading = true
			action := m.pendingAction
			m.pendingAction = ""
			m.confirmAction = ""
			m.confirmTitle = ""
			m.confirmQuestion = ""
			m.confirmTypeInput.Clear()

			ns := m.actionCtx.namespace
			name := m.actionCtx.name
			ctx := m.actionCtx.context
			rt := m.actionCtx.resourceType
			nsArg := ""
			if rt.Namespaced {
				nsArg = " -n " + ns
			}

			// Bulk force delete.
			if m.bulkMode && len(m.bulkItems) > 0 && action == "Force Delete" {
				m.clearSelection()
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete --force --grace-period=0 %s (%d items)%s --context %s", rt.Resource, len(m.bulkItems), nsArg, ctx))
				return m, m.bulkForceDeleteResources()
			}

			switch action {
			case "Force Finalize":
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl patch %s %s --type merge -p '{\"metadata\":{\"finalizers\":null}}'%s --context %s", rt.Resource, name, nsArg, ctx))
				return m, m.removeFinalizers()
			case "Force Delete":
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s %s --grace-period=0 --force%s --context %s", rt.Resource, name, nsArg, ctx))
				return m, m.forceDeleteResource()
			}
		}
		return m, nil
	case "backspace":
		m.confirmTypeInput.Backspace()
		return m, nil
	case "ctrl+w":
		m.confirmTypeInput.DeleteWord()
		return m, nil
	case "ctrl+u":
		m.confirmTypeInput.Clear()
		return m, nil
	default:
		if len(msg.String()) == 1 {
			m.confirmTypeInput.Insert(msg.String())
		}
		return m, nil
	}
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
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl scale %s --replicas=%d (%d items) -n %s --context %s", strings.ToLower(m.actionCtx.kind), replicas, len(m.bulkItems), m.actionCtx.namespace, m.actionCtx.context))
			m.clearSelection()
			return m, m.bulkScaleResources(int32(replicas))
		}

		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl scale %s %s --replicas=%d -n %s --context %s", strings.ToLower(m.actionCtx.kind), m.actionCtx.name, replicas, m.actionCtx.namespace, m.actionCtx.context))
		return m, m.scaleResource(int32(replicas))
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

func (m Model) handlePVCResizeOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.scaleInput.Clear()
		return m, nil
	case "enter":
		newSize := strings.TrimSpace(m.scaleInput.Value)
		if newSize == "" {
			m.setStatusMessage("No size specified", true)
			m.overlay = overlayNone
			m.scaleInput.Clear()
			return m, scheduleStatusClear()
		}
		m.overlay = overlayNone
		m.loading = true
		m.addLogEntry("DBG", fmt.Sprintf("Resizing PVC %s to %s in %s", m.actionCtx.name, newSize, m.actionNamespace()))
		m.scaleInput.Clear()
		return m, m.resizePVC(newSize)
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
		if len(key) == 1 {
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
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(m.overlayItems)-1)
		return m, nil
	case "down", "j", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(m.overlayItems)-1)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleQuitConfirmOverlayKey handles keyboard input for the quit confirmation overlay.
func (m Model) handleQuitConfirmOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.overlay = overlayNone
		if m.portForwardMgr != nil {
			m.portForwardMgr.StopAll()
		}
		m.saveCurrentSession()
		return m, tea.Quit
	case "n", "N", "esc", "q":
		m.overlay = overlayNone
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}
