package app

import (
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/paths"
)

// persistedSortPref is the on-disk form of sortPref. sortPref's fields are
// unexported (so they never leak across packages); this mirror exposes them for
// YAML serialisation.
type persistedSortPref struct {
	Column    string `json:"column" yaml:"column"`
	Ascending bool   `json:"ascending" yaml:"ascending"`
}

// SortMemoryState is the on-disk schema for remembered per-kind sort
// preferences. It mirrors HiddenTypesState/PinnedState scoping: a kube context
// maps to a set of resource kinds (keyed by GVR) and their chosen sort. The
// in-memory representation flattens this to a "context\x00gvr" keyed map; see
// sortMemoryKey in app_sort.go.
type SortMemoryState struct {
	Contexts map[string]map[string]persistedSortPref `json:"contexts" yaml:"contexts"`
}

// sortMemoryFilePath returns the path to the sort-memory state file.
func sortMemoryFilePath() string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "sort_memory.yaml")
}

// sortMemoryToState converts the in-memory "context\x00gvr" keyed map into the
// nested on-disk shape. Keys missing the separator are dropped defensively.
func sortMemoryToState(mem map[string]sortPref) SortMemoryState {
	s := SortMemoryState{Contexts: make(map[string]map[string]persistedSortPref)}
	for key, pref := range mem {
		context, gvr, ok := strings.Cut(key, "\x00")
		if !ok {
			continue
		}
		if s.Contexts[context] == nil {
			s.Contexts[context] = make(map[string]persistedSortPref)
		}
		s.Contexts[context][gvr] = persistedSortPref{Column: pref.column, Ascending: pref.ascending}
	}
	return s
}

// sortMemoryFromState flattens the nested on-disk shape back into the in-memory
// "context\x00gvr" keyed map. Never nil.
func sortMemoryFromState(s SortMemoryState) map[string]sortPref {
	mem := make(map[string]sortPref)
	for context, kinds := range s.Contexts {
		for gvr, pref := range kinds {
			mem[context+"\x00"+gvr] = sortPref{column: pref.Column, ascending: pref.Ascending}
		}
	}
	return mem
}

// loadSortMemory reads remembered sort preferences from disk, returning an empty
// (never nil) map when the file is missing or corrupt.
func loadSortMemory() map[string]sortPref {
	path := sortMemoryFilePath()
	if path == "" {
		return make(map[string]sortPref)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read sort-memory state", "error", err, "path", path)
		}
		return make(map[string]sortPref)
	}
	var s SortMemoryState
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Sort-memory file is corrupt; starting fresh", "error", err, "path", path)
		return make(map[string]sortPref)
	}
	return sortMemoryFromState(s)
}

// saveSortMemory writes remembered sort preferences to disk.
func saveSortMemory(mem map[string]sortPref) error {
	path := sortMemoryFilePath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(sortMemoryToState(mem))
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// persistRememberedSort writes a single remembered sort pref to disk, merging it
// with prefs persisted by other tabs/contexts so a sort in one tab never
// clobbers another's (sortMemory is per-tab; the state file is shared).
// Best-effort: a write failure means the sort won't survive the next restart, so
// log it for disk-full / permissions diagnosis from lfk.log.
//
// The load-modify-save is unsynchronised. Callers run on the single Bubble Tea
// Update goroutine (rememberSort/forgetSort from key and mouse handlers), so the
// sequence is never concurrent; do not call this from a background goroutine.
func persistRememberedSort(key string, pref sortPref) {
	mem := loadSortMemory()
	mem[key] = pref
	if err := saveSortMemory(mem); err != nil {
		logger.Error("Failed to persist sort memory", "error", err)
	}
}

// persistForgottenSort removes a single sort pref from disk (the sort-reset
// action), leaving every other remembered sort intact. Best-effort.
func persistForgottenSort(key string) {
	mem := loadSortMemory()
	if _, ok := mem[key]; !ok {
		return
	}
	delete(mem, key)
	if err := saveSortMemory(mem); err != nil {
		logger.Error("Failed to persist sort memory", "error", err)
	}
}
