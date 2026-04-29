package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// isSecurityActionEligibleKind reports whether a given resource kind is a
// reasonable target for the "Security Findings" action. The security
// dashboard indexes findings by (namespace, kind, name), so only kinds that
// security sources actually emit findings for are worth offering. Cluster
// menu virtual kinds (e.g. "__port_forwards__") and empty kinds are excluded.
func isSecurityActionEligibleKind(kind string) bool {
	if kind == "" {
		return false
	}
	if strings.HasPrefix(kind, "__") {
		return false
	}
	return true
}

func (m Model) openActionMenu() Model {
	// Security findings get a tailored action menu instead of the k8s one.
	if strings.HasPrefix(m.nav.ResourceType.Kind, "__security_") {
		return m.openSecurityActionMenu()
	}

	// Bulk mode: when items are selected, show bulk action menu.
	if m.hasSelection() {
		selectedList := m.selectedItemsList()
		if len(selectedList) == 0 {
			return m
		}
		m.bulkMode = true
		m.bulkItems = selectedList

		// Build action context from first selected item (for resource type info).
		kind := m.selectedResourceKind()
		if kind == "" {
			return m
		}
		m.actionCtx = m.buildActionCtx(&selectedList[0], kind)

		actions := model.ActionsForBulk(kind)
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
		return m
	}

	kind := m.selectedResourceKind()
	if kind == "" {
		return m
	}

	sel := m.selectedMiddleItem()
	if sel == nil {
		return m
	}

	// Security findings and groups get a tailored action menu.
	if strings.HasPrefix(sel.Kind, "__security_") {
		return m.openSecurityActionMenu()
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

	// Append "Security Findings" when security sources are available for
	// the current cluster. The action will be re-wired in Phase E of the
	// security navigation revamp to jump to the Security category.
	if m.securityAvailableAny() && isSecurityActionEligibleKind(kind) {
		actions = append(actions, model.ActionMenuItem{
			Label:       "Security Findings",
			Description: "Show security findings for this resource",
			Key:         "y",
		})
	}

	items := make([]model.Item, 0, len(actions))
	for _, a := range actions {
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
					items[i].Extra = "Force delete (--force --grace-period=0)"
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
	// On security source views, invalidate the Manager's fetch +
	// availability caches so the subsequent loadResources does a real
	// fetch instead of returning stale cached results.
	if m.nav.Level == model.LevelResources &&
		strings.HasPrefix(m.nav.ResourceType.Kind, "__security_") &&
		m.securityManager != nil {
		m.securityManager.Invalidate()
	}
	m.cancelAndReset()
	m.requestGen++
	m.setStatusMessage("Refreshing...", false)
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
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
	// If resource is already deleting, escalate the action.
	if sel.Deleting {
		m.confirmTypeInput.Clear()
		m.overlay = overlayConfirmType
		if model.IsForceDeleteableKind(kind) {
			// Pod/Job: offer force delete (--force --grace-period=0).
			m.confirmAction = sel.Name + " (FORCE)"
			m.confirmTitle = "Confirm Force Delete"
			m.confirmQuestion = fmt.Sprintf("Force delete %s? (--force --grace-period=0)", sel.Name)
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

	// Security source picker: the user selected a source from the
	// overlay shown by "Security Findings". Only applies when the
	// current level is LevelResourceTypes (where the picker lands).
	if m.pendingSecurityFilter != "" && m.nav.Level == model.LevelResourceTypes {
		return m.navigateToSecuritySource(actionLabel)
	}
	m.pendingSecurityFilter = "" // clear stale filter

	// Security ignore actions.
	if strings.HasPrefix(actionLabel, "Ignore") || actionLabel == "Un-ignore" {
		return m.executeSecurityIgnoreAction(actionLabel)
	}
	// "Refresh" from the security action menu — reuse the direct-action logic.
	if actionLabel == "Refresh" && strings.HasPrefix(m.nav.ResourceType.Kind, "__security_") {
		ret, cmd := m.directActionRefresh()
		return ret, cmd
	}

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
	case "Security Findings":
		mdl, cmd := m.executeActionSecurityFindings()
		return mdl, cmd, true
	}
	return m, nil, false
}

// executeActionSecurityFindings handles the "Security Findings" action.
// Shows an overlay listing available security sources (Heuristic, Trivy, etc.)
// so the user can pick which one to view. On selection, navigates to that
// source and applies a filter for the originally selected resource.
func (m Model) executeActionSecurityFindings() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}

	// Build overlay items from available security sources.
	m = m.ascendToResourceTypes()
	var items []model.Item
	for _, item := range m.middleItems {
		if item.Category == "Security" && item.Extra != "" {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		m.setStatusMessage("No security sources available", true)
		return m, scheduleStatusClear()
	}

	// If only one source, go directly.
	if len(items) == 1 {
		m.pendingSecurityFilter = sel.Name
		for i, it := range m.middleItems {
			if it.Kind == items[0].Kind {
				m.setCursor(i)
				break
			}
		}
		return m.navigateChild()
	}

	// Multiple sources — show a picker overlay.
	m.pendingSecurityFilter = sel.Name
	m.overlay = overlayAction
	m.overlayItems = items
	m.overlayCursor = 0
	return m, nil
}

// navigateToSecuritySource handles the selection from the security source
// picker overlay. Finds the matching source in middleItems, moves the
// cursor to it, and drills in. The pendingSecurityFilter is consumed by
// navigateChildResourceType to apply the resource name filter.
func (m Model) navigateToSecuritySource(sourceName string) (tea.Model, tea.Cmd) {
	// Find the source entry in middleItems by display name.
	for i, item := range m.middleItems {
		if item.Category == "Security" && item.Name == sourceName {
			m.setCursor(i)
			m.clampCursor()
			return m.navigateChild()
		}
	}
	// Not found — clear pending filter and show error.
	m.pendingSecurityFilter = ""
	m.setStatusMessage("Security source not found: "+sourceName, true)
	return m, scheduleStatusClear()
}

// openSecurityActionMenu builds a context-sensitive action menu for security
// finding groups and affected resources. Offers Ignore (Global), Ignore
// (This Resource), Un-ignore, and Refresh.
func (m Model) openSecurityActionMenu() Model {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m
	}

	var items []model.Item
	kctx := m.nav.Context
	sourceName := strings.TrimSuffix(strings.TrimPrefix(m.nav.ResourceType.Kind, "__security_"), "__")

	switch sel.Kind {
	case "__security_finding_group__":
		groupKey := sel.Extra
		ignored := isGroupIgnored(m.securityIgnores, kctx, sourceName, groupKey)
		if !ignored {
			items = append(items, model.Item{
				Name:   "Ignore (Global)",
				Extra:  "Hide this finding group from results",
				Status: "i",
			})
		} else {
			items = append(items, model.Item{
				Name:   "Un-ignore",
				Extra:  "Stop ignoring this finding group",
				Status: "u",
			})
		}

	case "__security_affected_resource__":
		groupKey := sel.Extra
		resourceKey := sel.ColumnValue("__resource_key__")
		groupIgnored := isGroupIgnored(m.securityIgnores, kctx, sourceName, groupKey)
		resourceIgnored := isResourceIgnored(m.securityIgnores, kctx, sourceName, groupKey, resourceKey)
		if !groupIgnored {
			items = append(items, model.Item{
				Name:   "Ignore (Global)",
				Extra:  "Hide the parent finding group from results",
				Status: "i",
			})
		}
		if !resourceIgnored && !groupIgnored && resourceKey != "" {
			items = append(items, model.Item{
				Name:   "Ignore (This Resource)",
				Extra:  "Hide this specific resource from the group",
				Status: "r",
			})
		}
		if resourceIgnored || groupIgnored {
			items = append(items, model.Item{
				Name:   "Un-ignore",
				Extra:  "Stop ignoring this entry",
				Status: "u",
			})
		}
	}

	items = append(items, model.Item{
		Name:   "Refresh",
		Extra:  "Reload security findings",
		Status: "R",
	})

	m.overlay = overlayAction
	m.overlayItems = items
	m.overlayCursor = 0
	return m
}

// executeSecurityIgnoreAction handles Ignore and Un-ignore actions from the
// security action menu. It updates the persistent SecurityIgnoreState, saves
// it to disk, re-wires the IgnoreChecker in the client, and refreshes.
func (m Model) executeSecurityIgnoreAction(actionLabel string) (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}

	kctx := m.nav.Context
	groupKey := sel.Extra
	sourceName := strings.TrimSuffix(strings.TrimPrefix(m.nav.ResourceType.Kind, "__security_"), "__")

	switch actionLabel {
	case "Ignore (Global)":
		m.securityIgnores = addSecurityIgnore(m.securityIgnores, kctx, SecurityIgnoreRule{
			Source:   sourceName,
			GroupKey: groupKey,
		})
		if err := saveSecurityIgnores(m.securityIgnores); err != nil {
			logger.Info("Failed to save security ignores", "error", err)
			m.setStatusMessage("Failed to save ignore rule", true)
			return m, scheduleStatusClear()
		}
		m.setStatusMessage("Ignored: "+groupKey, false)

	case "Ignore (This Resource)":
		resourceKey := sel.ColumnValue("__resource_key__")
		if resourceKey == "" {
			m.setStatusMessage("Cannot determine resource key", true)
			return m, scheduleStatusClear()
		}
		m.securityIgnores = addSecurityIgnore(m.securityIgnores, kctx, SecurityIgnoreRule{
			Source:   sourceName,
			GroupKey: groupKey,
			Resource: resourceKey,
		})
		if err := saveSecurityIgnores(m.securityIgnores); err != nil {
			logger.Info("Failed to save security ignores", "error", err)
			m.setStatusMessage("Failed to save ignore rule", true)
			return m, scheduleStatusClear()
		}
		m.setStatusMessage("Ignored resource: "+resourceKey, false)

	case "Un-ignore":
		// Determine whether this is a group-level or resource-level un-ignore.
		resourceKey := ""
		if sel.Kind == "__security_affected_resource__" {
			resourceKey = sel.ColumnValue("__resource_key__")
			// If the resource itself is not individually ignored but the group
			// is, un-ignore at the group level.
			if !isResourceSpecificIgnored(m.securityIgnores, kctx, sourceName, groupKey, resourceKey) {
				resourceKey = ""
			}
		}
		m.securityIgnores = removeSecurityIgnore(m.securityIgnores, kctx, sourceName, groupKey, resourceKey)
		if err := saveSecurityIgnores(m.securityIgnores); err != nil {
			logger.Info("Failed to save security ignores", "error", err)
			m.setStatusMessage("Failed to save ignore rule removal", true)
			return m, scheduleStatusClear()
		}
		m.setStatusMessage("Un-ignored: "+groupKey, false)
	}

	// Re-wire the ignore checker so the client picks up the new state.
	if m.client != nil {
		m.client.SetIgnoreChecker(&modelIgnoreChecker{state: m.securityIgnores, ctx: kctx})
	}

	// Invalidate the security manager cache so the refresh does a real
	// re-filter instead of returning stale results.
	if m.securityManager != nil {
		m.securityManager.Invalidate()
	}

	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

// executeActionExtended dispatches Argo, Helm, Flux, and other extended actions.
// Returns the model, cmd, and true if the action was handled.
func (m Model) executeActionExtended(actionLabel string) (tea.Model, tea.Cmd, bool) {
	switch actionLabel {
	case "Configure AutoSync", "Sync", "Sync (Apply Only)", "Refresh",
		"Terminate Sync", "Watch Workflow", "Suspend Workflow",
		"Resume Workflow", "Stop Workflow", "Terminate Workflow",
		"Resubmit Workflow", "Submit Workflow",
		"Suspend CronWorkflow", "Resume CronWorkflow":
		mdl, cmd := m.executeActionArgo(actionLabel)
		return mdl, cmd, true
	case "Force Renew":
		mdl, cmd := m.executeActionSimpleLoading("Triggering renewal for", m.forceRenewCertificate)
		return mdl, cmd, true
	case "Force Refresh":
		mdl, cmd := m.executeActionSimpleLoading("Force refreshing", m.forceRefreshExternalSecret)
		return mdl, cmd, true
	case "Pause":
		mdl, cmd := m.executeActionSimpleLoading("Pausing", m.pauseKEDAResource)
		return mdl, cmd, true
	case "Unpause":
		mdl, cmd := m.executeActionSimpleLoading("Unpausing", m.unpauseKEDAResource)
		return mdl, cmd, true
	case "Reconcile":
		mdl, cmd := m.executeActionSimpleLoading("Reconciling", m.reconcileFluxResource)
		return mdl, cmd, true
	case "Suspend":
		mdl, cmd := m.executeActionSimpleLoading("Suspending", m.suspendFluxResource)
		return mdl, cmd, true
	case "Resume":
		mdl, cmd := m.executeActionSimpleLoading("Resuming", m.resumeFluxResource)
		return mdl, cmd, true
	case "Values":
		mdl, cmd := m.executeActionHelmValues(false)
		return mdl, cmd, true
	case "All Values":
		mdl, cmd := m.executeActionHelmValues(true)
		return mdl, cmd, true
	case "Edit Values":
		mdl, cmd := m.executeActionEditValues()
		return mdl, cmd, true
	case "Diff":
		mdl, cmd := m.executeActionDiff()
		return mdl, cmd, true
	case "Upgrade":
		mdl, cmd := m.executeActionUpgrade()
		return mdl, cmd, true
	case "History":
		mdl, cmd := m.executeActionHelmHistory()
		return mdl, cmd, true
	}
	return m, nil, false
}

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

// executeActionForceDelete handles the "Force Delete" action.
func (m Model) executeActionForceDelete() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	rt := m.actionCtx.resourceType
	nsArg := ""
	if rt.Namespaced {
		nsArg = " -n " + ns
	}
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s %s --grace-period=0 --force%s --context %s", rt.Resource, name, nsArg, ctx))
	m.loading = true
	return m, m.forceDeleteResource()
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

// executeActionShell handles the "Shell" action.
func (m Model) executeActionShell() (tea.Model, tea.Cmd) {
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl debug node/%s -it --image=busybox --context %s -- chroot /host /bin/sh", name, ctx))
	return m, m.execKubectlNodeShell()
}

// executeActionDebugPod handles the "Debug Pod" action.
func (m Model) executeActionDebugPod() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl run lfk-debug-<id> --image=alpine --rm -it --restart=Never -n %s --context %s -- sh", ns, ctx))
	return m, m.runDebugPod()
}

