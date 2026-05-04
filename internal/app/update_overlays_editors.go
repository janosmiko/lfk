package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleSecretEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.secretData == nil {
		m.overlay = overlayNone
		return m, nil
	}

	// Handle editing mode.
	if m.secretEditing {
		return m.handleSecretEditorEditKey(msg)
	}

	// Search input mode: typing extends the / query, esc clears, enter
	// accepts. Routed first so q/esc don't accidentally close the
	// overlay while the user is typing.
	if m.editorSearch.active {
		return m.handleEditorSearchKey(msg, clampSecretCursorToVisible)
	}

	// Normal mode.
	switch msg.String() {
	case "esc", "q":
		return m.handleSecretEditorKeyEsc()
	case "/":
		// Enter search input mode. Reset query so a fresh / always
		// starts blank — matching the convention used elsewhere in lfk.
		m.editorSearch.active = true
		m.editorSearch.query.Clear()
		m.secretCursor = 0
		return m, nil
	case "j", "down":
		return m.handleSecretEditorKeyJ()
	case "k", "up":
		return m.handleSecretEditorKeyK()
	case "v":
		// Toggle visibility for selected row.
		return m.handleSecretEditorKeyV()
	case "V":
		// Toggle all values visibility.
		m.secretAllRevealed = !m.secretAllRevealed
		return m, nil
	case "e":
		// Edit selected value.
		return m.handleSecretEditorKeyE()
	case "a":
		// Add new key-value pair. Clear the / search so the newly-added
		// row is visible (it likely won't match an existing query).
		m.resetEditorSearch()
		newKey := fmt.Sprintf("new-key-%d", len(m.secretData.Keys)+1)
		m.secretData.Keys = append(m.secretData.Keys, newKey)
		m.secretData.Data[newKey] = ""
		m.secretCursor = len(m.secretData.Keys) - 1
		m.secretEditing = true
		m.secretEditColumn = 0
		m.secretEditKey.Set(newKey)
		m.secretEditValue.Clear()
		return m, nil
	case "D":
		// Delete selected row.
		return m.handleSecretEditorKeyD()
	case "y":
		// Copy current value to clipboard.
		visible := m.secretVisibleKeys()
		if m.secretCursor >= 0 && m.secretCursor < len(visible) {
			key := visible[m.secretCursor]
			val := m.secretData.Data[key]
			m.setStatusMessage("Copied value of "+key, false)
			return m, tea.Batch(copyToSystemClipboard(val), scheduleStatusClear())
		}
		return m, nil
	case "enter":
		// Save the secret only if something changed. If nothing changed,
		// close the overlay silently so Enter can double as "done".
		if !m.secretDataDirty() {
			m.overlay = overlayNone
			m.secretData = nil
			m.secretDataOriginal = nil
			return m, nil
		}
		if m.readOnly {
			m.setStatusMessage(readOnlyBlockedMessage("Secret Editor"), true)
			return m, scheduleStatusClear()
		}
		return m, m.saveSecretData()
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleConfigMapEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.configMapData == nil {
		m.overlay = overlayNone
		return m, nil
	}

	// Handle editing mode.
	if m.configMapEditing {
		return m.handleConfigMapEditorEditKey(msg)
	}

	if m.editorSearch.active {
		return m.handleEditorSearchKey(msg, clampConfigMapCursorToVisible)
	}

	// Normal mode.
	switch msg.String() {
	case "esc", "q":
		return m.handleConfigMapEditorKeyEsc()
	case "/":
		m.editorSearch.active = true
		m.editorSearch.query.Clear()
		m.configMapCursor = 0
		return m, nil
	case "j", "down":
		return m.handleConfigMapEditorKeyJ()
	case "k", "up":
		return m.handleConfigMapEditorKeyK()
	case "e":
		// Edit selected value.
		return m.handleConfigMapEditorKeyE()
	case "a":
		// Add new key-value pair. Clear search so the new row is visible.
		m.resetEditorSearch()
		newKey := fmt.Sprintf("new-key-%d", len(m.configMapData.Keys)+1)
		m.configMapData.Keys = append(m.configMapData.Keys, newKey)
		m.configMapData.Data[newKey] = ""
		m.configMapCursor = len(m.configMapData.Keys) - 1
		m.configMapEditing = true
		m.configMapEditColumn = 0
		m.configMapEditKey.Set(newKey)
		m.configMapEditValue.Clear()
		return m, nil
	case "D":
		// Delete selected row.
		return m.handleConfigMapEditorKeyD()
	case "y":
		// Copy current value to clipboard.
		visible := m.configMapVisibleKeys()
		if m.configMapCursor >= 0 && m.configMapCursor < len(visible) {
			key := visible[m.configMapCursor]
			val := m.configMapData.Data[key]
			m.setStatusMessage("Copied value of "+key, false)
			return m, tea.Batch(copyToSystemClipboard(val), scheduleStatusClear())
		}
		return m, nil
	case "enter":
		// Save the configmap only if something changed. If nothing
		// changed, close the overlay silently.
		if !m.configMapDataDirty() {
			m.overlay = overlayNone
			m.configMapData = nil
			m.configMapDataOriginal = nil
			return m, nil
		}
		if m.readOnly {
			m.setStatusMessage(readOnlyBlockedMessage("ConfigMap Editor"), true)
			return m, scheduleStatusClear()
		}
		return m, m.saveConfigMapData()
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleAutoSyncKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		return m, nil
	case "j", "down":
		if m.autoSyncCursor < 2 {
			m.autoSyncCursor++
		}
		return m, nil
	case "k", "up":
		if m.autoSyncCursor > 0 {
			m.autoSyncCursor--
		}
		return m, nil
	case " ":
		switch m.autoSyncCursor {
		case 0:
			m.autoSyncEnabled = !m.autoSyncEnabled
		case 1:
			if m.autoSyncEnabled {
				m.autoSyncSelfHeal = !m.autoSyncSelfHeal
			}
		case 2:
			if m.autoSyncEnabled {
				m.autoSyncPrune = !m.autoSyncPrune
			}
		}
		return m, nil
	case "enter", "ctrl+s":
		if m.readOnly {
			m.setStatusMessage(readOnlyBlockedMessage("Auto Sync"), true)
			return m, scheduleStatusClear()
		}
		return m, m.saveAutoSyncConfig()
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleLabelEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.labelData == nil {
		m.overlay = overlayNone
		return m, nil
	}

	currentKeys := m.labelData.LabelKeys
	currentData := m.labelData.Labels
	if m.labelTab == 1 {
		currentKeys = m.labelData.AnnotKeys
		currentData = m.labelData.Annotations
	}

	if m.labelEditing {
		return m.handleLabelEditorEditKey(msg, currentKeys, currentData)
	}

	if m.editorSearch.active {
		return m.handleEditorSearchKey(msg, clampLabelCursorToVisible)
	}

	visible := m.labelVisibleKeys()

	switch msg.String() {
	case "esc", "q":
		return m.handleLabelEditorKeyEsc()
	case "/":
		m.editorSearch.active = true
		m.editorSearch.query.Clear()
		m.labelCursor = 0
		return m, nil
	case "tab":
		// Switch between labels and annotations tabs. Reset cursor +
		// search since the active key list changes.
		m.resetEditorSearch()
		return m.handleLabelEditorKeyTab()
	case "j", "down":
		if m.labelCursor < len(visible)-1 {
			m.labelCursor++
		}
		return m, nil
	case "k", "up":
		return m.handleLabelEditorKeyK()
	case "e":
		if m.labelCursor >= 0 && m.labelCursor < len(visible) {
			key := visible[m.labelCursor]
			m.labelEditing = true
			m.labelEditColumn = 1
			m.labelEditKey.Set(key)
			m.labelEditValue.Set(currentData[key])
		}
		return m, nil
	case "a":
		// Clear search so the new row is visible.
		m.resetEditorSearch()
		newKey := fmt.Sprintf("new-key-%d", len(currentKeys)+1)
		currentKeys = append(currentKeys, newKey)
		currentData[newKey] = ""
		if m.labelTab == 0 {
			m.labelData.LabelKeys = currentKeys
		} else {
			m.labelData.AnnotKeys = currentKeys
		}
		m.labelCursor = len(currentKeys) - 1
		m.labelEditing = true
		m.labelEditColumn = 0
		m.labelEditKey.Set(newKey)
		m.labelEditValue.Clear()
		return m, nil
	case "D":
		if m.labelCursor < 0 || m.labelCursor >= len(visible) {
			return m, nil
		}
		key := visible[m.labelCursor]
		delete(currentData, key)
		for i, k := range currentKeys {
			if k == key {
				currentKeys = append(currentKeys[:i], currentKeys[i+1:]...)
				break
			}
		}
		if m.labelTab == 0 {
			m.labelData.LabelKeys = currentKeys
		} else {
			m.labelData.AnnotKeys = currentKeys
		}
		clampLabelCursorToVisible(&m)
		return m, nil
	case "y":
		if m.labelCursor >= 0 && m.labelCursor < len(currentKeys) {
			key := currentKeys[m.labelCursor]
			val := currentData[key]
			m.setStatusMessage("Copied value of "+key, false)
			return m, tea.Batch(copyToSystemClipboard(val), scheduleStatusClear())
		}
		return m, nil
	case "enter":
		// Save labels/annotations only if something changed. If nothing
		// changed, close the overlay silently.
		if !m.labelDataDirty() {
			m.overlay = overlayNone
			m.labelData = nil
			m.labelLabelsOriginal = nil
			m.labelAnnotationsOriginal = nil
			return m, nil
		}
		if m.readOnly {
			m.setStatusMessage(readOnlyBlockedMessage("Labels / Annotations"), true)
			return m, scheduleStatusClear()
		}
		return m, m.saveLabelData()
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleSecretEditorKeyEsc() (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	m.secretData = nil
	m.secretDataOriginal = nil
	m.resetEditorSearch()
	return m, nil
}

func (m Model) handleSecretEditorKeyJ() (tea.Model, tea.Cmd) {
	visible := m.secretVisibleKeys()
	if m.secretCursor < len(visible)-1 {
		m.secretCursor++
	}
	return m, nil
}

func (m Model) handleSecretEditorKeyK() (tea.Model, tea.Cmd) {
	if m.secretCursor > 0 {
		m.secretCursor--
	}
	return m, nil
}

func (m Model) handleSecretEditorKeyV() (tea.Model, tea.Cmd) {
	visible := m.secretVisibleKeys()
	if m.secretCursor >= 0 && m.secretCursor < len(visible) {
		key := visible[m.secretCursor]
		m.secretRevealed[key] = !m.secretRevealed[key]
	}
	return m, nil
}

func (m Model) handleSecretEditorKeyE() (tea.Model, tea.Cmd) {
	visible := m.secretVisibleKeys()
	if m.secretCursor >= 0 && m.secretCursor < len(visible) {
		key := visible[m.secretCursor]
		m.secretEditing = true
		m.secretEditColumn = 1
		m.secretEditKey.Set(key)
		m.secretEditValue.Set(m.secretData.Data[key])
	}
	return m, nil
}

func (m Model) handleSecretEditorKeyD() (tea.Model, tea.Cmd) {
	visible := m.secretVisibleKeys()
	if m.secretCursor < 0 || m.secretCursor >= len(visible) {
		return m, nil
	}
	key := visible[m.secretCursor]
	// Delete from the underlying maps + the canonical Keys order.
	delete(m.secretData.Data, key)
	for i, k := range m.secretData.Keys {
		if k == key {
			m.secretData.Keys = append(m.secretData.Keys[:i], m.secretData.Keys[i+1:]...)
			break
		}
	}
	clampSecretCursorToVisible(&m)
	return m, nil
}

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
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			activeInput.Insert(key)
		}
		return m, nil
	}
}

