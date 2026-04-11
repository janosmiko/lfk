package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/model"
)

// newFakeDynClientWith creates a fake dynamic client with additional custom GVR registrations.
func newFakeDynClientWith(extraGVRs map[schema.GroupVersionResource]string, objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:                                      "PodList",
		{Group: "", Version: "v1", Resource: "namespaces"}:                                "NamespaceList",
		{Group: "", Version: "v1", Resource: "secrets"}:                                   "SecretList",
		{Group: "", Version: "v1", Resource: "configmaps"}:                                "ConfigMapList",
		{Group: "", Version: "v1", Resource: "services"}:                                  "ServiceList",
		{Group: "", Version: "v1", Resource: "events"}:                                    "EventList",
		{Group: "", Version: "v1", Resource: "resourcequotas"}:                            "ResourceQuotaList",
		{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}:                    "PersistentVolumeClaimList",
		{Group: "apps", Version: "v1", Resource: "deployments"}:                           "DeploymentList",
		{Group: "apps", Version: "v1", Resource: "replicasets"}:                           "ReplicaSetList",
		{Group: "apps", Version: "v1", Resource: "statefulsets"}:                          "StatefulSetList",
		{Group: "apps", Version: "v1", Resource: "daemonsets"}:                            "DaemonSetList",
		{Group: "batch", Version: "v1", Resource: "jobs"}:                                 "JobList",
		{Group: "batch", Version: "v1", Resource: "cronjobs"}:                             "CronJobList",
		{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}:          "NetworkPolicyList",
		{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}:             "ApplicationList",
		{Group: "argoproj.io", Version: "v1alpha1", Resource: "workflows"}:                "WorkflowList",
		{Group: "argoproj.io", Version: "v1alpha1", Resource: "cronworkflows"}:            "CronWorkflowList",
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"}: "KustomizationList",
		{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}:               "CertificateList",
	}
	for k, v := range extraGVRs {
		gvrs[k] = v
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, objects...)
}

// =====================================================================
// gitops.go: ArgoCD operations
// =====================================================================

func TestSyncArgoApp(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
			"spec": map[string]interface{}{
				"syncPolicy": map[string]interface{}{
					"syncOptions": []interface{}{"CreateNamespace=true"},
					"automated":   map[string]interface{}{"prune": true},
				},
			},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)

	err := c.SyncArgoApp("", "argocd", "my-app", false)
	require.NoError(t, err)
}

func TestSyncArgoApp_ApplyOnly(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
			"spec":       map[string]interface{}{},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)

	err := c.SyncArgoApp("", "argocd", "my-app", true)
	require.NoError(t, err)
}

func TestSyncArgoApp_OperationInProgress(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
			"operation":  map[string]interface{}{"sync": map[string]interface{}{}},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)

	err := c.SyncArgoApp("", "argocd", "my-app", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "another operation is already in progress")
}

func TestSyncArgoApp_ClearsStaleOperationState(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
			"spec":       map[string]interface{}{},
			"status": map[string]interface{}{
				"operationState": map[string]interface{}{
					"operation": map[string]interface{}{
						"sync": map[string]interface{}{
							"syncStrategy": map[string]interface{}{"hook": map[string]interface{}{}},
						},
					},
				},
			},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)

	err := c.SyncArgoApp("", "argocd", "my-app", false)
	require.NoError(t, err)
}

func TestTerminateArgoSync(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
			"status": map[string]interface{}{
				"operationState": map[string]interface{}{"phase": "Running"},
			},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)

	err := c.TerminateArgoSync("", "argocd", "my-app")
	require.NoError(t, err)
}

func TestTerminateArgoSync_NoOperation(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)

	err := c.TerminateArgoSync("", "argocd", "my-app")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no sync operation")
}

func TestTerminateArgoSync_WrongPhase(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
			"status": map[string]interface{}{
				"operationState": map[string]interface{}{"phase": "Succeeded"},
			},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)

	err := c.TerminateArgoSync("", "argocd", "my-app")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no running sync operation")
}

func TestRefreshArgoApp(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)

	err := c.RefreshArgoApp("", "argocd", "my-app")
	require.NoError(t, err)
}

func TestRefreshArgoAppSet(t *testing.T) {
	appset := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "ApplicationSet",
			"metadata":   map[string]interface{}{"name": "my-appset", "namespace": "argocd"},
		},
	}
	dc := newFakeDynClientWith(map[schema.GroupVersionResource]string{
		{Group: "argoproj.io", Version: "v1alpha1", Resource: "applicationsets"}: "ApplicationSetList",
	}, appset)
	c := newFakeClient(nil, dc)

	err := c.RefreshArgoAppSet("", "argocd", "my-appset")
	require.NoError(t, err)
}

func TestGetAutoSyncConfig_Enabled(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
			"spec": map[string]interface{}{
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{"selfHeal": true, "prune": true},
				},
			},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)

	enabled, selfHeal, prune, err := c.GetAutoSyncConfig(context.Background(), "", "argocd", "my-app")
	require.NoError(t, err)
	assert.True(t, enabled)
	assert.True(t, selfHeal)
	assert.True(t, prune)
}

func TestGetAutoSyncConfig_Disabled(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
			"spec":       map[string]interface{}{},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)

	enabled, _, _, err := c.GetAutoSyncConfig(context.Background(), "", "argocd", "my-app")
	require.NoError(t, err)
	assert.False(t, enabled)
}

