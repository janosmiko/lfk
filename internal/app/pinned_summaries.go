// Package app - pinned_summaries.go
// Per-context / per-union-set state for resource-type summaries pinned to the
// cluster dashboard (issue #525). Reuses the PinnedState shape and toggle
// helpers from pinned.go; only the file differs.
package app

import (
	"path/filepath"

	"github.com/janosmiko/lfk/internal/paths"
)

// maxPinnedSummaries caps how many summaries one scope may pin: each pinned
// kind is a full cluster-wide list call on every dashboard refresh.
const maxPinnedSummaries = 10 //nolint:unused // wired in Task 4+ of the pinned-summaries feature

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
