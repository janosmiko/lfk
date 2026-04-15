// Package bgtasks tracks the in-flight async operations lfk is currently
// running (resource list fetches, YAML loads, metrics enrichment, etc.) so
// the title bar can show an ambient indicator and the :tasks overlay can
// list them with elapsed time. Long-lived sessions like port forwards and
// log streams are deliberately excluded — they have their own surfaces.
package bgtasks

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultThreshold is the standard display threshold for the title-bar
// indicator and :tasks overlay. Tasks whose age is below this threshold
// are stored but excluded from Snapshot. Set to 0 so every tracked load
// surfaces immediately — the user explicitly wants visibility into
// every fetch, even the fast ones. Watch-mode auto-refresh is kept off
// the indicator via Registry.StartUntracked() instead of via a time
// filter.
const DefaultThreshold = 0

// DefaultCompletedCap is the maximum number of completed tasks the
// Registry retains for the :tasks overlay's history view. Once the cap
// is reached, oldest entries are evicted on each Finish. 50 is a
// reasonable "what did I just do?" window without unbounded memory.
const DefaultCompletedCap = 50

// Kind classifies a tracked async operation. Used to label rows in the
// :tasks overlay.
type Kind int

const (
	KindResourceList Kind = iota // main/owned/children resource fetches
	KindYAMLFetch                // single-resource YAML preview
	KindMetrics                  // pod/node metrics enrichment
	KindResourceTree             // resource map / owned tree
	KindDashboard                // cluster + monitoring dashboards
	KindContainers               // container listing for a pod
	KindMutation                 // write operations: delete, scale, restart, sync, reconcile, etc.
	KindSubprocess               // external command: helm, trivy, kubectl describe/explain
)

// String returns the human-readable label for a Kind.
func (k Kind) String() string {
	switch k {
	case KindResourceList:
		return "ResourceList"
	case KindYAMLFetch:
		return "YAMLFetch"
	case KindMetrics:
		return "Metrics"
	case KindResourceTree:
		return "ResourceTree"
	case KindDashboard:
		return "Dashboard"
	case KindContainers:
		return "Containers"
	case KindMutation:
		return "Mutation"
	case KindSubprocess:
		return "Subprocess"
	default:
		return "Unknown"
	}
}

// Task is a single in-flight unit of work. Immutable once created.
type Task struct {
	ID        uint64
	Kind      Kind
	Name      string // human label, e.g. "List Pods"
	Target    string // human context, e.g. "default / web-7d8c-abc"
	StartedAt time.Time
	Current   int // progress: items processed so far (0 = not started)
	Total     int // progress: total items (0 = unknown/not applicable)
}

// CompletedTask is a Task that has finished. FinishedAt - StartedAt
// gives the total duration the operation took.
type CompletedTask struct {
	Task
	FinishedAt time.Time
}

// Duration returns how long the task took from Start to Finish.
func (c CompletedTask) Duration() time.Duration {
	return c.FinishedAt.Sub(c.StartedAt)
}

// Registry is a process-global record of in-flight tracked operations.
// Safe for concurrent use from any number of goroutines.
type Registry struct {
	mu           sync.Mutex
	tasks        map[uint64]*Task
	cancels      map[uint64]context.CancelFunc // cancel funcs for cancellable tasks
	order        []uint64                      // insertion order for stable Snapshot output
	nextID       atomic.Uint64
	threshold    time.Duration
	completed    []CompletedTask // newest-first; capped at completedCap
	completedCap int
}

// New constructs a Registry with the given display threshold and the
// default completed-history cap (DefaultCompletedCap). Tasks whose age
// is below this threshold are stored but excluded from Snapshot, so
// fast loads never flicker through the UI.
func New(threshold time.Duration) *Registry {
	return NewWithCap(threshold, DefaultCompletedCap)
}

// NewWithCap constructs a Registry with a custom completed-history cap.
// Mostly for tests that want to exercise cap/eviction without pushing
// DefaultCompletedCap entries.
func NewWithCap(threshold time.Duration, completedCap int) *Registry {
	if completedCap < 0 {
		completedCap = 0
	}
	return &Registry{
		tasks:        make(map[uint64]*Task),
		cancels:      make(map[uint64]context.CancelFunc),
		threshold:    threshold,
		completedCap: completedCap,
	}
}

