package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleLogSinceInputKey drives the overlay that prompts for a
// kubectl-style --since duration.  Enter commits the value: an empty
// input clears the filter, a valid duration updates logSinceDuration
// and restarts the stream, and an invalid value leaves state untouched
// and shows a status message.  Esc / q close the prompt without
// applying.
func (m Model) handleLogSinceInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.logSinceInput.Clear()
		return m, nil
	case "enter":
		return m.commitLogSinceInput()
	case "backspace":
		m.logSinceInput.Backspace()
		return m, nil
	case "ctrl+w":
		m.logSinceInput.DeleteWord()
		return m, nil
	case "ctrl+u":
		m.logSinceInput.Clear()
		return m, nil
	case "ctrl+a":
		m.logSinceInput.Home()
		return m, nil
	case "ctrl+e":
		m.logSinceInput.End()
		return m, nil
	case "left":
		m.logSinceInput.Left()
		return m, nil
	case "right":
		m.logSinceInput.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		// Allow printable ASCII so users can type fractions ("1.5h"),
		// mixed units ("1h30m"), and the "d" shorthand freely.
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.logSinceInput.Insert(key)
		}
		return m, nil
	}
}

// commitLogSinceInput applies the value currently in m.logSinceInput.
// Empty input clears any active filter; a valid duration replaces
// logSinceDuration and restarts the log stream so kubectl is re-invoked
// with the new --since flag.  Invalid input is rejected with a status
// message — existing state is left alone so the user can correct it.
func (m Model) commitLogSinceInput() (tea.Model, tea.Cmd) {
	raw := m.logSinceInput.Value
	trimmed := trimSpace(raw)

	// Empty input clears the filter.  If nothing was set in the first
	// place, close the overlay silently — no need to restart the stream
	// or flash a status message.
	if trimmed == "" {
		alreadyOff := m.logSinceDuration == ""
		m.overlay = overlayNone
		m.logSinceInput.Clear()
		if alreadyOff {
			return m, nil
		}
		m.logSinceDuration = ""
		m.setStatusMessage("Since filter cleared", false)
		return m.restartLogStreamForSince()
	}

	if _, display, err := parseLogSinceDuration(trimmed); err != nil {
		m.setStatusMessage("Invalid duration: "+trimmed, true)
		return m, scheduleStatusClear()
	} else {
		m.overlay = overlayNone
		m.logSinceInput.Clear()
		m.logSinceDuration = display
		m.setStatusMessage("Since "+display, false)
		return m.restartLogStreamForSince()
	}
}

// restartLogStreamForSince cancels the current log stream and any
// in-flight history fetch, clears the buffered lines, then re-dispatches
// the stream with the new logSinceDuration applied.  Multi-log streams
// use restartMultiLogStream; single streams use startLogStream.  The
// filter chain is preserved — only the source data is reset.
func (m Model) restartLogStreamForSince() (tea.Model, tea.Cmd) {
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
	}
	if m.logHistoryCancel != nil {
		m.logHistoryCancel()
		m.logHistoryCancel = nil
	}
	m.logCh = nil
	m.logLines = nil
	m.logVisibleIndices = nil
	m.logScroll = 0
	m.logCursor = 0
	m.logVisualMode = false
	m.logFollow = true
	// A --since window makes older-history fetches pointless: kubectl
	// won't return anything older than the window regardless of --tail.
	m.logHasMoreHistory = m.logSinceDuration == "" && !m.logPrevious && !m.logIsMulti
	m.logLoadingHistory = false

	if m.logIsMulti && len(m.logMultiItems) > 0 {
		var cmd tea.Cmd
		m, cmd = m.restartMultiLogStream()
		return m, cmd
	}
	return m, m.startLogStream()
}

// trimSpace is a small helper kept local so the overlay stays
// self-contained and doesn't pull in strings for one call.
func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
