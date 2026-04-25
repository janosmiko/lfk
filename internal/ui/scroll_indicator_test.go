package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

// RenderScrollIndicator renders a single dim row showing how many items
// are hidden above and/or below the current viewport. Returns "" when
// nothing is hidden, so callers don't have to gate the call.

func TestRenderScrollIndicator_NothingHiddenReturnsEmpty(t *testing.T) {
	got := RenderScrollIndicator(0, 5, 5, 40)
	assert.Empty(t, got, "no overflow → no indicator row")
}

func TestRenderScrollIndicator_OnlyBelow(t *testing.T) {
	got := RenderScrollIndicator(0, 5, 20, 40)
	plain := stripANSI(got)
	assert.NotContains(t, plain, "above")
	assert.Contains(t, plain, "15")
	assert.Contains(t, plain, "below")
}

func TestRenderScrollIndicator_OnlyAbove(t *testing.T) {
	got := RenderScrollIndicator(15, 5, 20, 40)
	plain := stripANSI(got)
	assert.Contains(t, plain, "15")
	assert.Contains(t, plain, "above")
	assert.NotContains(t, plain, "below")
}

func TestRenderScrollIndicator_BothDirections(t *testing.T) {
	got := RenderScrollIndicator(5, 5, 20, 40)
	plain := stripANSI(got)
	assert.Contains(t, plain, "5")
	assert.Contains(t, plain, "above")
	assert.Contains(t, plain, "10")
	assert.Contains(t, plain, "below")
}

func TestRenderScrollIndicator_SingleLine(t *testing.T) {
	got := RenderScrollIndicator(5, 5, 20, 40)
	assert.NotContains(t, got, "\n",
		"indicator must be exactly one row so callers can budget it")
}

func TestRenderScrollIndicator_HandlesBadInput(t *testing.T) {
	// Negative or zero values shouldn't panic — defensive default is "no
	// indicator", same as the no-overflow case.
	assert.Empty(t, RenderScrollIndicator(-1, 5, 10, 40))
	assert.Empty(t, RenderScrollIndicator(0, 0, 0, 40))
}

func TestRenderScrollIndicator_TruncatesToWidth(t *testing.T) {
	// Even with absurd numbers the row fits the requested width — the
	// overlay frame budget can't grow.
	got := RenderScrollIndicator(99999, 1, 999999, 20)
	assert.LessOrEqual(t, lipgloss.Width(strings.TrimRight(stripANSI(got), " ")), 20,
		"indicator must respect the width budget")
}
