package app

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

func newRightsizingTestModel() Model {
	m := Model{
		overlay:          overlayRightsizing,
		actionCtx:        actionContext{context: "c", namespace: "ns", kind: "Pod", name: "pod-a"},
		rightsizingCache: make(map[string]*model.Rightsizing),
	}
	m.rightsizing.strategy = model.StrategyVPA
	m.rightsizing.available = []model.RightsizingStrategy{model.StrategyVPA, model.StrategySnapshot}
	m.rightsizing.headroom = model.DefaultRightsizingHeadroom
	m.rightsizing.data = &model.Rightsizing{
		Source:              "VPA",
		Strategy:            model.StrategyVPA,
		AvailableStrategies: []model.RightsizingStrategy{model.StrategyVPA, model.StrategySnapshot},
		Headroom:            model.DefaultRightsizingHeadroom,
		PodCount:            2,
		Containers: []model.ContainerRec{{
			Name: "app",
			CPU:  model.ResourceRec{CurrentRequest: "100m", CurrentLimit: "500m", RecommendedRequest: "60m", RecommendedLimit: "300m"},
			Mem:  model.ResourceRec{CurrentRequest: "256Mi", CurrentLimit: "512Mi", RecommendedRequest: "200Mi", RecommendedLimit: "400Mi"},
		}},
	}
	return m
}

func TestRightsizingOverlay_EscClosesAndClears(t *testing.T) {
	m := newRightsizingTestModel()
	ret, _ := m.handleRightsizingOverlayKey(specialKey(tea.KeyEsc))
	r := ret.(Model)
	assert.Equal(t, overlayNone, r.overlay)
	assert.Nil(t, r.rightsizing.data, "esc clears overlay's data so a stale view doesn't flash on next open")
}

func TestRightsizingOverlay_QClosesAndClears(t *testing.T) {
	m := newRightsizingTestModel()
	ret, _ := m.handleRightsizingOverlayKey(runeKey('q'))
	r := ret.(Model)
	assert.Equal(t, overlayNone, r.overlay)
}

func TestRightsizingOverlay_RBustsCacheAndDispatches(t *testing.T) {
	m := newRightsizingTestModel()
	key := rightsizingCacheKey("c", "ns", "Pod", "pod-a", m.rightsizing.strategy, m.rightsizing.headroom)
	m.rightsizingCache[key] = m.rightsizing.data

	ret, cmd := m.handleRightsizingOverlayKey(runeKey('r'))
	r := ret.(Model)
	_, present := r.rightsizingCache[key]
	assert.False(t, present, "r must invalidate cached entry so the next fetch hits the cluster")
	assert.True(t, r.rightsizing.loading, "r flips loading on while the fresh fetch runs")
	assert.NotNil(t, cmd, "r dispatches the loader cmd")
}

func TestRightsizingOverlay_YCopiesYAML(t *testing.T) {
	m := newRightsizingTestModel()
	ret, cmd := m.handleRightsizingOverlayKey(runeKey('y'))
	r := ret.(Model)
	assert.NotNil(t, cmd, "y dispatches a clipboard-copy cmd")
	assert.Contains(t, r.statusMessage, "Copied")
	assert.Contains(t, r.statusMessage, "container")
}

func TestRightsizingOverlay_YWithNoRecommendationsShowsError(t *testing.T) {
	m := Model{
		overlay:          overlayRightsizing,
		rightsizingCache: make(map[string]*model.Rightsizing),
	}
	m.rightsizing.data = &model.Rightsizing{
		Source:     "estimated",
		Containers: []model.ContainerRec{{Name: "x"}}, // all rec fields empty
	}
	ret, _ := m.handleRightsizingOverlayKey(runeKey('y'))
	r := ret.(Model)
	assert.Contains(t, r.statusMessage, "Nothing to copy")
	assert.True(t, r.statusMessageErr, "no-data copy must surface as error status")
}