func TestUpdateAutoSyncConfig_Enable(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
			"spec":       map[string]interface{}{},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)

	err := c.UpdateAutoSyncConfig(context.Background(), "", "argocd", "my-app", true, true, false)
	require.NoError(t, err)
}

func TestUpdateAutoSyncConfig_Disable(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
			"spec": map[string]interface{}{
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{"selfHeal": true},
				},
			},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)

	err := c.UpdateAutoSyncConfig(context.Background(), "", "argocd", "my-app", false, false, false)
	require.NoError(t, err)
}

// =====================================================================
// gitops.go: Flux operations
// =====================================================================

func TestReconcileFluxResource(t *testing.T) {
	ks := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata":   map[string]interface{}{"name": "my-ks", "namespace": "flux-system"},
		},
	}
	dc := newFakeDynClient(ks)
	c := newFakeClient(nil, dc)

	err := c.ReconcileFluxResource("", "flux-system", "my-ks",
		schema.GroupVersionResource{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"})
	require.NoError(t, err)
}

func TestSuspendFluxResource(t *testing.T) {
	ks := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata":   map[string]interface{}{"name": "my-ks", "namespace": "flux-system"},
		},
	}
	dc := newFakeDynClient(ks)
	c := newFakeClient(nil, dc)

	err := c.SuspendFluxResource("", "flux-system", "my-ks",
		schema.GroupVersionResource{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"})
	require.NoError(t, err)
}

func TestResumeFluxResource(t *testing.T) {
	ks := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata":   map[string]interface{}{"name": "my-ks", "namespace": "flux-system"},
		},
	}
	dc := newFakeDynClient(ks)
	c := newFakeClient(nil, dc)

	err := c.ResumeFluxResource("", "flux-system", "my-ks",
		schema.GroupVersionResource{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"})
	require.NoError(t, err)
}

func TestForceRenewCertificate(t *testing.T) {
	cert := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "Certificate",
			"metadata":   map[string]interface{}{"name": "my-cert", "namespace": "default"},
		},
	}
	dc := newFakeDynClient(cert)
	c := newFakeClient(nil, dc)

	err := c.ForceRenewCertificate("", "default", "my-cert")
	require.NoError(t, err)
}

func TestForceRefreshExternalSecret(t *testing.T) {
	es := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "external-secrets.io/v1beta1",
			"kind":       "ExternalSecret",
			"metadata":   map[string]interface{}{"name": "my-es", "namespace": "default"},
		},
	}
	gvr := schema.GroupVersionResource{Group: "external-secrets.io", Version: "v1beta1", Resource: "externalsecrets"}
	dc := newFakeDynClientWith(map[schema.GroupVersionResource]string{
		gvr: "ExternalSecretList",
	}, es)
	c := newFakeClient(nil, dc)

	err := c.ForceRefreshExternalSecret("", "default", "my-es", gvr)
	require.NoError(t, err)
}

// =====================================================================
// gitops.go: Argo Workflow operations
// =====================================================================

func TestSuspendArgoWorkflow(t *testing.T) {
	wf := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Workflow",
			"metadata":   map[string]interface{}{"name": "my-wf", "namespace": "default"},
		},
	}
	dc := newFakeDynClient(wf)
	c := newFakeClient(nil, dc)

	err := c.SuspendArgoWorkflow("", "default", "my-wf")
	require.NoError(t, err)
}

func TestResumeArgoWorkflow(t *testing.T) {
	wf := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Workflow",
			"metadata":   map[string]interface{}{"name": "my-wf", "namespace": "default"},
		},
	}
	dc := newFakeDynClient(wf)
	c := newFakeClient(nil, dc)

	err := c.ResumeArgoWorkflow("", "default", "my-wf")
	require.NoError(t, err)
}

func TestStopArgoWorkflow(t *testing.T) {
	wf := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Workflow",
			"metadata":   map[string]interface{}{"name": "my-wf", "namespace": "default"},
		},
	}
	dc := newFakeDynClient(wf)
	c := newFakeClient(nil, dc)

	err := c.StopArgoWorkflow("", "default", "my-wf")
	require.NoError(t, err)
}

func TestTerminateArgoWorkflow(t *testing.T) {
	wf := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Workflow",
			"metadata":   map[string]interface{}{"name": "my-wf", "namespace": "default"},
		},
	}
	dc := newFakeDynClient(wf)
	c := newFakeClient(nil, dc)

	err := c.TerminateArgoWorkflow("", "default", "my-wf")
	require.NoError(t, err)
}

func TestResubmitArgoWorkflow(t *testing.T) {
	wf := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Workflow",
			"metadata":   map[string]interface{}{"name": "my-wf", "namespace": "default"},
			"spec":       map[string]interface{}{"entrypoint": "main"},
		},
	}
	dc := newFakeDynClient(wf)
	c := newFakeClient(nil, dc)

	name, err := c.ResubmitArgoWorkflow("", "default", "my-wf")
	require.NoError(t, err)
	assert.Contains(t, name, "my-wf-resubmit-")
}

func TestSuspendCronWorkflow(t *testing.T) {
	cw := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "CronWorkflow",
			"metadata":   map[string]interface{}{"name": "my-cw", "namespace": "default"},
		},
	}
	dc := newFakeDynClient(cw)
	c := newFakeClient(nil, dc)

	err := c.SuspendCronWorkflow("", "default", "my-cw")
	require.NoError(t, err)
}

