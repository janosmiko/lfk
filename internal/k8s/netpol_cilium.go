package k8s

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ciliumParsedSpec is one spec of a Cilium policy: its display info plus the
// pod-matching selector. A nil selector means the spec does not apply to
// pods (node policy, or an unparsable selector).
type ciliumParsedSpec struct {
	selector labels.Selector
	info     NetworkPolicyInfo
}

// parseCiliumPolicy parses a CiliumNetworkPolicy or
// CiliumClusterwideNetworkPolicy into one entry per spec (Cilium allows a
// single `spec` or a `specs` list). kind is recorded on each entry.
func parseCiliumPolicy(obj *unstructured.Unstructured, kind string) []ciliumParsedSpec {
	// spec and specs are mutually exclusive per the Cilium API; prefer
	// specs when both are present rather than displaying duplicates.
	var rawSpecs []map[string]any
	if specs, ok := obj.Object["specs"].([]any); ok && len(specs) > 0 {
		for _, s := range specs {
			if spec, ok := s.(map[string]any); ok {
				rawSpecs = append(rawSpecs, spec)
			}
		}
	} else if spec, ok := obj.Object["spec"].(map[string]any); ok {
		rawSpecs = append(rawSpecs, spec)
	}

	name := obj.GetName()
	parsed := make([]ciliumParsedSpec, 0, len(rawSpecs))
	for i, spec := range rawSpecs {
		entryName := name
		if len(rawSpecs) > 1 {
			entryName = fmt.Sprintf("%s #%d", name, i+1)
		}
		parsed = append(parsed, parseCiliumSpec(spec, entryName, obj.GetNamespace(), kind))
	}
	return parsed
}

// parseCiliumSpec parses a single Cilium policy spec.
func parseCiliumSpec(spec map[string]any, name, namespace, kind string) ciliumParsedSpec {
	info := NetworkPolicyInfo{
		Name:      name,
		Namespace: namespace,
		Kind:      kind,
	}

	var sel labels.Selector
	if _, hasNode := spec["nodeSelector"]; hasNode {
		// Node policies select nodes, not pods; they never match in the
		// pod/service reverse lookup.
		info.NodePolicy = true
	} else {
		endpointSel, _ := spec["endpointSelector"].(map[string]any)
		info.PodSelector = ciliumSelectorLabels(endpointSel)
		sel = ciliumSelector(endpointSel)
	}

	parseDirection := func(field string, deny bool, dst *[]NetpolRule, peerPrefix string) {
		rules, ok := spec[field].([]any)
		if !ok {
			return
		}
		for _, r := range rules {
			ruleMap, ok := r.(map[string]any)
			if !ok {
				continue
			}
			rule := parseCiliumRule(ruleMap, peerPrefix)
			rule.Deny = deny
			*dst = append(*dst, rule)
		}
	}
	parseDirection("ingress", false, &info.IngressRules, "from")
	parseDirection("ingressDeny", true, &info.IngressRules, "from")
	parseDirection("egress", false, &info.EgressRules, "to")
	parseDirection("egressDeny", true, &info.EgressRules, "to")

	if len(info.IngressRules) > 0 {
		info.PolicyTypes = append(info.PolicyTypes, "Ingress")
	}
	if len(info.EgressRules) > 0 {
		info.PolicyTypes = append(info.PolicyTypes, "Egress")
	}

	return ciliumParsedSpec{selector: sel, info: info}
}

