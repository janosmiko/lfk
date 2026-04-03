package app

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- randomSuffix ---

func TestRandomSuffix(t *testing.T) {
	t.Run("returns correct length", func(t *testing.T) {
		s := randomSuffix(5)
		assert.Len(t, s, 5)
	})

	t.Run("zero length returns empty", func(t *testing.T) {
		s := randomSuffix(0)
		assert.Empty(t, s)
	})

	t.Run("contains only valid characters", func(t *testing.T) {
		const validChars = "abcdefghijklmnopqrstuvwxyz0123456789"
		s := randomSuffix(100)
		for _, c := range s {
			assert.Contains(t, validChars, string(c))
		}
	})

	t.Run("different calls produce different results", func(t *testing.T) {
		// With 36^10 possible values, collision probability is negligible.
		s1 := randomSuffix(10)
		s2 := randomSuffix(10)
		assert.NotEqual(t, s1, s2)
	})
}

func TestPush3ExecKubectlNodeCmdNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{name: "node-1", context: "test-ctx"}
	cmd := m.execKubectlNodeCmd("cordon")
	require.NotNil(t, cmd)
	msg := cmd()
	amsg, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.Error(t, amsg.err)
	assert.Contains(t, amsg.err.Error(), "kubectl not found")
}

func TestPush3ExecKubectlExplainNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	cmd := m.execKubectlExplain("pods", "v1", "")
	require.NotNil(t, cmd)
	msg := cmd()
	emsg, ok := msg.(explainLoadedMsg)
	require.True(t, ok)
	assert.Error(t, emsg.err)
	assert.Contains(t, emsg.err.Error(), "kubectl not found")
}

func TestPush3ExecKubectlExplainRecursiveNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	cmd := m.execKubectlExplainRecursive("pods", "v1", "")
	require.NotNil(t, cmd)
	msg := cmd()
	emsg, ok := msg.(explainRecursiveMsg)
	require.True(t, ok)
	assert.Error(t, emsg.err)
}

func TestPush3RollbackHelmReleaseNoHelm(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{name: "release-1", namespace: "default", context: "test-ctx"}
	cmd := m.rollbackHelmRelease(1)
	require.NotNil(t, cmd)
	msg := cmd()
	hmsg, ok := msg.(helmRollbackDoneMsg)
	require.True(t, ok)
	assert.Error(t, hmsg.err)
}

func TestPush3HelmDiffNoHelm(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{name: "release-1", namespace: "default", context: "test-ctx"}
	cmd := m.helmDiff()
	require.NotNil(t, cmd)
	msg := cmd()
	dmsg, ok := msg.(diffLoadedMsg)
	require.True(t, ok)
	assert.Error(t, dmsg.err)
}

func TestPush3ExecKubectlDrainNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{name: "node-1", context: "test-ctx"}
	cmd := m.execKubectlDrain()
	require.NotNil(t, cmd)
	msg := cmd()
	amsg, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.Error(t, amsg.err)
}

func TestPush3ExecKubectlNodeShellNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{name: "node-1", context: "test-ctx"}
	cmd := m.execKubectlNodeShell()
	require.NotNil(t, cmd)
	// This returns a tea.ExecProcess or error cmd.
}

func TestPush3ExecKubectlDescribeNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{
		name:         "pod-1",
		namespace:    "default",
		context:      "test-ctx",
		resourceType: model.ResourceTypeEntry{Resource: "pods", Kind: "Pod", Namespaced: true},
	}
	cmd := m.execKubectlDescribe()
	require.NotNil(t, cmd)
	msg := cmd()
	amsg, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.Error(t, amsg.err)
}

func TestPush3ExecKubectlEditNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{
		name:         "pod-1",
		namespace:    "default",
		context:      "test-ctx",
		resourceType: model.ResourceTypeEntry{Resource: "pods", Kind: "Pod", Namespaced: true},
	}
	cmd := m.execKubectlEdit()
	require.NotNil(t, cmd)
	msg := cmd()
	amsg, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.Error(t, amsg.err)
}

