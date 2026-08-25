package app

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

// listMetricsCmds returns the list-wide enrichment loaders for kind. Node
// uptime comes from Prometheus rather than metrics-server, so the throttle
// leaves it alone.
func (m *Model) listMetricsCmds(kind string) []tea.Cmd {
	var cmds []tea.Cmd
	switch kind {
	case "Pod":
		if m.allowMetricsFetch(kind) {
			cmds = append(cmds, m.loadPodMetricsForList())
		}
		if m.metricsSpark.Mode == ui.MetricsDisplaySpark && m.allowSparklineFetch(kind) {
			if cmd := m.loadPodMetricsRangeForList(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case "Node":
		if m.allowMetricsFetch(kind) {
			cmds = append(cmds, m.loadNodeMetricsForList())
		}
		cmds = append(cmds, m.loadNodeUptimeForList())
		if m.metricsSpark.Mode == ui.MetricsDisplaySpark && m.allowSparklineFetch(kind) {
			if cmd := m.loadNodeMetricsRangeForList(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	return cmds
}

// allowMetricsFetch reports whether the list-wide CPU/MEM fetch for kind may
// run now, and stamps it when it does. metrics-server recomputes roughly every
// 15s, so most 2s watch ticks refetch identical numbers. Only suppressed
// refreshes are throttled, so a first load and a manual refresh always fetch.
func (m *Model) allowMetricsFetch(kind string) bool {
	key := m.nav.Context + "/" + kind
	if interval := ui.ConfigMetricsInterval; interval > 0 && m.suppressBgtasks {
		if last, ok := m.metricsLastFetch[key]; ok && time.Since(last) < interval {
			return false
		}
	}
	if m.metricsLastFetch == nil {
		m.metricsLastFetch = make(map[string]time.Time)
	}
	m.metricsLastFetch[key] = time.Now()
	return true
}

// allowSparklineFetch throttles the range fetch harder than allowMetricsFetch,
// since a range query reads a whole window per series, and stamps it under a
// separate key so the two throttles cannot starve each other.
func (m *Model) allowSparklineFetch(kind string) bool {
	key := m.nav.Context + "/spark/" + kind
	if interval := ui.ConfigSparklineInterval; interval > 0 {
		if last, ok := m.metricsLastFetch[key]; ok && time.Since(last) < interval {
			return false
		}
	}
	if m.metricsLastFetch == nil {
		m.metricsLastFetch = make(map[string]time.Time)
	}
	m.metricsLastFetch[key] = time.Now()
	return true
}

// containerMetricsCmds returns the metrics loaders for a freshly listed
// container list. Containers need their own entry point because they load
// through updateContainersLoaded and never reach listMetricsCmds, so without
// this the range fetch only ever fired from the hotkey handler and arriving at
// the container level in sparkline mode drew nothing.
func (m *Model) containerMetricsCmds() []tea.Cmd {
	var cmds []tea.Cmd
	if m.allowMetricsFetch("Container") {
		cmds = append(cmds, m.loadContainerMetricsForList())
	}
	if m.metricsSpark.Mode == ui.MetricsDisplaySpark && m.allowSparklineFetch("Container") {
		if cmd := m.loadContainerMetricsRangeForList(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}
