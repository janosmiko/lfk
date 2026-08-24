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
		cmds = append(cmds, m.loadMetricsRangeForList()...)
	} else {
		// Leaving sparkline mode drops the series so the columns repaint as
		// plain values on the next tick instead of keeping stale glyphs.
		m.metricsSeries = metricsSeriesCache{}
		cmds = append(cmds, m.refreshMetricsForList()...)
	}
	return m, tea.Batch(cmds...)
}

// TODO(task-9): replaced by the real loaders in Task 9.
func (m Model) loadMetricsRangeForList() []tea.Cmd { return nil }
func (m Model) refreshMetricsForList() []tea.Cmd   { return nil }

type metricsSeriesCache struct{}
