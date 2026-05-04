package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// update_overlays_editors_configmap_copy.go holds the ConfigMap
// editor's multi-row selection toggle (`s`) and Shift+Y format-picker
// dispatch. Mirrors the Secret variant — kept in its own file so the
// main editor switch stays under the 800-line file-length cap.

// handleConfigMapEditorKeyS toggles the cursor row's key in the
// multi-row selection set, then auto-advances the cursor.
func (m Model) handleConfigMapEditorKeyS() (tea.Model, tea.Cmd) {
	visible := m.configMapVisibleKeys()
	if m.configMapCursor < 0 || m.configMapCursor >= len(visible) {
		return m, nil
	}
	m.toggleEditorSelection(visible[m.configMapCursor])
	if m.configMapCursor < len(visible)-1 {
		m.configMapCursor++
	}
	return m, nil
}

// handleConfigMapFormatPickerKey services the Shift+Y format picker.
// Apply target = selected keys, falling back to the cursor row.
func (m Model) handleConfigMapFormatPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.editorFormatCancel()
		return m, nil
	case "h", "left":
		m.editorFormatPickerStep(-1)
		return m, nil
	case "l", "right":
		m.editorFormatPickerStep(1)
		return m, nil
	case "enter":
		pairs := m.configMapCopyPairs()
		if len(pairs) == 0 {
			m.editorFormatCancel()
			return m, nil
		}
		entry := ui.KVFormats[m.editorSearch.formatCursor]
		out, label := ui.FormatKVPairs(pairs, entry.Format)
		m.editorFormatCancel()
		m.setStatusMessage(fmt.Sprintf("Copied %d configmap pair(s) as %s", len(pairs), label), false)
		return m, tea.Batch(copyToSystemClipboard(out), scheduleStatusClear())
	}
	return m, nil
}

// configMapCopyPairs assembles the [{Key, Value}] payload for the
// Shift+Y copy. Selection wins when set; otherwise the cursor row.
// Order follows m.configMapData.Keys so the payload matches on-screen
// order. Nil-data → empty result; callers treat nil as "nothing to
// copy" and close the picker.
func (m Model) configMapCopyPairs() []ui.KVPair {
	if m.configMapData == nil {
		return nil
	}
	selected := m.editorSearch.selected
	if len(selected) > 0 {
		out := make([]ui.KVPair, 0, len(selected))
		for _, k := range m.configMapData.Keys {
			if selected[k] {
				out = append(out, ui.KVPair{Key: k, Value: m.configMapData.Data[k]})
			}
		}
		return out
	}
	visible := m.configMapVisibleKeys()
	if m.configMapCursor < 0 || m.configMapCursor >= len(visible) {
		return nil
	}
	k := visible[m.configMapCursor]
	return []ui.KVPair{{Key: k, Value: m.configMapData.Data[k]}}
}
