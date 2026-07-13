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

func TestDashboardPinnedRows_MultiDimensionSplitsIntoTwoRows(t *testing.T) {
	apps := ui.BuildListSummary("Application", []model.Item{
		{Kind: "Application", Columns: []model.KeyValue{{Key: "Health", Value: "Degraded"}, {Key: "Sync Status", Value: "OutOfSync"}}},
	})
	data := dashboardData{pinnedSummaries: []pinnedSummaryResult{
		{index: 0, key: "argoproj.io/applications", displayName: "Applications", summary: apps},
	}}
	out := ansi.Strip(strings.Join(dashboardPinnedRows(nil, data, testDashboardWidths()), "\n"))
	// The first row carries the kind label (Health dimension), the second the
	// "Sync" dimension label - two rows, not a stacked block under one label.
	assert.Contains(t, out, "Applications")
	assert.Contains(t, out, "Sync")
	assert.Less(t, strings.Index(out, "Applications"), strings.Index(out, "Sync"))
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
