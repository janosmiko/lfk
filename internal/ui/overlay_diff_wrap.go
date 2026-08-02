package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// diffSideStyles bundles the colors used by the side-by-side wrap renderer.
type diffSideStyles struct {
	removed   lipgloss.Style
	added     lipgloss.Style
	normal    lipgloss.Style
	cursor    lipgloss.Style
	separator lipgloss.Style
}

// diffUnifiedStyles bundles the colors used by the unified wrap renderer.
type diffUnifiedStyles struct {
	removed lipgloss.Style
	added   lipgloss.Style
	normal  lipgloss.Style
}

// diffLineNumsBefore returns the left/right source line numbers reached just
// before the given visible-line index, so a partial scroll keeps the gutter
// numbers correct.
func diffLineNumsBefore(raw []diffLine, vis []VisibleDiffLine, upto int) (int, int) {
	leftNum, rightNum := 1, 1
	for i := 0; i < len(vis) && i < upto; i++ {
		vl := vis[i]
		if vl.IsFoldPlaceholder || vl.Original < 0 {
			continue
		}
		switch raw[vl.Original].status {
		case '=':
			leftNum++
			rightNum++
		case '<':
			leftNum++
		case '>':
			rightNum++
		}
	}
	return leftNum, rightNum
}

// unifiedLineNumBefore returns the 1-based unified line number for the first
// rendered row at the given scroll offset, counting only real diff lines so
// fold placeholders never advance the gutter number (matching the non-wrap
// path).
func unifiedLineNumBefore(vis []VisibleDiffLine, scroll int) int {
	lineNum := 1
	for i := 0; i < len(vis) && i < scroll; i++ {
		if !vis[i].IsFoldPlaceholder && vis[i].Original >= 0 {
			lineNum++
		}
	}
	return lineNum
}

// wrapColumn hard-wraps text to width columns, always returning at least one
// (possibly empty) sub-line so an absent diff side still occupies a row and
// keeps the two columns vertically aligned.
func wrapColumn(text string, width int) []string {
	if text == "" {
		return []string{""}
	}
	return WrapLine(text, width)
}

// subAt returns the i-th sub-line or "" when i is past the end.
func subAt(subs []string, i int) string {
	if i < len(subs) {
		return subs[i]
	}
	return ""
}

// styleDiffSub applies the diff color (and search highlight, if any) to one
// wrapped sub-line. isCurrent marks this line as the current n/N match so it
// gets the distinct current-match style instead of the regular highlight.
func styleDiffSub(text string, style lipgloss.Style, searchQuery string, isCurrent bool) string {
	if text == "" {
		return ""
	}
	if searchQuery != "" {
		return style.Render(highlightDiffSearchInLine(text, searchQuery, isCurrent))
	}
	return style.Render(text)
}

// unifiedLineParts returns the prefix character, content text, and style for a
// unified diff line.
func unifiedLineParts(dl diffLine, st diffUnifiedStyles) (string, string, lipgloss.Style) {
	switch dl.status {
	case '<':
		return "-", dl.left, st.removed
	case '>':
		return "+", dl.right, st.added
	default:
		return " ", dl.left, st.normal
	}
}

// buildWrappedDiffRows renders side-by-side diff rows with hard line wrapping.
// Long values wrap to multiple rows within their column instead of being
// truncated. The block cursor and visual selection are not drawn in wrap mode
// (they need fixed-width column math); the cursor line still gets its ">"
// indicator and search matches are still highlighted.
// currentMatchLine is the original diff line index of the n/N current match
// (-1 means none); matching rows use the distinct current-match highlight.
func buildWrappedDiffRows(raw []diffLine, vis []VisibleDiffLine, scroll, maxLines, colWidth, gutterWidth int, lineNumbers bool, searchQuery string, cursor, cursorSide, currentMatchLine int, st diffSideStyles) []string {
	leftNum, rightNum := diffLineNumsBefore(raw, vis, scroll)
	sep := st.separator.Render(" | ")
	emptyGutter := strings.Repeat(" ", gutterWidth)
	rows := make([]string, 0, maxLines)

	for vi := scroll; vi < len(vis) && len(rows) < maxLines; vi++ {
		vl := vis[vi]
		leftInd, rightInd := " ", " "
		if vi == cursor {
			if cursorSide == 0 {
				leftInd = st.cursor.Render(">")
			} else {
				rightInd = st.cursor.Render(">")
			}
		}

		if vl.IsFoldPlaceholder {
			ph := padToWidth(DiffFoldPlaceholderText(vl.HiddenCount), colWidth)
			rows = append(rows, leftInd+emptyGutter+ph+sep+rightInd+emptyGutter+ph)
			continue
		}

		dl := raw[vl.Original]
		isCurrent := currentMatchLine >= 0 && vl.Original == currentMatchLine
		leftText, rightText, leftStyle, rightStyle, showLeftNum, showRightNum := sideBySideParts(dl, st)

		leftSubs := wrapColumn(leftText, colWidth)
		rightSubs := wrapColumn(rightText, colWidth)
		subCount := max(len(leftSubs), len(rightSubs))
		for r := 0; r < subCount && len(rows) < maxLines; r++ {
			li, ri := " ", " "
			lg, rg := emptyGutter, emptyGutter
			if r == 0 {
				li, ri = leftInd, rightInd
				if lineNumbers {
					if showLeftNum {
						lg = DimStyle.Render(fmt.Sprintf("%*d ", gutterWidth-1, leftNum))
					}
					if showRightNum {
						rg = DimStyle.Render(fmt.Sprintf("%*d ", gutterWidth-1, rightNum))
					}
				}
			}
			lcell := padToWidth(styleDiffSub(subAt(leftSubs, r), leftStyle, searchQuery, isCurrent), colWidth)
			rcell := padToWidth(styleDiffSub(subAt(rightSubs, r), rightStyle, searchQuery, isCurrent), colWidth)
			rows = append(rows, li+lg+lcell+sep+ri+rg+rcell)
		}

		switch dl.status {
		case '=':
			leftNum++
			rightNum++
		case '<':
			leftNum++
		case '>':
			rightNum++
		}
	}
	return rows
}

