package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleDescribeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search input mode first.
	if m.describeSearchActive {
		return m.handleDescribeSearchKey(msg)
	}

	// Handle visual mode keys.
	if m.describeVisualMode != 0 {
		return m.handleDescribeVisualKey(msg)
	}

	return m.handleDescribeNormalKey(msg)
}

// handleDescribeNormalKey handles key events in normal describe view mode.
func (m Model) handleDescribeNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := strings.Split(m.describeContent, "\n")
	maxIdx := max(len(lines)-1, 0)
	key := msg.String()

	switch key {
	case "?", "f1":
		m.describeLineInput = ""
		m.helpPreviousMode = modeDescribe
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "Describe View"
		return m, nil
	case "ctrl+w", ">":
		m.describeLineInput = ""
		m.describeWrap = !m.describeWrap
		return m, nil
	case "q", "esc":
		return m.handleDescribeQuit()
	case "j", "down":
		m.describeLineInput = ""
		if m.describeCursor < maxIdx {
			m.describeCursor++
		}
		m.ensureDescribeCursorVisible()
		return m, nil
	case "k", "up":
		m.describeLineInput = ""
		if m.describeCursor > 0 {
			m.describeCursor--
		}
		m.ensureDescribeCursorVisible()
		return m, nil
	case "h", "left":
		m.describeLineInput = ""
		if m.describeCursorCol > 0 {
			m.describeCursorCol--
		}
		return m, nil
	case "l", "right":
		m.describeLineInput = ""
		m.describeCursorCol++
		return m, nil
	case "0":
		if m.describeLineInput != "" {
			m.describeLineInput += "0"
			return m, nil
		}
		m.describeCursorCol = 0
		return m, nil
	case "$", "^", "w", "W", "b", "B", "e", "E":
		m.describeLineInput = ""
		m.describeWordMotion(key, lines)
		return m, nil
	case "ctrl+d", "ctrl+u", "ctrl+f", "ctrl+b":
		return m.describePageMoveByKey(key, maxIdx)
	case "g":
		m.describeLineInput = ""
		if m.pendingG {
			m.pendingG = false
			m.describeCursor = 0
			m.ensureDescribeCursorVisible()
		} else {
			m.pendingG = true
		}
		return m, nil
	case "G":
		return m.handleDescribeG(maxIdx)
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.describeLineInput += key
		return m, nil
	case "v":
		return m.describeEnterVisual('v')
	case "V":
		return m.describeEnterVisual('V')
	case "ctrl+v":
		return m.describeEnterVisual('B')
	case "y":
		m.describeLineInput = ""
		if m.describeCursor >= 0 && m.describeCursor < len(lines) {
			text := lines[m.describeCursor]
			m.setStatusMessage("Copied 1 line", false)
			return m, tea.Batch(copyToSystemClipboard(text), scheduleStatusClear())
		}
		return m, nil
	case "/":
		m.describeLineInput = ""
		m.describeSearchActive = true
		m.describeSearchInput.Clear()
		return m, nil
	case "n":
		m.describeLineInput = ""
		m.findNextDescribeMatch(true)
		return m, nil
	case "N":
		m.describeLineInput = ""
		m.findNextDescribeMatch(false)
		return m, nil
	case "ctrl+c":
		m.describeLineInput = ""
		return m.closeTabOrQuit()
	default:
		m.describeLineInput = ""
	}
	return m, nil
}

// handleDescribeQuit handles quit/escape in describe view.
func (m Model) handleDescribeQuit() (tea.Model, tea.Cmd) {
	if m.describeSearchQuery != "" {
		m.describeSearchQuery = ""
		return m, nil
	}
	m.describeLineInput = ""
	m.mode = modeExplorer
	m.describeScroll = 0
	m.describeCursor = 0
	m.describeCursorCol = 0
	m.describeWrap = false
	m.describeAutoRefresh = false
	m.describeRefreshFunc = nil
	m.describeVisualMode = 0
	m.describeSearchQuery = ""
	m.describeSearchInput.Clear()
	return m, nil
}

