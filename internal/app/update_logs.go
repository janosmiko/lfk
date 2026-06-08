package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleLogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle log search input mode.
	if m.logView.searchActive {
		return m.handleLogSearchKey(msg)
	}

	// Handle visual select mode keys.
	if m.logView.visualMode {
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
	m.logView.lineInput = ""
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
	case "ctrl+d", "shift+down":
		ret := m.handleLogKeyCtrlD()
		return ret, nil, true
	case "ctrl+u", "shift+up":
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
		m.logView.cursor = 0
		m.logView.scroll = 0
		m.logView.follow = false
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
		m.logView.lineInput += msg.String()
		return m, nil, true
	}
	return m, nil, false
}

// handleLogActionKey handles action/mode keys in the log viewer.
func (m Model) handleLogActionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	kb := ui.ActiveKeybindings
	switch msg.String() {
	case kb.Help, "f1":
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
	case "y":
		ret, cmd := m.handleLogNormalCopy()
		return ret, cmd, true
	case kb.ToggleFollow:
		ret := m.handleLogKeyF()
		return ret, nil, true
	case kb.ToggleWrap:
		ret := m.handleLogKeyTab()
		return ret, nil, true
	case kb.Search:
		ret := m.handleLogKeySlash()
		return ret, nil, true
	case kb.NextMatch:
		ret := m.handleLogKeyN()
		return ret, nil, true
	case kb.PrevMatch:
		ret := m.handleLogKeyN2()
		return ret, nil, true
	case kb.TogglePrefixes:
		ret := m.handleLogKeyP()
		return ret, nil, true
	case kb.TogglePreview:
		ret := m.handleLogKeyP2()
		return ret, nil, true
	case "J":
		if !m.logView.previewVisible {
			return m, nil, false
		}
		ret := m.handleLogKeyJ2()
		return ret, nil, true
	case "K":
		if !m.logView.previewVisible {
			return m, nil, false
		}
		ret := m.handleLogKeyK2()
		return ret, nil, true
	case kb.ToggleLineNumbers:
		ret := m.handleLogKeyHash()
		return ret, nil, true
	case kb.ToggleTimestamps:
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
	key := msg.String()
	if op, motion, ok := m.consumeTextObjectPrelude(key); ok {
		return m.applyLogTextObject(op, motion)
	}
	switch key {
	case "esc":
		m.logView.visualMode = false
		return m, nil
	case "i", "a":
		// Clear any digit prefix accumulated before visual entry so it can't
		// leak into a later counted command via the post-visual normal mode.
		m.logView.lineInput = ""
		m.pendingTextObject = key[0]
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
	case "ctrl+d", "shift+down":
		return m.handleLogVisualKeyCtrlD()
	case "ctrl+u", "shift+up":
		return m.handleLogVisualKeyCtrlU()
	case "ctrl+c":
		m.logView.visualMode = false
		return m.closeTabOrQuit()
	case "q":
		m.logView.visualMode = false
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
		m.logView.visualCurCol = 0
		return m, nil
	case "^":
		return m.handleLogVisualKeyCaret()
	}
	return m, nil
}

func (m Model) handleLogSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.logView.searchActive = false
		m.logView.searchQuery = m.logView.searchInput.Value
		m.logView.searchHistory.add(m.logView.searchInput.Value)
		m.logView.searchHistory.save()
		m.findNextLogMatch(true)
	case "esc":
		m.logView.searchActive = false
		m.logView.searchInput.Clear()
		m.logView.searchQuery = ""
	case "up":
		m.logView.searchInput.Set(m.logView.searchHistory.up(m.logView.searchInput.Value))
		m.logView.searchQuery = m.logView.searchInput.Value
	case "down":
		m.logView.searchInput.Set(m.logView.searchHistory.down())
		m.logView.searchQuery = m.logView.searchInput.Value
	case "backspace":
		if len(m.logView.searchInput.Value) > 0 {
			m.logView.searchInput.Backspace()
			m.logView.searchQuery = m.logView.searchInput.Value
			// Editing a recalled entry leaves history navigation but
			// keeps the pre-recall draft intact, so a later Down past
			// newest restores the original draft the user had before
			// pressing Up — not the recalled-then-edited text.
			m.logView.searchHistory.leaveBrowse()
		}
	case "ctrl+w":
		m.logView.searchInput.DeleteWord()
		m.logView.searchQuery = m.logView.searchInput.Value
		m.logView.searchHistory.leaveBrowse()
	case "ctrl+u":
		m.logView.searchInput.DeleteLine()
		m.logView.searchQuery = m.logView.searchInput.Value
		m.logView.searchHistory.leaveBrowse()
	case "ctrl+a":
		m.logView.searchInput.Home()
	case "ctrl+e":
		m.logView.searchInput.End()
	case "left":
		m.logView.searchInput.Left()
	case "right":
		m.logView.searchInput.Right()
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.logView.searchInput.Insert(key)
			// Live-update the highlight query so matches paint as the user
			// types. Enter still "commits" search-input mode and triggers
			// findNextLogMatch; before that the user only saw the input
			// echo with no feedback on whether the query matches anything.
			m.logView.searchQuery = m.logView.searchInput.Value
			m.logView.searchHistory.leaveBrowse()
		}
	}
	return m, nil
}
