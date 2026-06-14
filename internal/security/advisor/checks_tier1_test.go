package advisor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func forbidList(client *fake.Clientset, res string) {
	client.PrependReactor("list", res, func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: res}, "", nil)
	})
}

func daemonSet(ns, name string, labels map[string]string, containers ...corev1.Container) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       corev1.PodSpec{Containers: containers},
			},
		},
	}
}

func fetchChecks(t *testing.T, objs ...runtime.Object) map[string]map[string]bool {
	t.Helper()
	s := NewWithClient(fake.NewSimpleClientset(objs...))
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)
	return checksFor(t, findings)
}

func TestNoTopologySpread(t *testing.T) {
	spread := deployment("prod", "spread", 2, nil, hardened("c"))
	spread.Spec.Template.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{{
		MaxSkew: 1, TopologyKey: "kubernetes.io/hostname",
	}}
	anti := deployment("prod", "anti", 2, nil, hardened("c"))
	anti.Spec.Template.Spec.Affinity = &corev1.Affinity{PodAntiAffinity: &corev1.PodAntiAffinity{}}

	got := fetchChecks(t,
		deployment("prod", "clumped", 2, nil, hardened("c")),
		deployment("prod", "solo", 1, nil, hardened("c")),
		spread, anti,
		daemonSet("prod", "ds", nil, hardened("c")),
	)
	assert.True(t, got["prod/Deployment/clumped"]["no_topology_spread"])
	assert.False(t, got["prod/Deployment/solo"]["no_topology_spread"], "single replica needs no spreading")
	assert.False(t, got["prod/Deployment/spread"]["no_topology_spread"])
	assert.False(t, got["prod/Deployment/anti"]["no_topology_spread"])
	assert.False(t, got["prod/DaemonSet/ds"]["no_topology_spread"], "daemonsets are per-node by design")
}

func TestBadRolloutStrategy(t *testing.T) {
	recreate := deployment("prod", "recreate", 2, nil, hardened("c"))
	recreate.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
	recreateSolo := deployment("prod", "recreate-solo", 1, nil, hardened("c"))
	recreateSolo.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
	fullUnavail := deployment("prod", "full-unavail", 3, nil, hardened("c"))
	fullUnavail.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxUnavailable: new(intstr.FromString("100%")),
		},
	}

	intUnavail := func(name string, replicas, maxUnavailable int32) *appsv1.Deployment {
		d := deployment("prod", name, replicas, nil, hardened("c"))
		d.Spec.Strategy = appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{
				MaxUnavailable: new(intstr.FromInt32(maxUnavailable)),
			},
		}
		return d
	}

	got := fetchChecks(t,
		recreate, recreateSolo, fullUnavail,
		deployment("prod", "default", 3, nil, hardened("c")),
		intUnavail("int-all", 3, 3),
		intUnavail("int-partial", 3, 1),
	)
	assert.True(t, got["prod/Deployment/recreate"]["bad_rollout_strategy"])
	assert.False(t, got["prod/Deployment/recreate-solo"]["bad_rollout_strategy"],
		"single replica is down during rollout either way")
	assert.True(t, got["prod/Deployment/full-unavail"]["bad_rollout_strategy"])
	assert.False(t, got["prod/Deployment/default"]["bad_rollout_strategy"])
	assert.True(t, got["prod/Deployment/int-all"]["bad_rollout_strategy"],
		"integer maxUnavailable == replicas takes everything down")
	assert.False(t, got["prod/Deployment/int-partial"]["bad_rollout_strategy"])
}

func TestEmptyDirNoSizeLimit(t *testing.T) {
	limit := resource.MustParse("1Gi")
	unbounded := deployment("prod", "unbounded", 1, nil, hardened("c"))
	unbounded.Spec.Template.Spec.Volumes = []corev1.Volume{
		{Name: "scratch", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}
	bounded := deployment("prod", "bounded", 1, nil, hardened("c"))
	bounded.Spec.Template.Spec.Volumes = []corev1.Volume{
		{Name: "scratch", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: &limit}}},
	}

	s := NewWithClient(fake.NewSimpleClientset(unbounded, bounded))
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)
	got := checksFor(t, findings)
	assert.True(t, got["prod/Deployment/unbounded"]["emptydir_no_sizelimit"])
	assert.False(t, got["prod/Deployment/bounded"]["emptydir_no_sizelimit"])
	for _, f := range findings {
		if f.Labels["check"] == "emptydir_no_sizelimit" {
			assert.Contains(t, f.Summary, "scratch")
		}
	}
}

