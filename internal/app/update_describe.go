package app

import (
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
//
//nolint:gocyclo // switch-based key dispatch is inherently high-complexity
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
		n := consumeCountPrefix(&m.describeLineInput)
		m.describeCursor = min(m.describeCursor+n, maxIdx)
		m.ensureDescribeCursorVisible()
		return m, nil
	case "k", "up":
		n := consumeCountPrefix(&m.describeLineInput)
		m.describeCursor = max(m.describeCursor-n, 0)
		m.ensureDescribeCursorVisible()
		return m, nil
	case "h", "left":
		n := consumeCountPrefix(&m.describeLineInput)
		m.describeCursorCol = max(m.describeCursorCol-n, 0)
		return m, nil
	case "l", "right":
		n := consumeCountPrefix(&m.describeLineInput)
		m.describeCursorCol += n
		return m, nil
	case "0":
		if m.describeLineInput != "" {
			m.describeLineInput += "0"
			return m, nil
		}
		m.describeCursorCol = 0
		return m, nil
	case "$", "^":
		// Absolute-position motions ignore counts but still consume the
		// buffer so a stray digit prefix doesn't leak forward.
		consumeCountPrefix(&m.describeLineInput)
		m.describeWordMotion(key, lines)
		return m, nil
	case "w", "W", "b", "B", "e", "E":
		count := consumeCountPrefix(&m.describeLineInput)
		for range count {
			m.describeWordMotion(key, lines)
		}
		return m, nil
	case "ctrl+d":
		step := vimScrollStep(&m.describeLineInput, &m.describeScrollOption, m.describeContentHeight())
		return m.describePageMove(step, maxIdx)
	case "ctrl+u":
		step := vimScrollStep(&m.describeLineInput, &m.describeScrollOption, m.describeContentHeight())
		return m.describePageMove(-step, maxIdx)
	case "ctrl+f", "pgdown":
		count := consumeCountPrefix(&m.describeLineInput)
		return m.describePageMove(count*m.describeContentHeight(), maxIdx)
	case "ctrl+b", "pgup":
		count := consumeCountPrefix(&m.describeLineInput)
		return m.describePageMove(-count*m.describeContentHeight(), maxIdx)
	case "home":
		m.describeLineInput = ""
		m.pendingG = false
		m.describeCursor = 0
		m.ensureDescribeCursorVisible()
		return m, nil
	case "end":
		m.describeLineInput = ""
		m.describeCursor = maxIdx
		m.ensureDescribeCursorVisible()
		return m, nil
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
		n := consumeCountPrefix(&m.describeLineInput)
		if m.describeCursor < 0 || m.describeCursor >= len(lines) {
			return m, nil
		}
		end := min(m.describeCursor+n, len(lines))
		text := strings.Join(lines[m.describeCursor:end], "\n")
		m.setStatusMessage(formatCopiedLines(end-m.describeCursor), false)
		return m, tea.Batch(copyToSystemClipboard(text), scheduleStatusClear())
	case "/":
		m.describeLineInput = ""
		m.describeSearchActive = true
		m.describeSearchInput.Clear()
		return m, nil
	case "n":
		count := consumeCountPrefix(&m.describeLineInput)
		for range count {
			m.findNextDescribeMatch(true)
		}
		return m, nil
	case "N":
		count := consumeCountPrefix(&m.describeLineInput)
		for range count {
			m.findNextDescribeMatch(false)
		}
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

	if op, motion, ok := m.consumeTextObjectPrelude(key); ok {
		return m.applyDescribeTextObject(op, motion, lines)
	}

	switch key {
	case "esc":
		m.describeVisualMode = 0
		return m, nil
	case "i", "a":
		// Clear any digit prefix accumulated before visual entry so it can't
		// leak into a later counted command via the post-visual normal mode.
		m.describeLineInput = ""
		m.pendingTextObject = key[0]
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
		m.describeCursor += scrollStep(m.describeScrollOption, m.describeContentHeight())
		if m.describeCursor > maxIdx {
			m.describeCursor = maxIdx
		}
		m.ensureDescribeCursorVisible()
	case "ctrl+u":
		m.describeCursor -= scrollStep(m.describeScrollOption, m.describeContentHeight())
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

// applyDescribeTextObject resolves an `iw`/`aw`/`iW`/`aW` text object on the
// describe line under the cursor and switches the visual selection to
// character mode covering the resulting range.
func (m Model) applyDescribeTextObject(op byte, motion string, lines []string) (tea.Model, tea.Cmd) {
	if m.describeCursor < 0 || m.describeCursor >= len(lines) {
		return m, nil
	}
	start, end, ok := textObjectRange(lines[m.describeCursor], m.describeCursorCol, op, motion)
	if !ok {
		return m, nil
	}
	m.describeVisualMode = 'v'
	m.describeVisualStart = m.describeCursor
	m.describeVisualCol = start
	m.describeCursorCol = end
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
	visualType := rune(m.describeVisualMode)
	clipText := visualCopyText(lines, selStart, selEnd,
		visualType, m.describeVisualCol, m.describeCursorCol,
		m.describeVisualStart > m.describeCursor)
	lineCount := selEnd - selStart + 1
	m.describeVisualMode = 0
	m.setStatusMessage(formatVisualYank(clipText, visualType, lineCount), false)
	return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
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
		m.describeSearchQuery = ""
	case "backspace":
		if len(m.describeSearchInput.Value) > 0 {
			m.describeSearchInput.Backspace()
		}
		m.describeSearchQuery = m.describeSearchInput.Value
	case "ctrl+w":
		m.describeSearchInput.DeleteWord()
		m.describeSearchQuery = m.describeSearchInput.Value
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
			// Live-update the highlight query so matches paint as the
			// user types instead of waiting for Enter to commit.
			m.describeSearchQuery = m.describeSearchInput.Value
		}
	}
	return m, nil
}

// describeContentHeight returns the visible content height for the describe view.
func (m *Model) describeContentHeight() int {
	h := max(m.height-4, 3)
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
	so := min(ui.ConfigScrollOff, viewH/2)
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
