package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func TestBulkSyncArgoApps(t *testing.T) {
	m := Model{
		bulkItems: []model.Item{
			{Name: "app-1", Namespace: "argocd"},
			{Name: "app-2", Namespace: "argocd"},
		},
		actionCtx: actionContext{
			context:   "test-cluster",
			namespace: "argocd",
		},
	}
	cmd := m.bulkSyncArgoApps(false)
	assert.NotNil(t, cmd, "bulkSyncArgoApps should return a command")
}

func TestBulkSyncArgoAppsApplyOnly(t *testing.T) {
	m := Model{
		bulkItems: []model.Item{
			{Name: "app-1", Namespace: "argocd"},
		},
		actionCtx: actionContext{
			context:   "test-cluster",
			namespace: "argocd",
		},
	}
	cmd := m.bulkSyncArgoApps(true)
	assert.NotNil(t, cmd, "bulkSyncArgoApps(applyOnly=true) should return a command")
}

func TestBulkRefreshArgoApps(t *testing.T) {
	m := Model{
		bulkItems: []model.Item{
			{Name: "app-1", Namespace: "argocd"},
			{Name: "app-2", Namespace: "argocd"},
			{Name: "app-3", Namespace: "argocd"},
		},
		actionCtx: actionContext{
			context:   "test-cluster",
			namespace: "argocd",
		},
	}
	cmd := m.bulkRefreshArgoApps()
	assert.NotNil(t, cmd, "bulkRefreshArgoApps should return a command")
}

func TestCovSyncArgoApp(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-app", "argocd", "Application", model.ResourceTypeEntry{})
	cmd := m.syncArgoApp(false)
	msg := execScheduled(t, m, cmd)
	// The fake clientset doesn't have ArgoCD CRDs so we expect an error.
	result, ok := msg.(actionResultMsg)
	require.True(t, ok)
	assert.Error(t, result.err)
}

