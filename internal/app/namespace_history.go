package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/model"
)

// nsScope is a snapshot of the full namespace selection, so a jump can restore
// it as one unit. It holds the four fields that together decide the effective
// scope: a single namespace, all-namespaces mode, the multi-select set, and
// whether that set is an exclude list.
type nsScope struct {
	namespace          string
	allNamespaces      bool
	selectedNamespaces map[string]bool
	nsSelectionNegated bool
}

// captureNamespaceScope snapshots the current namespace selection.
func (m Model) captureNamespaceScope() nsScope {
	return nsScope{
		namespace:          m.namespace,
		allNamespaces:      m.allNamespaces,
		selectedNamespaces: copyMapStringBool(m.selectedNamespaces),
		nsSelectionNegated: m.nsSelectionNegated,
	}
}

// equal reports whether two scopes select the same namespaces.
func (s nsScope) equal(o nsScope) bool {
	if s.namespace != o.namespace || s.allNamespaces != o.allNamespaces || s.nsSelectionNegated != o.nsSelectionNegated {
		return false
	}
	if len(s.selectedNamespaces) != len(o.selectedNamespaces) {
		return false
	}
	for k := range s.selectedNamespaces {
		if !o.selectedNamespaces[k] {
			return false
		}
	}
	return true
}

// clone deep-copies a scope so per-tab snapshots never alias the live map.
func (s *nsScope) clone() *nsScope {
	if s == nil {
		return nil
	}
	c := *s
	c.selectedNamespaces = copyMapStringBool(s.selectedNamespaces)
	return &c
}

// applyNamespaceScope restores a snapshot onto the model.
func (m *Model) applyNamespaceScope(s nsScope) {
	m.namespace = s.namespace
	m.allNamespaces = s.allNamespaces
	m.selectedNamespaces = copyMapStringBool(s.selectedNamespaces)
	m.nsSelectionNegated = s.nsSelectionNegated
}

// namespaceOverlayEntryScope returns the scope captured when the namespace
// overlay opened. It falls back to the current scope if no snapshot exists
// (e.g. a commit path that never went through openNamespaceSelectorForContext),
// so recordPreviousNamespace still gets a sane baseline.
func (m Model) namespaceOverlayEntryScope() nsScope {
	if m.nsOverlayEntryScope != nil {
		return *m.nsOverlayEntryScope
	}
	return m.captureNamespaceScope()
}

// recordPreviousNamespace stores before as the previous scope, but only when
// the selection actually changed, so a later jump-to-previous returns to the
// last distinct scope rather than a no-op re-selection.
func (m *Model) recordPreviousNamespace(before nsScope) {
	if before.equal(m.captureNamespaceScope()) {
		return
	}
	m.previousNsScope = before.clone()
}

// jumpToPreviousNamespace swaps the current namespace scope with the previously
// recorded one. The swap means repeated presses toggle between the two scopes,
// like a vim alternate buffer. Reports an error when there is no previous scope
// or the current level/mode forbids changing the namespace.
func (m Model) jumpToPreviousNamespace() (tea.Model, tea.Cmd) {
	if m.nav.Level == model.LevelClusters {
		m.setStatusMessage("Namespace selection requires a selected context", true)
		return m, scheduleStatusClear()
	}
	if m.unionMode {
		m.setStatusMessage("Union mode supports exactly one namespace", true)
		return m, scheduleStatusClear()
	}
	if m.previousNsScope == nil {
		m.setStatusMessage("No previous namespace to jump to", true)
		return m, scheduleStatusClear()
	}
	oldNs := m.namespace
	current := m.captureNamespaceScope()
	m.applyNamespaceScope(*m.previousNsScope)
	m.previousNsScope = current.clone()
	// The all-namespaces stash belongs to the scope we just left; drop it so a
	// later A-toggle doesn't restore a selection that no longer applies.
	m.savedSelectedNamespaces = nil
	m.savedNsSelectionNegated = false
	m.nsSelectionModified = false
	m.invalidateOrphanCacheForNamespace(m.nav.Context, oldNs)
	m.saveCurrentSession()
	m.cancelAndReset()
	m.requestGen++
	m.setStatusMessage("Namespace: "+m.buildNsLabelText(), false)
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}
