package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle mouse scroll in log viewer mode.
	if m.mode == modeLogs {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.logFollow = false
			if m.logScroll > 0 {
				m.logScroll -= 3
				if m.logScroll < 0 {
					m.logScroll = 0
				}
			}
			cmd := m.maybeLoadMoreHistory()
			return m, cmd
		case tea.MouseButtonWheelDown:
			m.logFollow = false
			m.logScroll += 3
			m.clampLogScroll()
		}
		return m, nil
	}

	// Wheel scroll in the other full-screen viewer modes (YAML, Describe,
	// Diff, Help, Explain). Synthesize 3 j/k key presses per tick so the
	// existing per-mode scroll logic — cursor advance, ensure-visible,
	// clamps, page-X tracking, sub-mode dispatch — runs unchanged.
	// Other mouse buttons fall through so tab-bar clicks still work.
	if isViewerMode(m.mode) {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			return m.dispatchWheelKey("k")
		case tea.MouseButtonWheelDown:
			return m.dispatchWheelKey("j")
		}
	}

	// Handle tab bar clicks in any mode.
	if len(m.tabs) > 1 && msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && msg.Y == 1 {
		if tab := m.tabAtX(msg.X); tab >= 0 && tab != m.activeTab {
			return m.switchToTab(tab)
		}
		return m, nil
	}

	// Don't handle mouse in overlay/help/yaml modes.
	if m.overlay != overlayNone || m.mode != modeExplorer {
		return m, nil
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.moveCursor(-3)
	case tea.MouseButtonWheelDown:
		return m.moveCursor(3)
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		return m.handleMouseClick(msg.X, msg.Y)
	}
	return m, nil
}

func (m Model) handleMouseClick(x, y int) (tea.Model, tea.Cmd) {
	// Calculate column boundaries (must match viewExplorer).
	var leftEnd, middleEnd int
	if m.fullscreenMiddle || m.fullscreenDashboard {
		// Fullscreen: only middle column exists.
		leftEnd = 0
		middleEnd = m.width
	} else {
		usable := m.width - 6
		leftW := max(10, usable*12/100)
		middleW := max(10, usable*51/100)
		// Each column has border(2) + padding(2) = 4 extra chars width.
		leftEnd = leftW + 4
		middleEnd = leftEnd + middleW + 4
	}

	switch {
	case x < leftEnd:
		// Left column click: navigate parent.
		return m.navigateParent()
	case x < middleEnd:
		// Middle column click: select item.
		// y offset depends on whether column has a header line.
		// Title bar (1) + top border (1) = 2 base offset.
		baseOffset := 2 // title bar (1) + top border (1)
		if len(m.tabs) > 1 {
			baseOffset = 3 // title bar (1) + tab bar (1) + top border (1)
		}
		itemY := y - baseOffset
		switch m.nav.Level {
		case model.LevelResources, model.LevelOwned, model.LevelContainers:
			// Table view has a header row for column names.
			itemY-- // subtract table header row
			if itemY < 0 {
				// Header row click — sort by the clicked column.
				relX := x - 2 // border + padding
				if !m.fullscreenMiddle && !m.fullscreenDashboard {
					relX = x - leftEnd - 2
				}
				return m.handleHeaderClick(relX)
			}
			// Use line map built during rendering for accurate click targeting.
			if itemY < len(ui.ActiveMiddleLineMap) {
				targetIdx := ui.ActiveMiddleLineMap[itemY]
				if targetIdx >= 0 && targetIdx < len(m.visibleMiddleItems()) {
					m.setCursor(targetIdx)
					return m, m.loadPreview()
				}
			}
		default:
			// Column view has a header line (rendered by RenderColumn).
			itemY-- // subtract column header
			if itemY >= 0 && itemY < len(ui.ActiveMiddleLineMap) {
				targetIdx := ui.ActiveMiddleLineMap[itemY]
				if targetIdx >= 0 && targetIdx < len(m.visibleMiddleItems()) {
					m.setCursor(targetIdx)
					m.syncExpandedGroup()
					return m, m.loadPreview()
				}
			}
		}
		return m, nil
	default:
		// Right column click: navigate child.
		return m.navigateChild()
	}
}

