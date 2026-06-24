package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// pendingSessionListState holds the list filter and cursor target a restored
// session must reapply once its resource list reloads. It is only armed when
// the descent to the resource list is deferred behind CRD discovery; the
// immediate path applies the same values directly.
type pendingSessionListState struct {
	armed       bool
	filter      string
	filterBroad bool
	cursorName  string
	cursorNs    string
}

// sessionCursor resolves the row a restored session should land on. A drilled-in
// ResourceName takes precedence and matches on name alone (its namespace was
// never persisted); otherwise the captured cursor name/namespace is used.
func sessionCursor(sess *SessionState) (name, ns string) {
	if sess.ResourceName != "" {
		return sess.ResourceName, ""
	}
	return sess.CursorName, sess.CursorNamespace
}

// applySessionFilterAndCursor seeds the list filter and cursor target so they
// take effect when the resource list finishes loading. The filter is also
// committed to filterMemory so navigating away and back keeps it.
func (m *Model) applySessionFilterAndCursor(filter string, broad bool, cursorName, cursorNs string) {
	m.filterText = filter
	m.filterInput.Set(filter)
	m.filterBroadMode = broad
	m.filterActive = false
	m.searchActive = false
	m.saveLevelFilter()
	if cursorName != "" {
		m.pendingTarget = cursorName
		m.pendingTargetNamespace = cursorNs
	}
}

// applyPendingSessionList applies a deferred session list state (armed when a
// CRD discovery roundtrip was needed) and clears it. No-op when nothing armed.
func (m *Model) applyPendingSessionList() {
	p := m.pendingSessionList
	m.pendingSessionList = pendingSessionListState{}
	if !p.armed {
		return
	}
	m.applySessionFilterAndCursor(p.filter, p.filterBroad, p.cursorName, p.cursorNs)
}

func (m Model) restoreSession(contexts []model.Item) (tea.Model, tea.Cmd) {
	sess := m.pendingSession
	m.pendingSession = nil
	m.sessionRestored = true

	var restoredModel tea.Model
	var cmd tea.Cmd
	if len(sess.Tabs) > 0 {
		restoredModel, cmd = m.restoreMultiTabSession(sess, contexts)
	} else {
		restoredModel, cmd = m.restoreSingleTabSession(sess, contexts)
	}

	// In union mode the session restore navigates using unionContexts[0] for
	// API discovery, but nav.Context must be the UnionContextSentinel so that
	// loadResources fans out to all union contexts when at LevelResources.
	if m.unionMode {
		if tm, ok := restoredModel.(Model); ok {
			tm.nav.Context = UnionContextSentinel
			for i := range tm.tabs {
				tm.tabs[i].nav.Context = UnionContextSentinel
			}
			return tm, cmd
		}
	}
	return restoredModel, cmd
}

