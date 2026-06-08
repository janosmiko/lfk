package app

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleDescribeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search input mode first.
	if m.describeView.searchActive {
		return m.handleDescribeSearchKey(msg)
	}

	// Handle visual mode keys.
	if m.describeView.visualMode != 0 {
		return m.handleDescribeVisualKey(msg)
	}

	return m.handleDescribeNormalKey(msg)
}

// handleDescribeNormalKey handles key events in normal describe view mode.
//
//nolint:gocyclo // switch-based key dispatch is inherently high-complexity
func (m Model) handleDescribeNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := strings.Split(m.describeView.content, "\n")
	maxIdx := max(len(lines)-1, 0)
	key := msg.String()

	switch key {
	case "?", "f1":
		m.describeView.lineInput = ""
		m.helpPreviousMode = modeDescribe
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "Describe View"
		return m, nil
	case ui.ActiveKeybindings.ToggleWrap:
		m.describeView.lineInput = ""
		m.describeView.wrap = !m.describeView.wrap
		return m, nil
	case "q", "esc":
		return m.handleDescribeQuit()
	case "j", "down":
		n := consumeCountPrefix(&m.describeView.lineInput)
		m.describeView.cursor = min(m.describeView.cursor+n, maxIdx)
		m.ensureDescribeCursorVisible()
		return m, nil
	case "k", "up":
		n := consumeCountPrefix(&m.describeView.lineInput)
		m.describeView.cursor = max(m.describeView.cursor-n, 0)
		m.ensureDescribeCursorVisible()
		return m, nil
	case "h", "left":
		n := consumeCountPrefix(&m.describeView.lineInput)
		m.describeView.cursorCol = max(m.describeView.cursorCol-n, 0)
		return m, nil
	case "l", "right":
		n := consumeCountPrefix(&m.describeView.lineInput)
		m.describeView.cursorCol += n
		return m, nil
	case "0":
		if m.describeView.lineInput != "" {
			m.describeView.lineInput += "0"
			return m, nil
		}
		m.describeView.cursorCol = 0
		return m, nil
	case "$", "^":
		// Absolute-position motions ignore counts but still consume the
		// buffer so a stray digit prefix doesn't leak forward.
		consumeCountPrefix(&m.describeView.lineInput)
		m.describeWordMotion(key, lines)
		return m, nil
	case "w", "W", "b", "B", "e", "E":
		count := consumeCountPrefix(&m.describeView.lineInput)
		for range count {
			m.describeWordMotion(key, lines)
		}
		return m, nil
	case "ctrl+d", "shift+down":
		step := vimScrollStep(&m.describeView.lineInput, &m.describeView.scrollOption, m.describeContentHeight())
		return m.describePageMove(step, maxIdx)
	case "ctrl+u", "shift+up":
		step := vimScrollStep(&m.describeView.lineInput, &m.describeView.scrollOption, m.describeContentHeight())
		return m.describePageMove(-step, maxIdx)
	case "ctrl+f", "pgdown":
		count := consumeCountPrefix(&m.describeView.lineInput)
		return m.describePageMove(count*m.describeContentHeight(), maxIdx)
	case "ctrl+b", "pgup":
		count := consumeCountPrefix(&m.describeView.lineInput)
		return m.describePageMove(-count*m.describeContentHeight(), maxIdx)
	case "home":
		m.describeView.lineInput = ""
		m.pendingG = false
		m.describeView.cursor = 0
		m.ensureDescribeCursorVisible()
		return m, nil
	case "end":
		m.describeView.lineInput = ""
		m.describeView.cursor = maxIdx
		m.ensureDescribeCursorVisible()
		return m, nil
	case "g":
		m.describeView.lineInput = ""
		if m.pendingG {
			m.pendingG = false
			m.describeView.cursor = 0
			m.ensureDescribeCursorVisible()
		} else {
			m.pendingG = true
		}
		return m, nil
	case "G":
		return m.handleDescribeG(maxIdx)
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.describeView.lineInput += key
		return m, nil
	case "v":
		return m.describeEnterVisual('v')
	case "V":
		return m.describeEnterVisual('V')
	case "ctrl+v":
		return m.describeEnterVisual('B')
	case "y":
		n := consumeCountPrefix(&m.describeView.lineInput)
		if m.describeView.cursor < 0 || m.describeView.cursor >= len(lines) {
			return m, nil
		}
		end := min(m.describeView.cursor+n, len(lines))
		text := strings.Join(lines[m.describeView.cursor:end], "\n")
		m.setStatusMessage(formatCopiedLines(end-m.describeView.cursor), false)
		return m, tea.Batch(copyToSystemClipboard(text), scheduleStatusClear())
	case "/":
		m.describeView.lineInput = ""
		m.describeView.searchActive = true
		m.describeView.searchInput.Clear()
		return m, nil
	case "n":
		count := consumeCountPrefix(&m.describeView.lineInput)
		for range count {
			m.findNextDescribeMatch(true)
		}
		return m, nil
	case "N":
		count := consumeCountPrefix(&m.describeView.lineInput)
		for range count {
			m.findNextDescribeMatch(false)
		}
		return m, nil
	case "ctrl+c":
		m.describeView.lineInput = ""
		return m.closeTabOrQuit()
	default:
		m.describeView.lineInput = ""
	}
	return m, nil
}

