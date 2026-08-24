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
// "namespace/pod" for pods, node name for nodes, container name for
// containers.
type metricsSeriesCache struct {
	cpu map[string]k8s.MetricSeries
	mem map[string]k8s.MetricSeries
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
// numeric when Prometheus produced nothing.
//
// Reverting rather than showing a placeholder is deliberate: the columns keep
// exactly the rendering they have without this feature, so a cluster with no
// Prometheus sees no layout shift and no empty glyph cells.
func (m Model) updatePodMetricsRange(msg podMetricsRangeMsg) Model {
	if msg.gen != m.requestGen {
		return m // stale response, and the mode may have moved on since
	}
	if msg.err != nil || (len(msg.cpu) == 0 && len(msg.mem) == 0) {
		m.metricsSpark = ui.MetricsSparkState{}
		m.metricsSeries = metricsSeriesCache{}
		m.setStatusMessage("CPU/MEM history needs Prometheus, showing values", true)
		return m
	}
	m.metricsSeries = metricsSeriesCache{cpu: msg.cpu, mem: msg.mem}
	m.middleItemsRev++
	return m
}
