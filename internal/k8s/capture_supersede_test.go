package k8s

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

	// Shrunk so the Starting->Running flip notify lands inside the churn
	// window below (which closes in low-single-digit milliseconds), instead
	// of firing 100ms later once the churn loop has long since stopped.
	origFlipDelay := captureRunningFlipDelay.Load()
	captureRunningFlipDelay.Store(int64(time.Microsecond))
	defer captureRunningFlipDelay.Store(origFlipDelay)

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

	// Several concurrent churners, not one, give a nanosecond-wide race
	// window a realistic chance of being hit. assert (not require) is the
	// only failure call safe to make off the test's own goroutine.
	const churners = 16
	done := make(chan struct{})
	var wg sync.WaitGroup
	churnCounts := make([]int, churners)
	for g := range churners {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ring := make([]chan struct{}, checkWindow)
			cur := make(chan struct{}, 1)
			mgr.SetUpdateListener(cur)
			churns := 0
			for {
				select {
				case <-done:
					for i, ch := range ring {
						assert.Emptyf(t, ch, "retired listener %d (churner %d) was handed an update after its supersession, so nobody read it", i, idx)
					}
					churnCounts[idx] = churns
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
				assert.Emptyf(t, ring[slot], "a retired listener (churner %d) was handed an update after its supersession, so nobody read it", idx)
				ring[slot] = cur
				cur = next
				churns++
			}
		}(g)
	}

	require.NoError(t, <-started)
	mgr.StopAll()
	close(done)
	wg.Wait()

	total := 0
	for _, c := range churnCounts {
		total += c
	}
	t.Logf("%d supersessions (across %d churners) raced %d captures", total, churners, captures)
}
