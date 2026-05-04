package app

import (
	"fmt"

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
	// Format-picker mode: routes h/l/enter/esc through the picker,
	// resolves to a clipboard write on apply. Same precedence as
	// search — must dodge the editor's normal-mode key dispatch.
	if m.editorSearch.formatActive {
		return m.handleSecretFormatPickerKey(msg)
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
	case "s":
		// Toggle current row in the multi-select set. Picked over
		// space (already used by the existing reveal-toggle) and v
		// (also reveal). The set lives across cursor moves so users
		// can select non-adjacent rows.
		return m.handleSecretEditorKeyS()
	case "Y":
		// Open the Shift+Y format picker. Apply target = selected
		// rows if any are marked, else the cursor row. Resolved at
		// apply time inside handleSecretFormatPickerKey.
		m.editorSearch.formatActive = true
		m.editorSearch.formatCursor = 0
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
	// Format-picker mode: routes h/l/enter/esc through the picker, then
	// resolves to a clipboard write on apply. Same precedence as search.
	if m.editorSearch.formatActive {
		return m.handleConfigMapFormatPickerKey(msg)
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
	case "s":
		// Toggle current row in the multi-select set. See secret editor
		// for the broader rationale (persists across cursor moves).
		return m.handleConfigMapEditorKeyS()
	case "Y":
		// Open the format picker — apply target = selected rows if any,
		// else the cursor row. Resolved inside handleConfigMapFormatPickerKey.
		m.editorSearch.formatActive = true
		m.editorSearch.formatCursor = 0
		return m, nil
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
	// Format-picker mode: routes h/l/enter/esc through the picker, then
	// resolves to a clipboard write on apply. Same precedence as search.
	if m.editorSearch.formatActive {
		return m.handleLabelFormatPickerKey(msg)
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
		// search since the active key list changes; selection too,
		// since label and annotation namespaces are disjoint.
		m.resetEditorSearch()
		return m.handleLabelEditorKeyTab()
	case "j", "down":
		if m.labelCursor < len(visible)-1 {
			m.labelCursor++
		}
		return m, nil
	case "k", "up":
		return m.handleLabelEditorKeyK()
	case "s":
		// Toggle current row in the multi-select set. See secret editor.
		return m.handleLabelEditorKeyS()
	case "Y":
		// Open the format picker — apply target = selected rows if any,
		// else the cursor row. Resolved inside handleLabelFormatPickerKey.
		m.editorSearch.formatActive = true
		m.editorSearch.formatCursor = 0
		return m, nil
	case "e":
		if m.labelCursor >= 0 && m.labelCursor < len(visible) {
			key := visible[m.labelCursor]
			m.labelEditing = true
			m.labelEditColumn = 1
			m.labelEditKey.Set(key)
			m.labelEditValue.Set(currentData[key])
			m.editorSearch.editValueScroll = 0
			adjustEditValueScrollFor(&m, m.labelEditValue.Value, m.labelEditValue.Cursor, 1, 2)
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
		return m.handleLabelEditorKeyD(currentKeys, currentData, visible)
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
		// Initial scroll: TextInput.Set() puts the cursor at len(value).
		// For long values that's past the visible window — without this
		// the first frame would show the top of the value with a cursor
		// somewhere off-screen.
		m.editorSearch.editValueScroll = 0
		adjustEditValueScrollFor(&m, m.secretEditValue.Value, m.secretEditValue.Cursor, 1, 1)
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

func (m Model) handleLabelEditorKeyD(currentKeys []string, currentData map[string]string, visible []string) (tea.Model, tea.Cmd) {
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
		m.editorSearch.editValueScroll = 0
		adjustEditValueScrollFor(&m, m.configMapEditValue.Value, m.configMapEditValue.Cursor, 1, 1)
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

// secretDataDirty / configMapDataDirty / labelDataDirty report
// whether the in-memory editor state differs from the snapshot taken
// when the editor was loaded. Used by the Enter-to-save handler so
// it can skip the API call when there's nothing to save.
func (m *Model) secretDataDirty() bool {
	return m.secretData != nil && m.secretDataOriginal != nil &&
		!stringMapsEqual(m.secretData.Data, m.secretDataOriginal)
}

func (m *Model) configMapDataDirty() bool {
	return m.configMapData != nil && m.configMapDataOriginal != nil &&
		!stringMapsEqual(m.configMapData.Data, m.configMapDataOriginal)
}

func (m *Model) labelDataDirty() bool {
	return m.labelData != nil &&
		(!stringMapsEqual(m.labelData.Labels, m.labelLabelsOriginal) ||
			!stringMapsEqual(m.labelData.Annotations, m.labelAnnotationsOriginal))
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