func TestResumeCronWorkflow(t *testing.T) {
	cw := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "CronWorkflow",
			"metadata":   map[string]interface{}{"name": "my-cw", "namespace": "default"},
		},
	}
	dc := newFakeDynClient(cw)
	c := newFakeClient(nil, dc)

	err := c.ResumeCronWorkflow("", "default", "my-cw")
	require.NoError(t, err)
}

func TestPauseKEDAResource(t *testing.T) {
	so := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "keda.sh/v1alpha1",
			"kind":       "ScaledObject",
			"metadata":   map[string]interface{}{"name": "my-so", "namespace": "default"},
		},
	}
	dc := newFakeDynClientWith(map[schema.GroupVersionResource]string{
		{Group: "keda.sh", Version: "v1alpha1", Resource: "scaledobjects"}: "ScaledObjectList",
	}, so)
	c := newFakeClient(nil, dc)

	err := c.PauseKEDAResource("", "default", "my-so",
		schema.GroupVersionResource{Group: "keda.sh", Version: "v1alpha1", Resource: "scaledobjects"})
	require.NoError(t, err)
}

func TestUnpauseKEDAResource(t *testing.T) {
	so := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "keda.sh/v1alpha1",
			"kind":       "ScaledObject",
			"metadata": map[string]interface{}{
				"name":        "my-so",
				"namespace":   "default",
				"annotations": map[string]interface{}{"autoscaling.keda.sh/paused": "true"},
			},
		},
	}
	dc := newFakeDynClientWith(map[schema.GroupVersionResource]string{
		{Group: "keda.sh", Version: "v1alpha1", Resource: "scaledobjects"}: "ScaledObjectList",
	}, so)
	c := newFakeClient(nil, dc)

	err := c.UnpauseKEDAResource("", "default", "my-so",
		schema.GroupVersionResource{Group: "keda.sh", Version: "v1alpha1", Resource: "scaledobjects"})
	require.NoError(t, err)
}

func TestGetWorkflowStatus(t *testing.T) {
	wf := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Workflow",
			"metadata":   map[string]interface{}{"name": "my-wf", "namespace": "default"},
			"status": map[string]interface{}{
				"phase":      "Succeeded",
				"startedAt":  "2026-01-01T00:00:00Z",
				"finishedAt": "2026-01-01T00:05:00Z",
				"progress":   "3/3",
				"nodes": map[string]interface{}{
					"node1": map[string]interface{}{
						"displayName": "main",
						"phase":       "Succeeded",
						"type":        "Steps",
						"startedAt":   "2026-01-01T00:00:00Z",
						"finishedAt":  "2026-01-01T00:05:00Z",
					},
				},
			},
		},
	}
	dc := newFakeDynClient(wf)
	c := newFakeClient(nil, dc)

	statusStr, running, err := c.GetWorkflowStatus("", "default", "my-wf")
	require.NoError(t, err)
	assert.Contains(t, statusStr, "Succeeded")
	assert.False(t, running) // Succeeded is not running
}

func TestSubmitWorkflowFromTemplate(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	name, err := c.SubmitWorkflowFromTemplate("", "default", "my-template", false)
	require.NoError(t, err)
	assert.Contains(t, name, "my-template-")
}

func TestSubmitWorkflowFromTemplate_ClusterScope(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	name, err := c.SubmitWorkflowFromTemplate("", "default", "my-template", true)
	require.NoError(t, err)
	assert.Contains(t, name, "my-template-")
}

// =====================================================================
// resources.go: Tree builders
// =====================================================================

func TestBuildDeploymentTree(t *testing.T) {
	rs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]interface{}{
				"name": "deploy-abc", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "Deployment", "name": "deploy"},
				},
			},
		},
	}
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "deploy-abc-xyz", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "ReplicaSet", "name": "deploy-abc"},
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{map[string]interface{}{"name": "app"}},
			},
		},
	}
	dc := newFakeDynClient(rs, pod)
	c := newFakeClient(nil, dc)

	root := &model.ResourceNode{Name: "deploy", Kind: "Deployment", Namespace: "default"}
	err := c.buildDeploymentTree(context.Background(), dc, "default", "deploy", root)
	require.NoError(t, err)
	assert.Len(t, root.Children, 1)
	assert.Equal(t, "ReplicaSet", root.Children[0].Kind)
	assert.Len(t, root.Children[0].Children, 1)
}

func TestBuildPodOwnerTree(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "sts-0", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "StatefulSet", "name": "my-sts"},
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{map[string]interface{}{"name": "db"}},
			},
		},
	}
	dc := newFakeDynClient(pod)
	c := newFakeClient(nil, dc)

	root := &model.ResourceNode{Name: "my-sts", Kind: "StatefulSet", Namespace: "default"}
	err := c.buildPodOwnerTree(context.Background(), dc, "default", "StatefulSet", "my-sts", root)
	require.NoError(t, err)
	assert.Len(t, root.Children, 1)
	assert.Equal(t, "Pod", root.Children[0].Kind)
}

