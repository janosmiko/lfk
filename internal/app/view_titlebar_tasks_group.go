package app

import (
	"slices"
	"time"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/ui"
)

// historyMinDuration is the cutoff below which a completed task is
// hidden from the :scheduler history. Watch-tick refreshes are the
// dominant source of completed entries (one tick fires 5+ tasks every
// second on the active list view) and they typically finish in tens
// of milliseconds against an in-cluster control plane. Showing every
// such tick floods the history with the same handful of recurring
// signatures and makes "what just happened that's interesting?"
// impossible to answer.
//
// 500ms is generous enough to keep API discovery, RBAC checks, helm
// subprocess work, and any user-initiated mutation visible — those
// rarely complete that fast — while filtering out the constant low-
// latency refresh stream.
const historyMinDuration = 500 * time.Millisecond

// historyTasksForDisplay converts the registry's completed-task
// snapshot into the display rows shown in the :scheduler overlay's
// Completed view. Two transformations:
//
//  1. Filter: rows whose Duration is below historyMinDuration are
//     hidden when showAll is false. This drops watch-tick refresh
//     noise so genuinely interesting work surfaces. The user can
//     press `a` in the overlay to toggle showAll and inspect the
//     full unfiltered history.
//  2. Sort by FinishedAt descending. Each task remains its own row —
//     no grouping — so the natural "first item is the most recent"
//     ordering is stable across renders and recently-finished work
//     reliably sits on top.
//
// Filtering instead of grouping was chosen after grouping caused
// visible cycling: watch-tick bursts produced groups whose newest
// member kept changing per tick, so the displayed group order kept
// flipping. Showing individual entries with a duration cutoff sidesteps
// the entire problem — each row has its own unique FinishedAt and the
// fast watch-tick rows simply aren't there.
func historyTasksForDisplay(snap []scheduler.CompletedTask, showAll bool) []ui.BackgroundTaskRow {
	if len(snap) == 0 {
		return nil
	}
	rows := make([]ui.BackgroundTaskRow, 0, len(snap))
	for _, t := range snap {
		if !showAll && t.Duration() < historyMinDuration {
			continue
		}
		rows = append(rows, ui.BackgroundTaskRow{
			Kind:       t.Kind.String(),
			Priority:   t.Priority,
			Name:       t.Name,
			Target:     t.Target,
			StartedAt:  t.StartedAt,
			FinishedAt: t.FinishedAt,
			Duration:   t.Duration(),
		})
	}
	if len(rows) == 0 {
		return nil
	}
	// SnapshotCompleted is already prepended-newest-first, so this
	// preserves order in the common case. The explicit sort guards
	// against any future change to the registry's insertion strategy.
	slices.SortStableFunc(rows, func(a, b ui.BackgroundTaskRow) int {
		return b.FinishedAt.Compare(a.FinishedAt)
	})
	return rows
}
