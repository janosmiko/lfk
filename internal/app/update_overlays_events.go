package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

// buildEventTimelineLines converts event timeline data into flat text lines
// for cursor navigation. Each event becomes a single line with format:
// {age}  {type}  {reason}  {message}
func (m *Model) buildEventTimelineLines() []string {
	lines := make([]string, 0, len(m.eventTimelineData))
	for _, e := range m.eventTimelineData {
		ts := ui.RelativeTime(e.Timestamp)
		countStr := ""
		if e.Count > 1 {
			countStr = fmt.Sprintf(" (x%d)", e.Count)
		}
		src := ""
		if e.Source != "" {
			src = " [" + e.Source + "]"
		}
		line := fmt.Sprintf("%-8s %-7s %-20s %s%s%s",
			ts, e.Type, e.Reason, e.Message, countStr, src)
		lines = append(lines, line)
	}
	return lines
}

// eventContentHeight returns the visible content height for the event timeline overlay.
// Must match the maxVisible calculation in RenderEventViewer: Height - 4.
func (m *Model) eventContentHeight() int {
	var h int
	if m.mode == modeEventViewer {
		// Fullscreen mode: same calc as viewEventViewer (m.height - 4).
		h = m.height - 4
	} else {
		// Overlay mode: RenderEventViewer uses Height - 4 for maxVisible.
		overlayH := min(30, m.height-4)
		h = overlayH - 4
	}
	if h < 1 {
		h = 1
	}
	return h
}

// ensureEventCursorVisible scrolls the event timeline to keep the cursor visible
// with scrolloff padding, following the same pattern as the log viewer.
func (m *Model) ensureEventCursorVisible() {
	if m.eventTimelineCursor < 0 {
		return
	}
	total := len(m.eventTimelineLines)
	if total > 0 && m.eventTimelineCursor >= total {
		m.eventTimelineCursor = total - 1
	}
	viewH := m.eventContentHeight()
	if viewH < 1 {
		viewH = 1
	}
	so := ui.ConfigScrollOff
	if so > viewH/2 {
		so = viewH / 2
	}
	if m.eventTimelineCursor < m.eventTimelineScroll+so {
		m.eventTimelineScroll = m.eventTimelineCursor - so
	}
	if m.eventTimelineCursor >= m.eventTimelineScroll+viewH-so {
		m.eventTimelineScroll = m.eventTimelineCursor - viewH + so + 1
	}
	if m.eventTimelineScroll < 0 {
		m.eventTimelineScroll = 0
	}
	maxScroll := max(total-viewH, 0)
	if m.eventTimelineScroll > maxScroll {
		m.eventTimelineScroll = maxScroll
	}
}

// findNextEventMatch searches for the next/previous occurrence of the search
// query in the event timeline lines and moves the cursor to it.
func (m *Model) findNextEventMatch(forward bool) {
	if m.eventTimelineSearchQuery == "" || len(m.eventTimelineLines) == 0 {
		return
	}
	query := strings.ToLower(m.eventTimelineSearchQuery)
	start := m.eventTimelineCursor
	total := len(m.eventTimelineLines)

	for i := 1; i <= total; i++ {
		var idx int
		if forward {
			idx = (start + i) % total
		} else {
			idx = (start - i + total) % total
		}
		if strings.Contains(strings.ToLower(m.eventTimelineLines[idx]), query) {
			m.eventTimelineCursor = idx
			m.ensureEventCursorVisible()
			return
		}
	}
	m.setStatusMessage("Pattern not found: "+m.eventTimelineSearchQuery, false)
}

