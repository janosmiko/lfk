package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

func inlineTestWidths() dashboardWidths {
	return dashboardWidths{bar: 40, node: 20, sep: 40, label: 8, content: 80}
}

func inlineTestSegments() []struct {
	count int
	style lipgloss.Style
} {
	return []struct {
		count int
		style lipgloss.Style
	}{
		{60, ui.StatusRunning},
		{20, ui.StatusOther},
	}
}

func inlineTestBar(width int) metricBar {
	return stackedMetricBar(inlineTestSegments(), 100, width).
		withLabels([]string{"60 Running", "20 Done"}, "20 Free")
}

const inlineTestLegend = "60 Running · 20 Done · 20 Free"

func TestDashboardMetricLines_LabelsStartAtTheirSegments(t *testing.T) {
	lines := dashboardMetricLines("Pods", inlineTestBar(40), inlineTestLegend, inlineTestWidths())

	require.Len(t, lines, 1)
	plain := stripANSI(lines[0])
	// 40 cells: Running 24, Done 8, empty tail 8.
	assert.Contains(t, plain, "["+"60 Running"+strings.Repeat(" ", 14)+"20 Done "+"20 Free "+"]")
	assert.NotContains(t, plain, "·")
	assert.Equal(t, 2+8+2+1+40+1, lipgloss.Width(plain))
}

func TestDashboardMetricLines_LabelOverflowingItsSegmentFallsBack(t *testing.T) {
	// 20 cells: Running 12, Done 4 (too narrow for "20 Done"), tail 4.
	lines := dashboardMetricLines("Pods", inlineTestBar(20), inlineTestLegend, inlineTestWidths())

	require.Len(t, lines, 2)
	assert.Contains(t, stripANSI(lines[0]), "[████████████████░░░░]")
	assert.Contains(t, stripANSI(lines[1]), inlineTestLegend)
}

func TestDashboardMetricLines_TailLabelNeedsItsOwnRoom(t *testing.T) {
	bar := stackedMetricBar(inlineTestSegments(), 100, 40).
		withLabels([]string{"60 Running", "20 Done"}, "20 Unallocated headroom")
	lines := dashboardMetricLines("Pods", bar, "legend", inlineTestWidths())

	require.Len(t, lines, 2)
	assert.Contains(t, stripANSI(lines[1]), "legend")
}

func TestDashboardMetricLines_ZeroCountSegmentHasNoLabel(t *testing.T) {
	segs := []struct {
		count int
		style lipgloss.Style
	}{{10, ui.StatusRunning}, {0, ui.StatusFailed}}
	bar := stackedMetricBar(segs, 10, 20).withLabels([]string{"10 Ready", ""}, "")
	lines := dashboardMetricLines("Nodes", bar, "10 Ready", inlineTestWidths())

	require.Len(t, lines, 1)
	assert.Contains(t, stripANSI(lines[0]), "[10 Ready"+strings.Repeat(" ", 12)+"]")
}

func TestDashboardMetricLines_WideLegendFallsBackToTwoLines(t *testing.T) {
	lines := dashboardMetricLines("Pods", inlineTestBar(10), inlineTestLegend, inlineTestWidths())

	require.Len(t, lines, 2)
	assert.Contains(t, stripANSI(lines[1]), inlineTestLegend)
}

func TestDashboardMetricLines_NoColorFallsBackToTwoLines(t *testing.T) {
	ui.SetNoColor(true)
	t.Cleanup(func() { ui.SetNoColor(false) })

	lines := dashboardMetricLines("Pods", inlineTestBar(40), inlineTestLegend, inlineTestWidths())

	require.Len(t, lines, 2)
	assert.Contains(t, stripANSI(lines[0]), "█")
	assert.Contains(t, stripANSI(lines[1]), inlineTestLegend)
}

func TestDashboardMetricLines_PercentBarKeepsSuffix(t *testing.T) {
	bar := percentMetricBar(50, 100, 40)
	lines := dashboardMetricLines("CPU", bar, "2 / 4 cores", inlineTestWidths())

	require.Len(t, lines, 1)
	plain := stripANSI(lines[0])
	assert.Contains(t, plain, "[2 / 4 cores")
	assert.True(t, strings.HasSuffix(plain, "]  50%"), plain)
}

