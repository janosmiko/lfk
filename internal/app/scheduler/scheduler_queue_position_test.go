package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A task resubmitted on a timer keeps losing its place: sent to the back of the
// lane, it is coalesced again before a worker reaches it and its result never
// arrives. That is what froze the cluster dashboard in issue #646.
func TestSubmitCoalescedTaskKeepsQueuePosition(t *testing.T) {
	r := New(0)
	defer r.Close()
	r.SetWorkersForTest(1, 0)
	r.StartWorkers()
	defer r.StopWorkers()

	blockerStarted := make(chan struct{}, 1)
	release := make(chan struct{})

	order := make(chan string, 4)
	record := func(name string) func(context.Context) (any, error) {
		return func(context.Context) (any, error) {
			order <- name
			return nil, nil
		}
	}
	req := func(name string, fn func(context.Context) (any, error)) SubmitReq {
		return SubmitReq{
			KubeContext: "c1", Kind: KindDashboard, Priority: PriorityLow,
			Name: name, Target: "c1 / " + name, Gen: 1,
			Fn: fn, Timeout: 10 * time.Second,
		}
	}

	r.Submit(req("blocker", blockUntil(blockerStarted, release)))
	<-blockerStarted

	r.Submit(req("first", record("first")))
	r.Submit(req("second", record("second")))
	r.Submit(req("first", record("first")))
	require.Equal(t, 2, r.QueueLen("c1"), "test setup: only the two distinct tasks stay queued")

	close(release)

	select {
	case name := <-order:
		assert.Equal(t, "first", name,
			"the resubmitted task must run in the position its coalesced twin held")
	case <-time.After(2 * time.Second):
		t.Fatal("no queued task ran")
	}
}
