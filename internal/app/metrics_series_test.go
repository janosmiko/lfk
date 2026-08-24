package app

import (
	"math"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestSparklineCell_PrependsSparklineToValue(t *testing.T) {
	t.Cleanup(func() { ui.ConfigSparklineWidth = ui.DefaultSparklineWidth })
	ui.ConfigSparklineWidth = 5

	got := sparklineCell(k8s.MetricSeries{Points: []float64{1, 2, 3, 4, 5}}, "240m")

	assert.Equal(t, "▁▃▅▆█ 240m", got)
}

// The value is what the user reads. It must never be pushed past the column
// cap, because fitExtraColumns truncates the tail.
func TestSparklineCell_NeverExceedsTheColumnCap(t *testing.T) {
	t.Cleanup(func() { ui.ConfigSparklineWidth = ui.DefaultSparklineWidth })
	ui.ConfigSparklineWidth = ui.MaxSparklineWidth

	got := sparklineCell(k8s.MetricSeries{Points: []float64{1, 2, 3}}, "1024Mi")

	require.LessOrEqual(t, lipgloss.Width(got), sparklineColumnCap,
		"cell %q is wider than the column cap and would truncate the value", got)
	assert.True(t, strings.HasSuffix(got, "1024Mi"), "the value must survive intact")
}

// A value so wide that fewer than MinSparklineWidth glyphs fit gets no
// sparkline at all, rather than a two-glyph stub that reads as noise.
func TestSparklineCell_TooLittleRoomStaysNumeric(t *testing.T) {
	got := sparklineCell(k8s.MetricSeries{Points: []float64{1, 2, 3}}, strings.Repeat("9", 18))

	assert.Equal(t, strings.Repeat("9", 18), got)
}

func TestSparklineCell_EmptySeriesStaysNumeric(t *testing.T) {
	assert.Equal(t, "240m", sparklineCell(k8s.MetricSeries{}, "240m"))
	assert.Equal(t, "240m", sparklineCell(k8s.MetricSeries{Points: []float64{math.NaN()}}, "240m"))
}

// A cell built in sparkline mode must still sort by its number, which is the
// whole reason stripValueDecoration exists.
func TestSparklineCell_StillParsesForSort(t *testing.T) {
	t.Cleanup(func() { ui.ConfigSparklineWidth = ui.DefaultSparklineWidth })
	ui.ConfigSparklineWidth = 6

	cell := sparklineCell(k8s.MetricSeries{Points: []float64{1, 9, 3}}, "240m")

	v, ok := ui.ParseResourceValueOK(cell, true)
	require.True(t, ok, "cell %q must parse", cell)
	assert.Equal(t, int64(240), v)
}

func TestUpdatePodMetricsRange_FallsBackToNumericOnError(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m, cmd := m.updatePodMetricsRange(podMetricsRangeMsg{gen: m.requestGen, err: assert.AnError})

	assert.Equal(t, ui.MetricsDisplayNumeric, m.metricsSpark.Mode,
		"a failed range query must return the columns to numeric")
	assert.NotEmpty(t, m.statusMessage, "the user must be told once why the mode reverted")
	assert.Nil(t, m.metricsSeries.cpu)
	assert.Nil(t, cmd, "a fallback must not also fire the instant loader")
}

// An empty result is the "Prometheus is there but has no data for these pods"
// case. It reverts too, because a sparkline mode with no series draws nothing.
func TestUpdatePodMetricsRange_EmptyResultFallsBack(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m, _ = m.updatePodMetricsRange(podMetricsRangeMsg{gen: m.requestGen})

	assert.Equal(t, ui.MetricsDisplayNumeric, m.metricsSpark.Mode)
}

func TestUpdatePodMetricsRange_StoresSeriesAndKeepsMode(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m, cmd := m.updatePodMetricsRange(podMetricsRangeMsg{
		gen: m.requestGen,
		cpu: map[string]k8s.MetricSeries{"default/api-1": {Points: []float64{1, 2}}},
		mem: map[string]k8s.MetricSeries{"default/api-1": {Points: []float64{3, 4}}},
	})

	assert.Equal(t, ui.MetricsDisplaySpark, m.metricsSpark.Mode)
	assert.Equal(t, []float64{1, 2}, m.metricsSeries.cpu["default/api-1"].Points)
	assert.NotNil(t, cmd, "a new series must trigger an instant repaint of the cells")
}

// A metrics-less row must keep rendering exactly "n/a" in sparkline mode, with
// no glyph prefix. metricValueMissing (internal/app/tabs_compare.go) matches
// the RAW string "n/a" to push metrics-less rows to the bottom of a sort in
// both directions. A cell rendered as glyphs followed by "n/a" would stop being
// recognised, and those rows would silently start interleaving with real
// values. clearStalePodMetricsColumns already sets a bare "n/a" and
// sparklineCell is never reached for such a row, so this asserts the existing
// routing rather than adding behaviour - but nothing else pins it.
//
// Assert the value directly rather than looping over columns and checking
// inside an if: a loop whose body never runs passes vacuously, and an absent
// CPU column is exactly the failure this test should catch. getColumnValue
// returns "" for a missing column, which fails the comparison.
func TestUpdatePodMetricsEnriched_SparkModeLeavesMissingRowsAsNA(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}
	// Seed history under the metrics-less row's OWN key, deliberately. A
	// fixture that seeds only some other key cannot catch the likely
	// regression: if the continue guard were removed, the sparklineCell
	// lookup would miss, fall back to the bare value, and the test would
	// still pass. Seeding the row's own key means such a regression renders
	// glyphs and fails.
	m.metricsSeries = metricsSeriesCache{
		cpu: map[string]k8s.MetricSeries{"default/no-metrics": {Points: []float64{1, 9}}},
		mem: map[string]k8s.MetricSeries{"default/no-metrics": {Points: []float64{3, 7}}},
	}
	m.middleItems = []model.Item{{Name: "no-metrics", Namespace: "default"}}

	got := m.updatePodMetricsEnriched(podMetricsEnrichedMsg{gen: m.requestGen})

	assert.Equal(t, "n/a", getColumnValue(got.middleItems[0], "CPU"),
		"CPU must stay exactly n/a in sparkline mode, with no glyph prefix")
	assert.Equal(t, "n/a", getColumnValue(got.middleItems[0], "MEM"),
		"MEM must stay exactly n/a in sparkline mode, with no glyph prefix")
}

