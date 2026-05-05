package k8s

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestDetectOrphans_EmptyCluster(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	report, err := c.DetectOrphans(context.Background(), "", "")

	require.NoError(t, err)
	assert.Empty(t, report.Pods)
	assert.Empty(t, report.Secrets)
	assert.Empty(t, report.ConfigMaps)
	assert.Empty(t, report.Services)
}

func TestDetectOrphans_PodNoOwner(t *testing.T) {
	now := metav1.Now()
	twoHoursAgo := metav1.NewTime(time.Now().Add(-2 * time.Hour))

	pods := []corev1.Pod{
		// Orphan: no ownerRef, not a mirror pod, not terminal.
		{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "naked-pod", CreationTimestamp: now}},
		// Not orphan: has owner.
		{ObjectMeta: metav1.ObjectMeta{
			Namespace: "default", Name: "owned-pod",
			OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "rs"}},
		}},
		// Not orphan: static (mirror) pod.
		{ObjectMeta: metav1.ObjectMeta{
			Namespace:   "kube-system",
			Name:        "kube-apiserver-foo",
			Annotations: map[string]string{mirrorPodAnnotation: "abc"},
		}},
		// Orphan with terminal tag: Succeeded + age > 1h, no owner.
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "old-job-pod", CreationTimestamp: twoHoursAgo},
			Status:     corev1.PodStatus{Phase: corev1.PodSucceeded},
		},
	}

	objs := podsToRuntimeObjects(pods)
	cs := k8sfake.NewSimpleClientset(objs...)
	c := newFakeClient(cs, nil)

	report, err := c.DetectOrphans(context.Background(), "", "")

	require.NoError(t, err)
	require.Len(t, report.Pods, 2)
	naked := findOrphan(report.Pods, "default", "naked-pod")
	require.NotNil(t, naked)
	assert.Equal(t, "no owner", naked.Reason)
	terminal := findOrphan(report.Pods, "default", "old-job-pod")
	require.NotNil(t, terminal)
	assert.Equal(t, "no owner (terminal)", terminal.Reason)
}

func TestDetectOrphans_SecretExclusions(t *testing.T) {
	pods := []corev1.Pod{
		podWithVolumes("default", "app", []string{"db-creds"}, nil), // mounts db-creds
	}
	secrets := []corev1.Secret{
		// Excluded: helm release storage.
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "sh.helm.release.v1.foo.v1"},
			Type:       corev1.SecretType("helm.sh/release.v1"),
		},
		// Excluded: SA token.
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "default-token-x"},
			Type:       corev1.SecretTypeServiceAccountToken,
		},
		// Excluded: owner-managed.
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default", Name: "managed",
				OwnerReferences: []metav1.OwnerReference{{Kind: "Certificate", Name: "cm"}},
			},
		},
		// Mounted (not orphan).
		{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "db-creds"}},
		// Orphan.
		{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "old-tls-cert"}},
	}

	objs := append(podsToRuntimeObjects(pods), secretsToRuntimeObjects(secrets)...)
	cs := k8sfake.NewSimpleClientset(objs...)
	c := newFakeClient(cs, nil)

	report, err := c.DetectOrphans(context.Background(), "", "")

	require.NoError(t, err)
	require.Len(t, report.Secrets, 1, "expected only old-tls-cert to be flagged")
	assert.Equal(t, "old-tls-cert", report.Secrets[0].Name)
	assert.Equal(t, "unmounted", report.Secrets[0].Reason)
}

func TestDetectOrphans_ConfigMapExclusions(t *testing.T) {
	pods := []corev1.Pod{
		podWithVolumes("default", "app", nil, []string{"feature-flags"}),
	}
	cms := []corev1.ConfigMap{
		// Excluded: kube-root-ca.crt is auto-injected by the API server.
		{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "kube-root-ca.crt"}},
		// Excluded: owner-managed.
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default", Name: "managed-by-flux",
				OwnerReferences: []metav1.OwnerReference{{Kind: "Kustomization", Name: "k"}},
			},
		},
		// Mounted (not orphan).
		{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "feature-flags"}},
		// Orphan.
		{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "deprecated-grafana-cfg"}},
	}

	objs := append(podsToRuntimeObjects(pods), cmsToRuntimeObjects(cms)...)
	cs := k8sfake.NewSimpleClientset(objs...)
	c := newFakeClient(cs, nil)

	report, err := c.DetectOrphans(context.Background(), "", "")

	require.NoError(t, err)
	require.Len(t, report.ConfigMaps, 1)
	assert.Equal(t, "deprecated-grafana-cfg", report.ConfigMaps[0].Name)
	assert.Equal(t, "unmounted", report.ConfigMaps[0].Reason)
}

