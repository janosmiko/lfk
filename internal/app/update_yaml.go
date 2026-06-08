package app

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// yamlViewportLines returns the number of content lines available for the
// YAML viewer, accounting for the title bar, tab bar, borders, and hint bar.
func (m Model) yamlViewportLines() int {
	// Overhead: YAML title (1) + border top/bottom (2) + hint bar (1) = 4,
	// plus the global title bar (1) and tab bar (1 when multi-tab) which are
	// subtracted from m.height by View() at render time but NOT in Update().
	overhead := 5 // title bar + yaml title + border*2 + hint
	if len(m.tabs) > 1 {
		overhead = 6
	}
	lines := max(m.height-overhead, 3)
	return lines
}

func (m Model) handleYAMLKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When in search input mode, handle text input.
	if m.yamlView.searchMode {
		return m.handleYAMLSearchInput(msg)
	}

	// In visual selection mode, restrict keys to selection/copy/cancel.
	if m.yamlView.visualMode {
		return m.handleYAMLVisualKey(msg)
	}

	return m.handleYAMLNormalKey(msg)
}

// handleYAMLSearchInput handles key events when the YAML search input is active.
//
// Match highlights update on every keystroke so the user sees results land
// in real time instead of having to commit with Enter just to see whether
// the query matches anything. Enter still ends search-input mode and
// scrolls to the first match -- it's the "commit" action; typing only
// drives the live highlight overlay.
func (m Model) handleYAMLSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	viewportLines := m.yamlViewportLines()

	switch msg.String() {
	case "enter":
		m.yamlView.searchMode = false
		m.updateYAMLSearchMatches()
		if len(m.yamlView.matchLines) > 0 {
			m.yamlView.matchIdx = m.findYAMLMatchFromCursor()
			m.yamlScrollToMatchFolded(viewportLines)
		}
		return m, nil
	case "esc":
		m.yamlView.searchMode = false
		m.yamlView.searchText.Clear()
		m.yamlView.matchLines = nil
		m.yamlView.matchIdx = 0
		return m, nil
	case "backspace":
		if len(m.yamlView.searchText.Value) > 0 {
			m.yamlView.searchText.Backspace()
		}
		m.updateYAMLSearchMatches()
		return m, nil
	case "ctrl+w":
		m.yamlView.searchText.DeleteWord()
		m.updateYAMLSearchMatches()
		return m, nil
	case "ctrl+a":
		m.yamlView.searchText.Home()
		return m, nil
	case "ctrl+e":
		m.yamlView.searchText.End()
		return m, nil
	case "left":
		m.yamlView.searchText.Left()
		return m, nil
	case "right":
		m.yamlView.searchText.Right()
		return m, nil
	case "ctrl+c":
		m.yamlView.searchMode = false
		m.yamlView.searchText.Clear()
		m.yamlView.matchLines = nil
		return m, nil
	default:
		if len(msg.String()) == 1 || msg.String() == " " {
			m.yamlView.searchText.Insert(msg.String())
			m.updateYAMLSearchMatches()
		}
		return m, nil
	}
}

// handleYAMLNormalCopy copies the original YAML line under the cursor to the
// clipboard. A digit prefix (e.g. `123y`) yanks that many visible lines
// starting at the cursor; an empty buffer falls back to a single line.
// Counts operate on the visible-line mapping, so folded children are simply
// not in scope — a count that reaches a folded section jumps over its hidden
// children and continues with the lines that follow.
func (m Model) handleYAMLNormalCopy() (tea.Model, tea.Cmd) {
	n := consumeCountPrefix(&m.yamlView.lineInput)
	_, mapping := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	if m.yamlView.cursor < 0 || m.yamlView.cursor >= len(mapping) {
		return m, nil
	}
	origLines := strings.Split(m.yamlView.content, "\n")
	end := min(m.yamlView.cursor+n, len(mapping))
	parts := make([]string, 0, end-m.yamlView.cursor)
	for i := m.yamlView.cursor; i < end; i++ {
		origIdx := mapping[i]
		if origIdx < 0 || origIdx >= len(origLines) {
			continue
		}
		parts = append(parts, origLines[origIdx])
	}
	if len(parts) == 0 {
		return m, nil
	}
	m.setStatusMessage(formatCopiedLines(len(parts)), false)
	return m, tea.Batch(copyToSystemClipboard(strings.Join(parts, "\n")), scheduleStatusClear())
}

