package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

const strippedManifest = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: web\n"

func pickerModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.openExportTemplatePicker("web", "Pod", strippedManifest, false)
	return m
}

// secretPickerModel is the picker as it opens for a Secret, whose values
// StripToTemplate blanked.
func secretPickerModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.openExportTemplatePicker("db-creds", "Secret", "kind: Secret\ndata:\n  password: \"\"\n", true)
	return m
}

// TestExecuteAction_ExportTemplateAllowedInReadOnly is AC #6: the export reads
// the object and writes locally, so read-only mode must not block it.
func TestExecuteAction_ExportTemplateAllowedInReadOnly(t *testing.T) {
	assert.False(t, isMutatingAction(model.ActionLabelExportTemplate),
		"Export Template writes nothing to the cluster")

	m := basePush80Model()
	m.readOnly = true
	m.actionCtx = actionContext{kind: "Pod", name: "web", namespace: "default", context: "test-ctx"}

	ret, _ := m.executeAction(model.ActionLabelExportTemplate)
	assert.NotEqual(t, readOnlyBlockedMessage(model.ActionLabelExportTemplate), ret.(Model).statusMessage)
}

func TestUpdateExportTemplateReady_OpensDestinationPicker(t *testing.T) {
	m := basePush80Model()

	ret, _ := m.updateExportTemplateReady(exportTemplateReadyMsg{
		name: "web", kind: "Pod", manifest: strippedManifest,
	})
	got := ret.(Model)

	assert.Equal(t, overlayExportTemplate, got.overlay)
	assert.True(t, got.exportTemplatePicker.active)
	assert.Equal(t, strippedManifest, got.exportTemplatePicker.manifest)
}

func TestUpdateExportTemplateReady_ErrorDoesNotOpenPicker(t *testing.T) {
	m := basePush80Model()

	ret, _ := m.updateExportTemplateReady(exportTemplateReadyMsg{err: errors.New("boom")})
	got := ret.(Model)

	assert.NotEqual(t, overlayExportTemplate, got.overlay)
	assert.True(t, got.statusMessageErr)
}

// TestApplyExportTemplatePicker_TemplateListSavesFile is AC #5's third
// destination: the manifest lands in the same directory a hand-authored
// template would.
func TestApplyExportTemplatePicker_TemplateListSavesFile(t *testing.T) {
	dir := withTempConfigDir(t)
	m := pickerModel(t)
	m.exportTemplatePicker.cursor = indexOfDestination(t, exportDestTemplateList)

	ret, cmd := m.applyExportTemplatePicker()
	require.NotNil(t, cmd)
	assert.False(t, ret.(Model).exportTemplatePicker.active, "the picker closes on apply")

	msg, ok := cmd().(actionResultMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)

	saved, err := os.ReadFile(filepath.Join(dir, "web.yaml"))
	require.NoError(t, err)
	assert.Equal(t, strippedManifest, string(saved))
}