// describeWordMotion applies a word/cursor motion in describe view.
func (m *Model) describeWordMotion(key string, lines []string) {
	if m.describeCursor < 0 || m.describeCursor >= len(lines) {
		return
	}
	line := lines[m.describeCursor]
	switch key {
	case "$":
		lineLen := len([]rune(line))
		if lineLen > 0 {
			m.describeCursorCol = lineLen - 1
		}
	case "^":
		m.describeCursorCol = firstNonWhitespace(line)
	case "w":
		m.describeCursorCol = nextWordStart(line, m.describeCursorCol)
	case "W":
		m.describeCursorCol = nextWORDStart(line, m.describeCursorCol)
	case "b":
		if nc := prevWordStart(line, m.describeCursorCol); nc >= 0 {
			m.describeCursorCol = nc
		}
	case "B":
		if nc := prevWORDStart(line, m.describeCursorCol); nc >= 0 {
			m.describeCursorCol = nc
		}
	case "e":
		m.describeCursorCol = wordEnd(line, m.describeCursorCol)
	case "E":
		m.describeCursorCol = WORDEnd(line, m.describeCursorCol)
	}
}

// describePageMoveByKey moves the cursor by a page amount based on the key pressed.
func (m Model) describePageMoveByKey(key string, maxIdx int) (tea.Model, tea.Cmd) {
	h := m.describeContentHeight()
	switch key {
	case "ctrl+d":
		return m.describePageMove(h/2, maxIdx)
	case "ctrl+u":
		return m.describePageMove(-h/2, maxIdx)
	case "ctrl+f":
		return m.describePageMove(h, maxIdx)
	case "ctrl+b":
		return m.describePageMove(-h, maxIdx)
	}
	return m, nil
}

// describePageMove moves the cursor by delta lines and clamps.
func (m Model) describePageMove(delta, maxIdx int) (tea.Model, tea.Cmd) {
	m.describeLineInput = ""
	m.describeCursor += delta
	if m.describeCursor > maxIdx {
		m.describeCursor = maxIdx
	}
	if m.describeCursor < 0 {
		m.describeCursor = 0
	}
	m.ensureDescribeCursorVisible()
	return m, nil
}

// handleDescribeG handles the G key (jump to line or end) in describe view.
func (m Model) handleDescribeG(maxIdx int) (tea.Model, tea.Cmd) {
	if m.describeLineInput != "" {
		lineNum, _ := strconv.Atoi(m.describeLineInput)
		m.describeLineInput = ""
		if lineNum > 0 {
			lineNum--
		}
		m.describeCursor = min(lineNum, maxIdx)
	} else {
		m.describeCursor = maxIdx
	}
	m.ensureDescribeCursorVisible()
	return m, nil
}

// describeEnterVisual enters visual selection mode in describe view.
func (m Model) describeEnterVisual(mode byte) (tea.Model, tea.Cmd) {
	m.describeLineInput = ""
	m.describeVisualMode = mode
	m.describeVisualStart = m.describeCursor
	m.describeVisualCol = m.describeCursorCol
	return m, nil
}

// handleDescribeVisualKey handles keys while visual mode is active in the describe view.
func (m Model) handleDescribeVisualKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := strings.Split(m.describeContent, "\n")
	maxIdx := max(len(lines)-1, 0)
	key := msg.String()

	switch key {
	case "esc":
		m.describeVisualMode = 0
		return m, nil
	case "V":
		return m.describeVisualToggle('V')
	case "v":
		return m.describeVisualToggle('v')
	case "ctrl+v":
		return m.describeVisualToggle('B')
	case "j", "down":
		if m.describeCursor < maxIdx {
			m.describeCursor++
		}
		m.ensureDescribeCursorVisible()
	case "k", "up":
		if m.describeCursor > 0 {
			m.describeCursor--
		}
		m.ensureDescribeCursorVisible()
	case "h", "left":
		if m.describeCursorCol > 0 {
			m.describeCursorCol--
		}
	case "l", "right":
		m.describeCursorCol++
	case "0":
		m.describeCursorCol = 0
	case "$", "^", "w", "W", "b", "B", "e", "E":
		m.describeWordMotion(key, lines)
	case "G":
		m.describeCursor = maxIdx
		m.ensureDescribeCursorVisible()
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.describeCursor = 0
			m.ensureDescribeCursorVisible()
		} else {
			m.pendingG = true
		}
	case "ctrl+d":
		m.describeCursor += m.describeContentHeight() / 2
		if m.describeCursor > maxIdx {
			m.describeCursor = maxIdx
		}
		m.ensureDescribeCursorVisible()
	case "ctrl+u":
		m.describeCursor -= m.describeContentHeight() / 2
		if m.describeCursor < 0 {
			m.describeCursor = 0
		}
		m.ensureDescribeCursorVisible()
	case "y":
		return m.describeVisualCopy(lines)
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// describeVisualToggle toggles the visual selection mode in describe view.
func (m Model) describeVisualToggle(mode byte) (tea.Model, tea.Cmd) {
	if m.describeVisualMode == mode {
		m.describeVisualMode = 0
	} else {
		m.describeVisualMode = mode
	}
	return m, nil
}

