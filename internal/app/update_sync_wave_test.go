package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/k8s"
)

// errString is a sentinel error type used by these tests so they don't need
// to import errors.New from another package.
type errString string

func (e errString) Error() string { return string(e) }

var assertAnError = errString("boom")

func TestUpdateSyncWaveTimeline_StaleTokenDropped(t *testing.T) {
	m := Model{}
	m.syncWave.token = 5

	tl := &k8s.SyncWaveTimeline{AppName: "old"}
	got, cmd := m.updateSyncWaveTimeline(syncWaveTimelineMsg{info: tl, token: 4})

	gotM := got.(Model)
	assert.Nil(t, cmd)
	assert.Nil(t, gotM.syncWave.data, "stale token must not overwrite state")
}

func TestUpdateSyncWaveTimeline_FreshTokenAccepted(t *testing.T) {
	m := Model{}
	m.syncWave.token = 7
	m.overlay = overlayNone // handler must open the overlay on first data
	m.loading = true        // must be cleared by the handler

	tl := &k8s.SyncWaveTimeline{AppName: "live", LivePhase: "Succeeded"} // Loading: false → full-path message
	got, _ := m.updateSyncWaveTimeline(syncWaveTimelineMsg{info: tl, token: 7})

	gotM := got.(Model)
	require.NotNil(t, gotM.syncWave.data)
	assert.Equal(t, "live", gotM.syncWave.data.AppName)
	assert.False(t, gotM.loading)
	assert.Equal(t, overlaySyncWave, gotM.overlay, "handler must open the overlay once data lands")
}

// Two-phase load: the skeleton message keeps loading=true and chains a
// full fetch, but does NOT schedule the auto-refresh tick yet. The
// overlay must remain open (it was opened by executeActionSyncWaveTimeline
// before the load was kicked off) so the user sees the partial result.
func TestUpdateSyncWaveTimeline_SkeletonChainsToFull(t *testing.T) {
	m := Model{}
	m.syncWave.token = 9
	m.overlay = overlaySyncWave
	m.loading = true
	m.bgtasks = bgtasks.New(bgtasks.DefaultThreshold) // loadSyncWaveTimeline calls bgtasks.Start

	tl := &k8s.SyncWaveTimeline{AppName: "skel", LivePhase: "Running", Loading: true}
	got, cmd := m.updateSyncWaveTimeline(syncWaveTimelineMsg{info: tl, token: 9})

	gotM := got.(Model)
	require.NotNil(t, gotM.syncWave.data)
	assert.True(t, gotM.syncWave.data.Loading, "skeleton data must keep Loading: true")
	assert.True(t, gotM.loading, "model.loading must remain true while skeleton awaits full fetch")
	assert.Equal(t, overlaySyncWave, gotM.overlay)
	require.NotNil(t, cmd, "skeleton message must chain to the full loader")
}

func TestUpdateSyncWaveTimeline_RunningSchedulesTick(t *testing.T) {
	m := Model{}
	m.syncWave.token = 1
	m.overlay = overlaySyncWave

	tl := &k8s.SyncWaveTimeline{AppName: "x", LivePhase: "Running"}
	_, cmd := m.updateSyncWaveTimeline(syncWaveTimelineMsg{info: tl, token: 1})
	require.NotNil(t, cmd, "Running phase must schedule a tick")
}

func TestUpdateSyncWaveTimeline_NotRunningNoTick(t *testing.T) {
	m := Model{}
	m.syncWave.token = 1
	m.overlay = overlaySyncWave

	tl := &k8s.SyncWaveTimeline{AppName: "x", LivePhase: "Succeeded"}
	_, cmd := m.updateSyncWaveTimeline(syncWaveTimelineMsg{info: tl, token: 1})
	assert.Nil(t, cmd, "non-Running phase must not schedule a tick")
}

