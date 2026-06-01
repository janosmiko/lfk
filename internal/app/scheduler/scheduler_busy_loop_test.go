//go:build !windows

package scheduler

import (
	"syscall"
	"testing"
	"time"
)

// TestWorkerLoop_DoesNotBusyLoopOnStaleWake exercises the regression
// fixed for issue #206. After the inner drain loop empties the queue,
// any wake signal that arrived during the drain stays in q.wake. The
// previous code would consume it on the next outer-select iteration,
// pickTask would return (nil, false) for the empty queue, the worker
// would unconditionally re-signal, and the same goroutine would race
// its way back to the receive — feeding itself stale wakes in a hot
// loop. Production symptom: 2.26M goroutine block/unblock events per
// second, ~145% idle CPU on macOS.
//
// The bug requires a wake signal to land in q.wake while the queue is
// empty. We inject one directly so the test is deterministic (a
// burst-submit test races against the runtime and the bug may not
// surface in the test window). The fix gates the re-signal on
// q.hasPendingWork(), so the injected stale wake should be consumed
// once and the loop should park.
func TestWorkerLoop_DoesNotBusyLoopOnStaleWake(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.SetWorkersForTest(4, 1) // matches prod defaults: 1 critical + 3 general

	// Create the ctxQueue for "c1" and start the worker pool against
	// it WITHOUT submitting any task. ensurePoolFor walks ctxQueues
	// when r.started, so we need the queue to exist before
	// StartWorkers.
	r.mu.Lock()
	q := newCtxQueue(DefaultAgingThreshold)
	r.ctxQueues["c1"] = q
	r.mu.Unlock()
	r.StartWorkers()
	defer r.StopWorkers()

	// Wait briefly for workers to park on the empty queue.
	time.Sleep(20 * time.Millisecond)

	// Inject a stale wake — simulating a leftover signal from a prior
	// drain. The queue is empty, so on the buggy code a worker will
	// pick it up, fail pickTask, re-signal, and hot-loop indefinitely.
	q.wake <- struct{}{}

	// Sample CPU over 300 ms of wall time. Use kernel rusage so the
	// measurement reflects real CPU burn, independent of Go's
	// scheduler accounting (which the busy loop would distort).
	const window = 300 * time.Millisecond
	cpuStart := rusageCPU(t)
	time.Sleep(window)
	cpuUsed := rusageCPU(t) - cpuStart

	// Pre-fix: cpuUsed ~ 400-700 ms (1+ core burning the whole window).
	// Post-fix: a few ms at most. 50 ms gives plenty of headroom for
	// loaded CI runners.
	const ceiling = 50 * time.Millisecond
	if cpuUsed > ceiling {
		t.Fatalf("scheduler busy-loops on stale wake: used %v of CPU in %v of wall time (ceiling %v)",
			cpuUsed, window, ceiling)
	}
}

// rusageCPU returns this process's combined user+sys CPU time so far.
// Used by the busy-loop regression test so the measurement is taken
// from the kernel rather than Go's runtime accounting.
func rusageCPU(t *testing.T) time.Duration {
	t.Helper()
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		t.Fatalf("getrusage: %v", err)
	}
	return tvToDur(ru.Utime) + tvToDur(ru.Stime)
}

func tvToDur(tv syscall.Timeval) time.Duration {
	return time.Duration(tv.Sec)*time.Second + time.Duration(tv.Usec)*time.Microsecond
}
