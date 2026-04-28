package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleLogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle log search input mode.
	if m.logSearchActive {
		return m.handleLogSearchKey(msg)
	}

	// Handle visual select mode keys.
	if m.logVisualMode {
		return m.handleLogVisualKey(msg)
	}

	// Try movement keys.
	if ret, cmd, ok := m.handleLogMovementKey(msg); ok {
		return ret, cmd
	}
	// Try action/mode keys.
	if ret, cmd, ok := m.handleLogActionKey(msg); ok {
		return ret, cmd
	}
	m.logLineInput = ""
	return m, nil
}

// handleLogMovementKey handles cursor/scroll movement keys in the log viewer.
func (m Model) handleLogMovementKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "j", "down":
		ret := m.handleLogKeyJ()
		return ret, nil, true
	case "k", "up":
		ret, cmd := m.handleLogKeyK()
		return ret, cmd, true
	case "ctrl+d":
		ret := m.handleLogKeyCtrlD()
		return ret, nil, true
	case "ctrl+u":
		ret, cmd := m.handleLogKeyCtrlU()
		return ret, cmd, true
	case "ctrl+f", "pgdown":
		ret := m.handleLogKeyCtrlF()
		return ret, nil, true
	case "ctrl+b", "pgup":
		ret, cmd := m.handleLogKeyCtrlB()
		return ret, cmd, true
	case "G", "end":
		ret := m.handleLogKeyG()
		return ret, nil, true
	case "g":
		ret, cmd := m.handleLogKeyG2()
		return ret, cmd, true
	case "home":
		m.pendingG = false
		m.logCursor = 0
		m.logScroll = 0
		m.logFollow = false
		return m, nil, true
	case "h", "left":
		ret := m.handleLogKeyH()
		return ret, nil, true
	case "l", "right":
		ret := m.handleLogKeyL()
		return ret, nil, true
	case "$":
		ret := m.handleLogKeyDollar()
		return ret, nil, true
	case "e":
		ret := m.handleLogKeyE()
		return ret, nil, true
	case "b":
		ret := m.handleLogKeyB()
		return ret, nil, true
	case "w":
		ret := m.handleLogKeyW()
		return ret, nil, true
	case "W":
		ret := m.handleLogKeyW2()
		return ret, nil, true
	case "E":
		ret := m.handleLogKeyE2()
		return ret, nil, true
	case "B":
		ret := m.handleLogKeyB2()
		return ret, nil, true
	case "^":
		ret := m.handleLogKeyCaret()
		return ret, nil, true
	case "0":
		ret := m.handleLogKeyZero()
		return ret, nil, true
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.logLineInput += msg.String()
		return m, nil, true
	}
	return m, nil, false
}

// handleLogActionKey handles action/mode keys in the log viewer.
func (m Model) handleLogActionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.String() {
	case "?", "f1":
		ret := m.handleLogKeyQuestion()
		return ret, nil, true
	case "q", "esc":
		ret := m.handleLogKeyQ()
		return ret, nil, true
	case "V":
		ret := m.handleLogKeyV()
		return ret, nil, true
	case "v":
		ret := m.handleLogKeyV2()
		return ret, nil, true
	case "ctrl+v":
		ret := m.handleLogKeyCtrlV()
		return ret, nil, true
	case "f":
		ret := m.handleLogKeyF()
		return ret, nil, true
	case "tab", "z", ">":
		ret := m.handleLogKeyTab()
		return ret, nil, true
	case "/":
		ret := m.handleLogKeySlash()
		return ret, nil, true
	case "n":
		ret := m.handleLogKeyN()
		return ret, nil, true
	case "N":
		ret := m.handleLogKeyN2()
		return ret, nil, true
	case "p":
		ret := m.handleLogKeyP()
		return ret, nil, true
	case "P":
		ret := m.handleLogKeyP2()
		return ret, nil, true
	case "J":
		if !m.logPreviewVisible {
			return m, nil, false
		}
		ret := m.handleLogKeyJ2()
		return ret, nil, true
	case "K":
		if !m.logPreviewVisible {
			return m, nil, false
		}
		ret := m.handleLogKeyK2()
		return ret, nil, true
	case "#":
		ret := m.handleLogKeyHash()
		return ret, nil, true
	case "s":
		ret := m.handleLogKeyS()
		return ret, nil, true
	case "S":
		ret, cmd := m.handleLogKeyS2()
		return ret, cmd, true
	case "ctrl+s":
		ret, cmd := m.handleLogKeyCtrlS()
		return ret, cmd, true
	case "c":
		ret, cmd := m.handleLogKeyC()
		return ret, cmd, true
	case "\\":
		ret, cmd := m.handleLogKeyOther()
		return ret, cmd, true
	case "ctrl+c":
		ret, cmd := m.handleLogKeyCtrlC()
		return ret, cmd, true
	}
	return m, nil, false
}

