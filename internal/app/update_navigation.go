package app

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) moveCursor(delta int) (tea.Model, tea.Cmd) {
	visible := m.visibleMiddleItems()
	c := max(m.cursor()+delta, 0)
	if c >= len(visible) {
		c = len(visible) - 1
	}
	if c < 0 {
		c = 0
	}
	m.setCursor(c)

	// Accordion behavior: auto-expand the group the cursor just entered.
	if m.nav.Level == model.LevelResourceTypes && !m.allGroupsExpanded {
		visible = m.visibleMiddleItems()
		if c >= 0 && c < len(visible) {
			newCat := visible[c].Category
			if newCat != "" && newCat != m.expandedGroup {
				m.expandedGroup = newCat
				// Recompute visible items after expansion change.
				newVisible := m.visibleMiddleItems()

				if delta < 0 {
					// Moving UP into a group: land on the LAST item of that group.
					for i, item := range slices.Backward(newVisible) {
						if item.Category == newCat && item.Kind != "__collapsed_group__" {
							m.setCursor(i)
							break
						}
					}
				} else {
					// Moving DOWN into a group: land on the FIRST real item of that group.
					for i, item := range newVisible {
						if item.Category == newCat && item.Kind != "__collapsed_group__" {
							m.setCursor(i)
							break
						}
					}
				}
			}
		}
	}

	m.invalidatePreviewForCursorChange()
	return m, schedulePreviewDebounce(m.previewDebounceGen)
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
	m.previewDebounceGen++
	m.rightItems = nil
	m.previewYAML = ""
	m.previewScroll = 0
	m.resourceTree = nil
	m.loading = true
	m.previewLoading = true
	if isUnionDashboardResourceKind(m.nav.ResourceType.Kind) {
		m.dashboardPreview = ""
		m.dashboardEventsPreview = ""
		m.monitoringPreview = ""
	}
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

func (m Model) navigateParent() (tea.Model, tea.Cmd) {
	m.cancelAndReset()
	m.requestGen++
	m.clearSelection()
	m.activeFilterPreset = nil
	m.unfilteredMiddleItems = nil

	// Remember this level's filter before clearing it, so the parent level
	// can restore its own saved filter (see restoreLevelFilter calls below)
	// and a later re-visit to this level recalls what was typed here.
	m.saveLevelFilter()

	// Clear filter when navigating to a parent. Without this, a filter
	// committed at a child level (e.g. "deploy" at LevelResourceTypes)
	// stays in m.filterText and visibleMiddleItems silently filters out
	// every parent-level item whose name doesn't match — making the
	// cluster picker look empty after backing out of a filtered view.
	// The per-level restore below re-applies the parent's own filter (if any).
	m.filterText = ""
	m.filterInput.Clear()
	m.filterActive = false

	// Clear search highlight on level change so it doesn't bleed onto
	// the parent level (issue requested fix). The Esc cascade clears
	// search as its own step before navigating, so this only fires for
	// programmatic navigateParent paths (h/left key, owner-jump back,
	// etc.) where the search step hasn't already run.
	m.searchInput.Clear()

	// Reset scroll positions when navigating to a new level.
	ui.ActiveMiddleScroll = 0
	ui.ActiveLeftScroll = 0
	switch m.nav.Level {
	case model.LevelClusters:
		return m, nil

	case model.LevelResourceTypes:
		if m.unionMode && m.nav.Context != "" && m.nav.Context != UnionContextSentinel && isUnionDashboardMemberList(m.leftItems) {
			return m.navigateParentToUnionDashboardMembers()
		}
		if m.unionMode {
			if m.unionStartedFromPicker {
				return m.navigateParentFromPickerUnion()
			}
			return m, nil // no cluster selection level in CLI-started union mode
		}
		m.saveCursor()
		m.nav.Level = model.LevelClusters
		m.nav.Context = ""
		m.setMiddleItems(m.leftItems)
		m.popLeft()
		m.clearRight()
		m.restoreCursor()
		m.restoreLevelFilter()
		// The restored rows were captured on context entry and carry stale
		// [RO] markers; an in-context Ctrl+R toggle since then updated the
		// override but not this snapshot. Re-apply so the picker marker
		// matches the context's current read-only state.
		m.refreshContextReadOnlyMarkers()
		return m, m.loadPreview()

	case model.LevelResources:
		m.saveCursor()
		m.nav.Level = model.LevelResourceTypes
		m.nav.ResourceType = model.ResourceTypeEntry{}
		// When session restore puts us at LevelResources before API
		// discovery has completed, m.leftItems holds the seed set
		// (Pods/Deployments/...) as a fallback. Popping those into the
		// middle column on back-navigation makes the user see a "short
		// list" that then jumps to the full list when discovery arrives.
		// Instead, show the loader until apiResourceDiscoveryMsg
		// populates middleItems with the real CRD-inclusive set.
		// In union mode, discovery is stored under unionContexts[0], not the sentinel.
		discoveryCtx := m.nav.Context
		if m.isUnionSentinel() && len(m.unionContexts) > 0 {
			discoveryCtx = m.unionContexts[0]
		}
		if discovered, ok := m.discoveredResources[discoveryCtx]; ok && len(discovered) > 0 {
			m.setMiddleItems(m.leftItems)
		} else {
			m.setMiddleItems(nil)
			m.loading = true
		}
		m.popLeft()
		m.clearRight()
		m.restoreCursor()
		m.restoreLevelFilter()
		m.syncExpandedGroup()
		return m, m.loadPreview()

	case model.LevelOwned:
		m.saveCursor()
		// If we drilled into a nested owned level (e.g., ArgoCD → Deployment),
		// pop back to the parent owned level instead of jumping to LevelResources.
		if n := len(m.ownedParentStack); n > 0 {
			parent := m.ownedParentStack[n-1]
			m.ownedParentStack = m.ownedParentStack[:n-1]
			m.nav.ResourceType = parent.resourceType
			m.nav.ResourceName = parent.resourceName
			m.nav.Namespace = parent.namespace
			// Stay at LevelOwned — we're returning to the parent's owned view.
			if cached, ok := m.itemCache[m.navKey()]; ok {
				m.setMiddleItems(cached)
			} else {
				m.setMiddleItems(m.leftItems)
			}
			m.popLeft()
			m.clearRight()
			m.restoreCursor()
			m.restoreLevelFilter()
			return m, m.loadPreview()
		}
		m.nav.Level = model.LevelResources
		m.nav.ResourceName = ""
		if m.unionMode && !m.hasUnionDashboardMemberBreadcrumb() {
			m.nav.Context = UnionContextSentinel
		}
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.setMiddleItems(cached)
		} else {
			m.setMiddleItems(m.leftItems)
		}
		m.popLeft()
		m.clearRight()
		m.restoreCursor()
		m.restoreLevelFilter()
		return m, m.loadPreview()

	case model.LevelContainers:
		m.saveCursor()
		// If we came directly from Pods (skipping LevelOwned), go back to LevelResources.
		if m.nav.ResourceType.Kind == "Pod" {
			m.nav.Level = model.LevelResources
			m.nav.ResourceName = ""
			m.nav.OwnedName = ""
			if m.unionMode && !m.hasUnionDashboardMemberBreadcrumb() {
				m.nav.Context = UnionContextSentinel
			}
		} else {
			m.nav.Level = model.LevelOwned
			m.nav.OwnedName = ""
		}
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.setMiddleItems(cached)
		} else {
			m.setMiddleItems(m.leftItems)
		}
		m.popLeft()
		m.clearRight()
		m.restoreCursor()
		m.restoreLevelFilter()
		return m, m.loadPreview()
	}
	return m, nil
}

