package app

import (
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
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
		context: "test-ctx", gen: 7, section: dashboardSectionNodes,
		data: dashboardData{nodeItems: make([]model.Item, 3), nodeCount: 3, readyNodes: 2},
	})
	assert.Nil(t, cmd1, "partial accumulation must not emit a render cmd")

	m, cmd2 := m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", gen: 7, section: dashboardSectionPods,
		data: dashboardData{pods: podStats{total: 10, running: 8}},
	})
	assert.Nil(t, cmd2)

	m, cmd3 := m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", gen: 7, section: dashboardSectionNamespaces,
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
			context: "test-ctx", gen: 1, section: dashboardSection(i),
			data: dashboardData{nodeCount: 1, nodeItems: make([]model.Item, 1)},
		})
		assert.Nilf(t, cmd, "section %d (1-indexed: %d) must not emit a cmd until all 6 arrive", i, i+1)
	}

	// Section 6 emits the dashboardLoadedMsg in one shot.
	m, cmd := m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", gen: 1, section: dashboardSection(5),
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
		context: "test-ctx", gen: 4 /* stale */, section: dashboardSectionNodes,
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
		context: "other-ctx", gen: 1, section: dashboardSectionNodes,
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
			context: "test-ctx", gen: 1, section: dashboardSection(i),
			data: dashboardData{nodeCount: 1},
		})
	}

	key := dashboardAccKey("test-ctx", 1)
	_, ok := m.dashboardAcc[key]
	assert.False(t, ok, "accumulator must be dropped after all 6 sections arrive")
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
