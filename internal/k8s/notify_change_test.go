package k8s

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The delayed Starting->Running flip can find an entry the watchdog already
// moved to a terminal state. That transition notified on its own, so a second
// token here wakes the UI with nothing to redraw.
func TestCaptureManager_RunningFlipNotifiesOnlyOnAStatusChange(t *testing.T) {
	mgr := NewCaptureManager()
	ch := make(chan struct{}, 1)
	mgr.SetUpdateListener(ch)

	done := &CaptureEntry{Status: CaptureStopped}
	mgr.markCaptureRunning(done)
	assert.Equal(t, CaptureStopped, done.Status, "a terminal capture must not flip back to Running")
	assert.Empty(t, ch, "a flip that changed nothing must not wake the UI")

	starting := &CaptureEntry{Status: CaptureStarting}
	mgr.markCaptureRunning(starting)
	assert.Equal(t, CaptureRunning, starting.Status)
	require.Len(t, ch, 1, "the Starting->Running flip must reach the UI")
}

// Stop and StopAll set the terminal status themselves and announce it. The
// monitor then sees the process exit and must stay quiet about a state it did
// not change.
func TestPortForwardManager_MonitorExitNotifiesOnlyOnAStateChange(t *testing.T) {
	mgr := NewPortForwardManager()
	ch := make(chan struct{}, 1)
	mgr.SetUpdateListener(ch)

	stopped := &PortForwardEntry{Status: PortForwardStopped}
	mgr.recordProcessExit(stopped, errors.New("signal: killed"), "")
	assert.Equal(t, PortForwardStopped, stopped.Status, "an already-stopped forward must not turn into a failure")
	assert.Empty(t, stopped.Error)
	assert.Empty(t, ch, "an exit that changed nothing must not wake the UI")

	crashed := &PortForwardEntry{Status: PortForwardRunning}
	mgr.recordProcessExit(crashed, errors.New("exit status 1"), "bind: address already in use")
	assert.Equal(t, PortForwardFailed, crashed.Status)
	assert.Equal(t, "bind: address already in use", crashed.Error)
	require.Len(t, ch, 1, "a live forward that failed must reach the UI")
}
