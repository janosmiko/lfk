package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestLoadRightsizing_CacheHitReturnsSync(t *testing.T) {
	m := Model{
		actionCtx:        actionContext{context: "ctx-a", namespace: "default", kind: "Pod", name: "pod-x"},
		rightsizingCache: make(map[string]*model.Rightsizing),
	}
	m.rightsizing.strategy = model.StrategyVPA
	m.rightsizing.headroom = model.DefaultRightsizingHeadroom
	key := rightsizingCacheKey("ctx-a", "default", "Pod", "pod-x", model.StrategyVPA, model.DefaultRightsizingHeadroom)
	cached := &model.Rightsizing{Source: "VPA", PodCount: 1}
	m.rightsizingCache[key] = cached
	m.rightsizing.gen = 7

	cmd := m.loadRightsizing()
	assert.NotNil(t, cmd, "cache hit must still emit the msg so the handler runs uniformly")
	msg := cmd().(rightsizingLoadedMsg)
	assert.Equal(t, key, msg.key)
	assert.Same(t, cached, msg.data, "cache hit returns the same pointer (no copy)")
	assert.NoError(t, msg.err)
	assert.Equal(t, 7, msg.generation, "generation captured at dispatch time")
}

func TestRightsizingCacheKey_IncludesStrategy(t *testing.T) {
	// Switching strategies on the same workload must NOT collide in
	// the cache — otherwise pressing > would sometimes return the
	// previous strategy's payload because both keys hashed to the same
	// string.
	a := rightsizingCacheKey("c", "ns", "Pod", "p", model.StrategyVPA, 1.25)
	b := rightsizingCacheKey("c", "ns", "Pod", "p", model.StrategyPromMax1D, 1.25)
	assert.NotEqual(t, a, b, "same workload + different strategy must yield different cache keys")
	assert.Contains(t, a, string(model.StrategyVPA))
	assert.Contains(t, b, string(model.StrategyPromMax1D))
}

func TestRightsizingCacheKey_IncludesHeadroom(t *testing.T) {
	// Cycling headroom on the same workload + strategy must produce
	// different cache keys — otherwise pressing `>` (next headroom)
	// would return the cached payload from the previous multiplier and
	// the user would see stale numbers without realizing it.
	a := rightsizingCacheKey("c", "ns", "Pod", "p", model.StrategyVPA, 1.25)
	b := rightsizingCacheKey("c", "ns", "Pod", "p", model.StrategyVPA, 1.5)
	assert.NotEqual(t, a, b, "same workload+strategy + different headroom must yield different cache keys")
	// The chosen format is %.2f for stability ("1.25", "1.50") rather
	// than %g ("1.25", "1.5") — keeps the key shape constant across
	// values so a future log/grep reader can see the headroom column
	// without surprises.
	assert.Contains(t, a, "1.25")
	assert.Contains(t, b, "1.50")
}

func TestLoadRightsizing_NoActionContext_ReturnsNil(t *testing.T) {
	m := Model{} // empty actionCtx
	cmd := m.loadRightsizing()
	assert.Nil(t, cmd, "no actionCtx → no fetch")
}

func TestUpdateRightsizingLoaded_StaleGenDiscarded(t *testing.T) {
	m := Model{rightsizingCache: make(map[string]*model.Rightsizing)}
	m.rightsizing.gen = 5
	existing := &model.Rightsizing{Source: "VPA", PodCount: 2}
	m.rightsizing.data = existing

	stale := rightsizingLoadedMsg{
		key:        "ctx/ns/Pod/p",
		data:       &model.Rightsizing{Source: "estimated"},
		generation: 3, // older than current gen
	}
	r := m.updateRightsizingLoaded(stale)
	assert.Same(t, existing, r.rightsizing.data, "stale msg must NOT replace current data")
	_, present := r.rightsizingCache["ctx/ns/Pod/p"]
	assert.False(t, present, "stale msg must NOT populate cache")
}

func TestUpdateRightsizingLoaded_FreshSetsDataAndCaches(t *testing.T) {
	m := Model{rightsizingCache: make(map[string]*model.Rightsizing)}
	m.rightsizing.gen = 5
	m.rightsizing.loading = true

	fresh := rightsizingLoadedMsg{
		key:        "ctx/ns/Pod/p",
		data:       &model.Rightsizing{Source: "VPA", PodCount: 3},
		generation: 5, // matches current gen
	}
	r := m.updateRightsizingLoaded(fresh)
	assert.False(t, r.rightsizing.loading, "fresh msg flips loading off")
	assert.Equal(t, "VPA", r.rightsizing.data.Source)
	cached := r.rightsizingCache["ctx/ns/Pod/p"]
	assert.NotNil(t, cached)
	assert.Equal(t, 3, cached.PodCount)
}

func TestUpdateRightsizingLoaded_ErrorSurfacedNotCached(t *testing.T) {
	m := Model{rightsizingCache: make(map[string]*model.Rightsizing)}
	m.rightsizing.gen = 5
	m.rightsizing.loading = true

	errMsg := rightsizingLoadedMsg{
		key:        "ctx/ns/Pod/p",
		err:        assert.AnError,
		generation: 5,
	}
	r := m.updateRightsizingLoaded(errMsg)
	assert.False(t, r.rightsizing.loading)
	assert.NotNil(t, r.rightsizing.err)
	_, present := r.rightsizingCache["ctx/ns/Pod/p"]
	assert.False(t, present, "error response must NOT pollute the cache")
}
