package app

import (
	"fmt"
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
	// labels parallel cells ("" = none). tail labels the empty cells after
	// the last segment. With neither set, one whole-bar summary is written.
	labels []string
	tail   string
}

// countLabels names each non-zero segment "<count> <name>".
func countLabels(segments []struct {
	count int
	style lipgloss.Style
}, names ...string,
) []string {
	labels := make([]string, len(segments))
	for i, s := range segments {
		if s.count > 0 {
			labels[i] = fmt.Sprintf("%d %s", s.count, names[i])
		}
	}
	return labels
}

func (b metricBar) withLabels(labels []string, tail string) metricBar {
	b.labels, b.tail = labels, tail
	return b
}

func fitsIn(text string, room int) bool {
	return utf8.RuneCountInString(text) == lipgloss.Width(text) && lipgloss.Width(text) <= room
}

// anchoredText writes each label at the first cell of its own region, or
// reports false if any label does not fit there.
func (b metricBar) anchoredText() (string, bool) {
	runes := []rune(strings.Repeat(" ", b.width))
	pos := 0
	place := func(label string, room int) bool {
		if label == "" {
			return true
		}
		// A gap cell must follow the label unless the region ends the bar.
		if !fitsIn(label, room-min(1, b.width-pos-room)) {
			return false
		}
		copy(runes[pos:], []rune(label))
		return true
	}
	for i, c := range b.cells {
		n := min(c.n, b.width-pos)
		if i < len(b.labels) && !place(b.labels[i], n) {
			return "", false
		}
		pos += max(n, 0)
	}
	if !place(b.tail, b.width-pos) {
		return "", false
	}
	return string(runes), true
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

// inline reports false when any label cannot fit its region or colors are
// off, so the caller keeps the two-line layout. text is only used for bars
// without labels.
func (b metricBar) inline(text string) (string, bool) {
	if ui.ConfigNoColor || b.width <= 0 {
		return "", false
	}
	var runes []rune
	if len(b.labels) > 0 || b.tail != "" {
		padded, ok := b.anchoredText()
		if !ok {
			return "", false
		}
		runes = []rune(padded)
	} else {
		if !fitsIn(text, b.width) {
			return "", false
		}
		runes = []rune(text + strings.Repeat(" ", b.width-lipgloss.Width(text)))
	}
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
