package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dashReq builds a Dashboard-section submission with an explicit Name and Gen
// so the gen-agnostic coalesce path can be exercised directly.
func dashReq(kctx, name, target string, gen uint64) SubmitReq {
	return SubmitReq{
		KubeContext: kctx,
		Kind:        KindDashboard,
		Priority:    PriorityLow,
		Name:        name,
		Target:      target,
		Gen:         gen,
		Fn:          noopFn,
		Timeout:     time.Second,
	}
}

// TestSubmit_DashboardCoalescesAcrossGen is the fix for the stale-dashboard
// pile-up: a fresh dashboard batch (newer Gen) must displace the queued
// same-section task from an older Gen instead of stacking a second copy
// behind it. Without this, every cursor move / watch tick that bumps
// requestGen left another full 6-task batch in the Low lane.
func TestSubmit_DashboardCoalescesAcrossGen(t *testing.T) {
	r := New(0)
	defer r.Close()

	old := r.Submit(dashReq("c1", "Dashboard: metrics", "c1#metrics", 1))
	fresh := r.Submit(dashReq("c1", "Dashboard: metrics", "c1#metrics", 2))

	select {
	case res := <-old:
		assert.ErrorIs(t, res.Err, ErrCoalesced, "older-gen dashboard section must coalesce into the newer one")
	case <-time.After(time.Second):
		t.Fatal("older dashboard future should have been coalesced")
	}
	_ = fresh
	assert.Equal(t, 1, r.QueueLen("c1"), "only the freshest dashboard section should remain queued")
}

// TestSubmit_DashboardDifferentSectionDoesNotCoalesce guards against the
// gen-agnostic rule collapsing distinct sections into one — the six sections
// share Kind/Target/Gen but differ by Name and must all survive.
func TestSubmit_DashboardDifferentSectionDoesNotCoalesce(t *testing.T) {
	r := New(0)
	defer r.Close()

	r.Submit(dashReq("c1", "Dashboard: pods", "c1#pods", 1))
	r.Submit(dashReq("c1", "Dashboard: nodes", "c1#nodes", 1))

	assert.Equal(t, 2, r.QueueLen("c1"), "distinct dashboard sections must not coalesce")
}

// TestCancelStaleByGen_DropsQueuedLowOlderGen verifies the reclaim path:
// queued Low-priority tasks from an older generation are dropped (future gets
// ErrSuperseded), while the current generation and any High-priority work the
// user is actively waiting for are left untouched.
func TestCancelStaleByGen_DropsQueuedLowOlderGen(t *testing.T) {
	r := New(0)
	defer r.Close()

	keep := r.Submit(dashReq("c1", "Dashboard: pods", "c1#pods", 2))  // current gen
	stale := r.Submit(dashReq("c1", "Dashboard: metrics", "c1#m", 1)) // older gen, Low
	high := r.Submit(SubmitReq{KubeContext: "c1", Kind: KindResourceList, Priority: PriorityHigh, Name: "List Events", Target: "c1", Gen: 1, Fn: noopFn})

	r.CancelStaleByGen("c1", 2)

	select {
	case res := <-stale:
		assert.ErrorIs(t, res.Err, ErrSuperseded, "older-gen Low task must be superseded")
	case <-time.After(time.Second):
		t.Fatal("stale future should have been delivered ErrSuperseded")
	}

	// keep (current gen) and high (older gen but High) both survive.
	select {
	case <-keep:
		t.Fatal("current-gen task must not be cancelled")
	case <-high:
		t.Fatal("High-priority task must never be cancelled by gen reclaim")
	default:
	}
	assert.Equal(t, 2, r.QueueLen("c1"))
}

// TestCancelStaleByGen_KeepsGenZero ensures tasks submitted without a
// requestGen (Gen == 0, e.g. non-navigational background work) are never
// treated as stale.
func TestCancelStaleByGen_KeepsGenZero(t *testing.T) {
	r := New(0)
	defer r.Close()

	r.Submit(dashReq("c1", "Dashboard: pods", "c1#pods", 0))
	r.CancelStaleByGen("c1", 5)
	assert.Equal(t, 1, r.QueueLen("c1"), "Gen==0 tasks are not subject to gen reclaim")
}

// TestCancelStaleByGen_CancelsInflightLow verifies a running Low task from an
// older gen is cancelled via its context and its Future delivers ErrSuperseded
// rather than being requeued (the preempt path) or delivering a stale value.
func TestCancelStaleByGen_CancelsInflightLow(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.StartWorkers()
	defer r.StopWorkers()
	r.SetWorkersForTest(1, 0)

	started := make(chan struct{})
	fut := r.Submit(SubmitReq{
		KubeContext: "c1", Kind: KindDashboard, Priority: PriorityLow,
		Name: "Dashboard: pods", Target: "c1#pods", Gen: 1,
		Fn: func(ctx context.Context) (any, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("task never started")
	}

	r.CancelStaleByGen("c1", 2)

	select {
	case res := <-fut:
		require.ErrorIs(t, res.Err, ErrSuperseded, "in-flight stale Low task must resolve as superseded")
	case <-time.After(2 * time.Second):
		t.Fatal("superseded in-flight future never delivered")
	}
}

// TestCancelStaleByGen_NilAndUnknown is a no-op safety check.
func TestCancelStaleByGen_NilAndUnknown(t *testing.T) {
	var nilReg *Registry
	require.NotPanics(t, func() { nilReg.CancelStaleByGen("c1", 1) })

	r := New(0)
	defer r.Close()
	require.NotPanics(t, func() { r.CancelStaleByGen("missing", 1) })
}
