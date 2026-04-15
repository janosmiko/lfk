package policyreport

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/security"
)

// parseSeverity converts a Policy Reports API severity string to our scale.
// The wgpolicyk8s.io spec uses: critical, high, medium, low, info.
func parseSeverity(s string) security.Severity {
	switch strings.ToLower(s) {
	case "critical":
		return security.SeverityCritical
	case "high":
		return security.SeverityHigh
	case "medium":
		return security.SeverityMedium
	case "low", "info":
		return security.SeverityLow
	}
	return security.SeverityUnknown
}

// parseResult converts a Policy Reports API result status to whether it's
// a finding. Only "fail" and "error" results are treated as findings.
func isFailingResult(status string) bool {
	switch strings.ToLower(status) {
	case "fail", "error":
		return true
	}
	return false
}

// extractResourceRef builds a ResourceRef from a result or scope object.
// Tries the result's resources[0] first, then falls back to the
// report-level scope (Kyverno sets scope on the report, not per-result).
func extractResourceRef(result map[string]interface{}, scope security.ResourceRef) security.ResourceRef {
	resources, ok := result["resources"].([]interface{})
	if ok && len(resources) > 0 {
		if res, ok := resources[0].(map[string]interface{}); ok {
			ref := refFromMap(res, scope.Namespace)
			if ref.Kind != "" && ref.Name != "" {
				return ref
			}
		}
	}
	// Fall back to report-level scope.
	return scope
}

// extractScope reads the report-level scope field (used by Kyverno to
// identify the resource the entire report applies to).
func extractScope(u *unstructured.Unstructured) security.ResourceRef {
	scopeMap, ok, _ := unstructured.NestedMap(u.Object, "scope")
	if !ok || len(scopeMap) == 0 {
		return security.ResourceRef{Namespace: u.GetNamespace()}
	}
	return refFromMap(scopeMap, u.GetNamespace())
}

// refFromMap extracts a ResourceRef from a map with kind/name/namespace keys.
func refFromMap(m map[string]interface{}, fallbackNs string) security.ResourceRef {
	kind, _ := m["kind"].(string)
	name, _ := m["name"].(string)
	ns, _ := m["namespace"].(string)
	if ns == "" {
		ns = fallbackNs
	}
	return security.ResourceRef{
		Namespace: ns,
		Kind:      kind,
		Name:      name,
	}
}

// parsePolicyReport extracts failing results from a PolicyReport or
// ClusterPolicyReport. Passing results are skipped.
func parsePolicyReport(u *unstructured.Unstructured) []security.Finding {
	results, ok, _ := unstructured.NestedSlice(u.Object, "results")
	if !ok || len(results) == 0 {
		return nil
	}

	// Report-level scope: Kyverno sets this to identify the resource the
	// entire report applies to. Used as fallback when per-result resources
	// are not set.
	scope := extractScope(u)
	var findings []security.Finding

	for _, r := range results {
		m, ok := r.(map[string]interface{})
		if !ok {
			continue
		}

		status, _ := m["result"].(string)
		if !isFailingResult(status) {
			continue
		}

		policy, _ := m["policy"].(string)
		rule, _ := m["rule"].(string)
		severity, _ := m["severity"].(string)
		message, _ := m["message"].(string)
		category, _ := m["category"].(string)

		if policy == "" {
			continue
		}

		ref := extractResourceRef(m, scope)

		// Build a stable finding ID.
		id := fmt.Sprintf("policy-report/%s/%s/%s/%s/%s",
			ref.Namespace, ref.Kind, ref.Name, policy, rule)

		// Title is the rule name; falls back to policy name.
		title := rule
		if title == "" {
			title = policy
		}

		// Map the category string to our security category.
		cat := security.CategoryPolicy
		switch strings.ToLower(category) {
		case "compliance", "best practices", "best-practices":
			cat = security.CategoryCompliance
		}

		f := security.Finding{
			ID:       id,
			Source:   "policy-report",
			Category: cat,
			Severity: parseSeverity(severity),
			Title:    title,
			Resource: ref,
			Summary:  message,
			Labels: map[string]string{
				"policy":   policy,
				"rule":     rule,
				"category": category,
				"status":   status,
			},
		}
		findings = append(findings, f)
	}

	return findings
}
