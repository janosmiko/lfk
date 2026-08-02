package app

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleDiffKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	foldRegions := ui.ComputeDiffFoldRegions(m.diffView.left, m.diffView.right)
	m.ensureDiffFoldState(foldRegions)

	totalLines, visibleLines, maxScroll := m.diffViewMetrics(foldRegions)

	// When in search input mode, handle text input first.
	if m.diffView.searchMode {
		return m.handleDiffSearchInput(msg, foldRegions, visibleLines)
	}

	// In visual selection mode, delegate to the visual key handler.
	if m.diffView.visualMode {
		return m.handleDiffVisualKey(msg, foldRegions, totalLines, visibleLines, maxScroll)
	}

	return m.handleDiffNormalKey(msg, foldRegions, totalLines, visibleLines, maxScroll)
}

// diffViewMetrics computes the total lines, visible lines, and max scroll for the diff view.
func (m Model) diffViewMetrics(foldRegions []ui.DiffFoldRegion) (totalLines, visibleLines, maxScroll int) {
	totalLines = ui.DiffViewTotalLines(m.diffView.left, m.diffView.right, foldRegions, m.diffView.foldState)
	overhead := 1
	if len(m.tabs) > 1 {
		overhead++
	}
	visibleLines = m.height - overhead - 6
	if m.diffView.unified {
		totalLines = ui.UnifiedDiffViewTotalLines(m.diffView.left, m.diffView.right, foldRegions, m.diffView.foldState)
		visibleLines = m.height - overhead - 6
	}
	if visibleLines < 3 {
		visibleLines = 3
	}
	maxScroll = max(totalLines-visibleLines, 0)
	return totalLines, visibleLines, maxScroll
}

// handleDiffSearchInput handles key events in diff search input mode.
func (m Model) handleDiffSearchInput(msg tea.KeyPressMsg, foldRegions []ui.DiffFoldRegion, visibleLines int) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.diffView.searchMode = false
		m.diffView.searchQuery = m.diffView.searchText.Value
		m.diffView.matchLines = ui.UpdateDiffSearchMatches(m.diffView.left, m.diffView.right, m.diffView.searchQuery, m.diffView.cursorSide, m.diffView.unified)
		if len(m.diffView.matchLines) > 0 {
			m.diffView.matchIdx = 0
			m.diffScrollToMatch(foldRegions, visibleLines)
		}
		return m, nil
	case "esc":
		m.diffView.searchMode = false
		m.diffView.searchText.Clear()
		m.diffView.searchQuery = ""
		m.diffView.matchLines = nil
		m.diffView.matchIdx = 0
		return m, nil
	case "backspace":
		if len(m.diffView.searchText.Value) > 0 {
			m.diffView.searchText.Backspace()
		}
		return m, nil
	case "ctrl+w":
		m.diffView.searchText.DeleteWord()
		return m, nil
	case "ctrl+a":
		m.diffView.searchText.Home()
		return m, nil
	case "ctrl+e":
		m.diffView.searchText.End()
		return m, nil
	case "left":
		m.diffView.searchText.Left()
		return m, nil
	case "right":
		m.diffView.searchText.Right()
		return m, nil
	case "ctrl+c":
		m.diffView.searchMode = false
		m.diffView.searchText.Clear()
		m.diffView.matchLines = nil
		return m, nil
	default:
		if msg.Text != "" {
			m.diffView.searchText.Insert(msg.Text)
		}
		return m, nil
	}
}

