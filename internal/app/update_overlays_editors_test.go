package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- multi-row selection + Shift+Y format picker ---

func TestSecretEditor_SpaceTogglesSelectionAndAdvanceCursor(t *testing.T) {
	data := &model.SecretData{
		Keys: []string{"k1", "k2", "k3"},
		Data: map[string]string{"k1": "v1", "k2": "v2", "k3": "v3"},
	}
	m := Model{
		overlay:      overlaySecretEditor,
		secretData:   data,
		secretCursor: 0,
		tabs:         []TabState{{}},
		width:        80, height: 40,
	}

	// First Space toggles k1 ON, cursor advances to k2.
	ret, _ := m.handleSecretEditorKey(specialKey(tea.KeySpace))
	r1 := ret.(Model)
	assert.True(t, r1.editorSearch.selected["k1"], "first Space selects k1")
	assert.Equal(t, 1, r1.secretCursor, "cursor advances so spamming Space checks consecutive rows")

	// Second Space toggles k2 ON, cursor advances to k3.
	ret, _ = r1.handleSecretEditorKey(specialKey(tea.KeySpace))
	r2 := ret.(Model)
	assert.True(t, r2.editorSearch.selected["k2"], "k2 selected")
	assert.True(t, r2.editorSearch.selected["k1"], "k1 still selected — toggles persist across cursor moves")

	// Move back and toggle k1 OFF — set should drop the entry, not
	// keep it as "false".
	r2.secretCursor = 0
	ret, _ = r2.handleSecretEditorKey(specialKey(tea.KeySpace))
	r3 := ret.(Model)
	_, present := r3.editorSearch.selected["k1"]
	assert.False(t, present, "second toggle on the same key removes it from the set entirely")
}

func TestSecretEditor_YWithSelectionOpensFormatPicker(t *testing.T) {
	// With selections present, plain `y` no longer copies just the
	// cursor value — it opens the format picker so the user can
	// pick how to combine the selected pairs. With NO selections,
	// `y` keeps the original single-value copy semantics.
	data := &model.SecretData{
		Keys: []string{"k1", "k2"},
		Data: map[string]string{"k1": "v1", "k2": "v2"},
	}
	m := Model{
		overlay:      overlaySecretEditor,
		secretData:   data,
		secretCursor: 0,
		tabs:         []TabState{{}},
		width:        80, height: 40,
	}
	m.editorSearch.selected = map[string]bool{"k1": true, "k2": true}

	ret, _ := m.handleSecretEditorKey(runeKey('y'))
	r := ret.(Model)
	assert.True(t, r.editorSearch.formatActive,
		"y with selections must auto-open the format picker — copying a single cursor value would silently ignore the user's marked rows")
	assert.Empty(t, r.statusMessage,
		"y must NOT trigger a copy when it opens the picker — copy happens on Enter inside the picker")
}

func TestSecretEditor_YWithoutSelectionCopiesCursor(t *testing.T) {
	data := &model.SecretData{
		Keys: []string{"k1"},
		Data: map[string]string{"k1": "value-1"},
	}
	m := Model{
		overlay:      overlaySecretEditor,
		secretData:   data,
		secretCursor: 0,
		tabs:         []TabState{{}},
		width:        80, height: 40,
	}
	ret, _ := m.handleSecretEditorKey(runeKey('y'))
	r := ret.(Model)
	assert.False(t, r.editorSearch.formatActive,
		"y without selections must NOT open the picker — single-value copy is the original behaviour")
	assert.Contains(t, r.statusMessage, "Copied",
		"y without selections must surface the single-value copy via the status bar")
}

func TestSecretEditor_ShiftYOpensFormatPicker(t *testing.T) {
	data := &model.SecretData{
		Keys: []string{"k1"},
		Data: map[string]string{"k1": "v1"},
	}
	m := Model{
		overlay:      overlaySecretEditor,
		secretData:   data,
		secretCursor: 0,
		tabs:         []TabState{{}},
		width:        80, height: 40,
	}
	ret, _ := m.handleSecretEditorKey(runeKey('Y'))
	result := ret.(Model)
	assert.True(t, result.editorSearch.formatActive, "Shift+Y opens the format picker")
	assert.Equal(t, 0, result.editorSearch.formatCursor, "picker starts at the first format (YAML)")
}

func TestSecretEditor_FormatPickerEnterCopiesAndCloses(t *testing.T) {
	data := &model.SecretData{
		Keys: []string{"k1", "k2"},
		Data: map[string]string{"k1": "v1", "k2": "v2"},
	}
	m := Model{
		overlay:      overlaySecretEditor,
		secretData:   data,
		secretCursor: 0,
		tabs:         []TabState{{}},
		width:        80, height: 40,
	}
	m.editorSearch.formatActive = true
	m.editorSearch.formatCursor = 0 // YAML
	m.editorSearch.selected = map[string]bool{"k2": true}

	ret, _ := m.handleSecretEditorKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.editorSearch.formatActive, "enter closes the picker")
	// Status message is set; clipboard cmd is returned via tea.Batch
	// — assert the human-readable label landed on the status bar.
	assert.Contains(t, result.statusMessage, "Copied",
		"status message confirms the copy so the user knows it happened")
}

func TestSecretEditor_FormatPickerEscCancels(t *testing.T) {
	data := &model.SecretData{
		Keys: []string{"k1"},
		Data: map[string]string{"k1": "v1"},
	}
	m := Model{
		overlay:    overlaySecretEditor,
		secretData: data,
		tabs:       []TabState{{}},
		width:      80, height: 40,
	}
	m.editorSearch.formatActive = true
	m.editorSearch.formatCursor = 2 // dotenv

	ret, _ := m.handleSecretEditorKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.editorSearch.formatActive, "esc closes the picker")
	assert.Empty(t, result.statusMessage, "esc must NOT trigger a copy — no status message")
}

// --- ConfigMap multi-row selection + Shift+Y format picker ---

