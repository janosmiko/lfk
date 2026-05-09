package app

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

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
	maxScroll = max(totalLines-visibleLines, 0)
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
//
//nolint:gocyclo // switch-based key dispatch is inherently high-complexity
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
		n := consumeCountPrefix(&m.diffLineInput)
		m.diffCursor = min(m.diffCursor+n, maxCursor)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "k", "up":
		n := consumeCountPrefix(&m.diffLineInput)
		m.diffCursor = max(m.diffCursor-n, 0)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "h", "left":
		n := consumeCountPrefix(&m.diffLineInput)
		m.diffVisualCurCol = max(m.diffVisualCurCol-n, 0)
		return m, nil
	case "l", "right":
		n := consumeCountPrefix(&m.diffLineInput)
		m.diffVisualCurCol += n
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
	case "end":
		m.diffLineInput = ""
		m.diffCursor = maxCursor
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "home":
		m.pendingG = false
		m.diffLineInput = ""
		m.diffCursor = 0
		m.diffScroll = 0
		return m, nil
	case "ctrl+d", "ctrl+u", "ctrl+f", "ctrl+b", "pgdown", "pgup":
		return m.diffPageMoveByKey(msg.String(), maxCursor, visibleLines, maxScroll)
	case "0":
		if m.diffLineInput != "" {
			m.diffLineInput += "0"
		} else {
			m.diffVisualCurCol = 0
		}
		return m, nil
	case "$", "^":
		// Absolute-position motions ignore counts but still consume the
		// buffer so a stray digit prefix doesn't leak forward.
		consumeCountPrefix(&m.diffLineInput)
		m.diffWordMotion(msg.String(), foldRegions)
		return m, nil
	case "w", "b", "e", "E", "W", "B":
		count := consumeCountPrefix(&m.diffLineInput)
		for range count {
			m.diffWordMotion(msg.String(), foldRegions)
		}
		return m, nil
	case "v", "V", "ctrl+v":
		modeMap := map[string]rune{"v": 'v', "V": 'V', "ctrl+v": 'B'}
		return m.diffEnterVisual(modeMap[msg.String()])
	case "y":
		return m.handleDiffNormalCopy(foldRegions, totalLines)
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
			newCol := max(prevWordStart(lineText, m.diffVisualCurCol), 0)
			m.diffVisualCurCol = newCol
		}
	case "B":
		if lineText != "" {
			newCol := max(prevWORDStart(lineText, m.diffVisualCurCol), 0)
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
	count := consumeCountPrefix(&m.diffLineInput)
	if len(m.diffMatchLines) == 0 {
		return m, nil
	}
	for range count {
		if key == "n" {
			m.diffMatchIdx = (m.diffMatchIdx + 1) % len(m.diffMatchLines)
		} else {
			m.diffMatchIdx = (m.diffMatchIdx - 1 + len(m.diffMatchLines)) % len(m.diffMatchLines)
		}
	}
	m.diffScrollToMatch(foldRegions, visibleLines)
	return m, nil
}