// parseCiliumRule parses one ingress/egress rule. peerPrefix is "from" for
// ingress and "to" for egress, matching Cilium's field naming
// (fromEndpoints/toEndpoints, fromCIDR/toCIDR, ...). toFQDNs and toServices
// only ever appear on egress rules. Known scope limitation: fromRequires /
// toRequires (additive AND constraints) are not rendered — a rule using only
// those displays as "All".
func parseCiliumRule(ruleMap map[string]any, peerPrefix string) NetpolRule {
	var rule NetpolRule

	if endpoints, ok := ruleMap[peerPrefix+"Endpoints"].([]any); ok {
		for _, e := range endpoints {
			selMap, ok := e.(map[string]any)
			if !ok {
				continue
			}
			rule.Peers = append(rule.Peers, NetpolPeer{
				Type:     "Pod",
				Selector: ciliumSelectorLabels(selMap),
			})
		}
	}
	if cidrs, ok := ruleMap[peerPrefix+"CIDR"].([]any); ok {
		for _, c := range cidrs {
			rule.Peers = append(rule.Peers, NetpolPeer{Type: "CIDR", CIDR: fmt.Sprintf("%v", c)})
		}
	}
	if cidrSets, ok := ruleMap[peerPrefix+"CIDRSet"].([]any); ok {
		for _, c := range cidrSets {
			setMap, ok := c.(map[string]any)
			if !ok {
				continue
			}
			peer := NetpolPeer{Type: "CIDR"}
			if cidr, ok := setMap["cidr"]; ok {
				peer.CIDR = fmt.Sprintf("%v", cidr)
			}
			if except, ok := setMap["except"].([]any); ok {
				for _, e := range except {
					peer.Except = append(peer.Except, fmt.Sprintf("%v", e))
				}
			}
			rule.Peers = append(rule.Peers, peer)
		}
	}
	if entities, ok := ruleMap[peerPrefix+"Entities"].([]any); ok {
		for _, e := range entities {
			rule.Peers = append(rule.Peers, NetpolPeer{Type: "Entity", Value: fmt.Sprintf("%v", e)})
		}
	}
	if fqdns, ok := ruleMap["toFQDNs"].([]any); ok {
		for _, f := range fqdns {
			fqdnMap, ok := f.(map[string]any)
			if !ok {
				continue
			}
			value := ""
			if v, ok := fqdnMap["matchName"]; ok {
				value = fmt.Sprintf("%v", v)
			} else if v, ok := fqdnMap["matchPattern"]; ok {
				value = fmt.Sprintf("%v", v)
			}
			rule.Peers = append(rule.Peers, NetpolPeer{Type: "FQDN", Value: value})
		}
	}
	if services, ok := ruleMap["toServices"].([]any); ok {
		for _, s := range services {
			svcMap, ok := s.(map[string]any)
			if !ok {
				continue
			}
			if k8sSvc, ok := svcMap["k8sService"].(map[string]any); ok {
				ns, _ := k8sSvc["namespace"].(string)
				svcName, _ := k8sSvc["serviceName"].(string)
				if ns == "" {
					ns = "(unknown)"
				}
				if svcName == "" {
					svcName = "(unknown)"
				}
				rule.Peers = append(rule.Peers, NetpolPeer{Type: "Service", Value: ns + "/" + svcName})
			}
		}
	}

	rule.Ports, rule.L7 = parseCiliumToPorts(ruleMap)

	// A rule with no peers allows (or denies) all sources/destinations.
	if rule.Peers == nil {
		rule.Peers = []NetpolPeer{{Type: "All"}}
	}
	return rule
}

// parseCiliumToPorts extracts ports and an L7 protocol summary from a rule's
// toPorts list.
func parseCiliumToPorts(ruleMap map[string]any) ([]NetpolPort, string) {
	toPorts, ok := ruleMap["toPorts"].([]any)
	if !ok {
		return nil, ""
	}

	var ports []NetpolPort
	l7Set := map[string]bool{}
	for _, tp := range toPorts {
		tpMap, ok := tp.(map[string]any)
		if !ok {
			continue
		}
		if portList, ok := tpMap["ports"].([]any); ok {
			for _, p := range portList {
				portMap, ok := p.(map[string]any)
				if !ok {
					continue
				}
				np := NetpolPort{Protocol: "TCP"} // Cilium default
				if proto, ok := portMap["protocol"]; ok {
					np.Protocol = fmt.Sprintf("%v", proto)
				}
				if port, ok := portMap["port"]; ok {
					np.Port = fmt.Sprintf("%v", port)
				}
				ports = append(ports, np)
			}
		}
		if rules, ok := tpMap["rules"].(map[string]any); ok {
			for proto := range rules {
				switch proto {
				case "http":
					l7Set["HTTP"] = true
				case "dns":
					l7Set["DNS"] = true
				case "kafka":
					l7Set["Kafka"] = true
				default:
					l7Set[proto] = true
				}
			}
		}
	}

	l7 := make([]string, 0, len(l7Set))
	for p := range l7Set {
		l7 = append(l7, p)
	}
	sort.Strings(l7)
	return ports, strings.Join(l7, ", ")
}

