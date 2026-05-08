package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

func (m Model) updateContextsLoaded(msg contextsLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.setErrorFromErr("Warning: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.err = nil
	// Annotate each context row with its effective read-only state. CLI
	// flag wins, then per-context session override (set by Ctrl+R on a
	// row), then per-context/global config. Re-applying overrides here
	// ensures Ctrl+R toggles survive a context list refresh.
	for i := range msg.items {
		msg.items[i].IsContext = true
		msg.items[i].ReadOnly = m.effectiveContextReadOnly(msg.items[i].Name)
		msg.items[i].ClusterColor = m.clusterColors[msg.items[i].Name]
		// Stamp LocalClusterStatus from the on-Model cache so the picker
		// row renderer can paint the running / stopped icon on rows that
		// belong to a known local-cluster provider. Rows without an entry
		// in the cache leave LocalClusterStatus empty so the renderer
		// skips the icon for managed contexts.
		if e, ok := m.localClusterCache[msg.items[i].Name]; ok {
			msg.items[i].LocalClusterStatus = e.Status
		}
	}
	m.setMiddleItems(msg.items)
	m.itemCache[m.navKey()] = m.middleItems
	m.leftItems = nil
	m.clearRight()
	m.clampCursor()

	// Restore saved port forwards in the background.
	var pfCmds []tea.Cmd
	if m.pendingPortForwards != nil && len(m.pendingPortForwards.PortForwards) > 0 {
		pfCmds = m.restorePortForwards()
		m.pendingPortForwards = nil
	}

	// Restore session: navigate to the saved context/namespace/resource type.
	if m.pendingSession != nil && !m.sessionRestored {
		mdl, cmd := m.restoreSession(msg.items)
		if len(pfCmds) > 0 {
			pfCmds = append(pfCmds, cmd)
			return mdl, tea.Batch(pfCmds...)
		}
		return mdl, cmd
	}

	cmds := make([]tea.Cmd, 0, 1+len(pfCmds))
	cmds = append(cmds, m.loadPreview())
	cmds = append(cmds, pfCmds...)
	return m, tea.Batch(cmds...)
}

func (m Model) updateResourceTypes(msg resourceTypesMsg) (tea.Model, tea.Cmd) {
	m.err = nil
	if m.nav.Level == model.LevelClusters {
		// Right-pane preview at the cluster list: always update so the user
		// sees *something* (seeds or real) while hovering a context.
		m.rightItems = msg.items
		m.loading = false
		return m, nil
	}
	// Middle pane at LevelResourceTypes: if discovery is still in flight
	// and only seeds are available, don't clobber the loader with seeds
	// — that would cause a one-tick flash of basic resource types every
	// watch interval. Real discovery results (seeded=false) or an
	// explicit seed fallback from updateAPIResourceDiscovery (which
	// writes middleItems directly) take precedence via their own paths.
	if msg.seeded && m.loading {
		return m, nil
	}
	m.loading = false
	m.setMiddleItems(msg.items)
	m.itemCache[m.navKey()] = m.middleItems
	m.clampCursor()
	return m, m.loadPreview()
}

func (m Model) updateAPIResourceDiscovery(msg apiResourceDiscoveryMsg) (Model, tea.Cmd) {
	// Clear the in-flight flag for this context regardless of outcome so
	// the user can retry (or hover again) without getting stuck on a
	// permanently-deduplicated call.
	delete(m.discoveringContexts, msg.context)
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		// API resource discovery failed (permissions, etc.) -- fall back to
		// seed resources so the user can still navigate.
		logger.Info("API resource discovery failed", "context", msg.context, "error", msg.err.Error())
		if m.nav.Context == msg.context && m.loading {
			// Mirror the success branch's wasInitial guard. m.loading alone
			// is unreliable as an "is this the initial discovery" signal
			// because invalidatePreviewForCursorChange flips it true on
			// every j/k. Without this guard, a discovery retry that lands
			// while the user is mid-scroll calls restoreCursor and snaps
			// the cursor back to cursorMemory[ctx] (e.g., the resource type
			// saved by session-restore), undoing the user's navigation.
			wasInitial := len(m.middleItems) == 0
			m.loading = false
			m.setMiddleItems(model.BuildSidebarItems(model.SeedResources()))
			m.itemCache[m.navKey()] = m.middleItems
			if wasInitial {
				m.restoreCursor()
			} else {
				m.clampCursor()
			}
			m.syncExpandedGroup()
		}
		// On discovery failure, drop any queued bookmark so we don't loop
		// retrying. The user can re-open the overlay and try again.
		if m.bookmarkAwaitingDiscovery != nil {
			m.bookmarkAwaitingDiscovery = nil
			m.setStatusMessage("Resource type not found in current cluster", true)
			return m, scheduleStatusClear()
		}
		return m, nil
	}
	// Prepend LFK pseudo-resources (helm releases, port forwards) so they
	// resolve via FindResourceType* and appear in the sidebar uniformly
	// with real discovered resources.
	entries := append(model.PseudoResources(), msg.entries...)
	m.discoveredResources[msg.context] = entries
	if m.discoveryRefreshedContexts == nil {
		m.discoveryRefreshedContexts = make(map[string]bool)
	}
	m.discoveryRefreshedContexts[msg.context] = true
	// Persist the cluster-reported entries (without pseudo-resources, which
	// are runtime-only) into the per-host file under ~/.kube/cache/discovery
	// so the next launch can prefill discoveredResources from disk and so
	// `kubectl api-resources --invalidate-cache` wipes lfk's snapshot too.
	// Best-effort: a write failure leaves the in-memory state authoritative
	// for this session.
	if err := updateDiscoveryCacheForContext(m.client, msg.context, msg.entries); err != nil {
		logger.Warn("Could not persist discovery cache", "context", msg.context, "error", err)
	}
	merged := model.BuildSidebarItems(entries)
	// If the user is at LevelClusters and hovering this context, refresh
	// the right-pane preview so the discovered list replaces the seed
	// fallback that was emitted synchronously when loadPreviewClusters ran.
	if m.nav.Level == model.LevelClusters {
		if sel := m.selectedMiddleItem(); sel != nil && sel.Name == msg.context {
			m.rightItems = merged
		}
	}
	if m.nav.Context == msg.context {
		// Update the item cache for the resource types level.
		rtCacheKey := msg.context
		m.itemCache[rtCacheKey] = merged
		if m.nav.Level == model.LevelResourceTypes {
			// User is on resource types level: update the visible list.
			//
			// Distinguish initial discovery (no list yet) from periodic
			// re-discovery (watch tick / shift+r): the initial path
			// honors cursorMemory so context-switch / session-resume land
			// on the user's previous position, while subsequent refreshes
			// preserve the live cursor via clampCursor. m.loading is NOT
			// a reliable signal because invalidatePreviewForCursorChange
			// flips it true on every cursor move at this level — using
			// it would reset the cursor every watch interval after any
			// j/k press.
			wasInitial := len(m.middleItems) == 0
			m.loading = false
			m.setMiddleItems(merged)
			if wasInitial {
				m.restoreCursor()
			} else {
				m.clampCursor()
			}
			m.syncExpandedGroup()
		} else if m.nav.Level != model.LevelClusters {
			// User is deeper: update leftItems so back-navigation shows CRDs.
			m.leftItems = merged
			// Update cursor memory for the resource types level so
			// navigating back lands on the correct resource type.
			if m.nav.ResourceType.Resource != "" {
				rtRef := m.nav.ResourceType.ResourceRef()
				for i, item := range merged {
					if item.Extra == rtRef {
						m.cursorMemory[msg.context] = i
						break
					}
				}
			}
		}
	}
	// Replay a bookmark that was queued waiting on this context's discovery.
	// IsContextAware switches the bookmark's effective context; for context-free
	// bookmarks the effective context is the model's current context, which we
	// match against msg.context so we only replay when the right discovery lands.
	if m.bookmarkAwaitingDiscovery != nil {
		bm := *m.bookmarkAwaitingDiscovery
		effective := bm.Context
		if !bm.IsContextAware() {
			effective = m.nav.Context
		}
		if effective == msg.context {
			m.bookmarkAwaitingDiscovery = nil
			result, cmd := m.navigateToBookmark(bm)
			return result.(Model), cmd
		}
	}
	// Resume a deferred session restore that was holding for this context's
	// CRD discovery. Without this, quitting on an ArgoCD Application view
	// and re-opening lfk drops the user at the resource types level instead
	// of the saved view.
	if m.sessionResourceTypeAwaitingDiscovery != "" && msg.context == m.nav.Context {
		ref := m.sessionResourceTypeAwaitingDiscovery
		name := m.sessionResourceNameAwaitingDiscovery
		m.sessionResourceTypeAwaitingDiscovery = ""
		m.sessionResourceNameAwaitingDiscovery = ""
		if rt, ok := model.FindResourceTypeIn(ref, entries); ok {
			rtRef := rt.ResourceRef()
			for i, item := range merged {
				if item.Extra == rtRef {
					m.cursorMemory[m.nav.Context] = i
					break
				}
			}
			m.leftItemsHistory = append(m.leftItemsHistory[:0:0], m.leftItemsHistory...)
			if len(m.leftItemsHistory) == 0 {
				contexts, _ := m.client.GetContexts()
				m.leftItemsHistory = [][]model.Item{contexts}
			}
			m.leftItems = merged
			m.nav.ResourceType = rt
			m.nav.Level = model.LevelResources
			m.setMiddleItems(nil)
			m.clearRight()
			m.setCursor(0)
			m.loading = true
			if name != "" {
				m.pendingTarget = name
			}
			return m, m.loadResources(false)
		}
	}
	return m, nil
}

