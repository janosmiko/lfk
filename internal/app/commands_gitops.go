package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
)

func (m Model) syncArgoApp(applyOnly bool) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	label := "Sync ArgoApp"
	if applyOnly {
		label = "Sync ArgoApp (apply only)"
	}
	return m.trackBgTask(bgtasks.KindMutation, label+": "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.SyncArgoApp(ctx, ns, name, applyOnly)
		if err != nil {
			return actionResultMsg{err: err}
		}
		message := "Sync"
		if applyOnly {
			message = "Sync (apply only)"
		}
		return actionResultMsg{message: fmt.Sprintf("%s initiated for %s", message, name)}
	})
}

func (m Model) refreshArgoApp() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindMutation, "Refresh ArgoApp: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.RefreshArgoApp(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Hard refresh initiated for %s", name)}
	})
}

func (m Model) refreshArgoAppSet() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindMutation, "Refresh ApplicationSet: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.RefreshArgoAppSet(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Refresh triggered for ApplicationSet %s", name)}
	})
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
	return m.trackBgTask(bgtasks.KindMutation, "Reconcile Flux: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.ReconcileFluxResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Reconciliation triggered for %s", name)}
	})
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
	return m.trackBgTask(bgtasks.KindMutation, "Suspend Flux: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.SuspendFluxResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Suspended %s", name)}
	})
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
	return m.trackBgTask(bgtasks.KindMutation, "Resume Flux: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.ResumeFluxResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Resumed %s", name)}
	})
}

// --- cert-manager commands ---

func (m Model) forceRenewCertificate() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindMutation, "Renew certificate: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		if err := m.client.ForceRenewCertificate(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Renewal triggered for %s", name)}
	})
}

// --- Argo Workflows commands ---

func (m Model) suspendArgoWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindMutation, "Suspend workflow: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		if err := m.client.SuspendArgoWorkflow(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Suspended workflow %s", name)}
	})
}

func (m Model) resumeArgoWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindMutation, "Resume workflow: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		if err := m.client.ResumeArgoWorkflow(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Resumed workflow %s", name)}
	})
}

func (m Model) stopArgoWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindMutation, "Stop workflow: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		if err := m.client.StopArgoWorkflow(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Stopping workflow %s", name)}
	})
}

func (m Model) terminateArgoWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindMutation, "Terminate workflow: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		if err := m.client.TerminateArgoWorkflow(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Terminated workflow %s", name)}
	})
}

func (m Model) resubmitArgoWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindMutation, "Resubmit workflow: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		newName, err := m.client.ResubmitArgoWorkflow(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Resubmitted as %s", newName)}
	})
}

func (m Model) submitWorkflowFromTemplate(clusterScope bool) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindMutation, "Submit workflow from template: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		newName, err := m.client.SubmitWorkflowFromTemplate(ctx, ns, name, clusterScope)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Submitted workflow %s", newName)}
	})
}

func (m Model) suspendCronWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindMutation, "Suspend cron workflow: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		if err := m.client.SuspendCronWorkflow(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Suspended cron workflow %s", name)}
	})
}

func (m Model) resumeCronWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindMutation, "Resume cron workflow: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		if err := m.client.ResumeCronWorkflow(ctx, ns, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Resumed cron workflow %s", name)}
	})
}

// watchArgoWorkflow reads workflow status for the describe viewer. Called
// both on initial open and on every describe-refresh tick, so it uses the
// m.suppressBgtasks flag (propagated from describe-refresh ticks the same
// way watch-mode refreshes propagate via resourcesLoadedMsg.silent) to
// stay off the :tasks overlay on polling calls while still being visible
// on the first manual invocation.
func (m Model) watchArgoWorkflow() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindResourceList, "Watch workflow: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		content, _, err := m.client.GetWorkflowStatus(ctx, ns, name)
		if err != nil {
			return describeLoadedMsg{title: "Watch: " + name, err: err}
		}
		return describeLoadedMsg{title: "Watch: " + name, content: content}
	})
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
	return m.trackBgTask(bgtasks.KindMutation, "Refresh ExternalSecret: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.ForceRefreshExternalSecret(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Force refresh triggered for %s", name)}
	})
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
	return m.trackBgTask(bgtasks.KindMutation, "Pause KEDA: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.PauseKEDAResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Paused %s", name)}
	})
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
	return m.trackBgTask(bgtasks.KindMutation, "Unpause KEDA: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.UnpauseKEDAResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Unpaused %s", name)}
	})
}

func (m Model) bulkSyncArgoApps(applyOnly bool) tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	client := m.client

	label := "Bulk sync ArgoApps"
	if applyOnly {
		label = "Bulk sync ArgoApps (apply only)"
	}
	return m.trackBgTask(bgtasks.KindMutation, fmt.Sprintf("%s (%d)", label, len(items)), ctx, func() tea.Msg {
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
	})
}

func (m Model) bulkRefreshArgoApps() tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	client := m.client

	return m.trackBgTask(bgtasks.KindMutation, fmt.Sprintf("Bulk refresh ArgoApps (%d)", len(items)), ctx, func() tea.Msg {
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
	})
}

func (m Model) terminateArgoSync() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindMutation, "Terminate sync: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.TerminateArgoSync(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Sync termination requested for %s", name)}
	})
}

// autoSyncLoadedMsg carries the current autosync configuration.
type autoSyncLoadedMsg struct {
	enabled, selfHeal, prune bool
	err                      error
}

// autoSyncSavedMsg carries the result of saving autosync configuration.
type autoSyncSavedMsg struct {
	err error
}

func (m Model) loadAutoSyncConfig() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	kctx := m.nav.Context
	ns := sel.Namespace
	if ns == "" {
		ns = m.resolveNamespace()
	}
	name := sel.Name
	client := m.client

	return m.trackBgTask(bgtasks.KindResourceList, "Autosync config: "+name, bgtaskTarget(kctx, ns), func() tea.Msg {
		enabled, selfHeal, prune, err := client.GetAutoSyncConfig(context.Background(), kctx, ns, name)
		return autoSyncLoadedMsg{enabled: enabled, selfHeal: selfHeal, prune: prune, err: err}
	})
}

func (m Model) saveAutoSyncConfig() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	kctx := m.nav.Context
	ns := sel.Namespace
	if ns == "" {
		ns = m.resolveNamespace()
	}
	name := sel.Name
	enabled := m.autoSyncEnabled
	selfHeal := m.autoSyncSelfHeal
	prune := m.autoSyncPrune
	client := m.client

	return m.trackBgTask(bgtasks.KindMutation, "Save autosync config: "+name, bgtaskTarget(kctx, ns), func() tea.Msg {
		err := client.UpdateAutoSyncConfig(context.Background(), kctx, ns, name, enabled, selfHeal, prune)
		return autoSyncSavedMsg{err: err}
	})
}
