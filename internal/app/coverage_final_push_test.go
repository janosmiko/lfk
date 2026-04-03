package app

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// newFinalDynClient creates a fake dynamic client with common GVRs registered.
func newFinalDynClient() *dynfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "nodes"}:                                     "NodeList",
		{Group: "", Version: "v1", Resource: "pods"}:                                      "PodList",
		{Group: "", Version: "v1", Resource: "namespaces"}:                                "NamespaceList",
		{Group: "", Version: "v1", Resource: "events"}:                                    "EventList",
		{Group: "", Version: "v1", Resource: "secrets"}:                                   "SecretList",
		{Group: "", Version: "v1", Resource: "configmaps"}:                                "ConfigMapList",
		{Group: "", Version: "v1", Resource: "services"}:                                  "ServiceList",
		{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}:                    "PersistentVolumeClaimList",
		{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}:                "PodDisruptionBudgetList",
		{Group: "apps", Version: "v1", Resource: "deployments"}:                           "DeploymentList",
		{Group: "apps", Version: "v1", Resource: "replicasets"}:                           "ReplicaSetList",
		{Group: "apps", Version: "v1", Resource: "statefulsets"}:                          "StatefulSetList",
		{Group: "apps", Version: "v1", Resource: "daemonsets"}:                            "DaemonSetList",
		{Group: "batch", Version: "v1", Resource: "jobs"}:                                 "JobList",
		{Group: "batch", Version: "v1", Resource: "cronjobs"}:                             "CronJobList",
		{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}:          "NetworkPolicyList",
		{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}:             "ApplicationList",
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"}: "KustomizationList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes"}:                  "NodeMetricsList",
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}:                   "PodMetricsList",
		{Group: "", Version: "v1", Resource: "resourcequotas"}:                            "ResourceQuotaList",
	}
	return dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs)
}

// baseFinalModelWithDynamic returns a Model with a properly configured dynamic client.
func baseFinalModelWithDynamic() Model {
	cs := fake.NewClientset()
	dyn := newFinalDynClient()

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

// baseFinalModel returns a Model with fake K8s client for final push tests.
func baseFinalModel() Model {
	cs := fake.NewClientset()
	dyn := dynfake.NewSimpleDynamicClient(runtime.NewScheme())

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
// loadDashboard -- covers the massive function at ~2% coverage
// =====================================================================

func TestFinalLoadDashboardReturnsCmd(t *testing.T) {
	m := baseFinalModel()
	cmd := m.loadDashboard()
	require.NotNil(t, cmd)
}

func TestFinalLoadDashboardExecutesAndReturnsDashboardMsg(t *testing.T) {
	m := baseFinalModelWithDynamic()
	cmd := m.loadDashboard()
	require.NotNil(t, cmd)
	msg := cmd()
	result, ok := msg.(dashboardLoadedMsg)
	require.True(t, ok, "expected dashboardLoadedMsg, got %T", msg)
	assert.NotEmpty(t, result.content)
	assert.Equal(t, "test-ctx", result.context)
}

func TestFinalLoadDashboardContentContainsSections(t *testing.T) {
	m := baseFinalModelWithDynamic()
	cmd := m.loadDashboard()
	msg := cmd()
	result := msg.(dashboardLoadedMsg)
	stripped := stripANSI(result.content)
	assert.Contains(t, stripped, "CLUSTER DASHBOARD")
	assert.Contains(t, stripped, "Nodes:")
	assert.Contains(t, stripped, "Namespaces:")
	assert.Contains(t, stripped, "Pods:")
}

func TestFinalLoadDashboardEventsColumn(t *testing.T) {
	m := baseFinalModelWithDynamic()
	cmd := m.loadDashboard()
	msg := cmd()
	result := msg.(dashboardLoadedMsg)
	stripped := stripANSI(result.events)
	assert.Contains(t, stripped, "RECENT EVENTS")
}

// =====================================================================
// loadMonitoringDashboard -- covers ~96% uncovered
// =====================================================================

func TestFinalLoadMonitoringDashboardReturnsCmd(t *testing.T) {
	m := baseFinalModelWithDynamic()
	cmd := m.loadMonitoringDashboard()
	require.NotNil(t, cmd)
}

func TestFinalLoadMonitoringDashboardNamespace(t *testing.T) {
	m := baseFinalModelWithDynamic()
	m.namespace = "custom-ns"
	cmd := m.loadMonitoringDashboard()
	require.NotNil(t, cmd)
}

func TestFinalLoadMonitoringDashboardAllNamespaces(t *testing.T) {
	m := baseFinalModelWithDynamic()
	m.allNamespaces = true
	cmd := m.loadMonitoringDashboard()
	require.NotNil(t, cmd)
}

// =====================================================================
// executeAction -- test more branches of the switch (currently 13.7%)
// =====================================================================

func TestFinalExecuteActionDescribe(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Describe")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalExecuteActionEdit(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Edit")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalExecuteActionDelete(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Delete")
	assert.Nil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayConfirm, rm.overlay)
	assert.Equal(t, "Delete", rm.pendingAction)
}

func TestFinalExecuteActionScale(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Scale")
	assert.Nil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayScaleInput, rm.overlay)
}

func TestFinalExecuteActionResize(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = []model.KeyValue{{Key: "Capacity", Value: "10Gi"}}
	result, cmd := m.executeAction("Resize")
	assert.Nil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayPVCResize, rm.overlay)
	assert.Equal(t, "10Gi", rm.pvcCurrentSize)
}

func TestFinalExecuteActionResizeCapacity(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = []model.KeyValue{{Key: "CAPACITY", Value: "5Gi"}}
	result, _ := m.executeAction("Resize")
	rm := result.(Model)
	assert.Equal(t, "5Gi", rm.pvcCurrentSize)
}

func TestFinalExecuteActionResizeNoCapacity(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = []model.KeyValue{{Key: "Other", Value: "5Gi"}}
	result, _ := m.executeAction("Resize")
	rm := result.(Model)
	assert.Equal(t, "", rm.pvcCurrentSize)
}

func TestFinalExecuteActionRestart(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Restart")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionRestartPortForward(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "__port_forward_entry__"
	result, _ := m.executeAction("Restart")
	_ = result.(Model)
}

