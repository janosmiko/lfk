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
		{260, ui.StatusRunning},
		{67, ui.StatusOther},
	}
}

func TestDashboardMetricLines_LegendFitsInsideBar(t *testing.T) {
	bar := stackedMetricBar(inlineTestSegments(), 860, 40)
	lines := dashboardMetricLines("Pods", bar, ui.StatusRunning.Render("260 Running")+" 67 Succeeded", inlineTestWidths())

	require.Len(t, lines, 1)
	plain := stripANSI(lines[0])
	assert.Contains(t, plain, "Pods")
	assert.Contains(t, plain, "[260 Running 67 Succeeded")
	assert.True(t, strings.HasSuffix(plain, "]"), plain)
	assert.Equal(t, 2+8+2+1+40+1, lipgloss.Width(plain))
}

func TestDashboardMetricLines_WideLegendFallsBackToTwoLines(t *testing.T) {
	bar := stackedMetricBar(inlineTestSegments(), 860, 10)
	legend := "260 Running 67 Succeeded 533 Unallocated"
	lines := dashboardMetricLines("Pods", bar, legend, inlineTestWidths())

	require.Len(t, lines, 2)
	assert.Equal(t, "["+strings.Repeat("█", 3)+strings.Repeat("█", 1)+strings.Repeat("░", 6)+"]",
		strings.TrimSpace(strings.TrimPrefix(stripANSI(lines[0]), "  Pods")))
	assert.Contains(t, stripANSI(lines[1]), legend)
}

func TestDashboardMetricLines_NoColorFallsBackToTwoLines(t *testing.T) {
	ui.SetNoColor(true)
	t.Cleanup(func() { ui.SetNoColor(false) })

	bar := stackedMetricBar(inlineTestSegments(), 860, 40)
	lines := dashboardMetricLines("Pods", bar, "260 Running", inlineTestWidths())

	require.Len(t, lines, 2)
	assert.Contains(t, stripANSI(lines[0]), "█")
	assert.Contains(t, stripANSI(lines[1]), "260 Running")
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
