package app

import (
	"fmt"
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// syncWaveRefreshInterval is how often the overlay re-fetches while
// the Application's operationState.phase reports "Running".
const syncWaveRefreshInterval = 3 * time.Second

// syncWaveSpinnerInterval is the cadence of the loading-spinner glyph
// rotation. 100ms is the standard cadence for terminal spinners — fast
// enough to feel responsive, slow enough not to burn CPU. Tighter than
// 100ms looks jittery on most terminals; slower looks laggy.
const syncWaveSpinnerInterval = 100 * time.Millisecond

// scheduleSyncWaveSpinnerTick fires the next spinner-rotation message.
// The handler stops scheduling further ticks once data.Loading is
// false, so no goroutines accumulate after the wave-map fetch lands.
func scheduleSyncWaveSpinnerTick(token uint64) tea.Cmd {
	return tea.Tick(syncWaveSpinnerInterval, func(time.Time) tea.Msg {
		return syncWaveSpinnerTickMsg{token: token}
	})
}

// updateSyncWaveTimeline applies a fresh SyncWaveTimeline to Model when
// the message's token still matches the current overlay session. On a
// skeleton message (info.Loading == true) it chains a full fetch so the
// wave annotations land asynchronously. On the full message it schedules
// the auto-refresh tick if the live phase is Running.
func (m Model) updateSyncWaveTimeline(msg syncWaveTimelineMsg) (tea.Model, tea.Cmd) {
	if msg.token != m.syncWave.token {
		return m, nil // stale fetch from a previous overlay session
	}
	if msg.err != nil {
		m.loading = false
		m.setStatusMessage(fmt.Sprintf("Sync wave timeline failed: %v", msg.err), true)
		return m, withSyncWaveAutoRefresh(m, scheduleStatusClear())
	}
	if msg.info == nil {
		m.loading = false
		m.setStatusMessage("Sync wave timeline returned no data", true)
		return m, withSyncWaveAutoRefresh(m, scheduleStatusClear())
	}
	// Keep the loading flag set while we still await the wave map; the
	// skeleton message paints the phase structure but waves are not yet
	// known.
	m.loading = msg.info.Loading

	// Preserve the user-visible bits across refreshes — focused phase,
	// body cursor, active pane, collapse state, scroll position, spinner
	// frame, and the rotating session token. Resetting any of these on
	// the inbound message would make the overlay snap back to its zero
	// state every refresh.
	//
	// First open is detected by prev.collapsed == nil — at that point we
	// install smart defaults (collapse empty fail/delete phases, focus
	// the first non-empty phase, place the body cursor on the first wave
	// header). On a refresh we instead clamp the existing cursors so a
	// shrunk data shape can't leave them dangling past the end.
	prev := m.syncWave
	firstOpen := prev.collapsed == nil

	m.syncWave = syncWaveState{
		data:          msg.info,
		collapsed:     prev.collapsed,
		token:         prev.token,
		lastRefreshAt: time.Now(),
		loadingFrame:  prev.loadingFrame,
		sidebarCursor: prev.sidebarCursor,
		bodyCursor:    prev.bodyCursor,
		bodyScroll:    prev.bodyScroll,
		activePane:    prev.activePane,
	}

	if firstOpen {
		m.syncWave.collapsed = map[string]bool{}
		applySmartDefaults(&m.syncWave)
		m.syncWave.activePane = paneSidebar
		m.syncWave.sidebarCursor = firstNonEmptyPhase(m.syncWave.data.Phases)
		m.syncWave.bodyCursor = initialBodyCursor(m.syncWave.data.Phases, m.syncWave.sidebarCursor, m.syncWave.collapsed)
		m.syncWave.bodyScroll = 0
	} else {
		clampSyncWaveCursors(&m.syncWave)
	}

	// Two-phase load: the skeleton message chains the slow full fetch.
	// The overlay is already open (set by executeActionSyncWaveTimeline
	// or by a previous skeleton/full message in this session) so the
	// renderer paints the partial result with an animated spinner +
	// "Loading wave map…" indicator while the chain runs.
	if msg.info.Loading {
		return m, tea.Batch(
			m.loadSyncWaveTimeline(m.syncWave.token),
			scheduleSyncWaveSpinnerTick(m.syncWave.token),
		)
	}

	// Defensive guard for the rare path where a full message arrives
	// without the action having opened the overlay (e.g. a future caller
	// loads directly via loadSyncWaveTimeline). Mirrors the prior
	// "open on data" behavior.
	m.overlay = overlaySyncWave

	if m.syncWave.data.LivePhase == "Running" {
		token := m.syncWave.token
		return m, tea.Tick(syncWaveRefreshInterval, func(time.Time) tea.Msg {
			return syncWaveTickMsg{token: token}
		})
	}
	return m, nil
}

// withSyncWaveAutoRefresh batches the next auto-refresh tick onto the
// supplied cmd when the previous timeline reported a Running phase.
// Used by the error / nil-info branches so a transient fetch failure
// (apiserver hiccup, network blip, RBAC race) does NOT silently kill
// the auto-refresh loop while the operation is still in flight — the
// next tick fires syncWaveTickMsg{token: m.syncWave.token}, and the
// tick handler keeps issuing fetches until the live phase leaves
// "Running" or the user closes the overlay.
func withSyncWaveAutoRefresh(m Model, base tea.Cmd) tea.Cmd {
	if m.syncWave.data == nil || m.syncWave.data.LivePhase != "Running" {
		return base
	}
	token := m.syncWave.token
	tick := tea.Tick(syncWaveRefreshInterval, func(time.Time) tea.Msg {
		return syncWaveTickMsg{token: token}
	})
	return tea.Batch(base, tick)
}

// applySmartDefaults sets the default collapsed state on first open:
// empty fail/delete phases collapse so they don't visually clutter the
// sidebar; non-empty phases stay expanded.
func applySmartDefaults(s *syncWaveState) {
	if s.data == nil {
		return
	}
	for _, phase := range s.data.Phases {
		if len(phase.Waves) == 0 {
			switch phase.Name {
			case "PostSync", "SyncFail", "PostSyncFail", "PreDelete", "PostDelete":
				s.collapsed[phase.Name] = true
			}
		}
	}
}

// firstNonEmptyPhase returns the index of the first phase with at least
// one wave. Falls back to 0 if all phases are empty.
func firstNonEmptyPhase(phases []k8s.SyncWavePhase) int {
	for i, p := range phases {
		if len(p.Waves) > 0 {
			return i
		}
	}
	return 0
}

// initialBodyCursor places the cursor on the first wave header of the
// selected phase. Returns {-1, -1} when the phase has no waves (placeholder).
func initialBodyCursor(phases []k8s.SyncWavePhase, sidebarIdx int, collapsed map[string]bool) syncWaveBodyCursor {
	if sidebarIdx < 0 || sidebarIdx >= len(phases) {
		return syncWaveBodyCursor{waveIdx: -1, resourceIdx: -1}
	}
	phase := phases[sidebarIdx]
	if collapsed[phase.Name] || len(phase.Waves) == 0 {
		return syncWaveBodyCursor{waveIdx: -1, resourceIdx: -1}
	}
	return syncWaveBodyCursor{waveIdx: 0, resourceIdx: -1}
}

// clampSyncWaveCursors fixes cursor positions when the data shape
// shrank (e.g., a phase lost waves between refreshes). Also clamps
// bodyScroll against the new flattened-row count so a refresh that
// removes rows can't leave bodyScroll pointing past the end (the body
// renderer would otherwise paint nothing).
func clampSyncWaveCursors(s *syncWaveState) {
	if s.data == nil {
		return
	}
	if n := len(s.data.Phases); n > 0 {
		if s.sidebarCursor >= n {
			s.sidebarCursor = n - 1
		}
	}
	if s.sidebarCursor < 0 || s.sidebarCursor >= len(s.data.Phases) {
		s.bodyCursor = syncWaveBodyCursor{waveIdx: -1, resourceIdx: -1}
		s.bodyScroll = 0
		return
	}
	phase := s.data.Phases[s.sidebarCursor]
	clampBodyCursorForPhase(s, phase)
	clampBodyScrollForPhase(s, phase)
}

// clampBodyCursorForPhase clamps bodyCursor to a valid row in the
// flattened sequence for the focused phase. Extracted to keep
// clampSyncWaveCursors short and to make the bodyScroll clamp easier
// to read.
func clampBodyCursorForPhase(s *syncWaveState, phase k8s.SyncWavePhase) {
	if s.collapsed[phase.Name] || len(phase.Waves) == 0 {
		s.bodyCursor = syncWaveBodyCursor{waveIdx: -1, resourceIdx: -1}
		return
	}
	if s.bodyCursor.waveIdx < 0 || s.bodyCursor.waveIdx >= len(phase.Waves) {
		s.bodyCursor = syncWaveBodyCursor{waveIdx: 0, resourceIdx: -1}
		return
	}
	wave := phase.Waves[s.bodyCursor.waveIdx]
	waveLabel := "wave ?"
	if wave.Wave != math.MinInt {
		waveLabel = fmt.Sprintf("wave %d", wave.Wave)
	}
	if s.collapsed[phase.Name+"/"+waveLabel] {
		s.bodyCursor.resourceIdx = -1
		return
	}
	if s.bodyCursor.resourceIdx >= len(wave.Resources) {
		if len(wave.Resources) > 0 {
			s.bodyCursor.resourceIdx = len(wave.Resources) - 1
		} else {
			s.bodyCursor.resourceIdx = -1
		}
	}
}

// clampBodyScrollForPhase pins bodyScroll to a valid row index for the
// focused phase. Without this, a refresh that shrinks the data while
// the user is scrolled deep would leave bodyScroll past the end and
// the body renderer would paint nothing.
func clampBodyScrollForPhase(s *syncWaveState, phase k8s.SyncWavePhase) {
	rows := syncWaveFlattenBody(phase, s.collapsed)
	maxScroll := max(0, len(rows)-1)
	if s.bodyScroll > maxScroll {
		s.bodyScroll = maxScroll
	}
}

// handleSyncWaveTick checks that the tick belongs to the current overlay
// session, that the overlay is still open, and that the phase is still
// Running. On any miss, returns no cmd. On match, issues the next
// loadSyncWaveTimeline.
func (m Model) handleSyncWaveTick(msg syncWaveTickMsg) (tea.Model, tea.Cmd) { //nolint:unparam // consistent message handler signature; tea.Model return is consumed by the dispatch wired in Task 13.
	if msg.token != m.syncWave.token {
		return m, nil
	}
	if m.overlay != overlaySyncWave {
		return m, nil
	}
	if m.syncWave.data == nil || m.syncWave.data.LivePhase != "Running" {
		return m, nil
	}
	return m, m.loadSyncWaveTimeline(msg.token)
}

// handleSyncWaveSpinnerTick advances the spinner glyph index by 1 and
// schedules the next tick — but only while the overlay is still
// loading. Token mismatch (stale session), closed overlay, or
// data.Loading == false all cause the tick chain to stop cleanly. The
// modulo wrap on the frame index lives in the renderer, so the
// counter can roll forever without overflowing in any practical session.
func (m Model) handleSyncWaveSpinnerTick(msg syncWaveSpinnerTickMsg) (tea.Model, tea.Cmd) { //nolint:unparam // tea.Model return is consumed by the central dispatch in update.go.
	if msg.token != m.syncWave.token {
		return m, nil
	}
	if m.overlay != overlaySyncWave {
		return m, nil
	}
	if m.syncWave.data == nil || !m.syncWave.data.Loading {
		return m, nil
	}
	m.syncWave.loadingFrame++
	return m, scheduleSyncWaveSpinnerTick(m.syncWave.token)
}

// renderOverlaySyncWave converts the k8s.SyncWaveTimeline into a
// presentation-only ui.SyncWaveTimelineEntry and delegates to the renderer.
// Width/height match the budget used by other fullscreen overlays
// (Crash Investigator). The renderer takes INNER dimensions: OverlayStyle
// adds 6 cols of horizontal chrome (2 border + 4 padding) and 4 rows of
// vertical chrome (2 border + 2 padding) on top of (w, h), so the
// renderer is sized to (w-6, h-4) and emits exactly h-4 lines, each at
// most w-6 cells wide. This pins the outer overlay to a fixed size so it
// doesn't visibly shrink as the user scrolls the body.
func (m Model) renderOverlaySyncWave() (string, int, int) {
	w, h := min(160, m.width-8), min(35, m.height-6)
	innerW, innerH := max(w-6, 20), max(h-4, 5)

	if m.syncWave.data == nil {
		// Two-phase load: the overlay opens immediately, before the
		// skeleton message arrives (~100ms). A non-empty placeholder
		// keeps the framed box visible while we wait so the operator
		// gets instant visual feedback that their action took effect.
		return ui.Truncate("Loading sync wave timeline…", innerW), w, h
	}
	d := m.syncWave.data

	pane := ui.SyncWavePaneSidebar
	if m.syncWave.activePane == paneBody {
		pane = ui.SyncWavePaneBody
	}

	entry := ui.SyncWaveTimelineEntry{
		AppName:       d.AppName,
		AppNamespace:  d.AppNamespace,
		LivePhase:     d.LivePhase,
		Revision:      d.Revision,
		Loading:       d.Loading,
		LoadingFrame:  m.syncWave.loadingFrame,
		SidebarCursor: m.syncWave.sidebarCursor,
		BodyCursor: ui.SyncWaveBodyCursor{
			WaveIdx:     m.syncWave.bodyCursor.waveIdx,
			ResourceIdx: m.syncWave.bodyCursor.resourceIdx,
		},
		BodyScroll: m.syncWave.bodyScroll,
		ActivePane: pane,
		SinglePane: innerW < 50,
		Collapsed:  m.syncWave.collapsed,
	}
	if d.LastOperation != nil {
		entry.LastOperation = &ui.SyncWaveLastOperation{
			Phase:      d.LastOperation.Phase,
			Message:    d.LastOperation.Message,
			StartedAt:  d.LastOperation.StartedAt,
			FinishedAt: d.LastOperation.FinishedAt,
			Revision:   d.LastOperation.Revision,
		}
	}
	for _, p := range d.Phases {
		pe := ui.SyncWavePhaseEntry{
			Name:      p.Name,
			Collapsed: m.syncWave.collapsed[p.Name],
		}
		for _, wave := range p.Waves {
			// k8s.unknownWave and ui.SyncWaveUnknownWave are both
			// math.MinInt — direct copy.
			be := ui.SyncWaveBucketEntry{Wave: wave.Wave}
			for _, r := range wave.Resources {
				be.Resources = append(be.Resources, syncWaveResourceToEntry(r))
			}
			pe.Waves = append(pe.Waves, be)
		}
		entry.Phases = append(entry.Phases, pe)
	}
	return ui.RenderSyncWaveTimeline(entry, innerW, innerH), w, h
}

func syncWaveResourceToEntry(r k8s.SyncWaveResource) ui.SyncWaveResourceEntry {
	return ui.SyncWaveResourceEntry{
		Group:        r.Group,
		Kind:         r.Kind,
		Namespace:    r.Namespace,
		Name:         r.Name,
		SyncStatus:   r.SyncStatus,
		HealthStatus: r.HealthStatus,
		HookPhase:    r.HookPhase,
		OpStatus:     r.OpStatus,
		Message:      r.Message,
		IsHook:       r.IsHook,
	}
}

// syncWaveBodyRow mirrors ui.bodyRow but for app-side k8s types.
type syncWaveBodyRow struct {
	kind        int
	waveIdx     int
	resourceIdx int
}

const (
	syncWaveRowWaveHeader  = 0
	syncWaveRowResource    = 1
	syncWaveRowPlaceholder = 2
)

// syncWaveFlattenBody returns the visible row sequence for a phase given
// the collapsed map. Mirrors ui.flattenBodyRows but works on k8s types.
func syncWaveFlattenBody(phase k8s.SyncWavePhase, collapsed map[string]bool) []syncWaveBodyRow {
	if collapsed[phase.Name] || len(phase.Waves) == 0 {
		return []syncWaveBodyRow{{kind: syncWaveRowPlaceholder, waveIdx: -1, resourceIdx: -1}}
	}
	out := make([]syncWaveBodyRow, 0)
	for wi, wave := range phase.Waves {
		out = append(out, syncWaveBodyRow{kind: syncWaveRowWaveHeader, waveIdx: wi, resourceIdx: -1})
		waveLabel := "wave ?"
		if wave.Wave != math.MinInt {
			waveLabel = fmt.Sprintf("wave %d", wave.Wave)
		}
		if collapsed[phase.Name+"/"+waveLabel] {
			continue
		}
		for ri := range wave.Resources {
			out = append(out, syncWaveBodyRow{kind: syncWaveRowResource, waveIdx: wi, resourceIdx: ri})
		}
	}
	return out
}

// findBodyCursorIdx returns the index of the cursor within the flattened
// body rows. -1 if not found.
func findBodyCursorIdx(rows []syncWaveBodyRow, c syncWaveBodyCursor) int {
	for i, r := range rows {
		if r.waveIdx == c.waveIdx && r.resourceIdx == c.resourceIdx {
			return i
		}
	}
	return -1
}