func TestBuildCronJobTree(t *testing.T) {
	job := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "batch/v1",
			"kind":       "Job",
			"metadata": map[string]interface{}{
				"name": "cron-12345", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "CronJob", "name": "my-cron"},
				},
			},
		},
	}
	dc := newFakeDynClient(job)
	c := newFakeClient(nil, dc)

	root := &model.ResourceNode{Name: "my-cron", Kind: "CronJob", Namespace: "default"}
	err := c.buildCronJobTree(context.Background(), dc, "default", "my-cron", root)
	require.NoError(t, err)
	assert.Len(t, root.Children, 1)
	assert.Equal(t, "Job", root.Children[0].Kind)
}

func TestGetPodsViaReplicaSets(t *testing.T) {
	rs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]interface{}{
				"name": "deploy-abc", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "Deployment", "name": "deploy"},
				},
			},
		},
	}
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "deploy-abc-xyz", "namespace": "default",
				"creationTimestamp": "2026-01-01T00:00:00Z",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "ReplicaSet", "name": "deploy-abc"},
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{map[string]interface{}{"name": "app"}},
			},
			"status": map[string]interface{}{"phase": "Running"},
		},
	}
	dc := newFakeDynClient(rs, pod)
	c := newFakeClient(nil, dc)

	items, err := c.getPodsViaReplicaSets(context.Background(), dc, "default", "deploy")
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "deploy-abc-xyz", items[0].Name)
}

func TestGetPodsByOwner(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "sts-0", "namespace": "default",
				"creationTimestamp": "2026-01-01T00:00:00Z",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "StatefulSet", "name": "my-sts"},
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{map[string]interface{}{"name": "app"}},
			},
			"status": map[string]interface{}{"phase": "Running"},
		},
	}
	dc := newFakeDynClient(pod)
	c := newFakeClient(nil, dc)

	items, err := c.getPodsByOwner(context.Background(), dc, "default", "StatefulSet", "my-sts")
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestGetJobsByOwner(t *testing.T) {
	job := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "batch/v1",
			"kind":       "Job",
			"metadata": map[string]interface{}{
				"name": "cron-12345", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "CronJob", "name": "my-cron"},
				},
			},
		},
	}
	dc := newFakeDynClient(job)
	c := newFakeClient(nil, dc)

	items, err := c.getJobsByOwner(context.Background(), dc, "default", "my-cron")
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestBuildGenericOwnerTree(t *testing.T) {
	sts := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "StatefulSet",
			"metadata": map[string]interface{}{
				"name": "cluster-sts", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "Cluster", "name": "my-cluster"},
				},
			},
		},
	}
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "cluster-sts-0", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "StatefulSet", "name": "cluster-sts"},
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{map[string]interface{}{"name": "pg"}},
			},
		},
	}
	dc := newFakeDynClient(sts, pod)
	c := newFakeClient(nil, dc)

	root := &model.ResourceNode{Name: "my-cluster", Kind: "Cluster", Namespace: "default"}
	err := c.buildGenericOwnerTree(context.Background(), dc, "default", "Cluster", "my-cluster", root)
	require.NoError(t, err)
	assert.Greater(t, len(root.Children), 0)
}

// =====================================================================
// netpol.go
// =====================================================================

func TestGetNetworkPolicyInfo(t *testing.T) {
	np := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "NetworkPolicy",
			"metadata":   map[string]interface{}{"name": "my-np", "namespace": "default"},
			"spec": map[string]interface{}{
				"podSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{"app": "web"},
				},
				"policyTypes": []interface{}{"Ingress", "Egress"},
				"ingress": []interface{}{
					map[string]interface{}{
						"from": []interface{}{
							map[string]interface{}{
								"podSelector": map[string]interface{}{
									"matchLabels": map[string]interface{}{"role": "frontend"},
								},
							},
						},
						"ports": []interface{}{
							map[string]interface{}{"protocol": "TCP", "port": float64(80)},
						},
					},
				},
				"egress": []interface{}{
					map[string]interface{}{
						"to": []interface{}{
							map[string]interface{}{
								"ipBlock": map[string]interface{}{
									"cidr":   "10.0.0.0/8",
									"except": []interface{}{"10.0.0.1/32"},
								},
							},
						},
					},
				},
			},
		},
	}
	// Also add a pod that matches the selector so findAffectedPods covers the list path.
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "web-pod", "namespace": "default",
				"labels": map[string]interface{}{"app": "web"},
			},
		},
	}
	dc := newFakeDynClient(np, pod)
	c := newFakeClient(nil, dc)

	info, err := c.GetNetworkPolicyInfo(context.Background(), "", "default", "my-np")
	require.NoError(t, err)
	assert.Equal(t, "my-np", info.Name)
	assert.Equal(t, map[string]string{"app": "web"}, info.PodSelector)
	assert.Contains(t, info.PolicyTypes, "Ingress")
	assert.Len(t, info.IngressRules, 1)
	assert.Len(t, info.EgressRules, 1)
	// findAffectedPods returns pods (fake doesn't filter by label, returns all).
	assert.GreaterOrEqual(t, len(info.AffectedPods), 1)
}

func TestGetNetworkPolicyInfo_NoSpec(t *testing.T) {
	np := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "NetworkPolicy",
			"metadata":   map[string]interface{}{"name": "empty-np", "namespace": "default"},
		},
	}
	dc := newFakeDynClient(np)
	c := newFakeClient(nil, dc)

	info, err := c.GetNetworkPolicyInfo(context.Background(), "", "default", "empty-np")
	require.NoError(t, err)
	assert.Equal(t, "empty-np", info.Name)
	assert.Nil(t, info.IngressRules)
}

