// Package scheduler tracks the in-flight async operations lfk is currently
// running (resource list fetches, YAML loads, metrics enrichment, etc.) so
// the title bar can show an ambient indicator and the :tasks overlay can
// list them with elapsed time. Long-lived sessions like port forwards and
// log streams are deliberately excluded — they have their own surfaces.
package scheduler

import (
	"context"
	"maps"
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
// Registry retains for the :scheduler overlay's history view. Once the
// cap is reached, oldest entries are evicted on each Finish. 1000 is
// large enough that a typical session never loses entries while still
// keeping memory bounded (~80 KB at peak with average task fields).
const DefaultCompletedCap = 1000

// DefaultLingerDuration is how long a finished task remains visible in
// the Running list after Finish() is called. The same task is also in
// the completed-history ring immediately, so it shows up in both views
// during the linger window. After the window expires, the task is
// evicted from the running map (lazily, on the next Snapshot/Len call)
// and only the history entry remains.
const DefaultLingerDuration = 10 * time.Second

// Kind classifies a tracked async operation. Used to label rows in the
// :tasks overlay.
type Kind int

const (
	KindResourceList  Kind = iota // main/owned/children resource fetches
	KindYAMLFetch                 // single-resource YAML preview
	KindMetrics                   // pod/node metrics enrichment
	KindResourceTree              // resource map / owned tree
	KindDashboard                 // cluster + monitoring dashboards
	KindContainers                // container listing for a pod
	KindMutation                  // write operations: delete, scale, restart, sync, reconcile, etc.
	KindSubprocess                // external command: helm, trivy, kubectl describe/explain
	KindAPIDiscovery              // discover API resource types for a context (foundational)
	KindNamespaceList             // list namespaces for a context (foundational)
	KindRBACCheck                 // RBAC SelfSubjectAccessReview / Can-I checks (foundational)
	KindSecurityScan              // background security source availability + finding scans
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
	case KindAPIDiscovery:
		return "APIDiscovery"
	case KindNamespaceList:
		return "NamespaceList"
	case KindRBACCheck:
		return "RBACCheck"
	case KindSecurityScan:
		return "SecurityScan"
	default:
		return "Unknown"
	}
}

// Task is a single tracked unit of work. While in flight FinishedAt is
// the zero value. After Finish() is called the field is set and the
// task remains in the Running list for DefaultLingerDuration so users
// can see what just ran.
//
// Silent marks routine work that should NOT activate the title-bar
// spinner (watch-mode auto-refresh). Silent tasks still appear in
// Snapshot/SnapshotCompleted so they show up in the :scheduler
// overlay history. Only the title-bar renderer filters them out.
type Task struct {
	ID         uint64
	Kind       Kind
	Priority   Priority // scheduler priority for visibility chip
	Name       string   // human label, e.g. "List Pods"
	Target     string   // human context, e.g. "default / web-7d8c-abc"
	StartedAt  time.Time
	FinishedAt time.Time // zero while running. Set on Finish()
	Silent     bool      // suppress from title-bar indicator only
	Current    int       // progress: items processed so far (0 = not started)
	Total      int       // progress: total items (0 = unknown/not applicable)
	// Owner is the UID of the tab that started the task, so Ctrl+C only
	// cancels work started from the tab the user is looking at. 0 means
	// no owning tab and matches every tab.
	Owner uint64
}

// IsFinished reports whether the task has had Finish() called and is
// now in its post-completion linger window. Returns false for in-flight
// tasks.
func (t Task) IsFinished() bool {
	return !t.FinishedAt.IsZero()
}