func TestConfigMapEditor_SpaceTogglesSelectionAndAdvanceCursor(t *testing.T) {
	data := &model.ConfigMapData{
		Keys: []string{"k1", "k2", "k3"},
		Data: map[string]string{"k1": "v1", "k2": "v2", "k3": "v3"},
	}
	m := Model{
		overlay:         overlayConfigMapEditor,
		configMapData:   data,
		configMapCursor: 0,
		tabs:            []TabState{{}},
		width:           80, height: 40,
	}

	ret, _ := m.handleConfigMapEditorKey(specialKey(tea.KeySpace))
	r1 := ret.(Model)
	assert.True(t, r1.editorSearch.selected["k1"], "first Space selects k1")
	assert.Equal(t, 1, r1.configMapCursor, "cursor advances on Space")

	ret, _ = r1.handleConfigMapEditorKey(specialKey(tea.KeySpace))
	r2 := ret.(Model)
	assert.True(t, r2.editorSearch.selected["k2"], "k2 selected")
	assert.True(t, r2.editorSearch.selected["k1"], "k1 still selected")

	r2.configMapCursor = 0
	ret, _ = r2.handleConfigMapEditorKey(specialKey(tea.KeySpace))
	r3 := ret.(Model)
	_, present := r3.editorSearch.selected["k1"]
	assert.False(t, present, "second toggle on the same key removes the entry")
}

func TestConfigMapEditor_ShiftYOpensFormatPicker(t *testing.T) {
	data := &model.ConfigMapData{
		Keys: []string{"k1"},
		Data: map[string]string{"k1": "v1"},
	}
	m := Model{
		overlay:         overlayConfigMapEditor,
		configMapData:   data,
		configMapCursor: 0,
		tabs:            []TabState{{}},
		width:           80, height: 40,
	}
	ret, _ := m.handleConfigMapEditorKey(runeKey('Y'))
	result := ret.(Model)
	assert.True(t, result.editorSearch.formatActive, "Shift+Y opens the format picker")
	assert.Equal(t, 0, result.editorSearch.formatCursor, "picker starts at YAML")
}

func TestConfigMapEditor_FormatPickerEnterCopiesAndCloses(t *testing.T) {
	data := &model.ConfigMapData{
		Keys: []string{"k1", "k2"},
		Data: map[string]string{"k1": "v1", "k2": "v2"},
	}
	m := Model{
		overlay:         overlayConfigMapEditor,
		configMapData:   data,
		configMapCursor: 0,
		tabs:            []TabState{{}},
		width:           80, height: 40,
	}
	m.editorSearch.formatActive = true
	m.editorSearch.formatCursor = 0
	m.editorSearch.selected = map[string]bool{"k2": true}

	ret, _ := m.handleConfigMapEditorKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.editorSearch.formatActive, "enter closes the picker")
	assert.Contains(t, result.statusMessage, "Copied",
		"status message confirms the copy so the user knows it happened")
}

func TestConfigMapEditor_FormatPickerEscCancels(t *testing.T) {
	data := &model.ConfigMapData{
		Keys: []string{"k1"},
		Data: map[string]string{"k1": "v1"},
	}
	m := Model{
		overlay:       overlayConfigMapEditor,
		configMapData: data,
		tabs:          []TabState{{}},
		width:         80, height: 40,
	}
	m.editorSearch.formatActive = true
	m.editorSearch.formatCursor = 1

	ret, _ := m.handleConfigMapEditorKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.editorSearch.formatActive, "esc closes the picker")
	assert.Empty(t, result.statusMessage, "esc must NOT trigger a copy")
}

// --- Label multi-row selection + Shift+Y format picker ---

func TestLabelEditor_SpaceTogglesSelectionAndAdvanceCursor(t *testing.T) {
	data := &model.LabelAnnotationData{
		Labels:    map[string]string{"app": "nginx", "env": "prod", "tier": "web"},
		LabelKeys: []string{"app", "env", "tier"},
	}
	m := Model{
		overlay:     overlayLabelEditor,
		labelData:   data,
		labelTab:    0,
		labelCursor: 0,
		tabs:        []TabState{{}},
		width:       80, height: 40,
	}

	ret, _ := m.handleLabelEditorKey(specialKey(tea.KeySpace))
	r1 := ret.(Model)
	assert.True(t, r1.editorSearch.selected["app"], "first Space selects app")
	assert.Equal(t, 1, r1.labelCursor, "cursor advances on Space")

	ret, _ = r1.handleLabelEditorKey(specialKey(tea.KeySpace))
	r2 := ret.(Model)
	assert.True(t, r2.editorSearch.selected["env"], "env selected")
	assert.True(t, r2.editorSearch.selected["app"], "app still selected")
}

func TestLabelEditor_TabClearsSelection(t *testing.T) {
	// Label and annotation namespaces are disjoint — switching tabs
	// must clear the selection set so a key marked in Labels doesn't
	// accidentally apply to a same-named annotation.
	data := &model.LabelAnnotationData{
		Labels:      map[string]string{"app": "nginx"},
		LabelKeys:   []string{"app"},
		Annotations: map[string]string{"note": "test"},
		AnnotKeys:   []string{"note"},
	}
	m := Model{
		overlay:     overlayLabelEditor,
		labelData:   data,
		labelTab:    0,
		labelCursor: 0,
		tabs:        []TabState{{}},
		width:       80, height: 40,
	}
	ret, _ := m.handleLabelEditorKey(specialKey(tea.KeySpace))
	r1 := ret.(Model)
	assert.True(t, r1.editorSearch.selected["app"], "selection set has app")

	ret, _ = r1.handleLabelEditorKey(specialKey(tea.KeyTab))
	r2 := ret.(Model)
	assert.Equal(t, 1, r2.labelTab, "tab switches to annotations")
	assert.Empty(t, r2.editorSearch.selected, "selection cleared on tab switch")
}

func TestLabelEditor_ShiftYOpensFormatPicker(t *testing.T) {
	data := &model.LabelAnnotationData{
		Labels:    map[string]string{"app": "nginx"},
		LabelKeys: []string{"app"},
	}
	m := Model{
		overlay:     overlayLabelEditor,
		labelData:   data,
		labelTab:    0,
		labelCursor: 0,
		tabs:        []TabState{{}},
		width:       80, height: 40,
	}
	ret, _ := m.handleLabelEditorKey(runeKey('Y'))
	result := ret.(Model)
	assert.True(t, result.editorSearch.formatActive, "Shift+Y opens the format picker")
	assert.Equal(t, 0, result.editorSearch.formatCursor, "picker starts at YAML")
}