// handleDiffNormalKey handles key events in normal diff view mode.
//
//nolint:gocyclo // switch-based key dispatch is inherently high-complexity
func (m Model) handleDiffNormalKey(msg tea.KeyPressMsg, foldRegions []ui.DiffFoldRegion, totalLines, visibleLines, maxScroll int) (tea.Model, tea.Cmd) {
	maxCursor := max(totalLines-1, 0)
	kb := ui.ActiveKeybindings

	switch msg.String() {
	case kb.Help, "f1":
		m.helpPreviousMode = modeDiff
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "Diff View"
		return m, nil
	case kb.ToggleWrap:
		m.diffView.wrap = !m.diffView.wrap
		return m, nil
	case "q", "esc":
		return m.handleDiffQuit()
	case "j", "down":
		n := consumeCountPrefix(&m.diffView.lineInput)
		m.diffView.cursor = min(m.diffView.cursor+n, maxCursor)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "k", "up":
		n := consumeCountPrefix(&m.diffView.lineInput)
		m.diffView.cursor = max(m.diffView.cursor-n, 0)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "h", "left":
		n := consumeCountPrefix(&m.diffView.lineInput)
		m.diffView.visualCurCol = max(m.diffView.visualCurCol-n, 0)
		return m, nil
	case "l", "right":
		n := consumeCountPrefix(&m.diffView.lineInput)
		m.diffView.visualCurCol += n
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.diffView.lineInput = ""
			m.diffView.cursor = 0
			m.diffView.scroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		return m.handleDiffG(maxCursor, visibleLines, maxScroll)
	case "end":
		m.diffView.lineInput = ""
		m.diffView.cursor = maxCursor
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "home":
		m.pendingG = false
		m.diffView.lineInput = ""
		m.diffView.cursor = 0
		m.diffView.scroll = 0
		return m, nil
	case "ctrl+d", "ctrl+u", "ctrl+f", "ctrl+b", "pgdown", "pgup", "shift+down", "shift+up":
		return m.diffPageMoveByKey(msg.String(), maxCursor, visibleLines, maxScroll)
	case "0":
		if m.diffView.lineInput != "" {
			m.diffView.lineInput += "0"
		} else {
			m.diffView.visualCurCol = 0
		}
		return m, nil
	case "$", "^":
		// Absolute-position motions ignore counts but still consume the
		// buffer so a stray digit prefix doesn't leak forward.
		consumeCountPrefix(&m.diffView.lineInput)
		m.diffWordMotion(msg.String(), foldRegions)
		return m, nil
	case "w", "b", "e", "E", "W", "B":
		count := consumeCountPrefix(&m.diffView.lineInput)
		for range count {
			m.diffWordMotion(msg.String(), foldRegions)
		}
		return m, nil
	case "v", "V", "ctrl+v":
		modeMap := map[string]rune{"v": 'v', "V": 'V', "ctrl+v": 'B'}
		return m.diffEnterVisual(modeMap[msg.String()])
	case "y":
		return m.handleDiffNormalCopy(foldRegions, totalLines)
	case kb.ToggleUnified:
		m.diffView.lineInput = ""
		m.diffView.unified = !m.diffView.unified
		m.diffView.scroll = 0
		return m, nil
	case kb.ToggleLineNumbers:
		m.diffView.lineInput = ""
		m.diffView.lineNumbers = !m.diffView.lineNumbers
		return m, nil
	case kb.Search:
		m.diffView.lineInput = ""
		m.diffView.searchMode = true
		m.diffView.searchText.Clear()
		m.diffView.matchLines = nil
		m.diffView.matchIdx = 0
		return m, nil
	case kb.NextMatch:
		return m.handleDiffSearchNav("n", foldRegions, visibleLines)
	case kb.PrevMatch:
		return m.handleDiffSearchNav("N", foldRegions, visibleLines)
	case "tab":
		if !m.diffView.unified {
			m.diffView.cursorSide = 1 - m.diffView.cursorSide
		}
		return m, nil
	case kb.ToggleFold, kb.ToggleFoldAll:
		m.diffView.lineInput = ""
		if msg.String() == kb.ToggleFoldAll {
			m.toggleAllDiffFolds(foldRegions)
		} else {
			m.toggleDiffFoldAtCursor(foldRegions)
		}
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.diffView.lineInput += msg.String()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		m.diffView.lineInput = ""
	}
	return m, nil
}

// handleDiffQuit handles quit/escape in diff view.
func (m Model) handleDiffQuit() (tea.Model, tea.Cmd) {
	m.mode = modeExplorer
	m.diffView.scroll = 0
	m.diffView.cursor = 0
	m.diffView.cursorSide = 0
	m.diffView.lineInput = ""
	m.diffView.wrap = ui.ConfigDiffViewerWrap
	m.diffView.searchQuery = ""
	m.diffView.searchText.Clear()
	m.diffView.matchLines = nil
	m.diffView.matchIdx = 0
	m.diffView.foldState = nil
	m.diffView.visualMode = false
	m.diffView.visualCurCol = 0
	return m, nil
}

