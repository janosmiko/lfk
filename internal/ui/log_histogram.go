package ui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// LogHistogramView is the value type the app-side passes to
// RenderLogHistogram. It exists because internal/ui cannot import
// internal/app — the histogram is built upstream (BuildLogHistogram)
// and projected into this struct so the renderer never sees the
// app-side type or pulls in the time-bucketing logic.
//
// Counts is the per-bucket density (one entry per source-side
// bucket). The renderer rebuckets / scales Counts to fit the target
// width on the fly so the caller need not concern itself with the
// final column count.
//
// CursorBucket is the source-side bucket index containing the cursor
// line, or -1 when not applicable. The renderer projects the index
// through the same width-mapping it applies to Counts.
//
// BucketStepStr is a short human spelling of the bucket width
// (e.g. "5m"). Surfaced in the title bar / footer when the strip is
// visible; never used in the strip itself.
type LogHistogramView struct {
	Counts        []int
	CursorBucket  int
	BucketStepStr string
}

// histogramBlocks lists the box-drawing block characters used to
// represent bucket counts as visual heights. Index 0 is "no count"
// (rendered as a blank space — the strip stays as a single row, so we
// must reserve a non-block character to mean "empty bucket"). Indices
// 1..8 are the eight Unicode block-eighth characters from "▁" up to
// "█". Selecting an index = log2(count) / log2(maxCount) * 8 produces
// a log-scaled histogram that still surfaces low-count buckets as a
// visible "▁" rather than vanishing entirely.
var histogramBlocks = []rune{
	' ',      // 0: empty bucket
	'\u2581', // 1: ▁ lower one eighth block
	'\u2582', // 2: ▂
	'\u2583', // 3: ▃
	'\u2584', // 4: ▄
	'\u2585', // 5: ▅
	'\u2586', // 6: ▆
	'\u2587', // 7: ▇
	'\u2588', // 8: █
}

// histogramCursorStyle highlights the bucket containing the cursor.
// Inverse video is the cleanest way to make the indicator legible
// regardless of theme: it works on the empty " " (when the cursor
// bucket has count zero) AND on filled blocks alike.
var histogramCursorStyle = lipgloss.NewStyle().Reverse(true).Bold(true)

// histogramBlockStyle is the default style applied to non-cursor
// buckets. Subdued so the histogram doesn't compete with the log
// content for attention; the cursor bucket pops only because it
// flips to inverse video.
var histogramBlockStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary))

// RenderLogHistogram returns exactly one row of width `width`
// suitable for splicing into the log viewer between the title bar and
// the content area. Returns the empty string when the view contains
// no counts (caller should skip the row entirely in that case).
//
// The renderer:
//   - Rebuckets `view.Counts` into `width` columns: when there are
//     more source buckets than columns, neighbouring buckets are
//     summed; when there are fewer, each source bucket spans multiple
//     columns. This keeps the strip readable regardless of terminal
//     width.
//   - Scales each rebucketed count to one of nine block heights
//     (' ', '▁'..'█') using a log2 transform so a few hot buckets
//     don't crush all the others to '▁'.
//   - Highlights the column containing `view.CursorBucket` (also
//     projected through the rebucketing) with reverse video.
//
// The output is always exactly `width` runes wide so callers can
// concatenate it with the title bar / content without re-measuring.
func RenderLogHistogram(view LogHistogramView, width int) string {
	if width <= 0 || len(view.Counts) == 0 {
		return ""
	}

	cols := projectHistogramToWidth(view.Counts, width)
	cursorCol := projectCursorToWidth(view.CursorBucket, len(view.Counts), width)
	maxCount := 0
	for _, c := range cols {
		if c > maxCount {
			maxCount = c
		}
	}

	var b strings.Builder
	b.Grow(width * 4) // ANSI escapes + UTF-8 block chars
	for i, c := range cols {
		ch := histogramBlockForCount(c, maxCount)
		styled := histogramBlockStyle.Render(string(ch))
		if i == cursorCol {
			styled = histogramCursorStyle.Render(string(ch))
		}
		b.WriteString(styled)
	}
	return b.String()
}