// handleYAMLNormalWordMotion applies a w/b/e/W/B/E motion N times, where N is
// the digit-prefix count (defaulting to 1). Each iteration mutates m via the
// pointer-receiver step so the loop avoids per-iteration Model copies — only
// the value-receiver method boundary copy remains.
func (m Model) handleYAMLNormalWordMotion(key string) (tea.Model, tea.Cmd) {
	n := consumeCountPrefix(&m.yamlView.lineInput)
	for range n {
		m.yamlWordMotionStep(key)
	}
	return m, nil
}

// yamlMaxScroll returns the maximum scroll offset for the YAML viewer.
func (m Model) yamlMaxScroll(totalVisible int) int {
	viewportLines := m.yamlViewportLines()
	maxScroll := max(totalVisible-viewportLines, 0)
	return maxScroll
}

// handleYAMLNormalKey handles key events in normal YAML viewing mode.
//
//nolint:gocyclo // flat key dispatcher: complexity is "number of keys we route", not branching depth
func (m Model) handleYAMLNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalVisible := visibleLineCount(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	viewportLines := m.yamlViewportLines()
	maxScroll := m.yamlMaxScroll(totalVisible)
	kb := ui.ActiveKeybindings

	switch msg.String() {
	case kb.Help, "f1":
		return m.handleYAMLKeyQuestion()
	case "O":
		return m.handleYAMLKeyObjectExplorer()
	case "I":
		return m.openExplainAtObjectPath(m.yamlCursorPath(), modeYAML)
	case "V":
		return m.handleYAMLKeyV()
	case "v":
		return m.handleYAMLKeyV2()
	case "ctrl+v":
		return m.handleYAMLKeyCtrlV()
	case "q", "esc":
		return m.handleYAMLKeyQ()
	case "ctrl+c":
		return m.handleYAMLKeyCtrlC()
	case kb.Search:
		return m.handleYAMLKeySlash()
	case kb.NextMatch:
		return m.handleYAMLKeyN(viewportLines)
	case kb.PrevMatch:
		return m.handleYAMLKeyShiftN(viewportLines)
	case "ctrl+e":
		return m.handleYAMLKeyCtrlE()
	case "y":
		return m.handleYAMLNormalCopy()
	case kb.ToggleWrap:
		m.yamlView.wrap = !m.yamlView.wrap
		return m, nil
	case kb.ToggleFold:
		return m.handleYAMLKeyFoldToggle()
	case kb.ToggleFoldAll:
		return m.handleYAMLKeyZ()
	case "h", "left":
		return m.handleYAMLKeyH()
	case "l", "right":
		n := consumeCountPrefix(&m.yamlView.lineInput)
		m.yamlView.visualCurCol += n
		return m, nil
	case "0":
		return m.handleYAMLKeyZero()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.yamlView.lineInput += msg.String()
		return m, nil
	case "$", "^":
		// $ / ^ are absolute-position motions and ignore counts, but still
		// consume the buffer so a stray digit prefix doesn't leak forward.
		consumeCountPrefix(&m.yamlView.lineInput)
		return m.handleYAMLVisualWordMotion(msg.String())
	case "w", "b", "e", "E", "B", "W":
		return m.handleYAMLNormalWordMotion(msg.String())
	case "j", "down":
		n := consumeCountPrefix(&m.yamlView.lineInput)
		m.yamlView.cursor = min(m.yamlView.cursor+n, max(totalVisible-1, 0))
		m.ensureYAMLCursorVisible()
		return m, nil
	case "k", "up":
		return m.handleYAMLKeyK()
	case "g":
		return m.handleYAMLKeyG()
	case "G", "end":
		return m.handleYAMLNormalG(totalVisible, maxScroll)
	case "home":
		m.yamlView.cursor = 0
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+d", "shift+down":
		return m.handleYAMLNormalHalfPageDown(totalVisible)
	case "ctrl+u", "shift+up":
		return m.handleYAMLKeyCtrlU()
	case "ctrl+f", "pgdown":
		return m.handleYAMLNormalPageDown(totalVisible)
	case "ctrl+b", "pgup":
		return m.handleYAMLKeyCtrlB()
	default:
		m.yamlView.lineInput = ""
	}
	return m, nil
}

