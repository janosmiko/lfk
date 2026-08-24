package app

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

// sparklineColumnCap is the width a CPU or MEM cell must fit inside. It is the
// non-fullscreen column cap from fitExtraColumns, used even in fullscreen
// (where the cap is 40) because the cell string is built during metrics
// enrichment, before the layout knows which mode the terminal is in. Sizing to
// the narrower of the two guarantees the value survives in both, since
// fitExtraColumns truncates the tail and the value sits at the end.
const sparklineColumnCap = 20

// metricsSeriesCache holds the CPU and memory history the current sparkline
// mode draws from, keyed the same way the instant metrics maps are:
// "namespace/pod" for pods, node name for nodes.
type metricsSeriesCache struct {
	// cpu and mem hold either pod or node series depending on which list is
	// active, never both: a "namespace/pod" key always contains a slash and a
	// node name never does, so the two key spaces cannot collide.
	cpu map[string]k8s.MetricSeries
	mem map[string]k8s.MetricSeries

	// clusterCPU and clusterMem are keyed by context so a union dashboard
	// member switch cannot draw one member's history under another's numbers
	// on a cursor move, the way an unkeyed single series would.
	clusterCPU map[string]k8s.MetricSeries
	clusterMem map[string]k8s.MetricSeries
}

// podMetricsRangeMsg carries a pod CPU/memory history fetch back to the update
// loop. err non-nil, or both maps empty, returns the columns to numeric.
type podMetricsRangeMsg struct {
	cpu map[string]k8s.MetricSeries
	mem map[string]k8s.MetricSeries
	gen uint64
	err error
}

// sparklineCell renders "<sparkline> <value>", or value alone when there is no
// series or no room for one.
//
// The glyph count is bounded by what is left of sparklineColumnCap after the
// value and its separating space, so the value is never truncated. Below
// MinSparklineWidth glyphs the cell stays numeric: a two-glyph sparkline reads
// as noise rather than as a trend.
func sparklineCell(series k8s.MetricSeries, value string) string {
	room := sparklineColumnCap - lipgloss.Width(value) - 1
	width := min(ui.ConfigSparklineWidth, room)
	if width < ui.MinSparklineWidth {
		return value
	}
	spark := ui.RenderSparkline(series.Points, width)
	if spark == "" {
		return value
	}
	return spark + " " + value
}

// loadPodMetricsRangeForList fetches pod CPU and memory history for the
// current sparkline window.
func (m Model) loadPodMetricsRangeForList() tea.Cmd {
	// See loadPodMetricsForList: the union sentinel is not a real cluster.
	if m.isUnionSentinel() {
		return nil
	}
	window := m.metricsSpark.Window()
	if window <= 0 {
		return nil
	}
	// One sample per drawn column: querying finer than the terminal can draw
	// costs Prometheus read work that the renderer throws away.
	points := ui.ClampSparklineWidth(ui.ConfigSparklineWidth)
	step := window / time.Duration(points)

	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	gen := m.requestGen
	client := m.client
	return m.scheduleK8sCall(
		scheduler.PriorityLow,
		scheduler.KindMetrics,
		"Pod metrics history",
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			cpu, mem, err := client.GetPodMetricsRange(ctx, kctx, ns, window, step)
			if err != nil {
				logger.WarnOnce("pod-metrics-range-load", kctx+"/"+ns,
					"pod metrics history unavailable: no Prometheus source",
					"context", kctx, "namespace", ns, "error", logger.Redact(err.Error()))
				return podMetricsRangeMsg{gen: gen, err: err}
			}
			return podMetricsRangeMsg{cpu: cpu, mem: mem, gen: gen}
		},
	)
}

