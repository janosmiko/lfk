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
	case kb.NewTab, kb.NextTab, kb.PrevTab, kb.MoveTabLeft, kb.MoveTabRight:
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

	// Cursor-column movement (h/l/0/$/^ and word motions) — extracted to keep
	// this function under the gocyclo cap.
	if mdl, handled := m.errorLogColumnMotion(key); handled {
		return mdl, nil
	}

	switch key {
	case "esc", "q":
		return m.handleErrorLogOverlayKeyEsc()

	case ui.ActiveKeybindings.Fullscreen:
		return m.handleErrorLogOverlayKeyF()

	case "V":
		return m.handleErrorLogOverlayKeyV()

	case "v":
		return m.handleErrorLogOverlayKeyV2()

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

	case "ctrl+d", "shift+down":
		m.errorLogLineInput = ""
		halfPage := maxVisible / 2
		m.errorLogCursorLine = min(m.errorLogCursorLine+halfPage, maxCursor)
		m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
		return m, nil

	case "ctrl+u", "shift+up":
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
		// Clamp first so the selection anchor isn't left past end-of-line by a
		// prior vertical move onto a shorter line.
		m.errorLogCursorCol = m.errorLogClampedCursorCol()
		m.errorLogVisualMode = 'v'
		m.errorLogVisualStart = m.errorLogCursorLine
		m.errorLogVisualStartCol = m.errorLogCursorCol
	}
	return m, nil
}

// errorLogColumnMotion dispatches the cursor-column movement keys (h/l/0/$/^
// and the vim word motions). Returns handled=false for any other key so the
// main key switch can run. Works in both normal and char-visual modes. These
// motions never emit a command, so only the updated Model is returned.
func (m Model) errorLogColumnMotion(key string) (Model, bool) {
	// Clamp a column left over from a vertical move onto a shorter line before
	// applying the motion. The clamp only sticks when a column key is handled;
	// for other keys the returned Model is discarded by the caller.
	m.errorLogCursorCol = m.errorLogClampedCursorCol()
	switch key {
	case "h", "left":
		return m.handleErrorLogOverlayKeyH(), true
	case "l", "right":
		return m.handleErrorLogOverlayKeyL(), true
	case "0":
		return m.handleErrorLogOverlayKeyZero(), true
	case "$":
		return m.handleErrorLogOverlayKeyDollar(), true
	case "w", "e", "b", "W", "E", "B", "^":
		return m.handleErrorLogOverlayWordMotion(key), true
	}
	return m, false
}

// errorLogCurrentLine returns the cursor entry's plain text (the basis for
// horizontal cursor movement and word motions), or "" when out of range.
func (m Model) errorLogCurrentLine() string {
	reversed := ui.FilteredErrorLogEntries(m.errorLog, m.showDebugLogs)
	if m.errorLogCursorLine < 0 || m.errorLogCursorLine >= len(reversed) {
		return ""
	}
	return ui.ErrorLogEntryPlainText(reversed[m.errorLogCursorLine])
}

// errorLogCurrentLineLen returns the rune length of the cursor entry's plain
// text, used to clamp horizontal cursor movement.
func (m Model) errorLogCurrentLineLen() int {
	return len([]rune(m.errorLogCurrentLine()))
}

// errorLogClampedCursorCol returns errorLogCursorCol clamped to the current
// line. A vertical move (j/k) onto a shorter line leaves the column past that
// line's end; clamping on read keeps horizontal motions and selection anchors
// valid while preserving the column when moving back onto a longer line.
func (m Model) errorLogClampedCursorCol() int {
	return min(m.errorLogCursorCol, max(m.errorLogCurrentLineLen()-1, 0))
}

// handleErrorLogOverlayWordMotion applies a vim word/WORD motion (w/e/b/W/E/B)
// or ^ to the cursor column, reusing the shared motion helpers (update_vim.go).
// Works in both normal and char-visual modes — char-visual additionally
// extends the selection to the new column. Results are clamped to the line so
// the cursor stays on a real character (matching the clamped h/l behaviour).
func (m Model) handleErrorLogOverlayWordMotion(key string) Model {
	m.errorLogLineInput = ""
	line := m.errorLogCurrentLine()
	if line == "" {
		return m
	}
	maxCol := len([]rune(line)) - 1
	switch key {
	case "^":
		m.errorLogCursorCol = firstNonWhitespace(line)
	case "w":
		m.errorLogCursorCol = min(nextWordStart(line, m.errorLogCursorCol), maxCol)
	case "W":
		m.errorLogCursorCol = min(nextWORDStart(line, m.errorLogCursorCol), maxCol)
	case "e":
		m.errorLogCursorCol = min(wordEnd(line, m.errorLogCursorCol), maxCol)
	case "E":
		m.errorLogCursorCol = min(WORDEnd(line, m.errorLogCursorCol), maxCol)
	case "b":
		if nc := prevWordStart(line, m.errorLogCursorCol); nc >= 0 {
			m.errorLogCursorCol = nc
		}
	case "B":
		if nc := prevWORDStart(line, m.errorLogCursorCol); nc >= 0 {
			m.errorLogCursorCol = nc
		}
	}
	return m
}

