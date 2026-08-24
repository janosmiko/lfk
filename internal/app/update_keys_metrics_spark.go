package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

// handleMetricsSparkCycle advances the CPU/MEM display mode and refetches so
// the new mode has data to draw.
func (m Model) handleMetricsSparkCycle() (Model, tea.Cmd) {
	m.metricsSpark = m.metricsSpark.Next()
	m.setStatusMessage("CPU/MEM: "+m.metricsSpark.Label(), false)

	cmds := []tea.Cmd{scheduleStatusClear()}
	if m.metricsSpark.Mode == ui.MetricsDisplaySpark {
		// Bypass the throttle: the user just asked for this window, so a
		// stamped recent fetch must not swallow the request.
		if cmd := m.loadPodMetricsRangeForList(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	} else {
		m.metricsSeries = metricsSeriesCache{}
	}
	// Both branches need a repaint now rather than at the next watch tick:
	// leaving mode drops the series above, entering it changes what the
	// cells should show as soon as the fetch lands.
	m.middleItemsRev++
	return m, tea.Batch(cmds...)
}
