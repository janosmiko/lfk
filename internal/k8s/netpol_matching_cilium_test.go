package k8s

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

func testCCNP(name string, spec map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cilium.io/v2",
		"kind":       "CiliumClusterwideNetworkPolicy",
		"metadata":   map[string]any{"name": name},
		"spec":       spec,
	}}
}

func cnpObj(name string, spec map[string]any) *unstructured.Unstructured {
	return testCNP(name, "default", spec, nil)
}

func TestGetNetworkPoliciesForPod_IncludesCiliumPolicies(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web"}),
		testPod("db-1", map[string]any{"app": "db"}),
		cnpObj("cnp-web", map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{"k8s:app": "web"},
			},
			"ingress": []any{
				map[string]any{"fromEndpoints": []any{
					map[string]any{"matchLabels": map[string]any{"app": "db"}},
				}},
			},
		}),
		cnpObj("cnp-db", map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{"app": "db"},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "default", "web-1")
	require.NoError(t, err)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "cnp-web", info.Policies[0].Name)
	assert.Equal(t, "CiliumNetworkPolicy", info.Policies[0].Kind)
	assert.Equal(t, []string{"web-1"}, info.Policies[0].AffectedPods)
}

func TestGetNetworkPoliciesForPod_ClusterwideMatchesByNamespaceLabel(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web"}),
		testCCNP("ccnp-default-ns", map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{"k8s:io.kubernetes.pod.namespace": "default"},
			},
		}),
		testCCNP("ccnp-other-ns", map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{"k8s:io.kubernetes.pod.namespace": "other"},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "default", "web-1")
	require.NoError(t, err)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "ccnp-default-ns", info.Policies[0].Name)
	assert.Equal(t, "CiliumClusterwideNetworkPolicy", info.Policies[0].Kind)
}

func TestGetNetworkPoliciesForPod_ClusterwideMatchesByNamespaceDerivedLabel(t *testing.T) {
	// Cilium projects a namespace's labels onto every endpoint in that
	// namespace under the io.cilium.k8s.namespace.labels.<key> prefix, so a
	// policy selecting such a label must match all pods in labeled namespaces.
	dyn := netpolMatchFakeDyn(
		testNamespace("mynamespace", map[string]any{"egress/kube-dns": "true"}),
		testNamespace("other", map[string]any{}),
		testPodInNS("app-1", "mynamespace", map[string]any{"app": "foo"}),
		testPodInNS("app-2", "other", map[string]any{"app": "foo"}),
		testCCNP("egress-kube-dns", map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{
					"k8s:io.cilium.k8s.namespace.labels.egress/kube-dns": "true",
				},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "mynamespace", "app-1")
	require.NoError(t, err)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "egress-kube-dns", info.Policies[0].Name)
	// app-2 sits in "other" (no matching namespace label) -> excluded.
	assert.Equal(t, []string{"app-1"}, info.Policies[0].AffectedPods)
}

func TestGetNetworkPoliciesForPod_NamespaceDerivedLabelExcludesUnlabeledNS(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testNamespace("mynamespace", map[string]any{"egress/kube-dns": "true"}),
		testNamespace("other", map[string]any{}),
		testPodInNS("app-2", "other", map[string]any{"app": "foo"}),
		testCCNP("egress-kube-dns", map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{
					"k8s:io.cilium.k8s.namespace.labels.egress/kube-dns": "true",
				},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "other", "app-2")
	require.NoError(t, err)
	assert.Empty(t, info.Policies, "pod in an unlabeled namespace must not match")
}

func TestGetCiliumNetworkPolicyInfo_MatchesByNamespaceDerivedLabel(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testNamespace("mynamespace", map[string]any{"egress/kube-dns": "true"}),
		testPodInNS("app-1", "mynamespace", map[string]any{"app": "foo"}),
		testCCNP("egress-kube-dns", map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{
					"k8s:io.cilium.k8s.namespace.labels.egress/kube-dns": "true",
				},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	infos, err := c.GetCiliumNetworkPolicyInfo(t.Context(), "test-ctx", "", "egress-kube-dns", "CiliumClusterwideNetworkPolicy")
	require.NoError(t, err)
	require.Len(t, infos, 1)
	assert.Equal(t, []string{"app-1"}, infos[0].AffectedPods)
}

func TestGetNetworkPoliciesForPod_NamespacedCNPMatchesByNamespaceDerivedLabel(t *testing.T) {
	// A namespaced CiliumNetworkPolicy (not clusterwide) may also select on a
	// namespace-derived label; the service/pod matcher must resolve it too.
	dyn := netpolMatchFakeDyn(
		testNamespace("mynamespace", map[string]any{"egress/kube-dns": "true"}),
		testPodInNS("app-1", "mynamespace", map[string]any{"app": "foo"}),
		testCNP("cnp-egress", "mynamespace", map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{
					"k8s:io.cilium.k8s.namespace.labels.egress/kube-dns": "true",
				},
			},
		}, nil),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "mynamespace", "app-1")
	require.NoError(t, err)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "cnp-egress", info.Policies[0].Name)
	assert.Equal(t, "CiliumNetworkPolicy", info.Policies[0].Kind)
	assert.Equal(t, []string{"app-1"}, info.Policies[0].AffectedPods)
}

