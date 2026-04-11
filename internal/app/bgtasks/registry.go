// Package bgtasks tracks the in-flight async operations lfk is currently
// running (resource list fetches, YAML loads, metrics enrichment, etc.) so
// the title bar can show an ambient indicator and the :tasks overlay can
// list them with elapsed time. Long-lived sessions like port forwards and
// log streams are deliberately excluded — they have their own surfaces.
package bgtasks

import (
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
}

// Registry is a process-global record of in-flight tracked operations.
// Safe for concurrent use from any number of goroutines.
type Registry struct {
	mu        sync.Mutex
	tasks     map[uint64]*Task
	order     []uint64 // insertion order for stable Snapshot output
	nextID    atomic.Uint64
	threshold time.Duration
}

// New constructs a Registry with the given display threshold. Tasks whose
// age is below this threshold are stored but excluded from Snapshot, so
// fast loads never flicker through the UI.
func New(threshold time.Duration) *Registry {
	return &Registry{
		tasks:     make(map[uint64]*Task),
		threshold: threshold,
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

// Finish removes the task with the given ID. Finishing 0 or an unknown ID
// is a no-op (idempotent — important because cancel + late Finish can race).
// A nil receiver is also a no-op to mirror Start's nil behavior.
func (r *Registry) Finish(id uint64) {
	if r == nil || id == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[id]; !ok {
		return
	}
	delete(r.tasks, id)
	for i, oid := range r.order {
		if oid == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
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

// NextIDForTest exposes the next-ID atomic for use by integration tests
// in the parent package. Production code MUST NOT call this — use
// Snapshot or Len instead.
func (r *Registry) NextIDForTest() uint64 {
	return r.nextID.Load()
}
