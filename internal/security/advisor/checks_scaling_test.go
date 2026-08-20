package advisor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPDBNoUnhealthyPolicy(t *testing.T) {
	one := intstr.FromInt32(1)
	withPolicy := pdb("with-policy", map[string]string{"app": "a"}, &one, nil)
	policy := policyv1.AlwaysAllow
	withPolicy.Spec.UnhealthyPodEvictionPolicy = &policy

	systemPDB := &policyv1.PodDisruptionBudget{
		Namespace: "kube-system", Name: "sys",
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app": "s"}},
			MinAvailable: &one,
		},
	}

	checks := fetchChecks(t,
		deployment("prod", "a", 5, map[string]string{"app": "a"}, hardened("web")),
		withPolicy,
		pdb("no-policy", map[string]string{"app": "a"}, &one, nil),
		systemPDB,
	)
	assert.True(t, checks["prod/PodDisruptionBudget/no-policy"]["pdb_no_unhealthy_policy"])
	assert.False(t, checks["prod/PodDisruptionBudget/with-policy"]["pdb_no_unhealthy_policy"])
	assert.False(t, checks["kube-system/PodDisruptionBudget/sys"]["pdb_no_unhealthy_policy"])
}

func TestPDBVsHPAMin(t *testing.T) {
	three := intstr.FromInt32(3)
	two := intstr.FromInt32(2)
	half := intstr.FromString("50%")

	hpaNilMin := &autoscalingv2.HorizontalPodAutoscaler{
		Namespace: "prod", Name: "hpa-e",
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", Name: "e"},
			MaxReplicas:    10,
		},
	}

	checks := fetchChecks(t,
		// minAvailable 3 > HPA minReplicas 2 -> deadlock risk at min scale.
		deployment("prod", "a", 5, map[string]string{"app": "a"}, hardened("web")),
		pdb("pdb-a", map[string]string{"app": "a"}, &three, nil),
		hpaScaled("prod", "hpa-a", "a", 2, 10, 5),
		// minAvailable == minReplicas: deliberately not flagged (too common).
		deployment("prod", "b", 5, map[string]string{"app": "b"}, hardened("web")),
		pdb("pdb-b", map[string]string{"app": "b"}, &two, nil),
		hpaScaled("prod", "hpa-b", "b", 2, 10, 5),
		// percentage minAvailable: skipped, replica math is ambiguous.
		deployment("prod", "c", 5, map[string]string{"app": "c"}, hardened("web")),
		pdb("pdb-c", map[string]string{"app": "c"}, &half, nil),
		hpaScaled("prod", "hpa-c", "c", 1, 10, 5),
		// no HPA on the target: nothing to conflict with.
		deployment("prod", "d", 5, map[string]string{"app": "d"}, hardened("web")),
		pdb("pdb-d", map[string]string{"app": "d"}, &three, nil),
		// nil minReplicas defaults to 1: minAvailable 2 > 1 fires.
		deployment("prod", "e", 5, map[string]string{"app": "e"}, hardened("web")),
		pdb("pdb-e", map[string]string{"app": "e"}, &two, nil),
		hpaNilMin,
	)
	assert.True(t, checks["prod/PodDisruptionBudget/pdb-a"]["pdb_vs_hpa_min"])
	assert.False(t, checks["prod/PodDisruptionBudget/pdb-b"]["pdb_vs_hpa_min"])
	assert.False(t, checks["prod/PodDisruptionBudget/pdb-c"]["pdb_vs_hpa_min"])
	assert.False(t, checks["prod/PodDisruptionBudget/pdb-d"]["pdb_vs_hpa_min"])
	assert.True(t, checks["prod/PodDisruptionBudget/pdb-e"]["pdb_vs_hpa_min"])
}

func withReplicasOwner(dep *appsv1.Deployment, manager, fields string) *appsv1.Deployment {
	dep.ManagedFields = []metav1.ManagedFieldsEntry{{
		Manager:    manager,
		Operation:  metav1.ManagedFieldsOperationApply,
		FieldsType: "FieldsV1",
		FieldsV1:   metav1.NewFieldsV1(fields),
	}}
	return dep
}

