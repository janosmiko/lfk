package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

// Top/bottom scroll indicators tell the user how many items are hidden
// above the viewport (rendered ABOVE the items list) and below (rendered
// BELOW it). Both functions always return exactly one row — the row is
// blank when there's no overflow in that direction so the surrounding
// layout doesn't shift when the user filters and the overflow
// disappears.

func TestRenderScrollAbove_HasItemsAbove(t *testing.T) {
	got := RenderScrollAbove(15, 5, 20, 40)
	plain := stripANSI(got)
	assert.Contains(t, plain, "15")
	assert.Contains(t, plain, "above")
	assert.Contains(t, plain, "↑")
}

func TestRenderScrollAbove_NothingAboveStillReturnsRow(t *testing.T) {
	// Layout must be stable — the row is present (just blank) so the
	// overlay's row budget doesn't shrink when the filter narrows the
	// viewport to fit.
	got := RenderScrollAbove(0, 5, 5, 40)
	assert.NotEmpty(t, got, "blank-but-present row keeps the layout stable")
	assert.NotContains(t, stripANSI(got), "↑",
		"no overflow above → no arrow drawn, just the placeholder row")
}

func TestRenderScrollBelow_HasItemsBelow(t *testing.T) {
	got := RenderScrollBelow(0, 5, 20, 40)
	plain := stripANSI(got)
	assert.Contains(t, plain, "15")
	assert.Contains(t, plain, "below")
	assert.Contains(t, plain, "↓")
}

func TestRenderScrollBelow_NothingBelowStillReturnsRow(t *testing.T) {
	got := RenderScrollBelow(15, 5, 20, 40)
	assert.NotEmpty(t, got, "blank-but-present row keeps the layout stable")
	assert.NotContains(t, stripANSI(got), "↓")
}

func TestRenderScrollIndicators_SingleRow(t *testing.T) {
	for _, got := range []string{
		RenderScrollAbove(5, 5, 20, 40),
		RenderScrollBelow(5, 5, 20, 40),
	} {
		assert.NotContains(t, got, "\n",
			"indicator must be exactly one row so callers can budget it")
	}
}

func TestRenderScrollIndicators_TruncateToWidth(t *testing.T) {
	for _, got := range []string{
		RenderScrollAbove(99999, 1, 999999, 20),
		RenderScrollBelow(0, 1, 999999, 20),
	} {
		assert.LessOrEqual(t, lipgloss.Width(strings.TrimRight(stripANSI(got), " ")), 20,
			"indicator must respect the width budget")
	}
}

// User-visible stability guarantee: each indicator function returns
// exactly one row regardless of how much overflow exists in that
// direction. Without this, the overlay's total row count shifts when
// the user filters and the overflow disappears — the box visibly
// shrinks by a line.
func TestRenderScrollIndicators_LineCountConstantAcrossOverflow(t *testing.T) {
	cases := [][3]int{
		{0, 5, 5},   // no overflow
		{0, 5, 20},  // overflow below
		{15, 5, 20}, // overflow above
		{5, 5, 20},  // overflow both
	}
	for _, c := range cases {
		above := RenderScrollAbove(c[0], c[1], c[2], 0)
		below := RenderScrollBelow(c[0], c[1], c[2], 0)
		assert.Equal(t, 0, strings.Count(above, "\n"),
			"above indicator must always be exactly one row (case=%v)", c)
		assert.Equal(t, 0, strings.Count(below, "\n"),
			"below indicator must always be exactly one row (case=%v)", c)
	}
}
