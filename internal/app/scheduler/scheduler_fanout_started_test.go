package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// TestWorkerDispatch_FanOutWhileBusy mirrors the real jam: the pool is already
// running, one worker is busy on a long task, and a burst of further work is
// submitted. Every remaining worker must engage — not leave the burst queued
// behind the one busy worker.
func TestWorkerDispatch_FanOutWhileBusy(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.SetWorkersForTest(4, 1) // production shape: 4 workers, 1 Critical-reserved
	r.StartWorkers()
	defer r.StopWorkers()

	var running atomic.Int32
	reached := make(chan struct{}, 8)
	release := make(chan struct{})
	blockingFn := func(ctx context.Context) (any, error) {
		running.Add(1)
		reached <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		running.Add(-1)
		return nil, nil
	}

	// One long-running Critical task occupies the reserved worker.
	r.Submit(SubmitReq{
		KubeContext: "c1", Kind: KindResourceList, Priority: PriorityCritical,
		Target: "crit", Fn: blockingFn, Timeout: 10 * time.Second,
	})
	// Wait until it is actually running before submitting the burst, so the
	// burst lands while a worker is busy (the real scenario).
	<-reached

	// Burst of High work that must run on the 3 general workers.
	const high = 3
	for i := range high {
		r.Submit(SubmitReq{
			KubeContext: "c1", Kind: KindContainers, Priority: PriorityHigh,
			Target: fmt.Sprintf("h%d", i), Fn: blockingFn, Timeout: 10 * time.Second,
		})
	}

	// Expect 1 (critical) + 3 (high) = 4 concurrent.
	timeout := time.After(2 * time.Second)
	for got := 1; got < 1+high; got++ {
		select {
		case <-reached:
		case <-timeout:
			close(release)
			t.Fatalf("only %d of %d tasks running; general workers did not engage", got, 1+high)
		}
	}
	close(release)
}
