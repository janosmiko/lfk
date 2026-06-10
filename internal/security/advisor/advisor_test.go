package advisor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/security"
)

// hardened returns a container with probes and requests so it triggers no
// container-level findings; tests strip fields to provoke specific checks.
func hardened(name string) corev1.Container {
	return corev1.Container{
		Name:           name,
		ReadinessProbe: &corev1.Probe{},
		LivenessProbe:  &corev1.Probe{},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		},
	}
}

func deployment(ns, name string, replicas int32, labels map[string]string, containers ...corev1.Container) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       corev1.PodSpec{Containers: containers},
			},
		},
	}
}

func statefulSet(ns, name string, replicas int32, labels map[string]string, containers ...corev1.Container) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       corev1.PodSpec{Containers: containers},
			},
		},
	}
}

func pdb(name string, selector map[string]string, minAvailable, maxUnavailable *intstr.IntOrString) *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: name},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector:       &metav1.LabelSelector{MatchLabels: selector},
			MinAvailable:   minAvailable,
			MaxUnavailable: maxUnavailable,
		},
	}
}

func namespaceObj(name string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

// checksFor maps "namespace/kind/name" -> set of emitted check labels.
func checksFor(t *testing.T, findings []security.Finding) map[string]map[string]bool {
	t.Helper()
	out := map[string]map[string]bool{}
	for _, f := range findings {
		assert.Equal(t, "advisor", f.Source)
		assert.Equal(t, security.CategoryReliability, f.Category)
		key := f.Resource.Namespace + "/" + f.Resource.Kind + "/" + f.Resource.Name
		if out[key] == nil {
			out[key] = map[string]bool{}
		}
		out[key][f.Labels["check"]] = true
	}
	return out
}

func TestSourceMetadata(t *testing.T) {
	s := NewWithClient(fake.NewSimpleClientset())
	assert.Equal(t, "advisor", s.Name())
	assert.Equal(t, []security.Category{security.CategoryReliability}, s.Categories())
	ok, err := s.IsAvailable(context.Background(), "")
	require.NoError(t, err)
	assert.True(t, ok)

	none := New()
	ok, err = none.IsAvailable(context.Background(), "")
	require.NoError(t, err)
	assert.False(t, ok)

	findings, err := none.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestFetchNamespaceChecks(t *testing.T) {
	client := fake.NewSimpleClientset(
		namespaceObj("bare"),
		namespaceObj("guarded"),
		namespaceObj("kube-system"),
		&corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Namespace: "guarded", Name: "q"}},
		&corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{Namespace: "guarded", Name: "lr"}},
	)
	s := NewWithClient(client)
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	got := checksFor(t, findings)

	assert.True(t, got["bare/Namespace/bare"]["namespace_no_quota"])
	assert.True(t, got["bare/Namespace/bare"]["namespace_no_limitrange"])
	assert.Nil(t, got["guarded/Namespace/guarded"])
	assert.Nil(t, got["kube-system/Namespace/kube-system"], "system namespaces are skipped")
}

func TestFetchWorkloadChecks(t *testing.T) {
	web := map[string]string{"app": "web"}
	api := map[string]string{"app": "api"}
	db := map[string]string{"app": "db"}
	client := fake.NewSimpleClientset(
		deployment("prod", "solo", 1, web, hardened("c")),
		deployment("prod", "nopdb", 2, api, hardened("c")),
		deployment("prod", "covered", 3, web, hardened("c")),
		statefulSet("prod", "db", 2, db, hardened("c")),
		pdb("web-pdb", web, new(intstr.FromInt32(1)), nil),
		deployment("kube-system", "coredns", 1, nil, corev1.Container{Name: "c"}),
	)
	s := NewWithClient(client)
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	got := checksFor(t, findings)

	assert.True(t, got["prod/Deployment/solo"]["single_replica"])
	assert.False(t, got["prod/Deployment/solo"]["no_pdb"], "single-replica workloads are not asked for a PDB")
	assert.True(t, got["prod/Deployment/nopdb"]["no_pdb"])
	assert.False(t, got["prod/Deployment/covered"]["no_pdb"], "matching PDB satisfies the check")
	assert.True(t, got["prod/StatefulSet/db"]["no_pdb"])
	assert.Nil(t, got["kube-system/Deployment/coredns"], "system namespaces are skipped")
}

func TestFetchProbeAndRequestChecks(t *testing.T) {
	bare := corev1.Container{Name: "bare"}
	client := fake.NewSimpleClientset(
		deployment("prod", "lax", 1, nil, bare, hardened("good")),
	)
	s := NewWithClient(client)
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)

	var probeSummary, reqSummary string
	for _, f := range findings {
		switch f.Labels["check"] {
		case "missing_probes":
			probeSummary = f.Summary
		case "missing_requests":
			reqSummary = f.Summary
		}
	}
	require.NotEmpty(t, probeSummary)
	require.NotEmpty(t, reqSummary)
	assert.Contains(t, probeSummary, "bare")
	assert.NotContains(t, probeSummary, "good")
	assert.Contains(t, reqSummary, "bare")
	assert.NotContains(t, reqSummary, "good")
}

