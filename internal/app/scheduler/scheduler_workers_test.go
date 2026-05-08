package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerDispatch_PriorityOrder(t *testing.T) {
	r := New(0)
	defer r.Close()
	// Single worker, no Critical reservation, so order is fully
	// determined by what's in the queue when the worker pulls.
	r.SetWorkersForTest(1, 0)

	gate := make(chan struct{})
	var mu atomicSlice

	rec := func(p Priority) func(ctx context.Context) (any, error) {
		return func(ctx context.Context) (any, error) {
			mu.Append(p)
			<-gate
			return nil, nil
		}
	}

	// Queue ALL THREE before any worker runs so the dispatcher sees
	// the full queue on its first pick. Calling StartWorkers after
	// Submit makes the test a pure dequeue-order check — without this,
	// the first task can begin running before the others are queued
	// and a later High/Critical Submit goes through preempt-and-requeue
	// instead of the priority lane on the first dispatch.
	low := r.Submit(mkReq("c1", KindDashboard, PriorityLow, "low", rec(PriorityLow)))
	high := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "high", rec(PriorityHigh)))
	crit := r.Submit(mkReq("c1", KindAPIDiscovery, PriorityCritical, "crit", rec(PriorityCritical)))

	r.StartWorkers()
	defer r.StopWorkers()

	close(gate)
	<-low
	<-high
	<-crit

	got := mu.Snapshot()
	require.Len(t, got, 3)
	assert.Equal(t, []Priority{PriorityCritical, PriorityHigh, PriorityLow}, got,
		"with all three queued before workers start, dispatch must be Critical → High → Low")
}

func TestWorkerDispatch_CriticalReservedSlotRunsImmediately(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.StartWorkers()
	defer r.StopWorkers()
	// 2 workers, 1 reserved for Critical: so 1 Critical-only + 1 general worker.
	r.SetWorkersForTest(2, 1)

	// Block the general worker on a Low task.
	lowGate := make(chan struct{})
	var lowStarted atomic.Bool
	r.Submit(mkReq("c1", KindDashboard, PriorityLow, "blocking-low", func(ctx context.Context) (any, error) {
		lowStarted.Store(true)
		<-lowGate
		return nil, nil
	}))
	// Wait for the Low to start.
	deadline := time.Now().Add(time.Second)
	for !lowStarted.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	require.True(t, lowStarted.Load(), "Low task should have started on the general worker")

	// Now Submit Critical: it must run on the reserved worker without
	// waiting for Low to finish.
	critDone := make(chan struct{})
	fut := r.Submit(mkReq("c1", KindAPIDiscovery, PriorityCritical, "crit", func(_ context.Context) (any, error) {
		close(critDone)
		return "crit-ok", nil
	}))

	select {
	case <-critDone:
	case <-time.After(2 * time.Second):
		close(lowGate)
		t.Fatal("Critical did not run on reserved worker while Low was blocking the general worker")
	}
	res := <-fut
	assert.NoError(t, res.Err)
	assert.Equal(t, "crit-ok", res.Value)

	close(lowGate)
}

// atomicSlice is a small helper for tests that record values from
// concurrent goroutines.
type atomicSlice struct {
	mu  sync.Mutex
	out []Priority
}

func (s *atomicSlice) Append(p Priority) {
	s.mu.Lock()
	s.out = append(s.out, p)
	s.mu.Unlock()
}

func (s *atomicSlice) Snapshot() []Priority {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := make([]Priority, len(s.out))
	copy(c, s.out)
	return c
}

func TestPreempt_LowYieldsToHigh(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.StartWorkers()
	defer r.StopWorkers()
	r.SetWorkersForTest(1, 0) // single worker, no Critical reservation

	// Low task: blocks on ctx until preempted, then completes second time.
	lowAttempts := atomic.Int32{}
	lowFinished := make(chan struct{})
	lowFn := func(ctx context.Context) (any, error) {
		attempt := lowAttempts.Add(1)
		if attempt == 1 {
			// First attempt: block until ctx cancelled (preempt).
			<-ctx.Done()
			return nil, ctx.Err()
		}
		// Second attempt (after requeue): finish.
		close(lowFinished)
		return "low-finished", nil
	}
	low := r.Submit(mkReq("c1", KindDashboard, PriorityLow, "low task", lowFn))

	// Wait for first attempt to start.
	require.Eventually(t, func() bool {
		return lowAttempts.Load() >= 1
	}, time.Second, 5*time.Millisecond, "Low task should start running")

	// Submit High — must preempt Low.
	highStarted := make(chan struct{})
	high := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "high task", func(ctx context.Context) (any, error) {
		close(highStarted)
		return "high-finished", nil
	}))

	select {
	case <-highStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("High task never ran (preempt failed)")
	}
	highRes := <-high
	assert.NoError(t, highRes.Err)
	assert.Equal(t, "high-finished", highRes.Value)

	// Low should now restart and complete.
	select {
	case <-lowFinished:
	case <-time.After(2 * time.Second):
		t.Fatal("Low never restarted after preemption")
	}
	res := <-low
	assert.NoError(t, res.Err, "Low must complete after High finishes")
	assert.Equal(t, "low-finished", res.Value)
	assert.GreaterOrEqual(t, lowAttempts.Load(), int32(2),
		"Low must have been started at least twice (preempted, requeued, ran again)")
}

