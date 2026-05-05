package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// newRightsizingActionTestModel builds a Model wired for
// executeActionRightsizing tests. It uses the bare-bones
// k8s.NewTestClient (no kubeconfig backend), so
// AvailableRightsizingStrategies returns only StrategySnapshot for any
// workload — sufficient for sticky-state behavior tests where the
// strategy list is the controlled variable.
func newRightsizingActionTestModel() Model {
	return Model{
		actionCtx:        actionContext{context: "test-ctx", namespace: "default", kind: "Pod", name: "pod-a"},
		rightsizingCache: make(map[string]*model.Rightsizing),
		client:           k8s.NewTestClient(nil, nil),
		reqCtx:           context.Background(),
	}
}

// withPrometheusConfigured installs a Prometheus endpoint config for
// the test context so AvailableRightsizingStrategies surfaces the
// Prometheus-backed strategies (PromMax1D / PromAvg1D / PromP957D).
// Restores the previous ConfigMonitoring on cleanup so test ordering
// can't leak into other suites.
func withPrometheusConfigured(t *testing.T) {
	t.Helper()
	prev := model.ConfigMonitoring
	t.Cleanup(func() { model.ConfigMonitoring = prev })
	model.ConfigMonitoring = map[string]model.MonitoringConfig{
		"test-ctx": {
			Prometheus: &model.MonitoringEndpoint{
				Namespaces: []string{"monitoring"},
				Services:   []string{"prometheus"},
				Port:       "9090",
			},
		},
	}
}

// TestExecuteActionRightsizing_StickyStrategyKeptWhenAvailable verifies
// that when the user has previously selected a strategy in the overlay,
// re-opening the overlay (even on a different workload) keeps that
// selection — provided the new workload's available strategies still
// include it.
func TestExecuteActionRightsizing_StickyStrategyKeptWhenAvailable(t *testing.T) {
	withPrometheusConfigured(t) // makes StrategyPromMax1D appear in available list

	m := newRightsizingActionTestModel()
	// Simulate a prior overlay session that left the picker on
	// PromMax1D + 1.5 headroom.
	m.rightsizing.strategy = model.StrategyPromMax1D
	m.rightsizing.headroom = 1.5

	ret, _ := m.executeActionRightsizing()
	r := ret.(Model)

	assert.Equal(t, model.StrategyPromMax1D, r.rightsizing.strategy,
		"sticky strategy must survive overlay re-open when still available")
	assert.InDelta(t, 1.5, r.rightsizing.headroom, 1e-9,
		"sticky headroom must survive overlay re-open")
	assert.Nil(t, r.rightsizing.data, "data must reset to nil so the overlay shows loading")
	assert.True(t, r.rightsizing.loading, "loading must flip on while the new fetch runs")
}

// TestExecuteActionRightsizing_StickyStrategyDroppedWhenUnavailable
// covers the case where the previously selected strategy is not
// available for the new workload — the picker must fall back to the
// first available strategy after the async probe lands rather than
// displaying a stale, unusable selection.
//
// The strategy probe runs as a deferred tea.Cmd to keep the blocking
// findVPA list call off the update loop, so the optimistic open keeps
// the sticky value and the reconciliation happens when
// rightsizingStrategiesProbedMsg arrives.
func TestExecuteActionRightsizing_StickyStrategyDroppedWhenUnavailable(t *testing.T) {
	// No Prometheus configured → PromMax1D is not in the available
	// list, so the sticky strategy will be dropped after probe
	// reconciliation.
	m := newRightsizingActionTestModel()
	m.rightsizing.strategy = model.StrategyPromMax1D
	m.rightsizing.headroom = 1.5

	ret, _ := m.executeActionRightsizing()
	r := ret.(Model)

	// Optimistic open: sticky is preserved as the initial guess —
	// the probe hasn't returned yet.
	assert.Equal(t, model.StrategyPromMax1D, r.rightsizing.strategy,
		"optimistic open keeps sticky value pending the async probe")
	assert.InDelta(t, 1.5, r.rightsizing.headroom, 1e-9,
		"headroom is strategy-independent and must remain sticky")

	// Simulate the probe returning [snapshot] — the bare test client
	// configures no Prometheus and no VPA, so that's the cluster-true
	// list.
	probe := rightsizingStrategiesProbedMsg{
		available:  []model.RightsizingStrategy{model.StrategySnapshot},
		generation: r.rightsizing.gen,
	}
	r2, _ := r.updateRightsizingStrategiesProbed(probe)
	assert.Equal(t, model.StrategySnapshot, r2.rightsizing.strategy,
		"unavailable sticky strategy must be dropped after probe reconciliation")
	assert.InDelta(t, 1.5, r2.rightsizing.headroom, 1e-9,
		"headroom is strategy-independent and must remain sticky after reconciliation")
}

