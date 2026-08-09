package app

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"
	policyv1 "k8s.io/api/policy/v1"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
)

// errNoBlastTarget means there is nothing to measure: no client, or an object
// with no selector to find pods by. The dialog drops the line and carries on.
var errNoBlastTarget = errors.New("blast radius is not available for this action")

// blastRadiusLoadedMsg carries the cost of the pending action. req numbers the
// fetch, because closing and reopening a confirm can leave an older reply in
// flight that would otherwise land on the new dialog.
type blastRadiusLoadedMsg struct {
	radius *k8s.BlastRadius
	req    uint64
	err    error

	// pods and pdbs are carried only for the scale overlay, which recomputes
	// from them as the user types instead of refetching per digit.
	pods []k8s.EvictedPod
	pdbs []policyv1.PodDisruptionBudget
}

// loadScaleBlastRadius fetches once when the scale overlay opens. The overlay
// then recomputes locally as the replica count is typed.
func (m Model) loadScaleBlastRadius() tea.Cmd {
	req := m.blast.req
	client := m.client
	ctxName := m.actionCtx.context
	namespace := m.actionCtx.namespace
	selector := workloadSelectorFrom(m.actionCtx.raw)
	if client == nil || selector == nil {
		// Still answer. beginBlastRadius already turned the spinner on, and a
		// silent return would leave the overlay reading "checking..." forever.
		return func() tea.Msg {
			return blastRadiusLoadedMsg{req: req, err: errNoBlastTarget}
		}
	}

	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindYAMLFetch,
		"Blast radius: "+m.actionCtx.name,
		bgtaskTarget(ctxName, namespace),
		func(ctx context.Context) tea.Msg {
			pods, err := client.PodsForSelector(ctx, ctxName, namespace, selector)
			if err != nil {
				return blastRadiusLoadedMsg{req: req, err: err}
			}
			pdbs, err := client.ListPodDisruptionBudgets(ctx, ctxName, namespace)
			if err != nil {
				return blastRadiusLoadedMsg{req: req, err: err}
			}
			return blastRadiusLoadedMsg{req: req, pods: pods, pdbs: pdbs}
		},
	)
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
		return func() tea.Msg { return blastRadiusLoadedMsg{req: req, err: errNoBlastTarget} }
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

// loadBulkBlastRadius totals what a whole selection costs, as one line rather
// than one per row. Cost is bounded by the number of distinct namespaces, not
// by how many rows are selected.
func (m Model) loadBulkBlastRadius() tea.Cmd {
	req := m.blast.req
	client := m.client
	ctxName := m.effectiveContext()
	byNS, uncounted := bulkPodTargets(m.bulkItems)
	if client == nil {
		return func() tea.Msg { return blastRadiusLoadedMsg{req: req, err: errNoBlastTarget} }
	}

	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindYAMLFetch,
		"Blast radius: selection",
		bgtaskTarget(ctxName, ""),
		func(ctx context.Context) tea.Msg {
			var evicting []k8s.EvictedPod
			for namespace, names := range byNS {
				pods, err := client.PodsInNamespace(ctx, ctxName, namespace)
				if err != nil {
					return blastRadiusLoadedMsg{req: req, err: err}
				}
				for _, p := range pods {
					if names[p.Name] {
						evicting = append(evicting, p.EvictedPod)
					}
				}
			}
			// One namespace keeps the budget list narrow; several make a
			// cluster-wide list cheaper than one call per namespace.
			pdbNamespace := ""
			if len(byNS) == 1 {
				for namespace := range byNS {
					pdbNamespace = namespace
				}
			}
			pdbs, err := client.ListPodDisruptionBudgets(ctx, ctxName, pdbNamespace)
			if err != nil {
				return blastRadiusLoadedMsg{req: req, err: err}
			}
			radius := k8s.ComputeBlastRadius(evicting, pdbs, 0)
			radius.Uncounted = uncounted
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
	m.blast.pods = msg.pods
	m.blast.pdbs = msg.pdbs
	return m, nil
}

// beginBlastRadius arms the line for a confirm that is about to open.
func (m *Model) beginBlastRadius() {
	m.blast.radius = nil
	m.blast.loading = true
	m.blast.req++
}
