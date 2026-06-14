package k8s

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func testCNP(name, namespace string, spec map[string]any, extra map[string]any) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": "cilium.io/v2",
		"kind":       "CiliumNetworkPolicy",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
	}
	if spec != nil {
		obj["spec"] = spec
	}
	maps.Copy(obj, extra)
	return &unstructured.Unstructured{Object: obj}
}

func TestParseCiliumPolicy_Basic(t *testing.T) {
	obj := testCNP("allow-web", "default", map[string]any{
		"endpointSelector": map[string]any{
			"matchLabels": map[string]any{"app": "web"},
		},
		"ingress": []any{
			map[string]any{
				"fromEndpoints": []any{
					map[string]any{"matchLabels": map[string]any{"app": "db"}},
				},
				"toPorts": []any{
					map[string]any{
						"ports": []any{map[string]any{"port": "8080", "protocol": "TCP"}},
					},
				},
			},
		},
	}, nil)

	specs := parseCiliumPolicy(obj, "CiliumNetworkPolicy")
	require.Len(t, specs, 1)
	info := specs[0].info
	assert.Equal(t, "allow-web", info.Name)
	assert.Equal(t, "CiliumNetworkPolicy", info.Kind)
	assert.Equal(t, map[string]string{"app": "web"}, info.PodSelector)
	assert.Equal(t, []string{"Ingress"}, info.PolicyTypes)
	require.Len(t, info.IngressRules, 1)
	rule := info.IngressRules[0]
	require.Len(t, rule.Peers, 1)
	assert.Equal(t, "Pod", rule.Peers[0].Type)
	assert.Equal(t, map[string]string{"app": "db"}, rule.Peers[0].Selector)
	require.Len(t, rule.Ports, 1)
	assert.Equal(t, "8080", rule.Ports[0].Port)
	assert.Equal(t, "TCP", rule.Ports[0].Protocol)

	require.NotNil(t, specs[0].selector)
	assert.True(t, specs[0].selector.Matches(labels.Set{"app": "web"}))
	assert.False(t, specs[0].selector.Matches(labels.Set{"app": "db"}))
}

func TestParseCiliumPolicy_PrefixedSelectorKeys(t *testing.T) {
	obj := testCNP("prefixed", "default", map[string]any{
		"endpointSelector": map[string]any{
			"matchLabels": map[string]any{"k8s:app": "web", "any:tier": "frontend"},
		},
	}, nil)

	specs := parseCiliumPolicy(obj, "CiliumNetworkPolicy")
	require.Len(t, specs, 1)
	require.NotNil(t, specs[0].selector, "k8s:/any: prefixes must be normalized to valid selector keys")
	assert.True(t, specs[0].selector.Matches(labels.Set{"app": "web", "tier": "frontend"}))
	assert.Equal(t, map[string]string{"app": "web", "tier": "frontend"}, specs[0].info.PodSelector)
}

func TestParseCiliumPolicy_EmptySelectorMatchesAll(t *testing.T) {
	obj := testCNP("default-deny", "default", map[string]any{
		"endpointSelector": map[string]any{},
		"ingressDeny": []any{
			map[string]any{"fromEntities": []any{"world"}},
		},
	}, nil)

	specs := parseCiliumPolicy(obj, "CiliumNetworkPolicy")
	require.Len(t, specs, 1)
	require.NotNil(t, specs[0].selector)
	assert.True(t, specs[0].selector.Matches(labels.Set{"anything": "goes"}))

	info := specs[0].info
	assert.Equal(t, []string{"Ingress"}, info.PolicyTypes)
	require.Len(t, info.IngressRules, 1)
	assert.True(t, info.IngressRules[0].Deny, "ingressDeny rules must be marked deny")
	require.Len(t, info.IngressRules[0].Peers, 1)
	assert.Equal(t, "Entity", info.IngressRules[0].Peers[0].Type)
	assert.Equal(t, "world", info.IngressRules[0].Peers[0].Value)
}

