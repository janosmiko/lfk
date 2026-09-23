package app

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) viewYAML() string {
	// The schema pane takes columns off the right. The hint bar keeps the full
	// terminal width and goes under both panes, or it would lose entries to
	// the narrower column. Render the pane first, then narrow m.width so the
	// viewer lays itself out in what is left; m is a value copy, so the
	// narrowing stays inside this render.
	fullWidth := m.width
	schemaPane := m.renderFieldDocPane(m.height, true)
	if schemaPane != "" {
		m.width -= lipgloss.Width(schemaPane)
	}

	yamlTitleText := m.yamlTitle()
	if m.yamlView.wrap {
		yamlTitleText += " [WRAP]"
	}
	if m.yamlView.visualMode {
		switch m.yamlView.visualType {
		case 'v':
			yamlTitleText += " [VISUAL]"
		case 'B':
			yamlTitleText += " [VISUAL BLOCK]"
		default:
			yamlTitleText += " [VISUAL LINE]"
		}
	}
	title := ui.ViewTitle(m.width, yamlTitleText)
	hint := m.yamlHintBar(fullWidth)

	maxLines := max(m.height-4, 3)
	// Content width for wrapping/truncation: border (2) + padding (2).
	contentWidth := max(m.width-4, 10)

	// Build visible lines with fold indicators, respecting collapsed sections.
	visLines, mapping := m.yamlVisibleLines()

	// Clamp the cursor before anything reads it. Collapsing a fold shortens
	// visLines without moving the cursor, and the scroll offset below treats an
	// out-of-range cursor as no offset, which strands it off the bottom.
	if m.yamlView.cursor < 0 {
		m.yamlView.cursor = 0
	}
	if m.yamlView.cursor >= len(visLines) {
		m.yamlView.cursor = len(visLines) - 1
	}
	if m.yamlView.cursor < 0 {
		m.yamlView.cursor = 0
	}

	yamlScroll := m.yamlView.scroll
	if yamlScroll >= len(visLines) {
		yamlScroll = len(visLines) - 1
	}
	if yamlScroll < 0 {
		yamlScroll = 0
	}
	// Compute line number gutter width.
	totalOrigLines := len(strings.Split(m.yamlView.content, "\n"))
	gutterWidth := max(len(fmt.Sprintf("%d", totalOrigLines)), 2)

	topSkip := 0
	if m.yamlView.wrap {
		topSkip = yamlWrapTopSkip(visLines, yamlScroll, m.yamlView.cursor,
			yamlWrapWidth(contentWidth, gutterWidth), maxLines,
			m.yamlCursorCol()-yamlFoldPrefixLen)
	}
	// Each source line fills at least one row, so this many of them always
	// covers the viewport plus the rows scrolled off its top.
	viewport := visLines[yamlScroll:min(yamlScroll+maxLines+topSkip, len(visLines))]

	// Build a set of original matching lines for search highlight.
	matchSet := make(map[int]bool)
	for _, ml := range m.yamlView.matchLines {
		matchSet[ml] = true
	}
	currentMatchLine := -1
	if len(m.yamlView.matchLines) > 0 && m.yamlView.matchIdx >= 0 && m.yamlView.matchIdx < len(m.yamlView.matchLines) {
		currentMatchLine = m.yamlView.matchLines[m.yamlView.matchIdx]
	}

	// Compute visual selection range (if active).
	selStart, selEnd := -1, -1
	if m.yamlView.visualMode {
		selStart = min(m.yamlView.visualStart, m.yamlView.cursor)
		selEnd = max(m.yamlView.visualStart, m.yamlView.cursor)
	}

	// Compute column range for char/block visual modes.
	// For char mode: anchorCol on anchor line, cursorCol on cursor line.
	// For block mode: rectangular column range on every selected line.
	visualColStart, visualColEnd := 0, 0
	if m.yamlView.visualMode && (m.yamlView.visualType == 'v' || m.yamlView.visualType == 'B') {
		visualColStart = min(m.yamlView.visualCol, m.yamlCursorCol())
		visualColEnd = max(m.yamlView.visualCol, m.yamlCursorCol())
	}

	// Apply YAML highlighting to visible lines, with search highlights and cursor.
	renderCtx := yamlRenderCtx{
		blame:          m.yamlView.blame,
		blameInline:    m.yamlView.blameOn,
		yamlScroll:     yamlScroll,
		mapping:        mapping,
		matchSet:       matchSet,
		currentMatch:   currentMatchLine,
		searchQuery:    m.yamlView.searchText.Value,
		gutterWidth:    gutterWidth,
		contentWidth:   contentWidth,
		maxLines:       maxLines,
		yamlCursor:     m.yamlView.cursor,
		visualMode:     m.yamlView.visualMode,
		visualType:     m.yamlView.visualType,
		visualStart:    m.yamlView.visualStart,
		visualCol:      m.yamlView.visualCol,
		cursorCol:      m.yamlCursorCol(),
		visualCurCol:   m.yamlView.visualCurCol,
		selStart:       selStart,
		selEnd:         selEnd,
		visualColStart: visualColStart,
		visualColEnd:   visualColEnd,
		wrap:           m.yamlView.wrap,
		topSkip:        topSkip,
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

	if schemaPane != "" {
		panes := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.JoinVertical(lipgloss.Left, title, body), schemaPane)
		return lipgloss.JoinVertical(lipgloss.Left, panes, hint)
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, body, hint)
}

