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
		idx, ok := sortColumnIndex(m.sortColumnName)
		if !ok {
			// The active sort column is hidden in this layout (e.g. a
			// wide-only column after leaving fullscreen). Enter the visible
			// cycle at the first column instead of skipping past it.
			idx = -1
		}
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
		idx, ok := sortColumnIndex(m.sortColumnName)
		if !ok {
			// The active sort column is hidden in this layout; enter the
			// visible cycle at the last column (issue #339).
			idx = colCount
		}
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
	// The default sort key is Name. If Name is hidden via the column overlay it
	// is absent from the visible sort cycle, so fall back to the first visible
	// column to keep the status bar and header indicator coherent (mirrors the
	// hidden-column guard in Next/Prev).
	if _, ok := sortColumnIndex(m.sortColumnName); !ok && ui.ActiveSortableColumnCount > 0 {
		m.sortColumnName = ui.ActiveSortableColumns[0]
	}
	m.sortAscending = true
	m.forgetSort()
	m.sortMiddleItems()
	m.clampCursor()
	m.setStatusMessage("Sort: "+m.sortModeName(), false)
	return m, tea.Batch(m.loadPreview(), scheduleStatusClear()), true
}
