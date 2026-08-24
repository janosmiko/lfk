package ui

import (
	"math"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSparkline_RisingRampUsesFullGlyphRange(t *testing.T) {
	got := RenderSparkline([]float64{0, 1, 2, 3, 4, 5, 6, 7}, 8)

	require.Equal(t, 8, lipgloss.Width(got))
	runes := []rune(got)
	assert.Equal(t, '▁', runes[0], "series minimum draws the lowest glyph")
	assert.Equal(t, '█', runes[7], "series maximum draws the highest glyph")
	// Monotone input must produce a monotone glyph sequence.
	glyphs := []rune(SparklineGlyphs)
	idx := func(r rune) int { return strings.IndexRune(string(glyphs), r) }
	for i := 1; i < len(runes); i++ {
		assert.GreaterOrEqual(t, idx(runes[i]), idx(runes[i-1]),
			"glyph %d dipped below its predecessor", i)
	}
}

// A flat series has no range to scale against. It must still draw, because a
// blank cell reads as "no data" when the truth is "steady load".
func TestRenderSparkline_FlatSeriesDraws(t *testing.T) {
	got := RenderSparkline([]float64{5, 5, 5, 5}, 4)

	require.Equal(t, 4, lipgloss.Width(got))
	assert.Equal(t, strings.Repeat("▁", 4), got)
}

func TestRenderSparkline_SinglePoint(t *testing.T) {
	got := RenderSparkline([]float64{42}, 1)

	assert.Equal(t, "▁", got)
}

func TestRenderSparkline_NaNDrawsBlank(t *testing.T) {
	got := RenderSparkline([]float64{0, math.NaN(), 7}, 3)

	require.Equal(t, 3, lipgloss.Width(got))
	runes := []rune(got)
	assert.Equal(t, ' ', runes[1], "a gap must be blank, not a glyph")
	assert.Equal(t, '▁', runes[0])
	assert.Equal(t, '█', runes[2])
}

func TestRenderSparkline_AllNaNReturnsEmpty(t *testing.T) {
	got := RenderSparkline([]float64{math.NaN(), math.NaN()}, 2)

	assert.Empty(t, got, "no real samples means no sparkline, so the cell stays numeric")
}

func TestRenderSparkline_EmptyAndZeroWidth(t *testing.T) {
	assert.Empty(t, RenderSparkline(nil, 8))
	assert.Empty(t, RenderSparkline([]float64{}, 8))
	assert.Empty(t, RenderSparkline([]float64{1, 2}, 0))
	assert.Empty(t, RenderSparkline([]float64{1, 2}, -3))
}

// More samples than columns: the newest sample must survive downsampling,
// because the value beside the sparkline is the newest sample and the two
// would otherwise disagree.
func TestRenderSparkline_DownsampleKeepsNewest(t *testing.T) {
	points := make([]float64, 60)
	for i := range points {
		points[i] = 1
	}
	points[59] = 100

	got := RenderSparkline(points, 10)

	require.Equal(t, 10, lipgloss.Width(got))
	runes := []rune(got)
	assert.Equal(t, '█', runes[9], "the newest sample must land in the last column")
}

// Fewer samples than columns: the sparkline is exactly as wide as the data,
// so a short history does not pretend to fill the window.
func TestRenderSparkline_FewerPointsThanWidth(t *testing.T) {
	got := RenderSparkline([]float64{0, 5}, 10)

	assert.Equal(t, 2, lipgloss.Width(got))
}

func TestRenderSparkline_NegativeValues(t *testing.T) {
	got := RenderSparkline([]float64{-10, 0, 10}, 3)

	require.Equal(t, 3, lipgloss.Width(got))
	runes := []rune(got)
	assert.Equal(t, '▁', runes[0])
	assert.Equal(t, '█', runes[2])
}

// The value is what the user reads. The glyphs are context. Rendering must
// never introduce an escape sequence, because the column value is parsed for
// numeric sort and measured for layout.
func TestRenderSparkline_ContainsNoEscapeSequences(t *testing.T) {
	got := RenderSparkline([]float64{1, 2, 3, 4}, 4)

	assert.NotContains(t, got, "\x1b")
}