func TestLabelEditor_FormatPickerEnterCopiesAndCloses(t *testing.T) {
	data := &model.LabelAnnotationData{
		Labels:    map[string]string{"app": "nginx", "env": "prod"},
		LabelKeys: []string{"app", "env"},
	}
	m := Model{
		overlay:     overlayLabelEditor,
		labelData:   data,
		labelTab:    0,
		labelCursor: 0,
		tabs:        []TabState{{}},
		width:       80, height: 40,
	}
	m.editorSearch.formatActive = true
	m.editorSearch.formatCursor = 0
	m.editorSearch.selected = map[string]bool{"env": true}

	ret, _ := m.handleLabelEditorKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.False(t, result.editorSearch.formatActive, "enter closes the picker")
	assert.Contains(t, result.statusMessage, "Copied",
		"status message confirms the copy so the user knows it happened")
	assert.Contains(t, result.statusMessage, "label",
		"status message indicates labels were copied (not annotations)")
}

func TestLabelEditor_FormatPickerStatusMentionsAnnotation(t *testing.T) {
	// On the annotations tab the status message should call them
	// "annotation pair(s)" so the user sees what got copied.
	data := &model.LabelAnnotationData{
		Annotations: map[string]string{"note": "test"},
		AnnotKeys:   []string{"note"},
	}
	m := Model{
		overlay:     overlayLabelEditor,
		labelData:   data,
		labelTab:    1,
		labelCursor: 0,
		tabs:        []TabState{{}},
		width:       80, height: 40,
	}
	m.editorSearch.formatActive = true
	m.editorSearch.formatCursor = 0

	ret, _ := m.handleLabelEditorKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.Contains(t, result.statusMessage, "annotation",
		"status message indicates annotations were copied")
}

func TestLabelEditor_FormatPickerEscCancels(t *testing.T) {
	data := &model.LabelAnnotationData{
		Labels:    map[string]string{"app": "nginx"},
		LabelKeys: []string{"app"},
	}
	m := Model{
		overlay:   overlayLabelEditor,
		labelData: data,
		tabs:      []TabState{{}},
		width:     80, height: 40,
	}
	m.editorSearch.formatActive = true
	m.editorSearch.formatCursor = 1

	ret, _ := m.handleLabelEditorKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.editorSearch.formatActive, "esc closes the picker")
	assert.Empty(t, result.statusMessage, "esc must NOT trigger a copy")
}

// --- handleSecretEditorKey ---

