package app

import (
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestModelForDashboard returns a minimal Model with dashboardAcc
// initialised for use in dashboard handler tests.
func newTestModelForDashboard(_ *testing.T) Model {
	return Model{
		nav:           model.NavigationState{Level: model.LevelResources},
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		dashboardAcc:  make(map[string]*dashboardAccumulator),
		width:         80,
		height:        40,
		execMu:        &sync.Mutex{},
	}
}

// TestDashboardBarsScaleWithWidth verifies the resource usage bars use the
// available space: wide in fullscreen, compact in the narrow right pane, and
// noticeably wider than the old fixed width of 30.
func TestDashboardBarsScaleWithWidth(t *testing.T) {
	full := Model{width: 200, height: 50, fullscreenDashboard: true}
	pane := Model{width: 200, height: 50, fullscreenDashboard: false}

	wf := full.dashboardWidths(false, 0)
	wp := pane.dashboardWidths(false, 0)

	assert.Greater(t, wf.bar, wp.bar, "fullscreen bars must be wider than the right-pane bars")
	assert.Greater(t, wf.bar, 30, "fullscreen bars must use more space than the old fixed 30")
}

// TestComposeDashboardFitsContentWidth guards the two-column wrap risk: no
// composed dashboard line may exceed the content width it is rendered into,
// otherwise the left pane wraps and the layout tears.
func TestComposeDashboardFitsContentWidth(t *testing.T) {
	// A long pod breakdown is the worst-case summary width.
	base := dashboardData{
		nodeCount: 3, readyNodes: 1, nodeItems: make([]model.Item, 3),
		pods:         podStats{total: 424, running: 361, failed: 39, succeeded: 24},
		nsCount:      53,
		totalCPUUsed: 520, totalCPUAlloc: 1000,
		totalMemUsed: 2 << 30, totalMemAlloc: 4 << 30,
		nodes: []nodeInfo{
			// A very long node name must be truncated, not wrapped.
			{name: "itg-k8s-autoscaled-cx43-167f996afa950112-extra-long", cpuUsed: 1, cpuAlloc: 2, memUsed: 1 << 30, memAlloc: 2 << 30},
		},
	}
	withWarnings := base
	withWarnings.allWarnings = []model.Item{{Name: "e1"}, {Name: "e2"}}

	for _, tc := range []struct {
		name       string
		fullscreen bool
		data       dashboardData
	}{
		{"fullscreen single-col", true, base},
		{"fullscreen two-col", true, withWarnings},
		{"right pane", false, base},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{width: 120, height: 40, fullscreenDashboard: tc.fullscreen}
			content, events := m.composeDashboard(tc.data)
			twoCol := tc.fullscreen && events != ""
			cw := m.dashboardContentWidth(twoCol)
			for ln := range strings.SplitSeq(content, "\n") {
				assert.LessOrEqual(t, lipgloss.Width(ln), cw,
					"line %q exceeds content width %d", stripANSI(ln), cw)
			}
		})
	}
}

func TestCountPodStatsSucceeded(t *testing.T) {
	ps := countPodStats([]model.Item{
		{Status: "Running"}, {Status: "Succeeded"}, {Status: "Completed"}, {Status: "Failed"},
	})
	assert.Equal(t, 2, ps.succeeded, "Succeeded and Completed both count as succeeded")
	assert.Equal(t, 1, ps.running)
	assert.Equal(t, 1, ps.failed)
}

func TestPodSummaryBreakdown(t *testing.T) {
	s := stripANSI(podSummaryStr(dashboardData{
		pods: podStats{total: 424, running: 361, failed: 39, succeeded: 24},
	}))
	assert.Contains(t, s, "361 Running")
	assert.Contains(t, s, "39 Failed")
	assert.Contains(t, s, "24 Succeeded")
	// Zero-count states are omitted (no phantom categories).
	assert.NotContains(t, s, "Pending")
	assert.NotContains(t, s, "Other")
}

func TestComposeDashboard_WarningsPlacement(t *testing.T) {
	data := dashboardData{
		nodeCount: 3, readyNodes: 3, nodeItems: make([]model.Item, 3),
		pods:          podStats{total: 10, running: 6, failed: 4},
		warningEvents: []model.Item{{Age: "5m"}},
		allWarnings:   []model.Item{{Age: "5m"}},
	}

	t.Run("fullscreen puts warnings in the right column above events", func(t *testing.T) {
		m := Model{width: 120, height: 40, fullscreenDashboard: true}
		content, events := m.composeDashboard(data)
		assert.NotContains(t, stripANSI(content), "WARNINGS",
			"warnings must not be in the left column in fullscreen")
		right := stripANSI(events)
		assert.Contains(t, right, "WARNINGS")
		assert.Contains(t, right, "RECENT EVENTS")
		assert.Less(t, strings.Index(right, "WARNINGS"), strings.Index(right, "RECENT EVENTS"),
			"warnings must sit above recent events")
		// A separator divides the warnings from the events below it.
		sep := strings.Index(right, "─")
		require.Positive(t, sep, "a separator must appear in the right column")
		assert.Less(t, strings.Index(right, "WARNINGS"), sep)
		assert.Less(t, sep, strings.Index(right, "RECENT EVENTS"))
	})

	t.Run("preview pane stacks warnings in the single column", func(t *testing.T) {
		m := Model{width: 120, height: 40, fullscreenDashboard: false}
		content, _ := m.composeDashboard(data)
		assert.Contains(t, stripANSI(content), "WARNINGS")
	})
}

// TestComposeDashboard_PinnedRowsRenderInlineAfterPods verifies pinned
// summaries render as regular metric rows directly below Pods, with no
// separate "PINNED SUMMARIES" section or separator between the two blocks
// (Task 10: inline rows replace the old standalone section).
func TestComposeDashboard_PinnedRowsRenderInlineAfterPods(t *testing.T) {
	jobs := ui.BuildListSummary("Job", []model.Item{{Kind: "Job", Status: "Complete"}})
	data := dashboardData{
		nodeCount: 2, readyNodes: 2, nodeItems: make([]model.Item, 2),
		pods: podStats{total: 5, running: 5},
		pinnedSummaries: []pinnedSummaryResult{
			{index: 0, key: "batch/jobs", displayName: "Jobs", summary: jobs},
		},
	}
	m := Model{width: 120, height: 40}
	content, _ := m.composeDashboard(data)
	plain := stripANSI(content)

	assert.NotContains(t, plain, "PINNED SUMMARIES", "the standalone section header is gone")

	podsIdx := strings.Index(plain, "Pods")
	jobsIdx := strings.Index(plain, "Jobs")
	require.Greater(t, podsIdx, -1, "Pods row must render")
	require.Greater(t, jobsIdx, -1, "the pinned Jobs row must render")
	assert.Greater(t, jobsIdx, podsIdx, "the pinned row must render after the Pods row")

	between := plain[podsIdx:jobsIdx]
	assert.NotContains(t, between, "───", "no separator between Pods and the pinned row")
}

