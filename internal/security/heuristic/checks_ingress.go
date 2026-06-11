// Ingress-level heuristic checks: missing TLS and catch-all empty hosts.
// The list is best-effort like the other non-pod scans.

package heuristic

import (
	"context"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/security"
)

func makeIngressFinding(ns, name, check string, sev security.Severity, title, summary string) security.Finding {
	return security.Finding{
		ID:       fmt.Sprintf("heuristic/%s/Ingress/%s/%s", ns, name, check),
		Source:   "heuristic",
		Category: security.CategoryMisconfig,
		Severity: sev,
		Title:    title,
		Resource: security.ResourceRef{Namespace: ns, Kind: "Ingress", Name: name},
		Summary:  summary,
		Labels:   map[string]string{"check": check},
	}
}

// fetchIngressFindings lists Ingresses (best-effort) and flags those without
// a TLS block and rules with an empty host.
func (s *Source) fetchIngressFindings(ctx context.Context, namespace string) []security.Finding {
	ingresses, ok := security.Collect(func(o metav1.ListOptions) ([]networkingv1.Ingress, string, error) {
		l, err := s.client.NetworkingV1().Ingresses(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	if !ok {
		return nil
	}
	var out []security.Finding
	for i := range ingresses {
		ing := &ingresses[i]
		if s.ignoredNamespaces[ing.Namespace] {
			continue
		}
		if len(ing.Spec.TLS) == 0 {
			out = append(out, makeIngressFinding(ing.Namespace, ing.Name, "ingress_no_tls", security.SeverityMedium,
				"Ingress without TLS",
				fmt.Sprintf("Ingress %q has no tls section; traffic through this entry point is unencrypted. Terminate TLS here or document where it happens.", ing.Name)))
		}
		for j := range ing.Spec.Rules {
			if ing.Spec.Rules[j].Host == "" {
				out = append(out, makeIngressFinding(ing.Namespace, ing.Name, "ingress_empty_host", security.SeverityMedium,
					"Ingress rule with empty host",
					fmt.Sprintf("Ingress %q has a rule with an empty host, matching every hostname; it bypasses virtual-host isolation and can shadow other Ingresses.", ing.Name)))
				break // one finding per Ingress is enough
			}
		}
	}
	return out
}