func (m Model) navigateToOwner(kind, name string) (tea.Model, tea.Cmd) {
	crds := m.discoveredResources[m.discoveryContext()]
	rt, ok := model.FindResourceTypeByKind(kind, crds)
	if !ok {
		m.setStatusMessage(fmt.Sprintf("Unknown resource type: %s", kind), true)
		return m, scheduleStatusClear()
	}

	// Record the origin so jump_back can return here after this teleport.
	m.pushJumpHistory()

	// Navigate back to resource types level.
	for m.nav.Level > model.LevelResourceTypes {
		ret, _ := m.navigateParent()
		m = ret.(Model)
	}

	// Find and select the target resource type in middle items.
	for i, item := range m.middleItems {
		if item.Extra == rt.ResourceRef() {
			m.setCursor(i)
			break
		}
	}

	// Set pending target to auto-select the owner resource after load.
	m.pendingTarget = name

	// Navigate into the resource type.
	return m.navigateChild()
}

func (m Model) navigateChild() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}

	m.cancelAndReset()
	m.requestGen++
	m.clearSelection()

	// Reset scroll positions when navigating to a new level.
	ui.ActiveMiddleScroll = 0
	ui.ActiveLeftScroll = 0

	// Remember this level's filter before clearing it, so navigating back
	// (navigateParent) restores the list exactly as the user left it.
	m.saveLevelFilter()

	// Clear filter when navigating into a child.
	m.filterText = ""
	m.filterInput.Clear()
	m.filterActive = false
	m.activeFilterPreset = nil
	m.unfilteredMiddleItems = nil

	// Clear search highlight on level change so it doesn't bleed onto
	// the child level — opening a resource is a "fresh start" for the
	// user (issue requested fix). The Esc cascade in handleExplorerEsc
	// already clears search as its own step before navigating parent,
	// but programmatic navigateChild/navigateParent paths previously
	// preserved searchInput.Value, leaving the highlight stuck.
	m.searchInput.Clear()

	switch m.nav.Level {
	case model.LevelClusters:
		return m.navigateChildCluster(sel)
	case model.LevelResourceTypes:
		return m.navigateChildResourceType(sel)
	case model.LevelResources:
		return m.navigateChildResource(sel)
	case model.LevelOwned:
		return m.navigateChildOwned(sel)
	case model.LevelContainers:
		return m, nil
	}
	return m, nil
}

