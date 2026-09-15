package app

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/janosmiko/lfk/internal/tainted"
	"github.com/janosmiko/lfk/internal/ui"
)

// logDisplayLine returns the log line as shown on screen (timestamps/prefixes stripped).
func (m *Model) logDisplayLine(lineIdx int) string {
	line := m.logView.lines[lineIdx]
	if !m.logView.timestamps {
		line = ui.StripTimestamp(line)
	}
	if m.logView.hidePrefixes {
		line = ui.StripPodPrefix(line)
	}
	return line
}

// logMotionLine returns the text the cursor columns address. It runs the same
// sanitize the renderer does, which expands tabs, and drops producer ANSI, so
// one column counts one of the cells actually drawn.
func (m *Model) logMotionLine(lineIdx int) string {
	return ansi.Strip(tainted.SanitizeLogBody(m.logDisplayLine(lineIdx), ui.ConfigLogRenderAnsi))
}

// logJumpToCol sets the cursor to the given line and cell column.
func (m *Model) logJumpToCol(lineIdx, col int) {
	m.logView.cursor = lineIdx
	m.logView.visualCurCol = col
	m.logView.follow = false
	m.ensureLogCursorVisible()
}

// logFindFirstMatch finds the first match in a line and jumps to it.
func (m *Model) logFindFirstMatch(lineIdx int, query string) bool {
	dl := m.logMotionLine(lineIdx)
	col := ui.FindColumnInLine(dl, query)
	if col < 0 {
		return false
	}
	m.logJumpToCol(lineIdx, col)
	return true
}

// logFindLastMatch finds the last (rightmost) match in a line and jumps to it.
func (m *Model) logFindLastMatch(lineIdx int, query string) bool {
	dl := m.logMotionLine(lineIdx)
	if !ui.MatchLine(dl, query) {
		return false
	}
	lastCol := findLastMatchInStr(dl, query)
	if lastCol < 0 {
		return false
	}
	m.logJumpToCol(lineIdx, lastCol)
	return true
}

func (m *Model) findNextLogMatch(forward bool) {
	if m.logView.searchQuery == "" {
		return
	}
	rawQuery := m.logView.searchQuery
	start := m.logView.cursor
	if start < 0 {
		start = m.logView.scroll
	}

	if forward {
		m.findNextLogMatchForward(rawQuery, start)
	} else {
		m.findNextLogMatchBackward(rawQuery, start)
	}
}

func (m *Model) findNextLogMatchForward(rawQuery string, start int) {
	// Check for another match on the current line after the cursor.
	if start >= 0 && start < len(m.logView.lines) {
		dl := m.logMotionLine(start)
		w := ui.LineWidth(dl)
		// Start after the whole character under the cursor. A column one cell
		// on lands inside a wide glyph, and CutCols widens back to its start,
		// so the match offset would count from a column that was never cut.
		from := ui.SnapColEnd(dl, m.logView.visualCurCol+1)
		if from < w {
			col := ui.FindColumnInLine(ui.CutCols(dl, from, w), rawQuery)
			if col >= 0 {
				m.logJumpToCol(start, from+col)
				return
			}
		}
	}
	for i := start + 1; i < len(m.logView.lines); i++ {
		if m.logFindFirstMatch(i, rawQuery) {
			return
		}
	}
	for i := 0; i <= start; i++ {
		if m.logFindFirstMatch(i, rawQuery) {
			return
		}
	}
}

func (m *Model) findNextLogMatchBackward(rawQuery string, start int) {
	// Check for a match on the current line before the cursor.
	if start >= 0 && start < len(m.logView.lines) {
		dl := m.logMotionLine(start)
		// Clamp: visualCurCol may exceed this line's width. Backward search
		// ends at the cursor, excluding it.
		to := min(m.logView.visualCurCol, ui.LineWidth(dl))
		if to > 0 {
			lastCol := findLastMatchInStr(ui.CutCols(dl, 0, to), rawQuery)
			if lastCol >= 0 {
				m.logJumpToCol(start, lastCol)
				return
			}
		}
	}
	for i := start - 1; i >= 0; i-- {
		if m.logFindLastMatch(i, rawQuery) {
			return
		}
	}
	for i := len(m.logView.lines) - 1; i >= start; i-- {
		if m.logFindLastMatch(i, rawQuery) {
			return
		}
	}
}

// findLastMatchInStr finds the rightmost match column in a string.
func findLastMatchInStr(text, query string) int {
	lastCol := -1
	remaining := text
	offset := 0
	for {
		col := ui.FindColumnInLine(remaining, query)
		if col < 0 {
			break
		}
		lastCol = offset + col
		w := ui.LineWidth(remaining)
		// Step past the whole matched character. A rune step stalls on a
		// combining mark, which adds no cell, and the unchanged slice then
		// matches again forever.
		next := stepCol(remaining, col, 1)
		if next >= w || next <= col {
			break
		}
		remaining = ui.CutCols(remaining, next, w)
		offset += next
	}
	return lastCol
}
