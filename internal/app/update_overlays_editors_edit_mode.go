package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// editValuePaneDims returns the value field's content (W, H) inside
// the K/V editor edit pane. titleH varies per editor (1 for Secret +
// ConfigMap, 2 for Label — the latter has a tab bar). Used by the
// page-scroll keys (ctrl+u/d/f/b) and by the sticky-scroll adjuster
// to know how tall the visible window is.
func editValuePaneDims(m Model, titleH int) (w, h int) {
	pW, pH := ui.EditorPanelDims(m.width, m.height, titleH, m.editorSearch.active, m.editorSearch.formatActive)
	return ui.EditValueContentDims(pW, pH)
}

// editPaneMoveByLines moves t's cursor by n hard-newline lines (n>0
// is down, n<0 is up). Stops when no further movement is possible.
// Used to implement ctrl+u/d/f/b in the edit pane — vim semantics
// (cursor moves with the view).
func editPaneMoveByLines(t *TextInput, n int) {
	switch {
	case n > 0:
		for range n {
			before := t.Cursor
			t.Down()
			if t.Cursor == before {
				return
			}
		}
	case n < 0:
		for range -n {
			before := t.Cursor
			t.Up()
			if t.Cursor == before {
				return
			}
		}
	}
}

// handleEditPanePageKey services the page-scroll keys (ctrl+u/d/f/b)
// when col == 1 (value column). Returns (handled, _) — when handled
// is true the caller should return immediately. ctrl+u/d move by half
// the visible page; ctrl+f/b move by a full page.
func handleEditPanePageKey(m Model, valInput *TextInput, col int, key string, titleH int) bool {
	if col != 1 {
		return false
	}
	_, pageH := editValuePaneDims(m, titleH)
	switch key {
	case "ctrl+d":
		editPaneMoveByLines(valInput, max(pageH/2, 1))
		return true
	case "ctrl+u":
		editPaneMoveByLines(valInput, -max(pageH/2, 1))
		return true
	case "ctrl+f":
		editPaneMoveByLines(valInput, pageH)
		return true
	case "ctrl+b":
		editPaneMoveByLines(valInput, -pageH)
		return true
	}
	return false
}

// adjustEditValueScrollFor recomputes the editor's value scroll so
// the cursor stays inside the visible window. Called from each
// editor's edit-mode handler via a `defer` so EVERY cursor-affecting
// case (typing, arrows, ctrl+u/d, etc.) benefits without having to
// remember the call at each return site.
func adjustEditValueScrollFor(m *Model, value string, cursor, col, titleH int) {
	if col != 1 {
		return
	}
	w, h := editValuePaneDims(*m, titleH)
	m.editorSearch.editValueScroll = ui.AdjustEditValueScroll(value, cursor, m.editorSearch.editValueScroll, w, h)
}

// handleSecretEditorEditKey handles key events while editing a secret value.
func (m Model) handleSecretEditorEditKey(msg tea.KeyMsg) (newM tea.Model, cmd tea.Cmd) {
	keyInput := &m.secretEditKey
	valInput := &m.secretEditValue
	col := m.secretEditColumn
	activeInput := valInput
	if col == 0 {
		activeInput = keyInput
	}

	// After the handler decides on a return value, recompute scroll
	// for the value field so cursor mutations from any case (typing,
	// arrows, paste, page-scroll keys) end up with a sane viewport.
	defer func() {
		if rm, ok := newM.(Model); ok && rm.secretEditing {
			adjustEditValueScrollFor(&rm, rm.secretEditValue.Value, rm.secretEditValue.Cursor, rm.secretEditColumn, 1)
			newM = rm
		}
	}()

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

	keyStr := msg.String()
	if handleEditPanePageKey(m, valInput, col, keyStr, 1) {
		return m, nil
	}

	switch keyStr {
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
		// Edit-pane ctrl+a is line-scoped (vim `0`) — single-line key
		// inputs degenerate to buffer-Home() so the binding works in
		// both columns.
		activeInput.LineHome()
		return m, nil
	case "ctrl+e":
		activeInput.LineEnd()
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
		if len(keyStr) == 1 && keyStr[0] >= 32 && keyStr[0] < 127 {
			activeInput.Insert(keyStr)
		}
		return m, nil
	}
}

// handleConfigMapEditorEditKey handles key events while editing a configmap value.
func (m Model) handleConfigMapEditorEditKey(msg tea.KeyMsg) (newM tea.Model, cmd tea.Cmd) {
	keyInput := &m.configMapEditKey
	valInput := &m.configMapEditValue
	col := m.configMapEditColumn
	activeInput := valInput
	if col == 0 {
		activeInput = keyInput
	}

	defer func() {
		if rm, ok := newM.(Model); ok && rm.configMapEditing {
			adjustEditValueScrollFor(&rm, rm.configMapEditValue.Value, rm.configMapEditValue.Cursor, rm.configMapEditColumn, 1)
			newM = rm
		}
	}()

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

	keyStr := msg.String()
	if handleEditPanePageKey(m, valInput, col, keyStr, 1) {
		return m, nil
	}

	switch keyStr {
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
		activeInput.LineHome()
		return m, nil
	case "ctrl+e":
		activeInput.LineEnd()
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
		if len(keyStr) == 1 && keyStr[0] >= 32 && keyStr[0] < 127 {
			activeInput.Insert(keyStr)
		}
		return m, nil
	}
}

// handleLabelEditorEditKey handles key events while editing a label/annotation value.
func (m Model) handleLabelEditorEditKey(msg tea.KeyMsg, currentKeys []string, currentData map[string]string) (newM tea.Model, cmd tea.Cmd) {
	keyInput := &m.labelEditKey
	valInput := &m.labelEditValue
	col := m.labelEditColumn
	activeInput := valInput
	if col == 0 {
		activeInput = keyInput
	}

	defer func() {
		if rm, ok := newM.(Model); ok && rm.labelEditing {
			adjustEditValueScrollFor(&rm, rm.labelEditValue.Value, rm.labelEditValue.Cursor, rm.labelEditColumn, 2)
			newM = rm
		}
	}()

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

	keyStr := msg.String()
	if handleEditPanePageKey(m, valInput, col, keyStr, 2) {
		return m, nil
	}

	switch keyStr {
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
	case "enter":
		// Same contract as Secret/ConfigMap: Enter inserts a literal
		// newline only when editing the value column. Without this,
		// pressing Enter on a label value would silently drop into
		// the default branch and the user couldn't switch a label
		// from inline → multi-line edit mode.
		if col == 1 {
			valInput.Insert("\n")
		}
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
		activeInput.LineHome()
		return m, nil
	case "ctrl+e":
		activeInput.LineEnd()
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
		if len(keyStr) == 1 && keyStr[0] >= 32 && keyStr[0] < 127 {
			activeInput.Insert(keyStr)
		}
		return m, nil
	}
}
