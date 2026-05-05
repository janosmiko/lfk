package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// cmdLoadOrphans returns a tea.Cmd that runs DetectOrphans for the given
// cache key. Returns nil when a previous load for the same key is still
// inflight — duplicate-fire protection so the overlay opening and a
// filter preset applying in the same tick don't issue two scans.
//
// The scan runs on a cancellable context whose cancel is stored per
// key; invalidators (namespace switch, context switch, R refresh) call
// it so the in-flight scan stops immediately rather than racing the
// new state. Each scan also carries a generation number so a result
// arriving after cancellation is dropped on arrival in
// handleOrphansLoaded.
//
// On completion the result lands as orphansLoadedMsg.
func (m *Model) cmdLoadOrphans(key orphanCacheKey) tea.Cmd {
	if m.orphanLoadInflight == nil {
		m.orphanLoadInflight = make(map[orphanCacheKey]orphanInflight)
	}
	if _, ok := m.orphanLoadInflight[key]; ok {
		return nil
	}
	m.orphanGen++
	gen := m.orphanGen
	ctx, cancel := context.WithCancel(context.Background())
	m.orphanLoadInflight[key] = orphanInflight{gen: gen, cancel: cancel}
	client := m.client
	return func() tea.Msg {
		report, err := client.DetectOrphans(ctx, key.kubeContext, key.namespace)
		return orphansLoadedMsg{key: key, gen: gen, report: report, err: err}
	}
}

// invalidateOrphanCacheForNamespace drops the cache entry for one
// namespace of the given context and cancels any in-flight scan for
// that key — without the cancel + generation gate, a stale scan could
// repopulate the cache after the user moved on. The cluster-wide entry
// (namespace == "") is preserved so the overlay's data isn't blown
// away by a per-namespace refresh.
func (m *Model) invalidateOrphanCacheForNamespace(kubeCtx, ns string) {
	key := orphanCacheKey{kubeContext: kubeCtx, namespace: ns}
	delete(m.orphanCache, key)
	if inflight, ok := m.orphanLoadInflight[key]; ok {
		inflight.cancel()
		delete(m.orphanLoadInflight, key)
	}
}

// invalidateOrphanCacheForContext drops every cache entry and cancels
// every in-flight scan for the given context. Called on context switch.
func (m *Model) invalidateOrphanCacheForContext(kubeCtx string) {
	for k := range m.orphanCache {
		if k.kubeContext == kubeCtx {
			delete(m.orphanCache, k)
		}
	}
	for k, inflight := range m.orphanLoadInflight {
		if k.kubeContext == kubeCtx {
			inflight.cancel()
			delete(m.orphanLoadInflight, k)
		}
	}
}

// handleOrphansLoaded persists the report to cache. A result whose gen
// no longer matches the inflight entry (or has no entry at all) is
// silently dropped — the scan was cancelled or superseded, and writing
// its data back would resurrect stale state on top of the user's newer
// scope. On a successful match the inflight entry is removed (idempotent
// cancel — calling cancel a second time is a no-op).
func (m Model) handleOrphansLoaded(msg orphansLoadedMsg) (Model, tea.Cmd) {
	if m.orphanLoadInflight == nil {
		m.orphanLoadInflight = make(map[orphanCacheKey]orphanInflight)
	}
	inflight, ok := m.orphanLoadInflight[msg.key]
	if !ok || inflight.gen != msg.gen {
		return m, nil
	}
	inflight.cancel()
	delete(m.orphanLoadInflight, msg.key)

	if m.orphanCache == nil {
		m.orphanCache = make(map[orphanCacheKey]*k8s.OrphanReport)
	}
	report := msg.report
	m.orphanCache[msg.key] = &report

	// If the overlay is showing the same key, push the report into
	// orphanState so the next render reflects the load.
	clusterKey := orphanCacheKey{kubeContext: m.nav.Context, namespace: ""}
	if m.overlay == overlayOrphans && msg.key == clusterKey {
		m.orphans.report = report
		m.orphans.partial = msg.err
		m.orphans.loading = false
	}

	// When a per-list orphan filter is active, the user originally saw
	// an empty list because the matcher's cache slot was unpopulated.
	// Now that the slot is filled, re-run the matcher against the
	// captured unfiltered list so the visible rows reflect the just-
	// loaded orphans. The matcher rebuilds itself lazily via the cache
	// pointer compare; if msg.key isn't the matcher's key the rerun is
	// effectively a no-op (no new positives), which is fine.
	if m.activeFilterPreset != nil && len(m.unfilteredMiddleItems) > 0 {
		var filtered []model.Item
		for _, item := range m.unfilteredMiddleItems {
			if m.activeFilterPreset.MatchFn(item) {
				filtered = append(filtered, item)
			}
		}
		m.setMiddleItems(filtered)
		// Replace the "Scanning for X orphans..." status with the
		// final match count so the user has feedback the scan is done.
		// Only fire when the overlay is closed — the overlay has its
		// own loading indicator and shouldn't be shadowed by the
		// global status bar.
		if m.overlay != overlayOrphans {
			m.setStatusMessage(fmt.Sprintf("Filter: %s (%d matches)", m.activeFilterPreset.Name, len(filtered)), false)
		}
	}

	if msg.err != nil && m.overlay != overlayOrphans {
		m.setStatusMessage("Orphan scan: partial result ("+msg.err.Error()+")", true)
		return m, scheduleStatusClear()
	}
	return m, nil
}