// updatePodMetricsRange stores a history fetch, or returns the columns to
// numeric when Prometheus produced nothing. Reverting rather than showing a
// placeholder keeps a cluster with no Prometheus free of layout shift.
//
// The returned Cmd re-runs the instant fetch: only updatePodMetricsEnriched
// builds the cell strings, so without it the new series sits unused until
// the next throttled tick.
func (m Model) updatePodMetricsRange(msg podMetricsRangeMsg) (Model, tea.Cmd) {
	if msg.gen != m.requestGen {
		return m, nil // stale response, and the mode may have moved on since
	}
	if msg.err != nil || (len(msg.cpu) == 0 && len(msg.mem) == 0) {
		m.metricsSpark = ui.MetricsSparkState{}
		m.metricsSeries = metricsSeriesCache{}
		m.setStatusMessage("CPU/MEM history needs Prometheus, showing values", true)
		return m, nil
	}
	m.metricsSeries = metricsSeriesCache{cpu: msg.cpu, mem: msg.mem}
	m.middleItemsRev++
	return m, m.loadPodMetricsForList()
}

// nodeMetricsRangeMsg carries a node CPU/memory history fetch back to the
// update loop. See podMetricsRangeMsg for the fallback contract.
type nodeMetricsRangeMsg struct {
	cpu map[string]k8s.MetricSeries
	mem map[string]k8s.MetricSeries
	gen uint64
	err error
}

// loadNodeMetricsRangeForList fetches node CPU and memory history for the
// current sparkline window. The node list is not namespaced, so it passes ""
// where loadPodMetricsRangeForList passes the effective namespace.
func (m Model) loadNodeMetricsRangeForList() tea.Cmd {
	if m.isUnionSentinel() {
		return nil
	}
	window := m.metricsSpark.Window()
	if window <= 0 {
		return nil
	}
	points := ui.ClampSparklineWidth(ui.ConfigSparklineWidth)
	step := window / time.Duration(points)

	kctx := m.nav.Context
	gen := m.requestGen
	client := m.client
	return m.scheduleK8sCall(
		scheduler.PriorityLow,
		scheduler.KindMetrics,
		"Node metrics history",
		bgtaskTarget(kctx, ""),
		func(ctx context.Context) tea.Msg {
			cpu, mem, err := client.GetNodeMetricsRange(ctx, kctx, window, step)
			if err != nil {
				logger.WarnOnce("node-metrics-range-load", kctx,
					"node metrics history unavailable: no Prometheus source",
					"context", kctx, "error", logger.Redact(err.Error()))
				return nodeMetricsRangeMsg{gen: gen, err: err}
			}
			return nodeMetricsRangeMsg{cpu: cpu, mem: mem, gen: gen}
		},
	)
}

// updateNodeMetricsRange stores a history fetch, or returns the columns to
// numeric when Prometheus produced nothing. See updatePodMetricsRange for why
// it reverts rather than showing a placeholder, and why it returns a Cmd.
func (m Model) updateNodeMetricsRange(msg nodeMetricsRangeMsg) (Model, tea.Cmd) {
	if msg.gen != m.requestGen {
		return m, nil // stale response, and the mode may have moved on since
	}
	if msg.err != nil || (len(msg.cpu) == 0 && len(msg.mem) == 0) {
		m.metricsSpark = ui.MetricsSparkState{}
		m.metricsSeries = metricsSeriesCache{}
		m.setStatusMessage("CPU/MEM history needs Prometheus, showing values", true)
		return m, nil
	}
	m.metricsSeries = metricsSeriesCache{cpu: msg.cpu, mem: msg.mem}
	m.middleItemsRev++
	return m, m.loadNodeMetricsForList()
}

// clusterMetricsRangeMsg carries a cluster history fetch back to the update
// loop. context is the target cluster the fetch was for, since a union
// dashboard member switch does not wait for the previous member's fetch to
// land. See podMetricsRangeMsg for the fallback contract.
type clusterMetricsRangeMsg struct {
	context string
	cpu     k8s.MetricSeries
	mem     k8s.MetricSeries
	gen     uint64
	err     error
}

