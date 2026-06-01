package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// qtask builds a bare queued task carrying only a priority, for white-box
// dequeue tests that append directly to lanes (bypassing enqueueLocked's
// wake signal).
func qtask(p Priority) *queuedTask {
	return &queuedTask{req: SubmitReq{Priority: p}, future: make(chan Result, 1)}
}

// TestDequeueByPriority_AgingPromotesStarvedLane verifies the anti-starvation
// aging: a Low lane passed over more than agingThreshold times is promoted
// ahead of a still-non-empty High lane for a single dequeue, then strict
// priority resumes.
func TestDequeueByPriority_AgingPromotesStarvedLane(t *testing.T) {
	q := newCtxQueue(3)
	q.mu.Lock()
	defer q.mu.Unlock()
	for range 10 {
		q.lanes[int(PriorityHigh)] = append(q.lanes[int(PriorityHigh)], qtask(PriorityHigh))
	}
	q.lanes[int(PriorityLow)] = append(q.lanes[int(PriorityLow)], qtask(PriorityLow))

	got := make([]Priority, 0, 4)
	for range 4 {
		tk, ok := q.dequeueByPriorityLocked()
		require.True(t, ok)
		got = append(got, tk.req.Priority)
	}
	// threshold=3: three High picks age Low to 3, the fourth dequeue promotes Low.
	assert.Equal(t,
		[]Priority{PriorityHigh, PriorityHigh, PriorityHigh, PriorityLow},
		got,
		"Low must be promoted after agingThreshold High picks")
}

// TestDequeueByPriority_CriticalNeverAged verifies Critical is absolute: it is
// served outright even when a lower lane has been starved far past the
// threshold, so foundational/mutation work is never delayed by aging.
func TestDequeueByPriority_CriticalNeverAged(t *testing.T) {
	q := newCtxQueue(2)
	q.mu.Lock()
	defer q.mu.Unlock()
	q.skips[int(PriorityLow)] = 100 // Low starved "forever"
	q.lanes[int(PriorityCritical)] = append(q.lanes[int(PriorityCritical)], qtask(PriorityCritical))
	q.lanes[int(PriorityLow)] = append(q.lanes[int(PriorityLow)], qtask(PriorityLow))

	tk, ok := q.dequeueByPriorityLocked()
	require.True(t, ok)
	assert.Equal(t, PriorityCritical, tk.req.Priority, "Critical must win even when Low is starved")
}

// TestDequeueByPriority_AgingDisabledIsStrict verifies a non-positive threshold
// disables aging entirely (strict priority) — the kill switch.
func TestDequeueByPriority_AgingDisabledIsStrict(t *testing.T) {
	q := newCtxQueue(0)
	q.mu.Lock()
	defer q.mu.Unlock()
	for range 5 {
		q.lanes[int(PriorityHigh)] = append(q.lanes[int(PriorityHigh)], qtask(PriorityHigh))
	}
	q.lanes[int(PriorityLow)] = append(q.lanes[int(PriorityLow)], qtask(PriorityLow))

	for i := range 5 {
		tk, ok := q.dequeueByPriorityLocked()
		require.True(t, ok)
		assert.Equal(t, PriorityHigh, tk.req.Priority, "strict priority: High before Low at dequeue %d", i)
	}
	tk, ok := q.dequeueByPriorityLocked()
	require.True(t, ok)
	assert.Equal(t, PriorityLow, tk.req.Priority)
}

// TestWorkerDispatch_AgingPreventsLowStarvation is the end-to-end repro of the
// reported bug: under a continuous backlog of High work on a slow cluster, a
// Low task must still run rather than being starved to the very end. A single
// worker serializes dispatch so the order is deterministic; each task blocks
// until the test releases it, mirroring a slow API server.
func TestWorkerDispatch_AgingPreventsLowStarvation(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.SetWorkersForTest(1, 0)     // single worker => fully serial dispatch
	r.SetAgingThresholdForTest(3) // promote a starved lane after 3 skips

	executed := make(chan Priority, 64)
	release := make(chan struct{})
	mk := func(p Priority) func(context.Context) (any, error) {
		return func(ctx context.Context) (any, error) {
			executed <- p
			select {
			case <-release:
			case <-ctx.Done():
			}
			return nil, nil
		}
	}

	// One Low task, then a backlog of distinct High tasks (distinct targets so
	// they do not coalesce). Submitted before StartWorkers so all are queued.
	r.Submit(SubmitReq{
		KubeContext: "c1", Kind: KindSecurityScan, Priority: PriorityLow,
		Target: "low", Fn: mk(PriorityLow), Timeout: 10 * time.Second,
	})
	const highN = 12
	for i := range highN {
		r.Submit(SubmitReq{
			KubeContext: "c1", Kind: KindResourceList, Priority: PriorityHigh,
			Target: fmt.Sprintf("h%d", i), Fn: mk(PriorityHigh), Timeout: 10 * time.Second,
		})
	}

	r.StartWorkers()
	defer r.StopWorkers()

	lowAt := -1
	for i := 1; i <= highN+1; i++ {
		var p Priority
		select {
		case p = <-executed:
		case <-time.After(2 * time.Second):
			close(release)
			t.Fatalf("dispatch stalled at #%d", i)
		}
		if p == PriorityLow {
			lowAt = i
			close(release) // let this and remaining tasks finish
			break
		}
		release <- struct{}{} // let this High finish so the worker picks the next
	}
	require.NotEqual(t, -1, lowAt, "Low task never ran (starved to the end)")
	// Strict priority would put Low at highN+1 (=13). Aging must surface it no
	// later than threshold+1 dispatches.
	assert.LessOrEqual(t, lowAt, 4, "Low must be promoted after ~threshold High picks, not starved")
}
