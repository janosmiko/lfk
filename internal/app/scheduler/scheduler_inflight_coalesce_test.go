package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockUntil returns a Fn that signals it has started, then blocks until
// release is closed (or ctx is cancelled). Used to hold a task "in flight"
// while a second identical submission races against it.
func blockUntil(started chan<- struct{}, release <-chan struct{}) func(context.Context) (any, error) {
	return func(ctx context.Context) (any, error) {
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil, nil
	}
}

// TestSubmit_CoalescesAgainstRunningTask is the core fix for the metrics
// pile-up: a submission identical (same Sig) to a task already in flight must
// be dropped with ErrCoalesced rather than queued behind it.
func TestSubmit_CoalescesAgainstRunningTask(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.SetWorkersForTest(1, 0)
	r.StartWorkers()
	defer r.StopWorkers()

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	defer close(release)

	req := SubmitReq{
		KubeContext: "c1", Kind: KindMetrics, Priority: PriorityLow,
		Name: "Metrics: Pod/x", Target: "c1 / ns", Gen: 1,
		Fn: blockUntil(started, release), Timeout: 10 * time.Second,
	}
	r.Submit(req)
	<-started // first task is now running

	// Identical second submission while the first is in flight.
	fut := r.Submit(req)
	select {
	case res := <-fut:
		assert.ErrorIs(t, res.Err, ErrCoalesced, "identical in-flight submission must be coalesced")
	case <-time.After(2 * time.Second):
		t.Fatal("second submission neither ran nor coalesced")
	}
	assert.Equal(t, 0, r.QueueLen("c1"), "coalesced submission must not be enqueued behind the running one")
}

// TestSubmit_DoesNotCoalesceAcrossGen verifies a submission with a different
// Gen (i.e. the user navigated) is NOT coalesced against the running task — it
// represents a fresh logical request and must run.
func TestSubmit_DoesNotCoalesceAcrossGen(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.SetWorkersForTest(1, 0)
	r.StartWorkers()
	defer r.StopWorkers()

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	defer close(release)

	base := SubmitReq{
		KubeContext: "c1", Kind: KindMetrics, Priority: PriorityLow,
		Name: "Metrics: Pod/x", Target: "c1 / ns", Gen: 1,
		Fn: blockUntil(started, release), Timeout: 10 * time.Second,
	}
	r.Submit(base)
	<-started

	newer := base
	newer.Gen = 2
	newer.Fn = func(context.Context) (any, error) { return nil, nil }
	r.Submit(newer)
	// Different gen => enqueued behind the running task, not coalesced.
	assert.Equal(t, 1, r.QueueLen("c1"), "different-gen submission must enqueue, not coalesce")
}

// TestSubmit_MutationNeverCoalescesAgainstRunning verifies write operations are
// never coalesced (NeverCoalesce), even against an identical in-flight one.
func TestSubmit_MutationNeverCoalescesAgainstRunning(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.SetWorkersForTest(1, 0)
	r.StartWorkers()
	defer r.StopWorkers()

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	defer close(release)

	req := SubmitReq{
		KubeContext: "c1", Kind: KindMutation, Priority: PriorityCritical,
		Name: "Delete Pod/x", Target: "c1 / ns", Gen: 1,
		Fn: blockUntil(started, release), Timeout: 10 * time.Second,
	}
	r.Submit(req)
	<-started

	dup := req
	dup.Fn = func(context.Context) (any, error) { return nil, nil }
	r.Submit(dup)
	require.Equal(t, 1, r.QueueLen("c1"), "a second mutation must always enqueue (never coalesce)")
}
