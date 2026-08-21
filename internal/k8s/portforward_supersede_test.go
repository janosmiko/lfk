package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A racing SetUpdateListener must not close the old superseded channel between
// the choice of listener and the send, which would strand the update on a
// channel its listener has stopped reading.
func TestPortForwardManager_NotifyBeatsRacingSupersession(t *testing.T) {
	mgr := NewPortForwardManager()
	first := make(chan struct{}, 1)
	superseded := mgr.SetUpdateListener(first)

	mgr.mu.Lock()

	superseding := make(chan struct{})
	go func() {
		mgr.SetUpdateListener(make(chan struct{}, 1))
		close(superseding)
	}()
	require.Never(t, func() bool { return isClosed(superseding) },
		50*time.Millisecond, 5*time.Millisecond,
		"SetUpdateListener must wait for the lock a notify site holds")

	mgr.notifyLocked()

	require.False(t, isClosed(superseded),
		"the old listener was retired before its update was delivered")
	require.Len(t, first, 1, "the update must reach the listener chosen under the lock")

	mgr.mu.Unlock()

	require.Eventually(t, func() bool { return isClosed(superseding) },
		time.Second, 5*time.Millisecond)
	require.True(t, isClosed(superseded))
	require.Len(t, first, 1, "the delivered update must survive the supersession")
}

// A listener that returned leaves its channel unread forever, so the manager
// must never wait on it.
func TestPortForwardManager_NotifyDoesNotBlockOnAbandonedListener(t *testing.T) {
	mgr := NewPortForwardManager()
	mgr.SetUpdateListener(make(chan struct{})) // unbuffered, nobody receiving

	done := make(chan struct{})
	go func() {
		mgr.mu.Lock()
		mgr.notifyLocked()
		mgr.notifyLocked()
		mgr.mu.Unlock()
		close(done)
	}()

	require.Eventually(t, func() bool { return isClosed(done) },
		time.Second, 5*time.Millisecond, "notifyLocked blocked on an abandoned listener")
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
