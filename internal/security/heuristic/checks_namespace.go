// Namespace-level heuristic checks: missing Pod Security Admission labels
// and namespaces whose pods run with no NetworkPolicy at all. Both lists are
// best-effort — a Forbidden list skips the checks without hiding the pod
// findings.

package heuristic

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/security"
)

// systemNamespaces are skipped by the namespace-level checks: kube-system
// requires privileged pods (no PSA enforce label fits) and securing it is
// the distribution's job, not the user's.
var systemNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
}

// psaEnforceLabel is the official Pod Security Admission enforcement label.
const psaEnforceLabel = "pod-security.kubernetes.io/enforce"

func makeNamespaceFinding(ns, check string, sev security.Severity, title, summary string) security.Finding {
	return security.Finding{
		ID:       fmt.Sprintf("heuristic/%s/Namespace/%s/%s", ns, ns, check),
		Source:   "heuristic",
		Category: security.CategoryMisconfig,
		Severity: sev,
		Title:    title,
		Resource: security.ResourceRef{Namespace: ns, Kind: "Namespace", Name: ns},
		Summary:  summary,
		Labels:   map[string]string{"check": check},
	}
}

// fetchNamespaceFindings lists namespaces and NetworkPolicies (best-effort,
// independently) and flags namespaces without a PSA enforce label, plus
// namespaces that contain pods but no NetworkPolicy.
func (s *Source) fetchNamespaceFindings(ctx context.Context, namespace string, nsWithPods map[string]bool) []security.Finding {
	var out []security.Finding

	namespaces, nsOK := security.Collect(func(o metav1.ListOptions) ([]corev1.Namespace, string, error) {
		if namespace != "" {
			// Single-namespace fetch: get just that namespace.
			ns, err := s.client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
			if err != nil {
				return nil, "", err
			}
			return []corev1.Namespace{*ns}, "", nil
		}
		l, err := s.client.CoreV1().Namespaces().List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	if nsOK {
		for i := range namespaces {
			ns := &namespaces[i]
			if systemNamespaces[ns.Name] || s.ignoredNamespaces[ns.Name] {
				continue
			}
			if ns.Labels[psaEnforceLabel] == "" {
				out = append(out, makeNamespaceFinding(ns.Name, "psa_labels_missing", security.SeverityMedium,
					"no Pod Security enforcement",
					fmt.Sprintf("Namespace %q has no %s label; nothing prevents privileged pods from being admitted.", ns.Name, psaEnforceLabel)))
			}
		}
	}

	netpols, npOK := security.Collect(func(o metav1.ListOptions) ([]networkingv1.NetworkPolicy, string, error) {
		l, err := s.client.NetworkingV1().NetworkPolicies(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	if npOK {
		nsWithPolicy := make(map[string]bool, len(netpols))
		for i := range netpols {
			nsWithPolicy[netpols[i].Namespace] = true
		}
		for ns := range nsWithPods {
			// ignoredNamespaces is re-checked explicitly even though the pod
			// scan never records ignored namespaces — the guard must not
			// depend on the caller's filtering.
			if systemNamespaces[ns] || s.ignoredNamespaces[ns] || nsWithPolicy[ns] {
				continue
			}
			out = append(out, makeNamespaceFinding(ns, "namespace_no_netpol", security.SeverityMedium,
				"no NetworkPolicy",
				fmt.Sprintf("Namespace %q runs pods with no NetworkPolicy; all pod-to-pod traffic in and out of the namespace is allowed.", ns)))
		}
	}
	return out
}
