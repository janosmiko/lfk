// Package app - pinned_summaries.go
// Per-context / per-union-set state for resource-type summaries pinned to the
// cluster dashboard (issue #525). Reuses the PinnedState shape and toggle
// helpers from pinned.go; only the file differs.
package app

import (
	"fmt"
	"path/filepath"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/paths"
	"github.com/janosmiko/lfk/internal/ui"
)

// maxPinnedSummaries caps how many summaries one scope may pin: each pinned
// kind is a full cluster-wide list call on every dashboard refresh.
const maxPinnedSummaries = 10

// defaultPinnedSummaries render when nothing is pinned anywhere (issue #525,
// Task 10). CRD-backed entries are silently skipped when the cluster does not
// have the type - defaults must never show a "(not installed)" placeholder,
// unlike an explicit pin. The set applies only while effectivePinnedSummaries
// is empty: config pins replace it outright, while the first interactive pin
// seeds the visible defaults into state before toggling (see
// seedDefaultPinnedSummaries) so the rows the user sees are kept.
var defaultPinnedSummaries = []string{
	"batch/jobs",
	"apps/deployments",
	"argoproj.io/applications",
	"kustomize.toolkit.fluxcd.io/kustomizations",
	"cert-manager.io/certificates",
}

// defaultPinsDisabled reports whether the user explicitly configured
// pinned_summaries: [] (as opposed to leaving the key absent): "no summaries,
// not even the defaults" rather than "use the defaults". ConfigPinnedSummaries
// alone can't distinguish these - both leave it empty.
func defaultPinsDisabled() bool {
	return ui.ConfigPinnedSummariesSet && len(ui.ConfigPinnedSummaries) == 0
}

// pinnedSummariesFilePath returns the path to the pinned-summaries state file.
func pinnedSummariesFilePath() string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "pinned_summaries.yaml")
}

// loadPinnedSummariesState reads pinned dashboard summaries from disk.
func loadPinnedSummariesState() *PinnedState { return loadPinStateFile(pinnedSummariesFilePath()) }

// savePinnedSummariesState writes pinned dashboard summaries to disk.
func savePinnedSummariesState(s *PinnedState) error {
	return savePinStateFile(pinnedSummariesFilePath(), s)
}

// effectivePinnedSummaries merges config-level pinned summaries with the
// active scope's state (context, or named union set), config first, deduped,
// capped at maxPinnedSummaries. Order is pin order, which is also render
// order on the dashboard.
func (m Model) effectivePinnedSummaries() []string {
	seen := make(map[string]bool)
	var merged []string
	add := func(keys []string) {
		for _, k := range keys {
			if k == "" || seen[k] {
				continue
			}
			seen[k] = true
			merged = append(merged, k)
		}
	}
	add(ui.ConfigPinnedSummaries)
	if m.pinnedSummariesState != nil {
		switch {
		case m.isUnionSentinel() && m.unionSetName != "":
			add(m.pinnedSummariesState.UnionSets[m.unionSetName])
		case !m.isUnionSentinel() && m.nav.Context != "":
			add(m.pinnedSummariesState.Contexts[m.nav.Context])
		}
	}
	if len(merged) > maxPinnedSummaries {
		merged = merged[:maxPinnedSummaries]
	}
	return merged
}

// isSummaryPinned reports whether the type key's summary is pinned in the
// active scope's state (config-level pins are file-managed and not consulted,
// mirroring isTypePinned).
func (m Model) isSummaryPinned(key string) bool {
	if m.pinnedSummariesState == nil {
		return false
	}
	if m.isUnionSentinel() && m.unionSetName != "" {
		return slices.Contains(m.pinnedSummariesState.UnionSets[m.unionSetName], key)
	}
	return slices.Contains(m.pinnedSummariesState.Contexts[m.nav.Context], key)
}