// ciliumSelectorLabels extracts matchLabels from a Cilium endpoint selector
// for display, with source prefixes (k8s:, any:) stripped.
func ciliumSelectorLabels(sel map[string]any) map[string]string {
	matchLabels, ok := sel["matchLabels"].(map[string]any)
	if !ok || len(matchLabels) == 0 {
		return nil
	}
	out := make(map[string]string, len(matchLabels))
	for k, v := range matchLabels {
		out[stripCiliumPrefix(k)] = fmt.Sprintf("%v", v)
	}
	return out
}

// ciliumSelector builds a pod-matching label selector from a Cilium endpoint
// selector. Source prefixes (k8s:, any:) are stripped: they are not valid
// Kubernetes label-key characters and pods carry the unprefixed labels.
// A nil or empty selector matches all pods. Returns nil when the selector
// cannot be parsed.
func ciliumSelector(sel map[string]any) labels.Selector {
	if len(sel) == 0 {
		return labels.Everything()
	}

	normalized := map[string]any{}
	if matchLabels, ok := sel["matchLabels"].(map[string]any); ok {
		ml := make(map[string]any, len(matchLabels))
		for k, v := range matchLabels {
			ml[stripCiliumPrefix(k)] = v
		}
		normalized["matchLabels"] = ml
	}
	if matchExprs, ok := sel["matchExpressions"].([]any); ok {
		exprs := make([]any, 0, len(matchExprs))
		for _, e := range matchExprs {
			exprMap, ok := e.(map[string]any)
			if !ok {
				continue
			}
			cloned := make(map[string]any, len(exprMap))
			maps.Copy(cloned, exprMap)
			if key, ok := cloned["key"].(string); ok {
				cloned["key"] = stripCiliumPrefix(key)
			}
			exprs = append(exprs, cloned)
		}
		normalized["matchExpressions"] = exprs
	}

	var ls metav1.LabelSelector
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(normalized, &ls); err != nil {
		return nil
	}
	sel2, err := metav1.LabelSelectorAsSelector(&ls)
	if err != nil {
		return nil
	}
	return sel2
}

// stripCiliumPrefix removes a Cilium label source prefix ("k8s:", "any:")
// from a selector key.
func stripCiliumPrefix(key string) string {
	for _, prefix := range []string{"k8s:", "any:"} {
		if rest, ok := strings.CutPrefix(key, prefix); ok {
			return rest
		}
	}
	return key
}

// GetCiliumNetworkPolicyInfo fetches a CiliumNetworkPolicy or
// CiliumClusterwideNetworkPolicy and parses it for visualization. Returns
// one entry per spec (Cilium policies may carry a `specs` list).
func (c *Client) GetCiliumNetworkPolicyInfo(ctx context.Context, kubeCtx, namespace, name, kind string) ([]NetworkPolicyInfo, error) {
	dynClient, err := c.dynamicForContext(kubeCtx)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	var obj *unstructured.Unstructured
	clusterwide := kind == "CiliumClusterwideNetworkPolicy"
	if clusterwide {
		obj, err = dynClient.Resource(ciliumCCNPGVR).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = dynClient.Resource(ciliumCNPGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("getting %s %s: %w", kind, name, err)
	}

	// Affected pods are best-effort display data: namespace-scoped for a
	// CNP; across all namespaces for a clusterwide policy.
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podIface := dynClient.Resource(podGVR).Namespace(namespace)
	if clusterwide {
		podIface = dynClient.Resource(podGVR)
	}
	var pods []unstructured.Unstructured
	if podList, err := podIface.List(ctx, metav1.ListOptions{}); err == nil {
		pods = podList.Items
	}

	parsed := parseCiliumPolicy(obj, kind)
	infos := make([]NetworkPolicyInfo, 0, len(parsed))
	for _, ps := range parsed {
		if ps.selector != nil {
			ps.info.AffectedPods = matchingCiliumPodNames(pods, ps.selector)
		}
		infos = append(infos, ps.info)
	}
	return infos, nil
}
