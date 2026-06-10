package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The fullscreen event viewer paginates by physical line from a logical scroll,
// so under wrap a wrapped entry above the cursor can clip it off-screen — same
// class as the overlay bug. viewEventViewer must keep the cursor gutter visible.
func TestFullscreenEventViewerCursorVisibleUnderWrap(t *testing.T) {
	m := baseModelCov()
	m.width = 30
	m.height = 10
	m.eventTimelineWrap = true
	m.eventTimelineCursor = 5
	m.eventTimelineScroll = 0
	m.eventTimelineLines = make([]string, 6)
	for i := range m.eventTimelineLines {
		m.eventTimelineLines[i] = "event row that is quite long and will wrap several times over"
	}

	out := stripANSI(m.viewEventViewer())

	assert.True(t, strings.Contains(out, "▎"),
		"cursor gutter must stay on-screen even when earlier entries wrap")
}
