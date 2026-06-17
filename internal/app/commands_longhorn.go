package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/logger"
)

// forceDeleteLonghornNode disables scheduling on the selected longhorn.io node
// and then deletes it. The validating webhook (validator.longhorn.io) rejects
// deletion of a still-schedulable node, so the plain delete path fails; this
// satisfies the webhook without removing replicas/engines (a node that still
// holds data is rejected on purpose and the error is surfaced).
func (m Model) forceDeleteLonghornNode() tea.Cmd {
	kctx := m.actionCtx.context
	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name
	logger.Info("Force deleting Longhorn node", "name", name, "namespace", ns, "context", kctx)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Force delete Longhorn node "+name, bgtaskTarget(kctx, ns), func(ctx context.Context) tea.Msg {
		if err := m.client.ForceDeleteLonghornNode(ctx, kctx, ns, rt, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Force deleted Longhorn node %s", name)}
	})
}

// setLonghornNodeEviction requests (evict=true) or cancels (evict=false)
// replica eviction on the selected longhorn.io node. Requesting eviction also
// disables scheduling; Longhorn rebuilds each replica on another node before
// removing it here, so no data is lost.
func (m Model) setLonghornNodeEviction(evict bool) tea.Cmd {
	kctx := m.actionCtx.context
	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name

	taskName := "Evict replicas from Longhorn node " + name
	doneMsg := fmt.Sprintf("Replica eviction requested for %s", name)
	if !evict {
		taskName = "Cancel eviction on Longhorn node " + name
		doneMsg = fmt.Sprintf("Replica eviction cancelled for %s", name)
	}

	logger.Info("Setting Longhorn node eviction", "name", name, "evict", evict, "namespace", ns, "context", kctx)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, taskName, bgtaskTarget(kctx, ns), func(ctx context.Context) tea.Msg {
		if err := m.client.SetLonghornNodeEviction(ctx, kctx, ns, rt, name, evict); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: doneMsg}
	})
}