// =====================================================================
// client_operations.go: RollbackDeployment
// =====================================================================

func newDeploymentForRollback(name, ns, uid, image string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, UID: k8stypes.UID(uid)},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: image}}},
			},
		},
	}
}

func newReplicaSetForRollback(name, ns, ownerUID, revision, image string) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: ns,
			Annotations:     map[string]string{"deployment.kubernetes.io/revision": revision},
			OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "my-deploy", UID: k8stypes.UID(ownerUID)}},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: image}}},
			},
		},
	}
}

func TestRollbackDeployment(t *testing.T) {
	dep := newDeploymentForRollback("my-deploy", "default", "uid-1", "app:v2")
	rs1 := newReplicaSetForRollback("my-deploy-old", "default", "uid-1", "1", "app:v1")
	rs2 := newReplicaSetForRollback("my-deploy-new", "default", "uid-1", "2", "app:v2")

	cs := k8sfake.NewClientset(dep, rs1, rs2)
	c := newFakeClient(cs, nil)

	err := c.RollbackDeployment(context.Background(), "", "default", "my-deploy", 1)
	require.NoError(t, err)
}

func TestRollbackDeployment_RevisionNotFound(t *testing.T) {
	dep := newDeploymentForRollback("my-deploy", "default", "uid-2", "app:v1")
	rs := newReplicaSetForRollback("my-deploy-abc", "default", "uid-2", "1", "app:v1")

	cs := k8sfake.NewClientset(dep, rs)
	c := newFakeClient(cs, nil)

	err := c.RollbackDeployment(context.Background(), "", "default", "my-deploy", 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "revision 99 not found")
}

// =====================================================================
// gitops.go: getFluxManagedResources
// =====================================================================

func TestGetFluxManagedResources(t *testing.T) {
	ks := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata":   map[string]interface{}{"name": "my-ks", "namespace": "flux-system"},
			"status": map[string]interface{}{
				"inventory": map[string]interface{}{
					"entries": []interface{}{
						map[string]interface{}{"id": "default_my-deploy_apps_Deployment", "v": "v1"},
						map[string]interface{}{"id": "default_my-svc__Service", "v": "v1"},
						map[string]interface{}{"id": "default_my-cm__ConfigMap", "v": "v1"},
						map[string]interface{}{"id": "default_my-secret__Secret", "v": "v1"},
						map[string]interface{}{"id": "kube-system_my-ds_apps_DaemonSet", "v": "v1"},
					},
				},
			},
		},
	}
	dc := newFakeDynClient(ks)
	c := newFakeClient(nil, dc)

	items, err := c.getFluxManagedResources(context.Background(), dc, "flux-system", "my-ks")
	require.NoError(t, err)
	assert.Len(t, items, 5)
	// Items should be sorted by Kind then Name.
	assert.Equal(t, "ConfigMap", items[0].Kind)
}

// TestGetFluxManagedResources_PreservesK8sNameForCrossNamespace is a regression
// guard mirroring the helm-side fix: cross-namespace inventory entries must not
// have their Item.Name mutated to "<namespace>/<name>", because that mutated
// value is forwarded to the Kubernetes API as the resource name when the user
// loads YAML or labels for the item, and the API rejects any name containing
// '/'.
func TestGetFluxManagedResources_PreservesK8sNameForCrossNamespace(t *testing.T) {
	// Kustomization lives in flux-system; the inventory holds an entry in a
	// different namespace (cilium-secrets) so the cross-namespace path is
	// exercised.
	ks := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata":   map[string]interface{}{"name": "my-ks", "namespace": "flux-system"},
			"status": map[string]interface{}{
				"inventory": map[string]interface{}{
					"entries": []interface{}{
						map[string]interface{}{"id": "cilium-secrets_cilium-operator-tlsinterception-secrets__Secret", "v": "v1"},
						map[string]interface{}{"id": "flux-system_local-cm__ConfigMap", "v": "v1"},
					},
				},
			},
		},
	}
	dc := newFakeDynClient(ks)
	c := newFakeClient(nil, dc)

	items, err := c.getFluxManagedResources(context.Background(), dc, "flux-system", "my-ks")
	require.NoError(t, err)
	require.Len(t, items, 2)

	// Find the cross-namespace Secret in the result regardless of sort order.
	var secret *model.Item
	for i := range items {
		if items[i].Kind == "Secret" {
			secret = &items[i]
			break
		}
	}
	require.NotNil(t, secret)

	// Name must be the raw K8s name, namespace must live in Item.Namespace.
	assert.Equal(t, "cilium-operator-tlsinterception-secrets", secret.Name,
		"cross-namespace inventory entries must not be mutated with namespace prefix")
	assert.NotContains(t, secret.Name, "/",
		"K8s resource names cannot contain slashes")
	assert.Equal(t, "cilium-secrets", secret.Namespace,
		"namespace must still be carried in Item.Namespace for the renderer")
}

func TestGetFluxManagedResources_NoStatus(t *testing.T) {
	ks := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata":   map[string]interface{}{"name": "my-ks", "namespace": "flux-system"},
		},
	}
	dc := newFakeDynClient(ks)
	c := newFakeClient(nil, dc)

	items, err := c.getFluxManagedResources(context.Background(), dc, "flux-system", "my-ks")
	require.NoError(t, err)
	assert.Nil(t, items)
}

