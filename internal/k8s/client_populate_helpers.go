package k8s

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/robfig/cron/v3"
)

// nextCronFire returns the next time a CronJob with the given crontab
// schedule will fire. timeZone follows spec.timeZone semantics: when
// empty, the schedule is evaluated in UTC — the same default
// kube-controller-manager uses for CronJobs without spec.timeZone, so
// the displayed time matches what the cluster will actually do
// regardless of the user's local timezone.
//
// Returns ok=false when schedule is empty, when timeZone fails to load,
// or when schedule fails to parse. Accepts standard 5-field crontabs
// and the predefined descriptors (`@hourly`, `@daily`, …).
func nextCronFire(schedule, timeZone string, now time.Time) (time.Time, bool) {
	if schedule == "" {
		return time.Time{}, false
	}
	loc := time.UTC
	if timeZone != "" {
		l, err := time.LoadLocation(timeZone)
		if err != nil {
			return time.Time{}, false
		}
		loc = l
	}
	parsed, err := cron.ParseStandard(schedule)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.Next(now.In(loc)), true
}

// populateMetadataFields extracts labels, finalizers, and annotations from the
// object metadata and appends them as columns for preview display.
func populateMetadataFields(ti *model.Item, obj map[string]interface{}) {
	metadata, _ := obj["metadata"].(map[string]interface{})
	if metadata == nil {
		return
	}

	// Labels: extract key=value pairs, skip noisy labels.
	if labels, ok := metadata["labels"].(map[string]interface{}); ok && len(labels) > 0 {
		var labelPairs []string
		for k, v := range labels {
			if k == "helm.sh/chart" || strings.HasPrefix(k, "app.kubernetes.io/managed-by") {
				continue
			}
			labelPairs = append(labelPairs, k+"="+fmt.Sprint(v))
		}
		sort.Strings(labelPairs)
		if len(labelPairs) > 0 {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Labels", Value: strings.Join(labelPairs, ", ")})
		}
	}

	// Finalizers: important for debugging stuck deletions.
	if finalizers, ok := metadata["finalizers"].([]interface{}); ok && len(finalizers) > 0 {
		var fins []string
		for _, f := range finalizers {
			if s, ok := f.(string); ok {
				fins = append(fins, s)
			}
		}
		if len(fins) > 0 {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Finalizers", Value: strings.Join(fins, ", ")})
		}
	}

	// Annotations: store key=value pairs (sorted, noisy ones filtered).
	if annotations, ok := metadata["annotations"].(map[string]interface{}); ok && len(annotations) > 0 {
		var annPairs []string
		for k, v := range annotations {
			// Skip very long values (e.g., kubectl.kubernetes.io/last-applied-configuration).
			val := fmt.Sprint(v)
			if len(val) > 100 {
				val = val[:97] + "..."
			}
			annPairs = append(annPairs, k+"="+val)
		}
		sort.Strings(annPairs)
		ti.Columns = append(ti.Columns, model.KeyValue{
			Key:   "Annotations",
			Value: strings.Join(annPairs, ", "),
		})
	}
}

// extractGenericConditions extracts condition information from a status.conditions
// array for generic CRD resources. It prefers the "Ready" condition; if not found,
// it falls back to the last condition in the array.
func extractGenericConditions(ti *model.Item, conditions []interface{}) {
	var readyCond, trueCond, lastCond map[string]interface{}
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		lastCond = cond
		condType, _ := cond["type"].(string)
		condStatus, _ := cond["status"].(string)
		condReason, _ := cond["reason"].(string)
		condMessage, _ := cond["message"].(string)
		if condType == "Ready" {
			readyCond = cond
		}
		if condStatus == "True" {
			trueCond = cond
		}
		// Store every condition for the details pane.
		if condType != "" {
			ti.Conditions = append(ti.Conditions, model.ConditionEntry{
				Type:    condType,
				Status:  condStatus,
				Reason:  condReason,
				Message: condMessage,
			})
		}
	}

	// Priority: Ready condition > True condition (when last is a negated
	// negative type like "Failed: False") > last condition.
	chosen := readyCond
	if chosen == nil {
		// If the last condition is a negative type with status "False"
		// (e.g., "Failed: False"), prefer a True condition instead — the
		// negative state is inactive and showing it is misleading.
		if lastCond != nil && trueCond != nil {
			lastStatus, _ := lastCond["status"].(string)
			lastType, _ := lastCond["type"].(string)
			if lastStatus == "False" && isNegativeConditionType(lastType) {
				chosen = trueCond
			}
		}
	}
	if chosen == nil {
		chosen = lastCond
	}
	if chosen == nil {
		return
	}

	condType, _ := chosen["type"].(string)
	condStatus, _ := chosen["status"].(string)
	condReason, _ := chosen["reason"].(string)
	condMessage, _ := chosen["message"].(string)

	if condType != "" && condStatus != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: condType, Value: condStatus})
	}
	if condReason != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: condReason})
	}
	if condMessage != "" && condStatus != "True" {
		// Truncate long messages for table display.
		if len(condMessage) > 80 {
			condMessage = condMessage[:77] + "..."
		}
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Message", Value: condMessage})
	}
	if lastTransition, ok := chosen["lastTransitionTime"].(string); ok && lastTransition != "" {
		if t, err := time.Parse(time.RFC3339, lastTransition); err == nil {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Last Transition", Value: formatRelativeTime(t)})
		}
	}
}

