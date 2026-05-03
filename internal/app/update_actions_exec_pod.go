package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

// executeActionHelmHistory handles the read-only "History" action for HelmRelease.
// It opens the history overlay in a loading state and issues a command to
// fetch revisions. The overlay shows a loading placeholder until the fetch
// completes so the user never sees a misleading empty-state message.
func (m Model) executeActionHelmHistory() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ helm history %s -n %s --kube-context %s -o json", name, ns, ctx))
	m.overlay = overlayHelmHistory
	m.helmHistoryCursor = 0
	m.helmHistoryRevisions = nil
	m.helmRevisionsLoading = true
	return m, m.loadHelmHistory()
}

// executeActionLogs handles the "Logs" action.
func (m Model) executeActionLogs() (tea.Model, tea.Cmd) {
	return m.executeActionLogsWithTail("Logs", ui.ConfigLogTailLines)
}

// executeActionTailLogs handles the "Tail Logs" action, loading only the short
// tail count (ConfigLogTailLinesShort) for a lightweight quick peek.
func (m Model) executeActionTailLogs() (tea.Model, tea.Cmd) {
	return m.executeActionLogsWithTail("Tail Logs", ui.ConfigLogTailLinesShort)
}

// executeActionLogsWithTail is the shared implementation for both Logs and Tail
// Logs. tailLines controls the --tail value; pendingLabel is stored in
// m.pendingAction so the pod/container-selection overlays can continue with the
// correct action label.
func (m Model) executeActionLogsWithTail(pendingLabel string, tailLines int) (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kind := m.actionCtx.kind
	isGroupResource := kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" ||
		kind == "Job" || kind == "CronJob" || kind == "Service"

	if isGroupResource && m.actionCtx.containerName == "" {
		// Save parent resource context for pod/container re-selection from the log viewer.
		m.logParentKind = m.actionCtx.kind
		m.logParentName = m.actionCtx.name
		// Stream all pods at once using label selector (no pod selection step).
		// The user can still filter pods/containers from the log viewer overlay.
	}

	if kind != "Pod" && !isGroupResource && m.actionCtx.containerName == "" {
		m.pendingAction = pendingLabel
		return m, m.loadContainersForAction()
	}

	// Direct log streaming for pods or when container is already selected.
	// Reset parent context only for non-group resources so stale values
	// from a previous session don't leak. Group resources keep their
	// parent context for the pod/container re-selection overlay.
	if !isGroupResource {
		m.logParentKind = ""
		m.logParentName = ""
	}

	kubectlCtx := m.kubectlContext(ctx)
	if m.actionCtx.containerName != "" {
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl logs -f %s -c %s -n %s --context %s", name, m.actionCtx.containerName, ns, kubectlCtx))
	} else {
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl logs -f %s --all-containers --prefix -n %s --context %s", name, ns, kubectlCtx))
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
	m.logContainers = nil
	// For single-container logs, pre-select that container so the
	// container selector overlay shows the correct active state.
	if m.actionCtx.containerName != "" {
		m.logSelectedContainers = []string{m.actionCtx.containerName}
	} else {
		m.logSelectedContainers = nil
	}
	m.logTailLines = tailLines
	m.logHasMoreHistory = true
	m.logLoadingHistory = false
	m.logCursor = 0 // will track end as lines stream in with follow mode
	m.logVisualMode = false
	m.logVisualStart = 0
	isTail := pendingLabel == "Tail Logs"
	if m.actionCtx.containerName != "" {
		if isTail {
			m.logTitle = fmt.Sprintf("Logs (tail): %s/%s [%s]", m.actionNamespace(), m.actionCtx.name, m.actionCtx.containerName)
		} else {
			m.logTitle = fmt.Sprintf("Logs: %s/%s [%s]", m.actionNamespace(), m.actionCtx.name, m.actionCtx.containerName)
		}
	} else {
		if isTail {
			m.logTitle = fmt.Sprintf("Logs (tail): %s/%s", m.actionNamespace(), m.actionCtx.name)
		} else {
			m.logTitle = fmt.Sprintf("Logs: %s/%s", m.actionNamespace(), m.actionCtx.name)
		}
	}
	return m, m.startLogStream()
}

// executeActionExec handles the "Exec" action.
func (m Model) executeActionExec() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kind := m.actionCtx.kind
	isParentExec := kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" ||
		kind == "Job" || kind == "CronJob" || kind == "Service"
	if isParentExec {
		m.pendingAction = "Exec"
		m.loading = true
		m.setStatusMessage("Loading pods...", false)
		return m, m.loadPodsForAction()
	}
	if m.actionCtx.containerName == "" {
		m.pendingAction = "Exec"
		m.loading = true
		m.setStatusMessage("Loading containers...", false)
		return m, m.loadContainersForAction()
	}
	cArg := ""
	if m.actionCtx.containerName != "" {
		cArg = " -c " + m.actionCtx.containerName
	}
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl exec -it %s%s -n %s --context %s -- /bin/sh -c 'clear; command -v bash >/dev/null && exec bash || { command -v ash >/dev/null && exec ash || exec sh; }'", name, cArg, ns, ctx))
	return m, m.execKubectlExec()
}