func (m Model) navigateChildCluster(sel *model.Item) (tea.Model, tea.Cmd) {
	if isUnionSetItem(sel) {
		return m.navigateChildUnionSet(sel)
	}

	logger.Info("Context selected", "context", sel.Name)
	m.saveCursor()
	oldCtx := m.nav.Context
	m.nav.Context = sel.Name
	m.invalidateOrphanCacheForContext(oldCtx)
	m.recomputeReadOnly(sel.Name)
	m.dashboardPreview = ""
	m.dashboardEventsPreview = ""
	m.monitoringPreview = ""
	m.applyPinnedTypes()
	// Rebuild the security manager against the new cluster's clientsets so
	// findings, availability, and the SEC badge index reflect the active
	// cluster instead of lingering on the prior one.
	m.refreshSecuritySources()
	m.securityIndex = nil
	m.securityActiveGroup = ""
	m.nav.Level = model.LevelResourceTypes
	// Capture whatever the right-pane preview was already displaying for
	// this context (real discovery hit or seed fallback). We use this
	// below to avoid a blank loader if navigation beats hover-discovery
	// to the punch.
	previewItems := append([]model.Item(nil), m.rightItems...)
	m.pushLeft()
	m.clearRight()
	switch {
	case len(m.discoveredResources[sel.Name]) > 0:
		// Discovery already completed while hovering — pop straight into
		// the real list, no loader at all.
		m.setMiddleItems(model.BuildSidebarItems(m.discoveredResources[sel.Name]))
		m.itemCache[m.navKey()] = m.middleItems
		m.restoreCursor()
		m.syncExpandedGroup()
	case m.discoveringContexts[sel.Name] && len(previewItems) > 0:
		// Hover-discovery is in flight and the right pane already had
		// something to show (seed fallback). Reuse those items so the
		// user sees content immediately; apiResourceDiscoveryMsg will
		// replace them with the real list when discovery completes.
		m.setMiddleItems(previewItems)
		m.itemCache[m.navKey()] = m.middleItems
		m.restoreCursor()
		m.syncExpandedGroup()
		m.loading = true
	default:
		// No preview content and no in-flight discovery — show the
		// loader and kick off discovery below.
		m.setMiddleItems(nil)
		m.loading = true
	}
	m.setStatusMessage(fmt.Sprintf("Context: %s", sel.Name), false)
	m.saveCurrentSession()
	cmds := []tea.Cmd{m.loadPreview(), scheduleStatusClear()}
	// Re-probe security source availability for the new cluster. The
	// findings fetch chains off the probe result (handled by
	// updateSecurityAvailabilityLoaded).
	if cmd := m.loadSecurityAvailability(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	// Fire discovery once per session per context. The disk cache may have
	// prefilled m.discoveredResources, but stale-while-revalidate still
	// wants a live refresh on the user's first interaction with the
	// context. shouldFireDiscoveryFor handles both the prefilled-but-stale
	// case and the in-flight dedup. loadPreviewClusters typically already
	// fires this on hover, so navigation usually no-ops here.
	if m.shouldFireDiscoveryFor(sel.Name) {
		m.markDiscoveryStarted(sel.Name)
		cmds = append(cmds, m.discoverAPIResources(sel.Name))
	}
	if cmd := m.ensureNamespaceCacheFresh(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) navigateChildResourceType(sel *model.Item) (tea.Model, tea.Cmd) {
	if sel.Extra == "__overview__" || sel.Extra == "__monitoring__" {
		if m.isUnionSentinel() {
			mode, ok := unionDashboardModeFromExtra(sel.Extra)
			if !ok {
				return m, nil
			}
			m.saveCursor()
			m.nav.ResourceType = unionDashboardResourceType(mode)
			m.nav.Level = model.LevelResources
			m.dashboardPreview = ""
			m.dashboardEventsPreview = ""
			m.monitoringPreview = ""
			m.pushLeft()
			m.clearRight()
			m.setMiddleItems(unionDashboardMemberItems(m.unionContexts, m.unionContextColors, mode, m.namespace))
			m.setCursor(0)
			m.clampCursor()
			m.saveCurrentSession()
			return m, m.loadPreview()
		}
		m.fullscreenDashboard = true
		m.previewScroll = 0
		m = m.recomposeDashboard()
		m.setStatusMessage("Dashboard fullscreen ON", false)
		return m, scheduleStatusClear()
	}
	if sel.Kind == "__port_forwards__" {
		m.saveCursor()
		m.nav.ResourceType = model.ResourceTypeEntry{
			DisplayName: "Port Forwards",
			Kind:        "__port_forwards__",
			APIGroup:    "_portforward",
			APIVersion:  "v1",
			Resource:    "portforwards",
			Namespaced:  false,
		}
		m.nav.Level = model.LevelResources
		m.pushLeft()
		m.clearRight()
		m.setMiddleItems(m.portForwardItems())
		m.setCursor(0)
		m.clampCursor()
		m.saveCurrentSession()
		return m, m.waitForPortForwardUpdate()
	}
	if sel.Kind == "__captures__" {
		m.saveCursor()
		m.nav.ResourceType = model.ResourceTypeEntry{
			DisplayName: "Captures",
			Kind:        "__captures__",
			APIGroup:    "_capture",
			APIVersion:  "v1",
			Resource:    "captures",
			Namespaced:  false,
		}
		m.nav.Level = model.LevelResources
		m.pushLeft()
		m.clearRight()
		m.setMiddleItems(capturesPseudoItems(m.captureMgr))
		m.setCursor(0)
		m.clampCursor()
		m.saveCurrentSession()
		return m, m.waitForCaptureUpdate()
	}
	if sel.Kind == "__collapsed_group__" {
		m.expandedGroup = sel.Category
		visible := m.visibleMiddleItems()
		for i, item := range visible {
			if item.Category == sel.Category && item.Kind != "__collapsed_group__" {
				m.setCursor(i)
				break
			}
		}
		m.rightItems = nil
		m.previewYAML = ""
		m.loading = true
		return m, m.loadPreview()
	}
	if rt, ok := securityResourceTypeForItem(sel); ok {
		m.saveCursor()
		m.nav.ResourceType = rt
		m.nav.Level = model.LevelResources
		m.pushLeft()
		m.clearRight()
		m.saveCurrentSession()
		if cached, cacheHit := m.itemCache[m.navKey()]; cacheHit {
			m.setMiddleItems(cached)
			m.restoreCursor()
		} else {
			m.setMiddleItems(nil)
			m.setCursor(0)
		}
		m.loading = true
		return m, m.loadResources(false)
	}
	discoveryCtx := m.nav.Context
	if m.isUnionSentinel() && len(m.unionContexts) > 0 {
		discoveryCtx = m.unionContexts[0]
	}
	rt, ok := model.FindResourceTypeIn(sel.Extra, m.discoveredResources[discoveryCtx])
	if !ok {
		return m, nil
	}
	m.saveCursor()
	m.nav.ResourceType = rt
	m.applyResourceTypeSortDefault(m.nav.ResourceType, m.nav.Context)
	m.nav.Level = model.LevelResources
	m.pushLeft()
	m.clearRight()
	m.saveCurrentSession()
	// Show the cached list immediately while loadResources decides
	// whether to serve from cache (fresh-cache shortcut) or issue a real
	// fetch. The cache-then-refresh UX is unchanged; the refetch-suppression
	// now lives in loadResources, which compares the cache's freshness
	// fingerprint against the current fetch parameters.
	if cached, cacheHit := m.itemCache[m.navKey()]; cacheHit {
		m.setMiddleItems(cached)
		m.restoreCursor()
	} else {
		m.setMiddleItems(nil)
		m.setCursor(0)
	}
	m.loading = true
	return m, m.loadResources(false)
}

func (m Model) navigateChildResource(sel *model.Item) (tea.Model, tea.Cmd) {
	if isUnionDashboardResourceKind(m.nav.ResourceType.Kind) {
		return m.navigateChildUnionDashboardMember(sel)
	}
	if sel.Kind == "__security_finding_group__" {
		m.saveCursor()
		m.securityActiveGroup = sel.Extra
		m.nav.ResourceName = sel.Name
		m.nav.Level = model.LevelOwned
		m.pushLeft()
		m.clearRight()
		m.saveCurrentSession()
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.setMiddleItems(cached)
			m.restoreCursor()
		} else {
			m.setMiddleItems(nil)
			m.setCursor(0)
		}
		m.loading = true
		return m, m.loadSecurityAffectedResources(false)
	}
	if !m.resourceTypeHasChildren() && m.nav.ResourceType.Kind != "Pod" {
		return m, nil
	}
	m.saveCursor()
	m.nav.ResourceName = sel.Name
	if sel.Namespace != "" {
		m.nav.Namespace = sel.Namespace
	} else if !m.allNamespaces {
		m.nav.Namespace = m.namespace
	}
	if m.unionMode && sel.ClusterName != "" {
		m.nav.Context = sel.ClusterName
	}
	m.saveCurrentSession()
	if m.nav.ResourceType.Kind == "Pod" {
		m.nav.OwnedName = sel.Name
		m.nav.Level = model.LevelContainers
		m.pushLeft()
		m.clearRight()
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.setMiddleItems(cached)
			m.restoreCursor()
		} else {
			m.setMiddleItems(nil)
			m.setCursor(0)
		}
		m.loading = true
		return m, m.loadContainers(false)
	}
	m.nav.Level = model.LevelOwned
	m.pushLeft()
	m.clearRight()
	if cached, ok := m.itemCache[m.navKey()]; ok {
		m.setMiddleItems(cached)
		m.restoreCursor()
	} else {
		m.setMiddleItems(nil)
		m.setCursor(0)
	}
	m.loading = true
	return m, m.loadOwned(false)
}