// describeVisualCopy copies the visually selected text in describe view.
func (m Model) describeVisualCopy(lines []string) (tea.Model, tea.Cmd) {
	selStart := min(m.describeVisualStart, m.describeCursor)
	selEnd := max(m.describeVisualStart, m.describeCursor)
	if selStart < 0 {
		selStart = 0
	}
	if selEnd >= len(lines) {
		selEnd = len(lines) - 1
	}
	clipText := visualCopyText(lines, selStart, selEnd,
		rune(m.describeVisualMode), m.describeVisualCol, m.describeCursorCol,
		m.describeVisualStart > m.describeCursor)
	lineCount := selEnd - selStart + 1
	m.describeVisualMode = 0
	m.setStatusMessage(fmt.Sprintf("Copied %d line(s)", lineCount), false)
	return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
}

// visualCopyText extracts text from lines based on visual selection mode.
// This is shared between describe and diff visual copy.
func visualCopyText(lines []string, selStart, selEnd int, mode rune, anchorCol, cursorCol int, reversed bool) string {
	switch mode {
	case 'v':
		return visualCopyChar(lines, selStart, selEnd, anchorCol, cursorCol, reversed)
	case 'B':
		return visualCopyBlock(lines, selStart, selEnd, anchorCol, cursorCol)
	default:
		var parts []string
		for i := selStart; i <= selEnd; i++ {
			parts = append(parts, lines[i])
		}
		return strings.Join(parts, "\n")
	}
}

// visualCopyChar extracts character-mode visual selection text.
func visualCopyChar(lines []string, selStart, selEnd, anchorCol, cursorCol int, reversed bool) string {
	var parts []string
	startCol, endCol := anchorCol, cursorCol
	if reversed {
		startCol, endCol = cursorCol, anchorCol
	}
	for i := selStart; i <= selEnd; i++ {
		line := lines[i]
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
	return strings.Join(parts, "\n")
}

// visualCopyBlock extracts block-mode visual selection text.
func visualCopyBlock(lines []string, selStart, selEnd, col1, col2 int) string {
	colStart := min(col1, col2)
	colEnd := max(col1, col2) + 1
	var parts []string
	for i := selStart; i <= selEnd; i++ {
		line := lines[i]
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

// handleDescribeSearchKey handles keyboard input during describe search.
func (m Model) handleDescribeSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.describeSearchActive = false
		m.describeSearchQuery = m.describeSearchInput.Value
		m.findNextDescribeMatch(true)
	case "esc":
		m.describeSearchActive = false
		m.describeSearchInput.Clear()
	case "backspace":
		if len(m.describeSearchInput.Value) > 0 {
			m.describeSearchInput.Backspace()
		}
	case "ctrl+w":
		m.describeSearchInput.DeleteWord()
	case "ctrl+a":
		m.describeSearchInput.Home()
	case "ctrl+e":
		m.describeSearchInput.End()
	case "left":
		m.describeSearchInput.Left()
	case "right":
		m.describeSearchInput.Right()
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.describeSearchInput.Insert(key)
		}
	}
	return m, nil
}

// describeContentHeight returns the visible content height for the describe view.
func (m *Model) describeContentHeight() int {
	h := m.height - 4
	if h < 3 {
		h = 3
	}
	return h
}