// executeActionAttach handles the "Attach" action.
func (m Model) executeActionAttach() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kind := m.actionCtx.kind
	isParentAttach := kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" ||
		kind == "Job" || kind == "CronJob" || kind == "Service"
	if isParentAttach {
		m.pendingAction = "Attach"
		m.loading = true
		m.setStatusMessage("Loading pods...", false)
		return m, m.loadPodsForAction()
	}
	if m.actionCtx.containerName == "" {
		m.pendingAction = "Attach"
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
}

// executeActionDescribe handles the "Describe" action.
func (m Model) executeActionDescribe() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	rt := m.actionCtx.resourceType
	nsArg := ""
	if rt.Namespaced {
		nsArg = " -n " + ns
	}
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl describe %s %s%s --context %s", rt.Resource, name, nsArg, ctx))
	return m, m.execKubectlDescribe()
}

// executeActionEdit handles the "Edit" action.
func (m Model) executeActionEdit() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	rt := m.actionCtx.resourceType
	nsArg := ""
	if rt.Namespaced {
		nsArg = " -n " + ns
	}
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl edit %s %s%s --context %s", rt.Resource, name, nsArg, ctx))
	return m, m.execKubectlEdit()
}

// executeActionDelete handles the "Delete" action.
func (m Model) executeActionDelete() (tea.Model, tea.Cmd) { //nolint:unparam // consistent action handler signature
	m.confirmAction = m.actionCtx.name
	m.overlay = overlayConfirm
	m.pendingAction = "Delete"
	return m, nil
}

// executeActionResize handles the "Resize" action.
func (m Model) executeActionResize() (tea.Model, tea.Cmd) { //nolint:unparam // consistent action handler signature
	// Extract current PVC size from columns for display in the overlay.
	m.pvcCurrentSize = ""
	for _, kv := range m.actionCtx.columns {
		if kv.Key == "Capacity" || kv.Key == "CAPACITY" {
			m.pvcCurrentSize = kv.Value
			break
		}
	}
	m.scaleInput.Clear()
	m.overlay = overlayPVCResize
	return m, nil
}

// executeActionRestart handles the "Restart" action.
func (m Model) executeActionRestart() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	// Restart a stopped/failed port forward entry.
	if m.actionCtx.kind == "__port_forward_entry__" || m.actionCtx.kind == "__port_forwards__" {
		pfID := m.getPortForwardID(m.actionCtx.columns)
		if pfID > 0 {
			m.setStatusMessage("Restarting port forward...", false)
			return m, m.restartPortForward(pfID)
		}
		return m, nil
	}
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl rollout restart %s %s -n %s --context %s", strings.ToLower(m.actionCtx.kind), name, ns, ctx))
	m.loading = true
	return m, m.restartResource()
}

// executeActionRollback handles the "Rollback" action. For HelmRelease it
// opens the rollback overlay optimistically in a loading state so the user
// gets immediate feedback while the helm history subprocess runs in the
// background. For other kinds the overlay is opened by the message handler
// when the deployment revisions arrive.
func (m Model) executeActionRollback() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	if m.actionCtx.kind == "HelmRelease" {
		m.addLogEntry("DBG", fmt.Sprintf("$ helm history %s -n %s --kube-context %s -o json", name, ns, ctx))
		m.overlay = overlayHelmRollback
		m.helmRollbackCursor = 0
		m.helmRollbackRevisions = nil
		m.helmRevisionsLoading = true
		return m, m.loadHelmRevisions()
	}
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl rollout undo deployment %s -n %s --context %s", name, ns, ctx))
	return m, m.loadRevisions()
}

// executeActionPortForward handles the "Port Forward" action.
func (m Model) executeActionPortForward() (tea.Model, tea.Cmd) {
	m.portForwardInput.Clear()
	m.pfAvailablePorts = nil
	m.pfPortCursor = -1
	m.loading = true
	m.setStatusMessage("Loading ports...", false)
	return m, m.loadContainerPorts()
}