// handleDescribeQuit handles quit/escape in describe view.
func (m Model) handleDescribeQuit() (tea.Model, tea.Cmd) {
	if m.describeView.searchQuery != "" {
		m.describeView.searchQuery = ""
		return m, nil
	}
	m.describeView.lineInput = ""
	m.mode = modeExplorer
	m.describeView.scroll = 0
	m.describeView.cursor = 0
	m.describeView.cursorCol = 0
	m.describeView.wrap = false
	m.describeView.autoRefresh = false
	m.describeView.refreshFunc = nil
	m.describeView.visualMode = 0
	m.describeView.searchQuery = ""
	m.describeView.searchInput.Clear()
	return m, nil
}

// describeWordMotion applies a word/cursor motion in describe view.
func (m *Model) describeWordMotion(key string, lines []string) {
	if m.describeView.cursor < 0 || m.describeView.cursor >= len(lines) {
		return
	}
	line := lines[m.describeView.cursor]
	switch key {
	case "$":
		lineLen := len([]rune(line))
		if lineLen > 0 {
			m.describeView.cursorCol = lineLen - 1
		}
	case "^":
		m.describeView.cursorCol = firstNonWhitespace(line)
	case "w":
		m.describeView.cursorCol = nextWordStart(line, m.describeView.cursorCol)
	case "W":
		m.describeView.cursorCol = nextWORDStart(line, m.describeView.cursorCol)
	case "b":
		if nc := prevWordStart(line, m.describeView.cursorCol); nc >= 0 {
			m.describeView.cursorCol = nc
		}
	case "B":
		if nc := prevWORDStart(line, m.describeView.cursorCol); nc >= 0 {
			m.describeView.cursorCol = nc
		}
	case "e":
		m.describeView.cursorCol = wordEnd(line, m.describeView.cursorCol)
	case "E":
		m.describeView.cursorCol = WORDEnd(line, m.describeView.cursorCol)
	}
}

// describePageMove moves the cursor by delta lines and clamps.
func (m Model) describePageMove(delta, maxIdx int) (tea.Model, tea.Cmd) {
	m.describeView.lineInput = ""
	m.describeView.cursor += delta
	if m.describeView.cursor > maxIdx {
		m.describeView.cursor = maxIdx
	}
	if m.describeView.cursor < 0 {
		m.describeView.cursor = 0
	}
	m.ensureDescribeCursorVisible()
	return m, nil
}