func (m Model) restoreSingleTabSession(sess *SessionState, contexts []model.Item) (tea.Model, tea.Cmd) {
	if !contextInList(sess.Context, contexts) {
		return m, m.loadPreview()
	}

	discoveryCtx := sess.Context
	navCtx := sess.Context
	if m.unionMode {
		navCtx = UnionContextSentinel
	}

	for i, ctx := range contexts {
		if ctx.Name == sess.Context {
			m.cursorMemory[""] = i
			break
		}
	}

	m.nav.Context = navCtx
	if m.unionMode {
		m.readOnly = m.cliReadOnly
	} else {
		m.recomputeReadOnly(discoveryCtx)
	}
	m.applyPinnedTypes()
	m.nav.Level = model.LevelResourceTypes
	// Rebuild the security manager for the restored context. NewModel
	// called refreshSecuritySources with m.nav.Context = "", which fell
	// back to the kubeconfig default and registered sources for that
	// context. If the session lands on a different context, the manager
	// targets the wrong cluster and the initial probe's result message
	// is dropped by updateSecurityAvailabilityLoaded's stale-context
	// gate — leaving the sidebar's "(probing sources...)" loader stuck
	// until the user navigates out and back in. Re-run refreshSecuritySources
	// here so the manager + cached availability + dispatched probe all
	// target sess.Context.
	securitySeedCmd := m.refreshSecuritySources()

	m.leftItemsHistory = nil
	m.leftItems = contexts

	if discovered, ok := m.discoveredResources[discoveryCtx]; ok && len(discovered) > 0 {
		m.setMiddleItems(model.BuildSidebarItems(discovered))
	} else {
		m.setMiddleItems(model.BuildSidebarItems(model.SeedResources()))
	}
	m.itemCache[m.navKey()] = m.middleItems
	m.clearRight()

	applySessionNamespaces(&m, sess.AllNamespaces, sess.Namespace, sess.SelectedNamespaces, sess.NsSelectionNegated)

	var cmds []tea.Cmd
	if securitySeedCmd != nil {
		cmds = append(cmds, securitySeedCmd)
	}
	needsDiscovery := m.shouldFireDiscoveryFor(discoveryCtx)
	if needsDiscovery {
		m.markDiscoveryStarted(discoveryCtx)
		cmds = append(cmds, m.discoverAPIResources(discoveryCtx))
	}
	if cmd := m.ensureNamespaceCacheFresh(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	// Security availability is probed lazily on first focus of the Security
	// category (maybeProbeSecurityOnFocus), not eagerly on session restore.
	// refreshSecuritySources above reseeds the sidebar from the on-disk cache
	// and clears the per-context probe guard for sess.Context. When that cache
	// already knows the sources, kick off a no-probe findings scan so the SEC
	// badges populate on the restored cluster without a trip to Security.
	if cmd := m.maybeEagerSecurityScan(); cmd != nil {
		cmds = append(cmds, cmd)
	}

	if sess.ResourceType != "" {
		rt, ok := resolveSessionResourceType(sess.ResourceType, m.discoveredResources[discoveryCtx])
		if !ok && needsDiscovery {
			m.sessionResourceTypeAwaitingDiscovery = sess.ResourceType
			m.sessionResourceNameAwaitingDiscovery = sess.ResourceName
			cursorName, cursorNs := sessionCursor(sess)
			m.pendingSessionList = pendingSessionListState{
				armed:       true,
				filter:      sess.Filter,
				filterBroad: sess.FilterBroad,
				cursorName:  cursorName,
				cursorNs:    cursorNs,
			}
		}
		if ok {
			rtRef := rt.ResourceRef()
			for i, item := range m.middleItems {
				if item.Extra == rtRef {
					m.cursorMemory[m.nav.Context] = i
					break
				}
			}

			m.leftItemsHistory = [][]model.Item{contexts}
			m.leftItems = m.middleItems
			m.nav.ResourceType = rt
			m.applyResourceTypeSortDefault(m.nav.ResourceType, m.nav.Context)
			m.nav.Level = model.LevelResources
			m.setMiddleItems(nil)
			m.clearRight()
			m.setCursor(0)
			m.loading = true

			cursorName, cursorNs := sessionCursor(sess)
			m.applySessionFilterAndCursor(sess.Filter, sess.FilterBroad, cursorName, cursorNs)

			cmds = append(cmds, m.loadResources(false))
			return m, tea.Batch(cmds...)
		}
	}

	if needsDiscovery {
		m.setMiddleItems(nil)
		m.loading = true
	}
	m.clampCursor()
	cmds = append(cmds, m.loadPreview())
	return m, tea.Batch(cmds...)
}

func (m Model) restoreMultiTabSession(sess *SessionState, contexts []model.Item) (tea.Model, tea.Cmd) {
	activeIdx := sess.ActiveTab
	if activeIdx < 0 || activeIdx >= len(sess.Tabs) {
		activeIdx = 0
	}

	activeSess := sess.Tabs[activeIdx]
	if !contextInList(activeSess.Context, contexts) {
		return m, m.loadPreview()
	}

	tabs := make([]TabState, 0, len(sess.Tabs))
	for i, st := range sess.Tabs {
		tab := buildSessionTabState(&st, m.discoveredResources[st.Context])
		if i != activeIdx {
			tab.needsLoad = true
		}
		tabs = append(tabs, tab)
	}

	m.tabs = tabs
	m.activeTab = activeIdx

	return m.restoreSingleTabSession(&SessionState{
		Context:            activeSess.Context,
		Namespace:          activeSess.Namespace,
		AllNamespaces:      activeSess.AllNamespaces,
		SelectedNamespaces: activeSess.SelectedNamespaces,
		NsSelectionNegated: activeSess.NsSelectionNegated,
		ResourceType:       activeSess.ResourceType,
		ResourceName:       activeSess.ResourceName,
		Filter:             activeSess.Filter,
		FilterBroad:        activeSess.FilterBroad,
		CursorName:         activeSess.CursorName,
		CursorNamespace:    activeSess.CursorNamespace,
	}, contexts)
}

func buildSessionTabState(st *SessionTab, discovered []model.ResourceTypeEntry) TabState {
	tab := TabState{
		nav: model.NavigationState{
			Context: st.Context,
		},
		splitPreview:      ui.ConfigSplitPreview,
		watchMode:         ui.ConfigWatchMode,
		warningEventsOnly: ui.ConfigEventsWarningsOnly,
		eventGrouping:     ui.ConfigEventsGrouping,
		allGroupsExpanded: true,
		cursorMemory:      make(map[string]int),
		filterMemory:      make(map[string]savedFilter),
		// Seed from disk so a session-restored tab keeps the sort the user
		// chose before quitting (issue #353); a blank map here would override
		// the model's loaded memory when this tab is activated.
		sortMemory:      loadSortMemory(),
		itemCache:       make(map[string][]model.Item),
		selectedItems:   make(map[string]bool),
		selectionAnchor: -1,
	}

	if st.AllNamespaces {
		tab.allNamespaces = true
	} else if st.Namespace != "" {
		tab.namespace = st.Namespace
		if len(st.SelectedNamespaces) > 0 {
			tab.selectedNamespaces = make(map[string]bool, len(st.SelectedNamespaces))
			for _, ns := range st.SelectedNamespaces {
				tab.selectedNamespaces[ns] = true
			}
		} else {
			tab.selectedNamespaces = map[string]bool{st.Namespace: true}
		}
		tab.nsSelectionNegated = st.NsSelectionNegated
	} else {
		tab.allNamespaces = true
	}

	if st.ResourceType != "" {
		rt, ok := resolveSessionResourceType(st.ResourceType, discovered)
		if ok {
			tab.nav.ResourceType = rt
			tab.nav.Level = model.LevelResources
			if st.ResourceName != "" {
				tab.nav.ResourceName = st.ResourceName
			}
			// Carry the saved list filter so switching to this lazily-loaded
			// tab reopens it filtered, matching the active tab's restore.
			tab.filterText = st.Filter
			tab.filterBroadMode = st.FilterBroad
		} else {
			tab.nav.Level = model.LevelResourceTypes
		}
	} else if st.Context != "" {
		tab.nav.Level = model.LevelResourceTypes
	} else {
		tab.nav.Level = model.LevelClusters
	}

	return tab
}

func resolveSessionResourceType(ref string, discovered []model.ResourceTypeEntry) (model.ResourceTypeEntry, bool) {
	if rt, ok := model.FindResourceTypeIn(ref, discovered); ok {
		return rt, true
	}
	return model.FindResourceTypeIn(ref, model.SeedResources())
}

func contextInList(ctx string, items []model.Item) bool {
	for _, item := range items {
		if item.Name == ctx {
			return true
		}
	}
	return false
}

func applySessionNamespaces(m *Model, allNS bool, ns string, selectedNS []string, negated bool) {
	if allNS {
		m.allNamespaces = true
		m.selectedNamespaces = nil
		m.nsSelectionNegated = false
	} else if ns != "" {
		m.namespace = ns
		m.allNamespaces = false
		if len(selectedNS) > 0 {
			m.selectedNamespaces = make(map[string]bool, len(selectedNS))
			for _, n := range selectedNS {
				m.selectedNamespaces[n] = true
			}
		} else {
			m.selectedNamespaces = map[string]bool{ns: true}
		}
		m.nsSelectionNegated = negated
	} else {
		// Session omitted the namespace (older or partially-populated file):
		// reset to all-namespaces rather than inheriting the prior tab's
		// namespace filter, which would silently scope the restored context.
		m.namespace = ""
		m.allNamespaces = true
		m.selectedNamespaces = nil
		m.nsSelectionNegated = false
	}
}
