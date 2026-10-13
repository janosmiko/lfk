package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

type wholeSaveCase struct {
	name string
	file string
	run  func(m Model) Model
}

func wholeSaveCases() []wholeSaveCase {
	cursorOnCronJobs := func(m *Model) { m.setCursor(cursorIndexOfItem(m, "Gadgets")) }
	return []wholeSaveCase{
		{"hidden types", hiddenTypesFileName, func(m Model) Model {
			cursorOnCronJobs(&m)
			res, _ := m.toggleHiddenResourceType()
			return res.(Model)
		}},
		{"pinned types", pinnedFileName, func(m Model) Model {
			cursorOnCronJobs(&m)
			res, _ := m.handleKeyPinGroup()
			return res.(Model)
		}},
		{"pinned summaries", pinnedSummariesFileName, func(m Model) Model {
			cursorOnCronJobs(&m)
			res, _ := m.togglePinnedSummary()
			return res.(Model)
		}},
	}
}

func wholeSaveModel(t *testing.T) (Model, string) {
	t.Helper()
	origHidden, origPinned := model.HiddenTypes, model.PinnedTypes
	t.Cleanup(func() { model.HiddenTypes, model.PinnedTypes = origHidden, origPinned })
	m := seedSummaryTestModel(t, []model.ResourceTypeEntry{
		{Kind: "Gadget", APIGroup: "example.com", APIVersion: "v1", Resource: "gadgets"},
	})
	m.pinnedState = newPinnedState()
	dir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", dir)
	return m, dir
}

func TestWholeStateSave_CorruptFileIsNeverOverwritten(t *testing.T) {
	const corrupt = "contexts: [unterminated\n\t- {{{"
	for _, c := range wholeSaveCases() {
		t.Run(c.name, func(t *testing.T) {
			m, dir := wholeSaveModel(t)
			path := filepath.Join(dir, c.file)
			require.NoError(t, os.WriteFile(path, []byte(corrupt), 0o640))

			got := c.run(m)

			assert.True(t, got.statusMessageErr)
			assert.Contains(t, got.statusMessage, "Failed to save")
			assert.Contains(t, got.statusMessage, path)
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, corrupt, string(data))
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
		})
	}
}

func TestWholeStateSave_UnreadableFileIsNeverOverwritten(t *testing.T) {
	const valid = "contexts:\n  prod:\n    - batch/cronjobs\n"
	for _, c := range wholeSaveCases() {
		t.Run(c.name, func(t *testing.T) {
			m, dir := wholeSaveModel(t)
			path := filepath.Join(dir, c.file)
			require.NoError(t, os.WriteFile(path, []byte(valid), 0o600))
			require.NoError(t, os.Chmod(path, 0o000))
			t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
			if _, err := os.ReadFile(path); err == nil {
				t.Skip("file permissions are not enforced here")
			}

			got := c.run(m)

			assert.True(t, got.statusMessageErr)
			assert.Contains(t, got.statusMessage, "Failed to save")
			assert.Contains(t, got.statusMessage, path)
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o000), info.Mode().Perm())
			require.NoError(t, os.Chmod(path, 0o600))
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, valid, string(data))
		})
	}
}

func TestWholeStateSave_MissingFileSaves(t *testing.T) {
	for _, c := range wholeSaveCases() {
		t.Run(c.name, func(t *testing.T) {
			m, dir := wholeSaveModel(t)
			got := c.run(m)
			assert.False(t, got.statusMessageErr, got.statusMessage)
			assert.FileExists(t, filepath.Join(dir, c.file))
		})
	}
}

func TestStateSaveErrors_NameTheFullPath(t *testing.T) {
	dir := useStateDir(t)
	path := filepath.Join(dir, viewerPrefsFileName)

	release := holdStateLock(t, path)
	err := persistViewerPref(prefLogWrap, true)
	release()
	require.Error(t, err)
	assert.True(t, errors.Is(err, errStateUnavailable))
	assert.Contains(t, err.Error(), path)

	require.NoError(t, os.WriteFile(path, []byte("{{{"), 0o600))
	err = persistViewerPref(prefLogWrap, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
}
