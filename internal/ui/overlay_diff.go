package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// diffLine classifies a line in the diff output.
type diffLine struct {
	left   string // content for left column ("" if absent)
	right  string // content for right column ("" if absent)
	status byte   // '=' same, '<' only left, '>' only right, '~' both present but different
}

// computeDiff produces a line-by-line diff of two YAML texts using a simple
// longest-common-subsequence algorithm, then pairs up the differences.
func computeDiff(leftText, rightText string) []diffLine {
	leftLines := strings.Split(leftText, "\n")
	rightLines := strings.Split(rightText, "\n")

	// Remove trailing empty line from Split if the text ends with newline.
	if len(leftLines) > 0 && leftLines[len(leftLines)-1] == "" {
		leftLines = leftLines[:len(leftLines)-1]
	}
	if len(rightLines) > 0 && rightLines[len(rightLines)-1] == "" {
		rightLines = rightLines[:len(rightLines)-1]
	}

	n := len(leftLines)
	m := len(rightLines)

	// Build LCS table.
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			switch {
			case leftLines[i-1] == rightLines[j-1]:
				dp[i][j] = dp[i-1][j-1] + 1
			case dp[i-1][j] >= dp[i][j-1]:
				dp[i][j] = dp[i-1][j]
			default:
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce the diff.
	var result []diffLine
	i, j := n, m
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && leftLines[i-1] == rightLines[j-1]:
			result = append(result, diffLine{left: leftLines[i-1], right: rightLines[j-1], status: '='})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			result = append(result, diffLine{right: rightLines[j-1], status: '>'})
			j--
		default:
			result = append(result, diffLine{left: leftLines[i-1], status: '<'})
			i--
		}
	}

	// Reverse the result (backtracking produces it in reverse order).
	for lo, hi := 0, len(result)-1; lo < hi; lo, hi = lo+1, hi-1 {
		result[lo], result[hi] = result[hi], result[lo]
	}

	return result
}

// ComputeDiffLines is the exported wrapper for computeDiff.
func ComputeDiffLines(left, right string) []diffLine {
	return computeDiff(left, right)
}

// DiffViewTotalLines returns the total number of scrollable lines for a
// side-by-side diff view after applying fold state. The header and separator
// are rendered outside the scrollable area, so they are not counted here.
func DiffViewTotalLines(left, right string, foldRegions []DiffFoldRegion, foldState []bool) int {
	diffLines := computeDiff(left, right)
	visLines := BuildVisibleDiffLines(diffLines, foldRegions, foldState)
	return len(visLines)
}

// DiffVisualParams holds visual selection state passed to diff renderers.
type DiffVisualParams struct {
	CursorSide  int  // 0=left, 1=right (side-by-side only)
	CursorCol   int  // current cursor column
	VisualMode  bool // true when in visual selection mode
	VisualType  rune // 'V' = line, 'v' = char, 'B' = block
	VisualStart int  // anchor line (visible-line index)
	VisualCol   int  // anchor column
}