func (m Model) navigateChildOwned(sel *model.Item) (tea.Model, tea.Cmd) {
	if sel.Kind == "__security_affected_resource__" {
		return m.jumpToFindingResource(sel)
	}
	if m.unionMode && sel.ClusterName != "" {
		m.nav.Context = sel.ClusterName
	}
	if sel.Kind == "Pod" {
		m.saveCursor()
		m.nav.OwnedName = sel.Name
		if sel.Namespace != "" {
			m.nav.Namespace = sel.Namespace
		}
		m.nav.Level = model.LevelContainers
		m.pushLeft()
		m.clearRight()
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.setMiddleItems(cached)
			m.restoreCursor()
		} else {
			m.setMiddleItems(nil)
			m.setCursor(0)
		}
		m.loading = true
		return m, m.loadContainers(false)
	}
	if kindHasOwnedChildren(sel.Kind) {
		m.saveCursor()
		m.ownedParentStack = append(m.ownedParentStack, ownedParentState{
			resourceType: m.nav.ResourceType,
			resourceName: m.nav.ResourceName,
			namespace:    m.nav.Namespace,
		})
		m.nav.ResourceType.Kind = sel.Kind
		m.nav.ResourceName = sel.Name
		if sel.Namespace != "" {
			m.nav.Namespace = sel.Namespace
		}
		m.pushLeft()
		m.clearRight()
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.setMiddleItems(cached)
			m.restoreCursor()
		} else {
			m.setMiddleItems(nil)
			m.setCursor(0)
		}
		m.loading = true
		return m, m.loadOwned(false)
	}
	return m, nil
}