// handleEventViewerModeKey handles keys for the fullscreen event viewer mode.
// It wraps the overlay key handler but overrides q/esc/f for mode transitions.
func (m Model) handleEventViewerModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "q", "esc":
		if m.eventTimelineSearchActive {
			// Let the search handler deal with esc.
			return m.handleEventTimelineSearchKey(msg)
		}
		if m.eventTimelineVisualMode != 0 {
			m.eventTimelineVisualMode = 0
			return m, nil
		}
		if m.eventTimelineSearchQuery != "" && key == "esc" {
			m.eventTimelineSearchQuery = ""
			return m, nil
		}
		// Exit fullscreen mode back to explorer.
		m.mode = modeExplorer
		m.eventTimelineFullscreen = false
		return m, nil
	case "f":
		// Minimize: go back to overlay mode.
		m.mode = modeExplorer
		m.overlay = overlayEventTimeline
		m.eventTimelineFullscreen = false
		m.ensureEventCursorVisible()
		return m, nil
	}
	// Delegate all other keys to the overlay handler.
	return m.handleEventTimelineOverlayKey(msg)
}

// handleEventTimelineOverlayKey handles keyboard input for the event timeline overlay.
func (m Model) handleEventTimelineOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search input mode first.
	if m.eventTimelineSearchActive {
		return m.handleEventTimelineSearchKey(msg)
	}

	// Handle visual mode keys.
	if m.eventTimelineVisualMode != 0 {
		return m.handleEventTimelineVisualKey(msg)
	}

	// Try movement keys.
	if ret, ok := m.handleEventTimelineMovementKey(msg); ok {
		return ret, nil
	}
	// Try action keys.
	key := msg.String()
	switch key {
	case "esc":
		return m.handleEventTimelineOverlayKeyEsc()
	case "q":
		return m.handleEventTimelineOverlayKeyQ()
	case "v":
		return m.handleEventTimelineOverlayKeyV()
	case "V":
		return m.handleEventTimelineOverlayKeyV2()
	case "ctrl+v":
		return m.handleEventTimelineOverlayKeyCtrlV()
	case "y":
		return m.handleEventTimelineOverlayKeyY()
	case "/":
		return m.handleEventTimelineOverlayKeySlash()
	case "n":
		m.eventTimelineLineInput = ""
		m.findNextEventMatch(true)
	case "N":
		m.eventTimelineLineInput = ""
		m.findNextEventMatch(false)
	case "f":
		return m.handleEventTimelineOverlayKeyF()
	case "tab", "z", ">":
		m.eventTimelineLineInput = ""
		m.eventTimelineWrap = !m.eventTimelineWrap
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		m.eventTimelineLineInput = ""
	}
	return m, nil
}

// handleEventTimelineMovementKey handles cursor/scroll movement keys.
func (m Model) handleEventTimelineMovementKey(msg tea.KeyMsg) (Model, bool) {
	key := msg.String()
	maxIdx := max(len(m.eventTimelineLines)-1, 0)
	switch key {
	case "j", "down":
		m.eventTimelineLineInput = ""
		if m.eventTimelineCursor < maxIdx {
			m.eventTimelineCursor++
		}
		m.ensureEventCursorVisible()
		return m, true
	case "k", "up":
		return m.handleEventTimelineOverlayKeyK(), true
	case "h", "left":
		return m.handleEventTimelineOverlayKeyH(), true
	case "l", "right":
		m.eventTimelineLineInput = ""
		m.eventTimelineCursorCol++
		return m, true
	case "0":
		return m.handleEventTimelineOverlayKeyZero(), true
	case "$":
		return m.handleEventTimelineOverlayKeyDollar(), true
	case "^":
		return m.handleEventTimelineOverlayKeyCaret(), true
	case "w":
		return m.handleEventTimelineOverlayKeyW(), true
	case "W":
		return m.handleEventTimelineOverlayKeyW2(), true
	case "b":
		return m.handleEventTimelineOverlayKeyB(), true
	case "B":
		return m.handleEventTimelineOverlayKeyB2(), true
	case "e":
		return m.handleEventTimelineOverlayKeyE(), true
	case "E":
		return m.handleEventTimelineOverlayKeyE2(), true
	case "ctrl+d":
		m.eventTimelineLineInput = ""
		m.eventTimelineCursor = min(m.eventTimelineCursor+m.eventContentHeight()/2, maxIdx)
		m.ensureEventCursorVisible()
		return m, true
	case "ctrl+u":
		return m.handleEventTimelineOverlayKeyCtrlU(), true
	case "ctrl+f", "pgdown":
		m.eventTimelineLineInput = ""
		m.eventTimelineCursor = min(m.eventTimelineCursor+m.eventContentHeight(), maxIdx)
		m.ensureEventCursorVisible()
		return m, true
	case "ctrl+b", "pgup":
		return m.handleEventTimelineOverlayKeyCtrlB(), true
	case "home":
		m.eventTimelineLineInput = ""
		m.pendingG = false
		m.eventTimelineCursor = 0
		m.ensureEventCursorVisible()
		return m, true
	case "end":
		m.eventTimelineLineInput = ""
		m.eventTimelineCursor = maxIdx
		m.ensureEventCursorVisible()
		return m, true
	case "g":
		return m.handleEventTimelineOverlayKeyG(), true
	case "G":
		if m.eventTimelineLineInput != "" {
			lineNum, _ := strconv.Atoi(m.eventTimelineLineInput)
			m.eventTimelineLineInput = ""
			if lineNum > 0 {
				lineNum--
			}
			m.eventTimelineCursor = min(lineNum, maxIdx)
		} else {
			m.eventTimelineCursor = maxIdx
		}
		m.ensureEventCursorVisible()
		return m, true
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.eventTimelineLineInput += key
		return m, true
	}
	return m, false
}