// Start records a new tracked task and returns its ID. The caller MUST
// call Finish (typically via defer inside the goroutine) when the work
// completes, regardless of success/error/cancel.
//
// If the registry already contains a task with the same (Kind, Name,
// Target) signature, that earlier entry is removed first so only the
// most recent attempt is visible. The earlier task's goroutine keeps
// running — its deferred Finish will become a no-op because the id is
// no longer in the map. This dedupes the common case where the user
// cursor-hovers across the sidebar, generating a fresh preview load
// for each row while the previous one is still in flight.
//
// A nil receiver is treated as an untracked no-op and returns 0, so call
// sites in loaders do not have to guard against a Model constructed
// without a registry (e.g. minimal test fixtures). Finish(0) is already
// a no-op, so the standard defer pattern still works.
func (r *Registry) Start(kind Kind, name, target string) uint64 {
	if r == nil {
		return 0
	}
	id := r.nextID.Add(1)
	task := &Task{
		ID:        id,
		Kind:      kind,
		Name:      name,
		Target:    target,
		StartedAt: time.Now(),
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// Dedupe: drop any prior task with the same visible signature.
	for oldID, t := range r.tasks {
		if t.Kind == kind && t.Name == name && t.Target == target {
			delete(r.tasks, oldID)
			delete(r.cancels, oldID)
			for i, oid := range r.order {
				if oid == oldID {
					r.order = append(r.order[:i], r.order[i+1:]...)
					break
				}
			}
		}
	}
	r.tasks[id] = task
	r.order = append(r.order, id)
	return id
}

// StartUntracked is the no-op variant used by routine work that should not
// surface in the indicator (watch-mode refreshes). Returns 0; Finish(0) is
// also a no-op, so callers can use the same defer pattern as the tracked
// path. Safe to call on a nil receiver.
func (r *Registry) StartUntracked() uint64 {
	return 0
}

// StartCancellable records a new tracked task with a cancel function and
// returns its ID. The cancel function can be invoked later via Cancel(id)
// or CancelMutations(). Otherwise identical to Start.
func (r *Registry) StartCancellable(kind Kind, name, target string, cancel context.CancelFunc) uint64 {
	id := r.Start(kind, name, target)
	if r == nil || id == 0 {
		return id
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if cancel != nil {
		r.cancels[id] = cancel
	}
	return id
}

// UpdateProgress sets the Current and Total counters on a tracked task.
// Called from background goroutines during bulk operations; the values
// are read on the next View() cycle via Snapshot(). No-op for unknown
// IDs or nil receiver.
func (r *Registry) UpdateProgress(id uint64, current, total int) {
	if r == nil || id == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tasks[id]; ok {
		t.Current = current
		t.Total = total
	}
}

// Cancel invokes the cancel function for the given task ID, if one was
// registered via StartCancellable. The cancel func is removed after
// invocation to prevent double-cancel. No-op for unknown IDs, IDs
// without a cancel func, or nil receiver.
func (r *Registry) Cancel(id uint64) {
	if r == nil || id == 0 {
		return
	}
	r.mu.Lock()
	fn := r.cancels[id]
	delete(r.cancels, id)
	r.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// CancelMutations cancels all in-flight tasks of KindMutation that have
// a registered cancel function. Used when the user presses Ctrl+C or Esc
// during bulk operations.
func (r *Registry) CancelMutations() {
	if r == nil {
		return
	}
	r.mu.Lock()
	var toCancel []context.CancelFunc
	for id, t := range r.tasks {
		if t.Kind == KindMutation {
			if fn, ok := r.cancels[id]; ok {
				toCancel = append(toCancel, fn)
				delete(r.cancels, id)
			}
		}
	}
	r.mu.Unlock()
	for _, fn := range toCancel {
		fn()
	}
}

// HasActiveMutations returns true if there are any in-flight KindMutation
// tasks. Used by the key handler to decide whether Ctrl+C/Esc should
// cancel bulk operations instead of closing the tab/quitting.
func (r *Registry) HasActiveMutations() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tasks {
		if t.Kind == KindMutation {
			return true
		}
	}
	return false
}

// Finish removes the task with the given ID and appends it to the
// completed-history ring (newest-first, capped at completedCap).
// Finishing 0 or an unknown ID is a no-op (idempotent — important
// because cancel + late Finish can race, and dedupe eviction also
// leaves stale IDs behind). A nil receiver is also a no-op to mirror
// Start's nil behavior.
func (r *Registry) Finish(id uint64) {
	if r == nil || id == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[id]
	if !ok {
		return
	}
	delete(r.tasks, id)
	delete(r.cancels, id)
	for i, oid := range r.order {
		if oid == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	// Append to completed history, newest-first. Prepend to the front so
	// SnapshotCompleted returns most-recent at index 0.
	if r.completedCap > 0 {
		done := CompletedTask{Task: *task, FinishedAt: time.Now()}
		r.completed = append([]CompletedTask{done}, r.completed...)
		if len(r.completed) > r.completedCap {
			r.completed = r.completed[:r.completedCap]
		}
	}
}

// Snapshot returns a copy of the tasks currently visible (age >= threshold),
// in insertion order. Safe to call from the render goroutine. Returns nil
// when no tasks are visible. A nil receiver returns nil.
func (r *Registry) Snapshot() []Task {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	out := make([]Task, 0, len(r.order))
	for _, id := range r.order {
		t, ok := r.tasks[id]
		if !ok {
			continue
		}
		if now.Sub(t.StartedAt) < r.threshold {
			continue
		}
		out = append(out, *t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Len returns the number of tasks currently visible (above the threshold).
// Cheaper than len(r.Snapshot()) because it doesn't allocate the slice.
// Used by the title bar to decide whether to render the indicator at all.
//
// The result may differ from len(Snapshot()) by at most one task when a
// task's age is crossing the threshold between the two calls, because
// each method reads time.Now() independently. This is acceptable for the
// render loop: a one-frame flicker is invisible, and callers that need
// strict consistency should call Snapshot() once and cache the slice.
// A nil receiver returns 0.
func (r *Registry) Len() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	n := 0
	for _, t := range r.tasks {
		if now.Sub(t.StartedAt) >= r.threshold {
			n++
		}
	}
	return n
}

// SnapshotCompleted returns a copy of the finished-task history, newest
// first. Mutating the returned slice does not affect subsequent calls.
// A nil receiver returns nil.
func (r *Registry) SnapshotCompleted() []CompletedTask {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.completed) == 0 {
		return nil
	}
	out := make([]CompletedTask, len(r.completed))
	copy(out, r.completed)
	return out
}

// NextIDForTest exposes the next-ID atomic for use by integration tests
// in the parent package. Production code MUST NOT call this — use
// Snapshot or Len instead.
func (r *Registry) NextIDForTest() uint64 {
	return r.nextID.Load()
}
