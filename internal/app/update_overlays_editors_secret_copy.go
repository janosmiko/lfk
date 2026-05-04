package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// update_overlays_editors_secret_copy.go holds the Secret editor's
// multi-row selection toggle (`s`) and Shift+Y format-picker
// dispatch. Split out so update_overlays_editors.go stays under the
// 800-line file-length cap. ConfigMap and Label will get sibling
// files when their copy paths land in the next slice.

// handleSecretEditorKeyS toggles the cursor row's key in the
// multi-row selection set. Auto-advances the cursor so spamming `s`
// quickly checks off consecutive rows (vim-like vS).
func (m Model) handleSecretEditorKeyS() (tea.Model, tea.Cmd) {
	visible := m.secretVisibleKeys()
	if m.secretCursor < 0 || m.secretCursor >= len(visible) {
		return m, nil
	}
	m.toggleEditorSelection(visible[m.secretCursor])
	if m.secretCursor < len(visible)-1 {
		m.secretCursor++
	}
	return m, nil
}

// handleSecretFormatPickerKey services the Shift+Y format picker.
// h/l/←/→ moves the format cursor; Enter applies (writes the
// selected pairs to the clipboard in the chosen format and closes
// the picker); Esc cancels. Apply target = the keys in
// editorSearch.selected, falling back to the cursor row when no
// selection is active.
func (m Model) handleSecretFormatPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		pairs := m.secretCopyPairs()
		if len(pairs) == 0 {
			m.editorFormatCancel()
			return m, nil
		}
		entry := ui.KVFormats[m.editorSearch.formatCursor]
		out, label := ui.FormatKVPairs(pairs, entry.Format)
		m.editorFormatCancel()
		m.setStatusMessage(fmt.Sprintf("Copied %d secret pair(s) as %s", len(pairs), label), false)
		return m, tea.Batch(copyToSystemClipboard(out), scheduleStatusClear())
	}
	return m, nil
}

// secretCopyPairs assembles the [{Key, Value}] payload for the
// Shift+Y copy. Selection wins when set; otherwise the cursor row.
// Order: m.secretData.Keys traversal order so the clipboard payload
// matches the on-screen order (selection map is unordered).
func (m Model) secretCopyPairs() []ui.KVPair {
	selected := m.editorSearch.selected
	if len(selected) > 0 {
		out := make([]ui.KVPair, 0, len(selected))
		for _, k := range m.secretData.Keys {
			if selected[k] {
				out = append(out, ui.KVPair{Key: k, Value: m.secretData.Data[k]})
			}
		}
		return out
	}
	visible := m.secretVisibleKeys()
	if m.secretCursor < 0 || m.secretCursor >= len(visible) {
		return nil
	}
	k := visible[m.secretCursor]
	return []ui.KVPair{{Key: k, Value: m.secretData.Data[k]}}
}
