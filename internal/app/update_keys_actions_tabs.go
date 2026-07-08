package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Tab action-key handlers (new / next / prev / move), dispatched from
// handleExplorerActionKey in explorer and exec modes.

func (m Model) handleExplorerActionKeyNewTab() (tea.Model, tea.Cmd, bool) {
	if m.unionMode {
		m.setStatusMessage("New tab is not available in union view", true)
		return m, scheduleStatusClear(), true
	}
	if len(m.tabs) >= 9 {
		m.setStatusMessage("Max 9 tabs", true)
		return m, scheduleStatusClear(), true
	}
	m.saveCurrentTab()
	insertAt := m.activeTab + 1
	newTab := m.cloneCurrentTab()
	m.tabs = append(m.tabs[:insertAt], append([]TabState{newTab}, m.tabs[insertAt:]...)...)
	m.activeTab = insertAt
	m.setStatusMessage(fmt.Sprintf("Tab %d created", m.activeTab+1), false)
	return m, scheduleStatusClear(), true
}

func (m Model) handleExplorerActionKeyNextTab() (tea.Model, tea.Cmd, bool) {
	if len(m.tabs) <= 1 {
		return m, nil, true
	}
	m.saveCurrentTab()
	next := (m.activeTab + 1) % len(m.tabs)
	if cmd := m.loadTab(next); cmd != nil {
		return m, cmd, true
	}
	if m.mode == modeExec && m.execPTY != nil {
		return m, m.scheduleExecTick(), true
	}
	return m, m.loadPreview(), true
}

func (m Model) handleExplorerActionKeyPrevTab() (tea.Model, tea.Cmd, bool) {
	if len(m.tabs) <= 1 {
		return m, nil, true
	}
	m.saveCurrentTab()
	prev := (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
	if cmd := m.loadTab(prev); cmd != nil {
		return m, cmd, true
	}
	if m.mode == modeExec && m.execPTY != nil {
		return m, m.scheduleExecTick(), true
	}
	return m, m.loadPreview(), true
}

// handleExplorerActionKeyMoveTab reorders the active tab one slot in the given
// direction (-1 = left, +1 = right). A no-op at the edges or with a single tab
// still claims the key so the brace doesn't leak to another handler.
func (m Model) handleExplorerActionKeyMoveTab(direction int) (tea.Model, tea.Cmd, bool) {
	if m.moveActiveTab(direction) {
		m.setStatusMessage(fmt.Sprintf("Tab moved to position %d", m.activeTab+1), false)
		return m, scheduleStatusClear(), true
	}
	return m, nil, true
}
