package app

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

// barCell is a run of filled bar cells drawn in one segment color.
type barCell struct {
	n     int
	style lipgloss.Style
}

// metricBar is a dashboard bar that can draw itself two ways: classic
// (foreground blocks, legend on its own line) or with the legend written
// inside the bar.
type metricBar struct {
	classic string
	cells   []barCell
	width   int
	suffix  string
}

func stackedMetricBar(segments []struct {
	count int
	style lipgloss.Style
}, total, width int,
) metricBar {
	b := metricBar{classic: renderStackedBar(segments, total, width), width: width}
	for i, n := range stackedBarCounts(segments, total, width) {
		b.cells = append(b.cells, barCell{n: n, style: segments[i].style})
	}
	return b
}

func percentMetricBar(used, total int64, width int) metricBar {
	b := metricBar{classic: renderBar(used, total, width), width: width, suffix: "  N/A"}
	if total > 0 {
		filled, style, pct := barFill(used, total, width)
		b.cells = []barCell{{n: filled, style: style}}
		b.suffix = " " + formatBarPct(pct)
	}
	return b
}

// inline reports false when the text cannot fit or colors are off, so the
// caller keeps the two-line layout.
func (b metricBar) inline(text string) (string, bool) {
	if ui.ConfigNoColor || b.width <= 0 || utf8.RuneCountInString(text) != lipgloss.Width(text) ||
		lipgloss.Width(text) > b.width {
		return "", false
	}
	runes := []rune(text + strings.Repeat(" ", b.width-lipgloss.Width(text)))
	filledText := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBase))
	track := ui.NormalStyle.Background(lipgloss.Color(ui.ColorBarBg))

	var sb strings.Builder
	pos := 0
	for _, c := range b.cells {
		n := min(c.n, b.width-pos)
		if n <= 0 {
			continue
		}
		bg := c.style.GetForeground()
		sb.WriteString(filledText.Background(bg).Render(string(runes[pos : pos+n])))
		pos += n
	}
	if pos < b.width {
		sb.WriteString(track.Render(string(runes[pos:])))
	}
	return "[" + sb.String() + "]" + b.suffix, true
}