func TestUpdateSyncWaveTimeline_ErrorSetsStatus(t *testing.T) {
	m := Model{}
	m.syncWave.token = 1
	m.overlay = overlaySyncWave
	got, cmd := m.updateSyncWaveTimeline(syncWaveTimelineMsg{err: assertAnError, token: 1})
	gotM := got.(Model)
	assert.False(t, gotM.loading)
	assert.True(t, gotM.statusMessageErr, "error message must mark status as error")
	require.NotNil(t, cmd, "must return scheduleStatusClear() cmd to clear the error message")
}

func TestUpdateSyncWaveTimeline_NilInfoSetsStatus(t *testing.T) {
	m := Model{}
	m.syncWave.token = 1
	m.overlay = overlaySyncWave
	got, cmd := m.updateSyncWaveTimeline(syncWaveTimelineMsg{info: nil, err: nil, token: 1})
	gotM := got.(Model)
	assert.False(t, gotM.loading)
	assert.True(t, gotM.statusMessageErr, "nil info must mark status as error")
	require.NotNil(t, cmd, "must return scheduleStatusClear() cmd to clear the error message")
}

// A transient fetch error during a Running sync must not silently kill
// the auto-refresh loop. The handler should batch the next tick onto
// the status-clear cmd so the loop keeps polling until the live phase
// leaves Running or the user closes the overlay.
func TestUpdateSyncWaveTimeline_ErrorDuringRunningKeepsTickAlive(t *testing.T) {
	m := Model{}
	m.syncWave.token = 1
	m.overlay = overlaySyncWave
	m.syncWave.data = &k8s.SyncWaveTimeline{LivePhase: "Running"}
	got, cmd := m.updateSyncWaveTimeline(syncWaveTimelineMsg{err: assertAnError, token: 1})
	gotM := got.(Model)
	assert.False(t, gotM.loading)
	assert.True(t, gotM.statusMessageErr)
	require.NotNil(t, cmd,
		"error during Running phase must batch the auto-refresh tick onto the status-clear cmd")
}

// Same guarantee as the error path: nil-info during a Running sync must
// keep the auto-refresh loop alive.
func TestUpdateSyncWaveTimeline_NilInfoDuringRunningKeepsTickAlive(t *testing.T) {
	m := Model{}
	m.syncWave.token = 1
	m.overlay = overlaySyncWave
	m.syncWave.data = &k8s.SyncWaveTimeline{LivePhase: "Running"}
	got, cmd := m.updateSyncWaveTimeline(syncWaveTimelineMsg{info: nil, token: 1})
	gotM := got.(Model)
	assert.False(t, gotM.loading)
	assert.True(t, gotM.statusMessageErr)
	require.NotNil(t, cmd,
		"nil info during Running phase must batch the auto-refresh tick onto the status-clear cmd")
}

// clampSyncWaveCursors must pin bodyScroll to a valid row index when
// a refresh shrinks the data while the user is scrolled deep. Without
// this, the body renderer would paint nothing because bodyScroll is
// past the end of the flattened row sequence.
func TestClampSyncWaveCursors_BodyScrollClampsAfterShrink(t *testing.T) {
	s := &syncWaveState{
		data: &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{
			{Name: "Sync", Waves: []k8s.SyncWaveBucket{
				{Wave: 0, Resources: []k8s.SyncWaveResource{{Kind: "Pod", Name: "a"}}},
			}},
		}},
		sidebarCursor: 0,
		bodyCursor:    syncWaveBodyCursor{waveIdx: 0, resourceIdx: 0},
		bodyScroll:    50, // way past the end of the flattened sequence
		collapsed:     map[string]bool{},
	}
	clampSyncWaveCursors(s)
	// Flattened rows: [wave header, resource a] → 2 rows → max scroll 1.
	assert.LessOrEqual(t, s.bodyScroll, 1,
		"bodyScroll must clamp to within the flattened row count after shrink")
}

