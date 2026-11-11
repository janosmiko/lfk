package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
)

// typedTextKeys are keystrokes whose String() is not the character they
// type: bubbletea v2 names space "space", and non-ASCII runes are multi-byte.
var typedTextKeys = []struct{ key, want string }{
	{"space", " "},
	{"ж", "ж"},
}

func TestKVEditors_InsertSpaceAndNonASCII(t *testing.T) {
	editors := []struct {
		name  string
		press func(m Model, msg tea.KeyPressMsg) string
	}{
		{"secret", func(m Model, msg tea.KeyPressMsg) string {
			m.secretEditing, m.secretEditColumn = true, 1
			ret, _ := m.handleSecretEditorEditKey(msg)
			return ret.(Model).secretEditValue.Value
		}},
		{"configmap", func(m Model, msg tea.KeyPressMsg) string {
			m.configMapEditing, m.configMapEditColumn = true, 1
			ret, _ := m.handleConfigMapEditorEditKey(msg)
			return ret.(Model).configMapEditValue.Value
		}},
		{"label", func(m Model, msg tea.KeyPressMsg) string {
			m.labelEditing, m.labelEditColumn = true, 1
			ret, _ := m.handleLabelEditorEditKey(msg, nil, nil)
			return ret.(Model).labelEditValue.Value
		}},
	}
	for _, ed := range editors {
		for _, tc := range typedTextKeys {
			t.Run(ed.name+"/"+tc.key, func(t *testing.T) {
				m := Model{width: 80, height: 40}
				assert.Equal(t, tc.want, ed.press(m, keyMsg(tc.key)))
			})
		}
	}
}

func TestLogTopInputs_InsertSpaceAndNonASCII(t *testing.T) {
	for _, tc := range typedTextKeys {
		t.Run("filter/"+tc.key, func(t *testing.T) {
			m := basePush80Model()
			m.mode = modeLogTop
			m.logTop.filterActive = true
			ret, _ := m.handleLogTopFilterKey(keyMsg(tc.key))
			assert.Equal(t, tc.want, ret.(Model).logTop.filterInput.Value)
		})
		t.Run("search/"+tc.key, func(t *testing.T) {
			m := basePush80Model()
			m.mode = modeLogTop
			m.logTop.searchActive = true
			ret, _ := m.handleLogTopSearchKey(keyMsg(tc.key))
			assert.Equal(t, tc.want, ret.(Model).logTop.searchInput.Value)
		})
	}
}
