package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/logagg"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleLogTopKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) { //nolint:unparam // tea.Cmd return is part of the action-key handler convention; may carry cmds in future
	kb := ui.ActiveKeybindings
	switch msg.String() {
	case "esc", "q":
		// Pop a drill level, or return to the log viewer.
		if n := len(m.logTop.drillField); n > 0 {
			m.logTop.drillField = m.logTop.drillField[:n-1]
			m.logTop.drillValue = m.logTop.drillValue[:n-1]
			m.logTop.cursor = 0
			m.logTopRebuildRows()
			return m, nil
		}
		m.mode = modeLogs
		return m, nil
	case kb.Down, "j":
		if m.logTop.cursor < len(m.logTop.rows)-1 {
			m.logTop.cursor++
		}
		return m, nil
	case kb.Up, "k":
		if m.logTop.cursor > 0 {
			m.logTop.cursor--
		}
		return m, nil
	case kb.JumpTop, "g":
		m.logTop.cursor = 0
		return m, nil
	case kb.JumpBottom, "G":
		m.logTop.cursor = max(len(m.logTop.rows)-1, 0)
		return m, nil
	case kb.SortNext, kb.SortFlip:
		if m.logTop.sortKey == logagg.SortReq {
			m.logTop.sortKey = logagg.SortErr
		} else {
			m.logTop.sortKey = logagg.SortReq
		}
		m.logTopRebuildRows()
		return m, nil
	case "enter":
		return m.logTopDrillIn(), nil
	}
	return m, nil
}

// logTopDrillIn pins the selected row's group values as constraints and groups
// by the next dimension (status for HTTP, level otherwise).
func (m Model) logTopDrillIn() Model {
	if m.logTop.cursor >= len(m.logTop.rows) {
		return m
	}
	row := m.logTop.rows[m.logTop.cursor]
	for i, field := range m.logTop.groupBy {
		m.logTop.drillField = append(m.logTop.drillField, field)
		m.logTop.drillValue = append(m.logTop.drillValue, row.Values[i])
	}
	next := logagg.FieldStatus
	if !httpProfile(m.logTop.profile) {
		next = logagg.FieldLevel
	}
	m.logTop.groupBy = []string{next}
	m.logTop.cursor = 0
	m.logTopRebuildRows()
	return m
}