func TestRightsizingOverlay_JScrollsDown(t *testing.T) {
	m := newRightsizingTestModel()
	ret, _ := m.handleRightsizingOverlayKey(runeKey('j'))
	r := ret.(Model)
	assert.Equal(t, 1, r.rightsizing.scroll)
}

func TestRightsizingOverlay_KScrollsUp(t *testing.T) {
	// Build a fixture with enough containers (and a screen height)
	// to make scroll = 1 a valid position. clampRightsizingScroll
	// caps scroll at maxScroll = max(0, totalRows - visibleRows),
	// so a one-container fixture with the default zero height has
	// maxScroll=1 — valid scroll values are 0 and 1.
	m := newRightsizingTestModel()
	m.rightsizing.scroll = 1
	ret, _ := m.handleRightsizingOverlayKey(runeKey('k'))
	r := ret.(Model)
	assert.Equal(t, 0, r.rightsizing.scroll, "k decrements within bounds")
}

func TestRightsizingOverlay_KAtTopStays(t *testing.T) {
	m := newRightsizingTestModel()
	m.rightsizing.scroll = 0
	ret, _ := m.handleRightsizingOverlayKey(runeKey('k'))
	r := ret.(Model)
	assert.Equal(t, 0, r.rightsizing.scroll)
}

func TestRightsizingOverlay_JClampsAtMaxScroll(t *testing.T) {
	// CodeRabbit-style regression: spamming `j` past the last
	// useful position must NOT push content off the top of the
	// visible window. clampRightsizingScroll caps at totalRows -
	// visibleRows, so once we're parked at maxScroll, further `j`
	// keystrokes should be no-ops.
	m := newRightsizingTestModel()
	for range 50 {
		ret, _ := m.handleRightsizingOverlayKey(runeKey('j'))
		m = ret.(Model)
	}
	totalRows := len(m.rightsizing.data.Containers) * 2
	assert.LessOrEqual(t, m.rightsizing.scroll, totalRows,
		"after 50 j-presses scroll must NOT exceed totalRows — clamp keeps the table on-screen")
}

func TestRightsizingOverlay_GGoesToTop(t *testing.T) {
	m := newRightsizingTestModel()
	m.rightsizing.scroll = 1
	ret, _ := m.handleRightsizingOverlayKey(runeKey('g'))
	r := ret.(Model)
	assert.Equal(t, 0, r.rightsizing.scroll, "lowercase g jumps to top")
}

func TestRightsizingOverlay_GoesToBottomG(t *testing.T) {
	// Capital G jumps to the last useful scroll position. With
	// the default fixture (1 container = 2 rows, default zero
	// screen height → 1 visible row), maxScroll = 1.
	m := newRightsizingTestModel()
	ret, _ := m.handleRightsizingOverlayKey(runeKey('G'))
	r := ret.(Model)
	totalRows := len(m.rightsizing.data.Containers) * 2
	assert.LessOrEqual(t, r.rightsizing.scroll, totalRows, "G clamps at maxScroll, never past the data")
	assert.GreaterOrEqual(t, r.rightsizing.scroll, 0)
}

// --- buildRightsizingYAML pure function tests ---

func TestBuildRightsizingYAML_SkipsContainersWithoutRecs(t *testing.T) {
	data := &model.Rightsizing{
		Containers: []model.ContainerRec{
			{Name: "with-rec", CPU: model.ResourceRec{RecommendedRequest: "60m", RecommendedLimit: "300m"}},
			{Name: "no-rec"}, // empty — should be skipped
		},
	}
	out := buildRightsizingYAML(data)
	assert.Contains(t, out, "name: with-rec")
	assert.NotContains(t, out, "name: no-rec", "containers without recommendations must be omitted")
}

func TestBuildRightsizingYAML_NilOrEmptyReturnsEmpty(t *testing.T) {
	assert.Empty(t, buildRightsizingYAML(nil))
	assert.Empty(t, buildRightsizingYAML(&model.Rightsizing{}))
	// Even with containers, all-empty recs should produce empty output
	assert.Empty(t, buildRightsizingYAML(&model.Rightsizing{
		Containers: []model.ContainerRec{{Name: "x"}},
	}))
}

