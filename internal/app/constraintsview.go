package app

import "github.com/janosmiko/lfk/internal/k8s"

// constraintsViewState backs the fullscreen "what constrains this
// object" view (modeConstraints). Mirrors describeViewState/diffViewState.
type constraintsViewState struct {
	report     k8s.ConstraintReport
	loading    bool
	err        error
	cursor     int
	scroll     int
	returnMode viewMode
	title      string
	name       string // the object's own name, so kb.Refresh can re-issue the load
	target     k8s.ConstraintTarget
	// gen guards a load result against a superseded open — bumped on every
	// loadConstraints dispatch, echoed back by constraintsLoadedMsg.
	gen uint64
}

func (s constraintsViewState) visibleRows() []k8s.ConstraintRow {
	return s.report.Rows
}

func (s *constraintsViewState) clampCursor() {
	last := len(s.visibleRows()) - 1
	switch {
	case s.cursor > last:
		s.cursor = last
	case s.cursor < 0:
		s.cursor = 0
	}
}

// clampScroll keeps the window inside a report that shrank under it. A
// scroll past the last row makes the renderer's row count negative.
func (s *constraintsViewState) clampScroll(viewportHeight int) {
	maxScroll := max(len(s.visibleRows())-max(viewportHeight, 1), 0)
	s.scroll = min(max(s.scroll, 0), maxScroll)
}
