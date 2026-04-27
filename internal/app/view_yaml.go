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
			{Key: "123G", Desc: "goto"},
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
		yamlModeInd := ui.SearchModeIndicator(m.yamlSearchText.Value)
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarDimStyle.Render(yamlModeInd) + ui.BarNormalStyle.Render(m.yamlSearchText.CursorLeft()) + ui.BarDimStyle.Render("\u2588") + ui.BarNormalStyle.Render(m.yamlSearchText.CursorRight())
		hint = ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(searchBar)
	} else if m.yamlSearchText.Value != "" {
		matchInfo := fmt.Sprintf(" [%d/%d]", m.yamlMatchIdx+1, len(m.yamlMatchLines))
		if len(m.yamlMatchLines) == 0 {
			matchInfo = " [no matches]"
		}
		nav := ""
		if len(m.yamlMatchLines) > 0 {
			nav = ui.BarDimStyle.Render(" | ") + ui.HelpKeyStyle.Render("n/N") + ui.BarDimStyle.Render(": next/prev")
		}
		searchBar := ui.HelpKeyStyle.Render("/") + ui.BarNormalStyle.Render(m.yamlSearchText.Value) + ui.BarDimStyle.Render(matchInfo) + nav
		hint = ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(searchBar)
	}

	maxLines := max(m.height-4, 3)

	// Build visible lines with fold indicators, respecting collapsed sections.
	visLines, mapping := buildVisibleLines(m.yamlContent, m.yamlSections, m.yamlCollapsed)

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
	gutterWidth := max(len(fmt.Sprintf("%d", totalOrigLines)), 2)

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
	contentWidth := max(
		// border (2) + padding (2)
		m.width-4, 10)

	// Apply YAML highlighting to visible lines, with search highlights and cursor.
	renderCtx := yamlRenderCtx{
		yamlScroll:     yamlScroll,
		mapping:        mapping,
		matchSet:       matchSet,
		currentMatch:   currentMatchLine,
		searchQuery:    m.yamlSearchText.Value,
		gutterWidth:    gutterWidth,
		contentWidth:   contentWidth,
		maxLines:       maxLines,
		yamlCursor:     m.yamlCursor,
		visualMode:     m.yamlVisualMode,
		visualType:     m.yamlVisualType,
		visualStart:    m.yamlVisualStart,
		visualCol:      m.yamlVisualCol,
		cursorCol:      m.yamlCursorCol(),
		visualCurCol:   m.yamlVisualCurCol,
		selStart:       selStart,
		selEnd:         selEnd,
		visualColStart: visualColStart,
		visualColEnd:   visualColEnd,
		wrap:           m.yamlWrap,
	}
	highlightedLines := renderYAMLViewportLines(viewport, renderCtx)

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

// yamlRenderCtx holds the rendering parameters for YAML viewport lines.
type yamlRenderCtx struct {
	yamlScroll                          int
	mapping                             []int
	matchSet                            map[int]bool
	currentMatch                        int
	searchQuery                         string
	gutterWidth, contentWidth, maxLines int
	yamlCursor                          int
	visualMode                          bool
	visualType                          rune
	visualStart, visualCol              int
	cursorCol, visualCurCol             int
	selStart, selEnd                    int
	visualColStart, visualColEnd        int
	wrap                                bool
}

// renderYAMLViewportLines renders visible YAML lines with highlighting, search, and cursor.
func renderYAMLViewportLines(viewport []string, ctx yamlRenderCtx) []string {
	highlighted := make([]string, 0, len(viewport))
	for i, line := range viewport {
		visIdx := ctx.yamlScroll + i
		origLine := -1
		if visIdx < len(ctx.mapping) {
			origLine = ctx.mapping[visIdx]
		}
		foldPrefix, contentLine := splitFoldPrefix(line)

		if ctx.wrap {
			highlighted = renderYAMLWrappedLine(highlighted, contentLine, foldPrefix, visIdx, origLine, ctx)
		} else {
			highlighted = renderYAMLNonWrappedLine(highlighted, contentLine, foldPrefix, visIdx, origLine, ctx)
		}
		if len(highlighted) >= ctx.maxLines {
			break
		}
	}
	return highlighted
}

// splitFoldPrefix separates the fold indicator prefix from the YAML content.
func splitFoldPrefix(line string) (string, string) {
	lineRunes := []rune(line)
	if len(lineRunes) > yamlFoldPrefixLen {
		return string(lineRunes[:yamlFoldPrefixLen]), string(lineRunes[yamlFoldPrefixLen:])
	}
	return "", line
}

// yamlLineNumStr returns the formatted line number string.
func yamlLineNumStr(origLine, gutterWidth int) string {
	if origLine >= 0 {
		return fmt.Sprintf("%*d ", gutterWidth, origLine+1)
	}
	return strings.Repeat(" ", gutterWidth+1)
}