func TestSyncWaveTickValidation(t *testing.T) {
	m := Model{}
	m.syncWave.token = 5
	m.overlay = overlaySyncWave
	m.syncWave.data = &k8s.SyncWaveTimeline{LivePhase: "Running"}

	// Stale tick.
	_, cmd := m.handleSyncWaveTick(syncWaveTickMsg{token: 4})
	assert.Nil(t, cmd)

	// Closed overlay.
	closed := m
	closed.overlay = overlayNone
	_, cmd = closed.handleSyncWaveTick(syncWaveTickMsg{token: 5})
	assert.Nil(t, cmd)

	// Phase no longer Running.
	stopped := m
	stopped.syncWave.data = &k8s.SyncWaveTimeline{LivePhase: "Succeeded"}
	_, cmd = stopped.handleSyncWaveTick(syncWaveTickMsg{token: 5})
	assert.Nil(t, cmd)
}

func TestSyncWaveTickValidation_FreshTickIssuesLoad(t *testing.T) {
	m := Model{}
	m.syncWave.token = 5
	m.overlay = overlaySyncWave
	m.syncWave.data = &k8s.SyncWaveTimeline{LivePhase: "Running"}
	// loadSyncWaveTimeline issues bgtasks.Start synchronously, so the
	// model needs a Registry. We still can't drive the returned closure
	// without a real client — only confirm a cmd was returned.
	m.bgtasks = bgtasks.New(bgtasks.DefaultThreshold)

	_, cmd := m.handleSyncWaveTick(syncWaveTickMsg{token: 5})
	require.NotNil(t, cmd)
}

// Spinner tick advances the loading frame and schedules another tick
// while the data is still in the Loading state. The next tick keeps
// the spinner glyph rotating in the header.
func TestHandleSyncWaveSpinnerTick_AdvancesFrame(t *testing.T) {
	m := Model{}
	m.syncWave.token = 9
	m.overlay = overlaySyncWave
	m.syncWave.data = &k8s.SyncWaveTimeline{Loading: true}
	m.syncWave.loadingFrame = 4

	got, cmd := m.handleSyncWaveSpinnerTick(syncWaveSpinnerTickMsg{token: 9})
	gotM := got.(Model)
	assert.Equal(t, 5, gotM.syncWave.loadingFrame, "spinner frame must advance by 1")
	require.NotNil(t, cmd, "spinner tick must schedule the next tick while loading")
}

// Spinner tick must NOT schedule another tick once the wave-map fetch
// completes (data.Loading flips to false). Without this guard, the
// tick chain would keep ticking forever after every overlay open.
func TestHandleSyncWaveSpinnerTick_StopsWhenLoadingFalse(t *testing.T) {
	m := Model{}
	m.syncWave.token = 9
	m.overlay = overlaySyncWave
	m.syncWave.data = &k8s.SyncWaveTimeline{Loading: false}
	m.syncWave.loadingFrame = 4

	got, cmd := m.handleSyncWaveSpinnerTick(syncWaveSpinnerTickMsg{token: 9})
	gotM := got.(Model)
	assert.Equal(t, 4, gotM.syncWave.loadingFrame,
		"frame must not advance when loading is already false")
	assert.Nil(t, cmd, "spinner tick must stop scheduling once loading is false")
}

// Stale tokens (left over from a closed-and-reopened overlay) must be
// dropped cleanly so they don't keep ticking forever.
func TestHandleSyncWaveSpinnerTick_StaleTokenDropped(t *testing.T) {
	m := Model{}
	m.syncWave.token = 9
	m.overlay = overlaySyncWave
	m.syncWave.data = &k8s.SyncWaveTimeline{Loading: true}

	got, cmd := m.handleSyncWaveSpinnerTick(syncWaveSpinnerTickMsg{token: 8})
	gotM := got.(Model)
	assert.Equal(t, 0, gotM.syncWave.loadingFrame, "stale token must not advance the frame")
	assert.Nil(t, cmd, "stale token must not schedule another tick")
}