func secretsToRuntimeObjects(secrets []corev1.Secret) []runtime.Object {
	out := make([]runtime.Object, 0, len(secrets))
	for i := range secrets {
		out = append(out, &secrets[i])
	}
	return out
}

func cmsToRuntimeObjects(cms []corev1.ConfigMap) []runtime.Object {
	out := make([]runtime.Object, 0, len(cms))
	for i := range cms {
		out = append(out, &cms[i])
	}
	return out
}

func servicesToRuntimeObjects(svcs []corev1.Service) []runtime.Object {
	out := make([]runtime.Object, 0, len(svcs))
	for i := range svcs {
		out = append(out, &svcs[i])
	}
	return out
}

func endpointSlicesToRuntimeObjects(s []discoveryv1.EndpointSlice) []runtime.Object {
	out := make([]runtime.Object, 0, len(s))
	for i := range s {
		out = append(out, &s[i])
	}
	return out
}

// helpers ---

func podsToRuntimeObjects(pods []corev1.Pod) []runtime.Object {
	out := make([]runtime.Object, 0, len(pods))
	for i := range pods {
		out = append(out, &pods[i])
	}
	return out
}

func findOrphan(items []OrphanItem, ns, name string) *OrphanItem {
	for i := range items {
		if items[i].Namespace == ns && items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func TestDetectOrphans_ServiceNoEndpoints(t *testing.T) {
	services := []corev1.Service{
		// Orphan: ClusterIP without backing endpoint slice.
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "legacy-api"},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, ClusterIP: "10.0.0.1"},
		},
		// Excluded: Headless.
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "stateful"},
			Spec:       corev1.ServiceSpec{ClusterIP: "None"},
		},
		// Excluded: ExternalName.
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "extern"},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName, ExternalName: "example.com"},
		},
		// Not orphan: has backing slice (added below).
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "live-api"},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, ClusterIP: "10.0.0.2"},
		},
	}
	ready := true
	slices := []discoveryv1.EndpointSlice{{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default", Name: "live-api-abc",
			Labels: map[string]string{"kubernetes.io/service-name": "live-api"},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{{
			Addresses:  []string{"10.244.0.10"},
			Conditions: discoveryv1.EndpointConditions{Ready: &ready},
		}},
	}}

	objs := append(servicesToRuntimeObjects(services), endpointSlicesToRuntimeObjects(slices)...)
	cs := k8sfake.NewSimpleClientset(objs...)
	c := newFakeClient(cs, nil)

	report, err := c.DetectOrphans(context.Background(), "", "")

	require.NoError(t, err)
	require.Len(t, report.Services, 1)
	assert.Equal(t, "legacy-api", report.Services[0].Name)
	assert.Equal(t, "0/0 endpoints", report.Services[0].Reason)
}