// handleYAMLKeyN handles the 'n' key (next search match) in normal YAML mode.
func (m Model) handleYAMLKeyN(viewportLines int) (tea.Model, tea.Cmd) {
	count := consumeCountPrefix(&m.yamlView.lineInput)
	for range count {
		if len(m.yamlView.matchLines) == 0 {
			break
		}
		if m.yamlNextIntraLineMatch(true) {
			continue
		}
		m.yamlView.matchIdx = (m.yamlView.matchIdx + 1) % len(m.yamlView.matchLines)
		m.yamlScrollToMatchFolded(viewportLines)
	}
	return m, nil
}

// handleYAMLKeyShiftN handles the 'N' key (previous search match) in normal YAML mode.
func (m Model) handleYAMLKeyShiftN(viewportLines int) (tea.Model, tea.Cmd) {
	count := consumeCountPrefix(&m.yamlView.lineInput)
	for range count {
		if len(m.yamlView.matchLines) == 0 {
			break
		}
		if m.yamlNextIntraLineMatch(false) {
			continue
		}
		m.yamlView.matchIdx--
		if m.yamlView.matchIdx < 0 {
			m.yamlView.matchIdx = len(m.yamlView.matchLines) - 1
		}
		m.yamlScrollToMatchFolded(viewportLines)
	}
	return m, nil
}

// handleYAMLKeyCtrlE handles ctrl+e (edit resource) in normal YAML mode.
func (m Model) handleYAMLKeyCtrlE() (tea.Model, tea.Cmd) {
	kind := m.selectedResourceKind()
	sel := m.selectedMiddleItem()
	if kind != "" && sel != nil {
		if m.readOnly {
			m.setStatusMessage(readOnlyBlockedMessage("Edit"), true)
			return m, scheduleStatusClear()
		}
		m.actionCtx = m.buildActionCtx(sel, kind)
		return m, m.execKubectlEdit()
	}
	return m, nil
}

// handleYAMLKeyFoldToggle toggles the fold on the section at the cursor position.
func (m Model) handleYAMLKeyFoldToggle() (tea.Model, tea.Cmd) {
	_, mapping := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	sec := sectionAtScrollPos(m.yamlView.cursor, mapping, m.yamlView.sections)
	if sec != "" {
		if m.yamlView.collapsed == nil {
			m.yamlView.collapsed = make(map[string]bool)
		}
		m.yamlView.collapsed[sec] = !m.yamlView.collapsed[sec]

		if m.yamlView.collapsed[sec] {
			var startLine int
			for _, s := range m.yamlView.sections {
				if s.key == sec {
					startLine = s.startLine
					break
				}
			}
			_, newMapping := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
			for vi, orig := range newMapping {
				if orig == startLine {
					m.yamlView.cursor = vi
					break
				}
			}
		}

		m.clampYAMLScroll()
		m.ensureYAMLCursorVisible()
	}
	return m, nil
}

