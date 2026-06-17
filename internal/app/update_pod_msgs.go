package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) updatePodSelect(msg podSelectMsg) (tea.Model, tea.Cmd) {
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
	m.logView.podFilterText = ""
	m.logView.podFilterActive = false
	return m, nil
}

func (m Model) updatePodLogSelect(msg podLogSelectMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Error: ", msg.err)
		m.pendingAction = ""
		// If in log mode, restart the previous log stream on error.
		if m.mode == modeLogs && m.logView.savedPodName != "" {
			m.actionCtx.name = m.logView.savedPodName
			m.actionCtx.kind = "Pod"
			m.logView.savedPodName = ""
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
		if m.mode == modeLogs && m.logView.savedPodName != "" {
			m.actionCtx.name = m.logView.savedPodName
			m.actionCtx.kind = "Pod"
			m.logView.savedPodName = ""
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
			m.logView.savedPodName = ""
			m.resetLogBuffer()
			m.logView.scroll = 0
			m.logView.follow = true
			m.logView.tailLines = ui.ConfigLogTailLines
			m.logView.hasMoreHistory = true
			m.logView.loadingHistory = false
			m.logView.cursor = 0
			m.logView.visualMode = false
			m.logView.title = "Logs: " + resourceTitleLabel(m.actionCtx.kind, m.actionNamespace(), m.actionCtx.name)
			return m, m.startLogStream()
		}
		// Multiple pods; show inline pod selector overlay with "All Pods" at top.
		allItem := model.Item{Name: "All Pods", Status: "all"}
		m.overlayItems = append([]model.Item{allItem}, pods...)
		m.overlay = overlayLogPodSelect
		m.overlayCursor = 0
		m.logView.podFilterText = ""
		m.logView.podFilterActive = false
		return m, nil
	}

	if len(pods) == 1 {
		// Only one pod, start log streaming directly. Preserve the originally
		// requested action (e.g., "Tail Logs") instead of always dispatching
		// "Logs".
		m.actionCtx.name = pods[0].Name
		m.actionCtx.kind = "Pod"
		if pods[0].Namespace != "" {
			m.actionCtx.namespace = pods[0].Namespace
		}
		action := m.pendingAction
		if action == "" {
			action = "Logs"
		}
		m.pendingAction = ""
		return m.executeAction(action)
	}
	// Multiple pods, show picker.
	m.overlayItems = pods
	m.overlay = overlayPodSelect
	m.overlayCursor = 0
	m.logView.podFilterText = ""
	m.logView.podFilterActive = false
	return m, nil
}

func (m Model) updateContainerSelect(msg containerSelectMsg) (tea.Model, tea.Cmd) {
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
}

func (m Model) updatePodStartup(msg podStartupMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("Startup analysis failed: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	m.podStartupData = msg.info
	m.overlay = overlayPodStartup
	return m, nil
}

func (m Model) updateLogContainersLoaded(msg logContainersLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Failed to load containers: ", msg.err)
		m.overlay = overlayNone
		return m, scheduleStatusClear()
	}
	if len(msg.containers) <= 1 {
		// Only one container (or none), no need for a selector — fall through
		// to the log stream so the action does not stall on this fast path.
		m.overlay = overlayNone
		if len(msg.containers) == 1 {
			m.actionCtx.containerName = msg.containers[0]
		} else {
			m.actionCtx.containerName = ""
		}
		return m, m.startLogStream()
	}
	m.logView.containers = msg.containers
	// Build overlay items with "All Containers" virtual item at the top.
	items := []model.Item{{Name: "All Containers", Status: "all"}}
	for _, c := range msg.containers {
		items = append(items, model.Item{Name: c})
	}
	m.overlayItems = items
	// Open the overlay only now that the data is ready, so the user never
	// sees a flashing empty/loading overlay before the real content arrives.
	m.overlay = overlayLogContainerSelect
	m.overlayCursor = 0
	m.logView.containerFilterText = ""
	m.logView.containerFilterActive = false
	m.logView.containerSelectionModified = false
	return m, nil
}