func TestPush3DeleteResourceNoClient(t *testing.T) {
	m := basePush80v3Model()
	m.actionCtx = actionContext{
		name:         "pod-1",
		namespace:    "default",
		context:      "test-ctx",
		resourceType: model.ResourceTypeEntry{Resource: "pods", Kind: "Pod", Namespaced: true},
	}
	cmd := m.deleteResource()
	require.NotNil(t, cmd)
	// Execute -- should try to delete via fake client.
	msg := cmd()
	amsg, ok := msg.(actionResultMsg)
	require.True(t, ok)
	// Fake client will fail since the pod doesn't exist.
	assert.Error(t, amsg.err)
}

func TestPush3ForceDeleteResourceNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{
		name:         "pod-1",
		namespace:    "default",
		context:      "test-ctx",
		resourceType: model.ResourceTypeEntry{Resource: "pods", Kind: "Pod", Namespaced: true},
	}
	cmd := m.forceDeleteResource()
	require.NotNil(t, cmd)
	msg := cmd()
	amsg, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.Error(t, amsg.err)
}

func TestPush3RemoveFinalizersNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{
		name:         "pod-1",
		namespace:    "default",
		context:      "test-ctx",
		resourceType: model.ResourceTypeEntry{Resource: "pods", Kind: "Pod", Namespaced: true},
	}
	cmd := m.removeFinalizers()
	require.NotNil(t, cmd)
	msg := cmd()
	amsg, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.Error(t, amsg.err)
}

func TestPush3UninstallHelmReleaseNoHelm(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{name: "release-1", namespace: "default", context: "test-ctx"}
	cmd := m.uninstallHelmRelease()
	require.NotNil(t, cmd)
	msg := cmd()
	amsg, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.Error(t, amsg.err)
}

func TestPush3EditHelmValuesNoHelm(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{name: "release-1", namespace: "default", context: "test-ctx"}
	cmd := m.editHelmValues()
	require.NotNil(t, cmd)
	msg := cmd()
	amsg, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.Error(t, amsg.err)
}

func TestPush3HelmUpgradeNoHelm(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{name: "release-1", namespace: "default", context: "test-ctx"}
	cmd := m.helmUpgrade()
	require.NotNil(t, cmd)
	msg := cmd()
	amsg, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.Error(t, amsg.err)
}

func TestPush3VulnScanImageNoTrivy(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{image: "nginx:latest"}
	cmd := m.vulnScanImage("nginx:latest")
	require.NotNil(t, cmd)
	msg := cmd()
	amsg, ok := msg.(describeLoadedMsg)
	require.True(t, ok)
	assert.Error(t, amsg.err)
}

func TestPush3ScaleResourceNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	m.actionCtx = actionContext{
		name:      "deploy-1",
		namespace: "default",
		context:   "test-ctx",
		kind:      "Deployment",
	}
	cmd := m.scaleResource(3)
	require.NotNil(t, cmd)
}

func TestPush3RestartResourceNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	m.actionCtx = actionContext{
		name:      "deploy-1",
		namespace: "default",
		context:   "test-ctx",
		kind:      "Deployment",
	}
	cmd := m.restartResource()
	require.NotNil(t, cmd)
}

func TestPush3ExecKubectlExecNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{
		name:          "pod-1",
		namespace:     "default",
		context:       "test-ctx",
		containerName: "app",
	}
	cmd := m.execKubectlExec()
	require.NotNil(t, cmd)
	// This returns an error action result.
	msg := cmd()
	amsg, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.Error(t, amsg.err)
}

func TestPush3ExecKubectlAttachNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{
		name:          "pod-1",
		namespace:     "default",
		context:       "test-ctx",
		containerName: "app",
	}
	cmd := m.execKubectlAttach()
	require.NotNil(t, cmd)
	msg := cmd()
	amsg, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.Error(t, amsg.err)
}

func TestPush3ExecKubectlDebugNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{
		name:          "pod-1",
		namespace:     "default",
		context:       "test-ctx",
		containerName: "app",
		image:         "nginx:latest",
	}
	cmd := m.execKubectlDebug()
	require.NotNil(t, cmd)
	msg := cmd()
	amsg, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.Error(t, amsg.err)
}

func TestPush3ExecCustomActionNoKubectl(t *testing.T) {
	m := basePush80v3Model()
	t.Setenv("PATH", "/nonexistent")
	m.actionCtx = actionContext{
		name:      "pod-1",
		namespace: "default",
		context:   "test-ctx",
	}
	ca := ui.CustomAction{
		Label:   "test",
		Command: "echo hello",
	}
	cmd := m.execCustomAction(ca.Command)
	require.NotNil(t, cmd)
}

