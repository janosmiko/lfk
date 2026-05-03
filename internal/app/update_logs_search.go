package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// logDisplayLine returns the log line as shown on screen (timestamps/prefixes stripped).
func (m *Model) logDisplayLine(lineIdx int) string {
	line := m.logLines[lineIdx]
	if !m.logTimestamps {
		line = ui.StripTimestamp(line)
	}
	if m.logHidePrefixes {
		line = ui.StripPodPrefix(line)
	}
	return line
}

// logJumpToCol sets the cursor to the given line and rune column.
func (m *Model) logJumpToCol(lineIdx, runeCol int) {
	m.logCursor = lineIdx
	m.logVisualCurCol = runeCol
	m.logFollow = false
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
	lastCol := -1
	remaining := dl
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
	if lastCol < 0 {
		return false
	}
	m.logJumpToCol(lineIdx, lastCol)
	return true
}

func (m *Model) findNextLogMatch(forward bool) {
	if m.logSearchQuery == "" {
		return
	}
	rawQuery := m.logSearchQuery
	start := m.logCursor
	if start < 0 {
		start = m.logScroll
	}

	if forward {
		m.findNextLogMatchForward(rawQuery, start)
	} else {
		m.findNextLogMatchBackward(rawQuery, start)
	}
}

func (m *Model) findNextLogMatchForward(rawQuery string, start int) {
	// Check for another match on the current line after the cursor.
	if start >= 0 && start < len(m.logLines) {
		dl := m.logDisplayLine(start)
		runes := []rune(dl)
		// Clamp: logVisualCurCol carries the column from a previously
		// focused line and may exceed this line's rune length. Forward
		// uses +1 because the search starts after (not at) the cursor.
		end := min(m.logVisualCurCol+1, len(runes))
		curBytePos := len(string(runes[:end]))
		if curBytePos < len(dl) {
			col := ui.FindColumnInLine(dl[curBytePos:], rawQuery)
			if col >= 0 {
				m.logJumpToCol(start, m.logVisualCurCol+1+col)
				return
			}
		}
	}
	for i := start + 1; i < len(m.logLines); i++ {
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
	if start >= 0 && start < len(m.logLines) {
		dl := m.logDisplayLine(start)
		runes := []rune(dl)
		// Clamp: logVisualCurCol may exceed this line's rune length;
		// backward search ends at (excluding) the cursor.
		end := min(m.logVisualCurCol, len(runes))
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
	for i := len(m.logLines) - 1; i >= start; i-- {
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