// TestExecuteActionRightsizing_StickyHeadroomKept covers the headroom
// stickiness in isolation: any headroom value is valid for any
// strategy, so it must survive overlay re-opens regardless of which
// strategy is active.
func TestExecuteActionRightsizing_StickyHeadroomKept(t *testing.T) {
	m := newRightsizingActionTestModel()
	m.rightsizing.headroom = 1.75
	// Strategy left empty — first overlay open exercise.
	m.rightsizing.strategy = ""

	ret, _ := m.executeActionRightsizing()
	r := ret.(Model)

	assert.InDelta(t, 1.75, r.rightsizing.headroom, 1e-9,
		"sticky headroom must survive overlay re-open regardless of strategy state")
}

// TestExecuteActionRightsizing_NoStickyUsesConfigDefaults verifies that
// when no sticky values exist (first overlay open of the session), the
// config-file defaults seed the picker.
func TestExecuteActionRightsizing_NoStickyUsesConfigDefaults(t *testing.T) {
	withPrometheusConfigured(t) // ensure config default StrategyPromMax1D is available

	prevStrategy := model.ConfigDefaultRightsizingStrategy
	prevHeadroom := model.ConfigDefaultRightsizingHeadroom
	t.Cleanup(func() {
		model.ConfigDefaultRightsizingStrategy = prevStrategy
		model.ConfigDefaultRightsizingHeadroom = prevHeadroom
	})

	m := newRightsizingActionTestModel()
	m.rightsizing.strategy = ""
	m.rightsizing.headroom = 0

	// Configure a default that's also in the available list for the
	// new workload.
	model.ConfigDefaultRightsizingStrategy = model.StrategyPromMax1D
	model.ConfigDefaultRightsizingHeadroom = 2.0

	ret, _ := m.executeActionRightsizing()
	r := ret.(Model)

	assert.Equal(t, model.StrategyPromMax1D, r.rightsizing.strategy,
		"first overlay open with no sticky must seed strategy from config default")
	assert.InDelta(t, 2.0, r.rightsizing.headroom, 1e-9,
		"first overlay open with no sticky must seed headroom from config default")
}

// TestExecuteActionRightsizing_NoStickyNoConfigUsesBuiltinDefaults
// verifies the cold-cold path: no sticky values, no config defaults,
// fall back to the highest-priority available strategy + the built-in
// DefaultRightsizingHeadroom.
func TestExecuteActionRightsizing_NoStickyNoConfigUsesBuiltinDefaults(t *testing.T) {
	prevStrategy := model.ConfigDefaultRightsizingStrategy
	prevHeadroom := model.ConfigDefaultRightsizingHeadroom
	t.Cleanup(func() {
		model.ConfigDefaultRightsizingStrategy = prevStrategy
		model.ConfigDefaultRightsizingHeadroom = prevHeadroom
	})
	model.ConfigDefaultRightsizingStrategy = ""
	model.ConfigDefaultRightsizingHeadroom = 0

	m := newRightsizingActionTestModel()
	m.rightsizing.strategy = ""
	m.rightsizing.headroom = 0

	ret, _ := m.executeActionRightsizing()
	r := ret.(Model)

	// Bare test client → only StrategySnapshot is available.
	assert.Equal(t, model.StrategySnapshot, r.rightsizing.strategy,
		"no sticky + no config → fall back to first available strategy")
	assert.InDelta(t, model.DefaultRightsizingHeadroom, r.rightsizing.headroom, 1e-9,
		"no sticky + no config → fall back to DefaultRightsizingHeadroom")
}

