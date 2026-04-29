package app

import (
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
)

// PinnedState stores per-context pinned CRD groups.
type PinnedState struct {
	Contexts map[string][]string `json:"contexts" yaml:"contexts"`
}

// pinnedFilePath returns the path to the pinned groups state file.
func pinnedFilePath() string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "lfk", "pinned.yaml")
}

// loadPinnedState reads pinned groups from disk.
func loadPinnedState() *PinnedState {
	path := pinnedFilePath()
	if path == "" {
		return &PinnedState{Contexts: make(map[string][]string)}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read pinned-groups state", "error", err, "path", path)
		}
		return &PinnedState{Contexts: make(map[string][]string)}
	}
	var s PinnedState
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Pinned-groups file is corrupt; starting fresh", "error", err, "path", path)
		return &PinnedState{Contexts: make(map[string][]string)}
	}
	if s.Contexts == nil {
		s.Contexts = make(map[string][]string)
	}
	return &s
}

// savePinnedState writes pinned groups to disk.
func savePinnedState(s *PinnedState) error {
	path := pinnedFilePath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// togglePinnedGroup adds or removes a group from the per-context pinned list.
// Returns true if the group was added (pinned), false if removed (unpinned).
func togglePinnedGroup(s *PinnedState, context, group string) bool {
	groups := s.Contexts[context]
	for i, g := range groups {
		if g == group {
			// Remove (unpin).
			s.Contexts[context] = append(groups[:i], groups[i+1:]...)
			return false
		}
	}
	// Add (pin).
	s.Contexts[context] = append(groups, group)
	return true
}
