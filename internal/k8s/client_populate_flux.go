package k8s

import (
	"fmt"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

func populateFluxCDResource(ti *model.Item, obj map[string]any, status map[string]any, kind string) {
	spec, _ := obj["spec"].(map[string]any)
	if spec != nil {
		if suspended, ok := spec["suspend"].(bool); ok && suspended {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Suspended", Value: "True"})
		}
		if interval, ok := spec["interval"].(string); ok && interval != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Interval", Value: interval})
		}
		populateFluxKindColumns(ti, spec, kind)
	}
	if status == nil {
		return
	}
	if conditions, ok := status["conditions"].([]any); ok {
		if !extractReadyCondition(ti, conditions) && len(conditions) > 0 {
			extractGenericConditions(ti, conditions)
		}
	}
	populateFluxRevision(ti, status)
}

func populateFluxKindColumns(ti *model.Item, spec map[string]any, kind string) {
	switch kind {
	case "Kustomization":
		if sourceRef, ok := spec["sourceRef"].(map[string]any); ok {
			refKind, _ := sourceRef["kind"].(string)
			refName, _ := sourceRef["name"].(string)
			if refKind != "" && refName != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Source", Value: fmt.Sprintf("%s/%s", refKind, refName)})
			}
		}
		if path, ok := spec["path"].(string); ok && path != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Path", Value: path})
		}
	case "GitRepository", "HelmRepository", "OCIRepository", "Bucket":
		if url, ok := spec["url"].(string); ok && url != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "URL", Value: url})
		}
	case "HelmChart":
		if chart, ok := spec["chart"].(string); ok && chart != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Chart", Value: chart})
		}
	}
}

func extractReadyCondition(ti *model.Item, conditions []any) bool {
	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		condType, _ := cond["type"].(string)
		if condType != "Ready" {
			continue
		}
		condStatus, _ := cond["status"].(string)
		condMessage, _ := cond["message"].(string)
		condReason, _ := cond["reason"].(string)
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ready", Value: condStatus})
		if condReason != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: condReason})
		}
		if condMessage != "" && condStatus != "True" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Message", Value: condMessage})
		}
		if lastTransition, ok := cond["lastTransitionTime"].(string); ok && lastTransition != "" {
			if t, err := time.Parse(time.RFC3339, lastTransition); err == nil {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Last Transition", Value: formatRelativeTime(t)})
			}
		}
		return true
	}
	return false
}

func populateFluxRevision(ti *model.Item, status map[string]any) {
	if rev, ok := status["lastAppliedRevision"].(string); ok && rev != "" {
		if len(rev) > 12 {
			rev = rev[:12]
		}
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Revision", Value: rev})
	} else if artifact, ok := status["artifact"].(map[string]any); ok {
		if rev, ok := artifact["revision"].(string); ok && rev != "" {
			if len(rev) > 12 {
				rev = rev[:12]
			}
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Revision", Value: rev})
		}
	}
}

func populateCertManagerResource(ti *model.Item, status, spec map[string]any, kind string) {
	if status != nil {
		if conditions, ok := status["conditions"].([]any); ok {
			extractReadyCondition(ti, conditions)
		}
		if notAfter, ok := status["notAfter"].(string); ok && notAfter != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Expires", Value: notAfter})
		}
		if renewalTime, ok := status["renewalTime"].(string); ok && renewalTime != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Renewal", Value: renewalTime})
		}
	}
	if spec != nil {
		if secretName, ok := spec["secretName"].(string); ok && secretName != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Secret", Value: secretName})
		}
		if kind == "Certificate" {
			populateCertificateColumns(ti, spec)
		}
	}
}

func populateCertificateColumns(ti *model.Item, spec map[string]any) {
	if dnsNames, ok := spec["dnsNames"].([]any); ok && len(dnsNames) > 0 {
		names := make([]string, 0, len(dnsNames))
		for _, n := range dnsNames {
			if s, ok := n.(string); ok {
				names = append(names, s)
			}
		}
		if len(names) > 0 {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "DNS Names", Value: strings.Join(names, ", ")})
		}
	}
	if issuerRef, ok := spec["issuerRef"].(map[string]any); ok {
		if name, ok := issuerRef["name"].(string); ok && name != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Issuer", Value: name})
		}
	}
}
