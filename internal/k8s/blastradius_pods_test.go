package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func runningPod(namespace, name, node string, labels map[string]string, ready bool) *corev1.Pod {
	cond := corev1.ConditionFalse
	if ready {
		cond = corev1.ConditionTrue
	}
	return &corev1.Pod{
		Namespace: namespace, Name: name, Labels: labels,
		Spec: corev1.PodSpec{NodeName: node},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: cond}},
		},
	}
}

func TestEvictedPodsFrom_CarriesLabelsNamespaceAndReadiness(t *testing.T) {
	pods := []corev1.Pod{
		*runningPod("prod", "web-1", "node-a", map[string]string{"app": "web"}, true),
		*runningPod("prod", "web-2", "node-a", map[string]string{"app": "web"}, false),
	}

	got := EvictedPodsFrom(pods)

	require.Len(t, got, 2)
	assert.Equal(t, "prod", got[0].Namespace)
	assert.Equal(t, map[string]string{"app": "web"}, got[0].Labels)
	assert.True(t, got[0].Ready)
	assert.False(t, got[1].Ready, "a pod without a true Ready condition is not ready")
}

func TestEvictedPodsFrom_MissingReadyConditionIsNotReady(t *testing.T) {
	pods := []corev1.Pod{{Namespace: "prod", Name: "p"}}

	got := EvictedPodsFrom(pods)

	require.Len(t, got, 1)
	assert.False(t, got[0].Ready)
}

func TestPodsOnNode_FiltersByNodeClientSide(t *testing.T) {
	// The fake clientset ignores FieldSelector, which is exactly why the
	// production code re-filters instead of trusting the server.
	cs := fake.NewSimpleClientset(
		runningPod("prod", "web-1", "node-a", map[string]string{"app": "web"}, true),
		runningPod("prod", "web-2", "node-b", map[string]string{"app": "web"}, true),
		runningPod("kube-system", "dns-1", "node-a", map[string]string{"k8s-app": "dns"}, true),
	)

	got, err := podsOnNodeFrom(t.Context(), cs, "node-a")

	require.NoError(t, err)
	require.Len(t, got, 2, "both namespaces on node-a, and nothing from node-b")
	assert.ElementsMatch(t, []string{"prod", "kube-system"},
		[]string{got[0].Namespace, got[1].Namespace})
}

func TestPodsForSelector_MatchesOnlyTheWorkloadsPods(t *testing.T) {
	cs := fake.NewSimpleClientset(
		runningPod("prod", "web-1", "node-a", map[string]string{"app": "web"}, true),
		runningPod("prod", "api-1", "node-a", map[string]string{"app": "api"}, true),
		runningPod("staging", "web-1", "node-a", map[string]string{"app": "web"}, true),
	)

	got, err := podsForSelectorFrom(t.Context(), cs, "prod",
		&metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}})

	require.NoError(t, err)
	require.Len(t, got, 1, "namespace scoped, and only the matching label")
	assert.Equal(t, "prod", got[0].Namespace)
}

func TestPodsForSelector_NilSelectorReturnsNothing(t *testing.T) {
	cs := fake.NewSimpleClientset(runningPod("prod", "web-1", "node-a", nil, true))

	got, err := podsForSelectorFrom(t.Context(), cs, "prod", nil)

	require.NoError(t, err)
	assert.Empty(t, got, "no selector means the workload claims no pods")
}

func TestPodsInNamespace_CarriesTheNameForMatching(t *testing.T) {
	cs := fake.NewSimpleClientset(
		runningPod("prod", "web-1", "node-a", map[string]string{"app": "web"}, true),
		runningPod("prod", "web-2", "node-a", map[string]string{"app": "web"}, false),
	)

	got, err := podsInNamespaceFrom(t.Context(), cs, "prod")

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.ElementsMatch(t, []string{"web-1", "web-2"}, []string{got[0].Name, got[1].Name})
	assert.Equal(t, "prod", got[0].Namespace, "the embedded EvictedPod still carries the namespace")
}

func daemonSetPod(namespace, name, node string) *corev1.Pod {
	p := runningPod(namespace, name, node, map[string]string{"app": "agent"}, true)
	yes := true
	p.OwnerReferences = []metav1.OwnerReference{{Kind: "DaemonSet", Name: "agent", Controller: &yes}}
	return p
}

func TestPodsOnNode_SkipsDaemonSetPods(t *testing.T) {
	// Drain runs with --ignore-daemonsets, so those pods never go. Counting
	// them would overstate the blast radius and invent budget impacts.
	cs := fake.NewSimpleClientset(
		runningPod("prod", "web-1", "node-a", map[string]string{"app": "web"}, true),
		daemonSetPod("kube-system", "agent-1", "node-a"),
	)

	got, err := podsOnNodeFrom(t.Context(), cs, "node-a")

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "prod", got[0].Namespace)
}

func TestPodsOnNode_SkipsMirrorPods(t *testing.T) {
	// A static pod is not removable through the API, so a drain leaves it.
	mirror := runningPod("kube-system", "etcd-node-a", "node-a", nil, true)
	mirror.Annotations = map[string]string{"kubernetes.io/config.mirror": "abc123"}
	cs := fake.NewSimpleClientset(
		mirror,
		runningPod("prod", "web-1", "node-a", map[string]string{"app": "web"}, true),
	)

	got, err := podsOnNodeFrom(t.Context(), cs, "node-a")

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "prod", got[0].Namespace)
}
