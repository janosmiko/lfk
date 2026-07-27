package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// taintPresetKey drives the preset picker, which owns its own overlay and so
// routes through the overlay dispatcher rather than the editor's handler.
func taintPresetKey(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		mdl, _ := m.handleTaintPresetKey(msg)
		var ok bool
		m, ok = mdl.(Model)
		require.True(t, ok)
	}
	return m
}

func TestTaintEditor_POpensPresetPicker(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "p")

	assert.Equal(t, overlayTaintPresets, m.overlay)
	assert.True(t, m.taintEditor.active, "the editor stays alive underneath")
}

// Selecting a preset fills all three add-row fields and drops the user on the
// key field, so every part stays editable before staging.
func TestTaintPresets_SelectFillsAddRowAndFocusesKey(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "p")
	m = taintPresetKey(t, m, "j", "enter")

	want := model.CommonTaints[1].Taint
	assert.Equal(t, overlayTaintEditor, m.overlay)
	assert.Equal(t, want.Key, m.taintEditor.addKey)
	assert.Equal(t, want.Value, m.taintEditor.addVal)
	assert.Equal(t, want.Effect, model.ValidTaintEffects[m.taintEditor.addEff])
	assert.Equal(t, taintFocusKey, m.taintEditor.focus)
}

// A picked preset is a starting point, not a commitment: it must still go
// through the normal validation and staging path.
func TestTaintPresets_PickedPresetStages(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	before := len(m.taintEditor.rows)
	m = taintKey(t, m, "p")
	m = taintPresetKey(t, m, "enter")
	m = taintKey(t, m, "enter")

	require.Len(t, m.taintEditor.rows, before+1)
	staged := m.taintEditor.rows[before]
	assert.True(t, staged.staged)
	assert.Equal(t, model.CommonTaints[0].Taint.String(), staged.taint.String())
}

// Esc backs out to the editor without adopting the highlighted preset — the
// picker only writes to the add-row on an explicit selection.
func TestTaintPresets_EscAdoptsNothing(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "p")
	m = taintPresetKey(t, m, "j", "esc")

	assert.Equal(t, overlayTaintEditor, m.overlay)
	assert.Empty(t, m.taintEditor.addKey, "esc must not fill the add-row")
	assert.Equal(t, taintFocusList, m.taintEditor.focus)
}

func TestTaintPresets_CursorClampsAtBothEnds(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "p")

	m = taintPresetKey(t, m, "k")
	assert.Equal(t, 0, m.taintEditor.presetCursor, "cursor clamps at the top")

	m = taintPresetKey(t, m, "G")
	assert.Equal(t, len(model.CommonTaints)-1, m.taintEditor.presetCursor)

	m = taintPresetKey(t, m, "j")
	assert.Equal(t, len(model.CommonTaints)-1, m.taintEditor.presetCursor, "cursor clamps at the bottom")

	m = taintPresetKey(t, m, "g")
	assert.Equal(t, 0, m.taintEditor.presetCursor)
}

func TestRenderOverlayTaintPresets_ShowsTaintsAndDescriptions(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "p")

	content, w, h := m.renderOverlayTaintPresets()
	require.NotEmpty(t, content)
	assert.Positive(t, w)
	assert.Positive(t, h)

	plain := stripANSI(content)
	assert.Contains(t, plain, "node-role.kubernetes.io/control-plane:NoSchedule")
	assert.Contains(t, plain, "Reserve control-plane nodes")
}

// The picker's hints must describe the picker, not the editor underneath.
func TestTaintPresets_HintBar(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "p")

	hints := stripANSI(m.overlayHintBar())
	assert.Contains(t, hints, "use taint")
}

// The editor's own hint bar must advertise the picker, or nobody finds it.
func TestTaintEditor_HintBarAdvertisesPresets(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())

	hints := stripANSI(m.overlayHintBar())
	assert.Contains(t, hints, "common taints")
}
