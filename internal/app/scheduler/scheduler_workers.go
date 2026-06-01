package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
)

// workerClass selects which priority lane a worker prefers when picking the
// next task. General workers honor strict priority (with aging); the reserved
// classes bias toward one end of the range but always fall back so a reserved
// slot is never wasted when its preferred lane is empty.
type workerClass int

const (
	workerClassGeneral  workerClass = iota // strict priority + aging
	workerClassCritical                    // prefer Critical, else fall back
	workerClassLow                         // prefer Low, else fall back
)

// StartWorkers enables worker dispatch. Idempotent. Workers are spawned
// per cluster context on first Submit AND retroactively for any
// queues that already exist when StartWorkers is called — this lets
// tests Submit before starting workers to set up a deterministic
// dequeue scenario.
//
// In production, StartWorkers is called once at app init right after New().
// Tests that don't exercise dispatch can skip StartWorkers; only Submit's
// queueing surface is needed.
func (r *Registry) StartWorkers() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return
	}
	r.started = true
	// Retroactively spawn pools for any queues created via Submit
	// before StartWorkers was called. ensurePoolFor itself short-
	// circuits when q.poolStarted is true, so this is safe to call on
	// every queue regardless of state.
	for kctx, q := range r.ctxQueues {
		r.ensurePoolFor(kctx, q)
	}
	r.mu.Unlock()
}

// StopWorkers signals all workers to exit and waits for them. Idempotent.
// In-flight Fns continue but their Futures receive nothing further;
// Close (or CancelContext, in a later commit) is what drains pending
// Futures with ErrContextSwitched.
func (r *Registry) StopWorkers() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return
	}
	r.started = false
	queues := make([]*ctxQueue, 0, len(r.ctxQueues))
	for _, q := range r.ctxQueues {
		queues = append(queues, q)
	}
	r.mu.Unlock()
	for _, q := range queues {
		select {
		case <-q.stop:
		default:
			close(q.stop)
		}
	}
	r.workersWG.Wait()
}

// SetWorkersForTest overrides the configured worker count for a Registry
// before any pool is spawned. Tests use this to make dispatch
// deterministic. Production code MUST NOT call this.
func (r *Registry) SetWorkersForTest(workers, criticalReserved int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg.WorkersPerContext = ClampWorkers(workers)
	r.cfg.CriticalReserved = ClampCriticalReserved(criticalReserved, r.cfg.WorkersPerContext)
	// Re-clamp the existing low reservation against the new totals so an
	// earlier SetLowReservedForTest stays valid (and the default doesn't
	// exceed a small test pool). Tests that want a specific value call
	// SetLowReservedForTest after this.
	r.cfg.LowReserved = ClampLowReserved(r.cfg.LowReserved, r.cfg.WorkersPerContext, r.cfg.CriticalReserved)
}

// SetLowReservedForTest overrides the low-reserved worker count before any
// pool is spawned. Clamped against the current worker/critical totals.
// Production code MUST NOT call this; use the ConfigLowReserved global.
func (r *Registry) SetLowReservedForTest(n int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg.LowReserved = ClampLowReserved(n, r.cfg.WorkersPerContext, r.cfg.CriticalReserved)
}

// SetAgingThresholdForTest overrides the anti-starvation aging threshold on a
// Registry before any per-context queue is created (queues capture it lazily on
// first Submit). The value is used verbatim — including 0 to disable aging — so
// tests can exercise both small thresholds and the strict-priority kill switch.
// Production code MUST NOT call this; use the ConfigAgingThreshold global.
func (r *Registry) SetAgingThresholdForTest(n int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg.AgingThreshold = n
}

// ensurePoolFor spawns the per-context worker pool the first time a
// kctx receives a Submit. Caller must hold r.mu.
func (r *Registry) ensurePoolFor(_ string, q *ctxQueue) {
	if !r.started {
		return
	}
	if q.poolStarted {
		return
	}
	q.poolStarted = true
	for i := range r.cfg.WorkersPerContext {
		r.workersWG.Add(1)
		go r.workerLoop(q, workerClassFor(i, r.cfg.CriticalReserved, r.cfg.LowReserved))
	}
}

// workerClassFor assigns worker index i a class: the first CriticalReserved
// workers prefer Critical, the last LowReserved workers prefer Low, and the
// rest are general. The ranges never overlap because ClampLowReserved keeps
// critical + low <= workers - 1.
func workerClassFor(i, criticalReserved, lowReserved int) workerClass {
	switch {
	case i < criticalReserved:
		return workerClassCritical
	case i >= criticalReserved && i < criticalReserved+lowReserved:
		return workerClassLow
	default:
		return workerClassGeneral
	}
}

