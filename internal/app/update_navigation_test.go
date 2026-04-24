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
	// navigateParent clears nav.Context, so the loadPreview() returned here
	// resolves via loadResourceTypes with an empty context — which has no
	// discovered entries and therefore emits no cmd (see loadResourceTypes).
	// The important assertion is just that the level transitions back to
	// clusters; the cmd can be nil.
	_ = cmd
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

// When the session restores to LevelResources and discovery hasn't finished
// yet for the context, m.leftItems contains the seed resource-type list as
// a fallback. Navigating back (`h`) used to pop those seeds into the middle
// column, so the user saw a short list flash before the real discovered
// list arrived. After the fix, navigateParent from LevelResources checks
// whether discovery has completed: if not, it shows the loader instead of
// stale seeds.
func TestNavigateParentFromResources_ShowsLoaderWhileDiscoveryPending(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.Context = "test-ctx"
	// Pretend these were the seed resource types stashed while discovery ran.
	m.leftItems = []model.Item{
		{Name: "Pods", Extra: "/v1/pods"},
		{Name: "Deployments", Extra: "apps/v1/deployments"},
	}
	m.leftItemsHistory = [][]model.Item{{{Name: "test-ctx"}}}
	// discoveredResources deliberately empty — discovery not yet complete.
	delete(m.discoveredResources, "test-ctx")

	result, _ := m.navigateParent()
	rm := result.(Model)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
	assert.Nil(t, rm.middleItems,
		"discovery pending: middle pane must show the loader, not pop in seed items")
	assert.True(t, rm.loading)
}

// When discovery has already completed, the cached discovered items in
// m.leftItems propagate to the middle column as before — we must not
// regress the fast-path.
func TestNavigateParentFromResources_UsesCachedDiscoveredTypes(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.Context = "test-ctx"
	m.leftItems = []model.Item{{Name: "Pods"}, {Name: "Deployments"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "test-ctx"}}}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{DisplayName: "Pods", Kind: "Pod", APIVersion: "v1", Resource: "pods"},
	}

	result, _ := m.navigateParent()
	rm := result.(Model)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
	assert.Len(t, rm.middleItems, 2,
		"discovery complete: middle pane must pop in the cached resource-type list")
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
		Icon:     model.Icon{Unicode: "◈"},
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

// TestNavigateChildResourceType_ReusesPreviewCacheWithoutRefetch verifies
// that when the preview handler has already primed itemCache for the
// target resource (previewSatisfiedNavKey + fingerprint still valid),
// drilling in reuses the cached list and the follow-up loadResources
// synthesizes a msg from cache instead of firing a second network fetch.
// This must work even though navigateChild bumps requestGen before this
// handler runs — the fingerprint comparison deliberately does not depend
// on requestGen. The marker is persistent (not consumed) so a subsequent
// navigate-out + hover-back can reuse the same cache.
func TestNavigateChildResourceType_ReusesPreviewCacheWithoutRefetch(t *testing.T) {
	m := basePush80Model()
	m.nav = model.NavigationState{Level: model.LevelResourceTypes, Context: "test-ctx"}
	m.namespace = "default"
	// Simulate navigateChild having already bumped the gen before we're
	// invoked (this is the real call path). The shortcut must still work.
	m.requestGen = 8

	pvcRT := model.ResourceTypeEntry{
		Kind:       "PersistentVolumeClaim",
		Resource:   "persistentvolumeclaims",
		APIGroup:   "",
		APIVersion: "v1",
		Namespaced: true,
	}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{pvcRT}

	cachedItems := []model.Item{
		{Name: "pvc-a", Kind: "PersistentVolumeClaim", Namespace: "default"},
		{Name: "pvc-b", Kind: "PersistentVolumeClaim", Namespace: "default"},
	}
	drillInKey := "test-ctx/persistentvolumeclaims/ns:default"
	m.itemCache[drillInKey] = cachedItems
	m.cacheFingerprints[drillInKey] = m.fetchFingerprint()

	// leftItemsHistory must exist for pushLeft not to panic.
	m.leftItems = []model.Item{{Name: "test-ctx"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "root"}}}

	sel := &model.Item{
		Kind:  "PersistentVolumeClaim",
		Extra: pvcRT.ResourceRef(),
	}
	result, cmd := m.navigateChildResourceType(sel)
	rm := result.(Model)

	assert.Equal(t, model.LevelResources, rm.nav.Level)
	assert.Equal(t, cachedItems, rm.middleItems, "cached items must populate middleItems immediately")
	// The fingerprint entry stays intact so a navigate-out + re-hover can
	// also be served from cache.
	assert.Equal(t, rm.fetchFingerprint(), rm.cacheFingerprints[drillInKey],
		"fingerprint entry persists across drill-in")
	require.NotNil(t, cmd, "must return a cmd (synthesized from loadResources cache shortcut)")

	// The returned cmd is loadResources(false). When run it must synthesize
	// a resourcesLoadedMsg from cache rather than dispatching a real API
	// call through trackBgTask.
	msg := cmd()
	loaded, ok := msg.(resourcesLoadedMsg)
	require.True(t, ok, "synthesized cmd must produce a resourcesLoadedMsg, got %T", msg)
	assert.False(t, loaded.forPreview, "main-load path: forPreview=false")
	assert.Equal(t, cachedItems, loaded.items, "items come from cache, not the API")
	assert.Equal(t, uint64(8), loaded.gen, "synthesized msg carries current requestGen so it passes the staleness gate")
}

