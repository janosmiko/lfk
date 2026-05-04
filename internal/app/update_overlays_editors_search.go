package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// update_overlays_editors_search.go holds the / search plumbing
// shared by the K/V editor overlays (secret, configmap, label).
// Split out from update_overlays_editors.go so the per-editor
// handler bodies can stay under the file-length cap.

// resetEditorSearch clears the / search + multi-row selection +
// format-picker state. Called when an editor overlay opens / closes
// so stale state from a prior session doesn't leak into the next.
func (m *Model) resetEditorSearch() {
	m.editorSearch.active = false
	m.editorSearch.query.Clear()
	m.editorSearch.selected = nil
	m.editorSearch.formatActive = false
	m.editorSearch.formatCursor = 0
	m.editorSearch.editValueScroll = 0
}

// toggleEditorSelection flips the membership of `key` in the multi-
// row selection set. Lazy-init the map so editors that never use
// the feature don't pay the allocation. Removes the key entirely
// (vs setting to false) so iteration only sees actively-selected
// keys — copy paths can range over the map directly.
func (m *Model) toggleEditorSelection(key string) {
	if m.editorSearch.selected == nil {
		m.editorSearch.selected = make(map[string]bool, 4)
	}
	if m.editorSearch.selected[key] {
		delete(m.editorSearch.selected, key)
		if len(m.editorSearch.selected) == 0 {
			m.editorSearch.selected = nil // free the map when empty
		}
	} else {
		m.editorSearch.selected[key] = true
	}
}

// editorFormatPickerStep moves the format-picker cursor by delta,
// clamped to the bounds of ui.KVFormats. Shared so each editor's
// per-key dispatch only needs to call this for h/l (and j/k).
func (m *Model) editorFormatPickerStep(delta int) {
	next := max(m.editorSearch.formatCursor+delta, 0)
	if next >= len(ui.KVFormats) {
		next = len(ui.KVFormats) - 1
	}
	m.editorSearch.formatCursor = next
}

// editorFormatCancel closes the format picker without copying.
func (m *Model) editorFormatCancel() {
	m.editorSearch.formatActive = false
	m.editorSearch.formatCursor = 0
}

// handleEditorSearchKey dispatches keys while the user is typing into
// the / search input. Esc clears + exits, Enter accepts (keeps the
// filter active but exits input mode), printable characters extend
// the query. Cursor clamps to the new filtered list size after each
// edit so the user never lands on an out-of-range index.
func (m Model) handleEditorSearchKey(msg tea.KeyMsg, clampCursor func(*Model)) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editorSearch.active = false
		m.editorSearch.query.Clear()
		clampCursor(&m)
		return m, nil
	case "enter":
		m.editorSearch.active = false
		clampCursor(&m)
		return m, nil
	case "backspace":
		m.editorSearch.query.Backspace()
		clampCursor(&m)
		return m, nil
	}
	if len(msg.Runes) > 0 {
		m.editorSearch.query.Insert(string(msg.Runes))
		clampCursor(&m)
	}
	return m, nil
}

// secretVisibleKeys returns the secret editor's key list narrowed by
// the active / search query. All cursor-based handlers (j/k/v/e/D/y)
// must index into THIS list — using m.secretData.Keys directly would
// silently navigate keys the user can't see.
func (m Model) secretVisibleKeys() []string {
	return ui.FilterKVKeys(m.secretData.Keys, m.editorSearch.query.Value)
}

// clampSecretCursorToVisible keeps m.secretCursor inside the bounds
// of the currently filtered key list. Called after any operation that
// could shrink the visible set (filter typing/clear, key delete).
func clampSecretCursorToVisible(m *Model) {
	visible := m.secretVisibleKeys()
	if len(visible) == 0 {
		m.secretCursor = 0
		return
	}
	if m.secretCursor >= len(visible) {
		m.secretCursor = len(visible) - 1
	}
	if m.secretCursor < 0 {
		m.secretCursor = 0
	}
}

// configMapVisibleKeys returns the configmap editor's key list
// narrowed by the active / search query. See secretVisibleKeys for
// the contract.
func (m Model) configMapVisibleKeys() []string {
	return ui.FilterKVKeys(m.configMapData.Keys, m.editorSearch.query.Value)
}

// clampConfigMapCursorToVisible keeps m.configMapCursor inside the
// bounds of the currently filtered configmap key list.
func clampConfigMapCursorToVisible(m *Model) {
	visible := m.configMapVisibleKeys()
	if len(visible) == 0 {
		m.configMapCursor = 0
		return
	}
	if m.configMapCursor >= len(visible) {
		m.configMapCursor = len(visible) - 1
	}
	if m.configMapCursor < 0 {
		m.configMapCursor = 0
	}
}

// labelVisibleKeys returns the active label tab's key list narrowed
// by the / search query. The active tab is determined by m.labelTab
// (0 = labels, 1 = annotations).
func (m Model) labelVisibleKeys() []string {
	keys := m.labelData.LabelKeys
	if m.labelTab == 1 {
		keys = m.labelData.AnnotKeys
	}
	return ui.FilterKVKeys(keys, m.editorSearch.query.Value)
}

// clampLabelCursorToVisible keeps m.labelCursor inside the bounds of
// the currently filtered label/annotation key list.
func clampLabelCursorToVisible(m *Model) {
	visible := m.labelVisibleKeys()
	if len(visible) == 0 {
		m.labelCursor = 0
		return
	}
	if m.labelCursor >= len(visible) {
		m.labelCursor = len(visible) - 1
	}
	if m.labelCursor < 0 {
		m.labelCursor = 0
	}
}