// TestExecuteActionRightsizing_ConfigDefaultStrategyDroppedWhenUnavailable
// covers the case where the configured default strategy is not
// available for the workload — the optimistic open seeds it but the
// async probe reconciliation drops it once the cluster-true
// availability list arrives.
func TestExecuteActionRightsizing_ConfigDefaultStrategyDroppedWhenUnavailable(t *testing.T) {
	prevStrategy := model.ConfigDefaultRightsizingStrategy
	prevHeadroom := model.ConfigDefaultRightsizingHeadroom
	t.Cleanup(func() {
		model.ConfigDefaultRightsizingStrategy = prevStrategy
		model.ConfigDefaultRightsizingHeadroom = prevHeadroom
	})

	m := newRightsizingActionTestModel()
	m.rightsizing.strategy = ""
	m.rightsizing.headroom = 0

	// Config wants PromMax1D but the workload only supports Snapshot
	// (no Prometheus configured).
	model.ConfigDefaultRightsizingStrategy = model.StrategyPromMax1D

	ret, _ := m.executeActionRightsizing()
	r := ret.(Model)

	// Optimistic open: config default is the initial guess (sticky
	// is empty so config wins).
	assert.Equal(t, model.StrategyPromMax1D, r.rightsizing.strategy,
		"optimistic open seeds config default pending the async probe")

	// Probe returns [snapshot] — config default isn't actually
	// available on this workload.
	probe := rightsizingStrategiesProbedMsg{
		available:  []model.RightsizingStrategy{model.StrategySnapshot},
		generation: r.rightsizing.gen,
	}
	r2, _ := r.updateRightsizingStrategiesProbed(probe)
	assert.Equal(t, model.StrategySnapshot, r2.rightsizing.strategy,
		"unavailable config default must fall back to first available strategy after probe")
}

// TestUpdateRightsizingStrategiesProbed_KeepsStickyWhenAvailable
// verifies the no-op path: when the optimistic strategy survives the
// probe, the handler doesn't reload data or bump generation — the
// initial load dispatched alongside the probe is still valid for the
// chosen strategy.
func TestUpdateRightsizingStrategiesProbed_KeepsStickyWhenAvailable(t *testing.T) {
	withPrometheusConfigured(t)

	m := newRightsizingActionTestModel()
	m.rightsizing.strategy = model.StrategyPromMax1D
	m.rightsizing.headroom = 1.5

	ret, _ := m.executeActionRightsizing()
	r := ret.(Model)
	prevGen := r.rightsizing.gen

	probe := rightsizingStrategiesProbedMsg{
		available: []model.RightsizingStrategy{
			model.StrategyPromMax1D, model.StrategySnapshot,
		},
		generation: prevGen,
	}
	r2, cmd := r.updateRightsizingStrategiesProbed(probe)

	assert.Equal(t, model.StrategyPromMax1D, r2.rightsizing.strategy,
		"sticky strategy survives the probe → no demotion")
	assert.Equal(t, prevGen, r2.rightsizing.gen,
		"no-op reconciliation must NOT bump gen — the in-flight load is still valid")
	assert.Nil(t, cmd, "no-op reconciliation must NOT fire a fresh load cmd")
	assert.Equal(t, []model.RightsizingStrategy{
		model.StrategyPromMax1D, model.StrategySnapshot,
	}, r2.rightsizing.available, "available list reflects the probe result")
}

// TestUpdateRightsizingStrategiesProbed_StaleGenerationDropped guards
// the stale-response path: a probe response from a previous overlay
// open must be discarded, otherwise it could overwrite the freshly
// resolved strategy with stale data.
func TestUpdateRightsizingStrategiesProbed_StaleGenerationDropped(t *testing.T) {
	m := newRightsizingActionTestModel()
	m.rightsizing.strategy = model.StrategyVPA
	m.rightsizing.available = []model.RightsizingStrategy{model.StrategyVPA}
	m.rightsizing.gen = 5

	stale := rightsizingStrategiesProbedMsg{
		available:  []model.RightsizingStrategy{model.StrategySnapshot},
		generation: 4, // older than the current gen
	}
	r, cmd := m.updateRightsizingStrategiesProbed(stale)
	assert.Equal(t, model.StrategyVPA, r.rightsizing.strategy,
		"stale probe must NOT overwrite current strategy")
	assert.Nil(t, cmd, "stale probe must NOT dispatch follow-up cmds")
	assert.Equal(t, []model.RightsizingStrategy{model.StrategyVPA}, r.rightsizing.available,
		"stale probe must NOT replace the available list")
}
