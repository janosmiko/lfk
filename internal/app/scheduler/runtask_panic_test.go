package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunTask_RecoversPanicInFn locks in the recover-and-Finish semantics
// the worker loop must apply when a task's Fn panics. Without the
// recovery, the worker goroutine dies mid-flight and the registry entry
// stays in r.tasks with FinishedAt zero — lenLocked counts it as a
// running task forever, so LenIndicator never drops to zero and the
// title-bar spinner spins endlessly. The future also stays unresolved,
// so the caller's scheduleK8sCall closure blocks forever on the channel
// receive. Both symptoms are user-visible (frozen spinner, leaked
// goroutine).
//
// The fix wraps Fn in a defer-recover that converts a panic to an error,
// then runs the normal cleanup (unregisterRunning, Finish, deliver to
// future, close future) so the worker survives and the registry stays
// consistent.
func TestRunTask_RecoversPanicInFn(t *testing.T) {
	r := New(0)
	r.SetWorkersForTest(2, 0)
	r.StartWorkers()
	defer r.StopWorkers()
	r.SetLingerDurationForTest(20 * time.Millisecond)

	future := r.Submit(SubmitReq{
		KubeContext: "c1",
		Kind:        KindResourceList,
		Priority:    PriorityHigh,
		Name:        "panicker",
		Target:      "tgt",
		Fn: func(ctx context.Context) (any, error) {
			panic("simulated client crash")
		},
	})

	// The future MUST resolve. Pre-fix, the worker dies from the
	// panic and this select hits the timeout (test detects the leak).
	select {
	case res := <-future:
		require.Error(t, res.Err, "panicking Fn must deliver a non-nil error to the future")
		assert.Contains(t, res.Err.Error(), "simulated client crash",
			"panic value should be surfaced in the error")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("future never resolved — worker goroutine presumably died from the panic and Finish was skipped")
	}

	// Past linger: the recovered task's registry entry must be Finished
	// and prunable, so LenIndicator drops to zero. Pre-fix the entry is
	// still "running" (FinishedAt zero) and lenLocked counts it forever.
	time.Sleep(40 * time.Millisecond)
	assert.Equal(t, 0, r.LenIndicator(),
		"panicking task must not leak a registry entry past its linger window")

	// Worker pool must still be functional: submit a second task and
	// confirm it runs. Pre-fix the worker that handled the panicker
	// goroutine has died, reducing the pool by one (and on a 1-worker
	// pool this would hang forever).
	future2 := r.Submit(SubmitReq{
		KubeContext: "c1",
		Kind:        KindResourceList,
		Priority:    PriorityHigh,
		Name:        "follower",
		Target:      "tgt",
		Fn: func(ctx context.Context) (any, error) {
			return "ok", nil
		},
	})
	select {
	case res := <-future2:
		assert.NoError(t, res.Err, "follow-up task must run on a surviving worker")
		assert.Equal(t, "ok", res.Value)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("follow-up task did not run — worker pool drained by the unrecovered panic")
	}
}
