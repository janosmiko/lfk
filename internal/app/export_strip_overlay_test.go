package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
)

const exportPickerYAML = `apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
  namespace: prod
  labels:
    app: web
data:
  key: value
`

// openedExportPicker is the state the two-keystroke path lands in: the manifest
// fetched and the destination picker up.
func openedExportPicker(t *testing.T) Model {
	t.Helper()
	// Toggling a category persists; keep it out of the developer's real state dir.
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	m := basePush80Model()
	m.openExportTemplatePicker("settings", "ConfigMap", exportPickerYAML)
	require.Equal(t, overlayExportTemplate, m.overlay)
	return m
}

func pressKey(t *testing.T, m Model, key string) Model {
	t.Helper()
	mdl, _ := m.handleOverlayKey(tea.KeyPressMsg{Code: []rune(key)[0], Text: key})
	got, ok := mdl.(Model)
	require.True(t, ok)
	return got
}

func TestExportStripOverlay_DefaultPathStaysTwoKeystrokes(t *testing.T) {
	m := openedExportPicker(t)

	want, err := k8s.StripToTemplate(exportPickerYAML)
	require.NoError(t, err)
	assert.Equal(t, want, m.exportTemplatePicker.manifest,
		"a user who never opens the field picker sees the default strip")
}

func TestExportStripOverlay_OpensFromDestinationPicker(t *testing.T) {
	m := pressKey(t, openedExportPicker(t), exportStripKey)

	assert.Equal(t, overlayExportStrip, m.overlay)
	assert.Equal(t, overlayExportTemplate, m.previousOverlay,
		"the field picker is a detour from the destination picker, not a step before it")
}

func TestExportStripOverlay_EscReturnsToDestinationPicker(t *testing.T) {
	m := pressKey(t, openedExportPicker(t), exportStripKey)
	mdl, _ := m.handleOverlayKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	m, ok := mdl.(Model)
	require.True(t, ok)

	assert.Equal(t, overlayExportTemplate, m.overlay)
	assert.Equal(t, "settings", m.exportTemplatePicker.name, "the captured manifest survives the detour")
}

func TestExportStripOverlay_UntickingNamespaceRestripsTheManifest(t *testing.T) {
	m := pressKey(t, openedExportPicker(t), exportStripKey)
	require.Equal(t, k8s.TemplateNamespace, k8s.TemplateCategories[0])
	require.NotContains(t, m.exportTemplatePicker.manifest, "namespace: prod")

	m = pressKey(t, m, " ")

	assert.False(t, m.exportTemplatePicker.strip[k8s.TemplateNamespace])
	assert.Contains(t, m.exportTemplatePicker.manifest, "namespace: prod",
		"the manifest must be re-stripped, not left as captured")
	assert.Contains(t, m.exportTemplatePicker.manifest, "app: web",
		"an author-written sibling still survives")
}

func TestExportStripOverlay_CursorSkipsLockedRows(t *testing.T) {
	m := pressKey(t, openedExportPicker(t), exportStripKey)
	for range len(k8s.TemplateCategories) + 3 {
		m = pressKey(t, m, "j")
	}

	assert.Less(t, m.exportTemplatePicker.stripCursor, len(k8s.TemplateCategories),
		"the cursor must never land on a locked row")
}

func TestExportStripOverlay_ViewShowsLockedRowsAndReason(t *testing.T) {
	m := pressKey(t, openedExportPicker(t), exportStripKey)
	view := stripANSI(m.View().Content)

	assert.Contains(t, view, "Finalizers")
	assert.Contains(t, view, "Controller-generated labels")
	assert.Contains(t, view, exportStripLockedNote,
		"the user must be told why the locked rows offer no choice")
	assert.Contains(t, view, "Helm ownership")
}

func TestExportTemplateHints_AdvertiseTheFieldPicker(t *testing.T) {
	keys := make([]string, 0, len(exportTemplateHints()))
	for _, h := range exportTemplateHints() {
		keys = append(keys, h.Key)
	}
	assert.Contains(t, keys, exportStripKey,
		"an overlay key nobody can discover is the same bug as no key at all")
}

// TestExportStripKey_DoesNotCollideWithDestinationPicker guards the TASK-891
// class of bug: a hosted key equal to one the host handler consumes first can
// never fire.
func TestExportStripKey_DoesNotCollideWithDestinationPicker(t *testing.T) {
	consumed := make([]string, 0, 10+len(exportDestinations))
	consumed = append(consumed, "esc", "q", "enter", "j", "k", "up", "down", "ctrl+n", "ctrl+p", "ctrl+c")
	for _, d := range exportDestinations {
		consumed = append(consumed, d.ShortcutKey())
	}
	for _, key := range consumed {
		assert.NotEqual(t, key, exportStripKey,
			"the field-picker key is consumed by handleExportTemplateKey before it can open the overlay")
	}
}

func TestExportStripOverlay_HintBarCarriesTheHotkeys(t *testing.T) {
	m := pressKey(t, openedExportPicker(t), exportStripKey)
	hints := stripANSI(m.overlayHintBar())

	for _, want := range []string{"space", "esc"} {
		assert.True(t, strings.Contains(hints, want), "hint bar is missing %q: %q", want, hints)
	}
}
