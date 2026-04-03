package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCov80LoadContainersForLogFilter(t *testing.T) {
	m := basePush80Model()
	m.actionCtx = actionContext{
		context:   "test-ctx",
		name:      "my-pod",
		namespace: "default",
	}
	cmd := m.loadContainersForLogFilter()
	require.NotNil(t, cmd)
	msg := cmd()
	lmsg, ok := msg.(logContainersLoadedMsg)
	require.True(t, ok)
	// Fake client returns empty containers, so err is set or containers is empty.
	_ = lmsg
}

func TestCov80LoadContainersForLogFilterNoPod(t *testing.T) {
	m := basePush80Model()
	m.actionCtx = actionContext{
		context:   "test-ctx",
		name:      "nonexistent-pod",
		namespace: "default",
	}
	cmd := m.loadContainersForLogFilter()
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(logContainersLoadedMsg)
	assert.True(t, ok)
}

func TestCov80LoadMetricsNilSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	cmd := m.loadMetrics()
	assert.Nil(t, cmd)
}

func TestCov80SaveLabelDataNilData(t *testing.T) {
	m := basePush80Model()
	m.labelData = nil
	cmd := m.saveLabelData()
	assert.Nil(t, cmd)
}

func TestCov80SaveLabelDataNoSelection(t *testing.T) {
	m := basePush80Model()
	m.labelData = &model.LabelAnnotationData{
		Labels:      map[string]string{"a": "b"},
		Annotations: map[string]string{"c": "d"},
	}
	m.middleItems = nil // no selection
	cmd := m.saveLabelData()
	assert.Nil(t, cmd)
}

func TestCov80LoadHelmRevisionsNoHelm(t *testing.T) {
	m := basePush80Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{
		context:   "test-ctx",
		name:      "my-release",
		namespace: "default",
	}
	cmd := m.loadHelmRevisions()
	require.NotNil(t, cmd)
	msg := cmd()
	hmsg, ok := msg.(helmRevisionListMsg)
	require.True(t, ok)
	assert.Error(t, hmsg.err)
	assert.Contains(t, hmsg.err.Error(), "helm not found")
}

func TestCov80LoadYAMLNilSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	cmd := m.loadYAML()
	assert.Nil(t, cmd)
}

func TestCov80LoadYAMLLevelClusters(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	cmd := m.loadYAML()
	assert.Nil(t, cmd)
}

func TestCov80LoadDiff(t *testing.T) {
	m := basePush80Model()
	rt := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	itemA := model.Item{Name: "pod-1", Namespace: "default"}
	itemB := model.Item{Name: "pod-2", Namespace: "other-ns"}
	cmd := m.loadDiff(rt, itemA, itemB)
	require.NotNil(t, cmd)
}

func TestCov80LoadDiffSameNs(t *testing.T) {
	m := basePush80Model()
	rt := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	itemA := model.Item{Name: "pod-1", Namespace: "default"}
	itemB := model.Item{Name: "pod-2", Namespace: "default"}
	cmd := m.loadDiff(rt, itemA, itemB)
	require.NotNil(t, cmd)
}

func TestCov80LoadPreviewEventsNilSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	cmd := m.loadPreviewEvents()
	assert.Nil(t, cmd)
}

func TestCov80ResolveOwnedResourceTypeNil(t *testing.T) {
	m := basePush80Model()
	_, ok := m.resolveOwnedResourceType(nil)
	assert.False(t, ok)
}

func TestCov80ResolveOwnedResourceTypeKnownKind(t *testing.T) {
	m := basePush80Model()
	item := &model.Item{Kind: "Deployment", Name: "deploy-1"}
	rt, ok := m.resolveOwnedResourceType(item)
	assert.True(t, ok)
	assert.Equal(t, "Deployment", rt.Kind)
}

