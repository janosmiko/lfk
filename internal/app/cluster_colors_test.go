package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withClusterColorsStateDir points XDG_STATE_HOME at a fresh temp dir so each
// test gets an isolated cluster-colors file without touching the user's real
// state dir. Returns the dir for path assertions.
func withClusterColorsStateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	return dir
}

func TestClusterColors_RoundTrip(t *testing.T) {
	withClusterColorsStateDir(t)

	in := map[string]string{
		"prod-eu":    "red",
		"staging-eu": "yellow",
		"dev-local":  "green",
	}
	require.NoError(t, saveClusterColors(in))

	loaded := loadClusterColors()
	assert.Equal(t, in, loaded)
}

func TestClusterColors_LivesUnderXDGStateHome(t *testing.T) {
	dir := withClusterColorsStateDir(t)

	require.NoError(t, saveClusterColors(map[string]string{"prod": "red"}))

	expected := filepath.Join(dir, "lfk", "cluster-colors.yaml")
	_, err := os.Stat(expected)
	assert.NoError(t, err, "state file must live at <XDG_STATE_HOME>/lfk/cluster-colors.yaml, got: %s", expected)
}

func TestLoadClusterColors_MissingFileReturnsEmpty(t *testing.T) {
	withClusterColorsStateDir(t)
	got := loadClusterColors()
	// NotNil + Empty: callers always range over the returned map, so it
	// must be a real (empty) map and not nil — defends the contract
	// loadClusterColors documents.
	assert.NotNil(t, got, "missing file must yield a real map, not nil")
	assert.Empty(t, got, "missing file should yield an empty map (graceful fallback)")
}

func TestLoadClusterColors_CorruptFileReturnsEmpty(t *testing.T) {
	dir := withClusterColorsStateDir(t)
	path := filepath.Join(dir, "lfk", "cluster-colors.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("this: is: not: yaml: ::"), 0o644))

	got := loadClusterColors()
	assert.NotNil(t, got, "corrupt file must yield a real map, not nil")
	assert.Empty(t, got, "corrupt YAML must not panic and must fall back to empty map")
}

func TestLoadClusterColors_FutureSchemaReturnsEmpty(t *testing.T) {
	dir := withClusterColorsStateDir(t)
	path := filepath.Join(dir, "lfk", "cluster-colors.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`schema_version: 999
contexts:
  prod: red
`), 0o644))

	got := loadClusterColors()
	assert.NotNil(t, got, "future schema must yield a real map, not nil")
	assert.Empty(t, got, "unknown schema version must be ignored so older binaries don't trip on a forward-incompat write")
}

func TestLoadClusterColors_DropsUnknownColors(t *testing.T) {
	dir := withClusterColorsStateDir(t)
	path := filepath.Join(dir, "lfk", "cluster-colors.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`schema_version: 1
contexts:
  prod-eu: red
  weird: chartreuse
  dev: green
`), 0o644))

	got := loadClusterColors()
	assert.Equal(t, map[string]string{
		"prod-eu": "red",
		"dev":     "green",
	}, got, "unknown color names must be dropped silently so a typo doesn't poison every other entry")
}

func TestSaveClusterColors_Atomic(t *testing.T) {
	dir := withClusterColorsStateDir(t)
	require.NoError(t, saveClusterColors(map[string]string{"prod": "red"}))

	// No leftover .tmp file after a successful write.
	tmpPath := filepath.Join(dir, "lfk", "cluster-colors.yaml.tmp")
	_, err := os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), ".tmp sibling must be renamed away on successful save")
}

func TestSaveClusterColors_EmptyMapClearsFile(t *testing.T) {
	withClusterColorsStateDir(t)
	require.NoError(t, saveClusterColors(map[string]string{"prod": "red"}))
	require.NoError(t, saveClusterColors(map[string]string{}))

	got := loadClusterColors()
	assert.Empty(t, got, "saving an empty map should leave nothing for a subsequent load")
}

func TestSaveClusterColors_RejectsUnknownColor(t *testing.T) {
	withClusterColorsStateDir(t)
	err := saveClusterColors(map[string]string{"prod": "chartreuse"})
	assert.Error(t, err, "unknown colors must be rejected at save time so callers can surface a UI error")
}