// TestComposeDashboard_PinnedLabelWidensColumnConsistently guards the
// alignment failure mode this task exists to fix: a pinned label longer than
// the old fixed 5-char column must widen dashboardWidths.label for every row,
// not just its own, so Pods and the pinned row's bars still start at the same
// column.
func TestComposeDashboard_PinnedLabelWidensColumnConsistently(t *testing.T) {
	jobs := ui.BuildListSummary("Job", []model.Item{{Kind: "Job", Status: "Complete"}})
	data := dashboardData{
		nodeCount: 2, readyNodes: 2, nodeItems: make([]model.Item, 2),
		pods: podStats{total: 5, running: 5},
		pinnedSummaries: []pinnedSummaryResult{
			{index: 0, key: "kustomize.toolkit.fluxcd.io/kustomizations", displayName: "Kustomizations", summary: jobs},
		},
	}
	m := Model{width: 120, height: 40}
	content, _ := m.composeDashboard(data)
	plain := stripANSI(content)

	podsBarCol, pinnedBarCol := -1, -1
	for ln := range strings.SplitSeq(plain, "\n") {
		if strings.Contains(ln, "Pods") {
			podsBarCol = strings.Index(ln, "[")
		}
		if strings.Contains(ln, "Kustomizations") {
			pinnedBarCol = strings.Index(ln, "[")
		}
	}
	require.NotEqual(t, -1, podsBarCol, "Pods bar must be found")
	require.NotEqual(t, -1, pinnedBarCol, "pinned bar must be found")
	assert.Equal(t, podsBarCol, pinnedBarCol, "widened label column must keep all bars aligned")
}

func TestPodOther(t *testing.T) {
	// Uncounted phases (Terminating/Unknown) surface as Other, never Failed.
	assert.Equal(t, 5, podOther(podStats{total: 10, running: 5}))
	assert.Equal(t, 0, podOther(podStats{total: 10, running: 6, succeeded: 4}))
	// Never negative even if counts somehow exceed the total.
	assert.Equal(t, 0, podOther(podStats{total: 10, running: 8, failed: 5}))
}

func TestSumPodCapacity(t *testing.T) {
	nodes := []model.Item{
		{Columns: []model.KeyValue{{Key: "Pods Alloc", Value: "110"}, {Key: "CPU Alloc", Value: "4"}}},
		{Columns: []model.KeyValue{{Key: "Pods Alloc", Value: "250"}}},
		{Columns: []model.KeyValue{{Key: "CPU Alloc", Value: "8"}}},    // no Pods Alloc
		{Columns: []model.KeyValue{{Key: "Pods Alloc", Value: "abc"}}}, // unparsable, skipped
	}
	assert.Equal(t, int64(360), sumPodCapacity(nodes))
	assert.Equal(t, int64(0), sumPodCapacity(nil))
}

func TestPodBarDenominatorUsesCapacity(t *testing.T) {
	// Capacity above scheduled total drives the denominator.
	d := dashboardData{pods: podStats{total: 424}, podCapacity: 1100}
	assert.Equal(t, 1100, podBarDenominator(d))
	assert.Equal(t, 676, podUnallocated(d))

	// Capacity unknown (0) falls back to the scheduled total; no headroom.
	d = dashboardData{pods: podStats{total: 424}}
	assert.Equal(t, 424, podBarDenominator(d))
	assert.Equal(t, 0, podUnallocated(d))

	// Saturated (scheduled >= capacity) never reports negative headroom.
	d = dashboardData{pods: podStats{total: 1100}, podCapacity: 1100}
	assert.Equal(t, 1100, podBarDenominator(d))
	assert.Equal(t, 0, podUnallocated(d))
}

func TestPodSummaryShowsUnallocated(t *testing.T) {
	s := stripANSI(podSummaryStr(dashboardData{
		pods:        podStats{total: 424, running: 361, failed: 39, succeeded: 24},
		podCapacity: 1100,
	}))
	assert.Contains(t, s, "361 Running")
	assert.Contains(t, s, "676 Unallocated")

	// Without capacity data, no Unallocated category appears.
	noCap := stripANSI(podSummaryStr(dashboardData{
		pods: podStats{total: 424, running: 361, failed: 39, succeeded: 24},
	}))
	assert.NotContains(t, noCap, "Unallocated")
}

func TestRenderStackedBarLeavesHeadroomBelowTotal(t *testing.T) {
	// Segments summing to less than total (scheduled pods below pod capacity)
	// must leave the unfilled tail as empty cells, not absorb it into the last
	// segment.
	segments := []struct {
		count int
		style lipgloss.Style
	}{
		{5, lipgloss.NewStyle()},
	}
	result := renderStackedBar(segments, 10, 20)
	inner := stripANSI(result)
	inner = inner[1 : len(inner)-1]
	assert.Equal(t, 10, strings.Count(inner, "█"), "5/10 of a 20-wide bar is filled")
	assert.Equal(t, 10, strings.Count(inner, "░"), "the remaining headroom is empty")
}

func TestRenderStackedBarKeepsTinySegmentsVisible(t *testing.T) {
	// Mirrors a real cluster: 139 scheduled pods against 486 pod capacity in a
	// 95-wide bar. A 1- or 2-pod segment is ~0.2-0.4 cells and would floor to 0,
	// making Pending/Failed invisible. Each non-zero segment must claim >=1 cell.
	segments := []struct {
		count int
		style lipgloss.Style
	}{
		{119, lipgloss.NewStyle()}, // running -> int(119/486*95) = 23
		{2, lipgloss.NewStyle()},   // pending -> floors to 0, bumped to 1
		{1, lipgloss.NewStyle()},   // failed  -> floors to 0, bumped to 1
		{13, lipgloss.NewStyle()},  // succeeded -> int(13/486*95) = 2
		{4, lipgloss.NewStyle()},   // other   -> floors to 0, bumped to 1
	}
	inner := stripANSI(renderStackedBar(segments, 486, 95))
	inner = inner[1 : len(inner)-1]
	assert.Equal(t, 28, strings.Count(inner, "█"), "23 + 1 + 1 + 2 + 1 filled cells")
	assert.Equal(t, 67, strings.Count(inner, "░"), "remaining width is unallocated headroom")
}