// jumpToFindingResource navigates from a __security_affected_resource__ row
// at LevelOwned to the underlying real Kubernetes resource at LevelResources.
// The target identity is read from the synthetic __resource_key__ column
// (format: namespace/Kind/name) and resolved against the active context's
// discoveredResources. When the kind has not yet been discovered (e.g., a
// CRD whose API discovery is still pending), surfaces a status message and
// stays at LevelOwned rather than silently no-op'ing — callers expect the
// hint "[Enter] jump to resource" to do something visible.
func (m Model) jumpToFindingResource(sel *model.Item) (tea.Model, tea.Cmd) {
	parts := strings.SplitN(sel.ColumnValue("__resource_key__"), "/", 3)
	if len(parts) != 3 || parts[1] == "" || parts[2] == "" {
		return m, nil
	}
	namespace, kind, name := parts[0], parts[1], parts[2]
	rt, ok := model.FindResourceTypeByKind(kind, m.discoveredResources[m.discoveryContext()])
	if !ok {
		m.setStatusMessage("Cannot jump: "+kind+" not discovered in this context", true)
		return m, scheduleStatusClear()
	}
	m.saveCursor()
	// Drop the security finding-groups view from leftItems and restore the
	// resource-types sidebar (pushed onto history when the user entered the
	// security view from LevelResourceTypes). After this the Esc cascade
	// behaves as if the user came from LevelResourceTypes directly.
	m.popLeft()
	m.nav.ResourceType = rt
	m.nav.ResourceName = ""
	m.nav.Namespace = namespace
	m.nav.Level = model.LevelResources
	m.securityActiveGroup = ""
	m.clearRight()
	m.saveCurrentSession()
	if cached, cacheHit := m.itemCache[m.navKey()]; cacheHit {
		m.setMiddleItems(cached)
		placed := false
		for i, item := range cached {
			if item.Name == name && (item.Namespace == namespace || item.Namespace == "") {
				m.setCursor(i)
				placed = true
				break
			}
		}
		if !placed {
			m.setCursor(0)
		}
	} else {
		m.setMiddleItems(nil)
		m.setCursor(0)
	}
	m.loading = true
	return m, m.loadResources(false)
}