// ensureDescribeCursorVisible adjusts describeScroll so the cursor is within
// the viewport with scrolloff padding.
func (m *Model) ensureDescribeCursorVisible() {
	lines := strings.Split(m.describeContent, "\n")
	total := len(lines)
	if m.describeCursor >= total {
		m.describeCursor = total - 1
	}
	if m.describeCursor < 0 {
		m.describeCursor = 0
	}
	viewH := m.describeContentHeight()
	so := ui.ConfigScrollOff
	if so > viewH/2 {
		so = viewH / 2
	}
	if m.describeCursor < m.describeScroll+so {
		m.describeScroll = m.describeCursor - so
	}
	if m.describeCursor >= m.describeScroll+viewH-so {
		m.describeScroll = m.describeCursor - viewH + so + 1
	}
	if m.describeScroll < 0 {
		m.describeScroll = 0
	}
	maxScroll := max(total-viewH, 0)
	if m.describeScroll > maxScroll {
		m.describeScroll = maxScroll
	}
}

// findNextDescribeMatch searches for the next/previous occurrence of the search
// query in the describe content lines and moves the cursor to it.
func (m *Model) findNextDescribeMatch(forward bool) {
	if m.describeSearchQuery == "" {
		return
	}
	lines := strings.Split(m.describeContent, "\n")
	if len(lines) == 0 {
		return
	}
	query := strings.ToLower(m.describeSearchQuery)
	start := m.describeCursor
	total := len(lines)

	for i := 1; i <= total; i++ {
		var idx int
		if forward {
			idx = (start + i) % total
		} else {
			idx = (start - i + total) % total
		}
		if strings.Contains(strings.ToLower(lines[idx]), query) {
			m.describeCursor = idx
			m.ensureDescribeCursorVisible()
			return
		}
	}
	m.setStatusMessage("Pattern not found: "+m.describeSearchQuery, false)
}

func (m Model) handleDiffKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	foldRegions := ui.ComputeDiffFoldRegions(m.diffLeft, m.diffRight)
	m.ensureDiffFoldState(foldRegions)

	totalLines, visibleLines, maxScroll := m.diffViewMetrics(foldRegions)

	// When in search input mode, handle text input first.
	if m.diffSearchMode {
		return m.handleDiffSearchInput(msg, foldRegions, visibleLines)
	}

	// In visual selection mode, delegate to the visual key handler.
	if m.diffVisualMode {
		return m.handleDiffVisualKey(msg, foldRegions, totalLines, visibleLines, maxScroll)
	}

	return m.handleDiffNormalKey(msg, foldRegions, totalLines, visibleLines, maxScroll)
}

// diffViewMetrics computes the total lines, visible lines, and max scroll for the diff view.
func (m Model) diffViewMetrics(foldRegions []ui.DiffFoldRegion) (totalLines, visibleLines, maxScroll int) {
	totalLines = ui.DiffViewTotalLines(m.diffLeft, m.diffRight, foldRegions, m.diffFoldState)
	overhead := 1
	if len(m.tabs) > 1 {
		overhead++
	}
	visibleLines = m.height - overhead - 6
	if m.diffUnified {
		totalLines = ui.UnifiedDiffViewTotalLines(m.diffLeft, m.diffRight, foldRegions, m.diffFoldState)
		visibleLines = m.height - overhead - 6
	}
	if visibleLines < 3 {
		visibleLines = 3
	}
	maxScroll = totalLines - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	return totalLines, visibleLines, maxScroll
}

