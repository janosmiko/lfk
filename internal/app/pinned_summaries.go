// Package app - pinned_summaries.go
// Per-context / per-union-set state for resource-type summaries pinned to the
// cluster dashboard (issue #525). Reuses the PinnedState shape and toggle
// helpers from pinned.go; only the file differs.
package app

import (
	"path/filepath"
	"slices"

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