func TestParseCiliumPolicy_EgressPeers(t *testing.T) {
	obj := testCNP("egress-mix", "default", map[string]any{
		"endpointSelector": map[string]any{"matchLabels": map[string]any{"app": "web"}},
		"egress": []any{
			map[string]any{"toFQDNs": []any{
				map[string]any{"matchName": "api.example.com"},
				map[string]any{"matchPattern": "*.example.org"},
			}},
			map[string]any{"toCIDRSet": []any{
				map[string]any{"cidr": "10.0.0.0/8", "except": []any{"10.0.1.0/24"}},
			}},
			map[string]any{"toCIDR": []any{"192.168.0.0/16"}},
			map[string]any{"toServices": []any{
				map[string]any{"k8sService": map[string]any{"serviceName": "db", "namespace": "data"}},
			}},
			map[string]any{"toEntities": []any{"kube-apiserver"}},
		},
	}, nil)

	specs := parseCiliumPolicy(obj, "CiliumNetworkPolicy")
	require.Len(t, specs, 1)
	info := specs[0].info
	assert.Equal(t, []string{"Egress"}, info.PolicyTypes)
	require.Len(t, info.EgressRules, 5)

	fqdns := info.EgressRules[0].Peers
	require.Len(t, fqdns, 2)
	assert.Equal(t, "FQDN", fqdns[0].Type)
	assert.Equal(t, "api.example.com", fqdns[0].Value)
	assert.Equal(t, "*.example.org", fqdns[1].Value)

	cidrSet := info.EgressRules[1].Peers
	require.Len(t, cidrSet, 1)
	assert.Equal(t, "CIDR", cidrSet[0].Type)
	assert.Equal(t, "10.0.0.0/8", cidrSet[0].CIDR)
	assert.Equal(t, []string{"10.0.1.0/24"}, cidrSet[0].Except)

	cidr := info.EgressRules[2].Peers
	require.Len(t, cidr, 1)
	assert.Equal(t, "192.168.0.0/16", cidr[0].CIDR)

	svc := info.EgressRules[3].Peers
	require.Len(t, svc, 1)
	assert.Equal(t, "Service", svc[0].Type)
	assert.Equal(t, "data/db", svc[0].Value)

	ent := info.EgressRules[4].Peers
	require.Len(t, ent, 1)
	assert.Equal(t, "Entity", ent[0].Type)
	assert.Equal(t, "kube-apiserver", ent[0].Value)
}

func TestParseCiliumPolicy_L7Summary(t *testing.T) {
	obj := testCNP("l7", "default", map[string]any{
		"endpointSelector": map[string]any{},
		"ingress": []any{
			map[string]any{
				"fromEndpoints": []any{map[string]any{}},
				"toPorts": []any{
					map[string]any{
						"ports": []any{map[string]any{"port": "80"}},
						"rules": map[string]any{
							"http": []any{map[string]any{"method": "GET", "path": "/"}},
						},
					},
				},
			},
		},
	}, nil)

	specs := parseCiliumPolicy(obj, "CiliumNetworkPolicy")
	require.Len(t, specs, 1)
	require.Len(t, specs[0].info.IngressRules, 1)
	assert.Equal(t, "HTTP", specs[0].info.IngressRules[0].L7)
}

func TestParseCiliumPolicy_EmptyRuleMatchesAll(t *testing.T) {
	obj := testCNP("allow-all-in", "default", map[string]any{
		"endpointSelector": map[string]any{},
		"ingress":          []any{map[string]any{}},
	}, nil)

	specs := parseCiliumPolicy(obj, "CiliumNetworkPolicy")
	require.Len(t, specs, 1)
	require.Len(t, specs[0].info.IngressRules, 1)
	require.Len(t, specs[0].info.IngressRules[0].Peers, 1)
	assert.Equal(t, "All", specs[0].info.IngressRules[0].Peers[0].Type)
}

func TestParseCiliumPolicy_SpecsPreferredOverSpec(t *testing.T) {
	obj := testCNP("both", "default", map[string]any{
		"endpointSelector": map[string]any{"matchLabels": map[string]any{"app": "ignored"}},
	}, map[string]any{
		"specs": []any{
			map[string]any{"endpointSelector": map[string]any{"matchLabels": map[string]any{"app": "a"}}},
		},
	})

	// spec and specs are mutually exclusive per the API; specs wins to
	// avoid displaying duplicates from a malformed object.
	specs := parseCiliumPolicy(obj, "CiliumNetworkPolicy")
	require.Len(t, specs, 1)
	assert.Equal(t, map[string]string{"app": "a"}, specs[0].info.PodSelector)
}

func TestParseCiliumPolicy_ServicePeerMissingFields(t *testing.T) {
	obj := testCNP("svc-partial", "default", map[string]any{
		"endpointSelector": map[string]any{},
		"egress": []any{
			map[string]any{"toServices": []any{
				map[string]any{"k8sService": map[string]any{"serviceName": "db"}},
			}},
		},
	}, nil)

	specs := parseCiliumPolicy(obj, "CiliumNetworkPolicy")
	require.Len(t, specs, 1)
	require.Len(t, specs[0].info.EgressRules, 1)
	peers := specs[0].info.EgressRules[0].Peers
	require.Len(t, peers, 1)
	assert.Equal(t, "(unknown)/db", peers[0].Value, "missing namespace must not render as <nil>")
}