// diffPageMoveByKey moves the diff cursor by a page amount based on the key pressed.
//
// `<C-d>`/`<C-u>` follow vim's `[count]<C-d>` semantics via vimScrollStep:
// counted presses set a sticky 'scroll' option, plain presses reuse it (or
// fall back to half-viewport). `<C-f>`/`<C-b>` scroll `count` full pages.
func (m Model) diffPageMoveByKey(key string, maxCursor, visibleLines, maxScroll int) (tea.Model, tea.Cmd) {
	switch key {
	case "ctrl+d":
		step := vimScrollStep(&m.diffLineInput, &m.diffScrollOption, visibleLines)
		m.diffCursor = min(m.diffCursor+step, maxCursor)
	case "ctrl+u":
		step := vimScrollStep(&m.diffLineInput, &m.diffScrollOption, visibleLines)
		m.diffCursor = max(m.diffCursor-step, 0)
	case "ctrl+f", "pgdown":
		n := consumeCountPrefix(&m.diffLineInput)
		m.diffCursor = min(m.diffCursor+n*visibleLines, maxCursor)
	case "ctrl+b", "pgup":
		n := consumeCountPrefix(&m.diffLineInput)
		m.diffCursor = max(m.diffCursor-n*visibleLines, 0)
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
//
// diffLineInput must be cleared on entry: visual-mode page/word handlers
// don't call consumeCountPrefix, so a stale digit prefix typed before `v`
// (e.g. `5v<Esc>j`) would otherwise leak into the next normal-mode motion
// and silently multiply it. Mirrors handleLogKeyV/V2/CtrlV.
func (m Model) diffEnterVisual(mode rune) (tea.Model, tea.Cmd) {
	m.diffLineInput = ""
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
	key := msg.String()

	if op, motion, ok := m.consumeTextObjectPrelude(key); ok {
		return m.applyDiffTextObject(op, motion, foldRegions)
	}

	switch key {
	case "esc":
		m.diffVisualMode = false
		return m, nil
	case "i", "a":
		// Clear any digit prefix accumulated before visual entry so it can't
		// leak into a later counted command via the post-visual normal mode.
		m.diffLineInput = ""
		m.pendingTextObject = key[0]
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
		m.diffCursor = min(m.diffCursor+scrollStep(m.diffScrollOption, visibleLines), maxCursor)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+u":
		m.diffCursor = max(m.diffCursor-scrollStep(m.diffScrollOption, visibleLines), 0)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+f", "pgdown":
		m.diffCursor = min(m.diffCursor+visibleLines, maxCursor)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+b", "pgup":
		m.diffCursor = max(m.diffCursor-visibleLines, 0)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+c":
		m.diffVisualMode = false
		return m.closeTabOrQuit()
	}
	return m, nil
}

// applyDiffTextObject resolves an `iw`/`aw`/`iW`/`aW` text object on the
// active-side line text under the cursor and switches the visual selection
// to character mode covering the resulting range.
func (m Model) applyDiffTextObject(op byte, motion string, foldRegions []ui.DiffFoldRegion) (tea.Model, tea.Cmd) {
	start, end, ok := textObjectRange(m.diffCurrentLineText(foldRegions), m.diffVisualCurCol, op, motion)
	if !ok {
		return m, nil
	}
	m.diffVisualType = 'v'
	m.diffVisualStart = m.diffCursor
	m.diffVisualCol = start
	m.diffVisualCurCol = end
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

// handleDiffNormalCopy copies diff lines at and below the cursor (on the
// active side) to the clipboard. A digit prefix (e.g. `123y`) yanks that
// many lines; an empty buffer falls back to a single line. Empty-side lines
// are skipped so a count that straddles them still copies real content.
func (m Model) handleDiffNormalCopy(foldRegions []ui.DiffFoldRegion, totalLines int) (tea.Model, tea.Cmd) {
	n := consumeCountPrefix(&m.diffLineInput)
	end := min(m.diffCursor+n, totalLines)
	parts := make([]string, 0, end-m.diffCursor)
	for i := m.diffCursor; i < end; i++ {
		lineText := ui.DiffLineTextAt(m.diffLeft, m.diffRight, foldRegions, m.diffFoldState, i, m.diffCursorSide, m.diffUnified)
		if lineText == "" {
			continue
		}
		parts = append(parts, lineText)
	}
	if len(parts) == 0 {
		return m, nil
	}
	m.setStatusMessage(formatCopiedLines(len(parts)), false)
	return m, tea.Batch(copyToSystemClipboard(strings.Join(parts, "\n")), scheduleStatusClear())
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
	visualType := m.diffVisualType
	m.diffVisualMode = false
	m.setStatusMessage(formatVisualYank(clipText, visualType, len(diffLines)), false)
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
	so := min(ui.ConfigScrollOff, viewportLines/2)
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
	m.diffScroll = max(visIdx-viewportLines/2, 0)

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
