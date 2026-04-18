package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withForcedColor pins the lipgloss default renderer to TrueColor for
// the duration of t so style assertions (cursor highlight, etc.) can
// inspect ANSI escapes. Without this pinned profile, lipgloss detects
// that tests run without a TTY and strips all color output.
func withForcedColor(t *testing.T) {
	t.Helper()
	r := lipgloss.DefaultRenderer()
	prev := r.ColorProfile()
	r.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { r.SetColorProfile(prev) })
}

// TestRenderLogHistogramWidth pins the visible width contract: no
// matter what the source bucket count is, the rendered string must
// be exactly `width` cells wide. This is critical because the
// renderer slots the strip into a column-precise layout — any
// width drift causes the title bar / content border to misalign.
func TestRenderLogHistogramWidth(t *testing.T) {
	cases := []struct {
		name   string
		view   LogHistogramView
		width  int
		expect int
	}{
		{
			name:   "more buckets than width is rebucketed",
			view:   LogHistogramView{Counts: makeCounts(120, 1)},
			width:  40,
			expect: 40,
		},
		{
			name:   "fewer buckets than width is stretched",
			view:   LogHistogramView{Counts: []int{1, 2, 3}},
			width:  40,
			expect: 40,
		},
		{
			name:   "exact bucket count matches width",
			view:   LogHistogramView{Counts: makeCounts(40, 1)},
			width:  40,
			expect: 40,
		},
		{
			name:   "with cursor highlight still respects width",
			view:   LogHistogramView{Counts: makeCounts(40, 1), CursorBucket: 10},
			width:  40,
			expect: 40,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := RenderLogHistogram(tc.view, tc.width)
			require.NotEmpty(t, out, "non-empty histogram should render some output")
			assert.Equal(t, tc.expect, lipgloss.Width(out),
				"rendered histogram width must equal target width")
		})
	}
}

// TestRenderLogHistogramEmpty asserts that a zero view returns the
// empty string so callers can drop the row entirely without producing
// a blank line in the layout.
func TestRenderLogHistogramEmpty(t *testing.T) {
	t.Run("nil counts", func(t *testing.T) {
		out := RenderLogHistogram(LogHistogramView{}, 40)
		assert.Equal(t, "", out)
	})

	t.Run("zero width", func(t *testing.T) {
		out := RenderLogHistogram(LogHistogramView{Counts: []int{1, 2, 3}}, 0)
		assert.Equal(t, "", out)
	})

	t.Run("negative width", func(t *testing.T) {
		out := RenderLogHistogram(LogHistogramView{Counts: []int{1, 2, 3}}, -5)
		assert.Equal(t, "", out)
	})
}

// TestRenderLogHistogramCursorHighlight verifies that the column
// containing the cursor bucket gets the inverse-video escape applied.
// We assert by counting the inverse-video escape `\x1b[7m` in the
// rendered output — the renderer's other styling does not use
// reverse, so the presence/absence is unambiguous.
func TestRenderLogHistogramCursorHighlight(t *testing.T) {
	withForcedColor(t)

	// Reverse video is SGR 7. Lipgloss may emit it bundled with bold
	// (`\x1b[1;7m`) or alone — we accept either by spotting the `;7m`
	// or `[7m` suffix inside any SGR escape.
	hasReverse := func(s string) bool {
		// Strip trailing "m" terminators in escape sequences.
		// Look for both forms: `[7m` (reverse alone) and `;7m`
		// (reverse combined with another SGR attribute like bold).
		return strings.Contains(s, "[7m") || strings.Contains(s, ";7m")
	}

	t.Run("cursor highlight present", func(t *testing.T) {
		view := LogHistogramView{
			Counts:       makeCounts(20, 5),
			CursorBucket: 5,
		}
		out := RenderLogHistogram(view, 40)
		assert.True(t, hasReverse(out),
			"cursor highlight should include reverse-video ANSI escape: %q", out)
	})

	t.Run("no cursor → no highlight", func(t *testing.T) {
		view := LogHistogramView{
			Counts:       makeCounts(20, 5),
			CursorBucket: -1,
		}
		out := RenderLogHistogram(view, 40)
		assert.False(t, hasReverse(out),
			"no cursor → no reverse-video escape should appear")
	})

	t.Run("out-of-range cursor is ignored", func(t *testing.T) {
		view := LogHistogramView{
			Counts:       makeCounts(20, 5),
			CursorBucket: 9999,
		}
		out := RenderLogHistogram(view, 40)
		assert.False(t, hasReverse(out),
			"out-of-range cursor must not produce a phantom highlight")
	})
}

