package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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
// When the combined width exceeds `width`, the RIGHT side gets priority — the
// left chunk is truncated (with `~`) to make room for the right intact. This
// matches the explorer's bottom bar contract: the right-anchored info chips
// (sort, counter / selected count, filter preset, NYAN) are stable, compact
// state indicators that the user expects to see at a glance, while the
// keymap hints on the left are a long, mostly-static list that degrades
// gracefully when truncated. Overlay hint bars do not use this helper —
// they render hints alone via a separate code path — so the right-priority
// rule is scoped to the chip-bearing explorer bar.
//
// If the right alone exceeds `width`, the right is truncated and the left
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

	// Right alone exceeds available width — truncate right, drop left.
	if rightW >= width {
		return Truncate(right, width)
	}

	// Left needs trimming to fit alongside the right with one separating space.
	leftMax := width - rightW - 1
	if leftMax <= 0 {
		// No room for any left content; pad to right edge.
		return strings.Repeat(" ", width-rightW) + right
	}
	truncatedLeft := Truncate(left, leftMax)
	spacer := max(width-lipgloss.Width(truncatedLeft)-rightW, 1)
	return truncatedLeft + strings.Repeat(" ", spacer) + right
}
