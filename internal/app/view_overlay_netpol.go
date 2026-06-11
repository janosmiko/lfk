package app

import (
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// renderOverlayNetworkPolicy renders the network policy visualizer: the
// single-policy view when netpolData is set (Visualize on a NetworkPolicy),
// or the multi-policy view when netpolsData is set (Network Policies on a
// Pod/Service).
// netpolOverlayDims returns the overlay frame width and its inner content
// area. The key handler derives scroll bounds from the same numbers the
// renderer draws with.
func (m Model) netpolOverlayDims() (w, innerW, innerH int) {
	w, h := min(100, m.width-6), min(35, m.height-4)
	return w, w - 4, h - 2
}

// netpolMaxScroll returns the bottom scroll position for the currently
// loaded netpol overlay content; 0 when nothing is loaded.
func (m Model) netpolMaxScroll() int {
	_, innerW, innerH := m.netpolOverlayDims()
	switch {
	case m.netpolsData != nil:
		return ui.OverlayMaxScroll(ui.NetworkPoliciesOverlayLineCount(convertNetpolsForResource(m.netpolsData), innerW), innerH)
	case m.netpolData != nil:
		return ui.OverlayMaxScroll(ui.NetworkPolicyOverlayLineCount(convertNetpolInfo(*m.netpolData), innerW), innerH)
	}
	return 0
}

func (m Model) renderOverlayNetworkPolicy(background string) string {
	if m.netpolData == nil && m.netpolsData == nil {
		return ""
	}
	w, innerW, innerH := m.netpolOverlayDims()
	var netpolContent string
	if m.netpolsData != nil {
		netpolContent = ui.RenderNetworkPoliciesOverlay(convertNetpolsForResource(m.netpolsData), m.netpolScroll, innerW, innerH)
	} else {
		netpolContent = ui.RenderNetworkPolicyOverlay(convertNetpolInfo(*m.netpolData), m.netpolScroll, innerW, innerH)
	}
	netpolContent = ui.FillLinesBg(netpolContent, innerW, ui.SurfaceBg)
	overlay := ui.OverlayStyle.Width(w).Render(netpolContent)
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlay(m.width, m.height, overlay, bg)
}

// convertNetpolInfo converts a k8s network policy info into its ui entry.
func convertNetpolInfo(info k8s.NetworkPolicyInfo) ui.NetworkPolicyEntry {
	entry := ui.NetworkPolicyEntry{
		Name: info.Name, Namespace: info.Namespace,
		Kind: info.Kind, NodePolicy: info.NodePolicy,
		PodSelector: info.PodSelector, PolicyTypes: info.PolicyTypes,
		AffectedPods: info.AffectedPods,
	}
	for _, r := range info.IngressRules {
		entry.IngressRules = append(entry.IngressRules, convertNetpolRule(r))
	}
	for _, r := range info.EgressRules {
		entry.EgressRules = append(entry.EgressRules, convertNetpolRule(r))
	}
	return entry
}

// convertNetpolsForResource converts the multi-policy k8s result into its ui entry.
func convertNetpolsForResource(info *k8s.NetpolsForResource) ui.ResourceNetpolsEntry {
	entry := ui.ResourceNetpolsEntry{
		Kind:        info.Kind,
		Name:        info.Name,
		Namespace:   info.Namespace,
		NoSelector:  info.NoSelector,
		BackingPods: info.BackingPods,
	}
	for _, pol := range info.Policies {
		polEntry := convertNetpolInfo(pol.NetworkPolicyInfo)
		polEntry.MatchedPods = pol.MatchedPods
		entry.Policies = append(entry.Policies, polEntry)
	}
	return entry
}

// convertNetpolRule converts a k8s.NetpolRule to a ui.NetpolRuleEntry.
func convertNetpolRule(r k8s.NetpolRule) ui.NetpolRuleEntry {
	re := ui.NetpolRuleEntry{Deny: r.Deny, L7: r.L7}
	for _, p := range r.Ports {
		re.Ports = append(re.Ports, ui.NetpolPortEntry{Protocol: p.Protocol, Port: p.Port})
	}
	for _, p := range r.Peers {
		re.Peers = append(re.Peers, ui.NetpolPeerEntry{
			Type: p.Type, Selector: p.Selector,
			CIDR: p.CIDR, Except: p.Except, Namespace: p.Namespace,
			Value: p.Value,
		})
	}
	return re
}