func TestBuildRightsizingYAML_OmitsEmptyResourceGroups(t *testing.T) {
	// Container has only CPU request; no limit, no memory recs
	data := &model.Rightsizing{
		Containers: []model.ContainerRec{{
			Name: "app",
			CPU:  model.ResourceRec{RecommendedRequest: "60m"},
		}},
	}
	out := buildRightsizingYAML(data)
	assert.Contains(t, out, "cpu: 60m")
	assert.NotContains(t, out, "limits:", "no limit recs → no limits: stanza")
	assert.NotContains(t, out, "memory:", "no memory recs → no memory: line")
}

// --- Strategy picker ([ / ]) ---

func TestRightsizingOverlay_StrategyPickerNext(t *testing.T) {
	m := newRightsizingTestModel()
	// Set up an available list: VPA (current), PromMax1D, Snapshot.
	m.rightsizing.strategy = model.StrategyVPA
	m.rightsizing.available = []model.RightsizingStrategy{
		model.StrategyVPA, model.StrategyPromMax1D, model.StrategySnapshot,
	}
	prevData := m.rightsizing.data
	ret, cmd := m.handleRightsizingOverlayKey(runeKey(']'))
	r := ret.(Model)
	assert.Equal(t, model.StrategyPromMax1D, r.rightsizing.strategy, "] moves to the next strategy")
	assert.NotNil(t, cmd, "] dispatches a fresh load for the new strategy")
	assert.True(t, r.rightsizing.loading, "switching strategy flips loading on while the new fetch runs")
	// Stale data stays visible while the new strategy fetches — the
	// renderer adds a "Loading…" hint instead of wiping the table.
	assert.Same(t, prevData, r.rightsizing.data, "previous strategy's data must remain visible during fetch")
}

func TestRightsizingOverlay_StrategyPickerPrev(t *testing.T) {
	m := newRightsizingTestModel()
	m.rightsizing.strategy = model.StrategyPromMax1D
	m.rightsizing.available = []model.RightsizingStrategy{
		model.StrategyVPA, model.StrategyPromMax1D, model.StrategySnapshot,
	}
	prevData := m.rightsizing.data
	ret, cmd := m.handleRightsizingOverlayKey(runeKey('['))
	r := ret.(Model)
	assert.Equal(t, model.StrategyVPA, r.rightsizing.strategy, "[ moves to the previous strategy")
	assert.NotNil(t, cmd)
	assert.Same(t, prevData, r.rightsizing.data, "stale data preserved during fetch")
}

func TestRightsizingOverlay_StrategyPickerWrapsForward(t *testing.T) {
	// At the last strategy, ] wraps to the first — vim-like cycling.
	m := newRightsizingTestModel()
	m.rightsizing.strategy = model.StrategySnapshot
	m.rightsizing.available = []model.RightsizingStrategy{
		model.StrategyVPA, model.StrategyPromMax1D, model.StrategySnapshot,
	}
	ret, _ := m.handleRightsizingOverlayKey(runeKey(']'))
	r := ret.(Model)
	assert.Equal(t, model.StrategyVPA, r.rightsizing.strategy, "after last → wraps to first")
}

func TestRightsizingOverlay_StrategyPickerWrapsBackward(t *testing.T) {
	m := newRightsizingTestModel()
	m.rightsizing.strategy = model.StrategyVPA
	m.rightsizing.available = []model.RightsizingStrategy{
		model.StrategyVPA, model.StrategyPromMax1D, model.StrategySnapshot,
	}
	ret, _ := m.handleRightsizingOverlayKey(runeKey('['))
	r := ret.(Model)
	assert.Equal(t, model.StrategySnapshot, r.rightsizing.strategy, "before first → wraps to last")
}

