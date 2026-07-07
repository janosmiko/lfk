package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// copyFormatPickerHints is the copy-as picker's bottom hint bar
// (kept here to co-locate the feature and keep overlay_hintbar.go
// under the file-length cap).
func copyFormatPickerHints() []ui.HintEntry {
	return []ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: "y/J/t", Desc: "shortcut"},
		{Key: "enter", Desc: "apply"},
		{Key: "esc", Desc: "cancel"},
	}
}

// handleCopyFormatPickerKey routes key events to the active copy-as
// picker. j/k/down/up cycle the cursor (j/k stays consistent with
// the global navigation aliases); enter applies the cursor row;
// esc/q cancel; letter shortcuts apply the matching format directly.
// JSON's shortcut is uppercase "J" so it doesn't collide with the
// lowercase "j" cursor-down alias.
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
	// that return an empty ShortcutKey are skipped.
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
