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
	case "Node":
		if m.allowMetricsFetch(kind) {
			cmds = append(cmds, m.loadNodeMetricsForList())
		}
		cmds = append(cmds, m.loadNodeUptimeForList())
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
