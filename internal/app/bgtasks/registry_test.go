package bgtasks

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testThreshold is small so tests don't have to sleep long. The Snapshot
// filter behavior is exercised explicitly in TestSnapshotFiltersBelowThreshold.
const testThreshold = 10 * time.Millisecond

func TestStartReturnsMonotonicID(t *testing.T) {
	r := New(testThreshold)
	id1 := r.Start(KindResourceList, "List Pods", "default")
	id2 := r.Start(KindYAMLFetch, "Get YAML", "default/web-7d8c")
	id3 := r.Start(KindMetrics, "Pod metrics", "default")

	assert.NotEqual(t, uint64(0), id1)
	assert.Greater(t, id2, id1)
	assert.Greater(t, id3, id2)
}

// TestStartDedupesBySignature verifies that Start evicts any prior task
// with the same (Kind, Name, Target) signature before inserting the new
// entry. This is how the registry handles the cursor-hover-on-sidebar
// case: each hover creates a fresh preview load, but only the most
// recent one should be visible in the overlay.
func TestStartDedupesBySignature(t *testing.T) {
	r := New(0)
	id1 := r.Start(KindResourceList, "List Pods", "test-ctx / default")
	id2 := r.Start(KindResourceList, "List Pods", "test-ctx / default")

	assert.Greater(t, id2, id1, "second Start must still return a new ID")
	assert.Equal(t, 1, r.Len(),
		"duplicate signature should evict the earlier task so only one is visible")

	snap := r.Snapshot()
	require.Len(t, snap, 1)
	assert.Equal(t, id2, snap[0].ID, "the NEWEST task wins the signature slot")
}

// TestStartDedupesKeepsOtherSignatures verifies that the dedupe logic
// only evicts same-signature entries. A task with a different Kind, Name,
// or Target must remain untouched.
func TestStartDedupesKeepsOtherSignatures(t *testing.T) {
	r := New(0)
	idA := r.Start(KindResourceList, "List Pods", "ctx-a")
	idB := r.Start(KindResourceList, "List Pods", "ctx-b")    // different target
	idC := r.Start(KindYAMLFetch, "List Pods", "ctx-a")       // different kind
	idD := r.Start(KindResourceList, "List Secrets", "ctx-a") // different name
	// Now duplicate idA's signature — only idA should disappear.
	idA2 := r.Start(KindResourceList, "List Pods", "ctx-a")

	assert.Equal(t, 4, r.Len(), "only idA should be evicted, other signatures retained")

	snap := r.Snapshot()
	ids := make(map[uint64]bool, len(snap))
	for _, t := range snap {
		ids[t.ID] = true
	}
	assert.True(t, ids[idB], "ctx-b task must remain")
	assert.True(t, ids[idC], "YAMLFetch task must remain")
	assert.True(t, ids[idD], "List Secrets task must remain")
	assert.True(t, ids[idA2], "the new ctx-a task must be present")
	assert.False(t, ids[idA], "the old ctx-a task must be evicted")
}

// TestFinishAfterDedupeIsNoop verifies that after a dedupe eviction, the
// evicted task's deferred Finish(oldID) correctly becomes a no-op and
// doesn't disturb the replacement or other entries.
func TestFinishAfterDedupeIsNoop(t *testing.T) {
	r := New(0)
	old := r.Start(KindResourceList, "List Pods", "ctx")
	other := r.Start(KindMetrics, "Pod metrics", "ctx")
	replacement := r.Start(KindResourceList, "List Pods", "ctx")

	// The goroutine that started `old` would now call Finish(old) from
	// its defer — simulate that. It must not touch the replacement or
	// the other task.
	r.Finish(old)

	assert.Equal(t, 2, r.Len(), "replacement and other task must both survive")
	snap := r.Snapshot()
	ids := make(map[uint64]bool, len(snap))
	for _, t := range snap {
		ids[t.ID] = true
	}
	assert.True(t, ids[replacement], "replacement must remain after old goroutine's Finish")
	assert.True(t, ids[other], "unrelated task must remain")
	assert.False(t, ids[old], "old task must stay evicted")
}

func TestStartStoresTask(t *testing.T) {
	r := New(testThreshold)
	id := r.Start(KindResourceList, "List Pods", "default")

	r.mu.Lock()
	defer r.mu.Unlock()
	require.Contains(t, r.tasks, id)
	assert.Equal(t, KindResourceList, r.tasks[id].Kind)
	assert.Equal(t, "List Pods", r.tasks[id].Name)
	assert.Equal(t, "default", r.tasks[id].Target)
	assert.False(t, r.tasks[id].StartedAt.IsZero())
}

func TestFinishRemoves(t *testing.T) {
	r := New(testThreshold)
	id := r.Start(KindResourceList, "List Pods", "default")
	r.Finish(id)

	r.mu.Lock()
	defer r.mu.Unlock()
	assert.NotContains(t, r.tasks, id)
	assert.NotContains(t, r.order, id)
}

