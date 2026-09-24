package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A control query that matches nothing takes the same search path, so a line
// can only differ from it through match highlighting.
func TestRenderEventViewerLines_HighlightsCursorAndNonCursorLines(t *testing.T) {
	m := basePush80Model()
	lines := []string{"warning pod evicted", "warning pod killed", "normal pod scheduled"}
	m.eventTimelineCursor = 1

	m.eventTimelineSearchQuery = "zzz"
	control := m.renderEventViewerLines(lines, 0, len(lines), 40)
	m.eventTimelineSearchQuery = "warning"
	highlighted := m.renderEventViewerLines(lines, 0, len(lines), 40)
	require.Len(t, highlighted, len(control))

	assert.NotEqual(t, control[0], highlighted[0], "non-cursor match not highlighted")
	assert.NotEqual(t, control[1], highlighted[1], "cursor match not highlighted")
	assert.Equal(t, control[2], highlighted[2], "line without a match must not change")
	for i := range control {
		assert.Equal(t, stripANSI(control[i]), stripANSI(highlighted[i]), "line %d: text changed", i)
	}
}
