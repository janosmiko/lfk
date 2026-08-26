package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// Re-issuing the fan-out while the previous one is still filling cannot speed
// it up: every section coalesces against the submission already in flight. It
// only churns the queue, and on a cluster slower than the watch interval the
// page sits on the loading placeholder between rare full frames (issue #646).
func TestLoadDashboardForSkipsReissueWhileFanOutIsFilling(t *testing.T) {
	m, key := dashboardModelWithFillingFanOut(t, time.Now())

	assert.Nil(t, m.loadDashboardFor("test-ctx"),
		"a fan-out still filling must not be re-issued")

	acc, ok := m.dashboardAcc[key]
	require.True(t, ok, "the in-flight accumulator must survive the skipped tick")
	assert.Equal(t, 2, acc.count, "the skipped tick must not touch its progress")
}

// A section that never answers would otherwise hold the dashboard on the
// placeholder for the life of the process.
func TestLoadDashboardForReissuesAfterAStuckFanOut(t *testing.T) {
	m, key := dashboardModelWithFillingFanOut(t, time.Now().Add(-2*dashboardFanOutStuckAfter))

	cmd := m.loadDashboardFor("test-ctx")
	require.NotNil(t, cmd, "a stuck fan-out must be replaced by a fresh one")

	_, ok := m.dashboardAcc[key]
	assert.False(t, ok, "the stuck accumulator must be evicted")

	drainBatch(t, cmd, m.scheduler.Close)
}

// dashboardModelWithFillingFanOut returns a model whose fan-out for test-ctx is
// two sections into the six fixed ones, having started at startedAt.
func dashboardModelWithFillingFanOut(t *testing.T, startedAt time.Time) (Model, string) {
	t.Helper()
	m := newTestModelWithScheduler()
	m.nav.Context = "test-ctx"
	m.dashboardAcc = make(map[string]*dashboardAccumulator)
	m.scheduler.StopWorkers()

	key := dashboardAccKey("test-ctx", m.requestGen)
	m.dashboardAcc[key] = &dashboardAccumulator{
		gen:       m.requestGen,
		received:  map[string]bool{"nodes": true, "pods": true},
		expected:  6, // the six fixed sections, no pins resolve in this fixture
		count:     2,
		startedAt: startedAt,
	}
	return m, key
}

// A section that answers nothing would otherwise pin the page on its loading
// placeholder for the life of the process (issue #646). Once the fan-out is old
// enough that loadDashboardFor writes it off, a partial paints what did arrive.
func TestDashboardPartialPaintsOnceTheFanOutIsStuck(t *testing.T) {
	m := newTestModelForDashboard(t)
	m.nav.Context = "test-ctx"
	m.dashboardAcc[dashboardAccKey("test-ctx", m.requestGen)] = &dashboardAccumulator{
		gen:       m.requestGen,
		received:  map[string]bool{"pods": true},
		expected:  6,
		count:     1,
		startedAt: time.Now().Add(-2 * dashboardFanOutStuckAfter),
	}

	m, cmd := m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", key: "nodes", total: 6,
		data: dashboardData{nodeItems: []model.Item{{Name: "n1"}}, nodeCount: 1, readyNodes: 1},
	})

	require.NotNil(t, cmd, "a stuck fan-out must paint what it has")
	loaded, ok := cmd().(dashboardLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, 1, loaded.data.nodeCount)

	// Painting must not consume the accumulator: the sections still in flight
	// have to be able to complete this same frame.
	acc, ok := m.dashboardAcc[dashboardAccKey("test-ctx", m.requestGen)]
	require.True(t, ok, "an incomplete paint must keep the accumulator")
	assert.Equal(t, 2, acc.count)
}

// A mid-fill repaint would only cost renders and show the dashboard filling in
// row by row: the fan-out completes on its own a moment later.
func TestDashboardPartialWaitsForTheRestOnceAFrameIsUp(t *testing.T) {
	m := newTestModelForDashboard(t)
	m.nav.Context = "test-ctx"

	_, cmd := m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", key: "nodes", total: 6,
		data: dashboardData{nodeItems: []model.Item{{Name: "n1"}}, nodeCount: 1},
	})

	assert.Nil(t, cmd, "a partial fill must not repaint over a frame that is already up")
}