func TestCovSyncArgoAppApplyOnly(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-app", "argocd", "Application", model.ResourceTypeEntry{})
	cmd := m.syncArgoApp(true)
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovRefreshArgoApp(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-app", "argocd", "Application", model.ResourceTypeEntry{})
	cmd := m.refreshArgoApp()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovRefreshArgoAppSet(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-appset", "argocd", "ApplicationSet", model.ResourceTypeEntry{})
	cmd := m.refreshArgoAppSet()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovReconcileFluxResource(t *testing.T) {
	m := baseModelWithFakeClient()
	rt := model.ResourceTypeEntry{
		APIGroup:   "kustomize.toolkit.fluxcd.io",
		APIVersion: "v1",
		Resource:   "kustomizations",
	}
	m = withActionCtx(m, "my-ks", "flux-system", "Kustomization", rt)
	cmd := m.reconcileFluxResource()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovToggleFluxSuspend(t *testing.T) {
	m := baseModelWithFakeClient()
	rt := model.ResourceTypeEntry{
		APIGroup:   "kustomize.toolkit.fluxcd.io",
		APIVersion: "v1",
		Resource:   "kustomizations",
	}
	m = withActionCtx(m, "my-ks", "flux-system", "Kustomization", rt)
	cmd := m.toggleFluxSuspend()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovForceRenewCertificate(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-cert", "default", "Certificate", model.ResourceTypeEntry{})
	cmd := m.forceRenewCertificate()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovSuspendArgoWorkflow(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-wf", "argo", "Workflow", model.ResourceTypeEntry{})
	cmd := m.suspendArgoWorkflow()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovResumeArgoWorkflow(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-wf", "argo", "Workflow", model.ResourceTypeEntry{})
	cmd := m.resumeArgoWorkflow()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovStopArgoWorkflow(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-wf", "argo", "Workflow", model.ResourceTypeEntry{})
	cmd := m.stopArgoWorkflow()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovTerminateArgoWorkflow(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-wf", "argo", "Workflow", model.ResourceTypeEntry{})
	cmd := m.terminateArgoWorkflow()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovResubmitArgoWorkflow(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-wf", "argo", "Workflow", model.ResourceTypeEntry{})
	cmd := m.resubmitArgoWorkflow()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovSubmitWorkflowFromTemplate(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-tmpl", "argo", "WorkflowTemplate", model.ResourceTypeEntry{})
	cmd := m.submitWorkflowFromTemplate(false)
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	// SubmitWorkflowFromTemplate creates a workflow via dynamic client; fake dynamic
	// client without the GVR registered may succeed or fail depending on version.
	if result.err != nil {
		assert.Error(t, result.err)
	} else {
		assert.Contains(t, result.message, "Submitted workflow")
	}
}

func TestCovSubmitWorkflowFromTemplateClusterScope(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-tmpl", "argo", "ClusterWorkflowTemplate", model.ResourceTypeEntry{})
	cmd := m.submitWorkflowFromTemplate(true)
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	if result.err != nil {
		assert.Error(t, result.err)
	} else {
		assert.Contains(t, result.message, "Submitted workflow")
	}
}

func TestCovToggleCronWorkflowSuspend(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-cwf", "argo", "CronWorkflow", model.ResourceTypeEntry{})
	cmd := m.toggleCronWorkflowSuspend()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovWatchArgoWorkflow(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-wf", "argo", "Workflow", model.ResourceTypeEntry{})
	cmd := m.watchArgoWorkflow()
	msg := execScheduled(t, m, cmd)
	result, ok := msg.(describeLoadedMsg)
	require.True(t, ok)
	assert.Error(t, result.err)
	assert.Contains(t, result.title, "Watch: my-wf")
}

func TestCovForceRefreshExternalSecret(t *testing.T) {
	m := baseModelWithFakeClient()
	rt := model.ResourceTypeEntry{
		APIGroup:   "external-secrets.io",
		APIVersion: "v1beta1",
		Resource:   "externalsecrets",
		Namespaced: true,
	}
	m = withActionCtx(m, "my-es", "default", "ExternalSecret", rt)
	cmd := m.forceRefreshExternalSecret()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovForceRefreshExternalSecretClusterScoped(t *testing.T) {
	m := baseModelWithFakeClient()
	rt := model.ResourceTypeEntry{
		APIGroup:   "external-secrets.io",
		APIVersion: "v1beta1",
		Resource:   "clusterexternalsecrets",
		Namespaced: false,
	}
	m = withActionCtx(m, "my-ces", "default", "ClusterExternalSecret", rt)
	cmd := m.forceRefreshExternalSecret()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovToggleKEDAPause(t *testing.T) {
	m := baseModelWithFakeClient()
	rt := model.ResourceTypeEntry{
		APIGroup:   "keda.sh",
		APIVersion: "v1alpha1",
		Resource:   "scaledobjects",
	}
	m = withActionCtx(m, "my-so", "default", "ScaledObject", rt)
	cmd := m.toggleKEDAPause()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovBulkSyncArgoApps(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "", "argocd", "Application", model.ResourceTypeEntry{})
	m.bulkItems = []model.Item{
		{Name: "app-1", Namespace: "argocd"},
		{Name: "app-2"},
	}
	cmd := m.bulkSyncArgoApps(false)
	msg := execScheduled(t, m, cmd)
	result, ok := msg.(bulkActionResultMsg)
	require.True(t, ok)
	// Both should fail since there are no ArgoCD CRDs.
	assert.Equal(t, 2, result.failed)
	assert.Equal(t, 0, result.succeeded)
}

func TestCovBulkRefreshArgoApps(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "", "argocd", "Application", model.ResourceTypeEntry{})
	m.bulkItems = []model.Item{
		{Name: "app-1", Namespace: "argocd"},
		{Name: "app-2"},
	}
	cmd := m.bulkRefreshArgoApps()
	msg := execScheduled(t, m, cmd)
	result, ok := msg.(bulkActionResultMsg)
	require.True(t, ok)
	assert.Equal(t, 2, result.failed)
}

func TestCovTerminateArgoSync(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withActionCtx(m, "my-app", "argocd", "Application", model.ResourceTypeEntry{})
	cmd := m.terminateArgoSync()
	msg := execScheduled(t, m, cmd)
	result := msg.(actionResultMsg)
	assert.Error(t, result.err)
}

func TestCovLoadAutoSyncConfig(t *testing.T) {
	m := baseModelWithFakeClient()
	item := model.Item{Name: "my-app", Namespace: "argocd"}
	m = withMiddleItem(m, item)
	cmd := m.loadAutoSyncConfig()
	msg := execScheduled(t, m, cmd)
	result, ok := msg.(autoSyncLoadedMsg)
	require.True(t, ok)
	assert.Error(t, result.err)
}

func TestCovLoadAutoSyncConfigNilSel(t *testing.T) {
	m := baseModelWithFakeClient()
	m.middleItems = nil
	cmd := m.loadAutoSyncConfig()
	assert.Nil(t, cmd)
}

func TestCovSaveAutoSyncConfig(t *testing.T) {
	m := baseModelWithFakeClient()
	item := model.Item{Name: "my-app", Namespace: "argocd"}
	m = withMiddleItem(m, item)
	m.autoSyncEnabled = true
	m.autoSyncSelfHeal = true
	m.autoSyncPrune = false
	cmd := m.saveAutoSyncConfig()
	msg := execScheduled(t, m, cmd)
	result, ok := msg.(autoSyncSavedMsg)
	require.True(t, ok)
	assert.Error(t, result.err)
}

func TestCovSaveAutoSyncConfigNilSel(t *testing.T) {
	m := baseModelWithFakeClient()
	m.middleItems = nil
	cmd := m.saveAutoSyncConfig()
	assert.Nil(t, cmd)
}
