package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// setMetricsInterval pins the throttle window for one test and restores the
// process-global afterwards.
func setMetricsInterval(t *testing.T, d time.Duration) {
	t.Helper()
	prev := ui.ConfigMetricsInterval
	t.Cleanup(func() { ui.ConfigMetricsInterval = prev })
	ui.ConfigMetricsInterval = d
}

func newMetricsThrottleModel(kind string) Model {
	return Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			Context:      "ctx-a",
			ResourceType: model.ResourceTypeEntry{Kind: kind, Resource: "pods", APIVersion: "v1"},
		},
	}
}

func TestAllowMetricsFetch_FirstLoadDispatchesImmediately(t *testing.T) {
	setMetricsInterval(t, time.Hour)
	for _, kind := range []string{"Pod", "Node"} {
		t.Run(kind, func(t *testing.T) {
			m := newMetricsThrottleModel(kind)
			m.suppressBgtasks = true
			assert.True(t, m.allowMetricsFetch(kind),
				"a first list load must fetch metrics even on a suppressed tick")
		})
	}
}

func TestAllowMetricsFetch_SuppressedTicksInsideIntervalFetchOnce(t *testing.T) {
	setMetricsInterval(t, time.Hour)
	for _, kind := range []string{"Pod", "Node"} {
		t.Run(kind, func(t *testing.T) {
			m := newMetricsThrottleModel(kind)
			m.suppressBgtasks = true
			require.True(t, m.allowMetricsFetch(kind))
			assert.False(t, m.allowMetricsFetch(kind),
				"a second suppressed tick inside the interval must skip the fetch")
		})
	}
}

func TestAllowMetricsFetch_ManualRefreshIgnoresInterval(t *testing.T) {
	setMetricsInterval(t, time.Hour)
	for _, kind := range []string{"Pod", "Node"} {
		t.Run(kind, func(t *testing.T) {
			m := newMetricsThrottleModel(kind)
			m.suppressBgtasks = true
			require.True(t, m.allowMetricsFetch(kind))
			m.suppressBgtasks = false
			assert.True(t, m.allowMetricsFetch(kind),
				"a user-driven refresh must fetch metrics regardless of the interval")
		})
	}
}

func TestAllowMetricsFetch_ZeroIntervalFetchesEveryTick(t *testing.T) {
	setMetricsInterval(t, 0)
	for _, kind := range []string{"Pod", "Node"} {
		t.Run(kind, func(t *testing.T) {
			m := newMetricsThrottleModel(kind)
			m.suppressBgtasks = true
			require.True(t, m.allowMetricsFetch(kind))
			assert.True(t, m.allowMetricsFetch(kind),
				"metrics_interval 0 disables the throttle")
		})
	}
}

func TestAllowMetricsFetch_ElapsedIntervalFetchesAgain(t *testing.T) {
	setMetricsInterval(t, time.Millisecond)
	m := newMetricsThrottleModel("Pod")
	m.suppressBgtasks = true
	require.True(t, m.allowMetricsFetch("Pod"))
	time.Sleep(2 * time.Millisecond)
	assert.True(t, m.allowMetricsFetch("Pod"),
		"a suppressed tick past the interval must fetch again")
}

func TestAllowMetricsFetch_KeyedByContextAndKind(t *testing.T) {
	setMetricsInterval(t, time.Hour)
	m := newMetricsThrottleModel("Pod")
	m.suppressBgtasks = true
	require.True(t, m.allowMetricsFetch("Pod"))
	assert.True(t, m.allowMetricsFetch("Node"),
		"Node metrics keep their own stamp")
	m.nav.Context = "ctx-b"
	assert.True(t, m.allowMetricsFetch("Pod"),
		"another cluster's metrics keep their own stamp")
}

// secondTickQueue runs two list loads on a fresh model and returns the names of
// the cluster calls the second load submitted. Reading the queue beats counting
// the returned cmds: tea.Batch collapses a single-cmd batch into that cmd, and
// the preview cmd is itself a batch, so the counts are ambiguous.
func secondTickQueue(t *testing.T, kind string, interval time.Duration, silent bool) []string {
	t.Helper()
	m := newLoadResourcesTestModel(t)
	m.cacheFingerprints = make(map[string]string)
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: kind, Resource: "pods", APIVersion: "v1", Namespaced: true}
	msg := resourcesLoadedMsg{
		items:  []model.Item{{Name: "a", Kind: kind, Namespace: "default"}},
		gen:    m.requestGen,
		rt:     m.nav.ResourceType,
		silent: true,
	}
	ui.ConfigMetricsInterval = time.Hour
	loaded, _ := m.updateResourcesLoadedMain(msg)
	m, _ = loaded.(Model)

	// A fresh registry for the second load. The scheduler coalesces a repeat
	// submission against the still-queued first one, which would hide whether
	// the second load submitted anything at all.
	m.scheduler = scheduler.New(0)
	ui.ConfigMetricsInterval = interval
	msg.silent = silent
	res, _ := m.updateResourcesLoadedMain(msg)
	m2, ok := res.(Model)
	require.True(t, ok)

	queued := m2.scheduler.QueueSnapshot()
	names := make([]string, 0, len(queued))
	for _, e := range queued {
		names = append(names, e.Name)
	}
	return names
}

// Proves the gate sits at the dispatch site, not only in the helper.
func TestUpdateResourcesLoadedMain_ThrottlesSilentMetricsFetch(t *testing.T) {
	setMetricsInterval(t, time.Hour)
	for _, kind := range []string{"Pod", "Node"} {
		t.Run(kind, func(t *testing.T) {
			call := kind + " metrics"
			assert.NotContains(t, secondTickQueue(t, kind, time.Hour, true), call,
				"a silent tick inside the interval must not fetch metrics")
			assert.Contains(t, secondTickQueue(t, kind, 0, true), call,
				"metrics_interval 0 fetches on every tick")
			assert.Contains(t, secondTickQueue(t, kind, time.Hour, false), call,
				"a manual refresh inside the interval must still fetch metrics")
		})
	}
}

// Node uptime rides the same dispatch branch but is not a metrics-server call,
// so the throttle must leave it alone.
func TestUpdateResourcesLoadedMain_ThrottleKeepsNodeUptime(t *testing.T) {
	setMetricsInterval(t, time.Hour)
	assert.Contains(t, secondTickQueue(t, "Node", time.Hour, true), "Node uptime")
}