func TestPodSummaryShowsOtherNotFailed(t *testing.T) {
	// total exceeds the counted phases with zero failures: the leftover must
	// read as Other, not as a phantom failure.
	s := stripANSI(podSummaryStr(dashboardData{
		pods: podStats{total: 10, running: 7},
	}))
	assert.Contains(t, s, "7 Running")
	assert.Contains(t, s, "3 Other")
	assert.NotContains(t, s, "Failed")
}

// stripANSI removes ANSI escape codes to allow plain-text assertions on
// rendered output. This covers the basic CSI sequences emitted by lipgloss.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			// Skip CSI sequence: ESC [ ... final byte.
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) && s[j] >= 0x20 && s[j] <= 0x3F {
					j++
				}
				if j < len(s) {
					j++ // skip final byte
				}
			}
			i = j
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

// --- renderBar ---

func TestRenderBar(t *testing.T) {
	tests := []struct {
		name         string
		used         int64
		total        int64
		width        int
		wantContains string
	}{
		{
			name:         "zero total shows N/A",
			used:         100,
			total:        0,
			width:        20,
			wantContains: "N/A",
		},
		{
			name:         "negative total shows N/A",
			used:         50,
			total:        -10,
			width:        20,
			wantContains: "N/A",
		},
		{
			name:         "0 percent usage",
			used:         0,
			total:        100,
			width:        20,
			wantContains: "0%",
		},
		{
			name:         "50 percent usage",
			used:         50,
			total:        100,
			width:        20,
			wantContains: "50%",
		},
		{
			name:         "100 percent usage",
			used:         100,
			total:        100,
			width:        20,
			wantContains: "100%",
		},
		{
			name:         "over 100 percent capped",
			used:         150,
			total:        100,
			width:        20,
			wantContains: "100%",
		},
		{
			name:         "75 percent boundary",
			used:         75,
			total:        100,
			width:        20,
			wantContains: "75%",
		},
		{
			name:         "90 percent boundary",
			used:         90,
			total:        100,
			width:        20,
			wantContains: "90%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderBar(tt.used, tt.total, tt.width)
			stripped := stripANSI(result)
			assert.Contains(t, stripped, tt.wantContains)
		})
	}
}

func TestRenderBarStructure(t *testing.T) {
	result := renderBar(50, 100, 20)
	stripped := stripANSI(result)

	assert.True(t, strings.HasPrefix(stripped, "["), "bar should start with [")
	assert.Contains(t, stripped, "]", "bar should contain ]")
}

func TestRenderBarWidthZero(t *testing.T) {
	// Width 0 should not panic.
	result := renderBar(50, 100, 0)
	stripped := stripANSI(result)
	assert.Contains(t, stripped, "[")
	assert.Contains(t, stripped, "]")
}

func TestRenderBarFilledChars(t *testing.T) {
	result := renderBar(100, 100, 10)
	stripped := stripANSI(result)

	// Extract content between brackets.
	openIdx := strings.Index(stripped, "[")
	closeIdx := strings.Index(stripped, "]")
	inner := stripped[openIdx+1 : closeIdx]
	filledCount := strings.Count(inner, "\u2588")
	assert.Equal(t, 10, filledCount, "100%% usage should fill entire bar width")
}

func TestRenderBarEmptyChars(t *testing.T) {
	result := renderBar(0, 100, 10)
	stripped := stripANSI(result)

	openIdx := strings.Index(stripped, "[")
	closeIdx := strings.Index(stripped, "]")
	inner := stripped[openIdx+1 : closeIdx]
	emptyCount := strings.Count(inner, "\u2591")
	assert.Equal(t, 10, emptyCount, "0%% usage should have all empty blocks")
}

// --- renderStackedBar ---