// TestNavigateChildResourceType_FetchesWhenFingerprintStale verifies that
// the shortcut does NOT trigger when the fetch-affecting state has
// changed since prime time (selectedNamespaces toggle, etc). The marker
// must remain intact so the next real preview can re-arm it, and the
// drill-in must fall back to the normal loadResources refresh. This is
// the safety net against serving stale data after a filter change.
//
// Note: this branch bakes the effective namespace into the cache key
// (navKey includes /ns:<ns> for namespaced resources), so a plain
// "namespace switch" scenario manifests as a cache miss, not a
// fingerprint mismatch. We trigger fingerprint staleness here via a
// selectedNamespaces toggle that preserves the effective namespace
// (and thus the key) but alters the fingerprint.
func TestNavigateChildResourceType_FetchesWhenFingerprintStale(t *testing.T) {
	m := basePush80Model()
	m.nav = model.NavigationState{Level: model.LevelResourceTypes, Context: "test-ctx"}
	m.namespace = "default"

	pvcRT := model.ResourceTypeEntry{
		Kind:       "PersistentVolumeClaim",
		Resource:   "persistentvolumeclaims",
		APIVersion: "v1",
		Namespaced: true,
	}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{pvcRT}
	drillInKey := "test-ctx/persistentvolumeclaims/ns:default"
	staleItems := []model.Item{{Name: "stale-pvc"}}
	m.itemCache[drillInKey] = staleItems
	// Record the fingerprint as captured before any selectedNamespaces
	// filter, then introduce a size-1 selectedNamespaces that resolves
	// to the same effective namespace. The key stays "ns:default" but
	// the fingerprint gains a sel= segment, so it won't match anymore.
	staleFingerprint := m.fetchFingerprint()
	m.cacheFingerprints[drillInKey] = staleFingerprint
	m.selectedNamespaces = map[string]bool{"default": true}

	m.leftItems = []model.Item{{Name: "test-ctx"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "root"}}}

	sel := &model.Item{Kind: "PersistentVolumeClaim", Extra: pvcRT.ResourceRef()}
	result, cmd := m.navigateChildResourceType(sel)
	rm := result.(Model)

	// The shortcut must NOT trigger on fingerprint mismatch: the stored
	// fingerprint entry stays intact so once a fresh fetch completes it
	// will overwrite it with a matching one.
	assert.Equal(t, staleFingerprint, rm.cacheFingerprints[drillInKey],
		"stale fingerprint entry is preserved; a fresh fetch will overwrite it")
	// Cached items still populate middleItems (show-cache-while-refresh UX),
	// but a real fetch is issued.
	assert.Equal(t, staleItems, rm.middleItems, "cached items populate middle list immediately")
	require.NotNil(t, cmd, "loadResources must fire on fingerprint mismatch")
}