func TestUpdateOverlayRightsizing_StrategyPickerCacheHitInstant(t *testing.T) {
	// When the cache already has an entry for the (newStrategy,
	// headroom) pair, the picker swaps in place: no async cmd, loading
	// stays false, s.data points at the cached entry.
	m := newRightsizingTestModel()
	m.rightsizing.strategy = model.StrategyVPA
	m.rightsizing.available = []model.RightsizingStrategy{
		model.StrategyVPA, model.StrategyPromMax1D, model.StrategySnapshot,
	}
	cached := &model.Rightsizing{
		Source:   "1d-max",
		Strategy: model.StrategyPromMax1D,
		Headroom: m.rightsizing.headroom,
		Containers: []model.ContainerRec{{
			Name: "app",
			CPU:  model.ResourceRec{RecommendedRequest: "777m"},
		}},
	}
	cacheKey := rightsizingCacheKey("c", "ns", "Pod", "pod-a", model.StrategyPromMax1D, m.rightsizing.headroom)
	m.rightsizingCache[cacheKey] = cached

	ret, cmd := m.handleRightsizingOverlayKey(runeKey(']'))
	r := ret.(Model)
	assert.Equal(t, model.StrategyPromMax1D, r.rightsizing.strategy)
	assert.Nil(t, cmd, "cache hit → no async cmd queued")
	assert.False(t, r.rightsizing.loading, "cache hit → loading stays false (no flash)")
	assert.Same(t, cached, r.rightsizing.data, "s.data must point at the cached entry")
}

func TestUpdateOverlayRightsizing_StrategyPickerKeepsStaleDataDuringFetch(t *testing.T) {
	// Cache miss for the new (strategy, headroom) pair: previous
	// strategy's data stays in s.data while the new fetch runs. The
	// renderer combines `loading=true` + `data != nil` into a "fetching
	// new strategy, but you can still see the old numbers" view —
	// avoids the disorienting wipe-then-loading-flash on every press.
	m := newRightsizingTestModel()
	m.rightsizing.strategy = model.StrategyVPA
	m.rightsizing.available = []model.RightsizingStrategy{
		model.StrategyVPA, model.StrategyPromMax1D, model.StrategySnapshot,
	}
	prevData := m.rightsizing.data
	require.NotNil(t, prevData, "fixture must seed s.data so this test exercises the stale-data path")

	ret, cmd := m.handleRightsizingOverlayKey(runeKey(']'))
	r := ret.(Model)
	assert.NotNil(t, cmd, "cache miss → async cmd queued")
	assert.True(t, r.rightsizing.loading, "loading flips on while the new fetch runs")
	assert.Same(t, prevData, r.rightsizing.data, "previous strategy's data must remain visible — no wipe + flash")
	assert.Nil(t, r.rightsizing.err, "switching strategy clears any prior error")
}

func TestRightsizingOverlay_StrategyPickerNoOpWhenSingleAvailable(t *testing.T) {
	// When only one strategy is available, [ / ] should be no-ops —
	// no point firing a load that returns the same data.
	m := newRightsizingTestModel()
	m.rightsizing.strategy = model.StrategySnapshot
	m.rightsizing.available = []model.RightsizingStrategy{model.StrategySnapshot}
	ret, cmd := m.handleRightsizingOverlayKey(runeKey(']'))
	r := ret.(Model)
	assert.Equal(t, model.StrategySnapshot, r.rightsizing.strategy, "no movement with one strategy")
	assert.Nil(t, cmd, "no cmd dispatched when nothing changes")
}

// --- Headroom picker (< / >) ---

func TestRightsizingOverlay_HeadroomPickerNext(t *testing.T) {
	// > steps to the next preset value in model.RightsizingHeadrooms.
	// Starting at the default 1.25 → next is 1.5. With s.data populated
	// (the realistic mid-session case), the headroom picker now does a
	// LOCAL rescale instead of round-tripping through the cluster — so
	// no async load is queued and loading stays false.
	m := newRightsizingTestModel()
	m.rightsizing.headroom = model.DefaultRightsizingHeadroom // 1.25
	ret, cmd := m.handleRightsizingOverlayKey(runeKey('>'))
	r := ret.(Model)
	assert.InDelta(t, 1.5, r.rightsizing.headroom, 1e-9, "> moves to the next headroom")
	assert.Nil(t, cmd, "populated data → local rescale, no async cmd")
	assert.False(t, r.rightsizing.loading, "local rescale leaves loading off — table never flashes")
}