func (m Model) handleLogVisualKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.logVisualMode = false
		return m, nil
	case "V":
		// Toggle: if already in line mode, cancel; otherwise switch to line mode.
		return m.handleLogVisualKeyV()
	case "v":
		// Toggle: if already in char mode, cancel; otherwise switch to char mode.
		return m.handleLogVisualKeyV2()
	case "ctrl+v":
		// Toggle: if already in block mode, cancel; otherwise switch to block mode.
		return m.handleLogVisualKeyCtrlV()
	case "y":
		// Copy selected content to clipboard.
		return m.handleLogVisualKeyY()
	case "h", "left":
		// Move cursor column left (for char and block modes).
		return m.handleLogVisualKeyH()
	case "l", "right":
		// Move cursor column right (for char and block modes).
		return m.handleLogVisualKeyL()
	case "j", "down":
		return m.handleLogVisualKeyJ()
	case "k", "up":
		return m.handleLogVisualKeyK()
	case "G":
		return m.handleLogVisualKeyG()
	case "g":
		return m.handleLogVisualKeyG2()
	case "ctrl+d":
		return m.handleLogVisualKeyCtrlD()
	case "ctrl+u":
		return m.handleLogVisualKeyCtrlU()
	case "ctrl+c":
		m.logVisualMode = false
		return m.closeTabOrQuit()
	case "q":
		m.logVisualMode = false
		return m, nil
	case "$":
		return m.handleLogVisualKeyDollar()
	case "e":
		return m.handleLogVisualKeyE()
	case "b":
		return m.handleLogVisualKeyB()
	case "w":
		return m.handleLogVisualKeyW()
	case "W":
		return m.handleLogVisualKeyW2()
	case "E":
		return m.handleLogVisualKeyE2()
	case "B":
		return m.handleLogVisualKeyB2()
	case "0":
		m.logVisualCurCol = 0
		return m, nil
	case "^":
		return m.handleLogVisualKeyCaret()
	}
	return m, nil
}

func (m Model) handleLogSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.logSearchActive = false
		m.logSearchQuery = m.logSearchInput.Value
		m.findNextLogMatch(true)
	case "esc":
		m.logSearchActive = false
		m.logSearchInput.Clear()
	case "backspace":
		if len(m.logSearchInput.Value) > 0 {
			m.logSearchInput.Backspace()
		}
	case "ctrl+w":
		m.logSearchInput.DeleteWord()
	case "ctrl+a":
		m.logSearchInput.Home()
	case "ctrl+e":
		m.logSearchInput.End()
	case "left":
		m.logSearchInput.Left()
	case "right":
		m.logSearchInput.Right()
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.logSearchInput.Insert(key)
		}
	}
	return m, nil
}

// logDisplayLine returns the log line as shown on screen (timestamps/prefixes stripped).
func (m *Model) logDisplayLine(lineIdx int) string {
	line := m.logLines[lineIdx]
	if !m.logTimestamps {
		line = ui.StripTimestamp(line)
	}
	if m.logHidePrefixes {
		line = ui.StripPodPrefix(line)
	}
	return line
}

// logJumpToCol sets the cursor to the given line and rune column.
func (m *Model) logJumpToCol(lineIdx, runeCol int) {
	m.logCursor = lineIdx
	m.logVisualCurCol = runeCol
	m.logFollow = false
	m.ensureLogCursorVisible()
}

// logFindFirstMatch finds the first match in a line and jumps to it.
func (m *Model) logFindFirstMatch(lineIdx int, query string) bool {
	dl := m.logDisplayLine(lineIdx)
	col := ui.FindColumnInLine(dl, query)
	if col < 0 {
		return false
	}
	m.logJumpToCol(lineIdx, col)
	return true
}

// logFindLastMatch finds the last (rightmost) match in a line and jumps to it.
func (m *Model) logFindLastMatch(lineIdx int, query string) bool {
	dl := m.logDisplayLine(lineIdx)
	if !ui.MatchLine(dl, query) {
		return false
	}
	lastCol := -1
	remaining := dl
	offset := 0
	for {
		col := ui.FindColumnInLine(remaining, query)
		if col < 0 {
			break
		}
		lastCol = offset + col
		advanceRunes := col + 1
		runes := []rune(remaining)
		if advanceRunes >= len(runes) {
			break
		}
		remaining = string(runes[advanceRunes:])
		offset += advanceRunes
	}
	if lastCol < 0 {
		return false
	}
	m.logJumpToCol(lineIdx, lastCol)
	return true
}