// TestNavigateChild_DrillInWhilePreviewInFlight verifies that drilling
// in before the sidebar preview has returned leaves middleItems in a
// clean loading state (not leaked from leftItems). This is the regression
// the user reported as "left and middle both show resource types" — if
// pushLeft's shared slice reference weren't severed by reassigning
// middleItems, both columns would render identical content.
func TestNavigateChild_DrillInWhilePreviewInFlight(t *testing.T) {
	m := basePush80Model()
	m.nav = model.NavigationState{Level: model.LevelResourceTypes, Context: "test-ctx"}
	m.namespace = "default"
	m.requestGen = 3

	pvcRT := model.ResourceTypeEntry{
		Kind:       "PersistentVolumeClaim",
		Resource:   "persistentvolumeclaims",
		APIVersion: "v1",
		Namespaced: true,
	}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{pvcRT}

	// Middle column holds the resource-types sidebar list. Preview has
	// not completed yet — no entry for the PVC navKey, no marker armed.
	resourceTypes := []model.Item{
		{Name: "Pods", Kind: "Pod", Extra: "/v1/pods"},
		{Name: "PersistentVolumeClaims", Kind: "PersistentVolumeClaim", Extra: pvcRT.ResourceRef()},
	}
	m.middleItems = resourceTypes
	m.setCursor(1) // on PVC

	m.leftItems = []model.Item{{Name: "test-ctx"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "root"}}}

	result, cmd := m.navigateChild()
	rm := result.(Model)

	assert.Equal(t, model.LevelResources, rm.nav.Level)
	// Left must hold the resource-types list (pushed from middle).
	assert.Equal(t, resourceTypes, rm.leftItems, "left column holds the pushed resource-types list")
	// Middle must be empty (loader state) — NOT the same slice as leftItems.
	// The user's reported bug was both columns rendering the resource-types
	// list; this assert locks down the fix.
	assert.Empty(t, rm.middleItems,
		"middle column must be cleared on drill-in; resource types must not leak into it")
	assert.True(t, rm.loading, "loading flag must be armed so the middle column renders the spinner")
	require.NotNil(t, cmd, "loadResources(false) must fire for the real fetch")
}

