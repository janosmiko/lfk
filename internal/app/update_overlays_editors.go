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
		switch msg.String() {
		case "esc":
			m.secretEditing = false
			m.secretEditColumn = -1
			return m, nil
		case "ctrl+s":
			// Save the edit.
			if m.secretEditColumn == 0 {
				// Renaming key.
				oldKey := m.secretData.Keys[m.secretCursor]
				newKey := m.secretEditKey.Value
				if newKey != "" && newKey != oldKey {
					val := m.secretData.Data[oldKey]
					delete(m.secretData.Data, oldKey)
					m.secretData.Data[newKey] = val
					m.secretData.Keys[m.secretCursor] = newKey
				}
			} else {
				// Editing value.
				key := m.secretData.Keys[m.secretCursor]
				m.secretData.Data[key] = m.secretEditValue.Value
			}
			m.secretEditing = false
			m.secretEditColumn = -1
			return m, nil
		case "enter":
			// Insert newline into current value.
			if m.secretEditColumn == 1 {
				m.secretEditValue.Insert("\n")
			}
			return m, nil
		case "tab":
			// Switch between key and value columns.
			if m.secretEditColumn == 0 {
				m.secretEditColumn = 1
			} else {
				m.secretEditColumn = 0
			}
			return m, nil
		case "backspace":
			if m.secretEditColumn == 0 && len(m.secretEditKey.Value) > 0 {
				m.secretEditKey.Backspace()
			} else if m.secretEditColumn == 1 && len(m.secretEditValue.Value) > 0 {
				m.secretEditValue.Backspace()
			}
			return m, nil
		case "ctrl+w":
			if m.secretEditColumn == 0 {
				m.secretEditKey.DeleteWord()
			} else {
				m.secretEditValue.DeleteWord()
			}
			return m, nil
		case "ctrl+a":
			if m.secretEditColumn == 0 {
				m.secretEditKey.Home()
			} else {
				m.secretEditValue.Home()
			}
			return m, nil
		case "ctrl+e":
			if m.secretEditColumn == 0 {
				m.secretEditKey.End()
			} else {
				m.secretEditValue.End()
			}
			return m, nil
		case "left":
			if m.secretEditColumn == 0 {
				m.secretEditKey.Left()
			} else {
				m.secretEditValue.Left()
			}
			return m, nil
		case "right":
			if m.secretEditColumn == 0 {
				m.secretEditKey.Right()
			} else {
				m.secretEditValue.Right()
			}
			return m, nil
		default:
			key := msg.String()
			if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
				if m.secretEditColumn == 0 {
					m.secretEditKey.Insert(key)
				} else {
					m.secretEditValue.Insert(key)
				}
			}
			return m, nil
		}
	}

	// Normal mode.
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.secretData = nil
		return m, nil
	case "j", "down":
		if m.secretCursor < len(m.secretData.Keys)-1 {
			m.secretCursor++
		}
		return m, nil
	case "k", "up":
		if m.secretCursor > 0 {
			m.secretCursor--
		}
		return m, nil
	case "v":
		// Toggle visibility for selected row.
		if m.secretCursor >= 0 && m.secretCursor < len(m.secretData.Keys) {
			key := m.secretData.Keys[m.secretCursor]
			m.secretRevealed[key] = !m.secretRevealed[key]
		}
		return m, nil
	case "V":
		// Toggle all values visibility.
		m.secretAllRevealed = !m.secretAllRevealed
		return m, nil
	case "e":
		// Edit selected value.
		if m.secretCursor >= 0 && m.secretCursor < len(m.secretData.Keys) {
			key := m.secretData.Keys[m.secretCursor]
			m.secretEditing = true
			m.secretEditColumn = 1
			m.secretEditKey.Set(key)
			m.secretEditValue.Set(m.secretData.Data[key])
		}
		return m, nil
	case "a":
		// Add new key-value pair.
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
		if m.secretCursor >= 0 && m.secretCursor < len(m.secretData.Keys) {
			key := m.secretData.Keys[m.secretCursor]
			delete(m.secretData.Data, key)
			m.secretData.Keys = append(m.secretData.Keys[:m.secretCursor], m.secretData.Keys[m.secretCursor+1:]...)
			if m.secretCursor >= len(m.secretData.Keys) && m.secretCursor > 0 {
				m.secretCursor--
			}
		}
		return m, nil
	case "y":
		// Copy current value to clipboard.
		if m.secretCursor >= 0 && m.secretCursor < len(m.secretData.Keys) {
			key := m.secretData.Keys[m.secretCursor]
			val := m.secretData.Data[key]
			m.setStatusMessage("Copied value of "+key, false)
			return m, tea.Batch(copyToSystemClipboard(val), scheduleStatusClear())
		}
		return m, nil
	case "s":
		// Save the secret.
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
		switch msg.String() {
		case "esc":
			m.configMapEditing = false
			m.configMapEditColumn = -1
			return m, nil
		case "ctrl+s":
			// Save the edit.
			if m.configMapEditColumn == 0 {
				// Renaming key.
				oldKey := m.configMapData.Keys[m.configMapCursor]
				newKey := m.configMapEditKey.Value
				if newKey != "" && newKey != oldKey {
					val := m.configMapData.Data[oldKey]
					delete(m.configMapData.Data, oldKey)
					m.configMapData.Data[newKey] = val
					m.configMapData.Keys[m.configMapCursor] = newKey
				}
			} else {
				// Editing value.
				key := m.configMapData.Keys[m.configMapCursor]
				m.configMapData.Data[key] = m.configMapEditValue.Value
			}
			m.configMapEditing = false
			m.configMapEditColumn = -1
			return m, nil
		case "enter":
			// Insert newline into current value.
			if m.configMapEditColumn == 1 {
				m.configMapEditValue.Insert("\n")
			}
			return m, nil
		case "tab":
			// Switch between key and value columns.
			if m.configMapEditColumn == 0 {
				m.configMapEditColumn = 1
			} else {
				m.configMapEditColumn = 0
			}
			return m, nil
		case "backspace":
			if m.configMapEditColumn == 0 && len(m.configMapEditKey.Value) > 0 {
				m.configMapEditKey.Backspace()
			} else if m.configMapEditColumn == 1 && len(m.configMapEditValue.Value) > 0 {
				m.configMapEditValue.Backspace()
			}
			return m, nil
		case "ctrl+w":
			if m.configMapEditColumn == 0 {
				m.configMapEditKey.DeleteWord()
			} else {
				m.configMapEditValue.DeleteWord()
			}
			return m, nil
		case "ctrl+a":
			if m.configMapEditColumn == 0 {
				m.configMapEditKey.Home()
			} else {
				m.configMapEditValue.Home()
			}
			return m, nil
		case "ctrl+e":
			if m.configMapEditColumn == 0 {
				m.configMapEditKey.End()
			} else {
				m.configMapEditValue.End()
			}
			return m, nil
		case "left":
			if m.configMapEditColumn == 0 {
				m.configMapEditKey.Left()
			} else {
				m.configMapEditValue.Left()
			}
			return m, nil
		case "right":
			if m.configMapEditColumn == 0 {
				m.configMapEditKey.Right()
			} else {
				m.configMapEditValue.Right()
			}
			return m, nil
		default:
			key := msg.String()
			if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
				if m.configMapEditColumn == 0 {
					m.configMapEditKey.Insert(key)
				} else {
					m.configMapEditValue.Insert(key)
				}
			}
			return m, nil
		}
	}

	// Normal mode.
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.configMapData = nil
		return m, nil
	case "j", "down":
		if m.configMapCursor < len(m.configMapData.Keys)-1 {
			m.configMapCursor++
		}
		return m, nil
	case "k", "up":
		if m.configMapCursor > 0 {
			m.configMapCursor--
		}
		return m, nil
	case "e":
		// Edit selected value.
		if m.configMapCursor >= 0 && m.configMapCursor < len(m.configMapData.Keys) {
			key := m.configMapData.Keys[m.configMapCursor]
			m.configMapEditing = true
			m.configMapEditColumn = 1
			m.configMapEditKey.Set(key)
			m.configMapEditValue.Set(m.configMapData.Data[key])
		}
		return m, nil
	case "a":
		// Add new key-value pair.
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
		if m.configMapCursor >= 0 && m.configMapCursor < len(m.configMapData.Keys) {
			key := m.configMapData.Keys[m.configMapCursor]
			delete(m.configMapData.Data, key)
			m.configMapData.Keys = append(m.configMapData.Keys[:m.configMapCursor], m.configMapData.Keys[m.configMapCursor+1:]...)
			if m.configMapCursor >= len(m.configMapData.Keys) && m.configMapCursor > 0 {
				m.configMapCursor--
			}
		}
		return m, nil
	case "y":
		// Copy current value to clipboard.
		if m.configMapCursor >= 0 && m.configMapCursor < len(m.configMapData.Keys) {
			key := m.configMapData.Keys[m.configMapCursor]
			val := m.configMapData.Data[key]
			m.setStatusMessage("Copied value of "+key, false)
			return m, tea.Batch(copyToSystemClipboard(val), scheduleStatusClear())
		}
		return m, nil
	case "s":
		// Save the configmap.
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
	case " ", "enter":
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
	case "ctrl+s":
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
		switch msg.String() {
		case "esc":
			m.labelEditing = false
			m.labelEditColumn = -1
			return m, nil
		case "ctrl+s":
			// Save the edit.
			if m.labelEditColumn == 0 {
				oldKey := currentKeys[m.labelCursor]
				newKey := m.labelEditKey.Value
				if newKey != "" && newKey != oldKey {
					val := currentData[oldKey]
					delete(currentData, oldKey)
					currentData[newKey] = val
					currentKeys[m.labelCursor] = newKey
				}
			} else {
				key := currentKeys[m.labelCursor]
				currentData[key] = m.labelEditValue.Value
			}
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
			// Switch between key and value columns.
			if m.labelEditColumn == 0 {
				m.labelEditColumn = 1
			} else {
				m.labelEditColumn = 0
			}
			return m, nil
		case "backspace":
			if m.labelEditColumn == 0 && len(m.labelEditKey.Value) > 0 {
				m.labelEditKey.Backspace()
			} else if m.labelEditColumn == 1 && len(m.labelEditValue.Value) > 0 {
				m.labelEditValue.Backspace()
			}
			return m, nil
		case "ctrl+w":
			if m.labelEditColumn == 0 {
				m.labelEditKey.DeleteWord()
			} else {
				m.labelEditValue.DeleteWord()
			}
			return m, nil
		case "ctrl+a":
			if m.labelEditColumn == 0 {
				m.labelEditKey.Home()
			} else {
				m.labelEditValue.Home()
			}
			return m, nil
		case "ctrl+e":
			if m.labelEditColumn == 0 {
				m.labelEditKey.End()
			} else {
				m.labelEditValue.End()
			}
			return m, nil
		case "left":
			if m.labelEditColumn == 0 {
				m.labelEditKey.Left()
			} else {
				m.labelEditValue.Left()
			}
			return m, nil
		case "right":
			if m.labelEditColumn == 0 {
				m.labelEditKey.Right()
			} else {
				m.labelEditValue.Right()
			}
			return m, nil
		default:
			key := msg.String()
			if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
				if m.labelEditColumn == 0 {
					m.labelEditKey.Insert(key)
				} else {
					m.labelEditValue.Insert(key)
				}
			}
			return m, nil
		}
	}

	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.labelData = nil
		return m, nil
	case "tab":
		// Switch between labels and annotations tabs.
		m.labelTab = (m.labelTab + 1) % 2
		m.labelCursor = 0
		return m, nil
	case "j", "down":
		if m.labelCursor < len(currentKeys)-1 {
			m.labelCursor++
		}
		return m, nil
	case "k", "up":
		if m.labelCursor > 0 {
			m.labelCursor--
		}
		return m, nil
	case "e":
		if m.labelCursor >= 0 && m.labelCursor < len(currentKeys) {
			key := currentKeys[m.labelCursor]
			m.labelEditing = true
			m.labelEditColumn = 1
			m.labelEditKey.Set(key)
			m.labelEditValue.Set(currentData[key])
		}
		return m, nil
	case "a":
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
		if m.labelCursor >= 0 && m.labelCursor < len(currentKeys) {
			key := currentKeys[m.labelCursor]
			delete(currentData, key)
			currentKeys = append(currentKeys[:m.labelCursor], currentKeys[m.labelCursor+1:]...)
			if m.labelTab == 0 {
				m.labelData.LabelKeys = currentKeys
			} else {
				m.labelData.AnnotKeys = currentKeys
			}
			if m.labelCursor >= len(currentKeys) && m.labelCursor > 0 {
				m.labelCursor--
			}
		}
		return m, nil
	case "y":
		if m.labelCursor >= 0 && m.labelCursor < len(currentKeys) {
			key := currentKeys[m.labelCursor]
			val := currentData[key]
			m.setStatusMessage("Copied value of "+key, false)
			return m, tea.Batch(copyToSystemClipboard(val), scheduleStatusClear())
		}
		return m, nil
	case "s":
		return m, m.saveLabelData()
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}
