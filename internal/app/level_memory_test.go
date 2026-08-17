package app

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// levelMemoryModel returns a model sitting on a pod list in "test-ctx" with
// the parent columns filled in, so navigateParent can walk up to the cluster
// picker the way the real app does.
func levelMemoryModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.nav.Namespace = "default"
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	m.leftItems = []model.Item{{Name: "Pods", Kind: "Pod"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "test-ctx"}}}
	return m
}

func TestLevelKeyClusterThenResourcesReturnsToTheSameList(t *testing.T) {
	m := levelMemoryModel(t)

	up, _, handled := m.handleExplorerActionKeyLevelCluster()
	require.True(t, handled)
	atClusters := up.(Model)
	require.Equal(t, model.LevelClusters, atClusters.nav.Level)

	back, _, handled := atClusters.handleExplorerActionKeyLevelResources()
	require.True(t, handled)
	bm := back.(Model)

	assert.Equal(t, model.LevelResources, bm.nav.Level, "2 must return to the resource list")
	assert.Equal(t, "test-ctx", bm.nav.Context)
	assert.Equal(t, "Pod", bm.nav.ResourceType.Kind, "the resource type must be restored")
}

func TestLevelKeyClusterThenTypesReturnsToTheClusterLeft(t *testing.T) {
	m := levelMemoryModel(t)

	up, _, _ := m.handleExplorerActionKeyLevelCluster()
	back, _, handled := up.(Model).handleExplorerActionKeyLevelTypes()
	require.True(t, handled)
	bm := back.(Model)

	assert.Equal(t, model.LevelResourceTypes, bm.nav.Level, "1 must return to the type list")
	assert.Equal(t, "test-ctx", bm.nav.Context, "the cluster the user left must be restored")
}

func TestLevelMemoryIsPerCluster(t *testing.T) {
	m := levelMemoryModel(t)

	up, _, _ := m.handleExplorerActionKeyLevelCluster()
	other := up.(Model)
	// The user enters a different cluster by hand. Its own memory is empty,
	// so 2 must not replay the pod list of the cluster they left.
	other.nav.Level = model.LevelResourceTypes
	other.nav.Context = "other-ctx"

	back, _, handled := other.handleExplorerActionKeyLevelResources()
	require.True(t, handled)
	bm := back.(Model)

	assert.Equal(t, model.LevelResourceTypes, bm.nav.Level, "another cluster's memory must not apply")
	assert.Equal(t, "other-ctx", bm.nav.Context)
}

func TestLevelKeysWithoutMemoryKeepCurrentBehaviour(t *testing.T) {
	m := basePush80Model()
	m.nav = model.NavigationState{Level: model.LevelClusters}

	for _, tc := range []struct {
		name string
		key  func(Model) (any, any, bool)
	}{
		{"types", func(mm Model) (any, any, bool) {
			r, c, h := mm.handleExplorerActionKeyLevelTypes()
			return r, c, h
		}},
		{"resources", func(mm Model) (any, any, bool) {
			r, c, h := mm.handleExplorerActionKeyLevelResources()
			return r, c, h
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ret, _, handled := tc.key(m)
			require.True(t, handled)
			rm := ret.(Model)
			assert.Equal(t, model.LevelClusters, rm.nav.Level, "level must not change")
			assert.Empty(t, rm.statusMessage, "an empty memory must not raise an error")
		})
	}
}

func TestLevelMemorySkipsUnionMode(t *testing.T) {
	m := levelMemoryModel(t)
	m.unionMode = true
	m.nav.Context = UnionContextSentinel

	up, _, _ := m.handleExplorerActionKeyLevelCluster()

	assert.Empty(t, up.(Model).levelMem.views,
		"a union snapshot carries the sentinel context and could only restore as an error")
}

func TestLevelMemoryStaysUnderItsCap(t *testing.T) {
	m := levelMemoryModel(t)
	for i := range levelMemoryCap * 2 {
		m.nav.Level = model.LevelResources
		m.nav.Context = fmt.Sprintf("ctx-%d", i)
		m.rememberLevel()
	}

	assert.Len(t, m.levelMem.views, levelMemoryCap, "the oldest entries must be evicted")
	assert.Len(t, m.levelMem.order, levelMemoryCap)
	_, oldest := m.levelMem.views[levelMemoryKey{context: "ctx-0", level: model.LevelResources}]
	assert.False(t, oldest, "the first cluster recorded must be gone")
}

func TestNewTabStartsWithoutTheLevelMemoryOfItsSource(t *testing.T) {
	m := levelMemoryModel(t)
	up, _, _ := m.handleExplorerActionKeyLevelCluster()
	source := up.(Model)
	require.NotEmpty(t, source.levelMem.views, "precondition: the source tab remembers a view")

	ret, _, handled := source.handleExplorerActionKeyNewTab()
	require.True(t, handled)
	fresh := ret.(Model)

	assert.Empty(t, fresh.levelMem.views, "a new tab must not inherit where another tab has been")
	assert.Empty(t, fresh.jumpBackStack, "the same holds for the teleport history")
}

func TestLevelMemoryFollowsItsTab(t *testing.T) {
	m := levelMemoryModel(t)
	up, _, _ := m.handleExplorerActionKeyLevelCluster()
	saved := up.(Model)
	require.NotEmpty(t, saved.levelMem.views, "precondition: the jump recorded a memory")

	saved.saveCurrentTab()
	saved.levelMem.views = nil
	saved.levelMem.lastContext = ""
	saved.loadTab(0)

	assert.Equal(t, "test-ctx", saved.levelMem.lastContext, "the tab must carry the cluster back")
	assert.NotEmpty(t, saved.levelMem.views, "the tab must carry the remembered levels back")
}