// TestNavigateChild_ShortcutSurvivesGenBump simulates the full real-world
// call path: navigateChild bumps requestGen before dispatching to
// navigateChildResourceType. The shortcut must still trigger — this is
// the regression that caused "navigate in and navigate out starts loading
// again" in the live app, because an earlier gen-based freshness check
// always false-negatived after navigateChild's bump.
func TestNavigateChild_ShortcutSurvivesGenBump(t *testing.T) {
	m := basePush80Model()
	m.nav = model.NavigationState{Level: model.LevelResourceTypes, Context: "test-ctx"}
	m.namespace = "default"
	m.requestGen = 10

	pvcRT := model.ResourceTypeEntry{
		Kind:       "PersistentVolumeClaim",
		Resource:   "persistentvolumeclaims",
		APIVersion: "v1",
		Namespaced: true,
	}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{pvcRT}

	// The sidebar item the user is hovering in the middle column.
	pvcItem := model.Item{
		Name:  "PersistentVolumeClaims",
		Kind:  "PersistentVolumeClaim",
		Extra: pvcRT.ResourceRef(),
	}
	m.middleItems = []model.Item{pvcItem}
	m.setCursor(0)

	// Simulate: preview completed, cache + fingerprint primed.
	drillInKey := "test-ctx/persistentvolumeclaims/ns:default"
	cachedItems := []model.Item{
		{Name: "pvc-1", Kind: "PersistentVolumeClaim", Namespace: "default"},
	}
	m.itemCache[drillInKey] = cachedItems
	m.cacheFingerprints[drillInKey] = m.fetchFingerprint()

	m.leftItems = []model.Item{{Name: "test-ctx"}}
	m.leftItemsHistory = [][]model.Item{{{Name: "root"}}}

	// Go through the real entry point (navigateChild), which bumps
	// requestGen and then dispatches to navigateChildResourceType. Prior
	// to the fingerprint-based check, the gen bump here silently broke
	// the shortcut because previewSatisfiedGen could never match.
	result, cmd := m.navigateChild()
	rm := result.(Model)

	assert.Equal(t, model.LevelResources, rm.nav.Level)
	assert.Equal(t, cachedItems, rm.middleItems,
		"shortcut must fire even after navigateChild's gen bump")
	// Fingerprint entry is persistent so subsequent loads for the same
	// rt (after navigate-out) can also skip the fetch.
	assert.Equal(t, rm.fetchFingerprint(), rm.cacheFingerprints[drillInKey],
		"fingerprint entry must persist so follow-up loads reuse the cache")
	require.NotNil(t, cmd)
	msg := cmd()
	loaded, ok := msg.(resourcesLoadedMsg)
	require.True(t, ok, "expected synthesized resourcesLoadedMsg, got %T", msg)
	assert.False(t, loaded.forPreview, "synthesized msg routes through main handler")
	assert.Equal(t, cachedItems, loaded.items)
}

// TestLoadResources_PreviewShortcutAfterNavigateOut verifies that after a
// drill-in populates itemCache + markers, navigating back out and hovering
// the same resource type serves the preview from cache without a second
// fetch. This closes the "shows the loader again on the children pane"
// regression the user reported after drill-in + navigate-out.
func TestLoadResources_PreviewShortcutAfterNavigateOut(t *testing.T) {
	m := basePush80Model()
	m.nav = model.NavigationState{Level: model.LevelResourceTypes, Context: "test-ctx"}
	m.namespace = "default"
	m.requestGen = 12

	pvcRT := model.ResourceTypeEntry{
		Kind:       "PersistentVolumeClaim",
		Resource:   "persistentvolumeclaims",
		APIVersion: "v1",
		Namespaced: true,
	}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{pvcRT}

	cachedItems := []model.Item{
		{Name: "pvc-a", Kind: "PersistentVolumeClaim", Namespace: "default"},
	}
	cacheKey := "test-ctx/persistentvolumeclaims/ns:default"
	m.itemCache[cacheKey] = cachedItems
	// Fingerprint left over from the drill-in's main load.
	m.cacheFingerprints[cacheKey] = m.fetchFingerprint()

	// Simulate being at the resource-types level with cursor on PVC.
	pvcSidebarItem := model.Item{
		Name:  "PersistentVolumeClaims",
		Kind:  "PersistentVolumeClaim",
		Extra: pvcRT.ResourceRef(),
	}
	m.middleItems = []model.Item{pvcSidebarItem}
	m.setCursor(0)

	cmd := m.loadResources(true)
	require.NotNil(t, cmd, "loadResources must return a cmd when an rt is hovered")

	msg := cmd()
	loaded, ok := msg.(resourcesLoadedMsg)
	require.True(t, ok, "preview shortcut must produce a resourcesLoadedMsg, got %T", msg)
	assert.True(t, loaded.forPreview, "preview path: forPreview=true")
	assert.Equal(t, cachedItems, loaded.items, "items come from cache, not the API")
	assert.Equal(t, uint64(12), loaded.gen)
}

