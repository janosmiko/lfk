package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

	// Handle pending bracket (]e/[e/]w/[w jump-to-severity) before movement
	// keys so 'e' and 'w' get routed to jumpToSeverity, not word-movement.
	if m.logPendingBracket != 0 {
		return m.handleLogPendingBracketKey(msg), nil
	}

	// Prime pending bracket on '[' or ']'.
	if ret, ok := m.handleLogBracketPrimeKey(msg); ok {
		return ret, nil
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

// handleLogBracketPrimeKey arms the pending bracket state on '[' or ']'.
// Returns true when consumed.
func (m Model) handleLogBracketPrimeKey(msg tea.KeyMsg) (Model, bool) {
	switch msg.String() {
	case "]":
		m.logPendingBracket = ']'
		return m, true
	case "[":
		m.logPendingBracket = '['
		return m, true
	}
	return m, false
}

// handleLogPendingBracketKey resolves the pending bracket state. If the next
// key is 'e' or 'w', it jumps to the next/prev visible line of the matching
// severity; any other key clears the pending state and is consumed so the
// caller does not double-process it.
func (m Model) handleLogPendingBracketKey(msg tea.KeyMsg) Model {
	dir := +1
	if m.logPendingBracket == '[' {
		dir = -1
	}
	switch msg.String() {
	case "e":
		m.logPendingBracket = 0
		return m.jumpToSeverity(dir, SeverityError)
	case "w":
		m.logPendingBracket = 0
		return m.jumpToSeverity(dir, SeverityWarn)
	}
	// Any other key cancels the pending bracket; swallow it so that
	// neither the bracket nor the follow-up key triggers unrelated actions.
	m.logPendingBracket = 0
	return m
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
	case "ctrl+f":
		ret := m.handleLogKeyCtrlF()
		return ret, nil, true
	case "ctrl+b":
		ret, cmd := m.handleLogKeyCtrlB()
		return ret, cmd, true
	case "G":
		ret := m.handleLogKeyG()
		return ret, nil, true
	case "g":
		ret, cmd := m.handleLogKeyG2()
		return ret, cmd, true
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
	case "F":
		ret := m.handleLogKeyShiftF()
		return ret, nil, true
	case "tab", "z":
		ret := m.handleLogKeyTab()
		return ret, nil, true
	case ">":
		ret := m.handleLogKeyCycleSeverityUp()
		return ret, nil, true
	case "<":
		ret := m.handleLogKeyCycleSeverityDown()
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
	case "#":
		ret := m.handleLogKeyHash()
		return ret, nil, true
	case "s":
		ret := m.handleLogKeyS()
		return ret, nil, true
	case "S":
		ret, cmd := m.handleLogKeyS2()
		return ret, cmd, true
	case "R":
		ret, cmd := m.handleLogKeyShiftR()
		return ret, cmd, true
	case "ctrl+s":
		ret, cmd := m.handleLogKeyCtrlS()
		return ret, cmd, true
	case "c":
		ret, cmd := m.handleLogKeyC()
		return ret, cmd, true
	case "t":
		ret := m.handleLogKeyT()
		return ret, nil, true
	case "\\":
		ret, cmd := m.handleLogKeyOther()
		return ret, cmd, true
	case "ctrl+c":
		ret, cmd := m.handleLogKeyCtrlC()
		return ret, cmd, true
	}
	return m, nil, false
}

