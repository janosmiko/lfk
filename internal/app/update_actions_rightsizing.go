package app

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
)

// executeActionRightsizing opens the Right-sizing overlay for the
// current actionCtx. Cache hit pre-fills m.rightsizing.data so the
// overlay opens with data on the first frame; cache miss flips
// m.rightsizing.loading=true while loadRightsizing fetches in a
// goroutine. The generation token is bumped before dispatch so any
// in-flight fetch from a previous open is dropped on arrival.
//
// Probes which strategies are available for this workload (VPA
// reachable, Prometheus configured, snapshot always) and seeds the
// initial strategy + headroom using a sticky-then-config-then-builtin
// chain (see pickRightsizingStrategy / pickRightsizingHeadroom).
// Once the overlay is open the [/] picker walks the available strategy
// list and </> walks the headroom values — see
// handleRightsizingOverlayKey.
func (m Model) executeActionRightsizing() (tea.Model, tea.Cmd) {
	m.overlay = overlayRightsizing

	available := m.client.AvailableRightsizingStrategies(
		m.reqCtx,
		m.actionCtx.context, m.actionCtx.namespace, m.actionCtx.kind, m.actionCtx.name,
	)
	if len(available) == 0 {
		// Defensive — AvailableRightsizingStrategies always returns
		// snapshot at minimum, but a future refactor that removes the
		// guarantee shouldn't strand the picker.
		available = []model.RightsizingStrategy{model.StrategySnapshot}
	}
	m.rightsizing.available = available

	// Sticky-then-config-then-builtin defaults. Strategy and headroom
	// each follow their own fallback chain — see the helpers below.
	m.rightsizing.strategy = pickRightsizingStrategy(m.rightsizing.strategy, available)
	m.rightsizing.headroom = pickRightsizingHeadroom(m.rightsizing.headroom)

	// Reset the per-workload transient fields. data is recomputed by
	// the loader (cache hit short-circuits below); err / scroll start
	// fresh; gen bumps so a slow fetch from a prior open is ignored.
	m.rightsizing.data = nil
	m.rightsizing.err = nil
	m.rightsizing.scroll = 0
	m.rightsizing.gen++

	key := rightsizingCacheKey(m.actionCtx.context, m.actionCtx.namespace, m.actionCtx.kind, m.actionCtx.name, m.rightsizing.strategy, m.rightsizing.headroom)
	if cached, ok := m.rightsizingCache[key]; ok && cached != nil {
		m.rightsizing.data = cached
		m.rightsizing.loading = false
	} else {
		m.rightsizing.loading = true
	}
	return m, m.loadRightsizing()
}

// pickRightsizingStrategy resolves the initial strategy for a fresh
// overlay open using a three-step fallback chain:
//
//  1. Sticky: keep the currently-selected strategy if it's still
//     available for this workload. This is what makes the picker feel
//     persistent across overlay opens within a session — close the
//     overlay on pod1 with PromMax1D selected, open it on pod2 and
//     PromMax1D is still active (provided pod2 also has Prometheus).
//
//  2. Config default: if no sticky value (or sticky was unavailable),
//     fall back to model.ConfigDefaultRightsizingStrategy when set
//     and available.
//
//  3. First available: the highest-priority strategy from the available
//     list (StrategyVPA > Prom* > Snapshot per AllRightsizingStrategies
//     order). Last resort StrategySnapshot when the list is empty
//     (defensive — AvailableRightsizingStrategies always returns at
//     least snapshot).
//
// Returning the new strategy keeps the helper pure and easily testable.
func pickRightsizingStrategy(sticky model.RightsizingStrategy, available []model.RightsizingStrategy) model.RightsizingStrategy {
	switch {
	case sticky != "" && slices.Contains(available, sticky):
		return sticky
	case model.ConfigDefaultRightsizingStrategy != "" && slices.Contains(available, model.ConfigDefaultRightsizingStrategy):
		return model.ConfigDefaultRightsizingStrategy
	case len(available) > 0:
		return available[0]
	default:
		return model.StrategySnapshot
	}
}

// pickRightsizingHeadroom resolves the initial headroom for a fresh
// overlay open. Headroom is a pure multiplier — every value works for
// every strategy — so the only fallback condition is "is there any
// sticky value?" If sticky == 0 we fall back to the config default,
// then to the built-in DefaultRightsizingHeadroom.
func pickRightsizingHeadroom(sticky float64) float64 {
	if sticky > 0 {
		return sticky
	}
	if model.ConfigDefaultRightsizingHeadroom > 0 {
		return model.ConfigDefaultRightsizingHeadroom
	}
	return model.DefaultRightsizingHeadroom
}
