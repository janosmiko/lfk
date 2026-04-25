package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/ui"
)

// New help-screen contract:
//
// /  → search input (highlights matches inline, doesn't filter); ctrl+n /
//      ctrl+p step through matches while typing; Enter applies + exits
//      input but keeps helpSearchQuery for n/N navigation; Esc cancels.
// f  → filter input (narrows visible lines to matches); Enter applies,
//      Esc clears.
// n/N (no input active) → step through matches of the persisted search
//      query.

func newHelpModel() Model {
	m := baseModelSearch()
	m.mode = modeHelp
	m.height = 40
	m.width = 100
	return m
}

func TestHelpSlashOpensSearchInput(t *testing.T) {
	m := newHelpModel()
	r, _ := m.handleHelpKey(keyMsg("/"))
	rm := r.(Model)
	assert.True(t, rm.helpSearchActive,
		"/ must open the search input (search mode, not filter)")
	assert.False(t, rm.helpFilterActive,
		"/ must not open filter input")
}

func TestHelpFOpensFilterInput(t *testing.T) {
	m := newHelpModel()
	r, _ := m.handleHelpKey(keyMsg("f"))
	rm := r.(Model)
	assert.True(t, rm.helpFilterActive,
		"f must open the filter input (filter mode, not search)")
	assert.False(t, rm.helpSearchActive)
}

func TestHelpSearchEnterPreservesQueryForNavigation(t *testing.T) {
	// Enter in search input commits — exits the input but keeps the
	// query so highlights persist and n/N can navigate.
	m := newHelpModel()
	m.helpSearchActive = true
	m.helpSearchInput.SetValue("bookmark")
	m.helpSearchQuery = "bookmark"

	r, _ := m.handleHelpKey(keyMsg("enter"))
	rm := r.(Model)
	assert.False(t, rm.helpSearchActive, "Enter exits the input")
	assert.Equal(t, "bookmark", rm.helpSearchQuery,
		"Enter preserves the query so highlights stay and n/N work")
}

func TestHelpSearchEscClearsQuery(t *testing.T) {
	m := newHelpModel()
	m.helpSearchActive = true
	m.helpSearchInput.SetValue("bookmark")
	m.helpSearchQuery = "bookmark"

	r, _ := m.handleHelpKey(keyMsg("esc"))
	rm := r.(Model)
	assert.False(t, rm.helpSearchActive)
	assert.Empty(t, rm.helpSearchQuery, "Esc cancels the search")
}

func TestHelpSearchTypingRecomputesMatches(t *testing.T) {
	// As the user types, helpSearchQuery follows and helpMatchLines is
	// recomputed so ctrl+n/ctrl+p have something to walk through.
	m := newHelpModel()
	r, _ := m.handleHelpKey(keyMsg("/"))
	rm := r.(Model)
	r2, _ := rm.handleHelpKey(keyMsg("b"))
	rm2 := r2.(Model)
	assert.Equal(t, "b", rm2.helpSearchQuery, "typing populates the search query")
	// Help has many lines containing "b" — match list should be non-empty.
	assert.NotEmpty(t, rm2.helpMatchLines,
		"helpMatchLines must be recomputed on each keystroke")
}

func TestHelpSearchCtrlNStepsToNextMatch(t *testing.T) {
	m := newHelpModel()
	m.helpSearchActive = true
	m.helpSearchQuery = "filter"
	m.helpRecomputeMatches()
	if len(m.helpMatchLines) < 2 {
		t.Skipf("need at least two matches for this test; got %d", len(m.helpMatchLines))
	}
	m.helpMatchIdx = 0
	startScroll := m.helpScroll

	r, _ := m.handleHelpKey(keyMsg("ctrl+n"))
	rm := r.(Model)
	assert.Equal(t, 1, rm.helpMatchIdx, "ctrl+n advances the match cursor")
	assert.NotEqual(t, startScroll, rm.helpScroll,
		"viewport must scroll to keep the new match in view")
}

func TestHelpNAfterSearchClosesNavigatesMatches(t *testing.T) {
	// After Enter, n/N (without input active) keep walking matches.
	m := newHelpModel()
	m.helpSearchQuery = "filter"
	m.helpRecomputeMatches()
	if len(m.helpMatchLines) < 2 {
		t.Skipf("need at least two matches for this test; got %d", len(m.helpMatchLines))
	}
	m.helpMatchIdx = 0

	r, _ := m.handleHelpKey(keyMsg("n"))
	rm := r.(Model)
	assert.Equal(t, 1, rm.helpMatchIdx, "n must step through matches after search closes")

	r2, _ := rm.handleHelpKey(keyMsg("N"))
	rm2 := r2.(Model)
	assert.Equal(t, 0, rm2.helpMatchIdx, "N steps backward")
}