// TestDetectOrphans_WorkloadTemplatesPreventFalsePositives covers the
// real-cluster bug a user reported: Secrets/ConfigMaps mounted by
// CronJobs (and other workload controllers) were flagged as "unmounted"
// because we previously walked only live Pods. Between cron firings or
// during a Deployment rollout there's no Pod that references the
// resource — but the workload's PodTemplate still does, and that's what
// makes the Secret a real, in-use resource.
//
// Build a cluster with NO live Pods but a CronJob, Deployment, Job, and
// StatefulSet each referencing a different Secret/ConfigMap, plus a
// truly orphan Secret that's referenced by nothing. Only the latter
// should be flagged.
func TestDetectOrphans_WorkloadTemplatesPreventFalsePositives(t *testing.T) {
	mountSecret := func(name string) corev1.PodSpec {
		return corev1.PodSpec{
			Volumes: []corev1.Volume{{
				Name:         "creds",
				VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: name}},
			}},
		}
	}
	mountCM := func(name string) corev1.PodSpec {
		return corev1.PodSpec{
			Volumes: []corev1.Volume{{
				Name: "cfg",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: name},
					},
				},
			}},
		}
	}

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "nightly-backup"},
		Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{Spec: mountSecret("backup-creds")},
				},
			},
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{Spec: mountSecret("api-creds")},
		},
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "migrate"},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{Spec: mountCM("migrate-cm")},
		},
	}
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "db"},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{Spec: mountSecret("db-pass")},
		},
	}

	secrets := []runtime.Object{
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "backup-creds"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api-creds"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "db-pass"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "actually-orphan"}},
	}
	configMaps := []runtime.Object{
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "migrate-cm"}},
	}

	objs := append([]runtime.Object{cronJob, deployment, job, statefulSet}, secrets...)
	objs = append(objs, configMaps...)
	cs := k8sfake.NewSimpleClientset(objs...)
	c := newFakeClient(cs, nil)

	report, err := c.DetectOrphans(context.Background(), "", "")

	require.NoError(t, err)

	// Strict orphans (LenientOnly=false): only the truly unreferenced
	// "actually-orphan" Secret. The 3 template-referenced Secrets and
	// the Job-template ConfigMap appear in the report tagged
	// LenientOnly=true so the lenient-mode UI can show them.
	strictSecrets := filterLenient(report.Secrets, false)
	require.Len(t, strictSecrets, 1, "only one Secret has no consumer at all")
	assert.Equal(t, "actually-orphan", strictSecrets[0].Name)
	assert.Equal(t, "unmounted", strictSecrets[0].Reason)

	lenientSecrets := filterLenient(report.Secrets, true)
	lenientNames := make([]string, 0, len(lenientSecrets))
	for _, s := range lenientSecrets {
		lenientNames = append(lenientNames, s.Name)
		assert.Equal(t, "no live consumer", s.Reason)
	}
	assert.ElementsMatch(t,
		[]string{"api-creds", "backup-creds", "db-pass"}, lenientNames,
		"template-referenced Secrets get the lenient tag, not the strict one")

	// migrate-cm is mounted by the Job's PodTemplate but no live Pod
	// exists, so under the new semantics it appears as a lenient-only
	// orphan rather than being absent from the report.
	require.Len(t, report.ConfigMaps, 1)
	assert.Equal(t, "migrate-cm", report.ConfigMaps[0].Name)
	assert.True(t, report.ConfigMaps[0].LenientOnly)
}

func filterLenient(items []OrphanItem, lenientOnly bool) []OrphanItem {
	out := make([]OrphanItem, 0, len(items))
	for _, it := range items {
		if it.LenientOnly == lenientOnly {
			out = append(out, it)
		}
	}
	return out
}

// TestDetectOrphans_PVC covers the PVC-as-orphan-kind detector. Three
// PVCs: one mounted by a live Pod (not orphan), one mounted only by a
// Deployment template (lenient-only), one in Pending state (unbound),
// one with no consumer at all (strict orphan), and one owned by a
// StatefulSet (excluded via ownerRef).
func TestDetectOrphans_PVC(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "live"},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "live-data",
					},
				},
			}},
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "api-data",
						},
					},
				}},
			}},
		},
	}
	pvcs := []runtime.Object{
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "live-data"}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api-data"}},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "stuck"},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
		},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "abandoned"}},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default", Name: "owned-by-sts",
				OwnerReferences: []metav1.OwnerReference{{Kind: "StatefulSet", Name: "db"}},
			},
		},
	}

	objs := append([]runtime.Object{pod, deployment}, pvcs...)
	cs := k8sfake.NewSimpleClientset(objs...)
	c := newFakeClient(cs, nil)

	report, err := c.DetectOrphans(context.Background(), "", "")

	require.NoError(t, err)
	require.Len(t, report.PVCs, 3, "live-data and owned-by-sts should be excluded")

	byName := map[string]OrphanItem{}
	for _, p := range report.PVCs {
		byName[p.Name] = p
	}
	assert.Equal(t, "unmounted", byName["abandoned"].Reason)
	assert.False(t, byName["abandoned"].LenientOnly, "no template ref => strict orphan")
	assert.Equal(t, "no live consumer", byName["api-data"].Reason)
	assert.True(t, byName["api-data"].LenientOnly, "Deployment template ref => lenient-only")
	assert.Equal(t, "unbound", byName["stuck"].Reason, "Pending phase wins over the strict/lenient distinction")
}

// TestDetectOrphans_HPA covers the HPA scaleTargetRef validation —
// HPA pointing at a missing Deployment is orphan, HPA pointing at an
// existing one is not.
func TestDetectOrphans_HPA(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api"},
	}
	live := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api-hpa"},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind: "Deployment", Name: "api", APIVersion: "apps/v1",
			},
		},
	}
	stale := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "ghost-hpa"},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind: "Deployment", Name: "deleted", APIVersion: "apps/v1",
			},
		},
	}

	cs := k8sfake.NewSimpleClientset(deployment, live, stale)
	c := newFakeClient(cs, nil)

	report, err := c.DetectOrphans(context.Background(), "", "")
	require.NoError(t, err)
	require.Len(t, report.HPAs, 1)
	assert.Equal(t, "ghost-hpa", report.HPAs[0].Name)
	assert.Contains(t, report.HPAs[0].Reason, "target not found: Deployment/deleted")
}

