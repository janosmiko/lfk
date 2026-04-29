package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// renderOverlayColumnToggle must size the renderer to the overlay box
// dimensions, not the full screen. Otherwise on tall terminals the
// renderer emits ~maxScreen-6 lines into a 20-tall box; the box grows
// past its target on overflow and "shrinks" back as the filter
// narrows results — looks like the window is resizing.
func TestRenderOverlayColumnToggle_StableSizeAcrossFilter(t *testing.T) {
	m := baseModelCov()
	m.height = 60 // tall terminal — exposes the screen-vs-box mismatch
	m.width = 200

	// Stuff plenty of entries so an unfiltered view has too many to fit.
	m.columnToggleItems = make([]columnToggleEntry, 30)
	for i := range m.columnToggleItems {
		m.columnToggleItems[i] = columnToggleEntry{key: "col" + string(rune('A'+i%26))}
	}

	_, _, fullH := m.renderOverlayColumnToggle()

	m.columnToggleFilter = "colA"
	_, _, filteredH := m.renderOverlayColumnToggle()

	assert.Equal(t, fullH, filteredH,
		"overlay box height must stay constant whether 30 or 1 entry matches the filter")
}
