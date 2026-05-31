package app

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/logger"
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
//
// In union mode at LevelResources, nav.Context holds UnionContextSentinel
// — a synthetic value that is never a valid kubeconfig context name. Every
// caller of activeContext (cache key, GetNamespaces, completion) needs a
// real cluster, so resolve it to unionContexts[0] here. The union assumes
// homogeneous clusters, so any one of them is a representative source for
// namespace listing and similar metadata; if clusters diverge, the user
// can drill into a specific cluster and re-run the operation.
func (m Model) activeContext() string {
	if m.isUnionSentinel() {
		if len(m.unionContexts) > 0 {
			return m.unionContexts[0]
		}
		return ""
	}
	if m.nav.Context != "" {
		return m.nav.Context
	}
	if m.client != nil {
		return m.client.CurrentContext()
	}
	return ""
}

// discoveryContext returns the context key used for API discovery metadata.
// Union-mode discovery is intentionally stored under the first member context,
// while row-level API calls may target any member via effectiveContext().
func (m Model) discoveryContext() string {
	if m.isUnionSentinel() && len(m.unionContexts) > 0 {
		return m.unionContexts[0]
	}
	return m.nav.Context
}

// ensureNamespaceCacheFresh returns a command that refreshes the
// namespace cache for the current context when the entry is missing,
// empty, or older than namespaceCacheTTL; returns nil otherwise.
// Context-open paths (drilling into a cluster, `:ctx`, bookmark
// activation, session restore) batch it so the first `:` open in the
// newly-opened context has completions ready without waiting for the
// user's keystroke to trigger the fetch.
func (m Model) ensureNamespaceCacheFresh() tea.Cmd {
	return m.ensureNamespaceCacheFreshForContext(m.activeContext())
}

func (m Model) ensureNamespaceCacheFreshForContext(contextName string) tea.Cmd {
	entry, ok := m.cachedNamespaces[contextName]
	if !ok || len(entry.names) == 0 || time.Since(entry.fetchedAt) > namespaceCacheTTL {
		// Silent: this is a background cache refresh, not an overlay-
		// triggered load. The handler must NOT clear m.loading or we
		// race with in-flight API discovery on session restore and
		// produce a "No items" flash in the resource-types list.
		return m.loadNamespacesForContext(contextName, true)
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

// applyPinnedTypes recomputes model.PinnedTypes from config-level pins plus the
// pins scoped to the active context (or named union set). Legacy whole-group
// pins (a bare group name with no "/", from older pinned.yaml files or the
// config's pinned_groups) are expanded into their currently-discovered member
// resource types. Per-context state holding legacy group entries is migrated to
// expanded type keys in place once discovery has surfaced members.
func (m *Model) applyPinnedTypes() {
	discCtx := m.nav.Context
	if m.isUnionSentinel() && len(m.unionContexts) > 0 {
		discCtx = m.unionContexts[0]
	}
	discovered := m.discoveredResources[discCtx]

	seen := make(map[string]bool)
	var merged []string
	add := func(key string) {
		if key != "" && !seen[key] {
			merged = append(merged, key)
			seen[key] = true
		}
	}
	// expand resolves raw entries (type keys with "/", or legacy group names
	// without "/") into pin keys, expanding groups via discovery.
	expand := func(raw []string) {
		for _, e := range raw {
			if strings.Contains(e, "/") {
				add(e)
				continue
			}
			for _, rt := range discovered {
				if rt.APIGroup == e {
					add(rt.APIGroup + "/" + rt.Resource)
				}
			}
		}
	}

	// Config-level pins: new type keys plus legacy group names.
	expand(ui.ConfigPinnedTypes)
	expand(ui.ConfigPinnedGroups)

	// Per-scope pins, migrating any legacy group entries in place.
	if m.pinnedState != nil {
		switch {
		case m.isUnionSentinel() && m.unionSetName != "":
			expand(m.pinnedState.UnionSets[m.unionSetName])
			m.migratePinnedScope(m.pinnedState.UnionSets, m.unionSetName, discovered)
		case !m.isUnionSentinel() && m.nav.Context != "":
			expand(m.pinnedState.Contexts[m.nav.Context])
			m.migratePinnedScope(m.pinnedState.Contexts, m.nav.Context, discovered)
		}
	}

	model.PinnedTypes = merged
}

// migratePinnedScope rewrites legacy whole-group entries (no "/") for one scope
// key into their currently-discovered member type keys, persisting the change
// once. Group entries with no discovered members are kept as-is so a pin for a
// not-yet-installed CRD group is not silently lost. No-op (no disk write) when
// there is nothing to migrate.
func (m *Model) migratePinnedScope(scope map[string][]string, key string, discovered []model.ResourceTypeEntry) {
	entries := scope[key]
	hasLegacy := false
	for _, e := range entries {
		if !strings.Contains(e, "/") {
			hasLegacy = true
			break
		}
	}
	if !hasLegacy {
		return
	}
	seen := make(map[string]bool)
	var out []string
	keep := func(str string) {
		if str != "" && !seen[str] {
			out = append(out, str)
			seen[str] = true
		}
	}
	for _, e := range entries {
		if strings.Contains(e, "/") {
			keep(e)
			continue
		}
		members := 0
		for _, rt := range discovered {
			if rt.APIGroup == e {
				keep(rt.APIGroup + "/" + rt.Resource)
				members++
			}
		}
		if members == 0 {
			keep(e) // keep legacy group pin until its CRD is installed
		}
	}
	scope[key] = out
	if err := savePinnedState(m.pinnedState); err != nil {
		logger.Warn("Failed to persist pinned-type migration", "error", err, "scope", key)
	}
}

// SetVersion sets the application version string displayed in the title bar.
func (m *Model) SetVersion(v string) {
	m.version = v
}

// SetStderrChan sets the channel for receiving captured stderr messages.
func (m *Model) SetStderrChan(ch <-chan string) {
	m.stderrChan = ch
}

// contextsOrEmpty returns the current kubeconfig context list as sidebar
// items, or an empty slice when GetContexts reports an error. Errors here
// are observable in lfk.log but never block the caller — the worst case
// is a brief "no contexts" state until the next refresh.
func (m Model) contextsOrEmpty() []model.Item {
	contexts, err := m.client.GetContexts()
	if err != nil {
		logger.Warn("GetContexts failed while rebuilding left history; using empty list", "error", err)
		return nil
	}
	return contexts
}