// sideBySideParts splits a diff line into its left/right text, styles, and
// which gutters should show a line number.
func sideBySideParts(dl diffLine, st diffSideStyles) (leftText, rightText string, leftStyle, rightStyle lipgloss.Style, showLeftNum, showRightNum bool) {
	switch dl.status {
	case '<':
		return dl.left, "", st.removed, st.normal, true, false
	case '>':
		return "", dl.right, st.normal, st.added, false, true
	default:
		return dl.left, dl.right, st.normal, st.normal, true, true
	}
}

// buildSideBySideRows renders the side-by-side diff rows in non-wrap mode:
// long values are truncated to the column width and the block cursor and
// visual selection are drawn. Extracted from RenderDiffView so the wrap and
// non-wrap paths can share the surrounding layout scaffolding.
// currentMatchLine is the original diff line index of the n/N current match
// (-1 means none); matching rows use the distinct current-match highlight.
func buildSideBySideRows(raw []diffLine, vis []VisibleDiffLine, scroll, maxLines, colWidth, gutterWidth int, lineNumbers bool, searchQuery string, cursor, currentMatchLine int, vp DiffVisualParams, normalStyle, removedStyle, addedStyle, cursorStyle, separatorStyle lipgloss.Style) []string { //nolint:gocyclo // diff row layout: inherent per-status (=/</>) branching
	visible := vis[scroll:]
	if len(visible) > maxLines {
		visible = visible[:maxLines]
	}

	leftNum, rightNum := diffLineNumsBefore(raw, vis, scroll)

	selStart, selEnd := -1, -1
	if vp.VisualMode {
		selStart = min(vp.VisualStart, cursor)
		selEnd = max(vp.VisualStart, cursor)
	}

	rows := make([]string, 0, len(visible))
	for ri, vl := range visible {
		visIdx := ri + scroll
		isCursorLine := visIdx == cursor

		leftCursorInd := " "
		rightCursorInd := " "
		if isCursorLine {
			if vp.CursorSide == 0 {
				leftCursorInd = cursorStyle.Render(">")
			} else {
				rightCursorInd = cursorStyle.Render(">")
			}
		}

		if vl.IsFoldPlaceholder {
			placeholder := DiffFoldPlaceholderText(vl.HiddenCount)
			gutterPadL := strings.Repeat(" ", gutterWidth)
			leftPlaceholder := padToWidth(placeholder, colWidth)
			rightPlaceholder := padToWidth(placeholder, colWidth)
			row := leftCursorInd + gutterPadL + leftPlaceholder + separatorStyle.Render(" | ") + rightCursorInd + gutterPadL + rightPlaceholder
			rows = append(rows, row)
			continue
		}
		dl := raw[vl.Original]

		isSelected := vp.VisualMode && visIdx >= selStart && visIdx <= selEnd
		isCurrent := currentMatchLine >= 0 && vl.Original == currentMatchLine

		var leftCol, rightCol, leftGutter, rightGutter string
		switch dl.status {
		case '=':
			var leftText, rightText string
			// Apply visual selection on the active side.
			if isSelected {
				leftText, rightText = applyDiffVisualSelection(dl.left, dl.right, vp, visIdx, selStart, selEnd, colWidth)
			} else {
				if searchQuery != "" {
					leftText = normalStyle.Render(highlightDiffSearchInLine(truncateToWidth(dl.left, colWidth), searchQuery, isCurrent))
					rightText = normalStyle.Render(highlightDiffSearchInLine(truncateToWidth(dl.right, colWidth), searchQuery, isCurrent))
				} else {
					leftText = normalStyle.Render(truncateToWidth(dl.left, colWidth))
					rightText = normalStyle.Render(truncateToWidth(dl.right, colWidth))
				}
			}
			// Block cursor on cursor line (non-visual mode).
			if isCursorLine && !vp.VisualMode {
				if vp.CursorSide == 0 {
					leftText = RenderCursorAtCol(leftText, dl.left, vp.CursorCol)
				} else {
					rightText = RenderCursorAtCol(rightText, dl.right, vp.CursorCol)
				}
			}
			leftCol = leftText
			rightCol = rightText
			if lineNumbers {
				leftGutter = DimStyle.Render(fmt.Sprintf("%*d ", gutterWidth-1, leftNum))
				rightGutter = DimStyle.Render(fmt.Sprintf("%*d ", gutterWidth-1, rightNum))
			}
			leftNum++
			rightNum++
		case '<':
			var leftText string
			if isSelected && vp.CursorSide == 0 {
				leftText = applyDiffVisualSide(dl.left, vp, visIdx, selStart, selEnd)
			} else if searchQuery != "" {
				leftText = removedStyle.Render(highlightDiffSearchInLine(truncateToWidth(dl.left, colWidth), searchQuery, isCurrent))
			} else {
				leftText = removedStyle.Render(truncateToWidth(dl.left, colWidth))
			}
			if isCursorLine && !vp.VisualMode && vp.CursorSide == 0 {
				leftText = RenderCursorAtCol(leftText, dl.left, vp.CursorCol)
			}
			leftCol = leftText
			rightCol = ""
			if lineNumbers {
				leftGutter = DimStyle.Render(fmt.Sprintf("%*d ", gutterWidth-1, leftNum))
				rightGutter = strings.Repeat(" ", gutterWidth)
			}
			leftNum++
		case '>':
			var rightText string
			if isSelected && vp.CursorSide == 1 {
				rightText = applyDiffVisualSide(dl.right, vp, visIdx, selStart, selEnd)
			} else if searchQuery != "" {
				rightText = addedStyle.Render(highlightDiffSearchInLine(truncateToWidth(dl.right, colWidth), searchQuery, isCurrent))
			} else {
				rightText = addedStyle.Render(truncateToWidth(dl.right, colWidth))
			}
			if isCursorLine && !vp.VisualMode && vp.CursorSide == 1 {
				rightText = RenderCursorAtCol(rightText, dl.right, vp.CursorCol)
			}
			leftCol = ""
			rightCol = rightText
			if lineNumbers {
				leftGutter = strings.Repeat(" ", gutterWidth)
				rightGutter = DimStyle.Render(fmt.Sprintf("%*d ", gutterWidth-1, rightNum))
			}
			rightNum++
		}
		row := leftCursorInd + leftGutter + padToWidth(leftCol, colWidth) + separatorStyle.Render(" | ") + rightCursorInd + rightGutter + padToWidth(rightCol, colWidth)
		rows = append(rows, row)
	}
	return rows
}