// h/l/0/$ move the cursor column in both normal and char-visual modes (the
// cursor line carries a block cursor in normal mode, matching the event
// viewer); char-visual additionally extends the selection to that column.
// These never emit a command, so they return the updated Model directly.
func (m Model) handleErrorLogOverlayKeyH() Model {
	m.errorLogLineInput = ""
	if m.errorLogCursorCol > 0 {
		m.errorLogCursorCol--
	}
	return m
}

func (m Model) handleErrorLogOverlayKeyL() Model {
	m.errorLogLineInput = ""
	if m.errorLogCursorCol < m.errorLogCurrentLineLen()-1 {
		m.errorLogCursorCol++
	}
	return m
}

func (m Model) handleErrorLogOverlayKeyZero() Model {
	if m.errorLogLineInput != "" {
		m.errorLogLineInput += "0"
		return m
	}
	m.errorLogCursorCol = 0
	return m
}

func (m Model) handleErrorLogOverlayKeyDollar() Model {
	m.errorLogLineInput = ""
	m.errorLogCursorCol = max(m.errorLogCurrentLineLen()-1, 0)
	return m
}

func (m Model) handleErrorLogOverlayKeyD() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.errorLogVisualMode != 0 {
		// Don't toggle debug in visual mode — 'd' is ambiguous.
		return m, nil
	}

	// Anchor on the currently selected entry so the cursor stays on the same
	// log line across the filter change instead of jumping back to the top.
	// When hiding debug, the selected line may itself be a DBG entry that is
	// about to disappear — fall back to the nearest surviving entry.
	before := ui.FilteredErrorLogEntries(m.errorLog, m.showDebugLogs)
	m.showDebugLogs = !m.showDebugLogs
	anchor, ok := nearestSurvivingErrorLogEntry(before, m.errorLogCursorLine, m.showDebugLogs)

	after := ui.FilteredErrorLogEntries(m.errorLog, m.showDebugLogs)
	if ok {
		if idx := indexOfErrorLogEntry(after, anchor); idx >= 0 {
			_, maxVisible, maxScroll := m.errorLogVisibleCount()
			m.errorLogCursorLine = idx
			m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
			return m, nil
		}
	}

	// Nothing to anchor to (empty log, or the entry vanished): reset to top.
	m.errorLogCursorLine = 0
	m.errorLogScroll = 0
	return m, nil
}

// nearestSurvivingErrorLogEntry returns the entry at cursor if it survives the
// post-toggle debug filter, otherwise the closest entry that does. The list is
// newest-first, so the outward scan tries the higher index (older entry) before
// the lower index (newer entry). Returns ok=false when no entry survives or the
// cursor is out of range.
func nearestSurvivingErrorLogEntry(entries []ui.ErrorLogEntry, cursor int, showDebugAfter bool) (ui.ErrorLogEntry, bool) {
	survives := func(e ui.ErrorLogEntry) bool { return showDebugAfter || e.Level != "DBG" }
	if cursor < 0 || cursor >= len(entries) {
		return ui.ErrorLogEntry{}, false
	}
	if survives(entries[cursor]) {
		return entries[cursor], true
	}
	for off := 1; off < len(entries); off++ {
		if i := cursor + off; i < len(entries) && survives(entries[i]) {
			return entries[i], true
		}
		if i := cursor - off; i >= 0 && survives(entries[i]) {
			return entries[i], true
		}
	}
	return ui.ErrorLogEntry{}, false
}

// indexOfErrorLogEntry finds the index of an entry by its identity
// (timestamp + level + message). Returns -1 when absent.
func indexOfErrorLogEntry(entries []ui.ErrorLogEntry, target ui.ErrorLogEntry) int {
	for i, e := range entries {
		if e.Time.Equal(target.Time) && e.Level == target.Level && e.Message == target.Message {
			return i
		}
	}
	return -1
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
