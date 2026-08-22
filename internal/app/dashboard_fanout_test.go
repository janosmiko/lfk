package app

import (
	"testing"
	"time"

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

// Waiting for all sections leaves the page on its loading placeholder for as
// long as the slowest section takes, and forever when one never answers. While
// nothing is on screen yet, each partial must paint what it has (issue #646).
func TestDashboardPartialPaintsWhileTheScreenIsEmpty(t *testing.T) {
	m := newTestModelForDashboard(t)
	m.nav.Context = "test-ctx"

	_, cmd := m.handleDashboardPartial(dashboardPartialMsg{
		context: "test-ctx", key: "nodes", total: 6,
		data: dashboardData{nodeItems: []model.Item{{Name: "n1"}}, nodeCount: 1, readyNodes: 1},
	})

	require.NotNil(t, cmd, "the first section must paint instead of waiting for the rest")
	loaded, ok := cmd().(dashboardLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, 1, loaded.data.nodeCount)
}

// Once a frame is up, a mid-fill repaint would only cost renders: the fan-out
// completes on its own a moment later.
func TestDashboardPartialWaitsForTheRestOnceAFrameIsUp(t *testing.T) {
	m := newTestModelForDashboard(t)
	m.nav.Context = "test-ctx"
	m.dashboardPreview = "CLUSTER DASHBOARD"

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