// buildWrappedUnifiedRows renders unified diff content rows with hard line
// wrapping. The cursor line keeps its ">" indicator; column cursor and visual
// selection are not drawn in wrap mode.
// currentMatchLine is the original diff line index of the n/N current match
// (-1 means none); matching rows use the distinct current-match highlight.
func buildWrappedUnifiedRows(raw []diffLine, vis []VisibleDiffLine, scroll, contentMaxLines, contentWidth, gutterWidth int, lineNumbers bool, searchQuery string, cursor, currentMatchLine int, st diffUnifiedStyles, cursorStyle lipgloss.Style) []string {
	rows := make([]string, 0, contentMaxLines)
	lineNum := unifiedLineNumBefore(vis, scroll)
	emptyGutter := strings.Repeat(" ", gutterWidth)

	for vi := scroll; vi < len(vis) && len(rows) < contentMaxLines; vi++ {
		vl := vis[vi]
		ind := " "
		if vi == cursor {
			ind = cursorStyle.Render(">")
		}

		if vl.IsFoldPlaceholder {
			rows = append(rows, ind+emptyGutter+DiffFoldPlaceholderText(vl.HiddenCount))
			continue
		}

		isCurrent := currentMatchLine >= 0 && vl.Original == currentMatchLine
		prefix, text, style := unifiedLineParts(raw[vl.Original], st)
		subs := wrapColumn(prefix+text, contentWidth)
		for r := 0; r < len(subs) && len(rows) < contentMaxLines; r++ {
			rowInd := " "
			gutter := emptyGutter
			if r == 0 {
				rowInd = ind
				if lineNumbers {
					gutter = DimStyle.Render(fmt.Sprintf("%*d ", gutterWidth-1, lineNum))
				}
			}
			rows = append(rows, rowInd+gutter+styleDiffSub(subs[r], style, searchQuery, isCurrent))
		}
		lineNum++
	}
	return rows
}
