package k8s

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/janosmiko/lfk/internal/model"
)

// populateMutatingAdmissionPolicy adds a compact match-resources summary and
// a mutation count to a MutatingAdmissionPolicy's list columns.
func populateMutatingAdmissionPolicy(ti *model.Item, spec map[string]any) {
	if spec == nil {
		return
	}
	if matchConstraints, ok := spec["matchConstraints"].(map[string]any); ok {
		if summary := admissionResourceRulesSummary(matchConstraints); summary != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Match Resources", Value: summary})
		}
	}
	mutations, _ := spec["mutations"].([]any)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Mutations", Value: fmt.Sprintf("%d", len(mutations))})
}

// populateMutatingAdmissionPolicyBinding adds the bound policy name and a
// compact match-namespaces summary to a MutatingAdmissionPolicyBinding's
// list columns.
func populateMutatingAdmissionPolicyBinding(ti *model.Item, spec map[string]any) {
	if spec == nil {
		return
	}
	if policyName, ok := spec["policyName"].(string); ok && policyName != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Policy Name", Value: policyName})
	}
	matchResources, _ := spec["matchResources"].(map[string]any)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Match Namespaces", Value: admissionNamespaceSelectorSummary(matchResources)})
}

// admissionResourceRulesSummary renders spec.matchConstraints.resourceRules
// as a compact "group/resources" entry per rule.
func admissionResourceRulesSummary(matchConstraints map[string]any) string {
	rules, ok := matchConstraints["resourceRules"].([]any)
	if !ok || len(rules) == 0 {
		return ""
	}
	parts := make([]string, 0, len(rules))
	for _, r := range rules {
		rule, ok := r.(map[string]any)
		if !ok {
			continue
		}
		groups := stringsFromAny(rule["apiGroups"])
		resources := stringsFromAny(rule["resources"])
		if len(groups) == 0 && len(resources) == 0 {
			continue
		}
		parts = append(parts, strings.Join(groups, ",")+"/"+strings.Join(resources, ","))
	}
	return strings.Join(parts, "; ")
}

// admissionNamespaceSelectorSummary renders
// spec.matchResources.namespaceSelector as a compact label selector string.
// An unset or empty selector matches every namespace, so it renders as "all".
func admissionNamespaceSelectorSummary(matchResources map[string]any) string {
	if matchResources == nil {
		return "all"
	}
	nsSelector, ok := matchResources["namespaceSelector"].(map[string]any)
	if !ok || len(nsSelector) == 0 {
		return "all"
	}
	var ls metav1.LabelSelector
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(nsSelector, &ls); err != nil {
		return "all"
	}
	formatted := metav1.FormatLabelSelector(&ls)
	if formatted == "<none>" {
		return "all"
	}
	return formatted
}

// stringsFromAny converts a JSON-decoded []any of strings into []string,
// skipping non-string elements.
func stringsFromAny(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
