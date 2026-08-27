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
		m.monitoringPreview = composeMonitoring(ctx, data.alerts, data.errMsg)
	}
	return m
}

// recomposeMetrics re-renders the right-pane resource-usage bar at the current
// width: the real bar from retained raw numbers, or a loading placeholder while
// a metrics fetch is in flight. No-op when neither metrics nor a load are
// pending, so it preserves an already-empty footer on theme/resize.
func (m Model) recomposeMetrics() Model {
	switch {
	case m.metricsData != nil:
		d := m.metricsData
		m.metricsContent = ui.RenderResourceUsage(
			d.cpuUsed, d.cpuReq, d.cpuLim,
			d.memUsed, d.memReq, d.memLim,
			m.rightFooterInnerWidth(),
		)
	case m.metricsLoading:
		m.metricsContent = ui.RenderResourceUsagePlaceholder()
	}
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
// region (resource-usage bar / events). It must equal the column's inner width
// (rightW-colPad, colPad=2) so the footer fills the same span as the separator
// above it and the children table — see renderRightColumn and the matching
// innerW in clampPreviewScroll.
func (m Model) rightFooterInnerWidth() int {
	_, _, rightW := m.explorerColumnWidths()
	return max(rightW-2, 20)
}