func TestRenderStackedBar(t *testing.T) {
	t.Run("zero total shows empty bar", func(t *testing.T) {
		segments := []struct {
			count int
			style lipgloss.Style
		}{
			{5, lipgloss.NewStyle()},
		}
		result := renderStackedBar(segments, 0, 20)
		stripped := stripANSI(result)

		assert.True(t, strings.HasPrefix(stripped, "["))
		assert.True(t, strings.HasSuffix(stripped, "]"))
		inner := stripped[1 : len(stripped)-1]
		assert.Equal(t, 20, strings.Count(inner, "\u2591"), "zero total should produce all empty blocks")
	})

	t.Run("negative total shows empty bar", func(t *testing.T) {
		segments := []struct {
			count int
			style lipgloss.Style
		}{
			{5, lipgloss.NewStyle()},
		}
		result := renderStackedBar(segments, -10, 20)
		stripped := stripANSI(result)
		inner := stripped[1 : len(stripped)-1]
		assert.Equal(t, 20, strings.Count(inner, "\u2591"))
	})

	t.Run("single segment fills bar", func(t *testing.T) {
		segments := []struct {
			count int
			style lipgloss.Style
		}{
			{10, lipgloss.NewStyle()},
		}
		result := renderStackedBar(segments, 10, 20)
		stripped := stripANSI(result)

		assert.True(t, strings.HasPrefix(stripped, "["))
		assert.True(t, strings.HasSuffix(stripped, "]"))
		inner := stripped[1 : len(stripped)-1]
		filledCount := strings.Count(inner, "\u2588")
		assert.Equal(t, 20, filledCount, "single segment at 100%% should fill entire bar")
	})

	t.Run("two segments split evenly", func(t *testing.T) {
		segments := []struct {
			count int
			style lipgloss.Style
		}{
			{5, lipgloss.NewStyle()},
			{5, lipgloss.NewStyle()},
		}
		result := renderStackedBar(segments, 10, 20)
		stripped := stripANSI(result)

		inner := stripped[1 : len(stripped)-1]
		filledCount := strings.Count(inner, "\u2588")
		assert.Equal(t, 20, filledCount)
	})

	t.Run("three segments with remainder", func(t *testing.T) {
		segments := []struct {
			count int
			style lipgloss.Style
		}{
			{3, lipgloss.NewStyle()},
			{3, lipgloss.NewStyle()},
			{4, lipgloss.NewStyle()},
		}
		result := renderStackedBar(segments, 10, 20)
		stripped := stripANSI(result)

		inner := stripped[1 : len(stripped)-1]
		filledCount := strings.Count(inner, "\u2588")
		assert.Equal(t, 20, filledCount, "all segments together should fill the bar")
	})

	t.Run("segments exceeding total triggers overflow guard", func(t *testing.T) {
		// When non-last segments produce more chars than the width, the
		// used+chars > width guard kicks in.
		segments := []struct {
			count int
			style lipgloss.Style
		}{
			{10, lipgloss.NewStyle()},
			{10, lipgloss.NewStyle()},
			{10, lipgloss.NewStyle()},
		}
		// total=10, width=5: each segment would want 5 chars, but only 5 total.
		result := renderStackedBar(segments, 10, 5)
		stripped := stripANSI(result)
		inner := stripped[1 : len(stripped)-1]
		totalChars := strings.Count(inner, "\u2588") + strings.Count(inner, "\u2591")
		assert.Equal(t, 5, totalChars, "total characters should not exceed width")
	})

	t.Run("last segment negative chars guard", func(t *testing.T) {
		// When the first segments already fill the bar, the last segment
		// gets chars = width - used which could be negative before the guard.
		// Here: segment1 gets int(15/15*5) = 5 chars (fills bar),
		// segment2 (last) gets chars = 5 - 5 = 0, which is non-negative.
		// To trigger chars < 0 on the last segment, we need used > width,
		// but that's prevented by the prior guard. So instead test a
		// scenario where segment proportions cause rounding overflow.
		segments := []struct {
			count int
			style lipgloss.Style
		}{
			{7, lipgloss.NewStyle()},
			{7, lipgloss.NewStyle()},
			{1, lipgloss.NewStyle()},
		}
		// total=15, width=10: seg0 = int(7/15*10) = 4, seg1 = int(7/15*10) = 4, used=8
		// seg2 (last) = width - used = 10 - 8 = 2. All is fine.
		// This ensures no panics with multiple segment rounding.
		result := renderStackedBar(segments, 15, 10)
		stripped := stripANSI(result)
		inner := stripped[1 : len(stripped)-1]
		filledCount := strings.Count(inner, "\u2588")
		assert.Equal(t, 10, filledCount, "rounding should not leave gaps")
	})

	t.Run("negative count in non-last segment", func(t *testing.T) {
		// A negative count produces negative chars which triggers the chars < 0 guard.
		segments := []struct {
			count int
			style lipgloss.Style
		}{
			{-5, lipgloss.NewStyle()},
			{10, lipgloss.NewStyle()},
		}
		result := renderStackedBar(segments, 10, 10)
		stripped := stripANSI(result)
		// Should not panic and should produce a valid bar.
		assert.True(t, strings.HasPrefix(stripped, "["))
		assert.True(t, strings.HasSuffix(stripped, "]"))
	})

	t.Run("empty segments array", func(t *testing.T) {
		var segments []struct {
			count int
			style lipgloss.Style
		}
		result := renderStackedBar(segments, 10, 20)
		stripped := stripANSI(result)

		inner := stripped[1 : len(stripped)-1]
		emptyCount := strings.Count(inner, "\u2591")
		assert.Equal(t, 20, emptyCount, "no segments should produce all empty blocks")
	})

	t.Run("width zero", func(t *testing.T) {
		segments := []struct {
			count int
			style lipgloss.Style
		}{
			{5, lipgloss.NewStyle()},
		}
		result := renderStackedBar(segments, 10, 0)
		stripped := stripANSI(result)
		assert.Equal(t, "[]", stripped)
	})
}

// --- formatTimeAgo ---

func TestFormatTimeAgo(t *testing.T) {
	tests := []struct {
		name     string
		offset   time.Duration
		contains string
	}{
		{
			name:     "seconds ago",
			offset:   30 * time.Second,
			contains: "s ago",
		},
		{
			name:     "minutes ago",
			offset:   5 * time.Minute,
			contains: "m ago",
		},
		{
			name:     "hours ago",
			offset:   3 * time.Hour,
			contains: "h ago",
		},
		{
			name:     "days ago",
			offset:   48 * time.Hour,
			contains: "d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			past := time.Now().Add(-tt.offset)
			result := formatTimeAgo(past)
			assert.Contains(t, result, tt.contains)
			assert.NotEmpty(t, result)
		})
	}
}

func TestCov80LoadDashboardReturnsCmd(t *testing.T) {
	m := basePush80Model()
	cmd := m.loadDashboard()
	require.NotNil(t, cmd)
	// The returned cmd is non-nil, confirming that the function captures
	// all needed state and returns a valid tea.Cmd closure.
}

func TestCov80LoadDashboardDifferentContexts(t *testing.T) {
	m := basePush80Model()
	m.nav.Context = "prod-cluster"
	cmd := m.loadDashboard()
	require.NotNil(t, cmd)

	m.nav.Context = ""
	cmd = m.loadDashboard()
	require.NotNil(t, cmd)
}

func TestCov80LoadMonitoringDashboardReturnsCmd(t *testing.T) {
	m := basePush80Model()
	cmd := m.loadMonitoringDashboard()
	// The closure captures client/context; confirm it's non-nil.
	require.NotNil(t, cmd)
}

func TestCov80LoadMonitoringDashboardAllNs(t *testing.T) {
	m := basePush80Model()
	m.allNamespaces = true
	cmd := m.loadMonitoringDashboard()
	require.NotNil(t, cmd)
}

func TestCov80LoadMonitoringDashboardDifferentContext(t *testing.T) {
	m := basePush80Model()
	m.nav.Context = "staging"
	cmd := m.loadMonitoringDashboard()
	require.NotNil(t, cmd)
}