func TestPreempt_DoesNotPreemptCritical(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.StartWorkers()
	defer r.StopWorkers()
	r.SetWorkersForTest(1, 0)

	critGate := make(chan struct{})
	critAttempts := atomic.Int32{}
	critFn := func(ctx context.Context) (any, error) {
		critAttempts.Add(1)
		<-critGate
		return "crit-finished", nil
	}
	crit := r.Submit(mkReq("c1", KindAPIDiscovery, PriorityCritical, "crit", critFn))

	// Wait for Crit to start.
	require.Eventually(t, func() bool {
		return critAttempts.Load() >= 1
	}, time.Second, 5*time.Millisecond)

	// Submit High — must NOT preempt Critical.
	high := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "high", func(_ context.Context) (any, error) {
		return "high", nil
	}))

	// Give the system a generous window to potentially mis-preempt.
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(1), critAttempts.Load(), "Critical must not have been preempted (only one attempt)")

	close(critGate)
	<-crit
	<-high
}

func TestPreempt_HighYieldsToCritical(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.StartWorkers()
	defer r.StopWorkers()
	r.SetWorkersForTest(1, 0)

	highAttempts := atomic.Int32{}
	highFinished := make(chan struct{})
	highFn := func(ctx context.Context) (any, error) {
		attempt := highAttempts.Add(1)
		if attempt == 1 {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		close(highFinished)
		return "high-finished", nil
	}
	high := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "high task", highFn))

	require.Eventually(t, func() bool { return highAttempts.Load() >= 1 }, time.Second, 5*time.Millisecond)

	critStarted := make(chan struct{})
	crit := r.Submit(mkReq("c1", KindAPIDiscovery, PriorityCritical, "crit", func(_ context.Context) (any, error) {
		close(critStarted)
		return "crit-finished", nil
	}))

	select {
	case <-critStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("Critical did not preempt High")
	}
	<-crit

	select {
	case <-highFinished:
	case <-time.After(2 * time.Second):
		t.Fatal("High never restarted after preemption by Critical")
	}
	<-high
}

func TestCancelContext_DropsQueuedAndCancelsInFlight(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.StartWorkers()
	defer r.StopWorkers()
	r.SetWorkersForTest(1, 0)

	// One running task that respects ctx cancellation.
	running := make(chan struct{})
	gate := make(chan struct{})
	runFut := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "running", func(ctx context.Context) (any, error) {
		close(running)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-gate:
			return nil, nil
		}
	}))
	<-running

	// One queued task we'll drop.
	queuedFut := r.Submit(mkReq("c1", KindResourceList, PriorityHigh, "queued", noopFn))

	// Sanity: queued task is still queued (worker is busy on running).
	assert.Equal(t, 1, r.QueueLen("c1"))

	r.CancelContext("c1")

	// Both Futures must receive ErrContextSwitched.
	select {
	case res := <-runFut:
		assert.ErrorIs(t, res.Err, ErrContextSwitched)
	case <-time.After(time.Second):
		t.Fatal("running Future not delivered after CancelContext")
	}
	select {
	case res := <-queuedFut:
		assert.ErrorIs(t, res.Err, ErrContextSwitched)
	case <-time.After(time.Second):
		t.Fatal("queued Future not delivered after CancelContext")
	}

	assert.Equal(t, 0, r.QueueLen("c1"))
	close(gate) // cleanup
}

func TestCancelContext_LeavesOtherContextsAlone(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.StartWorkers()
	defer r.StopWorkers()
	r.SetWorkersForTest(1, 0)

	other := r.Submit(mkReq("c2", KindResourceList, PriorityHigh, "in c2", func(_ context.Context) (any, error) {
		return "ok", nil
	}))

	r.CancelContext("c1")

	select {
	case res := <-other:
		assert.NoError(t, res.Err)
		assert.Equal(t, "ok", res.Value)
	case <-time.After(time.Second):
		t.Fatal("c2 Future should still be delivered")
	}
}

