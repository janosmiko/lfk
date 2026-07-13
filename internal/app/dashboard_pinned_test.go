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
	return dashboardWidths{bar: 20, node: 10, sep: 40, label: 6, content: 76}
}

func TestDashboardPinnedSection_Empty(t *testing.T) {
	lines := dashboardPinnedSection(nil, dashboardData{}, testDashboardWidths())
	assert.Empty(t, lines, "no pinned summaries -> no section")
}

func TestDashboardPinnedSection_RendersInPinOrder(t *testing.T) {
	apps := ui.BuildListSummary("Application", []model.Item{
		{Kind: "Application", Columns: []model.KeyValue{{Key: "Health", Value: "Degraded"}, {Key: "Sync Status", Value: "OutOfSync"}}},
	})
	jobs := ui.BuildListSummary("Job", []model.Item{{Kind: "Job", Status: "Complete"}})
	data := dashboardData{pinnedSummaries: []pinnedSummaryResult{
		{index: 1, key: "argoproj.io/applications", displayName: "Applications", summary: apps},
		{index: 0, key: "batch/jobs", displayName: "Jobs", summary: jobs},
	}}

	out := ansi.Strip(strings.Join(dashboardPinnedSection(nil, data, testDashboardWidths()), "\n"))
	assert.Contains(t, out, "PINNED SUMMARIES")
	assert.Less(t, strings.Index(out, "Jobs"), strings.Index(out, "Applications"), "index order wins over arrival order")
	assert.Contains(t, out, "1 Degraded")
}

func TestDashboardPinnedSection_NotFoundAndError(t *testing.T) {
	data := dashboardData{pinnedSummaries: []pinnedSummaryResult{
		{index: 0, key: "chores.example.com/chores", displayName: "chores.example.com/chores", notFound: true},
		{index: 1, key: "batch/jobs", displayName: "Jobs", err: assert.AnError},
	}}
	out := ansi.Strip(strings.Join(dashboardPinnedSection(nil, data, testDashboardWidths()), "\n"))
	assert.Contains(t, out, "not installed")
	assert.Contains(t, out, "unavailable")
}

func TestDashboardPinnedSection_DoesNotMutateInput(t *testing.T) {
	data := dashboardData{pinnedSummaries: []pinnedSummaryResult{
		{index: 1, displayName: "B"}, {index: 0, displayName: "A"},
	}}
	_ = dashboardPinnedSection(nil, data, testDashboardWidths())
	assert.Equal(t, 1, data.pinnedSummaries[0].index, "render must sort a copy, not the shared slice")
}