// executeActionDebug handles the "Debug" action.
func (m Model) executeActionDebug() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl debug %s -it --image=busybox -n %s --context %s", name, ns, ctx))
	return m, m.execKubectlDebug()
}

// executeActionEvents handles the "Events" action.
func (m Model) executeActionEvents() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	m.loading = true
	m.setStatusMessage("Loading events...", false)
	m.addLogEntry("DBG", fmt.Sprintf("Loading event timeline for %s/%s in %s", m.actionCtx.kind, name, ns))
	return m, m.loadEventTimeline()
}

// executeActionArgo handles Argo-related actions (sync, refresh, workflows, etc.).
func (m Model) executeActionArgo(actionLabel string) (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	rt := m.actionCtx.resourceType

	switch actionLabel {
	case "Configure AutoSync":
		m.addLogEntry("DBG", fmt.Sprintf("Loading autosync config for %s/%s in %s", ns, name, ctx))
		return m, m.loadAutoSyncConfig()
	case "Sync":
		m.addLogEntry("DBG", fmt.Sprintf("Sync (hook strategy) %s/%s in %s", ns, name, ctx))
		m.loading = true
		return m, m.syncArgoApp(false)
	case "Sync (Apply Only)":
		m.addLogEntry("DBG", fmt.Sprintf("Sync (apply strategy) %s/%s in %s", ns, name, ctx))
		m.loading = true
		return m, m.syncArgoApp(true)
	case "Refresh":
		m.addLogEntry("DBG", fmt.Sprintf("Hard refresh %s (%s) %s/%s in %s", m.actionCtx.kind, rt.Resource, ns, name, ctx))
		m.loading = true
		if m.actionCtx.kind == "ApplicationSet" || rt.Resource == "applicationsets" {
			return m, m.refreshArgoAppSet()
		}
		return m, m.refreshArgoApp()
	case "Terminate Sync":
		m.addLogEntry("DBG", fmt.Sprintf("Terminate sync for %s/%s in %s", ns, name, ctx))
		m.loading = true
		return m, m.terminateArgoSync()
	case "Watch Workflow":
		m.addLogEntry("DBG", fmt.Sprintf("Watching workflow %s in %s", name, ns))
		m.loading = true
		m.describeAutoRefresh = true
		m.describeRefreshFunc = func() tea.Cmd { return m.watchArgoWorkflow() }
		return m, m.watchArgoWorkflow()
	case "Suspend Workflow":
		m.addLogEntry("DBG", fmt.Sprintf("Suspending workflow %s in %s", name, ns))
		m.loading = true
		return m, m.suspendArgoWorkflow()
	case "Resume Workflow":
		m.addLogEntry("DBG", fmt.Sprintf("Resuming workflow %s in %s", name, ns))
		m.loading = true
		return m, m.resumeArgoWorkflow()
	case "Stop Workflow":
		m.addLogEntry("DBG", fmt.Sprintf("Stopping workflow %s in %s", name, ns))
		m.loading = true
		return m, m.stopArgoWorkflow()
	case "Terminate Workflow":
		m.addLogEntry("DBG", fmt.Sprintf("Terminating workflow %s in %s", name, ns))
		m.loading = true
		return m, m.terminateArgoWorkflow()
	case "Resubmit Workflow":
		m.addLogEntry("DBG", fmt.Sprintf("Resubmitting workflow %s in %s", name, ns))
		m.loading = true
		return m, m.resubmitArgoWorkflow()
	case "Submit Workflow":
		clusterScope := m.actionCtx.kind == "ClusterWorkflowTemplate"
		m.addLogEntry("DBG", fmt.Sprintf("Submitting workflow from template %s in %s", name, ns))
		m.loading = true
		return m, m.submitWorkflowFromTemplate(clusterScope)
	case "Suspend CronWorkflow":
		m.addLogEntry("DBG", fmt.Sprintf("Suspending cron workflow %s in %s", name, ns))
		m.loading = true
		return m, m.suspendCronWorkflow()
	case "Resume CronWorkflow":
		m.addLogEntry("DBG", fmt.Sprintf("Resuming cron workflow %s in %s", name, ns))
		m.loading = true
		return m, m.resumeCronWorkflow()
	}
	return m, nil
}

