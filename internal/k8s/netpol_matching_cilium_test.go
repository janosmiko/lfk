package k8s

import (
	"context"
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

	info, err := c.GetNetworkPoliciesForPod(context.Background(), "test-ctx", "default", "web-1")
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

	info, err := c.GetNetworkPoliciesForPod(context.Background(), "test-ctx", "default", "web-1")
	require.NoError(t, err)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "ccnp-default-ns", info.Policies[0].Name)
	assert.Equal(t, "CiliumClusterwideNetworkPolicy", info.Policies[0].Kind)
}

func TestGetNetworkPoliciesForPod_CiliumNodePolicySkipped(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web"}),
		testCCNP("node-only", map[string]any{
			"nodeSelector": map[string]any{"matchLabels": map[string]any{"role": "worker"}},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(context.Background(), "test-ctx", "default", "web-1")
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

	info, err := c.GetNetworkPoliciesForPod(context.Background(), "test-ctx", "default", "web-1")
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

	info, err := c.GetNetworkPoliciesForPod(context.Background(), "test-ctx", "default", "web-1")
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

	info, err := c.GetNetworkPoliciesForService(context.Background(), "test-ctx", "default", "web-svc")
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

	info, err := c.GetNetworkPoliciesForService(context.Background(), "test-ctx", "default", "web-svc")
	require.NoError(t, err)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "cnp-canary", info.Policies[0].Name)
	assert.Equal(t, "CiliumNetworkPolicy", info.Policies[0].Kind)
	assert.Equal(t, []string{"web-2"}, info.Policies[0].MatchedPods)
}
