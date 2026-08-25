package app

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// loadMetrics triggers async metrics loading for the current resource.
func (m Model) loadMetrics() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	kctx := m.effectiveContext()
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
					logger.WarnOnce("pod-metrics-load", kctx+"/"+ns+"/"+podName,
						"pod metrics load failed", "context", kctx, "namespace", ns, "pod", podName, "error", logger.Redact(err.Error()))
					return metricsLoadedMsg{gen: gen}
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
				if err != nil {
					logger.WarnOnce("owned-resources-load", kctx+"/"+ns+"/"+kind+"/"+name,
						"owned resources load failed", "context", kctx, "namespace", ns, "kind", kind, "name", name, "error", logger.Redact(err.Error()))
					return metricsLoadedMsg{gen: gen}
				}
				if len(childItems) == 0 {
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
				if err != nil {
					logger.WarnOnce("pods-metrics-load", kctx+"/"+ns+"/"+kind+"/"+name,
						"workload pod metrics load failed", "context", kctx, "namespace", ns, "kind", kind, "name", name, "error", logger.Redact(err.Error()))
					return metricsLoadedMsg{gen: gen}
				}
				if len(metrics) == 0 {
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

	kctx := m.effectiveContext()
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
				logger.WarnOnce("preview-events-load", kctx+"/"+ns+"/"+kind+"/"+name,
					"preview events load failed", "context", kctx, "namespace", ns, "kind", kind, "name", name, "error", logger.Redact(err.Error()))
				return previewEventsLoadedMsg{gen: gen}
			}
			return previewEventsLoadedMsg{events: events, gen: gen}
		},
	)
}

// loadPodMetricsForList fetches metrics for all pods in the current namespace
// and returns them to enrich the middle pane items.
func (m Model) loadPodMetricsForList() tea.Cmd {
	// At the union sentinel, list-wide metrics enrichment would target the
	// sentinel string instead of a real cluster. Skip rather than fan out:
	// the merged list spans clusters that may not all run metrics-server,
	// and a per-row metrics column is not part of the union view's contract.
	if m.isUnionSentinel() {
		return nil
	}
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
				logger.WarnOnce("pod-metrics-list-load", kctx+"/"+ns,
					"pod metrics unavailable: no metrics-server or Prometheus source", "context", kctx, "namespace", ns, "error", logger.Redact(err.Error()))
				return podMetricsEnrichedMsg{gen: gen}
			}
			return podMetricsEnrichedMsg{metrics: metrics, gen: gen}
		},
	)
}

// loadNodeMetricsForList fetches metrics for all nodes and returns them
// to enrich the middle pane items with CPU/MEM usage columns.
func (m Model) loadNodeMetricsForList() tea.Cmd {
	// See loadPodMetricsForList: skip at the union sentinel.
	if m.isUnionSentinel() {
		return nil
	}
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
				logger.WarnOnce("node-metrics-list-load", kctx,
					"node metrics list load failed", "context", kctx, "error", logger.Redact(err.Error()))
				return nodeMetricsEnrichedMsg{gen: gen}
			}
			return nodeMetricsEnrichedMsg{metrics: metrics, gen: gen}
		},
	)
}

// loadContainerMetricsForList fetches per-container CPU/MEM usage, keyed by
// container name. Namespace/pod resolution mirrors loadContainers exactly.
func (m Model) loadContainerMetricsForList() tea.Cmd {
	// See loadPodMetricsForList: skip at the union sentinel.
	if m.isUnionSentinel() {
		return nil
	}
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	if ns == "" && m.nav.Namespace != "" {
		ns = m.nav.Namespace
	}
	podName := m.nav.OwnedName
	gen := m.requestGen
	client := m.client
	return m.scheduleK8sCall(
		scheduler.PriorityLow,
		scheduler.KindMetrics,
		"Container metrics",
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			metrics, err := client.GetPodContainerMetrics(ctx, kctx, ns, podName)
			if err != nil {
				logger.WarnOnce("container-metrics-list-load", kctx+"/"+ns+"/"+podName,
					"container metrics unavailable: no metrics-server or Prometheus source", "context", kctx, "namespace", ns, "pod", podName, "error", logger.Redact(err.Error()))
				return containerMetricsEnrichedMsg{gen: gen}
			}
			return containerMetricsEnrichedMsg{metrics: metrics, gen: gen}
		},
	)
}

// loadNodeUptimeForList fetches Prometheus node uptimes and returns them to
// enrich the middle pane items with an Uptime column. A nil/empty result is
// not an error: it means Prometheus isn't the configured monitoring source.
func (m Model) loadNodeUptimeForList() tea.Cmd {
	// See loadPodMetricsForList: skip at the union sentinel.
	if m.isUnionSentinel() {
		return nil
	}
	kctx := m.nav.Context
	gen := m.requestGen
	client := m.client
	return m.scheduleK8sCall(
		scheduler.PriorityLow,
		scheduler.KindMetrics,
		"Node uptime",
		bgtaskTarget(kctx, ""),
		func(ctx context.Context) tea.Msg {
			uptimes, err := client.GetNodeUptimes(ctx, kctx)
			if err != nil {
				logger.WarnOnce("node-uptime-list-load", kctx,
					"node uptime list load failed", "context", kctx, "error", logger.Redact(err.Error()))
				return nodeUptimeEnrichedMsg{gen: gen}
			}
			return nodeUptimeEnrichedMsg{uptimes: uptimes, gen: gen}
		},
	)
}
