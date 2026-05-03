package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle paste events.
	if msg.Paste {
		text := strings.TrimRight(string(msg.Runes), "\n")
		if strings.Contains(text, "\n") {
			m.triggerPasteConfirm(text, pasteTargetFilter)
			return m, nil
		}
		if text != "" {
			m.filterInput.Insert(text)
			m.filterText = m.filterInput.Value
			m.setCursor(0)
			m.clampCursor()
			// Paste counts as an edit: leave history-browse so a
			// follow-up Down doesn't keep navigating history.
			m.queryHistory.leaveBrowse()
		}
		return m, nil
	}
	switch msg.String() {
	case "enter":
		m.filterText = m.filterInput.Value
		m.filterActive = false
		// Keep filterBroadMode as-is: visibleMiddleItems consults it to
		// decide whether to scan column values, so resetting here would
		// silently drop the user's broad-scope filter the moment they
		// confirm. Reset happens on Esc (cancel) or when a new filter
		// input starts via handleKeyFilter.
		m.setCursor(0)
		m.clampCursor()
		m.queryHistory.add(m.filterInput.Value)
		m.queryHistory.save()
		// The cursor now points at the first filter match — a different
		// item than before. Without invalidation the right pane keeps
		// rendering the previous selection's rightItems (and skips the
		// loader), so the user sees stale children for several seconds
		// until the new preview fetch returns. Bumping requestGen also
		// discards any in-flight preview from the pre-filter cursor.
		m.invalidatePreviewForCursorChange()
		return m, m.loadPreview()
	case "esc":
		m.filterActive = false
		m.filterBroadMode = false
		m.filterInput.Clear()
		m.filterText = ""
		m.setCursor(0)
		m.clampCursor()
		m.invalidatePreviewForCursorChange()
		return m, m.loadPreview()
	case "up":
		m.filterInput.Set(m.queryHistory.up(m.filterInput.Value))
		m.filterText = m.filterInput.Value
		m.setCursor(0)
		m.clampCursor()
		return m, nil
	case "down":
		m.filterInput.Set(m.queryHistory.down())
		m.filterText = m.filterInput.Value
		m.setCursor(0)
		m.clampCursor()
		return m, nil
	case "tab":
		// Toggle broad mode: also match against column values
		// (annotations, labels, finalizers, CRD printer columns, ...).
		m.filterBroadMode = !m.filterBroadMode
		// Reset cursor since the visible set may change shape.
		m.setCursor(0)
		m.clampCursor()
		return m, nil
	case "backspace":
		if len(m.filterInput.Value) > 0 {
			m.filterInput.Backspace()
			m.filterText = m.filterInput.Value
			m.setCursor(0)
			m.clampCursor()
			// Editing a recalled entry leaves history navigation but
			// keeps the pre-recall draft intact, so a later Down past
			// newest restores the original draft the user had before
			// pressing Up — not the recalled-then-edited text.
			m.queryHistory.leaveBrowse()
		}
		return m, nil
	case "ctrl+w":
		m.filterInput.DeleteWord()
		m.filterText = m.filterInput.Value
		m.setCursor(0)
		m.clampCursor()
		m.queryHistory.leaveBrowse()
		return m, nil
	case "ctrl+u":
		m.filterInput.DeleteLine()
		m.filterText = m.filterInput.Value
		m.setCursor(0)
		m.clampCursor()
		m.queryHistory.leaveBrowse()
		return m, nil
	case "ctrl+a":
		m.filterInput.Home()
		return m, nil
	case "ctrl+e":
		m.filterInput.End()
		return m, nil
	case "left":
		m.filterInput.Left()
		return m, nil
	case "right":
		m.filterInput.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.filterInput.Insert(key)
			m.filterText = m.filterInput.Value
			m.setCursor(0)
			m.clampCursor()
			m.queryHistory.leaveBrowse()
		}
		return m, nil
	}
}
