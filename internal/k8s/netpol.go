package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// NetworkPolicyInfo holds the parsed data from a Kubernetes NetworkPolicy resource.
type NetworkPolicyInfo struct {
	Name         string
	Namespace    string
	PodSelector  map[string]string // pod selector match labels
	PolicyTypes  []string          // "Ingress", "Egress"
	IngressRules []NetpolRule
	EgressRules  []NetpolRule
	AffectedPods []string // names of pods matching the selector
}

// NetpolRule represents a single ingress or egress rule.
type NetpolRule struct {
	Ports []NetpolPort
	Peers []NetpolPeer
}

// NetpolPort represents a port in a network policy rule.
type NetpolPort struct {
	Protocol string
	Port     string
}

// NetpolPeer represents a source/destination peer in a network policy rule.
type NetpolPeer struct {
	Type      string            // "Pod", "Namespace", "Namespace+Pod", "CIDR", "All"
	Selector  map[string]string // pod selector labels
	CIDR      string
	Except    []string
	Namespace string // namespace selector description
}

// GetNetworkPolicyInfo fetches and parses a NetworkPolicy, returning a structured
// representation suitable for visualization.
func (c *Client) GetNetworkPolicyInfo(ctx context.Context, kubeCtx, namespace, name string) (*NetworkPolicyInfo, error) {
	gvr := schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "networkpolicies",
	}

	dynClient, err := c.dynamicForContext(kubeCtx)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	obj, err := dynClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting network policy %s: %w", name, err)
	}

	info := &NetworkPolicyInfo{
		Name:      name,
		Namespace: namespace,
	}

	spec, _ := obj.Object["spec"].(map[string]interface{})
	if spec == nil {
		return info, nil
	}

	// Extract podSelector.
	if podSel, ok := spec["podSelector"].(map[string]interface{}); ok {
		if matchLabels, ok := podSel["matchLabels"].(map[string]interface{}); ok {
			info.PodSelector = make(map[string]string, len(matchLabels))
			for k, v := range matchLabels {
				info.PodSelector[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	// Extract policyTypes.
	if types, ok := spec["policyTypes"].([]interface{}); ok {
		for _, t := range types {
			info.PolicyTypes = append(info.PolicyTypes, fmt.Sprintf("%v", t))
		}
	}

	// Extract ingress rules.
	if ingress, ok := spec["ingress"].([]interface{}); ok {
		for _, rule := range ingress {
			info.IngressRules = append(info.IngressRules, parseNetpolRule(rule, "from"))
		}
	}

	// Extract egress rules.
	if egress, ok := spec["egress"].([]interface{}); ok {
		for _, rule := range egress {
			info.EgressRules = append(info.EgressRules, parseNetpolRule(rule, "to"))
		}
	}

	// Find affected pods matching the pod selector.
	info.AffectedPods = findAffectedPods(ctx, dynClient, namespace, info.PodSelector)

	return info, nil
}

// parseNetpolRule extracts ports and peers from a single ingress/egress rule.
// peerField is "from" for ingress rules and "to" for egress rules.
func parseNetpolRule(rule interface{}, peerField string) NetpolRule {
	ruleMap, ok := rule.(map[string]interface{})
	if !ok {
		return NetpolRule{}
	}

	var result NetpolRule

	// Parse ports.
	if ports, ok := ruleMap["ports"].([]interface{}); ok {
		for _, p := range ports {
			portMap, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			np := NetpolPort{
				Protocol: "TCP", // default per Kubernetes spec
			}
			if proto, ok := portMap["protocol"]; ok {
				np.Protocol = fmt.Sprintf("%v", proto)
			}
			if port, ok := portMap["port"]; ok {
				np.Port = fmt.Sprintf("%v", port)
			}
			result.Ports = append(result.Ports, np)
		}
	}

	// Parse peers (from/to).
	if peers, ok := ruleMap[peerField].([]interface{}); ok {
		for _, p := range peers {
			peerMap, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			result.Peers = append(result.Peers, parsePeer(peerMap))
		}
	}

	// If no peers specified, the rule matches all sources/destinations.
	if result.Peers == nil {
		result.Peers = []NetpolPeer{{Type: "All"}}
	}

	return result
}

// parsePeer extracts a single peer (podSelector, namespaceSelector, or ipBlock).
func parsePeer(peerMap map[string]interface{}) NetpolPeer {
	peer := NetpolPeer{}

	// Check for ipBlock (mutually exclusive with selectors).
	if ipBlock, ok := peerMap["ipBlock"].(map[string]interface{}); ok {
		peer.Type = "CIDR"
		if cidr, ok := ipBlock["cidr"]; ok {
			peer.CIDR = fmt.Sprintf("%v", cidr)
		}
		if except, ok := ipBlock["except"].([]interface{}); ok {
			for _, e := range except {
				peer.Except = append(peer.Except, fmt.Sprintf("%v", e))
			}
		}
		return peer
	}

	hasNsSel := false
	hasPodSel := false

	// Check for namespace selector.
	if nsSel, ok := peerMap["namespaceSelector"].(map[string]interface{}); ok {
		hasNsSel = true
		if matchLabels, ok := nsSel["matchLabels"].(map[string]interface{}); ok {
			nsLabels := make(map[string]string, len(matchLabels))
			for k, v := range matchLabels {
				nsLabels[k] = fmt.Sprintf("%v", v)
			}
			peer.Namespace = formatLabels(nsLabels)
		} else {
			peer.Namespace = "(all namespaces)"
		}
	}

	// Check for pod selector.
	if podSel, ok := peerMap["podSelector"].(map[string]interface{}); ok {
		hasPodSel = true
		if matchLabels, ok := podSel["matchLabels"].(map[string]interface{}); ok {
			peer.Selector = make(map[string]string, len(matchLabels))
			for k, v := range matchLabels {
				peer.Selector[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	// Determine the type based on which selectors were present.
	switch {
	case hasNsSel && hasPodSel:
		peer.Type = "Namespace+Pod"
	case hasNsSel:
		peer.Type = "Namespace"
	case hasPodSel:
		peer.Type = "Pod"
	default:
		peer.Type = "All"
	}

	return peer
}

// formatLabels renders a label map as "key=value, ..." sorted by key.
func formatLabels(lbls map[string]string) string {
	if len(lbls) == 0 {
		return "(all)"
	}
	keys := make([]string, 0, len(lbls))
	for k := range lbls {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+lbls[k])
	}
	return strings.Join(parts, ", ")
}

// findAffectedPods lists pod names in the namespace that match the given selector labels.
// Returns nil on error (best-effort; the visualization still works without this).
func findAffectedPods(ctx context.Context, dynClient dynamic.Interface, namespace string, selector map[string]string) []string {
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}

	listOpts := metav1.ListOptions{}
	if len(selector) > 0 {
		listOpts.LabelSelector = labels.Set(selector).String()
	}

	podList, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, listOpts)
	if err != nil {
		return nil
	}

	var names []string
	for _, pod := range podList.Items {
		names = append(names, pod.GetName())
	}
	sort.Strings(names)
	return names
}