// Once the user closes the overlay, queued spinner ticks must stop.
func TestHandleSyncWaveSpinnerTick_OverlayClosedDropped(t *testing.T) {
	m := Model{}
	m.syncWave.token = 9
	m.overlay = overlayNone
	m.syncWave.data = &k8s.SyncWaveTimeline{Loading: true}

	_, cmd := m.handleSyncWaveSpinnerTick(syncWaveSpinnerTickMsg{token: 9})
	assert.Nil(t, cmd, "closed overlay must drop the spinner tick chain")
}

func TestSyncWaveOverlayKey_EscClosesAndClears(t *testing.T) {
	m := Model{}
	m.overlay = overlaySyncWave
	m.syncWave.token = 7
	m.syncWave.data = &k8s.SyncWaveTimeline{AppName: "x"}
	m.loading = true

	got, cmd := m.handleSyncWaveOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	gotM := got.(Model)
	assert.Equal(t, overlayNone, gotM.overlay)
	assert.Nil(t, gotM.syncWave.data)
	assert.Equal(t, uint64(8), gotM.syncWave.token,
		"token must rotate on close so any in-flight fetch is invalidated")
	assert.False(t, gotM.loading,
		"loading flag must clear on close so the spinner stops")
	assert.Nil(t, cmd)
}

func TestSyncWaveOverlayKey_TabTogglesPane(t *testing.T) {
	m := Model{}
	m.width = 100 // multi-pane terminal — Tab must flip focus
	m.overlay = overlaySyncWave
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{{Name: "Sync"}}}
	m.syncWave.activePane = paneSidebar
	got, _ := m.handleSyncWaveOverlayKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, paneBody, got.(Model).syncWave.activePane)
	got, _ = got.(Model).handleSyncWaveOverlayKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, paneSidebar, got.(Model).syncWave.activePane)
}

// TestSyncWaveOverlayKey_TabNoOpInSinglePaneMode asserts that on narrow
// terminals (where the sidebar is hidden by the renderer), Tab forces
// focus on the body instead of toggling. Without this, subsequent
// j/k/Enter keys route to a sidebar the user can't see.
func TestSyncWaveOverlayKey_TabNoOpInSinglePaneMode(t *testing.T) {
	m := Model{}
	m.width = 50 // below the 64 single-pane threshold
	m.overlay = overlaySyncWave
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{{Name: "Sync"}}}
	m.syncWave.activePane = paneSidebar
	got, _ := m.handleSyncWaveOverlayKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, paneBody, got.(Model).syncWave.activePane,
		"Tab in single-pane mode must force focus on the body")
	// Pressing Tab again still keeps focus on the body (no toggle).
	got, _ = got.(Model).handleSyncWaveOverlayKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, paneBody, got.(Model).syncWave.activePane,
		"Tab in single-pane mode must remain a no-op on second press")
}

func TestSyncWaveOverlayKey_TabPreservesCursors(t *testing.T) {
	m := Model{}
	m.width = 100 // multi-pane terminal so Tab actually flips focus
	m.overlay = overlaySyncWave
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{{Name: "Sync"}}}
	m.syncWave.sidebarCursor = 0
	m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: 0, resourceIdx: 2}
	got, _ := m.handleSyncWaveOverlayKey(tea.KeyMsg{Type: tea.KeyTab})
	gotM := got.(Model)
	assert.Equal(t, 0, gotM.syncWave.sidebarCursor)
	assert.Equal(t, syncWaveBodyCursor{waveIdx: 0, resourceIdx: 2}, gotM.syncWave.bodyCursor)
}

func TestSyncWaveOverlayKey_RTriggersRefresh(t *testing.T) {
	m := Model{}
	m.overlay = overlaySyncWave
	m.syncWave.data = &k8s.SyncWaveTimeline{}
	got, cmd := m.handleSyncWaveOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	gotM := got.(Model)
	assert.True(t, gotM.loading)
	require.NotNil(t, cmd)
}

