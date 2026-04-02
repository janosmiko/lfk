package app

import (
	"fmt"
	"strconv"
	"strings"

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
	// Side-by-side: height - 6 (title + hint + border top/bottom + header + separator).
	// Unified: height - 4 (title + hint + border top/bottom; headers are inside content).
	visibleLines := m.height - 6
	if m.diffUnified {
		totalLines = ui.UnifiedDiffViewTotalLines(m.diffLeft, m.diffRight, foldRegions, m.diffFoldState)
		visibleLines = m.height - 4
	}
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
			m.diffMatchLines = ui.UpdateDiffSearchMatches(m.diffLeft, m.diffRight, m.diffSearchQuery, m.diffCursorSide, m.diffUnified)
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

	// In visual selection mode, delegate to the visual key handler.
	if m.diffVisualMode {
		return m.handleDiffVisualKey(msg, foldRegions, totalLines, visibleLines, maxScroll)
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
		m.diffVisualMode = false
		m.diffVisualCurCol = 0
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
	case "h", "left":
		m.diffLineInput = ""
		if m.diffVisualCurCol > 0 {
			m.diffVisualCurCol--
		}
		return m, nil
	case "l", "right":
		m.diffLineInput = ""
		m.diffVisualCurCol++
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
	case "0":
		// If digits are pending, append 0 (e.g. 10G, 20G).
		// Otherwise move cursor to beginning of line.
		if m.diffLineInput != "" {
			m.diffLineInput += "0"
		} else {
			m.diffVisualCurCol = 0
		}
		return m, nil
	case "$":
		m.diffLineInput = ""
		lineText := m.diffCurrentLineText(foldRegions)
		lineLen := len([]rune(lineText))
		if lineLen > 0 {
			m.diffVisualCurCol = lineLen - 1
		}
		return m, nil
	case "^":
		m.diffLineInput = ""
		lineText := m.diffCurrentLineText(foldRegions)
		m.diffVisualCurCol = firstNonWhitespace(lineText)
		return m, nil
	case "w":
		m.diffLineInput = ""
		lineText := m.diffCurrentLineText(foldRegions)
		if lineText != "" {
			lineLen := len([]rune(lineText))
			newCol := nextWordStart(lineText, m.diffVisualCurCol)
			if newCol >= lineLen {
				// Stay at end of line in diff view (no cross-line).
				newCol = max(lineLen-1, 0)
			}
			m.diffVisualCurCol = newCol
		}
		return m, nil
	case "b":
		m.diffLineInput = ""
		lineText := m.diffCurrentLineText(foldRegions)
		if lineText != "" {
			newCol := prevWordStart(lineText, m.diffVisualCurCol)
			if newCol < 0 {
				newCol = 0
			}
			m.diffVisualCurCol = newCol
		}
		return m, nil
	case "e":
		m.diffLineInput = ""
		lineText := m.diffCurrentLineText(foldRegions)
		if lineText != "" {
			lineLen := len([]rune(lineText))
			newCol := wordEnd(lineText, m.diffVisualCurCol)
			if newCol >= lineLen {
				newCol = max(lineLen-1, 0)
			}
			m.diffVisualCurCol = newCol
		}
		return m, nil
	case "E":
		m.diffLineInput = ""
		lineText := m.diffCurrentLineText(foldRegions)
		if lineText != "" {
			lineLen := len([]rune(lineText))
			newCol := WORDEnd(lineText, m.diffVisualCurCol)
			if newCol >= lineLen {
				newCol = max(lineLen-1, 0)
			}
			m.diffVisualCurCol = newCol
		}
		return m, nil
	case "W":
		m.diffLineInput = ""
		lineText := m.diffCurrentLineText(foldRegions)
		if lineText != "" {
			lineLen := len([]rune(lineText))
			newCol := nextWORDStart(lineText, m.diffVisualCurCol)
			if newCol >= lineLen {
				newCol = max(lineLen-1, 0)
			}
			m.diffVisualCurCol = newCol
		}
		return m, nil
	case "B":
		m.diffLineInput = ""
		lineText := m.diffCurrentLineText(foldRegions)
		if lineText != "" {
			newCol := prevWORDStart(lineText, m.diffVisualCurCol)
			if newCol < 0 {
				newCol = 0
			}
			m.diffVisualCurCol = newCol
		}
		return m, nil
	case "v":
		m.diffVisualMode = true
		m.diffVisualType = 'v'
		m.diffVisualStart = m.diffCursor
		m.diffVisualCol = m.diffVisualCurCol
		return m, nil
	case "V":
		m.diffVisualMode = true
		m.diffVisualType = 'V'
		m.diffVisualStart = m.diffCursor
		m.diffVisualCol = m.diffVisualCurCol
		return m, nil
	case "ctrl+v":
		m.diffVisualMode = true
		m.diffVisualType = 'B'
		m.diffVisualStart = m.diffCursor
		m.diffVisualCol = m.diffVisualCurCol
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
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.diffLineInput += msg.String()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		m.diffLineInput = ""
	}
	return m, nil
}

