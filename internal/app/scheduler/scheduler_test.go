package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitReq_Sig(t *testing.T) {
	req := SubmitReq{
		KubeContext: "c1",
		Kind:        KindResourceList,
		Target:      "default",
		Gen:         5,
	}
	assert.Equal(t, Sig{KubeContext: "c1", Kind: KindResourceList, Target: "default", Gen: 5}, req.Sig())
}

func TestSubmitReq_PriorityZeroValueIsCritical(t *testing.T) {
	// PriorityCritical is the zero value of the Priority type. Callers
	// that forget to set Priority get Critical — preferable to Low for
	// safety (it ensures things run; coalesce + reserved slot keep it
	// sensible).
	req := SubmitReq{Kind: KindMetrics}
	assert.Equal(t, PriorityCritical, req.Priority)
}

func TestErrSentinels(t *testing.T) {
	assert.True(t, errors.Is(ErrCoalesced, ErrCoalesced))
	assert.True(t, errors.Is(ErrContextSwitched, ErrContextSwitched))
	assert.NotErrorIs(t, ErrCoalesced, ErrContextSwitched)
	assert.NotErrorIs(t, ErrContextSwitched, ErrCoalesced)
}

func TestResult_Zero(t *testing.T) {
	var r Result
	assert.Nil(t, r.Value)
	assert.NoError(t, r.Err)
}

// helpers used across scheduler_test.go (and later tests in workers_test).

func mkReq(kctx string, kind Kind, prio Priority, target string, fn func(context.Context) (any, error)) SubmitReq {
	return SubmitReq{
		KubeContext: kctx,
		Kind:        kind,
		Priority:    prio,
		Target:      target,
		Fn:          fn,
		Timeout:     1 * time.Second,
	}
}

// noopFn returns nil/nil and never blocks. Used when only signature/queueing matters.
func noopFn(_ context.Context) (any, error) { return nil, nil }

// Confirm the package compiles with these types in use.
func TestPlaceholder_BuildOK(t *testing.T) {
	_ = mkReq("c", KindResourceList, PriorityHigh, "tgt", noopFn)
	require.NotPanics(t, func() {
		var f Future = make(chan Result, 1)
		_ = f
	})
	// Quiet unused-import warnings in this skeleton file.
	_ = sync.Mutex{}
}

func TestSubmit_EnqueuesByPriority(t *testing.T) {
	r := New(0)
	defer r.Close()

	// Submit one of each priority. No workers running yet (Start() not
	// called), so they sit in the queue.
	low := r.Submit(mkReq("c1", KindDashboard, PriorityLow, "Low task", noopFn))
	high := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "High task", noopFn))
	crit := r.Submit(mkReq("c1", KindAPIDiscovery, PriorityCritical, "Crit task", noopFn))

	require.NotNil(t, low)
	require.NotNil(t, high)
	require.NotNil(t, crit)

	// QueueLen reports total queued across all priorities.
	assert.Equal(t, 3, r.QueueLen("c1"))
	assert.Equal(t, 0, r.QueueLen("c2"))

	// QueueLenByPriority reports per-tier counts.
	assert.Equal(t, 1, r.QueueLenByPriority("c1", PriorityCritical))
	assert.Equal(t, 1, r.QueueLenByPriority("c1", PriorityHigh))
	assert.Equal(t, 1, r.QueueLenByPriority("c1", PriorityLow))
}

func TestSubmit_PerContextIsolation(t *testing.T) {
	r := New(0)
	defer r.Close()

	r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "in c1", noopFn))
	r.Submit(mkReq("c2", KindResourceList, PriorityHigh, "in c2", noopFn))

	assert.Equal(t, 1, r.QueueLen("c1"))
	assert.Equal(t, 1, r.QueueLen("c2"))
}

func TestSubmit_NilRegistry(t *testing.T) {
	var r *Registry
	fut := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "x", noopFn))
	require.NotNil(t, fut)
	select {
	case res := <-fut:
		assert.ErrorIs(t, res.Err, ErrContextSwitched)
	case <-time.After(time.Second):
		t.Fatal("nil Registry should immediately deliver ErrContextSwitched")
	}
}

func TestSubmit_AfterCloseReturnsContextSwitched(t *testing.T) {
	r := New(0)
	r.Close()
	fut := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "x", noopFn))
	select {
	case res := <-fut:
		assert.ErrorIs(t, res.Err, ErrContextSwitched)
	case <-time.After(time.Second):
		t.Fatal("Submit after Close should immediately deliver ErrContextSwitched")
	}
}

func TestClose_DrainsQueuedFutures(t *testing.T) {
	r := New(0)
	fut1 := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "x", noopFn))
	fut2 := r.Submit(mkReq("c1", KindMetrics, PriorityLow, "y", noopFn))

	r.Close()

	for _, f := range []Future{fut1, fut2} {
		select {
		case res := <-f:
			assert.ErrorIs(t, res.Err, ErrContextSwitched)
		case <-time.After(time.Second):
			t.Fatal("Close must drain pending Futures with ErrContextSwitched")
		}
	}
}

func TestClose_Idempotent(t *testing.T) {
	r := New(0)
	r.Close()
	r.Close() // must not panic
}