// handleDiffSearchInput handles key events in diff search input mode.
func (m Model) handleDiffSearchInput(msg tea.KeyMsg, foldRegions []ui.DiffFoldRegion, visibleLines int) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.diffSearchMode = false
		m.diffSearchQuery = m.diffSearchText.Value
		m.diffMatchLines = ui.UpdateDiffSearchMatches(m.diffLeft, m.diffRight, m.diffSearchQuery, m.diffCursorSide, m.diffUnified)
		if len(m.diffMatchLines) > 0 {
			m.diffMatchIdx = 0
			m.diffScrollToMatch(foldRegions, visibleLines)
		}
		return m, nil
	case "esc":
		m.diffSearchMode = false
		m.diffSearchText.Clear()
		m.diffSearchQuery = ""
		m.diffMatchLines = nil
		m.diffMatchIdx = 0
		return m, nil
	case "backspace":
		if len(m.diffSearchText.Value) > 0 {
			m.diffSearchText.Backspace()
		}
		return m, nil
	case "ctrl+w":
		m.diffSearchText.DeleteWord()
		return m, nil
	case "ctrl+a":
		m.diffSearchText.Home()
		return m, nil
	case "ctrl+e":
		m.diffSearchText.End()
		return m, nil
	case "left":
		m.diffSearchText.Left()
		return m, nil
	case "right":
		m.diffSearchText.Right()
		return m, nil
	case "ctrl+c":
		m.diffSearchMode = false
		m.diffSearchText.Clear()
		m.diffMatchLines = nil
		return m, nil
	default:
		if len(msg.String()) == 1 || msg.String() == " " {
			m.diffSearchText.Insert(msg.String())
		}
		return m, nil
	}
}

// handleDiffNormalKey handles key events in normal diff view mode.
func (m Model) handleDiffNormalKey(msg tea.KeyMsg, foldRegions []ui.DiffFoldRegion, totalLines, visibleLines, maxScroll int) (tea.Model, tea.Cmd) {
	maxCursor := max(totalLines-1, 0)

	switch msg.String() {
	case "?", "f1":
		m.helpPreviousMode = modeDiff
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "Diff View"
		return m, nil
	case "ctrl+w", ">":
		m.diffWrap = !m.diffWrap
		return m, nil
	case "q", "esc":
		return m.handleDiffQuit()
	case "j", "down":
		m.diffLineInput = ""
		if m.diffCursor < maxCursor {
			m.diffCursor++
		}
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "k", "up":
		m.diffLineInput = ""
		if m.diffCursor > 0 {
			m.diffCursor--
		}
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "h", "left":
		m.diffLineInput = ""
		if m.diffVisualCurCol > 0 {
			m.diffVisualCurCol--
		}
		return m, nil
	case "l", "right":
		m.diffLineInput = ""
		m.diffVisualCurCol++
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.diffLineInput = ""
			m.diffCursor = 0
			m.diffScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		return m.handleDiffG(maxCursor, visibleLines, maxScroll)
	case "ctrl+d", "ctrl+u", "ctrl+f", "ctrl+b":
		return m.diffPageMoveByKey(msg.String(), maxCursor, visibleLines, maxScroll)
	case "0":
		if m.diffLineInput != "" {
			m.diffLineInput += "0"
		} else {
			m.diffVisualCurCol = 0
		}
		return m, nil
	case "$", "^", "w", "b", "e", "E", "W", "B":
		m.diffLineInput = ""
		m.diffWordMotion(msg.String(), foldRegions)
		return m, nil
	case "v", "V", "ctrl+v":
		modeMap := map[string]rune{"v": 'v', "V": 'V', "ctrl+v": 'B'}
		return m.diffEnterVisual(modeMap[msg.String()])
	case "u":
		m.diffLineInput = ""
		m.diffUnified = !m.diffUnified
		m.diffScroll = 0
		return m, nil
	case "#":
		m.diffLineInput = ""
		m.diffLineNumbers = !m.diffLineNumbers
		return m, nil
	case "/":
		m.diffLineInput = ""
		m.diffSearchMode = true
		m.diffSearchText.Clear()
		m.diffMatchLines = nil
		m.diffMatchIdx = 0
		return m, nil
	case "n", "N":
		return m.handleDiffSearchNav(msg.String(), foldRegions, visibleLines)
	case "tab":
		if !m.diffUnified {
			m.diffCursorSide = 1 - m.diffCursorSide
		}
		return m, nil
	case "z", "Z":
		m.diffLineInput = ""
		if msg.String() == "Z" {
			m.toggleAllDiffFolds(foldRegions)
		} else {
			m.toggleDiffFoldAtCursor(foldRegions)
		}
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.diffLineInput += msg.String()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		m.diffLineInput = ""
	}
	return m, nil
}

