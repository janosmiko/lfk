package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// In wrap mode each event expands to several physical lines, but the scroll the
// key handler supplies is in logical-entry units. RenderEventViewer paginates by
// physical line, so without a physical-aware start resolution the cursor entry
// can be pushed off-screen while the footer still reports its number. The cursor
// gutter must always be rendered.
func TestRenderEventViewerCursorVisibleUnderWrap(t *testing.T) {
	lines := make([]string, 6)
	for i := range lines {
		// Long enough to wrap to multiple physical lines at the narrow width.
		lines[i] = "event row that is quite long and will wrap several times over"
	}

	p := EventViewerParams{
		Lines:  lines,
		Scroll: 0, // logical scroll pinned at top, as the wrap-unaware handler leaves it
		Cursor: 5, // last entry — far below the physical budget from scroll 0
		Width:  30,
		Height: 10, // maxVisible = 6 physical lines
		Wrap:   true,
	}

	out := stripANSI(RenderEventViewer(p))

	assert.Contains(t, out, "▎", "cursor gutter must be on-screen even when earlier entries wrap")
}

// The footer's truncated "line N/M" indicator is gated on `end` (the last
// logical row actually rendered). When wrapped rows fill the physical budget
// before all events are shown — even at scroll 0 — `end` must reflect that, or
// the footer wrongly claims everything fits.
func TestRenderEventViewerFooterTruncationUnderWrap(t *testing.T) {
	lines := make([]string, 6)
	for i := range lines {
		lines[i] = "event row that is quite long and will wrap several times over"
	}

	p := EventViewerParams{
		Lines:  lines,
		Scroll: 0,
		Cursor: 0, // top — only the cursor + a couple wrapped rows fit the budget
		Width:  30,
		Height: 10,
		Wrap:   true,
	}

	out := stripANSI(RenderEventViewer(p))

	assert.Contains(t, out, "line 1/6",
		"footer must report truncation when wrapped rows fill the budget before all events render")
}
