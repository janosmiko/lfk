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
		"", false, 0, false, 0, 'v', 0, 0, 0, "", "", false, false, nil,
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
		'v', false, "", 0, "", "", false, false,
	)
	assert.Contains(t, out, "[2 of 4 lines]")

	out2 := renderLogTitleBar("test", []string{"a", "b", "c", "d"}, 4, 80,
		false, false, false, false, false, false, false,
		'v', false, "", 0, "", "", false, false,
	)
	assert.Contains(t, out2, "[4 lines]")
	assert.NotContains(t, out2, "of")
}

func TestTitleBarShowsFilterChip(t *testing.T) {
	// ruleCount=3 should render a [FILTER: 3] chip.
	out := renderLogTitleBar("test", []string{"a"}, 1, 80,
		false, false, false, false, false, false, false,
		'v', false, "", 3, "", "", false, false,
	)
	assert.Contains(t, out, "[FILTER: 3]")

	// ruleCount=0 should not render the chip.
	out2 := renderLogTitleBar("test", []string{"a"}, 1, 80,
		false, false, false, false, false, false, false,
		'v', false, "", 0, "", "", false, false,
	)
	assert.NotContains(t, out2, "FILTER")
}

// TestTitleBarShowsSinceChip is the Phase 3A title-bar contract: a
// non-empty --since value surfaces as a `[SINCE: 5m]` chip next to
// the filter chip.  Empty means no chip.
func TestTitleBarShowsSinceChip(t *testing.T) {
	out := renderLogTitleBar("test", []string{"a"}, 1, 80,
		false, false, false, false, false, false, false,
		'v', false, "", 0, "", "5m", false, false,
	)
	assert.Contains(t, out, "[SINCE: 5m]")

	out2 := renderLogTitleBar("test", []string{"a"}, 1, 80,
		false, false, false, false, false, false, false,
		'v', false, "", 0, "", "", false, false,
	)
	assert.NotContains(t, out2, "SINCE")
}
