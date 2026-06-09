package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A value far wider than any column; wrapping must render every "x", while
// truncation drops most of them and leaves a "~" marker.
const wrapFillLen = 200

func longDiffLine() string {
	return "field: " + strings.Repeat("x", wrapFillLen)
}

// --- side-by-side ---

func TestRenderDiffView_WrapShowsFullLine(t *testing.T) {
	left := "a: 1\n" + longDiffLine()
	right := "a: 1\n" + longDiffLine()

	wrapped := stripANSI(RenderDiffView(left, right, "l", "r", 0, 80, 30, false, true, "", nil, nil, false, "", 0, DiffVisualParams{}, ""))
	assert.GreaterOrEqual(t, strings.Count(wrapped, "x"), wrapFillLen, "wrap must render the whole line, not truncate it")
	assert.NotContains(t, wrapped, "~", "wrap mode must not emit the truncation marker")
}

func TestRenderDiffView_NoWrapTruncates(t *testing.T) {
	left := "a: 1\n" + longDiffLine()
	right := "a: 1\n" + longDiffLine()

	truncated := stripANSI(RenderDiffView(left, right, "l", "r", 0, 80, 30, false, false, "", nil, nil, false, "", 0, DiffVisualParams{}, ""))
	assert.Contains(t, truncated, "~", "non-wrap mode truncates long lines with a marker")
	assert.Less(t, strings.Count(truncated, "x"), wrapFillLen, "non-wrap mode must drop the truncated tail")
}

func TestRenderDiffView_WrapKeepsColumnAlignment(t *testing.T) {
	left := "a: 1\n" + longDiffLine()
	right := "a: 1\n" + longDiffLine()
	out := stripANSI(RenderDiffView(left, right, "l", "r", 0, 80, 30, false, true, "", nil, nil, false, "", 0, DiffVisualParams{}, ""))
	// Every wrapped content row keeps the side-by-side separator.
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, "xxx") {
			assert.Contains(t, line, "|", "wrapped continuation rows keep the column separator")
		}
	}
}

// unifiedLineNumBefore must count only real diff lines, skipping fold
// placeholders, so wrapped gutter numbers match the non-wrap path.
func TestUnifiedLineNumBefore_SkipsFoldPlaceholders(t *testing.T) {
	vis := []VisibleDiffLine{
		{Original: 0},
		{IsFoldPlaceholder: true, Original: -1},
		{Original: 5},
		{Original: 6},
	}
	// Before index 0: line 1. The placeholder at index 1 must not advance the
	// count, so index 2 and 3 are lines 2 and 3.
	assert.Equal(t, 1, unifiedLineNumBefore(vis, 0))
	assert.Equal(t, 2, unifiedLineNumBefore(vis, 1)) // one real line before
	assert.Equal(t, 2, unifiedLineNumBefore(vis, 2)) // placeholder skipped
	assert.Equal(t, 3, unifiedLineNumBefore(vis, 3))
}

// A truncated cell that fills its column exactly (ends in "~", no trailing
// space) exposed the colWidth off-by-one in issue #386: the row was one cell
// over the border budget, so lipgloss re-wrapped it and the tail bled to
// column 0. lipgloss.Width can't catch this (every re-wrapped fragment still
// measures <= width); a bled row is instead detectable as a content-bearing
// body line missing the " | " column separator. Markers Q/Z are absent from
// the title, border, and hint bar, so any match is real diff content.
func TestRenderDiffView_NoWrapDoesNotBleed(t *testing.T) {
	left := "a: 1\nlongkey: " + strings.Repeat("Q", 300) + "\nz: 9\n"
	right := "a: 1\nlongkey: " + strings.Repeat("Z", 300) + "\nz: 9\n"
	for _, lineNumbers := range []bool{true, false} {
		for w := 60; w <= 200; w++ {
			out := stripANSI(RenderDiffView(left, right, "left", "right", 0, w, 24, lineNumbers, false, "", nil, nil, false, "", 0, DiffVisualParams{}, ""))
			for line := range strings.SplitSeq(out, "\n") {
				if strings.ContainsAny(line, "QZ") && !strings.Contains(line, "|") {
					t.Fatalf("w=%d lineNumbers=%v: diff content bled past its column (row has content but no separator): %q", w, lineNumbers, line)
				}
			}
		}
	}
}

// --- unified ---

func TestRenderUnifiedDiffView_WrapShowsFullLine(t *testing.T) {
	left := "a: 1\n" + longDiffLine()
	right := "a: 1\n" + longDiffLine()

	wrapped := stripANSI(RenderUnifiedDiffView(left, right, "l", "r", 0, 80, 30, false, true, "", nil, nil, false, "", 0, DiffVisualParams{}, ""))
	assert.GreaterOrEqual(t, strings.Count(wrapped, "x"), wrapFillLen, "unified wrap must render the whole line")
	assert.NotContains(t, wrapped, "~", "unified wrap must not truncate")
}

func TestRenderUnifiedDiffView_NoWrapTruncates(t *testing.T) {
	left := "a: 1\n" + longDiffLine()
	right := "a: 1\n" + longDiffLine()

	truncated := stripANSI(RenderUnifiedDiffView(left, right, "l", "r", 0, 80, 30, false, false, "", nil, nil, false, "", 0, DiffVisualParams{}, ""))
	// Non-wrap unified must truncate itself rather than leak the line past the
	// border and let lipgloss re-wrap it uncontrollably.
	assert.Contains(t, truncated, "~", "non-wrap unified truncates long lines")
	assert.Less(t, strings.Count(truncated, "x"), wrapFillLen, "non-wrap unified drops the truncated tail")
}
