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
// Strategy availability (VPA reachable, Prometheus configured,
// snapshot always) is probed ASYNCHRONOUSLY — the underlying
// AvailableRightsizingStrategies calls findVPA which issues blocking
// dynamic-client List requests, so running it inline on the Bubble
// Tea update loop freezes the UI for the duration of the API round
// trip. The probe cmd is dispatched alongside the data loader; when
// it returns, updateRightsizingStrategiesProbed reconciles
// m.rightsizing.available against the current sticky strategy and
// re-seeds + reloads only if the strategy is no longer available.
//
// The initial guess for `available` is just `[m.rightsizing.strategy]`
// (after sticky/config/builtin resolution) — a single-element list so
// the picker has somewhere to render before the probe lands. The
// strategy chip suppresses the [N/M] indicator while only one entry
// is in the list, so the user doesn't see a misleading "1/1" before
// the real list arrives.
//
// Headroom is independent of the probe — every value works for every
// strategy — so it follows its own pickRightsizingHeadroom chain
// here, no async work needed.
//
// Once the overlay is open the [/] picker walks the available strategy
// list and </> walks the headroom values — see
// handleRightsizingOverlayKey.
func (m Model) executeActionRightsizing() (tea.Model, tea.Cmd) {
	m.overlay = overlayRightsizing

	// Optimistic-strategy resolution at open time: the async probe
	// hasn't returned yet, so build a best-guess candidate list from
	// the sticky strategy + the configured default + snapshot (always
	// available). pickRightsizingStrategy walks them in priority order
	// (sticky → config → first available) so the initial guess matches
	// the user's prior selection / configured preference. The probe
	// reconciliation in updateRightsizingStrategiesProbed will demote
	// or replace the choice if the cluster doesn't actually support
	// it.
	optimistic := []model.RightsizingStrategy{}
	if m.rightsizing.strategy != "" {
		optimistic = append(optimistic, m.rightsizing.strategy)
	}
	if model.ConfigDefaultRightsizingStrategy != "" &&
		!slices.Contains(optimistic, model.ConfigDefaultRightsizingStrategy) {
		optimistic = append(optimistic, model.ConfigDefaultRightsizingStrategy)
	}
	if !slices.Contains(optimistic, model.StrategySnapshot) {
		// Snapshot is always available, so it's a safe last-resort
		// fallback in the initial-guess list.
		optimistic = append(optimistic, model.StrategySnapshot)
	}
	m.rightsizing.strategy = pickRightsizingStrategy(m.rightsizing.strategy, optimistic)
	m.rightsizing.available = []model.RightsizingStrategy{m.rightsizing.strategy}
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
	return m, tea.Batch(m.loadRightsizing(), m.probeRightsizingStrategies())
}

// probeRightsizingStrategies returns a tea.Cmd that runs
// k8s.AvailableRightsizingStrategies in a goroutine. The result is
// delivered via rightsizingStrategiesProbedMsg, which the handler
// reconciles against the optimistic single-element list seeded by
// executeActionRightsizing.
//
// The probe captures the active context fields + the generation
// token at dispatch time so a late response from a previous overlay
// open is dropped on arrival (the gen check in the handler matches
// the same pattern used by loadRightsizing).
func (m Model) probeRightsizingStrategies() tea.Cmd {
	if m.actionCtx.kind == "" || m.actionCtx.name == "" {
		return nil
	}
	client := m.client
	reqCtx := m.reqCtx
	ctxName := m.actionCtx.context
	namespace := m.actionCtx.namespace
	kind := m.actionCtx.kind
	name := m.actionCtx.name
	gen := m.rightsizing.gen
	return func() tea.Msg {
		available := client.AvailableRightsizingStrategies(reqCtx, ctxName, namespace, kind, name)
		if len(available) == 0 {
			// Defensive — AvailableRightsizingStrategies always returns
			// snapshot at minimum, but a future refactor that removes
			// the guarantee shouldn't strand the picker.
			available = []model.RightsizingStrategy{model.StrategySnapshot}
		}
		return rightsizingStrategiesProbedMsg{available: available, generation: gen}
	}
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