func TestFetchPDBBlocksDrain(t *testing.T) {
	web := map[string]string{"app": "web"}
	client := fake.NewSimpleClientset(
		deployment("prod", "web", 3, web, hardened("c")),
		pdb("zero-maxunavail", web, nil, new(intstr.FromInt32(0))),
		pdb("full-minavail", web, new(intstr.FromString("100%")), nil),
		pdb("min-eq-replicas", web, new(intstr.FromInt32(3)), nil),
		pdb("healthy", web, new(intstr.FromInt32(1)), nil),
	)
	s := NewWithClient(client)
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	got := checksFor(t, findings)

	assert.True(t, got["prod/PodDisruptionBudget/zero-maxunavail"]["pdb_blocks_drain"])
	assert.True(t, got["prod/PodDisruptionBudget/full-minavail"]["pdb_blocks_drain"])
	assert.True(t, got["prod/PodDisruptionBudget/min-eq-replicas"]["pdb_blocks_drain"])
	assert.Nil(t, got["prod/PodDisruptionBudget/healthy"])
}

func TestFetchHPAChecks(t *testing.T) {
	noReq := corev1.Container{
		Name:           "c",
		ReadinessProbe: &corev1.Probe{},
		LivenessProbe:  &corev1.Probe{},
	}
	utilization := autoscalingv2.MetricSpec{
		Type: autoscalingv2.ResourceMetricSourceType,
		Resource: &autoscalingv2.ResourceMetricSource{
			Name:   corev1.ResourceCPU,
			Target: autoscalingv2.MetricTarget{Type: autoscalingv2.UtilizationMetricType},
		},
	}
	hpa := func(name, target string) *autoscalingv2.HorizontalPodAutoscaler {
		return &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: name},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", Name: target},
				Metrics:        []autoscalingv2.MetricSpec{utilization},
			},
		}
	}
	client := fake.NewSimpleClientset(
		deployment("prod", "noreq", 1, nil, noReq),
		deployment("prod", "withreq", 1, nil, hardened("c")),
		hpa("hpa-noreq", "noreq"),
		hpa("hpa-withreq", "withreq"),
	)
	s := NewWithClient(client)
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	got := checksFor(t, findings)

	assert.True(t, got["prod/Deployment/noreq"]["hpa_no_requests"])
	assert.False(t, got["prod/Deployment/withreq"]["hpa_no_requests"])
	assert.False(t, got["prod/Deployment/noreq"]["single_replica"],
		"HPA-managed workloads are not flagged for replicas: 1")
	assert.False(t, got["prod/Deployment/withreq"]["single_replica"],
		"HPA-managed workloads are not flagged for replicas: 1")
}

// TestFetchBestEffortRBAC verifies that a Forbidden list error on one resource
// type silently drops only the checks that depend on it.
func TestFetchBestEffortRBAC(t *testing.T) {
	api := map[string]string{"app": "api"}
	client := fake.NewSimpleClientset(
		namespaceObj("prod"),
		deployment("prod", "api", 2, api, hardened("c")),
		deployment("prod", "lonely", 1, nil, hardened("c")),
	)
	forbid := func(resource string) {
		client.PrependReactor("list", resource, func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: resource}, "", nil)
		})
	}
	forbid("poddisruptionbudgets")
	forbid("resourcequotas")
	forbid("horizontalpodautoscalers")

	s := NewWithClient(client)
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	got := checksFor(t, findings)

	assert.False(t, got["prod/Deployment/api"]["no_pdb"],
		"no_pdb must not fire when PDBs are unlistable")
	assert.False(t, got["prod/Namespace/prod"]["namespace_no_quota"],
		"namespace_no_quota must not fire when quotas are unlistable")
	assert.True(t, got["prod/Namespace/prod"]["namespace_no_limitrange"],
		"checks with listable dependencies still run")
	assert.True(t, got["prod/Deployment/lonely"]["single_replica"],
		"single_replica still fires when HPAs are unlistable")
}

// TestFetchNilReplicasDefaultsToOne covers the Kubernetes defaulting path:
// a Deployment with spec.replicas omitted runs one replica and is flagged.
func TestFetchNilReplicasDefaultsToOne(t *testing.T) {
	dep := deployment("prod", "implicit", 1, nil, hardened("c"))
	dep.Spec.Replicas = nil
	client := fake.NewSimpleClientset(dep)
	s := NewWithClient(client)
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	got := checksFor(t, findings)
	assert.True(t, got["prod/Deployment/implicit"]["single_replica"])
}

func TestFetchNamespaceFilter(t *testing.T) {
	client := fake.NewSimpleClientset(
		deployment("prod", "a", 1, nil, hardened("c")),
		deployment("staging", "b", 1, nil, hardened("c")),
	)
	s := NewWithClient(client)
	findings, err := s.Fetch(context.Background(), "", "prod")
	require.NoError(t, err)
	for _, f := range findings {
		assert.Equal(t, "prod", f.Resource.Namespace)
	}
}
