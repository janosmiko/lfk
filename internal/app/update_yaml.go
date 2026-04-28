package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

// yamlViewportLines returns the number of content lines available for the
// YAML viewer, accounting for the title bar, tab bar, borders, and hint bar.
func (m Model) yamlViewportLines() int {
	// Overhead: YAML title (1) + border top/bottom (2) + hint bar (1) = 4,
	// plus the global title bar (1) and tab bar (1 when multi-tab) which are
	// subtracted from m.height by View() at render time but NOT in Update().
	overhead := 5 // title bar + yaml title + border*2 + hint
	if len(m.tabs) > 1 {
		overhead = 6
	}
	lines := max(m.height-overhead, 3)
	return lines
}

func (m Model) handleYAMLKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When in search input mode, handle text input.
	if m.yamlSearchMode {
		return m.handleYAMLSearchInput(msg)
	}

	// In visual selection mode, restrict keys to selection/copy/cancel.
	if m.yamlVisualMode {
		return m.handleYAMLVisualKey(msg)
	}

	return m.handleYAMLNormalKey(msg)
}

// handleYAMLSearchInput handles key events when the YAML search input is active.
//
// Match highlights update on every keystroke so the user sees results land
// in real time instead of having to commit with Enter just to see whether
// the query matches anything. Enter still ends search-input mode and
// scrolls to the first match -- it's the "commit" action; typing only
// drives the live highlight overlay.
func (m Model) handleYAMLSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	viewportLines := m.yamlViewportLines()

	switch msg.String() {
	case "enter":
		m.yamlSearchMode = false
		m.updateYAMLSearchMatches()
		if len(m.yamlMatchLines) > 0 {
			m.yamlMatchIdx = m.findYAMLMatchFromCursor()
			m.yamlScrollToMatchFolded(viewportLines)
		}
		return m, nil
	case "esc":
		m.yamlSearchMode = false
		m.yamlSearchText.Clear()
		m.yamlMatchLines = nil
		m.yamlMatchIdx = 0
		return m, nil
	case "backspace":
		if len(m.yamlSearchText.Value) > 0 {
			m.yamlSearchText.Backspace()
		}
		m.updateYAMLSearchMatches()
		return m, nil
	case "ctrl+w":
		m.yamlSearchText.DeleteWord()
		m.updateYAMLSearchMatches()
		return m, nil
	case "ctrl+a":
		m.yamlSearchText.Home()
		return m, nil
	case "ctrl+e":
		m.yamlSearchText.End()
		return m, nil
	case "left":
		m.yamlSearchText.Left()
		return m, nil
	case "right":
		m.yamlSearchText.Right()
		return m, nil
	case "ctrl+c":
		m.yamlSearchMode = false
		m.yamlSearchText.Clear()
		m.yamlMatchLines = nil
		return m, nil
	default:
		if len(msg.String()) == 1 || msg.String() == " " {
			m.yamlSearchText.Insert(msg.String())
			m.updateYAMLSearchMatches()
		}
		return m, nil
	}
}

