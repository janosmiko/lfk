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

	switch msg.String() {
	case "?":
		m.helpPreviousMode = modeLogs
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "Log Viewer"
		return m, nil
	case "q", "esc":
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
		return m, nil
	case "j", "down":
		m.logFollow = false
		m.logLineInput = ""
		if m.logCursor < len(m.logLines)-1 {
			m.logCursor++
		}
		m.ensureLogCursorVisible()
		return m, nil
	case "k", "up":
		m.logFollow = false
		m.logLineInput = ""
		if m.logCursor > 0 {
			m.logCursor--
		}
		m.ensureLogCursorVisible()
		cmd := m.maybeLoadMoreHistory()
		return m, cmd
	case "ctrl+d":
		m.logFollow = false
		m.logLineInput = ""
		m.logCursor += m.logContentHeight() / 2
		if m.logCursor >= len(m.logLines) {
			m.logCursor = len(m.logLines) - 1
		}
		m.ensureLogCursorVisible()
		return m, nil
	case "ctrl+u":
		m.logFollow = false
		m.logLineInput = ""
		m.logCursor -= m.logContentHeight() / 2
		if m.logCursor < 0 {
			m.logCursor = 0
		}
		m.ensureLogCursorVisible()
		cmd := m.maybeLoadMoreHistory()
		return m, cmd
	case "ctrl+f":
		m.logFollow = false
		m.logLineInput = ""
		m.logCursor += m.logContentHeight()
		if m.logCursor >= len(m.logLines) {
			m.logCursor = len(m.logLines) - 1
		}
		m.ensureLogCursorVisible()
		return m, nil
	case "ctrl+b":
		m.logFollow = false
		m.logLineInput = ""
		m.logCursor -= m.logContentHeight()
		if m.logCursor < 0 {
			m.logCursor = 0
		}
		m.ensureLogCursorVisible()
		cmd := m.maybeLoadMoreHistory()
		return m, cmd
	case "G":
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
		return m, nil
	case "g":
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
	case "h", "left":
		// Move cursor column left.
		m.logLineInput = ""
		if m.logVisualCurCol > 0 {
			m.logVisualCurCol--
		}
		return m, nil
	case "l", "right":
		// Move cursor column right.
		m.logLineInput = ""
		m.logVisualCurCol++
		return m, nil
	case "$":
		// Move cursor to end of current line.
		m.logLineInput = ""
		if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
			lineLen := len([]rune(m.logLines[m.logCursor]))
			if lineLen > 0 {
				m.logVisualCurCol = lineLen - 1
			}
		}
		return m, nil
	case "e":
		// Move cursor to end of current/next word; jump to next line at end of line.
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
		return m, nil
	case "b":
		// Move cursor to previous word start; jump to previous line at start of line.
		m.logLineInput = ""
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
				m.clampLogScroll()
			} else {
				m.logVisualCurCol = max(newCol, 0)
			}
		}
		return m, nil
	case "V":
		m.logLineInput = ""
		if m.logCursor < 0 {
			m.logCursor = m.logScroll
		}
		m.logVisualMode = true
		m.logVisualType = 'V'
		m.logVisualStart = m.logCursor
		m.logVisualCol = m.logVisualCurCol
		return m, nil
	case "v":
		m.logLineInput = ""
		if m.logCursor < 0 {
			m.logCursor = m.logScroll
		}
		m.logVisualMode = true
		m.logVisualType = 'v'
		m.logVisualStart = m.logCursor
		m.logVisualCol = m.logVisualCurCol
		return m, nil
	case "ctrl+v":
		m.logLineInput = ""
		if m.logCursor < 0 {
			m.logCursor = m.logScroll
		}
		m.logVisualMode = true
		m.logVisualType = 'B'
		m.logVisualStart = m.logCursor
		m.logVisualCol = m.logVisualCurCol
		return m, nil
	case "f":
		m.logLineInput = ""
		m.logFollow = !m.logFollow
		if m.logFollow {
			m.logCursor = len(m.logLines) - 1
			m.logScroll = m.logMaxScroll()
		}
		return m, nil
	case "tab", "z":
		m.logLineInput = ""
		m.logWrap = !m.logWrap
		m.clampLogScroll()
		return m, nil
	case "w":
		// Move cursor to next word start; jump to next line at end of line.
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
		return m, nil
	case "W":
		// Move cursor to next WORD start; jump to next line at end of line.
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
		return m, nil
	case "E":
		// Move cursor to end of current/next WORD; jump to next line at end of line.
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
		return m, nil
	case "B":
		// Move cursor to previous WORD start; jump to previous line at start of line.
		m.logLineInput = ""
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
				m.clampLogScroll()
			} else {
				m.logVisualCurCol = max(newCol, 0)
			}
		}
		return m, nil
	case "^":
		// Move cursor to first non-whitespace character.
		m.logLineInput = ""
		if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
			m.logVisualCurCol = firstNonWhitespace(m.logLines[m.logCursor])
		}
		return m, nil
	case "/":
		m.logLineInput = ""
		m.logSearchActive = true
		m.logSearchInput.Clear()
		return m, nil
	case "n":
		m.logLineInput = ""
		m.findNextLogMatch(true)
		return m, nil
	case "N", "p":
		m.logLineInput = ""
		m.findNextLogMatch(false)
		return m, nil
	case "#":
		m.logLineInput = ""
		m.logLineNumbers = !m.logLineNumbers
		return m, nil
	case "s":
		// Toggle timestamp visibility (no stream restart — timestamps are always streamed).
		m.logLineInput = ""
		m.logTimestamps = !m.logTimestamps
		return m, nil
	case "S":
		// Save loaded logs to file.
		m.logLineInput = ""
		path, err := m.saveLoadedLogs()
		if err != nil {
			m.setErrorFromErr("Log save failed: ", err)
			return m, scheduleStatusClear()
		}
		m.setStatusMessage("Loaded logs saved to "+path, false)
		return m, scheduleStatusClear()
	case "ctrl+s":
		// Save all logs (full kubectl logs without --tail) to file.
		m.logLineInput = ""
		m.setStatusMessage("Saving all logs...", false)
		return m, m.saveAllLogs()
	case "c":
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
	case "0":
		// If digits are pending, append 0 to the digit buffer (e.g. 10G, 20G).
		// If no digits pending, move cursor to beginning of line (vim 0 motion).
		if m.logLineInput != "" {
			m.logLineInput += "0"
		} else {
			m.logVisualCurCol = 0
		}
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.logLineInput += msg.String()
		return m, nil
	case "\\":
		// Pod selector for group resources, container selector for single pods.
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
	case "ctrl+c":
		if m.logCancel != nil {
			m.logCancel()
		}
		if m.logHistoryCancel != nil {
			m.logHistoryCancel()
			m.logHistoryCancel = nil
		}
		return m.closeTabOrQuit()
	default:
		m.logLineInput = ""
	}
	return m, nil
}

