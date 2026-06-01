package app

import (
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

// logJumpToCol sets the cursor to the given line and rune column.
func (m *Model) logJumpToCol(lineIdx, runeCol int) {
	m.logView.cursor = lineIdx
	m.logView.visualCurCol = runeCol
	m.logView.follow = false
	m.ensureLogCursorVisible()
}

// logFindFirstMatch finds the first match in a line and jumps to it.
func (m *Model) logFindFirstMatch(lineIdx int, query string) bool {
	dl := m.logDisplayLine(lineIdx)
	col := ui.FindColumnInLine(dl, query)
	if col < 0 {
		return false
	}
	m.logJumpToCol(lineIdx, col)
	return true
}

// logFindLastMatch finds the last (rightmost) match in a line and jumps to it.
func (m *Model) logFindLastMatch(lineIdx int, query string) bool {
	dl := m.logDisplayLine(lineIdx)
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
		dl := m.logDisplayLine(start)
		runes := []rune(dl)
		// Clamp: logVisualCurCol carries the column from a previously
		// focused line and may exceed this line's rune length. Forward
		// uses +1 because the search starts after (not at) the cursor.
		end := min(m.logView.visualCurCol+1, len(runes))
		curBytePos := len(string(runes[:end]))
		if curBytePos < len(dl) {
			col := ui.FindColumnInLine(dl[curBytePos:], rawQuery)
			if col >= 0 {
				m.logJumpToCol(start, m.logView.visualCurCol+1+col)
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
		dl := m.logDisplayLine(start)
		runes := []rune(dl)
		// Clamp: logVisualCurCol may exceed this line's rune length;
		// backward search ends at (excluding) the cursor.
		end := min(m.logView.visualCurCol, len(runes))
		curBytePos := len(string(runes[:end]))
		if curBytePos > 0 {
			lastCol := findLastMatchInStr(dl[:curBytePos], rawQuery)
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
		advanceRunes := col + 1
		runes := []rune(remaining)
		if advanceRunes >= len(runes) {
			break
		}
		remaining = string(runes[advanceRunes:])
		offset += advanceRunes
	}
	return lastCol
}
