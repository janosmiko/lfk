package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
)

func (m Model) handleConfirmOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		// Cycle the cascade policy in place. Ignored on confirms that don't
		// delete, so tab stays inert rather than silently changing state the
		// overlay isn't showing.
		if m.deleteConfirmShowsPolicy() {
			m.cycleDeletePropagation()
		}
		return m, nil
	case "enter", "y", "Y":
		// Read-only safety net: if RO was toggled on while a confirm overlay
		// was already showing, refuse to commit the mutation.
		if m.pendingActionBlockedByReadOnly() {
			m.overlay = overlayNone
			label := m.pendingAction
			m.pendingAction = ""
			m.confirmAction = ""
			m.confirmTitle = ""
			m.confirmQuestion = ""
			m.resetBulkAction()
			m.setStatusMessage(readOnlyBlockedMessage(label), true)
			return m, scheduleStatusClear()
		}
		m.overlay = overlayNone
		m.loading = true
		action := m.pendingAction
		m.pendingAction = ""
		m.confirmAction = ""
		// Clear any title/question override (set by non-delete confirms such as
		// Longhorn replica eviction) so the next overlayConfirm opener that
		// relies on the default "Delete X?" wording is not shown stale text.
		m.confirmTitle = ""
		m.confirmQuestion = ""

		if action == "Apply Taints" {
			return m, m.applyTaintEditor()
		}

		ns := m.actionCtx.namespace
		name := m.actionCtx.name
		ctx := m.actionCtx.context
		rt := m.actionCtx.resourceType
		nsArg := ""
		if rt.Namespaced {
			nsArg = " -n " + ns
		}

		// Bulk path. Dispatch by pendingAction so Restart and Delete go to
		// the right command (older code blindly fell through to bulk delete
		// regardless of pendingAction, which was safe only as long as
		// Restart never reached this overlay — that changed when we
		// gated bulk Restart behind a confirm).
		if m.bulkMode && len(m.bulkItems) > 0 {
			m.clearSelection()
			expanded := expandGroupedItems(m.bulkItems)
			switch action {
			case "Restart":
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl rollout restart %s (%d items)%s --context %s", rt.Resource, len(expanded), nsArg, ctx))
				cmd := m.bulkRestartResources()
				m.resetBulkAction()
				return m, cmd
			default:
				// Default remains Delete for compatibility with existing
				// callers that opened overlayConfirm without setting
				// pendingAction.
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s (%d items)%s --context %s", rt.Resource, len(expanded), nsArg, ctx))
				cmd := m.bulkDeleteResources()
				m.resetBulkAction()
				return m, cmd
			}
		}

		if action == "Drain" {
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl drain %s --ignore-daemonsets --delete-emptydir-data --context %s", name, ctx))
			return m, m.execKubectlDrain()
		}

		switch action {
		case "Evict Replicas":
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl patch %s.longhorn.io %s --type merge -p '{\"spec\":{\"allowScheduling\":false,\"evictionRequested\":true}}'%s --context %s", rt.Resource, name, nsArg, ctx))
			return m, m.setLonghornNodeEviction(true)
		case "Cancel Eviction":
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl patch %s.longhorn.io %s --type merge -p '{\"spec\":{\"evictionRequested\":false}}'%s --context %s", rt.Resource, name, nsArg, ctx))
			return m, m.setLonghornNodeEviction(false)
		}

		// Regular delete.
		if rt.APIGroup == "_helm" {
			m.addLogEntry("DBG", fmt.Sprintf("$ helm uninstall %s -n %s --kube-context %s", name, ns, ctx))
		} else {
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s %s%s --context %s", rt.Resource, name, nsArg, ctx))
		}
		return m, m.deleteResource()
	case "n", "N", "esc", "q":
		// A cancelled taint-apply returns to the still-alive editor so
		// the staged marks are not lost.
		returnToTaints := m.pendingAction == "Apply Taints" && m.taintEditor.active
		m.overlay = overlayNone
		m.confirmAction = ""
		m.confirmTitle = ""
		m.confirmQuestion = ""
		m.pendingAction = ""
		m.blast.reset()
		m.resetBulkAction()
		if returnToTaints {
			m.overlay = overlayTaintEditor
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleConfirmTypeOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		// Cycle the cascade policy in place, skipping None: this path shells
		// out to kubectl, which cannot express it. Inert on the confirms that
		// are not cascading deletes.
		if m.forceDeleteConfirmShowsPolicy() {
			m.cycleForceDeletePropagation()
		}
		return m, nil
	case "esc", "q":
		m.overlay = overlayNone
		m.confirmAction = ""
		m.confirmTitle = ""
		m.confirmQuestion = ""
		m.pendingAction = ""
		m.confirmTypeInput.Clear()
		m.resetBulkAction()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	case "enter":
		if m.confirmTypeInput.Value == "DELETE" {
			// Read-only safety net for force-delete / finalizer-remove paths.
			if m.pendingActionBlockedByReadOnly() {
				m.overlay = overlayNone
				label := m.pendingAction
				m.pendingAction = ""
				m.confirmAction = ""
				m.confirmTitle = ""
				m.confirmQuestion = ""
				m.confirmTypeInput.Clear()
				m.resetBulkAction()
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
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete --force --grace-period=0 --cascade=%s %s (%d items)%s --context %s", m.deletePropagation().KubectlCascade(), rt.Resource, len(expanded), nsArg, ctx))
				cmd := m.bulkForceDeleteResources()
				m.resetBulkAction()
				return m, cmd
			}

			switch action {
			case "Force Finalize":
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl patch %s %s --type merge -p '{\"metadata\":{\"finalizers\":null}}'%s --context %s", rt.Resource, name, nsArg, ctx))
				return m, m.removeFinalizers()
			case "Force Delete":
				if model.IsLonghornNode(rt) {
					m.addLogEntry("DBG", fmt.Sprintf("$ kubectl patch %s.longhorn.io %s --type merge -p '{\"spec\":{\"allowScheduling\":false}}'%s; kubectl delete %s.longhorn.io %s%s --context %s", rt.Resource, name, nsArg, rt.Resource, name, nsArg, ctx))
					return m, m.forceDeleteLonghornNode()
				}
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s %s --grace-period=0 --force --cascade=%s%s --context %s", rt.Resource, name, m.deletePropagation().KubectlCascade(), nsArg, ctx))
				return m, m.forceDeleteResource()
			case "Finalizer Remove":
				m.loading = false
				m.overlay = overlayFinalizerSearch
				selectedCount := len(m.finalizerSearchSelected)
				m.addLogEntry("DBG", fmt.Sprintf("Removing finalizer %q from %d resources", m.finalizerSearchPattern, selectedCount))
				return m, m.bulkRemoveFinalizer()
			case "Disrupt":
				// Karpenter NodeClaim disrupt: kubectl delete nodeclaim.
				// Cluster-scoped, so no -n flag; rt.Resource is "nodeclaims".
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s %s --context %s", rt.Resource, name, ctx))
				return m, m.disruptNodeClaim()
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
		if msg.Text != "" {
			m.confirmTypeInput.Insert(msg.Text)
		}
		return m, nil
	}
}

func (m Model) handleScaleOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.scaleInput.Clear()
		m.blast.reset()
		m.resetBulkAction()
		return m, nil
	case "enter":
		replicas, err := strconv.ParseInt(m.scaleInput.Value, 10, 32)
		if err != nil || replicas < 0 {
			m.setStatusMessage("Invalid replica count", true)
			m.overlay = overlayNone
			m.scaleInput.Clear()
			m.blast.reset()
			m.resetBulkAction()
			return m, scheduleStatusClear()
		}
		// Belt-and-suspenders read-only gate: the dispatcher already blocks
		// "Scale" upstream, but a user who toggled RO on while this overlay
		// was open could otherwise commit a scale operation.
		if m.actionTargetBlockedByReadOnly() {
			m.overlay = overlayNone
			m.scaleInput.Clear()
			m.blast.reset()
			m.resetBulkAction()
			m.setStatusMessage(readOnlyBlockedMessage("Scale"), true)
			return m, scheduleStatusClear()
		}
		m.overlay = overlayNone
		m.loading = true
		m.scaleInput.Clear()
		m.blast.reset()

		// Bulk mode.
		if m.bulkMode && len(m.bulkItems) > 0 {
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl scale %s --replicas=%d (%d items) -n %s --context %s", strings.ToLower(m.actionCtx.kind), replicas, len(m.bulkItems), m.actionCtx.namespace, m.actionCtx.context))
			m.clearSelection()
			cmd := m.bulkScaleResources(int32(replicas))
			m.resetBulkAction()
			return m, cmd
		}

		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl scale %s %s --replicas=%d -n %s --context %s", strings.ToLower(m.actionCtx.kind), m.actionCtx.name, replicas, m.actionCtx.namespace, m.actionCtx.context))
		return m, m.scaleResource(int32(replicas))
	case "l", "+":
		stepInput(&m.scaleInput, 1, 0)
		return m, nil
	case "h", "-":
		stepInput(&m.scaleInput, -1, 0)
		return m, nil
	case "left":
		m.scaleInput.Left()
		return m, nil
	case "right":
		m.scaleInput.Right()
		return m, nil
	case "backspace":
		if len(m.scaleInput.Value) > 0 {
			m.scaleInput.Backspace()
		}
		return m, nil
	case "ctrl+w":
		m.scaleInput.DeleteWord()
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

func (m Model) handlePVCResizeOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.scaleInput.Clear()
		m.blast.reset()
		return m, nil
	case "enter":
		newSize := strings.TrimSpace(m.scaleInput.Value)
		if newSize == "" {
			m.setStatusMessage("No size specified", true)
			m.overlay = overlayNone
			m.scaleInput.Clear()
			m.blast.reset()
			return m, scheduleStatusClear()
		}
		if m.actionTargetBlockedByReadOnly() {
			m.overlay = overlayNone
			m.scaleInput.Clear()
			m.blast.reset()
			m.setStatusMessage(readOnlyBlockedMessage("Resize PVC"), true)
			return m, scheduleStatusClear()
		}
		m.overlay = overlayNone
		m.loading = true
		m.addLogEntry("DBG", fmt.Sprintf("Resizing PVC %s to %s in %s", m.actionCtx.name, newSize, m.actionNamespace()))
		m.scaleInput.Clear()
		m.blast.reset()
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

func (m Model) handlePortForwardOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.portForwardInput.Clear()
		m.pfAvailablePorts = nil
		m.pfPortCursor = -1
		m.pfOpenInBrowserAfterStart = false
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
		// A ':' in the input means the user is typing a full local:remote
		// mapping manually — that takes precedence over any list selection.
		manualMapping := strings.Contains(m.portForwardInput.Value, ":")
		switch {
		case !manualMapping && m.pfPortCursor >= 0 && m.pfPortCursor < len(m.pfAvailablePorts):
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
			if len(parts) == 2 && parts[1] == "" {
				// "8080:" has no remote port; kubectl would reject it.
				// An empty local port (":80") is fine — kubectl picks one.
				m.setStatusMessage("Port mapping needs a remote port (e.g., 8080:80)", true)
				m.overlay = overlayNone
				m.pfOpenInBrowserAfterStart = false
				return m, scheduleStatusClear()
			}
			localPort = parts[0]
			if len(parts) == 2 {
				remotePort = parts[1]
			} else {
				remotePort = localPort
			}
		default:
			m.setStatusMessage("Port mapping required (e.g., 8080:80)", true)
			m.overlay = overlayNone
			m.pfOpenInBrowserAfterStart = false
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
		}
		return m, nil
	}
}

func (m Model) handleContainerSelectOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
func (m Model) handleQuitConfirmOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		return m.beginShutdown()
	case "n", "N", "esc", "q":
		m.overlay = overlayNone
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}