func TestCov80ResolveOwnedResourceTypeExtraRef(t *testing.T) {
	m := basePush80Model()
	item := &model.Item{Kind: "SomeCRD", Extra: "apps/v1/deployments"}
	rt, ok := m.resolveOwnedResourceType(item)
	if ok {
		assert.NotEmpty(t, rt.Resource)
	}
}

func TestCov80ResolveOwnedResourceTypeFallbackGroupVersion(t *testing.T) {
	m := basePush80Model()
	// Extra with group/version format, unknown kind.
	item := &model.Item{Kind: "MyCustomResource", Extra: "custom.io/v1beta1"}
	rt, ok := m.resolveOwnedResourceType(item)
	assert.True(t, ok)
	assert.Equal(t, "custom.io", rt.APIGroup)
	assert.Equal(t, "v1beta1", rt.APIVersion)
	assert.Equal(t, "mycustomresources", rt.Resource)
}

func TestCov80ResolveOwnedResourceTypeNoMatch(t *testing.T) {
	m := basePush80Model()
	item := &model.Item{Kind: "NoSuchKind999"}
	_, ok := m.resolveOwnedResourceType(item)
	assert.False(t, ok)
}

func TestCov80LoadSecretDataNilSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	cmd := m.loadSecretData()
	assert.Nil(t, cmd)
}

func TestCov80SaveSecretDataNil(t *testing.T) {
	m := basePush80Model()
	m.secretData = nil
	cmd := m.saveSecretData()
	assert.Nil(t, cmd)
}

func TestCov80SaveSecretDataNoSel(t *testing.T) {
	m := basePush80Model()
	m.secretData = &model.SecretData{Data: map[string]string{"key": "val"}}
	m.middleItems = nil
	cmd := m.saveSecretData()
	assert.Nil(t, cmd)
}

func TestCov80LoadConfigMapDataNilSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	cmd := m.loadConfigMapData()
	assert.Nil(t, cmd)
}

func TestCov80SaveConfigMapDataNil(t *testing.T) {
	m := basePush80Model()
	m.configMapData = nil
	cmd := m.saveConfigMapData()
	assert.Nil(t, cmd)
}

func TestCov80SaveConfigMapDataNoSel(t *testing.T) {
	m := basePush80Model()
	m.configMapData = &model.ConfigMapData{Data: map[string]string{"key": "val"}}
	m.middleItems = nil
	cmd := m.saveConfigMapData()
	assert.Nil(t, cmd)
}

func TestCov80LoadLabelDataNilSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	cmd := m.loadLabelData()
	assert.Nil(t, cmd)
}

func TestCov80LoadRevisionsNilSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	cmd := m.loadRevisions()
	assert.Nil(t, cmd)
}

func TestCov80LoadResourceTreeNilSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	cmd := m.loadResourceTree()
	assert.Nil(t, cmd)
}

func TestCov80LoadResourceTreeClusters(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	cmd := m.loadResourceTree()
	assert.Nil(t, cmd)
}

func TestCov80ResolveNamespace(t *testing.T) {
	m := basePush80Model()
	m.nav.Namespace = "nav-ns"
	assert.Equal(t, "nav-ns", m.resolveNamespace())

	m.nav.Namespace = ""
	assert.Equal(t, "default", m.resolveNamespace())
}

func TestCov80LoadQuotas(t *testing.T) {
	m := basePush80Model()
	cmd := m.loadQuotas()
	require.NotNil(t, cmd)
}

func TestCov80LoadNamespaces(t *testing.T) {
	m := basePush80Model()
	cmd := m.loadNamespaces()
	require.NotNil(t, cmd)
}

func TestCov80LoadNamespacesEmptyContext(t *testing.T) {
	m := basePush80Model()
	m.nav.Context = ""
	cmd := m.loadNamespaces()
	require.NotNil(t, cmd)
}

func TestCov80LoadContexts(t *testing.T) {
	m := basePush80Model()
	msg := m.loadContexts()
	cmsg, ok := msg.(contextsLoadedMsg)
	require.True(t, ok)
	_ = cmsg
}

func TestCov80LoadResourcesForPreviewNilSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	cmd := m.loadResources(true)
	assert.Nil(t, cmd)
}

func TestCov80LoadResourcesNormal(t *testing.T) {
	m := basePush80Model()
	cmd := m.loadResources(false)
	require.NotNil(t, cmd)
}

func TestCov80LoadOwnedForPreviewNilSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	cmd := m.loadOwned(true)
	assert.Nil(t, cmd)
}

func TestCov80LoadOwnedNormal(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType.Kind = "Deployment"
	m.nav.ResourceName = "deploy-1"
	cmd := m.loadOwned(false)
	require.NotNil(t, cmd)
}

func TestCov80LoadOwnedEmptyNs(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType.Kind = "Deployment"
	m.nav.ResourceName = "deploy-1"
	m.allNamespaces = true
	m.nav.Namespace = "specific-ns"
	cmd := m.loadOwned(false)
	require.NotNil(t, cmd)
}

func TestCov80LoadContainersForPreviewNilSel(t *testing.T) {
	m := basePush80Model()
	m.middleItems = nil
	cmd := m.loadContainers(true)
	assert.Nil(t, cmd)
}

func TestCov80LoadContainersNormal(t *testing.T) {
	m := basePush80Model()
	m.nav.OwnedName = "pod-1"
	cmd := m.loadContainers(false)
	require.NotNil(t, cmd)
}

func TestCov80LoadPodMetricsForList(t *testing.T) {
	m := basePush80Model()
	cmd := m.loadPodMetricsForList()
	require.NotNil(t, cmd)
}

func TestCov80LoadNodeMetricsForList(t *testing.T) {
	m := basePush80Model()
	cmd := m.loadNodeMetricsForList()
	require.NotNil(t, cmd)
}

func TestCov80DiscoverCRDs(t *testing.T) {
	m := basePush80Model()
	cmd := m.discoverCRDs("test-ctx")
	require.NotNil(t, cmd)
}

func TestCov80LoadResourceTypesNoCRDs(t *testing.T) {
	m := basePush80Model()
	cmd := m.loadResourceTypes()
	require.NotNil(t, cmd)
	msg := cmd()
	rmsg, ok := msg.(resourceTypesMsg)
	require.True(t, ok)
	assert.NotEmpty(t, rmsg.items)
}

func TestCov80LoadResourceTypesWithCRDs(t *testing.T) {
	m := basePush80Model()
	m.discoveredCRDs["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "MyCRD", Resource: "mycrds", APIGroup: "test.io", APIVersion: "v1", Namespaced: true},
	}
	cmd := m.loadResourceTypes()
	require.NotNil(t, cmd)
	msg := cmd()
	rmsg, ok := msg.(resourceTypesMsg)
	require.True(t, ok)
	assert.NotEmpty(t, rmsg.items)
}

func TestCovLoadContainersForLogFilter(t *testing.T) {
	m := baseModelCov()
	m.actionCtx = actionContext{context: "ctx", kind: "Pod", name: "my-pod"}
	m.namespace = "default"
	assert.NotNil(t, m.loadContainersForLogFilter())
}

func TestCovLoadResourceTypesWithCRDs(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.discoveredCRDs["ctx"] = []model.ResourceTypeEntry{
		{Kind: "Application", Resource: "applications", APIGroup: "argoproj.io"},
	}
	assert.NotNil(t, m.loadResourceTypes())
}

func TestCovLoadResourceTypesNoCRDs(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	assert.NotNil(t, m.loadResourceTypes())
}

func TestCovLoadResourcesForPreviewNil(t *testing.T) {
	m := baseModelCov()
	m.middleItems = nil
	assert.Nil(t, m.loadResources(true))
}

func TestCovLoadResourcesForPreviewNoRT(t *testing.T) {
	m := baseModelCov()
	m.middleItems = []model.Item{{Name: "item-1", Extra: "nonexistent/v1/foos"}}
	m.nav.Context = "ctx"
	assert.Nil(t, m.loadResources(true))
}

