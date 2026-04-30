package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// HintEntry represents a single key-description pair for a hint bar.
type HintEntry struct {
	Key  string
	Desc string
}

// FormatHintParts builds the inner styled content from hint entries using the
// standard HelpKeyStyle + BarDimStyle pattern, joined by a styled separator.
// This returns the joined content without the StatusBarBgStyle wrapper, useful
// when callers need to append extra content (e.g. scroll info) before wrapping.
func FormatHintParts(hints []HintEntry) string {
	if len(hints) == 0 {
		return ""
	}
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = HelpKeyStyle.Render(h.Key) + BarDimStyle.Render(": "+h.Desc)
	}
	return strings.Join(parts, BarDimStyle.Render(" | "))
}

// FormatHintPartsFit builds the same styled content as FormatHintParts but
// constrained to maxWidth visual columns. Each hint entry is included as a
// whole unit — keys and descriptions are never split mid-character, so the
// reader never sees a half-cut "naviga…" or "ctrl+r: toggle R~". Entries
// that don't fit are skipped; subsequent shorter entries can still be
// appended. The list's left-to-right reading order is preserved.
//
// Returns "" when no entry fits in maxWidth, or when hints is empty / the
// width is non-positive.
func FormatHintPartsFit(hints []HintEntry, maxWidth int) string {
	if len(hints) == 0 || maxWidth <= 0 {
		return ""
	}
	sep := BarDimStyle.Render(" | ")
	sepW := lipgloss.Width(sep)

	var b strings.Builder
	used := 0
	first := true
	for _, h := range hints {
		entry := HelpKeyStyle.Render(h.Key) + BarDimStyle.Render(": "+h.Desc)
		entryW := lipgloss.Width(entry)
		add := entryW
		if !first {
			add += sepW
		}
		if used+add > maxWidth {
			continue // skip this entry; a shorter later one might still fit
		}
		if !first {
			b.WriteString(sep)
		}
		b.WriteString(entry)
		used += add
		first = false
	}
	return b.String()
}

// RenderHintBar builds a full-width status bar from hint entries using the
// standard HelpKeyStyle + BarDimStyle pattern. This is the single source of
// truth for hint bar styling -- if the style needs to change, only this
// function needs updating.
func RenderHintBar(hints []HintEntry, width int) string {
	content := FormatHintParts(hints)
	return StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(content)
}

// JoinStatusBar composes a status bar with `left` content anchored to the left
// edge and `right` content anchored to the right edge, separated by an elastic
// run of spaces so the bar exactly fills `width` visual columns.
//
// A single-column gutter between the two halves is always preserved: when
// `leftW + rightW == width` (a perfect fit with no gap), the left side is
// trimmed by 1 column rather than rendering the two halves butted together.
//
// When the combined width exceeds `width`, the RIGHT side gets priority and
// the left chunk is hard-cut (no truncate marker) to make room for the right
// intact. The cut is unmarked deliberately: the separator between the two
// halves is whitespace only, never a stray `~`, so the eye cleanly reads the
// info chips on the right. Callers passing hint-entry content as `left`
// should pre-fit it with FormatHintPartsFit so the cut here is only ever
// a safety fallback for non-entry content.
//
// If the right alone exceeds `width`, the right is hard-cut and the left
// is dropped entirely. width <= 0 returns "".
func JoinStatusBar(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)

	if leftW == 0 && rightW == 0 {
		return ""
	}

	// Both fit with at least one column of separator.
	if leftW+rightW < width {
		spacer := width - leftW - rightW
		return left + strings.Repeat(" ", spacer) + right
	}

	// Right alone exceeds available width — hard-cut right, drop left.
	if rightW >= width {
		return ansi.Truncate(right, width, "")
	}

	// Left needs trimming to fit alongside the right with one separating space.
	leftMax := width - rightW - 1
	if leftMax <= 0 {
		// No room for any left content; pad to right edge.
		return strings.Repeat(" ", width-rightW) + right
	}
	truncatedLeft := ansi.Truncate(left, leftMax, "")
	spacer := max(width-lipgloss.Width(truncatedLeft)-rightW, 1)
	return truncatedLeft + strings.Repeat(" ", spacer) + right
}
