package app

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// newRichDynClient creates a fake dynamic client pre-populated with resources
// for comprehensive dashboard testing.
func newRichDynClient() *dynfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "nodes"}:                      "NodeList",
		{Group: "", Version: "v1", Resource: "pods"}:                       "PodList",
		{Group: "", Version: "v1", Resource: "namespaces"}:                 "NamespaceList",
		{Group: "", Version: "v1", Resource: "events"}:                     "EventList",
		{Group: "", Version: "v1", Resource: "secrets"}:                    "SecretList",
		{Group: "", Version: "v1", Resource: "configmaps"}:                 "ConfigMapList",
		{Group: "", Version: "v1", Resource: "services"}:                   "ServiceList",
		{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}:     "PersistentVolumeClaimList",
		{Group: "", Version: "v1", Resource: "resourcequotas"}:             "ResourceQuotaList",
		{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}: "PodDisruptionBudgetList",
		{Group: "apps", Version: "v1", Resource: "deployments"}:            "DeploymentList",
		{Group: "apps", Version: "v1", Resource: "replicasets"}:            "ReplicaSetList",
		{Group: "apps", Version: "v1", Resource: "statefulsets"}:           "StatefulSetList",
		{Group: "apps", Version: "v1", Resource: "daemonsets"}:             "DaemonSetList",
		{Group: "batch", Version: "v1", Resource: "jobs"}:                  "JobList",
		{Group: "batch", Version: "v1", Resource: "cronjobs"}:              "CronJobList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}:   "NodeMetricsList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:    "PodMetricsList",
	}

	// Create nodes.
	node1 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Node",
		"metadata": map[string]interface{}{"name": "node-1"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
			"allocatable": map[string]interface{}{
				"cpu":    "4",
				"memory": "8Gi",
			},
		},
	}}
	node2 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Node",
		"metadata": map[string]interface{}{"name": "node-2"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "False"},
			},
			"allocatable": map[string]interface{}{
				"cpu":    "2",
				"memory": "4Gi",
			},
		},
	}}

	// Create pods with different statuses.
	pod1 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]interface{}{"name": "pod-running", "namespace": "default"},
		"status":   map[string]interface{}{"phase": "Running"},
	}}
	pod2 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]interface{}{"name": "pod-pending", "namespace": "default"},
		"status":   map[string]interface{}{"phase": "Pending"},
	}}
	pod3 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]interface{}{"name": "pod-failed", "namespace": "default"},
		"status":   map[string]interface{}{"phase": "Failed"},
	}}

	// Namespaces.
	ns1 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Namespace",
		"metadata": map[string]interface{}{"name": "default"},
	}}
	ns2 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Namespace",
		"metadata": map[string]interface{}{"name": "kube-system"},
	}}

	// Events.
	evt1 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Event",
		"metadata": map[string]interface{}{"name": "evt-warning", "namespace": "default"},
		"type":     "Warning",
		"reason":   "FailedScheduling",
		"message":  "0/2 nodes are available",
		"count":    int64(3),
		"involvedObject": map[string]interface{}{
			"name": "pod-pending",
		},
	}}
	evt2 := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Event",
		"metadata": map[string]interface{}{"name": "evt-normal", "namespace": "default"},
		"type":     "Normal",
		"reason":   "Pulled",
		"message":  "Successfully pulled image",
		"involvedObject": map[string]interface{}{
			"name": "pod-running",
		},
	}}

	return dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs,
		node1, node2, pod1, pod2, pod3, ns1, ns2, evt1, evt2)
}

func baseRichModel() Model {
	cs := fake.NewClientset()
	dyn := newRichDynClient()

	m := Model{
		nav:            model.NavigationState{Level: model.LevelResources, Context: "test-ctx"},
		tabs:           []TabState{{}},
		selectedItems:  make(map[string]bool),
		cursorMemory:   make(map[string]int),
		itemCache:      make(map[string][]model.Item),
		discoveredCRDs: make(map[string][]model.ResourceTypeEntry),
		width:          120,
		height:         40,
		execMu:         &sync.Mutex{},
		client:         k8s.NewTestClient(cs, dyn),
		namespace:      "default",
		reqCtx:         context.Background(),
	}
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Pod",
		Resource:   "pods",
		Namespaced: true,
	}
	m.actionCtx = actionContext{
		context:      "test-ctx",
		name:         "test-resource",
		namespace:    "default",
		kind:         "Pod",
		resourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
	}

	return m
}