func TestSecretEditorNilDataCloses(t *testing.T) {
	m := Model{
		overlay:    overlaySecretEditor,
		secretData: nil,
		tabs:       []TabState{{}},
		width:      80,
		height:     40,
	}
	ret, _ := m.handleSecretEditorKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestSecretEditorNormalModeNavigation(t *testing.T) {
	data := &model.SecretData{
		Keys: []string{"username", "password", "token"},
		Data: map[string]string{"username": "admin", "password": "secret", "token": "abc123"},
	}

	t.Run("esc closes and clears data", func(t *testing.T) {
		m := Model{
			overlay:    overlaySecretEditor,
			secretData: data,
			tabs:       []TabState{{}},
			width:      80,
			height:     40,
		}
		ret, _ := m.handleSecretEditorKey(specialKey(tea.KeyEsc))
		result := ret.(Model)
		assert.Equal(t, overlayNone, result.overlay)
		assert.Nil(t, result.secretData)
	})

	t.Run("j moves cursor down", func(t *testing.T) {
		m := Model{
			overlay:      overlaySecretEditor,
			secretData:   data,
			secretCursor: 0,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		ret, _ := m.handleSecretEditorKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.secretCursor)
	})

	t.Run("j at bottom stays", func(t *testing.T) {
		m := Model{
			overlay:      overlaySecretEditor,
			secretData:   data,
			secretCursor: 2,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		ret, _ := m.handleSecretEditorKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 2, result.secretCursor)
	})

	t.Run("k moves cursor up", func(t *testing.T) {
		m := Model{
			overlay:      overlaySecretEditor,
			secretData:   data,
			secretCursor: 2,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		ret, _ := m.handleSecretEditorKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 1, result.secretCursor)
	})

	t.Run("k at top stays", func(t *testing.T) {
		m := Model{
			overlay:      overlaySecretEditor,
			secretData:   data,
			secretCursor: 0,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		ret, _ := m.handleSecretEditorKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 0, result.secretCursor)
	})

	t.Run("v toggles visibility", func(t *testing.T) {
		m := Model{
			overlay:        overlaySecretEditor,
			secretData:     data,
			secretCursor:   0,
			secretRevealed: make(map[string]bool),
			tabs:           []TabState{{}},
			width:          80,
			height:         40,
		}
		ret, _ := m.handleSecretEditorKey(runeKey('v'))
		result := ret.(Model)
		assert.True(t, result.secretRevealed["username"])

		// Toggle back
		ret2, _ := result.handleSecretEditorKey(runeKey('v'))
		result2 := ret2.(Model)
		assert.False(t, result2.secretRevealed["username"])
	})

	t.Run("V toggles all visibility", func(t *testing.T) {
		m := Model{
			overlay:           overlaySecretEditor,
			secretData:        data,
			secretAllRevealed: false,
			tabs:              []TabState{{}},
			width:             80,
			height:            40,
		}
		ret, _ := m.handleSecretEditorKey(runeKey('V'))
		result := ret.(Model)
		assert.True(t, result.secretAllRevealed)
	})

	t.Run("e enters edit mode on value column", func(t *testing.T) {
		m := Model{
			overlay:      overlaySecretEditor,
			secretData:   data,
			secretCursor: 1,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		ret, _ := m.handleSecretEditorKey(runeKey('e'))
		result := ret.(Model)
		assert.True(t, result.secretEditing)
		assert.Equal(t, 1, result.secretEditColumn)
		assert.Equal(t, "password", result.secretEditKey.Value)
		assert.Equal(t, "secret", result.secretEditValue.Value)
	})

	t.Run("a adds new key", func(t *testing.T) {
		dataCopy := &model.SecretData{
			Keys: []string{"username"},
			Data: map[string]string{"username": "admin"},
		}
		m := Model{
			overlay:    overlaySecretEditor,
			secretData: dataCopy,
			tabs:       []TabState{{}},
			width:      80,
			height:     40,
		}
		ret, _ := m.handleSecretEditorKey(runeKey('a'))
		result := ret.(Model)
		assert.True(t, result.secretEditing)
		assert.Equal(t, 0, result.secretEditColumn) // editing key
		assert.Equal(t, 1, result.secretCursor)
		assert.Len(t, result.secretData.Keys, 2)
	})

	t.Run("D deletes current row", func(t *testing.T) {
		dataCopy := &model.SecretData{
			Keys: []string{"a", "b", "c"},
			Data: map[string]string{"a": "1", "b": "2", "c": "3"},
		}
		m := Model{
			overlay:      overlaySecretEditor,
			secretData:   dataCopy,
			secretCursor: 1,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		ret, _ := m.handleSecretEditorKey(runeKey('D'))
		result := ret.(Model)
		assert.Len(t, result.secretData.Keys, 2)
		assert.Equal(t, []string{"a", "c"}, result.secretData.Keys)
		_, exists := result.secretData.Data["b"]
		assert.False(t, exists)
	})

	t.Run("D on last item adjusts cursor", func(t *testing.T) {
		dataCopy := &model.SecretData{
			Keys: []string{"a", "b"},
			Data: map[string]string{"a": "1", "b": "2"},
		}
		m := Model{
			overlay:      overlaySecretEditor,
			secretData:   dataCopy,
			secretCursor: 1,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		ret, _ := m.handleSecretEditorKey(runeKey('D'))
		result := ret.(Model)
		assert.Equal(t, 0, result.secretCursor)
	})
}

// --- handleSecretEditorKey: editing mode ---

func TestSecretEditorEditingMode(t *testing.T) {
	makeEditingModel := func(col int) Model {
		return Model{
			overlay: overlaySecretEditor,
			secretData: &model.SecretData{
				Keys: []string{"user"},
				Data: map[string]string{"user": "admin"},
			},
			secretCursor:     0,
			secretEditing:    true,
			secretEditColumn: col,
			secretEditKey:    TextInput{Value: "user", Cursor: 4},
			secretEditValue:  TextInput{Value: "admin", Cursor: 5},
			tabs:             []TabState{{}},
			width:            80,
			height:           40,
		}
	}

	t.Run("esc exits editing mode", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleSecretEditorKey(specialKey(tea.KeyEsc))
		result := ret.(Model)
		assert.False(t, result.secretEditing)
		assert.Equal(t, -1, result.secretEditColumn)
	})

	t.Run("tab switches between key and value", func(t *testing.T) {
		m := makeEditingModel(0)
		ret, _ := m.handleSecretEditorKey(specialKey(tea.KeyTab))
		result := ret.(Model)
		assert.Equal(t, 1, result.secretEditColumn)

		ret2, _ := result.handleSecretEditorKey(specialKey(tea.KeyTab))
		result2 := ret2.(Model)
		assert.Equal(t, 0, result2.secretEditColumn)
	})

	t.Run("enter inserts newline in value column", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleSecretEditorKey(specialKey(tea.KeyEnter))
		result := ret.(Model)
		assert.Contains(t, result.secretEditValue.Value, "\n")
	})

	t.Run("enter in key column does nothing", func(t *testing.T) {
		m := makeEditingModel(0)
		origValue := m.secretEditKey.Value
		ret, _ := m.handleSecretEditorKey(specialKey(tea.KeyEnter))
		result := ret.(Model)
		assert.Equal(t, origValue, result.secretEditKey.Value)
	})

	t.Run("typing inserts into key column", func(t *testing.T) {
		m := makeEditingModel(0)
		ret, _ := m.handleSecretEditorKey(runeKey('x'))
		result := ret.(Model)
		assert.Contains(t, result.secretEditKey.Value, "x")
	})

	t.Run("typing inserts into value column", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleSecretEditorKey(runeKey('x'))
		result := ret.(Model)
		assert.Contains(t, result.secretEditValue.Value, "x")
	})

	t.Run("backspace in key column", func(t *testing.T) {
		m := makeEditingModel(0)
		ret, _ := m.handleSecretEditorKey(specialKey(tea.KeyBackspace))
		result := ret.(Model)
		assert.Equal(t, "use", result.secretEditKey.Value)
	})

	t.Run("backspace in value column", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleSecretEditorKey(specialKey(tea.KeyBackspace))
		result := ret.(Model)
		assert.Equal(t, "admi", result.secretEditValue.Value)
	})

	t.Run("ctrl+s saves key rename", func(t *testing.T) {
		m := makeEditingModel(0)
		m.secretEditKey.Value = "username"
		ret, _ := m.handleSecretEditorKey(tea.KeyMsg{Type: tea.KeyCtrlS})
		result := ret.(Model)
		assert.False(t, result.secretEditing)
		assert.Contains(t, result.secretData.Keys, "username")
		_, hasOld := result.secretData.Data["user"]
		assert.False(t, hasOld)
	})

	t.Run("ctrl+s saves value edit", func(t *testing.T) {
		m := makeEditingModel(1)
		m.secretEditValue.Value = "newpassword"
		ret, _ := m.handleSecretEditorKey(tea.KeyMsg{Type: tea.KeyCtrlS})
		result := ret.(Model)
		assert.False(t, result.secretEditing)
		assert.Equal(t, "newpassword", result.secretData.Data["user"])
	})

	t.Run("ctrl+w deletes word in key column", func(t *testing.T) {
		m := makeEditingModel(0)
		ret, _ := m.handleSecretEditorKey(tea.KeyMsg{Type: tea.KeyCtrlW})
		result := ret.(Model)
		assert.Empty(t, result.secretEditKey.Value)
	})

	t.Run("ctrl+w deletes word in value column", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleSecretEditorKey(tea.KeyMsg{Type: tea.KeyCtrlW})
		result := ret.(Model)
		assert.Empty(t, result.secretEditValue.Value)
	})

	t.Run("ctrl+a moves home in key column", func(t *testing.T) {
		m := makeEditingModel(0)
		ret, _ := m.handleSecretEditorKey(tea.KeyMsg{Type: tea.KeyCtrlA})
		result := ret.(Model)
		assert.Equal(t, "user", result.secretEditKey.Value)
	})

	t.Run("ctrl+e moves end in value column", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleSecretEditorKey(tea.KeyMsg{Type: tea.KeyCtrlE})
		result := ret.(Model)
		assert.Equal(t, "admin", result.secretEditValue.Value)
	})

	t.Run("left moves cursor in key column", func(t *testing.T) {
		m := makeEditingModel(0)
		ret, _ := m.handleSecretEditorKey(specialKey(tea.KeyLeft))
		result := ret.(Model)
		assert.Equal(t, "user", result.secretEditKey.Value)
	})

	t.Run("right moves cursor in value column", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleSecretEditorKey(specialKey(tea.KeyRight))
		result := ret.(Model)
		assert.Equal(t, "admin", result.secretEditValue.Value)
	})

	t.Run("ctrl+a is line-scoped (not buffer Home)", func(t *testing.T) {
		// User asked: "ctrl+a should move the cursor to the
		// beginning of the CURRENT line and ctrl+e to the end of
		// the current line." Buffer-Home would land at offset 0;
		// line-Home should land at the start of the second line.
		m := makeEditingModel(1)
		m.secretEditValue = TextInput{Value: "first\nsecond", Cursor: 9} // on "second" col 3
		ret, _ := m.handleSecretEditorKey(tea.KeyMsg{Type: tea.KeyCtrlA})
		result := ret.(Model)
		assert.Equal(t, 6, result.secretEditValue.Cursor,
			"ctrl+a should land at start of 'second' (offset 6), not buffer offset 0")
	})

	t.Run("ctrl+e is line-scoped (not buffer End)", func(t *testing.T) {
		m := makeEditingModel(1)
		m.secretEditValue = TextInput{Value: "first\nsecond", Cursor: 2} // mid 'first'
		ret, _ := m.handleSecretEditorKey(tea.KeyMsg{Type: tea.KeyCtrlE})
		result := ret.(Model)
		assert.Equal(t, 5, result.secretEditValue.Cursor,
			"ctrl+e should land at end of 'first' (offset 5), not buffer end (12)")
	})
}