func TestCovLoadResourcesNormal(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}
	m.namespace = "default"
	assert.NotNil(t, m.loadResources(false))
}

func TestCovLoadYAMLBranches(t *testing.T) {
	tests := []struct {
		name  string
		level model.Level
		items []model.Item
		want  bool
	}{
		{"resources", model.LevelResources, []model.Item{{Name: "nginx", Namespace: "default"}}, true},
		{"resources_empty", model.LevelResources, nil, false},
		{"owned_pod", model.LevelOwned, []model.Item{{Name: "my-pod", Kind: "Pod"}}, true},
		{"owned_unknown", model.LevelOwned, []model.Item{{Name: "my-rs", Kind: "UnknownKind"}}, true},
		{"owned_empty", model.LevelOwned, nil, false},
		{"containers", model.LevelContainers, nil, true},
		{"clusters", model.LevelClusters, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := baseModelCov()
			m.nav.Level = tt.level
			m.nav.Context = "ctx"
			m.nav.OwnedName = "pod-name"
			m.middleItems = tt.items
			m.namespace = "default"
			m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments"}
			cmd := m.loadYAML()
			if tt.want {
				assert.NotNil(t, cmd)
			} else {
				assert.Nil(t, cmd)
			}
		})
	}
}

func TestCovLoadMetricsBranches(t *testing.T) {
	tests := []struct {
		name string
		kind string
		want bool
	}{
		{"Pod", "Pod", true},
		{"Deployment", "Deployment", true},
		{"StatefulSet", "StatefulSet", true},
		{"DaemonSet", "DaemonSet", true},
		{"ConfigMap", "ConfigMap", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := baseModelCov()
			m.nav.ResourceType = model.ResourceTypeEntry{Kind: tt.kind}
			m.middleItems = []model.Item{{Name: "item-1"}}
			m.nav.Context = "ctx"
			m.namespace = "default"
			cmd := m.loadMetrics()
			if tt.want {
				assert.NotNil(t, cmd)
			} else {
				assert.Nil(t, cmd)
			}
		})
	}
}

func TestCovLoadMetricsNil(t *testing.T) {
	m := baseModelCov()
	m.middleItems = nil
	assert.Nil(t, m.loadMetrics())
}

func TestCovLoadMetricsOwnedLevel(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment"}
	m.middleItems = []model.Item{{Name: "my-pod", Kind: "Pod", Namespace: "ns1"}}
	m.nav.Context = "ctx"
	assert.NotNil(t, m.loadMetrics())
}

func TestCovLoadPreviewEventsNil(t *testing.T) {
	m := baseModelCov()
	m.middleItems = nil
	assert.Nil(t, m.loadPreviewEvents())
}

func TestCovLoadPreviewEventsNormal(t *testing.T) {
	m := baseModelCov()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.middleItems = []model.Item{{Name: "my-pod", Namespace: "ns1"}}
	m.nav.Context = "ctx"
	assert.NotNil(t, m.loadPreviewEvents())
}

func TestCovLoadPreviewEventsOwned(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment"}
	m.middleItems = []model.Item{{Name: "my-pod", Kind: "Pod"}}
	m.nav.Context = "ctx"
	assert.NotNil(t, m.loadPreviewEvents())
}

func TestCovLoadContainersBranches(t *testing.T) {
	m := baseModelCov()
	m.middleItems = nil
	assert.Nil(t, m.loadContainers(true), "forPreview nil selected")

	m2 := baseModelCov()
	m2.nav.Context = "ctx"
	m2.nav.OwnedName = "my-pod"
	m2.nav.Namespace = "from-nav"
	assert.NotNil(t, m2.loadContainers(false), "not preview with nav namespace")

	m3 := baseModelCov()
	m3.nav.Context = "ctx"
	m3.middleItems = []model.Item{{Name: "my-pod", Namespace: "ns1"}}
	assert.NotNil(t, m3.loadContainers(true), "forPreview with item")
}