// isNegativeConditionType returns true if the condition type name represents a
// negative/failure state (e.g., "Failed", "Error"). When such a condition has
// status "False" it means the failure is NOT active and should not be shown as
// the primary status.
func isNegativeConditionType(condType string) bool {
	lower := strings.ToLower(condType)
	for _, neg := range []string{"fail", "error", "degrad"} {
		if strings.Contains(lower, neg) {
			return true
		}
	}
	return false
}

// populateContainerImages extracts container images from a pod template spec.
func populateContainerImages(ti *model.Item, spec map[string]interface{}) {
	tmpl, ok := spec["template"].(map[string]interface{})
	if !ok {
		return
	}
	tmplSpec, ok := tmpl["spec"].(map[string]interface{})
	if !ok {
		return
	}
	containers, ok := tmplSpec["containers"].([]interface{})
	if !ok {
		return
	}
	var images []string
	for _, c := range containers {
		if cMap, ok := c.(map[string]interface{}); ok {
			if img, ok := cMap["image"].(string); ok {
				images = append(images, img)
			}
		}
	}
	if len(images) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Images", Value: strings.Join(images, ", ")})
	}
}

// extractContainerResources sums CPU and memory requests/limits from a list of container specs.
// Returns cpuReq, cpuLim, memReq, memLim as human-readable strings.
func extractContainerResources(containers []interface{}) (cpuReq, cpuLim, memReq, memLim string) {
	var cpuReqs, cpuLims, memReqs, memLims []string
	for _, c := range containers {
		cMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		resources, ok := cMap["resources"].(map[string]interface{})
		if !ok {
			continue
		}
		if requests, ok := resources["requests"].(map[string]interface{}); ok {
			if v, ok := requests["cpu"].(string); ok {
				cpuReqs = append(cpuReqs, v)
			}
			if v, ok := requests["memory"].(string); ok {
				memReqs = append(memReqs, v)
			}
		}
		if limits, ok := resources["limits"].(map[string]interface{}); ok {
			if v, ok := limits["cpu"].(string); ok {
				cpuLims = append(cpuLims, v)
			}
			if v, ok := limits["memory"].(string); ok {
				memLims = append(memLims, v)
			}
		}
	}
	if len(cpuReqs) > 0 {
		cpuReq = strings.Join(cpuReqs, "+")
	}
	if len(cpuLims) > 0 {
		cpuLim = strings.Join(cpuLims, "+")
	}
	if len(memReqs) > 0 {
		memReq = strings.Join(memReqs, "+")
	}
	if len(memLims) > 0 {
		memLim = strings.Join(memLims, "+")
	}
	return
}

// extractTemplateResources navigates spec.template.spec.containers and extracts resource requests/limits.
func extractTemplateResources(spec map[string]interface{}) (cpuReq, cpuLim, memReq, memLim string) {
	tmpl, ok := spec["template"].(map[string]interface{})
	if !ok {
		return
	}
	tmplSpec, ok := tmpl["spec"].(map[string]interface{})
	if !ok {
		return
	}
	containers, ok := tmplSpec["containers"].([]interface{})
	if !ok {
		return
	}
	return extractContainerResources(containers)
}

