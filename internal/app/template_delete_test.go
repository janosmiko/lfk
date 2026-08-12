package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// templateModel builds a picker sitting on the first of the given rows.
func templateModel(items []model.ResourceTemplate) Model {
	m := basePush80Model()
	m.templateItems = items
	m.overlay = overlayTemplates
	return m
}

func pressTemplateDeleteKey(m Model) Model {
	mdl, _ := m.handleTemplateOverlayKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	return mdl.(Model)
}

func pressConfirmKey(m Model, key string) Model {
	mdl, _ := m.handleConfirmOverlayKey(tea.KeyPressMsg{Code: rune(key[0]), Text: key})
	return mdl.(Model)
}

func TestDeleteUserTemplate_RemovesFile(t *testing.T) {
	dir := withTempConfigDir(t)
	writeTemplateFile(t, dir, "web.yaml", "kind: Deployment\n")

	require.NoError(t, deleteUserTemplate("web"))

	assert.NoFileExists(t, filepath.Join(dir, "web.yaml"))
	assert.Empty(t, loadUserTemplates())
}

// A hand-authored .yml file is listed by the picker, so it has to be removable
// from the picker too.
func TestDeleteUserTemplate_RemovesYmlFile(t *testing.T) {
	dir := withTempConfigDir(t)
	writeTemplateFile(t, dir, "cron.yml", "kind: CronJob\n")

	require.NoError(t, deleteUserTemplate("cron"))

	assert.NoFileExists(t, filepath.Join(dir, "cron.yml"))
}

func TestDeleteUserTemplate_RejectsPathTraversal(t *testing.T) {
	dir := withTempConfigDir(t)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	outside := filepath.Join(filepath.Dir(dir), "victim.yaml")
	require.NoError(t, os.WriteFile(outside, []byte("kind: Pod\n"), 0o600))

	err := deleteUserTemplate("../victim")

	require.ErrorIs(t, err, errInvalidTemplateName)
	assert.FileExists(t, outside, "a name that escapes the directory must not delete anything")
}

func TestDeleteUserTemplate_ReportsMissingTemplate(t *testing.T) {
	withTempConfigDir(t)

	require.ErrorIs(t, deleteUserTemplate("absent"), errTemplateNotFound)
}

func TestTemplateOverlayKey_DConfirmsUserTemplate(t *testing.T) {
	m := templateModel([]model.ResourceTemplate{
		{Name: "web", Category: userTemplateCategory},
	})

	got := pressTemplateDeleteKey(m)

	assert.Equal(t, overlayConfirm, got.overlay)
	assert.Equal(t, deleteTemplateAction, got.pendingAction)
	assert.Equal(t, "web", got.confirmAction)
	assert.Contains(t, got.confirmQuestion, "web")
}

// A built-in ships with lfk. There is no file to remove, so the key has to say
// so rather than open a confirmation that cannot do anything.
func TestTemplateOverlayKey_DRefusesBuiltinTemplate(t *testing.T) {
	m := templateModel([]model.ResourceTemplate{
		{Name: "Deployment", Category: "Workloads"},
	})

	got := pressTemplateDeleteKey(m)

	assert.Equal(t, overlayTemplates, got.overlay, "the picker stays open")
	assert.Empty(t, got.confirmAction)
	assert.Contains(t, got.statusMessage, "own templates")
}

// The delete is confirmed from the picker, so cancelling has to land back in
// the picker and not on the explorer.
func TestConfirmOverlayKey_CancelledTemplateDeleteReturnsToPicker(t *testing.T) {
	dir := withTempConfigDir(t)
	writeTemplateFile(t, dir, "web.yaml", "kind: Deployment\n")
	m := templateModel([]model.ResourceTemplate{
		{Name: "web", Category: userTemplateCategory},
	})
	m = pressTemplateDeleteKey(m)

	got := pressConfirmKey(m, "n")

	assert.Equal(t, overlayTemplates, got.overlay)
	assert.Empty(t, got.confirmAction)
	assert.FileExists(t, filepath.Join(dir, "web.yaml"), "cancelling keeps the file")
}

func TestConfirmOverlayKey_TemplateDeleteRemovesFileAndRefreshesPicker(t *testing.T) {
	dir := withTempConfigDir(t)
	writeTemplateFile(t, dir, "web.yaml", "kind: Deployment\n")
	m := templateModel(mergedTemplates())
	require.Equal(t, "web", m.templateItems[0].Name, "the user template sorts first")
	m = pressTemplateDeleteKey(m)

	got := pressConfirmKey(m, "y")

	assert.NoFileExists(t, filepath.Join(dir, "web.yaml"))
	assert.Equal(t, overlayTemplates, got.overlay, "the picker reopens so the missing row is the visible result")
	for _, tmpl := range got.templateItems {
		assert.NotEqual(t, userTemplateCategory, tmpl.Category, "the deleted row is gone from the picker")
	}
	assert.False(t, got.loading, "removing a local file starts no cluster work")
	assert.Empty(t, got.confirmAction, "a stale name would title the next confirm")
	assert.Empty(t, got.pendingAction)
}
