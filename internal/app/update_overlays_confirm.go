package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleConfirmOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		// Read-only safety net: if RO was toggled on while a confirm overlay
		// was already showing, refuse to commit the mutation.
		if m.readOnly && isMutatingAction(m.pendingAction) {
			m.overlay = overlayNone
			label := m.pendingAction
			m.pendingAction = ""
			m.confirmAction = ""
			m.setStatusMessage(readOnlyBlockedMessage(label), true)
			return m, scheduleStatusClear()
		}
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
			expanded := expandGroupedItems(m.bulkItems)
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s (%d items)%s --context %s", rt.Resource, len(expanded), nsArg, ctx))
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
			// Read-only safety net for force-delete / finalizer-remove paths.
			if m.readOnly && isMutatingAction(m.pendingAction) {
				m.overlay = overlayNone
				label := m.pendingAction
				m.pendingAction = ""
				m.confirmAction = ""
				m.confirmTitle = ""
				m.confirmQuestion = ""
				m.confirmTypeInput.Clear()
				m.setStatusMessage(readOnlyBlockedMessage(label), true)
				return m, scheduleStatusClear()
			}
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
				expanded := expandGroupedItems(m.bulkItems)
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete --force --grace-period=0 %s (%d items)%s --context %s", rt.Resource, len(expanded), nsArg, ctx))
				return m, m.bulkForceDeleteResources()
			}

			switch action {
			case "Force Finalize":
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl patch %s %s --type merge -p '{\"metadata\":{\"finalizers\":null}}'%s --context %s", rt.Resource, name, nsArg, ctx))
				return m, m.removeFinalizers()
			case "Force Delete":
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s %s --grace-period=0 --force%s --context %s", rt.Resource, name, nsArg, ctx))
				return m, m.forceDeleteResource()
			case "Finalizer Remove":
				m.loading = false
				m.overlay = overlayFinalizerSearch
				selectedCount := len(m.finalizerSearchSelected)
				m.addLogEntry("DBG", fmt.Sprintf("Removing finalizer %q from %d resources", m.finalizerSearchPattern, selectedCount))
				return m, m.bulkRemoveFinalizer()
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
		// Belt-and-suspenders read-only gate: the dispatcher already blocks
		// "Scale" upstream, but a user who toggled RO on while this overlay
		// was open could otherwise commit a scale operation.
		if m.readOnly {
			m.overlay = overlayNone
			m.scaleInput.Clear()
			m.setStatusMessage(readOnlyBlockedMessage("Scale"), true)
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
		if m.readOnly {
			m.overlay = overlayNone
			m.scaleInput.Clear()
			m.setStatusMessage(readOnlyBlockedMessage("Resize PVC"), true)
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
	case "enter", "y", "Y":
		m.overlay = overlayNone
		m.performQuitCleanup()
		return m, tea.Quit
	case "n", "N", "esc", "q":
		m.overlay = overlayNone
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}