func TestRightsizingOverlay_HeadroomPickerPrev(t *testing.T) {
	m := newRightsizingTestModel()
	m.rightsizing.headroom = 1.5
	// Data fixture has Headroom: model.DefaultRightsizingHeadroom (1.25).
	// Bump it to 1.5 so the rescale path applies cleanly when < drops to 1.25.
	m.rightsizing.data.Headroom = 1.5
	ret, cmd := m.handleRightsizingOverlayKey(runeKey('<'))
	r := ret.(Model)
	assert.InDelta(t, 1.25, r.rightsizing.headroom, 1e-9, "< moves to the previous headroom")
	assert.Nil(t, cmd, "populated data → local rescale, no async cmd")
	assert.False(t, r.rightsizing.loading)
}

func TestRightsizingOverlay_HeadroomPickerWrapsForward(t *testing.T) {
	// At the last preset (2.0), > wraps back to the first (1.0).
	m := newRightsizingTestModel()
	m.rightsizing.headroom = 2.0
	ret, _ := m.handleRightsizingOverlayKey(runeKey('>'))
	r := ret.(Model)
	assert.InDelta(t, 1.0, r.rightsizing.headroom, 1e-9, "headroom > wraps from 2.0 to 1.0")
}

func TestRightsizingOverlay_HeadroomPickerWrapsBackward(t *testing.T) {
	// At the first preset (1.0), < wraps to the last (2.0) — vim cycle.
	m := newRightsizingTestModel()
	m.rightsizing.headroom = 1.0
	ret, _ := m.handleRightsizingOverlayKey(runeKey('<'))
	r := ret.(Model)
	assert.InDelta(t, 2.0, r.rightsizing.headroom, 1e-9, "headroom < wraps from 1.0 to 2.0")
}

func TestRightsizingOverlay_HeadroomPickerSnapsToNearestForward(t *testing.T) {
	// A headroom value not in the preset list (e.g. legacy 1.2) snaps
	// to the nearest neighbor in the press direction. > from 1.2 →
	// snap up to 1.25; not 1.5 (would skip the closer value).
	m := newRightsizingTestModel()
	m.rightsizing.headroom = 1.2
	ret, _ := m.handleRightsizingOverlayKey(runeKey('>'))
	r := ret.(Model)
	assert.InDelta(t, 1.25, r.rightsizing.headroom, 1e-9, "> from 1.2 snaps up to 1.25")
}

func TestRightsizingOverlay_HeadroomPickerSnapsToNearestBackward(t *testing.T) {
	m := newRightsizingTestModel()
	m.rightsizing.headroom = 1.2
	ret, _ := m.handleRightsizingOverlayKey(runeKey('<'))
	r := ret.(Model)
	assert.InDelta(t, 1.1, r.rightsizing.headroom, 1e-9, "< from 1.2 snaps down to 1.1")
}

func TestRightsizingOverlay_HeadroomPickerCacheKeyChanges(t *testing.T) {
	// Pressing > on the headroom picker advances state.headroom and the
	// resulting cache key for the new headroom must reflect 1.50 (not
	// 1.25). After the local-rescale rework no async cmd fires when
	// data is populated, but the new key is still the lookup the
	// rescaled entry is cached under.
	m := newRightsizingTestModel()
	m.rightsizing.headroom = model.DefaultRightsizingHeadroom // 1.25
	ret, _ := m.handleRightsizingOverlayKey(runeKey('>'))
	r := ret.(Model)
	expectedKey := rightsizingCacheKey("c", "ns", "Pod", "pod-a", r.rightsizing.strategy, 1.5)
	assert.Contains(t, expectedKey, "1.50")
	// Local-rescale path writes the rescaled payload into the cache
	// under the new key so a subsequent revisit is also instant.
	cached, ok := r.rightsizingCache[expectedKey]
	assert.True(t, ok, "local rescale must populate cache under the new headroom key")
	assert.NotNil(t, cached)
}