func TestGetFluxManagedResources_NoInventory(t *testing.T) {
	ks := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata":   map[string]interface{}{"name": "my-ks", "namespace": "flux-system"},
			"status":     map[string]interface{}{},
		},
	}
	dc := newFakeDynClient(ks)
	c := newFakeClient(nil, dc)

	items, err := c.getFluxManagedResources(context.Background(), dc, "flux-system", "my-ks")
	require.NoError(t, err)
	assert.Nil(t, items)
}

func TestGetFluxManagedResources_EmptyEntries(t *testing.T) {
	ks := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata":   map[string]interface{}{"name": "my-ks", "namespace": "flux-system"},
			"status": map[string]interface{}{
				"inventory": map[string]interface{}{
					"entries": []interface{}{},
				},
			},
		},
	}
	dc := newFakeDynClient(ks)
	c := newFakeClient(nil, dc)

	items, err := c.getFluxManagedResources(context.Background(), dc, "flux-system", "my-ks")
	require.NoError(t, err)
	assert.Nil(t, items)
}

func TestGetFluxManagedResources_IconMapping(t *testing.T) {
	ks := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata":   map[string]interface{}{"name": "my-ks", "namespace": "flux-system"},
			"status": map[string]interface{}{
				"inventory": map[string]interface{}{
					"entries": []interface{}{
						map[string]interface{}{"id": "default_pod1__Pod", "v": "v1"},
						map[string]interface{}{"id": "default_sts1_apps_StatefulSet", "v": "v1"},
						map[string]interface{}{"id": "default_ing1_networking.k8s.io_Ingress", "v": "v1"},
						map[string]interface{}{"id": "default_sa1__ServiceAccount", "v": "v1"},
						map[string]interface{}{"id": "_ns1__Namespace", "v": "v1"},
						map[string]interface{}{"id": "default_crd1_example.com_CustomResource", "v": "v1"},
					},
				},
			},
		},
	}
	dc := newFakeDynClient(ks)
	c := newFakeClient(nil, dc)

	items, err := c.getFluxManagedResources(context.Background(), dc, "flux-system", "my-ks")
	require.NoError(t, err)
	assert.Len(t, items, 6)
}

// =====================================================================
// gitops.go: getArgoManagedResources
// =====================================================================

func TestGetArgoManagedResources_WithStatusResources(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
			"status": map[string]interface{}{
				"resources": []interface{}{
					map[string]interface{}{
						"name": "my-deploy", "kind": "Deployment", "namespace": "default",
						"group": "apps", "version": "v1", "status": "Synced",
						"health": map[string]interface{}{"status": "Healthy"},
					},
					map[string]interface{}{
						"name": "my-svc", "kind": "Service", "namespace": "default",
						"group": "", "version": "v1", "status": "Synced",
					},
				},
			},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)

	items, err := c.getArgoManagedResources(context.Background(), dc, "", "argocd", "my-app")
	require.NoError(t, err)
	assert.Len(t, items, 2)
	// First should have combined status.
	for _, item := range items {
		if item.Name == "my-deploy" {
			assert.Equal(t, "Healthy/Synced", item.Status)
		}
	}
}

func TestGetArgoManagedResources_FallbackToLabels(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]interface{}{"name": "my-app", "namespace": "argocd"},
			"spec": map[string]interface{}{
				"destination": map[string]interface{}{
					"namespace": "production",
				},
			},
			"status": map[string]interface{}{},
		},
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app-deploy",
			Namespace: "production",
			Labels:    map[string]string{"app.kubernetes.io/instance": "my-app"},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "app:v1"}}},
			},
		},
	}
	cs := k8sfake.NewClientset(dep)
	dc := newFakeDynClient(app)
	c := newFakeClient(cs, dc)

	items, err := c.getArgoManagedResources(context.Background(), dc, "", "argocd", "my-app")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(items), 1)
}

// =====================================================================
// resources.go: buildServiceTree, buildNodeTree, getPodsOnNode, wrapWithOwners
// =====================================================================

func TestBuildServiceTree(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "web"},
			Ports:    []corev1.ServicePort{{Port: 80}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "default", Labels: map[string]string{"app": "web"}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	cs := k8sfake.NewClientset(svc, pod)
	c := newFakeClient(cs, nil)

	root := &model.ResourceNode{Name: "my-svc", Kind: "Service", Namespace: "default"}
	err := c.buildServiceTree(context.Background(), "", "default", "my-svc", root)
	require.NoError(t, err)
	assert.Len(t, root.Children, 1)
	assert.Equal(t, "Pod", root.Children[0].Kind)
}

func TestGetPodsOnNode(t *testing.T) {
	// The fake dynamic client doesn't support field selectors, but we can still
	// test that the function processes results correctly (it returns whatever
	// the list gives). Create a pod that matches.
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "pod-on-node", "namespace": "kube-system",
				"creationTimestamp": "2026-01-01T00:00:00Z",
			},
			"spec": map[string]interface{}{
				"nodeName":   "node-1",
				"containers": []interface{}{map[string]interface{}{"name": "kubelet"}},
			},
			"status": map[string]interface{}{"phase": "Running"},
		},
	}
	dc := newFakeDynClient(pod)
	c := newFakeClient(nil, dc)

	// Note: the fake client returns ALL pods regardless of field selector.
	items, err := c.getPodsOnNode(context.Background(), dc, "node-1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(items), 1)
}