func TestFinalExecuteActionRollback(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Rollback")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionRollbackHelm(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "HelmRelease"
	result, cmd := m.executeAction("Rollback")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionPortForward(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Port Forward")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionDebug(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Debug")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionEvents(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Events")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionSecretEditor(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Secret Editor")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionConfigMapEditor(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("ConfigMap Editor")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionForceDelete(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Force Delete")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionForceFinalize(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Force Finalize")
	assert.Nil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Equal(t, "Force Finalize", rm.pendingAction)
}

func TestFinalExecuteActionCordon(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	result, cmd := m.executeAction("Cordon")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionUncordon(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	result, cmd := m.executeAction("Uncordon")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionDrain(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	result, cmd := m.executeAction("Drain")
	assert.Nil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, overlayConfirm, rm.overlay)
	assert.Equal(t, "Drain", rm.pendingAction)
}

func TestFinalExecuteActionTaint(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	result, _ := m.executeAction("Taint")
	rm := result.(Model)
	assert.True(t, rm.commandBarActive)
}

func TestFinalExecuteActionUntaint(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	m.actionCtx.columns = []model.KeyValue{{Key: "Taints", Value: "key1=value1:NoSchedule, key2:NoExecute"}}
	result, _ := m.executeAction("Untaint")
	rm := result.(Model)
	assert.True(t, rm.commandBarActive)
}

func TestFinalExecuteActionUntaintNoTaints(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	result, _ := m.executeAction("Untaint")
	rm := result.(Model)
	assert.True(t, rm.commandBarActive)
}

func TestFinalExecuteActionTrigger(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "CronJob"
	result, cmd := m.executeAction("Trigger")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionShell(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Node"
	result, cmd := m.executeAction("Shell")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionDebugPod(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Debug Pod")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionLogsDirectPod(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = "main"
	result, cmd := m.executeAction("Logs")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, modeLogs, rm.mode)
	assert.Contains(t, rm.logTitle, "main")
}

func TestFinalExecuteActionLogsGroupResource(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Logs")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, modeLogs, rm.mode)
	assert.Equal(t, "Deployment", rm.logParentKind)
}

func TestFinalExecuteActionLogsUnknownKind(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "ConfigMap"
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Logs")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "Logs", rm.pendingAction)
}

func TestFinalExecuteActionExecDirectWithContainer(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = "web"
	result, cmd := m.executeAction("Exec")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionExecDeployment(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Exec")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "Exec", rm.pendingAction)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionExecNoContainer(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Exec")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "Exec", rm.pendingAction)
}

func TestFinalExecuteActionAttachDirect(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = "web"
	result, cmd := m.executeAction("Attach")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionAttachDeployment(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "StatefulSet"
	result, cmd := m.executeAction("Attach")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "Attach", rm.pendingAction)
}

func TestFinalExecuteActionAttachNoContainer(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Attach")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "Attach", rm.pendingAction)
}

func TestFinalExecuteActionGoToPodNoPods(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = nil
	result, cmd := m.executeAction("Go to Pod")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "No pods")
}

func TestFinalExecuteActionGoToPodSingle(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = []model.KeyValue{{Key: "Used By", Value: "my-pod"}}
	m.nav.Level = model.LevelResources
	result, _ := m.executeAction("Go to Pod")
	_ = result.(Model)
}

func TestFinalExecuteActionGoToPodMultiple(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = []model.KeyValue{{Key: "Used By", Value: "pod-a, pod-b"}}
	result, _ := m.executeAction("Go to Pod")
	rm := result.(Model)
	assert.Equal(t, overlayPodSelect, rm.overlay)
	assert.Equal(t, 2, len(rm.overlayItems))
}

func TestFinalExecuteActionBulkMode(t *testing.T) {
	m := baseFinalModel()
	m.bulkMode = true
	m.bulkItems = []model.Item{{Name: "pod-1", Namespace: "default"}}
	m.actionCtx.context = "test-ctx"
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}
	result, _ := m.executeAction("Delete")
	_ = result.(Model)
}

// =====================================================================
// handleOverlayKey -- covers more overlay types (currently 42.5%)
// =====================================================================

func TestFinalHandleOverlayKeyRBAC(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayRBAC
	result, cmd := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Nil(t, cmd)
}

func TestFinalHandleOverlayKeyPodStartup(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayPodStartup
	result, cmd := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Nil(t, cmd)
}

func TestFinalHandleOverlayKeyQuotaDashboard(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayQuotaDashboard
	result, cmd := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Nil(t, cmd)
}

func TestFinalHandleOverlayKeyQuotaDashboardQ(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayQuotaDashboard
	result, cmd := m.handleOverlayKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Nil(t, cmd)
}

func TestFinalHandleOverlayKeyQuotaDashboardOtherKey(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayQuotaDashboard
	result, cmd := m.handleOverlayKey(keyMsg("j"))
	rm := result.(Model)
	// Should stay open on other keys
	assert.Equal(t, overlayQuotaDashboard, rm.overlay)
	assert.Nil(t, cmd)
}