// handleDescribeG handles the G key (jump to line or end) in describe view.
func (m Model) handleDescribeG(maxIdx int) (tea.Model, tea.Cmd) {
	if m.describeView.lineInput != "" {
		lineNum, _ := strconv.Atoi(m.describeView.lineInput)
		m.describeView.lineInput = ""
		if lineNum > 0 {
			lineNum--
		}
		m.describeView.cursor = min(lineNum, maxIdx)
	} else {
		m.describeView.cursor = maxIdx
	}
	m.ensureDescribeCursorVisible()
	return m, nil
}

// describeEnterVisual enters visual selection mode in describe view.
func (m Model) describeEnterVisual(mode byte) (tea.Model, tea.Cmd) {
	m.describeView.lineInput = ""
	m.describeView.visualMode = mode
	m.describeView.visualStart = m.describeView.cursor
	m.describeView.visualCol = m.describeView.cursorCol
	return m, nil
}

// handleDescribeVisualKey handles keys while visual mode is active in the describe view.
func (m Model) handleDescribeVisualKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := strings.Split(m.describeView.content, "\n")
	maxIdx := max(len(lines)-1, 0)
	key := msg.String()

	if op, motion, ok := m.consumeTextObjectPrelude(key); ok {
		return m.applyDescribeTextObject(op, motion, lines)
	}

	switch key {
	case "esc":
		m.describeView.visualMode = 0
		return m, nil
	case "i", "a":
		// Clear any digit prefix accumulated before visual entry so it can't
		// leak into a later counted command via the post-visual normal mode.
		m.describeView.lineInput = ""
		m.pendingTextObject = key[0]
		return m, nil
	case "V":
		return m.describeVisualToggle('V')
	case "v":
		return m.describeVisualToggle('v')
	case "ctrl+v":
		return m.describeVisualToggle('B')
	case "j", "down":
		if m.describeView.cursor < maxIdx {
			m.describeView.cursor++
		}
		m.ensureDescribeCursorVisible()
	case "k", "up":
		if m.describeView.cursor > 0 {
			m.describeView.cursor--
		}
		m.ensureDescribeCursorVisible()
	case "h", "left":
		if m.describeView.cursorCol > 0 {
			m.describeView.cursorCol--
		}
	case "l", "right":
		m.describeView.cursorCol++
	case "0":
		m.describeView.cursorCol = 0
	case "$", "^", "w", "W", "b", "B", "e", "E":
		m.describeWordMotion(key, lines)
	case "G":
		m.describeView.cursor = maxIdx
		m.ensureDescribeCursorVisible()
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.describeView.cursor = 0
			m.ensureDescribeCursorVisible()
		} else {
			m.pendingG = true
		}
	case "ctrl+d", "shift+down":
		m.describeView.cursor += scrollStep(m.describeView.scrollOption, m.describeContentHeight())
		if m.describeView.cursor > maxIdx {
			m.describeView.cursor = maxIdx
		}
		m.ensureDescribeCursorVisible()
	case "ctrl+u", "shift+up":
		m.describeView.cursor -= scrollStep(m.describeView.scrollOption, m.describeContentHeight())
		if m.describeView.cursor < 0 {
			m.describeView.cursor = 0
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
	if m.describeView.cursor < 0 || m.describeView.cursor >= len(lines) {
		return m, nil
	}
	start, end, ok := textObjectRange(lines[m.describeView.cursor], m.describeView.cursorCol, op, motion)
	if !ok {
		return m, nil
	}
	m.describeView.visualMode = 'v'
	m.describeView.visualStart = m.describeView.cursor
	m.describeView.visualCol = start
	m.describeView.cursorCol = end
	return m, nil
}

// describeVisualToggle toggles the visual selection mode in describe view.
func (m Model) describeVisualToggle(mode byte) (tea.Model, tea.Cmd) {
	if m.describeView.visualMode == mode {
		m.describeView.visualMode = 0
	} else {
		m.describeView.visualMode = mode
	}
	return m, nil
}

