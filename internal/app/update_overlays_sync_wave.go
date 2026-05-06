package app

import (
	"fmt"
	"math"

	tea "github.com/charmbracelet/bubbletea"
)

// handleSyncWaveOverlayKey routes overlay keys for the Sync Wave Timeline:
// pane-agnostic keys (close, refresh, Tab) first, then dispatches to the
// per-pane handler based on activePane.
func (m Model) handleSyncWaveOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		// Rotate the session token first so any in-flight skeleton/full
		// fetch or auto-refresh tick that lands after we close the
		// overlay is treated as stale (the message handlers compare the
		// inbound token to m.syncWave.token and drop on mismatch).
		// Without this, a late syncWaveTimelineMsg would re-open the
		// overlay via updateSyncWaveTimeline's defensive "open on data"
		// branch, surprising the user.
		m.syncWave.token++
		m.loading = false
		m.overlay = overlayNone
		m.syncWave.data = nil
		m.syncWave.collapsed = nil
		m.syncWave.bodyScroll = 0
		m.syncWave.bodyCursor = syncWaveBodyCursor{}
		m.syncWave.sidebarCursor = 0
		m.syncWave.activePane = paneSidebar
		m.syncWave.loadingFrame = 0
		return m, nil
	case "R":
		m.loading = true
		m.setStatusMessage("Refreshing sync wave timeline…", false)
		return m, m.loadSyncWaveTimeline(m.syncWave.token)
	case "tab", "shift+tab":
		// Single-pane mode: the sidebar is hidden, so toggling focus to
		// it would route subsequent keys (j/k/Enter) to a pane the user
		// can't see. Force focus on the body and treat Tab as a no-op.
		// Threshold mirrors the renderer: outer w = min(160, m.width-8),
		// innerW = max(w-6, 20), and entry.SinglePane = innerW < 50.
		// Solving for m.width gives the < 64 cutoff used here.
		if m.width < 64 {
			m.syncWave.activePane = paneBody
			return m, nil
		}
		if m.syncWave.activePane == paneSidebar {
			m.syncWave.activePane = paneBody
		} else {
			m.syncWave.activePane = paneSidebar
		}
		return m, nil
	}
	if m.syncWave.activePane == paneSidebar {
		return m.handleSyncWaveSidebarKey(msg)
	}
	return m.handleSyncWaveBodyKey(msg)
}

// handleSyncWaveSidebarKey handles keys when the sidebar pane has focus.
// j/k move sidebarCursor (with wraparound) and reset the body cursor +
// scroll for the new phase. g/G jump to the first/last phase. Enter and
// Space toggle collapse on the focused phase.
func (m Model) handleSyncWaveSidebarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.syncWave.data == nil {
		return m, nil
	}
	n := len(m.syncWave.data.Phases)
	if n == 0 {
		return m, nil
	}
	switch msg.String() {
	case "j", "down":
		m.syncWave.sidebarCursor = (m.syncWave.sidebarCursor + 1) % n
		resetBodyForNewPhase(&m.syncWave)
		return m, nil
	case "k", "up":
		m.syncWave.sidebarCursor = (m.syncWave.sidebarCursor - 1 + n) % n
		resetBodyForNewPhase(&m.syncWave)
		return m, nil
	case "g":
		m.syncWave.sidebarCursor = 0
		resetBodyForNewPhase(&m.syncWave)
		return m, nil
	case "G":
		m.syncWave.sidebarCursor = n - 1
		resetBodyForNewPhase(&m.syncWave)
		return m, nil
	case "enter", " ", "space":
		togglePhaseCollapse(&m.syncWave)
		return m, nil
	}
	return m, nil
}

// resetBodyForNewPhase resets bodyCursor and bodyScroll when the
// sidebarCursor moves to a different phase.
func resetBodyForNewPhase(s *syncWaveState) {
	s.bodyCursor = initialBodyCursor(s.data.Phases, s.sidebarCursor, s.collapsed)
	s.bodyScroll = 0
}

// togglePhaseCollapse toggles collapsed[currentPhaseName] and re-anchors
// the body cursor + scroll. After a phase-level toggle the previous
// body cursor row no longer exists in the flattened sequence (an
// expand swaps the placeholder row for wave headers; a collapse swaps
// wave headers/resources for the placeholder), so we reset to a
// known-good position to keep navigation predictable.
func togglePhaseCollapse(s *syncWaveState) {
	if s.data == nil || s.sidebarCursor < 0 || s.sidebarCursor >= len(s.data.Phases) {
		return
	}
	if s.collapsed == nil {
		s.collapsed = map[string]bool{}
	}
	name := s.data.Phases[s.sidebarCursor].Name
	s.collapsed[name] = !s.collapsed[name]
	resetBodyForNewPhase(s)
}