// --- Sticky scroll + page-scroll keys ---

// TestSecretEditor_StickyScrollOnArrowUp pins the user's
// "scrolling up works differently than scrolling up in other
// overlays … the cursor stays in the last line and the text scrolls"
// report. With the previous always-pin-cursor-to-bottom heuristic,
// arrow-up looked like the view scrolled under a stationary cursor.
// With sticky scroll the cursor moves freely within the visible
// window; scroll only adjusts when the cursor leaves it.
func TestSecretEditor_StickyScrollOnArrowUp(t *testing.T) {
	// 60 short lines — comfortably more than the field box's height
	// (~14 rows for screen 120x30).
	lines := make([]string, 0, 60)
	for i := range 60 {
		lines = append(lines, "line-"+itoaTinyApp(i))
	}
	value := strings.Join(lines, "\n")

	m := Model{
		overlay:          overlaySecretEditor,
		secretData:       &model.SecretData{Keys: []string{"k"}, Data: map[string]string{"k": value}},
		secretCursor:     0,
		secretEditing:    true,
		secretEditColumn: 1,
		secretEditKey:    TextInput{Value: "k", Cursor: 1},
		secretEditValue:  TextInput{Value: value, Cursor: len(value)}, // cursor at end
		tabs:             []TabState{{}},
		width:            120,
		height:           30,
	}
	// Mimic edit-mode entry: position scroll for cursor at end.
	adjustEditValueScrollFor(&m, m.secretEditValue.Value, m.secretEditValue.Cursor, 1, 1)
	scrollAtEntry := m.editorSearch.editValueScroll
	assert.Greater(t, scrollAtEntry, 0, "cursor at end of long value must scroll past 0")

	// Press up once. Expected: cursor moves up by one line, scroll
	// unchanged (cursor was on the bottom row, now on second-to-bottom).
	ret, _ := m.handleSecretEditorKey(specialKey(tea.KeyUp))
	r1 := ret.(Model)
	assert.Equal(t, scrollAtEntry, r1.editorSearch.editValueScroll,
		"single arrow-up must NOT shift scroll — cursor moves within visible window")

	// Cursor moved up one line — verify byte offset decreased by ~1 line.
	assert.Less(t, r1.secretEditValue.Cursor, m.secretEditValue.Cursor,
		"cursor should have actually moved up (byte offset decreased)")
}

// TestSecretEditor_ScrollAdjustsWhenCursorLeavesWindow asserts that
// once the cursor reaches the top of the visible window, further
// arrow-up DOES nudge scroll (so the user can keep navigating up).
func TestSecretEditor_ScrollAdjustsWhenCursorLeavesWindow(t *testing.T) {
	lines := make([]string, 0, 60)
	for i := range 60 {
		lines = append(lines, "line-"+itoaTinyApp(i))
	}
	value := strings.Join(lines, "\n")

	m := Model{
		overlay:          overlaySecretEditor,
		secretData:       &model.SecretData{Keys: []string{"k"}, Data: map[string]string{"k": value}},
		secretEditing:    true,
		secretEditColumn: 1,
		secretEditKey:    TextInput{Value: "k", Cursor: 1},
		secretEditValue:  TextInput{Value: value, Cursor: len(value)},
		tabs:             []TabState{{}},
		width:            120, height: 30,
	}
	adjustEditValueScrollFor(&m, m.secretEditValue.Value, m.secretEditValue.Cursor, 1, 1)
	startScroll := m.editorSearch.editValueScroll

	// Walk up enough times that the cursor MUST leave the visible window.
	cur := tea.Model(m)
	for range 100 {
		cur, _ = cur.(Model).handleSecretEditorKey(specialKey(tea.KeyUp))
	}
	end := cur.(Model)
	assert.Less(t, end.editorSearch.editValueScroll, startScroll,
		"after enough up-presses scroll must decrease — cursor walked past the top of the visible window")
}