// executeActionSimpleLoading handles actions that log, set loading, and call a command.
func (m Model) executeActionSimpleLoading(verb string, cmdFn func() tea.Cmd) (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	m.addLogEntry("DBG", fmt.Sprintf("%s %s/%s in %s", verb, m.actionCtx.kind, name, ns))
	m.loading = true
	return m, cmdFn()
}

// executeActionForceDelete handles the "Force Delete" action from the action
// menu. Mirrors directActionForceDelete and the bulk Force Delete path: opens
// a typed-confirmation overlay (user must type DELETE + Enter) so a stray
// x->X cannot nuke a pod without explicit acknowledgement (#89).
func (m Model) executeActionForceDelete() (tea.Model, tea.Cmd) { //nolint:unparam // consistent action handler signature
	m.confirmAction = m.actionCtx.name + " (FORCE)"
	m.confirmTitle = "Confirm Force Delete"
	m.confirmQuestion = fmt.Sprintf("Force delete %s?", m.actionCtx.name)
	m.confirmTypeInput.Clear()
	m.overlay = overlayConfirmType
	m.pendingAction = "Force Delete"
	return m, nil
}

// executeActionForceFinalize handles the "Force Finalize" action.
func (m Model) executeActionForceFinalize() (tea.Model, tea.Cmd) { //nolint:unparam // consistent action handler signature
	m.confirmAction = m.actionCtx.name
	m.confirmTitle = "Confirm Force Finalize"
	m.confirmQuestion = fmt.Sprintf("Remove all finalizers from %s?", m.actionCtx.name)
	m.confirmTypeInput.Clear()
	m.overlay = overlayConfirmType
	m.pendingAction = "Force Finalize"
	return m, nil
}

// executeActionCordon handles the "Cordon" action.
func (m Model) executeActionCordon() (tea.Model, tea.Cmd) {
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl cordon %s --context %s", name, ctx))
	m.loading = true
	return m, m.execKubectlCordon()
}

// executeActionUncordon handles the "Uncordon" action.
func (m Model) executeActionUncordon() (tea.Model, tea.Cmd) {
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl uncordon %s --context %s", name, ctx))
	m.loading = true
	return m, m.execKubectlUncordon()
}

// executeActionDrain handles the "Drain" action.
func (m Model) executeActionDrain() (tea.Model, tea.Cmd) { //nolint:unparam // consistent action handler signature
	m.confirmAction = m.actionCtx.name + " (drain)"
	m.pendingAction = "Drain"
	m.overlay = overlayConfirm
	return m, nil
}

// executeActionTaint handles the "Taint" action. The bare "taint" subcommand
// does not classify as cmdKubectl, so the pre-fill must include the
// "kubectl" prefix to reach executeKubectlCommand on submit.
func (m Model) executeActionTaint() (tea.Model, tea.Cmd) { //nolint:unparam // consistent action handler signature
	name := m.actionCtx.name
	m.commandBarActive = true
	m.commandBarInput.Clear()
	m.commandBarInput.Insert("kubectl taint node " + name + " ")
	m.commandBarSuggestions = nil
	m.commandBarSelectedSuggestion = 0
	return m, nil
}

// executeActionUntaint handles the "Untaint" action.
func (m Model) executeActionUntaint() (tea.Model, tea.Cmd) { //nolint:unparam // consistent action handler signature
	name := m.actionCtx.name
	// Pre-fill with existing taint keys for convenient removal. The
	// "kubectl" prefix is required so the command classifies as cmdKubectl.
	var prefill strings.Builder
	prefill.WriteString("kubectl taint node " + name + " ")
	for _, col := range m.actionCtx.columns {
		if col.Key == "Taints" && col.Value != "" {
			// Parse taint strings and append removal syntax (key-).
			parts := strings.Split(col.Value, ", ")
			for i, p := range parts {
				// Extract just the key from key=value:effect or key:effect.
				taintKey := strings.SplitN(p, "=", 2)[0]
				taintKey = strings.SplitN(taintKey, ":", 2)[0]
				if i > 0 {
					prefill.WriteString(" ")
				}
				prefill.WriteString(taintKey + "-")
			}
			break
		}
	}
	m.commandBarActive = true
	m.commandBarInput.Clear()
	m.commandBarInput.Insert(prefill.String())
	m.commandBarSuggestions = nil
	m.commandBarSelectedSuggestion = 0
	return m, nil
}

// executeActionTrigger handles the "Trigger" action.
func (m Model) executeActionTrigger() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl create job --from=cronjob/%s manual-trigger -n %s --context %s", name, ns, ctx))
	m.loading = true
	return m, m.triggerCronJob()
}
