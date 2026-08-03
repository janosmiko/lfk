package app

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleLogTopKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) { //nolint:unparam // tea.Cmd return is part of the action-key handler convention; may carry cmds in future
	if m.logTop.searchActive {
		return m.handleLogTopSearchKey(msg)
	}
	if m.logTop.filterActive {
		return m.handleLogTopFilterKey(msg)
	}
	kb := ui.ActiveKeybindings
	visible := m.logTopVisibleRows()
	half := max(visible/2, 1)
	full := max(visible-1, 1)
	switch msg.String() {
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.logTop.lineInput += msg.String()
		return m, nil
	case "0":
		if m.logTop.lineInput != "" {
			m.logTop.lineInput += "0"
			return m, nil
		}
		// bare 0 with no prefix: not a count, fall through to clear and no-op
		m.logTop.lineInput = ""
		return m, nil
	case "esc", "q":
		m.logTop.lineInput = ""
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
	case kb.Down, "j", "down":
		n := consumeCountPrefix(&m.logTop.lineInput)
		last := max(len(m.logTop.rows)-1, 0)
		m.logTop.cursor = min(m.logTop.cursor+n, last)
		m.logTopSyncScroll()
		return m, nil
	case kb.Up, "k", "up":
		n := consumeCountPrefix(&m.logTop.lineInput)
		m.logTop.cursor = max(m.logTop.cursor-n, 0)
		m.logTopSyncScroll()
		return m, nil
	case kb.PageDown, "shift+down": // half-page down
		n := consumeCountPrefix(&m.logTop.lineInput)
		m.logTop.cursor = min(m.logTop.cursor+half*n, max(len(m.logTop.rows)-1, 0))
		m.logTopSyncScroll()
		return m, nil
	case kb.PageUp, "shift+up": // half-page up
		n := consumeCountPrefix(&m.logTop.lineInput)
		m.logTop.cursor = max(m.logTop.cursor-half*n, 0)
		m.logTopSyncScroll()
		return m, nil
	case kb.PageForward, "pgdown": // full-page down
		n := consumeCountPrefix(&m.logTop.lineInput)
		m.logTop.cursor = min(m.logTop.cursor+full*n, max(len(m.logTop.rows)-1, 0))
		m.logTopSyncScroll()
		return m, nil
	case kb.PageBack, "pgup": // full-page up
		n := consumeCountPrefix(&m.logTop.lineInput)
		m.logTop.cursor = max(m.logTop.cursor-full*n, 0)
		m.logTopSyncScroll()
		return m, nil
	case kb.JumpBottom, "G", "end":
		if m.logTop.lineInput != "" {
			// count-G: go to row n-1 (1-indexed like vim)
			n := consumeCountPrefix(&m.logTop.lineInput)
			last := max(len(m.logTop.rows)-1, 0)
			m.logTop.cursor = min(n-1, last)
		} else {
			m.logTop.cursor = max(len(m.logTop.rows)-1, 0)
		}
		m.logTopSyncScroll()
		return m, nil
	case kb.JumpTop, "home":
		m.logTop.lineInput = ""
		m.logTop.cursor = 0
		m.logTopSyncScroll()
		return m, nil
	case ".":
		m.logTop.lineInput = ""
		return m.openLogTopGroupBy(), nil
	case "p":
		m.logTop.lineInput = ""
		return m.openLogTopProfile(), nil
	case kb.ColumnToggle:
		m.logTop.lineInput = ""
		return m.openLogTopColumns(), nil
	case kb.SortNext:
		m.logTop.lineInput = ""
		m.logTopCycleSort(+1)
		return m, nil
	case kb.SortPrev:
		m.logTop.lineInput = ""
		m.logTopCycleSort(-1)
		return m, nil
	case kb.SortFlip:
		m.logTop.lineInput = ""
		m.logTop.sortAsc = !m.logTop.sortAsc
		m.logTopRefreshRows()
		return m, nil
	case kb.SortReset:
		m.logTop.lineInput = ""
		m.logTop.sortCol = logTopMetricREQ
		m.logTop.sortAsc = false
		m.logTopRefreshRows()
		return m, nil
	case "tab":
		m.logTop.lineInput = ""
		m.logTopCycleDrillTarget()
		return m, nil
	case "enter":
		m.logTop.lineInput = ""
		return m.logTopDrillIn(), nil
	case kb.Filter:
		m.logTop.lineInput = ""
		m.logTop.filterActive = true
		m.logTop.filterInput.Set(m.logTop.filterQuery)
		return m, nil
	case kb.Search:
		m.logTop.lineInput = ""
		m.logTop.searchActive = true
		m.logTop.searchInput.Set(m.logTop.searchQuery)
		return m, nil
	case kb.NextMatch:
		m.logTop.lineInput = ""
		m.logTopFindMatch(true)
		return m, nil
	case kb.PrevMatch:
		m.logTop.lineInput = ""
		m.logTopFindMatch(false)
		return m, nil
	}
	m.logTop.lineInput = ""
	return m, nil
}