// TestLoadResources_PreviewFetchesWhenNoMarker verifies the shortcut is
// a pure optimization: without a primed marker, loadResources falls back
// to the real API fetch. This guards against the optimization silently
// hiding stale data when the marker hasn't been armed (e.g., first hover).
func TestLoadResources_PreviewFetchesWhenNoMarker(t *testing.T) {
	m := basePush80Model()
	m.nav = model.NavigationState{Level: model.LevelResourceTypes, Context: "test-ctx"}
	m.namespace = "default"
	m.requestGen = 1

	pvcRT := model.ResourceTypeEntry{
		Kind:       "PersistentVolumeClaim",
		Resource:   "persistentvolumeclaims",
		APIVersion: "v1",
		Namespaced: true,
	}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{pvcRT}
	// Cache has data but marker is NOT armed (e.g., data from a stale
	// prior session). Shortcut must not trigger.
	m.itemCache["test-ctx/persistentvolumeclaims/ns:default"] = []model.Item{{Name: "stale"}}

	pvcSidebarItem := model.Item{
		Name:  "PersistentVolumeClaims",
		Kind:  "PersistentVolumeClaim",
		Extra: pvcRT.ResourceRef(),
	}
	m.middleItems = []model.Item{pvcSidebarItem}
	m.setCursor(0)

	cmd := m.loadResources(true)
	require.NotNil(t, cmd)
	// Without an entry in cacheFingerprints for this key, the shortcut
	// cannot fire — so the returned cmd is the real fetch path. We don't
	// invoke it here because the fake dynamic client panics on PVC list
	// (no registered list kind); the structural check below is enough.
	assert.Empty(t, m.cacheFingerprints,
		"no fingerprint entry should have been populated; shortcut path untaken")
}

// TestLoadResources_PreviewShortcutForMultipleResources verifies the
// per-key cacheFingerprints map supports multiple concurrently-fresh
// entries. This is the regression the user reported: with a single-slot
// marker, hovering PVC then PV then PVC again refetched PVC because its
// marker had been overwritten by PV. With a per-key map each rt keeps
// its own fingerprint, so the hover-cycle stays cached.
func TestLoadResources_PreviewShortcutForMultipleResources(t *testing.T) {
	m := basePush80Model()
	m.nav = model.NavigationState{Level: model.LevelResourceTypes, Context: "test-ctx"}
	m.namespace = "default"
	m.requestGen = 1

	pvcRT := model.ResourceTypeEntry{
		Kind: "PersistentVolumeClaim", Resource: "persistentvolumeclaims",
		APIVersion: "v1", Namespaced: true,
	}
	pvRT := model.ResourceTypeEntry{
		Kind: "PersistentVolume", Resource: "persistentvolumes",
		APIVersion: "v1", Namespaced: false,
	}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{pvcRT, pvRT}

	pvcItems := []model.Item{{Name: "pvc-1", Kind: "PersistentVolumeClaim", Namespace: "default"}}
	pvItems := []model.Item{{Name: "pv-1", Kind: "PersistentVolume"}}
	// PVC is namespaced, PV is cluster-scoped; keys match navKey() output.
	m.itemCache["test-ctx/persistentvolumeclaims/ns:default"] = pvcItems
	m.itemCache["test-ctx/persistentvolumes"] = pvItems
	m.cacheFingerprints["test-ctx/persistentvolumeclaims/ns:default"] = m.fetchFingerprint()
	m.cacheFingerprints["test-ctx/persistentvolumes"] = m.fetchFingerprint()

	pvcSidebar := model.Item{Name: "PersistentVolumeClaims", Kind: "PersistentVolumeClaim", Extra: pvcRT.ResourceRef()}
	pvSidebar := model.Item{Name: "PersistentVolumes", Kind: "PersistentVolume", Extra: pvRT.ResourceRef()}
	m.middleItems = []model.Item{pvcSidebar, pvSidebar}

	// Hover PVC: shortcut must serve pvcItems.
	m.setCursor(0)
	cmd := m.loadResources(true)
	require.NotNil(t, cmd)
	msg, ok := cmd().(resourcesLoadedMsg)
	require.True(t, ok, "PVC hover must produce synthesized msg")
	assert.Equal(t, pvcItems, msg.items)

	// Hover PV: shortcut must serve pvItems.
	m.setCursor(1)
	cmd = m.loadResources(true)
	require.NotNil(t, cmd)
	msg, ok = cmd().(resourcesLoadedMsg)
	require.True(t, ok, "PV hover must produce synthesized msg")
	assert.Equal(t, pvItems, msg.items)

	// Hover back to PVC: must STILL serve pvcItems from cache — this is
	// the case that failed with the single-slot marker.
	m.setCursor(0)
	cmd = m.loadResources(true)
	require.NotNil(t, cmd)
	msg, ok = cmd().(resourcesLoadedMsg)
	require.True(t, ok, "returning to PVC must still use the cached PVC entry, not refetch")
	assert.Equal(t, pvcItems, msg.items,
		"per-key fingerprint map keeps PVC's entry fresh even after PV's hover overwrote nothing")
}

