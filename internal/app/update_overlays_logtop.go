package app

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/logagg"
)

// logTopGroupByCandidates returns a sorted list of all field names seen in the
// currently parsed lines.
func (m Model) logTopGroupByCandidates() []string {
	seen := map[string]bool{}
	for _, f := range m.logTop.parsed {
		for k := range f {
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
		m.logTop.drillField = nil
		m.logTop.drillValue = nil
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
