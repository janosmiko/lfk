package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/k8s"
)

// cmdLoadOrphans returns a tea.Cmd that runs DetectOrphans for the given
// cache key. Returns nil when a previous load for the same key is still
// inflight — duplicate-fire protection so the overlay opening and a
// filter preset applying in the same tick don't issue two scans.
//
// On completion the result lands as orphansLoadedMsg, handled by
// handleOrphansLoaded.
func (m *Model) cmdLoadOrphans(key orphanCacheKey) tea.Cmd {
	if m.orphanLoadInflight == nil {
		m.orphanLoadInflight = make(map[orphanCacheKey]bool)
	}
	if m.orphanLoadInflight[key] {
		return nil
	}
	m.orphanLoadInflight[key] = true
	client := m.client
	return func() tea.Msg {
		report, err := client.DetectOrphans(context.Background(), key.kubeContext, key.namespace)
		return orphansLoadedMsg{key: key, report: report, err: err}
	}
}

// invalidateOrphanCacheForNamespace drops cache entries for one namespace
// of the given context. The cluster-wide entry (namespace == "") is
// preserved so the overlay's data isn't blown away by a per-namespace
// refresh.
func (m *Model) invalidateOrphanCacheForNamespace(kubeCtx, ns string) {
	delete(m.orphanCache, orphanCacheKey{kubeContext: kubeCtx, namespace: ns})
}

// invalidateOrphanCacheForContext drops every cache entry for the given
// context. Called on context switch.
func (m *Model) invalidateOrphanCacheForContext(kubeCtx string) {
	for k := range m.orphanCache {
		if k.kubeContext == kubeCtx {
			delete(m.orphanCache, k)
		}
	}
}

// handleOrphansLoaded persists the report to cache and clears the
// inflight flag. The caller (Update) is responsible for deciding what
// follow-up the UI needs.
func (m Model) handleOrphansLoaded(msg orphansLoadedMsg) (Model, tea.Cmd) {
	if m.orphanCache == nil {
		m.orphanCache = make(map[orphanCacheKey]*k8s.OrphanReport)
	}
	report := msg.report
	m.orphanCache[msg.key] = &report
	if m.orphanLoadInflight == nil {
		m.orphanLoadInflight = make(map[orphanCacheKey]bool)
	}
	m.orphanLoadInflight[msg.key] = false

	// If the overlay is showing the same key, push the report into
	// orphanState so the next render reflects the load.
	clusterKey := orphanCacheKey{kubeContext: m.nav.Context, namespace: ""}
	if m.overlay == overlayOrphans && msg.key == clusterKey {
		m.orphans.report = report
		m.orphans.partial = msg.err
		m.orphans.loading = false
	}

	if msg.err != nil && m.overlay != overlayOrphans {
		m.setStatusMessage("Orphan scan: partial result ("+msg.err.Error()+")", true)
		return m, scheduleStatusClear()
	}
	return m, nil
}