// Elapsed returns wall time from StartedAt to either FinishedAt (for
// finished tasks) or now (for running tasks). Used by the renderer's
// ELAPSED column so finished-lingering rows don't keep ticking after
// they're done.
func (t Task) Elapsed(now time.Time) time.Duration {
	if t.IsFinished() {
		return t.FinishedAt.Sub(t.StartedAt)
	}
	return now.Sub(t.StartedAt)
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

// Registry is a process-global record of tracked operations.
// Safe for concurrent use from any number of goroutines.
type Registry struct {
	mu      sync.Mutex
	tasks   map[uint64]*Task
	cancels map[uint64]context.CancelFunc // cancel funcs for cancellable tasks
	// disowned holds the UIDs of closed tabs. Work already queued when a
	// tab closes registers later, so the owner is stripped at Start time
	// too — otherwise it would come back owned by a tab that is gone.
	// Entries are never evicted: a queued task can register at any later
	// point, and there is no moment where that becomes impossible (it may
	// sit behind arbitrarily long work). Growth is one 8-byte key per tab
	// the user closes, so even an extreme session stays well under 100 KB.
	disowned       map[uint64]struct{}
	order          []uint64 // insertion order for stable Snapshot output
	nextID         atomic.Uint64
	threshold      time.Duration
	lingerDuration time.Duration   // how long finished tasks stay in the Running list
	completed      []CompletedTask // newest-first. Capped at completedCap
	completedCap   int

	// Scheduling state — per-context priority queues.
	cfg       *Config
	ctxQueues map[string]*ctxQueue
	closeOnce sync.Once
	stopAll   chan struct{}

	started   bool
	workersWG sync.WaitGroup

	// runningTasks maps kctx → list of currently-running tasks for the
	// preempt poker. Updated under r.mu.
	runningTasks map[string][]*runningTask
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
		tasks:          make(map[uint64]*Task),
		cancels:        make(map[uint64]context.CancelFunc),
		disowned:       make(map[uint64]struct{}),
		threshold:      threshold,
		lingerDuration: DefaultLingerDuration,
		completedCap:   completedCap,
		cfg:            FromGlobals(),
		ctxQueues:      make(map[string]*ctxQueue),
		stopAll:        make(chan struct{}),
		runningTasks:   make(map[string][]*runningTask),
	}
}

// SetLingerDurationForTest overrides DefaultLingerDuration on this
// registry. Tests use it to make linger-window assertions fast without
// real-time waits. Production code MUST NOT call this.
func (r *Registry) SetLingerDurationForTest(d time.Duration) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lingerDuration = d
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
	return r.StartOwned(0, kind, name, target)
}

// StartOwned is Start with an owning tab UID recorded on the task, so
// per-tab cancellation (Ctrl+C) only reaches work started from that tab.
// Owner 0 means "no owning tab" and matches every tab.
func (r *Registry) StartOwned(owner uint64, kind Kind, name, target string) uint64 {
	return r.startWithPriority(kind, DefaultPriorityFor(kind), name, target, false, owner)
}