// handleLogKeyT opens the --since duration prompt.  Only reachable from
// the log view's normal key dispatch — search, visual, pending-bracket,
// and overlay modes all short-circuit before we get here, so the caller
// doesn't need to re-check those states.
func (m Model) handleLogKeyT() Model {
	m.logLineInput = ""
	m.overlay = overlayLogSinceInput
	m.logSinceInput.Clear()
	return m
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
		curBytePos := len(string([]rune(dl)[:m.logVisualCurCol+1]))
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
		curBytePos := len(string([]rune(dl)[:m.logVisualCurCol]))
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
	if m.logCursor < m.logCursorMax() {
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
	if m.logCursor >= m.logVisibleCount() {
		m.logCursor = m.logCursorMax()
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
	if m.logCursor >= m.logVisibleCount() {
		m.logCursor = m.logCursorMax()
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
		m.logCursor = min(lineNum, m.logCursorMax())
		m.logFollow = false
	} else {
		m.logCursor = m.logCursorMax()
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
	line, ok := m.logCursorLine()
	if ok {
		lineLen := len([]rune(line))
		if lineLen > 0 {
			m.logVisualCurCol = lineLen - 1
		}
	}
	return m
}

func (m Model) handleLogKeyE() Model {
	m.logLineInput = ""
	line, ok := m.logCursorLine()
	if !ok {
		return m
	}
	lineLen := len([]rune(line))
	newCol := wordEnd(line, m.logVisualCurCol)
	if newCol >= lineLen && m.logCursor < m.logCursorMax() {
		m.logCursor++
		nextLine, _ := m.logCursorLine()
		newCol = wordEnd(nextLine, 0)
		nextLineLen := len([]rune(nextLine))
		if newCol >= nextLineLen {
			newCol = max(nextLineLen-1, 0)
		}
		m.logVisualCurCol = newCol
		m.clampLogScroll()
	} else {
		m.logVisualCurCol = newCol
	}
	return m
}

func (m Model) handleLogKeyB() Model {
	m.logLineInput = ""
	line, ok := m.logCursorLine()
	if !ok {
		return m
	}
	newCol := prevWordStart(line, m.logVisualCurCol)
	if newCol < 0 && m.logCursor > 0 {
		m.logCursor--
		prevLine, _ := m.logCursorLine()
		lineLen := len([]rune(prevLine))
		newCol = prevWordStart(prevLine, lineLen)
		if newCol < 0 {
			newCol = 0
		}
		m.logVisualCurCol = newCol
		m.clampLogScroll()
	} else {
		m.logVisualCurCol = max(newCol, 0)
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
	m.overlay = overlayLogFilter
	m.logFilterModalOpen = true
	// Open in list (nav) mode so the user can see existing rules and
	// navigate with j/k. Press `a` to add a new rule.
	m.logFilterFocusInput = false
	m.logFilterEditingIdx = -1
	m.logFilterInput.Set("")
	// Position the cursor on the first user-editable row so we never
	// land on the read-only severity slot when opening.
	m.logFilterListCursor = firstEditableRuleIdx(m.logRules)
	m.clampFilterCursor()
	return m
}

func (m Model) handleLogKeyShiftF() Model {
	m.logLineInput = ""
	m.logFollow = !m.logFollow
	if m.logFollow {
		m.logCursor = m.logCursorMax()
		m.logScroll = m.logMaxScroll()
	}
	return m
}

func (m Model) handleLogKeyTab() Model {
	m.logLineInput = ""
	m.logWrap = !m.logWrap
	m.clampLogScroll()
	return m
}

// handleLogKeyCycleSeverityUp cycles the severity floor forward:
//
//	off → DEBUG → INFO → WARN → ERROR → off
//
// Updates the single SeverityRule in m.logRules (adds or removes it), then
// rebuilds the filter chain and visible-indices projection.
func (m Model) handleLogKeyCycleSeverityUp() Model {
	m.logLineInput = ""
	return m.setSeverityFloor(nextSeverityFloor(m.currentSeverityFloor(), +1))
}

// handleLogKeyCycleSeverityDown cycles backward:
//
//	off → ERROR → WARN → INFO → DEBUG → off
func (m Model) handleLogKeyCycleSeverityDown() Model {
	m.logLineInput = ""
	return m.setSeverityFloor(nextSeverityFloor(m.currentSeverityFloor(), -1))
}

// currentSeverityFloor returns the floor of the existing SeverityRule in
// m.logRules, or SeverityUnknown (0) when none is active.
func (m *Model) currentSeverityFloor() Severity {
	for _, r := range m.logRules {
		if sev, ok := r.(SeverityRule); ok {
			return sev.Floor
		}
	}
	return SeverityUnknown
}

// nextSeverityFloor returns the next floor in the cycle when step is +1
// (forward, more restrictive) or -1 (backward, less restrictive).
//
// The cycle is:
//
//	off (Unknown) → DEBUG → INFO → WARN → ERROR → off
//
// Invalid step values are treated as +1.
func nextSeverityFloor(cur Severity, step int) Severity {
	order := []Severity{SeverityUnknown, SeverityDebug, SeverityInfo, SeverityWarn, SeverityError}
	idx := 0
	for i, s := range order {
		if s == cur {
			idx = i
			break
		}
	}
	if step < 0 {
		idx = (idx - 1 + len(order)) % len(order)
	} else {
		idx = (idx + 1) % len(order)
	}
	return order[idx]
}

// setSeverityFloor replaces any existing SeverityRule in m.logRules with
// a new one at floor, or removes the rule when floor is SeverityUnknown.
// Rebuilds the filter chain and visible indices, clamps cursor/scroll,
// and shows a brief status confirming the change.
func (m Model) setSeverityFloor(floor Severity) Model {
	// Build new rules slice (defensive copy).
	newRules := make([]Rule, 0, len(m.logRules)+1)
	for _, r := range m.logRules {
		if _, ok := r.(SeverityRule); ok {
			continue // drop existing severity rule
		}
		newRules = append(newRules, r)
	}
	if floor != SeverityUnknown {
		newRules = append(newRules, SeverityRule{Floor: floor})
	}
	m.logRules = pinSeverityFirst(newRules)
	// If the cursor was on the just-added / just-removed severity slot,
	// push it to the first editable rule so we don't hover on the
	// read-only severity.
	m.clampFilterCursor()
	m.logFilterChain = NewFilterChain(m.logRules, m.logIncludeMode, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()

	if floor == SeverityUnknown {
		m.setStatusMessage("Severity filter cleared", false)
	} else {
		m.setStatusMessage("Severity ≥ "+floor.String(), false)
	}
	return m
}

func (m Model) handleLogKeyW() Model {
	m.logLineInput = ""
	line, ok := m.logCursorLine()
	if !ok {
		return m
	}
	lineLen := len([]rune(line))
	newCol := nextWordStart(line, m.logVisualCurCol)
	if newCol >= lineLen && m.logCursor < m.logCursorMax() {
		m.logCursor++
		nextLine, _ := m.logCursorLine()
		newCol = nextWordStart(nextLine, 0)
		nextLineLen := len([]rune(nextLine))
		if newCol >= nextLineLen {
			newCol = max(nextLineLen-1, 0)
		}
		m.logVisualCurCol = newCol
		m.clampLogScroll()
	} else {
		m.logVisualCurCol = newCol
	}
	return m
}

func (m Model) handleLogKeyW2() Model {
	m.logLineInput = ""
	line, ok := m.logCursorLine()
	if !ok {
		return m
	}
	lineLen := len([]rune(line))
	newCol := nextWORDStart(line, m.logVisualCurCol)
	if newCol >= lineLen && m.logCursor < m.logCursorMax() {
		m.logCursor++
		nextLine, _ := m.logCursorLine()
		newCol = nextWORDStart(nextLine, 0)
		nextLineLen := len([]rune(nextLine))
		if newCol >= nextLineLen {
			newCol = max(nextLineLen-1, 0)
		}
		m.logVisualCurCol = newCol
		m.clampLogScroll()
	} else {
		m.logVisualCurCol = newCol
	}
	return m
}

func (m Model) handleLogKeyE2() Model {
	m.logLineInput = ""
	line, ok := m.logCursorLine()
	if !ok {
		return m
	}
	lineLen := len([]rune(line))
	newCol := WORDEnd(line, m.logVisualCurCol)
	if newCol >= lineLen && m.logCursor < m.logCursorMax() {
		m.logCursor++
		nextLine, _ := m.logCursorLine()
		newCol = WORDEnd(nextLine, 0)
		nextLineLen := len([]rune(nextLine))
		if newCol >= nextLineLen {
			newCol = max(nextLineLen-1, 0)
		}
		m.logVisualCurCol = newCol
		m.clampLogScroll()
	} else {
		m.logVisualCurCol = newCol
	}
	return m
}

func (m Model) handleLogKeyB2() Model {
	m.logLineInput = ""
	line, ok := m.logCursorLine()
	if !ok {
		return m
	}
	newCol := prevWORDStart(line, m.logVisualCurCol)
	if newCol < 0 && m.logCursor > 0 {
		m.logCursor--
		prevLine, _ := m.logCursorLine()
		lineLen := len([]rune(prevLine))
		newCol = prevWORDStart(prevLine, lineLen)
		if newCol < 0 {
			newCol = 0
		}
		m.logVisualCurCol = newCol
		m.clampLogScroll()
	} else {
		m.logVisualCurCol = max(newCol, 0)
	}
	return m
}

func (m Model) handleLogKeyCaret() Model {
	m.logLineInput = ""
	if line, ok := m.logCursorLine(); ok {
		m.logVisualCurCol = firstNonWhitespace(line)
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

// handleLogKeyShiftR toggles the relative-timestamp rendering mode.
// When m.logTimestamps is false the toggle is a no-op: rewriting the
// leading timestamp only makes sense while the absolute form is on, so
// we nudge the user to press `s` first rather than silently flipping a
// hidden flag.
func (m Model) handleLogKeyShiftR() (tea.Model, tea.Cmd) {
	m.logLineInput = ""
	if !m.logTimestamps {
		m.setStatusMessage("enable timestamps first (press s)", false)
		return m, scheduleStatusClear()
	}
	m.logRelativeTimestamps = !m.logRelativeTimestamps
	if m.logRelativeTimestamps {
		m.setStatusMessage("relative timestamps: on", false)
	} else {
		m.setStatusMessage("relative timestamps: off", false)
	}
	return m, scheduleStatusClear()
}

func (m Model) handleLogKeyS2() (tea.Model, tea.Cmd) {
	m.logLineInput = ""
	path, err := m.saveLoadedLogs()
	if err != nil {
		m.setErrorFromErr("Log save failed: ", err)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage("Loaded logs saved to "+path, false)
	return m, scheduleStatusClear()
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
	m.logVisibleIndices = nil
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
	selStart := min(m.logVisualStart, m.logCursor)
	selEnd := max(m.logVisualStart, m.logCursor)
	if selStart < 0 {
		selStart = 0
	}
	if selEnd >= len(m.logLines) {
		selEnd = len(m.logLines) - 1
	}
	var clipText string
	switch m.logVisualType {
	case 'v': // Character mode: partial first/last lines.
		var parts []string
		anchorCol := m.logVisualCol
		cursorCol := m.logVisualCurCol
		// Determine direction: assign columns to selStart/selEnd lines.
		startCol, endCol := anchorCol, cursorCol
		if m.logVisualStart > m.logCursor {
			// Upward selection: cursor is at selStart, anchor at selEnd.
			startCol, endCol = cursorCol, anchorCol
		}
		for i := selStart; i <= selEnd; i++ {
			line := m.logLines[i]
			runes := []rune(line)
			if selStart == selEnd {
				// Single line: extract column range.
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
				parts = append(parts, line)
			}
		}
		clipText = strings.Join(parts, "\n")
	case 'B': // Block mode: rectangular column range.
		colStart := min(m.logVisualCol, m.logVisualCurCol)
		colEnd := max(m.logVisualCol, m.logVisualCurCol) + 1
		var parts []string
		for i := selStart; i <= selEnd; i++ {
			line := m.logLines[i]
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
		clipText = strings.Join(parts, "\n")
	default: // Line mode: whole lines.
		var selected []string
		for i := selStart; i <= selEnd; i++ {
			selected = append(selected, m.logLines[i])
		}
		clipText = strings.Join(selected, "\n")
	}
	lineCount := selEnd - selStart + 1
	m.logVisualMode = false
	m.setStatusMessage(fmt.Sprintf("Copied %d lines", lineCount), false)
	return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
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
			newCol = prevWordStart(m.logLines[m.logCursor], lineLen)
			if newCol < 0 {
				newCol = 0
			}
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
			newCol = prevWORDStart(m.logLines[m.logCursor], lineLen)
			if newCol < 0 {
				newCol = 0
			}
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

// rebuildLogVisibleIndices recomputes m.logVisibleIndices from scratch
// based on the current logFilterChain and logLines. Called after any
// rule add/edit/delete/reorder/mode-toggle.
//
// When the chain is inactive (no rules), logVisibleIndices is cleared to
// nil so the renderer can short-circuit the projection path and render
// m.logLines directly. This also keeps the contract that
// "len(logVisibleIndices) == 0 && chain inactive" means "show everything,"
// which the streaming append path below relies on.
func (m *Model) rebuildLogVisibleIndices() {
	if m.logFilterChain == nil || !m.logFilterChain.Active() {
		m.logVisibleIndices = nil
	} else {
		m.logVisibleIndices = BuildVisibleIndices(m.logLines, m.logFilterChain)
	}
	m.clampLogCursorAndScroll()
}

// clampLogCursorAndScroll keeps logCursor and logScroll within the visible
// range. Called after any operation that may shrink the visible set (rule
// add/delete/reorder/mode toggle, preset apply) so the viewport doesn't
// end up past the last visible line — which would render a blank view.
func (m *Model) clampLogCursorAndScroll() {
	max := m.logCursorMax()
	if m.logCursor > max {
		m.logCursor = max
	}
	if m.logCursor < 0 {
		m.logCursor = 0
	}
	if m.logScroll > max {
		m.logScroll = max
	}
	if m.logScroll < 0 {
		m.logScroll = 0
	}
}

// maybeAppendVisibleIndex evaluates the line at index idx against the filter
// chain and appends idx to logVisibleIndices if it passes.
//
// When the chain is nil or inactive, logVisibleIndices stays nil so the
// renderer short-circuits to the raw buffer. Attempting to grow it here
// would produce a partial mirror of m.logLines that the renderer then
// uses for projection — causing the "X of Y" title and slice projection
// to compete with the unfiltered source.
func (m *Model) maybeAppendVisibleIndex(idx int) {
	if idx < 0 || idx >= len(m.logLines) {
		return
	}
	if m.logFilterChain == nil || !m.logFilterChain.Active() {
		return
	}
	if m.logFilterChain.Keep(m.logLines[idx]) {
		m.logVisibleIndices = append(m.logVisibleIndices, idx)
	}
}

// logCursorMax returns the max valid cursor position (inclusive).
// When filter is active, this is len(visibleIndices)-1; otherwise len(logLines)-1.
func (m *Model) logCursorMax() int {
	n := m.logVisibleCount()
	if n == 0 {
		return 0
	}
	return n - 1
}

// logVisibleCount returns the count of visible lines.
func (m *Model) logVisibleCount() int {
	if m.logFilterChain != nil && m.logFilterChain.Active() {
		return len(m.logVisibleIndices)
	}
	return len(m.logLines)
}

// logSourceLineAt returns the source-buffer index for a given visible cursor position.
func (m *Model) logSourceLineAt(visiblePos int) int {
	if m.logFilterChain != nil && m.logFilterChain.Active() {
		if visiblePos >= 0 && visiblePos < len(m.logVisibleIndices) {
			return m.logVisibleIndices[visiblePos]
		}
		return -1
	}
	return visiblePos
}

// logCursorLine returns the source log line that the cursor currently
// points at, honouring filter projection. Returns ("", false) when the
// cursor position is out of range for the current visible set.
//
// Always use this (or logSourceLineAt) instead of `m.logLines[m.logCursor]`
// in cursor-movement handlers: when the filter is active, m.logCursor is
// a *visible* position, not a source index, and direct indexing reads the
// wrong line.
func (m *Model) logCursorLine() (string, bool) {
	src := m.logSourceLineAt(m.logCursor)
	if src < 0 || src >= len(m.logLines) {
		return "", false
	}
	return m.logLines[src], true
}

// jumpToSeverity moves the cursor to the next (dir>0) or previous (dir<0)
// visible line whose detected severity matches sev. Wraps around if no
// match found in the requested direction.
func (m Model) jumpToSeverity(dir int, sev Severity) Model {
	n := m.logVisibleCount()
	if n == 0 {
		return m
	}
	start := m.logCursor
	for offset := 1; offset <= n; offset++ {
		probe := (start + dir*offset + n) % n
		srcIdx := m.logSourceLineAt(probe)
		if srcIdx < 0 || srcIdx >= len(m.logLines) {
			continue
		}
		if m.logSeverityDetector.Detect(m.logLines[srcIdx]) == sev {
			m.logCursor = probe
			m.ensureLogCursorVisible()
			return m
		}
	}
	return m
}
