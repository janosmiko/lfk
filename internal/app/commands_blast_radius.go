package app

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
)

// blastRadiusLoadedMsg carries the cost of the pending action. req numbers the
// fetch, because closing and reopening a confirm can leave an older reply in
// flight that would otherwise land on the new dialog.
type blastRadiusLoadedMsg struct {
	radius *k8s.BlastRadius
	req    uint64
	err    error
}

// loadBlastRadius fetches the pods an action would remove and the budgets that
// cover them, then does the arithmetic inside the goroutine. Nothing heavy
// runs on the event loop.
//
// drain crosses every namespace on the node; delete and scale-down stay inside
// the target's own namespace.
func (m Model) loadBlastRadius(drain bool) tea.Cmd {
	req := m.blast.req
	client := m.client
	ctxName := m.actionCtx.context
	namespace := m.actionCtx.namespace
	kind := m.actionCtx.kind
	name := m.actionCtx.name
	raw := m.actionCtx.raw
	if client == nil {
		return nil
	}

	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindYAMLFetch,
		"Blast radius: "+name,
		bgtaskTarget(ctxName, namespace),
		func(ctx context.Context) tea.Msg {
			pods, readyBefore, err := blastRadiusPods(ctx, client, drain, ctxName, namespace, kind, name, raw)
			if err != nil {
				return blastRadiusLoadedMsg{req: req, err: err}
			}
			// An empty namespace lists budgets cluster-wide, which a drain
			// needs and a namespaced action must not do.
			pdbNamespace := namespace
			if drain {
				pdbNamespace = ""
			}
			pdbs, err := client.ListPodDisruptionBudgets(ctx, ctxName, pdbNamespace)
			if err != nil {
				return blastRadiusLoadedMsg{req: req, err: err}
			}
			radius := k8s.ComputeBlastRadius(pods, pdbs, readyBefore)
			return blastRadiusLoadedMsg{radius: &radius, req: req}
		},
	)
}

// blastRadiusPods returns the pods the action removes and the ready replica
// count to measure against. A bare Pod is its own blast radius; a workload is
// whatever its selector claims.
func blastRadiusPods(
	ctx context.Context, client *k8s.Client, drain bool,
	ctxName, namespace, kind, name string, raw map[string]any,
) ([]k8s.EvictedPod, int, error) {
	if drain {
		pods, err := client.PodsOnNode(ctx, ctxName, name)
		// Drain has no single workload, so there is no replica count to show.
		return pods, 0, err
	}
	if kind == "Pod" {
		// A pod is its own blast radius, and the row already carries its
		// labels, so this costs no call at all.
		pod := evictedPodFromRaw(raw, namespace)
		ready := 0
		if pod.Ready {
			ready = 1
		}
		return []k8s.EvictedPod{pod}, ready, nil
	}
	selector := workloadSelectorFrom(raw)
	if selector == nil {
		return nil, 0, nil
	}
	pods, err := client.PodsForSelector(ctx, ctxName, namespace, selector)
	return pods, readyReplicasFrom(raw), err
}

// updateBlastRadiusLoaded stores the cost of the pending action. A failure
// leaves the dialog without the line rather than blocking the action: the
// user asked to delete something, not to read a budget report.
//
//nolint:unparam // consistent message handler signature; the caller passes the Cmd on
func (m Model) updateBlastRadiusLoaded(msg blastRadiusLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.req != m.blast.req {
		return m, nil // an older dialog answering late
	}
	m.blast.loading = false
	if msg.err != nil {
		m.blast.radius = nil
		return m, nil
	}
	m.blast.radius = msg.radius
	return m, nil
}

// beginBlastRadius arms the line for a confirm that is about to open.
func (m *Model) beginBlastRadius() {
	m.blast.radius = nil
	m.blast.loading = true
	m.blast.req++
}