// yamlHintBar builds the YAML viewer's hint bar, split from viewYAML to keep
// cyclomatic complexity under 30. Status messages and search prompts take
// precedence over the plain hint bar — same pattern the log viewer uses.
func (m Model) yamlHintBar(fullWidth int) string {
	var yamlHints []ui.HintEntry
	if m.yamlView.visualMode {
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
			{Key: ui.ActiveKeybindings.Search, Desc: "search"},
			{Key: "123G", Desc: "goto"},
			{Key: "v/V/ctrl+v", Desc: "visual select"},
			{Key: "y", Desc: "copy"},
			{Key: ui.ActiveKeybindings.ToggleFold, Desc: "fold"},
			{Key: ui.ActiveKeybindings.ToggleWrap, Desc: "wrap"},
			{Key: "K", Desc: "kyaml"},
			{Key: "m", Desc: "blame"},
			{Key: "ctrl+e", Desc: "edit"},
			{Key: "O", Desc: "object explorer"},
			{Key: "I", Desc: "explain"},
			{Key: ui.ActiveKeybindings.FieldDoc, Desc: "schema"},
			{Key: "q/esc", Desc: "back"},
		}
	}
	hint := ui.RenderHintBar(yamlHints, fullWidth)

	switch {
	case m.hasStatusMessage():
		hint = m.renderStatusHint()
	case m.yamlView.searchMode:
		yamlModeInd := ui.SearchModeIndicator(m.yamlView.searchText.Value)
		yamlModeHint := ui.FormatHintParts([]ui.HintEntry{ui.SearchModeHintEntry()})
		searchBar := ui.HelpKeyStyle.Render(ui.ActiveKeybindings.Search) + ui.BarDimStyle.Render(yamlModeInd) + ui.BarNormalStyle.Render(m.yamlView.searchText.CursorLeft()) + ui.BarDimStyle.Render("█") + ui.BarNormalStyle.Render(m.yamlView.searchText.CursorRight()) + ui.BarDimStyle.Render("  ") + yamlModeHint
		hint = ui.StatusBarBgStyle.Width(fullWidth).MaxWidth(fullWidth).MaxHeight(1).Render(searchBar)
	case m.yamlView.searchText.Value != "":
		matchInfo := fmt.Sprintf(" [%d/%d]", m.yamlView.matchIdx+1, len(m.yamlView.matchLines))
		if len(m.yamlView.matchLines) == 0 {
			matchInfo = " [no matches]"
		}
		nav := ""
		if len(m.yamlView.matchLines) > 0 {
			nav = ui.BarDimStyle.Render(" | ") + ui.HelpKeyStyle.Render(ui.ActiveKeybindings.NextMatch+"/"+ui.ActiveKeybindings.PrevMatch) + ui.BarDimStyle.Render(": next/prev")
		}
		searchBar := ui.HelpKeyStyle.Render(ui.ActiveKeybindings.Search) + ui.BarNormalStyle.Render(m.yamlView.searchText.Value) + ui.BarDimStyle.Render(matchInfo) + nav
		hint = ui.StatusBarBgStyle.Width(fullWidth).MaxWidth(fullWidth).MaxHeight(1).Render(searchBar)
	}
	return hint
}

func (m Model) yamlTitle() string {
	if label := m.yamlResourceLabel(); label != "" {
		return "YAML: " + label
	}
	return "YAML"
}

// yamlResourceLabel formats the displayed resource as "Kind namespace/name"
// (see resourceTitleLabel) so the YAML sub-title matches the Object Explorer
// and the other viewers for the same resource.
func (m Model) yamlResourceLabel() string {
	switch m.nav.Level {
	case model.LevelResources, model.LevelOwned:
		if sel := m.selectedMiddleItem(); sel != nil {
			ns := sel.Namespace
			if ns == "" {
				ns = m.namespace
			}
			return resourceTitleLabel(sel.Kind, ns, sel.Name)
		}
	case model.LevelContainers:
		return resourceTitleLabel("Pod", m.namespace, m.nav.OwnedName)
	}
	return ""
}