// executeActionGoToPod handles the "Go to Pod" action.
func (m Model) executeActionGoToPod() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	var podNames []string
	for _, kv := range m.actionCtx.columns {
		if kv.Key == "Used By" && kv.Value != "" {
			for p := range strings.SplitSeq(kv.Value, ", ") {
				p = strings.TrimSpace(p)
				if p != "" {
					podNames = append(podNames, p)
				}
			}
			break
		}
	}
	if len(podNames) == 0 {
		m.setStatusMessage("No pods using this PVC", true)
		return m, scheduleStatusClear()
	}
	if len(podNames) == 1 {
		return m.navigateToOwner("Pod", podNames[0])
	}
	var items []model.Item
	for _, pn := range podNames {
		items = append(items, model.Item{Name: pn, Namespace: ns})
	}
	m.overlayItems = items
	m.overlay = overlayPodSelect
	m.overlayCursor = 0
	m.pendingAction = "Go to Pod"
	m.logPodFilterText = ""
	m.logPodFilterActive = false
	ui.ResetOverlayPodScroll()
	return m, nil
}

// executeActionDebugMount handles the "Debug Mount" action.
func (m Model) executeActionDebugMount() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl run debug-pvc --image=alpine -it --rm --restart=Never --overrides='{...pvc:%s...}' -n %s --context %s", name, ns, ctx))
	return m, m.runDebugPodWithPVC()
}

