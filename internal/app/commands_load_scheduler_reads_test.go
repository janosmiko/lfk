package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newReadsSchedulerModel returns a baseFinalModel wired to a fresh, worker-less
// scheduler. With workers stopped the submitted task stays queued, so a wired
// loader can be observed synchronously via QueueLen — the same technique as
// TestLoadResourcesRegistersTaskSynchronously.
func newReadsSchedulerModel() Model {
	m := baseFinalModel()
	m.scheduler = scheduler.New(0)
	m.scheduler.StopWorkers()
	return m
}

// TestK8sReadsAreScheduled asserts every previously-unwired K8s read loader now
// dispatches through scheduleK8sCall (which Submits synchronously while the Cmd
// is built), rather than running in a raw closure that bypasses the worker pool,
// priority lanes, coalescing, and gen-based cancellation.
func TestK8sReadsAreScheduled(t *testing.T) {
	t.Run("loadSecretData", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadSecretData())
	})
	t.Run("loadConfigMapData", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadConfigMapData())
	})
	t.Run("loadLabelData", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadLabelData())
	})
	t.Run("loadRevisions", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadRevisions())
	})
	t.Run("loadContainersForAction", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadContainersForAction())
	})
	t.Run("loadContainersForLogFilter", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadContainersForLogFilter())
	})
	t.Run("detectExecPodOSCmd", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.detectExecPodOSCmd())
	})
	t.Run("loadPodsForAction", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadPodsForAction())
	})
	t.Run("loadRightsizing", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadRightsizing())
	})
	t.Run("copyYAMLToClipboard", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.copyYAMLToClipboard())
	})
	t.Run("copyYAMLForScope", func(t *testing.T) {
		m := newReadsSchedulerModel()
		scope := []model.Item{{Name: "pod-1", Namespace: "default", Kind: "Pod"}}
		assertSchedulesOne(t, m, m.copyYAMLForScope(scope))
	})
	t.Run("exportResourceToFile", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.exportResourceToFile())
	})
	t.Run("loadDiff", func(t *testing.T) {
		m := newReadsSchedulerModel()
		rt := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
		itemA := model.Item{Name: "pod-1", Namespace: "default", Kind: "Pod"}
		itemB := model.Item{Name: "pod-2", Namespace: "default", Kind: "Pod"}
		assertSchedulesOne(t, m, m.loadDiff(rt, itemA, itemB))
	})
	t.Run("watchArgoWorkflow", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.watchArgoWorkflow())
	})
	t.Run("loadAutoSyncConfig", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadAutoSyncConfig())
	})
}

// assertSchedulesOne fails unless cmd is non-nil and a task was Submitted
// synchronously onto the model's scheduler (QueueLen == 1) at Cmd-construction
// time — proof the loader routes through scheduleK8sCall, not a raw closure.
func assertSchedulesOne(t *testing.T, m Model, cmd tea.Cmd) {
	t.Helper()
	require.NotNil(t, cmd, "loader returned nil cmd; preconditions not met")
	assert.Equal(t, 1, m.scheduler.QueueLen(m.nav.Context),
		"loader must Submit one task synchronously via scheduleK8sCall")
}
