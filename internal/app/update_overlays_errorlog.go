package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

// errorLogVisibleCount returns the number of visible entries and max dimensions for the error log overlay.
func (m Model) errorLogVisibleCount() (visibleCount, maxVisible, maxScroll int) {
	reversed := ui.FilteredErrorLogEntries(m.errorLog, m.showDebugLogs)
	visibleCount = len(reversed)

	var overlayH int
	if m.errorLogFullscreen {
		overlayH = m.height - 1
	} else {
		overlayH = min(30, m.height-4)
	}
	maxVisible = max(overlayH-4, 1)
	maxScroll = max(visibleCount-maxVisible, 0)
	return
}

// handleErrorLogOverlayKey handles keyboard input when the error log overlay is open.
// errorLogForwardGlobalKey forwards a small set of "global" navigation keys
// (new/next/prev tab, theme selector) to the underlying explorer handlers so
// users can keep the error log overlay visible while switching tabs or while
// opening the theme selector on top. The error log overlay state is left
// alone — fullscreen + theme selector should layer the way the dashboard
// fullscreen + theme selector does, with the error log staying behind the
// colorscheme overlay until it closes.
// Returns handled=false for non-matching keys so the regular overlay key
// dispatch can run. Visual mode disables the forwarding so 't' / 'T' inside
// a selection stay local.
func (m Model) errorLogForwardGlobalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if m.errorLogVisualMode != 0 {
		return m, nil, false
	}
	kb := ui.ActiveKeybindings
	switch msg.String() {
	case kb.NewTab, kb.NextTab, kb.PrevTab:
		if mdl, cmd, ok := m.handleExplorerActionKey(msg); ok {
			return mdl, cmd, true
		}
	case kb.ThemeSelector:
		return m.handleKeyThemeSelector(), nil, true
	}
	return m, nil, false
}

func (m Model) handleErrorLogOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visibleCount, maxVisible, maxScroll := m.errorLogVisibleCount()
	maxCursor := max(visibleCount-1, 0)

	key := msg.String()

	// Toggle: pressing the error log hotkey again closes the overlay.
	if key == ui.ActiveKeybindings.ErrorLog {
		return m.handleErrorLogOverlayKeyEsc()
	}

	// Allow tab switching and theme selector to work while the overlay
	// is up — extracted to keep this function under the gocyclo cap.
	if mdl, cmd, handled := m.errorLogForwardGlobalKey(msg); handled {
		return mdl, cmd
	}

	// In visual mode, Esc cancels visual mode instead of closing.
	if key == "esc" && m.errorLogVisualMode != 0 {
		m.errorLogLineInput = ""
		m.errorLogVisualMode = 0
		return m, nil
	}

	switch key {
	case "esc", "q":
		return m.handleErrorLogOverlayKeyEsc()

	case "f":
		return m.handleErrorLogOverlayKeyF()

	case "V":
		return m.handleErrorLogOverlayKeyV()

	case "v":
		return m.handleErrorLogOverlayKeyV2()

	case "h", "left":
		return m.handleErrorLogOverlayKeyH()

	case "l", "right":
		return m.handleErrorLogOverlayKeyL()

	case "0":
		return m.handleErrorLogOverlayKeyZero()

	case "$":
		return m.handleErrorLogOverlayKeyDollar()

	case "y":
		m.errorLogLineInput = ""
		return m.errorLogYank()

	case "d":
		return m.handleErrorLogOverlayKeyD()

	case "j", "down":
		m.errorLogLineInput = ""
		if m.errorLogCursorLine < maxCursor {
			m.errorLogCursorLine++
		}
		m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
		return m, nil

	case "k", "up":
		m.errorLogLineInput = ""
		if m.errorLogCursorLine > 0 {
			m.errorLogCursorLine--
		}
		m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
		return m, nil

	case "g":
		return m.handleErrorLogOverlayKeyG()

	case "G":
		if m.errorLogLineInput != "" {
			lineNum, _ := strconv.Atoi(m.errorLogLineInput)
			m.errorLogLineInput = ""
			if lineNum > 0 {
				lineNum--
			}
			m.errorLogCursorLine = min(lineNum, maxCursor)
			m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
			return m, nil
		}
		m.errorLogCursorLine = maxCursor
		m.errorLogScroll = maxScroll
		return m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.errorLogLineInput += key
		return m, nil

	case "ctrl+d":
		m.errorLogLineInput = ""
		halfPage := maxVisible / 2
		m.errorLogCursorLine = min(m.errorLogCursorLine+halfPage, maxCursor)
		m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
		return m, nil

	case "ctrl+u":
		m.errorLogLineInput = ""
		halfPage := maxVisible / 2
		m.errorLogCursorLine = max(m.errorLogCursorLine-halfPage, 0)
		m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
		return m, nil

	case "ctrl+f", "pgdown":
		m.errorLogLineInput = ""
		m.errorLogCursorLine = min(m.errorLogCursorLine+maxVisible, maxCursor)
		m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
		return m, nil

	case "ctrl+b", "pgup":
		m.errorLogLineInput = ""
		m.errorLogCursorLine = max(m.errorLogCursorLine-maxVisible, 0)
		m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
		return m, nil

	case "home":
		m.pendingG = false
		m.errorLogLineInput = ""
		m.errorLogCursorLine = 0
		m.errorLogScroll = 0
		return m, nil

	case "end":
		m.errorLogLineInput = ""
		m.errorLogCursorLine = maxCursor
		m.errorLogScroll = maxScroll
		return m, nil

	default:
		m.errorLogLineInput = ""
	}
	return m, nil
}