// handleEventTimelineVisualKey handles keys while visual mode is active
// in the event timeline overlay.
func (m Model) handleEventTimelineVisualKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	maxIdx := max(len(m.eventTimelineLines)-1, 0)

	// Mode switches.
	switch key {
	case "esc":
		m.eventTimelineVisualMode = 0
		return m, nil
	case "V":
		return m.handleEventTimelineVisualKeyV()
	case "v":
		return m.handleEventTimelineVisualKeyV2()
	case "ctrl+v":
		return m.handleEventTimelineVisualKeyCtrlV()
	case "y":
		return m.handleEventTimelineVisualKeyY()
	case "ctrl+c":
		return m.closeTabOrQuit()
	}

	// Movement keys that extend selection.
	m.handleEventTimelineVisualMovement(key, maxIdx)
	return m, nil
}

// handleEventTimelineVisualMovement handles cursor movement in visual mode.
func (m *Model) handleEventTimelineVisualMovement(key string, maxIdx int) {
	switch key {
	case "j", "down":
		m.eventTimelineCursor = min(m.eventTimelineCursor+1, maxIdx)
		m.ensureEventCursorVisible()
	case "k", "up":
		m.eventTimelineCursor = max(m.eventTimelineCursor-1, 0)
		m.ensureEventCursorVisible()
	case "h", "left":
		m.eventTimelineCursorCol = max(m.eventTimelineCursorCol-1, 0)
	case "l", "right":
		m.eventTimelineCursorCol++
	case "0":
		m.eventTimelineCursorCol = 0
	case "G":
		m.eventTimelineCursor = maxIdx
		m.ensureEventCursorVisible()
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.eventTimelineCursor = 0
			m.ensureEventCursorVisible()
		} else {
			m.pendingG = true
		}
	case "ctrl+d":
		m.eventTimelineCursor = min(m.eventTimelineCursor+m.eventContentHeight()/2, maxIdx)
		m.ensureEventCursorVisible()
	case "ctrl+u":
		m.eventTimelineCursor = max(m.eventTimelineCursor-m.eventContentHeight()/2, 0)
		m.ensureEventCursorVisible()
	default:
		m.handleEventTimelineVisualWordMotion(key)
	}
}

// handleEventTimelineVisualWordMotion handles word/char motions in visual mode.
func (m *Model) handleEventTimelineVisualWordMotion(key string) {
	line := m.eventTimelineCurrentLine()
	if line == "" {
		return
	}
	switch key {
	case "$":
		if lineLen := len([]rune(line)); lineLen > 0 {
			m.eventTimelineCursorCol = lineLen - 1
		}
	case "^":
		m.eventTimelineCursorCol = firstNonWhitespace(line)
	case "w":
		m.eventTimelineCursorCol = nextWordStart(line, m.eventTimelineCursorCol)
	case "W":
		m.eventTimelineCursorCol = nextWORDStart(line, m.eventTimelineCursorCol)
	case "b":
		if nc := prevWordStart(line, m.eventTimelineCursorCol); nc >= 0 {
			m.eventTimelineCursorCol = nc
		}
	case "B":
		if nc := prevWORDStart(line, m.eventTimelineCursorCol); nc >= 0 {
			m.eventTimelineCursorCol = nc
		}
	case "e":
		m.eventTimelineCursorCol = wordEnd(line, m.eventTimelineCursorCol)
	case "E":
		m.eventTimelineCursorCol = WORDEnd(line, m.eventTimelineCursorCol)
	}
}