// projectHistogramToWidth rebuckets `counts` into exactly `width`
// columns. When len(counts) > width we sum each contiguous source
// slice into one output column. When len(counts) < width we expand:
// each source bucket maps to one or more adjacent output columns,
// with the count divided proportionally so the visual area still
// reflects the source distribution.
//
// We rebucket here rather than in the app-side builder so the
// builder doesn't need to know the target column count — it can
// produce a "natural" histogram and the renderer handles the layout.
func projectHistogramToWidth(counts []int, width int) []int {
	if width <= 0 || len(counts) == 0 {
		return nil
	}
	out := make([]int, width)
	if len(counts) >= width {
		// Many source buckets, few columns: sum each slice. The slice
		// boundaries use floating math to avoid bias toward the front
		// of the buffer when len(counts) is not a multiple of width.
		for i, c := range counts {
			col := int(float64(i) * float64(width) / float64(len(counts)))
			if col >= width {
				col = width - 1
			}
			out[col] += c
		}
		return out
	}

	// Few source buckets, many columns: stretch. Each source bucket
	// covers a slice of [colStart, colEnd) output columns; we spread
	// its count evenly with a final +1 going to the first column to
	// avoid losing the bucket's existence when the count is small.
	for i, c := range counts {
		colStart := int(float64(i) * float64(width) / float64(len(counts)))
		colEnd := int(float64(i+1) * float64(width) / float64(len(counts)))
		if colEnd <= colStart {
			colEnd = colStart + 1
		}
		if colEnd > width {
			colEnd = width
		}
		span := colEnd - colStart
		if span <= 0 || c == 0 {
			continue
		}
		share := c / span
		remainder := c - share*span
		for col := colStart; col < colEnd; col++ {
			out[col] += share
		}
		// Drop the remainder on the first column of the span so the
		// total count is preserved and the bucket doesn't visually
		// vanish when c < span.
		out[colStart] += remainder
	}
	return out
}

// projectCursorToWidth maps a source-side bucket index to the
// output column index after the histogram has been rebucketed to
// `width` columns. Returns -1 when the cursor index is out of
// range (negative or >= srcLen) so the renderer skips the highlight
// cleanly. Out-of-range values are NOT clamped — that would draw a
// phantom indicator on a column the source never reached.
func projectCursorToWidth(srcBucket, srcLen, width int) int {
	if srcBucket < 0 || srcLen <= 0 || width <= 0 {
		return -1
	}
	if srcBucket >= srcLen {
		return -1
	}
	col := int(float64(srcBucket) * float64(width) / float64(srcLen))
	if col >= width {
		col = width - 1
	}
	return col
}

// histogramBlockForCount returns the block character whose height
// represents `count` on a log2 scale relative to `maxCount`. Returns
// the empty-bucket placeholder (' ') when count is zero.
//
// The log scale uses ln(count+1) / ln(maxCount+1) so a single hot
// bucket (count == maxCount) still reaches the tallest block (█) and
// the smallest non-zero bucket reaches at least the lowest block (▁)
// rather than rounding to 0. The "+1" handles the maxCount==1 corner
// where ln(maxCount) would be 0 and the ratio undefined.
func histogramBlockForCount(count, maxCount int) rune {
	if count <= 0 {
		return histogramBlocks[0]
	}
	if maxCount <= 1 {
		// Only one count value across the whole strip — render every
		// non-zero bucket at the tallest block so the user can still
		// see the distribution shape.
		return histogramBlocks[len(histogramBlocks)-1]
	}
	ratio := math.Log(float64(count)+1) / math.Log(float64(maxCount)+1)
	idx := int(math.Round(ratio * float64(len(histogramBlocks)-1)))
	if idx < 1 {
		idx = 1 // ensure low-count buckets are at least one '▁' tall
	}
	if idx >= len(histogramBlocks) {
		idx = len(histogramBlocks) - 1
	}
	return histogramBlocks[idx]
}