// executeActionOpenInBrowser handles the "Open in Browser" action.
func (m Model) executeActionOpenInBrowser() (tea.Model, tea.Cmd) {
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
}

// executeActionHelmValues handles the "Values" and "All Values" actions.
func (m Model) executeActionHelmValues(all bool) (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	if all {
		m.addLogEntry("DBG", fmt.Sprintf("$ helm get values %s -n %s --kube-context %s -o yaml --all", name, ns, ctx))
	} else {
		m.addLogEntry("DBG", fmt.Sprintf("$ helm get values %s -n %s --kube-context %s -o yaml", name, ns, ctx))
	}
	m.loading = true
	return m, m.loadHelmValues(all)
}

// executeActionEditValues handles the "Edit Values" action.
func (m Model) executeActionEditValues() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ helm get values %s -n %s --kube-context %s -o yaml → $EDITOR → helm upgrade --reuse-values", name, ns, ctx))
	return m, m.editHelmValues()
}

// executeActionDiff handles the "Diff" action.
func (m Model) executeActionDiff() (tea.Model, tea.Cmd) {
	name := m.actionCtx.name
	if m.actionCtx.kind == "HelmRelease" {
		m.addLogEntry("DBG", fmt.Sprintf("Comparing default vs user values for %s", name))
		m.loading = true
		return m, m.helmDiff()
	}
	// Non-Helm diff (two-resource YAML diff) is handled via bulk action.
	return m, nil
}

