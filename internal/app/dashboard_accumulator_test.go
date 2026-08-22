package app

import (
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// withDashboardFrameUp puts a frame on screen so handleDashboardPartial takes
// its steady-state path, where the repaint waits for every section. With no
// frame up it paints each section as it lands instead, so the page never sits
// on its loading placeholder.
func withDashboardFrameUp(m Model) Model {
	m.dashboardPreview = "CLUSTER DASHBOARD"
	return m
}

func TestHandleDashboardPartial_AccumulatesSections(t *testing.T) {
	m := newTestModelForDashboard(t)
	m = withDashboardFrameUp(m)
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
	m = withDashboardFrameUp(m)
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
	m = withDashboardFrameUp(m)
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
	m = withDashboardFrameUp(m)
	m.nav.Context = "test-ctx"
	m.dashboardAcc = make(map[string]*dashboardAccumulator)
	m.scheduler.StopWorkers()
	defer m.scheduler.Close()

	gen := m.requestGen
	key := dashboardAccKey("test-ctx", gen)
	// Half-built accumulator from a prior (now stale) fan-out: 2 sections
	// already arrived, seeded with the OLD total. The 7 stands for a pin that
	// was dropped between the two loads, so it no longer matches the fresh
	// fan-out's 6 and the accumulator cannot be reused.
	m.dashboardAcc[key] = &dashboardAccumulator{
		gen:      gen,
		received: map[string]bool{"nodes": true, "pods": true},
		expected: 7,
		count:    2,
		// Recent, so only the expected mismatch can evict it. The age clause
		// has its own test.
		startedAt: time.Now(),
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
	m = withDashboardFrameUp(m)
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
	m = withDashboardFrameUp(m)
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