func TestUpdatePodMetricsRange_IgnoresStaleGeneration(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m, _ = m.updatePodMetricsRange(podMetricsRangeMsg{gen: m.requestGen + 1, err: assert.AnError})

	assert.Equal(t, ui.MetricsDisplaySpark, m.metricsSpark.Mode,
		"a stale response must not revert the mode the user just chose")
}

func TestUpdateNodeMetricsRange_FallsBackToNumericOnError(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m, cmd := m.updateNodeMetricsRange(nodeMetricsRangeMsg{gen: m.requestGen, err: assert.AnError})

	assert.Equal(t, ui.MetricsDisplayNumeric, m.metricsSpark.Mode,
		"a failed range query must return the columns to numeric")
	assert.NotEmpty(t, m.statusMessage, "the user must be told once why the mode reverted")
	assert.Nil(t, m.metricsSeries.cpu)
	assert.Nil(t, cmd, "a fallback must not also fire the instant loader")
}

// An empty result is the "Prometheus is there but has no data for these
// nodes" case. It reverts too, because a sparkline mode with no series draws
// nothing.
func TestUpdateNodeMetricsRange_EmptyResultFallsBack(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m, _ = m.updateNodeMetricsRange(nodeMetricsRangeMsg{gen: m.requestGen})

	assert.Equal(t, ui.MetricsDisplayNumeric, m.metricsSpark.Mode)
}

func TestUpdateNodeMetricsRange_StoresSeries(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m, cmd := m.updateNodeMetricsRange(nodeMetricsRangeMsg{
		gen: m.requestGen,
		cpu: map[string]k8s.MetricSeries{"node-1": {Points: []float64{1, 2}}},
	})

	assert.Equal(t, ui.MetricsDisplaySpark, m.metricsSpark.Mode)
	assert.Equal(t, []float64{1, 2}, m.metricsSeries.cpu["node-1"].Points)
	assert.NotNil(t, cmd, "a new series must trigger an instant repaint of the cells")
}