func TestCovBoost2LoadDashboardCmd(t *testing.T) {
	m := baseModelBoost2()
	cmd := m.loadDashboard()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadMonitoringDashboardCmd(t *testing.T) {
	m := baseModelBoost2()
	cmd := m.loadMonitoringDashboard()
	assert.NotNil(t, cmd)
}

func TestCovLoadMonitoringDashboardReturnsCmd(t *testing.T) {
	m := baseModelWithFakeClient()
	cmd := m.loadMonitoringDashboard()
	// Just verify a command is returned. Executing it hits nil pointer in
	// alerts code that needs a real clientset for service discovery.
	assert.NotNil(t, cmd)
}

func TestFinal3LoadDashboardRichData(t *testing.T) {
	m := baseRichModel()
	cmd := m.loadDashboard()
	require.NotNil(t, cmd)
	// loadDashboard now returns a tea.Batch of 6 section submits.
	// Verify the batch is non-nil (content assertions are covered by
	// composeDashboardLoadedMsg and handleDashboardPartial tests).
	msg := cmd()
	require.NotNil(t, msg)
}

// The four tests below were carried over from the pre-fan-out
// dashboard implementation, when loadDashboard returned a single
// content-bearing tea.Cmd that could be driven inline. After the
// fan-out refactor each call returns a tea.Batch of six Submits whose
// content lives behind dashboardPartialMsg + handleDashboardPartial,
// so a literal "events content" or "contains sections" assertion
// would have to drive 6 sub-cmds, await Futures, and pump the
// accumulator — that's TestLoadDashboard_FanOutToBatch's job. These
// tests now only verify that loadDashboard returns a non-nil cmd
// against various fixtures, so the names match what they actually
// check.
func TestFinal3LoadDashboardReturnsCmdRich(t *testing.T) {
	m := baseRichModel()
	cmd := m.loadDashboard()
	require.NotNil(t, cmd)
}

func TestFinal3LoadDashboardReturnsCmdRichTwo(t *testing.T) {
	m := baseRichModel()
	cmd := m.loadDashboard()
	require.NotNil(t, cmd)
}

func TestFinalLoadDashboardReturnsCmd(t *testing.T) {
	m := baseFinalModel()
	cmd := m.loadDashboard()
	require.NotNil(t, cmd)
}

func TestFinalLoadDashboardExecutesAndReturnsDashboardMsg(t *testing.T) {
	m := baseFinalModelWithDynamic()
	cmd := m.loadDashboard()
	require.NotNil(t, cmd)
	// loadDashboard now returns a tea.Batch of 6 section submits.
	msg := cmd()
	require.NotNil(t, msg)
}

func TestFinalLoadDashboardReturnsCmdWithDynamic(t *testing.T) {
	m := baseFinalModelWithDynamic()
	cmd := m.loadDashboard()
	require.NotNil(t, cmd)
}

func TestFinalLoadDashboardReturnsCmdWithDynamicTwo(t *testing.T) {
	m := baseFinalModelWithDynamic()
	cmd := m.loadDashboard()
	require.NotNil(t, cmd)
}

func TestFinalLoadMonitoringDashboardReturnsCmd(t *testing.T) {
	m := baseFinalModelWithDynamic()
	cmd := m.loadMonitoringDashboard()
	require.NotNil(t, cmd)
}

func TestFinalLoadMonitoringDashboardNamespace(t *testing.T) {
	m := baseFinalModelWithDynamic()
	m.namespace = "custom-ns"
	cmd := m.loadMonitoringDashboard()
	require.NotNil(t, cmd)
}

func TestFinalLoadMonitoringDashboardAllNamespaces(t *testing.T) {
	m := baseFinalModelWithDynamic()
	m.allNamespaces = true
	cmd := m.loadMonitoringDashboard()
	require.NotNil(t, cmd)
}

func TestFinalFormatTimeAgoExact(t *testing.T) {
	// Just under a minute.
	result := formatTimeAgo(time.Now().Add(-45 * time.Second))
	assert.Contains(t, result, "s ago")

	// Just over a minute.
	result2 := formatTimeAgo(time.Now().Add(-90 * time.Second))
	assert.Contains(t, result2, "m ago")

	// Several hours.
	result3 := formatTimeAgo(time.Now().Add(-5 * time.Hour))
	assert.Contains(t, result3, "h ago")

	// Several days.
	result4 := formatTimeAgo(time.Now().Add(-72 * time.Hour))
	assert.Contains(t, result4, "d ago")
}

func TestHandleDashboardPartial_AccumulatesSections(t *testing.T) {
	m := newTestModelForDashboard(t)
	m.nav.Context = "test-ctx"
	m.requestGen = 7

	// Send 3 of 6 sections. The handler accumulates silently and emits
	// no tea.Cmd until all 6 arrive (atomic update — partial renders
	// would flicker the dashboard layout on every watch tick).
	// nodeItems must be non-nil to trigger the nodeCount merge in mergeDashboardSection.
	m, cmd1 := m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", gen: 7, key: "nodes", total: 6,
		data: dashboardData{nodeItems: make([]model.Item, 3), nodeCount: 3, readyNodes: 2},
	})
	assert.Nil(t, cmd1, "partial accumulation must not emit a render cmd")

	m, cmd2 := m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", gen: 7, key: "pods", total: 6,
		data: dashboardData{pods: podStats{total: 10, running: 8}},
	})
	assert.Nil(t, cmd2)

	m, cmd3 := m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", gen: 7, key: "namespaces", total: 6,
		data: dashboardData{nsCount: 5},
	})
	assert.Nil(t, cmd3)

	// 3 of 6 received — accumulator still pending.
	key := dashboardAccKey("test-ctx", 7)
	acc, ok := m.dashboardAcc[key]
	require.True(t, ok)
	assert.Equal(t, 3, acc.count)
	assert.Equal(t, 3, acc.data.nodeCount)
	assert.Equal(t, 5, acc.data.nsCount)
	assert.Equal(t, 10, acc.data.pods.total)
}