// handleLogTopFilterKey processes a key while the Log Top filter input is open.
// The filter applies live: every edit calls logTopRefreshRows which re-applies
// the query to the current aggregation rows.
func (m Model) handleLogTopFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) { //nolint:unparam // tea.Cmd return is part of the handler convention
	switch msg.String() {
	case "enter":
		m.logTop.filterActive = false
		return m, nil
	case "esc":
		m.logTop.filterActive = false
		m.logTop.filterInput.Clear()
		m.logTop.filterQuery = ""
		m.logTop.filterTerms = nil
		m.logTopRebuildRows()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	case "backspace":
		m.logTop.filterInput.Backspace()
	case "ctrl+w":
		m.logTop.filterInput.DeleteWord()
	case "ctrl+u":
		m.logTop.filterInput.DeleteLine()
	case "ctrl+a":
		m.logTop.filterInput.Home()
		return m, nil
	case "ctrl+e":
		m.logTop.filterInput.End()
		return m, nil
	case "left":
		m.logTop.filterInput.Left()
		return m, nil
	case "right":
		m.logTop.filterInput.Right()
		return m, nil
	default:
		k := msg.String()
		if len(k) == 1 && k[0] >= 32 && k[0] < 127 {
			m.logTop.filterInput.Insert(k)
		} else {
			return m, nil
		}
	}
	m.logTop.filterQuery = m.logTop.filterInput.Value
	m.logTop.filterTerms = parseLogTopFilter(m.logTop.filterQuery)
	m.logTopRebuildRows()
	return m, nil
}

// handleLogTopSearchKey processes a key while the Log Top search input is open.
// Search jumps the cursor to matching rows (it does not hide rows — that is
// what the f filter does).
func (m Model) handleLogTopSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) { //nolint:unparam // tea.Cmd return is part of the handler convention
	switch msg.String() {
	case "enter":
		m.logTop.searchActive = false
		m.logTop.searchQuery = m.logTop.searchInput.Value
		m.logTopFindMatch(true)
	case "esc":
		m.logTop.searchActive = false
		m.logTop.searchInput.Clear()
		m.logTop.searchQuery = ""
	case "ctrl+c":
		return m.closeTabOrQuit()
	case "backspace":
		m.logTop.searchInput.Backspace()
	case "ctrl+w":
		m.logTop.searchInput.DeleteWord()
	case "ctrl+u":
		m.logTop.searchInput.DeleteLine()
	case "ctrl+a":
		m.logTop.searchInput.Home()
		return m, nil
	case "ctrl+e":
		m.logTop.searchInput.End()
		return m, nil
	case "left":
		m.logTop.searchInput.Left()
		return m, nil
	case "right":
		m.logTop.searchInput.Right()
		return m, nil
	default:
		k := msg.String()
		if len(k) == 1 && k[0] >= 32 && k[0] < 127 {
			m.logTop.searchInput.Insert(k)
		} else {
			return m, nil
		}
	}
	m.logTop.searchQuery = m.logTop.searchInput.Value
	return m, nil
}

// logTopFindMatch moves the cursor to the next (forward) or previous row whose
// dimension text matches searchQuery, wrapping around. No-op if query is empty.
func (m *Model) logTopFindMatch(forward bool) {
	if m.logTop.searchQuery == "" || len(m.logTop.rows) == 0 {
		return
	}
	n := len(m.logTop.rows)
	step := 1
	if !forward {
		step = -1
	}
	for off := 1; off <= n; off++ {
		idx := ((m.logTop.cursor+step*off)%n + n) % n
		if ui.MatchLine(m.logTopRowText(m.logTop.rows[idx]), m.logTop.searchQuery) {
			m.logTop.cursor = idx
			m.logTopSyncScroll()
			return
		}
	}
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
	m.logTop.drillTarget = ""
	m.logTop.cursor = 0
	m.logTopRebuildRows()
	return m
}

// logTopDrillCandidates returns the un-pinned display dims in order.
func (m *Model) logTopDrillCandidates() []string {
	pinned := map[string]bool{}
	for _, g := range m.logTop.groupBy {
		pinned[g] = true
	}
	for _, fr := range m.logTop.drillStack {
		for _, flt := range fr.filters {
			pinned[flt.field] = true
		}
	}
	var out []string
	for _, d := range m.logTop.displayDims {
		if !pinned[d] {
			out = append(out, d)
		}
	}
	return out
}

// logTopNextDrillDim returns the dimension the next Enter will drill into.
// If a drillTarget is set and still a valid candidate it is returned; otherwise
// the first candidate is returned. Returns "" when every dimension is already used.
func (m *Model) logTopNextDrillDim() string {
	cands := m.logTopDrillCandidates()
	if len(cands) == 0 {
		return ""
	}
	if m.logTop.drillTarget != "" && slices.Contains(cands, m.logTop.drillTarget) {
		return m.logTop.drillTarget
	}
	return cands[0]
}

// logTopCycleDrillTarget advances drillTarget to the next candidate, wrapping around.
func (m *Model) logTopCycleDrillTarget() {
	cands := m.logTopDrillCandidates()
	if len(cands) == 0 {
		return
	}
	current := m.logTopNextDrillDim()
	idx := 0
	for i, c := range cands {
		if c == current {
			idx = i
			break
		}
	}
	m.logTop.drillTarget = cands[(idx+1)%len(cands)]
}