func TestBuildNodeTree(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "pod-on-node", "namespace": "kube-system",
			},
			"spec": map[string]interface{}{
				"nodeName": "node-1",
			},
			"status": map[string]interface{}{"phase": "Running"},
		},
	}
	dc := newFakeDynClient(pod)
	c := newFakeClient(nil, dc)

	root := &model.ResourceNode{Name: "node-1", Kind: "Node"}
	err := c.buildNodeTree(context.Background(), dc, "node-1", root)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(root.Children), 1)
}

func TestWrapWithOwners(t *testing.T) {
	// Create a ReplicaSet owned by a Deployment, so wrapWithOwners can walk upward.
	rs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]interface{}{
				"name": "deploy-abc", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "Deployment", "name": "my-deploy"},
				},
			},
		},
	}
	deploy := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "my-deploy", "namespace": "default",
			},
		},
	}
	dc := newFakeDynClient(rs, deploy)
	c := newFakeClient(nil, dc)

	root := &model.ResourceNode{
		Name: "pod-xyz", Kind: "Pod", Namespace: "default", Status: "Running",
		Children: []*model.ResourceNode{
			{Name: "app", Kind: "Container", Namespace: "default"},
		},
	}
	c.wrapWithOwners(context.Background(), dc, "default", "ReplicaSet", "deploy-abc", root)

	// After wrapping, the root should be the Deployment (topmost owner).
	assert.Equal(t, "Deployment", root.Kind)
	assert.Equal(t, "my-deploy", root.Name)
	// The Pod should be somewhere in the descendants.
}

func TestWrapWithOwners_UnknownKind(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	root := &model.ResourceNode{
		Name: "pod-xyz", Kind: "Pod", Namespace: "default",
	}
	c.wrapWithOwners(context.Background(), dc, "default", "UnknownCRD", "some-resource", root)
	// Should still wrap even with unknown kind (adds single chain entry).
	assert.Equal(t, "UnknownCRD", root.Kind)
}

func TestWrapWithOwners_EmptyChain(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	root := &model.ResourceNode{
		Name: "pod-xyz", Kind: "Pod", Namespace: "default",
	}
	// No owner references from an existing resource - should still process.
	c.wrapWithOwners(context.Background(), dc, "default", "ReplicaSet", "nonexistent-rs", root)
	// Should wrap with the RS (even though it doesn't exist, it adds a chain entry).
	assert.Equal(t, "ReplicaSet", root.Kind)
}

// =====================================================================
// client_rbac.go: CheckRBAC
// =====================================================================

// =====================================================================
// resources.go: GetOwnedResources for more kinds
// =====================================================================

func TestGetOwnedResources_Deployment(t *testing.T) {
	rs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]interface{}{
				"name": "deploy-abc", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "Deployment", "name": "my-deploy"},
				},
			},
		},
	}
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "deploy-abc-xyz", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "ReplicaSet", "name": "deploy-abc"},
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{map[string]interface{}{"name": "app"}},
			},
			"status": map[string]interface{}{"phase": "Running"},
		},
	}
	dc := newFakeDynClient(rs, pod)
	c := newFakeClient(nil, dc)

	items, err := c.GetOwnedResources(context.Background(), "", "default", "Deployment", "my-deploy")
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "Pod", items[0].Kind)
}

func TestGetOwnedResources_StatefulSet(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "sts-0", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "StatefulSet", "name": "my-sts"},
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{map[string]interface{}{"name": "db"}},
			},
			"status": map[string]interface{}{"phase": "Running"},
		},
	}
	dc := newFakeDynClient(pod)
	c := newFakeClient(nil, dc)

	items, err := c.GetOwnedResources(context.Background(), "", "default", "StatefulSet", "my-sts")
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestGetOwnedResources_Node(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "kubelet-pod", "namespace": "kube-system",
			},
			"spec": map[string]interface{}{
				"nodeName": "node-1",
			},
			"status": map[string]interface{}{"phase": "Running"},
		},
	}
	dc := newFakeDynClient(pod)
	c := newFakeClient(nil, dc)

	items, err := c.GetOwnedResources(context.Background(), "", "", "Node", "node-1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(items), 1)
}

func TestGetOwnedResources_Service(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "web"},
			Ports:    []corev1.ServicePort{{Port: 80}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "default", Labels: map[string]string{"app": "web"}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	cs := k8sfake.NewClientset(svc, pod)
	dc := newFakeDynClient()
	c := newFakeClient(cs, dc)

	items, err := c.GetOwnedResources(context.Background(), "", "default", "Service", "my-svc")
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

// =====================================================================
// resources.go: GetResourceTree for more kinds
// =====================================================================

func TestGetResourceTree_Deployment(t *testing.T) {
	rs := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]interface{}{
				"name": "deploy-abc", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "Deployment", "name": "my-deploy"},
				},
			},
		},
	}
	dc := newFakeDynClient(rs)
	c := newFakeClient(nil, dc)

	root, err := c.GetResourceTree(context.Background(), "", "default", "Deployment", "my-deploy")
	require.NoError(t, err)
	assert.Equal(t, "my-deploy", root.Name)
}