func TestFinalHandleOverlayKeyConfirm(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayConfirm
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyAction(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayAction
	m.overlayItems = []model.Item{{Name: "Delete"}}
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyScaleInput(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayScaleInput
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyPVCResize(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayPVCResize
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyConfirmType(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayConfirmType
	m.confirmAction = "test-res"
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyBookmarks(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyTemplates(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayTemplates
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyColorscheme(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayColorscheme
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyNamespace(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayNamespace
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyContainerSelect(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayContainerSelect
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyPodSelect(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayPodSelect
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyPortForward(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayPortForward
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyRollback(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayRollback
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyHelmRollback(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayHelmRollback
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyLabelEditor(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayLabelEditor
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeySecretEditor(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlaySecretEditor
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyConfigMapEditor(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayConfigMapEditor
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyAlerts(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayAlerts
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyBatchLabel(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBatchLabel
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyEventTimeline(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayEventTimeline
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyNetworkPolicy(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayNetworkPolicy
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyCanI(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayCanI
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyCanISubject(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayCanISubject
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	// Esc in CanISubject may go back to CanI overlay, not overlayNone.
	assert.NotEqual(t, overlayCanISubject, rm.overlay)
}

func TestFinalHandleOverlayKeyExplainSearch(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayExplainSearch
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyFilterPreset(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayFilterPreset
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyAutoSync(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayAutoSync
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyQuitConfirm(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayQuitConfirm
	result, _ := m.handleOverlayKey(keyMsg("n"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

// =====================================================================
// navigateChild -- test more branches (currently 36.4%)
// =====================================================================

func TestFinalNavigateChildNoSelection(t *testing.T) {
	m := baseFinalModel()
	m.middleItems = nil
	result, cmd := m.navigateChild()
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinalNavigateChildLevelClusters(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelClusters
	m.middleItems = []model.Item{{Name: "cluster-1"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
	assert.Equal(t, "cluster-1", rm.nav.Context)
}

func TestFinalNavigateChildLevelResourceTypesOverview(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.middleItems = []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}}
	m.setCursor(0)
	result, _ := m.navigateChild()
	rm := result.(Model)
	assert.True(t, rm.fullscreenDashboard)
}

func TestFinalNavigateChildLevelResourceTypesMonitoring(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.middleItems = []model.Item{{Name: "Monitoring", Extra: "__monitoring__"}}
	m.setCursor(0)
	result, _ := m.navigateChild()
	rm := result.(Model)
	assert.True(t, rm.fullscreenDashboard)
}

func TestFinalNavigateChildLevelResourceTypesUnknownExtra(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	// Extra doesn't match any known resource type -> returns nil cmd.
	m.middleItems = []model.Item{{Name: "Unknown", Extra: "nonexistent-resource"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinalNavigateChildLevelResourceTypesCollapsedGroup(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.middleItems = []model.Item{{Name: "Core", Kind: "__collapsed_group__", Category: "Core"}}
	m.setCursor(0)
	result, _ := m.navigateChild()
	rm := result.(Model)
	assert.Equal(t, "Core", rm.expandedGroup)
}

func TestFinalNavigateChildLevelResourceTypesPods(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	// Set up CRDs so FindResourceTypeIn works.
	podRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.discoveredCRDs["test-ctx"] = []model.ResourceTypeEntry{podRT}
	// Extra must match ResourceRef format.
	m.middleItems = []model.Item{{Name: "Pods", Extra: podRT.ResourceRef()}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, model.LevelResources, rm.nav.Level)
}

func TestFinalNavigateChildLevelResourcesNonPodNoChildren(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "ConfigMap", Resource: "configmaps", Namespaced: true}
	m.middleItems = []model.Item{{Name: "cm-1", Namespace: "default"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	assert.Nil(t, cmd)
	_ = result.(Model)
}

func TestFinalNavigateChildLevelResourcesPod(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "pod-1", Namespace: "default"}}
	m.setCursor(0)
	result, cmd := m.navigateChild()
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, model.LevelContainers, rm.nav.Level)
}

// =====================================================================
// Update main handler -- test more message types
// =====================================================================

func TestFinalUpdateWindowSizeMsg(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	rm := result.(Model)
	assert.Equal(t, 200, rm.width)
	assert.Equal(t, 50, rm.height)
	assert.Nil(t, cmd)
}

func TestFinalUpdateResourceTypesMsg(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelClusters
	items := []model.Item{{Name: "Pods"}, {Name: "Deployments"}}
	result, _ := m.Update(resourceTypesMsg{items: items})
	rm := result.(Model)
	assert.Equal(t, items, rm.rightItems)
}

func TestFinalUpdateResourceTypesMsgAtResourceTypes(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	items := []model.Item{{Name: "Pods"}}
	result, _ := m.Update(resourceTypesMsg{items: items})
	rm := result.(Model)
	assert.Equal(t, items, rm.middleItems)
}

func TestFinalUpdateContextsLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	items := []model.Item{{Name: "cluster-1"}}
	result, cmd := m.Update(contextsLoadedMsg{items: items})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Equal(t, items, rm.middleItems)
	assert.NotNil(t, cmd)
}

func TestFinalUpdateContextsLoadedMsgWithError(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, _ := m.Update(contextsLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.NotNil(t, rm.err)
}

func TestFinalUpdateContextsLoadedMsgCanceled(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, _ := m.Update(contextsLoadedMsg{err: context.Canceled})
	rm := result.(Model)
	assert.Nil(t, rm.err)
}

func TestFinalUpdateCRDDiscoveryMsg(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResourceTypes
	entries := []model.ResourceTypeEntry{{Kind: "CustomResource", Resource: "crs"}}
	result, _ := m.Update(crdDiscoveryMsg{context: "test-ctx", entries: entries})
	rm := result.(Model)
	assert.Contains(t, rm.discoveredCRDs, "test-ctx")
}

func TestFinalUpdateCRDDiscoveryMsgError(t *testing.T) {
	m := baseFinalModel()
	result, _ := m.Update(crdDiscoveryMsg{context: "test-ctx", err: assert.AnError})
	_ = result.(Model)
}

func TestFinalUpdateCRDDiscoveryMsgCanceled(t *testing.T) {
	m := baseFinalModel()
	result, _ := m.Update(crdDiscoveryMsg{err: context.Canceled})
	_ = result.(Model)
}

func TestFinalUpdateDashboardLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.nav.Context = "test-ctx"
	result, _ := m.Update(dashboardLoadedMsg{content: "dashboard content", context: "test-ctx"})
	rm := result.(Model)
	assert.Equal(t, "dashboard content", rm.dashboardPreview)
}

func TestFinalUpdateDashboardLoadedMsgDifferentCtx(t *testing.T) {
	m := baseFinalModel()
	m.nav.Context = "test-ctx"
	result, _ := m.Update(dashboardLoadedMsg{content: "other", context: "other-ctx"})
	rm := result.(Model)
	assert.NotEqual(t, "other", rm.dashboardPreview)
}

func TestFinalUpdateMonitoringDashboardMsg(t *testing.T) {
	m := baseFinalModel()
	m.nav.Context = "test-ctx"
	result, _ := m.Update(monitoringDashboardMsg{content: "monitoring content", context: "test-ctx"})
	rm := result.(Model)
	assert.Equal(t, "monitoring content", rm.monitoringPreview)
}

func TestFinalUpdateActionResultMsgSuccess(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(actionResultMsg{message: "Done"})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.Contains(t, rm.statusMessage, "Done")
	assert.NotNil(t, cmd)
}

func TestFinalUpdateActionResultMsgError(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, _ := m.Update(actionResultMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.loading)
}

func TestFinalUpdateStatusClearMsg(t *testing.T) {
	m := baseFinalModel()
	m.setStatusMessage("test message", false)
	result, _ := m.Update(statusMessageExpiredMsg{})
	rm := result.(Model)
	assert.Empty(t, rm.statusMessage)
}

// =====================================================================
// Bookmark operations -- covers update_bookmarks.go
// =====================================================================

func TestFinalBookmarkToSlotTooLow(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelClusters
	result, _ := m.bookmarkToSlot("a")
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Navigate to a resource type")
}

func TestFinalBookmarkToSlotLocal(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	result, cmd := m.bookmarkToSlot("a")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Mark 'a' set")
}

func TestFinalBookmarkToSlotGlobal(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.Context = "prod-cluster"
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	result, cmd := m.bookmarkToSlot("A")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Mark 'A' set")
}

func TestFinalBookmarkToSlotOverwrite(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	m.bookmarks = []model.Bookmark{{Slot: "a", Name: "existing"}}
	result, cmd := m.bookmarkToSlot("a")
	assert.Nil(t, cmd) // Should prompt for confirmation
	rm := result.(Model)
	assert.NotNil(t, rm.pendingBookmark)
}

func TestFinalBookmarkToSlotAllNamespaces(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	m.allNamespaces = true
	result, cmd := m.bookmarkToSlot("b")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalBookmarkToSlotMultiNamespaces(t *testing.T) {
	m := baseFinalModel()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	m.selectedNamespaces = map[string]bool{"ns1": true, "ns2": true}
	result, cmd := m.bookmarkToSlot("c")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalJumpToSlotNotFound(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.jumpToSlot("z")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "not set")
}

func TestFinalFilteredBookmarksEmpty(t *testing.T) {
	m := baseFinalModel()
	m.bookmarks = nil
	result := m.filteredBookmarks()
	assert.Empty(t, result)
}

func TestFinalFilteredBookmarksNoFilter(t *testing.T) {
	m := baseFinalModel()
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}, {Name: "bm2", Slot: "b"}}
	result := m.filteredBookmarks()
	assert.Len(t, result, 2)
}

func TestFinalBookmarkDeleteCurrentEmpty(t *testing.T) {
	m := baseFinalModel()
	m.bookmarks = nil
	cmd := m.bookmarkDeleteCurrent()
	assert.Nil(t, cmd)
}

func TestFinalBookmarkDeleteCurrentValid(t *testing.T) {
	m := baseFinalModel()
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}
	m.overlayCursor = 0
	cmd := m.bookmarkDeleteCurrent()
	assert.NotNil(t, cmd)
	assert.Empty(t, m.bookmarks)
}

func TestFinalBookmarkDeleteAll(t *testing.T) {
	m := baseFinalModel()
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}, {Name: "bm2", Slot: "b"}}
	cmd := m.bookmarkDeleteAll()
	assert.NotNil(t, cmd)
	assert.Nil(t, m.bookmarks)
}

// =====================================================================
// Bookmark overlay key handling
// =====================================================================

func TestFinalHandleBookmarkNormalModeEsc(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	result, _ := m.handleBookmarkOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleBookmarkNormalModeJ(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}, {Name: "bm2", Slot: "b"}}
	m.overlayCursor = 0
	result, _ := m.handleBookmarkOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)
}

func TestFinalHandleBookmarkNormalModeK(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}, {Name: "bm2", Slot: "b"}}
	m.overlayCursor = 1
	result, _ := m.handleBookmarkOverlayKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.overlayCursor)
}

func TestFinalHandleBookmarkNormalModeGG(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}, {Name: "bm2", Slot: "b"}}
	m.overlayCursor = 1
	m.pendingG = true
	result, _ := m.handleBookmarkOverlayKey(keyMsg("g"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.overlayCursor)
}

func TestFinalHandleBookmarkNormalModeG(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	result, _ := m.handleBookmarkOverlayKey(keyMsg("G"))
	rm := result.(Model)
	_ = rm
}

func TestFinalHandleBookmarkNormalModeSlash(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	result, _ := m.handleBookmarkOverlayKey(keyMsg("/"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeFilter, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkNormalModeD(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}
	m.overlayCursor = 0
	result, _ := m.handleBookmarkOverlayKey(keyMsg("D"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeConfirmDelete, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkNormalModeCtrlX(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}
	result, _ := m.handleBookmarkOverlayKey(keyMsg("ctrl+x"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeConfirmDeleteAll, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkNormalModeCtrlD(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = make([]model.Bookmark, 20)
	m.overlayCursor = 0
	result, _ := m.handleBookmarkOverlayKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.Greater(t, rm.overlayCursor, 0)
}

func TestFinalHandleBookmarkNormalModeCtrlU(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = make([]model.Bookmark, 20)
	m.overlayCursor = 15
	result, _ := m.handleBookmarkOverlayKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.Less(t, rm.overlayCursor, 15)
}

func TestFinalHandleBookmarkNormalModeCtrlF(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = make([]model.Bookmark, 30)
	m.overlayCursor = 0
	result, _ := m.handleBookmarkOverlayKey(keyMsg("ctrl+f"))
	rm := result.(Model)
	assert.Greater(t, rm.overlayCursor, 0)
}

func TestFinalHandleBookmarkNormalModeCtrlB(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = make([]model.Bookmark, 30)
	m.overlayCursor = 25
	result, _ := m.handleBookmarkOverlayKey(keyMsg("ctrl+b"))
	rm := result.(Model)
	assert.Less(t, rm.overlayCursor, 25)
}

func TestFinalHandleBookmarkFilterModeEsc(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeFilter
	result, _ := m.handleBookmarkOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeNormal, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkFilterModeEnter(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeFilter
	result, _ := m.handleBookmarkOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeNormal, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkFilterModeTyping(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeFilter
	result, _ := m.handleBookmarkOverlayKey(keyMsg("a"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.overlayCursor)
}

func TestFinalHandleBookmarkConfirmDeleteYes(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeConfirmDelete
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}
	m.overlayCursor = 0
	result, cmd := m.handleBookmarkOverlayKey(keyMsg("y"))
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, bookmarkModeNormal, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkConfirmDeleteNo(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeConfirmDelete
	result, _ := m.handleBookmarkOverlayKey(keyMsg("n"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeNormal, rm.bookmarkSearchMode)
	assert.Contains(t, rm.statusMessage, "Cancelled")
}

func TestFinalHandleBookmarkConfirmDeleteAllYes(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeConfirmDeleteAll
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}
	result, cmd := m.handleBookmarkOverlayKey(keyMsg("Y"))
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, bookmarkModeNormal, rm.bookmarkSearchMode)
}

func TestFinalHandleBookmarkConfirmDeleteAllNo(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeConfirmDeleteAll
	result, _ := m.handleBookmarkOverlayKey(keyMsg("n"))
	rm := result.(Model)
	assert.Equal(t, bookmarkModeNormal, rm.bookmarkSearchMode)
}

// =====================================================================
// commands_logs.go helpers
// =====================================================================

func TestFinalMatchesContainerFilterEmptyLine(t *testing.T) {
	assert.True(t, matchesContainerFilter("", []string{"main"}))
}

func TestFinalMatchesContainerFilterNoBracket(t *testing.T) {
	assert.True(t, matchesContainerFilter("some log line", []string{"main"}))
}

func TestFinalMatchesContainerFilterNoCloseBracket(t *testing.T) {
	assert.True(t, matchesContainerFilter("[pod/name/container", []string{"main"}))
}

func TestFinalMatchesContainerFilterNoSlash(t *testing.T) {
	assert.True(t, matchesContainerFilter("[container] log", []string{"main"}))
}

func TestFinalMatchesContainerFilterMatch(t *testing.T) {
	assert.True(t, matchesContainerFilter("[pod/my-pod/main] log line", []string{"main"}))
}

func TestFinalMatchesContainerFilterNoMatch(t *testing.T) {
	assert.False(t, matchesContainerFilter("[pod/my-pod/sidecar] log line", []string{"main"}))
}

func TestFinalWaitForLogLineNilChannel(t *testing.T) {
	m := baseFinalModel()
	m.logCh = nil
	cmd := m.waitForLogLine()
	assert.Nil(t, cmd)
}

func TestFinalWaitForLogLineWithChannel(t *testing.T) {
	m := baseFinalModel()
	ch := make(chan string, 1)
	ch <- "test log line"
	m.logCh = ch
	cmd := m.waitForLogLine()
	require.NotNil(t, cmd)
	msg := cmd()
	logMsg := msg.(logLineMsg)
	assert.Equal(t, "test log line", logMsg.line)
	assert.False(t, logMsg.done)
}

func TestFinalWaitForLogLineChannelClosed(t *testing.T) {
	m := baseFinalModel()
	ch := make(chan string)
	close(ch)
	m.logCh = ch
	cmd := m.waitForLogLine()
	require.NotNil(t, cmd)
	msg := cmd()
	logMsg := msg.(logLineMsg)
	assert.True(t, logMsg.done)
}

func TestFinalSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"path/to/file", "path_to_file"},
		{"back\\slash", "back_slash"},
		{"has:colon", "has_colon"},
		{"has space", "has_space"},
		{"a/b:c d\\e", "a_b_c_d_e"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, sanitizeFilename(tt.input))
	}
}

func TestFinalSaveLoadedLogs(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.name = "test-pod"
	m.logLines = []string{"line1", "line2", "line3"}
	path, err := m.saveLoadedLogs()
	require.NoError(t, err)
	assert.Contains(t, path, "lfk-logs-test-pod")
}

func TestFinalMaybeLoadMoreHistoryNotAtTop(t *testing.T) {
	m := baseFinalModel()
	m.logScroll = 5
	m.logHasMoreHistory = true
	cmd := m.maybeLoadMoreHistory()
	assert.Nil(t, cmd)
}

func TestFinalMaybeLoadMoreHistoryNoMore(t *testing.T) {
	m := baseFinalModel()
	m.logScroll = 0
	m.logHasMoreHistory = false
	cmd := m.maybeLoadMoreHistory()
	assert.Nil(t, cmd)
}

func TestFinalMaybeLoadMoreHistoryAlreadyLoading(t *testing.T) {
	m := baseFinalModel()
	m.logScroll = 0
	m.logHasMoreHistory = true
	m.logLoadingHistory = true
	cmd := m.maybeLoadMoreHistory()
	assert.Nil(t, cmd)
}

func TestFinalMaybeLoadMoreHistoryPrevious(t *testing.T) {
	m := baseFinalModel()
	m.logScroll = 0
	m.logHasMoreHistory = true
	m.logPrevious = true
	cmd := m.maybeLoadMoreHistory()
	assert.Nil(t, cmd)
}

// =====================================================================
// commands_exec.go helper functions
// =====================================================================

func TestFinalRandomSuffix(t *testing.T) {
	s1 := randomSuffix(5)
	s2 := randomSuffix(5)
	assert.Len(t, s1, 5)
	assert.Len(t, s2, 5)
	// Extremely unlikely to be the same.
	if s1 == s2 {
		t.Log("random suffixes happened to match (extremely unlikely)")
	}
}

func TestFinalRandomSuffixLength(t *testing.T) {
	for _, n := range []int{0, 1, 3, 10, 20} {
		assert.Len(t, randomSuffix(n), n)
	}
}

func TestFinalCleanANSI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b[1;32mbold green\x1b[0m", "bold green"},
		{"no ansi", "no ansi"},
		{"\x1b[38;5;200mextended\x1b[0m", "extended"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, cleanANSI(tt.input))
	}
}

func TestFinalParseFirstJSONField(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		field  string
		suffix string
		want   string
	}{
		{"exact match", `[{"name":"cilium"}]`, "name", "cilium", "cilium"},
		{"repo prefix", `[{"name":"repo/cilium"}]`, "name", "cilium", "repo/cilium"},
		{"no match", `[{"name":"nginx"}]`, "name", "cilium", ""},
		{"empty json", `[]`, "name", "cilium", ""},
		{"multiple entries", `[{"name":"other"},{"name":"repo/cilium"}]`, "name", "cilium", "repo/cilium"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseFirstJSONField(tt.json, tt.field, tt.suffix))
		})
	}
}

// =====================================================================
// isContextCanceled
// =====================================================================

func TestFinalIsContextCanceled(t *testing.T) {
	assert.False(t, isContextCanceled(nil))
	assert.True(t, isContextCanceled(context.Canceled))
	assert.True(t, isContextCanceled(context.DeadlineExceeded))
	assert.False(t, isContextCanceled(assert.AnError))
}

// =====================================================================
// session restore helpers
// =====================================================================

func TestFinalContextInList(t *testing.T) {
	items := []model.Item{{Name: "ctx-1"}, {Name: "ctx-2"}}
	assert.True(t, contextInList("ctx-1", items))
	assert.True(t, contextInList("ctx-2", items))
	assert.False(t, contextInList("ctx-3", items))
}

func TestFinalContextInListEmpty(t *testing.T) {
	assert.False(t, contextInList("any", nil))
}

func TestFinalApplySessionNamespaces(t *testing.T) {
	m := baseFinalModel()
	applySessionNamespaces(&m, true, "", nil)
	assert.True(t, m.allNamespaces)

	m2 := baseFinalModel()
	applySessionNamespaces(&m2, false, "custom-ns", nil)
	assert.Equal(t, "custom-ns", m2.namespace)
	assert.False(t, m2.allNamespaces)

	m3 := baseFinalModel()
	applySessionNamespaces(&m3, false, "ns1", []string{"ns1", "ns2"})
	assert.Equal(t, "ns1", m3.namespace)
	assert.True(t, m3.selectedNamespaces["ns1"])
	assert.True(t, m3.selectedNamespaces["ns2"])
}

func TestFinalBuildSessionTabState(t *testing.T) {
	st := SessionTab{
		Context:      "ctx-1",
		Namespace:    "ns-1",
		ResourceType: "/v1/pods",
	}
	tab := buildSessionTabState(&st)
	assert.Equal(t, "ctx-1", tab.nav.Context)
	assert.Equal(t, model.LevelResources, tab.nav.Level)
}

func TestFinalBuildSessionTabStateNoResourceType(t *testing.T) {
	st := SessionTab{
		Context: "ctx-1",
	}
	tab := buildSessionTabState(&st)
	assert.Equal(t, model.LevelResourceTypes, tab.nav.Level)
}

func TestFinalBuildSessionTabStateNoContext(t *testing.T) {
	st := SessionTab{}
	tab := buildSessionTabState(&st)
	assert.Equal(t, model.LevelClusters, tab.nav.Level)
}

func TestFinalBuildSessionTabStateAllNamespaces(t *testing.T) {
	st := SessionTab{
		Context:       "ctx-1",
		AllNamespaces: true,
	}
	tab := buildSessionTabState(&st)
	assert.True(t, tab.allNamespaces)
}

func TestFinalBuildSessionTabStateSelectedNS(t *testing.T) {
	st := SessionTab{
		Context:            "ctx-1",
		Namespace:          "ns1",
		SelectedNamespaces: []string{"ns1", "ns2"},
	}
	tab := buildSessionTabState(&st)
	assert.True(t, tab.selectedNamespaces["ns1"])
	assert.True(t, tab.selectedNamespaces["ns2"])
}

func TestFinalBuildSessionTabStateNSOnly(t *testing.T) {
	st := SessionTab{
		Context:   "ctx-1",
		Namespace: "ns1",
	}
	tab := buildSessionTabState(&st)
	assert.Equal(t, "ns1", tab.namespace)
	assert.True(t, tab.selectedNamespaces["ns1"])
}

// =====================================================================
// Argo and Flux action branches (just reach switch cases)
// =====================================================================

func TestFinalExecuteActionSync(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Sync")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionSyncApplyOnly(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Sync (Apply Only)")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionRefreshArgo(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Application"
	result, cmd := m.executeAction("Refresh")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionRefreshArgoAppSet(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "ApplicationSet"
	result, cmd := m.executeAction("Refresh")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionTerminateSync(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Terminate Sync")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionConfigureAutoSync(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Configure AutoSync")
	require.NotNil(t, cmd)
	_ = result.(Model)
}

func TestFinalExecuteActionWatchWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Watch Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
	assert.True(t, rm.describeAutoRefresh)
}

func TestFinalExecuteActionSuspendWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Suspend Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionResumeWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Resume Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionStopWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Stop Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionTerminateWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Terminate Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionResubmitWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Resubmit Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionSubmitWorkflow(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "WorkflowTemplate"
	result, cmd := m.executeAction("Submit Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionSubmitWorkflowCluster(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "ClusterWorkflowTemplate"
	result, cmd := m.executeAction("Submit Workflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionSuspendCronWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Suspend CronWorkflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionResumeCronWorkflow(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Resume CronWorkflow")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionForceRenew(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Force Renew")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionForceRefresh(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Force Refresh")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionPause(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Pause")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionUnpause(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Unpause")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionReconcile(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Reconcile")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionSuspend(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Suspend")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionResume(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Resume")
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.loading)
}

func TestFinalExecuteActionUnknown(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("NonExistentAction")
	assert.Nil(t, cmd)
	_ = result.(Model)
}

// =====================================================================
// Additional formatTimeAgo edge cases
// =====================================================================

func TestFinalFormatTimeAgoExact(t *testing.T) {
	// Just under a minute.
	result := formatTimeAgo(time.Now().Add(-45 * time.Second))
	assert.Contains(t, result, "s ago")

	// Just over a minute.
	result2 := formatTimeAgo(time.Now().Add(-90 * time.Second))
	assert.Contains(t, result2, "m ago")

	// Several hours.
	result3 := formatTimeAgo(time.Now().Add(-5 * time.Hour))
	assert.Contains(t, result3, "h ago")

	// Several days.
	result4 := formatTimeAgo(time.Now().Add(-72 * time.Hour))
	assert.Contains(t, result4, "d ago")
}

// =====================================================================
// NavigateToBookmark edge cases
// =====================================================================

func TestFinalNavigateToBookmarkResourceNotFound(t *testing.T) {
	m := baseFinalModel()
	bm := model.Bookmark{
		ResourceType: "nonexistent",
		Global:       false,
	}
	result, cmd := m.navigateToBookmark(bm)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "not found")
}

func TestFinalNavigateToBookmarkAllNamespaces(t *testing.T) {
	m := baseFinalModel()
	podRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.discoveredCRDs["test-ctx"] = []model.ResourceTypeEntry{podRT}
	bm := model.Bookmark{
		ResourceType: podRT.ResourceRef(),
		Namespace:    "",
		Global:       false,
	}
	result, cmd := m.navigateToBookmark(bm)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.allNamespaces)
}

func TestFinalNavigateToBookmarkMultiNS(t *testing.T) {
	m := baseFinalModel()
	podRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.discoveredCRDs["test-ctx"] = []model.ResourceTypeEntry{podRT}
	bm := model.Bookmark{
		ResourceType: podRT.ResourceRef(),
		Namespaces:   []string{"ns1", "ns2"},
		Global:       false,
	}
	result, cmd := m.navigateToBookmark(bm)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.True(t, rm.selectedNamespaces["ns1"])
	assert.True(t, rm.selectedNamespaces["ns2"])
}

func TestFinalNavigateToBookmarkSingleNS(t *testing.T) {
	m := baseFinalModel()
	podRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.discoveredCRDs["test-ctx"] = []model.ResourceTypeEntry{podRT}
	bm := model.Bookmark{
		ResourceType: podRT.ResourceRef(),
		Namespace:    "production",
		Global:       false,
	}
	result, cmd := m.navigateToBookmark(bm)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Equal(t, "production", rm.namespace)
}

func TestFinalNavigateToBookmarkGlobal(t *testing.T) {
	m := baseFinalModel()
	// Global bookmarks switch context. CRD must have matching ResourceRef.
	podRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.discoveredCRDs["prod-ctx"] = []model.ResourceTypeEntry{podRT}
	bm := model.Bookmark{
		ResourceType: podRT.ResourceRef(),
		Context:      "prod-ctx",
		Namespace:    "default",
		Global:       true,
	}
	result, cmd := m.navigateToBookmark(bm)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Jumped to")
}

func TestFinalNavigateToBookmarkSingleNamespaceInList(t *testing.T) {
	m := baseFinalModel()
	podRT := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true}
	m.discoveredCRDs["test-ctx"] = []model.ResourceTypeEntry{podRT}
	bm := model.Bookmark{
		ResourceType: podRT.ResourceRef(),
		Namespaces:   []string{"only-ns"},
		Global:       false,
	}
	result, cmd := m.navigateToBookmark(bm)
	require.NotNil(t, cmd)
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Jumped to")
}

// =====================================================================
// BookmarkDeleteAll with filter
// =====================================================================

func TestFinalBookmarkDeleteAllWithFilter(t *testing.T) {
	m := baseFinalModel()
	m.bookmarks = []model.Bookmark{
		{Name: "alpha-bm", Slot: "a"},
		{Name: "beta-bm", Slot: "b"},
		{Name: "gamma-bm", Slot: "c"},
	}
	m.bookmarkFilter.Value = "alpha"
	cmd := m.bookmarkDeleteAll()
	assert.NotNil(t, cmd)
	// Only bookmarks not matching the filter should remain.
	assert.Equal(t, 2, len(m.bookmarks))
}

func TestFinalBookmarkDeleteAllEmptyFiltered(t *testing.T) {
	m := baseFinalModel()
	m.bookmarks = nil
	cmd := m.bookmarkDeleteAll()
	assert.Nil(t, cmd)
}

// =====================================================================
// Additional exec command tests -- coverage for functions at 0%
// =====================================================================

func TestFinalDeleteResourceHelm(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.resourceType = model.ResourceTypeEntry{APIGroup: "_helm"}
	cmd := m.deleteResource()
	assert.NotNil(t, cmd)
}

func TestFinalDeleteResourceNormal(t *testing.T) {
	m := baseFinalModel()
	cmd := m.deleteResource()
	assert.NotNil(t, cmd)
}

func TestFinalScaleResource(t *testing.T) {
	m := baseFinalModel()
	cmd := m.scaleResource(3)
	assert.NotNil(t, cmd)
}

func TestFinalRestartResource(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "Deployment"
	cmd := m.restartResource()
	assert.NotNil(t, cmd)
}

func TestFinalResizePVC(t *testing.T) {
	m := baseFinalModel()
	cmd := m.resizePVC("20Gi")
	assert.NotNil(t, cmd)
}

func TestFinalRollbackDeployment(t *testing.T) {
	m := baseFinalModel()
	cmd := m.rollbackDeployment(1)
	assert.NotNil(t, cmd)
}

func TestFinalTriggerCronJob(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "CronJob"
	cmd := m.triggerCronJob()
	assert.NotNil(t, cmd)
}

// =====================================================================
// Spin up the Update handler with more msgs for coverage
// =====================================================================

func TestFinalUpdateDescribeLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeDescribe
	result, _ := m.Update(describeLoadedMsg{content: "desc content", title: "Title"})
	rm := result.(Model)
	assert.Equal(t, "desc content", rm.describeContent)
	assert.Equal(t, "Title", rm.describeTitle)
}

func TestFinalUpdateDescribeLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeDescribe
	result, cmd := m.Update(describeLoadedMsg{err: assert.AnError, title: "Title"})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinalUpdateLogLineMsg(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	ch := make(chan string, 1)
	m.logCh = ch
	m.logFollow = true
	result, cmd := m.Update(logLineMsg{line: "log line", ch: ch})
	rm := result.(Model)
	assert.Contains(t, rm.logLines, "log line")
	assert.NotNil(t, cmd)
}

func TestFinalUpdateLogLineMsgDone(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	ch := make(chan string)
	m.logCh = ch
	result, _ := m.Update(logLineMsg{done: true, ch: ch})
	_ = result.(Model)
}

func TestFinalUpdateLogLineMsgStaleChannel(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	ch1 := make(chan string, 1)
	ch2 := make(chan string, 1)
	m.logCh = ch2 // current channel
	result, _ := m.Update(logLineMsg{line: "stale", ch: ch1})
	rm := result.(Model)
	// Should discard the stale line.
	assert.NotContains(t, rm.logLines, "stale")
}

func TestFinalUpdateTriggerCronJobMsg(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, cmd := m.Update(triggerCronJobMsg{jobName: "manual-trigger-1"})
	rm := result.(Model)
	assert.False(t, rm.loading)
	assert.NotNil(t, cmd)
}

func TestFinalUpdateTriggerCronJobMsgError(t *testing.T) {
	m := baseFinalModel()
	m.loading = true
	result, _ := m.Update(triggerCronJobMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.loading)
}

func TestFinalUpdateRollbackDoneMsg(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(rollbackDoneMsg{})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "Rollback successful")
	assert.NotNil(t, cmd)
}

func TestFinalUpdateRollbackDoneMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(rollbackDoneMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestFinalUpdateLogHistoryMsg(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	m.logLoadingHistory = true
	m.logLines = []string{"existing"}
	result, _ := m.Update(logHistoryMsg{
		lines:     []string{"older1", "older2", "existing"},
		prevTotal: 1,
	})
	rm := result.(Model)
	assert.False(t, rm.logLoadingHistory)
}

func TestFinalUpdateLogHistoryMsgError(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	m.logLoadingHistory = true
	result, _ := m.Update(logHistoryMsg{err: assert.AnError})
	rm := result.(Model)
	assert.False(t, rm.logLoadingHistory)
}

func TestFinalUpdateDiffLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	result, _ := m.Update(diffLoadedMsg{left: "left", right: "right", leftName: "L", rightName: "R"})
	rm := result.(Model)
	assert.Equal(t, modeDiff, rm.mode)
}

func TestFinalUpdateDiffLoadedMsgError(t *testing.T) {
	m := baseFinalModel()
	result, _ := m.Update(diffLoadedMsg{err: assert.AnError})
	rm := result.(Model)
	_ = rm
}

func TestFinalUpdateExplainLoadedMsg(t *testing.T) {
	m := baseFinalModel()
	result, _ := m.Update(explainLoadedMsg{
		title:       "pods",
		description: "A pod is...",
		fields:      []model.ExplainField{{Name: "metadata", Type: "Object"}},
	})
	rm := result.(Model)
	assert.Equal(t, modeExplain, rm.mode)
}

// =====================================================================
// Additional tests for uncovered view/helper functions
// =====================================================================

func TestFinalExecuteActionHelmValues(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "HelmRelease"
	m.actionCtx.resourceType = model.ResourceTypeEntry{APIGroup: "_helm"}
	result, cmd := m.executeAction("Values")
	// Values opens helm edit values.
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionHelmDiff(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "HelmRelease"
	m.actionCtx.resourceType = model.ResourceTypeEntry{APIGroup: "_helm"}
	result, cmd := m.executeAction("Diff Values")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionHelmUpgrade(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "HelmRelease"
	m.actionCtx.resourceType = model.ResourceTypeEntry{APIGroup: "_helm"}
	result, cmd := m.executeAction("Upgrade")
	_ = result.(Model)
	_ = cmd
}

// =====================================================================
// Bookmark navigate -- enter key on overlay
// =====================================================================

func TestFinalHandleBookmarkEnterNoItems(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = nil
	result, cmd := m.handleBookmarkOverlayKey(keyMsg("enter"))
	assert.Nil(t, cmd)
	_ = result.(Model)
}

// =====================================================================
// Bookmark overlay slot jump
// =====================================================================

func TestFinalHandleBookmarkSlotJump(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = []model.Bookmark{{Name: "bm", Slot: "x"}}
	m.discoveredCRDs["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true},
	}
	// Pressing a slot key that doesn't exist should show error.
	result, _ := m.handleBookmarkOverlayKey(keyMsg("z"))
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "not set")
}

// =====================================================================
// Additional view coverage -- log save msg
// =====================================================================

func TestFinalUpdateLogSaveAllMsg(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	result, _ := m.Update(logSaveAllMsg{path: "/tmp/logs.log"})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "/tmp/logs.log")
}

func TestFinalUpdateLogSaveAllMsgError(t *testing.T) {
	m := baseFinalModel()
	m.mode = modeLogs
	result, _ := m.Update(logSaveAllMsg{err: assert.AnError})
	rm := result.(Model)
	_ = rm
}

// =====================================================================
// Update: HelmRollbackDoneMsg
// =====================================================================

func TestFinalUpdateHelmRollbackDoneMsg(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(helmRollbackDoneMsg{})
	rm := result.(Model)
	assert.Contains(t, rm.statusMessage, "rollback successful")
	assert.NotNil(t, cmd)
}

func TestFinalUpdateHelmRollbackDoneMsgError(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.Update(helmRollbackDoneMsg{err: assert.AnError})
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

// =====================================================================
// Additional small-but-impactful tests
// =====================================================================

func TestFinalExecuteActionVulnScan(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.columns = []model.KeyValue{{Key: "Image", Value: "nginx:latest"}}
	result, cmd := m.executeAction("Vulnerability Scan")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionLabels(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Labels")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionDebugPVC(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.kind = "PersistentVolumeClaim"
	result, cmd := m.executeAction("Debug Pod (PVC)")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionResourceQuota(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Resource Quotas")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionRBACCheck(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("RBAC Check")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionPodStartup(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Pod Startup")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionNetworkPolicies(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Network Policies")
	_ = result.(Model)
	_ = cmd
}

func TestFinalExecuteActionExplain(t *testing.T) {
	m := baseFinalModel()
	result, cmd := m.executeAction("Explain")
	_ = result.(Model)
	_ = cmd
}

// Test the removeBookmark helper used by bookmark delete.
func TestFinalRemoveBookmark(t *testing.T) {
	bms := []model.Bookmark{
		{Slot: "a", Name: "first"},
		{Slot: "b", Name: "second"},
		{Slot: "c", Name: "third"},
	}
	result := removeBookmark(bms, 1)
	assert.Len(t, result, 2)
	assert.Equal(t, "a", result[0].Slot)
	assert.Equal(t, "c", result[1].Slot)
}

// =====================================================================
// Additional command coverage: resolveHelmChartName helper is pure parse
// =====================================================================

// helmShowDefaultValues with empty chart name returns early.
func TestFinalHelmShowDefaultValuesEmpty(t *testing.T) {
	out, label := helmShowDefaultValues("", "", nil)
	assert.Empty(t, out)
	assert.Empty(t, label)
}