func TestCovResizePVC(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-pvc", "default", "PersistentVolumeClaim", model.ResourceTypeEntry{})
	cmd := m.resizePVC("10Gi")
	msg := execCmd(t, cmd)
	result := msg.(actionResultMsg)
	// PVC doesn't exist in fake client.
	assert.Error(t, result.err)
}

func TestCovScaleResource(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-deploy", "default", "Deployment", model.ResourceTypeEntry{})
	cmd := m.scaleResource(3)
	msg := execCmd(t, cmd)
	result := msg.(actionResultMsg)
	// Deployment doesn't exist in fake client.
	assert.Error(t, result.err)
}

func TestCovRestartResource(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-deploy", "default", "Deployment", model.ResourceTypeEntry{})
	cmd := m.restartResource()
	msg := execCmd(t, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovRollbackDeployment(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-deploy", "default", "Deployment", model.ResourceTypeEntry{})
	cmd := m.rollbackDeployment(1)
	msg := execCmd(t, cmd)
	result, ok := msg.(rollbackDoneMsg)
	require.True(t, ok)
	assert.Error(t, result.err)
}

func TestCovTriggerCronJob(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-cj", "default", "CronJob", model.ResourceTypeEntry{})
	cmd := m.triggerCronJob()
	msg := execCmd(t, cmd)
	result, ok := msg.(triggerCronJobMsg)
	require.True(t, ok)
	assert.Error(t, result.err)
}

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

func TestCovExecKubectlDebugReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.execKubectlDebug()
	assert.NotNil(t, cmd)
}

func TestCovRunDebugPodReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.runDebugPod()
	assert.NotNil(t, cmd)
}

func TestCovRunDebugPodWithPVCReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-pvc"
	cmd := m.runDebugPodWithPVC()
	assert.NotNil(t, cmd)
}

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

func TestCovUninstallHelmReleaseReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-release"
	cmd := m.uninstallHelmRelease()
	assert.NotNil(t, cmd)
}

func TestCovEditHelmValuesReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-release"
	cmd := m.editHelmValues()
	assert.NotNil(t, cmd)
}

func TestCovHelmDiffReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-release"
	cmd := m.helmDiff()
	assert.NotNil(t, cmd)
}

func TestCovHelmUpgradeReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-release"
	cmd := m.helmUpgrade()
	assert.NotNil(t, cmd)
}

func TestCovResizePVCReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-pvc"
	cmd := m.resizePVC("10Gi")
	assert.NotNil(t, cmd)
}

func TestCovScaleResourceReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	m.actionCtx.name = "my-deploy"
	cmd := m.scaleResource(3)
	assert.NotNil(t, cmd)
}

func TestCovRestartResourceReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.kind = "Deployment"
	cmd := m.restartResource()
	assert.NotNil(t, cmd)
}

func TestCovRollbackDeploymentReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-deploy"
	cmd := m.rollbackDeployment(2)
	assert.NotNil(t, cmd)
}

func TestCovRollbackHelmReleaseReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-release"
	cmd := m.rollbackHelmRelease(3)
	assert.NotNil(t, cmd)
}

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

func TestCovExecKubectlDrainReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	cmd := m.execKubectlDrain()
	assert.NotNil(t, cmd)
}

func TestCovExecKubectlNodeCmdReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	cmd := m.execKubectlNodeCmd("cordon")
	assert.NotNil(t, cmd)
}

func TestCovExecKubectlNodeShellReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "node-1"
	cmd := m.execKubectlNodeShell()
	assert.NotNil(t, cmd)
}

func TestCovTriggerCronJobReturnsCmd(t *testing.T) {
	m := testModelExec()
	m.actionCtx.name = "my-cronjob"
	cmd := m.triggerCronJob()
	assert.NotNil(t, cmd)
}

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

func TestCovExecCustomActionReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.execCustomAction("echo hello")
	assert.NotNil(t, cmd)
}

func TestCovVulnScanImageReturnsCmd(t *testing.T) {
	m := testModelExec()
	cmd := m.vulnScanImage("nginx:latest")
	assert.NotNil(t, cmd)
}

