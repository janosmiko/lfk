package app

import (
	"math"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	m = m.updatePodMetricsRange(podMetricsRangeMsg{gen: m.requestGen, err: assert.AnError})

	assert.Equal(t, ui.MetricsDisplayNumeric, m.metricsSpark.Mode,
		"a failed range query must return the columns to numeric")
	assert.NotEmpty(t, m.statusMessage, "the user must be told once why the mode reverted")
	assert.Nil(t, m.metricsSeries.cpu)
}

// An empty result is the "Prometheus is there but has no data for these pods"
// case. It reverts too, because a sparkline mode with no series draws nothing.
func TestUpdatePodMetricsRange_EmptyResultFallsBack(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m = m.updatePodMetricsRange(podMetricsRangeMsg{gen: m.requestGen})

	assert.Equal(t, ui.MetricsDisplayNumeric, m.metricsSpark.Mode)
}

func TestUpdatePodMetricsRange_StoresSeriesAndKeepsMode(t *testing.T) {
	m := basePush80Model()
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	m = m.updatePodMetricsRange(podMetricsRangeMsg{
		gen: m.requestGen,
		cpu: map[string]k8s.MetricSeries{"default/api-1": {Points: []float64{1, 2}}},
		mem: map[string]k8s.MetricSeries{"default/api-1": {Points: []float64{3, 4}}},
	})

	assert.Equal(t, ui.MetricsDisplaySpark, m.metricsSpark.Mode)
	assert.Equal(t, []float64{1, 2}, m.metricsSeries.cpu["default/api-1"].Points)
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

	m = m.updatePodMetricsRange(podMetricsRangeMsg{gen: m.requestGen + 1, err: assert.AnError})

	assert.Equal(t, ui.MetricsDisplaySpark, m.metricsSpark.Mode,
		"a stale response must not revert the mode the user just chose")
}
