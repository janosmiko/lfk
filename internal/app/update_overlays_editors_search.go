package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// update_overlays_editors_search.go holds the / search plumbing
// shared by the K/V editor overlays (secret, configmap, label).
// Split out from update_overlays_editors.go so the per-editor
// handler bodies can stay under the file-length cap.

// resetEditorSearch clears the / search state. Called when an editor
// overlay opens so a stale query from a prior session doesn't carry
// over and silently hide rows.
func (m *Model) resetEditorSearch() {
	m.editorSearch.active = false
	m.editorSearch.query.Clear()
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