func TestHelpFilterEnterAppliesAndKeepsValue(t *testing.T) {
	m := newHelpModel()
	r, _ := m.handleHelpKey(keyMsg("f"))
	rm := r.(Model)
	r2, _ := rm.handleHelpKey(keyMsg("a"))
	rm2 := r2.(Model)
	assert.Equal(t, "a", rm2.helpFilter.Value, "typing populates the filter value")

	r3, _ := rm2.handleHelpKey(keyMsg("enter"))
	rm3 := r3.(Model)
	assert.False(t, rm3.helpFilterActive, "Enter exits filter input")
	assert.Equal(t, "a", rm3.helpFilter.Value, "Enter keeps the filter applied")
}

// G (and ctrl+d past the end) used to set helpScroll to 9999 / leave
// it incrementing unbounded. The renderer clamped what it showed, but
// the model state stayed at 9999 — so the next ctrl+u decremented from
// 9999 and the user had to press it many times before scrolling
// visibly moved. Scroll mutations must clamp to the actual max scroll
// (totalLines - visibleLines) so the next move responds immediately.
func TestHelpScrollClampsAfterG(t *testing.T) {
	m := newHelpModel()
	r, _ := m.handleHelpKey(keyMsg("G"))
	rm := r.(Model)
	// max scroll is at most totalLines (well below 9999). Anything in the
	// thousands means we never clamped.
	assert.Less(t, rm.helpScroll, 1000,
		"G must clamp to actual max scroll, not park the model at 9999")
}

// G must scroll all the way to the bottom — the renderer's "↓ more
// below" indicator should disappear after the jump. Reproduces a
// clamp-formula drift where helpVisibleLines was computed differently
// than the renderer's, so the clamp stopped short and the "more
// below" indicator stayed visible.
func TestHelpScrollGReachesActualBottom(t *testing.T) {
	m := newHelpModel()
	r, _ := m.handleHelpKey(keyMsg("G"))
	rm := r.(Model)

	// Render with the post-clamp scroll value and confirm the renderer
	// no longer signals more content below.
	out := ui.RenderHelpScreen(rm.width, rm.height-1, rm.helpScroll, rm.helpFilter.Value, rm.helpSearchQuery, rm.helpContextMode)
	assert.NotContains(t, out, "more below",
		"G must clamp to the renderer's actual max so the bottom indicator disappears")
}

func TestHelpScrollClampsAfterCtrlDPastEnd(t *testing.T) {
	m := newHelpModel()
	// Scroll to (claimed) end first.
	r, _ := m.handleHelpKey(keyMsg("G"))
	rm := r.(Model)
	atEnd := rm.helpScroll
	// Mash ctrl+d a few more times — these presses are off the end and
	// must NOT keep growing helpScroll past the valid range.
	for range 4 {
		r2, _ := rm.handleHelpKey(keyMsg("ctrl+d"))
		rm = r2.(Model)
	}
	assert.Equal(t, atEnd, rm.helpScroll,
		"ctrl+d past the end must be a no-op, not bank up phantom scroll the user has to undo")
}

func TestHelpEscCascadesSearchThenFilterThenClose(t *testing.T) {
	// Esc removes one layer of state at a time so the user doesn't lose
	// their position by accident.
	m := newHelpModel()
	m.helpSearchQuery = "bookmark"
	m.helpFilter.Value = "bookmark"

	// First Esc: clear search.
	r, _ := m.handleHelpKey(keyMsg("esc"))
	rm := r.(Model)
	assert.Empty(t, rm.helpSearchQuery)
	assert.Equal(t, "bookmark", rm.helpFilter.Value)
	assert.Equal(t, modeHelp, rm.mode, "still in help mode after first Esc")

	// Second Esc: clear filter.
	r2, _ := rm.handleHelpKey(keyMsg("esc"))
	rm2 := r2.(Model)
	assert.Empty(t, rm2.helpFilter.Value)
	assert.Equal(t, modeHelp, rm2.mode)

	// Third Esc: close help.
	r3, _ := rm2.handleHelpKey(keyMsg("esc"))
	rm3 := r3.(Model)
	assert.NotEqual(t, modeHelp, rm3.mode, "third Esc closes help")
}