func (m Model) handleLogVisualKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.logVisualMode = false
		return m, nil
	case "V":
		// Toggle: if already in line mode, cancel; otherwise switch to line mode.
		if m.logVisualType == 'V' {
			m.logVisualMode = false
		} else {
			m.logVisualType = 'V'
		}
		return m, nil
	case "v":
		// Toggle: if already in char mode, cancel; otherwise switch to char mode.
		if m.logVisualType == 'v' {
			m.logVisualMode = false
		} else {
			m.logVisualType = 'v'
		}
		return m, nil
	case "ctrl+v":
		// Toggle: if already in block mode, cancel; otherwise switch to block mode.
		if m.logVisualType == 'B' {
			m.logVisualMode = false
		} else {
			m.logVisualType = 'B'
		}
		return m, nil
	case "y":
		// Copy selected content to clipboard.
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
	case "h", "left":
		// Move cursor column left (for char and block modes).
		if m.logVisualType == 'v' || m.logVisualType == 'B' {
			if m.logVisualCurCol > 0 {
				m.logVisualCurCol--
			}
		}
		return m, nil
	case "l", "right":
		// Move cursor column right (for char and block modes).
		if m.logVisualType == 'v' || m.logVisualType == 'B' {
			m.logVisualCurCol++
		}
		return m, nil
	case "j", "down":
		if m.logCursor < len(m.logLines)-1 {
			m.logCursor++
		}
		m.ensureLogCursorVisible()
		return m, nil
	case "k", "up":
		if m.logCursor > 0 {
			m.logCursor--
		}
		m.ensureLogCursorVisible()
		cmd := m.maybeLoadMoreHistory()
		return m, cmd
	case "G":
		m.logCursor = len(m.logLines) - 1
		m.ensureLogCursorVisible()
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.logCursor = 0
			m.ensureLogCursorVisible()
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "ctrl+d":
		m.logCursor += m.logContentHeight() / 2
		if m.logCursor >= len(m.logLines) {
			m.logCursor = len(m.logLines) - 1
		}
		m.ensureLogCursorVisible()
		return m, nil
	case "ctrl+u":
		m.logCursor -= m.logContentHeight() / 2
		if m.logCursor < 0 {
			m.logCursor = 0
		}
		m.ensureLogCursorVisible()
		return m, nil
	case "ctrl+c":
		m.logVisualMode = false
		return m.closeTabOrQuit()
	case "q":
		m.logVisualMode = false
		return m, nil
	case "$":
		if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
			lineLen := len([]rune(m.logLines[m.logCursor]))
			if lineLen > 0 {
				m.logVisualCurCol = lineLen - 1
			}
		}
		return m, nil
	case "e":
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
	case "b":
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
	case "w":
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
	case "W":
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
	case "E":
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
	case "B":
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
	case "0":
		m.logVisualCurCol = 0
		return m, nil
	case "^":
		if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
			m.logVisualCurCol = firstNonWhitespace(m.logLines[m.logCursor])
		}
		return m, nil
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

func (m *Model) findNextLogMatch(forward bool) {
	if m.logSearchQuery == "" {
		return
	}
	query := strings.ToLower(m.logSearchQuery)
	start := m.logCursor
	if start < 0 {
		start = m.logScroll
	}
	if forward {
		for i := start + 1; i < len(m.logLines); i++ {
			if strings.Contains(strings.ToLower(m.logLines[i]), query) {
				m.logCursor = i
				m.logFollow = false
				m.ensureLogCursorVisible()
				return
			}
		}
		// Wrap around.
		for i := 0; i <= start; i++ {
			if strings.Contains(strings.ToLower(m.logLines[i]), query) {
				m.logCursor = i
				m.logFollow = false
				m.ensureLogCursorVisible()
				return
			}
		}
	} else {
		for i := start - 1; i >= 0; i-- {
			if strings.Contains(strings.ToLower(m.logLines[i]), query) {
				m.logCursor = i
				m.logFollow = false
				m.ensureLogCursorVisible()
				return
			}
		}
		// Wrap around.
		for i := len(m.logLines) - 1; i >= start; i-- {
			if strings.Contains(strings.ToLower(m.logLines[i]), query) {
				m.logCursor = i
				m.logFollow = false
				m.ensureLogCursorVisible()
				return
			}
		}
	}
}
