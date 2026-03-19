package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

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
	switch {
	case kind == "__port_forwards__" || kind == "__port_forward_entry__":
		actions = model.ActionsForPortForward()
	case m.nav.Level == model.LevelContainers:
		actions = model.ActionsForContainer()
	default:
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

	items := make([]model.Item, 0, len(actions))
	for _, a := range actions {
		items = append(items, model.Item{
			Name:   a.Label,
			Extra:  a.Description,
			Status: a.Key,
		})
	}

	// If the resource is being deleted, replace "Delete" with "Force Finalize".
	if sel.Deleting {
		for i, item := range items {
			if item.Name == "Delete" {
				items[i].Name = "Force Finalize"
				items[i].Extra = "Remove finalizers to force finalize"
				break
			}
		}
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
	// Priority: item namespace > navigation namespace > selector namespace.
	switch {
	case sel.Namespace != "":
		ctx.namespace = sel.Namespace
	case m.nav.Namespace != "":
		ctx.namespace = m.nav.Namespace
	default:
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

func (m Model) directActionEdit() (tea.Model, tea.Cmd) {
	kind := m.selectedResourceKind()
	if kind == "" || kind == "__port_forwards__" {
		return m, nil
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	m.actionCtx = m.buildActionCtx(sel, kind)
	return m.executeAction("Edit")
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
	// If resource is already deleting, offer Force Finalize instead.
	if sel.Deleting {
		m.confirmAction = sel.Name
		m.confirmTypeInput.Clear()
		m.overlay = overlayConfirmType
		m.pendingAction = "Force Finalize"
		return m, nil
	}
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
	if !model.IsForceDeleteableKind(kind) {
		m.setStatusMessage("Force delete not available for "+kind, true)
		return m, scheduleStatusClear()
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
		m.logTimestamps = false
		m.logPrevious = false
		m.logIsMulti = false
		m.logMultiItems = nil
		m.logSelectedContainers = nil
		m.logContainers = nil
		m.logTailLines = ui.ConfigLogTailLines
		m.logHasMoreHistory = true
		m.logLoadingHistory = false
		m.logCursor = 0 // will track end as lines stream in with follow mode
		m.logVisualMode = false
		m.logVisualStart = 0
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
	case "Secret Editor":
		return m, m.loadSecretData()
	case "ConfigMap Editor":
		return m, m.loadConfigMapData()
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
	case "Sync":
		m.addLogEntry("DBG", fmt.Sprintf("Sync (hook strategy) %s/%s in %s", ns, name, ctx))
		m.loading = true
		return m, m.syncArgoApp(false)
	case "Sync (Apply Only)":
		m.addLogEntry("DBG", fmt.Sprintf("Sync (apply strategy) %s/%s in %s", ns, name, ctx))
		m.loading = true
		return m, m.syncArgoApp(true)
	case "Refresh":
		m.addLogEntry("DBG", fmt.Sprintf("Hard refresh %s/%s in %s", ns, name, ctx))
		m.loading = true
		return m, m.refreshArgoApp()
	case "Terminate Sync":
		m.addLogEntry("DBG", fmt.Sprintf("Terminate sync for %s/%s in %s", ns, name, ctx))
		m.loading = true
		return m, m.terminateArgoSync()
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
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s %s --grace-period=0 --force%s --context %s", rt.Resource, name, nsArg, ctx))
		m.loading = true
		return m, m.forceDeleteResource()
	case "Force Finalize":
		m.confirmAction = m.actionCtx.name
		m.confirmTypeInput.Clear()
		m.overlay = overlayConfirmType
		m.pendingAction = "Force Finalize"
		return m, nil
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
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl run lfk-debug-<id> --image=alpine --rm -it --restart=Never -n %s --context %s -- sh", ns, ctx))
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

// closeTabOrQuit closes the current tab if multiple tabs are open,
// otherwise quits the application (with optional confirmation).
func (m Model) closeTabOrQuit() (tea.Model, tea.Cmd) {
	if len(m.tabs) > 1 {
		m.tabs = append(m.tabs[:m.activeTab], m.tabs[m.activeTab+1:]...)
		if m.activeTab >= len(m.tabs) {
			m.activeTab = len(m.tabs) - 1
		}
		m.saveCurrentSession()
		if cmd := m.loadTab(m.activeTab); cmd != nil {
			return m, cmd
		}
		return m, m.loadPreview()
	}
	// On last tab, show confirmation if configured.
	if ui.ConfigConfirmOnExit {
		m.overlay = overlayQuitConfirm
		return m, nil
	}
	// Stop all active port forwards before quitting.
	if m.portForwardMgr != nil {
		m.portForwardMgr.StopAll()
	}
	m.saveCurrentSession()
	return m, tea.Quit
}
