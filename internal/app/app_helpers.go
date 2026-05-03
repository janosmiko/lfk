package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// namespaceCacheEntry holds the result of a namespace fetch plus the
// time it completed. The fetchedAt timestamp lets the command bar
// refresh stale entries without refetching on every open.
//
// items is the full model.Item slice (Name + Status), so the namespace
// selector overlay can reuse the cache without losing the Active /
// Terminating status colour. names is a parallel slice kept for the
// command-bar autocompleter, which only needs the strings — the small
// duplication is cheaper than re-extracting names on every keystroke.
type namespaceCacheEntry struct {
	items     []model.Item
	names     []string
	fetchedAt time.Time
}

// namespaceCacheTTL is how long a cached namespace list stays fresh.
// After this interval the command bar will trigger a background
// refresh on next open so newly created namespaces show up in
// completions without requiring an app restart. The stale entry stays
// visible until the refresh lands (stale-while-revalidate), so the UI
// never blinks between "has completions" and "empty".
//
// Actions that directly mutate namespaces (`:k create|delete ns ...`
// and template applies) bypass the TTL via invalidateNamespaceCache,
// so the common "I just made it" case is instant — the TTL is only a
// backstop for changes made outside the TUI.
const namespaceCacheTTL = 60 * time.Second

// activeContext returns the kubectl context that queries on behalf of
// the current tab should target. It prefers the tab-scoped nav.Context
// and falls back to the client's current context; returns "" when the
// client has not been initialised yet (e.g. in pre-startup tests) so
// callers never panic on a nil client.
func (m Model) activeContext() string {
	if m.nav.Context != "" {
		return m.nav.Context
	}
	if m.client != nil {
		return m.client.CurrentContext()
	}
	return ""
}

// ensureNamespaceCacheFresh returns a command that refreshes the
// namespace cache for the current context when the entry is missing,
// empty, or older than namespaceCacheTTL; returns nil otherwise.
// Context-open paths (drilling into a cluster, `:ctx`, bookmark
// activation, session restore) batch it so the first `:` open in the
// newly-opened context has completions ready without waiting for the
// user's keystroke to trigger the fetch.
func (m Model) ensureNamespaceCacheFresh() tea.Cmd {
	entry, ok := m.cachedNamespaces[m.activeContext()]
	if !ok || len(entry.names) == 0 || time.Since(entry.fetchedAt) > namespaceCacheTTL {
		// Silent: this is a background cache refresh, not an overlay-
		// triggered load. The handler must NOT clear m.loading or we
		// race with in-flight API discovery on session restore and
		// produce a "No items" flash in the resource-types list.
		return m.loadNamespacesSilent(true)
	}
	return nil
}

// invalidateNamespaceCache drops the cache entry for the current
// context so the next command bar open triggers a fresh fetch. Called
// after actions that mutate the cluster's namespace list (`:k create
// ns`, `:k delete ns`, template applies) so the new state is reflected
// in completions immediately instead of up to namespaceCacheTTL later.
func (m *Model) invalidateNamespaceCache() {
	delete(m.cachedNamespaces, m.activeContext())
}

// cancelAndReset cancels any in-flight API requests and creates a fresh
// context for subsequent requests. Safe to call multiple times.
func (m *Model) cancelAndReset() {
	if m.reqCancel != nil {
		m.reqCancel()
	}
	m.reqCtx, m.reqCancel = context.WithCancel(context.Background())
}

// cancelInFlightRequests cancels every outstanding API request (lists,
// discovery, YAML fetches, etc.) without creating a fresh context. Used
// by the quit paths so in-flight goroutines abort with context.Canceled
// rather than riding out kernel TCP timeouts on an unreachable cluster
// — which can stretch the apparent "quit" wait to a minute or more
// while the process waits for those goroutines to release the
// resources held in main's deferred cleanup (informer wg, stderr-pipe
// reader, etc.). cancelAndReset would also work here, but allocating a
// fresh context we never use is wasted motion at shutdown.
func (m *Model) cancelInFlightRequests() {
	if m.reqCancel != nil {
		m.reqCancel()
	}
}

// applyPinnedGroups merges config-level pinned groups with per-context pinned groups
// and sets model.PinnedGroups.
func (m *Model) applyPinnedGroups() {
	// Start with config-level pins.
	seen := make(map[string]bool)
	var merged []string
	for _, g := range ui.ConfigPinnedGroups {
		if !seen[g] {
			merged = append(merged, g)
			seen[g] = true
		}
	}
	// Add per-context pins.
	if m.pinnedState != nil && m.nav.Context != "" {
		for _, g := range m.pinnedState.Contexts[m.nav.Context] {
			if !seen[g] {
				merged = append(merged, g)
				seen[g] = true
			}
		}
	}
	model.PinnedGroups = merged
}

// SetVersion sets the application version string displayed in the title bar.
func (m *Model) SetVersion(v string) {
	m.version = v
}

// SetStderrChan sets the channel for receiving captured stderr messages.
func (m *Model) SetStderrChan(ch <-chan string) {
	m.stderrChan = ch
}
