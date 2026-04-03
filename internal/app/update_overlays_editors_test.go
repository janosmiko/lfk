package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

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
