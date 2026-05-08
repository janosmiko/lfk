package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
)

// loadMetrics triggers async metrics loading for the current resource.
func (m Model) loadMetrics() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	kctx := m.nav.Context
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	gen := m.requestGen
	client := m.client

	kind := m.nav.ResourceType.Kind
	if m.nav.Level == model.LevelOwned {
		kind = sel.Kind
	}

	switch kind {
	case "Pod":
		podName := sel.Name
		return m.scheduleK8sCall(
			scheduler.PriorityLow,
			scheduler.KindMetrics,
			"Metrics: Pod/"+podName,
			bgtaskTarget(kctx, ns),
			func(ctx context.Context) tea.Msg {
				pm, err := client.GetPodMetrics(ctx, kctx, ns, podName)
				if err != nil {
					return metricsLoadedMsg{gen: gen} // silently ignore
				}
				cpuReq, cpuLim, memReq, memLim, err := client.GetPodResourceRequests(ctx, kctx, ns, podName)
				if err != nil {
					cpuReq, cpuLim, memReq, memLim = 0, 0, 0, 0
				}
				return metricsLoadedMsg{
					cpuUsed: pm.CPU, cpuReq: cpuReq, cpuLim: cpuLim,
					memUsed: pm.Memory, memReq: memReq, memLim: memLim,
					gen: gen,
				}
			},
		)
	case "Deployment", "StatefulSet", "DaemonSet":
		name := sel.Name
		return m.scheduleK8sCall(
			scheduler.PriorityLow,
			scheduler.KindMetrics,
			"Metrics: "+kind+"/"+name,
			bgtaskTarget(kctx, ns),
			func(ctx context.Context) tea.Msg {
				// Get child pods.
				childItems, err := client.GetOwnedResources(ctx, kctx, ns, kind, name)
				if err != nil || len(childItems) == 0 {
					return metricsLoadedMsg{gen: gen}
				}
				var podNames []string
				for _, item := range childItems {
					if item.Kind == "Pod" {
						podNames = append(podNames, item.Name)
					}
				}
				if len(podNames) == 0 {
					return metricsLoadedMsg{gen: gen}
				}
				metrics, err := client.GetPodsMetrics(ctx, kctx, ns, podNames)
				if err != nil || len(metrics) == 0 {
					return metricsLoadedMsg{gen: gen}
				}

				var totalCPU, totalMem int64
				for _, pm := range metrics {
					totalCPU += pm.CPU
					totalMem += pm.Memory
				}

				// Sum requests/limits from all pods.
				var totalCPUReq, totalCPULim, totalMemReq, totalMemLim int64
				for _, podName := range podNames {
					cpuReq, cpuLim, memReq, memLim, err := client.GetPodResourceRequests(ctx, kctx, ns, podName)
					if err != nil {
						continue
					}
					totalCPUReq += cpuReq
					totalCPULim += cpuLim
					totalMemReq += memReq
					totalMemLim += memLim
				}

				return metricsLoadedMsg{
					cpuUsed: totalCPU, cpuReq: totalCPUReq, cpuLim: totalCPULim,
					memUsed: totalMem, memReq: totalMemReq, memLim: totalMemLim,
					gen: gen,
				}
			},
		)
	}
	return nil
}

// loadPreviewEvents loads events for the currently selected resource to display
// in the preview pane below RESOURCE USAGE.
func (m Model) loadPreviewEvents() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	kctx := m.nav.Context
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	gen := m.requestGen
	client := m.client
	name := sel.Name

	kind := m.nav.ResourceType.Kind
	if m.nav.Level == model.LevelOwned {
		kind = sel.Kind
	}

	return m.scheduleK8sCall(
		scheduler.PriorityLow,
		scheduler.KindResourceList,
		"Preview events: "+name,
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			events, err := client.GetResourceEvents(ctx, kctx, ns, name, kind)
			if err != nil {
				return previewEventsLoadedMsg{gen: gen}
			}
			return previewEventsLoadedMsg{events: events, gen: gen}
		},
	)
}

// loadPodMetricsForList fetches metrics for all pods in the current namespace
// and returns them to enrich the middle pane items.
func (m Model) loadPodMetricsForList() tea.Cmd {
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	gen := m.requestGen
	client := m.client
	return m.scheduleK8sCall(
		scheduler.PriorityLow,
		scheduler.KindMetrics,
		"Pod metrics",
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			metrics, err := client.GetAllPodMetrics(ctx, kctx, ns)
			if err != nil {
				return podMetricsEnrichedMsg{gen: gen} // silently ignore
			}
			return podMetricsEnrichedMsg{metrics: metrics, gen: gen}
		},
	)
}

// loadNodeMetricsForList fetches metrics for all nodes and returns them
// to enrich the middle pane items with CPU/MEM usage columns.
func (m Model) loadNodeMetricsForList() tea.Cmd {
	kctx := m.nav.Context
	gen := m.requestGen
	client := m.client
	return m.scheduleK8sCall(
		scheduler.PriorityLow,
		scheduler.KindMetrics,
		"Node metrics",
		bgtaskTarget(kctx, ""),
		func(ctx context.Context) tea.Msg {
			metrics, err := client.GetAllNodeMetrics(ctx, kctx)
			if err != nil {
				return nodeMetricsEnrichedMsg{gen: gen}
			}
			return nodeMetricsEnrichedMsg{metrics: metrics, gen: gen}
		},
	)
}
