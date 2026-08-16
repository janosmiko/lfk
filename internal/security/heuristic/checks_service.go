// Service-level heuristic checks. Services are listed best-effort in Fetch:
// a Forbidden list skips these checks without hiding the pod findings.

package heuristic

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/janosmiko/lfk/internal/security"
)

func makeServiceFinding(svc *corev1.Service, check string, sev security.Severity, title, summary string) security.Finding {
	return security.Finding{
		ID:       fmt.Sprintf("heuristic/%s/Service/%s/%s", svc.Namespace, svc.Name, check),
		Source:   "heuristic",
		Category: security.CategoryMisconfig,
		Severity: sev,
		Title:    title,
		Resource: security.ResourceRef{Namespace: svc.Namespace, Kind: "Service", Name: svc.Name},
		Summary:  summary,
		Labels:   map[string]string{"check": check},
	}
}

// checkServiceExternalIPs flags Services with spec.externalIPs set. Anyone
// who can create or update such a Service can intercept traffic to that IP
// from any node (CVE-2020-8554 MITM). Most clusters never need the field.
func checkServiceExternalIPs(svc *corev1.Service) []security.Finding {
	if len(svc.Spec.ExternalIPs) == 0 {
		return nil
	}
	return []security.Finding{makeServiceFinding(svc, "service_external_ips", security.SeverityHigh,
		"Service with externalIPs",
		fmt.Sprintf("Service exposes externalIPs %s; the field enables traffic interception (CVE-2020-8554). Prefer a LoadBalancer or Ingress.", strings.Join(svc.Spec.ExternalIPs, ", ")))}
}

// serviceChecks is the ordered list of checks run against each Service.
var serviceChecks = []func(*corev1.Service) []security.Finding{
	checkServiceExternalIPs,
}
