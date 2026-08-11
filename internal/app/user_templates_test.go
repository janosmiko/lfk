package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// withTempDataDir points paths.DataDir() at a scratch directory for the test.
func withTempDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LFK_DATA_DIR", dir)
	return dir
}

func TestSaveUserTemplate_RoundTrips(t *testing.T) {
	withTempDataDir(t)

	require.NoError(t, saveUserTemplate(model.ResourceTemplate{
		Name:        "web",
		Description: "Deployment web",
		YAML:        "kind: Deployment\n",
	}))

	got := loadUserTemplates()
	require.Len(t, got, 1)
	assert.Equal(t, "web", got[0].Name)
	assert.Equal(t, "Deployment web", got[0].Description)
	assert.Equal(t, "kind: Deployment\n", got[0].YAML)
	assert.Equal(t, userTemplateCategory, got[0].Category)
}

func TestSaveUserTemplate_ReplacesSameName(t *testing.T) {
	withTempDataDir(t)

	require.NoError(t, saveUserTemplate(model.ResourceTemplate{Name: "web", YAML: "kind: Pod\n"}))
	require.NoError(t, saveUserTemplate(model.ResourceTemplate{Name: "web", YAML: "kind: Deployment\n"}))

	got := loadUserTemplates()
	require.Len(t, got, 1, "a second save under the same name replaces the first")
	assert.Equal(t, "kind: Deployment\n", got[0].YAML)
}

func TestSaveUserTemplate_SortsByName(t *testing.T) {
	withTempDataDir(t)

	for _, n := range []string{"zeta", "alpha", "mid"} {
		require.NoError(t, saveUserTemplate(model.ResourceTemplate{Name: n, YAML: "kind: Pod\n"}))
	}

	got := loadUserTemplates()
	require.Len(t, got, 3)
	assert.Equal(t, []string{"alpha", "mid", "zeta"}, []string{got[0].Name, got[1].Name, got[2].Name})
}

// TestSaveUserTemplate_SanitizesClusterText covers the boundary: a template
// name derived from a resource name is cluster-controlled text, and the picker
// renders it. The escape must be gone in what lands on disk.
func TestSaveUserTemplate_SanitizesClusterText(t *testing.T) {
	withTempDataDir(t)

	require.NoError(t, saveUserTemplate(model.ResourceTemplate{
		Name:        "web\x1b[31mred",
		Description: "kind\x1b]0;title\x07",
		YAML:        "kind: Pod\n",
	}))

	got := loadUserTemplates()
	require.Len(t, got, 1)
	assert.NotContains(t, got[0].Name, "\x1b")
	assert.NotContains(t, got[0].Description, "\x1b")
}

// TestSaveUserTemplate_LeavesNoTempFile guards the atomic write: the temp file
// the save writes before renaming must not survive the call.
func TestSaveUserTemplate_LeavesNoTempFile(t *testing.T) {
	dir := withTempDataDir(t)

	require.NoError(t, saveUserTemplate(model.ResourceTemplate{Name: "web", YAML: "kind: Pod\n"}))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Equal(t, []string{filepath.Base(userTemplatesPath())}, names)
}

// TestMergedTemplates_UserFirstAndNeverShadows is the name-collision rule: a
// user template named after a built-in does not replace it. Both rows stay in
// the picker, the user one first, and the Category column tells them apart.
func TestMergedTemplates_UserFirstAndNeverShadows(t *testing.T) {
	withTempDataDir(t)

	require.NoError(t, saveUserTemplate(model.ResourceTemplate{Name: "Pod", YAML: "kind: Pod # mine\n"}))

	merged := mergedTemplates()
	require.NotEmpty(t, merged)

	assert.Equal(t, "Pod", merged[0].Name)
	assert.Equal(t, userTemplateCategory, merged[0].Category)

	var builtinKept bool
	for _, tm := range merged[1:] {
		if tm.Name == "Pod" && tm.Category != userTemplateCategory {
			builtinKept = true
		}
	}
	assert.True(t, builtinKept, "the built-in Pod template must still be listed")
	assert.Len(t, merged, len(model.BuiltinTemplates())+1)
}

func TestLoadUserTemplates_NoFileReturnsNil(t *testing.T) {
	withTempDataDir(t)
	assert.Nil(t, loadUserTemplates())
}
