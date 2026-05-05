package app

import (
	"sync"
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestModel returns a minimal Model with the orphan cache maps initialized,
// sufficient for unit-testing cmdLoadOrphans and handleOrphansLoaded without a
// real Kubernetes cluster.
func newTestModel() Model {
	return Model{
		nav:                 model.NavigationState{Level: model.LevelResources},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		orphanCache:         make(map[orphanCacheKey]*k8s.OrphanReport),
		orphanLoadInflight:  make(map[orphanCacheKey]orphanInflight),
		width:               80,
		height:              40,
		execMu:              &sync.Mutex{},
	}
}

func TestCmdLoadOrphans_StoresInCacheOnSuccess(t *testing.T) {
	m := newTestModel()
	key := orphanCacheKey{kubeContext: "test", namespace: ""}

	// Simulate cmdLoadOrphans having registered an inflight entry —
	// handleOrphansLoaded ignores results without a matching gen, so
	// the test must record one before delivering the message.
	cmd := m.cmdLoadOrphans(key)
	require.NotNil(t, cmd)
	gen := m.orphanLoadInflight[key].gen

	msg := orphansLoadedMsg{
		key:    key,
		gen:    gen,
		report: k8s.OrphanReport{Pods: []k8s.OrphanItem{{Kind: "Pod", Name: "naked"}}},
	}

	updated, _ := m.handleOrphansLoaded(msg)

	require.NotNil(t, updated.orphanCache[key])
	assert.Equal(t, "naked", updated.orphanCache[key].Pods[0].Name)
	_, stillInflight := updated.orphanLoadInflight[key]
	assert.False(t, stillInflight, "inflight entry must be cleared on completion")
}

func TestCmdLoadOrphans_DedupesInflight(t *testing.T) {
	m := newTestModel()
	key := orphanCacheKey{kubeContext: "test", namespace: "default"}

	cmd1 := m.cmdLoadOrphans(key)
	require.NotNil(t, cmd1, "first call must return a Cmd")
	_, ok := m.orphanLoadInflight[key]
	require.True(t, ok, "first call must record an inflight entry")

	cmd2 := m.cmdLoadOrphans(key)
	assert.Nil(t, cmd2, "second call must dedupe while inflight")

	_ = cmd1
}

func TestHandleOrphansLoaded_DropsSupersededResult(t *testing.T) {
	m := newTestModel()
	key := orphanCacheKey{kubeContext: "test", namespace: ""}

	// Start a scan, then invalidate before its result arrives. The
	// late completion should not repopulate the cache — the user has
	// already moved on.
	require.NotNil(t, m.cmdLoadOrphans(key))
	gen := m.orphanLoadInflight[key].gen
	m.invalidateOrphanCacheForContext("test")

	msg := orphansLoadedMsg{
		key:    key,
		gen:    gen,
		report: k8s.OrphanReport{Pods: []k8s.OrphanItem{{Kind: "Pod", Name: "stale"}}},
	}
	updated, _ := m.handleOrphansLoaded(msg)

	assert.Nil(t, updated.orphanCache[key], "superseded result must not write to cache")
}

func TestInvalidateOrphanCacheForNamespace(t *testing.T) {
	m := newTestModel()
	m.orphanCache[orphanCacheKey{kubeContext: "ctxA", namespace: "ns1"}] = &k8s.OrphanReport{}
	m.orphanCache[orphanCacheKey{kubeContext: "ctxA", namespace: "ns2"}] = &k8s.OrphanReport{}
	m.orphanCache[orphanCacheKey{kubeContext: "ctxA", namespace: ""}] = &k8s.OrphanReport{}
	m.orphanCache[orphanCacheKey{kubeContext: "ctxB", namespace: "ns1"}] = &k8s.OrphanReport{}

	m.invalidateOrphanCacheForNamespace("ctxA", "ns1")

	assert.NotContains(t, m.orphanCache, orphanCacheKey{kubeContext: "ctxA", namespace: "ns1"})
	assert.Contains(t, m.orphanCache, orphanCacheKey{kubeContext: "ctxA", namespace: "ns2"})
	assert.Contains(t, m.orphanCache, orphanCacheKey{kubeContext: "ctxA", namespace: ""}, "cluster-wide entry preserved")
	assert.Contains(t, m.orphanCache, orphanCacheKey{kubeContext: "ctxB", namespace: "ns1"})
}

func TestInvalidateOrphanCacheForContext(t *testing.T) {
	m := newTestModel()
	m.orphanCache[orphanCacheKey{kubeContext: "ctxA", namespace: "ns1"}] = &k8s.OrphanReport{}
	m.orphanCache[orphanCacheKey{kubeContext: "ctxA", namespace: ""}] = &k8s.OrphanReport{}
	m.orphanCache[orphanCacheKey{kubeContext: "ctxB", namespace: "ns1"}] = &k8s.OrphanReport{}

	m.invalidateOrphanCacheForContext("ctxA")

	assert.NotContains(t, m.orphanCache, orphanCacheKey{kubeContext: "ctxA", namespace: "ns1"})
	assert.NotContains(t, m.orphanCache, orphanCacheKey{kubeContext: "ctxA", namespace: ""})
	assert.Contains(t, m.orphanCache, orphanCacheKey{kubeContext: "ctxB", namespace: "ns1"})
}