// addResourceColumns appends CPU/memory request/limit columns to an item if they are non-empty.
func addResourceColumns(ti *model.Item, cpuReq, cpuLim, memReq, memLim string) {
	if cpuReq != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "CPU Req", Value: cpuReq})
	}
	if cpuLim != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "CPU Lim", Value: cpuLim})
	}
	if memReq != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Mem Req", Value: memReq})
	}
	if memLim != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Mem Lim", Value: memLim})
	}
}

// getInt extracts an integer value from a map, handling both int64 and float64.
func getInt(m map[string]interface{}, key string) int64 {
	if v, ok := m[key].(int64); ok {
		return v
	}
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return 0
}

// parseEventTimestamp extracts a timestamp from a top-level event field (e.g., "lastTimestamp", "eventTime").
func parseEventTimestamp(obj map[string]interface{}, field string) time.Time {
	val, ok := obj[field]
	if !ok || val == nil {
		return time.Time{}
	}
	v, ok := val.(string)
	if !ok || v == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		// Try RFC3339Nano for eventTime which may include nanoseconds.
		t, err = time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return time.Time{}
		}
	}
	return t
}

// evaluateSimpleJSONPath traverses a map[string]interface{} using a simple
// dot-notation JSONPath expression like ".status.phase" or ".status.conditions[0].type".
// It returns the value found and a boolean indicating success.
// This does NOT handle complex JSONPath features (wildcards, filters, etc.).
func evaluateSimpleJSONPath(obj map[string]interface{}, path string) (interface{}, bool) {
	// Strip leading dot.
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return nil, false
	}

	parts := strings.Split(path, ".")
	var current interface{} = obj

	for _, part := range parts {
		if current == nil {
			return nil, false
		}

		// Check for array index notation, e.g. "conditions[0]".
		fieldName := part
		arrayIdx := -1
		if bracketIdx := strings.Index(part, "["); bracketIdx >= 0 {
			closeBracket := strings.Index(part, "]")
			if closeBracket > bracketIdx+1 {
				idxStr := part[bracketIdx+1 : closeBracket]
				if idx, err := strconv.Atoi(idxStr); err == nil {
					arrayIdx = idx
				}
			}
			fieldName = part[:bracketIdx]
		}

		// Navigate into the map.
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		val, exists := m[fieldName]
		if !exists {
			return nil, false
		}

		// If we need to index into an array.
		if arrayIdx >= 0 {
			arr, ok := val.([]interface{})
			if !ok || arrayIdx >= len(arr) {
				return nil, false
			}
			current = arr[arrayIdx]
		} else {
			current = val
		}
	}

	return current, true
}

// formatPrinterValue formats a value from a CRD printer column based on its type.
func formatPrinterValue(val interface{}, colType string) string {
	if val == nil {
		return ""
	}

	switch colType {
	case "date":
		// Try to parse as RFC3339 and format as relative time.
		s, ok := val.(string)
		if !ok {
			return fmt.Sprintf("%v", val)
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			// Try RFC3339Nano as fallback.
			t, err = time.Parse(time.RFC3339Nano, s)
			if err != nil {
				return s
			}
		}
		return formatAge(time.Since(t))

	case "integer":
		switch v := val.(type) {
		case float64:
			return strconv.FormatInt(int64(v), 10)
		case int64:
			return strconv.FormatInt(v, 10)
		default:
			return fmt.Sprintf("%v", val)
		}

	case "number":
		switch v := val.(type) {
		case float64:
			// Use compact formatting: no trailing zeros.
			if v == float64(int64(v)) {
				return strconv.FormatInt(int64(v), 10)
			}
			return strconv.FormatFloat(v, 'f', -1, 64)
		case int64:
			return strconv.FormatInt(v, 10)
		default:
			return fmt.Sprintf("%v", val)
		}

	case "boolean":
		switch v := val.(type) {
		case bool:
			if v {
				return "true"
			}
			return "false"
		default:
			return fmt.Sprintf("%v", val)
		}

	default: // "string" and everything else
		return fmt.Sprintf("%v", val)
	}
}

// applyDeletionStatus overrides the status to "Terminating" for resources
// that have a deletionTimestamp set (Deleting == true).
func applyDeletionStatus(ti *model.Item) {
	if ti.Deleting {
		ti.Status = "Terminating"
	}
}

// formatAge formats a duration into a human-readable age string.
func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days < 365 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dy", days/365)
}