// handleDiffQuit handles quit/escape in diff view.
func (m Model) handleDiffQuit() (tea.Model, tea.Cmd) {
	m.mode = modeExplorer
	m.diffScroll = 0
	m.diffCursor = 0
	m.diffCursorSide = 0
	m.diffLineInput = ""
	m.diffWrap = false
	m.diffSearchQuery = ""
	m.diffSearchText.Clear()
	m.diffMatchLines = nil
	m.diffMatchIdx = 0
	m.diffFoldState = nil
	m.diffVisualMode = false
	m.diffVisualCurCol = 0
	return m, nil
}

// handleDiffG handles the G key (jump to line or end) in diff view.
func (m Model) handleDiffG(maxCursor, visibleLines, maxScroll int) (tea.Model, tea.Cmd) {
	if m.diffLineInput != "" {
		lineNum, _ := strconv.Atoi(m.diffLineInput)
		m.diffLineInput = ""
		if lineNum > 0 {
			lineNum--
		}
		m.diffCursor = min(lineNum, maxCursor)
	} else {
		m.diffCursor = maxCursor
	}
	m.ensureDiffCursorVisible(visibleLines, maxScroll)
	return m, nil
}

// diffWordMotion applies a word/cursor motion in diff view.
func (m *Model) diffWordMotion(key string, foldRegions []ui.DiffFoldRegion) {
	lineText := m.diffCurrentLineText(foldRegions)
	switch key {
	case "$":
		lineLen := len([]rune(lineText))
		if lineLen > 0 {
			m.diffVisualCurCol = lineLen - 1
		}
	case "^":
		m.diffVisualCurCol = firstNonWhitespace(lineText)
	case "w":
		if lineText != "" {
			m.diffVisualCurCol = diffClampCol(nextWordStart(lineText, m.diffVisualCurCol), lineText)
		}
	case "W":
		if lineText != "" {
			m.diffVisualCurCol = diffClampCol(nextWORDStart(lineText, m.diffVisualCurCol), lineText)
		}
	case "b":
		if lineText != "" {
			newCol := prevWordStart(lineText, m.diffVisualCurCol)
			if newCol < 0 {
				newCol = 0
			}
			m.diffVisualCurCol = newCol
		}
	case "B":
		if lineText != "" {
			newCol := prevWORDStart(lineText, m.diffVisualCurCol)
			if newCol < 0 {
				newCol = 0
			}
			m.diffVisualCurCol = newCol
		}
	case "e":
		if lineText != "" {
			m.diffVisualCurCol = diffClampCol(wordEnd(lineText, m.diffVisualCurCol), lineText)
		}
	case "E":
		if lineText != "" {
			m.diffVisualCurCol = diffClampCol(WORDEnd(lineText, m.diffVisualCurCol), lineText)
		}
	}
}

// handleDiffSearchNav handles n/N (next/prev search match) in diff view.
func (m Model) handleDiffSearchNav(key string, foldRegions []ui.DiffFoldRegion, visibleLines int) (tea.Model, tea.Cmd) {
	m.diffLineInput = ""
	if len(m.diffMatchLines) > 0 {
		if key == "n" {
			m.diffMatchIdx = (m.diffMatchIdx + 1) % len(m.diffMatchLines)
		} else {
			m.diffMatchIdx = (m.diffMatchIdx - 1 + len(m.diffMatchLines)) % len(m.diffMatchLines)
		}
		m.diffScrollToMatch(foldRegions, visibleLines)
	}
	return m, nil
}

// diffPageMoveByKey moves the diff cursor by a page amount based on the key pressed.
func (m Model) diffPageMoveByKey(key string, maxCursor, visibleLines, maxScroll int) (tea.Model, tea.Cmd) {
	m.diffLineInput = ""
	switch key {
	case "ctrl+d":
		m.diffCursor = min(m.diffCursor+m.height/2, maxCursor)
	case "ctrl+u":
		m.diffCursor = max(m.diffCursor-m.height/2, 0)
	case "ctrl+f":
		m.diffCursor = min(m.diffCursor+m.height, maxCursor)
	case "ctrl+b":
		m.diffCursor = max(m.diffCursor-m.height, 0)
	}
	m.ensureDiffCursorVisible(visibleLines, maxScroll)
	return m, nil
}

// diffClampCol clamps a column to the end of a line.
func diffClampCol(col int, lineText string) int {
	lineLen := len([]rune(lineText))
	if col >= lineLen {
		return max(lineLen-1, 0)
	}
	return col
}