func TestDashboardMetricLines_ZeroTotalBar(t *testing.T) {
	lines := dashboardMetricLines("Jobs", stackedMetricBar(nil, 0, 40), "0", inlineTestWidths())

	require.Len(t, lines, 1)
	assert.Contains(t, stripANSI(lines[0]), "[0")
}

func TestDashboardPinnedRowLines_MultiDimOneLineRows(t *testing.T) {
	r := pinnedSummaryResult{
		key:         "applications",
		displayName: "Applications",
		summary: ui.ListSummary{Bars: []ui.SummaryBar{
			{Label: "Health", Total: 2, Buckets: []ui.SummaryBucket{{Value: "Healthy", Count: 2}}},
			{Label: "Sync", Total: 2, Buckets: []ui.SummaryBucket{{Value: "Synced", Count: 2}}},
		}},
	}
	var rows []string
	require.NotPanics(t, func() { rows = dashboardPinnedRowLines(r, inlineTestWidths()) })

	require.Len(t, rows, 3)
	assert.Contains(t, stripANSI(rows[0]), "Applications")
	assert.Contains(t, stripANSI(rows[1]), "├ Health")
	assert.Contains(t, stripANSI(rows[1]), "[2 Healthy")
	assert.Contains(t, stripANSI(rows[2]), "[2 Synced")
	assert.Contains(t, stripANSI(rows[2]), "└ Sync")
}

func TestDashboardPinnedRowLines_MultiDimTwoLineFallbackKeepsContinuation(t *testing.T) {
	w := inlineTestWidths()
	w.bar = 6
	r := pinnedSummaryResult{
		key:         "applications",
		displayName: "Applications",
		summary: ui.ListSummary{Bars: []ui.SummaryBar{
			{Label: "Health", Total: 2, Buckets: []ui.SummaryBucket{{Value: "Healthy", Count: 2}}},
			{Label: "Sync", Total: 2, Buckets: []ui.SummaryBucket{{Value: "Synced", Count: 2}}},
		}},
	}
	rows := dashboardPinnedRowLines(r, w)

	require.Len(t, rows, 5)
	assert.True(t, strings.HasPrefix(stripANSI(rows[2]), "  │"), stripANSI(rows[2]))
	assert.Contains(t, stripANSI(rows[2]), "2 Healthy")
}

func TestDashboardHeaderSection_PodsLabelsAnchoredWithTail(t *testing.T) {
	w := inlineTestWidths()
	w.bar = 60
	w.content = 120
	data := dashboardData{
		pods:        podStats{total: 180, running: 100, succeeded: 80},
		podCapacity: 400,
	}
	lines := dashboardHeaderSection(nil, data, w)

	var row string
	for _, l := range lines {
		if strings.Contains(stripANSI(l), "Pods") {
			row = stripANSI(l)
		}
	}
	// 60 cells: Running 15, Succeeded 12, empty tail 33.
	assert.Contains(t, row, "[100 Running    80 Succeeded220 Unallocated"+strings.Repeat(" ", 18)+"]")
}

func TestDashboardHeaderSection_PodsSliverFallsBackToTwoLines(t *testing.T) {
	w := inlineTestWidths()
	w.bar = 60
	w.content = 120
	data := dashboardData{
		pods:        podStats{total: 182, running: 100, pending: 2, succeeded: 80},
		podCapacity: 400,
	}
	lines := dashboardHeaderSection(nil, data, w)

	joined := stripANSI(strings.Join(lines, "\n"))
	assert.Contains(t, joined, "100 Running · 2 Pending · 80 Succeeded · 218 Unallocated")
}

func TestDashboardHeaderSection_NodesLabelsAnchored(t *testing.T) {
	w := inlineTestWidths()
	w.content = 120
	lines := dashboardHeaderSection(nil, dashboardData{nodeCount: 4, readyNodes: 3}, w)

	var row string
	for _, l := range lines {
		if strings.Contains(stripANSI(l), "Nodes") {
			row = stripANSI(l)
		}
	}
	// 40 cells: Ready 30, NotReady 10.
	assert.Contains(t, row, "[3 Ready"+strings.Repeat(" ", 23)+"1 NotReady]")
}
