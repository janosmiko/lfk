package app

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/k8s"
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

func (m Model) navigateParent() (tea.Model, tea.Cmd) {
	m.wheel.dead = true // left/right nav empties the wheel momentum queue (#524)
	m.cancelAndReset()
	m.requestGen++
	m.reclaimStaleBgWork()
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

// resolveOwnerResourceType finds the resource type an owner reference points
// at. apiVersion ("group/version", or "v1" for the core group) disambiguates
// the Kind: two CRDs can share one Kind across groups, and the Kind-only
// lookup returns whichever group discovery listed first — which sent jumps to
// the wrong CRD entirely (issue #562). Falls back to the Kind-only lookup when
// the caller has no apiVersion, or names a group this cluster never
// discovered, so a best-effort jump still beats a dead end.
func resolveOwnerResourceType(kind, apiVersion string, discovered []model.ResourceTypeEntry) (model.ResourceTypeEntry, bool) {
	if apiVersion != "" {
		group, _, _ := strings.Cut(apiVersion, "/")
		if !strings.Contains(apiVersion, "/") {
			group = "" // bare "v1" is the core group
		}
		if rt, ok := model.FindResourceTypeByKindAndGroup(kind, group, discovered); ok {
			return rt, true
		}
	}
	return model.FindResourceTypeByKind(kind, discovered)
}

// navigateToOwner teleports to the owning resource. apiVersion may be empty
// for callers that only know a built-in Kind ("Pod", "Node"); see
// resolveOwnerResourceType.
func (m Model) navigateToOwner(kind, name, apiVersion string) (tea.Model, tea.Cmd) {
	crds := m.discoveredResources[m.discoveryContext()]
	rt, ok := resolveOwnerResourceType(kind, apiVersion, crds)
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

	m.wheel.dead = true // left/right nav empties the wheel momentum queue (#524)
	m.cancelAndReset()
	m.requestGen++
	m.reclaimStaleBgWork()
	m.clearSelection()

	// Reset scroll positions when navigating to a new level.
	ui.ActiveMiddleScroll = 0
	ui.ActiveLeftScroll = 0

	// Remember this level's filter, then clear all live filter/search state so
	// the child level is a fresh start; navigating back (navigateParent)
	// restores the list exactly as the user left it.
	m.resetFilterForTypeSwitch()

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
	// Cancel any running live-log preview stream before switching clusters so
	// the old stream's goroutine does not outlive its context. Clear the flag
	// too — the new cluster starts at the resource-type level with no pod
	// selected, so the preview has nothing to show.
	// Also clear the per-pod buffer cache: entries are context-specific and
	// must not be restored after a context switch.
	m.cancelPreviewLogStream()
	m.fullLogPreview = false
	m.previewLogCache = make(map[string]previewLogCacheEntry)
	m.previewLogCacheOrder = nil
	oldCtx := m.nav.Context
	m.nav.Context = sel.Name
	m.invalidateOrphanCacheForContext(oldCtx)
	m.recomputeReadOnly(sel.Name)
	m.dashboardPreview = ""
	m.dashboardEventsPreview = ""
	m.monitoringPreview = ""
	m.applyPinnedTypes()
	// Clear the prior cluster's badge index BEFORE refreshSecuritySources:
	// that rebuild seeds m.securityIndex from the new cluster's disk-cached
	// findings (stale-while-revalidate), so nil-ing after would discard it.
	m.securityIndex = nil
	m.securityActiveGroup = ""
	m.securityActiveSource = ""
	m.securityResourceFilter = nil
	// Drop the prior cluster's preview caches: keyed by ctx/ns/name they are
	// dead weight for the new context, and the secret cache holds decoded
	// plaintext we shouldn't keep resident after leaving the cluster.
	m.secretPreviewCache = make(map[string]*model.SecretData)
	m.serviceEndpointsCache = make(map[string]*k8s.ServiceEndpoints)
	securitySeedCmd := m.refreshSecuritySources()
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
	if securitySeedCmd != nil {
		cmds = append(cmds, securitySeedCmd)
	}
	// Security availability is probed lazily on first focus of the Security
	// category (maybeProbeSecurityOnFocus), not eagerly on context switch, so
	// switching clusters never triggers the aws credential plugin for a user
	// who isn't looking at security. refreshSecuritySources above cleared the
	// per-context probe guard.
	//
	// Eager findings scan: when this cluster was inspected before,
	// refreshSecuritySources seeded availability (and the manager hint) from
	// the disk cache, so we can populate the SEC badge index now without any
	// probe. No cache (first-ever visit) -> this no-ops and badges stay lazy.
	if cmd := m.maybeEagerSecurityScan(); cmd != nil {
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
		// The per-resource findings view mixes sources, so the drilled
		// group's source must travel with the group key — it cannot be
		// recovered from nav.ResourceType.Kind there.
		m.securityActiveSource = sel.ColumnValue("__source__")
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
		// loadSecurityAffectedResources legitimately returns nil (manager
		// torn down, no group key); arming the spinner without a command
		// would strand it forever.
		if cmd := m.loadSecurityAffectedResources(false); cmd != nil {
			m.loading = true
			return m, cmd
		}
		return m, m.loadPreview()
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
	// Record the finding's affected-resources view as the jump origin so
	// JumpBack returns here after this teleport. Captured before popLeft and
	// the nav mutations below so the snapshot reflects the finding view.
	m.pushJumpHistory()
	// Drop the security finding-groups view from leftItems and restore the
	// resource-types sidebar (pushed onto history when the user entered the
	// security view from LevelResourceTypes). After this the Esc cascade
	// behaves as if the user came from LevelResourceTypes directly.
	m.popLeft()
	// The finding view's quick filter must not carry into the real resource
	// list it teleports to (TASK-839 class).
	m.resetFilterForTypeSwitch()
	m.nav.ResourceType = rt
	m.nav.ResourceName = ""
	m.nav.Namespace = namespace
	m.nav.Level = model.LevelResources
	m.securityActiveGroup = ""
	m.securityActiveSource = ""
	m.securityResourceFilter = nil
	// Arm the post-load auto-select: on a cold cache the cursor below stays
	// at 0 and only the resources-loaded handler can place it on the target.
	// Armed on the cache-hit path too, so the refresh load re-selects the
	// same row instead of drifting.
	m.pendingTarget = name
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
	m.yamlReturnMode = modeExplorer
	m.yamlView.scroll = 0
	m.yamlView.content = "Loading..."
	m.yamlView.sections = nil
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	m.yamlView.resetBlame()
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