// handleYAMLVisualKey handles key events while in YAML visual selection mode.
func (m Model) handleYAMLVisualKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalVisible := visibleLineCount(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	maxScroll := m.yamlMaxScroll(totalVisible)

	switch msg.String() {
	case "esc":
		m.yamlVisualMode = false
		return m, nil
	case "V":
		return m.handleYAMLVisualToggleMode('V')
	case "v":
		return m.handleYAMLVisualToggleMode('v')
	case "ctrl+v":
		return m.handleYAMLVisualToggleMode('B')
	case "y":
		return m.handleYAMLVisualCopy()
	case "h", "left":
		if m.yamlVisualType == 'v' || m.yamlVisualType == 'B' {
			if m.yamlVisualCurCol > yamlFoldPrefixLen {
				m.yamlVisualCurCol--
			}
		}
		return m, nil
	case "l", "right":
		if m.yamlVisualType == 'v' || m.yamlVisualType == 'B' {
			m.yamlVisualCurCol++
		}
		return m, nil
	case "j", "down":
		if m.yamlCursor < totalVisible-1 {
			m.yamlCursor++
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "k", "up":
		if m.yamlCursor > 0 {
			m.yamlCursor--
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.yamlLineInput = ""
			m.yamlCursor = 0
			m.yamlScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		return m.handleYAMLVisualG(totalVisible, maxScroll)
	case "ctrl+d":
		m.yamlCursor += m.height / 2
		if m.yamlCursor >= totalVisible {
			m.yamlCursor = totalVisible - 1
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+u":
		m.yamlCursor -= m.height / 2
		if m.yamlCursor < 0 {
			m.yamlCursor = 0
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+c":
		m.yamlVisualMode = false
		m.mode = modeExplorer
		m.yamlScroll = 0
		m.yamlCursor = 0
		return m, nil
	case "0":
		m.yamlVisualCurCol = yamlFoldPrefixLen
		return m, nil
	case "$", "w", "b", "e", "E", "B", "W", "^":
		return m.handleYAMLVisualWordMotion(msg.String())
	}
	return m, nil
}

// handleYAMLVisualToggleMode toggles the visual selection type or cancels it.
func (m Model) handleYAMLVisualToggleMode(mode rune) (tea.Model, tea.Cmd) {
	if m.yamlVisualType == mode {
		m.yamlVisualMode = false
	} else {
		m.yamlVisualType = mode
	}
	return m, nil
}

// handleYAMLVisualG handles the G key in visual mode (go-to-line or end).
func (m Model) handleYAMLVisualG(totalVisible, maxScroll int) (tea.Model, tea.Cmd) {
	if m.yamlLineInput != "" {
		lineNum, _ := strconv.Atoi(m.yamlLineInput)
		m.yamlLineInput = ""
		if lineNum > 0 {
			lineNum--
		}
		m.yamlCursor = max(min(lineNum, totalVisible-1), 0)
		m.ensureYAMLCursorVisible()
	} else {
		m.yamlCursor = max(totalVisible-1, 0)
		m.yamlScroll = maxScroll
	}
	return m, nil
}

// handleYAMLVisualCopy copies the visually selected content to the clipboard.
func (m Model) handleYAMLVisualCopy() (tea.Model, tea.Cmd) {
	_, mapping := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	selStart := min(m.yamlVisualStart, m.yamlCursor)
	selEnd := max(m.yamlVisualStart, m.yamlCursor)
	if selStart < 0 {
		selStart = 0
	}
	if selEnd >= len(mapping) {
		selEnd = len(mapping) - 1
	}
	origLines := strings.Split(m.yamlContent, "\n")
	var clipText string
	switch m.yamlVisualType {
	case 'v':
		clipText = m.yamlVisualCopyChar(selStart, selEnd, mapping, origLines)
	case 'B':
		clipText = m.yamlVisualCopyBlock(selStart, selEnd, mapping, origLines)
	default:
		clipText = m.yamlVisualCopyLine(selStart, selEnd, mapping, origLines)
	}
	lineCount := selEnd - selStart + 1
	m.yamlVisualMode = false
	m.setStatusMessage(fmt.Sprintf("Copied %d lines", lineCount), false)
	return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
}

// yamlVisualCopyChar extracts text for character-mode visual selection.
func (m Model) yamlVisualCopyChar(selStart, selEnd int, mapping []int, origLines []string) string {
	var parts []string
	anchorCol := m.yamlVisualCol - yamlFoldPrefixLen
	cursorCol := m.yamlVisualCurCol - yamlFoldPrefixLen
	startCol, endCol := anchorCol, cursorCol
	if m.yamlVisualStart > m.yamlCursor {
		startCol, endCol = cursorCol, anchorCol
	}
	for i := selStart; i <= selEnd; i++ {
		if i >= len(mapping) || mapping[i] < 0 || mapping[i] >= len(origLines) {
			continue
		}
		line := origLines[mapping[i]]
		runes := []rune(line)
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
			cs := min(startCol, len(runes))
			parts = append(parts, string(runes[cs:]))
		} else if i == selEnd {
			ce := min(endCol+1, len(runes))
			parts = append(parts, string(runes[:ce]))
		} else {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, "\n")
}

// yamlVisualCopyBlock extracts text for block-mode visual selection.
func (m Model) yamlVisualCopyBlock(selStart, selEnd int, mapping []int, origLines []string) string {
	colStart := min(m.yamlVisualCol, m.yamlVisualCurCol) - yamlFoldPrefixLen
	colEnd := max(m.yamlVisualCol, m.yamlVisualCurCol) - yamlFoldPrefixLen + 1
	var parts []string
	for i := selStart; i <= selEnd; i++ {
		if i >= len(mapping) || mapping[i] < 0 || mapping[i] >= len(origLines) {
			continue
		}
		line := origLines[mapping[i]]
		runes := []rune(line)
		cs := colStart
		ce := colEnd
		if cs > len(runes) {
			cs = len(runes)
		}
		if ce > len(runes) {
			ce = len(runes)
		}
		parts = append(parts, string(runes[cs:ce]))
	}
	return strings.Join(parts, "\n")
}

// yamlVisualCopyLine extracts text for line-mode visual selection.
func (m Model) yamlVisualCopyLine(selStart, selEnd int, mapping []int, origLines []string) string {
	var selected []string
	for i := selStart; i <= selEnd; i++ {
		if i < len(mapping) && mapping[i] >= 0 && mapping[i] < len(origLines) {
			selected = append(selected, origLines[mapping[i]])
		}
	}
	return strings.Join(selected, "\n")
}

// handleYAMLVisualWordMotion handles word/WORD motion and cursor position keys
// in visual mode ($, w, b, e, E, B, W, ^).
func (m Model) handleYAMLVisualWordMotion(key string) (tea.Model, tea.Cmd) {
	visLines, _ := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	if m.yamlCursor < 0 || m.yamlCursor >= len(visLines) {
		return m, nil
	}

	switch key {
	case "$":
		lineLen := len([]rune(visLines[m.yamlCursor]))
		if lineLen > 0 {
			m.yamlVisualCurCol = lineLen - 1
		}
	case "^":
		col := max(firstNonWhitespace(visLines[m.yamlCursor]), yamlFoldPrefixLen)
		m.yamlVisualCurCol = col
	case "w":
		m.yamlWordForward(visLines, nextWordStart)
	case "W":
		m.yamlWordForward(visLines, nextWORDStart)
	case "b":
		m.yamlWordBackward(visLines, prevWordStart)
	case "B":
		m.yamlWordBackward(visLines, prevWORDStart)
	case "e":
		m.yamlWordForward(visLines, wordEnd)
	case "E":
		m.yamlWordForward(visLines, WORDEnd)
	}
	return m, nil
}

// yamlWordForward moves the cursor forward using the given word-motion function.
func (m *Model) yamlWordForward(visLines []string, motionFn func(string, int) int) {
	lineLen := len([]rune(visLines[m.yamlCursor]))
	newCol := motionFn(visLines[m.yamlCursor], m.yamlVisualCurCol)
	if newCol >= lineLen && m.yamlCursor < len(visLines)-1 {
		m.yamlCursor++
		newCol = motionFn(visLines[m.yamlCursor], 0)
		nextLineLen := len([]rune(visLines[m.yamlCursor]))
		if newCol >= nextLineLen {
			newCol = max(nextLineLen-1, 0)
		}
		m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
		m.ensureYAMLCursorVisible()
	} else {
		m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
	}
}

// yamlWordBackward moves the cursor backward using the given word-motion function.
func (m *Model) yamlWordBackward(visLines []string, motionFn func(string, int) int) {
	newCol := motionFn(visLines[m.yamlCursor], m.yamlVisualCurCol)
	if newCol < 0 && m.yamlCursor > 0 {
		m.yamlCursor--
		lineLen := len([]rune(visLines[m.yamlCursor]))
		newCol = max(motionFn(visLines[m.yamlCursor], lineLen), 0)
		m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
		m.ensureYAMLCursorVisible()
	} else {
		m.yamlVisualCurCol = max(yamlFoldPrefixLen, max(newCol, 0))
	}
}

// yamlMaxScroll returns the maximum scroll offset for the YAML viewer.
func (m Model) yamlMaxScroll(totalVisible int) int {
	viewportLines := m.yamlViewportLines()
	maxScroll := max(totalVisible-viewportLines, 0)
	return maxScroll
}

// handleYAMLNormalKey handles key events in normal YAML viewing mode.
func (m Model) handleYAMLNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalVisible := visibleLineCount(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	viewportLines := m.yamlViewportLines()
	maxScroll := m.yamlMaxScroll(totalVisible)

	switch msg.String() {
	case "?", "f1":
		return m.handleYAMLKeyQuestion()
	case "V":
		return m.handleYAMLKeyV()
	case "v":
		return m.handleYAMLKeyV2()
	case "ctrl+v":
		return m.handleYAMLKeyCtrlV()
	case "q", "esc":
		return m.handleYAMLKeyQ()
	case "ctrl+c":
		return m.handleYAMLKeyCtrlC()
	case "/":
		return m.handleYAMLKeySlash()
	case "n":
		return m.handleYAMLKeyN(viewportLines)
	case "N":
		return m.handleYAMLKeyShiftN(viewportLines)
	case "ctrl+e":
		return m.handleYAMLKeyCtrlE()
	case "ctrl+w", ">":
		m.yamlWrap = !m.yamlWrap
		return m, nil
	case "z":
		return m.handleYAMLKeyFoldToggle()
	case "Z":
		return m.handleYAMLKeyZ()
	case "h", "left":
		return m.handleYAMLKeyH()
	case "l", "right":
		m.yamlVisualCurCol++
		return m, nil
	case "0":
		return m.handleYAMLKeyZero()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.yamlLineInput += msg.String()
		return m, nil
	case "$", "w", "b", "e", "E", "B", "W", "^":
		return m.handleYAMLVisualWordMotion(msg.String())
	case "j", "down":
		m.yamlLineInput = ""
		if m.yamlCursor < totalVisible-1 {
			m.yamlCursor++
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "k", "up":
		return m.handleYAMLKeyK()
	case "g":
		return m.handleYAMLKeyG()
	case "G", "end":
		return m.handleYAMLNormalG(totalVisible, maxScroll)
	case "home":
		m.yamlCursor = 0
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+d":
		return m.handleYAMLNormalHalfPageDown(totalVisible)
	case "ctrl+u":
		return m.handleYAMLKeyCtrlU()
	case "ctrl+f", "pgdown":
		return m.handleYAMLNormalPageDown(totalVisible)
	case "ctrl+b", "pgup":
		return m.handleYAMLKeyCtrlB()
	default:
		m.yamlLineInput = ""
	}
	return m, nil
}

// handleYAMLKeyN handles the 'n' key (next search match) in normal YAML mode.
func (m Model) handleYAMLKeyN(viewportLines int) (tea.Model, tea.Cmd) {
	if len(m.yamlMatchLines) > 0 {
		if m.yamlNextIntraLineMatch(true) {
			return m, nil
		}
		m.yamlMatchIdx = (m.yamlMatchIdx + 1) % len(m.yamlMatchLines)
		m.yamlScrollToMatchFolded(viewportLines)
	}
	return m, nil
}

// handleYAMLKeyShiftN handles the 'N' key (previous search match) in normal YAML mode.
func (m Model) handleYAMLKeyShiftN(viewportLines int) (tea.Model, tea.Cmd) {
	if len(m.yamlMatchLines) > 0 {
		if m.yamlNextIntraLineMatch(false) {
			return m, nil
		}
		m.yamlMatchIdx--
		if m.yamlMatchIdx < 0 {
			m.yamlMatchIdx = len(m.yamlMatchLines) - 1
		}
		m.yamlScrollToMatchFolded(viewportLines)
	}
	return m, nil
}

// handleYAMLKeyCtrlE handles ctrl+e (edit resource) in normal YAML mode.
func (m Model) handleYAMLKeyCtrlE() (tea.Model, tea.Cmd) {
	kind := m.selectedResourceKind()
	sel := m.selectedMiddleItem()
	if kind != "" && sel != nil {
		m.actionCtx = m.buildActionCtx(sel, kind)
		return m, m.execKubectlEdit()
	}
	return m, nil
}

// handleYAMLKeyFoldToggle toggles the fold on the section at the cursor position.
func (m Model) handleYAMLKeyFoldToggle() (tea.Model, tea.Cmd) {
	_, mapping := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	sec := sectionAtScrollPos(m.yamlCursor, mapping, m.yamlSections)
	if sec != "" {
		if m.yamlCollapsed == nil {
			m.yamlCollapsed = make(map[string]bool)
		}
		m.yamlCollapsed[sec] = !m.yamlCollapsed[sec]

		if m.yamlCollapsed[sec] {
			var startLine int
			for _, s := range m.yamlSections {
				if s.key == sec {
					startLine = s.startLine
					break
				}
			}
			_, newMapping := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
			for vi, orig := range newMapping {
				if orig == startLine {
					m.yamlCursor = vi
					break
				}
			}
		}

		m.clampYAMLScroll()
		m.ensureYAMLCursorVisible()
	}
	return m, nil
}

// handleYAMLNormalG handles the G key in normal mode (go-to-line or end).
func (m Model) handleYAMLNormalG(totalVisible, maxScroll int) (tea.Model, tea.Cmd) {
	if m.yamlLineInput != "" {
		lineNum, _ := strconv.Atoi(m.yamlLineInput)
		m.yamlLineInput = ""
		if lineNum > 0 {
			lineNum--
		}
		if lineNum >= totalVisible {
			lineNum = totalVisible - 1
		}
		if lineNum < 0 {
			lineNum = 0
		}
		m.yamlCursor = lineNum
		m.ensureYAMLCursorVisible()
		return m, nil
	}
	m.yamlCursor = max(totalVisible-1, 0)
	m.yamlScroll = maxScroll
	return m, nil
}

// handleYAMLNormalHalfPageDown handles ctrl+d (half page down) in normal YAML mode.
func (m Model) handleYAMLNormalHalfPageDown(totalVisible int) (tea.Model, tea.Cmd) {
	m.yamlLineInput = ""
	m.yamlCursor += m.height / 2
	if m.yamlCursor >= totalVisible {
		m.yamlCursor = totalVisible - 1
	}
	m.ensureYAMLCursorVisible()
	return m, nil
}

// handleYAMLNormalPageDown handles ctrl+f (full page down) in normal YAML mode.
func (m Model) handleYAMLNormalPageDown(totalVisible int) (tea.Model, tea.Cmd) {
	m.yamlLineInput = ""
	m.yamlCursor += m.height
	if m.yamlCursor >= totalVisible {
		m.yamlCursor = totalVisible - 1
	}
	m.ensureYAMLCursorVisible()
	return m, nil
}

// ensureYAMLCursorVisible adjusts yamlScroll so the cursor is within the viewport
// with scrolloff margin.
func (m *Model) ensureYAMLCursorVisible() {
	maxLines := m.yamlViewportLines()
	so := min(ui.ConfigScrollOff, maxLines/2)
	if m.yamlCursor < m.yamlScroll+so {
		m.yamlScroll = m.yamlCursor - so
	}
	if m.yamlCursor >= m.yamlScroll+maxLines-so {
		m.yamlScroll = m.yamlCursor - maxLines + so + 1
	}
	if m.yamlScroll < 0 {
		m.yamlScroll = 0
	}
}

// clampYAMLScroll ensures yamlScroll stays within bounds after fold changes.
func (m *Model) clampYAMLScroll() {
	totalVisible := visibleLineCount(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	viewportLines := m.yamlViewportLines()
	maxScroll := max(totalVisible-viewportLines, 0)
	if m.yamlScroll > maxScroll {
		m.yamlScroll = maxScroll
	}
	if m.yamlScroll < 0 {
		m.yamlScroll = 0
	}
}

// yamlScrollToMatchFolded scrolls to show the current search match, expanding
// the containing section if it is collapsed, and using visible-line coordinates.
func (m *Model) yamlScrollToMatchFolded(viewportLines int) {
	if m.yamlMatchIdx < 0 || m.yamlMatchIdx >= len(m.yamlMatchLines) {
		return
	}
	targetOrig := m.yamlMatchLines[m.yamlMatchIdx]

	// If the match is inside a collapsed section, expand it.
	for _, sec := range m.yamlSections {
		if m.yamlCollapsed[sec.key] && targetOrig > sec.startLine && targetOrig <= sec.endLine {
			m.yamlCollapsed[sec.key] = false
		}
	}

	// Convert original line to visible line.
	_, mapping := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	visIdx := originalToVisible(targetOrig, mapping)
	if visIdx < 0 {
		return
	}

	totalVisible := len(mapping)
	maxScroll := max(totalVisible-viewportLines, 0)

	// Move cursor to the match and center it in the viewport.
	m.yamlCursor = visIdx
	// Move cursor column to the match position within the visible line
	// (which includes fold prefixes).
	visibleLines, _ := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	if visIdx >= 0 && visIdx < len(visibleLines) {
		col := ui.FindColumnInLine(visibleLines[visIdx], m.yamlSearchText.Value)
		if col >= 0 {
			m.yamlVisualCurCol = col
		}
	}
	m.yamlScroll = max(min(visIdx-viewportLines/2, maxScroll), 0)
}

// yamlNextIntraLineMatch checks for another match on the current YAML line
// after (forward=true) or before (forward=false) the cursor column.
// Returns true if a match was found and cursor was moved.
func (m *Model) yamlNextIntraLineMatch(forward bool) bool {
	if m.yamlSearchText.Value == "" {
		return false
	}
	rawQuery := m.yamlSearchText.Value

	// Use visible lines (which include fold prefixes) for accurate column positions.
	visibleLines, _ := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	if m.yamlCursor < 0 || m.yamlCursor >= len(visibleLines) {
		return false
	}
	line := visibleLines[m.yamlCursor]

	runes := []rune(line)
	if forward {
		// Search for a match after the current cursor position.
		// Clamp: yamlVisualCurCol carries the column from a previously
		// focused line and may exceed this line's rune length. Forward
		// uses +1 because the search starts after (not at) the cursor.
		end := min(m.yamlVisualCurCol+1, len(runes))
		curBytePos := len(string(runes[:end]))
		if curBytePos < len(line) {
			remainder := line[curBytePos:]
			col := ui.FindColumnInLine(remainder, rawQuery)
			if col >= 0 {
				m.yamlVisualCurCol = m.yamlVisualCurCol + 1 + col
				return true
			}
		}
	} else {
		// Search for a match before the current cursor position.
		// Clamp: yamlVisualCurCol may exceed this line's rune length;
		// backward search ends at (excluding) the cursor.
		end := min(m.yamlVisualCurCol, len(runes))
		curBytePos := len(string(runes[:end]))
		if curBytePos > 0 {
			prefix := line[:curBytePos]
			// For backward search, find the last match in the prefix.
			// FindColumnInLine returns the first match; iterate to find the last.
			lastCol := -1
			remaining := prefix
			offset := 0
			for {
				col := ui.FindColumnInLine(remaining, rawQuery)
				if col < 0 {
					break
				}
				lastCol = offset + col
				// Advance past this match to find the next one.
				advanceRunes := col + 1
				runes := []rune(remaining)
				if advanceRunes >= len(runes) {
					break
				}
				remaining = string(runes[advanceRunes:])
				offset += advanceRunes
			}
			if lastCol >= 0 {
				m.yamlVisualCurCol = lastCol
				return true
			}
		}
	}
	return false
}

// updateYAMLSearchMatches finds all lines matching the current search text.
// Supports substring, regex, and fuzzy search modes.
func (m *Model) updateYAMLSearchMatches() {
	m.yamlMatchLines = nil
	if m.yamlSearchText.Value == "" {
		return
	}
	rawQuery := m.yamlSearchText.Value
	for i, line := range strings.Split(m.yamlContent, "\n") {
		if ui.MatchLine(line, rawQuery) {
			m.yamlMatchLines = append(m.yamlMatchLines, i)
		}
	}
}

// findYAMLMatchFromCursor returns the index of the first match at or after the
// current cursor position. Wraps to 0 if no match is found after the cursor.
func (m *Model) findYAMLMatchFromCursor() int {
	_, mapping := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	origLine := 0
	if m.yamlCursor >= 0 && m.yamlCursor < len(mapping) {
		origLine = mapping[m.yamlCursor]
	}
	for i, matchLine := range m.yamlMatchLines {
		if matchLine >= origLine {
			return i
		}
	}
	return 0
}

func (m Model) handleYAMLKeyQuestion() (tea.Model, tea.Cmd) {
	m.helpPreviousMode = modeYAML
	m.mode = modeHelp
	m.helpScroll = 0
	m.helpFilter.Clear()
	m.helpSearchActive = false
	m.helpContextMode = "YAML View"
	return m, nil
}

func (m Model) handleYAMLKeyV() (tea.Model, tea.Cmd) {
	m.yamlVisualMode = true
	m.yamlVisualType = 'V'
	m.yamlVisualStart = m.yamlCursor
	m.yamlVisualCol = m.yamlVisualCurCol
	return m, nil
}

func (m Model) handleYAMLKeyV2() (tea.Model, tea.Cmd) {
	m.yamlVisualMode = true
	m.yamlVisualType = 'v'
	m.yamlVisualStart = m.yamlCursor
	m.yamlVisualCol = m.yamlVisualCurCol
	return m, nil
}

func (m Model) handleYAMLKeyCtrlV() (tea.Model, tea.Cmd) {
	m.yamlVisualMode = true
	m.yamlVisualType = 'B'
	m.yamlVisualStart = m.yamlCursor
	m.yamlVisualCol = m.yamlVisualCurCol
	return m, nil
}

func (m Model) handleYAMLKeyQ() (tea.Model, tea.Cmd) {
	if m.yamlSearchText.Value != "" {
		// Clear search first.
		m.yamlSearchText.Clear()
		m.yamlMatchLines = nil
		m.yamlMatchIdx = 0
		return m, nil
	}
	m.mode = modeExplorer
	m.yamlScroll = 0
	m.yamlCursor = 0
	m.yamlWrap = false
	return m, nil
}

func (m Model) handleYAMLKeyCtrlC() (tea.Model, tea.Cmd) {
	m.mode = modeExplorer
	m.yamlScroll = 0
	m.yamlCursor = 0
	m.yamlWrap = false
	m.yamlSearchText.Clear()
	m.yamlMatchLines = nil
	return m, nil
}

func (m Model) handleYAMLKeySlash() (tea.Model, tea.Cmd) {
	m.yamlSearchMode = true
	m.yamlSearchText.Clear()
	m.yamlMatchLines = nil
	m.yamlMatchIdx = 0
	return m, nil
}

func (m Model) handleYAMLKeyZ() (tea.Model, tea.Cmd) {
	if m.yamlCollapsed == nil {
		m.yamlCollapsed = make(map[string]bool)
	}
	anyExpanded := false
	for _, sec := range m.yamlSections {
		if isMultiLineSection(sec) && !m.yamlCollapsed[sec.key] {
			anyExpanded = true
			break
		}
	}
	if anyExpanded {
		for _, sec := range m.yamlSections {
			if isMultiLineSection(sec) {
				m.yamlCollapsed[sec.key] = true
			}
		}
	} else {
		m.yamlCollapsed = make(map[string]bool)
	}
	m.clampYAMLScroll()
	return m, nil
}

func (m Model) handleYAMLKeyH() (tea.Model, tea.Cmd) {
	if m.yamlVisualCurCol > yamlFoldPrefixLen {
		m.yamlVisualCurCol--
	}
	return m, nil
}

func (m Model) handleYAMLKeyZero() (tea.Model, tea.Cmd) {
	if m.yamlLineInput != "" {
		m.yamlLineInput += "0"
	} else {
		m.yamlVisualCurCol = yamlFoldPrefixLen
	}
	return m, nil
}

func (m Model) handleYAMLKeyK() (tea.Model, tea.Cmd) {
	m.yamlLineInput = ""
	if m.yamlCursor > 0 {
		m.yamlCursor--
	}
	m.ensureYAMLCursorVisible()
	return m, nil
}

func (m Model) handleYAMLKeyG() (tea.Model, tea.Cmd) {
	m.yamlLineInput = ""
	if m.pendingG {
		m.pendingG = false
		m.yamlCursor = 0
		m.yamlScroll = 0
		return m, nil
	}
	m.pendingG = true
	return m, nil
}

func (m Model) handleYAMLKeyCtrlU() (tea.Model, tea.Cmd) {
	m.yamlLineInput = ""
	m.yamlCursor -= m.height / 2
	if m.yamlCursor < 0 {
		m.yamlCursor = 0
	}
	m.ensureYAMLCursorVisible()
	return m, nil
}

func (m Model) handleYAMLKeyCtrlB() (tea.Model, tea.Cmd) {
	m.yamlLineInput = ""
	m.yamlCursor -= m.height
	if m.yamlCursor < 0 {
		m.yamlCursor = 0
	}
	m.ensureYAMLCursorVisible()
	return m, nil
}
