package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) updateContextsLoaded(msg contextsLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.reloaded {
		// The kubeconfig was re-read, so a context name may now resolve to
		// another cluster or another user. Drop the verdicts filed under
		// those names instead of hiding actions the new identity may hold.
		m.perms.clear()
	}
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		// Without a context list there is nothing to restore into. Show the
		// error rather than a splash that never ends.
		m.abandonSessionRestore()
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
		msg.items[i].Category = contextCategory
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
	msg.items = m.withUnionSetRows(msg.items)
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

	// No session to restore (or it already ran): the cluster picker is ready.
	m.finishSessionRestore()
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
		// This list IS the preview clearRight() armed the flag for. Leaving it
		// armed keeps spinnerNeeded() true for the life of the process, and the
		// 10 FPS spinner loop then re-renders the whole screen forever (#646).
		m.previewLoading = false
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
	// Propagate the watch-tick silent flag into the preview cascade so a
	// freshly-invalidated cache (refreshCurrentLevel at LevelResourceTypes
	// drops the preview fingerprint to surface cluster-side mutations)
	// doesn't flash the title-bar indicator every 2s. Matches the pattern
	// in updateResourcesLoadedMain.
	savedSuppress := m.suppressBgtasks
	if msg.silent {
		m.suppressBgtasks = true
	}
	cmd := m.loadPreview()
	// Same alignment as updateResourcesLoadedMain: the flag may only stay armed
	// while a preview result is still coming. Without this a cursor sitting on
	// a type that dispatches nothing pins the 10 FPS spinner loop on (#646).
	m.previewLoading = cmd != nil && !msg.silent
	m.suppressBgtasks = savedSuppress
	return m, cmd
}

func (m Model) updateResourcesLoaded(msg resourcesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.requestGen {
		return m, nil // stale response, discard
	}
	m.loading = false
	// The list the restore was waiting for is here (or failed): either way the
	// explorer now has as much as it is going to get, so let it paint.
	if !msg.forPreview {
		m.finishSessionRestore()
	}
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.previewLoading = false
		m.setErrorFromErr("Warning: ", msg.err)
		if len(msg.items) == 0 {
			return m, scheduleStatusClear()
		}
	} else {
		m.err = nil
	}
	partialErr := msg.err
	var mdl tea.Model
	var cmd tea.Cmd
	if msg.forPreview {
		mdl = m.updateResourcesLoadedPreview(msg)
	} else {
		mdl, cmd = m.updateResourcesLoadedMain(msg)
	}
	if partialErr != nil {
		return mdl, tea.Batch(cmd, scheduleStatusClear())
	}
	return mdl, cmd
}

// restoreCursorAfterLoad places the cursor once a list has loaded: a pending
// target (matched against the visible list, namespace-disambiguated) wins, else
// the prior row, preferring the exact cluster in union mode.
func (m *Model) restoreCursorAfterLoad(prevName, prevNs, prevExtra, prevKind, prevCluster string) {
	if m.pendingTarget != "" {
		target, targetNs := m.pendingTarget, m.pendingTargetNamespace
		m.pendingTarget = ""
		m.pendingTargetNamespace = ""
		for i, item := range m.visibleMiddleItems() {
			if item.Name == target && (targetNs == "" || item.Namespace == targetNs) {
				m.setCursor(i)
				return
			}
		}
		// Target gone (deleted or hidden by the restored filter): fall back to
		// the prior row instead of a stale pre-load index.
		m.restoreCursorToItem(prevName, prevNs, prevExtra, prevKind)
		return
	}
	if m.unionMode && prevCluster != "" {
		for i, item := range m.visibleMiddleItems() {
			if item.Name == prevName && item.Namespace == prevNs && item.ClusterName == prevCluster {
				m.setCursor(i)
				return
			}
		}
	}
	m.restoreCursorToItem(prevName, prevNs, prevExtra, prevKind)
}

