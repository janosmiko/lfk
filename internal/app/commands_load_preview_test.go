package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
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