// executeActionUpgrade handles the "Upgrade" action.
func (m Model) executeActionUpgrade() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ helm upgrade %s -n %s --kube-context %s", name, ns, ctx))
	return m, m.helmUpgrade()
}

// executeActionPermissions handles the "Permissions" action.
func (m Model) executeActionPermissions() (tea.Model, tea.Cmd) {
	m.loading = true
	m.setStatusMessage("Checking RBAC permissions...", false)
	return m, m.checkRBAC()
}

// executeActionStartupAnalysis handles the "Startup Analysis" action.
func (m Model) executeActionStartupAnalysis() (tea.Model, tea.Cmd) {
	m.loading = true
	m.setStatusMessage("Analyzing pod startup...", false)
	return m, m.loadPodStartup()
}

// executeActionAlerts handles the "Alerts" action.
func (m Model) executeActionAlerts() (tea.Model, tea.Cmd) {
	m.loading = true
	m.setStatusMessage("Loading Prometheus alerts...", false)
	return m, m.loadAlerts()
}

// executeActionLabelsAnnotations handles the "Labels / Annotations" action.
func (m Model) executeActionLabelsAnnotations() (tea.Model, tea.Cmd) {
	m.labelResourceType = m.actionCtx.resourceType
	return m, m.loadLabelData()
}