// eventTimelineCurrentLine returns the current line under the cursor, or empty.
func (m *Model) eventTimelineCurrentLine() string {
	if m.eventTimelineCursor >= 0 && m.eventTimelineCursor < len(m.eventTimelineLines) {
		return m.eventTimelineLines[m.eventTimelineCursor]
	}
	return ""
}

// handleEventTimelineSearchKey handles keyboard input during event timeline search.
func (m Model) handleEventTimelineSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.eventTimelineSearchActive = false
		m.eventTimelineSearchQuery = m.eventTimelineSearchInput.Value
		m.findNextEventMatch(true)
	case "esc":
		m.eventTimelineSearchActive = false
		m.eventTimelineSearchInput.Clear()
	case "backspace":
		if len(m.eventTimelineSearchInput.Value) > 0 {
			m.eventTimelineSearchInput.Backspace()
		}
	case "ctrl+w":
		m.eventTimelineSearchInput.DeleteWord()
	case "ctrl+a":
		m.eventTimelineSearchInput.Home()
	case "ctrl+e":
		m.eventTimelineSearchInput.End()
	case "left":
		m.eventTimelineSearchInput.Left()
	case "right":
		m.eventTimelineSearchInput.Right()
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.eventTimelineSearchInput.Insert(key)
		}
	}
	return m, nil
}

func (m Model) handleEventTimelineVisualKeyV() (tea.Model, tea.Cmd) {
	if m.eventTimelineVisualMode == 'V' {
		m.eventTimelineVisualMode = 0
	} else {
		m.eventTimelineVisualMode = 'V'
	}
	return m, nil
}

func (m Model) handleEventTimelineVisualKeyV2() (tea.Model, tea.Cmd) {
	if m.eventTimelineVisualMode == 'v' {
		m.eventTimelineVisualMode = 0
	} else {
		m.eventTimelineVisualMode = 'v'
	}
	return m, nil
}

func (m Model) handleEventTimelineVisualKeyCtrlV() (tea.Model, tea.Cmd) {
	if m.eventTimelineVisualMode == 'B' {
		m.eventTimelineVisualMode = 0
	} else {
		m.eventTimelineVisualMode = 'B'
	}
	return m, nil
}