// workerLoop is one worker goroutine: pulls tasks from q honoring priority
// and its worker class, runs Fn with timeout, delivers Result. The class
// biases which lane it prefers (see pickTask); all classes fall back so no
// worker idles while work is queued.
func (r *Registry) workerLoop(q *ctxQueue, class workerClass) {
	defer r.workersWG.Done()
	for {
		select {
		case <-r.stopAll:
			return
		case <-q.stop:
			return
		case <-q.wake:
			// Re-check stop before picking — between the wake signal
			// and now, StopWorkers / CancelContext could have closed
			// q.stop, and we must not start new work past that point.
			if shutdownPending(r.stopAll, q.stop) {
				return
			}
			// Try to pick a task for this worker class. pickTask returns
			// (nil, false) only when the queue is genuinely empty (a race
			// between the wake signal and a concurrent pick) — a reserved
			// worker no longer idles on a non-empty queue since it falls
			// back to non-Critical work. Re-signal so a sibling retries, but
			// ONLY when work is actually pending: a stale wake left over from
			// the inner drain loop below would otherwise feed itself (this
			// worker re-signals, wins the receive race, fails pickTask again,
			// re-signals, ad infinitum — issue #206).
			task, ok := r.pickTask(q, class)
			if !ok {
				if q.hasPendingWork() {
					select {
					case q.wake <- struct{}{}:
					default:
					}
				}
				continue
			}
			// Wake a sibling before running: runTask blocks for the whole
			// duration of the (potentially slow) Fn, so without this a burst
			// of submits whose wake signals collapsed into the size-1 buffer
			// would be drained by THIS worker serially while siblings stay
			// parked — the lost-wakeup jam (one task runs, the rest queue
			// behind a single blocked worker). Re-signalling on every
			// successful pick cascades the wake across the pool so queued
			// work runs in parallel up to WorkersPerContext.
			r.wakeSibling(q)
			r.runTask(task)
			// Drain remaining work for this worker class after running
			// one — but bail at every iteration if shutdown has been
			// signalled, so an in-flight Stop doesn't have to wait for
			// the queue to drain.
			for {
				if shutdownPending(r.stopAll, q.stop) {
					return
				}
				task, ok = r.pickTask(q, class)
				if !ok {
					break
				}
				r.wakeSibling(q)
				r.runTask(task)
			}
		}
	}
}

// wakeSibling re-signals q.wake when work is still queued, so another parked
// worker engages instead of leaving the current (about-to-block) worker to
// drain the burst serially. Guarded by hasPendingWork so it never spins on an
// empty queue (issue #206): the non-blocking send + size-1 buffer cap the
// signal at one outstanding wake, and a spurious wake just parks the receiver
// again after a failed pick.
func (r *Registry) wakeSibling(q *ctxQueue) {
	if q.hasPendingWork() {
		select {
		case q.wake <- struct{}{}:
		default:
		}
	}
}

// shutdownPending non-blockingly reports whether either of the two
// stop channels has been closed. Used by workerLoop to bail before
// dispatching a new task once StopWorkers / Close has fired.
func shutdownPending(stopAll, stop chan struct{}) bool {
	select {
	case <-stopAll:
		return true
	case <-stop:
		return true
	default:
		return false
	}
}

// pickTask dequeues the next task for this worker, biased by its class.
//
// A Critical-class worker takes Critical first; a Low-class worker takes Low
// first. When the preferred lane is empty, BOTH fall through to
// dequeueByPriorityLocked (priority + aging) rather than idling while other
// work waits — the reservation guarantees a slot is biased toward that lane,
// not that the worker sits idle. This is what gives background (Low) work a
// guaranteed floor: under a flood of High submissions the Low-class workers
// still drain the Low lane, so metrics/events/dashboards keep running instead
// of queueing behind the foreground backlog. A General worker always honors
// strict priority (with aging). The Critical preempt poker still makes a
// Critical slot by bumping a running lower-priority task.
func (r *Registry) pickTask(q *ctxQueue, class workerClass) (*queuedTask, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	switch class {
	case workerClassCritical:
		if t, ok := dequeueLaneLocked(q, PriorityCritical); ok {
			return t, true
		}
	case workerClassLow:
		if t, ok := dequeueLaneLocked(q, PriorityLow); ok {
			return t, true
		}
	case workerClassGeneral:
		// strict priority + aging below
	}
	return q.dequeueByPriorityLocked()
}

// dequeueLaneLocked pops the head of a single priority lane. Caller holds q.mu.
func dequeueLaneLocked(q *ctxQueue, prio Priority) (*queuedTask, bool) {
	lane := q.lanes[int(prio)]
	if len(lane) == 0 {
		return nil, false
	}
	t := lane[0]
	q.lanes[int(prio)] = lane[1:]
	return t, true
}