func TestWorkerDispatch_HappyPath(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.StartWorkers()
	defer r.StopWorkers()

	want := "hello"
	fut := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "test", func(ctx context.Context) (any, error) {
		return want, nil
	}))

	select {
	case res := <-fut:
		require.NoError(t, res.Err)
		assert.Equal(t, want, res.Value)
	case <-time.After(2 * time.Second):
		t.Fatal("Future never delivered")
	}
}

func TestStart_Idempotent(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.StartWorkers()
	r.StartWorkers() // must not spawn duplicate workers or panic
	defer r.StopWorkers()

	fut := r.Submit(mkReq("c1", KindMetrics, PriorityLow, "x", func(_ context.Context) (any, error) { return 42, nil }))
	select {
	case res := <-fut:
		assert.Equal(t, 42, res.Value)
	case <-time.After(2 * time.Second):
		t.Fatal("Submit never returned after double-Start")
	}
}

func TestSetWorkersForTest_AppliesBeforeFirstSubmit(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.StartWorkers()
	defer r.StopWorkers()
	r.SetWorkersForTest(1, 0)

	// Single-worker config means only one Submit can run at a time.
	gate := make(chan struct{})
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	mkFn := func() func(context.Context) (any, error) {
		return func(ctx context.Context) (any, error) {
			c := concurrent.Add(1)
			for {
				m := maxConcurrent.Load()
				if c <= m || maxConcurrent.CompareAndSwap(m, c) {
					break
				}
			}
			<-gate
			concurrent.Add(-1)
			return nil, nil
		}
	}

	futs := make([]Future, 0, 4)
	for range 4 {
		futs = append(futs, r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "x", mkFn())))
	}
	close(gate)
	for _, f := range futs {
		select {
		case <-f:
		case <-time.After(2 * time.Second):
			t.Fatal("Future never delivered")
		}
	}
	assert.Equal(t, int32(1), maxConcurrent.Load(), "single-worker config must serialize execution")
}

func TestSubmit_CoalesceNewerWins(t *testing.T) {
	r := New(0)
	defer r.Close()
	// Don't StartWorkers — keep tasks in queue so we can observe coalesce.

	first := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "List Pods", noopFn))
	second := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "List Pods", noopFn))

	// First Future receives ErrCoalesced.
	select {
	case res := <-first:
		assert.ErrorIs(t, res.Err, ErrCoalesced)
	case <-time.After(time.Second):
		t.Fatal("first Future should have been coalesced")
	}

	// Second Future is still in the queue (no workers to deliver).
	select {
	case <-second:
		t.Fatal("second Future should not be delivered until a worker runs it")
	default:
	}

	assert.Equal(t, 1, r.QueueLen("c1"), "queue should hold exactly one task after coalesce")
}

func TestSubmit_CoalesceAcrossPriorityLanes(t *testing.T) {
	r := New(0)
	defer r.Close()

	// Same Sig but different Priority. The newer Submit's Priority is
	// the surviving entry (its Priority wins, older's lane is dropped).
	first := r.Submit(mkReq("c1", KindResourceList, PriorityLow, "List Pods", noopFn))
	second := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "List Pods", noopFn))

	select {
	case res := <-first:
		assert.ErrorIs(t, res.Err, ErrCoalesced, "older entry must be coalesced even across lanes")
	case <-time.After(time.Second):
		t.Fatal("first Future should have been coalesced")
	}
	_ = second
	assert.Equal(t, 0, r.QueueLenByPriority("c1", PriorityLow))
	assert.Equal(t, 1, r.QueueLenByPriority("c1", PriorityHigh))
}

func TestSubmit_DifferentSigDoesNotCoalesce(t *testing.T) {
	r := New(0)
	defer r.Close()

	a := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "List Pods", noopFn))
	b := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "List Secrets", noopFn))

	// Neither delivered (no workers).
	select {
	case <-a:
		t.Fatal("a should not be coalesced")
	case <-b:
		t.Fatal("b should not be coalesced")
	default:
	}

	assert.Equal(t, 2, r.QueueLen("c1"))
}

func TestSubmit_DifferentGenDoesNotCoalesce(t *testing.T) {
	r := New(0)
	defer r.Close()

	a := r.Submit(SubmitReq{KubeContext: "c1", Kind: KindResourceList, Priority: PriorityHigh, Target: "default", Gen: 1, Fn: noopFn})
	b := r.Submit(SubmitReq{KubeContext: "c1", Kind: KindResourceList, Priority: PriorityHigh, Target: "default", Gen: 2, Fn: noopFn})

	// Different Gen → different Sig → no coalesce.
	select {
	case <-a:
		t.Fatal("a should not be coalesced (different Gen)")
	case <-b:
		t.Fatal("b should not be coalesced (different Gen)")
	default:
	}

	assert.Equal(t, 2, r.QueueLen("c1"))
}

func TestSubmit_MutationsDoNotCoalesce(t *testing.T) {
	r := New(0)
	defer r.Close()

	// Same Sig but Kind == Mutation — must NOT coalesce.
	a := r.Submit(mkReq("c1", KindMutation, PriorityCritical, "delete pod foo", noopFn))
	b := r.Submit(mkReq("c1", KindMutation, PriorityCritical, "delete pod foo", noopFn))

	select {
	case <-a:
		t.Fatal("Mutation Sig should not coalesce")
	case <-b:
		t.Fatal("Mutation Sig should not coalesce")
	default:
	}

	assert.Equal(t, 2, r.QueueLen("c1"))
}
