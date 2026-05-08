package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/scheduler"
)

// newCompleted builds a ResourceList-kind CompletedTask with
// deterministic StartedAt / FinishedAt so tests can assert Duration
// and ordering exactly. Target is fixed to "ctx" — varying targets
// is exercised in the production-shape tests further down.
func newCompleted(name string, dur time.Duration) scheduler.CompletedTask {
	started := time.Unix(1_000_000, 0)
	return scheduler.CompletedTask{
		Task: scheduler.Task{
			Kind:      scheduler.KindResourceList,
			Name:      name,
			Target:    "ctx",
			StartedAt: started,
		},
		FinishedAt: started.Add(dur),
	}
}

// newCompletedAt is like newCompleted but lets the test pin StartedAt
// and Duration independently, so it can express a long-running task
// that finished recently (or a quick task that finished long ago).
func newCompletedAt(name string, started time.Time, dur time.Duration) scheduler.CompletedTask {
	return scheduler.CompletedTask{
		Task: scheduler.Task{
			Kind:      scheduler.KindResourceList,
			Priority:  scheduler.PriorityHigh,
			Name:      name,
			Target:    "ctx",
			StartedAt: started,
		},
		FinishedAt: started.Add(dur),
	}
}

func TestHistoryTasksForDisplayEmpty(t *testing.T) {
	assert.Nil(t, historyTasksForDisplay(nil, false))
	assert.Nil(t, historyTasksForDisplay([]scheduler.CompletedTask{}, false))
}

// TestHistoryTasksForDisplay_HidesSubSecondTasks pins the noise-filter
// behaviour: anything finishing in less than historyMinDuration is
// dropped from the rendered history. Watch-tick refreshes typically
// finish in tens of milliseconds and would otherwise dominate the
// view.
func TestHistoryTasksForDisplay_HidesSubSecondTasks(t *testing.T) {
	snap := []scheduler.CompletedTask{
		newCompleted("fast-A", 100*time.Millisecond),
		newCompleted("slow-B", 800*time.Millisecond),
		newCompleted("fast-C", 50*time.Millisecond),
		newCompleted("boundary-D", historyMinDuration),
	}
	rows := historyTasksForDisplay(snap, false)
	require.Len(t, rows, 2, "fast-A and fast-C must be hidden; slow-B and boundary-D must remain")
	names := []string{rows[0].Name, rows[1].Name}
	assert.Contains(t, names, "slow-B")
	assert.Contains(t, names, "boundary-D")
}

// TestHistoryTasksForDisplay_SortedByFinishedAtDesc verifies the
// ordering contract: the most recently finished task is at index 0,
// and ordering is by FinishedAt — a long-running task that just
// ended must outrank a quick task that finished earlier.
func TestHistoryTasksForDisplay_SortedByFinishedAtDesc(t *testing.T) {
	t0 := time.Unix(1_000_000, 0)
	snap := []scheduler.CompletedTask{
		// Quick task that finished EARLY: started t0+5s, ran 600ms,
		// finished at t0+5.6s.
		newCompletedAt("quick-then", t0.Add(5*time.Second), 600*time.Millisecond),
		// Long task that JUST FINISHED: started t0, ran 30s,
		// finished at t0+30s.
		newCompletedAt("long-now", t0, 30*time.Second),
		// Mid: started t0+10s, ran 1s, finished at t0+11s.
		newCompletedAt("mid", t0.Add(10*time.Second), 1*time.Second),
	}
	rows := historyTasksForDisplay(snap, false)
	require.Len(t, rows, 3)
	assert.Equal(t, "long-now", rows[0].Name, "most recently finished must be at index 0")
	assert.Equal(t, "mid", rows[1].Name)
	assert.Equal(t, "quick-then", rows[2].Name)
}

// TestHistoryTasksForDisplay_NoGrouping verifies that repeating
// signatures appear as separate rows — the grouping that previously
// caused per-tick cycling is gone. Each row carries its own
// FinishedAt so the natural sort produces a deterministic order
// across renders.
func TestHistoryTasksForDisplay_NoGrouping(t *testing.T) {
	t0 := time.Unix(1_000_000, 0)
	snap := []scheduler.CompletedTask{
		newCompletedAt("List Pods", t0.Add(3*time.Second), 800*time.Millisecond),
		newCompletedAt("List Pods", t0.Add(2*time.Second), 800*time.Millisecond),
		newCompletedAt("List Pods", t0.Add(1*time.Second), 800*time.Millisecond),
	}
	rows := historyTasksForDisplay(snap, false)
	require.Len(t, rows, 3, "each instance is its own row — no aggregation")
	for _, r := range rows {
		assert.Equal(t, "List Pods", r.Name, "no count suffix appended; raw Name preserved")
	}
}

// TestHistoryTasksForDisplay_StableAcrossRenders is the regression
// guard for the user-reported "items 3 and 4 keep swapping per tick".
// With grouping removed, each row has its own unique FinishedAt and
// the sort is fully deterministic — running the function twice on
// the same input always produces byte-identical output.
func TestHistoryTasksForDisplay_StableAcrossRenders(t *testing.T) {
	t0 := time.Unix(1_000_000, 0)
	snap := []scheduler.CompletedTask{
		newCompletedAt("A", t0.Add(2*time.Second), 600*time.Millisecond),
		newCompletedAt("B", t0.Add(1*time.Second), 600*time.Millisecond),
		newCompletedAt("A", t0, 600*time.Millisecond),
		newCompletedAt("B", t0.Add(3*time.Second), 600*time.Millisecond),
	}
	rowsA := historyTasksForDisplay(snap, false)
	rowsB := historyTasksForDisplay(snap, false)
	require.Len(t, rowsA, 4)
	require.Len(t, rowsB, 4)
	for i := range rowsA {
		assert.Equal(t, rowsA[i].Name, rowsB[i].Name,
			"row %d must be identical across calls — got %q vs %q", i, rowsA[i].Name, rowsB[i].Name)
		assert.Equal(t, rowsA[i].FinishedAt, rowsB[i].FinishedAt,
			"row %d FinishedAt must be identical across calls", i)
	}
}

// TestHistoryTasksForDisplay_PropagatesFields verifies that all
// metadata the renderer reads (Kind, Priority, Name, Target,
// Duration, StartedAt, FinishedAt) is copied through.
func TestHistoryTasksForDisplay_PropagatesFields(t *testing.T) {
	started := time.Unix(1_000_000, 0)
	snap := []scheduler.CompletedTask{
		{
			Task: scheduler.Task{
				Kind:      scheduler.KindAPIDiscovery,
				Priority:  scheduler.PriorityCritical,
				Name:      "Discover API resources",
				Target:    "dev-envs",
				StartedAt: started,
			},
			FinishedAt: started.Add(1900 * time.Millisecond),
		},
	}
	rows := historyTasksForDisplay(snap, false)
	require.Len(t, rows, 1)
	r := rows[0]
	assert.Equal(t, "APIDiscovery", r.Kind)
	assert.Equal(t, scheduler.PriorityCritical, r.Priority)
	assert.Equal(t, "Discover API resources", r.Name)
	assert.Equal(t, "dev-envs", r.Target)
	assert.Equal(t, 1900*time.Millisecond, r.Duration)
	assert.Equal(t, started, r.StartedAt)
	assert.Equal(t, started.Add(1900*time.Millisecond), r.FinishedAt)
}
