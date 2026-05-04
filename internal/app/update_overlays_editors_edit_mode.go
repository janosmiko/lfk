package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handleSecretEditorEditKey handles key events while editing a secret value.
func (m Model) handleSecretEditorEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyInput := &m.secretEditKey
	valInput := &m.secretEditValue
	col := m.secretEditColumn
	activeInput := valInput
	if col == 0 {
		activeInput = keyInput
	}

	// Handle paste events (Cmd+V on macOS, Ctrl+Shift+V on Linux) by
	// inserting the pasted text at the cursor. Newlines are stripped from
	// the key field but kept in the value field.
	if msg.Paste {
		text := string(msg.Runes)
		if col == 0 {
			text = strings.ReplaceAll(text, "\n", "")
			text = strings.ReplaceAll(text, "\r", "")
		}
		if text != "" {
			activeInput.Insert(text)
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.secretEditing = false
		m.secretEditColumn = -1
		return m, nil
	case "ctrl+s":
		// Commit both the key and the value edits at once, regardless of
		// which column is currently active. This lets the user type a
		// value, tab back to the key column (or vice versa), and save
		// without silently losing the other column's edit.
		oldKey := m.secretData.Keys[m.secretCursor]
		newKey := keyInput.Value
		if newKey == "" {
			newKey = oldKey
		}
		if newKey != oldKey {
			delete(m.secretData.Data, oldKey)
			m.secretData.Keys[m.secretCursor] = newKey
		}
		m.secretData.Data[newKey] = valInput.Value
		m.secretEditing = false
		m.secretEditColumn = -1
		return m, nil
	case "enter":
		if col == 1 {
			valInput.Insert("\n")
		}
		return m, nil
	case "tab":
		if col == 0 {
			m.secretEditColumn = 1
		} else {
			m.secretEditColumn = 0
		}
		return m, nil
	case "backspace":
		if len(activeInput.Value) > 0 {
			activeInput.Backspace()
		}
		return m, nil
	case "ctrl+w":
		activeInput.DeleteWord()
		return m, nil
	case "ctrl+a":
		activeInput.Home()
		return m, nil
	case "ctrl+e":
		activeInput.End()
		return m, nil
	case "left":
		activeInput.Left()
		return m, nil
	case "right":
		activeInput.Right()
		return m, nil
	case "up":
		activeInput.Up()
		return m, nil
	case "down":
		activeInput.Down()
		return m, nil
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			activeInput.Insert(key)
		}
		return m, nil
	}
}

// handleConfigMapEditorEditKey handles key events while editing a configmap value.
func (m Model) handleConfigMapEditorEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyInput := &m.configMapEditKey
	valInput := &m.configMapEditValue
	col := m.configMapEditColumn
	activeInput := valInput
	if col == 0 {
		activeInput = keyInput
	}

	// Handle paste events (Cmd+V on macOS, Ctrl+Shift+V on Linux) by
	// inserting the pasted text at the cursor. Newlines are stripped from
	// the key field but kept in the value field.
	if msg.Paste {
		text := string(msg.Runes)
		if col == 0 {
			text = strings.ReplaceAll(text, "\n", "")
			text = strings.ReplaceAll(text, "\r", "")
		}
		if text != "" {
			activeInput.Insert(text)
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.configMapEditing = false
		m.configMapEditColumn = -1
		return m, nil
	case "ctrl+s":
		// Commit both the key and the value edits at once, regardless of
		// which column is currently active. This lets the user type a
		// value, tab back to the key column (or vice versa), and save
		// without silently losing the other column's edit.
		oldKey := m.configMapData.Keys[m.configMapCursor]
		newKey := keyInput.Value
		if newKey == "" {
			newKey = oldKey
		}
		if newKey != oldKey {
			delete(m.configMapData.Data, oldKey)
			m.configMapData.Keys[m.configMapCursor] = newKey
		}
		m.configMapData.Data[newKey] = valInput.Value
		m.configMapEditing = false
		m.configMapEditColumn = -1
		return m, nil
	case "enter":
		if col == 1 {
			valInput.Insert("\n")
		}
		return m, nil
	case "tab":
		if col == 0 {
			m.configMapEditColumn = 1
		} else {
			m.configMapEditColumn = 0
		}
		return m, nil
	case "backspace":
		if len(activeInput.Value) > 0 {
			activeInput.Backspace()
		}
		return m, nil
	case "ctrl+w":
		activeInput.DeleteWord()
		return m, nil
	case "ctrl+a":
		activeInput.Home()
		return m, nil
	case "ctrl+e":
		activeInput.End()
		return m, nil
	case "left":
		activeInput.Left()
		return m, nil
	case "right":
		activeInput.Right()
		return m, nil
	case "up":
		activeInput.Up()
		return m, nil
	case "down":
		activeInput.Down()
		return m, nil
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			activeInput.Insert(key)
		}
		return m, nil
	}
}

// handleLabelEditorEditKey handles key events while editing a label/annotation value.
func (m Model) handleLabelEditorEditKey(msg tea.KeyMsg, currentKeys []string, currentData map[string]string) (tea.Model, tea.Cmd) {
	keyInput := &m.labelEditKey
	valInput := &m.labelEditValue
	col := m.labelEditColumn
	activeInput := valInput
	if col == 0 {
		activeInput = keyInput
	}

	// Handle paste events (Cmd+V on macOS, Ctrl+Shift+V on Linux) by
	// inserting the pasted text at the cursor. Newlines are stripped from
	// the key field but kept in the value field.
	if msg.Paste {
		text := string(msg.Runes)
		if col == 0 {
			text = strings.ReplaceAll(text, "\n", "")
			text = strings.ReplaceAll(text, "\r", "")
		}
		if text != "" {
			activeInput.Insert(text)
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.labelEditing = false
		m.labelEditColumn = -1
		return m, nil
	case "ctrl+s":
		// Commit both the key and the value edits at once, regardless of
		// which column is currently active. This lets the user type a new
		// key, tab to the value column, type a value, and save — without
		// silently losing the key edit that happened before the tab.
		oldKey := currentKeys[m.labelCursor]
		newKey := keyInput.Value
		if newKey == "" {
			newKey = oldKey
		}
		if newKey != oldKey {
			delete(currentData, oldKey)
			currentKeys[m.labelCursor] = newKey
		}
		currentData[newKey] = valInput.Value
		if m.labelTab == 0 {
			m.labelData.LabelKeys = currentKeys
			m.labelData.Labels = currentData
		} else {
			m.labelData.AnnotKeys = currentKeys
			m.labelData.Annotations = currentData
		}
		m.labelEditing = false
		m.labelEditColumn = -1
		return m, nil
	case "tab":
		if col == 0 {
			m.labelEditColumn = 1
		} else {
			m.labelEditColumn = 0
		}
		return m, nil
	case "backspace":
		if len(activeInput.Value) > 0 {
			activeInput.Backspace()
		}
		return m, nil
	case "ctrl+w":
		activeInput.DeleteWord()
		return m, nil
	case "ctrl+a":
		activeInput.Home()
		return m, nil
	case "ctrl+e":
		activeInput.End()
		return m, nil
	case "left":
		activeInput.Left()
		return m, nil
	case "right":
		activeInput.Right()
		return m, nil
	case "up":
		activeInput.Up()
		return m, nil
	case "down":
		activeInput.Down()
		return m, nil
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			activeInput.Insert(key)
		}
		return m, nil
	}
}
