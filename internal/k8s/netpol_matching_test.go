package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func netpolMatchFakeDyn(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:                                      "PodList",
		{Group: "", Version: "v1", Resource: "services"}:                                  "ServiceList",
		{Group: "", Version: "v1", Resource: "namespaces"}:                                "NamespaceList",
		{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}:          "NetworkPolicyList",
		{Group: "cilium.io", Version: "v2", Resource: "ciliumnetworkpolicies"}:            "CiliumNetworkPolicyList",
		{Group: "cilium.io", Version: "v2", Resource: "ciliumclusterwidenetworkpolicies"}: "CiliumClusterwideNetworkPolicyList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, objects...)
}

func testPod(name string, lbls map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "default",
			"labels":    lbls,
		},
	}}
}

func testPodInNS(name, namespace string, lbls map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
			"labels":    lbls,
		},
	}}
}

func testNamespace(name string, lbls map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]any{
			"name":   name,
			"labels": lbls,
		},
	}}
}

func testNetpol(name string, spec map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "NetworkPolicy",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "default",
		},
		"spec": spec,
	}}
}

func testService(name string, selector map[string]any) *unstructured.Unstructured {
	spec := map[string]any{}
	if selector != nil {
		spec["selector"] = selector
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "default",
		},
		"spec": spec,
	}}
}

func TestGetNetworkPoliciesForPod_MatchesByLabels(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web", "tier": "frontend"}),
		testPod("db-1", map[string]any{"app": "db"}),
		testNetpol("allow-web", map[string]any{
			"podSelector": map[string]any{
				"matchLabels": map[string]any{"app": "web"},
			},
			"policyTypes": []any{"Ingress"},
			"ingress": []any{
				map[string]any{
					"from": []any{
						map[string]any{"podSelector": map[string]any{
							"matchLabels": map[string]any{"app": "db"},
						}},
					},
				},
			},
		}),
		testNetpol("allow-db", map[string]any{
			"podSelector": map[string]any{
				"matchLabels": map[string]any{"app": "db"},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "default", "web-1")
	require.NoError(t, err)
	assert.Equal(t, "Pod", info.Kind)
	assert.Equal(t, "web-1", info.Name)
	assert.Equal(t, "default", info.Namespace)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "allow-web", info.Policies[0].Name)
	assert.Equal(t, []string{"Ingress"}, info.Policies[0].PolicyTypes)
	require.Len(t, info.Policies[0].IngressRules, 1)
	assert.Equal(t, []string{"web-1"}, info.Policies[0].AffectedPods)
}

func TestGetNetworkPoliciesForPod_EmptySelectorMatchesAll(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web"}),
		testNetpol("default-deny", map[string]any{
			"podSelector": map[string]any{},
			"policyTypes": []any{"Ingress", "Egress"},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "default", "web-1")
	require.NoError(t, err)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "default-deny", info.Policies[0].Name)
}

func TestGetNetworkPoliciesForPod_MatchExpressions(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web"}),
		testPod("api-1", map[string]any{"app": "api"}),
		testNetpol("expr-policy", map[string]any{
			"podSelector": map[string]any{
				"matchExpressions": []any{
					map[string]any{
						"key":      "app",
						"operator": "In",
						"values":   []any{"web", "worker"},
					},
				},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "default", "web-1")
	require.NoError(t, err)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "expr-policy", info.Policies[0].Name)

	info, err = c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "default", "api-1")
	require.NoError(t, err)
	assert.Empty(t, info.Policies)
}

func TestGetNetworkPoliciesForPod_NoMatch(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web"}),
		testNetpol("allow-db", map[string]any{
			"podSelector": map[string]any{
				"matchLabels": map[string]any{"app": "db"},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "default", "web-1")
	require.NoError(t, err)
	assert.Empty(t, info.Policies)
}

func TestGetNetworkPoliciesForPod_SpecLessPolicy(t *testing.T) {
	noSpec := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "NetworkPolicy",
		"metadata": map[string]any{
			"name":      "spec-less",
			"namespace": "default",
		},
	}}
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web"}),
		noSpec,
	)
	c := NewTestClient(nil, dyn)

	// No spec means an absent podSelector, which selects all pods.
	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "default", "web-1")
	require.NoError(t, err)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "spec-less", info.Policies[0].Name)
}

func TestGetNetworkPoliciesForPod_PodNotFound(t *testing.T) {
	dyn := netpolMatchFakeDyn()
	c := NewTestClient(nil, dyn)

	_, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "default", "missing")
	require.Error(t, err)
}

func TestGetNetworkPoliciesForPod_SortedByName(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web"}),
		testNetpol("zz-policy", map[string]any{
			"podSelector": map[string]any{},
		}),
		testNetpol("aa-policy", map[string]any{
			"podSelector": map[string]any{},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForPod(t.Context(), "test-ctx", "default", "web-1")
	require.NoError(t, err)
	require.Len(t, info.Policies, 2)
	assert.Equal(t, "aa-policy", info.Policies[0].Name)
	assert.Equal(t, "zz-policy", info.Policies[1].Name)
}

func TestGetNetworkPoliciesForService_MatchesBackingPods(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testService("web-svc", map[string]any{"app": "web"}),
		testPod("web-1", map[string]any{"app": "web", "track": "stable"}),
		testPod("web-2", map[string]any{"app": "web", "track": "canary"}),
		testPod("db-1", map[string]any{"app": "db"}),
		// Selects only the canary subset of the service's backing pods.
		testNetpol("canary-only", map[string]any{
			"podSelector": map[string]any{
				"matchLabels": map[string]any{"track": "canary"},
			},
		}),
		// Selects no backing pods at all.
		testNetpol("db-only", map[string]any{
			"podSelector": map[string]any{
				"matchLabels": map[string]any{"app": "db"},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForService(t.Context(), "test-ctx", "default", "web-svc")
	require.NoError(t, err)
	assert.Equal(t, "Service", info.Kind)
	assert.Equal(t, "web-svc", info.Name)
	assert.Equal(t, []string{"web-1", "web-2"}, info.BackingPods)
	require.Len(t, info.Policies, 1)
	assert.Equal(t, "canary-only", info.Policies[0].Name)
	assert.Equal(t, []string{"web-2"}, info.Policies[0].MatchedPods)
}

func TestGetNetworkPoliciesForService_NoSelector(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testService("external-svc", nil),
	)
	c := NewTestClient(nil, dyn)

	info, err := c.GetNetworkPoliciesForService(t.Context(), "test-ctx", "default", "external-svc")
	require.NoError(t, err)
	assert.True(t, info.NoSelector)
	assert.Empty(t, info.Policies)
}

func TestGetNetworkPoliciesForService_ServiceNotFound(t *testing.T) {
	dyn := netpolMatchFakeDyn()
	c := NewTestClient(nil, dyn)

	_, err := c.GetNetworkPoliciesForService(t.Context(), "test-ctx", "default", "missing")
	require.Error(t, err)
}