func (m *Model) findNextLogMatch(forward bool) {
	if m.logSearchQuery == "" {
		return
	}
	rawQuery := m.logSearchQuery
	start := m.logCursor
	if start < 0 {
		start = m.logScroll
	}

	if forward {
		m.findNextLogMatchForward(rawQuery, start)
	} else {
		m.findNextLogMatchBackward(rawQuery, start)
	}
}

func (m *Model) findNextLogMatchForward(rawQuery string, start int) {
	// Check for another match on the current line after the cursor.
	if start >= 0 && start < len(m.logLines) {
		dl := m.logDisplayLine(start)
		runes := []rune(dl)
		// Clamp: logVisualCurCol carries the column from a previously
		// focused line and may exceed this line's rune length. Forward
		// uses +1 because the search starts after (not at) the cursor.
		end := min(m.logVisualCurCol+1, len(runes))
		curBytePos := len(string(runes[:end]))
		if curBytePos < len(dl) {
			col := ui.FindColumnInLine(dl[curBytePos:], rawQuery)
			if col >= 0 {
				m.logJumpToCol(start, m.logVisualCurCol+1+col)
				return
			}
		}
	}
	for i := start + 1; i < len(m.logLines); i++ {
		if m.logFindFirstMatch(i, rawQuery) {
			return
		}
	}
	for i := 0; i <= start; i++ {
		if m.logFindFirstMatch(i, rawQuery) {
			return
		}
	}
}

func (m *Model) findNextLogMatchBackward(rawQuery string, start int) {
	// Check for a match on the current line before the cursor.
	if start >= 0 && start < len(m.logLines) {
		dl := m.logDisplayLine(start)
		runes := []rune(dl)
		// Clamp: logVisualCurCol may exceed this line's rune length;
		// backward search ends at (excluding) the cursor.
		end := min(m.logVisualCurCol, len(runes))
		curBytePos := len(string(runes[:end]))
		if curBytePos > 0 {
			lastCol := findLastMatchInStr(dl[:curBytePos], rawQuery)
			if lastCol >= 0 {
				m.logJumpToCol(start, lastCol)
				return
			}
		}
	}
	for i := start - 1; i >= 0; i-- {
		if m.logFindLastMatch(i, rawQuery) {
			return
		}
	}
	for i := len(m.logLines) - 1; i >= start; i-- {
		if m.logFindLastMatch(i, rawQuery) {
			return
		}
	}
}

// findLastMatchInStr finds the rightmost match column in a string.
func findLastMatchInStr(text, query string) int {
	lastCol := -1
	remaining := text
	offset := 0
	for {
		col := ui.FindColumnInLine(remaining, query)
		if col < 0 {
			break
		}
		lastCol = offset + col
		advanceRunes := col + 1
		runes := []rune(remaining)
		if advanceRunes >= len(runes) {
			break
		}
		remaining = string(runes[advanceRunes:])
		offset += advanceRunes
	}
	return lastCol
}

func (m Model) handleLogKeyQuestion() Model {
	m.helpPreviousMode = modeLogs
	m.mode = modeHelp
	m.helpScroll = 0
	m.helpFilter.Clear()
	m.helpSearchActive = false
	m.helpContextMode = "Log Viewer"
	return m
}

func (m Model) handleLogKeyQ() Model {
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
	}
	if m.logHistoryCancel != nil {
		m.logHistoryCancel()
		m.logHistoryCancel = nil
	}
	m.logCh = nil
	m.mode = modeExplorer
	m.logLineInput = ""
	m.logSearchQuery = ""
	m.logSearchInput.Clear()
	m.logParentKind = ""
	m.logParentName = ""
	m.logVisualMode = false
	return m
}

