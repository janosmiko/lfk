package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) viewYAML() string {
	yamlTitleText := m.yamlTitle()
	if m.yamlWrap {
		yamlTitleText += " [WRAP]"
	}
	if m.yamlVisualMode {
		switch m.yamlVisualType {
		case 'v':
			yamlTitleText += " [VISUAL]"
		case 'B':
			yamlTitleText += " [VISUAL BLOCK]"
		default:
			yamlTitleText += " [VISUAL LINE]"
		}
	}
	title := ui.TitleStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(yamlTitleText)
	var yamlHints []ui.HintEntry
	if m.yamlVisualMode {
		yamlHints = []ui.HintEntry{
			{Key: "j/k", Desc: "extend selection"},
			{Key: "y", Desc: "copy selected"},
			{Key: "v/V/ctrl+v", Desc: "switch mode"},
			{Key: "esc", Desc: "cancel"},
		}
	} else {
		yamlHints = []ui.HintEntry{
			{Key: "j/k", Desc: "scroll"},
			{Key: "g/G", Desc: "top/bottom"},
			{Key: "ctrl+d/u", Desc: "half page"},
			{Key: "ctrl+f/b", Desc: "page"},
			{Key: "/", Desc: "search"},
			{Key: "v/V/ctrl+v", Desc: "visual select"},
			{Key: "tab/z", Desc: "fold"},
			{Key: "ctrl+w/>", Desc: "wrap"},
			{Key: "ctrl+e", Desc: "edit"},
			{Key: "q/esc", Desc: "back"},
		}
	}
	hint := ui.RenderHintBar(yamlHints, m.width)

	// If search is active, show search bar instead of hints.
	if m.yamlSearchMode {
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.yamlSearchText.CursorLeft()) + ui.BarDimStyle.Render("\u2588") + ui.BarNormalStyle.Render(m.yamlSearchText.CursorRight())
		hint = ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(searchBar)
	} else if m.yamlSearchText.Value != "" {
		matchInfo := fmt.Sprintf(" [%d/%d]", m.yamlMatchIdx+1, len(m.yamlMatchLines))
		if len(m.yamlMatchLines) == 0 {
			matchInfo = " [no matches]"
		}
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.yamlSearchText.Value) + ui.BarDimStyle.Render(matchInfo)
		hint = ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(searchBar)
	}

	maxLines := m.height - 4
	if maxLines < 3 {
		maxLines = 3
	}

	// Build visible lines with fold indicators, respecting collapsed sections.
	// Mask secret data values when secret display is toggled off.
	yamlForDisplay := m.maskYAMLIfSecret(m.yamlContent)
	visLines, mapping := buildVisibleLines(yamlForDisplay, m.yamlSections, m.yamlCollapsed)

	yamlScroll := m.yamlScroll
	if yamlScroll >= len(visLines) {
		yamlScroll = len(visLines) - 1
	}
	if yamlScroll < 0 {
		yamlScroll = 0
	}
	viewport := visLines[yamlScroll:]
	if len(viewport) > maxLines {
		viewport = viewport[:maxLines]
	}

	// Compute line number gutter width.
	totalOrigLines := len(strings.Split(m.yamlContent, "\n"))
	gutterWidth := len(fmt.Sprintf("%d", totalOrigLines))
	if gutterWidth < 2 {
		gutterWidth = 2
	}

	// Build a set of original matching lines for search highlight.
	matchSet := make(map[int]bool)
	for _, ml := range m.yamlMatchLines {
		matchSet[ml] = true
	}
	currentMatchLine := -1
	if len(m.yamlMatchLines) > 0 && m.yamlMatchIdx >= 0 && m.yamlMatchIdx < len(m.yamlMatchLines) {
		currentMatchLine = m.yamlMatchLines[m.yamlMatchIdx]
	}

	// Clamp yamlCursor to valid range.
	if m.yamlCursor < 0 {
		m.yamlCursor = 0
	}
	if m.yamlCursor >= len(visLines) {
		m.yamlCursor = len(visLines) - 1
	}
	if m.yamlCursor < 0 {
		m.yamlCursor = 0
	}

	// Compute visual selection range (if active).
	selStart, selEnd := -1, -1
	if m.yamlVisualMode {
		selStart = min(m.yamlVisualStart, m.yamlCursor)
		selEnd = max(m.yamlVisualStart, m.yamlCursor)
	}

	// Compute column range for char/block visual modes.
	// For char mode: anchorCol on anchor line, cursorCol on cursor line.
	// For block mode: rectangular column range on every selected line.
	visualColStart, visualColEnd := 0, 0
	if m.yamlVisualMode && (m.yamlVisualType == 'v' || m.yamlVisualType == 'B') {
		visualColStart = min(m.yamlVisualCol, m.yamlCursorCol())
		visualColEnd = max(m.yamlVisualCol, m.yamlCursorCol())
	}

	// Content width for wrapping/truncation.
	contentWidth := m.width - 4 // border (2) + padding (2)
	if contentWidth < 10 {
		contentWidth = 10
	}

	// Apply YAML highlighting to visible lines, with search highlights and cursor.
	highlightedLines := make([]string, 0, len(viewport))
	for i, line := range viewport {
		visIdx := yamlScroll + i
		origLine := -1
		if visIdx < len(mapping) {
			origLine = mapping[visIdx]
		}
		// Separate fold prefix from content for column-accurate selection/cursor.
		// Use rune-based slicing because fold indicators are multi-byte UTF-8.
		foldPrefix := ""
		contentLine := line
		lineRunes := []rune(line)
		if len(lineRunes) > yamlFoldPrefixLen {
			foldPrefix = string(lineRunes[:yamlFoldPrefixLen])
			contentLine = string(lineRunes[yamlFoldPrefixLen:])
		}

		// When wrapping is enabled, split long content lines into sub-lines.
		if m.yamlWrap {
			// gutterOverhead: cursor indicator (1) + line number gutter + fold prefix
			gutterOverhead := 1 + gutterWidth + 1 + yamlFoldPrefixLen
			wrapWidth := contentWidth - gutterOverhead
			if wrapWidth < 10 {
				wrapWidth = 10
			}
			subLines := ui.WrapLine(contentLine, wrapWidth)
			for si, sub := range subLines {
				highlighted := ui.HighlightYAMLLine(sub)
				if m.yamlSearchText.Value != "" && origLine >= 0 && matchSet[origLine] {
					if origLine == currentMatchLine {
						highlighted = ui.HighlightSearchInLine(sub, m.yamlSearchText.Value, true)
					} else {
						highlighted = ui.HighlightSearchInLine(sub, m.yamlSearchText.Value, false)
					}
				}
				isSelected := m.yamlVisualMode && visIdx >= selStart && visIdx <= selEnd
				if isSelected && si == 0 {
					adjAnchorCol := m.yamlVisualCol - yamlFoldPrefixLen
					adjCursorCol := m.yamlCursorCol() - yamlFoldPrefixLen
					adjColStart := visualColStart - yamlFoldPrefixLen
					adjColEnd := visualColEnd - yamlFoldPrefixLen
					highlighted = ui.RenderVisualSelection(sub, m.yamlVisualType, visIdx, selStart, selEnd, m.yamlVisualStart, adjAnchorCol, adjCursorCol, adjColStart, adjColEnd)
				}
				if si == 0 {
					// First sub-line: show line number and fold prefix.
					lineNumStr := strings.Repeat(" ", gutterWidth+1)
					if origLine >= 0 {
						lineNumStr = fmt.Sprintf("%*d ", gutterWidth, origLine+1)
					}
					if visIdx == m.yamlCursor {
						if m.yamlVisualMode {
							highlighted = ui.YamlCursorIndicatorStyle.Render("\u258e") + ui.DimStyle.Render(lineNumStr) + foldPrefix + highlighted
						} else {
							highlighted = ui.YamlCursorIndicatorStyle.Render("\u258e") + ui.DimStyle.Render(lineNumStr) + foldPrefix + ui.RenderCursorAtCol(highlighted, sub, m.yamlVisualCurCol-yamlFoldPrefixLen)
						}
					} else if isSelected {
						highlighted = ui.YamlCursorIndicatorStyle.Render(" ") + ui.DimStyle.Render(lineNumStr) + foldPrefix + highlighted
					} else {
						highlighted = " " + ui.DimStyle.Render(lineNumStr) + foldPrefix + highlighted
					}
				} else {
					// Continuation sub-line: indent with padding to align with content.
					pad := strings.Repeat(" ", 1+gutterWidth+1+yamlFoldPrefixLen+2)
					highlighted = pad + highlighted
				}
				highlightedLines = append(highlightedLines, highlighted)
				if len(highlightedLines) >= maxLines {
					break
				}
			}
		} else {
			highlighted := ui.HighlightYAMLLine(contentLine)
			if m.yamlSearchText.Value != "" && origLine >= 0 && matchSet[origLine] {
				if origLine == currentMatchLine {
					highlighted = ui.HighlightSearchInLine(contentLine, m.yamlSearchText.Value, true)
				} else {
					highlighted = ui.HighlightSearchInLine(contentLine, m.yamlSearchText.Value, false)
				}
			}
			// Visual selection highlight: override with selected style.
			isSelected := m.yamlVisualMode && visIdx >= selStart && visIdx <= selEnd
			if isSelected {
				adjAnchorCol := m.yamlVisualCol - yamlFoldPrefixLen
				adjCursorCol := m.yamlCursorCol() - yamlFoldPrefixLen
				adjColStart := visualColStart - yamlFoldPrefixLen
				adjColEnd := visualColEnd - yamlFoldPrefixLen
				highlighted = ui.RenderVisualSelection(contentLine, m.yamlVisualType, visIdx, selStart, selEnd, m.yamlVisualStart, adjAnchorCol, adjCursorCol, adjColStart, adjColEnd)
			}
			// Line number gutter
			lineNumStr := strings.Repeat(" ", gutterWidth+1)
			if origLine >= 0 {
				lineNumStr = fmt.Sprintf("%*d ", gutterWidth, origLine+1)
			}
			// Cursor indicator + line number + content
			if visIdx == m.yamlCursor {
				if m.yamlVisualMode {
					// In visual mode, don't overlay block cursor on top of visual selection styling.
					highlighted = ui.YamlCursorIndicatorStyle.Render("\u258e") + ui.DimStyle.Render(lineNumStr) + foldPrefix + highlighted
				} else {
					highlighted = ui.YamlCursorIndicatorStyle.Render("\u258e") + ui.DimStyle.Render(lineNumStr) + foldPrefix + ui.RenderCursorAtCol(highlighted, contentLine, m.yamlVisualCurCol-yamlFoldPrefixLen)
				}
			} else if isSelected {
				highlighted = ui.YamlCursorIndicatorStyle.Render(" ") + ui.DimStyle.Render(lineNumStr) + foldPrefix + highlighted
			} else {
				highlighted = " " + ui.DimStyle.Render(lineNumStr) + foldPrefix + highlighted
			}
			highlightedLines = append(highlightedLines, highlighted)
		}

		if len(highlightedLines) >= maxLines {
			break
		}
	}

	// Pad to fill available height so the hint bar stays at the bottom.
	for len(highlightedLines) < maxLines {
		highlightedLines = append(highlightedLines, "")
	}

	// Truncate lines that exceed the content area width to prevent lipgloss
	// from wrapping them internally, which would push the bottom border off screen.
	for i, line := range highlightedLines {
		if lipgloss.Width(line) > contentWidth {
			highlightedLines[i] = ansi.Truncate(line, contentWidth, "")
		}
	}
	bodyContent := strings.Join(highlightedLines, "\n")
	// Fill background so ANSI resets from styled segments don't leave gaps.
	bodyContent = ui.FillLinesBg(bodyContent, contentWidth, ui.BaseBg)
	borderStyle := ui.FullscreenBorderStyle(m.width, maxLines)
	body := borderStyle.Render(bodyContent)

	return lipgloss.JoinVertical(lipgloss.Left, title, body, hint)
}

func (m Model) yamlTitle() string {
	switch m.nav.Level {
	case model.LevelResources:
		sel := m.selectedMiddleItem()
		if sel != nil {
			return fmt.Sprintf("YAML: %s/%s", m.namespace, sel.Name)
		}
	case model.LevelOwned:
		sel := m.selectedMiddleItem()
		if sel != nil {
			return fmt.Sprintf("YAML: %s/%s", m.namespace, sel.Name)
		}
	case model.LevelContainers:
		return fmt.Sprintf("YAML: %s/%s", m.namespace, m.nav.OwnedName)
	}
	return "YAML"
}

// yamlCursorCol returns the current cursor column position within the YAML line.
func (m Model) yamlCursorCol() int {
	return m.yamlVisualCurCol
}
