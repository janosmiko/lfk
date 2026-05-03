package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
)

func (m Model) restoreSession(contexts []model.Item) (tea.Model, tea.Cmd) {
	sess := m.pendingSession
	m.pendingSession = nil
	m.sessionRestored = true

	if len(sess.Tabs) > 0 {
		return m.restoreMultiTabSession(sess, contexts)
	}

	return m.restoreSingleTabSession(sess, contexts)
}

func (m Model) restoreSingleTabSession(sess *SessionState, contexts []model.Item) (tea.Model, tea.Cmd) {
	if !contextInList(sess.Context, contexts) {
		return m, m.loadPreview()
	}

	for i, ctx := range contexts {
		if ctx.Name == sess.Context {
			m.cursorMemory[""] = i
			break
		}
	}

	m.nav.Context = sess.Context
	m.recomputeReadOnly(sess.Context)
	m.applyPinnedGroups()
	m.nav.Level = model.LevelResourceTypes

	m.leftItemsHistory = nil
	m.leftItems = contexts

	if discovered, ok := m.discoveredResources[sess.Context]; ok && len(discovered) > 0 {
		m.setMiddleItems(model.BuildSidebarItems(discovered))
	} else {
		m.setMiddleItems(model.BuildSidebarItems(model.SeedResources()))
	}
	m.itemCache[m.navKey()] = m.middleItems
	m.clearRight()

	applySessionNamespaces(&m, sess.AllNamespaces, sess.Namespace, sess.SelectedNamespaces)

	var cmds []tea.Cmd
	needsDiscovery := m.shouldFireDiscoveryFor(sess.Context)
	if needsDiscovery {
		m.markDiscoveryStarted(sess.Context)
		cmds = append(cmds, m.discoverAPIResources(sess.Context))
	}
	if cmd := m.ensureNamespaceCacheFresh(); cmd != nil {
		cmds = append(cmds, cmd)
	}

	if sess.ResourceType != "" {
		rt, ok := resolveSessionResourceType(sess.ResourceType, m.discoveredResources[sess.Context])
		if !ok && needsDiscovery {
			m.sessionResourceTypeAwaitingDiscovery = sess.ResourceType
			m.sessionResourceNameAwaitingDiscovery = sess.ResourceName
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
			m.nav.Level = model.LevelResources
			m.setMiddleItems(nil)
			m.clearRight()
			m.setCursor(0)
			m.loading = true

			if sess.ResourceName != "" {
				m.pendingTarget = sess.ResourceName
			}

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
		ResourceType:       activeSess.ResourceType,
		ResourceName:       activeSess.ResourceName,
	}, contexts)
}

func buildSessionTabState(st *SessionTab, discovered []model.ResourceTypeEntry) TabState {
	tab := TabState{
		nav: model.NavigationState{
			Context: st.Context,
		},
		splitPreview:      true,
		watchMode:         true,
		warningEventsOnly: true,
		eventGrouping:     true,
		allGroupsExpanded: true,
		cursorMemory:      make(map[string]int),
		itemCache:         make(map[string][]model.Item),
		selectedItems:     make(map[string]bool),
		selectionAnchor:   -1,
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

func applySessionNamespaces(m *Model, allNS bool, ns string, selectedNS []string) {
	if allNS {
		m.allNamespaces = true
		m.selectedNamespaces = nil
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
	}
}