// RenderDiffView renders a side-by-side YAML diff view with search highlighting
// and fold support.
// footerOverride, when non-empty, is rendered as the bottom hint line
// verbatim in place of the default key-binding hint bar. Callers use this
// to surface status messages (copy feedback, errors) without resorting
// to post-render line surgery.
// currentMatchLine is the original diff line index of the current n/N match
// (-1 means none); that line gets the distinct SelectedSearchHighlightStyle.
func RenderDiffView(left, right, leftName, rightName string, scroll, width, height int, lineNumbers, wrap bool, searchQuery string, foldRegions []DiffFoldRegion, foldState []bool, searchMode bool, searchInput string, cursor, currentMatchLine int, vp DiffVisualParams, footerOverride string) string { //nolint:gocyclo // rendering function with inherent layout complexity
	rawDiffLines := computeDiff(left, right)
	visLines := BuildVisibleDiffLines(rawDiffLines, foldRegions, foldState)

	// Styles for diff highlighting.
	removedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Background(SurfaceBg)
	addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Background(SurfaceBg)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile)).Background(SurfaceBg)
	headerNameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true).Background(SurfaceBg)
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDimmed)).Background(SurfaceBg)

	// Line number gutter width.
	var gutterWidth int
	if lineNumbers {
		gutterWidth = len(fmt.Sprintf("%d", len(rawDiffLines))) + 1 // digits + space
	}

	// Calculate column widths: split the available content area in half with a
	// separator. Budget per row = border+padding (4) + two cursor indicators
	// (2) + " | " separator (3) = 9, plus a line-number gutter on each side.
	// Under-budgeting here lets a fully-filled wrapped row exceed the border
	// width, which makes lipgloss re-wrap it and shears the two columns apart.
	colWidth := max((width-9-gutterWidth*2)/2, 10)

	// Build header.
	gutterPad := strings.Repeat(" ", gutterWidth)
	leftHeader := headerNameStyle.Render(truncateToWidth(leftName, colWidth))
	rightHeader := headerNameStyle.Render(truncateToWidth(rightName, colWidth))
	header := gutterPad + padToWidth(leftHeader, colWidth) + separatorStyle.Render(" | ") + gutterPad + padToWidth(rightHeader, colWidth)

	// Reserve lines for title, hint bar, border (top+bottom), header, and separator.
	maxLines := max(height-6, 3)

	// Clamp scroll.
	totalLines := len(visLines)
	maxScroll := max(totalLines-maxLines, 0)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true).Background(SurfaceBg)

	var rows []string
	if wrap {
		// Wrap mode: long values wrap within their column instead of truncating.
		st := diffSideStyles{removed: removedStyle, added: addedStyle, normal: normalStyle, cursor: cursorStyle, separator: separatorStyle}
		rows = buildWrappedDiffRows(rawDiffLines, visLines, scroll, maxLines, colWidth, gutterWidth, lineNumbers, searchQuery, cursor, vp.CursorSide, currentMatchLine, st)
	} else {
		rows = buildSideBySideRows(rawDiffLines, visLines, scroll, maxLines, colWidth, gutterWidth, lineNumbers, searchQuery, cursor, currentMatchLine, vp, normalStyle, removedStyle, addedStyle, cursorStyle, separatorStyle)
	}

	// Pad rows to fill available height so content fills the border.
	for len(rows) < maxLines {
		rows = append(rows, "")
	}

	titleText := "Resource Diff"
	if wrap {
		titleText += " [WRAP]"
	}
	if vp.VisualMode {
		switch vp.VisualType {
		case 'v':
			titleText += " [VISUAL]"
		case 'B':
			titleText += " [VISUAL BLOCK]"
		default:
			titleText += " [VISUAL LINE]"
		}
	}
	if searchQuery != "" && !searchMode {
		titleText += " [/" + searchQuery + "]"
	}
	title := ViewTitle(width, titleText)
	sepLine := gutterPad + strings.Repeat("-", colWidth) + " + " + gutterPad + strings.Repeat("-", colWidth)
	bodyContent := header + "\n" + separatorStyle.Render(sepLine) + "\n" + strings.Join(rows, "\n")

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorPrimary)).
		Padding(0, 1).
		Width(width - 2).
		Height(maxLines + 2). // +2 for header + separator lines
		MaxHeight(maxLines + 4)
	body := borderStyle.Render(bodyContent)

	// Hint bar. A non-empty footerOverride wins over the default hint and
	// any search/visual-mode bars so callers can paint status messages
	// (e.g. copy feedback) in this slot without post-render line surgery.
	var hint string
	switch {
	case footerOverride != "":
		hint = footerOverride
	case searchMode:
		diffModeInd := SearchModeIndicator(searchInput)
		searchBar := HelpKeyStyle.Render("type: search") + BarDimStyle.Render(" | ") +
			HelpKeyStyle.Render("enter") + BarDimStyle.Render(": apply | ") +
			HelpKeyStyle.Render("esc") + BarDimStyle.Render(": cancel") +
			BarDimStyle.Render("  "+ActiveKeybindings.Search) + BarDimStyle.Render(diffModeInd) + BarNormalStyle.Render(searchInput) + BarDimStyle.Render("\u2588")
		hint = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(searchBar)
	case vp.VisualMode:
		hintContent := FormatHintParts([]HintEntry{
			{Key: "j/k", Desc: "extend"},
			{Key: "h/l", Desc: "column"},
			{Key: "y", Desc: "copy"},
			{Key: "v/V/ctrl+v", Desc: "switch mode"},
			{Key: "esc", Desc: "cancel"},
		})
		scrollInfo := BarDimStyle.Render(fmt.Sprintf(" [%d/%d]", scroll+1, max(1, maxScroll+1)))
		hint = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(hintContent + scrollInfo)
	default:
		hintContent := FormatHintParts([]HintEntry{
			{Key: "j/k", Desc: "scroll"},
			{Key: "g/G", Desc: "top/bottom"},
			{Key: ActiveKeybindings.Search, Desc: "search"},
			{Key: "v/V", Desc: "select"},
			{Key: "y", Desc: "copy"},
			{Key: "tab", Desc: "side"},
			{Key: ActiveKeybindings.ToggleFold, Desc: "fold"},
			{Key: ActiveKeybindings.ToggleLineNumbers, Desc: "lines"},
			{Key: ActiveKeybindings.ToggleWrap, Desc: "wrap"},
			{Key: ActiveKeybindings.ToggleUnified, Desc: "unified"},
			{Key: "q/esc", Desc: "back"},
		})
		scrollInfo := BarDimStyle.Render(fmt.Sprintf(" [%d/%d]", scroll+1, max(1, maxScroll+1)))
		hint = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(hintContent + scrollInfo)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, body, hint)
}

