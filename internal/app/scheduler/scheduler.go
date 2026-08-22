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
	Owner       uint64                             // UID of the tab that submitted the work
}

// Sig returns the coalesce signature for this submission.
func (r SubmitReq) Sig() Sig {
	return Sig{
		KubeContext: r.KubeContext,
		Kind:        r.Kind,
		Name:        r.Name,
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
// send even if the caller goroutine has already returned. This is
// defensive — in practice the caller always reads exactly once.
type Future <-chan Result

// Sentinel errors delivered via Result.Err.
var (
	// ErrCoalesced is delivered when a newer Submit with the same Sig
	// replaced this one in the queue. Caller's tea.Cmd should return nil
	// (the newer submission's Future is the one that matters).
	ErrCoalesced = errors.New("scheduler: coalesced by newer submission")

	// ErrContextSwitched is delivered when CancelContext drops this
	// task's context before it could run. Caller's tea.Cmd should return
	// nil (no UI update needed, the cluster context is gone).
	ErrContextSwitched = errors.New("scheduler: context switched")

	// ErrSuperseded is delivered when CancelStaleByGen drops or cancels a
	// Low-priority task whose Gen predates the caller's current requestGen.
	// The result would be discarded by the caller's gen check anyway, so
	// the tea.Cmd should return nil and free the worker for fresh work.
	ErrSuperseded = errors.New("scheduler: superseded by newer generation")
)

// queuedTask wraps a SubmitReq with its Future channel for delivery.
type queuedTask struct {
	req    SubmitReq
	future chan Result
}

// ctxQueue holds the three priority lanes for one cluster context plus
// a wake signal channel.
//
// skips and agingThreshold implement anti-starvation aging: skips[p] counts
// how many times lane p has been passed over while non-empty. Once it reaches
// agingThreshold the lane is promoted ahead of higher-priority lanes for one
// dequeue. Critical (lane 0) is never aged. agingThreshold == 0 disables aging
// (strict priority). See dequeueByPriorityLocked.
type ctxQueue struct {
	mu             sync.Mutex
	lanes          [3][]*queuedTask // indexed by Priority value
	skips          [3]int           // per-lane consecutive passed-over count
	agingThreshold int              // 0 = aging disabled (strict priority)
	wake           chan struct{}    // size 1, non-blocking signal
	stop           chan struct{}    // closes on context drop or Registry close
	poolStarted    bool
}

func newCtxQueue(agingThreshold int) *ctxQueue {
	return &ctxQueue{
		agingThreshold: agingThreshold,
		wake:           make(chan struct{}, 1),
		stop:           make(chan struct{}),
	}
}

// coalesceBySigLocked drops any queued task with a matching Sig and
// delivers ErrCoalesced to its Future. Caller must hold q.mu.
//
// Walks all priority lanes — coalesce is signature-based, not lane-
// based, so a Low task is correctly displaced by a same-Sig High
// resubmission (and vice versa).
//
// Returns the index in ownLane that the incoming task must take so it inherits
// its twin's queue position, or -1 when no twin sat in that lane. Appending
// instead starves a task that a timer resubmits: the dashboard re-issues its
// fan-out every watch interval, so a section sent to the back of the lane is
// coalesced away again before a worker ever reaches it (#646).
func (q *ctxQueue) coalesceBySigLocked(sig Sig, ownLane int) int {
	slot := -1
	for prio := range q.lanes {
		lane := q.lanes[prio]
		if len(lane) == 0 {
			continue
		}
		kept := lane[:0]
		for _, t := range lane {
			if sig.CoalescesWith(t.req.Sig()) {
				if prio == ownLane && slot < 0 {
					slot = len(kept)
				}
				t.future <- Result{Err: ErrCoalesced}
				close(t.future)
				continue
			}
			kept = append(kept, t)
		}
		q.lanes[prio] = kept
	}
	return slot
}

// staleByGen reports whether a queued/running req should be reclaimed by
// CancelStaleByGen(keepGen): a Low-priority read whose Gen predates keepGen.
// Gen == 0 (work submitted without a requestGen) and any non-Low priority
// (the user's current view, mutations) are never stale.
func staleByGen(req SubmitReq, keepGen uint64) bool {
	return req.Priority == PriorityLow &&
		req.Kind != KindMutation &&
		req.Gen != 0 &&
		req.Gen < keepGen
}

// dropStaleByGen removes queued tasks made stale by keepGen and delivers
// ErrSuperseded to their Futures. Acquires q.mu itself.
func (q *ctxQueue) dropStaleByGen(keepGen uint64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for prio := range q.lanes {
		lane := q.lanes[prio]
		if len(lane) == 0 {
			continue
		}
		kept := lane[:0]
		for _, t := range lane {
			if staleByGen(t.req, keepGen) {
				t.future <- Result{Err: ErrSuperseded}
				close(t.future)
				continue
			}
			kept = append(kept, t)
		}
		q.lanes[prio] = kept
	}
}

// enqueueAtLocked appends t to its priority lane and signals wake, or inserts
// it at slot when coalesceBySigLocked found a twin there whose position t
// inherits. Caller must hold q.mu.
func (q *ctxQueue) enqueueAtLocked(t *queuedTask, slot int) {
	prio := int(t.req.Priority)
	lane := q.lanes[prio]
	if slot < 0 || slot > len(lane) {
		lane = append(lane, t)
	} else {
		lane = append(lane, nil)
		copy(lane[slot+1:], lane[slot:])
		lane[slot] = t
	}
	q.lanes[prio] = lane
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

// dequeueByPriorityLocked removes and returns the next task to run, honoring
// priority with anti-starvation aging. Returns (nil, false) if all lanes are
// empty. Caller must hold q.mu.
//
// Critical is absolute: foundational gating work (API discovery, RBAC,
// namespaces) and destructive mutations must never be delayed. So it is
// served outright and is never aged. Among the non-Critical lanes the
// highest-priority non-empty lane wins by default. But a lower lane
// passed over more than agingThreshold times preempts it for a single
// dequeue. Without this, sustained High submissions on a slow cluster
// keep the High lane non-empty forever. Low work (security scans,
// dashboard, metrics) never runs (priority starvation). Aging bounds
// that wait to ~agingThreshold higher-priority dispatches.
// agingThreshold == 0 restores strict priority.
func (q *ctxQueue) dequeueByPriorityLocked() (*queuedTask, bool) {
	if lane := q.lanes[int(PriorityCritical)]; len(lane) > 0 {
		q.lanes[int(PriorityCritical)] = lane[1:]
		return lane[0], true
	}

	chosen := -1
	for prio := int(PriorityHigh); prio <= int(PriorityLow); prio++ {
		if len(q.lanes[prio]) == 0 {
			continue
		}
		if chosen == -1 {
			chosen = prio // strict-priority default: first non-empty lane
		}
		if q.agingThreshold > 0 && q.skips[prio] >= q.agingThreshold {
			chosen = prio // starved lane overrides the default for one pick
			break
		}
	}
	if chosen == -1 {
		return nil, false
	}

	if q.agingThreshold > 0 {
		// Reset the served lane and age every other non-empty non-Critical
		// lane that lost this round.
		for prio := int(PriorityHigh); prio <= int(PriorityLow); prio++ {
			switch {
			case prio == chosen:
				q.skips[prio] = 0
			case len(q.lanes[prio]) > 0:
				q.skips[prio]++
			}
		}
	}

	t := q.lanes[chosen][0]
	q.lanes[chosen] = q.lanes[chosen][1:]
	return t, true
}

// hasPendingWork reports whether any priority lane is non-empty. Used by
// the worker loop to decide whether to re-signal q.wake after a failed
// pickTask. If the queue is truly empty we must NOT re-signal. Otherwise
// a stale wake left over from a previous drain creates a hot loop. In
// that loop the same worker repeatedly receives and resends the signal
// without parking. This was issue #206 — observed as 2.26M goroutine
// block/unblock events per second. Caller does not need to hold q.mu.
// The read is racy but only used as a hint. The worst case is one
// extra empty wake, which the workers tolerate by parking again.
func (q *ctxQueue) hasPendingWork() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for prio := range q.lanes {
		if len(q.lanes[prio]) > 0 {
			return true
		}
	}
	return false
}

// Submit queues a unit of K8s work. Returns a buffered Future. If the
// Registry is nil or has been closed, the Future immediately receives
// Result{Err: ErrContextSwitched}.
//
// No workers are spawned by this commit. Submitted tasks accumulate in
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
		threshold := DefaultAgingThreshold
		if r.cfg != nil {
			threshold = r.cfg.AgingThreshold
		}
		q = newCtxQueue(threshold)
		r.ctxQueues[req.KubeContext] = q
	}
	r.mu.Unlock()

	t := &queuedTask{req: req, future: fut}
	sig := req.Sig()

	// Coalesce against work already IN FLIGHT, not just queued. Without this,
	// a slow fetch (e.g. Pod metrics on a sluggish cluster) that is still
	// running cannot absorb an identical resubmission. The duplicate queues
	// behind it and runs a redundant API call when the worker frees up. The
	// queue-only coalesce below never sees the running task because it lives
	// in runningTasks, not q.lanes. Checked before enqueue so the caller's
	// Future resolves immediately with ErrCoalesced (its tea.Cmd returns nil,
	// because the in-flight task's result is the one that matters). Mutations
	// opt out via NeverCoalesce — a second write must always run.
	if !sig.NeverCoalesce() && r.coalescesWithRunning(req.KubeContext, sig) {
		fut <- Result{Err: ErrCoalesced}
		close(fut)
		return fut
	}

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
	slot := -1
	if !sig.NeverCoalesce() {
		slot = q.coalesceBySigLocked(sig, int(req.Priority))
	}
	q.enqueueAtLocked(t, slot)
	q.mu.Unlock()

	r.mu.Lock()
	r.ensurePoolFor(req.KubeContext, q)
	r.mu.Unlock()

	r.pokePreempt(req.KubeContext, req.Priority)

	return fut
}

// coalescesWithRunning reports whether an in-flight task on kctx has a Sig that
// the incoming sig coalesces with. A task already being torn down (preempted,
// superseded, or context-switched) is skipped. It is about to free its slot
// and deliver a sentinel, so coalescing onto it would drop the fresh request.
func (r *Registry) coalescesWithRunning(kctx string, sig Sig) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rt := range r.runningTasks[kctx] {
		if rt.preempted.Load() || rt.superseded.Load() || rt.contextSwitched.Load() {
			continue
		}
		if sig.CoalescesWith(rt.task.req.Sig()) {
			return true
		}
	}
	return false
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