// TestSecretEditor_CtrlDScrollsByHalfPage asserts ctrl+d moves the
// cursor down by approximately half the visible window, exercising
// the new vim-like page-scroll keys.
func TestSecretEditor_CtrlDScrollsByHalfPage(t *testing.T) {
	lines := make([]string, 0, 60)
	for i := range 60 {
		lines = append(lines, "line-"+itoaTinyApp(i))
	}
	value := strings.Join(lines, "\n")

	m := Model{
		overlay:          overlaySecretEditor,
		secretData:       &model.SecretData{Keys: []string{"k"}, Data: map[string]string{"k": value}},
		secretEditing:    true,
		secretEditColumn: 1,
		secretEditKey:    TextInput{Value: "k", Cursor: 1},
		secretEditValue:  TextInput{Value: value, Cursor: 0}, // cursor at top
		tabs:             []TabState{{}},
		width:            120, height: 30,
	}
	ret, _ := m.handleSecretEditorKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	result := ret.(Model)
	assert.Greater(t, result.secretEditValue.Cursor, 0,
		"ctrl+d must move the cursor down by half a page")
	// Confirm we moved by multiple lines, not just 1.
	linesMoved := strings.Count(value[:result.secretEditValue.Cursor], "\n")
	assert.GreaterOrEqual(t, linesMoved, 2,
		"ctrl+d should advance several lines, not just one — got %d", linesMoved)
}

// itoaTinyApp mirrors itoaTiny in the ui test file (kept local to
// avoid cross-package test imports).
func itoaTinyApp(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [4]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	if n2 := len(digits) - i; n2 == 1 {
		return "0" + string(digits[i:])
	}
	return string(digits[i:])
}

// --- handleConfigMapEditorKey ---