// isSummaryActive reports whether key's summary is currently visible on the
// dashboard: either explicitly pinned, or implied by the built-in defaults
// being active for this scope. The action menu and the toggle's cap check
// both need this (not isSummaryPinned) so a default the user already sees
// is offered as "Unpin" and never blocked by the cap it hasn't actually hit
// yet.
func (m Model) isSummaryActive(key string) bool {
	return m.isSummaryPinned(key) || (m.defaultsActiveForScope() && slices.Contains(defaultPinnedSummaries, key))
}

// currentScopeSlice returns the active scope's raw persisted pin slice
// (context, or named union set), without merging config pins. nil when the
// scope has no entry yet.
func (m Model) currentScopeSlice() []string {
	if m.pinnedSummariesState == nil {
		return nil
	}
	if m.isUnionSentinel() && m.unionSetName != "" {
		return m.pinnedSummariesState.UnionSets[m.unionSetName]
	}
	return m.pinnedSummariesState.Contexts[m.nav.Context]
}

// setScopePinnedSummaries overwrites the active scope's persisted pin slice.
func (m Model) setScopePinnedSummaries(keys []string) {
	if m.isUnionSentinel() && m.unionSetName != "" {
		m.pinnedSummariesState.UnionSets[m.unionSetName] = keys
		return
	}
	m.pinnedSummariesState.Contexts[m.nav.Context] = keys
}

// defaultsActiveForScope reports whether the built-in defaults are what the
// dashboard currently renders for the active scope: nothing pinned there yet
// (state empty, no config pins), and defaults not explicitly disabled.
func (m Model) defaultsActiveForScope() bool {
	if defaultPinsDisabled() || len(ui.ConfigPinnedSummaries) > 0 {
		return false
	}
	return len(m.currentScopeSlice()) == 0
}

// defaultSeedDiscovery returns the discovery entries to resolve default pins
// against for the active scope: the per-context discovery map entry, or -
// for a named union set - every member context's discovery concatenated, so
// a default resolves if ANY member cluster has it.
func (m Model) defaultSeedDiscovery() []model.ResourceTypeEntry {
	if m.isUnionSentinel() && m.unionSetName != "" {
		var all []model.ResourceTypeEntry
		for _, ctx := range m.unionContexts {
			all = append(all, m.discoveredResources[ctx]...)
		}
		return all
	}
	return m.discoveredResources[m.nav.Context]
}

// seedDefaultPinnedSummaries returns the default keys to copy into the active
// scope's state before the requested toggle, so the user's first pin ADDS to
// what they already see instead of replacing it (issue #525 follow-up: a
// bare toggle wrote only the new key, making the previously-visible defaults
// vanish). Returns nil when defaults aren't currently active for this scope
// - the caller then performs a plain toggle, matching prior behavior.
//
// Only defaults that resolve against discovery are seeded: an unresolved CRD
// default must never become an explicit pin, since an explicit pin renders a
// "(not installed)" placeholder the user never saw as a default (see
// pinnedSummaryCmds' silentSkip). When there is no discovery data at all for
// the scope, fall back to seeding all five - better to over-seed than to
// silently drop rows the user is currently looking at.
func (m Model) seedDefaultPinnedSummaries() []string {
	if !m.defaultsActiveForScope() {
		return nil
	}
	discovered := m.defaultSeedDiscovery()
	if len(discovered) == 0 {
		return append([]string(nil), defaultPinnedSummaries...)
	}
	var resolved []string
	for _, k := range defaultPinnedSummaries {
		if _, ok := resolvePinnedSummaryEntry(discovered, k); ok {
			resolved = append(resolved, k)
		}
	}
	return resolved
}

