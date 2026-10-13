package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

func exportStripWriter() prefWriter {
	return prefWriter{"export strip", exportStripPrefsFileName, func() (*Model, any) {
		m := &Model{}
		m.exportTemplatePicker.strip = k8s.DefaultTemplateStripSet()
		return m, m.exportStripToggle()
	}}
}

// mutatingWriters are the writers that merge into an existing file.
func mutatingWriters() []prefWriter {
	return append(prefWriters(), exportStripWriter())
}

func seedPrefFile(t *testing.T, dir, file string, content string) string {
	t.Helper()
	path := filepath.Join(dir, file)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestPrefSave_CorruptFileIsNeverOverwritten(t *testing.T) {
	const corrupt = "contexts: [unterminated\n\t- {{{"
	for _, w := range mutatingWriters() {
		t.Run(w.name, func(t *testing.T) {
			dir := useStateDir(t)
			path := seedPrefFile(t, dir, w.file, corrupt)
			m, cmd := w.run()
			assertPrefSaveError(t, m, cmd)
			assert.Contains(t, m.statusMessage, w.file)
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, corrupt, string(got))
		})
	}
}

func TestPrefSave_UnreadableFileIsNeverOverwritten(t *testing.T) {
	const valid = "contexts:\n  prod:\n    Other:\n      visible_extras: [x]\n"
	for _, w := range mutatingWriters() {
		t.Run(w.name, func(t *testing.T) {
			dir := useStateDir(t)
			path := seedPrefFile(t, dir, w.file, valid)
			require.NoError(t, os.Chmod(path, 0o000))
			t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
			if _, err := os.ReadFile(path); err == nil {
				t.Skip("file permissions are not enforced here")
			}
			m, cmd := w.run()
			assertPrefSaveError(t, m, cmd)
			assert.Contains(t, m.statusMessage, w.file)
			require.NoError(t, os.Chmod(path, 0o600))
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, valid, string(got))
		})
	}
}

func TestExportStripSave_Failures(t *testing.T) {
	w := exportStripWriter()

	t.Run("write failure", func(t *testing.T) {
		dir := useStateDir(t)
		makeDirReadOnly(t, dir, w.file)
		m, cmd := w.run()
		assertPrefSaveError(t, m, cmd)
	})
	t.Run("lock timeout", func(t *testing.T) {
		dir := useStateDir(t)
		release := holdStateLock(t, filepath.Join(dir, w.file))
		defer release()
		m, cmd := w.run()
		assertPrefSaveError(t, m, cmd)
	})
	t.Run("success is silent", func(t *testing.T) {
		useStateDir(t)
		m, cmd := w.run()
		assert.Nil(t, cmd)
		assert.Empty(t, m.statusMessage)
	})
}

func TestPrefSave_NoStateDirReportsError(t *testing.T) {
	for _, w := range append(prefWriters(), exportStripWriter()) {
		t.Run(w.name, func(t *testing.T) {
			t.Setenv("LFK_STATE_DIR", "")
			t.Setenv("XDG_STATE_HOME", "")
			t.Setenv("HOME", "")
			if stateFilePath("x") != "" {
				t.Skip("state dir still resolves")
			}
			m, cmd := w.run()
			assertPrefSaveError(t, m, cmd)
		})
	}
}

func TestSaveStateFile_NoStateDirReturnsError(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")
	if stateFilePath("x") != "" {
		t.Skip("state dir still resolves")
	}
	assert.Error(t, savePinnedState(newPinnedState()))
}

func TestToggleObjectExplorerTree_SaveErrorAndTruncationShowTogether(t *testing.T) {
	dir := useStateDir(t)
	release := holdStateLock(t, filepath.Join(dir, "viewer_prefs.yaml"))
	defer release()

	root := make(map[string]any, model.DefaultObjectTreeRowLimit+1)
	for i := range model.DefaultObjectTreeRowLimit + 1 {
		root[fmt.Sprintf("k%d", i)] = i
	}
	m := Model{}
	m.objectExplorerView.root = root

	mdl, cmd := m.toggleObjectExplorerTree()
	got := mdl.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, got.statusMessageErr)
	assert.Contains(t, got.statusMessage, "Could not save preference")
	assert.Contains(t, got.statusMessage, "Tree truncated")
	assert.Less(t, strings.Index(got.statusMessage, "Could not save"), strings.Index(got.statusMessage, "Tree truncated"))
}
