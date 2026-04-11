package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// --- itemIndexFromDisplayLine ---

func TestItemIndexFromDisplayLine(t *testing.T) {
	t.Run("single category with items", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "pod-a"},
				{Name: "pod-b"},
				{Name: "pod-c"},
			},
		}
		// No categories, so display lines map 1:1 to items.
		assert.Equal(t, 0, m.itemIndexFromDisplayLine(0))
		assert.Equal(t, 1, m.itemIndexFromDisplayLine(1))
		assert.Equal(t, 2, m.itemIndexFromDisplayLine(2))
	})

	t.Run("display line out of range returns -1", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "pod-a"},
			},
		}
		assert.Equal(t, -1, m.itemIndexFromDisplayLine(100))
	})

	t.Run("empty items returns -1", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
		}
		assert.Equal(t, -1, m.itemIndexFromDisplayLine(0))
	})

	t.Run("items with categories include headers and separators", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResourceTypes},
			middleItems: []model.Item{
				{Name: "Pods", Category: "Workloads"},
				{Name: "Deployments", Category: "Workloads"},
				{Name: "Services", Category: "Networking"},
			},
			allGroupsExpanded: true,
		}
		// Display lines with categories:
		// 0: "Workloads" header
		// 1: Pods
		// 2: Deployments
		// 3: separator
		// 4: "Networking" header
		// 5: Services
		idx := m.itemIndexFromDisplayLine(1) // Pods
		assert.Equal(t, 0, idx)
	})
}

func TestCov80KubectlGetPodSelectorCronJob(t *testing.T) {
	// CronJob always returns empty.
	result := kubectlGetPodSelector("/usr/bin/kubectl", "/dev/null", "default", "CronJob", "my-cron", "test-ctx")
	assert.Empty(t, result)
}

func TestCov80NavigateParentFromClusters(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	result, cmd := m.navigateParent()
	rm := result.(Model)
	assert.Equal(t, model.LevelClusters, rm.nav.Level)
	assert.Nil(t, cmd)
}

func TestCov80NavigateParentFromResourceTypes(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.leftItems = []model.Item{{Name: "ctx-1"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "root"}}}
	result, cmd := m.navigateParent()
	rm := result.(Model)
	assert.Equal(t, model.LevelClusters, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCov80NavigateParentFromResources(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.leftItems = []model.Item{{Name: "Pods"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "cluster"}}}
	result, _ := m.navigateParent()
	rm := result.(Model)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
}

func TestCov80NavigateParentFromOwned(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceName = "deploy-1"
	m.leftItems = []model.Item{{Name: "deploy-1"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "cluster"}}, {{Name: "Deployments"}}}
	result, cmd := m.navigateParent()
	rm := result.(Model)
	assert.Equal(t, model.LevelResources, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCov80NavigateParentFromOwnedWithStack(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceName = "deploy-1"
	m.ownedParentStack = []ownedParentState{
		{
			resourceType: model.ResourceTypeEntry{Kind: "Application"},
			resourceName: "my-app",
			namespace:    "argocd",
		},
	}
	m.leftItems = []model.Item{{Name: "deploy-1"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "cluster"}}, {{Name: "Apps"}}, {{Name: "app-1"}}}
	result, cmd := m.navigateParent()
	rm := result.(Model)
	// Should pop to parent owned level.
	assert.Equal(t, model.LevelOwned, rm.nav.Level)
	assert.Equal(t, "my-app", rm.nav.ResourceName)
	assert.NotNil(t, cmd)
}

