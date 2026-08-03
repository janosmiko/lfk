package app

import (
	"context"
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