func TestUpdateNodeMetricsRange_IgnoresStaleGeneration(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m, _ = m.updateNodeMetricsRange(nodeMetricsRangeMsg{gen: m.requestGen + 1, err: assert.AnError})

	assert.Equal(t, ui.MetricsDisplaySpark, m.metricsSpark.Mode,
		"a stale response must not revert the mode the user just chose")
}

// A metrics-less node row must keep rendering exactly "n/a" in sparkline
// mode, with no glyph prefix. See
// TestUpdatePodMetricsEnriched_SparkModeLeavesMissingRowsAsNA for why the
// value is asserted directly rather than inside a conditional.
func TestUpdateNodeMetricsEnriched_SparkModeLeavesMissingRowsAsNA(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}
	m.metricsSeries = metricsSeriesCache{
		cpu: map[string]k8s.MetricSeries{"no-metrics": {Points: []float64{1, 9}}},
		mem: map[string]k8s.MetricSeries{"no-metrics": {Points: []float64{3, 7}}},
	}
	m.middleItems = []model.Item{{Name: "no-metrics"}}

	got := m.updateNodeMetricsEnriched(nodeMetricsEnrichedMsg{gen: m.requestGen})

	assert.Equal(t, "n/a", getColumnValue(got.middleItems[0], "CPU"),
		"CPU must stay exactly n/a in sparkline mode, with no glyph prefix")
	assert.Equal(t, "n/a", getColumnValue(got.middleItems[0], "MEM"),
		"MEM must stay exactly n/a in sparkline mode, with no glyph prefix")
}

// The node list is not namespaced, so the lookup key is the bare node name.
func TestUpdateNodeMetricsEnriched_SparkModeKeysByNodeName(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}
	ui.ConfigSparklineWidth = 5
	t.Cleanup(func() { ui.ConfigSparklineWidth = ui.DefaultSparklineWidth })
	m.metricsSeries = metricsSeriesCache{
		cpu: map[string]k8s.MetricSeries{"node-1": {Points: []float64{1, 2, 3, 4, 5}}},
		mem: map[string]k8s.MetricSeries{"node-1": {Points: []float64{1, 2, 3, 4, 5}}},
	}
	m.middleItems = []model.Item{{Name: "node-1"}}

	got := m.updateNodeMetricsEnriched(nodeMetricsEnrichedMsg{
		gen:     m.requestGen,
		metrics: map[string]model.PodMetrics{"node-1": {CPU: 240, Memory: 1024}},
	})

	cpu := getColumnValue(got.middleItems[0], "CPU")
	assert.NotEqual(t, "n/a", cpu)
	assert.Contains(t, cpu, "▁", "the sparkline glyphs must replace the trend arrow, not stack with it")
}

func TestLoadMetricsRangeForKind_DispatchesByKind(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	assert.NotNil(t, m.loadMetricsRangeForKind("Pod"), "Pod has CPU/MEM columns and must load history")
	assert.NotNil(t, m.loadMetricsRangeForKind("Node"), "Node has CPU/MEM columns and must load history")
	assert.NotNil(t, m.loadMetricsRangeForKind("Cluster"), "Cluster dashboard has a CPU/Mem section and must load history")
	assert.Nil(t, m.loadMetricsRangeForKind("Deployment"), "a kind with no CPU/MEM columns must be inert")
}

func TestUpdateClusterMetricsRange_FallsBackToNumericOnError(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m = m.updateClusterMetricsRange(clusterMetricsRangeMsg{gen: m.requestGen, err: assert.AnError})

	assert.Equal(t, ui.MetricsDisplayNumeric, m.metricsSpark.Mode)
}

func TestUpdateClusterMetricsRange_StoresSeries(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m = m.updateClusterMetricsRange(clusterMetricsRangeMsg{
		context: m.nav.Context,
		gen:     m.requestGen,
		cpu:     k8s.MetricSeries{Points: []float64{1, 5, 9}},
	})

	assert.Equal(t, ui.MetricsDisplaySpark, m.metricsSpark.Mode)
	assert.Equal(t, []float64{1, 5, 9}, m.metricsSeries.clusterCPU[m.nav.Context].Points)
}

