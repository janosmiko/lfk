package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderNetworkPolicyOverlay_CiliumKindShown(t *testing.T) {
	info := NetworkPolicyEntry{
		Name:        "cnp-web",
		Namespace:   "default",
		Kind:        "CiliumNetworkPolicy",
		PolicyTypes: []string{"Ingress"},
	}
	out := RenderNetworkPolicyOverlay(info, 0, 100, 40)
	assert.Contains(t, out, "CiliumNetworkPolicy: cnp-web")
}

func TestRenderNetworkPolicyOverlay_ClusterwideNamespace(t *testing.T) {
	info := NetworkPolicyEntry{
		Name: "ccnp-all",
		Kind: "CiliumClusterwideNetworkPolicy",
	}
	out := RenderNetworkPolicyOverlay(info, 0, 100, 40)
	assert.Contains(t, out, "(cluster-wide)")
	// Empty selector on a clusterwide policy spans every namespace.
	assert.Contains(t, out, "(all pods in all namespaces)")
}

func TestRenderNetworkPolicyOverlay_DenyRuleMarked(t *testing.T) {
	info := NetworkPolicyEntry{
		Name:        "deny-world",
		Namespace:   "default",
		Kind:        "CiliumNetworkPolicy",
		PolicyTypes: []string{"Ingress"},
		IngressRules: []NetpolRuleEntry{
			{Deny: true, Peers: []NetpolPeerEntry{{Type: "Entity", Value: "world"}}},
		},
	}
	out := RenderNetworkPolicyOverlay(info, 0, 100, 40)
	assert.Contains(t, out, "Rule 1 (deny):")
}

func TestRenderNetworkPolicyOverlay_L7Shown(t *testing.T) {
	info := NetworkPolicyEntry{
		Name:        "l7",
		Namespace:   "default",
		Kind:        "CiliumNetworkPolicy",
		PolicyTypes: []string{"Ingress"},
		IngressRules: []NetpolRuleEntry{
			{L7: "HTTP", Peers: []NetpolPeerEntry{{Type: "All"}}},
		},
	}
	out := RenderNetworkPolicyOverlay(info, 0, 100, 40)
	assert.Contains(t, out, "L7: HTTP")
}

func TestRenderNetworkPolicyOverlay_CiliumPeerTypes(t *testing.T) {
	info := NetworkPolicyEntry{
		Name:        "peers",
		Namespace:   "default",
		Kind:        "CiliumNetworkPolicy",
		PolicyTypes: []string{"Egress"},
		EgressRules: []NetpolRuleEntry{
			{Peers: []NetpolPeerEntry{{Type: "Entity", Value: "kube-apiserver"}}},
			{Peers: []NetpolPeerEntry{{Type: "FQDN", Value: "api.example.com"}}},
			{Peers: []NetpolPeerEntry{{Type: "Service", Value: "data/db"}}},
		},
	}
	out := RenderNetworkPolicyOverlay(info, 0, 100, 60)
	assert.Contains(t, out, "Entity:")
	assert.Contains(t, out, "kube-apiserver")
	assert.Contains(t, out, "FQDN:")
	assert.Contains(t, out, "api.example.com")
	assert.Contains(t, out, "Service:")
	assert.Contains(t, out, "data/db")
}

func TestRenderNetworkPolicyOverlay_NodePolicyNote(t *testing.T) {
	info := NetworkPolicyEntry{
		Name:       "node-policy",
		Kind:       "CiliumClusterwideNetworkPolicy",
		NodePolicy: true,
	}
	out := RenderNetworkPolicyOverlay(info, 0, 100, 40)
	assert.Contains(t, out, "selects nodes, not pods")
}

func TestRenderNetworkPoliciesOverlay_CiliumSpecsSummary(t *testing.T) {
	info := ResourceNetpolsEntry{
		Kind:      "CiliumNetworkPolicy",
		Name:      "multi",
		Namespace: "default",
		Policies: []NetworkPolicyEntry{
			{Name: "multi #1", Namespace: "default", Kind: "CiliumNetworkPolicy"},
			{Name: "multi #2", Namespace: "default", Kind: "CiliumNetworkPolicy"},
		},
	}
	out := RenderNetworkPoliciesOverlay(info, 0, 100, 60)
	assert.Contains(t, out, "2 policy specs")
	assert.NotContains(t, out, "select this ciliumnetworkpolicy")
}
