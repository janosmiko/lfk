package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/logger"
)

const sortMemoryFileName = "sort_memory.yaml"

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
	return stateFilePath(sortMemoryFileName)
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
	return sortMemoryFromState(loadStateFile[SortMemoryState](sortMemoryFileName))
}

// saveSortMemory writes remembered sort preferences to disk.
func saveSortMemory(mem map[string]sortPref) error {
	return saveStateFile(sortMemoryFileName, sortMemoryToState(mem))
}

// persistRememberedSort writes a single remembered sort pref to disk, merging it
// with prefs persisted by other tabs/contexts so a sort in one tab never
// clobbers another's (sortMemory is per-tab; the state file is shared).
// Best-effort: a write failure means the sort won't survive the next restart, so
// log it for disk-full / permissions diagnosis from lfk.log.
//
// Callers run on the single Bubble Tea Update goroutine (rememberSort and
// forgetSort from key and mouse handlers). The lock extends that across
// processes, so a second lfk instance cannot drop the sort this one just
// recorded. Do not call this from a background goroutine.
func persistRememberedSort(key string, pref sortPref) {
	path := sortMemoryFilePath()
	if path == "" {
		return
	}
	withStateFileLock(path, func() {
		mem := loadSortMemory()
		mem[key] = pref
		if err := saveSortMemory(mem); err != nil {
			logger.Error("Failed to persist sort memory", "error", err)
		}
	})
}

// persistForgottenSort removes a single sort pref from disk (the sort-reset
// action), leaving every other remembered sort intact. Best-effort.
func persistForgottenSort(key string) {
	path := sortMemoryFilePath()
	if path == "" {
		return
	}
	withStateFileLock(path, func() {
		mem := loadSortMemory()
		if _, ok := mem[key]; !ok {
			return
		}
		delete(mem, key)
		if err := saveSortMemory(mem); err != nil {
			logger.Error("Failed to persist sort memory", "error", err)
		}
	})
}