// RenderUnifiedDiffView renders a unified diff view of two YAML resources
// with search highlighting and fold support. footerOverride behaves the
// same as in RenderDiffView.
// currentMatchLine is the original diff line index of the current n/N match
// (-1 means none); that line gets the distinct SelectedSearchHighlightStyle.
func RenderUnifiedDiffView(left, right, leftName, rightName string, scroll, width, height int, lineNumbers, wrap bool, searchQuery string, foldRegions []DiffFoldRegion, foldState []bool, searchMode bool, searchInput string, cursor, currentMatchLine int, vp DiffVisualParams, footerOverride string) string { //nolint:gocyclo // rendering function with inherent layout complexity
	rawDiffLines := computeDiff(left, right)
	visLines := BuildVisibleDiffLines(rawDiffLines, foldRegions, foldState)

	// Styles.
	removedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Background(SurfaceBg)
	addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Background(SurfaceBg)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile)).Background(SurfaceBg)
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true).Background(SurfaceBg)

	// Line number gutter width (for the unified line number).
	var gutterWidth int
	if lineNumbers {
		gutterWidth = len(fmt.Sprintf("%d", len(rawDiffLines)+2)) + 1 // +2 for header lines
	}

	// Content width = border(2) + padding(2) + leading indicator(1) + gutter.
	// Lines are truncated (non-wrap) or wrapped (wrap) to this width so the
	// border never has to wrap them itself.
	contentWidth := max(width-5-gutterWidth, 10)
	uStyles := diffUnifiedStyles{removed: removedStyle, added: addedStyle, normal: normalStyle}

	// Precompute visual selection range.
	selStart, selEnd := -1, -1
	if vp.VisualMode {
		selStart = min(vp.VisualStart, cursor)
		selEnd = max(vp.VisualStart, cursor)
	}

	// Build unified diff lines from visible lines.
	type unifiedLine struct {
		text   string
		plain  string // plain text for cursor rendering
		visIdx int    // visible diff line index (for selection)
	}
	var lines []unifiedLine
	lines = append(lines, unifiedLine{text: headerStyle.Render("--- " + leftName), visIdx: -1})
	lines = append(lines, unifiedLine{text: headerStyle.Render("+++ " + rightName), visIdx: -1})

	lineNum := 1
	for vi, vl := range visLines {
		if vl.IsFoldPlaceholder {
			lines = append(lines, unifiedLine{text: DiffFoldPlaceholderText(vl.HiddenCount), visIdx: vi})
			continue
		}
		dl := rawDiffLines[vl.Original]
		var gutter string
		if lineNumbers {
			gutter = DimStyle.Render(fmt.Sprintf("%*d ", gutterWidth-1, lineNum))
		}
		lineNum++

		isSelected := vp.VisualMode && vi >= selStart && vi <= selEnd
		isCursorLine := vi == cursor
		isCurrentMatch := currentMatchLine >= 0 && vl.Original == currentMatchLine

		// Truncate to the content width so the border never re-wraps the line.
		prefix, text, style := unifiedLineParts(dl, uStyles)
		plain := truncateToWidth(prefix+text, contentWidth)
		var content string
		switch {
		case isSelected:
			content = applyDiffVisualSide(plain, vp, vi, selStart, selEnd)
		case searchQuery != "":
			content = style.Render(highlightDiffSearchInLine(plain, searchQuery, isCurrentMatch))
		default:
			content = style.Render(plain)
		}
		if isCursorLine && !vp.VisualMode {
			content = RenderCursorAtCol(content, plain, vp.CursorCol)
		}
		lines = append(lines, unifiedLine{text: gutter + content, plain: plain, visIdx: vi})
	}

	titleText := "Resource Diff (unified)"
	if wrap {
		titleText += " [WRAP]"
	}
	if vp.VisualMode {
		switch vp.VisualType {
		case 'v':
			titleText += " [VISUAL]"
		case 'B':
			titleText += " [VISUAL BLOCK]"
		default:
			titleText += " [VISUAL LINE]"
		}
	}
	if searchQuery != "" && !searchMode {
		titleText += " [/" + searchQuery + "]"
	}
	title := ViewTitle(width, titleText)

	// Reserve lines for title, hint bar, and border (top+bottom).
	maxLines := max(height-4, 3)

	// Separate headers (always visible) from scrollable content.
	var headerLines []unifiedLine
	var contentLines []unifiedLine
	for _, ul := range lines {
		if ul.visIdx < 0 {
			headerLines = append(headerLines, ul)
		} else {
			contentLines = append(contentLines, ul)
		}
	}

	// Content area = maxLines minus header lines.
	contentMaxLines := max(maxLines-len(headerLines), 1)
	totalContent := len(contentLines)

	cursorStyleU := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true).Background(SurfaceBg)

	// Build rendered lines: headers first, then scrollable content.
	var rendered []string
	for _, hl := range headerLines {
		rendered = append(rendered, " "+hl.text)
	}

	var maxScroll int
	if wrap {
		// Wrap mode clamps per logical line, not per page: one diff line can
		// expand to several visual rows, so page-fill clamping (which mixes a
		// visual-row count with a logical-line count) doesn't apply. Scroll
		// indexes whole content lines, matching the cursor model, and stops at
		// the last line so it stays reachable.
		maxScroll = max(totalContent-1, 0)
		scroll = min(max(scroll, 0), maxScroll)
		rendered = append(rendered, buildWrappedUnifiedRows(rawDiffLines, visLines, scroll, contentMaxLines, contentWidth, gutterWidth, lineNumbers, searchQuery, cursor, currentMatchLine, uStyles, cursorStyleU)...)
	} else {
		// Non-wrap: one row per line, so clamp to keep the last page full.
		maxScroll = max(totalContent-contentMaxLines, 0)
		scroll = min(max(scroll, 0), maxScroll)

		visibleContent := contentLines[scroll:]
		if len(visibleContent) > contentMaxLines {
			visibleContent = visibleContent[:contentMaxLines]
		}
		for _, ul := range visibleContent {
			if ul.visIdx == cursor {
				rendered = append(rendered, cursorStyleU.Render(">")+ul.text)
			} else {
				rendered = append(rendered, " "+ul.text)
			}
		}
	}

	// Pad to fill available height so content fills the border.
	for len(rendered) < maxLines {
		rendered = append(rendered, "")
	}

	bodyContent := strings.Join(rendered, "\n")
	borderStyle := FullscreenBorderStyle(width, maxLines)
	body := borderStyle.Render(bodyContent)

	// Hint bar. footerOverride wins over default hints \u2014 see RenderDiffView.
	var hint string
	switch {
	case footerOverride != "":
		hint = footerOverride
	case searchMode:
		diffModeInd := SearchModeIndicator(searchInput)
		searchBar := HelpKeyStyle.Render("type: search") + BarDimStyle.Render(" | ") +
			HelpKeyStyle.Render("enter") + BarDimStyle.Render(": apply | ") +
			HelpKeyStyle.Render("esc") + BarDimStyle.Render(": cancel") +
			BarDimStyle.Render("  "+ActiveKeybindings.Search) + BarDimStyle.Render(diffModeInd) + BarNormalStyle.Render(searchInput) + BarDimStyle.Render("\u2588")
		hint = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(searchBar)
	case vp.VisualMode:
		hintContent := FormatHintParts([]HintEntry{
			{Key: "j/k", Desc: "extend"},
			{Key: "h/l", Desc: "column"},
			{Key: "y", Desc: "copy"},
			{Key: "v/V/ctrl+v", Desc: "switch mode"},
			{Key: "esc", Desc: "cancel"},
		})
		scrollInfo := BarDimStyle.Render(fmt.Sprintf(" [%d/%d]", scroll+1, max(1, maxScroll+1)))
		hint = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(hintContent + scrollInfo)
	default:
		hintContent := FormatHintParts([]HintEntry{
			{Key: "j/k", Desc: "scroll"},
			{Key: "g/G", Desc: "top/bottom"},
			{Key: ActiveKeybindings.Search, Desc: "search"},
			{Key: "v/V", Desc: "select"},
			{Key: "y", Desc: "copy"},
			{Key: ActiveKeybindings.ToggleFold, Desc: "fold"},
			{Key: ActiveKeybindings.ToggleLineNumbers, Desc: "lines"},
			{Key: ActiveKeybindings.ToggleWrap, Desc: "wrap"},
			{Key: ActiveKeybindings.ToggleUnified, Desc: "side-by-side"},
			{Key: "q/esc", Desc: "back"},
		})
		scrollInfo := BarDimStyle.Render(fmt.Sprintf(" [%d/%d]", scroll+1, max(1, maxScroll+1)))
		hint = StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(hintContent + scrollInfo)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, body, hint)
}

