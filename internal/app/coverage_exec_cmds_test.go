package app

import (
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// testModelExec creates a Model with a fake client for exec command tests.
func testModelExec() Model {
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
// execKubectlDescribe -- returns a closure Cmd (not ExecProcess), testable
// =====================================================================

func TestCovExecKubectlDescribeReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.execKubectlDescribe()
	assert.NotNil(t, cmd)
}

func TestCovExecKubectlDescribeNonNamespaced(t *testing.T) {
	m := testModelExec()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Node", Resource: "nodes", Namespaced: false}
	m.actionCtx.name = "node-1"
	cmd := m.execKubectlDescribe()
	assert.NotNil(t, cmd)
}

// =====================================================================
// execKubectlEdit
// =====================================================================

func TestCovExecKubectlEditReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.execKubectlEdit()
	assert.NotNil(t, cmd)
}

func TestCovExecKubectlEditNonNamespaced(t *testing.T) {
	m := testModelExec()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Node", Resource: "nodes", Namespaced: false}
	cmd := m.execKubectlEdit()
	assert.NotNil(t, cmd)
}

// =====================================================================
// execKubectlExec
// =====================================================================

func TestCovExecKubectlExecReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.execKubectlExec()
	assert.NotNil(t, cmd)
}

func TestCovExecKubectlExecWithContainer(t *testing.T) {
	m := testModelExec()
	m.actionCtx.containerName = "main"
	cmd := m.execKubectlExec()
	assert.NotNil(t, cmd)
}

// =====================================================================
// execKubectlAttach
// =====================================================================

func TestCovExecKubectlAttachReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.execKubectlAttach()
	assert.NotNil(t, cmd)
}

func TestCovExecKubectlAttachWithContainer(t *testing.T) {
	m := testModelExec()
	m.actionCtx.containerName = "sidecar"
	cmd := m.execKubectlAttach()
	assert.NotNil(t, cmd)
}

// =====================================================================
// execKubectlDebug
// =====================================================================

func TestCovExecKubectlDebugReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.execKubectlDebug()
	assert.NotNil(t, cmd)
}

// =====================================================================
// runDebugPod
// =====================================================================

func TestCovRunDebugPodReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.runDebugPod()
	assert.NotNil(t, cmd)
}

// =====================================================================
// runDebugPodWithPVC
// =====================================================================

func TestCovRunDebugPodWithPVCReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-pvc"
	cmd := m.runDebugPodWithPVC()
	assert.NotNil(t, cmd)
}

// =====================================================================
// deleteResource -- calls client.DeleteResource
// =====================================================================

func TestCovDeleteResourceReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.deleteResource()
	assert.NotNil(t, cmd)
}

func TestCovDeleteResourceHelmRelease(t *testing.T) {
	m := testModelExec()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "HelmRelease", APIGroup: "_helm", Resource: "helmreleases"}
	cmd := m.deleteResource()
	// This dispatches to uninstallHelmRelease which returns a cmd.
	assert.NotNil(t, cmd)
}

// =====================================================================
// forceDeleteResource
// =====================================================================

func TestCovForceDeleteResourceReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.forceDeleteResource()
	assert.NotNil(t, cmd)
}

func TestCovForceDeleteNonNamespaced(t *testing.T) {
	m := testModelExec()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Node", Resource: "nodes", Namespaced: false}
	cmd := m.forceDeleteResource()
	assert.NotNil(t, cmd)
}

// =====================================================================
// removeFinalizers
// =====================================================================

func TestCovRemoveFinalizersReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.removeFinalizers()
	assert.NotNil(t, cmd)
}

func TestCovRemoveFinalizersNonNamespaced(t *testing.T) {
	m := testModelExec()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "PersistentVolume", Resource: "persistentvolumes", Namespaced: false}
	cmd := m.removeFinalizers()
	assert.NotNil(t, cmd)
}

// =====================================================================
// uninstallHelmRelease
// =====================================================================

func TestCovUninstallHelmReleaseReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-release"
	cmd := m.uninstallHelmRelease()
	assert.NotNil(t, cmd)
}

// =====================================================================
// editHelmValues
// =====================================================================

func TestCovEditHelmValuesReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-release"
	cmd := m.editHelmValues()
	assert.NotNil(t, cmd)
}

// =====================================================================
// helmDiff -- returns a closure
// =====================================================================

func TestCovHelmDiffReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-release"
	cmd := m.helmDiff()
	assert.NotNil(t, cmd)
}

// =====================================================================
// helmUpgrade
// =====================================================================

func TestCovHelmUpgradeReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-release"
	cmd := m.helmUpgrade()
	assert.NotNil(t, cmd)
}

// =====================================================================
// resizePVC
// =====================================================================

func TestCovResizePVCReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-pvc"
	cmd := m.resizePVC("10Gi")
	assert.NotNil(t, cmd)
}

// =====================================================================
// scaleResource
// =====================================================================

func TestCovScaleResourceReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	m.actionCtx.name = "my-deploy"
	cmd := m.scaleResource(3)
	assert.NotNil(t, cmd)
}

// =====================================================================
// restartResource
// =====================================================================

func TestCovRestartResourceReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	cmd := m.restartResource()
	assert.NotNil(t, cmd)
}

// =====================================================================
// rollbackDeployment
// =====================================================================

func TestCovRollbackDeploymentReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-deploy"
	cmd := m.rollbackDeployment(2)
	assert.NotNil(t, cmd)
}

// =====================================================================
// rollbackHelmRelease
// =====================================================================

func TestCovRollbackHelmReleaseReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-release"
	cmd := m.rollbackHelmRelease(3)
	assert.NotNil(t, cmd)
}

// =====================================================================
// execKubectlCordon / execKubectlUncordon
// =====================================================================

func TestCovExecKubectlCordonReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	cmd := m.execKubectlCordon()
	assert.NotNil(t, cmd)
}

func TestCovExecKubectlUncordonReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	cmd := m.execKubectlUncordon()
	assert.NotNil(t, cmd)
}

// =====================================================================
// execKubectlDrain
// =====================================================================

func TestCovExecKubectlDrainReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	cmd := m.execKubectlDrain()
	assert.NotNil(t, cmd)
}

// =====================================================================
// execKubectlNodeCmd (internal helper, tested via cordon/uncordon)
// =====================================================================

func TestCovExecKubectlNodeCmdReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	cmd := m.execKubectlNodeCmd("cordon")
	assert.NotNil(t, cmd)
}

// =====================================================================
// execKubectlNodeShell
// =====================================================================

func TestCovExecKubectlNodeShellReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	cmd := m.execKubectlNodeShell()
	assert.NotNil(t, cmd)
}

// =====================================================================
// triggerCronJob
// =====================================================================

func TestCovTriggerCronJobReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-cronjob"
	cmd := m.triggerCronJob()
	assert.NotNil(t, cmd)
}

// =====================================================================
// execKubectlExplain
// =====================================================================

func TestCovExecKubectlExplainReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.execKubectlExplain("pods", "", "")
	assert.NotNil(t, cmd)
}

func TestCovExecKubectlExplainWithAPIVersion(t *testing.T) {
	m := testModelExec()
	cmd := m.execKubectlExplain("deployments", "apps/v1", "")
	assert.NotNil(t, cmd)
}

func TestCovExecKubectlExplainWithFieldPath(t *testing.T) {
	m := testModelExec()
	cmd := m.execKubectlExplain("pods", "", "spec.containers")
	assert.NotNil(t, cmd)
}

func TestCovExecKubectlExplainWithBoth(t *testing.T) {
	m := testModelExec()
	cmd := m.execKubectlExplain("deployments", "apps/v1", "spec.template.spec")
	assert.NotNil(t, cmd)
}

// =====================================================================
// execKubectlExplainRecursive
// =====================================================================

func TestCovExecKubectlExplainRecursiveReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.execKubectlExplainRecursive("pods", "", "containers")
	assert.NotNil(t, cmd)
}

func TestCovExecKubectlExplainRecursiveWithAPIVersion(t *testing.T) {
	m := testModelExec()
	cmd := m.execKubectlExplainRecursive("deployments", "apps/v1", "spec")
	assert.NotNil(t, cmd)
}

// =====================================================================
// execCustomAction
// =====================================================================

func TestCovExecCustomActionReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.execCustomAction("echo hello")
	assert.NotNil(t, cmd)
}

// =====================================================================
// vulnScanImage
// =====================================================================

func TestCovVulnScanImageReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.vulnScanImage("nginx:latest")
	assert.NotNil(t, cmd)
}

// =====================================================================
// helmShowDefaultValues (package-level function)
// =====================================================================

func TestCovHelmShowDefaultValuesEmptyChart(t *testing.T) {
	out, label := helmShowDefaultValues("/usr/bin/helm", "", nil)
	assert.Empty(t, out)
	assert.Empty(t, label)
}

// =====================================================================
// resolveHelmChartName (package-level function)
// =====================================================================

func TestCovResolveHelmChartNameReturnsEmpty(t *testing.T) {
	// The command won't find helm with a bogus path.
	name := resolveHelmChartName("/nonexistent/helm", "release", "ns", "ctx", "/dev/null")
	assert.Empty(t, name)
}

// =====================================================================
// clearBeforeExec (package-level function)
// =====================================================================

func TestCovClearBeforeExec(t *testing.T) {
	// Wraps a command with a shell clear prefix.
	inner := exec.Command("echo", "hello")
	wrapped := clearBeforeExec(inner)
	require.NotNil(t, wrapped)
	assert.Contains(t, strings.Join(wrapped.Args, " "), "printf")
}

// =====================================================================
// executeAction -- test many branches in the big switch
// =====================================================================

func TestCovExecuteActionDescribe(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Describe")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	_ = rm
}

func TestCovExecuteActionDescribeNamespaced(t *testing.T) {
	m := testModelExec()
	m.actionCtx.resourceType.Namespaced = true
	result, cmd := m.executeAction("Describe")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionEdit(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Edit")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionDelete(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Delete")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayConfirm, rm.overlay)
	assert.Equal(t, "Delete", rm.pendingAction)
}

func TestCovExecuteActionScale(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Scale")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayScaleInput, rm.overlay)
}

func TestCovExecuteActionResize(t *testing.T) {
	m := testModelExec()
	m.actionCtx.columns = []model.KeyValue{{Key: "Capacity", Value: "5Gi"}}
	result, cmd := m.executeAction("Resize")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayPVCResize, rm.overlay)
	assert.Equal(t, "5Gi", rm.pvcCurrentSize)
}

func TestCovExecuteActionResizeNoCap(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Resize")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayPVCResize, rm.overlay)
	assert.Empty(t, rm.pvcCurrentSize)
}

func TestCovExecuteActionRestart(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Restart")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionRestartPortForward(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "__port_forwards__"
	m.actionCtx.columns = []model.KeyValue{{Key: "ID", Value: "0"}}
	result, cmd := m.executeAction("Restart")
	_ = result
	assert.Nil(t, cmd)
}

func TestCovExecuteActionDebug(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Debug")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionDebugPod(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Debug Pod")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionDebugMount(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-pvc"
	result, cmd := m.executeAction("Debug Mount")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionForceDelete(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Force Delete")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionForceFinalize(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Force Finalize")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayConfirmType, rm.overlay)
	assert.Equal(t, "Force Finalize", rm.pendingAction)
}

