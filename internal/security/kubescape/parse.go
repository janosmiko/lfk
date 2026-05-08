package kubescape

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/security"
)

// parseSeverity converts Kubescape's numeric severity (1-10 scale) into
// our coarser bucketed Severity. Kubescape uses 1=info ... 10=critical;
// we collapse to the four UI buckets the rest of the explorer renders.
func parseSeverity(score float64) security.Severity {
	switch {
	case score >= 9:
		return security.SeverityCritical
	case score >= 7:
		return security.SeverityHigh
	case score >= 4:
		return security.SeverityMedium
	case score > 0:
		return security.SeverityLow
	}
	return security.SeverityUnknown
}

// targetRefFromName parses the WorkloadConfigurationScan name into a
// ResourceRef. Kubescape names scans as "<kind-lower>-<workload-name>"
// (e.g., "deployment-api"); when the leading kind isn't recognised we
// fall back to using the entire name as the resource Name and leave Kind
// empty so shortResource renders "(unknown resource)" rather than
// fabricating a wrong Kind. Namespace comes from the CR itself.
func targetRefFromName(name, namespace string) security.ResourceRef {
	parts := strings.SplitN(name, "-", 2)
	if len(parts) != 2 {
		return security.ResourceRef{Namespace: namespace, Name: name}
	}
	kind, rest := parts[0], parts[1]
	// Kubescape lower-cases the kind in the scan name; restore the
	// canonical capitalisation for common workload kinds. Unknown kinds
	// fall through with the lower-cased value preserved so the row still
	// surfaces the real workload name.
	canonical := map[string]string{
		"pod":         "Pod",
		"deployment":  "Deployment",
		"statefulset": "StatefulSet",
		"daemonset":   "DaemonSet",
		"job":         "Job",
		"cronjob":     "CronJob",
		"replicaset":  "ReplicaSet",
		"service":     "Service",
		"ingress":     "Ingress",
		"configmap":   "ConfigMap",
		"secret":      "Secret",
		"node":        "Node",
		"namespace":   "Namespace",
	}
	if canon, ok := canonical[strings.ToLower(kind)]; ok {
		return security.ResourceRef{Namespace: namespace, Kind: canon, Name: rest}
	}
	return security.ResourceRef{Namespace: namespace, Name: name}
}

// parseWorkloadConfigurationScan converts every failing control on a
// WorkloadConfigurationScan into a security.Finding. Passing controls
// are skipped — security findings are deviations from policy, not
// successful checks.
func parseWorkloadConfigurationScan(u *unstructured.Unstructured) []security.Finding {
	controls, ok, _ := unstructured.NestedMap(u.Object, "spec", "controls")
	if !ok || len(controls) == 0 {
		return nil
	}
	ref := targetRefFromName(u.GetName(), u.GetNamespace())

	var findings []security.Finding
	for ctrlID, raw := range controls {
		ctrl, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		statusBlock, _ := ctrl["status"].(map[string]any)
		statusStr, _ := statusBlock["status"].(string)
		if !isFailingStatus(statusStr) {
			continue
		}
		controlName, _ := ctrl["name"].(string)
		// Severity in Kubescape lives under .severity.scoreFactor (1-10).
		var score float64
		if sev, ok := ctrl["severity"].(map[string]any); ok {
			switch v := sev["scoreFactor"].(type) {
			case float64:
				score = v
			case int64:
				score = float64(v)
			}
		}
		title := controlName
		if title == "" {
			title = ctrlID
		}
		summary, _ := statusBlock["info"].(string)

		findings = append(findings, security.Finding{
			ID:       fmt.Sprintf("kubescape/%s/%s/%s/%s", ref.Namespace, ref.Kind, ref.Name, ctrlID),
			Source:   "kubescape",
			Category: security.CategoryMisconfig,
			Severity: parseSeverity(score),
			Title:    title,
			Resource: ref,
			Summary:  summary,
			Labels: map[string]string{
				"control_id": ctrlID,
				"status":     statusStr,
			},
		})
	}
	return findings
}

// isFailingStatus reports whether a control's status string indicates a
// finding worth surfacing. Kubescape uses "passed" / "failed" / "skipped"
// / "irrelevant"; only "failed" produces a Finding.
func isFailingStatus(s string) bool {
	return strings.EqualFold(s, "failed")
}