func (m Model) updateResourcesLoadedMain(msg resourcesLoadedMsg) (tea.Model, tea.Cmd) {
	msg.items = m.filterLoadedItemsBySelectedNamespaces(msg.items)
	if len(msg.items) == 0 {
		logger.Info("No resources found", "resourceType", m.nav.ResourceType.Kind, "namespace", m.namespace)
	}
	prevName, prevNs, prevExtra, prevKind := m.cursorItemKey()
	// In union mode, items with the same name exist in multiple clusters.
	// Capture the source cluster before items are replaced so we can prefer
	// the exact cluster when restoring the cursor (otherwise it always jumps
	// to the first alphabetical cluster's entry).
	prevCluster := ""
	if m.unionMode {
		if visible := m.visibleMiddleItems(); m.cursor() < len(visible) {
			prevCluster = visible[m.cursor()].ClusterName
		}
		// Stamp the per-row cluster color from the union-set's per-cluster
		// color map so the table renderer can paint a 1-cell color tile
		// per row. Items whose source cluster has no configured color in
		// the union_sets entry get an empty string and the renderer
		// reserves a blank cell for alignment. Sourced from the per-set
		// map rather than the global clusterColors so users can pick
		// deliberate "traffic light" semantics per view without
		// disturbing the cluster picker's global tints.
		for i := range msg.items {
			if cn := msg.items[i].ClusterName; cn != "" {
				msg.items[i].ClusterColor = m.unionContextColors[cn]
			}
		}
	}

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
	if view, ok := ui.ResolveView(ui.ResourceRef{
		Group:    m.nav.ResourceType.APIGroup,
		Version:  m.nav.ResourceType.APIVersion,
		Resource: m.nav.ResourceType.Resource,
		Kind:     m.nav.ResourceType.Kind,
	}, m.nav.Context); ok {
		applyViewColumns(msg.items, view)
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
	m.restoreCursorAfterLoad(prevName, prevNs, prevExtra, prevKind, prevCluster)
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
	// spinner across the main-list arrival. If no preview cmd is
	// dispatched (e.g., empty namespace, so selectedMiddleItem is nil and
	// loadPreview returns nil), leaving the flag armed would render
	// "Loading..." forever instead of letting the right pane fall through
	// to its empty/details branches.
	previewCmd := m.loadPreview()
	m.previewLoading = previewCmd != nil && !msg.silent
	if previewCmd != nil {
		cmds = append(cmds, previewCmd)
	}
	cmds = append(cmds, m.listMetricsCmds(kind)...)
	// Review this kind's verbs for the namespace once, off the key path, so
	// the action menu can drop entries the cluster would refuse. Answers nil
	// for a kind with no verb map.
	cmds = append(cmds, m.loadActionPermissions(kind))
	m.suppressBgtasks = savedSuppress
	m.syncObjectExplorerLive()
	return m, tea.Batch(cmds...)
}

func (m Model) filterLoadedItemsBySelectedNamespaces(items []model.Item) []model.Item {
	if (!m.nsSelectionNegated && len(m.selectedNamespaces) <= 1) || (m.nsSelectionNegated && len(m.selectedNamespaces) == 0) {
		return items
	}
	filtered := make([]model.Item, 0, len(items))
	for _, item := range items {
		if item.Namespace == "" || m.selectedNamespaces[item.Namespace] != m.nsSelectionNegated {
			filtered = append(filtered, item)
		}
	}
	return filtered
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

// applyEventGrouping collapses duplicate Events sharing ClusterName/Type/
// Reason/Message/Object into a single row with a summed Count. Runs only when
// viewing the Event resource list with grouping enabled. Other resource kinds
// pass through untouched.
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
// leaves m.middleItems untouched. The next resource load will rebuild it.
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
	msg.items = m.filterLoadedItemsBySelectedNamespaces(msg.items)
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
	// loadPreview return nil. Without this the right pane spins forever.
	previewCmd := m.loadPreview()
	m.previewLoading = previewCmd != nil && !msg.silent
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
	if msg.err != nil {
		m.clearPreviewContentFingerprint()
		m.previewLoading = false
		if isContextCanceled(msg.err) {
			return m, nil
		}
		m.err = msg.err
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
	// flight. clearRight() armed the flag to true on navigation. At
	// LevelContainers loadPreview returns nil, so leaving it armed would
	// make the right pane render "Loading..." forever. Conversely, when a
	// preview cmd is dispatched the flag must stay true so the spinner
	// keeps showing until the reply clears it.
	previewCmd := m.loadPreview()
	m.previewLoading = previewCmd != nil && !msg.silent
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
	m.overlayItems = m.namespaceSelectorItems(msg.items)
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