// =====================================================================
// loadDashboard with rich data -- covers many branches
// =====================================================================

func TestFinal3LoadDashboardRichData(t *testing.T) {
	m := baseRichModel()
	cmd := m.loadDashboard()
	require.NotNil(t, cmd)
	msg := cmd()
	result, ok := msg.(dashboardLoadedMsg)
	require.True(t, ok, "expected dashboardLoadedMsg, got %T", msg)

	content := stripANSI(result.content)
	assert.Contains(t, content, "CLUSTER DASHBOARD")
	assert.Contains(t, content, "Nodes:")
	assert.Contains(t, content, "Pods:")
	assert.Contains(t, content, "Namespaces:")
	assert.Equal(t, "test-ctx", result.context)
}

func TestFinal3LoadDashboardEventsContent(t *testing.T) {
	m := baseRichModel()
	cmd := m.loadDashboard()
	msg := cmd()
	result := msg.(dashboardLoadedMsg)
	events := stripANSI(result.events)
	assert.Contains(t, events, "RECENT EVENTS")
}

func TestFinal3LoadDashboardNotEmpty(t *testing.T) {
	m := baseRichModel()
	cmd := m.loadDashboard()
	msg := cmd()
	result := msg.(dashboardLoadedMsg)
	assert.NotEmpty(t, result.content)
	assert.NotEmpty(t, result.events)
}

// =====================================================================
// Update: more message types that touch update.go branches
// =====================================================================

func TestFinal3UpdateContextsLoadedWithSession(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	m.pendingSession = &SessionState{Context: "test-ctx", Namespace: "default"}
	m.sessionRestored = false
	items := []model.Item{{Name: "test-ctx"}}
	result, cmd := m.Update(contextsLoadedMsg{items: items})
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.sessionRestored)
}

func TestFinal3UpdateYAMLClipboardMsg(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(yamlClipboardMsg{})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "YAML copied")
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateYAMLClipboardMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(yamlClipboardMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

// Error paths for loaded messages -- hit the switch case without complex struct setup.

func TestFinal3UpdateSecretDataLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(secretDataLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateConfigMapDataLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(configMapDataLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateLabelDataLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(labelDataLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateCanILoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(canILoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateAlertsLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(alertsLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateQuotaLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(quotaLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateNetpolLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(netpolLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateLogContainersLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	result, _ := m.Update(logContainersLoadedMsg{err: assert.AnError})
	_ = result.(Model)
}

func TestFinal3UpdateBulkActionResultMsg(t *testing.T) {
	m := baseFinalModel()
	m.bulkMode = true
	result, cmd := m.Update(bulkActionResultMsg{succeeded: 2})
	rm := result.(Model)
	assert.False(t, rm.bulkMode)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdateBulkActionResultMsgErrors(t *testing.T) {
	m := baseFinalModel()
	m.bulkMode = true
	result, cmd := m.Update(bulkActionResultMsg{
		failed: 1,
		errors: []string{"failed to delete pod-1"},
	})
	rm := result.(Model)
	assert.False(t, rm.bulkMode)
	assert.NotNil(t, cmd)
}

func TestFinal3UpdatePreviewEventsLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 1
	result, _ := m.Update(previewEventsLoadedMsg{gen: 1})
	_ = result.(Model)
}

func TestFinal3UpdatePreviewEventsLoadedMsgStale(t *testing.T) {
	m := baseFinalModel()
	m.requestGen = 2
	result, cmd := m.Update(previewEventsLoadedMsg{gen: 1})
	assert.Nil(t, cmd)
	_ = result.(Model)
}