func TestCov80NavigateParentFromContainersPod(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.leftItems = []model.Item{{Name: "container-1"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "cluster"}}, {{Name: "Pods"}}, {{Name: "pod-1"}}}
	result, cmd := m.navigateParent()
	rm := result.(Model)
	// Pod containers go back to LevelResources.
	assert.Equal(t, model.LevelResources, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCov80NavigateParentFromContainersOther(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment"}
	m.leftItems = []model.Item{{Name: "container-1"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "cluster"}}, {{Name: "Deploys"}}, {{Name: "dep-1"}}}
	result, cmd := m.navigateParent()
	rm := result.(Model)
	// Non-pod containers go back to LevelOwned.
	assert.Equal(t, model.LevelOwned, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCov80NavigateChildNoSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	result, cmd := m.navigateChild()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80EnterFullViewNoSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	result, cmd := m.enterFullView()
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80NavigateToOwnerUnknownKind(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	result, cmd := m.navigateToOwner("UnknownKind999", "my-resource")
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestCovBoost2NavigateChildClusters(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "test-ctx"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
}

func TestCovBoost2NavigateChildResourceTypes(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.middleItems = []model.Item{{Name: "Pods", Extra: "v1/pods"}}
	result, _ := m.navigateChild()
	_ = result
}

func TestCovBoost2NavigateChildResourceTypesOverview(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.True(t, rm.fullscreenDashboard)
	assert.NotNil(t, cmd)
}

func TestCovBoost2NavigateChildResourceTypesMonitoring(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Monitoring", Extra: "__monitoring__"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.True(t, rm.fullscreenDashboard)
	assert.NotNil(t, cmd)
}

func TestCovBoost2NavigateChildResourceTypesPortForwards(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.portForwardMgr = k8s.NewPortForwardManager()
	m.middleItems = []model.Item{{Name: "Port Forwards", Kind: "__port_forwards__"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelResources, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCovBoost2NavigateChildResourceTypesCollapsedGroup(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Workloads", Kind: "__collapsed_group__", Category: "Workloads"}}
	result, _ := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, "Workloads", rm.expandedGroup)
}

func TestCovBoost2NavigateChildResourcesNoChildren(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "ConfigMap", Resource: "configmaps", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-cm"}}
	result, cmd := m.navigateChild()
	assert.Nil(t, cmd) // ConfigMaps don't have children
	_ = result
}

func TestCovBoost2NavigateChildResourcesPod(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-pod", Namespace: "default"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelContainers, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCovBoost2NavigateChildResourcesDeployment(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", Namespaced: true, APIGroup: "apps", APIVersion: "v1"}
	m.middleItems = []model.Item{{Name: "my-deploy", Namespace: "default"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelOwned, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCovBoost2NavigateChildResourcesPodNoNamespace(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-pod"}}
	m.allNamespaces = false
	m.namespace = "default"
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelContainers, rm.nav.Level)
	assert.NotNil(t, cmd)
	assert.Equal(t, "default", rm.nav.Namespace)
}

func TestCovBoost2NavigateChildOwnedPod(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "ns1"}}
	result, cmd := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelContainers, rm.nav.Level)
	assert.NotNil(t, cmd)
}

func TestCovBoost2NavigateChildOwnedNonPod(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "rs-1", Kind: "ReplicaSet"}}
	result, cmd := m.navigateChild()
	_ = result
	_ = cmd
}

func TestCovBoost2NavigateChildContainers(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelContainers
	m.middleItems = []model.Item{{Name: "main"}}
	result, cmd := m.navigateChild()
	assert.Nil(t, cmd)
	_ = result
}

func TestCovBoost2NavigateChildNoSelection(t *testing.T) {
	m := baseModelBoost2()
	m.middleItems = nil
	result, cmd := m.navigateChild()
	assert.Nil(t, cmd)
	_ = result
}

func TestCovNavigateToOwnerUnknownKind(t *testing.T) {
	m := baseModelCov()
	m.discoveredResources = map[string][]model.ResourceTypeEntry{}
	ret, cmd := m.navigateToOwner("UnknownKind", "some-name")
	result := ret.(Model)
	assert.True(t, result.hasStatusMessage())
	assert.NotNil(t, cmd)
}

func TestCovNavigateToPortForwards(t *testing.T) {
	m := baseModelWithFakeClient()
	m.portForwardMgr = k8s.NewPortForwardManager()
	m.navigateToPortForwards()
	assert.Equal(t, model.LevelResources, m.nav.Level)
	assert.Equal(t, "__port_forwards__", m.nav.ResourceType.Kind)
}

func TestFinalNavigateChildNoSelection(t *testing.T) {
	m := baseFinalModel()
	m.middleItems = nil
	result, cmd := m.navigateChild()
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestCovNavigateParentFromResources(t *testing.T) {
	m := baseModelNav()
	m.nav.Level = model.LevelResources
	result, _ := m.navigateParent()
	rm := result.(Model)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
}

func TestCovNavigateParentFromResourceTypes(t *testing.T) {
	m := baseModelNav()
	m.nav.Level = model.LevelResourceTypes
	result, _ := m.navigateParent()
	rm := result.(Model)
	assert.Equal(t, model.LevelClusters, rm.nav.Level)
}

func TestCovNavigateParentFromClusters(t *testing.T) {
	m := baseModelNav()
	m.nav.Level = model.LevelClusters
	result, _ := m.navigateParent()
	rm := result.(Model)
	assert.Equal(t, model.LevelClusters, rm.nav.Level) // can't go back further
}

func TestCovNavigateParentFromOwned(t *testing.T) {
	m := baseModelNav()
	m.nav.Level = model.LevelOwned
	m.leftItemsHistory = [][]model.Item{{{Name: "deploy-1"}}}
	result, _ := m.navigateParent()
	rm := result.(Model)
	assert.Equal(t, model.LevelResources, rm.nav.Level)
}

func TestCovNavigateParentFromContainers(t *testing.T) {
	m := baseModelNav()
	m.nav.Level = model.LevelContainers
	m.leftItemsHistory = [][]model.Item{
		{{Name: "ns-items"}},
		{{Name: "rt-items"}},
		{{Name: "deploy-items"}},
	}
	result, _ := m.navigateParent()
	rm := result.(Model)
	assert.Less(t, int(rm.nav.Level), int(model.LevelContainers))
}

func TestCovNavigateChildFromClusters(t *testing.T) {
	m := baseModelNav()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "ctx-1", Extra: "ctx-1"}}
	result, _ := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
}