// --- Headroom picker fast paths (cache hit / local rescale / cold) ---

func TestUpdateOverlayRightsizing_HeadroomPickerCacheHitInstant(t *testing.T) {
	// When the cache already has an entry for the new (strategy,
	// headroom) pair, the headroom picker swaps to it in-place: no
	// async cmd, loading stays false, and s.data points at the cached
	// entry (not the rescaled-from-current value, so the user gets the
	// authoritative cluster-fresh numbers).
	m := newRightsizingTestModel()
	m.rightsizing.headroom = model.DefaultRightsizingHeadroom // 1.25
	cached := &model.Rightsizing{
		Headroom: 1.5,
		Containers: []model.ContainerRec{{
			Name: "app",
			CPU:  model.ResourceRec{RecommendedRequest: "999m"}, // distinguishable from a rescale of "60m"
		}},
	}
	cacheKey := rightsizingCacheKey("c", "ns", "Pod", "pod-a", m.rightsizing.strategy, 1.5)
	m.rightsizingCache[cacheKey] = cached

	ret, cmd := m.handleRightsizingOverlayKey(runeKey('>'))
	r := ret.(Model)
	assert.Nil(t, cmd, "cache hit → no async cmd queued")
	assert.False(t, r.rightsizing.loading, "cache hit → loading stays false (no flash)")
	assert.Same(t, cached, r.rightsizing.data, "s.data must point at the cached entry, not a rescale")
	assert.InDelta(t, 1.5, r.rightsizing.headroom, 1e-9, "state headroom advanced to 1.5")
}

func TestUpdateOverlayRightsizing_HeadroomPickerLocalRescaleInstant(t *testing.T) {
	// Cache miss but s.data is populated: the picker rescales locally
	// (newH/oldH multiplier on every recommendation) and never queues
	// an async load — the data is already in memory and a multiply is
	// instant.
	m := newRightsizingTestModel()
	m.rightsizing.headroom = model.DefaultRightsizingHeadroom // 1.25
	// Confirm data starts at headroom 1.25 (matches fixture).
	assert.InDelta(t, 1.25, m.rightsizing.data.Headroom, 1e-9)

	ret, cmd := m.handleRightsizingOverlayKey(runeKey('>'))
	r := ret.(Model)
	assert.Nil(t, cmd, "local rescale → no async cmd queued")
	assert.False(t, r.rightsizing.loading, "local rescale → loading stays false (no flash)")
	assert.InDelta(t, 1.5, r.rightsizing.headroom, 1e-9)
	require.NotNil(t, r.rightsizing.data)
	require.Len(t, r.rightsizing.data.Containers, 1)
	// Original CPU.RecommendedRequest = "60m" at headroom 1.25.
	// Rescale ratio 1.5/1.25 = 1.2 → 60m * 1.2 = 72m → snap up to 80m.
	assert.Equal(t, "80m", r.rightsizing.data.Containers[0].CPU.RecommendedRequest, "rec request scales locally")
	// Cache populated under the new key for future revisits.
	cacheKey := rightsizingCacheKey("c", "ns", "Pod", "pod-a", r.rightsizing.strategy, 1.5)
	_, ok := r.rightsizingCache[cacheKey]
	assert.True(t, ok, "rescaled payload cached under new key for future revisits")
}

func TestUpdateOverlayRightsizing_HeadroomPickerColdFallback(t *testing.T) {
	// Cache miss AND s.data is nil: the truly-cold path. Picker has
	// nothing to rescale and nothing to swap, so it falls through to
	// the async load — loading flips on, cmd is queued. This is the
	// rare path: executeActionRightsizing kicks the initial load on
	// overlay open, so by the time the user starts pressing </> the
	// data is almost always populated.
	m := newRightsizingTestModel()
	m.rightsizing.data = nil // simulate "first frame, no data yet"
	m.rightsizing.headroom = model.DefaultRightsizingHeadroom

	ret, cmd := m.handleRightsizingOverlayKey(runeKey('>'))
	r := ret.(Model)
	assert.NotNil(t, cmd, "cold path → async cmd queued")
	assert.True(t, r.rightsizing.loading, "cold path → loading flips on while fetch runs")
	assert.InDelta(t, 1.5, r.rightsizing.headroom, 1e-9)
}