// diffEnterVisual enters visual selection mode in diff view.
func (m Model) diffEnterVisual(mode rune) (tea.Model, tea.Cmd) {
	m.diffVisualMode = true
	m.diffVisualType = mode
	m.diffVisualStart = m.diffCursor
	m.diffVisualCol = m.diffVisualCurCol
	return m, nil
}

// diffCurrentLineText returns the plain text of the current diff line on the active side.
func (m *Model) diffCurrentLineText(foldRegions []ui.DiffFoldRegion) string {
	return ui.DiffLineTextAt(m.diffLeft, m.diffRight, foldRegions, m.diffFoldState, m.diffCursor, m.diffCursorSide, m.diffUnified)
}

// handleDiffVisualKey handles key events while in diff visual selection mode.
func (m Model) handleDiffVisualKey(msg tea.KeyMsg, foldRegions []ui.DiffFoldRegion, totalLines, visibleLines, maxScroll int) (tea.Model, tea.Cmd) {
	maxCursor := max(totalLines-1, 0)

	switch msg.String() {
	case "esc":
		m.diffVisualMode = false
		return m, nil
	case "V":
		return m.diffVisualToggle('V')
	case "v":
		return m.diffVisualToggle('v')
	case "ctrl+v":
		return m.diffVisualToggle('B')
	case "y":
		return m.diffVisualCopy(foldRegions)
	case "j", "down":
		if m.diffCursor < maxCursor {
			m.diffCursor++
		}
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "k", "up":
		if m.diffCursor > 0 {
			m.diffCursor--
		}
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "h", "left":
		if m.diffVisualType == 'v' || m.diffVisualType == 'B' {
			if m.diffVisualCurCol > 0 {
				m.diffVisualCurCol--
			}
		}
		return m, nil
	case "l", "right":
		if m.diffVisualType == 'v' || m.diffVisualType == 'B' {
			m.diffVisualCurCol++
		}
		return m, nil
	case "0":
		m.diffVisualCurCol = 0
		return m, nil
	case "$", "^", "w", "b", "e", "E", "W", "B":
		m.diffWordMotion(msg.String(), foldRegions)
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.diffCursor = 0
			m.diffScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		m.diffCursor = maxCursor
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+d":
		m.diffCursor = min(m.diffCursor+m.height/2, maxCursor)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+u":
		m.diffCursor = max(m.diffCursor-m.height/2, 0)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+f":
		m.diffCursor = min(m.diffCursor+m.height, maxCursor)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+b":
		m.diffCursor = max(m.diffCursor-m.height, 0)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+c":
		m.diffVisualMode = false
		return m.closeTabOrQuit()
	}
	return m, nil
}

// diffVisualToggle toggles the visual selection type in diff view.
func (m Model) diffVisualToggle(mode rune) (tea.Model, tea.Cmd) {
	if m.diffVisualType == mode {
		m.diffVisualMode = false
	} else {
		m.diffVisualType = mode
	}
	return m, nil
}

// diffVisualCopy copies the visually selected diff text to the clipboard.
func (m Model) diffVisualCopy(foldRegions []ui.DiffFoldRegion) (tea.Model, tea.Cmd) {
	selStart := min(m.diffVisualStart, m.diffCursor)
	selEnd := max(m.diffVisualStart, m.diffCursor)

	// Collect lines, skipping empty-side lines.
	var diffLines []string
	for i := selStart; i <= selEnd; i++ {
		lineText := ui.DiffLineTextAt(m.diffLeft, m.diffRight, foldRegions, m.diffFoldState, i, m.diffCursorSide, m.diffUnified)
		if lineText != "" {
			diffLines = append(diffLines, lineText)
		}
	}
	clipText := visualCopyText(diffLines, 0, len(diffLines)-1,
		m.diffVisualType, m.diffVisualCol, m.diffVisualCurCol,
		m.diffVisualStart > m.diffCursor)
	lineCount := selEnd - selStart + 1
	m.diffVisualMode = false
	m.setStatusMessage(fmt.Sprintf("Copied %d lines", lineCount), false)
	return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
}

