package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestDashboardResourcesSection_DrawsSparklineInSparkMode(t *testing.T) {
	data := dashboardData{
		totalCPUUsed: 1000, totalCPUAlloc: 4000,
		totalMemUsed: 1024 * 1024 * 512, totalMemAlloc: 1024 * 1024 * 1024,
		cpuSeries: k8s.MetricSeries{Points: []float64{1, 5, 9}},
		memSeries: k8s.MetricSeries{Points: []float64{9, 5, 1}},
	}
	// content must be set: dashboardMetricLines truncates every line to it,
	// so a zero value renders empty lines regardless of the sparkline.
	w := dashboardWidths{bar: 20, node: 20, sep: 60, label: 6, content: 80}

	lines := dashboardResourcesSection(nil, data, w, true)

	joined := stripANSI(strings.Join(lines, "\n"))
	// Not "█": renderBar's fill uses it too, so a nonzero usage bar draws it
	// with no sparkline present. "▁" is the series minimum and is unique to
	// the sparkline alphabet.
	assert.Contains(t, joined, "▁", "the series minimum must draw the bottom glyph")
	assert.Contains(t, joined, "1.0 / 4.0", "the numeric total must still be shown")
}

func TestDashboardResourcesSection_NumericModeDrawsNoGlyphs(t *testing.T) {
	data := dashboardData{
		totalCPUUsed: 1000, totalCPUAlloc: 4000,
		cpuSeries: k8s.MetricSeries{Points: []float64{1, 5, 9}},
	}
	// content must be set: dashboardMetricLines truncates every line to it,
	// so a zero value renders empty lines regardless of the sparkline.
	w := dashboardWidths{bar: 20, node: 20, sep: 60, label: 6, content: 80}

	lines := dashboardResourcesSection(nil, data, w, false)

	joined := stripANSI(strings.Join(lines, "\n"))
	// "█" is excluded: renderBar's fill uses it too, and a nonzero usage bar
	// (a more realistic fixture than an all-empty one) draws it regardless of
	// sparklines. The other seven glyphs are unique to the sparkline alphabet.
	for _, g := range ui.SparklineGlyphs {
		if g == '█' {
			continue
		}
		assert.NotContains(t, joined, string(g),
			"numeric mode must render identically to before this feature")
	}
}

// w.bar has a floor of 8, so it stops shrinking with the pane. On a narrow pane
// with a long pinned label the lead plus the bar runs wider than the content
// column, and an untruncated sparkline line then shifts the whole layout.
func TestAppendDashboardSparkline_FitsNarrowContentWidth(t *testing.T) {
	w := dashboardWidths{bar: 8, label: 14, content: 20}
	series := k8s.MetricSeries{Points: []float64{1, 3, 5, 7, 9, 7, 5, 3}}

	lines := appendDashboardSparkline(nil, series, w, true)

	require.Len(t, lines, 1, "spark mode must draw the line")
	got := lipgloss.Width(stripANSI(lines[0]))
	assert.LessOrEqual(t, got, w.content,
		"the sparkline line must be truncated to the content column like its siblings")
}