func (m Model) handleLogKeyJ() Model {
	m.logFollow = false
	m.logLineInput = ""
	if m.logCursor < len(m.logLines)-1 {
		m.logCursor++
	}
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyK() (tea.Model, tea.Cmd) {
	m.logFollow = false
	m.logLineInput = ""
	if m.logCursor > 0 {
		m.logCursor--
	}
	m.ensureLogCursorVisible()
	cmd := m.maybeLoadMoreHistory()
	return m, cmd
}

func (m Model) handleLogKeyCtrlD() Model {
	m.logFollow = false
	m.logLineInput = ""
	m.logCursor += m.logContentHeight() / 2
	if m.logCursor >= len(m.logLines) {
		m.logCursor = len(m.logLines) - 1
	}
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyCtrlU() (tea.Model, tea.Cmd) {
	m.logFollow = false
	m.logLineInput = ""
	m.logCursor -= m.logContentHeight() / 2
	if m.logCursor < 0 {
		m.logCursor = 0
	}
	m.ensureLogCursorVisible()
	cmd := m.maybeLoadMoreHistory()
	return m, cmd
}

func (m Model) handleLogKeyCtrlF() Model {
	m.logFollow = false
	m.logLineInput = ""
	m.logCursor += m.logContentHeight()
	if m.logCursor >= len(m.logLines) {
		m.logCursor = len(m.logLines) - 1
	}
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyCtrlB() (tea.Model, tea.Cmd) {
	m.logFollow = false
	m.logLineInput = ""
	m.logCursor -= m.logContentHeight()
	if m.logCursor < 0 {
		m.logCursor = 0
	}
	m.ensureLogCursorVisible()
	cmd := m.maybeLoadMoreHistory()
	return m, cmd
}

func (m Model) handleLogKeyG() Model {
	if m.logLineInput != "" {
		lineNum, _ := strconv.Atoi(m.logLineInput)
		m.logLineInput = ""
		if lineNum > 0 {
			lineNum-- // 0-indexed
		}
		m.logCursor = min(lineNum, len(m.logLines)-1)
		m.logFollow = false
	} else {
		m.logCursor = len(m.logLines) - 1
		m.logFollow = true
	}
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyG2() (tea.Model, tea.Cmd) {
	if m.pendingG {
		m.pendingG = false
		m.logFollow = false
		m.logLineInput = ""
		m.logCursor = 0
		m.ensureLogCursorVisible()
		cmd := m.maybeLoadMoreHistory()
		return m, cmd
	}
	m.pendingG = true
	return m, nil
}

func (m Model) handleLogKeyH() Model {
	m.logLineInput = ""
	if m.logVisualCurCol > 0 {
		m.logVisualCurCol--
	}
	return m
}

func (m Model) handleLogKeyL() Model {
	m.logLineInput = ""
	m.logVisualCurCol++
	return m
}

func (m Model) handleLogKeyDollar() Model {
	m.logLineInput = ""
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		lineLen := len([]rune(m.logLines[m.logCursor]))
		if lineLen > 0 {
			m.logVisualCurCol = lineLen - 1
		}
	}
	return m
}

func (m Model) handleLogKeyE() Model {
	m.logLineInput = ""
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		lineLen := len([]rune(m.logLines[m.logCursor]))
		newCol := wordEnd(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol >= lineLen && m.logCursor < len(m.logLines)-1 {
			m.logCursor++
			newCol = wordEnd(m.logLines[m.logCursor], 0)
			nextLineLen := len([]rune(m.logLines[m.logCursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logVisualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logVisualCurCol = newCol
		}
	}
	return m
}

func (m Model) handleLogKeyB() Model {
	m.logLineInput = ""
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		newCol := prevWordStart(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol < 0 && m.logCursor > 0 {
			m.logCursor--
			lineLen := len([]rune(m.logLines[m.logCursor]))
			newCol = max(prevWordStart(m.logLines[m.logCursor], lineLen), 0)
			m.logVisualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logVisualCurCol = max(newCol, 0)
		}
	}
	return m
}

func (m Model) handleLogKeyV() Model {
	m.logLineInput = ""
	if m.logCursor < 0 {
		m.logCursor = m.logScroll
	}
	m.logVisualMode = true
	m.logVisualType = 'V'
	m.logVisualStart = m.logCursor
	m.logVisualCol = m.logVisualCurCol
	return m
}

func (m Model) handleLogKeyV2() Model {
	m.logLineInput = ""
	if m.logCursor < 0 {
		m.logCursor = m.logScroll
	}
	m.logVisualMode = true
	m.logVisualType = 'v'
	m.logVisualStart = m.logCursor
	m.logVisualCol = m.logVisualCurCol
	return m
}

func (m Model) handleLogKeyCtrlV() Model {
	m.logLineInput = ""
	if m.logCursor < 0 {
		m.logCursor = m.logScroll
	}
	m.logVisualMode = true
	m.logVisualType = 'B'
	m.logVisualStart = m.logCursor
	m.logVisualCol = m.logVisualCurCol
	return m
}

func (m Model) handleLogKeyF() Model {
	m.logLineInput = ""
	m.logFollow = !m.logFollow
	if m.logFollow {
		m.logCursor = len(m.logLines) - 1
		m.logScroll, m.logWrapTopSkip = m.logMaxScrollAndSkip()
	}
	return m
}

func (m Model) handleLogKeyTab() Model {
	m.logLineInput = ""
	m.logWrap = !m.logWrap
	// Re-pin to the bottom on toggle: maxScroll and topSkip both depend on
	// wrap mode, so the previous values are stale. ensureLogCursorVisible
	// snaps to the follow position when m.logFollow is true and otherwise
	// just clamps + clears the sub-line skip.
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyW() Model {
	m.logLineInput = ""
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		lineLen := len([]rune(m.logLines[m.logCursor]))
		newCol := nextWordStart(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol >= lineLen && m.logCursor < len(m.logLines)-1 {
			m.logCursor++
			newCol = nextWordStart(m.logLines[m.logCursor], 0)
			nextLineLen := len([]rune(m.logLines[m.logCursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logVisualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logVisualCurCol = newCol
		}
	}
	return m
}

func (m Model) handleLogKeyW2() Model {
	m.logLineInput = ""
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		lineLen := len([]rune(m.logLines[m.logCursor]))
		newCol := nextWORDStart(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol >= lineLen && m.logCursor < len(m.logLines)-1 {
			m.logCursor++
			newCol = nextWORDStart(m.logLines[m.logCursor], 0)
			nextLineLen := len([]rune(m.logLines[m.logCursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logVisualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logVisualCurCol = newCol
		}
	}
	return m
}

func (m Model) handleLogKeyE2() Model {
	m.logLineInput = ""
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		lineLen := len([]rune(m.logLines[m.logCursor]))
		newCol := WORDEnd(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol >= lineLen && m.logCursor < len(m.logLines)-1 {
			m.logCursor++
			newCol = WORDEnd(m.logLines[m.logCursor], 0)
			nextLineLen := len([]rune(m.logLines[m.logCursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logVisualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logVisualCurCol = newCol
		}
	}
	return m
}

func (m Model) handleLogKeyB2() Model {
	m.logLineInput = ""
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		newCol := prevWORDStart(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol < 0 && m.logCursor > 0 {
			m.logCursor--
			lineLen := len([]rune(m.logLines[m.logCursor]))
			newCol = max(prevWORDStart(m.logLines[m.logCursor], lineLen), 0)
			m.logVisualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logVisualCurCol = max(newCol, 0)
		}
	}
	return m
}

func (m Model) handleLogKeyCaret() Model {
	m.logLineInput = ""
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		m.logVisualCurCol = firstNonWhitespace(m.logLines[m.logCursor])
	}
	return m
}

func (m Model) handleLogKeySlash() Model {
	m.logLineInput = ""
	m.logSearchActive = true
	m.logSearchInput.Clear()
	return m
}

func (m Model) handleLogKeyN() Model {
	m.logLineInput = ""
	m.findNextLogMatch(true)
	return m
}

func (m Model) handleLogKeyN2() Model {
	m.logLineInput = ""
	m.findNextLogMatch(false)
	return m
}

func (m Model) handleLogKeyP() Model {
	m.logLineInput = ""
	m.logHidePrefixes = !m.logHidePrefixes
	return m
}

func (m Model) handleLogKeyP2() Model {
	m.logLineInput = ""
	m.logPreviewVisible = !m.logPreviewVisible
	m.logPreviewScroll = 0
	// Effective viewer width changes when the panel toggles, so wrap-aware
	// scroll/skip values need recomputing for the new geometry.
	m.ensureLogCursorVisible()
	return m
}

// handleLogKeyJ2 scrolls the structured preview pane down by one body row.
// Caller is responsible for checking m.logPreviewVisible — this only runs
// when the panel is on, so it is safe to assume a valid preview width.
func (m Model) handleLogKeyJ2() Model {
	m.logLineInput = ""
	_, previewW := splitLogPreviewWidth(m.width)
	if previewW == 0 {
		return m
	}
	// LogPreviewMaxScroll's `height` arg is the outer panel height — it
	// subtracts 2 internally for the border to reach the inner content
	// height. logContentHeight already gives that inner height (it
	// accounts for the View()-time app title / tab bar reductions that
	// m.logViewHeight() can't see from Update context), so add 2 to map
	// back. Using logViewHeight here would over-count by 1 (or 2 with
	// tabs) and clip the last body rows off the user's viewport.
	previewH := m.logContentHeight() + 2
	maxScroll := ui.LogPreviewMaxScroll(m.logPreviewLine(), previewW, previewH)
	if m.logPreviewScroll < maxScroll {
		m.logPreviewScroll++
	}
	return m
}

// handleLogKeyK2 scrolls the structured preview pane up by one body row.
func (m Model) handleLogKeyK2() Model {
	m.logLineInput = ""
	if m.logPreviewScroll > 0 {
		m.logPreviewScroll--
	}
	return m
}

func (m Model) handleLogKeyHash() Model {
	m.logLineInput = ""
	m.logLineNumbers = !m.logLineNumbers
	return m
}

func (m Model) handleLogKeyS() Model {
	m.logLineInput = ""
	m.logTimestamps = !m.logTimestamps
	return m
}

func (m Model) handleLogKeyS2() (tea.Model, tea.Cmd) {
	m.logLineInput = ""
	path, err := m.saveLoadedLogs()
	if err != nil {
		m.setErrorFromErr("Log save failed: ", err)
		return m, scheduleStatusClear()
	}
	logger.Info("Saved loaded logs", "path", path)
	m.setStatusMessage("Loaded logs saved to "+path+" (copied to clipboard)", false)
	return m, tea.Batch(copyToSystemClipboard(path), scheduleStatusClear())
}

func (m Model) handleLogKeyCtrlS() (tea.Model, tea.Cmd) {
	m.logLineInput = ""
	m.setStatusMessage("Saving all logs...", false)
	return m, m.saveAllLogs()
}

func (m Model) handleLogKeyC() (tea.Model, tea.Cmd) {
	m.logLineInput = ""
	m.logPrevious = !m.logPrevious
	// --previous is incompatible with -f (follow).
	if m.logPrevious {
		m.logFollow = false
	}
	// Restart the log stream.
	if m.logCancel != nil {
		m.logCancel()
	}
	if m.logHistoryCancel != nil {
		m.logHistoryCancel()
		m.logHistoryCancel = nil
	}
	m.logLines = nil
	m.logScroll = 0
	m.logCursor = 0
	m.logVisualMode = false
	m.logTailLines = ui.ConfigLogTailLines
	m.logHasMoreHistory = !m.logPrevious && !m.logIsMulti
	m.logLoadingHistory = false
	if m.logIsMulti && len(m.logMultiItems) > 0 {
		var cmd tea.Cmd
		m, cmd = m.restartMultiLogStream()
		return m, cmd
	}
	return m, m.startLogStream()
}

func (m Model) handleLogKeyZero() Model {
	if m.logLineInput != "" {
		m.logLineInput += "0"
	} else {
		m.logVisualCurCol = 0
	}
	return m
}

func (m Model) handleLogKeyOther() (tea.Model, tea.Cmd) {
	m.logLineInput = ""
	if m.logParentKind != "" {
		// Group resource: show pod selector to switch between pods.
		m.logSavedPodName = m.actionCtx.name
		if m.logCancel != nil {
			m.logCancel()
			m.logCancel = nil
		}
		if m.logHistoryCancel != nil {
			m.logHistoryCancel()
			m.logHistoryCancel = nil
		}
		m.logCh = nil
		m.actionCtx.kind = m.logParentKind
		m.actionCtx.name = m.logParentName
		m.actionCtx.containerName = ""
		m.pendingAction = "Logs"
		m.loading = true
		m.setStatusMessage("Loading pods...", false)
		return m, m.loadPodsForLogAction()
	}
	if m.actionCtx.kind == "Pod" {
		// Single pod: show container selector for filtering.
		m.overlay = overlayLogContainerSelect
		m.overlayCursor = 0
		m.logContainerFilterText = ""
		m.logContainerFilterActive = false
		m.logContainerSelectionModified = false
		ui.ResetOverlayContainerScroll()
		return m, m.loadContainersForLogFilter()
	}
	return m, nil
}

func (m Model) handleLogKeyCtrlC() (tea.Model, tea.Cmd) {
	if m.logCancel != nil {
		m.logCancel()
	}
	if m.logHistoryCancel != nil {
		m.logHistoryCancel()
		m.logHistoryCancel = nil
	}
	return m.closeTabOrQuit()
}

func (m Model) handleLogVisualKeyV() (tea.Model, tea.Cmd) {
	if m.logVisualType == 'V' {
		m.logVisualMode = false
	} else {
		m.logVisualType = 'V'
	}
	return m, nil
}

func (m Model) handleLogVisualKeyV2() (tea.Model, tea.Cmd) {
	if m.logVisualType == 'v' {
		m.logVisualMode = false
	} else {
		m.logVisualType = 'v'
	}
	return m, nil
}

func (m Model) handleLogVisualKeyCtrlV() (tea.Model, tea.Cmd) {
	if m.logVisualType == 'B' {
		m.logVisualMode = false
	} else {
		m.logVisualType = 'B'
	}
	return m, nil
}

func (m Model) handleLogVisualKeyY() (tea.Model, tea.Cmd) {
	clipText, lineCount := m.buildLogYankText()
	m.logVisualMode = false
	m.setStatusMessage(fmt.Sprintf("Copied %d lines", lineCount), false)
	return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
}

// buildLogYankText returns the clipboard text and selection size for the
// active visual selection in the log viewer. Lines are returned in
// display form — timestamps and pod prefixes stripped per the user's
// toggles, mirroring ui.applyLineRewrites — so the clipboard matches
// what the user sees on screen. Char- and block-mode column positions
// are interpreted in display-line space (after stripping), which is
// where the cursor lives.
func (m *Model) buildLogYankText() (string, int) {
	selStart := min(m.logVisualStart, m.logCursor)
	selEnd := max(m.logVisualStart, m.logCursor)
	if selStart < 0 {
		selStart = 0
	}
	if selEnd >= len(m.logLines) {
		selEnd = len(m.logLines) - 1
	}
	if selStart > selEnd {
		return "", 0
	}

	displayed := make([]string, selEnd-selStart+1)
	for i := selStart; i <= selEnd; i++ {
		displayed[i-selStart] = m.logDisplayLine(i)
	}

	var clipText string
	switch m.logVisualType {
	case 'v': // Character mode: partial first/last lines.
		anchorCol := m.logVisualCol
		cursorCol := m.logVisualCurCol
		// Direction: when the user dragged upward, the start-of-selection
		// column is the cursor's, not the anchor's.
		startCol, endCol := anchorCol, cursorCol
		if m.logVisualStart > m.logCursor {
			startCol, endCol = cursorCol, anchorCol
		}
		parts := make([]string, 0, len(displayed))
		for j, line := range displayed {
			runes := []rune(line)
			switch {
			case len(displayed) == 1:
				cs := min(anchorCol, cursorCol)
				ce := max(anchorCol, cursorCol) + 1
				cs, ce = clampSliceBounds(cs, ce, len(runes))
				parts = append(parts, string(runes[cs:ce]))
			case j == 0:
				cs := min(startCol, len(runes))
				parts = append(parts, string(runes[cs:]))
			case j == len(displayed)-1:
				ce := min(endCol+1, len(runes))
				parts = append(parts, string(runes[:ce]))
			default:
				parts = append(parts, line)
			}
		}
		clipText = strings.Join(parts, "\n")
	case 'B': // Block mode: rectangular column range.
		colStart := min(m.logVisualCol, m.logVisualCurCol)
		colEnd := max(m.logVisualCol, m.logVisualCurCol) + 1
		parts := make([]string, 0, len(displayed))
		for _, line := range displayed {
			runes := []rune(line)
			cs, ce := clampSliceBounds(colStart, colEnd, len(runes))
			parts = append(parts, string(runes[cs:ce]))
		}
		clipText = strings.Join(parts, "\n")
	default: // Line mode: whole lines.
		clipText = strings.Join(displayed, "\n")
	}
	return clipText, len(displayed)
}

// clampSliceBounds returns cs/ce clamped to [0, n], preserving cs <= ce.
func clampSliceBounds(cs, ce, n int) (int, int) {
	if cs > n {
		cs = n
	}
	if ce > n {
		ce = n
	}
	return cs, ce
}

func (m Model) handleLogVisualKeyH() (tea.Model, tea.Cmd) {
	if m.logVisualType == 'v' || m.logVisualType == 'B' {
		if m.logVisualCurCol > 0 {
			m.logVisualCurCol--
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyL() (tea.Model, tea.Cmd) {
	if m.logVisualType == 'v' || m.logVisualType == 'B' {
		m.logVisualCurCol++
	}
	return m, nil
}

func (m Model) handleLogVisualKeyJ() (tea.Model, tea.Cmd) {
	if m.logCursor < len(m.logLines)-1 {
		m.logCursor++
	}
	m.ensureLogCursorVisible()
	return m, nil
}

func (m Model) handleLogVisualKeyK() (tea.Model, tea.Cmd) {
	if m.logCursor > 0 {
		m.logCursor--
	}
	m.ensureLogCursorVisible()
	cmd := m.maybeLoadMoreHistory()
	return m, cmd
}

func (m Model) handleLogVisualKeyG() (tea.Model, tea.Cmd) {
	m.logCursor = len(m.logLines) - 1
	m.ensureLogCursorVisible()
	return m, nil
}

func (m Model) handleLogVisualKeyG2() (tea.Model, tea.Cmd) {
	if m.pendingG {
		m.pendingG = false
		m.logCursor = 0
		m.ensureLogCursorVisible()
		return m, nil
	}
	m.pendingG = true
	return m, nil
}

func (m Model) handleLogVisualKeyCtrlD() (tea.Model, tea.Cmd) {
	m.logCursor += m.logContentHeight() / 2
	if m.logCursor >= len(m.logLines) {
		m.logCursor = len(m.logLines) - 1
	}
	m.ensureLogCursorVisible()
	return m, nil
}

func (m Model) handleLogVisualKeyCtrlU() (tea.Model, tea.Cmd) {
	m.logCursor -= m.logContentHeight() / 2
	if m.logCursor < 0 {
		m.logCursor = 0
	}
	m.ensureLogCursorVisible()
	return m, nil
}

func (m Model) handleLogVisualKeyDollar() (tea.Model, tea.Cmd) {
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		lineLen := len([]rune(m.logLines[m.logCursor]))
		if lineLen > 0 {
			m.logVisualCurCol = lineLen - 1
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyE() (tea.Model, tea.Cmd) {
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		lineLen := len([]rune(m.logLines[m.logCursor]))
		newCol := wordEnd(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol >= lineLen && m.logCursor < len(m.logLines)-1 {
			m.logCursor++
			newCol = wordEnd(m.logLines[m.logCursor], 0)
			nextLineLen := len([]rune(m.logLines[m.logCursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logVisualCurCol = newCol
			m.ensureLogCursorVisible()
		} else {
			m.logVisualCurCol = newCol
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyB() (tea.Model, tea.Cmd) {
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		newCol := prevWordStart(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol < 0 && m.logCursor > 0 {
			m.logCursor--
			lineLen := len([]rune(m.logLines[m.logCursor]))
			newCol = max(prevWordStart(m.logLines[m.logCursor], lineLen), 0)
			m.logVisualCurCol = newCol
			m.ensureLogCursorVisible()
		} else {
			m.logVisualCurCol = max(newCol, 0)
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyW() (tea.Model, tea.Cmd) {
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		lineLen := len([]rune(m.logLines[m.logCursor]))
		newCol := nextWordStart(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol >= lineLen && m.logCursor < len(m.logLines)-1 {
			m.logCursor++
			newCol = nextWordStart(m.logLines[m.logCursor], 0)
			nextLineLen := len([]rune(m.logLines[m.logCursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logVisualCurCol = newCol
			m.ensureLogCursorVisible()
		} else {
			m.logVisualCurCol = newCol
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyW2() (tea.Model, tea.Cmd) {
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		lineLen := len([]rune(m.logLines[m.logCursor]))
		newCol := nextWORDStart(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol >= lineLen && m.logCursor < len(m.logLines)-1 {
			m.logCursor++
			newCol = nextWORDStart(m.logLines[m.logCursor], 0)
			nextLineLen := len([]rune(m.logLines[m.logCursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logVisualCurCol = newCol
			m.ensureLogCursorVisible()
		} else {
			m.logVisualCurCol = newCol
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyE2() (tea.Model, tea.Cmd) {
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		lineLen := len([]rune(m.logLines[m.logCursor]))
		newCol := WORDEnd(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol >= lineLen && m.logCursor < len(m.logLines)-1 {
			m.logCursor++
			newCol = WORDEnd(m.logLines[m.logCursor], 0)
			nextLineLen := len([]rune(m.logLines[m.logCursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logVisualCurCol = newCol
			m.ensureLogCursorVisible()
		} else {
			m.logVisualCurCol = newCol
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyB2() (tea.Model, tea.Cmd) {
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		newCol := prevWORDStart(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol < 0 && m.logCursor > 0 {
			m.logCursor--
			lineLen := len([]rune(m.logLines[m.logCursor]))
			newCol = max(prevWORDStart(m.logLines[m.logCursor], lineLen), 0)
			m.logVisualCurCol = newCol
			m.ensureLogCursorVisible()
		} else {
			m.logVisualCurCol = max(newCol, 0)
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyCaret() (tea.Model, tea.Cmd) {
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		m.logVisualCurCol = firstNonWhitespace(m.logLines[m.logCursor])
	}
	return m, nil
}