// diffCurrentLineText returns the plain text of the current diff line on the active side.
func (m *Model) diffCurrentLineText(foldRegions []ui.DiffFoldRegion) string {
	return ui.DiffLineTextAt(m.diffLeft, m.diffRight, foldRegions, m.diffFoldState, m.diffCursor, m.diffCursorSide, m.diffUnified)
}

// handleDiffVisualKey handles key events while in diff visual selection mode.
func (m Model) handleDiffVisualKey(msg tea.KeyMsg, foldRegions []ui.DiffFoldRegion, totalLines, visibleLines, maxScroll int) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.diffVisualMode = false
		return m, nil
	case "V":
		if m.diffVisualType == 'V' {
			m.diffVisualMode = false
		} else {
			m.diffVisualType = 'V'
		}
		return m, nil
	case "v":
		if m.diffVisualType == 'v' {
			m.diffVisualMode = false
		} else {
			m.diffVisualType = 'v'
		}
		return m, nil
	case "ctrl+v":
		if m.diffVisualType == 'B' {
			m.diffVisualMode = false
		} else {
			m.diffVisualType = 'B'
		}
		return m, nil
	case "y":
		// Yank (copy) selected text to clipboard.
		selStart := min(m.diffVisualStart, m.diffCursor)
		selEnd := max(m.diffVisualStart, m.diffCursor)

		var parts []string
		for i := selStart; i <= selEnd; i++ {
			lineText := ui.DiffLineTextAt(m.diffLeft, m.diffRight, foldRegions, m.diffFoldState, i, m.diffCursorSide, m.diffUnified)
			// Skip lines where the active side has no content (e.g., additions
			// on the opposite side show as empty on this side).
			if lineText == "" {
				continue
			}

			switch m.diffVisualType {
			case 'v': // Character mode: partial first/last lines.
				runes := []rune(lineText)
				anchorCol := m.diffVisualCol
				cursorCol := m.diffVisualCurCol
				startCol, endCol := anchorCol, cursorCol
				if m.diffVisualStart > m.diffCursor {
					startCol, endCol = cursorCol, anchorCol
				}
				if selStart == selEnd {
					cs := min(anchorCol, cursorCol)
					ce := max(anchorCol, cursorCol) + 1
					if cs > len(runes) {
						cs = len(runes)
					}
					if ce > len(runes) {
						ce = len(runes)
					}
					parts = append(parts, string(runes[cs:ce]))
				} else if i == selStart {
					cs := startCol
					if cs > len(runes) {
						cs = len(runes)
					}
					parts = append(parts, string(runes[cs:]))
				} else if i == selEnd {
					ce := endCol + 1
					if ce > len(runes) {
						ce = len(runes)
					}
					parts = append(parts, string(runes[:ce]))
				} else {
					parts = append(parts, lineText)
				}
			case 'B': // Block mode: rectangular column range.
				runes := []rune(lineText)
				colStart := min(m.diffVisualCol, m.diffVisualCurCol)
				colEnd := max(m.diffVisualCol, m.diffVisualCurCol) + 1
				if colStart > len(runes) {
					colStart = len(runes)
				}
				if colEnd > len(runes) {
					colEnd = len(runes)
				}
				parts = append(parts, string(runes[colStart:colEnd]))
			default: // Line mode: full lines.
				parts = append(parts, lineText)
			}
		}
		clipText := strings.Join(parts, "\n")
		lineCount := selEnd - selStart + 1
		m.diffVisualMode = false
		m.setStatusMessage(fmt.Sprintf("Copied %d lines", lineCount), false)
		return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
	case "j", "down":
		maxCursor := max(totalLines-1, 0)
		if m.diffCursor < maxCursor {
			m.diffCursor++
		}
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "k", "up":
		if m.diffCursor > 0 {
			m.diffCursor--
		}
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "h", "left":
		if m.diffVisualType == 'v' || m.diffVisualType == 'B' {
			if m.diffVisualCurCol > 0 {
				m.diffVisualCurCol--
			}
		}
		return m, nil
	case "l", "right":
		if m.diffVisualType == 'v' || m.diffVisualType == 'B' {
			m.diffVisualCurCol++
		}
		return m, nil
	case "0":
		m.diffVisualCurCol = 0
		return m, nil
	case "$":
		lineText := m.diffCurrentLineText(foldRegions)
		lineLen := len([]rune(lineText))
		if lineLen > 0 {
			m.diffVisualCurCol = lineLen - 1
		}
		return m, nil
	case "^":
		lineText := m.diffCurrentLineText(foldRegions)
		m.diffVisualCurCol = firstNonWhitespace(lineText)
		return m, nil
	case "w":
		lineText := m.diffCurrentLineText(foldRegions)
		if lineText != "" {
			lineLen := len([]rune(lineText))
			newCol := nextWordStart(lineText, m.diffVisualCurCol)
			if newCol >= lineLen {
				newCol = max(lineLen-1, 0)
			}
			m.diffVisualCurCol = newCol
		}
		return m, nil
	case "b":
		lineText := m.diffCurrentLineText(foldRegions)
		if lineText != "" {
			newCol := prevWordStart(lineText, m.diffVisualCurCol)
			if newCol < 0 {
				newCol = 0
			}
			m.diffVisualCurCol = newCol
		}
		return m, nil
	case "e":
		lineText := m.diffCurrentLineText(foldRegions)
		if lineText != "" {
			lineLen := len([]rune(lineText))
			newCol := wordEnd(lineText, m.diffVisualCurCol)
			if newCol >= lineLen {
				newCol = max(lineLen-1, 0)
			}
			m.diffVisualCurCol = newCol
		}
		return m, nil
	case "E":
		lineText := m.diffCurrentLineText(foldRegions)
		if lineText != "" {
			lineLen := len([]rune(lineText))
			newCol := WORDEnd(lineText, m.diffVisualCurCol)
			if newCol >= lineLen {
				newCol = max(lineLen-1, 0)
			}
			m.diffVisualCurCol = newCol
		}
		return m, nil
	case "W":
		lineText := m.diffCurrentLineText(foldRegions)
		if lineText != "" {
			lineLen := len([]rune(lineText))
			newCol := nextWORDStart(lineText, m.diffVisualCurCol)
			if newCol >= lineLen {
				newCol = max(lineLen-1, 0)
			}
			m.diffVisualCurCol = newCol
		}
		return m, nil
	case "B":
		lineText := m.diffCurrentLineText(foldRegions)
		if lineText != "" {
			newCol := prevWORDStart(lineText, m.diffVisualCurCol)
			if newCol < 0 {
				newCol = 0
			}
			m.diffVisualCurCol = newCol
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.diffCursor = 0
			m.diffScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		maxCursor := max(totalLines-1, 0)
		m.diffCursor = maxCursor
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+d":
		maxCursor := max(totalLines-1, 0)
		m.diffCursor = min(m.diffCursor+m.height/2, maxCursor)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+u":
		m.diffCursor = max(m.diffCursor-m.height/2, 0)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+f":
		maxCursor := max(totalLines-1, 0)
		m.diffCursor = min(m.diffCursor+m.height, maxCursor)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+b":
		m.diffCursor = max(m.diffCursor-m.height, 0)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+c":
		m.diffVisualMode = false
		return m.closeTabOrQuit()
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

// diffScrollToMatch auto-expands the fold region containing the current match,
// scrolls to center it in the viewport, and moves the cursor column to the match.
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

	// Move cursor line and center in viewport.
	m.diffCursor = visIdx
	m.diffScroll = visIdx - viewportLines/2
	if m.diffScroll < 0 {
		m.diffScroll = 0
	}

	// Move cursor column to the match position on the active side.
	lineText := m.diffCurrentLineText(foldRegions)
	col := ui.DiffSearchColumnInLine(lineText, m.diffSearchQuery)
	if col >= 0 {
		m.diffVisualCurCol = col
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
