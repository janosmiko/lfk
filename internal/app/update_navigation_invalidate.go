package app

import (
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// reclaimStaleBgWork drops queued and cancels in-flight Low-priority
// scheduler tasks on the active context whose requestGen predates the
// current one. Call right after bumping m.requestGen on navigation: a
// cursor move or drill reclaims worker slots from dashboard/preview
// fetches whose results the gen check would discard anyway, instead of
// letting them block fresh work in the shared Low lane. High/Critical work
// (the user's current view, mutations) is left untouched — see
// scheduler.Registry.CancelStaleByGen.
func (m *Model) reclaimStaleBgWork() {
	m.scheduler.CancelStaleByGen(m.effectiveContext(), m.requestGen)
}

// invalidatePreviewForCursorChange resets the right-column state and bumps
// requestGen so any in-flight preview load triggered by the previous cursor
// position is discarded by its message handler instead of being applied to
// the wrong selection (which causes stale items to appear, followed by a
// brief "No resources found" flash before the new load returns).
//
// Does not cancel reqCtx: cancelling on cursor moves storms Bubble Tea's
// msg channel with context.Canceled deliveries that crowd out KeyMsgs.
func (m *Model) invalidatePreviewForCursorChange() {
	m.requestGen++
	m.reclaimStaleBgWork()
	m.previewDebounceGen++
	m.rightItems = nil
	m.previewYAML = ""
	m.previewScroll = 0
	m.resourceTree = nil
	m.loading = true
	m.previewLoading = true
	// Swap the previous resource's footers for the new selection's loading
	// state. Without this, the right-pane RESOURCE USAGE bar and events keep
	// the prior pod's numbers until the debounced fetch returns. For a
	// metrics-eligible selection we paint a placeholder bar; for everything
	// else the footer collapses.
	m.metricsData = nil
	m.previewEventsContent = ""
	m.previewEventsData = nil
	if m.selectionMetricsEligible() {
		m.metricsLoading = true
		m.metricsContent = ui.RenderResourceUsagePlaceholder()
	} else {
		m.metricsLoading = false
		m.metricsContent = ""
	}
	if isUnionDashboardResourceKind(m.nav.ResourceType.Kind) {
		m.dashboardPreview = ""
		m.dashboardEventsPreview = ""
		m.monitoringPreview = ""
	}
}

// selectionMetricsEligible reports whether the currently focused item is a kind
// for which loadPreview populates the RESOURCE USAGE footer. It mirrors the
// dispatch in loadPreviewResources / loadPreviewOwned so the loading
// placeholder appears exactly when a metrics fetch will follow — never for a
// kind (ConfigMap, Service, ...) whose footer stays empty.
func (m Model) selectionMetricsEligible() bool {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return false
	}
	switch m.nav.Level {
	case model.LevelResources:
		if isUnionDashboardResourceKind(m.nav.ResourceType.Kind) {
			return false
		}
		if m.nav.ResourceType.APIGroup == model.SecurityVirtualAPIGroup {
			return false
		}
		switch m.nav.ResourceType.Kind {
		case "Pod", "Deployment", "StatefulSet", "DaemonSet":
			return true
		}
		return false
	case model.LevelOwned:
		return sel.Kind == "Pod"
	}
	return false
}

// invalidatePreviewFingerprintForCurrentSelection drops the cache-freshness
// fingerprint for the resource type currently under the cursor at
// LevelResourceTypes. loadResources(forPreview=true) keys its hover-cycle
// cache shortcut on a (cache entry exists) AND (fingerprint matches)
// check; clearing the fingerprint forces the next loadPreview through
// the real fetch path. The cache entry itself is preserved so
// navigation-history instant-paint still hits.
//
// No-op if the cursor is not on a discovered resource type (e.g. a
// pseudo-resource like __overview__ or a collapsed group).
func (m *Model) invalidatePreviewFingerprintForCurrentSelection() {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return
	}
	rt, ok := model.FindResourceTypeIn(sel.Extra, m.discoveredResources[m.nav.Context])
	if !ok || rt.Resource == "" {
		return
	}
	delete(m.cacheFingerprints, m.nav.Context+"/"+rt.Resource)
}
