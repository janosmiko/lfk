package k8s

import (
	"context"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

// webhookRuleset normalizes a Validating/MutatingWebhookConfiguration
// entry so both kinds share one matching path below.
type webhookRuleset struct {
	configKind        string
	configName        string
	webhookName       string
	rules             []admissionregistrationv1.RuleWithOperations
	namespaceSelector *metav1.LabelSelector
	objectSelector    *metav1.LabelSelector
}

// webhookConstraintRows reports webhooks that would intercept an UPDATE
// or DELETE on the target (D3): GVR/operation match a rule, and any
// namespaceSelector/objectSelector match too.
func (c *Client) webhookConstraintRows(ctx context.Context, kubeCtx string, target ConstraintTarget) ([]ConstraintRow, error) {
	cs, err := c.clientsetForContext(kubeCtx)
	if err != nil {
		return nil, err
	}
	validating, err := cs.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing validatingwebhookconfigurations: %w", err)
	}
	mutating, err := cs.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing mutatingwebhookconfigurations: %w", err)
	}
	nsLabels, err := namespaceLabelsForWebhookMatch(ctx, cs, target.Namespace)
	if err != nil {
		return nil, err
	}

	rulesets := make([]webhookRuleset, 0, len(validating.Items)+len(mutating.Items))
	for _, wh := range validating.Items {
		rulesets = append(rulesets, validatingRulesets(wh)...)
	}
	for _, wh := range mutating.Items {
		rulesets = append(rulesets, mutatingRulesets(wh)...)
	}

	var rows []ConstraintRow
	for _, rs := range rulesets {
		if row, matched := webhookRow(rs, target, nsLabels); matched {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// namespaceLabelsForWebhookMatch fetches the target namespace's labels for
// namespaceSelector matching. Cluster-scoped targets (namespace == "")
// skip the call — there is no namespace to match against.
func namespaceLabelsForWebhookMatch(ctx context.Context, cs kubernetes.Interface, namespace string) (map[string]string, error) {
	if namespace == "" {
		return nil, nil
	}
	ns, err := cs.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting namespace %s: %w", namespace, err)
	}
	return ns.Labels, nil
}

func validatingRulesets(wh admissionregistrationv1.ValidatingWebhookConfiguration) []webhookRuleset {
	out := make([]webhookRuleset, 0, len(wh.Webhooks))
	for _, w := range wh.Webhooks {
		out = append(out, webhookRuleset{
			configKind:        "ValidatingWebhookConfiguration",
			configName:        wh.Name,
			webhookName:       w.Name,
			rules:             w.Rules,
			namespaceSelector: w.NamespaceSelector,
			objectSelector:    w.ObjectSelector,
		})
	}
	return out
}

func mutatingRulesets(wh admissionregistrationv1.MutatingWebhookConfiguration) []webhookRuleset {
	out := make([]webhookRuleset, 0, len(wh.Webhooks))
	for _, w := range wh.Webhooks {
		out = append(out, webhookRuleset{
			configKind:        "MutatingWebhookConfiguration",
			configName:        wh.Name,
			webhookName:       w.Name,
			rules:             w.Rules,
			namespaceSelector: w.NamespaceSelector,
			objectSelector:    w.ObjectSelector,
		})
	}
	return out
}

func webhookRow(rs webhookRuleset, target ConstraintTarget, nsLabels map[string]string) (ConstraintRow, bool) {
	matched, allResources := rulesMatchTarget(rs.rules, target.GVR)
	if !matched {
		return ConstraintRow{}, false
	}
	if !selectorMatches(rs.namespaceSelector, nsLabels) {
		return ConstraintRow{}, false
	}
	if !selectorMatches(rs.objectSelector, target.Labels) {
		return ConstraintRow{}, false
	}
	detail := rs.configName + "." + rs.webhookName
	if allResources {
		detail += ": all resources"
	}
	return ConstraintRow{
		Source:   "Webhook",
		Kind:     rs.configKind,
		Name:     rs.configName,
		Detail:   detail,
		Blocking: true,
	}, true
}

func rulesMatchTarget(rules []admissionregistrationv1.RuleWithOperations, gvr schema.GroupVersionResource) (matched, allResources bool) {
	for _, rule := range rules {
		if !operationsInclude(rule.Operations) {
			continue
		}
		if !valueOrWildcard(rule.APIGroups, gvr.Group) || !valueOrWildcard(rule.APIVersions, gvr.Version) {
			continue
		}
		if ok, wildcard := webhookResourceMatches(rule.Resources, gvr.Resource); ok {
			return true, wildcard
		}
	}
	return false, false
}

func operationsInclude(ops []admissionregistrationv1.OperationType) bool {
	for _, op := range ops {
		if op == admissionregistrationv1.OperationAll ||
			op == admissionregistrationv1.Update ||
			op == admissionregistrationv1.Delete {
			return true
		}
	}
	return false
}

func valueOrWildcard(values []string, want string) bool {
	for _, v := range values {
		if v == "*" || v == want {
			return true
		}
	}
	return false
}

func webhookResourceMatches(resources []string, want string) (matched, wildcard bool) {
	for _, r := range resources {
		if r == "*" {
			return true, true
		}
		if r == want {
			return true, false
		}
	}
	return false, false
}

func selectorMatches(sel *metav1.LabelSelector, objLabels map[string]string) bool {
	if sel == nil {
		return true
	}
	selector, err := metav1.LabelSelectorAsSelector(sel)
	if err != nil {
		return false
	}
	return selector.Matches(labels.Set(objLabels))
}