func (m Model) handleLabelEditorKeyEsc() (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	m.labelData = nil
	m.labelLabelsOriginal = nil
	m.labelAnnotationsOriginal = nil
	m.resetEditorSearch()
	return m, nil
}

func (m Model) handleLabelEditorKeyTab() (tea.Model, tea.Cmd) {
	m.labelTab = (m.labelTab + 1) % 2
	m.labelCursor = 0
	return m, nil
}

func (m Model) handleLabelEditorKeyK() (tea.Model, tea.Cmd) {
	if m.labelCursor > 0 {
		m.labelCursor--
	}
	return m, nil
}

func (m Model) handleConfigMapEditorKeyEsc() (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	m.configMapData = nil
	m.configMapDataOriginal = nil
	m.resetEditorSearch()
	return m, nil
}

func (m Model) handleConfigMapEditorKeyJ() (tea.Model, tea.Cmd) {
	visible := m.configMapVisibleKeys()
	if m.configMapCursor < len(visible)-1 {
		m.configMapCursor++
	}
	return m, nil
}

func (m Model) handleConfigMapEditorKeyK() (tea.Model, tea.Cmd) {
	if m.configMapCursor > 0 {
		m.configMapCursor--
	}
	return m, nil
}

func (m Model) handleConfigMapEditorKeyE() (tea.Model, tea.Cmd) {
	visible := m.configMapVisibleKeys()
	if m.configMapCursor >= 0 && m.configMapCursor < len(visible) {
		key := visible[m.configMapCursor]
		m.configMapEditing = true
		m.configMapEditColumn = 1
		m.configMapEditKey.Set(key)
		m.configMapEditValue.Set(m.configMapData.Data[key])
	}
	return m, nil
}