// executeActionStop handles the "Stop" action.
func (m Model) executeActionStop() (tea.Model, tea.Cmd) {
	// Stop a port forward entry.
	if m.actionCtx.kind == "__port_forward_entry__" || m.actionCtx.kind == "__port_forwards__" {
		pfID := m.getPortForwardID(m.actionCtx.columns)
		if pfID > 0 {
			return m, m.stopPortForward(pfID)
		}
	}
	return m, nil
}

// executeActionRemove handles the "Remove" action.
func (m Model) executeActionRemove() (tea.Model, tea.Cmd) {
	// Remove a port forward entry.
	if m.actionCtx.kind == "__port_forward_entry__" || m.actionCtx.kind == "__port_forwards__" {
		pfID := m.getPortForwardID(m.actionCtx.columns)
		if pfID > 0 {
			m.portForwardMgr.Remove(pfID)
			m.setMiddleItems(m.portForwardItems())
			m.clampCursor()
			m.saveCurrentPortForwards()
			m.setStatusMessage("Port forward removed", false)
			return m, scheduleStatusClear()
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

func (m Model) refreshCurrentLevel() tea.Cmd {
	switch m.nav.Level {
	case model.LevelClusters:
		return m.loadContexts()
	case model.LevelResourceTypes:
		// Discovery is cached for the lifetime of the session; without an
		// explicit re-run, newly-installed CRDs (or removed ones) stay
		// hidden until lfk restarts. shift+r at this level should pick
		// them up. Dedup against an already-in-flight discovery so rapid
		// presses don't stack API calls.
		var cmds []tea.Cmd
		if !m.discoveringContexts[m.nav.Context] {
			if m.discoveringContexts != nil {
				m.discoveringContexts[m.nav.Context] = true
			}
			// Force a round-trip; otherwise shift+r would serve stale cache.
			m.client.InvalidateDiscoveryCache(m.nav.Context)
			cmds = append(cmds, m.discoverAPIResources(m.nav.Context))
		}
		// Always emit the current cached list too so the UI repaints
		// immediately while the fresh discovery runs in the background.
		// updateAPIResourceDiscovery overwrites middleItems on completion.
		cmds = append(cmds, m.loadResourceTypes())
		return tea.Batch(cmds...)
	case model.LevelResources:
		// Port forwards are virtual - refresh from the manager directly.
		// The gen field MUST be captured and forwarded so the update
		// handler doesn't discard the message as stale when requestGen
		// has been bumped by any cursor movement since the cmd was built.
		if m.nav.ResourceType.Kind == "__port_forwards__" {
			gen := m.requestGen
			items := m.portForwardItems()
			return func() tea.Msg {
				return resourcesLoadedMsg{items: items, gen: gen}
			}
		}
		return m.loadResources(false)
	case model.LevelOwned:
		// Security finding groups use a dedicated loader; the generic
		// loadOwned has no dispatch for virtual security kinds and would
		// return nil items, wiping the affected resources list.
		if strings.HasPrefix(m.nav.ResourceType.Kind, "__security_") {
			// Find the group key from leftItems (the grouped findings pushed
			// when the user drilled in). nav.ResourceName holds the group title.
			for _, it := range m.leftItems {
				if it.Kind == "__security_finding_group__" && it.Name == m.nav.ResourceName {
					return m.loadSecurityAffectedResources(it.Extra)
				}
			}
			return nil
		}
		return m.loadOwned(false)
	case model.LevelContainers:
		return m.loadContainers(false)
	}
	return nil
}

// cancelActiveTabLogStreams cancels the live (Model-level) log stream
// and history-fetch contexts. Used by tab-close paths so the closing
// tab's kubectl subprocess + reader goroutine exit immediately, while
// sibling tabs' streams (held in TabState.logCancel) keep running.
func (m *Model) cancelActiveTabLogStreams() {
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
	}
	if m.logHistoryCancel != nil {
		m.logHistoryCancel()
		m.logHistoryCancel = nil
	}
}

// cancelAllTabLogStreams cancels every log stream owned by the Model:
// the active tab's stream + history (held on Model) and every inactive
// tab's stream (held in TabState.logCancel). Used by quit paths so no
// kubectl subprocess or reader goroutine outlives the lfk process.
func (m *Model) cancelAllTabLogStreams() {
	m.cancelActiveTabLogStreams()
	for i := range m.tabs {
		if m.tabs[i].logCancel != nil {
			m.tabs[i].logCancel()
			m.tabs[i].logCancel = nil
		}
	}
}

// closeTabOrQuit closes the current tab if multiple tabs are open,
// otherwise quits the application (with optional confirmation).
func (m Model) closeTabOrQuit() (tea.Model, tea.Cmd) {
	if len(m.tabs) > 1 {
		m.cancelActiveTabLogStreams()
		m.tabs = append(m.tabs[:m.activeTab], m.tabs[m.activeTab+1:]...)
		if m.activeTab > 0 {
			m.activeTab--
		}
		// Load the surviving tab BEFORE saving session, so saveCurrentTab
		// writes the surviving tab's data (not the closed tab's stale state).
		cmd := m.loadTab(m.activeTab)
		m.saveCurrentSession()
		if cmd != nil {
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
	m.cancelAllTabLogStreams()
	m.saveCurrentSession()
	return m, tea.Quit
}

func (m Model) executeActionScale() Model {
	m.scaleInput.Clear()
	m.overlay = overlayScaleInput
	return m
}

func (m Model) executeActionVulnScan() (tea.Model, tea.Cmd) {
	image := m.actionCtx.image
	if image == "" {
		m.setStatusMessage("No image found for this container", true)
		return m, scheduleStatusClear()
	}
	m.addLogEntry("DBG", fmt.Sprintf("$ trivy image %s", image))
	m.loading = true
	m.setStatusMessage("Scanning image for vulnerabilities...", false)
	return m, m.vulnScanImage(image)
}

func (m Model) executeActionVisualize() (tea.Model, tea.Cmd) {
	m.loading = true
	m.setStatusMessage("Loading network policy...", false)
	return m, m.loadNetworkPolicy()
}

func (m Model) executeActionDefault(actionLabel string) (tea.Model, tea.Cmd) {
	if ca, ok := findCustomAction(m.actionCtx.kind, actionLabel); ok {
		expandedCmd := expandCustomActionTemplate(ca.Command, m.actionCtx)
		m.addLogEntry("DBG", fmt.Sprintf("$ sh -c %q", expandedCmd))
		return m, m.execCustomAction(expandedCmd)
	}
	return m, nil
}
