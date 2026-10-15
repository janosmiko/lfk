package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func testDashboardWidths() dashboardWidths {
	// label is the production cap (14) so label-text assertions below aren't
	// truncated - dashboardMetricLines truncates any label wider than w.label.
	return dashboardWidths{bar: 20, node: 10, sep: 40, label: 14, content: 90}
}

func TestDashboardPinnedRows_Empty(t *testing.T) {
	lines := dashboardPinnedRows(nil, dashboardData{}, testDashboardWidths())
	assert.Empty(t, lines, "no pinned summaries -> no rows")
}

func TestDashboardPinnedRows_RendersInPinOrderNoHeader(t *testing.T) {
	apps := ui.BuildListSummary("Application", []model.Item{
		{Kind: "Application", Columns: []model.KeyValue{{Key: "Health", Value: "Degraded"}, {Key: "Sync Status", Value: "OutOfSync"}}},
	})
	jobs := ui.BuildListSummary("Job", []model.Item{{Kind: "Job", Status: "Complete"}})
	data := dashboardData{pinnedSummaries: []pinnedSummaryResult{
		{index: 1, key: "argoproj.io/applications", displayName: "Applications", summary: apps},
		{index: 0, key: "batch/jobs", displayName: "Jobs", summary: jobs},
	}}

	out := ansi.Strip(strings.Join(dashboardPinnedRows(nil, data, testDashboardWidths()), "\n"))
	assert.NotContains(t, out, "PINNED SUMMARIES", "no separate section header")
	assert.Less(t, strings.Index(out, "Jobs"), strings.Index(out, "Applications"), "index order wins over arrival order")
	assert.Contains(t, out, "1 Degraded")
}

func TestDashboardPinnedRows_MultiDimensionGroupsUnderKindHeader(t *testing.T) {
	apps := ui.BuildListSummary("Application", []model.Item{
		{Kind: "Application", Columns: []model.KeyValue{{Key: "Health", Value: "Degraded"}, {Key: "Sync Status", Value: "OutOfSync"}}},
	})
	data := dashboardData{pinnedSummaries: []pinnedSummaryResult{
		{index: 0, key: "argoproj.io/applications", displayName: "Applications", summary: apps},
	}}
	lines := dashboardPinnedRows(nil, data, testDashboardWidths())
	for i, l := range lines {
		lines[i] = strings.TrimRight(ansi.Strip(l), " ")
	}
	// A bare "Sync" row under "Applications" read as an unrelated metric, so
	// every dimension is a labeled tree branch under a kind header line.
	assert.Len(t, lines, 5)
	assert.Equal(t, "  Applications", lines[0])
	assert.True(t, strings.HasPrefix(lines[1], "  ├ Health"), lines[1])
	assert.True(t, strings.HasPrefix(lines[2], "  │"), "branch continues past the legend line: %q", lines[2])
	assert.Contains(t, lines[2], "1 Degraded")
	assert.True(t, strings.HasPrefix(lines[3], "  └ Sync"), lines[3])
	assert.Contains(t, lines[4], "1 OutOfSync")
	assert.NotContains(t, lines[4], "│")
}

func TestDashboardPinnedRows_SingleDimensionKeepsKindLabel(t *testing.T) {
	jobs := ui.BuildListSummary("Job", []model.Item{{Kind: "Job", Status: "Complete"}})
	data := dashboardData{pinnedSummaries: []pinnedSummaryResult{
		{index: 0, key: "batch/jobs", displayName: "Jobs", summary: jobs},
	}}
	lines := dashboardPinnedRows(nil, data, testDashboardWidths())
	assert.Len(t, lines, 2)
	assert.True(t, strings.HasPrefix(ansi.Strip(lines[0]), "  Jobs "), "bar shares the kind label's line")
}

func TestMaxPinnedLabelWidth_MeasuresTreeLabels(t *testing.T) {
	vols := ui.SummaryBar{Label: "Robustness", Total: 1, Buckets: []ui.SummaryBucket{{Value: "healthy", Count: 1}}}
	state := ui.SummaryBar{Label: "State", Total: 1, Buckets: []ui.SummaryBucket{{Value: "attached", Count: 1}}}
	data := dashboardData{pinnedSummaries: []pinnedSummaryResult{
		{displayName: "Volumes", summary: ui.ListSummary{Total: 1, Bars: []ui.SummaryBar{vols, state}}},
	}}
	// The kind name sits on its own header line, so only "├ Robustness" counts.
	assert.Equal(t, 12, maxPinnedLabelWidth(data))
}

func TestDashboardPinnedRows_ZeroTotalStillRenders(t *testing.T) {
	jobs := ui.BuildListSummary("Job", nil)
	data := dashboardData{pinnedSummaries: []pinnedSummaryResult{
		{index: 0, key: "batch/jobs", displayName: "Jobs", summary: jobs},
	}}
	out := ansi.Strip(strings.Join(dashboardPinnedRows(nil, data, testDashboardWidths()), "\n"))
	assert.Contains(t, out, "Jobs")
	assert.Contains(t, out, "0")
	assert.Contains(t, out, "░", "an empty bar still renders for a zero-total kind")
}

func TestDashboardPinnedRows_NotFoundAndError(t *testing.T) {
	data := dashboardData{pinnedSummaries: []pinnedSummaryResult{
		{index: 0, key: "chores.example.com/chores", displayName: "chores.example.com/chores", notFound: true},
		{index: 1, key: "batch/jobs", displayName: "Jobs", err: assert.AnError},
	}}
	out := ansi.Strip(strings.Join(dashboardPinnedRows(nil, data, testDashboardWidths()), "\n"))
	assert.Contains(t, out, "not installed")
	assert.Contains(t, out, "unavailable")
}

func TestDashboardPinnedRows_DoesNotMutateInput(t *testing.T) {
	data := dashboardData{pinnedSummaries: []pinnedSummaryResult{
		{index: 1, displayName: "B"}, {index: 0, displayName: "A"},
	}}
	_ = dashboardPinnedRows(nil, data, testDashboardWidths())
	assert.Equal(t, 1, data.pinnedSummaries[0].index, "render must sort a copy, not the shared slice")
}