func TestSyncWaveOverlayKey_RefreshKeyWorksFromBodyPane(t *testing.T) {
	m := Model{}
	m.overlay = overlaySyncWave
	m.syncWave.data = &k8s.SyncWaveTimeline{}
	m.syncWave.activePane = paneBody
	_, cmd := m.handleSyncWaveOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	require.NotNil(t, cmd)
}

func TestSyncWaveOverlayKey_EscClosesFromBodyPane(t *testing.T) {
	m := Model{}
	m.overlay = overlaySyncWave
	m.syncWave.data = &k8s.SyncWaveTimeline{}
	m.syncWave.activePane = paneBody
	got, _ := m.handleSyncWaveOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, overlayNone, got.(Model).overlay)
}

func TestUpdateSyncWaveTimeline_SmartDefaultsOnFirstOpen(t *testing.T) {
	m := Model{}
	m.syncWave.token = 1
	m.overlay = overlaySyncWave
	tl := &k8s.SyncWaveTimeline{
		AppName: "x",
		Phases: []k8s.SyncWavePhase{
			{Name: "PreSync"},
			{Name: "Sync", Waves: []k8s.SyncWaveBucket{
				{Wave: 0, Resources: []k8s.SyncWaveResource{{Kind: "Pod", Name: "a"}}},
			}},
			{Name: "PostSync"},
			{Name: "SyncFail"},
			{Name: "PostSyncFail"},
			{Name: "PreDelete"},
			{Name: "PostDelete"},
		},
	}
	got, _ := m.updateSyncWaveTimeline(syncWaveTimelineMsg{info: tl, token: 1})
	gotM := got.(Model)
	assert.Equal(t, 1, gotM.syncWave.sidebarCursor)
	assert.Equal(t, 0, gotM.syncWave.bodyCursor.waveIdx)
	assert.Equal(t, -1, gotM.syncWave.bodyCursor.resourceIdx)
	assert.Equal(t, paneSidebar, gotM.syncWave.activePane)
	assert.True(t, gotM.syncWave.collapsed["PostSync"])
	assert.True(t, gotM.syncWave.collapsed["SyncFail"])
	assert.True(t, gotM.syncWave.collapsed["PostSyncFail"])
	assert.True(t, gotM.syncWave.collapsed["PreDelete"])
	assert.True(t, gotM.syncWave.collapsed["PostDelete"])
	assert.False(t, gotM.syncWave.collapsed["Sync"])
}

func TestUpdateSyncWaveTimeline_AllEmptyPhasesSidebarCursorAtZero(t *testing.T) {
	m := Model{}
	m.syncWave.token = 1
	m.overlay = overlaySyncWave
	tl := &k8s.SyncWaveTimeline{
		AppName: "x",
		Phases: []k8s.SyncWavePhase{
			{Name: "PreSync"}, {Name: "Sync"}, {Name: "PostSync"},
		},
	}
	got, _ := m.updateSyncWaveTimeline(syncWaveTimelineMsg{info: tl, token: 1})
	gotM := got.(Model)
	assert.Equal(t, 0, gotM.syncWave.sidebarCursor)
}

