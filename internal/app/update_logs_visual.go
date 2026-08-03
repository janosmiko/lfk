package app

import (
	tea "charm.land/bubbletea/v2"
)

func (m Model) handleLogVisualKeyV() (tea.Model, tea.Cmd) {
	if m.logView.visualType == 'V' {
		m.logView.visualMode = false
	} else {
		m.logView.visualType = 'V'
	}
	return m, nil
}

func (m Model) handleLogVisualKeyV2() (tea.Model, tea.Cmd) {
	if m.logView.visualType == 'v' {
		m.logView.visualMode = false
	} else {
		m.logView.visualType = 'v'
	}
	return m, nil
}

func (m Model) handleLogVisualKeyCtrlV() (tea.Model, tea.Cmd) {
	if m.logView.visualType == 'B' {
		m.logView.visualMode = false
	} else {
		m.logView.visualType = 'B'
	}
	return m, nil
}

func (m Model) handleLogVisualKeyY() (tea.Model, tea.Cmd) {
	clipText, lineCount := m.buildLogYankText()
	m.logView.visualMode = false
	m.setStatusMessage(formatVisualYank(clipText, m.logView.visualType, lineCount), false)
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
	selStart := min(m.logView.visualStart, m.logView.cursor)
	selEnd := max(m.logView.visualStart, m.logView.cursor)
	if selStart < 0 {
		selStart = 0
	}
	if selEnd >= len(m.logView.lines) {
		selEnd = len(m.logView.lines) - 1
	}
	if selStart > selEnd {
		return "", 0
	}

	displayed := make([]string, selEnd-selStart+1)
	for i := selStart; i <= selEnd; i++ {
		displayed[i-selStart] = m.logDisplayLine(i)
	}

	clipText := visualCopyText(displayed, 0, len(displayed)-1,
		m.logView.visualType, m.logView.visualCol, m.logView.visualCurCol,
		m.logView.visualStart > m.logView.cursor)
	return clipText, len(displayed)
}