// UnifiedDiffViewTotalLines returns the total number of scrollable content
// lines for a unified diff view after applying fold state. The --- and +++
// header lines are always visible but not part of the cursor range.
func UnifiedDiffViewTotalLines(left, right string, foldRegions []DiffFoldRegion, foldState []bool) int {
	diffLines := computeDiff(left, right)
	visLines := BuildVisibleDiffLines(diffLines, foldRegions, foldState)
	return len(visLines)
}

// UpdateDiffSearchMatches finds all diff line indices where the content on the
// specified side matches the query. Supports substring, regex, and fuzzy modes.
// side: 0=left, 1=right. unified=true searches both sides.
func UpdateDiffSearchMatches(left, right, query string, side int, unified bool) []int {
	if query == "" {
		return nil
	}
	diffLines := computeDiff(left, right)
	var matches []int
	for i, dl := range diffLines {
		var text string
		if unified {
			// In unified mode, search whichever side has content.
			text = dl.left
			if text == "" {
				text = dl.right
			}
		} else if side == 0 {
			text = dl.left
		} else {
			text = dl.right
		}
		if MatchLine(text, query) {
			matches = append(matches, i)
		}
	}
	return matches
}

// DiffSearchColumnInLine returns the rune column of the first match of query
// in the given diff line text, or -1 if not found.
// Supports substring, regex, and fuzzy search modes.
func DiffSearchColumnInLine(lineText, query string) int {
	return FindColumnInLine(lineText, query)
}

