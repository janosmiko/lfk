package app

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/logger"
)

// logGitOps emits a structured intent log entry for a gitops mutation. Each
// caller uses a distinct event name so postmortem grep (jq '.msg') can find
// "we triggered X on Y" lines without scanning generic "Status message" rows.
func logGitOps(event, kctx, namespace, name string, extra ...any) {
	args := append([]any{"context", kctx, "namespace", namespace, "name", name}, extra...)
	logger.Info(event, args...)
}

func (m Model) syncArgoApp(applyOnly bool) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	logGitOps("ArgoCD app sync requested", ctx, ns, name, "applyOnly", applyOnly)
	label := "Sync ArgoApp"
	if applyOnly {
		label = "Sync ArgoApp (apply only)"
	}
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, label+": "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
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
	logGitOps("ArgoCD app refresh requested", ctx, ns, name)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Refresh ArgoApp: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
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
	logGitOps("ArgoCD ApplicationSet refresh requested", ctx, ns, name)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Refresh ApplicationSet: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
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
	logGitOps("Flux reconcile requested", ctx, ns, name, "kind", rt.Kind)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Reconcile Flux: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
		err := m.client.ReconcileFluxResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Reconciliation triggered for %s", name)}
	})
}

// toggleFluxSuspend suspends or resumes a Flux resource, flipping spec.suspend.
func (m Model) toggleFluxSuspend() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	logGitOps("Flux suspend toggle requested", ctx, ns, name, "kind", rt.Kind)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Toggle Flux suspend: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
		suspended, err := m.client.ToggleFluxSuspend(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		verb := "Resumed"
		if suspended {
			verb = "Suspended"
		}
		return actionResultMsg{message: fmt.Sprintf("%s %s", verb, name)}
	})
}

// --- cert-manager commands ---

func (m Model) forceRenewCertificate() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	logGitOps("cert-manager Certificate renewal requested", ctx, ns, name)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Renew certificate: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
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
	logGitOps("Argo Workflow suspend requested", ctx, ns, name)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Suspend workflow: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
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
	logGitOps("Argo Workflow resume requested", ctx, ns, name)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Resume workflow: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
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
	logGitOps("Argo Workflow stop requested", ctx, ns, name)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Stop workflow: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
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
	logGitOps("Argo Workflow terminate requested", ctx, ns, name)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Terminate workflow: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
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
	logGitOps("Argo Workflow resubmit requested", ctx, ns, name)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Resubmit workflow: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
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
	logGitOps("Argo Workflow submit-from-template requested", ctx, ns, name, "clusterScope", clusterScope)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Submit workflow from template: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
		newName, err := m.client.SubmitWorkflowFromTemplate(ctx, ns, name, clusterScope)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Submitted workflow %s", newName)}
	})
}

func (m Model) toggleCronWorkflowSuspend() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	logGitOps("Argo CronWorkflow suspend toggle requested", ctx, ns, name)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Toggle cron workflow suspend: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
		suspended, err := m.client.ToggleCronWorkflowSuspend(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		verb := "Resumed"
		if suspended {
			verb = "Suspended"
		}
		return actionResultMsg{message: fmt.Sprintf("%s cron workflow %s", verb, name)}
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
	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindResourceList,
		"Watch workflow: "+name,
		bgtaskTarget(ctx, ns),
		func(schedCtx context.Context) tea.Msg {
			content, _, err := m.client.GetWorkflowStatus(schedCtx, ctx, ns, name)
			if err != nil {
				return describeLoadedMsg{title: "Watch: " + name, err: err}
			}
			return describeLoadedMsg{title: "Watch: " + name, content: sanitizeDescribeContent(content)}
		},
	)
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
	logGitOps("ExternalSecret force-refresh requested", ctx, ns, name)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Refresh ExternalSecret: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
		err := m.client.ForceRefreshExternalSecret(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Force refresh triggered for %s", name)}
	})
}

// toggleKEDAPause pauses or resumes autoscaling on a KEDA ScaledObject or
// ScaledJob, flipping the paused-replicas annotation.
func (m Model) toggleKEDAPause() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	logGitOps("KEDA pause toggle requested", ctx, ns, name, "kind", rt.Kind)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Toggle KEDA pause: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
		paused, err := m.client.ToggleKEDAPause(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		verb := "Unpaused"
		if paused {
			verb = "Paused"
		}
		return actionResultMsg{message: fmt.Sprintf("%s %s", verb, name)}
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
	logger.Info("ArgoCD bulk sync requested", "context", ctx, "namespace", ns, "count", len(items), "applyOnly", applyOnly)
	return m.trackBgTask(scheduler.KindMutation, fmt.Sprintf("%s (%d)", label, len(items)), ctx, func() tea.Msg {
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

	logger.Info("ArgoCD bulk refresh requested", "context", ctx, "namespace", ns, "count", len(items))
	return m.trackBgTask(scheduler.KindMutation, fmt.Sprintf("Bulk refresh ArgoApps (%d)", len(items)), ctx, func() tea.Msg {
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
	logGitOps("ArgoCD sync termination requested", ctx, ns, name)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Terminate sync: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
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

	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindResourceList,
		"Autosync config: "+name,
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			enabled, selfHeal, prune, err := client.GetAutoSyncConfig(ctx, kctx, ns, name)
			return autoSyncLoadedMsg{enabled: enabled, selfHeal: selfHeal, prune: prune, err: err}
		},
	)
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

	logGitOps("ArgoCD autosync config save requested", kctx, ns, name, "enabled", enabled, "selfHeal", selfHeal, "prune", prune)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Save autosync config: "+name, bgtaskTarget(kctx, ns), func(ctx context.Context) tea.Msg {
		err := client.UpdateAutoSyncConfig(ctx, kctx, ns, name, enabled, selfHeal, prune)
		return autoSyncSavedMsg{err: err}
	})
}