// runningTask tracks a task currently executing in a worker. The
// preempt poker uses this to find Low-priority work to cancel when a
// higher-priority Submit lands.
type runningTask struct {
	task            *queuedTask
	cancel          context.CancelFunc
	preempted       atomic.Bool
	contextSwitched atomic.Bool
	superseded      atomic.Bool
}

func (r *Registry) registerRunning(kctx string, rt *runningTask) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runningTasks[kctx] = append(r.runningTasks[kctx], rt)
}

func (r *Registry) unregisterRunning(kctx string, rt *runningTask) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.runningTasks[kctx]
	for i, x := range list {
		if x == rt {
			r.runningTasks[kctx] = append(list[:i], list[i+1:]...)
			break
		}
	}
}

// pokePreempt finds the lowest-priority running task on kctx whose
// priority is strictly worse (numerically higher) than newPrio, sets
// its preempted flag, and cancels its context. Returns true if it
// preempted something.
//
// Critical can never be preempted because nothing has a strictly worse
// priority below Critical except by the same logic, but Submit only
// invokes pokePreempt for the new request's priority — and a Submit
// of priority X only preempts tasks of priority > X.
func (r *Registry) pokePreempt(kctx string, newPrio Priority) bool {
	r.mu.Lock()
	list := r.runningTasks[kctx]
	if len(list) == 0 {
		r.mu.Unlock()
		return false
	}
	var victim *runningTask
	worstPrio := newPrio
	for _, rt := range list {
		// Skip tasks already being torn down (preempted, or superseded by
		// CancelStaleByGen): preempting one wastes this submission's single
		// preemption on a worker slot that is already being reclaimed.
		if rt.preempted.Load() || rt.superseded.Load() {
			continue
		}
		if rt.task.req.Priority > worstPrio {
			victim = rt
			worstPrio = rt.task.req.Priority
		}
	}
	r.mu.Unlock()
	if victim == nil {
		return false
	}
	victim.preempted.Store(true)
	victim.cancel()
	return true
}

// runTask executes a single queued task with timeout. Future is
// delivered on completion (or error). If the task is preempted while
// running, it is requeued at the head of its priority lane and runTask
// returns; the same task will be picked up by a worker again later
// when the higher-priority work has cleared.
//
// The visibility surface (Start/Finish) is populated for every
// submission, including SilentTrack ones, so the :scheduler overlay
// shows the full pipeline. SilentTrack only suppresses the title-bar
// spinner — the rendering layer filters those out via Task.Silent.
func (r *Registry) runTask(task *queuedTask) {
	timeout := r.cfg.TimeoutFor(task.req.Kind)
	if task.req.Timeout > 0 {
		timeout = task.req.Timeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	rt := &runningTask{task: task, cancel: cancel}

	visID := r.startWithPriority(task.req.Kind, task.req.Priority, task.req.Name, task.req.Target, task.req.SilentTrack)
	r.registerRunning(task.req.KubeContext, rt)
	value, err := task.req.Fn(ctx)
	cancel()
	r.unregisterRunning(task.req.KubeContext, rt)

	if rt.contextSwitched.Load() {
		r.Finish(visID)
		task.future <- Result{Err: ErrContextSwitched}
		close(task.future)
		return
	}

	// Superseded is checked before preempted: a stale-cancelled task must
	// resolve as ErrSuperseded and NOT be requeued (the preempt path would
	// re-run it), since a newer generation already owns the result.
	if rt.superseded.Load() {
		r.Finish(visID)
		task.future <- Result{Err: ErrSuperseded}
		close(task.future)
		return
	}

	if rt.preempted.Load() && (err == nil || errors.Is(err, context.Canceled)) {
		// Requeue at head of the same priority lane.
		q := r.queueFor(task.req.KubeContext)
		if q == nil {
			// Registry closed mid-flight — drop with ErrContextSwitched.
			r.Finish(visID)
			task.future <- Result{Err: ErrContextSwitched}
			close(task.future)
			return
		}
		// Finish the visibility entry from this attempt; the next
		// attempt (after re-dispatch) calls Start again.
		r.Finish(visID)
		q.mu.Lock()
		lane := q.lanes[int(task.req.Priority)]
		q.lanes[int(task.req.Priority)] = append([]*queuedTask{task}, lane...)
		q.mu.Unlock()
		select {
		case q.wake <- struct{}{}:
		default:
		}
		return
	}

	r.Finish(visID)
	task.future <- Result{Value: value, Err: err}
	close(task.future)
}

// queueFor returns the ctxQueue for kctx or nil if Registry is closed.
func (r *Registry) queueFor(kctx string) *ctxQueue {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ctxQueues == nil {
		return nil
	}
	return r.ctxQueues[kctx]
}
