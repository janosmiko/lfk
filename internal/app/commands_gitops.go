package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (m Model) syncArgoApp(applyOnly bool) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		err := m.client.SyncArgoApp(ctx, ns, name, applyOnly)
		if err != nil {
			return actionResultMsg{err: err}
		}
		label := "Sync"
		if applyOnly {
			label = "Sync (apply only)"
		}
		return actionResultMsg{message: fmt.Sprintf("%s initiated for %s", label, name)}
	}
}

func (m Model) refreshArgoApp() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		err := m.client.RefreshArgoApp(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Hard refresh initiated for %s", name)}
	}
}

// reconcileFluxResource triggers reconciliation of a Flux resource.
func (m Model) reconcileFluxResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	return func() tea.Msg {
		err := m.client.ReconcileFluxResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Reconciliation triggered for %s", name)}
	}
}

// suspendFluxResource suspends a Flux resource.
func (m Model) suspendFluxResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	return func() tea.Msg {
		err := m.client.SuspendFluxResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Suspended %s", name)}
	}
}

// resumeFluxResource resumes a Flux resource.
func (m Model) resumeFluxResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	return func() tea.Msg {
		err := m.client.ResumeFluxResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Resumed %s", name)}
	}
}

// --- cert-manager commands ---

func (m Model) forceRenewCertificate() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		if err := m.client.ForceRenewCertificate(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Renewal triggered for %s", name)}
	}
}

// --- Argo Workflows commands ---

func (m Model) suspendArgoWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		if err := m.client.SuspendArgoWorkflow(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Suspended workflow %s", name)}
	}
}

func (m Model) resumeArgoWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		if err := m.client.ResumeArgoWorkflow(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Resumed workflow %s", name)}
	}
}

func (m Model) stopArgoWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		if err := m.client.StopArgoWorkflow(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Stopping workflow %s", name)}
	}
}

func (m Model) terminateArgoWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		if err := m.client.TerminateArgoWorkflow(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Terminated workflow %s", name)}
	}
}

func (m Model) resubmitArgoWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		newName, err := m.client.ResubmitArgoWorkflow(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Resubmitted as %s", newName)}
	}
}

func (m Model) submitWorkflowFromTemplate(clusterScope bool) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		newName, err := m.client.SubmitWorkflowFromTemplate(ctx, ns, name, clusterScope)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Submitted workflow %s", newName)}
	}
}

func (m Model) suspendCronWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		if err := m.client.SuspendCronWorkflow(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Suspended cron workflow %s", name)}
	}
}

func (m Model) resumeCronWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		if err := m.client.ResumeCronWorkflow(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Resumed cron workflow %s", name)}
	}
}

// forceRefreshExternalSecret triggers a force sync on an ESO resource.
func (m Model) forceRefreshExternalSecret() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	if !rt.Namespaced {
		ns = ""
	}
	return func() tea.Msg {
		err := m.client.ForceRefreshExternalSecret(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Force refresh triggered for %s", name)}
	}
}

// pauseKEDAResource pauses a KEDA ScaledObject or ScaledJob.
func (m Model) pauseKEDAResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	return func() tea.Msg {
		err := m.client.PauseKEDAResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Paused %s", name)}
	}
}

// unpauseKEDAResource unpauses a KEDA ScaledObject or ScaledJob.
func (m Model) unpauseKEDAResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	return func() tea.Msg {
		err := m.client.UnpauseKEDAResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Unpaused %s", name)}
	}
}

func (m Model) bulkSyncArgoApps(applyOnly bool) tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	client := m.client

	return func() tea.Msg {
		var succeeded, failed int
		var errors []string
		for _, item := range items {
			itemNs := ns
			if item.Namespace != "" {
				itemNs = item.Namespace
			}
			err := client.SyncArgoApp(ctx, itemNs, item.Name, applyOnly)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", item.Name, err.Error()))
			} else {
				succeeded++
			}
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}

func (m Model) bulkRefreshArgoApps() tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	client := m.client

	return func() tea.Msg {
		var succeeded, failed int
		var errors []string
		for _, item := range items {
			itemNs := ns
			if item.Namespace != "" {
				itemNs = item.Namespace
			}
			err := client.RefreshArgoApp(ctx, itemNs, item.Name)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", item.Name, err.Error()))
			} else {
				succeeded++
			}
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}

func (m Model) terminateArgoSync() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		err := m.client.TerminateArgoSync(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Sync termination requested for %s", name)}
	}
}