func TestHPAStaticReplicas(t *testing.T) {
	replicasOwned := `{"f:spec":{"f:replicas":{},"f:template":{}}}`
	noReplicas := `{"f:spec":{"f:template":{}}}`
	statusOnly := `{"f:status":{"f:replicas":{}}}`

	checks := fetchChecks(t,
		withReplicasOwner(deployment("prod", "owned", 3, map[string]string{"app": "a"}, hardened("web")),
			"kubectl-client-side-apply", replicasOwned),
		hpaScaled("prod", "hpa-owned", "owned", 1, 10, 3),
		// The HPA's own writes never count, under either manager name the
		// autoscaler has used across Kubernetes versions.
		withReplicasOwner(deployment("prod", "hpa-managed", 3, map[string]string{"app": "b"}, hardened("web")),
			"kube-controller-manager", replicasOwned),
		hpaScaled("prod", "hpa-managed-hpa", "hpa-managed", 1, 10, 3),
		withReplicasOwner(deployment("prod", "hpa-managed2", 3, map[string]string{"app": "b2"}, hardened("web")),
			"horizontal-pod-autoscaler", replicasOwned),
		hpaScaled("prod", "hpa-managed2-hpa", "hpa-managed2", 1, 10, 3),
		// Manifest no longer pins replicas: clean.
		withReplicasOwner(deployment("prod", "no-replicas", 3, map[string]string{"app": "c"}, hardened("web")),
			"kubectl-client-side-apply", noReplicas),
		hpaScaled("prod", "hpa-no-replicas", "no-replicas", 1, 10, 3),
		// f:status.f:replicas is not spec ownership.
		withReplicasOwner(deployment("prod", "status-only", 3, map[string]string{"app": "d"}, hardened("web")),
			"some-controller", statusOnly),
		hpaScaled("prod", "hpa-status-only", "status-only", 1, 10, 3),
		// Owner but no HPA: nothing to fight.
		withReplicasOwner(deployment("prod", "no-hpa", 3, map[string]string{"app": "e"}, hardened("web")),
			"kubectl-client-side-apply", replicasOwned),
	)
	assert.True(t, checks["prod/Deployment/owned"]["hpa_static_replicas"])
	assert.False(t, checks["prod/Deployment/hpa-managed"]["hpa_static_replicas"])
	assert.False(t, checks["prod/Deployment/hpa-managed2"]["hpa_static_replicas"])
	assert.False(t, checks["prod/Deployment/no-replicas"]["hpa_static_replicas"])
	assert.False(t, checks["prod/Deployment/status-only"]["hpa_static_replicas"])
	assert.False(t, checks["prod/Deployment/no-hpa"]["hpa_static_replicas"])
}

// TestHPAStaticReplicasScaleSubresource: any write through the scale
// subresource (HPA controller, kubectl scale) is excluded by Subresource,
// independent of the manager name.
func TestHPAStaticReplicasScaleSubresource(t *testing.T) {
	dep := deployment("apps", "scaled", 3, map[string]string{"app": "a"}, hardened("web"))
	dep.ManagedFields = []metav1.ManagedFieldsEntry{{
		Manager:     "kubectl-scale",
		Operation:   metav1.ManagedFieldsOperationUpdate,
		Subresource: "scale",
		FieldsType:  "FieldsV1",
		FieldsV1:    metav1.NewFieldsV1(`{"f:spec":{"f:replicas":{}}}`),
	}}
	checks := fetchChecks(t, dep, hpaScaled("apps", "scaled-hpa", "scaled", 1, 10, 3))
	assert.False(t, checks["apps/Deployment/scaled"]["hpa_static_replicas"])
}

// TestPDBVsHPAMinNoDuplicateFindings: a PDB whose selector matches two
// HPA-scaled workloads must emit exactly one finding (finding IDs are
// keyed by the PDB).
func TestPDBVsHPAMinNoDuplicateFindings(t *testing.T) {
	three := intstr.FromInt32(3)
	shared := map[string]string{"app": "shared"}
	s := NewWithClient(fake.NewSimpleClientset(
		deployment("prod", "a", 5, shared, hardened("web")),
		deployment("prod", "b", 5, shared, hardened("web")),
		pdb("pdb-shared", shared, &three, nil),
		hpaScaled("prod", "hpa-a", "a", 1, 10, 5),
		hpaScaled("prod", "hpa-b", "b", 1, 10, 5),
	))
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)
	count := 0
	for _, f := range findings {
		if f.Labels["check"] == "pdb_vs_hpa_min" {
			count++
		}
	}
	assert.Equal(t, 1, count, "one PDB must produce one pdb_vs_hpa_min finding")
}
