package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
)

// newCompleted builds a CompletedTask with deterministic StartedAt/FinishedAt
// so tests can assert Duration exactly.
func newCompleted(kind bgtasks.Kind, name, target string, dur time.Duration) bgtasks.CompletedTask {
	started := time.Unix(1_000_000, 0)
	return bgtasks.CompletedTask{
		Task: bgtasks.Task{
			Kind:      kind,
			Name:      name,
			Target:    target,
			StartedAt: started,
		},
		FinishedAt: started.Add(dur),
	}
}

func TestGroupCompletedTasksEmpty(t *testing.T) {
	assert.Nil(t, groupCompletedTasks(nil))
	assert.Nil(t, groupCompletedTasks([]bgtasks.CompletedTask{}))
}

// TestGroupCompletedTasksCollapsesIdenticalSignatures is the core case
// the user reported: twelve back-to-back "List Pods / dev-envs" entries
// must collapse into a single row with "×12" appended.
func TestGroupCompletedTasksCollapsesIdenticalSignatures(t *testing.T) {
	snap := make([]bgtasks.CompletedTask, 0, 12)
	for range 12 {
		snap = append(snap, newCompleted(bgtasks.KindResourceList, "List Pods", "dev-envs", 400*time.Millisecond))
	}

	rows := groupCompletedTasks(snap)

	require.Len(t, rows, 1)
	assert.Equal(t, "ResourceList", rows[0].Kind)
	assert.Equal(t, "List Pods ×12", rows[0].Name)
	assert.Equal(t, "dev-envs", rows[0].Target)
	assert.Equal(t, 400*time.Millisecond, rows[0].Duration)
}

// TestGroupCompletedTasksPreservesNewestFirstOrder verifies that groups
// appear in the position of their first-seen (newest) member, so a
// frequently-run task bubbles to the top when it runs again.
func TestGroupCompletedTasksPreservesNewestFirstOrder(t *testing.T) {
	// Input order (newest-first): A, B, A, C, A, B
	// Expected groups: A (×3), B (×2), C (×1) — in first-seen order.
	snap := []bgtasks.CompletedTask{
		newCompleted(bgtasks.KindResourceList, "List A", "ctx", 100*time.Millisecond),
		newCompleted(bgtasks.KindResourceList, "List B", "ctx", 200*time.Millisecond),
		newCompleted(bgtasks.KindResourceList, "List A", "ctx", 110*time.Millisecond),
		newCompleted(bgtasks.KindResourceList, "List C", "ctx", 300*time.Millisecond),
		newCompleted(bgtasks.KindResourceList, "List A", "ctx", 120*time.Millisecond),
		newCompleted(bgtasks.KindResourceList, "List B", "ctx", 210*time.Millisecond),
	}

	rows := groupCompletedTasks(snap)

	require.Len(t, rows, 3)
	assert.Equal(t, "List A ×3", rows[0].Name)
	assert.Equal(t, "List B ×2", rows[1].Name)
	assert.Equal(t, "List C", rows[2].Name, "single-instance group has no count suffix")
}

// TestGroupCompletedTasksKeepsMostRecentDuration checks that the
// Duration shown on a group is the FIRST member encountered in the
// newest-first input — i.e., "how long it took last time".
func TestGroupCompletedTasksKeepsMostRecentDuration(t *testing.T) {
	snap := []bgtasks.CompletedTask{
		// Newest (first encountered): 0.4s — this is what should render.
		newCompleted(bgtasks.KindResourceList, "List Pods", "dev", 400*time.Millisecond),
		// Older duplicates with different durations — ignored for display.
		newCompleted(bgtasks.KindResourceList, "List Pods", "dev", 900*time.Millisecond),
		newCompleted(bgtasks.KindResourceList, "List Pods", "dev", 500*time.Millisecond),
	}

	rows := groupCompletedTasks(snap)

	require.Len(t, rows, 1)
	assert.Equal(t, "List Pods ×3", rows[0].Name)
	assert.Equal(t, 400*time.Millisecond, rows[0].Duration, "duration is from the newest (first-encountered) member")
}

// TestGroupCompletedTasksDifferentTargetsAreDistinct checks that two
// "List Pods" entries in different namespaces do NOT collapse — the
// signature includes Target.
func TestGroupCompletedTasksDifferentTargetsAreDistinct(t *testing.T) {
	snap := []bgtasks.CompletedTask{
		newCompleted(bgtasks.KindResourceList, "List Pods", "dev", 400*time.Millisecond),
		newCompleted(bgtasks.KindResourceList, "List Pods", "prod", 500*time.Millisecond),
		newCompleted(bgtasks.KindResourceList, "List Pods", "dev", 410*time.Millisecond),
	}

	rows := groupCompletedTasks(snap)

	require.Len(t, rows, 2)
	assert.Equal(t, "List Pods ×2", rows[0].Name)
	assert.Equal(t, "dev", rows[0].Target)
	assert.Equal(t, "List Pods", rows[1].Name, "prod entry stays ungrouped")
	assert.Equal(t, "prod", rows[1].Target)
}

// TestGroupCompletedTasksDifferentKindsAreDistinct — a ResourceList and
// a YAMLFetch with the same Name and Target must NOT collapse.
func TestGroupCompletedTasksDifferentKindsAreDistinct(t *testing.T) {
	snap := []bgtasks.CompletedTask{
		newCompleted(bgtasks.KindResourceList, "my-pod", "dev", 400*time.Millisecond),
		newCompleted(bgtasks.KindYAMLFetch, "my-pod", "dev", 200*time.Millisecond),
	}

	rows := groupCompletedTasks(snap)

	require.Len(t, rows, 2)
	assert.Equal(t, "ResourceList", rows[0].Kind)
	assert.Equal(t, "YAMLFetch", rows[1].Kind)
}