func TestCancelContext_NoOpOnUnknownContext(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.StartWorkers()
	defer r.StopWorkers()

	// Calling CancelContext on a never-used kctx must not panic.
	r.CancelContext("never-existed")
	// And subsequent Submits to that kctx still work fine.
	fut := r.Submit(mkReq("never-existed", KindResourceList, PriorityHigh, "x", func(_ context.Context) (any, error) {
		return "ok", nil
	}))
	select {
	case res := <-fut:
		assert.NoError(t, res.Err)
	case <-time.After(time.Second):
		t.Fatal("Submit after no-op CancelContext should still work")
	}
}

// TestRunTask_PopulatesVisibility verifies that scheduler-routed tasks
// flow through the registry's Start/Finish visibility surface so the
// :scheduler overlay can show them in its history. Without this,
// scheduleK8sCall submissions are invisible to Snapshot() and
// SnapshotCompleted().
func TestRunTask_PopulatesVisibility(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.StartWorkers()
	defer r.StopWorkers()
	r.SetWorkersForTest(1, 0)
	// Tight linger window so the test can observe expulsion from the
	// running list without sleeping for the production 10s default.
	r.SetLingerDurationForTest(20 * time.Millisecond)

	gate := make(chan struct{})
	released := make(chan struct{})
	req := mkReq("c1", KindResourceList, PriorityHigh, "default", func(_ context.Context) (any, error) {
		close(gate)
		<-released
		return "ok", nil
	})
	req.Name = "List Secrets"
	fut := r.Submit(req)

	// Wait for the task to start so it is in the running snapshot.
	<-gate
	require.Eventually(t, func() bool {
		snap := r.Snapshot()
		for _, task := range snap {
			if task.Name == "List Secrets" && !task.IsFinished() {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond, "running task must appear in Snapshot")

	// Let the task complete and verify it lands in the completed history.
	close(released)
	<-fut
	require.Eventually(t, func() bool {
		for _, c := range r.SnapshotCompleted() {
			if c.Name == "List Secrets" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond, "finished task must appear in SnapshotCompleted")

	// During the linger window the task is still in the Running list
	// (now flagged Finished) — see DefaultLingerDuration. After the
	// linger expires it must drop out.
	require.Eventually(t, func() bool {
		for _, task := range r.Snapshot() {
			if task.Name == "List Secrets" {
				return false
			}
		}
		return true
	}, time.Second, 5*time.Millisecond, "finished task must leave Running once linger expires")
}

// TestRunTask_SilentTrackAppearsInHistoryButNotIndicator verifies that
// submissions flagged SilentTrack:true DO populate the visibility
// surface (Snapshot/SnapshotCompleted) so they show up in the
// :scheduler overlay history, but are excluded from LenIndicator()
// so the title-bar spinner doesn't flicker every second on watch-mode.
func TestRunTask_SilentTrackAppearsInHistoryButNotIndicator(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.StartWorkers()
	defer r.StopWorkers()

	gate := make(chan struct{})
	released := make(chan struct{})
	req := mkReq("c1", KindResourceList, PriorityHigh, "default", func(_ context.Context) (any, error) {
		close(gate)
		<-released
		return "ok", nil
	})
	req.Name = "Watch Refresh"
	req.SilentTrack = true
	fut := r.Submit(req)
	<-gate

	// While running: the task IS in Snapshot (with Silent=true), so the
	// :scheduler overlay shows it. But LenIndicator() filters it out so
	// the title-bar spinner stays quiet.
	require.Eventually(t, func() bool {
		for _, task := range r.Snapshot() {
			if task.Name == "Watch Refresh" {
				return task.Silent
			}
		}
		return false
	}, time.Second, 5*time.Millisecond, "silent task must appear in Snapshot with Silent=true")
	assert.Equal(t, 0, r.LenIndicator(), "silent tasks must not count toward the title-bar indicator")
	assert.Equal(t, 1, r.Len(), "Len() returns the full count (overlay-facing)")

	close(released)
	<-fut

	// After completion the task is in the history.
	require.Eventually(t, func() bool {
		for _, c := range r.SnapshotCompleted() {
			if c.Name == "Watch Refresh" {
				return true
			}
		}
		return false
	}, time.Second, 5*time.Millisecond, "silent task must appear in SnapshotCompleted so the user can see watch-tick history")
}