func (m Model) updateResourcesLoaded(msg resourcesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.requestGen {
		return m, nil // stale response, discard
	}
	m.loading = false
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.previewLoading = false
		m.setErrorFromErr("Warning: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.err = nil
	if msg.forPreview {
		return m.updateResourcesLoadedPreview(msg)
	}
	return m.updateResourcesLoadedMain(msg)
}

func (m Model) updateResourcesLoadedPreview(msg resourcesLoadedMsg) (tea.Model, tea.Cmd) {
	m.previewLoading = false
	// Filter by selected namespaces when multi-select is active.
	if len(m.selectedNamespaces) > 1 {
		filtered := make([]model.Item, 0, len(msg.items))
		for _, item := range msg.items {
			if item.Namespace == "" || m.selectedNamespaces[item.Namespace] {
				filtered = append(filtered, item)
			}
		}
		msg.items = filtered
	}
	// Prime itemCache under the drill-in navKey so loadResources can serve
	// the list instantly and skip a redundant fetch when the user drills
	// in or re-hovers this rt later. Only do this when msg.rt carries a
	// real resource — synthetic previews (port-forwards, dashboards) have
	// a zero-valued rt and must not write an empty-resource key. The
	// fingerprint records the fetch-affecting state so the shortcut can
	// detect later invalidations (namespace switch, allNS toggle,
	// multi-select update) without relying on requestGen, which
	// navigateChild bumps before child handlers even run.
	if msg.rt.Resource != "" {
		drillInKey := m.nav.Context + "/" + msg.rt.Resource
		m.itemCache[drillInKey] = msg.items
		m.cacheFingerprints[drillInKey] = m.fetchFingerprint()
	}
	m.rightItems = msg.items
	// Filter events in children view to warnings-only when enabled.
	if m.warningEventsOnly && len(m.rightItems) > 0 && m.rightItems[0].Kind == "Event" {
		filtered := make([]model.Item, 0, len(m.rightItems))
		for _, item := range m.rightItems {
			if item.Status == "Warning" {
				filtered = append(filtered, item)
			}
		}
		m.rightItems = filtered
	}
	// Collapse duplicate events so noisy pods don't drown out the preview.
	// The preview pane is always a summary, so we follow the main list's
	// grouping toggle without offering a separate control — toggling `z` in
	// the Events view also affects the preview shown for other resources.
	if m.eventGrouping && len(m.rightItems) > 0 && m.rightItems[0].Kind == "Event" {
		m.rightItems = groupEvents(m.rightItems)
	}
	if len(m.rightItems) == 0 {
		logger.Info("No child resources found", "resourceType", m.nav.ResourceType.Kind, "resource", m.nav.ResourceName)
	}
	return m, nil
}

func (m Model) updateResourcesLoadedMain(msg resourcesLoadedMsg) (tea.Model, tea.Cmd) {
	// Filter by selected namespaces when multi-select is active.
	if len(m.selectedNamespaces) > 1 {
		filtered := make([]model.Item, 0, len(msg.items))
		for _, item := range msg.items {
			if item.Namespace == "" || m.selectedNamespaces[item.Namespace] {
				filtered = append(filtered, item)
			}
		}
		msg.items = filtered
	}
	if len(msg.items) == 0 {
		logger.Info("No resources found", "resourceType", m.nav.ResourceType.Kind, "namespace", m.namespace)
	}
	prevName, prevNs, prevExtra, prevKind := m.cursorItemKey()

	kind := m.nav.ResourceType.Kind
	if (kind == "Pod" || kind == "Node") && len(m.middleItems) > 0 {
		m.carryOverMetricsColumns(msg.items)
	}
	if kind == "Service" && len(m.middleItems) > 0 {
		// Same anti-flash carry-over: keeps the lazily-fetched
		// "Backing Endpoints" + per-endpoint "Endpoints" rollup
		// columns visible across watch-tick refreshes, so the right
		// pane doesn't blank between setMiddleItems and the next
		// loadPreviewServiceEndpoints message landing.
		m.carryOverServiceEndpointColumns(msg.items)
	}
	m.setMiddleItems(msg.items)
	mainCacheKey := m.navKey()
	m.itemCache[mainCacheKey] = m.middleItems
	// Record the cache-freshness fingerprint so a subsequent load for the
	// same resource (drill-in from the sidebar, preview on navigate-out-
	// then-hover, or a hover-cycle between sibling rts) can serve from
	// cache instead of refetching. Only record for actual resource lists;
	// __port_forwards__ is synthetic (sourced from the in-process manager)
	// and doesn't go through GetResources.
	if m.nav.ResourceType.Resource != "" && m.nav.ResourceType.Kind != "__port_forwards__" && m.nav.ResourceType.Kind != "__captures__" {
		m.cacheFingerprints[mainCacheKey] = m.fetchFingerprint()
	}
	// Always sort: the k8s layer uses a non-stable single-key sort that
	// shuffles ties between refreshes (e.g. Helm releases with the same
	// name in different namespaces). Running sortMiddleItems guarantees
	// the app-level tiebreaker chain is applied on every load — even the
	// default Name/ascending case — so watch-mode output is deterministic.
	m.sortMiddleItems()
	m.applyWarningEventsFilter()
	m.applyEventGrouping()
	m.reapplyFilterPreset()
	if m.pendingTarget != "" {
		for i, item := range m.middleItems {
			if item.Name == m.pendingTarget {
				m.setCursor(i)
				break
			}
		}
		m.pendingTarget = ""
	} else {
		m.restoreCursorToItem(prevName, prevNs, prevExtra, prevKind)
	}
	// If this load originated from a watch-mode refresh, propagate the
	// suppress flag to the downstream preview/metrics cmds so they too
	// stay off the title-bar indicator. Capture the prior flag so the
	// returned model resets it cleanly for subsequent user Updates.
	savedSuppress := m.suppressBgtasks
	if msg.silent {
		m.suppressBgtasks = true
	}
	var cmds []tea.Cmd
	// Align previewLoading with whether a preview fetch is actually in
	// flight. clearRight() / invalidatePreviewForCursorChange() armed the
	// flag to true on navigation so the right pane keeps showing the
	// spinner across the main-list arrival; if no preview cmd is
	// dispatched (e.g., empty namespace, so selectedMiddleItem is nil and
	// loadPreview returns nil), leaving the flag armed would render
	// "Loading..." forever instead of letting the right pane fall through
	// to its empty/details branches.
	previewCmd := m.loadPreview()
	m.previewLoading = previewCmd != nil
	if previewCmd != nil {
		cmds = append(cmds, previewCmd)
	}
	switch kind {
	case "Pod":
		cmds = append(cmds, m.loadPodMetricsForList())
	case "Node":
		cmds = append(cmds, m.loadNodeMetricsForList())
	}
	m.suppressBgtasks = savedSuppress
	return m, tea.Batch(cmds...)
}

func (m *Model) applyWarningEventsFilter() {
	if m.warningEventsOnly && m.nav.ResourceType.Kind == "Event" {
		var filtered []model.Item
		for _, item := range m.middleItems {
			if item.Status == "Warning" {
				filtered = append(filtered, item)
			}
		}
		m.setMiddleItems(filtered)
	}
}

// applyEventGrouping collapses duplicate Events sharing Type/Reason/Message/Object
// into a single row with a summed Count. Runs only when viewing the Event
// resource list with grouping enabled; other resource kinds pass through untouched.
func (m *Model) applyEventGrouping() {
	if !m.eventGrouping || m.nav.ResourceType.Kind != "Event" {
		return
	}
	m.setMiddleItems(groupEvents(m.middleItems))
}

// rebuildEventsFromCache re-derives the visible Event list from the raw cache
// after an Events-view toggle (warnings-only, grouping). It re-applies the
// full pipeline — warning filter, grouping, and the active filter preset —
// so toggling any one of them never silently drops the others. A cache miss
// leaves m.middleItems untouched; the next resource load will rebuild it.
func (m *Model) rebuildEventsFromCache() {
	cached, ok := m.itemCache[m.navKey()]
	if !ok {
		return
	}
	m.setMiddleItems(append([]model.Item(nil), cached...))
	m.applyWarningEventsFilter()
	m.applyEventGrouping()
	m.reapplyFilterPreset()
	m.clampCursor()
}

func (m *Model) reapplyFilterPreset() {
	if m.activeFilterPreset != nil {
		m.unfilteredMiddleItems = append([]model.Item(nil), m.middleItems...)
		var filtered []model.Item
		for _, item := range m.middleItems {
			if m.activeFilterPreset.MatchFn(item) {
				filtered = append(filtered, item)
			}
		}
		m.setMiddleItems(filtered)
	}
}

func (m Model) updateOwnedLoaded(msg ownedLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.requestGen {
		return m, nil // stale response, discard
	}
	m.loading = false
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.previewLoading = false
		m.setErrorFromErr("Warning: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.err = nil
	// Filter by selected namespaces when multi-select is active.
	if len(m.selectedNamespaces) > 1 {
		filtered := make([]model.Item, 0, len(msg.items))
		for _, item := range msg.items {
			if m.selectedNamespaces[item.Namespace] {
				filtered = append(filtered, item)
			}
		}
		msg.items = filtered
	}
	if msg.forPreview {
		m.previewLoading = false
		m.rightItems = msg.items
		return m, nil
	}
	prevName, prevNs, prevExtra, prevKind := m.cursorItemKey()
	m.setMiddleItems(msg.items)
	m.itemCache[m.navKey()] = m.middleItems
	// Sort with the app-level tiebreaker on every load (see
	// updateResourcesLoadedMain for rationale): the k8s layer returns
	// items in a non-deterministic order for equal keys, so the
	// tiebreaker chain must run here too or owned-resource refreshes
	// will flicker.
	m.sortMiddleItems()
	// Re-apply active filter preset on owned refresh (same as resourcesLoadedMsg).
	if m.activeFilterPreset != nil {
		m.unfilteredMiddleItems = append([]model.Item(nil), m.middleItems...)
		var filtered []model.Item
		for _, item := range m.middleItems {
			if m.activeFilterPreset.MatchFn(item) {
				filtered = append(filtered, item)
			}
		}
		m.setMiddleItems(filtered)
		m.itemCache[m.navKey()] = m.middleItems
	}
	m.restoreCursorToItem(prevName, prevNs, prevExtra, prevKind)
	// Propagate the silent flag to the downstream preview cmd.
	savedSuppress := m.suppressBgtasks
	if msg.silent {
		m.suppressBgtasks = true
	}
	// Align previewLoading with whether a preview fetch is actually in
	// flight (see updateResourcesLoadedMain for rationale). At LevelOwned,
	// kinds without further owned children (or empty Helm releases) make
	// loadPreview return nil; without this the right pane spins forever.
	previewCmd := m.loadPreview()
	m.previewLoading = previewCmd != nil
	m.suppressBgtasks = savedSuppress
	return m, previewCmd
}

func (m Model) updateResourceTreeLoaded(msg resourceTreeLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.requestGen {
		return m, nil
	}
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.setErrorFromErr("Resource tree: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.resourceTree = msg.tree
	return m, nil
}

func (m Model) updateContainersLoaded(msg containersLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.requestGen {
		return m, nil // stale response, discard
	}
	m.loading = false
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.previewLoading = false
		m.setErrorFromErr("Warning: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.err = nil
	if msg.forPreview {
		m.previewLoading = false
		m.rightItems = msg.items
		return m, nil
	}
	m.setMiddleItems(msg.items)
	m.itemCache[m.navKey()] = m.middleItems
	// Sort with the app-level tiebreaker on every container-list load
	// (see updateResourcesLoadedMain for rationale): container rows use
	// the parent pod's namespace and only differ by Name/Kind, so the
	// tiebreaker still provides a stable order across refreshes.
	m.sortMiddleItems()
	m.clampCursor()
	// Propagate the silent flag to the downstream preview cmd.
	savedSuppress := m.suppressBgtasks
	if msg.silent {
		m.suppressBgtasks = true
	}
	// Align previewLoading with whether a preview fetch is actually in
	// flight. clearRight() armed the flag to true on navigation; at
	// LevelContainers loadPreview returns nil, so leaving it armed would
	// make the right pane render "Loading..." forever. Conversely, when a
	// preview cmd is dispatched the flag must stay true so the spinner
	// keeps showing until the reply clears it.
	previewCmd := m.loadPreview()
	m.previewLoading = previewCmd != nil
	m.suppressBgtasks = savedSuppress
	return m, previewCmd
}

func (m Model) updateNamespacesLoaded(msg namespacesLoadedMsg) (tea.Model, tea.Cmd) {
	// Only clear the global loading flag for overlay-triggered loads.
	// Background cache refreshes (session restore, context open) must not
	// touch it — it belongs to the middle-column/resource-types load and
	// clearing it asynchronously while API discovery is still in flight
	// produces a "No items" flash between the loader and the populated
	// list.
	if !msg.silent {
		m.loading = false
	}
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.overlay = overlayNone
		m.setErrorFromErr("Warning: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.err = nil
	// Cache namespace items + names for command-bar autocompletion and
	// for synchronous overlay seeding on subsequent opens. Keyed by the
	// context the fetch was issued for so tabs / `:ctx` switching the
	// nav.Context between request and reply doesn't leak stale results.
	// fetchedAt stamps the entry so callers can decide whether to use
	// it as-is or trigger a background refresh.
	if m.cachedNamespaces == nil {
		m.cachedNamespaces = make(map[string]namespaceCacheEntry)
	}
	names := make([]string, 0, len(msg.items))
	for _, item := range msg.items {
		names = append(names, item.Name)
	}
	m.cachedNamespaces[msg.context] = namespaceCacheEntry{
		items:     msg.items,
		names:     names,
		fetchedAt: time.Now(),
	}
	// Silent loads are background cache refreshes (stale-while-revalidate
	// after an overlay open, session restore, `:ctx` switch). They must
	// not mutate overlayItems / overlayCursor: the user may be navigating
	// the open namespace overlay right now, and overwriting the items
	// would yank the cursor off whatever they're hovering. The next open
	// will pick up the freshly cached entry.
	if msg.silent {
		return m, nil
	}
	m.overlayItems = buildNamespaceOverlayItems(msg.items)
	m.overlayCursor = namespaceOverlayCursor(m.overlayItems, m.namespace, m.allNamespaces)
	return m, nil
}

// buildNamespaceOverlayItems prepends the synthetic "All Namespaces" header
// to a fetched namespace list so the same shape is produced whether items
// come from a fresh API call or from cachedNamespaces.
func buildNamespaceOverlayItems(items []model.Item) []model.Item {
	allNsItem := model.Item{Name: "All Namespaces", Status: "all"}
	out := make([]model.Item, 0, len(items)+1)
	out = append(out, allNsItem)
	out = append(out, items...)
	return out
}

// namespaceOverlayCursor returns the row index the overlay cursor should
// land on when first opened: the "All Namespaces" header when the user is
// in all-ns mode, otherwise the row matching the active namespace name.
// Falls back to 0 when no match is found, which keeps the cursor on the
// "All Namespaces" header instead of leaving it at -1.
func namespaceOverlayCursor(items []model.Item, currentNs string, allNamespaces bool) int {
	if allNamespaces {
		return 0
	}
	for i, item := range items {
		if item.Name == currentNs {
			return i
		}
	}
	return 0
}