// findSortableCol returns the index of name in ActiveSortableColumns, or -1.
func findSortableCol(name string) int {
	for i, c := range ui.ActiveSortableColumns {
		if c == name {
			return i
		}
	}
	return -1
}

// handleHeaderClick sorts the table by the column that was clicked in the header row.
// relX is the click position relative to the start of the middle column content area.
// It consumes the ActiveMiddleColumnLayout populated by RenderTable so the mapping
// from click X to column key always matches the actual rendered order, even when
// the user has reordered columns via the column-toggle overlay.
func (m Model) handleHeaderClick(relX int) (tea.Model, tea.Cmd) {
	if !m.sortApplies() {
		return m, nil
	}
	items := m.visibleMiddleItems()
	if len(items) == 0 || len(ui.ActiveSortableColumns) == 0 || len(ui.ActiveMiddleColumnLayout) == 0 {
		return m, nil
	}

	// Find which column region the click falls into.
	clickedKey := ""
	for _, region := range ui.ActiveMiddleColumnLayout {
		if relX >= region.StartX && relX < region.EndX {
			clickedKey = region.Key
			break
		}
	}
	// Clicks past the last column fall through to the rightmost column so
	// the behavior matches the previous implementation.
	if clickedKey == "" {
		last := ui.ActiveMiddleColumnLayout[len(ui.ActiveMiddleColumnLayout)-1]
		if relX >= last.EndX {
			clickedKey = last.Key
		}
	}
	if clickedKey == "" {
		return m, nil
	}

	// Only react if the clicked column is actually sortable.
	if findSortableCol(clickedKey) < 0 {
		return m, nil
	}

	if m.sortColumnName == clickedKey {
		m.sortAscending = !m.sortAscending
	} else {
		m.sortColumnName = clickedKey
		m.sortAscending = true
	}
	m.sortMiddleItems()
	m.clampCursor()
	m.setStatusMessage("Sort: "+m.sortModeName(), false)
	return m, tea.Batch(m.loadPreview(), scheduleStatusClear())
}

// tabAtX returns the tab index at the given X coordinate in the tab bar,
// or -1 if the click is not on any tab.
func (m *Model) tabAtX(x int) int {
	labels := m.tabLabels()
	// Tab bar: each tab label is padded with 1 char on each side (Padding(0,1)),
	// separated by " | " (3 chars). Tab bar starts at x=1 (bar left padding).
	pos := 1
	for i, label := range labels {
		tabW := len(label) + 2 // label + padding(0,1) on each side
		if x >= pos && x < pos+tabW {
			return i
		}
		pos += tabW + 3 // separator " | "
	}
	return -1
}

// isViewerMode returns true for full-screen content viewers that don't
// have native wheel-scroll handling. modeLogs and modeExplorer have
// their own wheel paths and are handled separately.
func isViewerMode(mode viewMode) bool {
	switch mode {
	case modeYAML, modeDescribe, modeDiff, modeHelp, modeExplain:
		return true
	}
	return false
}

// dispatchWheelKey synthesizes 3 presses of key (typically "j" or "k")
// through handleKey so each viewer mode's existing scroll logic runs
// unchanged. The model is threaded between iterations; the last cmd is
// returned (per-mode scroll handlers are pure state mutations and
// typically return nil, so dropping intermediate cmds is safe).
func (m Model) dispatchWheelKey(key string) (tea.Model, tea.Cmd) {
	const wheelStep = 3
	var lastCmd tea.Cmd
	runes := []rune(key)
	for range wheelStep {
		mdl, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: runes})
		m = mdl.(Model)
		if cmd != nil {
			lastCmd = cmd
		}
	}
	return m, lastCmd
}

// switchToTab saves the current tab and loads the target tab.
func (m Model) switchToTab(tab int) (tea.Model, tea.Cmd) {
	m.saveCurrentTab()
	if cmd := m.loadTab(tab); cmd != nil {
		return m, cmd
	}
	if m.mode == modeExplorer {
		return m, m.loadPreview()
	}
	if m.mode == modeLogs && m.logCh != nil {
		return m, m.waitForLogLine()
	}
	if m.mode == modeExec && m.execPTY != nil {
		return m, m.scheduleExecTick()
	}
	return m, nil
}
