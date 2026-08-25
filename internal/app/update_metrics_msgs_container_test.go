package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateContainerMetricsEnrichedWithData verifies a container row is
// enriched from the per-container usage map, keyed by container name.
func TestUpdateContainerMetricsEnrichedWithData(t *testing.T) {
	m := basePush80Model()
	m.middleItems = []model.Item{
		{Name: "app", Kind: "Container"},
		{Name: "sidecar", Kind: "Container"},
	}

	metrics := map[string]k8s.ContainerUsage{
		"app": {CPUMilli: 250, MemBytes: 128 * 1024 * 1024},
	}
	result, cmd := m.Update(containerMetricsEnrichedMsg{metrics: metrics, gen: m.requestGen})
	mdl := result.(Model)
	assert.Nil(t, cmd)

	require.Equal(t, "250m", getColumnValue(mdl.middleItems[0], "CPU"))
	require.Equal(t, "128Mi", getColumnValue(mdl.middleItems[0], "MEM"))
}

// A map miss that fell through to FormatCPU/FormatMemory would render a
// fabricated "0m"/"0B" instead of "n/a" - the exact-string equality below
// catches that, where a loop-with-if would pass vacuously.
func TestUpdateContainerMetricsEnrichedMissingIsExactlyNA(t *testing.T) {
	m := basePush80Model()
	m.middleItems = []model.Item{
		{Name: "app", Kind: "Container"},
	}
	metrics := map[string]k8s.ContainerUsage{} // no entry for "app"

	result, _ := m.Update(containerMetricsEnrichedMsg{metrics: metrics, gen: m.requestGen})
	mdl := result.(Model)

	require.Equal(t, "n/a", getColumnValue(mdl.middleItems[0], "CPU"))
	require.Equal(t, "n/a", getColumnValue(mdl.middleItems[0], "MEM"))
}

// TestUpdateContainerMetricsEnrichedStaleGen verifies a reply for a
// superseded generation is dropped without touching the middle items.
func TestUpdateContainerMetricsEnrichedStaleGen(t *testing.T) {
	m := basePush80Model()
	m.requestGen = 10
	m.middleItems = []model.Item{{Name: "app", Kind: "Container"}}

	result, cmd := m.Update(containerMetricsEnrichedMsg{
		metrics: map[string]k8s.ContainerUsage{"app": {CPUMilli: 999, MemBytes: 999}},
		gen:     5,
	})
	mdl := result.(Model)
	assert.Nil(t, cmd)
	assert.Empty(t, mdl.middleItems[0].Columns, "stale reply must not write any column")
}

// TestUpdateContainerMetricsEnriched_SparkModeAddsSparkline verifies the
// sparkline branch keyed by container name, mirroring
// TestUpdateNodeMetricsEnriched_SparkModeKeysByNodeName.
func TestUpdateContainerMetricsEnriched_SparkModeAddsSparkline(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}
	ui.ConfigSparklineWidth = 5
	t.Cleanup(func() { ui.ConfigSparklineWidth = ui.DefaultSparklineWidth })
	m.metricsSeries = metricsSeriesCache{
		cpu: map[string]k8s.MetricSeries{"app": {Points: []float64{1, 2, 3, 4, 5}}},
		mem: map[string]k8s.MetricSeries{"app": {Points: []float64{1, 2, 3, 4, 5}}},
	}
	m.middleItems = []model.Item{{Name: "app", Kind: "Container"}}

	got := m.updateContainerMetricsEnriched(containerMetricsEnrichedMsg{
		metrics: map[string]k8s.ContainerUsage{"app": {CPUMilli: 240, MemBytes: 1024}},
		gen:     m.requestGen,
	})

	cpu := getColumnValue(got.middleItems[0], "CPU")
	assert.NotEqual(t, "n/a", cpu)
	assert.Contains(t, cpu, "▁", "the sparkline glyphs must replace the trend arrow, not stack with it")
}

// Series seeded under the missing row's own key: see
// TestUpdatePodMetricsEnriched_SparkModeLeavesMissingRowsAsNA.
func TestUpdateContainerMetricsEnriched_SparkModeLeavesMissingRowsAsNA(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}
	m.metricsSeries = metricsSeriesCache{
		cpu: map[string]k8s.MetricSeries{"no-metrics": {Points: []float64{1, 9}}},
		mem: map[string]k8s.MetricSeries{"no-metrics": {Points: []float64{3, 7}}},
	}
	m.middleItems = []model.Item{{Name: "no-metrics", Kind: "Container"}}

	got := m.updateContainerMetricsEnriched(containerMetricsEnrichedMsg{gen: m.requestGen})

	assert.Equal(t, "n/a", getColumnValue(got.middleItems[0], "CPU"),
		"CPU must stay exactly n/a in sparkline mode, with no glyph prefix")
	assert.Equal(t, "n/a", getColumnValue(got.middleItems[0], "MEM"),
		"MEM must stay exactly n/a in sparkline mode, with no glyph prefix")
}