func (m Model) handleLogVisualKeyH() (tea.Model, tea.Cmd) {
	if m.logView.visualType == 'v' || m.logView.visualType == 'B' {
		if m.logView.visualCurCol > 0 {
			m.logView.visualCurCol--
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyL() (tea.Model, tea.Cmd) {
	if m.logView.visualType == 'v' || m.logView.visualType == 'B' {
		m.logView.visualCurCol++
	}
	return m, nil
}

func (m Model) handleLogVisualKeyJ() (tea.Model, tea.Cmd) {
	if m.logView.cursor < len(m.logView.lines)-1 {
		m.logView.cursor++
	}
	m.ensureLogCursorVisible()
	return m, nil
}

func (m Model) handleLogVisualKeyK() (tea.Model, tea.Cmd) {
	if m.logView.cursor > 0 {
		m.logView.cursor--
	}
	m.ensureLogCursorVisible()
	cmd := m.maybeLoadMoreHistory()
	return m, cmd
}

func (m Model) handleLogVisualKeyG() (tea.Model, tea.Cmd) {
	m.logView.cursor = len(m.logView.lines) - 1
	m.ensureLogCursorVisible()
	return m, nil
}

func (m Model) handleLogVisualKeyG2() (tea.Model, tea.Cmd) {
	if m.pendingG {
		m.pendingG = false
		m.logView.cursor = 0
		m.ensureLogCursorVisible()
		return m, nil
	}
	m.pendingG = true
	return m, nil
}

func (m Model) handleLogVisualKeyCtrlD() (tea.Model, tea.Cmd) {
	m.logView.cursor += scrollStep(m.logView.scrollOption, m.logContentHeight())
	if m.logView.cursor >= len(m.logView.lines) {
		m.logView.cursor = len(m.logView.lines) - 1
	}
	m.ensureLogCursorVisible()
	return m, nil
}

func (m Model) handleLogVisualKeyCtrlU() (tea.Model, tea.Cmd) {
	m.logView.cursor -= scrollStep(m.logView.scrollOption, m.logContentHeight())
	if m.logView.cursor < 0 {
		m.logView.cursor = 0
	}
	m.ensureLogCursorVisible()
	return m, nil
}

func (m Model) handleLogVisualKeyDollar() (tea.Model, tea.Cmd) {
	if m.logView.cursor >= 0 && m.logView.cursor < len(m.logView.lines) {
		lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
		if lineLen > 0 {
			m.logView.visualCurCol = lineLen - 1
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyE() (tea.Model, tea.Cmd) {
	if m.logView.cursor >= 0 && m.logView.cursor < len(m.logView.lines) {
		lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
		newCol := wordEnd(m.logView.lines[m.logView.cursor], m.logView.visualCurCol)
		if newCol >= lineLen && m.logView.cursor < len(m.logView.lines)-1 {
			m.logView.cursor++
			newCol = wordEnd(m.logView.lines[m.logView.cursor], 0)
			nextLineLen := len([]rune(m.logView.lines[m.logView.cursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logView.visualCurCol = newCol
			m.ensureLogCursorVisible()
		} else {
			m.logView.visualCurCol = newCol
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyB() (tea.Model, tea.Cmd) {
	if m.logView.cursor >= 0 && m.logView.cursor < len(m.logView.lines) {
		newCol := prevWordStart(m.logView.lines[m.logView.cursor], m.logView.visualCurCol)
		if newCol < 0 && m.logView.cursor > 0 {
			m.logView.cursor--
			lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
			newCol = max(prevWordStart(m.logView.lines[m.logView.cursor], lineLen), 0)
			m.logView.visualCurCol = newCol
			m.ensureLogCursorVisible()
		} else {
			m.logView.visualCurCol = max(newCol, 0)
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyW() (tea.Model, tea.Cmd) {
	if m.logView.cursor >= 0 && m.logView.cursor < len(m.logView.lines) {
		lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
		newCol := nextWordStart(m.logView.lines[m.logView.cursor], m.logView.visualCurCol)
		if newCol >= lineLen && m.logView.cursor < len(m.logView.lines)-1 {
			m.logView.cursor++
			newCol = nextWordStart(m.logView.lines[m.logView.cursor], 0)
			nextLineLen := len([]rune(m.logView.lines[m.logView.cursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logView.visualCurCol = newCol
			m.ensureLogCursorVisible()
		} else {
			m.logView.visualCurCol = newCol
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyW2() (tea.Model, tea.Cmd) {
	if m.logView.cursor >= 0 && m.logView.cursor < len(m.logView.lines) {
		lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
		newCol := nextWORDStart(m.logView.lines[m.logView.cursor], m.logView.visualCurCol)
		if newCol >= lineLen && m.logView.cursor < len(m.logView.lines)-1 {
			m.logView.cursor++
			newCol = nextWORDStart(m.logView.lines[m.logView.cursor], 0)
			nextLineLen := len([]rune(m.logView.lines[m.logView.cursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logView.visualCurCol = newCol
			m.ensureLogCursorVisible()
		} else {
			m.logView.visualCurCol = newCol
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyE2() (tea.Model, tea.Cmd) {
	if m.logView.cursor >= 0 && m.logView.cursor < len(m.logView.lines) {
		lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
		newCol := WORDEnd(m.logView.lines[m.logView.cursor], m.logView.visualCurCol)
		if newCol >= lineLen && m.logView.cursor < len(m.logView.lines)-1 {
			m.logView.cursor++
			newCol = WORDEnd(m.logView.lines[m.logView.cursor], 0)
			nextLineLen := len([]rune(m.logView.lines[m.logView.cursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logView.visualCurCol = newCol
			m.ensureLogCursorVisible()
		} else {
			m.logView.visualCurCol = newCol
		}
	}
	return m, nil
}

func (m Model) handleLogVisualKeyB2() (tea.Model, tea.Cmd) {
	if m.logView.cursor >= 0 && m.logView.cursor < len(m.logView.lines) {
		newCol := prevWORDStart(m.logView.lines[m.logView.cursor], m.logView.visualCurCol)
		if newCol < 0 && m.logView.cursor > 0 {
			m.logView.cursor--
			lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
			newCol = max(prevWORDStart(m.logView.lines[m.logView.cursor], lineLen), 0)
			m.logView.visualCurCol = newCol
			m.ensureLogCursorVisible()
		} else {
			m.logView.visualCurCol = max(newCol, 0)
		}
	}
	return m, nil
}

// applyLogTextObject resolves an `iw`/`aw`/`iW`/`aW` text object on the line
// under the cursor and switches the visual selection to character mode
// covering the resulting range.
func (m Model) applyLogTextObject(op byte, motion string) (tea.Model, tea.Cmd) {
	if m.logView.cursor < 0 || m.logView.cursor >= len(m.logView.lines) {
		return m, nil
	}
	start, end, ok := textObjectRange(m.logView.lines[m.logView.cursor], m.logView.visualCurCol, op, motion)
	if !ok {
		return m, nil
	}
	m.logView.visualType = 'v'
	m.logView.visualStart = m.logView.cursor
	m.logView.visualCol = start
	m.logView.visualCurCol = end
	return m, nil
}

func (m Model) handleLogVisualKeyCaret() (tea.Model, tea.Cmd) {
	if m.logView.cursor >= 0 && m.logView.cursor < len(m.logView.lines) {
		m.logView.visualCurCol = firstNonWhitespace(m.logView.lines[m.logView.cursor])
	}
	return m, nil
}