// A section that answers nothing this cycle must keep the value it had, not
// blank out. The accumulator therefore starts from the last full frame.
func TestDashboardAccumulatorStartsFromTheLastFrame(t *testing.T) {
	m := newTestModelForDashboard(t)
	m.nav.Context = "test-ctx"
	m.dashboardData = map[string]dashboardData{"test-ctx": {
		nodeCount:       7,
		pinnedSummaries: []pinnedSummaryResult{{key: "apps/deployments"}},
	}}

	m, _ = m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", key: "namespaces", total: 6,
		data: dashboardData{nsCount: 3},
	})

	acc := m.dashboardAcc[dashboardAccKey("test-ctx", m.requestGen)]
	require.NotNil(t, acc)
	assert.Equal(t, 7, acc.data.nodeCount, "an unarrived section keeps its previous value")
	assert.Empty(t, acc.data.pinnedSummaries,
		"pinned summaries append, so carrying them over would duplicate every row")
}

// Navigation bumps requestGen and the scheduler cancels the sections feeding
// the old accumulator, so no partial arrives to clear it. Left in the map it
// would outlive the fan-out it belongs to.
func TestLoadDashboardForEvictsAccumulatorsFromAnOlderGeneration(t *testing.T) {
	m := newTestModelWithScheduler()
	m.nav.Context = "test-ctx"
	m.dashboardAcc = make(map[string]*dashboardAccumulator)
	m.scheduler.StopWorkers()

	stale := dashboardAccKey("test-ctx", m.requestGen)
	m.dashboardAcc[stale] = &dashboardAccumulator{
		gen: m.requestGen, received: map[string]bool{"nodes": true},
		expected: 6, count: 1, startedAt: time.Now(),
	}
	// A context name may contain a colon, so this key starts with "test-ctx:"
	// and a prefix match would take it with the stale one.
	sibling := dashboardAccKey("test-ctx:secondary", m.requestGen)
	m.dashboardAcc[sibling] = &dashboardAccumulator{
		gen: m.requestGen, received: map[string]bool{"nodes": true},
		expected: 6, count: 1, startedAt: time.Now(),
	}

	m.requestGen++
	cmd := m.loadDashboardFor("test-ctx")
	require.NotNil(t, cmd)

	_, ok := m.dashboardAcc[stale]
	assert.False(t, ok, "an accumulator from an older generation must be evicted")
	_, ok = m.dashboardAcc[sibling]
	assert.True(t, ok, "another context's accumulator must be left alone")

	drainBatch(t, cmd, m.scheduler.Close)
}

// The dashboard used to paint the first section that answered and then hold
// every later one, so the user watched it sit on a single row for the eight
// seconds the slowest section took. It must show one loader instead, then the
// whole frame at once. Asserted on the rendered pane, because what the user
// sees is the whole point of the change.
func TestDashboardPartialHoldsEverySectionUntilTheFrameIsWhole(t *testing.T) {
	m := dashboardOverviewModel()

	keys := []string{"namespaces", "pdbs", "events", "nodes", "metrics", "pods"}
	for _, key := range keys[:len(keys)-1] {
		var cmd tea.Cmd
		m, cmd = m.handleDashboardPartial(dashboardPartialMsg{
			context: "test-ctx", key: key, total: len(keys),
			data: dashboardData{nsCount: 29},
		})
		require.Nil(t, cmd, "section %q must not paint on its own", key)
		assert.Contains(t, stripANSI(m.renderRightResourceTypes(60, 20)),
			"Loading cluster dashboard", "the loader must stay up until the frame is whole")
	}

	var cmd tea.Cmd
	m, cmd = m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", key: keys[len(keys)-1], total: len(keys),
		data: dashboardData{nodeItems: []model.Item{{Name: "n1"}}, nodeCount: 1, readyNodes: 1},
	})
	require.NotNil(t, cmd, "the last section completes the frame and paints it")
	loaded, ok := cmd().(dashboardLoadedMsg)
	require.True(t, ok)
	m = m.updateDashboardLoaded(loaded)

	view := stripANSI(m.renderRightResourceTypes(60, 20))
	assert.NotContains(t, view, "Loading cluster dashboard", "the loader must be gone")
	assert.Contains(t, view, "Namespaces: 29",
		"every section arrives in the same frame, not one at a time")
	assert.Contains(t, view, "1 Ready", "including the one that completed it")
}

// dashboardOverviewModel parks the cursor on the Cluster dashboard entry, the
// only place renderRightResourceTypes renders the dashboard or its loader.
func dashboardOverviewModel() Model {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.dashboardAcc = make(map[string]*dashboardAccumulator)
	m.middleItems = []model.Item{{Name: "Cluster", Extra: "__overview__"}}
	m.setCursor(0)
	return m
}
