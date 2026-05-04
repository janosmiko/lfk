package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// update_overlays_editors_label_copy.go holds the Label editor's
// multi-row selection toggle (`s`) and Shift+Y format-picker
// dispatch. Mirrors Secret/ConfigMap variants — kept in its own file
// so the main editor switch stays under the 800-line cap. Selection
// is scoped to the currently active tab (labels vs annotations);
// switching tabs clears it via resetEditorSearch().

// handleLabelEditorKeyS toggles the cursor row's key in the
// multi-row selection set, then auto-advances the cursor.
func (m Model) handleLabelEditorKeyS() (tea.Model, tea.Cmd) {
	visible := m.labelVisibleKeys()
	if m.labelCursor < 0 || m.labelCursor >= len(visible) {
		return m, nil
	}
	m.toggleEditorSelection(visible[m.labelCursor])
	if m.labelCursor < len(visible)-1 {
		m.labelCursor++
	}
	return m, nil
}

// handleLabelFormatPickerKey services the Shift+Y format picker.
// Apply target = selected keys (within the active tab), falling back
// to the cursor row.
func (m Model) handleLabelFormatPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		pairs := m.labelCopyPairs()
		if len(pairs) == 0 {
			m.editorFormatCancel()
			return m, nil
		}
		entry := ui.KVFormats[m.editorSearch.formatCursor]
		out, label := ui.FormatKVPairs(pairs, entry.Format)
		m.editorFormatCancel()
		kindWord := "label"
		if m.labelTab == 1 {
			kindWord = "annotation"
		}
		m.setStatusMessage(fmt.Sprintf("Copied %d %s pair(s) as %s", len(pairs), kindWord, label), false)
		return m, tea.Batch(copyToSystemClipboard(out), scheduleStatusClear())
	}
	return m, nil
}

// labelCopyPairs assembles the [{Key, Value}] payload for the Shift+Y
// copy, scoped to the active tab. Selection wins when set; otherwise
// the cursor row. Order follows the active tab's key slice so the
// payload matches on-screen order. Nil-data → empty result; callers
// treat nil as "nothing to copy" and close the picker.
func (m Model) labelCopyPairs() []ui.KVPair {
	if m.labelData == nil {
		return nil
	}
	currentKeys := m.labelData.LabelKeys
	currentData := m.labelData.Labels
	if m.labelTab == 1 {
		currentKeys = m.labelData.AnnotKeys
		currentData = m.labelData.Annotations
	}

	selected := m.editorSearch.selected
	if len(selected) > 0 {
		out := make([]ui.KVPair, 0, len(selected))
		for _, k := range currentKeys {
			if selected[k] {
				out = append(out, ui.KVPair{Key: k, Value: currentData[k]})
			}
		}
		return out
	}
	visible := m.labelVisibleKeys()
	if m.labelCursor < 0 || m.labelCursor >= len(visible) {
		return nil
	}
	k := visible[m.labelCursor]
	return []ui.KVPair{{Key: k, Value: currentData[k]}}
}