// handleYAMLNormalG handles the G key in normal mode (go-to-line or end).
func (m Model) handleYAMLNormalG(totalVisible, maxScroll int) (tea.Model, tea.Cmd) {
	if m.yamlView.lineInput != "" {
		lineNum, _ := strconv.Atoi(m.yamlView.lineInput)
		m.yamlView.lineInput = ""
		if lineNum > 0 {
			lineNum--
		}
		if lineNum >= totalVisible {
			lineNum = totalVisible - 1
		}
		if lineNum < 0 {
			lineNum = 0
		}
		m.yamlView.cursor = lineNum
		m.ensureYAMLCursorVisible()
		return m, nil
	}
	m.yamlView.cursor = max(totalVisible-1, 0)
	m.yamlView.scroll = maxScroll
	return m, nil
}

// handleYAMLNormalHalfPageDown handles ctrl+d (half page down) in normal YAML mode.
//
// Vim semantics via vimScrollStep: a counted press sets the sticky 'scroll'
// option to min(count, viewport); plain presses reuse the sticky value
// (defaulting to viewport/2). The same option is shared with ctrl+u.
func (m Model) handleYAMLNormalHalfPageDown(totalVisible int) (tea.Model, tea.Cmd) {
	step := vimScrollStep(&m.yamlView.lineInput, &m.yamlView.scrollOption, m.yamlViewportLines())
	m.yamlView.cursor += step
	if m.yamlView.cursor >= totalVisible {
		m.yamlView.cursor = totalVisible - 1
	}
	m.ensureYAMLCursorVisible()
	return m, nil
}

// handleYAMLNormalPageDown handles ctrl+f (full page down) in normal YAML mode.
func (m Model) handleYAMLNormalPageDown(totalVisible int) (tea.Model, tea.Cmd) {
	n := consumeCountPrefix(&m.yamlView.lineInput)
	m.yamlView.cursor += n * m.yamlViewportLines()
	if m.yamlView.cursor >= totalVisible {
		m.yamlView.cursor = totalVisible - 1
	}
	m.ensureYAMLCursorVisible()
	return m, nil
}

// ensureYAMLCursorVisible adjusts yamlScroll so the cursor is within the viewport
// with scrolloff margin.
func (m *Model) ensureYAMLCursorVisible() {
	maxLines := m.yamlViewportLines()
	so := min(ui.ConfigScrollOff, maxLines/2)
	if m.yamlView.cursor < m.yamlView.scroll+so {
		m.yamlView.scroll = m.yamlView.cursor - so
	}
	if m.yamlView.cursor >= m.yamlView.scroll+maxLines-so {
		m.yamlView.scroll = m.yamlView.cursor - maxLines + so + 1
	}
	if m.yamlView.scroll < 0 {
		m.yamlView.scroll = 0
	}
}

