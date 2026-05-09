package app

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleYAMLVisualKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalVisible := visibleLineCount(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	maxScroll := m.yamlMaxScroll(totalVisible)

	key := msg.String()
	if op, motion, ok := m.consumeTextObjectPrelude(key); ok {
		return m.applyYAMLTextObject(op, motion)
	}
	switch key {
	case "esc":
		m.yamlVisualMode = false
		return m, nil
	case "i", "a":
		// Clear any digit prefix accumulated before visual entry so it can't
		// leak into a later counted command via the post-visual normal mode.
		m.yamlLineInput = ""
		m.pendingTextObject = key[0]
		return m, nil
	case "V":
		return m.handleYAMLVisualToggleMode('V')
	case "v":
		return m.handleYAMLVisualToggleMode('v')
	case "ctrl+v":
		return m.handleYAMLVisualToggleMode('B')
	case "y":
		return m.handleYAMLVisualCopy()
	case "h", "left":
		if m.yamlVisualType == 'v' || m.yamlVisualType == 'B' {
			if m.yamlVisualCurCol > yamlFoldPrefixLen {
				m.yamlVisualCurCol--
			}
		}
		return m, nil
	case "l", "right":
		if m.yamlVisualType == 'v' || m.yamlVisualType == 'B' {
			m.yamlVisualCurCol++
		}
		return m, nil
	case "j", "down":
		if m.yamlCursor < totalVisible-1 {
			m.yamlCursor++
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "k", "up":
		if m.yamlCursor > 0 {
			m.yamlCursor--
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.yamlLineInput = ""
			m.yamlCursor = 0
			m.yamlScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		return m.handleYAMLVisualG(totalVisible, maxScroll)
	case "ctrl+d":
		m.yamlCursor += scrollStep(m.yamlScrollOption, m.yamlViewportLines())
		if m.yamlCursor >= totalVisible {
			m.yamlCursor = totalVisible - 1
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+u":
		m.yamlCursor -= scrollStep(m.yamlScrollOption, m.yamlViewportLines())
		if m.yamlCursor < 0 {
			m.yamlCursor = 0
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+c":
		m.yamlVisualMode = false
		m.mode = modeExplorer
		m.yamlScroll = 0
		m.yamlCursor = 0
		return m, nil
	case "0":
		m.yamlVisualCurCol = yamlFoldPrefixLen
		return m, nil
	case "$", "w", "b", "e", "E", "B", "W", "^":
		return m.handleYAMLVisualWordMotion(msg.String())
	}
	return m, nil
}

func (m Model) handleYAMLVisualToggleMode(mode rune) (tea.Model, tea.Cmd) {
	if m.yamlVisualType == mode {
		m.yamlVisualMode = false
	} else {
		m.yamlVisualType = mode
	}
	return m, nil
}

func (m Model) handleYAMLVisualG(totalVisible, maxScroll int) (tea.Model, tea.Cmd) {
	if m.yamlLineInput != "" {
		lineNum, _ := strconv.Atoi(m.yamlLineInput)
		m.yamlLineInput = ""
		if lineNum > 0 {
			lineNum--
		}
		m.yamlCursor = max(min(lineNum, totalVisible-1), 0)
		m.ensureYAMLCursorVisible()
	} else {
		m.yamlCursor = max(totalVisible-1, 0)
		m.yamlScroll = maxScroll
	}
	return m, nil
}

func (m Model) handleYAMLVisualCopy() (tea.Model, tea.Cmd) {
	_, mapping := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	selStart := min(m.yamlVisualStart, m.yamlCursor)
	selEnd := max(m.yamlVisualStart, m.yamlCursor)
	if selStart < 0 {
		selStart = 0
	}
	if selEnd >= len(mapping) {
		selEnd = len(mapping) - 1
	}
	origLines := strings.Split(m.yamlContent, "\n")
	var clipText string
	switch m.yamlVisualType {
	case 'v':
		clipText = m.yamlVisualCopyChar(selStart, selEnd, mapping, origLines)
	case 'B':
		clipText = m.yamlVisualCopyBlock(selStart, selEnd, mapping, origLines)
	default:
		clipText = m.yamlVisualCopyLine(selStart, selEnd, mapping, origLines)
	}
	lineCount := selEnd - selStart + 1
	visualType := m.yamlVisualType
	m.yamlVisualMode = false
	m.setStatusMessage(formatVisualYank(clipText, visualType, lineCount), false)
	return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
}

func (m Model) yamlVisualCopyChar(selStart, selEnd int, mapping []int, origLines []string) string {
	var parts []string
	anchorCol := m.yamlVisualCol - yamlFoldPrefixLen
	cursorCol := m.yamlVisualCurCol - yamlFoldPrefixLen
	startCol, endCol := anchorCol, cursorCol
	if m.yamlVisualStart > m.yamlCursor {
		startCol, endCol = cursorCol, anchorCol
	}
	for i := selStart; i <= selEnd; i++ {
		if i >= len(mapping) || mapping[i] < 0 || mapping[i] >= len(origLines) {
			continue
		}
		line := origLines[mapping[i]]
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
			cs := min(startCol, len(runes))
			parts = append(parts, string(runes[cs:]))
		} else if i == selEnd {
			ce := min(endCol+1, len(runes))
			parts = append(parts, string(runes[:ce]))
		} else {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, "\n")
}

func (m Model) yamlVisualCopyBlock(selStart, selEnd int, mapping []int, origLines []string) string {
	colStart := min(m.yamlVisualCol, m.yamlVisualCurCol) - yamlFoldPrefixLen
	colEnd := max(m.yamlVisualCol, m.yamlVisualCurCol) - yamlFoldPrefixLen + 1
	var parts []string
	for i := selStart; i <= selEnd; i++ {
		if i >= len(mapping) || mapping[i] < 0 || mapping[i] >= len(origLines) {
			continue
		}
		line := origLines[mapping[i]]
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
	return strings.Join(parts, "\n")
}

func (m Model) yamlVisualCopyLine(selStart, selEnd int, mapping []int, origLines []string) string {
	var selected []string
	for i := selStart; i <= selEnd; i++ {
		if i < len(mapping) && mapping[i] >= 0 && mapping[i] < len(origLines) {
			selected = append(selected, origLines[mapping[i]])
		}
	}
	return strings.Join(selected, "\n")
}

// applyYAMLTextObject resolves an `iw`/`aw`/`iW`/`aW` text object on the
// visible YAML line under the cursor and switches the visual selection to
// character mode covering the resulting range. Columns are evaluated in
// visible-line space (with the fold prefix included) and clamped to keep the
// selection out of the fold prefix.
func (m Model) applyYAMLTextObject(op byte, motion string) (tea.Model, tea.Cmd) {
	visLines, _ := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	if m.yamlCursor < 0 || m.yamlCursor >= len(visLines) {
		return m, nil
	}
	start, end, ok := textObjectRange(visLines[m.yamlCursor], m.yamlVisualCurCol, op, motion)
	if !ok {
		return m, nil
	}
	// Drop ranges that resolve entirely inside the fold-prefix gutter; clamping
	// them would silently collapse the selection onto the first content column
	// without a corresponding visual change. Leaving early keeps the prior
	// selection state intact instead.
	if end < yamlFoldPrefixLen {
		return m, nil
	}
	if start < yamlFoldPrefixLen {
		start = yamlFoldPrefixLen
	}
	m.yamlVisualType = 'v'
	m.yamlVisualStart = m.yamlCursor
	m.yamlVisualCol = start
	m.yamlVisualCurCol = end
	return m, nil
}

func (m Model) handleYAMLVisualWordMotion(key string) (tea.Model, tea.Cmd) {
	m.yamlWordMotionStep(key)
	return m, nil
}

func (m *Model) yamlWordMotionStep(key string) {
	visLines, _ := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	if m.yamlCursor < 0 || m.yamlCursor >= len(visLines) {
		return
	}

	switch key {
	case "$":
		lineLen := len([]rune(visLines[m.yamlCursor]))
		if lineLen > 0 {
			m.yamlVisualCurCol = lineLen - 1
		}
	case "^":
		col := max(firstNonWhitespace(visLines[m.yamlCursor]), yamlFoldPrefixLen)
		m.yamlVisualCurCol = col
	case "w":
		m.yamlWordForward(visLines, nextWordStart)
	case "W":
		m.yamlWordForward(visLines, nextWORDStart)
	case "b":
		m.yamlWordBackward(visLines, prevWordStart)
	case "B":
		m.yamlWordBackward(visLines, prevWORDStart)
	case "e":
		m.yamlWordForward(visLines, wordEnd)
	case "E":
		m.yamlWordForward(visLines, WORDEnd)
	}
}

func (m *Model) yamlWordForward(visLines []string, motionFn func(string, int) int) {
	lineLen := len([]rune(visLines[m.yamlCursor]))
	newCol := motionFn(visLines[m.yamlCursor], m.yamlVisualCurCol)
	if newCol >= lineLen && m.yamlCursor < len(visLines)-1 {
		m.yamlCursor++
		newCol = motionFn(visLines[m.yamlCursor], 0)
		nextLineLen := len([]rune(visLines[m.yamlCursor]))
		if newCol >= nextLineLen {
			newCol = max(nextLineLen-1, 0)
		}
		m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
		m.ensureYAMLCursorVisible()
	} else {
		m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
	}
}

func (m *Model) yamlWordBackward(visLines []string, motionFn func(string, int) int) {
	newCol := motionFn(visLines[m.yamlCursor], m.yamlVisualCurCol)
	if newCol < 0 && m.yamlCursor > 0 {
		m.yamlCursor--
		lineLen := len([]rune(visLines[m.yamlCursor]))
		newCol = max(motionFn(visLines[m.yamlCursor], lineLen), 0)
		m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
		m.ensureYAMLCursorVisible()
	} else {
		m.yamlVisualCurCol = max(yamlFoldPrefixLen, max(newCol, 0))
	}
}
