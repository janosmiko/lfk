package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// --- columnPrefsFilePath ---

func TestColumnPrefsFilePath(t *testing.T) {
	t.Run("uses XDG_STATE_HOME", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "/custom/state")
		assert.Equal(t, "/custom/state/lfk/column_prefs.yaml", columnPrefsFilePath())
	})

	t.Run("falls back to home", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")
		assert.Contains(t, columnPrefsFilePath(), filepath.Join(".local", "state", "lfk", "column_prefs.yaml"))
	})
}

// --- save/load round-trip via the delta API ---

func TestColumnPrefsRoundTrip(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	persistColumnPrefsEntry("prod\x00Deployment", persistedColumnPrefs{
		Order:          []string{"Name", "cpu", "Status"},
		VisibleExtras:  []string{"cpu"},
		HiddenBuiltins: []string{"Age"},
	})
	persistColumnPrefsEntry("dev\x00Pod", persistedColumnPrefs{
		Order:         []string{"Name", "Status"},
		VisibleExtras: []string{},
	})

	maps := loadColumnPrefs()
	assert.Equal(t, []string{"cpu"}, maps.sessionColumns["prod\x00Deployment"])
	assert.Equal(t, []string{"Age"}, maps.hiddenBuiltinColumns["prod\x00Deployment"])
	assert.Equal(t, []string{"Name", "cpu", "Status"}, maps.columnOrder["prod\x00Deployment"])

	// "no extras" round-trips as a non-nil empty slice (auto-detect off).
	got, ok := maps.sessionColumns["dev\x00Pod"]
	require.True(t, ok)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestLoadColumnPrefsMissingFile(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	maps := loadColumnPrefs()
	assert.NotNil(t, maps.sessionColumns)
	assert.NotNil(t, maps.hiddenBuiltinColumns)
	assert.NotNil(t, maps.columnOrder)
	assert.Empty(t, maps.sessionColumns)
}

func TestLoadColumnPrefsCorrupt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", dir)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "column_prefs.yaml"), []byte("{bad: ["), 0o644))

	maps := loadColumnPrefs()
	assert.Empty(t, maps.sessionColumns)
	assert.Empty(t, maps.columnOrder)
}

// --- delta persistence merges rather than clobbering ---

func TestPersistColumnPrefsEntryMerges(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	persistColumnPrefsEntry("prod\x00Deployment", persistedColumnPrefs{VisibleExtras: []string{"cpu"}})
	persistColumnPrefsEntry("prod\x00Pod", persistedColumnPrefs{VisibleExtras: []string{"mem"}})

	maps := loadColumnPrefs()
	assert.Equal(t, []string{"cpu"}, maps.sessionColumns["prod\x00Deployment"])
	assert.Equal(t, []string{"mem"}, maps.sessionColumns["prod\x00Pod"])
}

func TestPersistForgottenColumnPrefsRemovesOnlyTarget(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	persistColumnPrefsEntry("prod\x00Deployment", persistedColumnPrefs{VisibleExtras: []string{"cpu"}})
	persistColumnPrefsEntry("prod\x00Pod", persistedColumnPrefs{VisibleExtras: []string{"mem"}})

	persistForgottenColumnPrefs("prod\x00Pod")

	maps := loadColumnPrefs()
	_, hasDep := maps.sessionColumns["prod\x00Deployment"]
	_, hasPod := maps.sessionColumns["prod\x00Pod"]
	assert.True(t, hasDep)
	assert.False(t, hasPod)
}

func TestPersistForgottenColumnPrefsNoEntryIsNoop(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	persistForgottenColumnPrefs("prod\x00Deployment") // nothing persisted yet
	assert.Empty(t, loadColumnPrefs().sessionColumns)
}

// --- model-level commit / reset persistence ---

func TestPersistColumnPrefs_CommitsCurrentKind(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	m := &Model{
		sessionColumns:       map[string][]string{"prod\x00Deployment": {"cpu"}},
		hiddenBuiltinColumns: map[string][]string{"prod\x00Deployment": {"Age"}},
		columnOrder:          map[string][]string{"prod\x00Deployment": {"Name", "cpu"}},
	}
	m.nav.Context = "prod"
	m.nav.ResourceType.Kind = "Deployment"
	m.nav.Level = 0 // forces middleColumnKind to fall back appropriately; overridden below

	// Drive the key explicitly to avoid depending on middleColumnKind internals.
	key := m.columnMemoryKey(m.middleColumnKind())
	m.sessionColumns[key] = m.sessionColumns["prod\x00Deployment"]
	m.hiddenBuiltinColumns[key] = m.hiddenBuiltinColumns["prod\x00Deployment"]
	m.columnOrder[key] = m.columnOrder["prod\x00Deployment"]

	m.persistColumnPrefs()

	maps := loadColumnPrefs()
	assert.Equal(t, []string{"cpu"}, maps.sessionColumns[key])
	assert.Equal(t, []string{"Age"}, maps.hiddenBuiltinColumns[key])
	assert.Equal(t, []string{"Name", "cpu"}, maps.columnOrder[key])
}

// TestColumnToggleEnterPersists drives the real Enter-commit handler and
// verifies the committed layout reaches disk and survives a reload (the
// restart path).
func TestColumnToggleEnterPersists(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	m := baseModelCov()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}
	m.nav.Context = "prod"
	m.columnToggleItems = []columnToggleEntry{
		{key: "Name", visible: true, builtin: true},
		{key: "Ready", visible: false, builtin: true},
		{key: "IP", visible: true, builtin: false},
	}

	m.handleColumnToggleKeyEnter()

	key := "prod\x00pod"
	maps := loadColumnPrefs()
	assert.Equal(t, []string{"IP"}, maps.sessionColumns[key])
	assert.Equal(t, []string{"Ready"}, maps.hiddenBuiltinColumns[key])
	assert.Equal(t, []string{"Name", "IP"}, maps.columnOrder[key])
}

// TestColumnToggleResetPersists verifies R drops the persisted layout so a
// reset survives a restart.
func TestColumnToggleResetPersists(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	persistColumnPrefsEntry("prod\x00pod", persistedColumnPrefs{VisibleExtras: []string{"IP"}})

	m := baseModelCov()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}
	m.nav.Context = "prod"
	// Seed the in-memory entry so R has something to clear.
	m.sessionColumns = map[string][]string{"prod\x00pod": {"IP"}}

	m.handleColumnToggleKeyR()

	assert.Empty(t, loadColumnPrefs().sessionColumns)
}

func TestPersistColumnPrefs_ResetRemovesEntry(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	m := &Model{
		sessionColumns:       map[string][]string{},
		hiddenBuiltinColumns: map[string][]string{},
		columnOrder:          map[string][]string{},
	}
	m.nav.Context = "prod"
	m.nav.ResourceType.Kind = "Deployment"
	key := m.columnMemoryKey(m.middleColumnKind())

	// Commit then clear (mirrors the overlay's R reset path: maps emptied first).
	persistColumnPrefsEntry(key, persistedColumnPrefs{VisibleExtras: []string{"cpu"}})
	m.persistColumnPrefs() // sessionColumns has no key -> forgotten

	assert.Empty(t, loadColumnPrefs().sessionColumns)
}
