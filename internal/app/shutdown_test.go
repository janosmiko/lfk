package app

import (
	"context"
	"errors"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// beginShutdown must give the user immediate visible feedback (the
// shutdown notice overlay + shuttingDown flag) and hand the blocking
// drain off to a command rather than running it inline, so the render
// loop is never frozen while workers wind down.
func TestBeginShutdown_ShowsNoticeAndReturnsDrainCmd(t *testing.T) {
	m := baseModelWithFakeClient()
	m.tabs = []TabState{{}}

	ret, cmd := m.beginShutdown()
	result := ret.(Model)

	assert.True(t, result.shuttingDown, "shutdown must mark the model")
	assert.Equal(t, overlayShuttingDown, result.overlay,
		"shutdown must switch to the graceful-shutdown notice")
	require.NotNil(t, cmd, "shutdown must return the drain command")
}

// The instant cancellations (in-flight API requests) run synchronously in
// beginShutdown so they abort before they ride out TCP timeouts — the
// blocking drain is deferred, but cancellation is not.
func TestBeginShutdown_CancelsInFlightRequestsSynchronously(t *testing.T) {
	m := baseModelWithFakeClient()
	m.tabs = []TabState{{}}
	reqCtx := armReqCtx(&m)
	require.NoError(t, reqCtx.Err())

	_, _ = m.beginShutdown()

	assert.ErrorIs(t, reqCtx.Err(), context.Canceled,
		"beginShutdown must cancel reqCtx before deferring the drain")
}

// The drain command, once executed, reports completion so Update can
// dispatch tea.Quit.
func TestShutdownDrainCmd_EmitsCompleteMsg(t *testing.T) {
	m := baseModelWithFakeClient()
	m.tabs = []TabState{{}}

	cmd := m.shutdownDrainCmd()
	require.NotNil(t, cmd)

	msg := cmd()
	assert.IsType(t, shutdownCompleteMsg{}, msg)
}

// While shutting down, Update freezes the model: every message except the
// drain's completion is swallowed so nothing mutates state under the drain
// goroutine.
func TestUpdate_FreezesModelWhileShuttingDown(t *testing.T) {
	m := baseModelWithFakeClient()
	m.tabs = []TabState{{}}
	m.shuttingDown = true
	m.overlay = overlayShuttingDown

	// A keypress must be ignored.
	ret, cmd := m.Update(keyMsg("q"))
	result := ret.(Model)
	assert.Nil(t, cmd, "no command should be issued during shutdown")
	assert.True(t, result.shuttingDown, "shutdown state must persist")
	assert.Equal(t, overlayShuttingDown, result.overlay)

	// The drain's completion still drives the quit.
	_, cmd = m.Update(shutdownCompleteMsg{})
	require.NotNil(t, cmd, "completion must dispatch quit")
	assert.Equal(t, tea.Quit(), cmd())
}

// beginShutdown invokes the registered notifier exactly once so main can
// arm its force-quit watchdog.
func TestBeginShutdown_FiresNotifier(t *testing.T) {
	m := baseModelWithFakeClient()
	m.tabs = []TabState{{}}
	calls := 0
	m.SetShutdownNotifier(func() { calls++ })

	_, _ = m.beginShutdown()

	assert.Equal(t, 1, calls, "shutdown notifier must fire once")
}

// isClosedFile reports whether f has already been closed.
func isClosedFile(t *testing.T, f *os.File) bool {
	t.Helper()
	_, err := f.Write([]byte{0})
	return errors.Is(err, os.ErrClosed)
}

func TestSignalShutdown_ClosesActiveExecPTY(t *testing.T) {
	m := baseModelWithFakeClient()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})
	m.execPTY = w
	// TabState mirrors Model's exec state after saveCurrentTab. The mirror
	// must be nilled too, so a later restore can't re-close the same fd.
	m.tabs = []TabState{{execPTY: w}}

	m.signalShutdown()

	assert.Nil(t, m.execPTY, "signalShutdown must nil out the active exec PTY")
	assert.Nil(t, m.tabs[0].execPTY, "signalShutdown must nil out the active tab's mirrored exec PTY")
	assert.True(t, isClosedFile(t, w), "signalShutdown must close the active exec PTY")
}

// A stale saveCurrentTab snapshot or a late-landing async PTY start can
// leave the active tab's mirror pointing at a different PTY than
// m.execPTY. Both must be closed, not just the live one.
func TestSignalShutdown_ClosesActiveTabMirrorWhenDistinctFromModelPTY(t *testing.T) {
	m := baseModelWithFakeClient()

	rLive, wLive, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = rLive.Close()
		_ = wLive.Close()
	})
	rStale, wStale, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = rStale.Close()
		_ = wStale.Close()
	})

	m.execPTY = wLive
	m.tabs = []TabState{{execPTY: wStale}}

	m.signalShutdown()

	assert.True(t, isClosedFile(t, wLive), "signalShutdown must close the live exec PTY")
	assert.True(t, isClosedFile(t, wStale), "signalShutdown must close the stale mirrored exec PTY too")
	assert.Nil(t, m.tabs[0].execPTY, "signalShutdown must nil out the active tab's mirrored exec PTY")
}

func TestSignalShutdown_ClosesBackgroundTabExecPTY(t *testing.T) {
	m := baseModelWithFakeClient()

	rBg, wBg, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = rBg.Close()
		_ = wBg.Close()
	})

	m.tabs = []TabState{
		{execPTY: wBg},
		{},
	}
	m.activeTab = 1

	m.signalShutdown()

	assert.Nil(t, m.tabs[0].execPTY, "signalShutdown must nil out the background tab's exec PTY")
	assert.True(t, isClosedFile(t, wBg), "signalShutdown must close the background tab's exec PTY")
}

func TestSignalShutdown_IsIdempotentAndNilSafe(t *testing.T) {
	m := baseModelWithFakeClient()
	m.tabs = []TabState{{}, {}}
	m.activeTab = 0

	assert.NotPanics(t, func() {
		m.signalShutdown()
		m.signalShutdown()
	})
}
