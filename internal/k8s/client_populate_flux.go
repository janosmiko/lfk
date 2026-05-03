package k8s

import (
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

func populateFluxCDResource(ti *model.Item, obj map[string]any, status map[string]any) {
	if spec, ok := obj["spec"].(map[string]any); ok {
		if suspended, ok := spec["suspend"].(bool); ok && suspended {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Suspended", Value: "True"})
		}
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

func populateCertManagerResource(ti *model.Item, status, spec map[string]any) {
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
	}
}
