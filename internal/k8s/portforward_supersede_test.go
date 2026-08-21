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

// A notify that picks its listener under the lock and sends after releasing it
// hands the update to a channel nobody reads any more. Only the public API can
// catch that: notifyLocked under a held lock excludes the race by construction.
func TestPortForwardManager_SupersededListenerNeverLosesAnUpdate(t *testing.T) {
	const rounds = 2000

	mgr := NewPortForwardManager()
	var retired []chan struct{}

	for i := range rounds {
		id := i + 1
		mgr.mu.Lock()
		mgr.entries = []*PortForwardEntry{{ID: id, Status: PortForwardRunning, cancel: func() {}}}
		mgr.mu.Unlock()

		ch := make(chan struct{}, 1)
		superseded := mgr.SetUpdateListener(ch)

		// One gate for both goroutines so the change and the supersession
		// reach the manager together instead of in a fixed order.
		start := make(chan struct{})
		changed := make(chan struct{})
		go func() {
			<-start
			if id%2 == 0 {
				mgr.Remove(id)
			} else {
				_ = mgr.Stop(id)
			}
			close(changed)
		}()
		swapped := make(chan struct{})
		go func() {
			<-start
			mgr.SetUpdateListener(make(chan struct{}, 1))
			close(swapped)
		}()
		close(start)

		delivered := awaitUpdate(ch, superseded)
		<-changed
		<-swapped
		if !delivered {
			retired = append(retired, ch)
		}
	}

	for i, ch := range retired {
		require.Emptyf(t, ch, "retired listener %d was handed an update after its supersession, so nobody read it", i)
	}
}

// awaitUpdate mirrors the app-side waiter. It reports false only when the
// listener was retired with nothing pending, so a token found in its channel
// afterwards is one the manager delivered too late for anyone to read.
func awaitUpdate(ch <-chan struct{}, superseded <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	case <-superseded:
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