func TestUpdateSyncWaveTimeline_RefreshPreservesCursor(t *testing.T) {
	m := Model{}
	m.syncWave.token = 1
	m.overlay = overlaySyncWave
	m.syncWave.sidebarCursor = 1
	m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: 0, resourceIdx: 1}
	// bodyScroll = 2 sits within the flattened row count (1 wave header
	// + 5 resources = 6 rows; max scroll 5). Avoids the clamp baked
	// into clampSyncWaveCursors interfering with the preserve-cursor
	// assertion below.
	m.syncWave.bodyScroll = 2
	m.syncWave.activePane = paneBody
	m.syncWave.collapsed = map[string]bool{"Sync": false}

	tl := &k8s.SyncWaveTimeline{
		AppName: "x",
		Phases: []k8s.SyncWavePhase{
			{Name: "PreSync"},
			{Name: "Sync", Waves: []k8s.SyncWaveBucket{
				{Wave: 0, Resources: []k8s.SyncWaveResource{
					{Kind: "Pod", Name: "a"},
					{Kind: "Pod", Name: "b"},
					{Kind: "Pod", Name: "c"},
					{Kind: "Pod", Name: "d"},
					{Kind: "Pod", Name: "e"},
				}},
			}},
		},
	}
	got, _ := m.updateSyncWaveTimeline(syncWaveTimelineMsg{info: tl, token: 1})
	gotM := got.(Model)
	assert.Equal(t, 1, gotM.syncWave.sidebarCursor)
	assert.Equal(t, syncWaveBodyCursor{waveIdx: 0, resourceIdx: 1}, gotM.syncWave.bodyCursor)
	assert.Equal(t, 2, gotM.syncWave.bodyScroll)
	assert.Equal(t, paneBody, gotM.syncWave.activePane)
}

func TestExecuteActionSyncWaveTimeline_RotatesTokenAndClearsData(t *testing.T) {
	m := Model{}
	m.syncWave.token = 3
	m.syncWave.data = &k8s.SyncWaveTimeline{AppName: "stale"}
	m.bgtasks = bgtasks.New(bgtasks.DefaultThreshold) // loadSyncWaveTimelineSkeleton calls bgtasks.Start

	got, cmd := m.executeActionSyncWaveTimeline()
	gotM := got.(Model)
	assert.Equal(t, uint64(4), gotM.syncWave.token)
	assert.Nil(t, gotM.syncWave.data, "previous data must be cleared")
	assert.True(t, gotM.loading)
	// Two-phase load: the overlay must open immediately (before any
	// fetch completes) so the user gets visual feedback within ~100ms
	// instead of waiting up to 30s for the wave annotations.
	assert.Equal(t, overlaySyncWave, gotM.overlay,
		"action must open the overlay frame immediately")
	require.NotNil(t, cmd, "action must kick off the skeleton fetch")
}

func TestSidebarKey_JMovesCursorAndResetsBody(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneSidebar
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{
		{Name: "PreSync"},
		{Name: "Sync", Waves: []k8s.SyncWaveBucket{{Wave: 0, Resources: []k8s.SyncWaveResource{{Kind: "Pod", Name: "p"}}}}},
	}}
	m.syncWave.sidebarCursor = 0
	m.syncWave.bodyScroll = 7
	m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: 0, resourceIdx: 3}
	got, _ := m.handleSyncWaveSidebarKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	gotM := got.(Model)
	assert.Equal(t, 1, gotM.syncWave.sidebarCursor)
	assert.Equal(t, 0, gotM.syncWave.bodyScroll)
	assert.Equal(t, syncWaveBodyCursor{waveIdx: 0, resourceIdx: -1}, gotM.syncWave.bodyCursor)
}

func TestSidebarKey_JWrapsAtEnd(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneSidebar
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{{Name: "A"}, {Name: "B"}}}
	m.syncWave.sidebarCursor = 1
	got, _ := m.handleSyncWaveSidebarKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 0, got.(Model).syncWave.sidebarCursor)
}

func TestSidebarKey_KMovesUp(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneSidebar
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{{Name: "A"}, {Name: "B"}}}
	m.syncWave.sidebarCursor = 1
	got, _ := m.handleSyncWaveSidebarKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, got.(Model).syncWave.sidebarCursor)
}

func TestSidebarKey_KWrapsAtTop(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneSidebar
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{{Name: "A"}, {Name: "B"}}}
	m.syncWave.sidebarCursor = 0
	got, _ := m.handleSyncWaveSidebarKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 1, got.(Model).syncWave.sidebarCursor)
}