func (m Model) enterFullView() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}

	if m.nav.Level == model.LevelClusters || m.nav.Level == model.LevelResourceTypes {
		return m.navigateChild()
	}

	// Port forward and capture entries are virtual — no YAML to display.
	if m.nav.ResourceType.Kind == "__port_forwards__" || m.nav.ResourceType.Kind == "__captures__" {
		return m, nil
	}
	// Synthetic security items have no YAML to render in modeYAML, but
	// Enter still has a meaningful action: drill into a finding group's
	// affected resources at LevelResources, or jump to the underlying
	// real resource at LevelOwned. Both are wired through navigateChild
	// (navigateChildResource handles __security_finding_group__,
	// navigateChildOwned routes __security_affected_resource__ through
	// jumpToFindingResource). Falling through to loadYAML here used to
	// produce "Warning: unknown resource type"; an earlier fix returned
	// nil here which silently no-op'd Enter and stranded users on the
	// finding-groups list.
	if onSecurityView(&m) {
		return m.navigateChild()
	}

	m.mode = modeYAML
	m.yamlScroll = 0
	m.yamlContent = "Loading..."
	m.yamlSections = nil
	m.yamlVisualCurCol = yamlFoldPrefixLen
	return m, m.loadYAML()
}

// itemIndexFromDisplayLine maps a display line number to the actual item index,
// accounting for category headers and separators in the middle column.
func (m *Model) itemIndexFromDisplayLine(displayLine int) int {
	visible := m.visibleMiddleItems()
	line := 0
	lastCategory := ""
	for i, item := range visible {
		if item.Category != "" && item.Category != lastCategory {
			lastCategory = item.Category
			if i > 0 {
				line++ // separator
			}
			line++ // category header
		}
		if line == displayLine {
			return i
		}
		line++
	}
	return -1
}
