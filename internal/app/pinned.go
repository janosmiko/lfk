package app

import (
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/paths"
)

// PinnedState stores pinned resource-type keys ("group/resource",
// version-agnostic) scoped either to a kube context or to a named union set.
// Older files may still hold legacy whole-group entries (a bare group name with
// no "/"); these are expanded into member types on first use (see
// applyPinnedTypes / migratePinnedScope).
type PinnedState struct {
	Contexts  map[string][]string `json:"contexts" yaml:"contexts"`
	UnionSets map[string][]string `json:"union_sets" yaml:"union_sets"`
}

// pinnedFilePath returns the path to the pinned groups state file.
func pinnedFilePath() string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "pinned.yaml")
}

// loadPinnedState reads pinned groups from disk.
func loadPinnedState() *PinnedState { return loadPinStateFile(pinnedFilePath()) }

// loadPinStateFile reads any PinnedState-shaped scope file (sidebar pins,
// pinned dashboard summaries). Missing or corrupt files start fresh.
func loadPinStateFile(path string) *PinnedState {
	if path == "" {
		return newPinnedState()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read pinned-scope state", "error", err, "path", path)
		}
		return newPinnedState()
	}
	var s PinnedState
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Pinned-scope file is corrupt; starting fresh", "error", err, "path", path)
		return newPinnedState()
	}
	if s.Contexts == nil {
		s.Contexts = make(map[string][]string)
	}
	if s.UnionSets == nil {
		s.UnionSets = make(map[string][]string)
	}
	return &s
}

func newPinnedState() *PinnedState {
	return &PinnedState{
		Contexts:  make(map[string][]string),
		UnionSets: make(map[string][]string),
	}
}

// savePinnedState writes pinned groups to disk.
func savePinnedState(s *PinnedState) error { return savePinStateFile(pinnedFilePath(), s) }

func savePinStateFile(path string, s *PinnedState) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// togglePinnedType adds or removes a resource-type key from the per-context
// pinned list. Returns true if it was added (pinned), false if removed.
func togglePinnedType(s *PinnedState, context, typeKey string) bool {
	if s.Contexts == nil {
		s.Contexts = make(map[string][]string)
	}
	return togglePinnedTypeIn(s.Contexts, context, typeKey)
}

func togglePinnedUnionSetType(s *PinnedState, unionSet, typeKey string) bool {
	if s.UnionSets == nil {
		s.UnionSets = make(map[string][]string)
	}
	return togglePinnedTypeIn(s.UnionSets, unionSet, typeKey)
}

func togglePinnedTypeIn(scope map[string][]string, key, typeKey string) bool {
	// Copy before mutating so we never shift elements in a backing array that
	// another slice header might still reference (e.g. an undo snapshot).
	keys := append([]string(nil), scope[key]...)
	for i, k := range keys {
		if k == typeKey {
			// Remove (unpin).
			scope[key] = append(keys[:i], keys[i+1:]...)
			return false
		}
	}
	// Add (pin).
	scope[key] = append(keys, typeKey)
	return true
}
