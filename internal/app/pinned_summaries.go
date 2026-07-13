// Package app - pinned_summaries.go
// Per-context / per-union-set state for resource-type summaries pinned to the
// cluster dashboard (issue #525). Reuses the PinnedState shape and toggle
// helpers from pinned.go; only the file differs.
package app

import (
	"fmt"
	"path/filepath"
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/paths"
	"github.com/janosmiko/lfk/internal/ui"
)

// maxPinnedSummaries caps how many summaries one scope may pin: each pinned
// kind is a full cluster-wide list call on every dashboard refresh.
const maxPinnedSummaries = 10

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
	// Check against the effective (config + state, capped) list, not the raw
	// state count: the dashboard renders effectivePinnedSummaries, so a new
	// state pin that passes a state-only check could still be truncated off
	// silently when config pins already fill the cap.
	if !m.isSummaryPinned(key) && len(m.effectivePinnedSummaries()) >= maxPinnedSummaries {
		m.setStatusMessage(fmt.Sprintf("Pinned-summary limit reached (%d)", maxPinnedSummaries), true)
		return m, scheduleStatusClear()
	}

	var pinned bool
	var undo func()
	scopeLabel := ""
	if m.isUnionSentinel() {
		pinned = togglePinnedUnionSetType(m.pinnedSummariesState, m.unionSetName, key)
		undo = func() { _ = togglePinnedUnionSetType(m.pinnedSummariesState, m.unionSetName, key) }
		scopeLabel = " for union set " + m.unionSetName
	} else {
		pinned = togglePinnedType(m.pinnedSummariesState, m.nav.Context, key)
		undo = func() { _ = togglePinnedType(m.pinnedSummariesState, m.nav.Context, key) }
	}
	if err := savePinnedSummariesState(m.pinnedSummariesState); err != nil {
		undo()
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
