package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// prefWriter runs one preference save through its Model path and returns the
// model, so a case can read the status bar afterwards.
type prefWriter struct {
	name string
	file string
	run  func() (*Model, any)
}

func prefWriters() []prefWriter {
	sortModel := func() *Model {
		m := &Model{sortColumnName: "Name", sortAscending: true}
		m.nav.Level = model.LevelResources
		m.nav.Context = "prod"
		m.nav.ResourceType.Resource = "deployments"
		m.nav.ResourceType.Kind = "Deployment"
		return m
	}
	return []prefWriter{
		{"viewer prefs", "viewer_prefs.yaml", func() (*Model, any) {
			m := &Model{}
			return m, m.setViewerPref(prefLogWrap, true)
		}},
		{"column prefs commit", columnPrefsFileName, func() (*Model, any) {
			m := &Model{sessionColumns: map[string][]string{}}
			m.nav.Context = "prod"
			key := m.columnMemoryKey(m.middleColumnKind())
			m.sessionColumns[key] = []string{"cpu"}
			return m, m.persistColumnPrefs()
		}},
		{"sort remember", sortMemoryFileName, func() (*Model, any) {
			m := sortModel()
			return m, m.rememberSort()
		}},
	}
}

// useStateDir points the state dir at a fresh temp dir and returns it.
func useStateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("LFK_STATE_DIR", dir)
	return dir
}

// makeDirReadOnly pre-creates the lock file so the lock still succeeds, then
// blocks every write into dir.
func makeDirReadOnly(t *testing.T, dir, file string) {
	t.Helper()
	lock := filepath.Join(dir, "."+file+".lock")
	require.NoError(t, os.WriteFile(lock, nil, 0o600))
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if f, err := os.Create(filepath.Join(dir, "probe")); err == nil {
		_ = f.Close()
		t.Skip("directory permissions are not enforced here")
	}
}

func assertPrefSaveError(t *testing.T, m *Model, cmd any) {
	t.Helper()
	assert.NotNil(t, cmd, "a failed save must schedule the status clear")
	assert.True(t, m.statusMessageErr)
	assert.Contains(t, m.statusMessage, "Could not save preference")
}

func TestPrefSave_WriteFailureShowsStatusError(t *testing.T) {
	for _, w := range prefWriters() {
		t.Run(w.name, func(t *testing.T) {
			dir := useStateDir(t)
			makeDirReadOnly(t, dir, w.file)
			m, cmd := w.run()
			assertPrefSaveError(t, m, cmd)
		})
	}
}

func TestPrefSave_LockTimeoutShowsStatusError(t *testing.T) {
	for _, w := range prefWriters() {
		t.Run(w.name, func(t *testing.T) {
			dir := useStateDir(t)
			release := holdStateLock(t, filepath.Join(dir, w.file))
			defer release()
			m, cmd := w.run()
			assertPrefSaveError(t, m, cmd)
		})
	}
}

func TestPrefSave_SuccessIsSilent(t *testing.T) {
	for _, w := range prefWriters() {
		t.Run(w.name, func(t *testing.T) {
			useStateDir(t)
			m, cmd := w.run()
			assert.Nil(t, cmd)
			assert.Empty(t, m.statusMessage)
		})
	}
}

func TestForgetSort_FailureShowsStatusError(t *testing.T) {
	dir := useStateDir(t)
	m := &Model{sortColumnName: "Name", sortAscending: true}
	m.nav.Level = model.LevelResources
	m.nav.Context = "prod"
	m.nav.ResourceType.Resource = "deployments"
	require.Nil(t, m.rememberSort())
	release := holdStateLock(t, filepath.Join(dir, sortMemoryFileName))
	defer release()

	cmd := m.forgetSort()
	assertPrefSaveError(t, m, cmd)
}

func TestColumnReset_FailureShowsStatusError(t *testing.T) {
	dir := useStateDir(t)
	m := &Model{sessionColumns: map[string][]string{}}
	m.nav.Context = "prod"
	key := m.columnMemoryKey(m.middleColumnKind())
	m.sessionColumns[key] = []string{"cpu"}
	require.Nil(t, m.persistColumnPrefs())
	delete(m.sessionColumns, key)
	release := holdStateLock(t, filepath.Join(dir, columnPrefsFileName))
	defer release()

	cmd := m.persistColumnPrefs()
	assertPrefSaveError(t, m, cmd)
}

func TestMetricsSparkCycle_SaveFailureShowsStatusError(t *testing.T) {
	dir := useStateDir(t)
	release := holdStateLock(t, filepath.Join(dir, "viewer_prefs.yaml"))
	defer release()

	m := Model{}
	m, cmd := m.handleMetricsSparkCycle()
	assert.NotNil(t, cmd)
	assert.True(t, m.statusMessageErr)
	assert.Contains(t, m.statusMessage, "Could not save preference")
}

func TestHiddenAndPinnedSave_WriteFailureReturnsError(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	require.NoError(t, os.WriteFile(blocked, []byte("x"), 0o600))
	t.Setenv("LFK_STATE_DIR", blocked)

	assert.Error(t, saveHiddenTypesState(newHiddenTypesState()))
	assert.Error(t, savePinnedState(newPinnedState()))
}
