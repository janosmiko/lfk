package app

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

// The API Explorer's cancellation scope and its generation counter. Split out
// of update_explain.go to keep that file under the length cap.

// explainSessionState is the cancellation scope shared by every kubectl
// explain of one API Explorer visit. It is set when the Explorer opens and
// cancelled when it closes, so leaving the view stops the subprocesses instead
// of leaving them to their deadlines. Deliberately outside the per-tab
// snapshot: the fetches belong to the process, not to the tab that started
// them.
type explainSessionState struct {
	explainCtx    context.Context
	explainCancel context.CancelFunc

	// explainGen numbers the sessions. Every explain fetch stamps the
	// generation it was started for, and the reply handlers drop anything
	// older, so a fetch answered after the user left the view - or switched
	// to another tab - cannot apply to what is on screen now. Per-tab
	// requestGen cannot do this job: each tab counts on its own, so two tabs
	// share generation numbers.
	explainGen uint64
}

// beginExplainSession starts a fresh cancellation scope for the API Explorer.
// Every explain fetch of this session hangs off it, so closing the view ends
// them all. Any earlier session is cancelled first, which also releases its
// child of m.reqCtx.
func (m *Model) beginExplainSession() {
	m.cancelExplainSession()
	base := m.reqCtx
	if base == nil {
		base = context.Background()
	}
	m.explainCtx, m.explainCancel = context.WithCancel(base)
}

// cancelExplainSession stops the explain fetches still running, if any. The
// generation moves on even when there was no session to cancel, so a reply
// already on its way back is stale from here on. beginExplainSession calls
// this first, which is where a new session gets its number.
func (m *Model) cancelExplainSession() {
	if m.explainCancel != nil {
		m.explainCancel()
	}
	m.explainCtx, m.explainCancel = nil, nil
	m.explainGen++
}

// resumeExplainFetch re-issues the schema load of a tab that comes back with
// the API Explorer open and a fetch still outstanding. Switching away cancels
// the fetch and the generation guard drops its reply, so the level the tab
// was waiting for never arrives on its own. Nil when there is nothing to
// resume.
//
// explainPending, not an empty title, is what marks a fetch as outstanding.
// A fetch can be issued while the previous level's title is still on screen -
// drilling in, going back, or opening the Explorer on another resource all
// start a fetch before clearing or replacing the title - so an empty-title
// check misses a fetch left pending behind an old level. explainPending is
// set at every execKubectlExplain call site and cleared by updateExplainLoaded
// once a reply of the current generation lands, so it tracks the fetch
// itself rather than what happens to be on screen.
func (m *Model) resumeExplainFetch() tea.Cmd {
	if m.mode != modeExplain || !m.explainPending || m.explainResource == "" {
		return nil
	}
	m.loading = true
	return m.execKubectlExplain(m.explainResource, m.explainAPIVersion, m.explainPath)
}

// explainRequestCtx is the parent context for one explain fetch. It falls back
// to m.reqCtx when no session is open, so a fetch is never left unbounded. A
// session already cancelled falls back too: cancelAndReset kills the session as
// a child of the old m.reqCtx, and every later fetch of that visit would fail
// on the spot if it kept using it.
func (m Model) explainRequestCtx() context.Context {
	if m.explainCtx != nil && m.explainCtx.Err() == nil {
		return m.explainCtx
	}
	if m.reqCtx != nil {
		return m.reqCtx
	}
	return context.Background()
}