// describeVisualCopy copies the visually selected text in describe view.
func (m Model) describeVisualCopy(lines []string) (tea.Model, tea.Cmd) {
	selStart := min(m.describeView.visualStart, m.describeView.cursor)
	selEnd := max(m.describeView.visualStart, m.describeView.cursor)
	if selStart < 0 {
		selStart = 0
	}
	if selEnd >= len(lines) {
		selEnd = len(lines) - 1
	}
	visualType := rune(m.describeView.visualMode)
	clipText := visualCopyText(lines, selStart, selEnd,
		visualType, m.describeView.visualCol, m.describeView.cursorCol,
		m.describeView.visualStart > m.describeView.cursor)
	lineCount := selEnd - selStart + 1
	m.describeView.visualMode = 0
	m.setStatusMessage(formatVisualYank(clipText, visualType, lineCount), false)
	return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
}

// handleDescribeSearchKey handles keyboard input during describe search.
func (m Model) handleDescribeSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.describeView.searchActive = false
		m.describeView.searchQuery = m.describeView.searchInput.Value
		m.findNextDescribeMatch(true)
	case "esc":
		m.describeView.searchActive = false
		m.describeView.searchInput.Clear()
		m.describeView.searchQuery = ""
	case "backspace":
		if len(m.describeView.searchInput.Value) > 0 {
			m.describeView.searchInput.Backspace()
		}
		m.describeView.searchQuery = m.describeView.searchInput.Value
	case "ctrl+w":
		m.describeView.searchInput.DeleteWord()
		m.describeView.searchQuery = m.describeView.searchInput.Value
	case "ctrl+a":
		m.describeView.searchInput.Home()
	case "ctrl+e":
		m.describeView.searchInput.End()
	case "left":
		m.describeView.searchInput.Left()
	case "right":
		m.describeView.searchInput.Right()
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.describeView.searchInput.Insert(key)
			// Live-update the highlight query so matches paint as the
			// user types instead of waiting for Enter to commit.
			m.describeView.searchQuery = m.describeView.searchInput.Value
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
	lines := strings.Split(m.describeView.content, "\n")
	total := len(lines)
	if m.describeView.cursor >= total {
		m.describeView.cursor = total - 1
	}
	if m.describeView.cursor < 0 {
		m.describeView.cursor = 0
	}
	viewH := m.describeContentHeight()
	so := min(ui.ConfigScrollOff, viewH/2)
	if m.describeView.cursor < m.describeView.scroll+so {
		m.describeView.scroll = m.describeView.cursor - so
	}
	if m.describeView.cursor >= m.describeView.scroll+viewH-so {
		m.describeView.scroll = m.describeView.cursor - viewH + so + 1
	}
	if m.describeView.scroll < 0 {
		m.describeView.scroll = 0
	}
	maxScroll := max(total-viewH, 0)
	if m.describeView.scroll > maxScroll {
		m.describeView.scroll = maxScroll
	}
}

// findNextDescribeMatch searches for the next/previous occurrence of the search
// query in the describe content lines and moves the cursor to it.
func (m *Model) findNextDescribeMatch(forward bool) {
	if m.describeView.searchQuery == "" {
		return
	}
	lines := strings.Split(m.describeView.content, "\n")
	if len(lines) == 0 {
		return
	}
	query := strings.ToLower(m.describeView.searchQuery)
	start := m.describeView.cursor
	total := len(lines)

	for i := 1; i <= total; i++ {
		var idx int
		if forward {
			idx = (start + i) % total
		} else {
			idx = (start - i + total) % total
		}
		if strings.Contains(strings.ToLower(lines[idx]), query) {
			m.describeView.cursor = idx
			m.ensureDescribeCursorVisible()
			return
		}
	}
	m.setStatusMessage("Pattern not found: "+m.describeView.searchQuery, false)
}