func TestSidebarKey_EnterTogglesPhaseCollapse(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneSidebar
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{{Name: "Sync"}}}
	m.syncWave.collapsed = map[string]bool{}
	got, _ := m.handleSyncWaveSidebarKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, got.(Model).syncWave.collapsed["Sync"])
	got, _ = got.(Model).handleSyncWaveSidebarKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, got.(Model).syncWave.collapsed["Sync"])
}

func TestSidebarKey_GJumpsToFirstPhase(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneSidebar
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{{Name: "A"}, {Name: "B"}, {Name: "C"}}}
	m.syncWave.sidebarCursor = 2
	got, _ := m.handleSyncWaveSidebarKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	assert.Equal(t, 0, got.(Model).syncWave.sidebarCursor)
}

func TestSidebarKey_BigGJumpsToLastPhase(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneSidebar
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{{Name: "A"}, {Name: "B"}, {Name: "C"}}}
	got, _ := m.handleSyncWaveSidebarKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	assert.Equal(t, 2, got.(Model).syncWave.sidebarCursor)
}

func TestBodyKey_JMovesThroughRows(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneBody
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{
		{Name: "Sync", Waves: []k8s.SyncWaveBucket{
			{Wave: 0, Resources: []k8s.SyncWaveResource{
				{Kind: "Pod", Name: "a"}, {Kind: "Pod", Name: "b"},
			}},
		}},
	}}
	m.syncWave.sidebarCursor = 0
	m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: 0, resourceIdx: -1}
	got, _ := m.handleSyncWaveBodyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, syncWaveBodyCursor{waveIdx: 0, resourceIdx: 0}, got.(Model).syncWave.bodyCursor)
	got, _ = got.(Model).handleSyncWaveBodyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, syncWaveBodyCursor{waveIdx: 0, resourceIdx: 1}, got.(Model).syncWave.bodyCursor)
}

func TestBodyKey_JStopsAtEnd(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneBody
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{
		{Name: "Sync", Waves: []k8s.SyncWaveBucket{
			{Wave: 0, Resources: []k8s.SyncWaveResource{{Kind: "Pod", Name: "a"}}},
		}},
	}}
	m.syncWave.sidebarCursor = 0
	m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: 0, resourceIdx: 0}
	got, _ := m.handleSyncWaveBodyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, syncWaveBodyCursor{waveIdx: 0, resourceIdx: 0}, got.(Model).syncWave.bodyCursor)
}

func TestBodyKey_JSkipsCollapsedWaveResources(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneBody
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{
		{Name: "Sync", Waves: []k8s.SyncWaveBucket{
			{Wave: 0, Resources: []k8s.SyncWaveResource{{Kind: "Pod", Name: "a"}}},
			{Wave: 1, Resources: []k8s.SyncWaveResource{{Kind: "Pod", Name: "b"}}},
		}},
	}}
	m.syncWave.sidebarCursor = 0
	m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: 0, resourceIdx: -1}
	m.syncWave.collapsed = map[string]bool{"Sync/wave 0": true}
	got, _ := m.handleSyncWaveBodyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, syncWaveBodyCursor{waveIdx: 1, resourceIdx: -1}, got.(Model).syncWave.bodyCursor)
}

func TestBodyKey_KMovesUp(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneBody
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{
		{Name: "Sync", Waves: []k8s.SyncWaveBucket{
			{Wave: 0, Resources: []k8s.SyncWaveResource{{Kind: "Pod", Name: "a"}}},
		}},
	}}
	m.syncWave.sidebarCursor = 0
	m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: 0, resourceIdx: 0}
	got, _ := m.handleSyncWaveBodyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, syncWaveBodyCursor{waveIdx: 0, resourceIdx: -1}, got.(Model).syncWave.bodyCursor)
}

