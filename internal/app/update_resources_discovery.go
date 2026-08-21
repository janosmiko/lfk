package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

func (m Model) updateAPIResourceDiscovery(msg apiResourceDiscoveryMsg) (Model, tea.Cmd) {
	// Clear the in-flight flag for this context regardless of outcome so
	// the user can retry (or hover again) without getting stuck on a
	// permanently-deduplicated call.
	delete(m.discoveringContexts, msg.context)
	if isContextCanceled(msg.err) {
		return m, nil
	}
	// In union mode, discovery runs against unionContexts[0] but nav.Context
	// is the UnionContextSentinel. Treat them as equivalent for sidebar and
	// loading state.
	isCurrentContext := m.nav.Context == msg.context ||
		(m.isUnionSentinel() && len(m.unionContexts) > 0 && msg.context == m.unionContexts[0])
	if msg.err != nil {
		return m.handleAPIResourceDiscoveryError(msg, isCurrentContext)
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
	if isCurrentContext {
		// Update the item cache for the resource types level. In union mode
		// use the sentinel as the cache key so navKey() lookups at
		// LevelResourceTypes find the sidebar items correctly.
		rtCacheKey := m.nav.Context
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
						m.cursorMemory[m.nav.Context] = i
						break
					}
				}
			}
		}
	}
	if mdl, cmd, handled := m.replayBookmarkAfterDiscovery(msg.context); handled {
		return mdl, cmd
	} else {
		m = mdl
	}
	if mdl, cmd, handled := m.resumeDeferredSessionRestore(msg.context, entries, merged); handled {
		return mdl, cmd
	} else {
		m = mdl
	}
	// Discovery landed without a list load to wait on, so the resource-type
	// sidebar is the finished restore. Let the explorer paint.
	m.finishSessionRestoreForContext(isCurrentContext)
	return m, nil
}

// replayBookmarkAfterDiscovery replays a bookmark queued waiting on this
// context's discovery. Context-free bookmarks match against the model's
// current context rather than IsContextAware's bookmark context.
func (m Model) replayBookmarkAfterDiscovery(msgContext string) (Model, tea.Cmd, bool) {
	if m.bookmarkAwaitingDiscovery != nil {
		bm := *m.bookmarkAwaitingDiscovery
		effective := bm.Context
		if !bm.IsContextAware() {
			effective = m.nav.Context
		}
		if effective == msgContext {
			m.bookmarkAwaitingDiscovery = nil
			result, cmd := m.navigateToBookmark(bm)
			return result.(Model), cmd, true
		}
	}
	return m, nil, false
}

// resumeDeferredSessionRestore resumes a deferred session restore that was
// holding for this context's CRD discovery, so quitting on an ArgoCD
// Application view and reopening lfk lands back on the saved view.
func (m Model) resumeDeferredSessionRestore(msgContext string, entries []model.ResourceTypeEntry, merged []model.Item) (Model, tea.Cmd, bool) {
	awaitingContext := m.nav.Context
	if m.isUnionSentinel() && len(m.unionContexts) > 0 {
		awaitingContext = m.unionContexts[0]
	}
	if m.sessionResourceTypeAwaitingDiscovery != "" && msgContext == awaitingContext {
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
				m.leftItemsHistory = [][]model.Item{m.contextsOrEmpty()}
			}
			m.leftItems = merged
			m.nav.ResourceType = rt
			m.applyResourceTypeSortDefault(m.nav.ResourceType, m.nav.Context)
			m.nav.Level = model.LevelResources
			m.setMiddleItems(nil)
			m.clearRight()
			m.setCursor(0)
			m.loading = true
			// Land on the deferred ResourceName, then apply the filter/cursor
			// that rode through discovery on pendingSessionList.
			if name != "" {
				m.pendingTarget = name
			}
			m.applyPendingSessionList()
			return m, m.loadResources(false), true
		}
		// Discovery answered and this cluster has no such type. The parked
		// filter and cursor have nowhere to land.
		m.dropDeferredSessionRestore()
	}
	return m, nil, false
}

func (m Model) handleAPIResourceDiscoveryError(msg apiResourceDiscoveryMsg, isCurrentContext bool) (Model, tea.Cmd) {
	// API resource discovery failed (permissions, etc.) -- fall back to
	// seed resources so the user can still navigate.
	logger.Info("API resource discovery failed", "context", msg.context, "error", msg.err.Error())
	// Nothing further is coming for this context. Drop the parked restore too:
	// left armed, it would resume against whichever cluster the user opens next.
	if isCurrentContext {
		m.dropDeferredSessionRestore()
	}
	m.finishSessionRestoreForContext(isCurrentContext)
	if isCurrentContext && m.loading {
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

// updateDiscoveryCacheLoaded merges per-host discovery snapshots produced by
// the async discoveryCachePreloadCmd dispatched from Init. The live
// apiResourceDiscoveryMsg path is authoritative: any context already marked
// in discoveryRefreshedContexts has a fresher view from the apiserver, so
// the stale-while-revalidate seed must not overwrite it. Contexts that
// haven't been refreshed yet — i.e. the user hasn't navigated into them —
// are populated so the sidebar can paint a CRD-aware list as soon as they
// hover the context.
func (m Model) updateDiscoveryCacheLoaded(msg discoveryCacheLoadedMsg) Model {
	if msg.cached == nil {
		return m
	}
	pseudo := model.PseudoResources()
	for ctx, entries := range msg.cached {
		if m.discoveryRefreshedContexts[ctx] {
			continue
		}
		merged := make([]model.ResourceTypeEntry, 0, len(pseudo)+len(entries))
		merged = append(merged, pseudo...)
		merged = append(merged, entries...)
		m.discoveredResources[ctx] = merged
	}
	// If the user is sitting at LevelResourceTypes for a context that just
	// got hydrated, swap the seed sidebar for the cached entries. Without
	// this, the sidebar would only update on the next user-initiated
	// navigation. Read the cache key the same way navKey() builds it for
	// LevelResourceTypes so we don't drift if that format changes.
	if m.nav.Level == model.LevelResourceTypes {
		discoveryCtx := m.nav.Context
		if m.unionMode && m.nav.Context == UnionContextSentinel && len(m.unionContexts) > 0 {
			discoveryCtx = m.unionContexts[0]
		}
		if entries, ok := m.discoveredResources[discoveryCtx]; ok && len(entries) > 0 {
			merged := model.BuildSidebarItems(entries)
			m.itemCache[m.nav.Context] = merged
			m.setMiddleItems(merged)
			m.syncExpandedGroup()
		}
	}
	return m
}
