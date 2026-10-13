package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

const corruptStateFile = "contexts: [unterminated\n\t- {{{"

func writeCorrupt(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(corruptStateFile), 0o640))
	return path
}

func assertUntouched(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, corruptStateFile, string(data))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
}

func TestStateWriters_CorruptFileIsNeverOverwritten(t *testing.T) {
	cases := []struct {
		name string
		file string
		save func() error
	}{
		{"bookmarks", "bookmarks.yaml", func() error {
			return saveBookmarks([]model.Bookmark{{Slot: "a", Name: "alpha"}})
		}},
		{"cluster colors", clusterColorsFileName, func() error {
			return saveClusterColors(map[string]string{"prod": ui.ClusterColorNames[0]})
		}},
		{"security ignores", securityIgnoresFileName, func() error {
			return saveSecurityIgnores(&SecurityIgnoreState{Contexts: map[string][]SecurityIgnoreRule{
				"prod": {{Source: "heuristic", GroupKey: "k"}},
			}})
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := useStateDir(t)
			path := writeCorrupt(t, dir, c.file)

			err := c.save()

			require.Error(t, err)
			assert.Contains(t, err.Error(), path)
			assertUntouched(t, path)
			_, statErr := os.Stat(path + ".bak")
			assert.True(t, os.IsNotExist(statErr), "no backup of the corrupt file")
		})
	}
}

func TestBookmarkDelete_SaveErrorIsShown(t *testing.T) {
	cases := map[string]func(m *Model){
		"delete current": func(m *Model) { m.bookmarkDeleteCurrent() },
		"delete all":     func(m *Model) { m.bookmarkDeleteAll() },
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			dir := useStateDir(t)
			path := writeCorrupt(t, dir, "bookmarks.yaml")
			m := baseFinalModel()
			m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}, {Name: "bm2", Slot: "b"}}

			run(&m)

			assert.True(t, m.statusMessageErr)
			assert.Contains(t, m.statusMessage, path)
			assertUntouched(t, path)
		})
	}
}

func TestBookmarkAdd_SaveErrorRollsBack(t *testing.T) {
	dir := useStateDir(t)
	path := writeCorrupt(t, dir, "bookmarks.yaml")
	m := baseFinalModel()
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}

	res, _ := m.saveBookmark(model.Bookmark{Name: "bm2", Slot: "b"})

	got := res.(Model)
	assert.Equal(t, []model.Bookmark{{Name: "bm1", Slot: "a"}}, got.bookmarks)
	assert.True(t, got.statusMessageErr)
	assert.Contains(t, got.statusMessage, path)
	assertUntouched(t, path)
}

func TestSecurityIgnoreSaveErr_ShowsReason(t *testing.T) {
	m := baseFinalModel()
	res, _ := m.updateSecurityIgnoresSaveErr(securityIgnoresSaveErrMsg{err: errors.New("/state/x.yaml is corrupt")})
	got := res.(Model)
	assert.True(t, got.statusMessageErr)
	assert.Contains(t, got.statusMessage, "/state/x.yaml is corrupt")
}
