package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestBuildActiveRows_FinishedBucketSortedByFinishedAtDesc is the
// regression guard for the user-reported "scheduler history is not
// ordered when they were executed; first item should be the one that
// was executed lately". The Finished bucket of the active table must
// be sorted by FinishedAt DESC, NOT by insertion (Start) order — a
// long-running task that just ended must appear above a quick task
// that started later but finished earlier.
func TestBuildActiveRows_FinishedBucketSortedByFinishedAtDesc(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0)
	snap := []scheduler.Task{
		// Task A: started early, finished late (long-running).
		{
			Kind: scheduler.KindResourceList, Priority: scheduler.PriorityHigh, Name: "long-running-A", Target: "ctx",
			StartedAt: now.Add(-10 * time.Second), FinishedAt: now.Add(-1 * time.Second),
		},
		// Task B: started late, finished early (quick).
		{
			Kind: scheduler.KindYAMLFetch, Priority: scheduler.PriorityHigh, Name: "quick-B", Target: "ctx",
			StartedAt: now.Add(-3 * time.Second), FinishedAt: now.Add(-2 * time.Second),
		},
		// Task C: most recent finish.
		{
			Kind: scheduler.KindMetrics, Priority: scheduler.PriorityLow, Name: "fresh-C", Target: "ctx",
			StartedAt: now.Add(-1 * time.Second), FinishedAt: now,
		},
	}

	rows := buildActiveRows(snap, nil)

	require.Len(t, rows, 3)
	require.Equal(t, ui.TaskStatusFinished, rows[0].Status)
	require.Equal(t, ui.TaskStatusFinished, rows[1].Status)
	require.Equal(t, ui.TaskStatusFinished, rows[2].Status)
	assert.Equal(t, "fresh-C", rows[0].Name, "the most recently finished task must be at index 0")
	assert.Equal(t, "long-running-A", rows[1].Name, "long-running task that just finished comes before the older-finish quick task")
	assert.Equal(t, "quick-B", rows[2].Name, "earliest-finished task is at the bottom of the Finished bucket")
}

// TestBuildActiveRows_BucketOrder verifies the Running → Queued →
// Finished bucket order is preserved.
func TestBuildActiveRows_BucketOrder(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0)
	snap := []scheduler.Task{
		{Kind: scheduler.KindResourceList, Name: "running-1", Target: "ctx", StartedAt: now.Add(-1 * time.Second)},
		{
			Kind: scheduler.KindResourceList, Name: "finished-1", Target: "ctx",
			StartedAt: now.Add(-2 * time.Second), FinishedAt: now.Add(-100 * time.Millisecond),
		},
	}
	queued := []scheduler.QueueEntry{
		{Kind: scheduler.KindDashboard, Name: "queued-1", Target: "ctx", Position: 1},
	}

	rows := buildActiveRows(snap, queued)

	require.Len(t, rows, 3)
	assert.Equal(t, ui.TaskStatusRunning, rows[0].Status)
	assert.Equal(t, "running-1", rows[0].Name)
	assert.Equal(t, ui.TaskStatusQueued, rows[1].Status)
	assert.Equal(t, "queued-1", rows[1].Name)
	assert.Equal(t, ui.TaskStatusFinished, rows[2].Status)
	assert.Equal(t, "finished-1", rows[2].Name)
}