// A union member switch must not lose the previous member's cached history:
// unkeyed clusterCPU/clusterMem could only ever hold one context's series, so
// hovering member A then B would draw A's history under B's numbers.
func TestUpdateClusterMetricsRange_KeyedByContext_DoesNotClobberOtherContext(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m = m.updateClusterMetricsRange(clusterMetricsRangeMsg{
		context: "member-a", gen: m.requestGen, cpu: k8s.MetricSeries{Points: []float64{1, 1, 1}},
	})
	m = m.updateClusterMetricsRange(clusterMetricsRangeMsg{
		context: "member-b", gen: m.requestGen, cpu: k8s.MetricSeries{Points: []float64{9, 9, 9}},
	})

	assert.Equal(t, []float64{1, 1, 1}, m.metricsSeries.clusterCPU["member-a"].Points,
		"member-b's fetch must not overwrite member-a's cached history")
	assert.Equal(t, []float64{9, 9, 9}, m.metricsSeries.clusterCPU["member-b"].Points)
}

// Unlike the pod/node fallback, only the failed context's cluster entry is
// cleared: a fallback for one member must not discard a pod or node mode's
// row maps, nor another member's still-good cluster history.
func TestUpdateClusterMetricsRange_EmptyResultFallsBackAndKeepsOtherSeries(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}
	m.metricsSeries = metricsSeriesCache{
		cpu: map[string]k8s.MetricSeries{"default/api-1": {Points: []float64{1, 2}}},
		mem: map[string]k8s.MetricSeries{"default/api-1": {Points: []float64{3, 4}}},
		clusterCPU: map[string]k8s.MetricSeries{
			m.nav.Context:  {Points: []float64{1, 2}},
			"other-member": {Points: []float64{7, 7}},
		},
		clusterMem: map[string]k8s.MetricSeries{m.nav.Context: {Points: []float64{3, 4}}},
	}

	m = m.updateClusterMetricsRange(clusterMetricsRangeMsg{context: m.nav.Context, gen: m.requestGen})

	assert.Equal(t, ui.MetricsDisplayNumeric, m.metricsSpark.Mode)
	assert.Empty(t, m.metricsSeries.clusterCPU[m.nav.Context].Points)
	assert.Empty(t, m.metricsSeries.clusterMem[m.nav.Context].Points)
	assert.Equal(t, []float64{7, 7}, m.metricsSeries.clusterCPU["other-member"].Points,
		"a fallback for one member must not discard another member's cluster history")
	assert.Equal(t, []float64{1, 2}, m.metricsSeries.cpu["default/api-1"].Points,
		"a cluster fallback must not discard the pod/node row maps sharing this cache")
	assert.Equal(t, []float64{3, 4}, m.metricsSeries.mem["default/api-1"].Points)
}

func TestUpdateClusterMetricsRange_IgnoresStaleGeneration(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m = m.updateClusterMetricsRange(clusterMetricsRangeMsg{gen: m.requestGen + 1, err: assert.AnError})

	assert.Equal(t, ui.MetricsDisplaySpark, m.metricsSpark.Mode,
		"a stale response must not revert the mode the user just chose")
}

// A union dashboard member's context is real, but m.nav.Context is the union
// sentinel while browsing the member list. The loader must target the
// member's own context rather than bail on the sentinel or query it.
func TestLoadClusterMetricsRangeForDashboard_TargetsGivenContextAtUnionSentinel(t *testing.T) {
	m := basePush80Model()
	m.scheduler = scheduler.New(0)
	m.unionMode = true
	m.nav.Context = UnionContextSentinel
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	cmd := m.loadClusterMetricsRangeForDashboard("member-b")
	require.NotNil(t, cmd)
	msg, ok := execScheduled(t, m, cmd).(clusterMetricsRangeMsg)
	require.True(t, ok, "must return a clusterMetricsRangeMsg rather than bail on the sentinel")
	assert.Equal(t, "member-b", msg.context)
}