// DiffVisibleIndexForOriginal finds the visible line index corresponding to
// the given original diff line index, or -1 if hidden.
func DiffVisibleIndexForOriginal(left, right string, foldRegions []DiffFoldRegion, foldState []bool, origIdx int) int {
	diffLines := computeDiff(left, right)
	visLines := BuildVisibleDiffLines(diffLines, foldRegions, foldState)
	for i, vl := range visLines {
		if vl.Original == origIdx {
			return i
		}
	}
	return -1
}

// DiffLineTextAt returns the text of a specific visible diff line on the given side.
// For side-by-side mode, side 0 returns the left text, side 1 returns the right text.
// For unified mode, it returns whichever side has content.
// Returns empty string if the index is out of range or the line is a fold placeholder.
func DiffLineTextAt(left, right string, foldRegions []DiffFoldRegion, foldState []bool, visibleIdx, side int, unified bool) string {
	rawDiffLines := computeDiff(left, right)
	visLines := BuildVisibleDiffLines(rawDiffLines, foldRegions, foldState)
	if visibleIdx < 0 || visibleIdx >= len(visLines) {
		return ""
	}
	vl := visLines[visibleIdx]
	if vl.IsFoldPlaceholder || vl.Original < 0 {
		return ""
	}
	dl := rawDiffLines[vl.Original]
	if unified {
		if dl.left != "" {
			return dl.left
		}
		return dl.right
	}
	if side == 0 {
		return dl.left
	}
	return dl.right
}

// applyDiffVisualSelection applies visual selection highlighting to both sides
// of a side-by-side diff line and returns the styled left and right text.
func applyDiffVisualSelection(leftPlain, rightPlain string, vp DiffVisualParams, visIdx, selStart, selEnd, colWidth int) (string, string) {
	if vp.CursorSide == 0 {
		leftResult := applyDiffVisualSide(leftPlain, vp, visIdx, selStart, selEnd)
		rightResult := truncateToWidth(rightPlain, colWidth)
		return leftResult, rightResult
	}
	leftResult := truncateToWidth(leftPlain, colWidth)
	rightResult := applyDiffVisualSide(rightPlain, vp, visIdx, selStart, selEnd)
	return leftResult, rightResult
}