func TestIdenticalProbes(t *testing.T) {
	probe := func(path string) *corev1.Probe {
		return &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{Path: path, Port: intstr.FromInt32(8080)},
		}}
	}
	same := deployment("prod", "same", 1, nil, corev1.Container{
		Name: "c", LivenessProbe: probe("/healthz"), ReadinessProbe: probe("/healthz"),
	})
	distinct := deployment("prod", "distinct", 1, nil, corev1.Container{
		Name: "c", LivenessProbe: probe("/livez"), ReadinessProbe: probe("/readyz"),
	})

	got := fetchChecks(t, same, distinct)
	assert.True(t, got["prod/Deployment/same"]["identical_probes"])
	assert.False(t, got["prod/Deployment/distinct"]["identical_probes"])
}

func quota(ns, name string, hard, used corev1.ResourceList) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec:       corev1.ResourceQuotaSpec{Hard: hard},
		Status:     corev1.ResourceQuotaStatus{Hard: hard, Used: used},
	}
}

func TestQuotaNearLimit(t *testing.T) {
	got := fetchChecks(t,
		quota("prod", "tight",
			corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")},
			corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")}),
		quota("prod", "roomy",
			corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")},
			corev1.ResourceList{corev1.ResourcePods: resource.MustParse("3")}),
	)
	assert.True(t, got["prod/ResourceQuota/tight"]["quota_near_limit"])
	assert.False(t, got["prod/ResourceQuota/roomy"]["quota_near_limit"])
}

func hpaScaled(ns, name, target string, minReplicas, maxReplicas, desired int32) *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", Name: target},
			MinReplicas:    &minReplicas,
			MaxReplicas:    maxReplicas,
		},
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{DesiredReplicas: desired},
	}
}

func TestHPAAtMaxAndFixed(t *testing.T) {
	got := fetchChecks(t,
		hpaScaled("prod", "capped", "a", 1, 5, 5),
		hpaScaled("prod", "headroom", "b", 1, 5, 3),
		hpaScaled("prod", "pinned", "c", 4, 4, 4),
	)
	assert.True(t, got["prod/HorizontalPodAutoscaler/capped"]["hpa_at_max"])
	assert.False(t, got["prod/HorizontalPodAutoscaler/headroom"]["hpa_at_max"])
	assert.True(t, got["prod/HorizontalPodAutoscaler/pinned"]["hpa_fixed"])
	assert.False(t, got["prod/HorizontalPodAutoscaler/pinned"]["hpa_at_max"],
		"a pinned HPA is hpa_fixed, not hpa_at_max")
}

func TestOrphanPDB(t *testing.T) {
	web := map[string]string{"app": "web"}
	ds := map[string]string{"app": "node-agent"}
	got := fetchChecks(t,
		deployment("prod", "web", 2, web, hardened("c")),
		daemonSet("prod", "agent", ds, hardened("c")),
		pdb("matched", web, new(intstr.FromInt32(1)), nil),
		pdb("ds-matched", ds, new(intstr.FromInt32(1)), nil),
		pdb("orphan", map[string]string{"app": "gone"}, new(intstr.FromInt32(1)), nil),
	)
	assert.False(t, got["prod/PodDisruptionBudget/matched"]["orphan_pdb"])
	assert.False(t, got["prod/PodDisruptionBudget/ds-matched"]["orphan_pdb"],
		"daemonset pods count as matched")
	assert.True(t, got["prod/PodDisruptionBudget/orphan"]["orphan_pdb"])
	// DaemonSets carry the replicas-0 sentinel; integer minAvailable compared
	// against it (1 >= 0) must not flag the PDB as drain-blocking.
	assert.False(t, got["prod/PodDisruptionBudget/ds-matched"]["pdb_blocks_drain"],
		"sentinel replicas must not satisfy the minAvailable >= replicas test")
}

// TestOrphanPDBRequiresWorkloadLists guards the best-effort gate: when any
// workload list is unlistable the orphan check cannot conclude anything and
// must stay silent.
func TestOrphanPDBRequiresWorkloadLists(t *testing.T) {
	client := fake.NewSimpleClientset(
		pdb("orphan", map[string]string{"app": "gone"}, new(intstr.FromInt32(1)), nil),
	)
	forbidList(client, "deployments")
	s := NewWithClient(client)
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)
	// Assert on the raw findings, not a map lookup that is vacuously false
	// for a mistyped key.
	for _, f := range findings {
		assert.NotEqual(t, "orphan_pdb", f.Labels["check"],
			"orphan_pdb must not fire when workloads are unlistable")
	}
}

// TestDaemonSetContainerChecks verifies DaemonSets participate in the
// container-level checks (probes/requests) without triggering the
// replica-based ones.
func TestDaemonSetContainerChecks(t *testing.T) {
	got := fetchChecks(t, daemonSet("prod", "agent", nil, corev1.Container{Name: "bare"}))
	assert.True(t, got["prod/DaemonSet/agent"]["missing_probes"])
	assert.True(t, got["prod/DaemonSet/agent"]["missing_requests"])
	assert.False(t, got["prod/DaemonSet/agent"]["single_replica"])
	assert.False(t, got["prod/DaemonSet/agent"]["no_pdb"])
}