func TestGetNetworkPoliciesForService_MatchesByNamespaceDerivedLabel(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testNamespace("mynamespace", map[string]any{"egress/kube-dns": "true"}),
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata":   map[string]any{"name": "web-svc", "namespace": "mynamespace"},
			"spec":       map[string]any{"selector": map[string]any{"app": "foo"}},
		}},
		testPodInNS("app-1", "mynamespace", map[string]any{"app": "foo"}),
		testCCNP("egress-kube-dns", map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{
					"k8s:io.cilium.k8s.namespace.labels.egress/kube-dns": "true",
				},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForService(t.Context(), "test-ctx", "mynamespace", "web-svc")
	require.NoError(t, err)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "egress-kube-dns", info.Policies[0].Name)
	assert.Equal(t, []string{"app-1"}, info.Policies[0].MatchedPods)
}

func TestGetNetworkPoliciesForPod_CiliumNodePolicySkipped(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web"}),
		testCCNP("node-only", map[string]any{
			"nodeSelector": map[string]any{"matchLabels": map[string]any{"role": "worker"}},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "default", "web-1")
	require.NoError(t, err)
	assert.Empty(t, info.Policies, "node policies must not appear in the pod view")
}

func TestGetNetworkPoliciesForPod_CiliumCRDAbsent(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web"}),
		testNetpol("std-policy", map[string]any{"podSelector": map[string]any{}}),
	)
	// Simulate a cluster without the Cilium CRDs: list calls fail.
	dyn.PrependReactor("list", "ciliumnetworkpolicies", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("the server could not find the requested resource")
	})
	dyn.PrependReactor("list", "ciliumclusterwidenetworkpolicies", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("the server could not find the requested resource")
	})
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "default", "web-1")
	require.NoError(t, err, "missing Cilium CRDs must not fail the lookup")
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "std-policy", info.Policies[0].Name)
}

func TestGetNetworkPoliciesForPod_ClusterwideAffectedPodsSpanNamespaces(t *testing.T) {
	otherNsPod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      "web-other",
			"namespace": "other",
			"labels":    map[string]any{"app": "web"},
		},
	}}
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web"}),
		otherNsPod,
		testCCNP("ccnp-web", map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{"k8s:app": "web"},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "default", "web-1")
	require.NoError(t, err)
	require.Len(t, info.Policies, 1)
	// Clusterwide policies report affected pods across all namespaces, not
	// just the queried pod's namespace.
	assert.Equal(t, []string{"web-1", "web-other"}, info.Policies[0].AffectedPods)
}

func TestGetNetworkPoliciesForService_IncludesClusterwide(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testService("web-svc", map[string]any{"app": "web"}),
		testPod("web-1", map[string]any{"app": "web"}),
		testCCNP("ccnp-default", map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{"k8s:io.kubernetes.pod.namespace": "default"},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForService(t.Context(), "test-ctx", "default", "web-svc")
	require.NoError(t, err)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "ccnp-default", info.Policies[0].Name)
	assert.Equal(t, []string{"web-1"}, info.Policies[0].MatchedPods)
}

func TestGetNetworkPoliciesForService_IncludesCilium(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testService("web-svc", map[string]any{"app": "web"}),
		testPod("web-1", map[string]any{"app": "web", "track": "stable"}),
		testPod("web-2", map[string]any{"app": "web", "track": "canary"}),
		cnpObj("cnp-canary", map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{"k8s:track": "canary"},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForService(t.Context(), "test-ctx", "default", "web-svc")
	require.NoError(t, err)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "cnp-canary", info.Policies[0].Name)
	assert.Equal(t, "CiliumNetworkPolicy", info.Policies[0].Kind)
	assert.Equal(t, []string{"web-2"}, info.Policies[0].MatchedPods)
}