// applyDiffVisualSide applies visual selection highlighting to a single side's text.
func applyDiffVisualSide(plainText string, vp DiffVisualParams, visIdx, selStart, selEnd int) string {
	colStart := min(vp.VisualCol, vp.CursorCol)
	colEnd := max(vp.VisualCol, vp.CursorCol)
	return RenderVisualSelection(plainText, vp.VisualType, visIdx, selStart, selEnd, vp.VisualStart, vp.VisualCol, vp.CursorCol, colStart, colEnd)
}

// truncateToWidth truncates a string to fit within the given visual width.
func truncateToWidth(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	// Progressively truncate until it fits.
	runes := []rune(s)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		if lipgloss.Width(string(runes)) <= maxWidth-1 {
			return string(runes) + "~"
		}
	}
	return ""
}

// padToWidth pads a styled string with spaces to reach the desired visual width.
func padToWidth(s string, targetWidth int) string {
	w := lipgloss.Width(s)
	if w >= targetWidth {
		return s
	}
	return s + strings.Repeat(" ", targetWidth-w)
}

// PlaceOverlayBottom anchors an overlay near the bottom edge of the background,
// horizontally centered, leaving marginBottom rows below it. Used for the
// which-key panel, which rises from the bottom of the screen like neovim's
// which-key "modern" preset rather than floating in the center.
func PlaceOverlayBottom(width, height, marginBottom int, overlay, background string) string {
	bgLines := strings.Split(background, "\n")
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}
	if len(bgLines) > height {
		bgLines = bgLines[:height]
	}

	ovLines := strings.Split(overlay, "\n")
	ovWidth := 0
	if len(ovLines) > 0 {
		ovWidth = lipgloss.Width(ovLines[0])
	}

	startRow := max(height-marginBottom-len(ovLines), 0)
	startCol := max((width-ovWidth)/2, 0)

	result := make([]string, len(bgLines))
	copy(result, bgLines)
	for i, line := range result {
		if lineW := lipgloss.Width(line); lineW < width {
			result[i] = line + strings.Repeat(" ", width-lineW)
		}
	}

	for i, ovLine := range ovLines {
		row := startRow + i
		if row < 0 || row >= len(result) {
			continue
		}
		ovVisualWidth := lipgloss.Width(ovLine)
		leftBg := ansi.Truncate(result[row], startCol, "")
		rightBg := ansi.TruncateLeft(result[row], startCol+ovVisualWidth, "")
		result[row] = leftBg + ovLine + rightBg
	}
	return strings.Join(result, "\n")
}

// PlaceOverlay centers an overlay on top of a background by building a full-screen
// layer with the overlay content padded to the correct position.
func PlaceOverlay(width, height int, overlay, background string) string {
	bgLines := strings.Split(background, "\n")

	// Ensure background has enough lines.
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}
	// Trim excess.
	if len(bgLines) > height {
		bgLines = bgLines[:height]
	}

	ovLines := strings.Split(overlay, "\n")
	ovHeight := len(ovLines)
	// Use the first line's width as the definitive overlay width.
	// The first line is the border which always has the correct width.
	// Measuring all lines can produce inconsistent results when content
	// lines have complex ANSI sequences.
	ovWidth := 0
	if len(ovLines) > 0 {
		ovWidth = lipgloss.Width(ovLines[0])
	}

	startRow := (height - ovHeight) / 2
	startCol := (width - ovWidth) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	// Build result: replace background lines in the overlay region.
	result := make([]string, len(bgLines))
	copy(result, bgLines)

	// Ensure each background line is at least full width so the overlay
	// doesn't leave gaps when the background has short lines.
	for i, line := range result {
		lineW := lipgloss.Width(line)
		if lineW < width {
			result[i] = line + strings.Repeat(" ", width-lineW)
		}
	}

	for i, ovLine := range ovLines {
		row := startRow + i
		if row >= len(result) {
			break
		}
		bgLine := result[row]
		ovVisualWidth := lipgloss.Width(ovLine)
		leftBg := ansi.Truncate(bgLine, startCol, "")
		rightBg := ansi.TruncateLeft(bgLine, startCol+ovVisualWidth, "")
		result[row] = leftBg + ovLine + rightBg
	}

	return strings.Join(result, "\n")
}
