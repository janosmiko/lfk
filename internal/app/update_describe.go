package app

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleDescribeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalLines := countLines(m.describeContent)
	visibleLines := m.height - 4
	if visibleLines < 3 {
		visibleLines = 3
	}
	maxScroll := totalLines - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch msg.String() {
	case "?":
		m.helpPreviousMode = modeDescribe
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "Navigation"
		return m, nil
	case "q", "esc":
		m.mode = modeExplorer
		m.describeScroll = 0
		m.describeAutoRefresh = false
		m.describeRefreshFunc = nil
		return m, nil
	case "j", "down":
		m.describeScroll++
		if m.describeScroll > maxScroll {
			m.describeScroll = maxScroll
		}
		return m, nil
	case "k", "up":
		if m.describeScroll > 0 {
			m.describeScroll--
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.describeScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		m.describeScroll = maxScroll
		return m, nil
	case "ctrl+d":
		m.describeScroll += m.height / 2
		if m.describeScroll > maxScroll {
			m.describeScroll = maxScroll
		}
		return m, nil
	case "ctrl+u":
		m.describeScroll -= m.height / 2
		if m.describeScroll < 0 {
			m.describeScroll = 0
		}
		return m, nil
	case "ctrl+f":
		m.describeScroll += m.height
		if m.describeScroll > maxScroll {
			m.describeScroll = maxScroll
		}
		return m, nil
	case "ctrl+b":
		m.describeScroll -= m.height
		if m.describeScroll < 0 {
			m.describeScroll = 0
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleDiffKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	foldRegions := ui.ComputeDiffFoldRegions(m.diffLeft, m.diffRight)
	m.ensureDiffFoldState(foldRegions)

	totalLines := ui.DiffViewTotalLines(m.diffLeft, m.diffRight, foldRegions, m.diffFoldState)
	if m.diffUnified {
		totalLines = ui.UnifiedDiffViewTotalLines(m.diffLeft, m.diffRight, foldRegions, m.diffFoldState)
	}
	visibleLines := m.height - 4
	if visibleLines < 3 {
		visibleLines = 3
	}
	maxScroll := totalLines - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	// When in search input mode, handle text input first.
	if m.diffSearchMode {
		switch msg.String() {
		case "enter":
			m.diffSearchMode = false
			m.diffSearchQuery = m.diffSearchText.Value
			m.diffMatchLines = ui.UpdateDiffSearchMatches(m.diffLeft, m.diffRight, m.diffSearchQuery)
			if len(m.diffMatchLines) > 0 {
				m.diffMatchIdx = 0
				m.diffScrollToMatch(foldRegions, visibleLines)
			}
			return m, nil
		case "esc":
			m.diffSearchMode = false
			m.diffSearchText.Clear()
			m.diffSearchQuery = ""
			m.diffMatchLines = nil
			m.diffMatchIdx = 0
			return m, nil
		case "backspace":
			if len(m.diffSearchText.Value) > 0 {
				m.diffSearchText.Backspace()
			}
			return m, nil
		case "ctrl+w":
			m.diffSearchText.DeleteWord()
			return m, nil
		case "ctrl+a":
			m.diffSearchText.Home()
			return m, nil
		case "ctrl+e":
			m.diffSearchText.End()
			return m, nil
		case "left":
			m.diffSearchText.Left()
			return m, nil
		case "right":
			m.diffSearchText.Right()
			return m, nil
		case "ctrl+c":
			m.diffSearchMode = false
			m.diffSearchText.Clear()
			m.diffMatchLines = nil
			return m, nil
		default:
			if len(msg.String()) == 1 || msg.String() == " " {
				m.diffSearchText.Insert(msg.String())
			}
			return m, nil
		}
	}

	switch msg.String() {
	case "?":
		m.helpPreviousMode = modeDiff
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "Diff View"
		return m, nil
	case "q", "esc":
		m.mode = modeExplorer
		m.diffScroll = 0
		m.diffCursor = 0
		m.diffCursorSide = 0
		m.diffLineInput = ""
		m.diffSearchQuery = ""
		m.diffSearchText.Clear()
		m.diffMatchLines = nil
		m.diffMatchIdx = 0
		m.diffFoldState = nil
		return m, nil
	case "j", "down":
		m.diffLineInput = ""
		maxCursor := max(totalLines-1, 0)
		if m.diffCursor < maxCursor {
			m.diffCursor++
		}
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "k", "up":
		m.diffLineInput = ""
		if m.diffCursor > 0 {
			m.diffCursor--
		}
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.diffLineInput = ""
			m.diffCursor = 0
			m.diffScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		maxCursor := max(totalLines-1, 0)
		if m.diffLineInput != "" {
			lineNum, _ := strconv.Atoi(m.diffLineInput)
			m.diffLineInput = ""
			if lineNum > 0 {
				lineNum-- // 0-indexed
			}
			m.diffCursor = min(lineNum, maxCursor)
		} else {
			m.diffCursor = maxCursor
		}
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+d":
		m.diffLineInput = ""
		maxCursor := max(totalLines-1, 0)
		m.diffCursor = min(m.diffCursor+m.height/2, maxCursor)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+u":
		m.diffLineInput = ""
		m.diffCursor = max(m.diffCursor-m.height/2, 0)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+f":
		m.diffLineInput = ""
		maxCursor := max(totalLines-1, 0)
		m.diffCursor = min(m.diffCursor+m.height, maxCursor)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+b":
		m.diffLineInput = ""
		m.diffCursor = max(m.diffCursor-m.height, 0)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "u":
		m.diffLineInput = ""
		m.diffUnified = !m.diffUnified
		m.diffScroll = 0
		return m, nil
	case "#":
		m.diffLineInput = ""
		m.diffLineNumbers = !m.diffLineNumbers
		return m, nil
	case "/":
		m.diffLineInput = ""
		m.diffSearchMode = true
		m.diffSearchText.Clear()
		m.diffMatchLines = nil
		m.diffMatchIdx = 0
		return m, nil
	case "n":
		m.diffLineInput = ""
		if len(m.diffMatchLines) > 0 {
			m.diffMatchIdx = (m.diffMatchIdx + 1) % len(m.diffMatchLines)
			m.diffScrollToMatch(foldRegions, visibleLines)
		}
		return m, nil
	case "N":
		m.diffLineInput = ""
		if len(m.diffMatchLines) > 0 {
			m.diffMatchIdx = (m.diffMatchIdx - 1 + len(m.diffMatchLines)) % len(m.diffMatchLines)
			m.diffScrollToMatch(foldRegions, visibleLines)
		}
		return m, nil
	case "tab":
		// Switch cursor side (left/right) in side-by-side mode.
		if !m.diffUnified {
			m.diffCursorSide = 1 - m.diffCursorSide
		}
		return m, nil
	case "z":
		m.diffLineInput = ""
		m.toggleDiffFoldAtCursor(foldRegions)
		return m, nil
	case "Z":
		m.diffLineInput = ""
		m.toggleAllDiffFolds(foldRegions)
		return m, nil
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.diffLineInput += msg.String()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		m.diffLineInput = ""
	}
	return m, nil
}

// ensureDiffFoldState ensures the fold state slice has the correct length for
// the current fold regions.
func (m *Model) ensureDiffFoldState(regions []ui.DiffFoldRegion) {
	if len(m.diffFoldState) < len(regions) {
		newState := make([]bool, len(regions))
		copy(newState, m.diffFoldState)
		m.diffFoldState = newState
	}
}

// ensureDiffCursorVisible adjusts diffScroll so the cursor is within the viewport.
func (m *Model) ensureDiffCursorVisible(viewportLines, maxScroll int) {
	if m.diffCursor < m.diffScroll {
		m.diffScroll = m.diffCursor
	}
	if m.diffCursor >= m.diffScroll+viewportLines {
		m.diffScroll = m.diffCursor - viewportLines + 1
	}
	m.diffScroll = max(min(m.diffScroll, maxScroll), 0)
}

// diffScrollToMatch auto-expands the fold region containing the current match
// and scrolls to center it in the viewport.
func (m *Model) diffScrollToMatch(foldRegions []ui.DiffFoldRegion, viewportLines int) {
	if len(m.diffMatchLines) == 0 || m.diffMatchIdx < 0 || m.diffMatchIdx >= len(m.diffMatchLines) {
		return
	}
	origIdx := m.diffMatchLines[m.diffMatchIdx]

	// Auto-expand any collapsed fold region containing this match.
	ui.ExpandDiffFoldForLine(foldRegions, m.diffFoldState, origIdx)

	// Find the visible index for this original line.
	visIdx := ui.DiffVisibleIndexForOriginal(m.diffLeft, m.diffRight, foldRegions, m.diffFoldState, origIdx)
	if visIdx < 0 {
		return
	}

	// Move cursor and center in viewport.
	m.diffCursor = visIdx
	m.diffScroll = visIdx - viewportLines/2
	if m.diffScroll < 0 {
		m.diffScroll = 0
	}
}

// toggleDiffFoldAtCursor toggles the fold on the unchanged section at the cursor.
// When collapsing, moves the cursor to the fold placeholder line.
func (m *Model) toggleDiffFoldAtCursor(foldRegions []ui.DiffFoldRegion) {
	rawDiffLines := ui.ComputeDiffLines(m.diffLeft, m.diffRight)
	visLines := ui.BuildVisibleDiffLines(rawDiffLines, foldRegions, m.diffFoldState)

	idx := m.diffCursor
	if idx >= len(visLines) {
		idx = len(visLines) - 1
	}
	if idx < 0 {
		return
	}

	vl := visLines[idx]
	if vl.RegionIdx < 0 || vl.RegionIdx >= len(m.diffFoldState) {
		return
	}

	wasCollapsed := m.diffFoldState[vl.RegionIdx]
	m.diffFoldState[vl.RegionIdx] = !wasCollapsed

	// When collapsing, reposition cursor to the fold placeholder.
	if !wasCollapsed {
		newVisLines := ui.BuildVisibleDiffLines(rawDiffLines, foldRegions, m.diffFoldState)
		for i, nvl := range newVisLines {
			if nvl.IsFoldPlaceholder && nvl.RegionIdx == vl.RegionIdx {
				m.diffCursor = i
				break
			}
		}
	}
}

// toggleAllDiffFolds toggles all fold regions at once. If any are collapsed,
// expand all; otherwise collapse all.
func (m *Model) toggleAllDiffFolds(foldRegions []ui.DiffFoldRegion) {
	anyCollapsed := false
	for i := range foldRegions {
		if i < len(m.diffFoldState) && m.diffFoldState[i] {
			anyCollapsed = true
			break
		}
	}
	for i := range foldRegions {
		if i < len(m.diffFoldState) {
			m.diffFoldState[i] = !anyCollapsed
		}
	}
}