// startWithPriority is the internal variant of Start that lets scheduler
// workers override the priority lookup with the submission's explicit
// choice and mark routine work as silent. External callers use Start
// and get DefaultPriorityFor(kind), Silent=false.
func (r *Registry) startWithPriority(kind Kind, prio Priority, name, target string, silent bool, owner uint64) uint64 {
	if r == nil {
		return 0
	}
	id := r.nextID.Add(1)
	task := &Task{
		ID:        id,
		Kind:      kind,
		Priority:  prio,
		Name:      name,
		Target:    target,
		StartedAt: time.Now(),
		Silent:    silent,
		Owner:     owner,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dead := r.disowned[task.Owner]; dead {
		task.Owner = 0
	}
	// Dedupe: drop any prior task with the same visible signature. Owner is
	// part of the signature — two tabs running the same operation are two
	// operations, and collapsing them would leave the loser uncancellable.
	for oldID, t := range r.tasks {
		if t.Kind == kind && t.Name == name && t.Target == target && t.Owner == task.Owner {
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
// surface in the indicator (watch-mode refreshes). Returns 0. Finish(0) is
// also a no-op, so callers can use the same defer pattern as the tracked
// path. Safe to call on a nil receiver.
func (r *Registry) StartUntracked() uint64 {
	return 0
}

// StartCancellable records a new tracked task with a cancel function and
// returns its ID. The cancel function can be invoked later via Cancel(id)
// or CancelMutationsOwnedBy(owner). Otherwise identical to StartOwned.
func (r *Registry) StartCancellable(owner uint64, kind Kind, name, target string, cancel context.CancelFunc) uint64 {
	id := r.StartOwned(owner, kind, name, target)
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
// Called from background goroutines during bulk operations. The values
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

// ownedBy reports whether a task belongs to the given tab. Tasks started
// without an owner (Owner 0) match every tab so untagged work stays
// cancellable from wherever the user is.
func (t *Task) ownedBy(owner uint64) bool {
	return t.Owner == 0 || t.Owner == owner
}

// DisownTasks clears the owner on every task started by the given tab, so
// work outliving its tab stays cancellable from the remaining ones. Called
// when a tab closes — UIDs are never reused, so an owner that no longer
// exists would otherwise match no tab at all.
func (r *Registry) DisownTasks(owner uint64) {
	if r == nil || owner == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tasks {
		if t.Owner == owner {
			t.Owner = 0
		}
	}
	r.disowned[owner] = struct{}{}
}

// CancelMutationsOwnedBy cancels the in-flight KindMutation tasks started
// from the given tab that have a registered cancel function. Used when
// the user presses Ctrl+C or Esc during bulk operations.
func (r *Registry) CancelMutationsOwnedBy(owner uint64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	var toCancel []context.CancelFunc
	for id, t := range r.tasks {
		if t.Kind == KindMutation && t.ownedBy(owner) {
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

// HasActiveMutationsOwnedBy returns true if the given tab has in-flight
// KindMutation tasks. Used by the key handler to decide whether Ctrl+C/Esc
// should cancel bulk operations instead of closing the tab/quitting — work
// started on another tab must not swallow the key.
func (r *Registry) HasActiveMutationsOwnedBy(owner uint64) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tasks {
		// Finished-lingering mutations are not "active" — they have
		// already returned. Only in-flight mutations count.
		if t.Kind == KindMutation && t.FinishedAt.IsZero() && t.ownedBy(owner) {
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
	now := time.Now()
	task.FinishedAt = now
	delete(r.cancels, id)
	// Append to completed history, newest-first. Prepend to the front so
	// SnapshotCompleted returns most-recent at index 0.
	if r.completedCap > 0 {
		done := CompletedTask{Task: *task, FinishedAt: now}
		r.completed = append([]CompletedTask{done}, r.completed...)
		if len(r.completed) > r.completedCap {
			r.completed = r.completed[:r.completedCap]
		}
	}
	// NOTE: task stays in r.tasks/r.order for DefaultLingerDuration so
	// the user can see what just ran in the Running list. Snapshot/Len
	// lazily evict expired entries.
}

// pruneExpiredLocked removes finished-lingering tasks whose linger
// window has expired. Called by Snapshot under r.mu. Returns the new
// order slice. Expired tasks are deleted from r.tasks and r.cancels.
func (r *Registry) pruneExpiredLocked(now time.Time) {
	if r.lingerDuration <= 0 {
		return
	}
	kept := r.order[:0]
	for _, id := range r.order {
		t, ok := r.tasks[id]
		if !ok {
			continue
		}
		if !t.FinishedAt.IsZero() && now.Sub(t.FinishedAt) > r.lingerDuration {
			delete(r.tasks, id)
			delete(r.cancels, id)
			continue
		}
		kept = append(kept, id)
	}
	r.order = kept
}

// Snapshot returns a copy of the tasks currently visible (running, plus
// finished tasks still inside their linger window), in insertion order.
// Safe to call from the render goroutine. Returns nil when no tasks are
// visible. A nil receiver returns nil.
//
// Snapshot opportunistically evicts tasks whose linger window has
// expired so r.tasks doesn't grow without bound between Finish calls.
func (r *Registry) Snapshot() []Task {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.pruneExpiredLocked(now)
	out := make([]Task, 0, len(r.order))
	for _, id := range r.order {
		t, ok := r.tasks[id]
		if !ok {
			continue
		}
		// Apply the start-time threshold only to running tasks so a fast
		// load that finishes inside the threshold still surfaces during
		// its linger window.
		if t.FinishedAt.IsZero() && now.Sub(t.StartedAt) < r.threshold {
			continue
		}
		out = append(out, *t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Len returns the number of tasks currently visible — running plus
// finished-within-linger, all priorities, including silent. Cheaper
// than len(r.Snapshot()) because it doesn't allocate the slice. The
// overlay-facing count.
//
// For the title-bar "is anything actually running right now" gate,
// use LenIndicator() instead — it additionally drops silent and
// finished-lingering entries.
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
	return r.lenLocked(false, false)
}

// LenIndicator returns the count of NON-silent, actually-running
// tasks. Used by the title-bar spinner gate. Two filters:
//   - silent tasks (watch-mode auto-refresh) are skipped so the spinner
//     doesn't flicker every second.
//   - finished-within-linger tasks are skipped so the spinner doesn't
//     keep spinning for DefaultLingerDuration after every navigation.
//     The linger window is for the :tasks overlay's Running list ("look
//     at what just ran"), not for the title-bar "work in progress"
//     indicator.
//
// Snapshot() / Len() retain the broader visible set (running +
// finished-within-linger) so the overlay continues to render history.
func (r *Registry) LenIndicator() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lenLocked(true, true)
}

// lenLocked counts visible tasks under r.mu. skipSilent drops silent
// tasks (watch-mode refresh). skipFinished drops finished-lingering
// tasks. Pass true for title-bar indicator semantics, false for the
// overlay-facing count that includes the linger window.
func (r *Registry) lenLocked(skipSilent, skipFinished bool) int {
	now := time.Now()
	n := 0
	for _, t := range r.tasks {
		if skipSilent && t.Silent {
			continue
		}
		// Past-linger entries are skipped unconditionally. We do not GC
		// here (kept read-only style), Snapshot's prune handles eviction.
		// A non-positive lingerDuration means "do not linger" so finished
		// tasks expire immediately for counting purposes.
		if !t.FinishedAt.IsZero() {
			if r.lingerDuration <= 0 || now.Sub(t.FinishedAt) > r.lingerDuration {
				continue
			}
		}
		// In-linger finished tasks: shown in the overlay (skipFinished
		// false) but excluded from the title-bar indicator (skipFinished
		// true) so the spinner reflects actual in-flight work.
		if skipFinished && !t.FinishedAt.IsZero() {
			continue
		}
		// Apply the start-time threshold only to running tasks.
		if t.FinishedAt.IsZero() && now.Sub(t.StartedAt) < r.threshold {
			continue
		}
		n++
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

// InjectCompletedForTest prepends a synthetic CompletedTask onto the
// history so tests can populate a Registry with deterministic durations
// without sleeping between Start and Finish. Production code MUST NOT
// call this. Honors completedCap eviction.
func (r *Registry) InjectCompletedForTest(c CompletedTask) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.completedCap <= 0 {
		return
	}
	r.completed = append([]CompletedTask{c}, r.completed...)
	if len(r.completed) > r.completedCap {
		r.completed = r.completed[:r.completedCap]
	}
}

// CancelContext drops every queued task for kctx (delivering
// ErrContextSwitched to their Futures) and cancels every in-flight
// task on that context. Worker goroutines for kctx exit. A subsequent
// Submit re-spawns them lazily.
//
// Called from app code when the user switches away from a cluster
// context (cluster picker change, tab close on the last tab for that
// context, etc.).
func (r *Registry) CancelContext(kctx string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	q := r.ctxQueues[kctx]
	delete(r.ctxQueues, kctx)
	running := r.runningTasks[kctx]
	delete(r.runningTasks, kctx)
	r.mu.Unlock()

	// Drop queued tasks first.
	if q != nil {
		q.drain(ErrContextSwitched)
	}

	// Mark in-flight tasks as context-switched so runTask delivers
	// ErrContextSwitched instead of treating ctx.Err() as preemption.
	for _, rt := range running {
		rt.contextSwitched.Store(true)
		rt.cancel()
	}
}

// CancelStaleByGen reclaims background work on kctx that a newer
// requestGen has made irrelevant: it drops queued Low-priority tasks and
// cancels in-flight Low-priority tasks whose Gen predates keepGen,
// delivering ErrSuperseded to their Futures.
//
// Scoped to Low priority on purpose. Cursor moves bump requestGen only to
// invalidate the preview pane, so the user's current view (High-priority
// resource list, YAML, containers) and foundational/mutation work
// (Critical) must survive a gen bump — only the decorative Low lane
// (dashboard sections, metrics, preview events) is safe to reclaim and
// cheap to re-issue. Called from navigation paths right after the bump so
// superseded dashboard/preview fetches stop blocking fresh work in the
// shared Low lane instead of running to completion only to be discarded.
func (r *Registry) CancelStaleByGen(kctx string, keepGen uint64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	q := r.ctxQueues[kctx]
	running := append([]*runningTask(nil), r.runningTasks[kctx]...)
	r.mu.Unlock()

	if q != nil {
		q.dropStaleByGen(keepGen)
	}

	for _, rt := range running {
		// Snapshot taken under r.mu above. We act outside the lock (mirrors
		// CancelContext). The window is safe: rt.cancel() is idempotent, and
		// the Future is only ever sent-to/closed by runTask — never here. If
		// a concurrent CancelContext already set contextSwitched, this guard
		// skips the task and runTask delivers ErrContextSwitched. Otherwise
		// runTask sees superseded and delivers ErrSuperseded. No double-send.
		if rt.superseded.Load() || rt.preempted.Load() || rt.contextSwitched.Load() {
			continue
		}
		if staleByGen(rt.task.req, keepGen) {
			rt.superseded.Store(true)
			rt.cancel()
		}
	}
}

// Close releases all per-context resources. Idempotent. Pending Futures
// receive Result{Err: ErrContextSwitched}. Safe on a nil receiver.
func (r *Registry) Close() {
	if r == nil {
		return
	}
	r.closeOnce.Do(func() {
		close(r.stopAll)
		r.mu.Lock()
		queues := r.ctxQueues
		r.ctxQueues = nil
		r.mu.Unlock()
		for _, q := range queues {
			q.drain(ErrContextSwitched)
		}
	})
}

// QueueEntry represents a queued (not yet running) task in a snapshot.
type QueueEntry struct {
	KubeContext string
	Kind        Kind
	Priority    Priority
	Name        string
	Target      string
	Position    int // 1-based from the head of the priority lane
}

// QueueSnapshot returns a copy of every queued task across every
// context, with 1-based head-of-lane positions per priority lane.
// Used by the :scheduler overlay to show queue positions.
//
// Returns nil if there are no queued tasks. A nil receiver returns nil.
func (r *Registry) QueueSnapshot() []QueueEntry {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	queues := make(map[string]*ctxQueue, len(r.ctxQueues))
	maps.Copy(queues, r.ctxQueues)
	r.mu.Unlock()

	var out []QueueEntry
	for kctx, q := range queues {
		q.mu.Lock()
		for prio, lane := range q.lanes {
			for i, t := range lane {
				out = append(out, QueueEntry{
					KubeContext: kctx,
					Kind:        t.req.Kind,
					Priority:    Priority(prio),
					Name:        t.req.Name,
					Target:      t.req.Target,
					Position:    i + 1,
				})
			}
		}
		q.mu.Unlock()
	}
	return out
}
