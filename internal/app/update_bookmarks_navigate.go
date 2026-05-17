package app

import (
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// applyBookmarkContextSwitch updates the navigation context, union-mode
// state, and per-context read-only state from a resolved bookmark target.
// Extracted from navigateToBookmark to keep that function under the
// gocyclo/length caps.
func (m *Model) applyBookmarkContextSwitch(target bookmarkTarget) {
	oldCtx := m.nav.Context
	m.nav.Context = target.context
	if oldCtx != target.context {
		m.invalidateOrphanCacheForContext(oldCtx)
	}
	switch target.kind {
	case bookmarkTargetUnionSet:
		m.unionMode = true
		m.unionStartedFromPicker = target.unionStartedFromRow
		m.unionSetName = target.unionSetName
		m.unionContexts = append([]string(nil), target.unionContexts...)
		m.unionContextColors = copyMapStringString(target.unionContextColors)
		m.allNamespaces = false
		m.namespace = target.unionNamespace
		m.selectedNamespaces = map[string]bool{target.unionNamespace: true}
		m.readOnly = m.cliReadOnly
	case bookmarkTargetContext:
		m.unionMode = false
		m.unionStartedFromPicker = false
		m.unionSetName = ""
		m.unionContexts = nil
		m.unionContextColors = nil
		m.recomputeReadOnly(target.context)
	default:
		if target.context == UnionContextSentinel {
			m.readOnly = m.cliReadOnly
		} else {
			m.recomputeReadOnly(target.context)
		}
	}
}

// applyBookmarkNamespace replays the bookmark's saved namespace selection
// only when the user explicitly requested it via Tab in the overlay.
// Consumes m.bookmarkLoadNamespace immediately so the flag can't leak
// into the next overlay open. Skips namespace replay for union-set
// targets — those carry their own namespace in the target record.
func (m *Model) applyBookmarkNamespace(bm model.Bookmark, target bookmarkTarget, oldCtx string) {
	applyNs := m.bookmarkLoadNamespace
	m.bookmarkLoadNamespace = false
	if target.kind == bookmarkTargetUnionSet {
		return
	}
	if !applyNs {
		return
	}
	oldNs := m.namespace
	switch {
	case bm.Namespace == "" && len(bm.Namespaces) == 0:
		m.allNamespaces = true
		m.selectedNamespaces = nil
	case len(bm.Namespaces) > 1:
		m.allNamespaces = false
		m.namespace = bm.Namespaces[0]
		m.selectedNamespaces = make(map[string]bool, len(bm.Namespaces))
		for _, ns := range bm.Namespaces {
			m.selectedNamespaces[ns] = true
		}
	default:
		m.allNamespaces = false
		ns := bm.Namespace
		if len(bm.Namespaces) == 1 {
			ns = bm.Namespaces[0]
		}
		m.namespace = ns
		m.selectedNamespaces = map[string]bool{ns: true}
	}
	// Invalidate the old namespace's cache within the new context. When the
	// context also changed, the entire old context was already wiped by
	// applyBookmarkContextSwitch, so only invalidate when staying put.
	if oldCtx == target.context {
		m.invalidateOrphanCacheForNamespace(target.context, oldNs)
	}
}

// rebuildLeftHistoryForBookmark rebuilds the leftItemsHistory + leftItems
// columns after a bookmark navigation. Returns the resource-types slice
// for the caller's downstream cursor restoration. Logs (but does not
// fail) a GetContexts error so the picker still falls back gracefully.
func (m *Model) rebuildLeftHistoryForBookmark(target bookmarkTarget) []model.Item {
	contexts, err := m.client.GetContexts()
	if err != nil {
		logger.Warn("GetContexts failed during bookmark navigation; rebuilding history without kubeconfig contexts", "error", err)
		contexts = nil
	}
	contexts = m.withUnionSetRows(contexts)
	var resourceTypes []model.Item
	if discovered := m.discoveredResources[target.lookupContext]; len(discovered) > 0 {
		resourceTypes = model.BuildSidebarItems(discovered)
	} else {
		resourceTypes = model.BuildSidebarItems(model.SeedResources())
	}
	m.leftItemsHistory = [][]model.Item{contexts}
	m.leftItems = resourceTypes
	return resourceTypes
}