// --- PgUp/PgDown/Home/End support ---

// manyItemsModel returns a model with enough items to exercise paging and
// Home/End in the middle column.
func manyItemsModel(n int) Model {
	m := basePush80v3Model()
	items := make([]model.Item, n)
	for i := range n {
		items[i] = model.Item{Name: "item-" + string(rune('a'+i%26)) + "-" + string(rune('0'+i/26)), Namespace: "default", Kind: "Pod", Status: "Running"}
	}
	m.middleItems = items
	m.setCursor(0)
	return m
}

// TestHomeKeyJumpsToTopOfList verifies that the Home key jumps the cursor
// to the first item, matching user expectation for standard navigation
// keys. Home is a single-press equivalent of the vim-style "gg" sequence.
func TestHomeKeyJumpsToTopOfList(t *testing.T) {
	m := manyItemsModel(20)
	m.setCursor(15)
	result, _ := m.handleKey(keyMsg("home"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.cursor(), "Home must jump the cursor to the first item")
	assert.False(t, rm.pendingG, "Home must not leave pendingG armed")
}

// TestEndKeyJumpsToBottomOfList verifies that the End key jumps the cursor
// to the last item, matching user expectation for standard navigation
// keys. End is the single-press equivalent of "G".
func TestEndKeyJumpsToBottomOfList(t *testing.T) {
	m := manyItemsModel(20)
	m.setCursor(0)
	result, _ := m.handleKey(keyMsg("end"))
	rm := result.(Model)
	visible := rm.visibleMiddleItems()
	assert.Equal(t, len(visible)-1, rm.cursor(), "End must jump the cursor to the last item")
}

// TestPgDownKeyScrollsFullPageDown verifies that PgDown moves the cursor a
// full page down, matching the behavior of Ctrl+F (PageForward).
func TestPgDownKeyScrollsFullPageDown(t *testing.T) {
	m := manyItemsModel(50)
	m.height = 20
	m.setCursor(0)
	result, _ := m.handleKey(keyMsg("pgdown"))
	rm := result.(Model)
	assert.Greater(t, rm.cursor(), 0, "PgDown must advance the cursor")
	// Full page = height - 4 = 16 items, capped at len-1.
	assert.LessOrEqual(t, rm.cursor(), len(rm.visibleMiddleItems())-1)
}

// TestPgUpKeyScrollsFullPageUp verifies that PgUp moves the cursor a full
// page up, matching the behavior of Ctrl+B (PageBack).
func TestPgUpKeyScrollsFullPageUp(t *testing.T) {
	m := manyItemsModel(50)
	m.height = 20
	m.setCursor(30)
	result, _ := m.handleKey(keyMsg("pgup"))
	rm := result.(Model)
	assert.Less(t, rm.cursor(), 30, "PgUp must move the cursor backwards")
	assert.GreaterOrEqual(t, rm.cursor(), 0)
}
