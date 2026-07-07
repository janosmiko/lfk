package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// copyFieldPickerHints is the field picker's bottom hint bar
// (rendered by overlayHintBarSelector; kept here to co-locate the
// feature and keep overlay_hintbar.go under the file-length cap).
func copyFieldPickerHints() []ui.HintEntry {
	return []ui.HintEntry{
		{Key: "tab", Desc: "columns/fields"},
		{Key: "/", Desc: "filter"},
		{Key: "j/k", Desc: "navigate"},
		{Key: "enter", Desc: "copy value"},
		{Key: "esc", Desc: "close"},
	}
}

// handleCopyFieldPickerKey routes key events to the ctrl+y field
// picker, delegating to the filter-input sub-handler when the filter is
// focused (same split as the Object Explorer's find overlay).
func (m Model) handleCopyFieldPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.copyFieldPicker.filterActive {
		return m.handleCopyFieldPickerFilterKey(msg)
	}
	p := &m.copyFieldPicker
	n := len(m.visibleCopyFieldEntries())
	_, _, visible := m.copyFieldOverlayDims()
	switch msg.String() {
	case "esc", "q":
		if p.filter != "" {
			p.filter = ""
			p.cursor = 0
			p.scroll = 0
			m.recomputeCopyFieldVisible()
			m.clampCopyFieldScroll()
			return m, nil
		}
		m.closeCopyFieldPicker()
		return m, nil
	case "enter":
		return m.applyCopyFieldPicker()
	case "tab":
		m.toggleCopyFieldMode()
		return m, nil
	case "/":
		p.filterActive = true
	case "j", "down", "ctrl+n":
		if p.cursor < n-1 {
			p.cursor++
		}
	case "k", "up", "ctrl+p":
		if p.cursor > 0 {
			p.cursor--
		}
	case "g", "home":
		p.cursor = 0
	case "G", "end":
		p.cursor = max(n-1, 0)
	case "ctrl+d", "pgdown", "shift+down":
		p.cursor = min(p.cursor+visible/2, max(n-1, 0))
	case "ctrl+u", "pgup", "shift+up":
		p.cursor = max(p.cursor-visible/2, 0)
	}
	m.clampCopyFieldScroll()
	return m, nil
}

// handleCopyFieldPickerFilterKey handles typing in the picker's filter input.
func (m Model) handleCopyFieldPickerFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := &m.copyFieldPicker
	switch msg.String() {
	case "esc":
		p.filterActive = false
		p.filter = ""
		p.cursor = 0
		p.scroll = 0
		m.recomputeCopyFieldVisible()
	case "enter":
		// Keep the filter, leave typing mode so j/k navigate the results.
		p.filterActive = false
	case "backspace":
		if p.filter != "" {
			p.filter = p.filter[:len(p.filter)-1]
		}
		p.cursor = 0
		p.scroll = 0
		m.recomputeCopyFieldVisible()
	default:
		if msg.Type == tea.KeyRunes {
			p.filter += string(msg.Runes)
			p.cursor = 0
			p.scroll = 0
			m.recomputeCopyFieldVisible()
		}
	}
	m.clampCopyFieldScroll()
	return m, nil
}
