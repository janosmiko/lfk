package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// jumpOwnerTestModel seeds discovery with Pod/ReplicaSet/Deployment/
// StatefulSet so navigateToOwner can resolve every hop of a
// Pod -> ReplicaSet -> Deployment chain, plus a sibling StatefulSet.
func jumpOwnerTestModel() Model {
	m := basePush80Model()
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
		{Kind: "ReplicaSet", APIGroup: "apps", APIVersion: "v1", Resource: "replicasets", Namespaced: true},
		{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true},
		{Kind: "StatefulSet", APIGroup: "apps", APIVersion: "v1", Resource: "statefulsets", Namespaced: true},
	}
	sidebar := model.BuildSidebarItems(m.discoveredResources["test-ctx"])
	m.leftItemsHistory = [][]model.Item{{{Name: "test-ctx"}}}
	m.leftItems = sidebar
	// A collapsed accordion with a different group expanded reproduces
	// issue #748: the fix must reveal ReplicaSet's own "Workloads" group
	// rather than leaving the jump silently landing on the wrong row.
	m.allGroupsExpanded = false
	m.expandedGroup = "Dashboards"
	return m
}

func TestActionKeyJumpOwnerPodToReplicaSet(t *testing.T) {
	m := jumpOwnerTestModel()
	m.middleItems = []model.Item{
		{
			Name: "myapp-7c9f9b6d8f-abcde", Namespace: "default", Kind: "Pod", Status: "Running",
			Columns: []model.KeyValue{
				{Key: "owner:0", Value: "apps/v1||ReplicaSet||myapp-7c9f9b6d8f"},
			},
		},
	}
	m.setCursor(0)

	ret, cmd, handled := m.handleExplorerActionKeyJumpOwner()
	assert.True(t, handled)
	require.NotNil(t, cmd, "a collapsed accordion group must not silently swallow the jump")
	rm := ret.(Model)
	assert.False(t, rm.statusMessageErr, "status: %q", rm.statusMessage)
	assert.Equal(t, model.LevelResources, rm.nav.Level)
	assert.Equal(t, "ReplicaSet", rm.nav.ResourceType.Kind)
	assert.Equal(t, "myapp-7c9f9b6d8f", rm.pendingTarget)
	assert.Equal(t, "Workloads", rm.expandedGroup, "the target's own group must be revealed")
}

func TestActionKeyJumpOwnerReplicaSetToDeployment(t *testing.T) {
	m := jumpOwnerTestModel()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "ReplicaSet", APIGroup: "apps", APIVersion: "v1", Resource: "replicasets", Namespaced: true}
	m.middleItems = []model.Item{
		{
			Name: "myapp-7c9f9b6d8f", Namespace: "default", Kind: "ReplicaSet", Status: "1/1",
			Columns: []model.KeyValue{
				{Key: "owner:0", Value: "apps/v1||Deployment||myapp"},
			},
		},
	}
	m.setCursor(0)

	ret, cmd, handled := m.handleExplorerActionKeyJumpOwner()
	assert.True(t, handled)
	require.NotNil(t, cmd)
	rm := ret.(Model)
	assert.False(t, rm.statusMessageErr, "status: %q", rm.statusMessage)
	assert.Equal(t, model.LevelResources, rm.nav.Level)
	assert.Equal(t, "Deployment", rm.nav.ResourceType.Kind)
	assert.Equal(t, "myapp", rm.pendingTarget)
}

func TestActionKeyJumpOwnerStatefulSetStillWorksWithAccordionCollapsed(t *testing.T) {
	m := jumpOwnerTestModel()
	m.middleItems = []model.Item{
		{
			Name: "myapp-0", Namespace: "default", Kind: "Pod", Status: "Running",
			Columns: []model.KeyValue{
				{Key: "owner:0", Value: "apps/v1||StatefulSet||myapp"},
			},
		},
	}
	m.setCursor(0)

	ret, cmd, handled := m.handleExplorerActionKeyJumpOwner()
	assert.True(t, handled)
	require.NotNil(t, cmd)
	rm := ret.(Model)
	assert.False(t, rm.statusMessageErr, "status: %q", rm.statusMessage)
	assert.Equal(t, model.LevelResources, rm.nav.Level)
	assert.Equal(t, "StatefulSet", rm.nav.ResourceType.Kind)
}

func TestActionKeyJumpOwnerHiddenTypeSetsStatusInsteadOfNoop(t *testing.T) {
	defer func(orig []string) { model.HiddenTypes = orig }(model.HiddenTypes)
	defer func(orig bool) { model.ShowRareResources = orig }(model.ShowRareResources)
	model.ShowRareResources = false
	model.HiddenTypes = []string{"apps/replicasets"}

	m := jumpOwnerTestModel()
	m.leftItems = model.BuildSidebarItems(m.discoveredResources["test-ctx"])
	m.middleItems = []model.Item{
		{
			Name: "myapp-7c9f9b6d8f-abcde", Namespace: "default", Kind: "Pod", Status: "Running",
			Columns: []model.KeyValue{
				{Key: "owner:0", Value: "apps/v1||ReplicaSet||myapp-7c9f9b6d8f"},
			},
		},
	}
	m.setCursor(0)

	ret, cmd, handled := m.handleExplorerActionKeyJumpOwner()
	assert.True(t, handled)
	require.NotNil(t, cmd)
	rm := ret.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotEmpty(t, rm.statusMessage)
}

func TestActionKeyJumpOwnerUnknownKindNotInSidebarSetsStatus(t *testing.T) {
	m := jumpOwnerTestModel()
	m.middleItems = []model.Item{
		{
			Name: "orphan-pod", Namespace: "default", Kind: "Pod", Status: "Running",
			Columns: []model.KeyValue{
				{Key: "owner:0", Value: "custom.example.com/v1||WidgetController||my-widget"},
			},
		},
	}
	m.setCursor(0)

	ret, cmd, handled := m.handleExplorerActionKeyJumpOwner()
	assert.True(t, handled)
	require.NotNil(t, cmd)
	rm := ret.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotEmpty(t, rm.statusMessage)
}
