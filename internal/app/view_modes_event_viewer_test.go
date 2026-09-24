package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// renderEventViewerLines highlights a committed search query on both the
// cursor line and the surrounding non-cursor lines. Passing the raw query
// through (rather than a lowered copy) keeps ~/regex prefixes meaningful.
func TestRenderEventViewerLines_HighlightsCursorAndNonCursorLines(t *testing.T) {
	m := basePush80Model()
	lines := []string{"warning pod evicted", "normal pod scheduled", "warning pod evicted"}
	m.eventTimelineCursor = 1 // non-cursor lines are index 0 and 2

	plain := m.renderEventViewerLines(lines, 0, len(lines), 40)

	m.eventTimelineSearchQuery = "warning"
	highlighted := m.renderEventViewerLines(lines, 0, len(lines), 40)

	assert.NotEqual(t, plain, highlighted, "a committed search query must change the rendered lines")
	assert.Equal(t, stripANSI(strings.Join(plain, "\n")), stripANSI(strings.Join(highlighted, "\n")),
		"highlighting must not alter the visible text")
}