func TestCovHelmShowDefaultValuesEmptyChart(t *testing.T) {
	out, label := helmShowDefaultValues("/usr/bin/helm", "", nil)
	assert.Empty(t, out)
	assert.Empty(t, label)
}

func TestCovResolveHelmChartNameReturnsEmpty(t *testing.T) {
	// The command won't find helm with a bogus path.
	name := resolveHelmChartName("/nonexistent/helm", "release", "ns", "ctx", "/dev/null")
	assert.Empty(t, name)
}

func TestCovClearBeforeExec(t *testing.T) {
	// Wraps a command with a shell clear prefix.
	inner := exec.Command("echo", "hello")
	wrapped := clearBeforeExec(inner)
	require.NotNil(t, wrapped)
	assert.Contains(t, strings.Join(wrapped.Args, " "), "printf")
}

func TestFinal2ForceDeleteResource(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	cmd := m.forceDeleteResource()
	assert.NotNil(t, cmd)
}

func TestFinal2ForceDeleteResourceNonNamespaced(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Node", Resource: "nodes", Namespaced: false}
	cmd := m.forceDeleteResource()
	assert.NotNil(t, cmd)
}

func TestFinal2RemoveFinalizers(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	cmd := m.removeFinalizers()
	assert.NotNil(t, cmd)
}

func TestFinal2RemoveFinalizersNonNamespaced(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.resourceType = model.ResourceTypeEntry{Kind: "Node", Resource: "nodes", Namespaced: false}
	cmd := m.removeFinalizers()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlNodeCmd(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.name = "node-1"
	m.actionCtx.context = "test-ctx"
	cmd := m.execKubectlNodeCmd("cordon")
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlDrain(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.name = "node-1"
	cmd := m.execKubectlDrain()
	assert.NotNil(t, cmd)
}

func TestFinal2RollbackHelmRelease(t *testing.T) {
	m := baseFinalModel()
	cmd := m.rollbackHelmRelease(1)
	assert.NotNil(t, cmd)
}

func TestFinal2VulnScanImage(t *testing.T) {
	m := baseFinalModel()
	cmd := m.vulnScanImage("nginx:latest")
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlExplain(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execKubectlExplain("pods", "", "")
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlExplainWithFieldPath(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execKubectlExplain("pods", "v1", "spec.containers")
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlExplainRecursive(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execKubectlExplainRecursive("pods", "", "container")
	assert.NotNil(t, cmd)
}

func TestFinal2ExecCustomAction(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execCustomAction("echo hello")
	assert.NotNil(t, cmd)
}

func TestFinal2HelmDiff(t *testing.T) {
	m := baseFinalModel()
	cmd := m.helmDiff()
	assert.NotNil(t, cmd)
}

func TestFinal2HelmUpgrade(t *testing.T) {
	m := baseFinalModel()
	cmd := m.helmUpgrade()
	assert.NotNil(t, cmd)
}

func TestFinal2EditHelmValues(t *testing.T) {
	m := baseFinalModel()
	cmd := m.editHelmValues()
	assert.NotNil(t, cmd)
}

func TestFinal2UninstallHelmRelease(t *testing.T) {
	m := baseFinalModel()
	cmd := m.uninstallHelmRelease()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlNodeShell(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.name = "node-1"
	cmd := m.execKubectlNodeShell()
	assert.NotNil(t, cmd)
}

func TestFinal2RunDebugPod(t *testing.T) {
	m := baseFinalModel()
	cmd := m.runDebugPod()
	assert.NotNil(t, cmd)
}

func TestFinal2RunDebugPodWithPVC(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.name = "my-pvc"
	cmd := m.runDebugPodWithPVC()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlExec(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execKubectlExec()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlExecWithContainer(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = "web"
	cmd := m.execKubectlExec()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlAttach(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execKubectlAttach()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlAttachWithContainer(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.containerName = "web"
	cmd := m.execKubectlAttach()
	assert.NotNil(t, cmd)
}

func TestFinal2ExecKubectlDebug(t *testing.T) {
	m := baseFinalModel()
	cmd := m.execKubectlDebug()
	assert.NotNil(t, cmd)
}

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

// helmShowDefaultValues with empty chart name returns early.
func TestFinalHelmShowDefaultValuesEmpty(t *testing.T) {
	out, label := helmShowDefaultValues("", "", nil)
	assert.Empty(t, out)
	assert.Empty(t, label)
}
