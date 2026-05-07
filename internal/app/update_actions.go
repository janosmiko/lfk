package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) openActionMenu() Model {
	if m.hasSelection() {
		return m.openBulkSelectionMenu()
	}
	if m.nav.Level == model.LevelClusters {
		return m.openClusterPickerActionMenu()
	}
	return m.openResourceActionMenu()
}

func (m Model) openResourceActionMenu() Model {
	kind := m.selectedResourceKind()
	if kind == "" {
		return m
	}

	sel := m.selectedMiddleItem()
	if sel == nil {
		return m
	}

	m.bulkMode = false
	m.actionCtx = m.buildActionCtx(sel, kind)

	var actions []model.ActionMenuItem
	switch {
	case kind == "__port_forwards__" || kind == "__port_forward_entry__":
		actions = model.ActionsForPortForward()
	case kind == "__captures__":
		actions = model.ActionsForCapture()
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
	targetReadOnly := m.readOnlyForContext(m.actionCtx.context)
	for _, a := range actions {
		// Use the kind-aware variant so custom actions are filtered based
		// on their ReadOnlySafe opt-in (defaults to false / mutating).
		if targetReadOnly && isMutatingActionForKind(kind, a.Label) {
			continue
		}
		if m.isUnionSentinel() && !isUnionAllowedAction(a.Label) {
			continue
		}
		items = append(items, model.Item{
			Name:   a.Label,
			Extra:  a.Description,
			Status: a.Key,
		})
	}

	// If the resource is being deleted, escalate the Delete action.
	if sel.Deleting {
		for i, item := range items {
			if item.Name == "Delete" {
				if model.IsForceDeleteableKind(kind) {
					items[i].Name = "Force Delete"
					items[i].Extra = "Force delete this " + strings.ToLower(kind)
				} else {
					items[i].Name = "Force Finalize"
					items[i].Extra = "Remove finalizers to force finalize"
				}
				break
			}
		}
	}

	m.overlay = overlayAction
	m.overlayItems = items
	m.overlayCursor = 0
	return m
}

// buildActionCtx creates an actionContext from the current selection, extracting
// the common logic shared between openActionMenu and direct action keybindings.
func (m *Model) buildActionCtx(sel *model.Item, kind string) actionContext {
	kctx := m.nav.Context
	if m.unionMode && sel.ClusterName != "" {
		kctx = sel.ClusterName
	}
	ctx := actionContext{
		kind:    kind,
		name:    sel.Name,
		context: kctx,
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
		ctx.image = sel.Extra
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
	if kind == "" || kind == "__port_forwards__" || kind == "__captures__" {
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
	m.invalidateOrphanCacheForNamespace(m.nav.Context, m.namespace)
	m.cancelAndReset()
	m.requestGen++
	m.setStatusMessage("Refreshing...", false)
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) directActionEdit() (tea.Model, tea.Cmd) {
	kind := m.selectedResourceKind()
	if kind == "" || kind == "__port_forwards__" || kind == "__captures__" {
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
	if kind == "" || kind == "__port_forwards__" || kind == "__captures__" {
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
	// Containers can't be deleted in Kubernetes — only the parent pod can.
	// Refuse the keypress here so the user isn't tricked into deleting the
	// parent pod from a view whose action menu (ActionsForContainer) doesn't
	// even list Delete. Covers both single and bulk-selection paths.
	if m.nav.Level == model.LevelContainers {
		m.setStatusMessage("Delete not available for containers", true)
		return m, scheduleStatusClear()
	}
	if m.hasSelection() {
		return m.openBulkActionDirect("Delete")
	}
	kind := m.selectedResourceKind()
	if kind == "" || kind == "__port_forwards__" || kind == "__captures__" {
		return m, nil
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	m.actionCtx = m.buildActionCtx(sel, kind)
	// If resource is already deleting, escalate the action.
	if sel.Deleting {
		m.confirmTypeInput.Clear()
		m.overlay = overlayConfirmType
		if model.IsForceDeleteableKind(kind) {
			// Pod/Job: offer force delete.
			m.confirmAction = sel.Name + " (FORCE)"
			m.confirmTitle = "Confirm Force Delete"
			m.confirmQuestion = fmt.Sprintf("Force delete %s?", sel.Name)
			m.pendingAction = "Force Delete"
		} else {
			// Other kinds: offer force finalize (remove finalizers).
			m.confirmAction = sel.Name
			m.confirmTitle = "Confirm Force Finalize"
			m.confirmQuestion = fmt.Sprintf("Remove all finalizers from %s?", sel.Name)
			m.pendingAction = "Force Finalize"
		}
		return m, nil
	}
	return m.executeAction("Delete")
}

func (m Model) directActionForceDelete() (tea.Model, tea.Cmd) {
	if m.hasSelection() {
		return m.openBulkActionDirect("Force Delete")
	}
	kind := m.selectedResourceKind()
	if kind == "" || kind == "__port_forwards__" || kind == "__captures__" {
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
	m.confirmTitle = "Confirm Force Delete"
	m.confirmQuestion = fmt.Sprintf("Force delete %s?", sel.Name)
	m.confirmTypeInput.Clear()
	m.overlay = overlayConfirmType
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

	// Cluster-picker actions live outside the kind-based machinery: they
	// don't have an actionCtx and there is no resource type at this level.
	// Dispatch them by label and short-circuit before the read-only check
	// fires on a label that has no kind to consult.
	if m.nav.Level == model.LevelClusters {
		switch actionLabel {
		case model.ActionLabelSetColor:
			return m.handleKeyClusterColorPicker()
		case model.ActionLabelManageLocalClusters:
			return m.openLocalClusterManager()
		}
	}

	// Handle bulk actions.
	if m.bulkMode && len(m.bulkItems) > 0 {
		return m.executeBulkAction(actionLabel)
	}

	if isMutatingAction(actionLabel) && m.readOnlyForContext(m.actionCtx.context) {
		logger.Info("Blocked by read-only mode", "action", actionLabel, "context", m.actionCtx.context)
		m.setStatusMessage(readOnlyBlockedMessage(actionLabel), true)
		return m, scheduleStatusClear()
	}

	// Defense-in-depth: even if a label slips through openActionMenu's
	// allowlist (a custom keybinding or future action handler that bypasses
	// the menu), refuse mutating actions outside the union allowlist at the
	// sentinel. The menu filter is the primary surface; this is the backstop.
	if m.isUnionSentinel() && !isUnionAllowedAction(actionLabel) {
		logger.Info("Blocked by union view", "action", actionLabel)
		m.setStatusMessage(actionLabel+" is not available in union view", true)
		return m, scheduleStatusClear()
	}

	logger.Info("Executing action",
		"action", actionLabel,
		"kind", m.actionCtx.kind,
		"name", m.actionCtx.name,
		"namespace", m.actionCtx.namespace,
		"context", m.actionCtx.context,
	)

	if mdl, cmd, ok := m.executeActionCore(actionLabel); ok {
		return mdl, cmd
	}
	if mdl, cmd, ok := m.executeActionExtended(actionLabel); ok {
		return mdl, cmd
	}
	return m.executeActionDefault(actionLabel)
}

// executeActionCore dispatches core kubectl-related actions.
// Returns the model, cmd, and true if the action was handled.
func (m Model) executeActionCore(actionLabel string) (tea.Model, tea.Cmd, bool) {
	if mdl, cmd, ok := m.executeActionCoreK8s(actionLabel); ok {
		return mdl, cmd, true
	}
	return m.executeActionCoreOps(actionLabel)
}

// executeActionCoreK8s dispatches core kubectl resource actions.
func (m Model) executeActionCoreK8s(actionLabel string) (tea.Model, tea.Cmd, bool) {
	switch actionLabel {
	case "Logs":
		mdl, cmd := m.executeActionLogs()
		return mdl, cmd, true
	case "Tail Logs":
		mdl, cmd := m.executeActionTailLogs()
		return mdl, cmd, true
	case "Exec":
		mdl, cmd := m.executeActionExec()
		return mdl, cmd, true
	case "Attach":
		mdl, cmd := m.executeActionAttach()
		return mdl, cmd, true
	case "Describe":
		mdl, cmd := m.executeActionDescribe()
		return mdl, cmd, true
	case "Edit":
		mdl, cmd := m.executeActionEdit()
		return mdl, cmd, true
	case "Secret Editor":
		return m, m.loadSecretData(), true
	case "ConfigMap Editor":
		return m, m.loadConfigMapData(), true
	case "Right-sizing":
		mdl, cmd := m.executeActionRightsizing()
		return mdl, cmd, true
	case "Delete":
		mdl, cmd := m.executeActionDelete()
		return mdl, cmd, true
	case "Resize":
		mdl, cmd := m.executeActionResize()
		return mdl, cmd, true
	case "Scale":
		mdl := m.executeActionScale()
		return mdl, nil, true
	case "Restart":
		mdl, cmd := m.executeActionRestart()
		return mdl, cmd, true
	case "Rollback":
		mdl, cmd := m.executeActionRollback()
		return mdl, cmd, true
	case "Port Forward":
		mdl, cmd := m.executeActionPortForward()
		return mdl, cmd, true
	case "Debug":
		mdl, cmd := m.executeActionDebug()
		return mdl, cmd, true
	case "Events":
		mdl, cmd := m.executeActionEvents()
		return mdl, cmd, true
	}
	return m, nil, false
}

// executeActionCoreOps dispatches node, PVC, and other operational actions.
func (m Model) executeActionCoreOps(actionLabel string) (tea.Model, tea.Cmd, bool) {
	// Capture-related actions are extracted to keep this switch under the
	// gocyclo cap (>30 fails CI). The helper returns ok=true only when it
	// actually handled the action, so non-capture labels fall through to
	// the switch below.
	if mdl, cmd, ok := m.executeActionCapture(actionLabel); ok {
		return mdl, cmd, true
	}
	switch actionLabel {
	case "Force Delete":
		mdl, cmd := m.executeActionForceDelete()
		return mdl, cmd, true
	case "Force Finalize":
		mdl, cmd := m.executeActionForceFinalize()
		return mdl, cmd, true
	case "Cordon":
		mdl, cmd := m.executeActionCordon()
		return mdl, cmd, true
	case "Uncordon":
		mdl, cmd := m.executeActionUncordon()
		return mdl, cmd, true
	case "Drain":
		mdl, cmd := m.executeActionDrain()
		return mdl, cmd, true
	case "Taint":
		mdl, cmd := m.executeActionTaint()
		return mdl, cmd, true
	case "Untaint":
		mdl, cmd := m.executeActionUntaint()
		return mdl, cmd, true
	case "Trigger":
		mdl, cmd := m.executeActionTrigger()
		return mdl, cmd, true
	case "Shell":
		mdl, cmd := m.executeActionShell()
		return mdl, cmd, true
	case "Debug Pod":
		mdl, cmd := m.executeActionDebugPod()
		return mdl, cmd, true
	case "Go to Pod":
		mdl, cmd := m.executeActionGoToPod()
		return mdl, cmd, true
	case "Debug Mount":
		mdl, cmd := m.executeActionDebugMount()
		return mdl, cmd, true
	case "Open in Browser":
		mdl, cmd := m.executeActionOpenInBrowser()
		return mdl, cmd, true
	case "Stop":
		mdl, cmd := m.executeActionStop()
		return mdl, cmd, true
	case "Remove":
		mdl, cmd := m.executeActionRemove()
		return mdl, cmd, true
	case "Permissions":
		mdl, cmd := m.executeActionPermissions()
		return mdl, cmd, true
	case "Startup Analysis":
		mdl, cmd := m.executeActionStartupAnalysis()
		return mdl, cmd, true
	case "Crash Investigator":
		mdl, cmd := m.executeActionCrashInvestigator()
		return mdl, cmd, true
	case "Sync Wave Timeline":
		return m.dispatchActionSyncWaveTimeline()
	case "Alerts":
		mdl, cmd := m.executeActionAlerts()
		return mdl, cmd, true
	case "Visualize":
		mdl, cmd := m.executeActionVisualize()
		return mdl, cmd, true
	case "Labels / Annotations":
		mdl, cmd := m.executeActionLabelsAnnotations()
		return mdl, cmd, true
	case "Vuln Scan":
		mdl, cmd := m.executeActionVulnScan()
		return mdl, cmd, true
	}
	return m, nil, false
}

// executeActionCapture dispatches the traffic-capture action set: opening
// the capture overlay from a Pod / Service action menu, and the row
// actions on the __captures__ pseudo-resource. Returns ok=true only when
// the label was actually handled, so executeActionCoreOps's caller-side
// `if ok { return }` pattern leaves unrelated labels to fall through to
// the main switch.
func (m Model) executeActionCapture(actionLabel string) (tea.Model, tea.Cmd, bool) {
	switch actionLabel {
	case "Capture Traffic":
		mdl, cmd := m.executeActionCaptureTraffic()
		return mdl, cmd, true
	case "Open":
		if m.actionCtx.kind == "__captures__" {
			if sel := m.selectedMiddleItem(); sel != nil {
				mdl, cmd := m.openCaptureFromPseudo(*sel)
				return mdl, cmd, true
			}
		}
	case "Delete File":
		if m.actionCtx.kind == "__captures__" {
			if sel := m.selectedMiddleItem(); sel != nil {
				mdl, cmd := m.deleteCaptureFile(*sel)
				return mdl, cmd, true
			}
		}
	}
	return m, nil, false
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
	if isMutatingAction(actionLabel) {
		if ctx, ok := m.bulkReadOnlyContext(); ok {
			logger.Info("Blocked by read-only mode (bulk)", "action", actionLabel, "count", len(m.bulkItems), "context", ctx)
			m.setStatusMessage(readOnlyBlockedMessage(actionLabel), true)
			return m, scheduleStatusClear()
		}
	}
	if m.isUnionSentinel() && !isUnionAllowedAction(actionLabel) {
		logger.Info("Blocked by union view (bulk)", "action", actionLabel, "count", len(m.bulkItems))
		m.setStatusMessage(actionLabel+" is not available in union view", true)
		return m, scheduleStatusClear()
	}

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
		m.confirmTitle = "Confirm Force Delete"
		m.confirmQuestion = fmt.Sprintf("Force delete %d resources?", len(m.bulkItems))
		m.confirmTypeInput.Clear()
		m.overlay = overlayConfirmType
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
	case "Sync":
		m.addLogEntry("DBG", fmt.Sprintf("Bulk sync (%d apps, hook strategy)", len(m.bulkItems)))
		m.loading = true
		m.clearSelection()
		return m, m.bulkSyncArgoApps(false)
	case "Sync (Apply Only)":
		m.addLogEntry("DBG", fmt.Sprintf("Bulk sync (%d apps, apply strategy)", len(m.bulkItems)))
		m.loading = true
		m.clearSelection()
		return m, m.bulkSyncArgoApps(true)
	case "Refresh":
		m.addLogEntry("DBG", fmt.Sprintf("Bulk refresh (%d apps)", len(m.bulkItems)))
		m.loading = true
		m.clearSelection()
		return m, m.bulkRefreshArgoApps()
	}

	return m, nil
}
