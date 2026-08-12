package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// withTempConfigDir points paths.ConfigDir() at a scratch directory and returns
// the template directory inside it.
func withTempConfigDir(t *testing.T) string {
	t.Helper()
	t.Setenv("LFK_CONFIG_DIR", t.TempDir())
	return userTemplateDir()
}

// writeTemplateFile drops a file straight into the template directory, the way
// a user hand-authoring a template would.
func writeTemplateFile(t *testing.T, dir, filename, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(body), 0o600))
}

func TestSaveUserTemplate_RoundTrips(t *testing.T) {
	dir := withTempConfigDir(t)

	require.NoError(t, saveUserTemplate("web", "apiVersion: apps/v1\nkind: Deployment\n"))

	assert.FileExists(t, filepath.Join(dir, "web.yaml"))

	got := loadUserTemplates()
	require.Len(t, got, 1)
	assert.Equal(t, "web", got[0].Name)
	assert.Equal(t, userTemplateCategory, got[0].Category)
	assert.Equal(t, "apiVersion: apps/v1\nkind: Deployment\n", got[0].YAML)
	assert.Contains(t, got[0].Description, "Deployment")
}

// TestLoadUserTemplates_ReadsHandAuthoredFile is the point of the directory:
// a file the user drops in shows up next to the built-ins with no export step.
func TestLoadUserTemplates_ReadsHandAuthoredFile(t *testing.T) {
	dir := withTempConfigDir(t)
	writeTemplateFile(t, dir, "my-cronjob.yml", "apiVersion: batch/v1\nkind: CronJob\n")

	got := loadUserTemplates()
	require.Len(t, got, 1)
	assert.Equal(t, "my-cronjob", got[0].Name, "the file name is the template name")
	assert.Contains(t, got[0].Description, "CronJob")
}

func TestSaveUserTemplate_ReplacesSameName(t *testing.T) {
	dir := withTempConfigDir(t)

	require.NoError(t, saveUserTemplate("web", "kind: Pod\n"))
	require.NoError(t, saveUserTemplate("web", "kind: Deployment\n"))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "a second save under the same name replaces the file")

	got := loadUserTemplates()
	require.Len(t, got, 1)
	assert.Equal(t, "kind: Deployment\n", got[0].YAML)
}

// TestLoadUserTemplates_SkipsMalformedFile is the hand-editing guard: one file
// with broken YAML must not empty the picker or hide its healthy siblings.
func TestLoadUserTemplates_SkipsMalformedFile(t *testing.T) {
	dir := withTempConfigDir(t)
	writeTemplateFile(t, dir, "good.yaml", "kind: Pod\n")
	writeTemplateFile(t, dir, "broken.yaml", "kind: Pod\n  bad: [indent\n")
	writeTemplateFile(t, dir, "notamap.yaml", "- just\n- a\n- list\n")

	got := loadUserTemplates()
	require.Len(t, got, 1)
	assert.Equal(t, "good", got[0].Name)
}

func TestLoadUserTemplates_IgnoresNonYAMLEntries(t *testing.T) {
	dir := withTempConfigDir(t)
	writeTemplateFile(t, dir, "good.yaml", "kind: Pod\n")
	writeTemplateFile(t, dir, "README.md", "not a template\n")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested"), 0o700))

	got := loadUserTemplates()
	require.Len(t, got, 1)
	assert.Equal(t, "good", got[0].Name)
}

func TestLoadUserTemplates_MissingDirIsNotAnError(t *testing.T) {
	withTempConfigDir(t)
	assert.Nil(t, loadUserTemplates())
	assert.Len(t, mergedTemplates(), len(model.BuiltinTemplates()))
}

// TestLoadUserTemplates_SanitizesUserSuppliedText covers the boundary: a
// template pasted from the internet, and a file name chosen by whoever wrote
// it, both reach the picker as rendered text.
func TestLoadUserTemplates_SanitizesUserSuppliedText(t *testing.T) {
	dir := withTempConfigDir(t)
	writeTemplateFile(t, dir, "web\x1b[31mred.yaml", "kind: \"Pod\\e[31m\"\n")

	got := loadUserTemplates()
	require.Len(t, got, 1)
	assert.NotContains(t, got[0].Name, "\x1b")
	assert.NotContains(t, got[0].Description, "\x1b")
}

// TestSaveUserTemplate_LeavesNoTempFile guards the atomic write: the temp file
// written before the rename must not survive the call.
func TestSaveUserTemplate_LeavesNoTempFile(t *testing.T) {
	dir := withTempConfigDir(t)

	require.NoError(t, saveUserTemplate("web", "kind: Pod\n"))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Equal(t, []string{"web.yaml"}, names)
}

// TestSaveUserTemplate_RejectsPathEscape keeps a resource name from steering
// the write outside the template directory.
func TestSaveUserTemplate_RejectsPathEscape(t *testing.T) {
	dir := withTempConfigDir(t)

	for _, name := range []string{"../escape", "sub/web", "..", ""} {
		assert.Error(t, saveUserTemplate(name, "kind: Pod\n"), "name %q must be rejected", name)
	}
	assert.NoDirExists(t, filepath.Join(dir, "sub"))
	assert.NoFileExists(t, filepath.Join(filepath.Dir(dir), "escape.yaml"))
}

// TestSaveUserTemplate_ConfigDirUnavailableIsDistinctError guards against
// blaming the resource name for an environment problem: when
// paths.ConfigDir() cannot resolve (no override env vars, no home directory),
// saveUserTemplate must report errTemplateDirUnavailable, not
// errInvalidTemplateName.
func TestSaveUserTemplate_ConfigDirUnavailableIsDistinctError(t *testing.T) {
	t.Setenv("LFK_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	err := saveUserTemplate("web", "kind: Pod\n")
	require.Error(t, err)
	assert.ErrorIs(t, err, errTemplateDirUnavailable)
	assert.NotErrorIs(t, err, errInvalidTemplateName)
}

// TestMergedTemplates_UserFirstAndNeverShadows is the name-collision rule: a
// user template named after a built-in does not replace it. Both rows stay in
// the picker, the user one first, and the Category column tells them apart.
func TestMergedTemplates_UserFirstAndNeverShadows(t *testing.T) {
	dir := withTempConfigDir(t)
	writeTemplateFile(t, dir, "Pod.yaml", "kind: Pod # mine\n")

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
