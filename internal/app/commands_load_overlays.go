package app

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
)

// rightsizingCacheKey is the lookup key for Model.rightsizingCache.
// Includes the context (so a context-switch wipe can invalidate
// per-cluster entries cleanly), the strategy (so flipping strategies
// on the same workload doesn't return cached data from the
// previously-selected strategy), and the headroom multiplier (so
// cycling </> doesn't return the previous multiplier's payload).
//
// The headroom is formatted with %.2f for stability — the picker
// values are 1.00 / 1.10 / 1.25 / 1.50 / 1.75 / 2.00 and a fixed
// width keeps cache-key shape constant across values so a future
// log/grep reader can see the headroom column without surprises.
func rightsizingCacheKey(ctx, ns, kind, name string, strategy model.RightsizingStrategy, headroom float64) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/%.2f", ctx, ns, kind, name, strategy, headroom)
}

// loadRightsizing dispatches a right-sizing fetch for the
// currently-active action context. Cache hit short-circuits to
// a synchronous msg emit so the overlay opens with data on the
// first frame; cache miss runs the GetRightsizing call in a
// goroutine via a tea.Cmd.
//
// The task name carries strategy and headroom: the scheduler folds a
// submission into a running task with the same name, so a shared name
// would leave a picker press during a slow fetch with no fetch at all.
func (m Model) loadRightsizing() tea.Cmd {
	ctx := m.actionCtx
	if ctx.kind == "" || ctx.name == "" {
		return nil
	}
	strategy := m.rightsizing.strategy
	headroom := m.rightsizing.headroom
	key := rightsizingCacheKey(ctx.context, ctx.namespace, ctx.kind, ctx.name, strategy, headroom)

	if cached, ok := m.rightsizingCache[key]; ok && cached != nil {
		return func() tea.Msg {
			return rightsizingLoadedMsg{key: key, data: cached}
		}
	}

	client := m.client
	return m.scheduleK8sCall(
		scheduler.PriorityLow,
		scheduler.KindMetrics,
		fmt.Sprintf("Rightsizing: %s (%s, %gx)", ctx.name, strategy.HumanLabel(), headroom),
		bgtaskTarget(ctx.context, ctx.namespace),
		func(reqCtx context.Context) tea.Msg {
			data, err := client.GetRightsizing(reqCtx, ctx.context, ctx.namespace, ctx.kind, ctx.name, strategy, headroom)
			return rightsizingLoadedMsg{key: key, data: data, err: err}
		},
	)
}
