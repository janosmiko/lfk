package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleLogTopKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) { //nolint:unparam // tea.Cmd return is part of the action-key handler convention; may carry cmds in future
	kb := ui.ActiveKeybindings
	switch msg.String() {
	case "esc", "q":
		// Pop a drill level, or return to the log viewer.
		if n := len(m.logTop.drillStack); n > 0 {
			top := m.logTop.drillStack[n-1]
			m.logTop.groupBy = top.groupBy
			m.logTop.drillStack = m.logTop.drillStack[:n-1]
			m.logTop.cursor = 0
			m.logTopRebuildRows()
			return m, nil
		}
		(&m).rebuildLogView() // populate logView.lines before returning to the viewer
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
	case kb.JumpBottom, "G":
		m.logTop.cursor = max(len(m.logTop.rows)-1, 0)
		return m, nil
	case "g":
		return m.openLogTopGroupBy(), nil
	case "p":
		return m.openLogTopProfile(), nil
	case kb.SortNext:
		m.logTopCycleSort(+1)
		return m, nil
	case kb.SortPrev:
		m.logTopCycleSort(-1)
		return m, nil
	case kb.SortFlip:
		m.logTop.sortAsc = !m.logTop.sortAsc
		m.logTopRefreshRows()
		return m, nil
	case kb.SortReset:
		m.logTop.sortCol = logTopMetricREQ
		m.logTop.sortAsc = false
		m.logTopRefreshRows()
		return m, nil
	case "enter":
		return m.logTopDrillIn(), nil
	}
	return m, nil
}

// logTopDrillIn pins the selected row's group values as a new frame and groups
// by the next unused display dimension. If every dimension is already pinned,
// the drill is a no-op.
func (m Model) logTopDrillIn() Model {
	if m.logTop.cursor >= len(m.logTop.rows) {
		return m
	}
	next := m.logTopNextDrillDim()
	if next == "" {
		return m // nothing left to break down
	}
	row := m.logTop.rows[m.logTop.cursor]
	frame := logTopDrillFrame{groupBy: append([]string(nil), m.logTop.groupBy...)}
	for i, field := range m.logTop.groupBy {
		frame.filters = append(frame.filters, logTopDrillFilter{field: field, value: row.Values[i]})
	}
	m.logTop.drillStack = append(m.logTop.drillStack, frame)
	m.logTop.groupBy = []string{next}
	m.logTop.cursor = 0
	m.logTopRebuildRows()
	return m
}

// logTopNextDrillDim returns the first display dimension that is not already
// part of the current groupBy or pinned by an active drill filter, or "" if
// every dimension is already used.
func (m *Model) logTopNextDrillDim() string {
	pinned := map[string]bool{}
	for _, g := range m.logTop.groupBy {
		pinned[g] = true
	}
	for _, fr := range m.logTop.drillStack {
		for _, flt := range fr.filters {
			pinned[flt.field] = true
		}
	}
	for _, d := range m.logTop.displayDims {
		if !pinned[d] {
			return d
		}
	}
	return ""
}