// togglePinnedSummary pins or unpins the selected resource type's summary on
// the cluster dashboard, persists the change, and refreshes the dashboard.
// Mirrors handleKeyPinGroup: per-context (or per named union set) scope, cap
// enforcement before the toggle, in-memory rollback if the disk write fails.
func (m Model) togglePinnedSummary() (tea.Model, tea.Cmd) {
	if m.nav.Level != model.LevelResourceTypes {
		return m, nil
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	if sel.Kind == "__collapsed_group__" || sel.Category == "Dashboards" {
		m.setStatusMessage("Select a resource type to pin", true)
		return m, scheduleStatusClear()
	}
	key := model.PinKeyFromRef(sel.Extra)
	if key == "" {
		m.setStatusMessage("This item has no summary to pin", true)
		return m, scheduleStatusClear()
	}
	if m.isUnionSentinel() && m.unionSetName == "" {
		m.setStatusMessage("Pinning in union mode requires a named union set", true)
		return m, scheduleStatusClear()
	}
	if m.pinnedSummariesState == nil {
		m.pinnedSummariesState = newPinnedState()
	}

	// A first pin while the defaults are showing must ADD to what the user
	// already sees, not replace it: seed the resolved default subset into
	// state before the requested toggle. preSeed snapshots the scope's slice
	// beforehand so a save failure below can roll back both the seed and the
	// toggle in one step.
	preSeed := m.currentScopeSlice()
	if seed := m.seedDefaultPinnedSummaries(); len(seed) > 0 {
		m.setScopePinnedSummaries(seed)
	}

	// Check against the effective (config + state, capped) list, not the raw
	// state count: the dashboard renders effectivePinnedSummaries, so a new
	// state pin that passes a state-only check could still be truncated off
	// silently when config pins already fill the cap. isSummaryActive (not
	// isSummaryPinned) so unpinning an already-active default is never
	// blocked by the cap its own seed just filled.
	if !m.isSummaryActive(key) && len(m.effectivePinnedSummaries()) >= maxPinnedSummaries {
		m.setScopePinnedSummaries(preSeed)
		m.setStatusMessage(fmt.Sprintf("Pinned-summary limit reached (%d)", maxPinnedSummaries), true)
		return m, scheduleStatusClear()
	}

	var pinned bool
	scopeLabel := ""
	if m.isUnionSentinel() {
		pinned = togglePinnedUnionSetType(m.pinnedSummariesState, m.unionSetName, key)
		scopeLabel = " for union set " + m.unionSetName
	} else {
		pinned = togglePinnedType(m.pinnedSummariesState, m.nav.Context, key)
	}
	if err := savePinnedSummariesState(m.pinnedSummariesState); err != nil {
		m.setScopePinnedSummaries(preSeed)
		m.setStatusMessage(fmt.Sprintf("Failed to save pinned summaries: %v", err), true)
		return m, scheduleStatusClear()
	}

	// Drop the cached dashboard frame so any later dashboard view recomposes
	// fresh instead of repainting the stale pin list from m.dashboardData.
	delete(m.dashboardData, m.dashboardPreviewTargetContext())
	m.dashboardPreview = ""
	m.dashboardEventsPreview = ""
	if pinned {
		m.setStatusMessage(fmt.Sprintf("Summary pinned to dashboard%s: %s", scopeLabel, sel.Name), false)
	} else {
		m.setStatusMessage(fmt.Sprintf("Summary unpinned from dashboard%s: %s", scopeLabel, sel.Name), false)
	}
	if reload := m.summaryDashboardReloadCmd(); reload != nil {
		return m, tea.Batch(reload, scheduleStatusClear())
	}
	return m, scheduleStatusClear()
}

// summaryDashboardReloadCmd returns the eager dashboard refresh for a pin
// toggle, or nil when the reload would be pointless: the dashboard is disabled
// by config (the gate every dashboard loader respects), or the union sentinel
// is active - there loadDashboard would fetch for unionContexts[0] while
// handleDashboardPartial filters on the sentinel context, discarding every
// section. The union dashboard reloads lazily via its own gated loader on the
// next view instead.
func (m Model) summaryDashboardReloadCmd() tea.Cmd {
	if !ui.ConfigDashboard || m.isUnionSentinel() {
		return nil
	}
	return m.loadDashboard()
}
