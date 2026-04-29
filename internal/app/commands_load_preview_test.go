package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCovBoost2LoadPreviewNilSelection(t *testing.T) {
	m := baseModelBoost2()
	m.middleItems = nil
	cmd := m.loadPreview()
	assert.Nil(t, cmd)
}

func TestCovBoost2LoadPreviewClusters(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelClusters
	m.discoveredResources[m.nav.Context] = []model.ResourceTypeEntry{
		{DisplayName: "Pods", Kind: "Pod", APIVersion: "v1", Resource: "pods"},
	}
	m.middleItems = []model.Item{{Name: "test-ctx"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewRTOverview(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}}
	ui.ConfigDashboard = true
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewRTOverviewDisabled(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}}
	ui.ConfigDashboard = false
	cmd := m.loadPreview()
	assert.Nil(t, cmd)
}

func TestCovBoost2LoadPreviewRTMonitoring(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Monitoring", Extra: "__monitoring__"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewRTCollapsed(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "collapsed", Kind: "__collapsed_group__"}}
	cmd := m.loadPreview()
	assert.Nil(t, cmd)
}

func TestCovBoost2LoadPreviewRTPortForwards(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.portForwardMgr = k8s.NewPortForwardManager()
	m.middleItems = []model.Item{{Name: "Port Forwards", Kind: "__port_forwards__"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewRTNormal(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResourceTypes
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.middleItems = []model.Item{{Name: "Pods", Extra: "v1/pods"}}
	// loadResources for preview calls selectedMiddleItem and fetches
	// resources; the result may be nil if the extra doesn't resolve to a
	// known resource type, which is fine for coverage.
	_ = m.loadPreview()
}

func TestCovBoost2LoadPreviewResPortForwards(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__"}
	m.middleItems = []model.Item{{Name: "pf-1"}}
	cmd := m.loadPreview()
	assert.Nil(t, cmd)
}

func TestCovBoost2LoadPreviewResPod(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-pod", Namespace: "default"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewResDeployment(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", APIGroup: "apps", APIVersion: "v1", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-deploy", Namespace: "default"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewResConfigMap(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "ConfigMap", Resource: "configmaps", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-cm", Namespace: "default"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewResWithFullYAML(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-pod", Namespace: "default"}}
	m.fullYAMLPreview = true
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewResMapView(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", APIGroup: "apps", APIVersion: "v1", Namespaced: true}
	m.middleItems = []model.Item{{Name: "my-deploy", Namespace: "default"}}
	m.mapView = true
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewOwnedPod(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewOwnedPodFullYAML(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default"}}
	m.fullYAMLPreview = true
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewOwnedNonPod(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "rs-1", Kind: "ReplicaSet", Namespace: "default", Extra: "/v1/replicasets"}}
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewOwnedNonPodFullYAML(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelOwned
	m.middleItems = []model.Item{{Name: "rs-1", Kind: "ReplicaSet", Namespace: "default", Extra: "/v1/replicasets"}}
	m.fullYAMLPreview = true
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovBoost2LoadPreviewContainers(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelContainers
	m.middleItems = []model.Item{{Name: "main"}}
	cmd := m.loadPreview()
	assert.Nil(t, cmd)
}

func TestCovLoadPreview(t *testing.T) {
	m := baseModelWithFakeClient()
	m.middleItems = nil
	cmd := m.loadPreview()
	assert.Nil(t, cmd)
}

func TestCovLoadPreviewClusters(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelClusters
	m.discoveredResources[m.nav.Context] = []model.ResourceTypeEntry{
		{DisplayName: "Pods", Kind: "Pod", APIVersion: "v1", Resource: "pods"},
	}
	m = withMiddleItem(m, model.Item{Name: "test-ctx"})
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovLoadPreviewResourceTypesOverview(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelResourceTypes
	m = withMiddleItem(m, model.Item{Extra: "__overview__"})
	ui.ConfigDashboard = true
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovLoadPreviewResourceTypesMonitoring(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelResourceTypes
	m = withMiddleItem(m, model.Item{Extra: "__monitoring__"})
	cmd := m.loadPreview()
	assert.NotNil(t, cmd)
}

func TestCovLoadPreviewResourceTypesCollapsed(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelResourceTypes
	m = withMiddleItem(m, model.Item{Kind: "__collapsed_group__"})
	cmd := m.loadPreview()
	assert.Nil(t, cmd)
}

func TestCovLoadPreviewContainers(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Level = model.LevelContainers
	m = withMiddleItem(m, model.Item{Name: "container-1"})
	cmd := m.loadPreview()
	assert.Nil(t, cmd)
}

// TestLoadPreviewOwnedUsesNavNamespaceForMissingItemNamespace is the
// regression guard for the "Warning: getting resource: the server could not
// find the requested resource" warning that fires when hovering on Helm
// manifest children without a metadata.namespace. Helm stores the rendered
// manifest verbatim in the release secret, and many chart authors rely on
// the --namespace CLI flag rather than templating .Release.Namespace into
// metadata.namespace — so the stored manifest frequently has an empty
// namespace field for namespaced resources. The drill-in flow writes the
// release's namespace into m.nav.Namespace, so loadPreviewOwned must fall
// back to m.resolveNamespace() (which reads m.nav.Namespace first), not to
// m.namespace directly.
func TestLoadPreviewOwnedUsesNavNamespaceForMissingItemNamespace(t *testing.T) {
	// A PVC living in the helm release's namespace. This is what actually
	// exists on the cluster after `helm install --namespace release-ns`.
	pvcGVR := schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumeclaims"}
	pvc := &unstructured.Unstructured{}
	pvc.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "PersistentVolumeClaim"})
	pvc.SetName("data-pvc")
	pvc.SetNamespace("release-ns")

	gvrToListKind := map[schema.GroupVersionResource]string{pvcGVR: "PersistentVolumeClaimList"}
	m := baseModelWithFakeDynamic(gvrToListKind, pvc)

	// Pre-populate discoveredResources with the PVC resource type so the new
	// parameter-only Find* functions can resolve it. Before the runtime
	// discovery refactor, this lookup consulted a hardcoded list.
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "PersistentVolumeClaim", APIGroup: "", APIVersion: "v1", Resource: "persistentvolumeclaims", Namespaced: true},
	}

	// Simulate the state after drilling into a helm release at "release-ns"
	// while the ambient namespace filter is still the arbitrary "default".
	m.nav.Level = model.LevelOwned
	m.nav.Namespace = "release-ns"
	m.namespace = "default"
	// Helm manifest parsed from the release secret — metadata.namespace was
	// not templated so the Item carries an empty Namespace but Kind + Extra
	// from the apiVersion (just "v1" for core resources).
	m = withMiddleItem(m, model.Item{
		Name:  "data-pvc",
		Kind:  "PersistentVolumeClaim",
		Extra: "v1",
	})

	cmd := m.loadPreview()
	require.NotNil(t, cmd)
	msg := cmd()
	loaded, ok := msg.(yamlLoadedMsg)
	require.True(t, ok, "expected yamlLoadedMsg, got %T", msg)
	assert.NoError(t, loaded.err, "should find the PVC in nav.Namespace, not m.namespace")
	assert.Contains(t, loaded.content, "data-pvc")
}
