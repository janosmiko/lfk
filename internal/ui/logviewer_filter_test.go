package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderLogViewerWithVisibleIndices(t *testing.T) {
	lines := []string{"keep1", "drop", "keep2", "drop", "keep3"}
	visible := []int{0, 2, 4}

	out := RenderLogViewer(
		lines, visible, // new param
		0, 80, 10,
		false, false, false, false, false, false,
		"test", "", "",
		false, false, false, false, false,
		"", false, 0, false, 0, 'v', 0, 0, 0, "", LogTimeRangeView{}, false, false, nil, LogHistogramView{},
	)

	assert.Contains(t, out, "keep1")
	assert.Contains(t, out, "keep2")
	assert.Contains(t, out, "keep3")
	assert.False(t, strings.Contains(out, "\ndrop\n"), "filtered lines should not appear")
}

func TestTitleBarShowsXofYWhenFiltered(t *testing.T) {
	// flags: follow, wrap, lineNumbers, timestamps, previous, hidePrefixes, visualMode
	out := renderLogTitleBar("test", []string{"a", "b", "c", "d"}, 2, 80,
		false, false, false, false, false, false, false,
		'v', false, "", 0, "", LogTimeRangeView{}, false, false, LogHistogramView{},
	)
	assert.Contains(t, out, "[2 of 4 lines]")

	out2 := renderLogTitleBar("test", []string{"a", "b", "c", "d"}, 4, 80,
		false, false, false, false, false, false, false,
		'v', false, "", 0, "", LogTimeRangeView{}, false, false, LogHistogramView{},
	)
	assert.Contains(t, out2, "[4 lines]")
	assert.NotContains(t, out2, "of")
}

func TestTitleBarShowsFilterChip(t *testing.T) {
	// ruleCount=3 should render a [FILTER: 3] chip.
	out := renderLogTitleBar("test", []string{"a"}, 1, 80,
		false, false, false, false, false, false, false,
		'v', false, "", 3, "", LogTimeRangeView{}, false, false, LogHistogramView{},
	)
	assert.Contains(t, out, "[FILTER: 3]")

	// ruleCount=0 should not render the chip.
	out2 := renderLogTitleBar("test", []string{"a"}, 1, 80,
		false, false, false, false, false, false, false,
		'v', false, "", 0, "", LogTimeRangeView{}, false, false, LogHistogramView{},
	)
	assert.NotContains(t, out2, "FILTER")
}

// TestTitleBarShowsRangeChip is the Phase 3A title-bar contract: a
// non-empty LogTimeRangeView surfaces as a `[RANGE: …]` chip next to
// the filter chip. A zero-value view means no chip.
func TestTitleBarShowsRangeChip(t *testing.T) {
	out := renderLogTitleBar("test", []string{"a"}, 1, 80,
		false, false, false, false, false, false, false,
		'v', false, "", 0, "", LogTimeRangeView{Display: "-24h"}, false, false, LogHistogramView{},
	)
	assert.Contains(t, out, "[RANGE: -24h]")

	out2 := renderLogTitleBar("test", []string{"a"}, 1, 80,
		false, false, false, false, false, false, false,
		'v', false, "", 0, "", LogTimeRangeView{}, false, false, LogHistogramView{},
	)
	assert.NotContains(t, out2, "RANGE")
}