// handleDiffG handles the G key (jump to line or end) in diff view.
func (m Model) handleDiffG(maxCursor, visibleLines, maxScroll int) (tea.Model, tea.Cmd) {
	if m.diffView.lineInput != "" {
		lineNum, _ := strconv.Atoi(m.diffView.lineInput)
		m.diffView.lineInput = ""
		if lineNum > 0 {
			lineNum--
		}
		m.diffView.cursor = min(lineNum, maxCursor)
	} else {
		m.diffView.cursor = maxCursor
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
			m.diffView.visualCurCol = lineLen - 1
		}
	case "^":
		m.diffView.visualCurCol = firstNonWhitespace(lineText)
	case "w":
		if lineText != "" {
			m.diffView.visualCurCol = diffClampCol(nextWordStart(lineText, m.diffView.visualCurCol), lineText)
		}
	case "W":
		if lineText != "" {
			m.diffView.visualCurCol = diffClampCol(nextWORDStart(lineText, m.diffView.visualCurCol), lineText)
		}
	case "b":
		if lineText != "" {
			newCol := max(prevWordStart(lineText, m.diffView.visualCurCol), 0)
			m.diffView.visualCurCol = newCol
		}
	case "B":
		if lineText != "" {
			newCol := max(prevWORDStart(lineText, m.diffView.visualCurCol), 0)
			m.diffView.visualCurCol = newCol
		}
	case "e":
		if lineText != "" {
			m.diffView.visualCurCol = diffClampCol(wordEnd(lineText, m.diffView.visualCurCol), lineText)
		}
	case "E":
		if lineText != "" {
			m.diffView.visualCurCol = diffClampCol(WORDEnd(lineText, m.diffView.visualCurCol), lineText)
		}
	}
}