// yamlApplySearchHighlight applies search highlighting to a YAML line.
func yamlApplySearchHighlight(text, searchQuery string, origLine, currentMatch int, matchSet map[int]bool) string {
	if searchQuery == "" || origLine < 0 || !matchSet[origLine] {
		return ui.HighlightYAMLLine(text)
	}
	return ui.HighlightSearchInLine(text, searchQuery, origLine == currentMatch)
}

// yamlAdjustedCols returns the column values adjusted for fold prefix offset.
func yamlAdjustedCols(ctx yamlRenderCtx) (anchorCol, cursorCol, colStart, colEnd int) {
	return ctx.visualCol - yamlFoldPrefixLen,
		ctx.cursorCol - yamlFoldPrefixLen,
		ctx.visualColStart - yamlFoldPrefixLen,
		ctx.visualColEnd - yamlFoldPrefixLen
}

// yamlPrependGutter prepends the cursor indicator, line number, and fold prefix.
func yamlPrependGutter(content, lineNum, foldPrefix string, isCursor, isSelected, visualMode bool, curCol int, rawContent string) string {
	if isCursor {
		if visualMode {
			return ui.YamlCursorIndicatorStyle.Render("\u258e") + ui.DimStyle.Render(lineNum) + foldPrefix + content
		}
		return ui.YamlCursorIndicatorStyle.Render("\u258e") + ui.DimStyle.Render(lineNum) + foldPrefix + ui.RenderCursorAtCol(content, rawContent, curCol)
	}
	if isSelected {
		return ui.YamlCursorIndicatorStyle.Render(" ") + ui.DimStyle.Render(lineNum) + foldPrefix + content
	}
	return " " + ui.DimStyle.Render(lineNum) + foldPrefix + content
}

// renderYAMLWrappedLine renders a single YAML line with word wrapping.
func renderYAMLWrappedLine(result []string, contentLine, foldPrefix string, visIdx, origLine int, ctx yamlRenderCtx) []string {
	gutterOverhead := 1 + ctx.gutterWidth + 1 + yamlFoldPrefixLen
	wrapWidth := max(ctx.contentWidth-gutterOverhead, 10)
	subLines := ui.WrapLine(contentLine, wrapWidth)
	for si, sub := range subLines {
		hl := yamlApplySearchHighlight(sub, ctx.searchQuery, origLine, ctx.currentMatch, ctx.matchSet)
		isSelected := ctx.visualMode && visIdx >= ctx.selStart && visIdx <= ctx.selEnd
		if isSelected && si == 0 {
			adjAnchor, adjCursor, adjColStart, adjColEnd := yamlAdjustedCols(ctx)
			hl = ui.RenderVisualSelection(sub, ctx.visualType, visIdx, ctx.selStart, ctx.selEnd, ctx.visualStart, adjAnchor, adjCursor, adjColStart, adjColEnd)
		}
		if si == 0 {
			lineNum := yamlLineNumStr(origLine, ctx.gutterWidth)
			hl = yamlPrependGutter(hl, lineNum, foldPrefix, visIdx == ctx.yamlCursor, isSelected, ctx.visualMode, ctx.visualCurCol-yamlFoldPrefixLen, sub)
		} else {
			pad := strings.Repeat(" ", 1+ctx.gutterWidth+1+yamlFoldPrefixLen+2)
			hl = pad + hl
		}
		result = append(result, hl)
		if len(result) >= ctx.maxLines {
			break
		}
	}
	return result
}

// renderYAMLNonWrappedLine renders a single YAML line without wrapping.
func renderYAMLNonWrappedLine(result []string, contentLine, foldPrefix string, visIdx, origLine int, ctx yamlRenderCtx) []string {
	hl := yamlApplySearchHighlight(contentLine, ctx.searchQuery, origLine, ctx.currentMatch, ctx.matchSet)
	isSelected := ctx.visualMode && visIdx >= ctx.selStart && visIdx <= ctx.selEnd
	if isSelected {
		adjAnchor, adjCursor, adjColStart, adjColEnd := yamlAdjustedCols(ctx)
		hl = ui.RenderVisualSelection(contentLine, ctx.visualType, visIdx, ctx.selStart, ctx.selEnd, ctx.visualStart, adjAnchor, adjCursor, adjColStart, adjColEnd)
	}
	lineNum := yamlLineNumStr(origLine, ctx.gutterWidth)
	hl = yamlPrependGutter(hl, lineNum, foldPrefix, visIdx == ctx.yamlCursor, isSelected, ctx.visualMode, ctx.visualCurCol-yamlFoldPrefixLen, contentLine)
	return append(result, hl)
}