// clampYAMLScroll ensures yamlScroll stays within bounds after fold changes.
func (m *Model) clampYAMLScroll() {
	totalVisible := visibleLineCount(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	viewportLines := m.yamlViewportLines()
	maxScroll := max(totalVisible-viewportLines, 0)
	if m.yamlView.scroll > maxScroll {
		m.yamlView.scroll = maxScroll
	}
	if m.yamlView.scroll < 0 {
		m.yamlView.scroll = 0
	}
}

// yamlScrollToMatchFolded scrolls to show the current search match, expanding
// the containing section if it is collapsed, and using visible-line coordinates.
func (m *Model) yamlScrollToMatchFolded(viewportLines int) {
	if m.yamlView.matchIdx < 0 || m.yamlView.matchIdx >= len(m.yamlView.matchLines) {
		return
	}
	targetOrig := m.yamlView.matchLines[m.yamlView.matchIdx]

	// If the match is inside a collapsed section, expand it.
	for _, sec := range m.yamlView.sections {
		if m.yamlView.collapsed[sec.key] && targetOrig > sec.startLine && targetOrig <= sec.endLine {
			m.yamlView.collapsed[sec.key] = false
		}
	}

	// Convert original line to visible line.
	_, mapping := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	visIdx := originalToVisible(targetOrig, mapping)
	if visIdx < 0 {
		return
	}

	totalVisible := len(mapping)
	maxScroll := max(totalVisible-viewportLines, 0)

	// Move cursor to the match and center it in the viewport.
	m.yamlView.cursor = visIdx
	// Move cursor column to the match position within the visible line
	// (which includes fold prefixes).
	visibleLines, _ := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	if visIdx >= 0 && visIdx < len(visibleLines) {
		col := ui.FindColumnInLine(visibleLines[visIdx], m.yamlView.searchText.Value)
		if col >= 0 {
			m.yamlView.visualCurCol = col
		}
	}
	m.yamlView.scroll = max(min(visIdx-viewportLines/2, maxScroll), 0)
}

// yamlNextIntraLineMatch checks for another match on the current YAML line
// after (forward=true) or before (forward=false) the cursor column.
// Returns true if a match was found and cursor was moved.
func (m *Model) yamlNextIntraLineMatch(forward bool) bool {
	if m.yamlView.searchText.Value == "" {
		return false
	}
	rawQuery := m.yamlView.searchText.Value

	// Use visible lines (which include fold prefixes) for accurate column positions.
	visibleLines, _ := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	if m.yamlView.cursor < 0 || m.yamlView.cursor >= len(visibleLines) {
		return false
	}
	line := visibleLines[m.yamlView.cursor]

	runes := []rune(line)
	if forward {
		// Search for a match after the current cursor position.
		// Clamp: yamlVisualCurCol carries the column from a previously
		// focused line and may exceed this line's rune length. Forward
		// uses +1 because the search starts after (not at) the cursor.
		end := min(m.yamlView.visualCurCol+1, len(runes))
		curBytePos := len(string(runes[:end]))
		if curBytePos < len(line) {
			remainder := line[curBytePos:]
			col := ui.FindColumnInLine(remainder, rawQuery)
			if col >= 0 {
				m.yamlView.visualCurCol = m.yamlView.visualCurCol + 1 + col
				return true
			}
		}
	} else {
		// Search for a match before the current cursor position.
		// Clamp: yamlVisualCurCol may exceed this line's rune length;
		// backward search ends at (excluding) the cursor.
		end := min(m.yamlView.visualCurCol, len(runes))
		curBytePos := len(string(runes[:end]))
		if curBytePos > 0 {
			prefix := line[:curBytePos]
			// For backward search, find the last match in the prefix.
			// FindColumnInLine returns the first match; iterate to find the last.
			lastCol := -1
			remaining := prefix
			offset := 0
			for {
				col := ui.FindColumnInLine(remaining, rawQuery)
				if col < 0 {
					break
				}
				lastCol = offset + col
				// Advance past this match to find the next one.
				advanceRunes := col + 1
				runes := []rune(remaining)
				if advanceRunes >= len(runes) {
					break
				}
				remaining = string(runes[advanceRunes:])
				offset += advanceRunes
			}
			if lastCol >= 0 {
				m.yamlView.visualCurCol = lastCol
				return true
			}
		}
	}
	return false
}

// updateYAMLSearchMatches finds all lines matching the current search text.
// Supports substring, regex, and fuzzy search modes.
func (m *Model) updateYAMLSearchMatches() {
	m.yamlView.matchLines = nil
	if m.yamlView.searchText.Value == "" {
		return
	}
	rawQuery := m.yamlView.searchText.Value
	for i, line := range strings.Split(m.yamlView.content, "\n") {
		if ui.MatchLine(line, rawQuery) {
			m.yamlView.matchLines = append(m.yamlView.matchLines, i)
		}
	}
}

// findYAMLMatchFromCursor returns the index of the first match at or after the
// current cursor position. Wraps to 0 if no match is found after the cursor.
func (m *Model) findYAMLMatchFromCursor() int {
	_, mapping := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	origLine := 0
	if m.yamlView.cursor >= 0 && m.yamlView.cursor < len(mapping) {
		origLine = mapping[m.yamlView.cursor]
	}
	for i, matchLine := range m.yamlView.matchLines {
		if matchLine >= origLine {
			return i
		}
	}
	return 0
}

func (m Model) handleYAMLKeyQuestion() (tea.Model, tea.Cmd) {
	m.helpPreviousMode = modeYAML
	m.mode = modeHelp
	m.helpScroll = 0
	m.helpFilter.Clear()
	m.helpSearchActive = false
	m.helpContextMode = "YAML View"
	return m, nil
}

func (m Model) handleYAMLKeyV() (tea.Model, tea.Cmd) {
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'V'
	m.yamlView.visualStart = m.yamlView.cursor
	m.yamlView.visualCol = m.yamlView.visualCurCol
	return m, nil
}

func (m Model) handleYAMLKeyV2() (tea.Model, tea.Cmd) {
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'
	m.yamlView.visualStart = m.yamlView.cursor
	m.yamlView.visualCol = m.yamlView.visualCurCol
	return m, nil
}

func (m Model) handleYAMLKeyCtrlV() (tea.Model, tea.Cmd) {
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'B'
	m.yamlView.visualStart = m.yamlView.cursor
	m.yamlView.visualCol = m.yamlView.visualCurCol
	return m, nil
}

// handleYAMLKeyObjectExplorer switches from the YAML viewer to the Object Explorer
// browser (O), positioning the tree on the node under the YAML cursor. When the
// viewer was opened from the tree, it reuses the preserved tree; otherwise
// (opened via Enter) it opens a fresh tree for the current resource. The tree
// returns to the YAML viewer with P (open full YAML), keeping the cursor in
// sync in both directions.
func (m Model) handleYAMLKeyObjectExplorer() (tea.Model, tea.Cmd) {
	segs := m.yamlCursorPath()
	if m.yamlReturnMode == modeObjectExplorer && m.objectExplorerView.root != nil {
		m.mode = modeObjectExplorer
		m.yamlReturnMode = modeExplorer
		if segs != nil {
			m.navigateObjectExplorerToPath(segs)
		}
		m.yamlView.scroll = 0
		m.yamlView.cursor = 0
		m.yamlView.wrap = false
		return m, nil
	}
	mdl, cmd := m.openObjectExplorer()
	if m2, ok := mdl.(Model); ok && m2.mode == modeObjectExplorer && segs != nil {
		m2.navigateObjectExplorerToPath(segs)
		return m2, cmd
	}
	return mdl, cmd
}

// yamlCursorPath returns the object path under the YAML viewer's cursor, mapping
// the visible cursor to a physical line and parsing the path there. Returns nil
// when there is no resolvable path.
func (m Model) yamlCursorPath() []string {
	_, mapping := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	if m.yamlView.cursor < 0 || m.yamlView.cursor >= len(mapping) {
		return nil
	}
	return model.PathForYAMLLine(m.yamlView.content, mapping[m.yamlView.cursor])
}

// applyYAMLPendingCursor positions the YAML cursor on yamlPendingPath once the
// document has loaded, then clears the pending path. Used to sync the cursor
// when switching from the Object Explorer into the YAML viewer.
func (m *Model) applyYAMLPendingCursor() {
	segs := m.yamlPendingPath
	m.yamlPendingPath = nil
	if len(segs) == 0 {
		return
	}
	origLine := model.YAMLLineForPath(m.yamlView.content, segs)
	if origLine < 0 {
		return
	}
	_, mapping := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	if vis := originalToVisible(origLine, mapping); vis >= 0 {
		m.yamlView.cursor = vis
		m.ensureYAMLCursorVisible()
	}
}

func (m Model) handleYAMLKeyQ() (tea.Model, tea.Cmd) {
	if m.yamlView.searchText.Value != "" {
		// Clear search first.
		m.yamlView.searchText.Clear()
		m.yamlView.matchLines = nil
		m.yamlView.matchIdx = 0
		return m, nil
	}
	m.mode = m.yamlReturnMode
	m.yamlReturnMode = modeExplorer
	m.yamlView.scroll = 0
	m.yamlView.cursor = 0
	m.yamlView.wrap = false
	return m, nil
}

func (m Model) handleYAMLKeyCtrlC() (tea.Model, tea.Cmd) {
	m.mode = m.yamlReturnMode
	m.yamlReturnMode = modeExplorer
	m.yamlView.scroll = 0
	m.yamlView.cursor = 0
	m.yamlView.wrap = false
	m.yamlView.searchText.Clear()
	m.yamlView.matchLines = nil
	return m, nil
}

func (m Model) handleYAMLKeySlash() (tea.Model, tea.Cmd) {
	m.yamlView.searchMode = true
	m.yamlView.searchText.Clear()
	m.yamlView.matchLines = nil
	m.yamlView.matchIdx = 0
	return m, nil
}

func (m Model) handleYAMLKeyZ() (tea.Model, tea.Cmd) {
	if m.yamlView.collapsed == nil {
		m.yamlView.collapsed = make(map[string]bool)
	}
	anyExpanded := false
	for _, sec := range m.yamlView.sections {
		if isMultiLineSection(sec) && !m.yamlView.collapsed[sec.key] {
			anyExpanded = true
			break
		}
	}
	if anyExpanded {
		for _, sec := range m.yamlView.sections {
			if isMultiLineSection(sec) {
				m.yamlView.collapsed[sec.key] = true
			}
		}
	} else {
		m.yamlView.collapsed = make(map[string]bool)
	}
	m.clampYAMLScroll()
	return m, nil
}

func (m Model) handleYAMLKeyH() (tea.Model, tea.Cmd) {
	n := consumeCountPrefix(&m.yamlView.lineInput)
	m.yamlView.visualCurCol = max(m.yamlView.visualCurCol-n, yamlFoldPrefixLen)
	return m, nil
}

func (m Model) handleYAMLKeyZero() (tea.Model, tea.Cmd) {
	if m.yamlView.lineInput != "" {
		m.yamlView.lineInput += "0"
	} else {
		m.yamlView.visualCurCol = yamlFoldPrefixLen
	}
	return m, nil
}

func (m Model) handleYAMLKeyK() (tea.Model, tea.Cmd) {
	n := consumeCountPrefix(&m.yamlView.lineInput)
	m.yamlView.cursor = max(m.yamlView.cursor-n, 0)
	m.ensureYAMLCursorVisible()
	return m, nil
}

func (m Model) handleYAMLKeyG() (tea.Model, tea.Cmd) {
	m.yamlView.lineInput = ""
	if m.pendingG {
		m.pendingG = false
		m.yamlView.cursor = 0
		m.yamlView.scroll = 0
		return m, nil
	}
	m.pendingG = true
	return m, nil
}

func (m Model) handleYAMLKeyCtrlU() (tea.Model, tea.Cmd) {
	step := vimScrollStep(&m.yamlView.lineInput, &m.yamlView.scrollOption, m.yamlViewportLines())
	m.yamlView.cursor -= step
	if m.yamlView.cursor < 0 {
		m.yamlView.cursor = 0
	}
	m.ensureYAMLCursorVisible()
	return m, nil
}

func (m Model) handleYAMLKeyCtrlB() (tea.Model, tea.Cmd) {
	n := consumeCountPrefix(&m.yamlView.lineInput)
	m.yamlView.cursor -= n * m.yamlViewportLines()
	if m.yamlView.cursor < 0 {
		m.yamlView.cursor = 0
	}
	m.ensureYAMLCursorVisible()
	return m, nil
}
