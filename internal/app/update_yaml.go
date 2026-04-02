package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
	lines := m.height - overhead
	if lines < 3 {
		lines = 3
	}
	return lines
}

func (m Model) handleYAMLKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Compute visible line count (accounting for collapsed sections).
	totalVisible := visibleLineCount(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	viewportLines := m.yamlViewportLines()
	maxScroll := totalVisible - viewportLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	// When in search input mode, handle text input.
	if m.yamlSearchMode {
		switch msg.String() {
		case "enter":
			m.yamlSearchMode = false
			m.updateYAMLSearchMatches()
			if len(m.yamlMatchLines) > 0 {
				m.yamlMatchIdx = m.findYAMLMatchFromCursor()
				m.yamlScrollToMatchFolded(viewportLines)
			}
			return m, nil
		case "esc":
			m.yamlSearchMode = false
			m.yamlSearchText.Clear()
			m.yamlMatchLines = nil
			m.yamlMatchIdx = 0
			return m, nil
		case "backspace":
			if len(m.yamlSearchText.Value) > 0 {
				m.yamlSearchText.Backspace()
			}
			return m, nil
		case "ctrl+w":
			m.yamlSearchText.DeleteWord()
			return m, nil
		case "ctrl+a":
			m.yamlSearchText.Home()
			return m, nil
		case "ctrl+e":
			m.yamlSearchText.End()
			return m, nil
		case "left":
			m.yamlSearchText.Left()
			return m, nil
		case "right":
			m.yamlSearchText.Right()
			return m, nil
		case "ctrl+c":
			m.yamlSearchMode = false
			m.yamlSearchText.Clear()
			m.yamlMatchLines = nil
			return m, nil
		default:
			if len(msg.String()) == 1 || msg.String() == " " {
				m.yamlSearchText.Insert(msg.String())
			}
			return m, nil
		}
	}

	// In visual selection mode, restrict keys to selection/copy/cancel.
	if m.yamlVisualMode {
		switch msg.String() {
		case "esc":
			m.yamlVisualMode = false
			return m, nil
		case "V":
			// Toggle: if already in line mode, cancel; otherwise switch to line mode.
			if m.yamlVisualType == 'V' {
				m.yamlVisualMode = false
			} else {
				m.yamlVisualType = 'V'
			}
			return m, nil
		case "v":
			// Toggle: if already in char mode, cancel; otherwise switch to char mode.
			if m.yamlVisualType == 'v' {
				m.yamlVisualMode = false
			} else {
				m.yamlVisualType = 'v'
			}
			return m, nil
		case "ctrl+v":
			// Toggle: if already in block mode, cancel; otherwise switch to block mode.
			if m.yamlVisualType == 'B' {
				m.yamlVisualMode = false
			} else {
				m.yamlVisualType = 'B'
			}
			return m, nil
		case "y":
			// Copy selected content to clipboard using original content (no fold indicators).
			yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
			_, mapping := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
			selStart := min(m.yamlVisualStart, m.yamlCursor)
			selEnd := max(m.yamlVisualStart, m.yamlCursor)
			if selStart < 0 {
				selStart = 0
			}
			if selEnd >= len(mapping) {
				selEnd = len(mapping) - 1
			}
			origLines := strings.Split(yamlForDisplay, "\n")
			var clipText string
			switch m.yamlVisualType {
			case 'v': // Character mode: partial first/last lines.
				var parts []string
				anchorCol := m.yamlVisualCol - yamlFoldPrefixLen
				cursorCol := m.yamlVisualCurCol - yamlFoldPrefixLen
				// Determine direction: assign columns to selStart/selEnd lines.
				startCol, endCol := anchorCol, cursorCol
				if m.yamlVisualStart > m.yamlCursor {
					// Upward selection: cursor is at selStart, anchor at selEnd.
					startCol, endCol = cursorCol, anchorCol
				}
				for i := selStart; i <= selEnd; i++ {
					if i >= len(mapping) || mapping[i] < 0 || mapping[i] >= len(origLines) {
						continue
					}
					line := origLines[mapping[i]]
					runes := []rune(line)
					if selStart == selEnd {
						// Single line: extract column range.
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
				clipText = strings.Join(parts, "\n")
			case 'B': // Block mode: rectangular column range.
				colStart := min(m.yamlVisualCol, m.yamlVisualCurCol) - yamlFoldPrefixLen
				colEnd := max(m.yamlVisualCol, m.yamlVisualCurCol) - yamlFoldPrefixLen + 1
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
				clipText = strings.Join(parts, "\n")
			default: // Line mode: whole lines.
				var selected []string
				for i := selStart; i <= selEnd; i++ {
					if i < len(mapping) && mapping[i] >= 0 && mapping[i] < len(origLines) {
						selected = append(selected, origLines[mapping[i]])
					}
				}
				clipText = strings.Join(selected, "\n")
			}
			lineCount := selEnd - selStart + 1
			m.yamlVisualMode = false
			m.setStatusMessage(fmt.Sprintf("Copied %d lines", lineCount), false)
			return m, tea.Batch(copyToSystemClipboard(clipText), scheduleStatusClear())
		case "h", "left":
			// Move cursor column left (for char and block modes).
			if m.yamlVisualType == 'v' || m.yamlVisualType == 'B' {
				if m.yamlVisualCurCol > yamlFoldPrefixLen {
					m.yamlVisualCurCol--
				}
			}
			return m, nil
		case "l", "right":
			// Move cursor column right (for char and block modes).
			if m.yamlVisualType == 'v' || m.yamlVisualType == 'B' {
				m.yamlVisualCurCol++
			}
			return m, nil
		case "j", "down":
			if m.yamlCursor < totalVisible-1 {
				m.yamlCursor++
			}
			m.ensureYAMLCursorVisible()
			return m, nil
		case "k", "up":
			if m.yamlCursor > 0 {
				m.yamlCursor--
			}
			m.ensureYAMLCursorVisible()
			return m, nil
		case "g":
			if m.pendingG {
				m.pendingG = false
				m.yamlCursor = 0
				m.yamlScroll = 0
				return m, nil
			}
			m.pendingG = true
			return m, nil
		case "G":
			m.yamlCursor = totalVisible - 1
			if m.yamlCursor < 0 {
				m.yamlCursor = 0
			}
			m.yamlScroll = maxScroll
			return m, nil
		case "ctrl+d":
			m.yamlCursor += m.height / 2
			if m.yamlCursor >= totalVisible {
				m.yamlCursor = totalVisible - 1
			}
			m.ensureYAMLCursorVisible()
			return m, nil
		case "ctrl+u":
			m.yamlCursor -= m.height / 2
			if m.yamlCursor < 0 {
				m.yamlCursor = 0
			}
			m.ensureYAMLCursorVisible()
			return m, nil
		case "ctrl+c":
			m.yamlVisualMode = false
			m.mode = modeExplorer
			m.yamlScroll = 0
			m.yamlCursor = 0
			return m, nil
		case "0":
			m.yamlVisualCurCol = yamlFoldPrefixLen
			return m, nil
		case "$":
			yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
			visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
			if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
				lineLen := len([]rune(visLines[m.yamlCursor]))
				if lineLen > 0 {
					m.yamlVisualCurCol = lineLen - 1
				}
			}
			return m, nil
		case "w":
			yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
			visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
			if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
				lineLen := len([]rune(visLines[m.yamlCursor]))
				newCol := nextWordStart(visLines[m.yamlCursor], m.yamlVisualCurCol)
				if newCol >= lineLen && m.yamlCursor < len(visLines)-1 {
					m.yamlCursor++
					newCol = nextWordStart(visLines[m.yamlCursor], 0)
					nextLineLen := len([]rune(visLines[m.yamlCursor]))
					if newCol >= nextLineLen {
						newCol = max(nextLineLen-1, 0)
					}
					m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
					m.ensureYAMLCursorVisible()
				} else {
					m.yamlVisualCurCol = newCol
				}
			}
			return m, nil
		case "b":
			yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
			visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
			if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
				newCol := prevWordStart(visLines[m.yamlCursor], m.yamlVisualCurCol)
				if newCol < 0 && m.yamlCursor > 0 {
					m.yamlCursor--
					lineLen := len([]rune(visLines[m.yamlCursor]))
					newCol = prevWordStart(visLines[m.yamlCursor], lineLen)
					if newCol < 0 {
						newCol = 0
					}
					m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
					m.ensureYAMLCursorVisible()
				} else {
					m.yamlVisualCurCol = max(yamlFoldPrefixLen, max(newCol, 0))
				}
			}
			return m, nil
		case "e":
			yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
			visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
			if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
				lineLen := len([]rune(visLines[m.yamlCursor]))
				newCol := wordEnd(visLines[m.yamlCursor], m.yamlVisualCurCol)
				if newCol >= lineLen && m.yamlCursor < len(visLines)-1 {
					m.yamlCursor++
					newCol = wordEnd(visLines[m.yamlCursor], 0)
					nextLineLen := len([]rune(visLines[m.yamlCursor]))
					if newCol >= nextLineLen {
						newCol = max(nextLineLen-1, 0)
					}
					m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
					m.ensureYAMLCursorVisible()
				} else {
					m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
				}
			}
			return m, nil
		case "E":
			yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
			visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
			if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
				lineLen := len([]rune(visLines[m.yamlCursor]))
				newCol := WORDEnd(visLines[m.yamlCursor], m.yamlVisualCurCol)
				if newCol >= lineLen && m.yamlCursor < len(visLines)-1 {
					m.yamlCursor++
					newCol = WORDEnd(visLines[m.yamlCursor], 0)
					nextLineLen := len([]rune(visLines[m.yamlCursor]))
					if newCol >= nextLineLen {
						newCol = max(nextLineLen-1, 0)
					}
					m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
					m.ensureYAMLCursorVisible()
				} else {
					m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
				}
			}
			return m, nil
		case "B":
			yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
			visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
			if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
				newCol := prevWORDStart(visLines[m.yamlCursor], m.yamlVisualCurCol)
				if newCol < 0 && m.yamlCursor > 0 {
					m.yamlCursor--
					lineLen := len([]rune(visLines[m.yamlCursor]))
					newCol = prevWORDStart(visLines[m.yamlCursor], lineLen)
					if newCol < 0 {
						newCol = 0
					}
					m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
					m.ensureYAMLCursorVisible()
				} else {
					m.yamlVisualCurCol = max(yamlFoldPrefixLen, max(newCol, 0))
				}
			}
			return m, nil
		case "W":
			yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
			visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
			if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
				lineLen := len([]rune(visLines[m.yamlCursor]))
				newCol := nextWORDStart(visLines[m.yamlCursor], m.yamlVisualCurCol)
				if newCol >= lineLen && m.yamlCursor < len(visLines)-1 {
					m.yamlCursor++
					newCol = nextWORDStart(visLines[m.yamlCursor], 0)
					nextLineLen := len([]rune(visLines[m.yamlCursor]))
					if newCol >= nextLineLen {
						newCol = max(nextLineLen-1, 0)
					}
					m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
					m.ensureYAMLCursorVisible()
				} else {
					m.yamlVisualCurCol = newCol
				}
			}
			return m, nil
		case "^":
			yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
			visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
			if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
				col := firstNonWhitespace(visLines[m.yamlCursor])
				if col < yamlFoldPrefixLen {
					col = yamlFoldPrefixLen
				}
				m.yamlVisualCurCol = col
			}
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "?":
		m.helpPreviousMode = modeYAML
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "YAML View"
		return m, nil
	case "V":
		// Enter visual line selection mode.
		m.yamlVisualMode = true
		m.yamlVisualType = 'V'
		m.yamlVisualStart = m.yamlCursor
		m.yamlVisualCol = m.yamlVisualCurCol
		return m, nil
	case "v":
		// Enter character visual selection mode; anchor at current cursor column.
		m.yamlVisualMode = true
		m.yamlVisualType = 'v'
		m.yamlVisualStart = m.yamlCursor
		m.yamlVisualCol = m.yamlVisualCurCol
		return m, nil
	case "ctrl+v":
		// Enter block visual selection mode; anchor at current cursor column.
		m.yamlVisualMode = true
		m.yamlVisualType = 'B'
		m.yamlVisualStart = m.yamlCursor
		m.yamlVisualCol = m.yamlVisualCurCol
		return m, nil
	case "q", "esc":
		if m.yamlSearchText.Value != "" {
			// Clear search first.
			m.yamlSearchText.Clear()
			m.yamlMatchLines = nil
			m.yamlMatchIdx = 0
			return m, nil
		}
		m.mode = modeExplorer
		m.yamlScroll = 0
		m.yamlCursor = 0
		return m, nil
	case "ctrl+c":
		m.mode = modeExplorer
		m.yamlScroll = 0
		m.yamlCursor = 0
		m.yamlSearchText.Clear()
		m.yamlMatchLines = nil
		return m, nil
	case "/":
		m.yamlSearchMode = true
		m.yamlSearchText.Clear()
		m.yamlMatchLines = nil
		m.yamlMatchIdx = 0
		return m, nil
	case "n":
		// Next match.
		if len(m.yamlMatchLines) > 0 {
			m.yamlMatchIdx = (m.yamlMatchIdx + 1) % len(m.yamlMatchLines)
			m.yamlScrollToMatchFolded(viewportLines)
		}
		return m, nil
	case "N":
		// Previous match.
		if len(m.yamlMatchLines) > 0 {
			m.yamlMatchIdx--
			if m.yamlMatchIdx < 0 {
				m.yamlMatchIdx = len(m.yamlMatchLines) - 1
			}
			m.yamlScrollToMatchFolded(viewportLines)
		}
		return m, nil
	case "ctrl+e":
		// Edit the resource in $EDITOR via kubectl edit.
		kind := m.selectedResourceKind()
		sel := m.selectedMiddleItem()
		if kind != "" && sel != nil {
			m.actionCtx = m.buildActionCtx(sel, kind)
			return m, m.execKubectlEdit()
		}
		return m, nil
	case "tab", "z":
		// Toggle fold on the section at the cursor position.
		_, mapping := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
		sec := sectionAtScrollPos(m.yamlCursor, mapping, m.yamlSections)
		if sec != "" {
			if m.yamlCollapsed == nil {
				m.yamlCollapsed = make(map[string]bool)
			}
			m.yamlCollapsed[sec] = !m.yamlCollapsed[sec]

			// Move cursor to the fold header line so it stays visible
			// after collapsing.
			if m.yamlCollapsed[sec] {
				// Find the section's startLine and locate it in the
				// new visible line mapping.
				var startLine int
				for _, s := range m.yamlSections {
					if s.key == sec {
						startLine = s.startLine
						break
					}
				}
				yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
				_, newMapping := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
				for vi, orig := range newMapping {
					if orig == startLine {
						m.yamlCursor = vi
						break
					}
				}
			}

			m.clampYAMLScroll()
			m.ensureYAMLCursorVisible()
		}
		return m, nil
	case "Z":
		// Toggle all folds: if any section is expanded, collapse all; otherwise expand all.
		if m.yamlCollapsed == nil {
			m.yamlCollapsed = make(map[string]bool)
		}
		anyExpanded := false
		for _, sec := range m.yamlSections {
			if isMultiLineSection(sec) && !m.yamlCollapsed[sec.key] {
				anyExpanded = true
				break
			}
		}
		if anyExpanded {
			for _, sec := range m.yamlSections {
				if isMultiLineSection(sec) {
					m.yamlCollapsed[sec.key] = true
				}
			}
		} else {
			m.yamlCollapsed = make(map[string]bool)
		}
		m.clampYAMLScroll()
		return m, nil
	case "h", "left":
		// Move cursor column left.
		if m.yamlVisualCurCol > yamlFoldPrefixLen {
			m.yamlVisualCurCol--
		}
		return m, nil
	case "l", "right":
		// Move cursor column right.
		m.yamlVisualCurCol++
		return m, nil
	case "0":
		// Move cursor to beginning of line.
		m.yamlVisualCurCol = yamlFoldPrefixLen
		return m, nil
	case "$":
		// Move cursor to end of current line.
		yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
		visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
		if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
			lineLen := len([]rune(visLines[m.yamlCursor]))
			if lineLen > 0 {
				m.yamlVisualCurCol = lineLen - 1
			}
		}
		return m, nil
	case "w":
		// Move cursor to next word start; jump to next line at end of line.
		yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
		visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
		if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
			lineLen := len([]rune(visLines[m.yamlCursor]))
			newCol := nextWordStart(visLines[m.yamlCursor], m.yamlVisualCurCol)
			if newCol >= lineLen && m.yamlCursor < len(visLines)-1 {
				m.yamlCursor++
				newCol = nextWordStart(visLines[m.yamlCursor], 0)
				nextLineLen := len([]rune(visLines[m.yamlCursor]))
				if newCol >= nextLineLen {
					newCol = max(nextLineLen-1, 0)
				}
				m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
				m.ensureYAMLCursorVisible()
			} else {
				m.yamlVisualCurCol = newCol
			}
		}
		return m, nil
	case "b":
		// Move cursor to previous word start; jump to previous line at start of line.
		yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
		visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
		if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
			newCol := prevWordStart(visLines[m.yamlCursor], m.yamlVisualCurCol)
			if newCol < 0 && m.yamlCursor > 0 {
				m.yamlCursor--
				lineLen := len([]rune(visLines[m.yamlCursor]))
				newCol = prevWordStart(visLines[m.yamlCursor], lineLen)
				if newCol < 0 {
					newCol = 0
				}
				m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
				m.ensureYAMLCursorVisible()
			} else {
				m.yamlVisualCurCol = max(yamlFoldPrefixLen, max(newCol, 0))
			}
		}
		return m, nil
	case "e":
		// Move cursor to end of current/next word; jump to next line at end of line.
		yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
		visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
		if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
			lineLen := len([]rune(visLines[m.yamlCursor]))
			newCol := wordEnd(visLines[m.yamlCursor], m.yamlVisualCurCol)
			if newCol >= lineLen && m.yamlCursor < len(visLines)-1 {
				m.yamlCursor++
				newCol = wordEnd(visLines[m.yamlCursor], 0)
				nextLineLen := len([]rune(visLines[m.yamlCursor]))
				if newCol >= nextLineLen {
					newCol = max(nextLineLen-1, 0)
				}
				m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
				m.ensureYAMLCursorVisible()
			} else {
				m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
			}
		}
		return m, nil
	case "E":
		// Move cursor to end of current/next WORD; jump to next line at end of line.
		yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
		visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
		if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
			lineLen := len([]rune(visLines[m.yamlCursor]))
			newCol := WORDEnd(visLines[m.yamlCursor], m.yamlVisualCurCol)
			if newCol >= lineLen && m.yamlCursor < len(visLines)-1 {
				m.yamlCursor++
				newCol = WORDEnd(visLines[m.yamlCursor], 0)
				nextLineLen := len([]rune(visLines[m.yamlCursor]))
				if newCol >= nextLineLen {
					newCol = max(nextLineLen-1, 0)
				}
				m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
				m.ensureYAMLCursorVisible()
			} else {
				m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
			}
		}
		return m, nil
	case "B":
		// Move cursor to previous WORD start; jump to previous line at start of line.
		yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
		visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
		if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
			newCol := prevWORDStart(visLines[m.yamlCursor], m.yamlVisualCurCol)
			if newCol < 0 && m.yamlCursor > 0 {
				m.yamlCursor--
				lineLen := len([]rune(visLines[m.yamlCursor]))
				newCol = prevWORDStart(visLines[m.yamlCursor], lineLen)
				if newCol < 0 {
					newCol = 0
				}
				m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
				m.ensureYAMLCursorVisible()
			} else {
				m.yamlVisualCurCol = max(yamlFoldPrefixLen, max(newCol, 0))
			}
		}
		return m, nil
	case "W":
		// Move cursor to next WORD start; jump to next line at end of line.
		yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
		visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
		if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
			lineLen := len([]rune(visLines[m.yamlCursor]))
			newCol := nextWORDStart(visLines[m.yamlCursor], m.yamlVisualCurCol)
			if newCol >= lineLen && m.yamlCursor < len(visLines)-1 {
				m.yamlCursor++
				newCol = nextWORDStart(visLines[m.yamlCursor], 0)
				nextLineLen := len([]rune(visLines[m.yamlCursor]))
				if newCol >= nextLineLen {
					newCol = max(nextLineLen-1, 0)
				}
				m.yamlVisualCurCol = max(yamlFoldPrefixLen, newCol)
				m.ensureYAMLCursorVisible()
			} else {
				m.yamlVisualCurCol = newCol
			}
		}
		return m, nil
	case "^":
		// Move cursor to first non-whitespace character.
		yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
		visLines, _ := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)
		if m.yamlCursor >= 0 && m.yamlCursor < len(visLines) {
			col := firstNonWhitespace(visLines[m.yamlCursor])
			if col < yamlFoldPrefixLen {
				col = yamlFoldPrefixLen
			}
			m.yamlVisualCurCol = col
		}
		return m, nil
	case "j", "down":
		if m.yamlCursor < totalVisible-1 {
			m.yamlCursor++
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "k", "up":
		if m.yamlCursor > 0 {
			m.yamlCursor--
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.yamlCursor = 0
			m.yamlScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		m.yamlCursor = totalVisible - 1
		if m.yamlCursor < 0 {
			m.yamlCursor = 0
		}
		m.yamlScroll = maxScroll
		return m, nil
	case "ctrl+d":
		m.yamlCursor += m.height / 2
		if m.yamlCursor >= totalVisible {
			m.yamlCursor = totalVisible - 1
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+u":
		m.yamlCursor -= m.height / 2
		if m.yamlCursor < 0 {
			m.yamlCursor = 0
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+f":
		m.yamlCursor += m.height
		if m.yamlCursor >= totalVisible {
			m.yamlCursor = totalVisible - 1
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	case "ctrl+b":
		m.yamlCursor -= m.height
		if m.yamlCursor < 0 {
			m.yamlCursor = 0
		}
		m.ensureYAMLCursorVisible()
		return m, nil
	}
	return m, nil
}

// ensureYAMLCursorVisible adjusts yamlScroll so the cursor is within the viewport.
func (m *Model) ensureYAMLCursorVisible() {
	maxLines := m.yamlViewportLines()
	if m.yamlCursor < m.yamlScroll {
		m.yamlScroll = m.yamlCursor
	}
	if m.yamlCursor >= m.yamlScroll+maxLines {
		m.yamlScroll = m.yamlCursor - maxLines + 1
	}
}

// clampYAMLScroll ensures yamlScroll stays within bounds after fold changes.
func (m *Model) clampYAMLScroll() {
	totalVisible := visibleLineCount(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	viewportLines := m.yamlViewportLines()
	maxScroll := totalVisible - viewportLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.yamlScroll > maxScroll {
		m.yamlScroll = maxScroll
	}
	if m.yamlScroll < 0 {
		m.yamlScroll = 0
	}
}

// yamlScrollToMatchFolded scrolls to show the current search match, expanding
// the containing section if it is collapsed, and using visible-line coordinates.
func (m *Model) yamlScrollToMatchFolded(viewportLines int) {
	if m.yamlMatchIdx < 0 || m.yamlMatchIdx >= len(m.yamlMatchLines) {
		return
	}
	targetOrig := m.yamlMatchLines[m.yamlMatchIdx]

	// If the match is inside a collapsed section, expand it.
	for _, sec := range m.yamlSections {
		if m.yamlCollapsed[sec.key] && targetOrig > sec.startLine && targetOrig <= sec.endLine {
			m.yamlCollapsed[sec.key] = false
		}
	}

	// Convert original line to visible line.
	_, mapping := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	visIdx := originalToVisible(targetOrig, mapping)
	if visIdx < 0 {
		return
	}

	totalVisible := len(mapping)
	maxScroll := totalVisible - viewportLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	// Move cursor to the match and center it in the viewport.
	m.yamlCursor = visIdx
	// Move cursor column to the match position within the line.
	yamlLines := strings.Split(m.yamlContent, "\n")
	if targetOrig >= 0 && targetOrig < len(yamlLines) {
		query := strings.ToLower(m.yamlSearchText.Value)
		col := strings.Index(strings.ToLower(yamlLines[targetOrig]), query)
		if col >= 0 {
			m.yamlVisualCurCol = len([]rune(yamlLines[targetOrig][:col]))
		}
	}
	m.yamlScroll = visIdx - viewportLines/2
	if m.yamlScroll > maxScroll {
		m.yamlScroll = maxScroll
	}
	if m.yamlScroll < 0 {
		m.yamlScroll = 0
	}
}

// updateYAMLSearchMatches finds all lines matching the current search text.
func (m *Model) updateYAMLSearchMatches() {
	m.yamlMatchLines = nil
	if m.yamlSearchText.Value == "" {
		return
	}
	query := strings.ToLower(m.yamlSearchText.Value)
	for i, line := range strings.Split(m.yamlContent, "\n") {
		if strings.Contains(strings.ToLower(line), query) {
			m.yamlMatchLines = append(m.yamlMatchLines, i)
		}
	}
}

// findYAMLMatchFromCursor returns the index of the first match at or after the
// current cursor position. Wraps to 0 if no match is found after the cursor.
func (m *Model) findYAMLMatchFromCursor() int {
	_, mapping := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)
	origLine := 0
	if m.yamlCursor >= 0 && m.yamlCursor < len(mapping) {
		origLine = mapping[m.yamlCursor]
	}
	for i, matchLine := range m.yamlMatchLines {
		if matchLine >= origLine {
			return i
		}
	}
	return 0
}