// j past the visible viewport must scroll the body window so the cursor
// stays visible. Without this, j moves the cursor off-screen and the
// user perceives "scrolling not working".
func TestBodyKey_JScrollsBodyWhenCursorLeavesViewport(t *testing.T) {
	resources := make([]k8s.SyncWaveResource, 50)
	for i := range resources {
		resources[i] = k8s.SyncWaveResource{Kind: "Pod", Name: "p"}
	}
	m := Model{}
	m.height = 40 // viewport ≈ min(35, 34) - 4 - 5 = 26 rows
	m.syncWave.activePane = paneBody
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{
		{Name: "Sync", Waves: []k8s.SyncWaveBucket{{Wave: 0, Resources: resources}}},
	}}
	m.syncWave.sidebarCursor = 0
	m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: 0, resourceIdx: -1}

	// Press j enough times to exceed the viewport.
	for range 30 {
		got, _ := m.handleSyncWaveBodyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		m = got.(Model)
	}

	// Cursor is now on resource ~29 (flat index 30 — wave header + 30 j's
	// minus 1 because cursor started at wave header). bodyScroll must
	// have advanced to keep the cursor visible.
	assert.Greater(t, m.syncWave.bodyScroll, 0,
		"body must scroll once cursor passes the viewport")
}

// k from a row above the current bodyScroll must reduce bodyScroll so
// the cursor stays visible at the top of the viewport.
func TestBodyKey_KScrollsBodyUpWhenCursorAboveViewport(t *testing.T) {
	resources := make([]k8s.SyncWaveResource, 20)
	for i := range resources {
		resources[i] = k8s.SyncWaveResource{Kind: "Pod", Name: "p"}
	}
	m := Model{}
	m.height = 30
	m.syncWave.activePane = paneBody
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{
		{Name: "Sync", Waves: []k8s.SyncWaveBucket{{Wave: 0, Resources: resources}}},
	}}
	m.syncWave.sidebarCursor = 0
	m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: 0, resourceIdx: 0} // first resource (flat index 1)
	m.syncWave.bodyScroll = 15                                             // cursor far above

	got, _ := m.handleSyncWaveBodyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	gotM := got.(Model)
	// k moves cursor to wave header (flat index 0); bodyScroll must drop
	// to 0 so the wave header is visible.
	assert.Equal(t, 0, gotM.syncWave.bodyScroll,
		"k from cursor above scroll must reduce scroll to keep cursor visible")
}

func TestBodyKey_EnterOnWaveHeaderTogglesWaveCollapse(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneBody
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{
		{Name: "Sync", Waves: []k8s.SyncWaveBucket{
			{Wave: 0, Resources: []k8s.SyncWaveResource{{Kind: "Pod", Name: "a"}}},
		}},
	}}
	m.syncWave.sidebarCursor = 0
	m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: 0, resourceIdx: -1}
	m.syncWave.collapsed = map[string]bool{}
	got, _ := m.handleSyncWaveBodyKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, got.(Model).syncWave.collapsed["Sync/wave 0"])
}

func TestBodyKey_EnterOnPlaceholderTogglesPhaseCollapse(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneBody
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{{Name: "Sync"}}}
	m.syncWave.sidebarCursor = 0
	m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: -1, resourceIdx: -1}
	m.syncWave.collapsed = map[string]bool{}
	got, _ := m.handleSyncWaveBodyKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, got.(Model).syncWave.collapsed["Sync"])
}

func TestBodyKey_EnterOnResourceIsNoOp(t *testing.T) {
	m := Model{}
	m.syncWave.activePane = paneBody
	m.syncWave.data = &k8s.SyncWaveTimeline{Phases: []k8s.SyncWavePhase{
		{Name: "Sync", Waves: []k8s.SyncWaveBucket{
			{Wave: 0, Resources: []k8s.SyncWaveResource{{Kind: "Pod", Name: "a"}}},
		}},
	}}
	m.syncWave.sidebarCursor = 0
	m.syncWave.bodyCursor = syncWaveBodyCursor{waveIdx: 0, resourceIdx: 0}
	m.syncWave.collapsed = map[string]bool{}
	got, _ := m.handleSyncWaveBodyKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Empty(t, got.(Model).syncWave.collapsed)
}
