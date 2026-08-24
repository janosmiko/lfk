package ui

import (
	"math"
	"strings"
)

// SparklineGlyphs are the eight block heights a sparkline draws with, lowest
// first. One glyph is one sample.
const SparklineGlyphs = "▁▂▃▄▅▆▇█"

// RenderSparkline draws points as a plain-text block sparkline at most width
// columns wide, oldest sample first.
//
// The result carries no ANSI styling on purpose. It is written into a table
// column value, which ParseResourceValueOK parses for numeric sort and
// lipgloss.Width measures for layout. An escape sequence there corrupts both.
// The caller styles the finished cell.
//
// Scaling is per series, from its own minimum to its own maximum, so a quiet
// pod and a busy pod are both readable. A NaN sample draws a blank column
// rather than being dropped, because dropping it would slide every later
// sample to the wrong position on the time axis. A series with no real
// samples returns "", which leaves the cell numeric.
func RenderSparkline(points []float64, width int) string {
	if width <= 0 || len(points) == 0 {
		return ""
	}

	drawn := resampleSparkline(points, width)

	minV, maxV := math.Inf(1), math.Inf(-1)
	for _, v := range drawn {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		minV = math.Min(minV, v)
		maxV = math.Max(maxV, v)
	}
	if math.IsInf(minV, 1) {
		return "" // no real samples
	}

	glyphs := []rune(SparklineGlyphs)
	span := maxV - minV
	var b strings.Builder
	b.Grow(len(drawn) * 3)
	for _, v := range drawn {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			b.WriteRune(' ')
			continue
		}
		if span == 0 {
			// A flat series has no range to scale against. Draw the lowest
			// glyph rather than nothing: a blank cell reads as "no data"
			// when the truth is "steady load".
			b.WriteRune(glyphs[0])
			continue
		}
		idx := int(math.Round((v - minV) / span * float64(len(glyphs)-1)))
		idx = max(0, min(idx, len(glyphs)-1))
		b.WriteRune(glyphs[idx])
	}
	return b.String()
}

// resampleSparkline reduces points to at most width samples. It anchors on the
// newest sample so the last column always shows the same reading as the
// numeric value printed beside the sparkline. Fewer points than width are
// returned as-is, so a short history draws short instead of stretching to
// fill a window it does not cover.
func resampleSparkline(points []float64, width int) []float64 {
	if len(points) <= width {
		return points
	}
	out := make([]float64, width)
	// Walk backwards from the newest sample so index width-1 is always
	// points[len-1], whatever the ratio.
	step := float64(len(points)-1) / float64(width-1)
	if width == 1 {
		return []float64{points[len(points)-1]}
	}
	for i := range width {
		idx := int(math.Round(float64(len(points)-1) - step*float64(width-1-i)))
		idx = max(0, min(idx, len(points)-1))
		out[i] = points[idx]
	}
	return out
}