func (m Model) handleEventTimelineVisualKeyY() (tea.Model, tea.Cmd) {
	selStart := min(m.eventTimelineVisualStart, m.eventTimelineCursor)
	selEnd := max(m.eventTimelineVisualStart, m.eventTimelineCursor)
	if selStart < 0 {
		selStart = 0
	}
	if selEnd >= len(m.eventTimelineLines) {
		selEnd = len(m.eventTimelineLines) - 1
	}
	var clipText string
	switch m.eventTimelineVisualMode {
	case 'v': // Character mode: partial first/last lines.
		var parts []string
		anchorCol := m.eventTimelineVisualCol
		cursorCol := m.eventTimelineCursorCol
		startCol, endCol := anchorCol, cursorCol
		if m.eventTimelineVisualStart > m.eventTimelineCursor {
			startCol, endCol = cursorCol, anchorCol
		}
		for i := selStart; i <= selEnd; i++ {
			line := m.eventTimelineLines[i]
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
		colStart := min(m.eventTimelineVisualCol, m.eventTimelineCursorCol)
		colEnd := max(m.eventTimelineVisualCol, m.eventTimelineCursorCol) + 1
		var parts []string
		for i := selStart; i <= selEnd; i++ {
			line := m.eventTimelineLines[i]
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
		var parts []string
		for i := selStart; i <= selEnd; i++ {
			parts = append(parts, m.eventTimelineLines[i])
		}
		clipText = strings.Join(parts, "\n")
	}
	lineCount := selEnd - selStart + 1
	m.eventTimelineVisualMode = 0
	m.setStatusMessage(fmt.Sprintf("Copied %d line(s)", lineCount), false)
	return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
}

func (m Model) handleEventTimelineOverlayKeyEsc() (tea.Model, tea.Cmd) {
	if m.eventTimelineSearchQuery != "" {
		m.eventTimelineSearchQuery = ""
		return m, nil
	}
	m.eventTimelineLineInput = ""
	m.eventTimelineFullscreen = false
	m.eventTimelineVisualMode = 0
	m.overlay = overlayNone
	return m, nil
}

func (m Model) handleEventTimelineOverlayKeyQ() (tea.Model, tea.Cmd) {
	m.eventTimelineLineInput = ""
	m.eventTimelineFullscreen = false
	m.eventTimelineVisualMode = 0
	m.overlay = overlayNone
	return m, nil
}

func (m Model) handleEventTimelineOverlayKeyK() Model {
	m.eventTimelineLineInput = ""
	if m.eventTimelineCursor > 0 {
		m.eventTimelineCursor--
	}
	m.ensureEventCursorVisible()
	return m
}

func (m Model) handleEventTimelineOverlayKeyH() Model {
	m.eventTimelineLineInput = ""
	if m.eventTimelineCursorCol > 0 {
		m.eventTimelineCursorCol--
	}
	return m
}

func (m Model) handleEventTimelineOverlayKeyZero() Model {
	if m.eventTimelineLineInput != "" {
		m.eventTimelineLineInput += "0"
		return m
	}
	m.eventTimelineCursorCol = 0
	return m
}

func (m Model) handleEventTimelineOverlayKeyDollar() Model {
	m.eventTimelineLineInput = ""
	if m.eventTimelineCursor >= 0 && m.eventTimelineCursor < len(m.eventTimelineLines) {
		lineLen := len([]rune(m.eventTimelineLines[m.eventTimelineCursor]))
		if lineLen > 0 {
			m.eventTimelineCursorCol = lineLen - 1
		}
	}
	return m
}

func (m Model) handleEventTimelineOverlayKeyCaret() Model {
	m.eventTimelineLineInput = ""
	if m.eventTimelineCursor >= 0 && m.eventTimelineCursor < len(m.eventTimelineLines) {
		m.eventTimelineCursorCol = firstNonWhitespace(m.eventTimelineLines[m.eventTimelineCursor])
	}
	return m
}

func (m Model) handleEventTimelineOverlayKeyW() Model {
	m.eventTimelineLineInput = ""
	if m.eventTimelineCursor >= 0 && m.eventTimelineCursor < len(m.eventTimelineLines) {
		m.eventTimelineCursorCol = nextWordStart(m.eventTimelineLines[m.eventTimelineCursor], m.eventTimelineCursorCol)
	}
	return m
}

func (m Model) handleEventTimelineOverlayKeyW2() Model {
	m.eventTimelineLineInput = ""
	if m.eventTimelineCursor >= 0 && m.eventTimelineCursor < len(m.eventTimelineLines) {
		m.eventTimelineCursorCol = nextWORDStart(m.eventTimelineLines[m.eventTimelineCursor], m.eventTimelineCursorCol)
	}
	return m
}

func (m Model) handleEventTimelineOverlayKeyB() Model {
	m.eventTimelineLineInput = ""
	if m.eventTimelineCursor >= 0 && m.eventTimelineCursor < len(m.eventTimelineLines) {
		nc := prevWordStart(m.eventTimelineLines[m.eventTimelineCursor], m.eventTimelineCursorCol)
		if nc >= 0 {
			m.eventTimelineCursorCol = nc
		}
	}
	return m
}

func (m Model) handleEventTimelineOverlayKeyB2() Model {
	m.eventTimelineLineInput = ""
	if m.eventTimelineCursor >= 0 && m.eventTimelineCursor < len(m.eventTimelineLines) {
		nc := prevWORDStart(m.eventTimelineLines[m.eventTimelineCursor], m.eventTimelineCursorCol)
		if nc >= 0 {
			m.eventTimelineCursorCol = nc
		}
	}
	return m
}

func (m Model) handleEventTimelineOverlayKeyE() Model {
	m.eventTimelineLineInput = ""
	if m.eventTimelineCursor >= 0 && m.eventTimelineCursor < len(m.eventTimelineLines) {
		m.eventTimelineCursorCol = wordEnd(m.eventTimelineLines[m.eventTimelineCursor], m.eventTimelineCursorCol)
	}
	return m
}

func (m Model) handleEventTimelineOverlayKeyE2() Model {
	m.eventTimelineLineInput = ""
	if m.eventTimelineCursor >= 0 && m.eventTimelineCursor < len(m.eventTimelineLines) {
		m.eventTimelineCursorCol = WORDEnd(m.eventTimelineLines[m.eventTimelineCursor], m.eventTimelineCursorCol)
	}
	return m
}

func (m Model) handleEventTimelineOverlayKeyCtrlU() Model {
	m.eventTimelineLineInput = ""
	m.eventTimelineCursor -= m.eventContentHeight() / 2
	if m.eventTimelineCursor < 0 {
		m.eventTimelineCursor = 0
	}
	m.ensureEventCursorVisible()
	return m
}

func (m Model) handleEventTimelineOverlayKeyCtrlB() Model {
	m.eventTimelineLineInput = ""
	m.eventTimelineCursor -= m.eventContentHeight()
	if m.eventTimelineCursor < 0 {
		m.eventTimelineCursor = 0
	}
	m.ensureEventCursorVisible()
	return m
}

func (m Model) handleEventTimelineOverlayKeyG() Model {
	m.eventTimelineLineInput = ""
	if m.pendingG {
		m.pendingG = false
		m.eventTimelineCursor = 0
		m.ensureEventCursorVisible()
	} else {
		m.pendingG = true
	}
	return m
}

func (m Model) handleEventTimelineOverlayKeyV() (tea.Model, tea.Cmd) {
	m.eventTimelineLineInput = ""
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineVisualStart = m.eventTimelineCursor
	m.eventTimelineVisualCol = m.eventTimelineCursorCol
	return m, nil
}

func (m Model) handleEventTimelineOverlayKeyV2() (tea.Model, tea.Cmd) {
	m.eventTimelineLineInput = ""
	m.eventTimelineVisualMode = 'V'
	m.eventTimelineVisualStart = m.eventTimelineCursor
	m.eventTimelineVisualCol = m.eventTimelineCursorCol
	return m, nil
}

func (m Model) handleEventTimelineOverlayKeyCtrlV() (tea.Model, tea.Cmd) {
	m.eventTimelineLineInput = ""
	m.eventTimelineVisualMode = 'B'
	m.eventTimelineVisualStart = m.eventTimelineCursor
	m.eventTimelineVisualCol = m.eventTimelineCursorCol
	return m, nil
}

func (m Model) handleEventTimelineOverlayKeyY() (tea.Model, tea.Cmd) {
	m.eventTimelineLineInput = ""
	if m.eventTimelineCursor >= 0 && m.eventTimelineCursor < len(m.eventTimelineLines) {
		text := m.eventTimelineLines[m.eventTimelineCursor]
		m.setStatusMessage("Copied 1 line", false)
		return m, tea.Batch(copyToSystemClipboard(text), scheduleStatusClear())
	}
	return m, nil
}

func (m Model) handleEventTimelineOverlayKeySlash() (tea.Model, tea.Cmd) {
	m.eventTimelineLineInput = ""
	m.eventTimelineSearchActive = true
	m.eventTimelineSearchInput.Clear()
	return m, nil
}

func (m Model) handleEventTimelineOverlayKeyF() (tea.Model, tea.Cmd) {
	m.eventTimelineLineInput = ""
	m.overlay = overlayNone
	m.mode = modeEventViewer
	m.ensureEventCursorVisible()
	return m, nil
}