// TestDetectOrphans_PDB_NetPolSelector covers selector-based orphan
// detection. A PDB / NetworkPolicy with a selector that no Pod (or
// workload-template Pod label) matches is orphan; a selector that
// matches at least one is not. Crucially the match against template
// labels means a PDB targeting a CronJob doesn't get flagged between
// firings.
func TestDetectOrphans_PDB_NetPolSelector(t *testing.T) {
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "nightly"},
		Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "nightly"}},
					},
				},
			},
		},
	}
	pdbMatches := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "matches-cron"},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nightly"}},
		},
	}
	pdbStale := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "stale-pdb"},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "deleted"}},
		},
	}
	netpolStale := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "stale-np"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "deleted"}},
		},
	}

	cs := k8sfake.NewSimpleClientset(cronJob, pdbMatches, pdbStale, netpolStale)
	c := newFakeClient(cs, nil)

	report, err := c.DetectOrphans(context.Background(), "", "")
	require.NoError(t, err)

	// matches-cron should NOT be orphan (CronJob template label matches).
	require.Len(t, report.PDBs, 1)
	assert.Equal(t, "stale-pdb", report.PDBs[0].Name)
	assert.Equal(t, "selects no pods", report.PDBs[0].Reason)

	require.Len(t, report.NetworkPolicies, 1)
	assert.Equal(t, "stale-np", report.NetworkPolicies[0].Name)
}

// TestDetectOrphans_RBAC covers the four RBAC orphan kinds in one
// fixture: a Role bound by a RoleBinding (not orphan), a Role with no
// binding (orphan), a RoleBinding with empty subjects (orphan), a
// RoleBinding referencing a missing ClusterRole (orphan), an
// AggregationRule ClusterRole (excluded), a system: ClusterRole
// (excluded), and a regular unbound ClusterRole (orphan).
func TestDetectOrphans_RBAC(t *testing.T) {
	roleBound := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "deployer"}}
	roleUnbound := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "leftover"}}

	rbValid := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "deployer-rb"},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "ci"}},
		RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "deployer"},
	}
	rbEmpty := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "empty"},
		RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "deployer"},
	}
	rbMissingRole := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "broken"},
		Subjects:   []rbacv1.Subject{{Kind: "User", Name: "alice"}},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "does-not-exist"},
	}

	crAggregate := &rbacv1.ClusterRole{
		ObjectMeta:      metav1.ObjectMeta{Name: "aggregate"},
		AggregationRule: &rbacv1.AggregationRule{},
	}
	crSystem := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "system:foo"}}
	crUnbound := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "leftover-cluster"}}

	cs := k8sfake.NewSimpleClientset(
		roleBound, roleUnbound, rbValid, rbEmpty, rbMissingRole,
		crAggregate, crSystem, crUnbound,
	)
	c := newFakeClient(cs, nil)

	report, err := c.DetectOrphans(context.Background(), "", "")
	require.NoError(t, err)

	roleNames := orphanNames(report.Roles)
	assert.ElementsMatch(t, []string{"leftover"}, roleNames,
		"only the unbound Role is orphan")

	crNames := orphanNames(report.ClusterRoles)
	assert.ElementsMatch(t, []string{"leftover-cluster"}, crNames,
		"system: and aggregation-rule ClusterRoles are excluded")

	rbNames := make(map[string]string)
	for _, rb := range report.RoleBindings {
		rbNames[rb.Name] = rb.Reason
	}
	assert.Equal(t, "empty subjects", rbNames["empty"])
	assert.Contains(t, rbNames["broken"], "missing role")
	assert.NotContains(t, rbNames, "deployer-rb", "valid binding is not orphan")
}

func orphanNames(items []OrphanItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Name)
	}
	return out
}

func TestDetectOrphans_PartialDenial(t *testing.T) {
	cs := k8sfake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "naked"}},
	)
	cs.PrependReactor("list", "ingresses", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})

	c := newFakeClient(cs, nil)
	report, err := c.DetectOrphans(context.Background(), "", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ingresses")
	// Pod scan still ran.
	require.Len(t, report.Pods, 1)
	assert.Equal(t, "naked", report.Pods[0].Name)
}