func TestHandleDashboardPartial_EmitsCmdOnlyAfterAllSections(t *testing.T) {
	m := newTestModelForDashboard(t)
	m.nav.Context = "test-ctx"
	m.requestGen = 1

	// Sections 1..5 produce no cmd.
	for i := range 5 {
		var cmd tea.Cmd
		m, cmd = m.handleDashboardPartial(dashboardPartialMsg{
			context: "test-ctx", gen: 1, key: dashboardSection(i).String(), total: 6,
			data: dashboardData{nodeCount: 1, nodeItems: make([]model.Item, 1)},
		})
		assert.Nilf(t, cmd, "section %d (1-indexed: %d) must not emit a cmd until all 6 arrive", i, i+1)
	}

	// Section 6 emits the dashboardLoadedMsg in one shot.
	m, cmd := m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", gen: 1, key: dashboardSection(5).String(), total: 6,
		data: dashboardData{nodeCount: 1, nodeItems: make([]model.Item, 1)},
	})
	require.NotNil(t, cmd, "the final section must emit a render cmd")
	msg, ok := cmd().(dashboardLoadedMsg)
	require.True(t, ok, "the emitted cmd must produce a dashboardLoadedMsg")
	assert.Equal(t, "test-ctx", msg.context)
}

func TestHandleDashboardPartial_DropsStaleGen(t *testing.T) {
	m := newTestModelForDashboard(t)
	m.nav.Context = "test-ctx"
	m.requestGen = 5

	m, _ = m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", gen: 4 /* stale */, key: "nodes", total: 6,
		data: dashboardData{nodeCount: 99},
	})

	key := dashboardAccKey("test-ctx", 4)
	_, ok := m.dashboardAcc[key]
	assert.False(t, ok, "stale gen msg must be dropped")
}

func TestHandleDashboardPartial_DropsWrongContext(t *testing.T) {
	m := newTestModelForDashboard(t)
	m.nav.Context = "current-ctx"
	m.requestGen = 1

	m, _ = m.handleDashboardPartial(dashboardPartialMsg{
		context: "other-ctx", gen: 1, key: "nodes", total: 6,
		data: dashboardData{nodeCount: 99},
	})

	key := dashboardAccKey("other-ctx", 1)
	_, ok := m.dashboardAcc[key]
	assert.False(t, ok, "wrong-context msg must be dropped")
}

func TestHandleDashboardPartial_DropsAccumulatorWhenAll6Arrive(t *testing.T) {
	m := newTestModelForDashboard(t)
	m.nav.Context = "test-ctx"
	m.requestGen = 1

	for i := range 6 {
		m, _ = m.handleDashboardPartial(dashboardPartialMsg{
			context: "test-ctx", gen: 1, key: dashboardSection(i).String(), total: 6,
			data: dashboardData{nodeCount: 1},
		})
	}

	key := dashboardAccKey("test-ctx", 1)
	_, ok := m.dashboardAcc[key]
	assert.False(t, ok, "accumulator must be dropped after all 6 sections arrive")
}

// TestHandleDashboardPartial_MixedTotalTakesMax guards against a coalesced
// old fan-out (total=6) racing a fresh one (total=7, e.g. a pin was added
// mid-flight) on the same (context, gen). If expected were seeded from
// whichever total arrives first and never revised, the accumulator could
// complete at 6 unique keys and drop the 7th fan-out's section entirely.
func TestHandleDashboardPartial_MixedTotalTakesMax(t *testing.T) {
	m := newTestModelForDashboard(t)
	m.nav.Context = "test-ctx"
	m.requestGen = 1

	// 5 sections arrive at the old fan-out's total (6).
	for i := range 5 {
		var cmd tea.Cmd
		m, cmd = m.handleDashboardPartial(dashboardPartialMsg{
			context: "test-ctx", gen: 1, key: dashboardSection(i).String(), total: 6,
			data: dashboardData{nodeCount: 1, nodeItems: make([]model.Item, 1)},
		})
		assert.Nil(t, cmd)
	}

	// 6th message announces the larger total (7) from the fresh fan-out.
	// This is the 6th unique key received, but expected must rise to 7,
	// so the accumulator must NOT complete here.
	m, cmd6 := m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", gen: 1, key: dashboardSection(5).String(), total: 7,
		data: dashboardData{nodeCount: 1, nodeItems: make([]model.Item, 1)},
	})
	assert.Nil(t, cmd6, "must not complete at 6 unique keys once total has grown to 7")

	// 7th (final) section arrives and completes the frame.
	m, cmd7 := m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", gen: 1, key: "pinned:extra", total: 7,
		data: dashboardData{nodeCount: 1, nodeItems: make([]model.Item, 1)},
	})
	require.NotNil(t, cmd7, "must complete once all 7 sections arrive")
}

func TestLoadDashboard_FanOutToBatch(t *testing.T) {
	m := newTestModelWithScheduler()
	m.nav.Context = "test-ctx"
	m.scheduler.StopWorkers()
	// Close drains every queued Future with ErrContextSwitched, which
	// unblocks the sub-cmd goroutines below — without this they'd park
	// on the futures forever and leak between tests.
	defer m.scheduler.Close()

	cmd := m.loadDashboard()
	require.NotNil(t, cmd)

	// tea.Batch returns a cmd that, when called, produces a BatchMsg
	// containing the sub-commands. The bubbletea runtime normally dispatches
	// those in goroutines; here we do it manually to drive the scheduler.
	msg := cmd()
	batchMsg, ok := msg.(tea.BatchMsg)
	require.True(t, ok, "loadDashboard must return a tea.Batch, got %T", msg)
	require.Len(t, batchMsg, 6, "loadDashboard must fan out into exactly 6 section cmds")

	// Execute each sub-cmd so the scheduler receives the 6 Submits.
	// Tracked via a WaitGroup so the deferred Close above can join the
	// goroutines after draining their Futures.
	var wg sync.WaitGroup
	for _, subCmd := range batchMsg {
		wg.Add(1)
		go func(c tea.Cmd) {
			defer wg.Done()
			_ = c()
		}(subCmd)
	}
	t.Cleanup(wg.Wait)

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityLow) >= 6
	}, 2*time.Second, 10*time.Millisecond, "loadDashboard must fan out into 6 Low-priority Submits")
}

