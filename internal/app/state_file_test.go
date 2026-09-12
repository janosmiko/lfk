package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stateFileFixture struct {
	Value string `yaml:"value"`
}

func TestLoadStateFile_Missing(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	got := loadStateFile[stateFileFixture]("nope.yaml")

	assert.Equal(t, stateFileFixture{}, got)
}

func TestLoadStateFile_Corrupt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(":::not yaml"), 0o600))

	got := loadStateFile[stateFileFixture]("bad.yaml")

	assert.Equal(t, stateFileFixture{}, got)
}

func TestSaveAndLoadStateFile_RoundTrip(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	want := stateFileFixture{Value: "hello"}

	require.NoError(t, saveStateFile("fixture.yaml", want))
	got := loadStateFile[stateFileFixture]("fixture.yaml")

	assert.Equal(t, want, got)
}

func TestSaveStateFile_Permissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "unmade")
	t.Setenv("LFK_STATE_DIR", dir)

	require.NoError(t, saveStateFile("fixture.yaml", stateFileFixture{Value: "x"}))

	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())

	fileInfo, err := os.Stat(filepath.Join(dir, "fixture.yaml"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())
}