// yamlResourceName returns the name of the resource whose YAML is displayed,
// or "" when it can't be resolved. Used by the top breadcrumb's drill path
// (see explorerDrillPath).
func (m Model) yamlResourceName() string {
	switch m.nav.Level {
	case model.LevelResources, model.LevelOwned:
		if sel := m.selectedMiddleItem(); sel != nil {
			return sel.Name
		}
	case model.LevelContainers:
		return m.nav.OwnedName
	}
	return ""
}

// yamlCursorCol returns the current cursor column position within the YAML line.
func (m Model) yamlCursorCol() int {
	return m.yamlView.visualCurCol
}

// yamlRenderCtx holds the rendering parameters for YAML viewport lines.
type yamlRenderCtx struct {
	yamlScroll                          int
	mapping                             []int
	matchSet                            map[int]bool
	currentMatch                        int
	searchQuery                         string
	gutterWidth, contentWidth, maxLines int
	blame                               []blameLine
	blameInline                         bool
	yamlCursor                          int
	visualMode                          bool
	visualType                          rune
	visualStart, visualCol              int
	cursorCol, visualCurCol             int
	selStart, selEnd                    int
	visualColStart, visualColEnd        int
	wrap                                bool
	// topSkip is how many wrapped rows the viewport drops off its top.
	topSkip int
}

// rowBudget is how many rows the renderer may produce, the visible ones plus
// the ones scrolled off the top.
func (c yamlRenderCtx) rowBudget() int { return c.maxLines + c.topSkip }

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
		if len(highlighted) >= ctx.rowBudget() {
			break
		}
	}
	return highlighted[min(ctx.topSkip, len(highlighted)):]
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
// The caller draws the block cursor first, so on a wrapped line it can land on
// the sub-line that holds its column rather than always on the first one.
func yamlPrependGutter(content, lineNum, foldPrefix string, isCursor, isSelected bool) string {
	switch {
	case isCursor:
		return ui.YamlCursorIndicatorStyle.Render("\u258e") + ui.DimStyle.Render(lineNum) + foldPrefix + content
	case isSelected:
		return ui.YamlCursorIndicatorStyle.Render(" ") + ui.DimStyle.Render(lineNum) + foldPrefix + content
	default:
		return " " + ui.DimStyle.Render(lineNum) + foldPrefix + content
	}
}

// yamlBlameInline renders the field-manager note that trails the cursor line,
// the way an editor shows git blame as virtual text. Only the cursor line gets
// it: managedFields records one timestamp per manager entry, so printing it on
// every line would repeat the same value down the whole document.
//
// used is the display width the line already occupies.
func yamlBlameInline(ctx yamlRenderCtx, origLine, used int) string {
	if !ctx.blameInline || ctx.visualMode || origLine < 0 || origLine >= len(ctx.blame) {
		return ""
	}
	entry := ctx.blame[origLine]
	if entry.manager == "" {
		return ""
	}
	text := "  " + strings.Join(yamlBlameParts(entry), " \u2022 ")
	available := ctx.contentWidth - used
	if available < yamlBlameMinInlineWidth {
		return ""
	}
	return ui.FieldManagerStyle(entry.manager, true).Render(ui.Truncate(text, available))
}

func yamlBlameParts(entry blameLine) []string {
	parts := make([]string, 0, 4)
	parts = append(parts, entry.manager)
	if entry.owner.Operation != "" {
		parts = append(parts, entry.owner.Operation)
	}
	if !entry.owner.Time.IsZero() {
		parts = append(parts, ui.FormatAge(entry.owner.Time)+" ago")
	}
	if entry.rolled {
		parts = append(parts, "inherited")
	}
	return parts
}

// yamlWrapIndent is the extra indent a wrapped continuation sub-line carries
// past the gutter, so it reads as a continuation and not a new key.
const yamlWrapIndent = 2

// yamlGutterOverhead is the width the cursor bar, line number, and fold prefix
// take before the content starts.
func yamlGutterOverhead(gutterWidth int) int {
	return 1 + gutterWidth + 1 + yamlFoldPrefixLen
}

// yamlWrapWidth returns how many columns a wrapped row can hold. Continuation
// rows carry an extra indent, so the width has to budget for it or they render
// wider than the content area and the outer width guard cuts their tail.
func yamlWrapWidth(contentWidth, gutterWidth int) int {
	return max(contentWidth-yamlGutterOverhead(gutterWidth)-yamlWrapIndent, 10)
}

// yamlCursorSubLine returns the index of the wrapped row that carries the block
// cursor.
func yamlCursorSubLine(subs []string, srcWidth, wrapWidth, col int) int {
	if len(subs) == 0 {
		return -1
	}
	base := 0
	for i, text := range subs {
		sub := ui.SubLine{Text: text, Base: base, SrcWidth: srcWidth, WrapWidth: wrapWidth}
		if sub.OwnsCursor(col) {
			return i
		}
		base += sub.Width()
	}
	return len(subs) - 1
}

