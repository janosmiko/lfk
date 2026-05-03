package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

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

	clipText := visualCopyText(displayed, 0, len(displayed)-1,
		m.logVisualType, m.logVisualCol, m.logVisualCurCol,
		m.logVisualStart > m.logCursor)
	return clipText, len(displayed)
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
	m.logCursor += scrollStep(m.logScrollOption, m.logContentHeight())
	if m.logCursor >= len(m.logLines) {
		m.logCursor = len(m.logLines) - 1
	}
	m.ensureLogCursorVisible()
	return m, nil
}

func (m Model) handleLogVisualKeyCtrlU() (tea.Model, tea.Cmd) {
	m.logCursor -= scrollStep(m.logScrollOption, m.logContentHeight())
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