// TestRenderLogHistogramLogScaling exercises the log-scaling guarantee
// from the spec: a skewed distribution (one tall bucket alongside many
// tiny ones) must still surface the tiny ones as ≥ '▁' rather than
// rounding them to blank columns. We verify by checking the rendered
// output for the lowest block character on every column that had a
// non-zero source count.
func TestRenderLogHistogramLogScaling(t *testing.T) {
	// One hot bucket (1000) and four tiny ones (1 each). Linear scaling
	// would push the tiny buckets to 0 visual height; log scaling keeps
	// them at '▁' or higher.
	counts := []int{1000, 1, 1, 1, 1}
	view := LogHistogramView{Counts: counts}
	out := RenderLogHistogram(view, 5) // 1 column per bucket

	// Strip ANSI escapes so we can index by visible rune.
	plain := stripANSI(out)
	require.Equal(t, 5, len([]rune(plain)),
		"plain output should be exactly width runes wide")

	runes := []rune(plain)

	// The hot bucket should be the tallest block.
	assert.Equal(t, '\u2588', runes[0],
		"hot bucket should render as full block '█'")

	// Each tiny bucket should be at least the lowest non-empty block.
	for i := 1; i < 5; i++ {
		assert.NotEqual(t, ' ', runes[i],
			"tiny bucket %d should NOT render as blank under log scaling", i)
		// The exact block char varies with the log scale, but it must
		// fall in the visible range '▁'..'█' (U+2581..U+2588).
		assert.GreaterOrEqual(t, runes[i], '\u2581',
			"tiny bucket %d should be ≥ '▁'", i)
		assert.LessOrEqual(t, runes[i], '\u2588',
			"tiny bucket %d should be ≤ '█'", i)
	}
}

// TestRenderLogHistogramEmptyBucketRendersBlank is the dual contract
// to TestRenderLogHistogramLogScaling: a bucket with count == 0 must
// render as a literal space, not as '▁'. Without this, the user
// would see a "noise floor" of low blocks even for periods with no
// log activity.
func TestRenderLogHistogramEmptyBucketRendersBlank(t *testing.T) {
	counts := []int{5, 0, 5, 0, 5}
	view := LogHistogramView{Counts: counts}
	out := RenderLogHistogram(view, 5)

	runes := []rune(stripANSI(out))
	require.Len(t, runes, 5)

	// Empty buckets render as ' ', non-empty as some block char.
	assert.Equal(t, ' ', runes[1], "empty bucket 1 should render blank")
	assert.Equal(t, ' ', runes[3], "empty bucket 3 should render blank")
	assert.NotEqual(t, ' ', runes[0], "non-empty bucket 0 should render a block")
	assert.NotEqual(t, ' ', runes[2], "non-empty bucket 2 should render a block")
	assert.NotEqual(t, ' ', runes[4], "non-empty bucket 4 should render a block")
}

// TestProjectHistogramToWidthRebuckets exercises the rebucketing
// helper directly: 100 source buckets summed into 10 columns must
// preserve the total count, and the per-column distribution should
// be roughly uniform when the source is uniform.
func TestProjectHistogramToWidthRebuckets(t *testing.T) {
	src := makeCounts(100, 1)
	out := projectHistogramToWidth(src, 10)
	require.Len(t, out, 10)

	total := 0
	for _, c := range out {
		total += c
	}
	assert.Equal(t, 100, total,
		"total count must be preserved through rebucketing")

	// Each output column should hold roughly 100/10 = 10 source counts.
	for i, c := range out {
		assert.GreaterOrEqual(t, c, 9, "col %d under-allocated", i)
		assert.LessOrEqual(t, c, 11, "col %d over-allocated", i)
	}
}

// TestProjectHistogramToWidthStretches exercises the opposite
// direction: 3 source buckets stretched to 10 columns. The total
// count must be preserved, and the columns should distribute the
// counts across roughly even spans.
func TestProjectHistogramToWidthStretches(t *testing.T) {
	src := []int{6, 9, 15}
	out := projectHistogramToWidth(src, 10)
	require.Len(t, out, 10)

	total := 0
	for _, c := range out {
		total += c
	}
	assert.Equal(t, 30, total,
		"total count must be preserved when stretching")
}

// TestProjectCursorToWidth verifies the cursor projection: a cursor
// at source index N out of M source buckets should map to roughly
// N/M * width. Out-of-range inputs return -1.
func TestProjectCursorToWidth(t *testing.T) {
	cases := []struct {
		name      string
		srcBucket int
		srcLen    int
		width     int
		want      int
	}{
		{"middle cursor projects to middle column", 5, 10, 40, 20},
		{"cursor at start projects to col 0", 0, 10, 40, 0},
		{"cursor at last source bucket lands inside width", 9, 10, 40, 36},
		{"cursor past end returns -1", 99, 10, 40, -1},
		{"negative cursor returns -1", -1, 10, 40, -1},
		{"empty source returns -1", 5, 0, 40, -1},
		{"zero width returns -1", 5, 10, 0, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := projectCursorToWidth(tc.srcBucket, tc.srcLen, tc.width)
			assert.Equal(t, tc.want, got)
		})
	}
}

// makeCounts returns a slice of length n where every element is v.
// Test helper to keep histogram fixtures terse.
func makeCounts(n, v int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = v
	}
	return out
}
