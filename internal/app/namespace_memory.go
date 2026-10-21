package app

import (
	"maps"
	"slices"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

const rememberedNamespacesFile = "remembered-namespaces.yaml"

type rememberedScope struct {
	Namespace          string   `json:"namespace"`
	AllNamespaces      bool     `json:"all_namespaces"`
	SelectedNamespaces []string `json:"selected_namespaces,omitempty"`
	NsSelectionNegated bool     `json:"ns_selection_negated,omitempty"`
}

type rememberedNamespacesState struct {
	Contexts map[string]rememberedScope `json:"contexts"`
}

func toRememberedScope(s nsScope) rememberedScope {
	return rememberedScope{
		Namespace:          s.namespace,
		AllNamespaces:      s.allNamespaces,
		SelectedNamespaces: slices.Sorted(maps.Keys(s.selectedNamespaces)),
		NsSelectionNegated: s.nsSelectionNegated,
	}
}

func (r rememberedScope) scope() nsScope {
	sel := make(map[string]bool, len(r.SelectedNamespaces))
	for _, ns := range r.SelectedNamespaces {
		sel[ns] = true
	}
	return nsScope{
		namespace:          r.Namespace,
		allNamespaces:      r.AllNamespaces,
		selectedNamespaces: sel,
		nsSelectionNegated: r.NsSelectionNegated,
	}
}

// loadRememberedScope reports false for a missing, corrupt or empty entry.
func loadRememberedScope(ctx string) (nsScope, bool) {
	r, ok := loadStateFile[rememberedNamespacesState](rememberedNamespacesFile).Contexts[ctx]
	if !ok || (!r.AllNamespaces && r.Namespace == "" && len(r.SelectedNamespaces) == 0) {
		return nsScope{}, false
	}
	return r.scope(), true
}

func (m *Model) rememberEnabled() bool {
	return ui.ConfigRememberNamespaces && !m.unionMode
}

// saveRememberedScopes merges updates into the file under its lock. A file
// that fails to decode is left untouched.
func saveRememberedScopes(updates map[string]nsScope) {
	if len(updates) == 0 {
		return
	}
	err := writeStateFileGuarded[rememberedNamespacesState](rememberedNamespacesFile, func(string) error {
		st, err := loadStateFileStrict[rememberedNamespacesState](rememberedNamespacesFile)
		if err != nil {
			return err
		}
		if st.Contexts == nil {
			st.Contexts = make(map[string]rememberedScope, len(updates))
		}
		for ctx, s := range updates {
			st.Contexts[ctx] = toRememberedScope(s)
		}
		return saveStateFile(rememberedNamespacesFile, st)
	})
	if err != nil {
		logger.Warn("Could not save remembered namespaces", "error", err)
	}
}

// rememberScope stores the current scope as the one used in oldCtx. Call it
// before nav.Context changes.
func (m *Model) rememberScope(oldCtx string) {
	if !m.rememberEnabled() || oldCtx == "" {
		return
	}
	saveRememberedScopes(map[string]nsScope{oldCtx: m.captureNamespaceScope()})
}

// rememberAllTabScopes stores the scope of every tab under its own context.
func (m *Model) rememberAllTabScopes() {
	if !m.rememberEnabled() {
		return
	}
	m.saveCurrentTab()
	updates := make(map[string]nsScope, len(m.tabs))
	for _, t := range m.tabs {
		ctx := t.nav.Context
		if ctx == "" || ctx == UnionContextSentinel {
			continue
		}
		updates[ctx] = nsScope{
			namespace:          t.namespace,
			allNamespaces:      t.allNamespaces,
			selectedNamespaces: t.selectedNamespaces,
			nsSelectionNegated: t.nsSelectionNegated,
		}
	}
	saveRememberedScopes(updates)
}

// applyRememberedToSessionTab fills tab from the stored scope for ctx and
// reports whether there was one. It is the startup twin of restoreRememberedScope.
func applyRememberedToSessionTab(tab *SessionTab, ctx string) bool {
	if !ui.ConfigRememberNamespaces {
		return false
	}
	s, ok := loadRememberedScope(ctx)
	if !ok {
		return false
	}
	r := toRememberedScope(s)
	tab.AllNamespaces = r.AllNamespaces
	tab.Namespace = r.Namespace
	tab.SelectedNamespaces = r.SelectedNamespaces
	tab.NsSelectionNegated = r.NsSelectionNegated
	return true
}

// restoreRememberedScope applies the stored scope for ctx and reports whether
// there was one.
func (m *Model) restoreRememberedScope(ctx string) bool {
	if !m.rememberEnabled() {
		return false
	}
	s, ok := loadRememberedScope(ctx)
	if !ok {
		return false
	}
	m.applyNamespaceScope(s)
	return true
}
