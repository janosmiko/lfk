package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// executeActionExtended dispatches Argo / Helm / Flux / cert-manager /
// KEDA / ExternalSecrets / Karpenter action labels. Returns (model, cmd,
// handled). Lives in its own file so update_actions.go stays under the
// 800-line file cap (revive: file-length-limit); the original switch
// was at the boundary before Karpenter labels were added.
func (m Model) executeActionExtended(actionLabel string) (tea.Model, tea.Cmd, bool) {
	switch actionLabel {
	case "Disrupt", "Cordon Node", "Uncordon Node", "Drain Node":
		return m.executeActionKarpenter(actionLabel)
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
