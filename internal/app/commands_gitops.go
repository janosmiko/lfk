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