func TestCovExecuteActionCordon(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	result, cmd := m.executeAction("Cordon")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionUncordon(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	result, cmd := m.executeAction("Uncordon")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionDrain(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	result, cmd := m.executeAction("Drain")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayConfirm, rm.overlay)
}

func TestCovExecuteActionTaint(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	result, cmd := m.executeAction("Taint")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.True(t, rm.commandBarActive)
}

func TestCovExecuteActionUntaint(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	m.actionCtx.columns = []model.KeyValue{{Key: "Taints", Value: "key1=val:NoSchedule, key2:NoExecute"}}
	result, cmd := m.executeAction("Untaint")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.True(t, rm.commandBarActive)
}

func TestCovExecuteActionUntaintEmpty(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	result, cmd := m.executeAction("Untaint")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.True(t, rm.commandBarActive)
}

func TestCovExecuteActionTrigger(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "CronJob"
	result, cmd := m.executeAction("Trigger")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionShell(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	result, cmd := m.executeAction("Shell")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionPortForward(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	result, cmd := m.executeAction("Port Forward")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionEvents(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Events")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionSecretEditor(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Secret"
	m.middleItems = []model.Item{{Name: "my-secret", Namespace: "default"}}
	result, cmd := m.executeAction("Secret Editor")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionConfigMapEditor(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "ConfigMap"
	m.middleItems = []model.Item{{Name: "my-cm", Namespace: "default"}}
	result, cmd := m.executeAction("ConfigMap Editor")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionRollbackDeployment(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	m.middleItems = []model.Item{{Name: "my-deploy", Namespace: "default"}}
	result, cmd := m.executeAction("Rollback")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionRollbackHelmRelease(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "HelmRelease"
	result, cmd := m.executeAction("Rollback")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionVulnScan(t *testing.T) {
	m := testModelExec()
	m.actionCtx.image = "nginx:latest"
	result, cmd := m.executeAction("Vuln Scan")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionVulnScanNoImage(t *testing.T) {
	m := testModelExec()
	m.actionCtx.image = ""
	result, cmd := m.executeAction("Vuln Scan")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.hasStatusMessage())
	_ = rm
}

func TestCovExecuteActionPermissions(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Permissions")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionStartupAnalysis(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Startup Analysis")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionAlerts(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Alerts")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionVisualize(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Visualize")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionLabelsAnnotations(t *testing.T) {
	m := testModelExec()
	m.middleItems = []model.Item{{Name: "my-pod", Namespace: "default"}}
	result, cmd := m.executeAction("Labels / Annotations")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionValues(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "HelmRelease"
	result, cmd := m.executeAction("Values")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionAllValues(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "HelmRelease"
	result, cmd := m.executeAction("All Values")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionEditValues(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "HelmRelease"
	result, cmd := m.executeAction("Edit Values")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionDiffHelm(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "HelmRelease"
	result, cmd := m.executeAction("Diff")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionDiffNonHelm(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Diff")
	assert.Nil(t, cmd)
	_ = result
}

func TestCovExecuteActionUpgrade(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "HelmRelease"
	result, cmd := m.executeAction("Upgrade")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionLogsDirectPod(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	m.actionCtx.containerName = "main"
	result, cmd := m.executeAction("Logs")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, modeLogs, rm.mode)
}

func TestCovExecuteActionLogsGroupResource(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Logs")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, modeLogs, rm.mode)
}

func TestCovExecuteActionLogsNonPodNonGroup(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "CRD"
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Logs")
	rm := result.(Model)
	// Should need container selection.
	assert.NotNil(t, cmd)
	assert.Equal(t, "Logs", rm.pendingAction)
}

func TestCovExecuteActionLogsPodAllContainers(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Logs")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, modeLogs, rm.mode)
	assert.Contains(t, rm.logTitle, "Logs:")
}

func TestCovExecuteActionExecDirectPod(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	m.actionCtx.containerName = "main"
	result, cmd := m.executeAction("Exec")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionExecParent(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	result, cmd := m.executeAction("Exec")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, "Exec", rm.pendingAction)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionExecNeedsContainer(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Exec")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, "Exec", rm.pendingAction)
}

func TestCovExecuteActionAttachDirect(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	m.actionCtx.containerName = "main"
	result, cmd := m.executeAction("Attach")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionAttachParent(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "StatefulSet"
	result, cmd := m.executeAction("Attach")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, "Attach", rm.pendingAction)
}

func TestCovExecuteActionAttachNeedsContainer(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Pod"
	m.actionCtx.containerName = ""
	result, cmd := m.executeAction("Attach")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, "Attach", rm.pendingAction)
}

func TestCovExecuteActionStopPortForward(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "__port_forwards__"
	m.actionCtx.columns = []model.KeyValue{{Key: "ID", Value: "0"}}
	result, cmd := m.executeAction("Stop")
	_ = result
	assert.Nil(t, cmd)
}

func TestCovExecuteActionRemovePortForward(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "__port_forwards__"
	m.actionCtx.columns = []model.KeyValue{{Key: "ID", Value: "0"}}
	result, cmd := m.executeAction("Remove")
	_ = result
	assert.Nil(t, cmd)
}

func TestCovExecuteActionUnknown(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("NonExistentAction12345")
	assert.Nil(t, cmd)
	_ = result
}

func TestCovExecuteActionGoToPodSingle(t *testing.T) {
	m := testModelExec()
	m.actionCtx.columns = []model.KeyValue{{Key: "Used By", Value: "single-pod"}}
	m.middleItems = []model.Item{{Name: "item1"}}
	result, _ := m.executeAction("Go to Pod")
	_ = result
}

func TestCovExecuteActionGoToPodMultiple(t *testing.T) {
	m := testModelExec()
	m.actionCtx.columns = []model.KeyValue{{Key: "Used By", Value: "pod-a, pod-b"}}
	result, cmd := m.executeAction("Go to Pod")
	rm := result.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlayPodSelect, rm.overlay)
}

func TestCovExecuteActionGoToPodNone(t *testing.T) {
	m := testModelExec()
	m.actionCtx.columns = nil
	result, cmd := m.executeAction("Go to Pod")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.hasStatusMessage())
}

// =====================================================================
// executeAction -- Argo CD and other integrations
// =====================================================================

func TestCovExecuteActionConfigureAutoSync(t *testing.T) {
	m := testModelExec()
	m.middleItems = []model.Item{{Name: "my-app", Namespace: "argocd"}}
	result, cmd := m.executeAction("Configure AutoSync")
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovExecuteActionSync(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Sync")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionSyncApplyOnly(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Sync (Apply Only)")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionRefreshApp(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Application"
	result, cmd := m.executeAction("Refresh")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionRefreshAppSet(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "ApplicationSet"
	result, cmd := m.executeAction("Refresh")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionRefreshAppSetByResource(t *testing.T) {
	m := testModelExec()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Resource: "applicationsets"}
	result, cmd := m.executeAction("Refresh")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionTerminateSync(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Terminate Sync")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionWatchWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Watch Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
	assert.True(t, rm.describeAutoRefresh)
}

func TestCovExecuteActionSuspendWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Suspend Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionResumeWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Resume Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionStopWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Stop Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionTerminateWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Terminate Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionResubmitWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Resubmit Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionSubmitWorkflow(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "WorkflowTemplate"
	result, cmd := m.executeAction("Submit Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionSubmitClusterWorkflow(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "ClusterWorkflowTemplate"
	result, cmd := m.executeAction("Submit Workflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionSuspendCronWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Suspend CronWorkflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionResumeCronWorkflow(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Resume CronWorkflow")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionForceRenew(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Force Renew")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionForceRefresh(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Force Refresh")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionPause(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Pause")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionUnpause(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Unpause")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionReconcile(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Reconcile")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionSuspend(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Suspend")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionResume(t *testing.T) {
	m := testModelExec()
	result, cmd := m.executeAction("Resume")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.loading)
}

func TestCovExecuteActionOpenInBrowserPF(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "__port_forwards__"
	m.actionCtx.columns = []model.KeyValue{{Key: "Local", Value: "8080"}}
	result, cmd := m.executeAction("Open in Browser")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.hasStatusMessage())
}

func TestCovExecuteActionOpenInBrowserPFNoPort(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "__port_forwards__"
	result, cmd := m.executeAction("Open in Browser")
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.True(t, rm.hasStatusMessage())
}

func TestCovExecuteActionOpenInBrowserIngress(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Ingress"
	m.middleItems = []model.Item{{Name: "my-ingress", Columns: []model.KeyValue{{Key: "__ingress_url", Value: "https://example.com"}}}}
	result, cmd := m.executeAction("Open in Browser")
	_ = result
	_ = cmd
}

// =====================================================================
// executeAction -- bulk mode
// =====================================================================

func TestCovExecuteActionBulkMode(t *testing.T) {
	m := testModelExec()
	m.bulkMode = true
	m.bulkItems = []model.Item{{Name: "pod-1", Namespace: "default"}}
	m.actionCtx.kind = "Pod"
	// Should dispatch to executeBulkAction.
	result, cmd := m.executeAction("Delete")
	_ = result
	_ = cmd
}
