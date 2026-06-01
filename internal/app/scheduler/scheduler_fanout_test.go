package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerDispatch_BurstFansOutAcrossWorkers reproduces the lost-wakeup jam:
// a burst of submits whose wake signals collapse into the size-1 wake buffer
// must still engage every worker, not leave one worker draining serially while
// the others stay parked. Submitting before StartWorkers makes the collapse
// deterministic — all N enqueues leave exactly one buffered wake — so without
// a wake-the-sibling step only one worker ever runs.
func TestWorkerDispatch_BurstFansOutAcrossWorkers(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.SetWorkersForTest(4, 0) // 4 general workers, no Critical reservation

	const n = 4
	var running atomic.Int32
	reached := make(chan struct{}, n)
	release := make(chan struct{})

	fn := func(ctx context.Context) (any, error) {
		running.Add(1)
		reached <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		running.Add(-1)
		return nil, nil
	}

	// Distinct targets => distinct Sig => no coalescing. Long timeout so a
	// blocked task does not auto-return and mask the lack of concurrency.
	for i := range n {
		r.Submit(SubmitReq{
			KubeContext: "c1",
			Kind:        KindResourceList,
			Priority:    PriorityHigh,
			Target:      fmt.Sprintf("t%d", i),
			Fn:          fn,
			Timeout:     10 * time.Second,
		})
	}

	r.StartWorkers()
	defer r.StopWorkers()

	timeout := time.After(2 * time.Second)
	for got := range n {
		select {
		case <-reached:
		case <-timeout:
			close(release)
			t.Fatalf("only %d of %d tasks started concurrently; workers did not fan out (lost wakeup)", got, n)
		}
	}
	assert.Equal(t, int32(n), running.Load(), "all workers must run the burst concurrently")
	close(release)
}
