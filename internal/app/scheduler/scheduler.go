package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"
)

// SubmitReq describes a unit of K8s work to dispatch via Registry.Submit.
//
// Fn receives a context that is cancelled if the task is preempted by
// higher-priority work or its KubeContext is dropped via CancelContext.
// Fn must respect the context — long-running calls that ignore Done() will
// hold the worker even after preemption is signalled.
type SubmitReq struct {
	KubeContext string                             // routes to per-context queue
	Kind        Kind                               // existing classification
	Priority    Priority                           // Critical / High / Low
	Name        string                             // human label, e.g. "List Pods"
	Target      string                             // e.g. "default / web-7d8c"
	Fn          func(context.Context) (any, error) // the actual K8s call
	Timeout     time.Duration                      // 0 = use config-default for Kind
	SilentTrack bool                               // mirrors existing suppressBgtasks
	Gen         uint64                             // caller's requestGen for Sig
}

// Sig returns the coalesce signature for this submission.
func (r SubmitReq) Sig() Sig {
	return Sig{
		KubeContext: r.KubeContext,
		Kind:        r.Kind,
		Target:      r.Target,
		Gen:         r.Gen,
	}
}

// Result is the value or error delivered to a Future.
type Result struct {
	Value any
	Err   error
}

// Future is a buffered (size 1) channel the caller awaits inside its
// tea.Cmd goroutine. The buffer guarantees the worker never blocks on
// send even if the caller goroutine has already returned (defensive —
// in practice the caller always reads exactly once).
type Future <-chan Result

// Sentinel errors delivered via Result.Err.
var (
	// ErrCoalesced is delivered when a newer Submit with the same Sig
	// replaced this one in the queue. Caller's tea.Cmd should return nil
	// (the newer submission's Future is the one that matters).
	ErrCoalesced = errors.New("scheduler: coalesced by newer submission")

	// ErrContextSwitched is delivered when CancelContext drops this
	// task's context before it could run. Caller's tea.Cmd should return
	// nil (no UI update needed; the cluster context is gone).
	ErrContextSwitched = errors.New("scheduler: context switched")
)

// queuedTask wraps a SubmitReq with its Future channel for delivery.
type queuedTask struct {
	req    SubmitReq
	future chan Result
}

// ctxQueue holds the three priority lanes for one cluster context plus
// a wake signal channel.
type ctxQueue struct {
	mu          sync.Mutex
	lanes       [3][]*queuedTask // indexed by Priority value
	wake        chan struct{}    // size 1, non-blocking signal
	stop        chan struct{}    // closes on context drop or Registry close
	poolStarted bool
}

func newCtxQueue() *ctxQueue {
	return &ctxQueue{
		wake: make(chan struct{}, 1),
		stop: make(chan struct{}),
	}
}

// coalesceBySigLocked drops any queued task with a matching Sig and
// delivers ErrCoalesced to its Future. Caller must hold q.mu.
//
// Walks all priority lanes — coalesce is signature-based, not lane-
// based, so a Low task is correctly displaced by a same-Sig High
// resubmission (and vice versa).
func (q *ctxQueue) coalesceBySigLocked(sig Sig) {
	for prio := range q.lanes {
		lane := q.lanes[prio]
		if len(lane) == 0 {
			continue
		}
		kept := lane[:0]
		for _, t := range lane {
			if t.req.Sig() == sig {
				t.future <- Result{Err: ErrCoalesced}
				close(t.future)
				continue
			}
			kept = append(kept, t)
		}
		q.lanes[prio] = kept
	}
}

// enqueueLocked appends to the priority lane and signals wake. Caller
// must hold q.mu.
func (q *ctxQueue) enqueueLocked(t *queuedTask) {
	prio := int(t.req.Priority)
	q.lanes[prio] = append(q.lanes[prio], t)
	select {
	case q.wake <- struct{}{}:
	default: // already signaled
	}
}

// drain delivers err to every queued task's Future and empties all lanes.
// Idempotent (safe to call multiple times). Closes the stop channel so
// any worker goroutine waiting on it exits.
func (q *ctxQueue) drain(err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for prio := range q.lanes {
		for _, t := range q.lanes[prio] {
			t.future <- Result{Err: err}
			close(t.future)
		}
		q.lanes[prio] = nil
	}
	select {
	case <-q.stop:
		// already closed
	default:
		close(q.stop)
	}
}

// dequeueByPriorityLocked removes and returns the head of the highest-
// priority non-empty lane. Returns (nil, false) if all lanes are empty.
// Caller must hold q.mu.
func (q *ctxQueue) dequeueByPriorityLocked() (*queuedTask, bool) {
	for prio := PriorityCritical; prio <= PriorityLow; prio++ {
		lane := q.lanes[int(prio)]
		if len(lane) > 0 {
			t := lane[0]
			q.lanes[int(prio)] = lane[1:]
			return t, true
		}
	}
	return nil, false
}

// Submit queues a unit of K8s work. Returns a buffered Future. If the
// Registry is nil or has been closed, the Future immediately receives
// Result{Err: ErrContextSwitched}.
//
// No workers are spawned by this commit; submitted tasks accumulate in
// their priority lane until a later commit adds the worker pool.
func (r *Registry) Submit(req SubmitReq) Future {
	fut := make(chan Result, 1)
	if r == nil {
		fut <- Result{Err: ErrContextSwitched}
		close(fut)
		return fut
	}
	r.mu.Lock()
	if r.ctxQueues == nil {
		r.mu.Unlock()
		fut <- Result{Err: ErrContextSwitched}
		close(fut)
		return fut
	}
	q, ok := r.ctxQueues[req.KubeContext]
	if !ok {
		q = newCtxQueue()
		r.ctxQueues[req.KubeContext] = q
	}
	r.mu.Unlock()

	t := &queuedTask{req: req, future: fut}
	sig := req.Sig()
	q.mu.Lock()
	// Re-check after taking q.mu: Close() / CancelContext() can drain
	// the queue in the window between r.mu.Unlock above and this lock.
	// Without this guard the task would land on a detached queue and
	// its Future would never resolve.
	select {
	case <-q.stop:
		q.mu.Unlock()
		fut <- Result{Err: ErrContextSwitched}
		close(fut)
		return fut
	default:
	}
	if !sig.NeverCoalesce() {
		q.coalesceBySigLocked(sig)
	}
	q.enqueueLocked(t)
	q.mu.Unlock()

	r.mu.Lock()
	r.ensurePoolFor(req.KubeContext, q)
	r.mu.Unlock()

	r.pokePreempt(req.KubeContext, req.Priority)

	return fut
}

// QueueLen returns the number of queued (not in-flight) tasks for kctx.
// A nil receiver or unknown context returns 0.
func (r *Registry) QueueLen(kctx string) int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	q := r.ctxQueues[kctx]
	r.mu.Unlock()
	if q == nil {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, lane := range q.lanes {
		n += len(lane)
	}
	return n
}

// QueueLenByPriority returns the queued (not in-flight) count for one
// priority lane on kctx. A nil receiver or unknown context returns 0.
func (r *Registry) QueueLenByPriority(kctx string, prio Priority) int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	q := r.ctxQueues[kctx]
	r.mu.Unlock()
	if q == nil {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	idx := int(prio)
	if idx < 0 || idx >= len(q.lanes) {
		return 0
	}
	return len(q.lanes[idx])
}
