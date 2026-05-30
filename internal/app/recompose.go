package app

import "github.com/janosmiko/lfk/internal/ui"

// recomposeThemedContent re-renders every cached, pre-rendered preview string
// (dashboard, monitoring, metrics bar, events footer) from its retained raw
// data. Those strings bake the active theme's color codes into their bytes, so
// a theme switch otherwise leaves them stale until the next data tick.
// Recomposing here applies the new theme immediately; because the bars are also
// width-dependent, it doubles as the resize refresh path.
func (m Model) recomposeThemedContent() Model {
	m = m.recomposeDashboard()
	m = m.recomposeMonitoring()
	m = m.recomposeMetrics()
	m = m.recomposePreviewEvents()
	return m
}

// recomposeDashboard re-renders dashboardPreview / dashboardEventsPreview from
// the retained data at the current width. No-op when no data is loaded for the
// active context. Called on data load, fullscreen toggle, and window resize so
// the bars always fit the available space.
func (m Model) recomposeDashboard() Model {
	ctx := m.dashboardPreviewTargetContext()
	if data, ok := m.dashboardData[ctx]; ok {
		m.dashboardPreview, m.dashboardEventsPreview = m.composeDashboard(data)
	}
	return m
}

// recomposeMonitoring re-renders monitoringPreview from the retained alert data
// for the active context. No-op when no data is loaded for that context.
func (m Model) recomposeMonitoring() Model {
	ctx := m.dashboardPreviewTargetContext()
	if data, ok := m.monitoringData[ctx]; ok {
		m.monitoringPreview = composeMonitoring(data.alerts, data.errMsg)
	}
	return m
}

// recomposeMetrics re-renders the right-pane resource-usage bar from the
// retained raw numbers at the current width. No-op when no metrics are loaded.
func (m Model) recomposeMetrics() Model {
	if m.metricsData == nil {
		return m
	}
	d := m.metricsData
	m.metricsContent = ui.RenderResourceUsage(
		d.cpuUsed, d.cpuReq, d.cpuLim,
		d.memUsed, d.memReq, d.memLim,
		m.rightFooterInnerWidth(),
	)
	return m
}

// recomposePreviewEvents re-renders the right-pane events footer from the
// retained entries at the current width. No-op when no events are loaded.
func (m Model) recomposePreviewEvents() Model {
	if len(m.previewEventsData) == 0 {
		return m
	}
	m.previewEventsContent = ui.RenderPreviewEvents(m.previewEventsData, m.rightFooterInnerWidth())
	return m
}

// rightFooterInnerWidth returns the content width of the right-column footer
// region (resource-usage bar / events), matching renderRightColumn's geometry.
func (m Model) rightFooterInnerWidth() int {
	usable := m.width - 6
	rightW := max(10, usable-max(10, usable*12/100)-max(10, usable*51/100))
	return max(rightW-4, 20)
}
