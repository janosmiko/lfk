package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/scheduler"
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

// The scheduler folds a resubmission into a running task with the same
// identity, so a shared name would leave a `]` press with no fetch (#705).
func TestLoadRightsizing_StrategyAndHeadroomChangeTaskIdentity(t *testing.T) {
	m := newRightsizingTestModel()
	m.rightsizing.data = nil
	m.scheduler = scheduler.New(0)

	require.NotNil(t, m.loadRightsizing())
	m.rightsizing.strategy = model.StrategySnapshot
	require.NotNil(t, m.loadRightsizing())
	m.rightsizing.headroom = 1.5
	require.NotNil(t, m.loadRightsizing())

	names := map[string]bool{}
	for _, e := range m.scheduler.QueueSnapshot() {
		names[e.Name] = true
	}
	assert.Len(t, names, 3, "each strategy/headroom pair must be its own scheduler task")
}

func TestUpdateRightsizingLoaded_OtherKeyOnlyWarmsCache(t *testing.T) {
	m := newRightsizingTestModel()
	existing := m.rightsizing.data
	m.rightsizing.loading = true

	other := rightsizingLoadedMsg{
		key:  "ctx/ns/Pod/p",
		data: &model.Rightsizing{Source: "estimated"},
	}
	r := m.updateRightsizingLoaded(other)
	assert.Same(t, existing, r.rightsizing.data, "other-key msg must NOT replace current data")
	assert.True(t, r.rightsizing.loading, "other-key msg must NOT end the current fetch's loading state")
	assert.Same(t, other.data, r.rightsizingCache["ctx/ns/Pod/p"], "other-key msg still warms the cache")
}

// The scheduler may have folded a newer submission into an older fetch,
// so the gen that dispatched a result must not decide whether it is shown.
func TestUpdateRightsizingLoaded_MatchingKeyAcceptedAcrossGen(t *testing.T) {
	m := newRightsizingTestModel()
	m.rightsizing.data = nil
	m.rightsizing.loading = true
	m.rightsizing.gen = 5
	key := rightsizingCacheKey("c", "ns", "Pod", "pod-a", model.StrategyVPA, model.DefaultRightsizingHeadroom)

	fresh := rightsizingLoadedMsg{
		key:  key,
		data: &model.Rightsizing{Source: "VPA", PodCount: 3},
	}
	r := m.updateRightsizingLoaded(fresh)
	assert.False(t, r.rightsizing.loading, "matching key ends the loading state")
	assert.Equal(t, "VPA", r.rightsizing.data.Source)
	cached := r.rightsizingCache[key]
	assert.NotNil(t, cached)
	assert.Equal(t, 3, cached.PodCount)
}

func TestUpdateRightsizingLoaded_ErrorSurfacedNotCached(t *testing.T) {
	m := newRightsizingTestModel()
	m.rightsizing.data = nil
	m.rightsizing.loading = true
	key := rightsizingCacheKey("c", "ns", "Pod", "pod-a", model.StrategyVPA, model.DefaultRightsizingHeadroom)

	errMsg := rightsizingLoadedMsg{
		key: key,
		err: assert.AnError,
	}
	r := m.updateRightsizingLoaded(errMsg)
	assert.False(t, r.rightsizing.loading)
	assert.NotNil(t, r.rightsizing.err)
	_, present := r.rightsizingCache[key]
	assert.False(t, present, "error response must NOT pollute the cache")
}
