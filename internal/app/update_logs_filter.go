package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleLogKeyFilter opens the live text-filter input, seeding it with the
// currently applied filter so reopening shows the existing query.
func (m Model) handleLogKeyFilter() (Model, tea.Cmd) { //nolint:unparam // tea.Cmd return is part of the action-key handler convention; may carry cmds in future
	m.logView.filterActive = true
	m.logView.filterInput.Set(m.logView.filterQuery)
	// Drop any pending count prefix so a digit typed before `f` can't leak
	// into the next motion after the filter input closes.
	m.logView.lineInput = ""
	return m, nil
}

// handleLogFilterKey processes a key while the log filter input is open. The
// filter applies live: every edit re-projects the view via rebuildLogView.
func (m Model) handleLogFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Keep the current filter; just close the input.
		m.logView.filterActive = false
		return m, nil
	case "esc":
		m.logView.filterActive = false
		m.logView.filterInput.Clear()
		m.logView.filterQuery = ""
		m.rebuildLogView()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	case "backspace":
		m.logView.filterInput.Backspace()
	case "ctrl+w":
		m.logView.filterInput.DeleteWord()
	case "ctrl+u":
		m.logView.filterInput.DeleteLine()
	case "ctrl+a":
		m.logView.filterInput.Home()
		return m, nil
	case "ctrl+e":
		m.logView.filterInput.End()
		return m, nil
	case "left":
		m.logView.filterInput.Left()
		return m, nil
	case "right":
		m.logView.filterInput.Right()
		return m, nil
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.logView.filterInput.Insert(key)
		} else {
			return m, nil
		}
	}
	// Live re-projection for any edit that changed the text.
	m.logView.filterQuery = m.logView.filterInput.Value
	m.rebuildLogView()
	return m, nil
}