// handleSyncWaveBodyKey handles keys when the body pane has focus. The
// body pane is a flattened sequence of wave headers + resources of
// expanded waves + a single placeholder when the phase is collapsed/empty.
// j/k advance/retreat through the sequence, g/G jump to ends,
// Ctrl+D/U/F/B half/full page scroll, Enter toggles wave or phase
// collapse depending on the cursor row kind.
func (m Model) handleSyncWaveBodyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.syncWave.data == nil || m.syncWave.sidebarCursor < 0 || m.syncWave.sidebarCursor >= len(m.syncWave.data.Phases) {
		return m, nil
	}
	phase := m.syncWave.data.Phases[m.syncWave.sidebarCursor]
	rows := syncWaveFlattenBody(phase, m.syncWave.collapsed)
	if len(rows) == 0 {
		return m, nil
	}
	cur := max(findBodyCursorIdx(rows, m.syncWave.bodyCursor), 0)
	viewport := bodyViewportRows(m)
	switch msg.String() {
	case "j", "down":
		if cur < len(rows)-1 {
			cur++
		}
		m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: rows[cur].waveIdx, resourceIdx: rows[cur].resourceIdx}
		adjustBodyScrollToCursor(&m.syncWave, cur, viewport)
		return m, nil
	case "k", "up":
		if cur > 0 {
			cur--
		}
		m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: rows[cur].waveIdx, resourceIdx: rows[cur].resourceIdx}
		adjustBodyScrollToCursor(&m.syncWave, cur, viewport)
		return m, nil
	case "g":
		m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: rows[0].waveIdx, resourceIdx: rows[0].resourceIdx}
		m.syncWave.bodyScroll = 0
		return m, nil
	case "G":
		last := rows[len(rows)-1]
		m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: last.waveIdx, resourceIdx: last.resourceIdx}
		adjustBodyScrollToCursor(&m.syncWave, len(rows)-1, viewport)
		return m, nil
	case "ctrl+d":
		cur = min(cur+10, len(rows)-1)
		m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: rows[cur].waveIdx, resourceIdx: rows[cur].resourceIdx}
		adjustBodyScrollToCursor(&m.syncWave, cur, viewport)
		return m, nil
	case "ctrl+u":
		cur = max(cur-10, 0)
		m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: rows[cur].waveIdx, resourceIdx: rows[cur].resourceIdx}
		adjustBodyScrollToCursor(&m.syncWave, cur, viewport)
		return m, nil
	case "ctrl+f", "pgdown":
		cur = min(cur+20, len(rows)-1)
		m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: rows[cur].waveIdx, resourceIdx: rows[cur].resourceIdx}
		adjustBodyScrollToCursor(&m.syncWave, cur, viewport)
		return m, nil
	case "ctrl+b", "pgup":
		cur = max(cur-20, 0)
		m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: rows[cur].waveIdx, resourceIdx: rows[cur].resourceIdx}
		adjustBodyScrollToCursor(&m.syncWave, cur, viewport)
		return m, nil
	case "enter", " ", "space":
		toggleBodyCursorCollapse(&m.syncWave, rows[cur])
		return m, nil
	}
	return m, nil
}

// bodyViewportRows estimates the body pane's visible row count based on
// the Model's terminal dimensions. Mirrors renderOverlaySyncWave's
// chrome math: outer h = min(35, m.height-6); inner h = max(h-4, 5);
// then ~5 rows of header (title, last-op, live-phase, loading, divider).
// The result is a conservative lower bound — over-estimating shrinks
// scroll less aggressively than under-estimating, which is fine.
func bodyViewportRows(m Model) int {
	outer := min(35, m.height-6)
	inner := max(outer-4, 5)
	const maxHeaderLines = 5
	return max(inner-maxHeaderLines, 3)
}

// adjustBodyScrollToCursor moves bodyScroll the minimum amount needed to
// keep the cursor row inside the visible window. Cursor above scroll →
// scroll up to cursor. Cursor at-or-below scroll+viewport → scroll down
// so the cursor sits on the last visible row.
func adjustBodyScrollToCursor(s *syncWaveState, cursorIdx, viewport int) {
	if cursorIdx < s.bodyScroll {
		s.bodyScroll = cursorIdx
		return
	}
	if cursorIdx >= s.bodyScroll+viewport {
		s.bodyScroll = cursorIdx - viewport + 1
	}
}

// toggleBodyCursorCollapse handles Enter on the cursor row:
//   - Wave header: toggle wave collapse.
//   - Placeholder: toggle phase collapse.
//   - Resource: no-op.
func toggleBodyCursorCollapse(s *syncWaveState, row syncWaveBodyRow) {
	if s.collapsed == nil {
		s.collapsed = map[string]bool{}
	}
	if s.sidebarCursor < 0 || s.sidebarCursor >= len(s.data.Phases) {
		return
	}
	phase := s.data.Phases[s.sidebarCursor]
	switch row.kind {
	case syncWaveRowWaveHeader:
		wave := phase.Waves[row.waveIdx]
		waveLabel := "wave ?"
		if wave.Wave != math.MinInt {
			waveLabel = fmt.Sprintf("wave %d", wave.Wave)
		}
		key := phase.Name + "/" + waveLabel
		s.collapsed[key] = !s.collapsed[key]
	case syncWaveRowPlaceholder:
		s.collapsed[phase.Name] = !s.collapsed[phase.Name]
		// Phase-level toggle from the body pane: re-anchor the body
		// cursor for the same reasons as togglePhaseCollapse — the
		// flattened row sequence shifts so the previous cursor
		// position is no longer meaningful.
		resetBodyForNewPhase(s)
	}
}