// --- Race-condition guards on the picker fast paths ---
//
// These tests cover the three windows where a naive rescale or
// cache-hit short-circuit can be overwritten by a stale in-flight
// response from a previous overlay open / strategy switch.

func TestUpdateOverlayRightsizing_HeadroomPickerSkipsRescaleWhileLoading(t *testing.T) {
	// During a strategy switch the renderer keeps the previous
	// strategy's data on screen with loading=true. Pressing < / > in
	// that window must NOT rescale-and-cache the stale payload under
	// the new (strategy, headroom) key — the in-flight response will
	// land later and overwrite the user's selection. Fall through to
	// the async path instead and bump gen so the older fetch is
	// dropped on arrival.
	m := newRightsizingTestModel()
	m.rightsizing.headroom = model.DefaultRightsizingHeadroom
	m.rightsizing.loading = true // simulate mid-fetch from a prior strategy switch
	prevGen := m.rightsizing.gen

	ret, cmd := m.handleRightsizingOverlayKey(runeKey('>'))
	r := ret.(Model)
	assert.NotNil(t, cmd, "loading=true → must take the async fetch path, not rescale stale data")
	assert.True(t, r.rightsizing.loading)
	assert.Greater(t, r.rightsizing.gen, prevGen, "async path must bump gen so the prior fetch is dropped")
	// The new (strategy, 1.5) cache key must NOT be populated by a stale rescale.
	cacheKey := rightsizingCacheKey("c", "ns", "Pod", "pod-a", r.rightsizing.strategy, 1.5)
	_, ok := r.rightsizingCache[cacheKey]
	assert.False(t, ok, "loading=true rescale would poison the cache under the new key — must not happen")
}

func TestUpdateOverlayRightsizing_HeadroomPickerSkipsRescaleOnStrategyMismatch(t *testing.T) {
	// If the in-memory data was generated for a different strategy
	// (e.g. the user pressed `]` then immediately `>`), rescaling it
	// would store a cross-strategy rescale under the new key. Skip the
	// fast path and fetch fresh.
	m := newRightsizingTestModel()
	m.rightsizing.headroom = model.DefaultRightsizingHeadroom
	m.rightsizing.strategy = model.StrategyVPA
	// Data was generated for the SNAPSHOT strategy (mismatch).
	m.rightsizing.data.Strategy = model.StrategySnapshot
	prevGen := m.rightsizing.gen

	ret, cmd := m.handleRightsizingOverlayKey(runeKey('>'))
	r := ret.(Model)
	assert.NotNil(t, cmd, "strategy mismatch → must take async path, not rescale across strategies")
	assert.Greater(t, r.rightsizing.gen, prevGen)
	cacheKey := rightsizingCacheKey("c", "ns", "Pod", "pod-a", r.rightsizing.strategy, 1.5)
	_, ok := r.rightsizingCache[cacheKey]
	assert.False(t, ok, "cross-strategy rescale would poison the cache — must not happen")
}

func TestUpdateOverlayRightsizing_HeadroomPickerSkipsRescaleOnHeadroomMismatch(t *testing.T) {
	// If the in-memory payload's recorded headroom doesn't match the
	// current one, the rescale ratio (newH/data.Headroom) would scale
	// from the wrong baseline. Fall through to async fetch instead.
	m := newRightsizingTestModel()
	m.rightsizing.headroom = model.DefaultRightsizingHeadroom // 1.25
	// Data was generated at headroom 2.0 — doesn't match current 1.25.
	m.rightsizing.data.Headroom = 2.0
	prevGen := m.rightsizing.gen

	ret, cmd := m.handleRightsizingOverlayKey(runeKey('>'))
	r := ret.(Model)
	assert.NotNil(t, cmd, "headroom mismatch → must take async path, not rescale from wrong baseline")
	assert.Greater(t, r.rightsizing.gen, prevGen)
}

