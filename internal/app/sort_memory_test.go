package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- sortMemoryFilePath ---

func TestSortMemoryFilePath(t *testing.T) {
	t.Run("uses XDG_STATE_HOME", func(t *testing.T) {
		t.Setenv("LFK_STATE_DIR", "") // takes precedence over XDG; clear it
		t.Setenv("XDG_STATE_HOME", "/custom/state")
		path := sortMemoryFilePath()
		assert.Equal(t, "/custom/state/lfk/sort_memory.yaml", path)
	})

	t.Run("falls back to home", func(t *testing.T) {
		t.Setenv("LFK_STATE_DIR", "")
		t.Setenv("XDG_STATE_HOME", "")
		path := sortMemoryFilePath()
		assert.Contains(t, path, filepath.Join(".local", "state", "lfk", "sort_memory.yaml"))
	})
}

// --- save / load round-trip ---

func TestSortMemoryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", dir)

	mem := map[string]sortPref{
		"prod\x00apps/v1/deployments": {column: "Ready", ascending: false},
		"prod\x00/v1/pods":            {column: "Status", ascending: true},
		"dev\x00/v1/pods":             {column: "Name", ascending: true},
	}

	require.NoError(t, saveSortMemory(mem))

	got := loadSortMemory()
	assert.Equal(t, mem, got)
}

func TestLoadSortMemoryMissingFile(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	got := loadSortMemory()
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestLoadSortMemoryCorrupt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sort_memory.yaml"), []byte("{not: valid: yaml: ["), 0o644))

	got := loadSortMemory()
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestLoadSortMemoryRebuildsValidKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", dir)

	// The nested on-disk shape is flattened back to context\x00gvr keys. (The
	// malformed-key skip lives on the save side and is covered by
	// TestSortMemoryToStateSkipsMalformedKeys.)
	yaml := "contexts:\n  prod:\n    apps/v1/deployments:\n      column: Ready\n      ascending: false\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sort_memory.yaml"), []byte(yaml), 0o644))

	got := loadSortMemory()
	assert.Equal(t, map[string]sortPref{
		"prod\x00apps/v1/deployments": {column: "Ready", ascending: false},
	}, got)
}

// --- conversion helpers ---

func TestSortMemoryStateConversion(t *testing.T) {
	mem := map[string]sortPref{
		"prod\x00apps/v1/deployments": {column: "Ready", ascending: false},
		"prod\x00/v1/pods":            {column: "Status", ascending: true},
	}

	state := sortMemoryToState(mem)
	require.Len(t, state.Contexts, 1)
	assert.Equal(t, persistedSortPref{Column: "Ready", Ascending: false}, state.Contexts["prod"]["apps/v1/deployments"])
	assert.Equal(t, persistedSortPref{Column: "Status", Ascending: true}, state.Contexts["prod"]["/v1/pods"])

	// Round-trips back to the same in-memory map.
	assert.Equal(t, mem, sortMemoryFromState(state))
}

func TestSortMemoryToStateSkipsMalformedKeys(t *testing.T) {
	mem := map[string]sortPref{
		"nogvrkey":                    {column: "Name", ascending: true}, // missing \x00 separator
		"prod\x00apps/v1/deployments": {column: "Ready", ascending: false},
	}
	state := sortMemoryToState(mem)
	require.Len(t, state.Contexts, 1)
	assert.Equal(t, persistedSortPref{Column: "Ready", Ascending: false}, state.Contexts["prod"]["apps/v1/deployments"])
}

// --- delta persistence merges rather than clobbering ---

func TestPersistRememberedSortMerges(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	// Tab A persists a sort for deployments.
	persistRememberedSort("prod\x00apps/v1/deployments", sortPref{column: "Ready", ascending: false})
	// Tab B (different kind) persists its own sort; must not drop tab A's.
	persistRememberedSort("prod\x00/v1/pods", sortPref{column: "Status", ascending: true})

	got := loadSortMemory()
	assert.Equal(t, map[string]sortPref{
		"prod\x00apps/v1/deployments": {column: "Ready", ascending: false},
		"prod\x00/v1/pods":            {column: "Status", ascending: true},
	}, got)
}

func TestPersistForgottenSortRemovesOnlyTarget(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	persistRememberedSort("prod\x00apps/v1/deployments", sortPref{column: "Ready", ascending: false})
	persistRememberedSort("prod\x00/v1/pods", sortPref{column: "Status", ascending: true})

	persistForgottenSort("prod\x00/v1/pods")

	got := loadSortMemory()
	assert.Equal(t, map[string]sortPref{
		"prod\x00apps/v1/deployments": {column: "Ready", ascending: false},
	}, got)
}

// TestBuildSessionTabStateSeedsSortMemory verifies a session-restored tab
// (the restart path for issue #353) loads persisted sort prefs from disk rather
// than starting blank, which would otherwise override the model's loaded memory
// when the tab is activated.
func TestBuildSessionTabStateSeedsSortMemory(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	require.NoError(t, saveSortMemory(map[string]sortPref{
		"prod\x00apps/v1/deployments": {column: "Ready", ascending: false},
	}))

	tab := buildSessionTabState(&SessionTab{Context: "prod", AllNamespaces: true}, nil)
	assert.Equal(t, sortPref{column: "Ready", ascending: false}, tab.sortMemory["prod\x00apps/v1/deployments"])
}

// --- empty map persists without creating an unreadable file ---

func TestSaveSortMemoryEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", dir)
	require.NoError(t, saveSortMemory(map[string]sortPref{}))
	got := loadSortMemory()
	assert.Empty(t, got)
}