// errorLogEnsureCursorVisible adjusts scroll so the cursor line is within the
// visible window with scrolloff margin.
func (m Model) errorLogEnsureCursorVisible(maxVisible, maxScroll int) int {
	scroll := m.errorLogScroll
	so := min(ui.ConfigScrollOff, maxVisible/2)
	if m.errorLogCursorLine < scroll+so {
		scroll = m.errorLogCursorLine - so
	}
	if m.errorLogCursorLine >= scroll+maxVisible-so {
		scroll = m.errorLogCursorLine - maxVisible + so + 1
	}
	return max(min(scroll, maxScroll), 0)
}

// errorLogYank copies error log content to clipboard.
// In visual mode: copies selected lines. Otherwise: copies all visible entries.
func (m Model) errorLogYank() (tea.Model, tea.Cmd) {
	reversed := ui.FilteredErrorLogEntries(m.errorLog, m.showDebugLogs)
	if len(reversed) == 0 {
		return m, nil
	}

	var lines []string
	switch m.errorLogVisualMode {
	case 'v':
		// Character visual mode: extract partial text respecting column positions.
		selStart := min(m.errorLogVisualStart, m.errorLogCursorLine)
		selEnd := max(m.errorLogVisualStart, m.errorLogCursorLine)
		// Determine start/end columns based on direction.
		var startCol, endCol int
		if m.errorLogVisualStart <= m.errorLogCursorLine {
			startCol = m.errorLogVisualStartCol
			endCol = m.errorLogCursorCol
		} else {
			startCol = m.errorLogCursorCol
			endCol = m.errorLogVisualStartCol
		}
		for i := selStart; i <= selEnd && i < len(reversed); i++ {
			plain := ui.ErrorLogEntryPlainText(reversed[i])
			runes := []rune(plain)
			if selStart == selEnd {
				// Single line: extract between columns.
				cStart := min(startCol, endCol)
				cEnd := min(max(startCol, endCol)+1, len(runes))
				if cStart < len(runes) {
					lines = append(lines, string(runes[cStart:cEnd]))
				}
			} else if i == selStart {
				if startCol < len(runes) {
					lines = append(lines, string(runes[startCol:]))
				}
			} else if i == selEnd {
				cEnd := min(endCol+1, len(runes))
				lines = append(lines, string(runes[:cEnd]))
			} else {
				lines = append(lines, plain)
			}
		}
		m.errorLogVisualMode = 0
	case 'V':
		// Line visual mode: full lines.
		selStart := min(m.errorLogVisualStart, m.errorLogCursorLine)
		selEnd := max(m.errorLogVisualStart, m.errorLogCursorLine)
		for i := selStart; i <= selEnd && i < len(reversed); i++ {
			lines = append(lines, ui.ErrorLogEntryPlainText(reversed[i]))
		}
		m.errorLogVisualMode = 0
	default:
		for _, e := range reversed {
			lines = append(lines, ui.ErrorLogEntryPlainText(e))
		}
	}

	text := strings.Join(lines, "\n")
	m.setStatusMessage(fmt.Sprintf("Copied %d entries to clipboard", len(lines)), false)
	return m, tea.Batch(copyToSystemClipboard(text), scheduleStatusClear())
}

func (m Model) handleErrorLogOverlayKeyEsc() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	m.overlayErrorLog = false
	m.errorLogScroll = 0
	m.errorLogFullscreen = false
	m.errorLogVisualMode = 0
	m.errorLogCursorLine = 0
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyF() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	m.errorLogFullscreen = !m.errorLogFullscreen
	// Reset scroll when toggling to avoid out-of-bounds.
	m.errorLogScroll = 0
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyV() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.errorLogVisualMode == 'V' {
		m.errorLogVisualMode = 0
	} else {
		m.errorLogVisualMode = 'V'
		m.errorLogVisualStart = m.errorLogCursorLine
	}
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyV2() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.errorLogVisualMode == 'v' {
		m.errorLogVisualMode = 0
	} else {
		m.errorLogVisualMode = 'v'
		m.errorLogVisualStart = m.errorLogCursorLine
		m.errorLogVisualStartCol = m.errorLogCursorCol
	}
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyH() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.errorLogVisualMode == 'v' && m.errorLogCursorCol > 0 {
		m.errorLogCursorCol--
	}
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyL() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.errorLogVisualMode == 'v' {
		// Clamp to line length.
		reversed := ui.FilteredErrorLogEntries(m.errorLog, m.showDebugLogs)
		if m.errorLogCursorLine < len(reversed) {
			lineLen := len([]rune(ui.ErrorLogEntryPlainText(reversed[m.errorLogCursorLine])))
			if m.errorLogCursorCol < lineLen-1 {
				m.errorLogCursorCol++
			}
		}
	}
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyZero() (tea.Model, tea.Cmd) {
	if m.errorLogLineInput != "" {
		m.errorLogLineInput += "0"
		return m, nil
	}
	if m.errorLogVisualMode == 'v' {
		m.errorLogCursorCol = 0
	}
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyDollar() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.errorLogVisualMode == 'v' {
		reversed := ui.FilteredErrorLogEntries(m.errorLog, m.showDebugLogs)
		if m.errorLogCursorLine < len(reversed) {
			lineLen := len([]rune(ui.ErrorLogEntryPlainText(reversed[m.errorLogCursorLine])))
			m.errorLogCursorCol = max(lineLen-1, 0)
		}
	}
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyD() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.errorLogVisualMode != 0 {
		// Don't toggle debug in visual mode — 'd' is ambiguous.
		return m, nil
	}
	m.showDebugLogs = !m.showDebugLogs
	m.errorLogScroll = 0
	m.errorLogCursorLine = 0
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyG() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.pendingG {
		m.pendingG = false
		m.errorLogCursorLine = 0
		m.errorLogScroll = 0
		return m, nil
	}
	m.pendingG = true
	return m, nil
}
