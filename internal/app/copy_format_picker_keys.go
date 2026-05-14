package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleCopyFormatPickerKey routes key events to the active copy-as
// picker. j/k/down/up cycle the cursor; enter applies the cursor row;
// esc/q cancel; lowercase letter shortcuts apply the matching format
// directly. The letter `j` is reserved for cursor-down navigation
// (cursor movement wins over the shadow JSON shortcut), so JSON must
// be applied via Enter or by typing the up arrow / k to navigate up
// to its row.
func (m Model) handleCopyFormatPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.closeCopyFormatPicker()
		return m, nil
	case "j", "down", "ctrl+n":
		m.copyFormatPickerStep(1)
		return m, nil
	case "k", "up", "ctrl+p":
		m.copyFormatPickerStep(-1)
		return m, nil
	case "enter":
		return m.applyCopyFormatPicker()
	}
	// Letter shortcuts: apply the matching format directly. Formats
	// that return an empty ShortcutKey (today: JSON, which would
	// otherwise collide with cursor-down) are skipped.
	pressed := msg.String()
	for i, f := range m.copyFormatPicker.formats {
		key := f.ShortcutKey()
		if key == "" {
			continue
		}
		if pressed == key {
			m.copyFormatPicker.cursor = i
			return m.applyCopyFormatPicker()
		}
	}
	return m, nil
}