func TestParseCiliumPolicy_FromRequiresOnlyRendersAll(t *testing.T) {
	// fromRequires is a known scope limitation: it is not rendered, so a
	// rule carrying only fromRequires displays as "All".
	obj := testCNP("requires", "default", map[string]any{
		"endpointSelector": map[string]any{},
		"ingress": []any{
			map[string]any{"fromRequires": []any{
				map[string]any{"matchLabels": map[string]any{"env": "prod"}},
			}},
		},
	}, nil)

	specs := parseCiliumPolicy(obj, "CiliumNetworkPolicy")
	require.Len(t, specs, 1)
	require.Len(t, specs[0].info.IngressRules, 1)
	require.Len(t, specs[0].info.IngressRules[0].Peers, 1)
	assert.Equal(t, "All", specs[0].info.IngressRules[0].Peers[0].Type)
}

func TestParseCiliumPolicy_MultiSpec(t *testing.T) {
	obj := testCNP("multi", "default", nil, map[string]any{
		"specs": []any{
			map[string]any{
				"endpointSelector": map[string]any{"matchLabels": map[string]any{"app": "a"}},
			},
			map[string]any{
				"endpointSelector": map[string]any{"matchLabels": map[string]any{"app": "b"}},
			},
		},
	})

	specs := parseCiliumPolicy(obj, "CiliumNetworkPolicy")
	require.Len(t, specs, 2)
	assert.Equal(t, "multi #1", specs[0].info.Name)
	assert.Equal(t, "multi #2", specs[1].info.Name)
	assert.True(t, specs[0].selector.Matches(labels.Set{"app": "a"}))
	assert.True(t, specs[1].selector.Matches(labels.Set{"app": "b"}))
}

func TestGetCiliumNetworkPolicyInfo_SingleSpec(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web"}),
		cnpObj("cnp-web", map[string]any{
			"endpointSelector": map[string]any{
				"matchLabels": map[string]any{"k8s:app": "web"},
			},
			"ingress": []any{map[string]any{}},
		}),
	)
	c := NewTestClient(nil, dyn)

	infos, err := c.GetCiliumNetworkPolicyInfo(t.Context(), "test-ctx", "default", "cnp-web", "CiliumNetworkPolicy")
	require.NoError(t, err)
	require.Len(t, infos, 1)
	assert.Equal(t, "CiliumNetworkPolicy", infos[0].Kind)
	assert.Equal(t, []string{"web-1"}, infos[0].AffectedPods)
}

func TestGetCiliumNetworkPolicyInfo_Clusterwide(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testPod("web-1", map[string]any{"app": "web"}),
		testCCNP("ccnp-all", map[string]any{
			"endpointSelector": map[string]any{},
		}),
	)
	c := NewTestClient(nil, dyn)

	infos, err := c.GetCiliumNetworkPolicyInfo(t.Context(), "test-ctx", "", "ccnp-all", "CiliumClusterwideNetworkPolicy")
	require.NoError(t, err)
	require.Len(t, infos, 1)
	assert.Equal(t, "CiliumClusterwideNetworkPolicy", infos[0].Kind)
	assert.Equal(t, []string{"web-1"}, infos[0].AffectedPods)
}

func TestGetCiliumNetworkPolicyInfo_MultiSpec(t *testing.T) {
	dyn := netpolMatchFakeDyn(
		testCNP("multi", "default", nil, map[string]any{
			"specs": []any{
				map[string]any{"endpointSelector": map[string]any{"matchLabels": map[string]any{"app": "a"}}},
				map[string]any{"endpointSelector": map[string]any{"matchLabels": map[string]any{"app": "b"}}},
			},
		}),
	)
	c := NewTestClient(nil, dyn)

	infos, err := c.GetCiliumNetworkPolicyInfo(t.Context(), "test-ctx", "default", "multi", "CiliumNetworkPolicy")
	require.NoError(t, err)
	require.Len(t, infos, 2)
}

func TestGetCiliumNetworkPolicyInfo_NotFound(t *testing.T) {
	dyn := netpolMatchFakeDyn()
	c := NewTestClient(nil, dyn)

	_, err := c.GetCiliumNetworkPolicyInfo(t.Context(), "test-ctx", "default", "missing", "CiliumNetworkPolicy")
	require.Error(t, err)
}

func TestParseCiliumPolicy_NodePolicyNotPodApplicable(t *testing.T) {
	obj := testCNP("node-policy", "", map[string]any{
		"nodeSelector": map[string]any{"matchLabels": map[string]any{"role": "worker"}},
		"ingress":      []any{map[string]any{}},
	}, nil)

	specs := parseCiliumPolicy(obj, "CiliumClusterwideNetworkPolicy")
	require.Len(t, specs, 1)
	assert.Nil(t, specs[0].selector, "node policies must not match pods")
	assert.True(t, specs[0].info.NodePolicy)
	assert.Equal(t, "CiliumClusterwideNetworkPolicy", specs[0].info.Kind)
}
