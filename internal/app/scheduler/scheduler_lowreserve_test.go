package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClampLowReserved verifies the low-reserved clamp always leaves at least
// one general worker (so High work is never fully without a dedicated slot)
// and floors at zero.
func TestClampLowReserved(t *testing.T) {
	assert.Equal(t, 0, ClampLowReserved(-1, 8, 1))
	assert.Equal(t, 0, ClampLowReserved(0, 8, 1))
	assert.Equal(t, 1, ClampLowReserved(1, 8, 1))
	assert.Equal(t, 6, ClampLowReserved(6, 8, 1))  // 8 - 1 critical - 1 general
	assert.Equal(t, 6, ClampLowReserved(99, 8, 1)) // capped
	assert.Equal(t, 0, ClampLowReserved(1, 1, 0))  // single worker: no floor
	assert.Equal(t, 1, ClampLowReserved(5, 2, 0))  // 2 - 0 - 1 general = 1
}

// TestPickTask_LowReservedPrefersLowLane verifies a low-reserved worker takes
// queued Low work ahead of High, while a general worker keeps strict priority
// (High first). This is the core of the background-work floor.
func TestPickTask_LowReservedPrefersLowLane(t *testing.T) {
	r := New(0)
	defer r.Close()
	q := newCtxQueue(DefaultAgingThreshold)
	q.lanes[int(PriorityHigh)] = append(q.lanes[int(PriorityHigh)], qtask(PriorityHigh))
	q.lanes[int(PriorityLow)] = append(q.lanes[int(PriorityLow)], qtask(PriorityLow))

	// Low-reserved worker pulls the Low task first.
	got, ok := r.pickTask(q, workerClassLow)
	require.True(t, ok)
	assert.Equal(t, PriorityLow, got.req.Priority, "low-reserved worker must prefer the Low lane")

	// A general worker, faced with only High remaining, takes High.
	got, ok = r.pickTask(q, workerClassGeneral)
	require.True(t, ok)
	assert.Equal(t, PriorityHigh, got.req.Priority)
}

// TestPickTask_LowReservedFallsBackWhenNoLow verifies a low-reserved worker
// does not idle when no Low work is queued — it falls back to the highest
// available priority, so the reserved slot is never wasted under High load.
func TestPickTask_LowReservedFallsBackWhenNoLow(t *testing.T) {
	r := New(0)
	defer r.Close()
	q := newCtxQueue(DefaultAgingThreshold)
	q.lanes[int(PriorityHigh)] = append(q.lanes[int(PriorityHigh)], qtask(PriorityHigh))

	got, ok := r.pickTask(q, workerClassLow)
	require.True(t, ok)
	assert.Equal(t, PriorityHigh, got.req.Priority, "no Low queued: low-reserved worker falls back to High")
}

// TestWorkerDispatch_LowReservedFloorRunsBackgroundUnderHighLoad is the
// end-to-end repro of the reported bug: with a backlog of High work, the
// reserved background worker still runs a Low task in the first dispatch wave
// instead of leaving it queued behind the whole High backlog.
//
// Pool: 1 general + 1 low-reserved (workers=2, critical=0, low=1). Submitted
// before StartWorkers so the lanes are fully populated when workers wake.
func TestWorkerDispatch_LowReservedFloorRunsBackgroundUnderHighLoad(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.SetWorkersForTest(2, 0)
	r.SetLowReservedForTest(1)

	executed := make(chan Priority, 16)
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

	// A flood of High work plus a single Low task queued behind it.
	const highN = 6
	for i := range highN {
		r.Submit(SubmitReq{
			KubeContext: "c1", Kind: KindResourceList, Priority: PriorityHigh,
			Target: fmt.Sprintf("h%d", i), Fn: mk(PriorityHigh), Timeout: 10 * time.Second,
		})
	}
	r.Submit(SubmitReq{
		KubeContext: "c1", Kind: KindMetrics, Priority: PriorityLow,
		Target: "low", Fn: mk(PriorityLow), Timeout: 10 * time.Second,
	})

	r.StartWorkers()
	defer r.StopWorkers()

	// Collect the first wave (2 workers => 2 concurrent dispatches before any
	// release). The low-reserved worker must have taken the Low task.
	first := make([]Priority, 0, 2)
	for range 2 {
		select {
		case p := <-executed:
			first = append(first, p)
		case <-time.After(2 * time.Second):
			close(release)
			t.Fatalf("only %d task(s) started; pool did not engage", len(first))
		}
	}
	close(release)

	sawLow := false
	for _, p := range first {
		if p == PriorityLow {
			sawLow = true
		}
	}
	assert.True(t, sawLow,
		"low-reserved worker must run the Low task in the first wave, not starve it behind the High backlog; got %v", first)
}