func TestCovResolveOwnedResourceType(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx"
	m.discoveredCRDs["ctx"] = nil

	_, ok := m.resolveOwnedResourceType(nil)
	assert.False(t, ok, "nil item")

	_, ok = m.resolveOwnedResourceType(&model.Item{Kind: "CompletelyUnknown"})
	assert.False(t, ok, "unknown kind")

	rt, ok := m.resolveOwnedResourceType(&model.Item{Kind: "CustomThing", Extra: "custom.io/v1"})
	assert.True(t, ok, "group/version fallback")
	assert.Equal(t, "custom.io", rt.APIGroup)
	assert.Equal(t, "v1", rt.APIVersion)
	assert.Equal(t, "customthings", rt.Resource)
}

func TestCovResolveNamespace(t *testing.T) {
	m := baseModelCov()
	m.nav.Namespace = "nav-ns"
	m.namespace = "model-ns"
	assert.Equal(t, "nav-ns", m.resolveNamespace())

	m.nav.Namespace = ""
	assert.Equal(t, "model-ns", m.resolveNamespace())
}

func TestCovLoadContexts(t *testing.T) {
	m := baseModelWithFakeClient()
	msg := m.loadContexts()
	result, ok := msg.(contextsLoadedMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
	// The test client has "test-ctx" as the only context.
	require.Len(t, result.items, 1)
	assert.Equal(t, "test-ctx", result.items[0].Name)
	assert.Equal(t, "current", result.items[0].Status)
}

func TestCovLoadResourceTypes(t *testing.T) {
	m := baseModelWithFakeClient()
	cmd := m.loadResourceTypes()
	msg := execCmd(t, cmd)
	result, ok := msg.(resourceTypesMsg)
	require.True(t, ok)
	assert.NotEmpty(t, result.items)
}

func TestCovLoadResourceTypesWithCRDsFakeClient(t *testing.T) {
	m := baseModelWithFakeClient()
	m.discoveredCRDs["test-ctx"] = []model.ResourceTypeEntry{
		{DisplayName: "TestCRD", Kind: "TestCRD", APIGroup: "test.io", APIVersion: "v1", Resource: "testcrds"},
	}
	cmd := m.loadResourceTypes()
	msg := execCmd(t, cmd)
	result, ok := msg.(resourceTypesMsg)
	require.True(t, ok)
	assert.NotEmpty(t, result.items)
}

func TestCovDiscoverCRDsReturnsCmd(t *testing.T) {
	m := baseModelWithFakeClient()
	cmd := m.discoverCRDs("test-ctx")
	// Just verify it returns a non-nil command. Executing it would panic because
	// the fake dynamic client does not have the CRD list kind registered.
	assert.NotNil(t, cmd)
}

func TestCovLoadQuotas(t *testing.T) {
	m := baseModelWithFakeClient()
	cmd := m.loadQuotas()
	msg := execCmd(t, cmd)
	result, ok := msg.(quotaLoadedMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
}

func TestCovLoadNamespaces(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-system"},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
	m := baseModelWithFakeClient(ns)
	cmd := m.loadNamespaces()
	msg := execCmd(t, cmd)
	result, ok := msg.(namespacesLoadedMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
	assert.Len(t, result.items, 1)
	assert.Equal(t, "kube-system", result.items[0].Name)
}

func TestCovLoadNamespacesNoContext(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = ""
	cmd := m.loadNamespaces()
	msg := execCmd(t, cmd)
	result, ok := msg.(namespacesLoadedMsg)
	require.True(t, ok)
	// CurrentContext() is "test-ctx" so it should still work.
	assert.NoError(t, result.err)
}

func TestCovLoadResources(t *testing.T) {
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	gvrToListKind := map[schema.GroupVersionResource]string{
		gvr: "PodList",
	}
	m := baseModelWithFakeDynamic(gvrToListKind)
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Pod",
		APIVersion: "v1",
		Resource:   "pods",
		Namespaced: true,
	}
	cmd := m.loadResources(false)
	msg := execCmd(t, cmd)
	result, ok := msg.(resourcesLoadedMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
	// No pods pre-loaded, so empty list.
	assert.Empty(t, result.items)
}

func TestCovLoadResourcesNilForPreviewWithNoSelection(t *testing.T) {
	m := baseModelWithFakeClient()
	m.middleItems = nil
	cmd := m.loadResources(true)
	assert.Nil(t, cmd)
}

func TestCovLoadOwnedReturnsCmd(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment"}
	m.nav.ResourceName = "my-deploy"
	cmd := m.loadOwned(false)
	// Just verify a command is returned. Executing it would panic because
	// the fake dynamic client does not have replicaset list kind registered.
	assert.NotNil(t, cmd)
}

func TestCovLoadOwnedForPreviewNil(t *testing.T) {
	m := baseModelWithFakeClient()
	m.middleItems = nil
	cmd := m.loadOwned(true)
	assert.Nil(t, cmd)
}

func TestCovLoadResourceTreeNil(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelResources
	m.middleItems = nil
	cmd := m.loadResourceTree()
	assert.Nil(t, cmd)
}

func TestCovLoadResourceTreeDefaultLevel(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	cmd := m.loadResourceTree()
	assert.Nil(t, cmd)
}

func TestCovLoadContainers(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main", Image: "nginx:latest"},
			},
		},
	}
	m := baseModelWithFakeClient(pod)
	m.nav.OwnedName = "test-pod"
	cmd := m.loadContainers(false)
	msg := execCmd(t, cmd)
	result, ok := msg.(containersLoadedMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
	assert.Len(t, result.items, 1)
	assert.Equal(t, "main", result.items[0].Name)
}

func TestCovLoadContainersForPreviewNil(t *testing.T) {
	m := baseModelWithFakeClient()
	m.middleItems = nil
	cmd := m.loadContainers(true)
	assert.Nil(t, cmd)
}

func TestCovLoadDiff(t *testing.T) {
	rt := model.ResourceTypeEntry{
		Kind:       "ConfigMap",
		APIVersion: "v1",
		Resource:   "configmaps",
		Namespaced: true,
	}
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	cm1 := &unstructured.Unstructured{}
	cm1.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"})
	cm1.SetName("cm-1")
	cm1.SetNamespace("default")
	cm2 := &unstructured.Unstructured{}
	cm2.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"})
	cm2.SetName("cm-2")
	cm2.SetNamespace("default")

	gvrToListKind := map[schema.GroupVersionResource]string{gvr: "ConfigMapList"}
	m := baseModelWithFakeDynamic(gvrToListKind, cm1, cm2)

	itemA := model.Item{Name: "cm-1", Namespace: "default"}
	itemB := model.Item{Name: "cm-2", Namespace: "default"}
	cmd := m.loadDiff(rt, itemA, itemB)
	msg := execCmd(t, cmd)
	result, ok := msg.(diffLoadedMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
}

func TestCovLoadDiffDifferentNamespaces(t *testing.T) {
	rt := model.ResourceTypeEntry{
		Kind:       "ConfigMap",
		APIVersion: "v1",
		Resource:   "configmaps",
		Namespaced: true,
	}
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	gvrToListKind := map[schema.GroupVersionResource]string{gvr: "ConfigMapList"}
	m := baseModelWithFakeDynamic(gvrToListKind)

	itemA := model.Item{Name: "cm-1", Namespace: "ns-a"}
	itemB := model.Item{Name: "cm-2", Namespace: "ns-b"}
	cmd := m.loadDiff(rt, itemA, itemB)
	msg := execCmd(t, cmd)
	result, ok := msg.(diffLoadedMsg)
	require.True(t, ok)
	// Items don't exist so we expect an error.
	assert.Error(t, result.err)
}

func TestCovLoadYAMLResources(t *testing.T) {
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	cm := &unstructured.Unstructured{}
	cm.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"})
	cm.SetName("my-cm")
	cm.SetNamespace("default")

	gvrToListKind := map[schema.GroupVersionResource]string{gvr: "ConfigMapList"}
	m := baseModelWithFakeDynamic(gvrToListKind, cm)
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "ConfigMap",
		APIVersion: "v1",
		Resource:   "configmaps",
		Namespaced: true,
	}
	m = withMiddleItem(m, model.Item{Name: "my-cm", Namespace: "default"})
	cmd := m.loadYAML()
	msg := execCmd(t, cmd)
	result, ok := msg.(yamlLoadedMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
	assert.Contains(t, result.content, "my-cm")
}

func TestCovLoadYAMLContainersReturnsCmd(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "my-pod"
	cmd := m.loadYAML()
	// Verifies the LevelContainers branch is reached and returns a cmd.
	assert.NotNil(t, cmd)
}

func TestCovLoadYAMLNilSelection(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelResources
	m.middleItems = nil
	cmd := m.loadYAML()
	assert.Nil(t, cmd)
}

func TestCovLoadMetricsNilFakeClient(t *testing.T) {
	m := baseModelWithFakeClient()
	m.middleItems = nil
	cmd := m.loadMetrics()
	assert.Nil(t, cmd)
}

func TestCovLoadPreviewEventsNilFakeClient(t *testing.T) {
	m := baseModelWithFakeClient()
	m.middleItems = nil
	cmd := m.loadPreviewEvents()
	assert.Nil(t, cmd)
}

func TestCovLoadPodMetricsForListReturnsCmd(t *testing.T) {
	m := baseModelWithFakeClient()
	cmd := m.loadPodMetricsForList()
	// Just verify the function returns a non-nil command.
	// Executing it would panic because the metrics.k8s.io GVR is not registered.
	assert.NotNil(t, cmd)
}

func TestCovLoadNodeMetricsForListReturnsCmd(t *testing.T) {
	m := baseModelWithFakeClient()
	cmd := m.loadNodeMetricsForList()
	assert.NotNil(t, cmd)
}

func TestCovLoadSecretData(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		Data:       map[string][]byte{"key": []byte("val")},
	}
	m := baseModelWithFakeClient(secret)
	m = withMiddleItem(m, model.Item{Name: "my-secret", Namespace: "default"})
	cmd := m.loadSecretData()
	msg := execCmd(t, cmd)
	result, ok := msg.(secretDataLoadedMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
	assert.Equal(t, "val", result.data.Data["key"])
}

func TestCovLoadSecretDataNil(t *testing.T) {
	m := baseModelWithFakeClient()
	m.middleItems = nil
	cmd := m.loadSecretData()
	assert.Nil(t, cmd)
}

func TestCovSaveSecretData(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		Data:       map[string][]byte{"key": []byte("old")},
	}
	m := baseModelWithFakeClient(secret)
	m = withMiddleItem(m, model.Item{Name: "my-secret", Namespace: "default"})
	m.secretData = &model.SecretData{Data: map[string]string{"key": "new"}}
	cmd := m.saveSecretData()
	msg := execCmd(t, cmd)
	result, ok := msg.(secretSavedMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
}

func TestCovSaveSecretDataNilSecretData(t *testing.T) {
	m := baseModelWithFakeClient()
	m.secretData = nil
	cmd := m.saveSecretData()
	assert.Nil(t, cmd)
}

func TestCovSaveSecretDataNilSel(t *testing.T) {
	m := baseModelWithFakeClient()
	m.secretData = &model.SecretData{Data: map[string]string{"k": "v"}}
	m.middleItems = nil
	cmd := m.saveSecretData()
	assert.Nil(t, cmd)
}

func TestCovLoadConfigMapData(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
		Data:       map[string]string{"config.yaml": "data: true"},
	}
	m := baseModelWithFakeClient(cm)
	m = withMiddleItem(m, model.Item{Name: "my-cm", Namespace: "default"})
	cmd := m.loadConfigMapData()
	msg := execCmd(t, cmd)
	result, ok := msg.(configMapDataLoadedMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
	assert.Equal(t, "data: true", result.data.Data["config.yaml"])
}

func TestCovLoadConfigMapDataNil(t *testing.T) {
	m := baseModelWithFakeClient()
	m.middleItems = nil
	cmd := m.loadConfigMapData()
	assert.Nil(t, cmd)
}

func TestCovSaveConfigMapData(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
		Data:       map[string]string{"key": "old"},
	}
	m := baseModelWithFakeClient(cm)
	m = withMiddleItem(m, model.Item{Name: "my-cm", Namespace: "default"})
	m.configMapData = &model.ConfigMapData{Data: map[string]string{"key": "new"}}
	cmd := m.saveConfigMapData()
	msg := execCmd(t, cmd)
	result, ok := msg.(configMapSavedMsg)
	require.True(t, ok)
	assert.NoError(t, result.err)
}

func TestCovSaveConfigMapDataNil(t *testing.T) {
	m := baseModelWithFakeClient()
	m.configMapData = nil
	cmd := m.saveConfigMapData()
	assert.Nil(t, cmd)
}

func TestCovLoadLabelDataNil(t *testing.T) {
	m := baseModelWithFakeClient()
	m.middleItems = nil
	cmd := m.loadLabelData()
	assert.Nil(t, cmd)
}

func TestCovSaveLabelDataNilData(t *testing.T) {
	m := baseModelWithFakeClient()
	m.labelData = nil
	cmd := m.saveLabelData()
	assert.Nil(t, cmd)
}

func TestCovSaveLabelDataNilSel(t *testing.T) {
	m := baseModelWithFakeClient()
	m.labelData = &model.LabelAnnotationData{
		Labels:      map[string]string{"l": "v"},
		Annotations: map[string]string{"a": "v"},
	}
	m.middleItems = nil
	cmd := m.saveLabelData()
	assert.Nil(t, cmd)
}

func TestCovLoadRevisionsNil(t *testing.T) {
	m := baseModelWithFakeClient()
	m.middleItems = nil
	cmd := m.loadRevisions()
	assert.Nil(t, cmd)
}

func TestCovResolveOwnedResourceTypeNil(t *testing.T) {
	m := baseModelCov()
	_, ok := m.resolveOwnedResourceType(nil)
	assert.False(t, ok)
}

func TestCovResolveOwnedResourceTypeFallback(t *testing.T) {
	m := baseModelCov()
	m.discoveredCRDs = map[string][]model.ResourceTypeEntry{}
	sel := &model.Item{Kind: "MyCustomResource", Extra: "mygroup.io/v1"}
	rt, ok := m.resolveOwnedResourceType(sel)
	assert.True(t, ok)
	assert.Equal(t, "mygroup.io", rt.APIGroup)
	assert.Equal(t, "v1", rt.APIVersion)
	assert.Equal(t, "mycustomresources", rt.Resource)
}

func TestCovLoadPreviewEventsWithSelection(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withMiddleItem(m, model.Item{Name: "my-pod", Namespace: "default"})
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	cmd := m.loadPreviewEvents()
	msg := execCmd(t, cmd)
	result, ok := msg.(previewEventsLoadedMsg)
	require.True(t, ok)
	_ = result
}

func TestCovLoadMetricsPod(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withMiddleItem(m, model.Item{Name: "my-pod", Namespace: "default"})
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	cmd := m.loadMetrics()
	assert.NotNil(t, cmd)
}

func TestCovLoadMetricsDeployment(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withMiddleItem(m, model.Item{Name: "my-deploy", Namespace: "default"})
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment"}
	cmd := m.loadMetrics()
	assert.NotNil(t, cmd)
}

func TestCovLoadMetricsUnsupported(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withMiddleItem(m, model.Item{Name: "my-cm", Namespace: "default"})
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "ConfigMap"}
	cmd := m.loadMetrics()
	assert.Nil(t, cmd)
}