// TestApplyExportTemplatePicker_FileWritesStrippedManifest covers AC #5's file
// destination, including the ".template" marker that keeps it apart from the
// existing "Save to file" export of the live object.
func TestApplyExportTemplatePicker_FileWritesStrippedManifest(t *testing.T) {
	t.Chdir(t.TempDir())
	m := pickerModel(t)
	m.exportTemplatePicker.cursor = indexOfDestination(t, exportDestFile)

	_, cmd := m.applyExportTemplatePicker()
	require.NotNil(t, cmd)

	msg, ok := cmd().(exportDoneMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	assert.Equal(t, "pod_web.template.yaml", filepath.Base(msg.path))

	written, err := os.ReadFile(msg.path)
	require.NoError(t, err)
	assert.Equal(t, strippedManifest, string(written))
}

// TestHandleExportTemplateKey_ShortcutAppliesThatDestination asserts the
// letter chips act on their own row rather than on wherever the cursor sits.
func TestHandleExportTemplateKey_ShortcutAppliesThatDestination(t *testing.T) {
	dir := withTempConfigDir(t)
	m := pickerModel(t)

	ret, cmd := m.handleExportTemplateKey(tea.KeyPressMsg{Code: 't', Text: "t"})
	require.NotNil(t, cmd)
	assert.False(t, ret.(Model).exportTemplatePicker.active)
	cmd()

	assert.FileExists(t, filepath.Join(dir, "web.yaml"))
}

func TestHandleExportTemplateKey_EscCloses(t *testing.T) {
	m := pickerModel(t)

	ret, _ := m.handleExportTemplateKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	got := ret.(Model)

	assert.False(t, got.exportTemplatePicker.active)
	assert.Equal(t, overlayNone, got.overlay)
}

func TestExportTemplateStep_WrapsBothWays(t *testing.T) {
	m := pickerModel(t)

	m.exportTemplateStep(-1)
	assert.Equal(t, len(exportDestinations)-1, m.exportTemplatePicker.cursor)
	m.exportTemplateStep(1)
	assert.Equal(t, 0, m.exportTemplatePicker.cursor)
}

// TestRenderOverlayExportTemplate_ListsEveryDestination is AC #5: all three
// choices are on screen, with their chips.
func TestRenderOverlayExportTemplate_ListsEveryDestination(t *testing.T) {
	m := pickerModel(t)

	view := stripANSI(mustRender(t, m))
	for _, d := range exportDestinations {
		assert.Contains(t, view, d.Label())
	}
	assert.Contains(t, view, "web")
}

func mustRender(t *testing.T, m Model) string {
	t.Helper()
	content, _, _ := m.renderOverlayExportTemplate()
	require.NotEmpty(t, content)
	return content
}

func indexOfDestination(t *testing.T, want exportDestination) int {
	t.Helper()
	for i, d := range exportDestinations {
		if d == want {
			return i
		}
	}
	t.Fatalf("destination %v is not in the picker", want)
	return 0
}

// TestApplyExportTemplatePicker_SecretRedactionIsAnnounced: every destination
// says the values were blanked. Discovering it by pasting an empty template is
// the failure this guards.
func TestApplyExportTemplatePicker_SecretRedactionIsAnnounced(t *testing.T) {
	t.Run("clipboard", func(t *testing.T) {
		m := secretPickerModel(t)
		m.exportTemplatePicker.cursor = indexOfDestination(t, exportDestClipboard)

		ret, _ := m.applyExportTemplatePicker()
		assert.Contains(t, ret.(Model).statusMessage, "Secret values redacted")
	})

	t.Run("file", func(t *testing.T) {
		t.Chdir(t.TempDir())
		m := secretPickerModel(t)
		m.exportTemplatePicker.cursor = indexOfDestination(t, exportDestFile)

		_, cmd := m.applyExportTemplatePicker()
		require.NotNil(t, cmd)
		msg, ok := cmd().(exportDoneMsg)
		require.True(t, ok)

		ret, _ := basePush80Model().updateExportDone(msg)
		assert.Contains(t, ret.(Model).statusMessage, "Secret values redacted")
	})

	t.Run("template list", func(t *testing.T) {
		withTempConfigDir(t)
		m := secretPickerModel(t)
		m.exportTemplatePicker.cursor = indexOfDestination(t, exportDestTemplateList)

		_, cmd := m.applyExportTemplatePicker()
		require.NotNil(t, cmd)
		msg, ok := cmd().(actionResultMsg)
		require.True(t, ok)
		assert.Contains(t, msg.message, "Secret values redacted")
	})
}

// TestApplyExportTemplatePicker_NoNoteForOtherKinds keeps the note off the
// kinds whose values pass through untouched.
func TestApplyExportTemplatePicker_NoNoteForOtherKinds(t *testing.T) {
	m := pickerModel(t)
	m.exportTemplatePicker.cursor = indexOfDestination(t, exportDestClipboard)

	ret, _ := m.applyExportTemplatePicker()
	assert.NotContains(t, ret.(Model).statusMessage, "redacted")
}