// loadClusterMetricsRangeForDashboard fetches cluster-wide CPU and memory
// history for the CLUSTER RESOURCES sparklines, for the target context kctx
// (the union dashboard's preview target, or the active context, never the
// union sentinel, since both callers resolve a real context first).
func (m Model) loadClusterMetricsRangeForDashboard(kctx string) tea.Cmd {
	window := m.metricsSpark.Window()
	if window <= 0 {
		return nil
	}
	points := ui.ClampSparklineWidth(ui.ConfigSparklineWidth)
	step := window / time.Duration(points)

	gen := m.requestGen
	client := m.client
	return m.scheduleK8sCall(
		scheduler.PriorityLow,
		scheduler.KindMetrics,
		"Cluster metrics history",
		bgtaskTarget(kctx, ""),
		func(ctx context.Context) tea.Msg {
			cpu, mem, err := client.GetClusterMetricsRange(ctx, kctx, window, step)
			if err != nil {
				logger.WarnOnce("cluster-metrics-range-load", kctx,
					"cluster metrics history unavailable: no Prometheus source",
					"context", kctx, "error", logger.Redact(err.Error()))
				return clusterMetricsRangeMsg{context: kctx, gen: gen, err: err}
			}
			return clusterMetricsRangeMsg{context: kctx, cpu: cpu, mem: mem, gen: gen}
		},
	)
}

// updateClusterMetricsRange stores a history fetch under its own context key,
// or clears that context's entry when Prometheus produced nothing. See
// updatePodMetricsRange for why it reverts rather than showing a placeholder.
// Only msg.context's entry is touched, leaving other contexts' cluster
// history and the pod/node row maps alone. Both branches end in
// recomposeDashboard, since dashboardPreview is a cached string.
func (m Model) updateClusterMetricsRange(msg clusterMetricsRangeMsg) Model {
	if msg.gen != m.requestGen {
		return m // stale response, and the mode may have moved on since
	}
	if msg.err != nil || (len(msg.cpu.Points) == 0 && len(msg.mem.Points) == 0) {
		m.metricsSpark = ui.MetricsSparkState{}
		delete(m.metricsSeries.clusterCPU, msg.context)
		delete(m.metricsSeries.clusterMem, msg.context)
		m.setStatusMessage("CPU/MEM history needs Prometheus, showing values", true)
		return m.recomposeDashboard()
	}
	if m.metricsSeries.clusterCPU == nil {
		m.metricsSeries.clusterCPU = make(map[string]k8s.MetricSeries)
	}
	if m.metricsSeries.clusterMem == nil {
		m.metricsSeries.clusterMem = make(map[string]k8s.MetricSeries)
	}
	m.metricsSeries.clusterCPU[msg.context] = msg.cpu
	m.metricsSeries.clusterMem[msg.context] = msg.mem
	return m.recomposeDashboard()
}

// loadMetricsRangeForKind picks the history loader for kind. A kind with no
// CPU/MEM columns returns nil, so the hotkey is inert there rather than
// firing a query whose result nothing draws.
func (m Model) loadMetricsRangeForKind(kind string) tea.Cmd {
	switch kind {
	case "Pod":
		return m.loadPodMetricsRangeForList()
	case "Node":
		return m.loadNodeMetricsRangeForList()
	case "Cluster":
		return m.loadClusterMetricsRangeForDashboard(m.dashboardPreviewTargetContext())
	default:
		return nil
	}
}

// loadInstantMetricsForKind picks the list-wide enrichment loader for kind,
// used to repaint CPU/MEM cells right away on a mode change instead of
// waiting for the next throttled tick. Cluster has no entry: composeDashboard
// already redraws from cached data via recomposeDashboard.
func (m Model) loadInstantMetricsForKind(kind string) tea.Cmd {
	switch kind {
	case "Pod":
		return m.loadPodMetricsForList()
	case "Node":
		return m.loadNodeMetricsForList()
	default:
		return nil
	}
}
