package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// handleMetricsSparkCycle advances the CPU/MEM display mode and refetches so
// the new mode has data to draw.
func (m Model) handleMetricsSparkCycle() (Model, tea.Cmd) {
	m.metricsSpark = m.metricsSpark.Next()
	m.setStatusMessage("CPU/MEM: "+m.metricsSpark.Label(), false)

	cmds := []tea.Cmd{scheduleStatusClear()}
	kind := m.dashboardMetricsKind()
	if m.metricsSpark.Mode == ui.MetricsDisplaySpark {
		// Bypass the throttle: the user just asked for this window, so a
		// stamped recent fetch must not swallow the request.
		if cmd := m.loadMetricsRangeForKind(kind); cmd != nil {
			cmds = append(cmds, cmd)
		}
	} else {
		m.metricsSeries = metricsSeriesCache{}
	}
	if kind == "Cluster" {
		// The dashboard bakes its sparkline lines into a cached preview
		// string instead of recomputing them per render like the list
		// columns do, so the mode toggle needs an explicit recompose.
		m = m.recomposeDashboard()
	}
	// Both branches need a repaint now rather than at the next watch tick:
	// leaving mode drops the series above, entering it changes what the
	// cells should show as soon as the fetch lands.
	m.middleItemsRev++
	return m, tea.Batch(cmds...)
}

// dashboardMetricsKind reports "Cluster" when the CLUSTER RESOURCES dashboard
// is on screen. nav.ResourceType stays empty there in both preview and
// fullscreen (see navigateChildResourceType), so the kind comes from the
// hovered pseudo-item instead, mirroring explorerHintEntries' isDashboard
// check. "__monitoring__" has no CPU/Mem section, so it is excluded.
func (m Model) dashboardMetricsKind() string {
	sel := m.selectedMiddleItem()
	if sel != nil && m.nav.Level == model.LevelResourceTypes && sel.Extra == "__overview__" {
		return "Cluster"
	}
	return m.nav.ResourceType.Kind
}