// ensureDiffFoldState ensures the fold state slice has the correct length for
// the current fold regions.
func (m *Model) ensureDiffFoldState(regions []ui.DiffFoldRegion) {
	if len(m.diffFoldState) < len(regions) {
		newState := make([]bool, len(regions))
		copy(newState, m.diffFoldState)
		m.diffFoldState = newState
	}
}

// ensureDiffCursorVisible adjusts diffScroll so the cursor is within the viewport.
func (m *Model) ensureDiffCursorVisible(viewportLines, maxScroll int) {
	so := ui.ConfigScrollOff
	if so > viewportLines/2 {
		so = viewportLines / 2
	}
	if m.diffCursor < m.diffScroll+so {
		m.diffScroll = m.diffCursor - so
	}
	if m.diffCursor >= m.diffScroll+viewportLines-so {
		m.diffScroll = m.diffCursor - viewportLines + so + 1
	}
	m.diffScroll = max(min(m.diffScroll, maxScroll), 0)
}

// diffScrollToMatch auto-expands the fold region containing the current match,
// scrolls to center it in the viewport, and moves the cursor column to the match.
func (m *Model) diffScrollToMatch(foldRegions []ui.DiffFoldRegion, viewportLines int) {
	if len(m.diffMatchLines) == 0 || m.diffMatchIdx < 0 || m.diffMatchIdx >= len(m.diffMatchLines) {
		return
	}
	origIdx := m.diffMatchLines[m.diffMatchIdx]

	// Auto-expand any collapsed fold region containing this match.
	ui.ExpandDiffFoldForLine(foldRegions, m.diffFoldState, origIdx)

	// Find the visible index for this original line.
	visIdx := ui.DiffVisibleIndexForOriginal(m.diffLeft, m.diffRight, foldRegions, m.diffFoldState, origIdx)
	if visIdx < 0 {
		return
	}

	// Move cursor line and center in viewport.
	m.diffCursor = visIdx
	m.diffScroll = visIdx - viewportLines/2
	if m.diffScroll < 0 {
		m.diffScroll = 0
	}

	// Move cursor column to the match position on the active side.
	lineText := m.diffCurrentLineText(foldRegions)
	col := ui.DiffSearchColumnInLine(lineText, m.diffSearchQuery)
	if col >= 0 {
		m.diffVisualCurCol = col
	}
}

// toggleDiffFoldAtCursor toggles the fold on the unchanged section at the cursor.
// When collapsing, moves the cursor to the fold placeholder line.
func (m *Model) toggleDiffFoldAtCursor(foldRegions []ui.DiffFoldRegion) {
	rawDiffLines := ui.ComputeDiffLines(m.diffLeft, m.diffRight)
	visLines := ui.BuildVisibleDiffLines(rawDiffLines, foldRegions, m.diffFoldState)

	idx := m.diffCursor
	if idx >= len(visLines) {
		idx = len(visLines) - 1
	}
	if idx < 0 {
		return
	}

	vl := visLines[idx]
	if vl.RegionIdx < 0 || vl.RegionIdx >= len(m.diffFoldState) {
		return
	}

	wasCollapsed := m.diffFoldState[vl.RegionIdx]
	m.diffFoldState[vl.RegionIdx] = !wasCollapsed

	// When collapsing, reposition cursor to the fold placeholder.
	if !wasCollapsed {
		newVisLines := ui.BuildVisibleDiffLines(rawDiffLines, foldRegions, m.diffFoldState)
		for i, nvl := range newVisLines {
			if nvl.IsFoldPlaceholder && nvl.RegionIdx == vl.RegionIdx {
				m.diffCursor = i
				break
			}
		}
	}
}

// toggleAllDiffFolds toggles all fold regions at once. If any are collapsed,
// expand all; otherwise collapse all.
func (m *Model) toggleAllDiffFolds(foldRegions []ui.DiffFoldRegion) {
	anyCollapsed := false
	for i := range foldRegions {
		if i < len(m.diffFoldState) && m.diffFoldState[i] {
			anyCollapsed = true
			break
		}
	}
	for i := range foldRegions {
		if i < len(m.diffFoldState) {
			m.diffFoldState[i] = !anyCollapsed
		}
	}
}