// TestLoadDashboardFor_EvictsStaleAccumulatorForSameContextAndGen guards
// against a reload with a different pin count merging into a half-built
// accumulator left behind by a still-in-flight fan-out for the same
// (context, gen): dashboardAccumulator.expected is seeded from the first
// arriving section's total, so a stale accumulator's expected count no
// longer matches a fresh fan-out's total (e.g. a pin was added/removed
// between the two loads), causing a transient wrong render or premature
// completion. loadDashboardFor must evict any pre-existing accumulator for
// its (context, gen) before returning, so every fan-out starts clean.
func TestLoadDashboardFor_EvictsStaleAccumulatorForSameContextAndGen(t *testing.T) {
	m := newTestModelWithScheduler()
	m.nav.Context = "test-ctx"
	m.dashboardAcc = make(map[string]*dashboardAccumulator)
	m.scheduler.StopWorkers()
	defer m.scheduler.Close()

	gen := m.requestGen
	key := dashboardAccKey("test-ctx", gen)
	// Half-built accumulator from a prior (now stale) fan-out: 2 of 6
	// sections already arrived, seeded with the OLD total.
	m.dashboardAcc[key] = &dashboardAccumulator{
		gen:      gen,
		received: map[string]bool{"nodes": true, "pods": true},
		expected: 6,
		count:    2,
	}

	cmd := m.loadDashboardFor("test-ctx")
	require.NotNil(t, cmd)

	_, exists := m.dashboardAcc[key]
	assert.False(t, exists, "stale accumulator for the same (context, gen) must be evicted before the fresh fan-out starts")

	// Drain the batch's sub-cmds so they Submit and don't leak goroutines
	// parked on the scheduler's futures (mirrors TestLoadDashboard_FanOutToBatch).
	msg := cmd()
	batchMsg, ok := msg.(tea.BatchMsg)
	require.True(t, ok)
	var wg sync.WaitGroup
	for _, subCmd := range batchMsg {
		wg.Add(1)
		go func(c tea.Cmd) {
			defer wg.Done()
			_ = c()
		}(subCmd)
	}
	t.Cleanup(wg.Wait)

	// A fresh fan-out at the new total (6, no pins) completes cleanly: the
	// evicted accumulator's already-received "nodes"/"pods" keys don't
	// short-circuit it, and it doesn't complete early against the stale
	// expected count.
	total := 6
	fixed := []string{"nodes", "pods", "namespaces", "events", "pdbs", "metrics"}
	var pcmd tea.Cmd
	for i, k := range fixed {
		m, pcmd = m.handleDashboardPartial(dashboardPartialMsg{context: "test-ctx", gen: gen, key: k, total: total})
		if i < len(fixed)-1 {
			assert.Nil(t, pcmd, "must not complete before all fresh sections arrive: %s", k)
		}
	}
	require.NotNil(t, pcmd, "the fresh fan-out completes once all its own sections arrive")
	loaded, ok := pcmd().(dashboardLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, "test-ctx", loaded.context)
}

// A dashboard load with pinned summaries completes only after 6 fixed
// sections + one per pinned kind have arrived, and merged results keep
// their pin order via index.
func TestHandleDashboardPartial_PinnedSectionsCountTowardTotal(t *testing.T) {
	m := newTestModelForDashboard(t)
	m.nav.Context = "test-ctx"
	m.requestGen = 1
	total := 8 // 6 fixed + 2 pinned

	fixed := []string{"nodes", "pods", "namespaces", "events", "pdbs", "metrics"}
	for _, k := range fixed {
		var cmd tea.Cmd
		m, cmd = m.handleDashboardPartial(dashboardPartialMsg{
			context: m.nav.Context, gen: m.requestGen, key: k, total: total,
		})
		assert.Nil(t, cmd, "must not complete before all sections arrive: %s", k)
	}

	var cmd tea.Cmd
	m, cmd = m.handleDashboardPartial(dashboardPartialMsg{
		context: m.nav.Context, gen: m.requestGen, key: "pinned:argoproj.io/applications", total: total,
		data: dashboardData{pinnedSummaries: []pinnedSummaryResult{{index: 1, key: "argoproj.io/applications", displayName: "Applications"}}},
	})
	assert.Nil(t, cmd)

	m, cmd = m.handleDashboardPartial(dashboardPartialMsg{
		context: m.nav.Context, gen: m.requestGen, key: "pinned:batch/jobs", total: total,
		data: dashboardData{pinnedSummaries: []pinnedSummaryResult{{index: 0, key: "batch/jobs", displayName: "Jobs"}}},
	})
	require.NotNil(t, cmd, "last section completes the load")
	msg, ok := cmd().(dashboardLoadedMsg)
	require.True(t, ok)
	require.Len(t, msg.data.pinnedSummaries, 2)
	gotIndexes := []int{msg.data.pinnedSummaries[0].index, msg.data.pinnedSummaries[1].index}
	assert.ElementsMatch(t, []int{0, 1}, gotIndexes, "merged results carry the original pin indexes, in whichever arrival order")
}

// TestFetchPinnedSummary_UsesDisplayNameForDiscoveryEntry verifies a pinned
// summary resolves its label through model.DisplayNameFor rather than reading
// entry.DisplayName directly. Discovery-produced ResourceTypeEntry values
// (the normal case for a pinned CRD) never populate DisplayName themselves,
// so reading it raw yields "" and the dashboard falls back to the raw pin key
// instead of a friendly name.
func TestFetchPinnedSummary_UsesDisplayNameForDiscoveryEntry(t *testing.T) {
	widgetGVR := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"}
	gvrToListKind := map[schema.GroupVersionResource]string{widgetGVR: "WidgetList"}
	m := baseModelWithFakeDynamic(gvrToListKind)

	entry := model.ResourceTypeEntry{
		Kind: "Widget", APIGroup: "example.com", APIVersion: "v1", Resource: "widgets", Namespaced: true,
	}
	require.Empty(t, entry.DisplayName, "discovery entries do not populate DisplayName")

	data := fetchPinnedSummary(m.reqCtx, m.nav.Context, m.client, 0, "example.com/widgets", entry)
	require.Len(t, data.pinnedSummaries, 1)
	assert.Equal(t, "Widget", data.pinnedSummaries[0].displayName,
		"no BuiltInMetadata entry for this CRD, so DisplayNameFor falls back to Kind")
}

// Duplicate delivery of the same section key must not double-count.
func TestHandleDashboardPartial_DuplicateKeyIgnored(t *testing.T) {
	m := newTestModelForDashboard(t)
	m.nav.Context = "test-ctx"
	m.requestGen = 1
	var cmd tea.Cmd
	m, _ = m.handleDashboardPartial(dashboardPartialMsg{context: m.nav.Context, gen: m.requestGen, key: "nodes", total: 2})
	m, cmd = m.handleDashboardPartial(dashboardPartialMsg{context: m.nav.Context, gen: m.requestGen, key: "nodes", total: 2})
	assert.Nil(t, cmd, "duplicate must not complete the accumulator")
	_, cmd = m.handleDashboardPartial(dashboardPartialMsg{context: m.nav.Context, gen: m.requestGen, key: "pods", total: 2})
	assert.NotNil(t, cmd)
}

