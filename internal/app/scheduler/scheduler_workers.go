package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
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
		isCritical := i < r.cfg.CriticalReserved
		r.workersWG.Add(1)
		go r.workerLoop(q, isCritical)
	}
}

// workerLoop is one worker goroutine: pulls tasks from q honoring
// priority, runs Fn with timeout, delivers Result. If isCritical is
// true, this worker only picks up Critical tasks.
func (r *Registry) workerLoop(q *ctxQueue, isCritical bool) {
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
			// Try to pick a task for this worker class. If none is found
			// (e.g. a Critical-only worker woke up but only non-Critical
			// tasks are queued), re-signal so the appropriate worker picks
			// it up.
			task, ok := r.pickTask(q, isCritical)
			if !ok {
				// Re-signal so another worker class can pick up the work.
				select {
				case q.wake <- struct{}{}:
				default:
				}
				continue
			}
			r.runTask(task)
			// Drain remaining work for this worker class after running
			// one — but bail at every iteration if shutdown has been
			// signalled, so an in-flight Stop doesn't have to wait for
			// the queue to drain.
			for {
				if shutdownPending(r.stopAll, q.stop) {
					return
				}
				task, ok = r.pickTask(q, isCritical)
				if !ok {
					break
				}
				r.runTask(task)
			}
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

// pickTask dequeues honoring isCritical (Critical-reserved workers only
// take Critical tasks; non-Critical workers take any priority).
func (r *Registry) pickTask(q *ctxQueue, isCritical bool) (*queuedTask, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if isCritical {
		lane := q.lanes[int(PriorityCritical)]
		if len(lane) == 0 {
			return nil, false
		}
		t := lane[0]
		q.lanes[int(PriorityCritical)] = lane[1:]
		return t, true
	}
	return q.dequeueByPriorityLocked()
}

// runningTask tracks a task currently executing in a worker. The
// preempt poker uses this to find Low-priority work to cancel when a
// higher-priority Submit lands.
type runningTask struct {
	task            *queuedTask
	cancel          context.CancelFunc
	preempted       atomic.Bool
	contextSwitched atomic.Bool
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
		if rt.preempted.Load() {
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
