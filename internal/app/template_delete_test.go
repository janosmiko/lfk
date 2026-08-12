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
	got, _ := pressConfirmKeyWithCmd(m, key)
	return got
}

func pressConfirmKeyWithCmd(m Model, key string) (Model, tea.Cmd) {
	mdl, cmd := m.handleConfirmOverlayKey(tea.KeyPressMsg{Code: rune(key[0]), Text: key})
	return mdl.(Model), cmd
}

func TestDeleteUserTemplate_RemovesFile(t *testing.T) {
	dir := withTempConfigDir(t)
	writeTemplateFile(t, dir, "web.yaml", "kind: Deployment\n")

	require.NoError(t, deleteUserTemplate(filepath.Join(dir, "web.yaml")))

	assert.NoFileExists(t, filepath.Join(dir, "web.yaml"))
	assert.Empty(t, loadUserTemplates())
}

// A hand-authored .yml file is listed by the picker, so it has to be removable
// from the picker too.
func TestDeleteUserTemplate_RemovesYmlFile(t *testing.T) {
	dir := withTempConfigDir(t)
	writeTemplateFile(t, dir, "cron.yml", "kind: CronJob\n")

	require.NoError(t, deleteUserTemplate(filepath.Join(dir, "cron.yml")))

	assert.NoFileExists(t, filepath.Join(dir, "cron.yml"))
}

// web.yaml and web.yml are two templates the picker lists as two rows. Deleting
// the row the cursor sits on must not take the other one with it.
func TestTemplateDelete_SameBaseNameKeepsTheOtherFile(t *testing.T) {
	dir := withTempConfigDir(t)
	writeTemplateFile(t, dir, "web.yaml", "kind: Deployment\n")
	writeTemplateFile(t, dir, "web.yml", "kind: Service\n")
	rows := loadUserTemplates()
	require.Len(t, rows, 2)
	require.Equal(t, "Deployment", rows[0].Description, "the .yaml row sorts first")

	m := templateModel(rows)
	got := pressConfirmKey(pressTemplateDeleteKey(m), "y")

	assert.NoFileExists(t, filepath.Join(dir, "web.yaml"), "the selected row is gone")
	assert.FileExists(t, filepath.Join(dir, "web.yml"), "the unselected row survives")
	require.Len(t, got.templateItems, len(model.BuiltinTemplates())+1)
	assert.Equal(t, "Service", got.templateItems[0].Description, "the surviving row is the .yml one")
}

func TestDeleteUserTemplate_RejectsPathTraversal(t *testing.T) {
	dir := withTempConfigDir(t)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	outside := filepath.Join(filepath.Dir(dir), "victim.yaml")
	require.NoError(t, os.WriteFile(outside, []byte("kind: Pod\n"), 0o600))

	err := deleteUserTemplate(filepath.Join(dir, "..", "victim.yaml"))

	require.ErrorIs(t, err, errInvalidTemplateName)
	assert.FileExists(t, outside, "a name that escapes the directory must not delete anything")
}

func TestDeleteUserTemplate_ReportsMissingTemplate(t *testing.T) {
	dir := withTempConfigDir(t)

	require.ErrorIs(t, deleteUserTemplate(filepath.Join(dir, "absent.yaml")), errTemplateNotFound)
}

func TestTemplateOverlayKey_DConfirmsUserTemplate(t *testing.T) {
	path := filepath.Join(withTempConfigDir(t), "web.yaml")
	m := templateModel([]model.ResourceTemplate{
		{Name: "web", Category: userTemplateCategory, Path: path},
	})

	got := pressTemplateDeleteKey(m)

	assert.Equal(t, overlayConfirm, got.overlay)
	assert.Equal(t, deleteTemplateAction, got.pendingAction)
	assert.Equal(t, path, got.confirmAction, "the file, not the name: two files can share one name")
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
		{Name: "web", Category: userTemplateCategory, Path: filepath.Join(dir, "web.yaml")},
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