// yamlWrapTopSkip returns how many wrapped rows to drop off the top so the
// cursor's own row stays on screen. The viewer scrolls whole source lines, so
// a line taller than the viewport would otherwise never show its tail.
func yamlWrapTopSkip(visLines []string, scroll, cursor, wrapWidth, maxLines, cursorCol int) int {
	if maxLines <= 0 || cursor < scroll || cursor >= len(visLines) {
		return 0
	}
	row := 0
	for i := scroll; i < cursor; i++ {
		_, content := splitFoldPrefix(visLines[i])
		row += len(ui.WrapLine(content, wrapWidth))
	}
	_, content := splitFoldPrefix(visLines[cursor])
	subs := ui.WrapLine(content, wrapWidth)
	row += max(yamlCursorSubLine(subs, lipgloss.Width(content), wrapWidth, cursorCol), 0)
	return max(row-maxLines+1, 0)
}

// renderYAMLWrappedLine renders a single YAML line with word wrapping.
func renderYAMLWrappedLine(result []string, contentLine, foldPrefix string, visIdx, origLine int, ctx yamlRenderCtx) []string {
	gutterOverhead := yamlGutterOverhead(ctx.gutterWidth)
	wrapWidth := yamlWrapWidth(ctx.contentWidth, ctx.gutterWidth)
	subLines := ui.WrapLine(contentLine, wrapWidth)
	srcWidth := lipgloss.Width(contentLine)
	adjAnchor, adjCursor, _, _ := yamlAdjustedCols(ctx)
	isSelected := ctx.visualMode && visIdx >= ctx.selStart && visIdx <= ctx.selEnd
	cursorSub := -1
	if visIdx == ctx.yamlCursor && !ctx.visualMode {
		cursorSub = yamlCursorSubLine(subLines, srcWidth, wrapWidth, adjCursor)
	}
	// base tracks where each sub-line starts in content-line columns. A wide
	// rune can stop a sub-line short of wrapWidth, so accumulate measured
	// widths instead of multiplying by the wrap width.
	base := 0
	for si, text := range subLines {
		sub := ui.SubLine{Text: text, Base: base, SrcWidth: srcWidth, WrapWidth: wrapWidth}
		base += sub.Width()

		hl := yamlApplySearchHighlight(text, ctx.searchQuery, origLine, ctx.currentMatch, ctx.matchSet)
		if isSelected {
			hl = ui.RenderVisualSelectionSub(sub, ctx.visualType, visIdx, ctx.selStart, ctx.selEnd, ctx.visualStart, adjAnchor, adjCursor)
		}
		if si == cursorSub {
			hl = ui.RenderCursorAtCol(hl, sub.ClampCol(adjCursor))
		}
		if si == 0 {
			lineNum := yamlLineNumStr(origLine, ctx.gutterWidth)
			hl = yamlPrependGutter(hl, lineNum, foldPrefix, visIdx == ctx.yamlCursor, isSelected)
		} else {
			hl = strings.Repeat(" ", gutterOverhead+yamlWrapIndent) + hl
		}
		if si == len(subLines)-1 && visIdx == ctx.yamlCursor {
			hl += yamlBlameInline(ctx, origLine, lipgloss.Width(hl))
		}
		result = append(result, hl)
		if len(result) >= ctx.rowBudget() {
			break
		}
	}
	return result
}

// renderYAMLNonWrappedLine renders a single YAML line without wrapping.
func renderYAMLNonWrappedLine(result []string, contentLine, foldPrefix string, visIdx, origLine int, ctx yamlRenderCtx) []string {
	hl := yamlApplySearchHighlight(contentLine, ctx.searchQuery, origLine, ctx.currentMatch, ctx.matchSet)
	isSelected := ctx.visualMode && visIdx >= ctx.selStart && visIdx <= ctx.selEnd
	adjAnchor, adjCursor, adjColStart, adjColEnd := yamlAdjustedCols(ctx)
	if isSelected {
		hl = ui.RenderVisualSelection(contentLine, ctx.visualType, visIdx, ctx.selStart, ctx.selEnd, ctx.visualStart, adjAnchor, adjCursor, adjColStart, adjColEnd)
	}
	if visIdx == ctx.yamlCursor && !ctx.visualMode {
		hl = ui.RenderCursorAtCol(hl, adjCursor)
	}
	lineNum := yamlLineNumStr(origLine, ctx.gutterWidth)
	hl = yamlPrependGutter(hl, lineNum, foldPrefix, visIdx == ctx.yamlCursor, isSelected)
	if visIdx == ctx.yamlCursor {
		hl += yamlBlameInline(ctx, origLine, lipgloss.Width(hl))
	}
	return append(result, hl)
}
