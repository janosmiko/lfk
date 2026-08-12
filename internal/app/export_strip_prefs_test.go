package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
)

func TestExportStripPrefsFilePath(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", "")
	t.Setenv("XDG_STATE_HOME", "/custom/state")
	assert.Equal(t, "/custom/state/lfk/export_strip_prefs.yaml", exportStripPrefsFilePath())
}

func TestExportStripPrefs_NoFileYieldsDefaults(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	assert.Equal(t, k8s.DefaultTemplateStripSet(), loadExportStripPrefs())
}

func TestExportStripPrefs_RoundTrip(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	set := k8s.DefaultTemplateStripSet()
	set[k8s.TemplateNamespace] = false
	set[k8s.TemplateLabels] = true
	saveExportStripPrefs(set)

	assert.Equal(t, set, loadExportStripPrefs())
}

// TestExportStripPrefs_PartialFileFallsBackPerCategory: the file is
// user-visible, so a hand-edited or half-written one must degrade to the
// default strip, never to "keep everything".
func TestExportStripPrefs_PartialFileFallsBackPerCategory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", dir)
	path := exportStripPrefsFilePath()
	require.NotEmpty(t, path)
	require.Contains(t, path, dir)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("categories:\n  labels: true\n"), 0o600))

	got := loadExportStripPrefs()
	assert.True(t, got[k8s.TemplateLabels], "the recorded category wins")
	assert.True(t, got[k8s.TemplateHelmOwnership], "an absent category keeps its default")
	assert.True(t, got[k8s.TemplateSecretValues], "an absent category keeps its default")
}

func TestExportStripPrefs_CorruptFileYieldsDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", dir)
	path := exportStripPrefsFilePath()
	require.NotEmpty(t, path)
	require.Contains(t, path, dir)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("categories: [not, a, map\n"), 0o600))

	assert.Equal(t, k8s.DefaultTemplateStripSet(), loadExportStripPrefs())
}

// TestExportStripPrefs_ToggleSurvivesANewExport is the user-visible promise:
// the choice outlives the picker it was made in.
func TestExportStripPrefs_ToggleSurvivesANewExport(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())

	m := pressKey(t, openedExportPicker(t), exportStripKey)
	m = pressKey(t, m, "space")
	require.False(t, m.exportTemplatePicker.strip[k8s.TemplateNamespace])

	next := basePush80Model()
	next.openExportTemplatePicker("settings", "ConfigMap", "prod", exportPickerYAML)

	assert.False(t, next.exportTemplatePicker.strip[k8s.TemplateNamespace])
	assert.Contains(t, next.exportTemplatePicker.manifest, "namespace: prod")
	assert.True(t, next.exportTemplatePicker.strip[k8s.TemplateHelmOwnership],
		"an untouched category is unaffected")
}
