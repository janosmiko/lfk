package app

import (
	"fmt"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/ui"
)

// groupCompletedTasks collapses identical completed entries (same Kind,
// Name, and Target) into a single row per signature, appending a " ×N"
// suffix to Name when N > 1.
//
// The input snap is expected in newest-first order (the order produced
// by Registry.SnapshotCompleted). The output preserves that order based
// on each group's first-seen (newest) instance: groups appear in the
// position of their most recent member, so frequently run tasks bubble
// to the top when they run.
//
// The Duration on each group row is the most recent duration — the
// FIRST member encountered in the newest-first input — so the column
// reads as "how long it took last time" rather than an aggregate.
// Users looking at "List Pods ×12  dev-envs  0.4s" read that as "it
// just ran and took 0.4s, and I've seen this ×12 times in history".
//
// Without grouping, a watch-mode user would quickly fill the 50-entry
// completed history with twelve identical refresh rows and evict
// genuinely interesting one-off tasks.
func groupCompletedTasks(snap []bgtasks.CompletedTask) []ui.BackgroundTaskRow {
	if len(snap) == 0 {
		return nil
	}

	type sig struct {
		kind   bgtasks.Kind
		name   string
		target string
	}

	seen := make(map[sig]int, len(snap))
	rows := make([]ui.BackgroundTaskRow, 0, len(snap))
	counts := make([]int, 0, len(snap))

	for _, t := range snap {
		key := sig{kind: t.Kind, name: t.Name, target: t.Target}
		if idx, ok := seen[key]; ok {
			counts[idx]++
			continue
		}
		seen[key] = len(rows)
		rows = append(rows, ui.BackgroundTaskRow{
			Kind:     t.Kind.String(),
			Name:     t.Name,
			Target:   t.Target,
			Duration: t.Duration(),
		})
		counts = append(counts, 1)
	}

	// Append count suffix to Name for groups larger than 1. Using a
	// trailing suffix (rather than a dedicated column) keeps the
	// existing column layout so the running view and completed view
	// share one header.
	for i, n := range counts {
		if n > 1 {
			rows[i].Name = fmt.Sprintf("%s ×%d", rows[i].Name, n)
		}
	}
	return rows
}
