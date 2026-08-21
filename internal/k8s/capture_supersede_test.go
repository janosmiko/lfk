package k8s

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// checkWindow is how many further supersessions happen before a retired channel
// is inspected. A late delivery lands microseconds after the supersession, so
// this only has to outlast that, and it bounds the memory the sweep holds.
const checkWindow = 256

// A notify that picks its listener under the lock and sends after releasing it
// hands the update to a channel nobody reads any more. The churn loop keeps a
// SetUpdateListener call queued on the manager lock so it observes that gap.
func TestCaptureManager_SupersededListenerNeverLosesAnUpdate(t *testing.T) {
	const captures = 50

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mgr := NewCaptureManager()
	mgr.SetBackendFactory(func(CaptureBackend) (captureBackend, error) {
		return &fakeBackend{stream: bytes.NewReader(emptyPcapHeader())}, nil
	})
	req := CaptureRequest{
		Backend:   BackendKubectlDebug,
		Context:   "ctx",
		Namespace: "ns",
		PodName:   "pod",
		OutputDir: t.TempDir(),
	}

	started := make(chan error, 1)
	go func() {
		for range captures {
			if _, err := mgr.Start(ctx, req, func(PacketSummary) {}); err != nil {
				started <- err
				return
			}
		}
		started <- nil
	}()

	// A retired channel that is still empty checkWindow supersessions later
	// was never handed a late update.
	ring := make([]chan struct{}, checkWindow)
	cur := make(chan struct{}, 1)
	mgr.SetUpdateListener(cur)
	churns := 0
	for {
		select {
		case err := <-started:
			require.NoError(t, err)
			mgr.StopAll()
			for i, ch := range ring {
				require.Emptyf(t, ch, "retired listener %d was handed an update after its supersession, so nobody read it", i)
			}
			t.Logf("%d supersessions raced %d captures", churns, captures)
			return
		default:
		}

		next := make(chan struct{}, 1)
		mgr.SetUpdateListener(next)
		// Whatever arrived before the supersession is the waiter's to collect.
		select {
		case <-cur:
		default:
		}
		slot := churns % checkWindow
		require.Emptyf(t, ring[slot], "a retired listener was handed an update after its supersession, so nobody read it")
		ring[slot] = cur
		cur = next
		churns++
	}
}
