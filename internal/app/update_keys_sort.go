package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleExplorerActionKeySortNext() (tea.Model, tea.Cmd, bool) {
	if !m.sortApplies() {
		return m, nil, true
	}
	colCount := ui.ActiveSortableColumnCount
	if colCount > 0 {
		idx := sortColumnIndex(m.sortColumnName)
		idx = (idx + 1) % colCount
		m.sortColumnName = ui.ActiveSortableColumns[idx]
	}
	m.rememberSort()
	m.sortMiddleItems()
	m.clampCursor()
	m.setStatusMessage("Sort: "+m.sortModeName(), false)
	return m, tea.Batch(m.loadPreview(), scheduleStatusClear()), true
}

func (m Model) handleExplorerActionKeySortPrev() (tea.Model, tea.Cmd, bool) {
	if !m.sortApplies() {
		return m, nil, true
	}
	colCount := ui.ActiveSortableColumnCount
	if colCount > 0 {
		idx := sortColumnIndex(m.sortColumnName)
		idx = (idx - 1 + colCount) % colCount
		m.sortColumnName = ui.ActiveSortableColumns[idx]
	}
	m.rememberSort()
	m.sortMiddleItems()
	m.clampCursor()
	m.setStatusMessage("Sort: "+m.sortModeName(), false)
	return m, tea.Batch(m.loadPreview(), scheduleStatusClear()), true
}

func (m Model) handleExplorerActionKeySortFlip() (tea.Model, tea.Cmd, bool) {
	if !m.sortApplies() {
		return m, nil, true
	}
	m.sortAscending = !m.sortAscending
	m.rememberSort()
	m.sortMiddleItems()
	m.clampCursor()
	m.setStatusMessage("Sort: "+m.sortModeName(), false)
	return m, tea.Batch(m.loadPreview(), scheduleStatusClear()), true
}

func (m Model) handleExplorerActionKeySortReset() (tea.Model, tea.Cmd, bool) {
	if !m.sortApplies() {
		return m, nil, true
	}
	m.sortColumnName = sortColDefault
	m.sortAscending = true
	m.forgetSort()
	m.sortMiddleItems()
	m.clampCursor()
	m.setStatusMessage("Sort: "+m.sortModeName(), false)
	return m, tea.Batch(m.loadPreview(), scheduleStatusClear()), true
}