func TestCovNavigateChildEmpty(t *testing.T) {
	m := baseModelNav()
	m.nav.Level = model.LevelClusters
	m.middleItems = nil
	result, _ := m.navigateChild()
	_ = result.(Model) // no panic
}

// --- navigateChildResourceType: synthetic security sources ---

// TestNavigateChildResourceTypeSynthesizesSecurityRT verifies that clicking a
// virtual Security source Item (Kind prefixed with __security_ and Extra
// carrying the _security/v1/findings-<name> ref) produces a navigation into
// LevelResources with a fully-populated ResourceTypeEntry. Security sources
// are not present in discoveredResources, so the handler has to synthesize
// the RT from the Item's Kind/Extra fields; otherwise FindResourceTypeIn
// silently fails and the user cannot enter the Security category at all.
func TestNavigateChildResourceTypeSynthesizesSecurityRT(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	sel := model.Item{
		Name:     "Trivy (5)",
		Kind:     "__security_trivy-operator__",
		Extra:    model.SecurityVirtualAPIGroup + "/v1/findings-trivy-operator",
		Category: "Security",
		Icon:     "◈",
	}
	m.middleItems = []model.Item{sel}

	result, cmd := m.navigateChildResourceType(&sel)
	rm, ok := result.(Model)
	require.True(t, ok)
	require.NotNil(t, cmd, "security navigation must produce a load command")
	assert.Equal(t, model.LevelResources, rm.nav.Level)
	assert.Equal(t, "__security_trivy-operator__", rm.nav.ResourceType.Kind)
	assert.Equal(t, model.SecurityVirtualAPIGroup, rm.nav.ResourceType.APIGroup)
	assert.Equal(t, "findings-trivy-operator", rm.nav.ResourceType.Resource)
	assert.Equal(t, "Trivy (5)", rm.nav.ResourceType.DisplayName)
}

// TestNavigateChildResourceTypeSecurityFallsThroughWhenNoPrefix verifies that
// items whose Kind lacks the __security_ prefix still go through the normal
// FindResourceTypeIn path, so existing navigation is untouched.
func TestNavigateChildResourceTypeSecurityFallsThroughWhenNoPrefix(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	sel := model.Item{Name: "Pods", Kind: "Pod", Extra: "/v1/pods"}
	m.middleItems = []model.Item{sel}

	result, cmd := m.navigateChildResourceType(&sel)
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, "Pod", rm.nav.ResourceType.Kind)
}
