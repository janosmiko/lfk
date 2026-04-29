package trivyop

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/security"
)

// parseSeverity converts a Trivy severity string to our scale.
func parseSeverity(s string) security.Severity {
	switch strings.ToUpper(s) {
	case "CRITICAL":
		return security.SeverityCritical
	case "HIGH":
		return security.SeverityHigh
	case "MEDIUM":
		return security.SeverityMedium
	case "LOW":
		return security.SeverityLow
	}
	return security.SeverityUnknown
}

// extractResourceRef reads the Trivy Operator labels that encode the owning workload.
func extractResourceRef(u *unstructured.Unstructured) security.ResourceRef {
	labels := u.GetLabels()
	return security.ResourceRef{
		Namespace: u.GetNamespace(),
		Kind:      labels["trivy-operator.resource.kind"],
		Name:      labels["trivy-operator.resource.name"],
		Container: labels["trivy-operator.container.name"],
	}
}

// parseVulnerabilityReport extracts findings from a single VulnerabilityReport.
// Malformed or missing fields are skipped silently (never panic).
func parseVulnerabilityReport(u *unstructured.Unstructured) []security.Finding {
	ref := extractResourceRef(u)
	vulns, ok, _ := unstructured.NestedSlice(u.Object, "report", "vulnerabilities")
	if !ok || len(vulns) == 0 {
		return nil
	}
	findings := make([]security.Finding, 0, len(vulns))
	for _, v := range vulns {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		cve, _ := m["vulnerabilityID"].(string)
		sev, _ := m["severity"].(string)
		pkg, _ := m["resource"].(string)
		installed, _ := m["installedVersion"].(string)
		fixed, _ := m["fixedVersion"].(string)
		title, _ := m["title"].(string)
		desc, _ := m["description"].(string)
		link, _ := m["primaryLink"].(string)

		if cve == "" {
			continue
		}
		summary := fmt.Sprintf("%s %s (installed: %s)", cve, pkg, installed)
		if title == "" {
			title = cve
		}
		details := desc
		if fixed != "" {
			details = fmt.Sprintf("Fixed in %s\n\n%s", fixed, details)
		}

		f := security.Finding{
			ID:       fmt.Sprintf("trivy-operator/%s/%s/%s/%s", ref.Namespace, ref.Kind, ref.Name, cve),
			Source:   "trivy-operator",
			Category: security.CategoryVuln,
			Severity: parseSeverity(sev),
			Title:    title,
			Resource: ref,
			Summary:  summary,
			Details:  details,
			Labels: map[string]string{
				"cve":           cve,
				"package":       pkg,
				"installed":     installed,
				"fixed_version": fixed,
			},
		}
		if link != "" {
			f.References = []string{link}
		}
		findings = append(findings, f)
	}
	return findings
}

// parseConfigAuditReport extracts failing misconfig checks from a ConfigAuditReport.
func parseConfigAuditReport(u *unstructured.Unstructured) []security.Finding {
	ref := extractResourceRef(u)
	checks, ok, _ := unstructured.NestedSlice(u.Object, "report", "checks")
	if !ok || len(checks) == 0 {
		return nil
	}
	var findings []security.Finding
	for _, c := range checks {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		passed, _ := m["success"].(bool)
		if passed {
			continue
		}
		checkID, _ := m["checkID"].(string)
		sev, _ := m["severity"].(string)
		title, _ := m["title"].(string)
		desc, _ := m["description"].(string)
		if checkID == "" {
			continue
		}
		findings = append(findings, security.Finding{
			ID:       fmt.Sprintf("trivy-operator/%s/%s/%s/%s", ref.Namespace, ref.Kind, ref.Name, checkID),
			Source:   "trivy-operator",
			Category: security.CategoryMisconfig,
			Severity: parseSeverity(sev),
			Title:    title,
			Resource: ref,
			Summary:  title,
			Details:  desc,
			Labels:   map[string]string{"check_id": checkID},
		})
	}
	return findings
}