func TestConfigMapEditorNilDataCloses(t *testing.T) {
	m := Model{
		overlay:       overlayConfigMapEditor,
		configMapData: nil,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleConfigMapEditorKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestConfigMapEditorNormalModeNavigation(t *testing.T) {
	data := &model.ConfigMapData{
		Keys: []string{"key1", "key2", "key3"},
		Data: map[string]string{"key1": "val1", "key2": "val2", "key3": "val3"},
	}

	t.Run("esc closes", func(t *testing.T) {
		m := Model{
			overlay:       overlayConfigMapEditor,
			configMapData: data,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleConfigMapEditorKey(specialKey(tea.KeyEsc))
		result := ret.(Model)
		assert.Equal(t, overlayNone, result.overlay)
		assert.Nil(t, result.configMapData)
	})

	t.Run("j moves cursor down", func(t *testing.T) {
		m := Model{
			overlay:         overlayConfigMapEditor,
			configMapData:   data,
			configMapCursor: 0,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleConfigMapEditorKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.configMapCursor)
	})

	t.Run("k moves cursor up", func(t *testing.T) {
		m := Model{
			overlay:         overlayConfigMapEditor,
			configMapData:   data,
			configMapCursor: 2,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleConfigMapEditorKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 1, result.configMapCursor)
	})

	t.Run("e enters edit mode", func(t *testing.T) {
		m := Model{
			overlay:         overlayConfigMapEditor,
			configMapData:   data,
			configMapCursor: 0,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleConfigMapEditorKey(runeKey('e'))
		result := ret.(Model)
		assert.True(t, result.configMapEditing)
		assert.Equal(t, 1, result.configMapEditColumn)
		assert.Equal(t, "key1", result.configMapEditKey.Value)
	})

	t.Run("a adds new key", func(t *testing.T) {
		dataCopy := &model.ConfigMapData{
			Keys: []string{"key1"},
			Data: map[string]string{"key1": "val1"},
		}
		m := Model{
			overlay:       overlayConfigMapEditor,
			configMapData: dataCopy,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleConfigMapEditorKey(runeKey('a'))
		result := ret.(Model)
		assert.True(t, result.configMapEditing)
		assert.Len(t, result.configMapData.Keys, 2)
	})

	t.Run("D deletes row", func(t *testing.T) {
		dataCopy := &model.ConfigMapData{
			Keys: []string{"a", "b", "c"},
			Data: map[string]string{"a": "1", "b": "2", "c": "3"},
		}
		m := Model{
			overlay:         overlayConfigMapEditor,
			configMapData:   dataCopy,
			configMapCursor: 1,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleConfigMapEditorKey(runeKey('D'))
		result := ret.(Model)
		assert.Len(t, result.configMapData.Keys, 2)
		assert.Equal(t, []string{"a", "c"}, result.configMapData.Keys)
	})
}

func TestConfigMapEditorEditingMode(t *testing.T) {
	makeEditingModel := func(col int) Model {
		return Model{
			overlay: overlayConfigMapEditor,
			configMapData: &model.ConfigMapData{
				Keys: []string{"mykey"},
				Data: map[string]string{"mykey": "myval"},
			},
			configMapCursor:     0,
			configMapEditing:    true,
			configMapEditColumn: col,
			configMapEditKey:    TextInput{Value: "mykey", Cursor: 5},
			configMapEditValue:  TextInput{Value: "myval", Cursor: 5},
			tabs:                []TabState{{}},
			width:               80,
			height:              40,
		}
	}

	t.Run("esc exits editing", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleConfigMapEditorKey(specialKey(tea.KeyEsc))
		result := ret.(Model)
		assert.False(t, result.configMapEditing)
	})

	t.Run("tab switches columns", func(t *testing.T) {
		m := makeEditingModel(0)
		ret, _ := m.handleConfigMapEditorKey(specialKey(tea.KeyTab))
		result := ret.(Model)
		assert.Equal(t, 1, result.configMapEditColumn)
	})

	t.Run("ctrl+s saves value", func(t *testing.T) {
		m := makeEditingModel(1)
		m.configMapEditValue.Value = "newval"
		ret, _ := m.handleConfigMapEditorKey(tea.KeyMsg{Type: tea.KeyCtrlS})
		result := ret.(Model)
		assert.False(t, result.configMapEditing)
		assert.Equal(t, "newval", result.configMapData.Data["mykey"])
	})

	t.Run("ctrl+s renames key", func(t *testing.T) {
		m := makeEditingModel(0)
		m.configMapEditKey.Value = "renamed"
		ret, _ := m.handleConfigMapEditorKey(tea.KeyMsg{Type: tea.KeyCtrlS})
		result := ret.(Model)
		assert.False(t, result.configMapEditing)
		assert.Contains(t, result.configMapData.Keys, "renamed")
	})

	t.Run("enter inserts newline in value", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleConfigMapEditorKey(specialKey(tea.KeyEnter))
		result := ret.(Model)
		assert.Contains(t, result.configMapEditValue.Value, "\n")
	})

	t.Run("typing in key column", func(t *testing.T) {
		m := makeEditingModel(0)
		ret, _ := m.handleConfigMapEditorKey(runeKey('x'))
		result := ret.(Model)
		assert.Contains(t, result.configMapEditKey.Value, "x")
	})

	t.Run("typing in value column", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleConfigMapEditorKey(runeKey('z'))
		result := ret.(Model)
		assert.Contains(t, result.configMapEditValue.Value, "z")
	})

	t.Run("backspace in key column", func(t *testing.T) {
		m := makeEditingModel(0)
		ret, _ := m.handleConfigMapEditorKey(specialKey(tea.KeyBackspace))
		result := ret.(Model)
		assert.Equal(t, "myke", result.configMapEditKey.Value)
	})

	t.Run("backspace in value column", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleConfigMapEditorKey(specialKey(tea.KeyBackspace))
		result := ret.(Model)
		assert.Equal(t, "myva", result.configMapEditValue.Value)
	})
}

// --- handleLabelEditorKey ---

func TestLabelEditorNilDataCloses(t *testing.T) {
	m := Model{
		overlay:   overlayLabelEditor,
		labelData: nil,
		tabs:      []TabState{{}},
		width:     80,
		height:    40,
	}
	ret, _ := m.handleLabelEditorKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestLabelEditorNormalMode(t *testing.T) {
	data := &model.LabelAnnotationData{
		Labels:      map[string]string{"app": "nginx", "env": "prod"},
		LabelKeys:   []string{"app", "env"},
		Annotations: map[string]string{"note": "test"},
		AnnotKeys:   []string{"note"},
	}

	t.Run("esc closes", func(t *testing.T) {
		m := Model{
			overlay:   overlayLabelEditor,
			labelData: data,
			tabs:      []TabState{{}},
			width:     80,
			height:    40,
		}
		ret, _ := m.handleLabelEditorKey(specialKey(tea.KeyEsc))
		result := ret.(Model)
		assert.Equal(t, overlayNone, result.overlay)
		assert.Nil(t, result.labelData)
	})

	t.Run("tab switches between labels and annotations", func(t *testing.T) {
		m := Model{
			overlay:   overlayLabelEditor,
			labelData: data,
			labelTab:  0,
			tabs:      []TabState{{}},
			width:     80,
			height:    40,
		}
		ret, _ := m.handleLabelEditorKey(specialKey(tea.KeyTab))
		result := ret.(Model)
		assert.Equal(t, 1, result.labelTab)
		assert.Equal(t, 0, result.labelCursor)

		ret2, _ := result.handleLabelEditorKey(specialKey(tea.KeyTab))
		result2 := ret2.(Model)
		assert.Equal(t, 0, result2.labelTab)
	})

	t.Run("j moves cursor down in labels tab", func(t *testing.T) {
		m := Model{
			overlay:     overlayLabelEditor,
			labelData:   data,
			labelTab:    0,
			labelCursor: 0,
			tabs:        []TabState{{}},
			width:       80,
			height:      40,
		}
		ret, _ := m.handleLabelEditorKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.labelCursor)
	})

	t.Run("k moves cursor up", func(t *testing.T) {
		m := Model{
			overlay:     overlayLabelEditor,
			labelData:   data,
			labelTab:    0,
			labelCursor: 1,
			tabs:        []TabState{{}},
			width:       80,
			height:      40,
		}
		ret, _ := m.handleLabelEditorKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 0, result.labelCursor)
	})

	t.Run("e enters edit mode", func(t *testing.T) {
		m := Model{
			overlay:     overlayLabelEditor,
			labelData:   data,
			labelTab:    0,
			labelCursor: 0,
			tabs:        []TabState{{}},
			width:       80,
			height:      40,
		}
		ret, _ := m.handleLabelEditorKey(runeKey('e'))
		result := ret.(Model)
		assert.True(t, result.labelEditing)
		assert.Equal(t, 1, result.labelEditColumn)
		assert.Equal(t, "app", result.labelEditKey.Value)
		assert.Equal(t, "nginx", result.labelEditValue.Value)
	})

	t.Run("a adds new label", func(t *testing.T) {
		dataCopy := &model.LabelAnnotationData{
			Labels:      map[string]string{"app": "nginx"},
			LabelKeys:   []string{"app"},
			Annotations: map[string]string{},
			AnnotKeys:   []string{},
		}
		m := Model{
			overlay:   overlayLabelEditor,
			labelData: dataCopy,
			labelTab:  0,
			tabs:      []TabState{{}},
			width:     80,
			height:    40,
		}
		ret, _ := m.handleLabelEditorKey(runeKey('a'))
		result := ret.(Model)
		assert.True(t, result.labelEditing)
		assert.Len(t, result.labelData.LabelKeys, 2)
	})

	t.Run("D deletes label", func(t *testing.T) {
		dataCopy := &model.LabelAnnotationData{
			Labels:      map[string]string{"app": "nginx", "env": "prod"},
			LabelKeys:   []string{"app", "env"},
			Annotations: map[string]string{},
			AnnotKeys:   []string{},
		}
		m := Model{
			overlay:     overlayLabelEditor,
			labelData:   dataCopy,
			labelTab:    0,
			labelCursor: 0,
			tabs:        []TabState{{}},
			width:       80,
			height:      40,
		}
		ret, _ := m.handleLabelEditorKey(runeKey('D'))
		result := ret.(Model)
		assert.Len(t, result.labelData.LabelKeys, 1)
		assert.Equal(t, "env", result.labelData.LabelKeys[0])
	})
}

func TestLabelEditorEditingMode(t *testing.T) {
	makeEditingModel := func(col int) Model {
		return Model{
			overlay: overlayLabelEditor,
			labelData: &model.LabelAnnotationData{
				Labels:    map[string]string{"app": "nginx"},
				LabelKeys: []string{"app"},
			},
			labelTab:        0,
			labelCursor:     0,
			labelEditing:    true,
			labelEditColumn: col,
			labelEditKey:    TextInput{Value: "app", Cursor: 3},
			labelEditValue:  TextInput{Value: "nginx", Cursor: 5},
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
	}

	t.Run("esc exits editing", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleLabelEditorKey(specialKey(tea.KeyEsc))
		result := ret.(Model)
		assert.False(t, result.labelEditing)
	})

	t.Run("tab switches columns", func(t *testing.T) {
		m := makeEditingModel(0)
		ret, _ := m.handleLabelEditorKey(specialKey(tea.KeyTab))
		result := ret.(Model)
		assert.Equal(t, 1, result.labelEditColumn)
	})

	t.Run("ctrl+s saves value edit", func(t *testing.T) {
		m := makeEditingModel(1)
		m.labelEditValue.Value = "apache"
		ret, _ := m.handleLabelEditorKey(tea.KeyMsg{Type: tea.KeyCtrlS})
		result := ret.(Model)
		assert.False(t, result.labelEditing)
		assert.Equal(t, "apache", result.labelData.Labels["app"])
	})

	t.Run("ctrl+s renames key", func(t *testing.T) {
		m := makeEditingModel(0)
		m.labelEditKey.Value = "application"
		ret, _ := m.handleLabelEditorKey(tea.KeyMsg{Type: tea.KeyCtrlS})
		result := ret.(Model)
		assert.Contains(t, result.labelData.LabelKeys, "application")
		_, hasOld := result.labelData.Labels["app"]
		assert.False(t, hasOld)
	})

	t.Run("typing inserts in key column", func(t *testing.T) {
		m := makeEditingModel(0)
		ret, _ := m.handleLabelEditorKey(runeKey('x'))
		result := ret.(Model)
		assert.Contains(t, result.labelEditKey.Value, "x")
	})

	t.Run("typing inserts in value column", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleLabelEditorKey(runeKey('z'))
		result := ret.(Model)
		assert.Contains(t, result.labelEditValue.Value, "z")
	})

	t.Run("backspace in key column", func(t *testing.T) {
		m := makeEditingModel(0)
		ret, _ := m.handleLabelEditorKey(specialKey(tea.KeyBackspace))
		result := ret.(Model)
		assert.Equal(t, "ap", result.labelEditKey.Value)
	})

	t.Run("backspace in value column", func(t *testing.T) {
		m := makeEditingModel(1)
		ret, _ := m.handleLabelEditorKey(specialKey(tea.KeyBackspace))
		result := ret.(Model)
		assert.Equal(t, "ngin", result.labelEditValue.Value)
	})
}

func TestCovBatchLabelOverlayKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayBatchLabel
	result, _ := m.handleBatchLabelOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovBatchLabelOverlayKeyTyping(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayBatchLabel
	m.batchLabelMode = 0
	result, _ := m.handleBatchLabelOverlayKey(keyMsg("a"))
	rm := result.(Model)
	assert.Contains(t, rm.batchLabelInput.Value, "a")
}

func TestCovBatchLabelOverlayKeyBackspace(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayBatchLabel
	m.batchLabelMode = 0
	m.batchLabelInput.Insert("abc")
	result, _ := m.handleBatchLabelOverlayKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "ab", rm.batchLabelInput.Value)
}

func TestCovPVCResizeOverlayKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayPVCResize
	result, _ := m.handlePVCResizeOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovPVCResizeOverlayKeyTyping(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayPVCResize
	result, _ := m.handlePVCResizeOverlayKey(keyMsg("5"))
	rm := result.(Model)
	assert.Contains(t, rm.scaleInput.Value, "5")
}

func TestCovHandlePVCResizeOverlayKeyEsc(t *testing.T) {
	m := baseModelCov()
	m.overlay = overlayPVCResize
	m.scaleInput = TextInput{Value: "10Gi"}

	r, _ := m.handlePVCResizeOverlayKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, overlayNone, r.(Model).overlay)
	assert.Empty(t, r.(Model).scaleInput.Value)
}

func TestCovHandlePVCResizeOverlayKeyEnterEmpty(t *testing.T) {
	m := baseModelCov()
	m.overlay = overlayPVCResize
	m.scaleInput = TextInput{}

	r, _ := m.handlePVCResizeOverlayKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, overlayNone, r.(Model).overlay)
	assert.True(t, r.(Model).statusMessageErr)
}

func TestCovHandlePVCResizeOverlayKeyBackspace(t *testing.T) {
	m := baseModelCov()
	m.scaleInput = TextInput{Value: "10G", Cursor: 3}

	r, _ := m.handlePVCResizeOverlayKey(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "10", r.(Model).scaleInput.Value)
}

func TestCovHandlePVCResizeOverlayKeyCtrlW(t *testing.T) {
	m := baseModelCov()
	m.scaleInput = TextInput{Value: "10 Gi", Cursor: 5}

	r, _ := m.handlePVCResizeOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Equal(t, "10 ", r.(Model).scaleInput.Value)
}

func TestCovHandlePVCResizeOverlayKeyCursorMovement(t *testing.T) {
	m := baseModelCov()
	m.scaleInput = TextInput{Value: "10Gi", Cursor: 2}

	r, _ := m.handlePVCResizeOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlA})
	assert.Equal(t, 0, r.(Model).scaleInput.Cursor)

	r, _ = m.handlePVCResizeOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlE})
	assert.Equal(t, 4, r.(Model).scaleInput.Cursor)

	m.scaleInput.Cursor = 2
	r, _ = m.handlePVCResizeOverlayKey(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 1, r.(Model).scaleInput.Cursor)

	m.scaleInput.Cursor = 2
	r, _ = m.handlePVCResizeOverlayKey(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, 3, r.(Model).scaleInput.Cursor)
}

func TestCovHandlePVCResizeOverlayKeyInsert(t *testing.T) {
	m := baseModelCov()
	m.scaleInput = TextInput{Value: "10", Cursor: 2}

	r, _ := m.handlePVCResizeOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	assert.Equal(t, "10G", r.(Model).scaleInput.Value)
}
