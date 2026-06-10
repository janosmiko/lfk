package app

import (
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/paths"
)

// HiddenTypesState stores the resource-type keys ("group/resource",
// version-agnostic — e.g. "networking.k8s.io/ingresses" or "/limitranges")
// the user has hidden from the sidebar, scoped either to a kube context or to
// a named union set. The shape mirrors PinnedState so the two preferences load,
// save, and scope identically; the difference is purely in how the sidebar
// consumes them (Pinned moves a type up, Hidden removes it unless reveal is on).
type HiddenTypesState struct {
	Contexts  map[string][]string `json:"contexts" yaml:"contexts"`
	UnionSets map[string][]string `json:"union_sets" yaml:"union_sets"`
}

// hiddenTypesFilePath returns the path to the hidden-types state file.
func hiddenTypesFilePath() string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "hidden_types.yaml")
}

// loadHiddenTypesState reads hidden types from disk, returning an empty state
// (never nil) when the file is missing or corrupt.
func loadHiddenTypesState() *HiddenTypesState {
	path := hiddenTypesFilePath()
	if path == "" {
		return newHiddenTypesState()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read hidden-types state", "error", err, "path", path)
		}
		return newHiddenTypesState()
	}
	var s HiddenTypesState
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Hidden-types file is corrupt; starting fresh", "error", err, "path", path)
		return newHiddenTypesState()
	}
	if s.Contexts == nil {
		s.Contexts = make(map[string][]string)
	}
	if s.UnionSets == nil {
		s.UnionSets = make(map[string][]string)
	}
	return &s
}

func newHiddenTypesState() *HiddenTypesState {
	return &HiddenTypesState{
		Contexts:  make(map[string][]string),
		UnionSets: make(map[string][]string),
	}
}

// saveHiddenTypesState writes hidden types to disk.
func saveHiddenTypesState(s *HiddenTypesState) error {
	path := hiddenTypesFilePath()
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

// toggleHiddenType adds or removes a resource-type key from the per-context
// hidden list. Returns true if it was added (hidden), false if removed (shown).
func toggleHiddenType(s *HiddenTypesState, context, typeKey string) bool {
	if s.Contexts == nil {
		s.Contexts = make(map[string][]string)
	}
	return toggleHiddenTypeIn(s.Contexts, context, typeKey)
}

// toggleHiddenUnionSetType adds or removes a resource-type key from a named
// union set's hidden list. Returns true if hidden, false if shown.
func toggleHiddenUnionSetType(s *HiddenTypesState, unionSet, typeKey string) bool {
	if s.UnionSets == nil {
		s.UnionSets = make(map[string][]string)
	}
	return toggleHiddenTypeIn(s.UnionSets, unionSet, typeKey)
}

func toggleHiddenTypeIn(scope map[string][]string, key, typeKey string) bool {
	// Copy before mutating so we never shift elements in a backing array that
	// another slice header might still reference (e.g. a snapshot taken for an
	// undo closure).
	keys := append([]string(nil), scope[key]...)
	for i, k := range keys {
		if k == typeKey {
			// Remove (show again).
			scope[key] = append(keys[:i], keys[i+1:]...)
			return false
		}
	}
	// Add (hide).
	scope[key] = append(keys, typeKey)
	return true
}
