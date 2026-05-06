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
			if m.readOnly && isMutatingAction(a.Label) {
				continue
			}
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

	// At the cluster picker, the action menu is a tiny per-cluster
	// metadata menu. Today only "Set color…" lives there; Ctrl+R remains
	// the hotkey for read-only. The dedicated branch keeps the kind-based
	// `selectedResourceKind` machinery below from firing on a level that
	// has no resource type.
	if m.nav.Level == model.LevelClusters {
		sel := m.selectedMiddleItem()
		if sel == nil {
			return m
		}
		m.bulkMode = false
		m.overlay = overlayAction
		m.overlayItems = []model.Item{
			{
				Name: "Set color",
				// "Status" doubles as the in-menu hotkey hint
				// rendered as "[L] Set color - …" by RenderActionOverlay.
				// Using the same key the global handler uses
				// (kb.ClusterColorPicker) keeps the menu and the bare
				// keypress in sync, so users who learn one path
				// automatically know the other.
				Status: ui.ActiveKeybindings.ClusterColorPicker,
				Extra:  "Assign a background tint to this context",
			},
		}
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
		// Use the kind-aware variant so custom actions are filtered based
		// on their ReadOnlySafe opt-in (defaults to false / mutating).
		if m.readOnly && isMutatingActionForKind(kind, a.Label) {
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
	m.invalidateOrphanCacheForNamespace(m.nav.Context, m.namespace)
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

	// Cluster-picker actions live outside the kind-based machinery: they
	// don't have an actionCtx and there is no resource type at this level.
	// Dispatch them by label and short-circuit before the read-only check
	// fires on a label that has no kind to consult.
	if m.nav.Level == model.LevelClusters && actionLabel == "Set color" {
		return m.handleKeyClusterColorPicker()
	}

	// Handle bulk actions.
	if m.bulkMode && len(m.bulkItems) > 0 {
		return m.executeBulkAction(actionLabel)
	}

	if m.readOnly && isMutatingAction(actionLabel) {
		logger.Info("Blocked by read-only mode", "action", actionLabel, "context", m.actionCtx.context)
		m.setStatusMessage(readOnlyBlockedMessage(actionLabel), true)
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
	if m.readOnly && isMutatingAction(actionLabel) {
		logger.Info("Blocked by read-only mode (bulk)", "action", actionLabel, "count", len(m.bulkItems))
		m.setStatusMessage(readOnlyBlockedMessage(actionLabel), true)
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
	m.performQuitCleanup()
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
		// Custom actions are arbitrary shell commands. Block them in
		// read-only mode unless the user explicitly marked the action
		// safe via read_only_safe: true. The dispatcher gate at the top
		// of executeAction only checks the static mutatingActions set,
		// which doesn't know about user-defined labels — this is the
		// last chance to refuse.
		if m.readOnly && !ca.ReadOnlySafe {
			m.setStatusMessage(readOnlyBlockedMessage(actionLabel), true)
			return m, scheduleStatusClear()
		}
		expandedCmd := expandCustomActionTemplate(ca.Command, m.actionCtx)
		m.addLogEntry("DBG", fmt.Sprintf("$ sh -c %q", expandedCmd))
		return m, m.execCustomAction(expandedCmd)
	}
	return m, nil
}
