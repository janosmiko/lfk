package app

import (
	"maps"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/logagg"
)

// logTopGroupByCandidates returns a sorted list of all field names seen in the
// currently parsed lines.
func (m Model) logTopGroupByCandidates() []string {
	seen := map[string]bool{}
	for _, f := range m.logTop.parsed {
		for k := range f {
			if k == logagg.FieldDurationMS {
				continue
			}
			seen[k] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// openLogTopGroupBy opens the group-by multi-select overlay, seeded from the
// current groupBy selection.
func (m Model) openLogTopGroupBy() Model {
	m.overlay = overlayLogTopGroupBy
	m.overlayCursor = 0
	m.logTop.pendingGroup = map[string]bool{}
	for _, g := range m.logTop.groupBy {
		m.logTop.pendingGroup[g] = true
	}
	return m
}

// handleLogTopGroupByKey handles key presses inside the group-by overlay.
func (m Model) handleLogTopGroupByKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) { //nolint:unparam // tea.Cmd return is part of the action-key handler convention; may carry cmds in future
	cands := m.logTopGroupByCandidates()
	switch msg.String() {
	case "esc":
		m.overlay = overlayNone
		return m, nil
	case "j", "down":
		if m.overlayCursor < len(cands)-1 {
			m.overlayCursor++
		}
		return m, nil
	case "k", "up":
		if m.overlayCursor > 0 {
			m.overlayCursor--
		}
		return m, nil
	case " ":
		if m.overlayCursor < len(cands) {
			k := cands[m.overlayCursor]
			m.logTop.pendingGroup[k] = !m.logTop.pendingGroup[k]
		}
		return m, nil
	case "enter":
		sel := make([]string, 0, len(cands))
		for _, c := range cands {
			if m.logTop.pendingGroup[c] {
				sel = append(sel, c)
			}
		}
		if len(sel) > 0 {
			m.logTop.groupBy = sel
		}
		m.logTop.drillStack = nil
		m.logTop.cursor = 0
		m.overlay = overlayNone
		m.logTopRebuildRows()
		return m, nil
	}
	return m, nil
}

// openLogTopProfile opens the profile-picker overlay.
func (m Model) openLogTopProfile() Model {
	m.overlay = overlayLogTopProfile
	m.overlayCursor = 0
	return m
}

// handleLogTopProfileKey handles key presses inside the profile overlay.
func (m Model) handleLogTopProfileKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) { //nolint:unparam // tea.Cmd return is part of the action-key handler convention; may carry cmds in future
	kinds := logagg.AllKinds()
	switch msg.String() {
	case "esc":
		m.overlay = overlayNone
		return m, nil
	case "j", "down":
		if m.overlayCursor < len(kinds)-1 {
			m.overlayCursor++
		}
		return m, nil
	case "k", "up":
		if m.overlayCursor > 0 {
			m.overlayCursor--
		}
		return m, nil
	case "enter":
		if m.overlayCursor < len(kinds) {
			m.logTop.profile = kinds[m.overlayCursor]
			m.logTop.autoProf = false
			m.logTop.groupBy = nil
			m.logTopReparseExisting()
		}
		m.overlay = overlayNone
		return m, nil
	}
	return m, nil
}

// logTopReparseExisting re-parses rawLines under the current manually chosen
// profile without re-running detection.
func (m *Model) logTopReparseExisting() {
	m.logTop.parsed = m.logTop.parsed[:0]
	m.logTop.unmatched = 0
	m.logTop.firstTS = 0
	m.logTop.lastTS = 0
	for _, line := range m.logView.rawLines {
		m.logTopParseInto(line)
	}
	if len(m.logTop.groupBy) == 0 {
		m.logTop.groupBy = defaultGroupBy(m.logTop.profile, m.logTop.parsed)
	}
	m.logTopRebuildRows()
}

// openLogTopColumns opens the column show/hide+reorder overlay, snapshotting
// current state for esc-cancel.
func (m Model) openLogTopColumns() Model {
	m.overlay = overlayLogTopColumns
	m.overlayCursor = 0
	m.logTop.colSnapOrder = append([]string(nil), m.logTop.colOrder...)
	if m.logTop.colHidden != nil {
		m.logTop.colSnapHidden = maps.Clone(m.logTop.colHidden)
	} else {
		m.logTop.colSnapHidden = nil
	}
	return m
}

// logTopColumnList returns the full ordered column list (dims then metrics).
func (m *Model) logTopColumnList() []string {
	return append(append([]string(nil), m.logTop.colOrder...), m.logTopAllMetrics()...)
}

// logTopFilteredColumns returns columns filtered by colFilter (case-insensitive substring).
func (m *Model) logTopFilteredColumns() []string {
	all := m.logTopColumnList()
	if m.logTop.colFilter == "" {
		return all
	}
	q := strings.ToLower(m.logTop.colFilter)
	out := all[:0:0]
	for _, c := range all {
		if strings.Contains(strings.ToLower(c), q) {
			out = append(out, c)
		}
	}
	return out
}

// handleLogTopColumnsKey handles key presses inside the column overlay.
func (m Model) handleLogTopColumnsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) { //nolint:unparam // tea.Cmd return is part of the action-key handler convention
	if m.logTop.colFilterActive {
		return m.handleLogTopColumnsFilterKey(msg)
	}
	cols := m.logTopFilteredColumns()
	maxIdx := len(cols) - 1
	switch msg.String() {
	case "esc", "q":
		m.logTop.colOrder = m.logTop.colSnapOrder
		m.logTop.colHidden = m.logTop.colSnapHidden
		m.logTop.colFilter = ""
		m.logTop.colFilterActive = false
		m.overlay = overlayNone
		m.logTopRebuildRows()
		return m, nil
	case "enter":
		m.logTop.colFilter = ""
		m.logTop.colFilterActive = false
		m.overlay = overlayNone
		m.logTopRebuildRows()
		return m, nil
	case "j", "down":
		if m.overlayCursor < maxIdx {
			m.overlayCursor++
		}
		return m, nil
	case "k", "up":
		if m.overlayCursor > 0 {
			m.overlayCursor--
		}
		return m, nil
	case "ctrl+d", "shift+down":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 10, maxIdx)
		return m, nil
	case "ctrl+u", "shift+up":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -10, maxIdx)
		return m, nil
	case " ":
		if m.overlayCursor >= 0 && m.overlayCursor < len(cols) {
			m = m.logTopColumnsToggleByName(cols[m.overlayCursor])
			if m.overlayCursor < maxIdx {
				m.overlayCursor++
			}
		}
		return m, nil
	case "J":
		return m.logTopColumnsMoveDownByName(cols), nil
	case "K":
		return m.logTopColumnsMoveUpByName(cols), nil
	case "/":
		m.logTop.colFilterActive = true
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleLogTopColumnsFilterKey handles key presses when the column filter input is active.
func (m Model) handleLogTopColumnsFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) { //nolint:unparam
	switch msg.String() {
	case "esc":
		if m.logTop.colFilter != "" {
			m.logTop.colFilter = ""
			m.overlayCursor = 0
		} else {
			m.logTop.colFilterActive = false
		}
		return m, nil
	case "enter":
		m.logTop.colFilterActive = false
		return m, nil
	case "backspace":
		if len(m.logTop.colFilter) > 0 {
			m.logTop.colFilter = m.logTop.colFilter[:len(m.logTop.colFilter)-1]
			m.overlayCursor = 0
		}
		return m, nil
	case "ctrl+w":
		f := strings.TrimRight(m.logTop.colFilter, " ")
		if idx := strings.LastIndex(f, " "); idx >= 0 {
			m.logTop.colFilter = f[:idx+1]
		} else {
			m.logTop.colFilter = ""
		}
		m.overlayCursor = 0
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	k := msg.String()
	if len(k) == 1 && k[0] >= 32 && k[0] < 127 {
		m.logTop.colFilter += k
		m.overlayCursor = 0
	}
	return m, nil
}

func (m Model) logTopColumnsToggleByName(colID string) Model {
	dims := m.logTop.colOrder
	mets := m.logTopAllMetrics()
	if m.logTop.colHidden == nil {
		m.logTop.colHidden = map[string]bool{}
	}
	currentlyHidden := m.logTop.colHidden[colID]
	if !currentlyHidden {
		// Count visible columns across dims and metrics.
		visible := 0
		for _, d := range dims {
			if !m.logTop.colHidden[d] {
				visible++
			}
		}
		for _, met := range mets {
			if !m.logTop.colHidden[met] {
				visible++
			}
		}
		if visible <= 1 {
			return m // last-visible guard
		}
	}
	m.logTop.colHidden[colID] = !currentlyHidden
	if !m.logTop.colHidden[colID] {
		delete(m.logTop.colHidden, colID)
	}
	m.logTopRefreshRows()
	return m
}

func (m Model) logTopColumnsMoveDownByName(cols []string) Model {
	if m.overlayCursor >= len(cols) {
		return m
	}
	colName := cols[m.overlayCursor]
	// Only move dimension columns (must be in colOrder).
	dimIdx := -1
	for i, d := range m.logTop.colOrder {
		if d == colName {
			dimIdx = i
			break
		}
	}
	if dimIdx < 0 || dimIdx >= len(m.logTop.colOrder)-1 {
		return m // not a dim, or last dim
	}
	order := append([]string(nil), m.logTop.colOrder...)
	order[dimIdx], order[dimIdx+1] = order[dimIdx+1], order[dimIdx]
	m.logTop.colOrder = order
	// Recompute cursor: find new position of colName in filtered list.
	newCols := m.logTopFilteredColumns()
	for i, c := range newCols {
		if c == colName {
			m.overlayCursor = i
			break
		}
	}
	return m
}

func (m Model) logTopColumnsMoveUpByName(cols []string) Model {
	if m.overlayCursor >= len(cols) {
		return m
	}
	colName := cols[m.overlayCursor]
	dimIdx := -1
	for i, d := range m.logTop.colOrder {
		if d == colName {
			dimIdx = i
			break
		}
	}
	if dimIdx <= 0 {
		return m // not a dim, or first dim
	}
	order := append([]string(nil), m.logTop.colOrder...)
	order[dimIdx], order[dimIdx-1] = order[dimIdx-1], order[dimIdx]
	m.logTop.colOrder = order
	newCols := m.logTopFilteredColumns()
	for i, c := range newCols {
		if c == colName {
			m.overlayCursor = i
			break
		}
	}
	return m
}