func TestUpdateOverlayRightsizing_StrategyPickerCacheHitBumpsGen(t *testing.T) {
	// A cache hit on the new strategy must bump m.rightsizing.gen so
	// any in-flight prior strategy fetch is dropped on arrival rather
	// than overwriting the cache hit.
	m := newRightsizingTestModel()
	m.rightsizing.strategy = model.StrategyVPA
	m.rightsizing.available = []model.RightsizingStrategy{
		model.StrategyVPA, model.StrategyPromMax1D, model.StrategySnapshot,
	}
	cached := &model.Rightsizing{
		Strategy: model.StrategyPromMax1D,
		Headroom: m.rightsizing.headroom,
		Containers: []model.ContainerRec{{
			Name: "app",
			CPU:  model.ResourceRec{RecommendedRequest: "777m"},
		}},
	}
	cacheKey := rightsizingCacheKey("c", "ns", "Pod", "pod-a", model.StrategyPromMax1D, m.rightsizing.headroom)
	m.rightsizingCache[cacheKey] = cached
	prevGen := m.rightsizing.gen

	ret, _ := m.handleRightsizingOverlayKey(runeKey(']'))
	r := ret.(Model)
	assert.Greater(t, r.rightsizing.gen, prevGen,
		"strategy cache hit must bump gen to invalidate any in-flight prior fetch")
	assert.Same(t, cached, r.rightsizing.data, "cache hit still wins the data slot")
}

func TestUpdateOverlayRightsizing_HeadroomPickerCacheHitBumpsGen(t *testing.T) {
	// Same race guard for the headroom fast-path: a cache hit must
	// bump gen so the prior in-flight fetch can't overwrite it.
	m := newRightsizingTestModel()
	m.rightsizing.headroom = model.DefaultRightsizingHeadroom
	cached := &model.Rightsizing{
		Strategy: m.rightsizing.strategy,
		Headroom: 1.5,
		Containers: []model.ContainerRec{{
			Name: "app",
			CPU:  model.ResourceRec{RecommendedRequest: "999m"},
		}},
	}
	cacheKey := rightsizingCacheKey("c", "ns", "Pod", "pod-a", m.rightsizing.strategy, 1.5)
	m.rightsizingCache[cacheKey] = cached
	prevGen := m.rightsizing.gen

	ret, _ := m.handleRightsizingOverlayKey(runeKey('>'))
	r := ret.(Model)
	assert.Greater(t, r.rightsizing.gen, prevGen,
		"headroom cache hit must bump gen to invalidate any in-flight prior fetch")
	assert.Same(t, cached, r.rightsizing.data, "cache hit still wins the data slot")
}

// --- Headroom seeded on overlay open ---

func TestExecuteActionRightsizing_SeedsDefaultHeadroom(t *testing.T) {
	// Opening the overlay must seed state.headroom to the configured
	// default so the [N/M] picker chip renders a real position on the
	// first frame instead of "?". The default is the value closest to
	// the previous hardcoded 1.2 factor (= 1.25).
	m := Model{
		actionCtx:        actionContext{context: "c", namespace: "ns", kind: "Pod", name: "pod-a"},
		rightsizingCache: make(map[string]*model.Rightsizing),
		client:           newRightsizingTestClient(t),
	}
	ret, _ := m.executeActionRightsizing()
	r := ret.(Model)
	assert.InDelta(t, model.DefaultRightsizingHeadroom, r.rightsizing.headroom, 1e-9,
		"opening the overlay must seed headroom to model.DefaultRightsizingHeadroom")
}

// newRightsizingTestClient builds a minimal fake k8s.Client suitable
// for handler-level tests that exercise overlay open/close paths but
// don't need real fetches to succeed.
func newRightsizingTestClient(_ *testing.T) *k8s.Client {
	return k8s.NewTestClient(nil, nil)
}

// silence unused err helper
var _ = errors.New
