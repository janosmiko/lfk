package app

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleYAMLVisualKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalVisible := visibleLineCount(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	maxScroll := m.yamlMaxScroll(totalVisible)

	key := msg.String()
	if op, motion, ok := m.consumeTextObjectPrelude(key); ok {
		return m.applyYAMLTextObject(op, motion)
	}
	switch key {
	case "esc":
		m.yamlView.visualMode = false
		return m, nil
	case "i", "a":
		// Clear any digit prefix accumulated before visual entry so it can't
		// leak into a later counted command via the post-visual normal mode.
		m.yamlView.lineInput = ""
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
		if m.yamlView.visualType == 'v' || m.yamlView.visualType == 'B' {
			if m.yamlView.visualCurCol > yamlFoldPrefixLen {
				m.yamlView.visualCurCol--
			}
		}
		return m, nil
	case "l", "right":
		if m.yamlView.visualType == 'v' || m.yamlView.visualType == 'B' {
			m.yamlView.visualCurCol++
		}
		return m, nil
	case "j", "down":
		if m.yamlView.cursor < totalVisible-1 {
			m.yamlView.cursor++
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "k", "up":
		if m.yamlView.cursor > 0 {
			m.yamlView.cursor--
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.yamlView.lineInput = ""
			m.yamlView.cursor = 0
			m.yamlView.scroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		return m.handleYAMLVisualG(totalVisible, maxScroll)
	case "ctrl+d", "shift+down":
		m.yamlView.cursor += scrollStep(m.yamlView.scrollOption, m.yamlViewportLines())
		if m.yamlView.cursor >= totalVisible {
			m.yamlView.cursor = totalVisible - 1
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+u", "shift+up":
		m.yamlView.cursor -= scrollStep(m.yamlView.scrollOption, m.yamlViewportLines())
		if m.yamlView.cursor < 0 {
			m.yamlView.cursor = 0
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+c":
		m.yamlView.visualMode = false
		m.mode = modeExplorer
		m.yamlView.scroll = 0
		m.yamlView.cursor = 0
		return m, nil
	case "0":
		m.yamlView.visualCurCol = yamlFoldPrefixLen
		return m, nil
	case "$", "w", "b", "e", "E", "B", "W", "^":
		return m.handleYAMLVisualWordMotion(msg.String())
	}
	return m, nil
}

func (m Model) handleYAMLVisualToggleMode(mode rune) (tea.Model, tea.Cmd) {
	if m.yamlView.visualType == mode {
		m.yamlView.visualMode = false
	} else {
		m.yamlView.visualType = mode
	}
	return m, nil
}

func (m Model) handleYAMLVisualG(totalVisible, maxScroll int) (tea.Model, tea.Cmd) {
	if m.yamlView.lineInput != "" {
		lineNum, _ := strconv.Atoi(m.yamlView.lineInput)
		m.yamlView.lineInput = ""
		if lineNum > 0 {
			lineNum--
		}
		m.yamlView.cursor = max(min(lineNum, totalVisible-1), 0)
		m.ensureYAMLCursorVisible()
	} else {
		m.yamlView.cursor = max(totalVisible-1, 0)
		m.yamlView.scroll = maxScroll
	}
	return m, nil
}

func (m Model) handleYAMLVisualCopy() (tea.Model, tea.Cmd) {
	_, mapping := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	selStart := min(m.yamlView.visualStart, m.yamlView.cursor)
	selEnd := max(m.yamlView.visualStart, m.yamlView.cursor)
	if selStart < 0 {
		selStart = 0
	}
	if selEnd >= len(mapping) {
		selEnd = len(mapping) - 1
	}
	origLines := strings.Split(m.yamlView.content, "\n")
	var clipText string
	switch m.yamlView.visualType {
	case 'v':
		clipText = m.yamlVisualCopyChar(selStart, selEnd, mapping, origLines)
	case 'B':
		clipText = m.yamlVisualCopyBlock(selStart, selEnd, mapping, origLines)
	default:
		clipText = m.yamlVisualCopyLine(selStart, selEnd, mapping, origLines)
	}
	lineCount := selEnd - selStart + 1
	visualType := m.yamlView.visualType
	m.yamlView.visualMode = false
	m.setStatusMessage(formatVisualYank(clipText, visualType, lineCount), false)
	return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
}

func (m Model) yamlVisualCopyChar(selStart, selEnd int, mapping []int, origLines []string) string {
	var parts []string
	anchorCol := m.yamlView.visualCol - yamlFoldPrefixLen
	cursorCol := m.yamlView.visualCurCol - yamlFoldPrefixLen
	startCol, endCol := anchorCol, cursorCol
	if m.yamlView.visualStart > m.yamlView.cursor {
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
	colStart := min(m.yamlView.visualCol, m.yamlView.visualCurCol) - yamlFoldPrefixLen
	colEnd := max(m.yamlView.visualCol, m.yamlView.visualCurCol) - yamlFoldPrefixLen + 1
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
	visLines, _ := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	if m.yamlView.cursor < 0 || m.yamlView.cursor >= len(visLines) {
		return m, nil
	}
	start, end, ok := textObjectRange(visLines[m.yamlView.cursor], m.yamlView.visualCurCol, op, motion)
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
	m.yamlView.visualType = 'v'
	m.yamlView.visualStart = m.yamlView.cursor
	m.yamlView.visualCol = start
	m.yamlView.visualCurCol = end
	return m, nil
}

func (m Model) handleYAMLVisualWordMotion(key string) (tea.Model, tea.Cmd) {
	m.yamlWordMotionStep(key)
	return m, nil
}

func (m *Model) yamlWordMotionStep(key string) {
	visLines, _ := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	if m.yamlView.cursor < 0 || m.yamlView.cursor >= len(visLines) {
		return
	}

	switch key {
	case "$":
		lineLen := len([]rune(visLines[m.yamlView.cursor]))
		if lineLen > 0 {
			m.yamlView.visualCurCol = lineLen - 1
		}
	case "^":
		col := max(firstNonWhitespace(visLines[m.yamlView.cursor]), yamlFoldPrefixLen)
		m.yamlView.visualCurCol = col
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
	lineLen := len([]rune(visLines[m.yamlView.cursor]))
	newCol := motionFn(visLines[m.yamlView.cursor], m.yamlView.visualCurCol)
	if newCol >= lineLen && m.yamlView.cursor < len(visLines)-1 {
		m.yamlView.cursor++
		newCol = motionFn(visLines[m.yamlView.cursor], 0)
		nextLineLen := len([]rune(visLines[m.yamlView.cursor]))
		if newCol >= nextLineLen {
			newCol = max(nextLineLen-1, 0)
		}
		m.yamlView.visualCurCol = max(yamlFoldPrefixLen, newCol)
		m.ensureYAMLCursorVisible()
	} else {
		m.yamlView.visualCurCol = max(yamlFoldPrefixLen, newCol)
	}
}

func (m *Model) yamlWordBackward(visLines []string, motionFn func(string, int) int) {
	newCol := motionFn(visLines[m.yamlView.cursor], m.yamlView.visualCurCol)
	if newCol < 0 && m.yamlView.cursor > 0 {
		m.yamlView.cursor--
		lineLen := len([]rune(visLines[m.yamlView.cursor]))
		newCol = max(motionFn(visLines[m.yamlView.cursor], lineLen), 0)
		m.yamlView.visualCurCol = max(yamlFoldPrefixLen, newCol)
		m.ensureYAMLCursorVisible()
	} else {
		m.yamlView.visualCurCol = max(yamlFoldPrefixLen, max(newCol, 0))
	}
}