func TestFinishUnknownIDIsNoop(t *testing.T) {
	r := New(testThreshold)
	r.Start(KindResourceList, "List Pods", "default")
	// Finishing a stale or never-issued ID should not panic or affect
	// other tasks. Important because cancellation can race with the
	// goroutine's deferred Finish call.
	r.Finish(99999)

	r.mu.Lock()
	defer r.mu.Unlock()
	assert.Len(t, r.tasks, 1)
}

func TestStartUntrackedReturnsZero(t *testing.T) {
	r := New(testThreshold)
	id := r.StartUntracked()
	assert.Equal(t, uint64(0), id)

	r.mu.Lock()
	defer r.mu.Unlock()
	assert.Empty(t, r.tasks, "untracked starts must not be stored")
	assert.Empty(t, r.order)
}

func TestFinishZeroIsNoop(t *testing.T) {
	r := New(testThreshold)
	r.Start(KindResourceList, "List Pods", "default")
	r.Finish(0)

	r.mu.Lock()
	defer r.mu.Unlock()
	assert.Len(t, r.tasks, 1)
}

func TestSnapshotEmptyRegistry(t *testing.T) {
	r := New(testThreshold)
	assert.Empty(t, r.Snapshot())
}

func TestSnapshotFiltersBelowThreshold(t *testing.T) {
	r := New(50 * time.Millisecond)
	r.Start(KindResourceList, "List Pods", "default")
	assert.Empty(t, r.Snapshot())
}

func TestSnapshotIncludesAboveThreshold(t *testing.T) {
	r := New(10 * time.Millisecond)
	id := r.Start(KindResourceList, "List Pods", "default")
	time.Sleep(50 * time.Millisecond)

	got := r.Snapshot()
	require.Len(t, got, 1)
	assert.Equal(t, id, got[0].ID)
	assert.Equal(t, "List Pods", got[0].Name)
}

func TestSnapshotInsertionOrder(t *testing.T) {
	r := New(0)
	id1 := r.Start(KindResourceList, "First", "")
	id2 := r.Start(KindYAMLFetch, "Second", "")
	id3 := r.Start(KindMetrics, "Third", "")

	got := r.Snapshot()
	require.Len(t, got, 3)
	assert.Equal(t, id1, got[0].ID)
	assert.Equal(t, id2, got[1].ID)
	assert.Equal(t, id3, got[2].ID)
}

func TestSnapshotAfterFinishMaintainsOrder(t *testing.T) {
	r := New(0)
	id1 := r.Start(KindResourceList, "First", "")
	id2 := r.Start(KindYAMLFetch, "Second", "")
	id3 := r.Start(KindMetrics, "Third", "")
	r.Finish(id2)

	got := r.Snapshot()
	require.Len(t, got, 2)
	assert.Equal(t, id1, got[0].ID)
	assert.Equal(t, id3, got[1].ID)
}

func TestSnapshotReturnsCopy(t *testing.T) {
	r := New(0)
	r.Start(KindResourceList, "Original", "")
	got := r.Snapshot()
	got[0].Name = "Mutated"

	got2 := r.Snapshot()
	assert.Equal(t, "Original", got2[0].Name,
		"Snapshot must return copies so callers can't mutate registry state")
}

func TestLenEmptyRegistry(t *testing.T) {
	r := New(testThreshold)
	assert.Equal(t, 0, r.Len())
}

func TestLenMatchesSnapshotLen(t *testing.T) {
	r := New(0)
	r.Start(KindResourceList, "First", "")
	r.Start(KindYAMLFetch, "Second", "")
	r.Start(KindMetrics, "Third", "")
	assert.Equal(t, 3, r.Len())
	assert.Equal(t, len(r.Snapshot()), r.Len())
}

func TestLenSkipsBelowThreshold(t *testing.T) {
	r := New(50 * time.Millisecond)
	r.Start(KindResourceList, "Hidden", "")
	assert.Equal(t, 0, r.Len(), "tasks below threshold should not be counted")
}

func TestConcurrentStartFinishSnapshot(t *testing.T) {
	r := New(0)
	const goroutines = 10
	const ops = 200

	done := make(chan struct{}, goroutines)
	for range goroutines {
		go func() {
			defer func() { done <- struct{}{} }()
			for range ops {
				id := r.Start(KindResourceList, "concurrent", "test")
				_ = r.Snapshot()
				_ = r.Len()
				r.Finish(id)
			}
		}()
	}
	for range goroutines {
		<-done
	}

	assert.Equal(t, 0, r.Len(), "all tasks should be finished")
	assert.Empty(t, r.Snapshot())
}

func TestNoAutoEvictionOfHungTask(t *testing.T) {
	r := New(10 * time.Millisecond)
	r.Start(KindResourceList, "stuck", "default")
	time.Sleep(50 * time.Millisecond)
	got := r.Snapshot()
	require.Len(t, got, 1, "hung task must remain visible")
	assert.Equal(t, "stuck", got[0].Name)
}

func TestKindString(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindResourceList, "ResourceList"},
		{KindYAMLFetch, "YAMLFetch"},
		{KindMetrics, "Metrics"},
		{KindResourceTree, "ResourceTree"},
		{KindDashboard, "Dashboard"},
		{KindContainers, "Containers"},
		{Kind(99), "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.kind.String())
		})
	}
}