// handleDiffSearchNav handles n/N (next/prev search match) in diff view.
func (m Model) handleDiffSearchNav(key string, foldRegions []ui.DiffFoldRegion, visibleLines int) (tea.Model, tea.Cmd) {
	count := consumeCountPrefix(&m.diffView.lineInput)
	if len(m.diffView.matchLines) == 0 {
		return m, nil
	}
	for range count {
		if key == "n" {
			m.diffView.matchIdx = (m.diffView.matchIdx + 1) % len(m.diffView.matchLines)
		} else {
			m.diffView.matchIdx = (m.diffView.matchIdx - 1 + len(m.diffView.matchLines)) % len(m.diffView.matchLines)
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
	case "ctrl+d", "shift+down":
		step := vimScrollStep(&m.diffView.lineInput, &m.diffView.scrollOption, visibleLines)
		m.diffView.cursor = min(m.diffView.cursor+step, maxCursor)
	case "ctrl+u", "shift+up":
		step := vimScrollStep(&m.diffView.lineInput, &m.diffView.scrollOption, visibleLines)
		m.diffView.cursor = max(m.diffView.cursor-step, 0)
	case "ctrl+f", "pgdown":
		n := consumeCountPrefix(&m.diffView.lineInput)
		m.diffView.cursor = min(m.diffView.cursor+n*visibleLines, maxCursor)
	case "ctrl+b", "pgup":
		n := consumeCountPrefix(&m.diffView.lineInput)
		m.diffView.cursor = max(m.diffView.cursor-n*visibleLines, 0)
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
	m.diffView.lineInput = ""
	m.diffView.visualMode = true
	m.diffView.visualType = mode
	m.diffView.visualStart = m.diffView.cursor
	m.diffView.visualCol = m.diffView.visualCurCol
	return m, nil
}

// diffCurrentLineText returns the plain text of the current diff line on the active side.
func (m *Model) diffCurrentLineText(foldRegions []ui.DiffFoldRegion) string {
	return ui.DiffLineTextAt(m.diffView.left, m.diffView.right, foldRegions, m.diffView.foldState, m.diffView.cursor, m.diffView.cursorSide, m.diffView.unified)
}

// handleDiffVisualKey handles key events while in diff visual selection mode.
func (m Model) handleDiffVisualKey(msg tea.KeyPressMsg, foldRegions []ui.DiffFoldRegion, totalLines, visibleLines, maxScroll int) (tea.Model, tea.Cmd) {
	maxCursor := max(totalLines-1, 0)
	key := msg.String()

	if op, motion, ok := m.consumeTextObjectPrelude(key); ok {
		return m.applyDiffTextObject(op, motion, foldRegions)
	}

	switch key {
	case "esc":
		m.diffView.visualMode = false
		return m, nil
	case "i", "a":
		// Clear any digit prefix accumulated before visual entry so it can't
		// leak into a later counted command via the post-visual normal mode.
		m.diffView.lineInput = ""
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
		if m.diffView.cursor < maxCursor {
			m.diffView.cursor++
		}
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "k", "up":
		if m.diffView.cursor > 0 {
			m.diffView.cursor--
		}
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "h", "left":
		if m.diffView.visualType == 'v' || m.diffView.visualType == 'B' {
			if m.diffView.visualCurCol > 0 {
				m.diffView.visualCurCol--
			}
		}
		return m, nil
	case "l", "right":
		if m.diffView.visualType == 'v' || m.diffView.visualType == 'B' {
			m.diffView.visualCurCol++
		}
		return m, nil
	case "0":
		m.diffView.visualCurCol = 0
		return m, nil
	case "$", "^", "w", "b", "e", "E", "W", "B":
		m.diffWordMotion(msg.String(), foldRegions)
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.diffView.cursor = 0
			m.diffView.scroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		m.diffView.cursor = maxCursor
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+d", "shift+down":
		m.diffView.cursor = min(m.diffView.cursor+scrollStep(m.diffView.scrollOption, visibleLines), maxCursor)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+u", "shift+up":
		m.diffView.cursor = max(m.diffView.cursor-scrollStep(m.diffView.scrollOption, visibleLines), 0)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+f", "pgdown":
		m.diffView.cursor = min(m.diffView.cursor+visibleLines, maxCursor)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+b", "pgup":
		m.diffView.cursor = max(m.diffView.cursor-visibleLines, 0)
		m.ensureDiffCursorVisible(visibleLines, maxScroll)
		return m, nil
	case "ctrl+c":
		m.diffView.visualMode = false
		return m.closeTabOrQuit()
	}
	return m, nil
}

// applyDiffTextObject resolves an `iw`/`aw`/`iW`/`aW` text object on the
// active-side line text under the cursor and switches the visual selection
// to character mode covering the resulting range.
func (m Model) applyDiffTextObject(op byte, motion string, foldRegions []ui.DiffFoldRegion) (tea.Model, tea.Cmd) {
	start, end, ok := textObjectRange(m.diffCurrentLineText(foldRegions), m.diffView.visualCurCol, op, motion)
	if !ok {
		return m, nil
	}
	m.diffView.visualType = 'v'
	m.diffView.visualStart = m.diffView.cursor
	m.diffView.visualCol = start
	m.diffView.visualCurCol = end
	return m, nil
}

// diffVisualToggle toggles the visual selection type in diff view.
func (m Model) diffVisualToggle(mode rune) (tea.Model, tea.Cmd) {
	if m.diffView.visualType == mode {
		m.diffView.visualMode = false
	} else {
		m.diffView.visualType = mode
	}
	return m, nil
}

// handleDiffNormalCopy copies diff lines at and below the cursor (on the
// active side) to the clipboard. A digit prefix (e.g. `123y`) yanks that
// many lines; an empty buffer falls back to a single line. Empty-side lines
// are skipped so a count that straddles them still copies real content.
func (m Model) handleDiffNormalCopy(foldRegions []ui.DiffFoldRegion, totalLines int) (tea.Model, tea.Cmd) {
	n := consumeCountPrefix(&m.diffView.lineInput)
	end := min(m.diffView.cursor+n, totalLines)
	parts := make([]string, 0, end-m.diffView.cursor)
	for i := m.diffView.cursor; i < end; i++ {
		lineText := ui.DiffLineTextAt(m.diffView.left, m.diffView.right, foldRegions, m.diffView.foldState, i, m.diffView.cursorSide, m.diffView.unified)
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
	selStart := min(m.diffView.visualStart, m.diffView.cursor)
	selEnd := max(m.diffView.visualStart, m.diffView.cursor)

	// Collect lines, skipping empty-side lines.
	var diffLines []string
	for i := selStart; i <= selEnd; i++ {
		lineText := ui.DiffLineTextAt(m.diffView.left, m.diffView.right, foldRegions, m.diffView.foldState, i, m.diffView.cursorSide, m.diffView.unified)
		if lineText != "" {
			diffLines = append(diffLines, lineText)
		}
	}
	clipText := visualCopyText(diffLines, 0, len(diffLines)-1,
		m.diffView.visualType, m.diffView.visualCol, m.diffView.visualCurCol,
		m.diffView.visualStart > m.diffView.cursor)
	visualType := m.diffView.visualType
	m.diffView.visualMode = false
	m.setStatusMessage(formatVisualYank(clipText, visualType, len(diffLines)), false)
	return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
}

// ensureDiffFoldState ensures the fold state slice has the correct length for
// the current fold regions.
func (m *Model) ensureDiffFoldState(regions []ui.DiffFoldRegion) {
	if len(m.diffView.foldState) < len(regions) {
		newState := make([]bool, len(regions))
		copy(newState, m.diffView.foldState)
		m.diffView.foldState = newState
	}
}

// ensureDiffCursorVisible adjusts diffScroll so the cursor is within the viewport.
func (m *Model) ensureDiffCursorVisible(viewportLines, maxScroll int) {
	so := min(ui.ConfigScrollOff, viewportLines/2)
	if m.diffView.cursor < m.diffView.scroll+so {
		m.diffView.scroll = m.diffView.cursor - so
	}
	if m.diffView.cursor >= m.diffView.scroll+viewportLines-so {
		m.diffView.scroll = m.diffView.cursor - viewportLines + so + 1
	}
	m.diffView.scroll = max(min(m.diffView.scroll, maxScroll), 0)
}

// diffScrollToMatch auto-expands the fold region containing the current match,
// scrolls to center it in the viewport, and moves the cursor column to the match.
func (m *Model) diffScrollToMatch(foldRegions []ui.DiffFoldRegion, viewportLines int) {
	if len(m.diffView.matchLines) == 0 || m.diffView.matchIdx < 0 || m.diffView.matchIdx >= len(m.diffView.matchLines) {
		return
	}
	origIdx := m.diffView.matchLines[m.diffView.matchIdx]

	// Auto-expand any collapsed fold region containing this match.
	ui.ExpandDiffFoldForLine(foldRegions, m.diffView.foldState, origIdx)

	// Find the visible index for this original line.
	visIdx := ui.DiffVisibleIndexForOriginal(m.diffView.left, m.diffView.right, foldRegions, m.diffView.foldState, origIdx)
	if visIdx < 0 {
		return
	}

	// Move cursor line and center in viewport.
	m.diffView.cursor = visIdx
	m.diffView.scroll = max(visIdx-viewportLines/2, 0)

	// Move cursor column to the match position on the active side.
	lineText := m.diffCurrentLineText(foldRegions)
	col := ui.DiffSearchColumnInLine(lineText, m.diffView.searchQuery)
	if col >= 0 {
		m.diffView.visualCurCol = col
	}
}

// toggleDiffFoldAtCursor toggles the fold on the unchanged section at the cursor.
// When collapsing, moves the cursor to the fold placeholder line.
func (m *Model) toggleDiffFoldAtCursor(foldRegions []ui.DiffFoldRegion) {
	rawDiffLines := ui.ComputeDiffLines(m.diffView.left, m.diffView.right)
	visLines := ui.BuildVisibleDiffLines(rawDiffLines, foldRegions, m.diffView.foldState)

	idx := m.diffView.cursor
	if idx >= len(visLines) {
		idx = len(visLines) - 1
	}
	if idx < 0 {
		return
	}

	vl := visLines[idx]
	if vl.RegionIdx < 0 || vl.RegionIdx >= len(m.diffView.foldState) {
		return
	}

	wasCollapsed := m.diffView.foldState[vl.RegionIdx]
	m.diffView.foldState[vl.RegionIdx] = !wasCollapsed

	// When collapsing, reposition cursor to the fold placeholder.
	if !wasCollapsed {
		newVisLines := ui.BuildVisibleDiffLines(rawDiffLines, foldRegions, m.diffView.foldState)
		for i, nvl := range newVisLines {
			if nvl.IsFoldPlaceholder && nvl.RegionIdx == vl.RegionIdx {
				m.diffView.cursor = i
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
		if i < len(m.diffView.foldState) && m.diffView.foldState[i] {
			anyCollapsed = true
			break
		}
	}
	for i := range foldRegions {
		if i < len(m.diffView.foldState) {
			m.diffView.foldState[i] = !anyCollapsed
		}
	}
}