func (m Model) handleConfigMapEditorKeyD() (tea.Model, tea.Cmd) {
	visible := m.configMapVisibleKeys()
	if m.configMapCursor < 0 || m.configMapCursor >= len(visible) {
		return m, nil
	}
	key := visible[m.configMapCursor]
	delete(m.configMapData.Data, key)
	for i, k := range m.configMapData.Keys {
		if k == key {
			m.configMapData.Keys = append(m.configMapData.Keys[:i], m.configMapData.Keys[i+1:]...)
			break
		}
	}
	clampConfigMapCursorToVisible(&m)
	return m, nil
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
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			activeInput.Insert(key)
		}
		return m, nil
	}
}

// secretDataDirty reports whether the in-memory secret data differs from
// the snapshot taken when the secret was loaded. Used by the Enter-to-save
// handler so it can skip the API call when there is nothing to save.
func (m *Model) secretDataDirty() bool {
	if m.secretData == nil || m.secretDataOriginal == nil {
		return false
	}
	return !stringMapsEqual(m.secretData.Data, m.secretDataOriginal)
}

// configMapDataDirty is the configmap counterpart of secretDataDirty.
func (m *Model) configMapDataDirty() bool {
	if m.configMapData == nil || m.configMapDataOriginal == nil {
		return false
	}
	return !stringMapsEqual(m.configMapData.Data, m.configMapDataOriginal)
}

// labelDataDirty reports whether either the labels map or the annotations
// map has changed since the editor was opened.
func (m *Model) labelDataDirty() bool {
	if m.labelData == nil {
		return false
	}
	if !stringMapsEqual(m.labelData.Labels, m.labelLabelsOriginal) {
		return true
	}
	if !stringMapsEqual(m.labelData.Annotations, m.labelAnnotationsOriginal) {
		return true
	}
	return false
}

// stringMapsEqual returns true when two string→string maps have the same
// set of keys with the same values.
func stringMapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || va != vb {
			return false
		}
	}
	return true
}