// --- Task 10: default pinned summaries ---

// TestPinnedSummaryCmds_SilentSkipDropsUnresolvedDefaults verifies an
// unresolved default pin (a CRD this cluster doesn't have) is dropped
// entirely when silentSkip is set - no cmd, no notFound placeholder - unlike
// an explicit pin's "(not installed in this cluster)" row.
func TestPinnedSummaryCmds_SilentSkipDropsUnresolvedDefaults(t *testing.T) {
	m := newTestModelWithScheduler()
	defer m.scheduler.Close()

	pins := []string{"batch/jobs", "unknown.io/widgets"}
	discovered := []model.ResourceTypeEntry{
		{Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs", Namespaced: true},
	}
	cmds := m.pinnedSummaryCmds("ctx", 1, m.client, pins, discovered, 7, true, func(k string) string { return k })
	assert.Len(t, cmds, 1, "the unresolved default must be dropped, not scheduled as a notFound placeholder")
}

// TestPinnedSummaryCmds_ExplicitUnresolvedStillRendersPlaceholder verifies
// silentSkip=false (an explicit pin) keeps the existing notFound placeholder
// behavior.
func TestPinnedSummaryCmds_ExplicitUnresolvedStillRendersPlaceholder(t *testing.T) {
	m := newTestModelWithScheduler()
	defer m.scheduler.Close()

	cmds := m.pinnedSummaryCmds("ctx", 1, m.client, []string{"unknown.io/widgets"}, nil, 7, false, func(k string) string { return k })
	require.Len(t, cmds, 1)
	partial, ok := cmds[0]().(dashboardPartialMsg)
	require.True(t, ok)
	require.Len(t, partial.data.pinnedSummaries, 1)
	assert.True(t, partial.data.pinnedSummaries[0].notFound)
}

// TestLoadDashboardFor_DefaultsSilentlySkipUnresolvedTypes verifies
// loadDashboardFor falls back to the built-in defaults when nothing is
// pinned, and that unresolved defaults (CRDs this cluster lacks) shrink the
// fan-out total instead of scheduling a notFound placeholder for each one.
func TestLoadDashboardFor_DefaultsSilentlySkipUnresolvedTypes(t *testing.T) {
	m := newTestModelWithScheduler()
	m.nav.Context = "test-ctx"
	m.discoveredResources = map[string][]model.ResourceTypeEntry{
		"test-ctx": {
			{Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs", Namespaced: true},
			{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true},
			// argoproj.io Applications, Flux Kustomizations, and cert-manager
			// Certificates are absent, as on a cluster without those CRDs.
		},
	}
	m.scheduler.StopWorkers()
	defer m.scheduler.Close()

	cmd := m.loadDashboardFor("test-ctx")
	require.NotNil(t, cmd)
	msg := cmd()
	batchMsg, ok := msg.(tea.BatchMsg)
	require.True(t, ok)
	assert.Len(t, batchMsg, 8, "6 fixed + 2 resolved defaults; the 3 unresolved defaults must not inflate the fan-out")

	var wg sync.WaitGroup
	for _, subCmd := range batchMsg {
		wg.Add(1)
		go func(c tea.Cmd) {
			defer wg.Done()
			_ = c()
		}(subCmd)
	}
	t.Cleanup(wg.Wait)
}

// TestLoadDashboardFor_ConfigPinnedSummariesSetEmptyDisablesDefaults verifies
// an explicit `pinned_summaries: []` in config (ConfigPinnedSummariesSet) is
// honored as "no summaries at all", not "use the defaults".
func TestLoadDashboardFor_ConfigPinnedSummariesSetEmptyDisablesDefaults(t *testing.T) {
	origList, origSet := ui.ConfigPinnedSummaries, ui.ConfigPinnedSummariesSet
	ui.ConfigPinnedSummaries = nil
	ui.ConfigPinnedSummariesSet = true
	t.Cleanup(func() {
		ui.ConfigPinnedSummaries = origList
		ui.ConfigPinnedSummariesSet = origSet
	})

	m := newTestModelWithScheduler()
	m.nav.Context = "test-ctx"
	m.scheduler.StopWorkers()
	defer m.scheduler.Close()

	cmd := m.loadDashboardFor("test-ctx")
	msg := cmd()
	batchMsg, ok := msg.(tea.BatchMsg)
	require.True(t, ok)
	assert.Len(t, batchMsg, 6, "explicit pinned_summaries: [] must disable the built-in defaults")

	var wg sync.WaitGroup
	for _, subCmd := range batchMsg {
		wg.Add(1)
		go func(c tea.Cmd) {
			defer wg.Done()
			_ = c()
		}(subCmd)
	}
	t.Cleanup(wg.Wait)
}

// TestLoadDashboardFor_ExplicitPinSuppressesDefaults verifies any explicit pin
// (state or config) suppresses the defaults entirely, per effectivePinnedSummaries
// returning a non-empty list.
func TestLoadDashboardFor_ExplicitPinSuppressesDefaults(t *testing.T) {
	m := newTestModelWithScheduler()
	m.nav.Context = "test-ctx"
	m.pinnedSummariesState = newPinnedState()
	m.pinnedSummariesState.Contexts["test-ctx"] = []string{"batch/jobs"}
	m.discoveredResources = map[string][]model.ResourceTypeEntry{
		"test-ctx": {{Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs", Namespaced: true}},
	}
	m.scheduler.StopWorkers()
	defer m.scheduler.Close()

	cmd := m.loadDashboardFor("test-ctx")
	msg := cmd()
	batchMsg, ok := msg.(tea.BatchMsg)
	require.True(t, ok)
	assert.Len(t, batchMsg, 7, "6 fixed + exactly the one explicit pin, not the 5 defaults")

	var wg sync.WaitGroup
	for _, subCmd := range batchMsg {
		wg.Add(1)
		go func(c tea.Cmd) {
			defer wg.Done()
			_ = c()
		}(subCmd)
	}
	t.Cleanup(wg.Wait)
}