func TestGetResourceTree_StatefulSet(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	root, err := c.GetResourceTree(context.Background(), "", "default", "StatefulSet", "my-sts")
	require.NoError(t, err)
	assert.Equal(t, "my-sts", root.Name)
}

func TestGetResourceTree_CronJob(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	root, err := c.GetResourceTree(context.Background(), "", "default", "CronJob", "my-cron")
	require.NoError(t, err)
	assert.Equal(t, "my-cron", root.Name)
}

func TestGetResourceTree_Service(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80}}},
	}
	cs := k8sfake.NewClientset(svc)
	dc := newFakeDynClient()
	c := newFakeClient(cs, dc)

	root, err := c.GetResourceTree(context.Background(), "", "default", "Service", "my-svc")
	require.NoError(t, err)
	assert.Equal(t, "my-svc", root.Name)
}

func TestGetResourceTree_Node(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	root, err := c.GetResourceTree(context.Background(), "", "", "Node", "node-1")
	require.NoError(t, err)
	assert.Equal(t, "node-1", root.Name)
}

func TestGetResourceTree_Pod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	cs := k8sfake.NewClientset(pod)
	dc := newFakeDynClient()
	c := newFakeClient(cs, dc)

	root, err := c.GetResourceTree(context.Background(), "", "default", "Pod", "my-pod")
	require.NoError(t, err)
	assert.Equal(t, "Running", root.Status)
}

func TestGetResourceTree_ReplicaSet(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	root, err := c.GetResourceTree(context.Background(), "", "default", "ReplicaSet", "my-rs")
	require.NoError(t, err)
	assert.Equal(t, "my-rs", root.Name)
}

func TestGetResourceTree_Job(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	root, err := c.GetResourceTree(context.Background(), "", "default", "Job", "my-job")
	require.NoError(t, err)
	assert.Equal(t, "my-job", root.Name)
}

func TestGetResourceTree_DaemonSet(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)

	root, err := c.GetResourceTree(context.Background(), "", "default", "DaemonSet", "my-ds")
	require.NoError(t, err)
	assert.Equal(t, "my-ds", root.Name)
}

// =====================================================================
// resources.go: GetOwnedResources for HelmRelease and Kustomization
// =====================================================================

func TestGetOwnedResources_Kustomization(t *testing.T) {
	ks := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata":   map[string]interface{}{"name": "my-ks", "namespace": "default"},
			"status": map[string]interface{}{
				"inventory": map[string]interface{}{
					"entries": []interface{}{
						map[string]interface{}{"id": "default_svc1__Service", "v": "v1"},
					},
				},
			},
		},
	}
	dc := newFakeDynClient(ks)
	c := newFakeClient(nil, dc)

	items, err := c.GetOwnedResources(context.Background(), "", "default", "Kustomization", "my-ks")
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestGetOwnedResources_Job(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "job-pod", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "Job", "name": "my-job"},
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{map[string]interface{}{"name": "worker"}},
			},
		},
	}
	dc := newFakeDynClient(pod)
	c := newFakeClient(nil, dc)

	items, err := c.GetOwnedResources(context.Background(), "", "default", "Job", "my-job")
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestGetOwnedResources_DaemonSet(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "ds-pod", "namespace": "default",
				"ownerReferences": []interface{}{
					map[string]interface{}{"kind": "DaemonSet", "name": "my-ds"},
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{map[string]interface{}{"name": "agent"}},
			},
		},
	}
	dc := newFakeDynClient(pod)
	c := newFakeClient(nil, dc)

	items, err := c.GetOwnedResources(context.Background(), "", "default", "DaemonSet", "my-ds")
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestGetOwnedResources_HelmRelease(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "myrel-deploy", Namespace: "default",
			Labels: map[string]string{"app.kubernetes.io/instance": "myrel"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "app:v1"}}},
			},
		},
		Status: appsv1.DeploymentStatus{AvailableReplicas: 1, ReadyReplicas: 1},
	}
	cs := k8sfake.NewClientset(dep)
	dc := newFakeDynClient()
	c := newFakeClient(cs, dc)

	items, err := c.GetOwnedResources(context.Background(), "", "default", "HelmRelease", "myrel")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(items), 1)
}

// =====================================================================
// netpol.go: findAffectedPods standalone
// =====================================================================

func TestFindAffectedPods_Empty(t *testing.T) {
	dc := newFakeDynClient()
	result := findAffectedPods(context.Background(), dc, "default", map[string]string{"app": "web"})
	assert.Empty(t, result)
}

func TestFindAffectedPods_WithPods(t *testing.T) {
	pod := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "web-pod", "namespace": "default",
				"labels": map[string]interface{}{"app": "web"},
			},
		},
	}
	dc := newFakeDynClient(pod)
	result := findAffectedPods(context.Background(), dc, "default", nil)
	assert.GreaterOrEqual(t, len(result), 1)
}

// =====================================================================
// client_operations.go: GetPodYAML (covers the delegation)
// =====================================================================

func TestGetPodYAML(t *testing.T) {
	// GetPodYAML delegates to GetResourceYAML with a non-namespaced ResourceTypeEntry.
	// We test it with a cluster-scoped object since that's how the delegation works.
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "my-pod",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": "app", "image": "nginx"},
				},
			},
		},
	}
	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	yamlStr, err := c.GetPodYAML(context.Background(), "", "", "my-pod")
	require.NoError(t, err)
	assert.Contains(t, yamlStr, "kind: Pod")
}
