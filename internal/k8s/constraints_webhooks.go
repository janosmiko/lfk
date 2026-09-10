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

// validatingWebhookConstraintRows reports configurations that intercept an
// UPDATE or DELETE on the target (D3). Mutating ones are a separate source,
// so a denial of one still leaves the other's rows visible.
func (c *Client) validatingWebhookConstraintRows(ctx context.Context, kubeCtx string, target ConstraintTarget) ([]ConstraintRow, error) {
	cs, err := c.clientsetForContext(kubeCtx)
	if err != nil {
		return nil, err
	}
	list, err := cs.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing validatingwebhookconfigurations: %w", err)
	}
	var rulesets []webhookRuleset
	for _, wh := range list.Items {
		rulesets = append(rulesets, validatingRulesets(wh)...)
	}
	return webhookRows(ctx, cs, rulesets, target)
}

// mutatingWebhookConstraintRows is validatingWebhookConstraintRows for
// MutatingWebhookConfigurations.
func (c *Client) mutatingWebhookConstraintRows(ctx context.Context, kubeCtx string, target ConstraintTarget) ([]ConstraintRow, error) {
	cs, err := c.clientsetForContext(kubeCtx)
	if err != nil {
		return nil, err
	}
	list, err := cs.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing mutatingwebhookconfigurations: %w", err)
	}
	var rulesets []webhookRuleset
	for _, wh := range list.Items {
		rulesets = append(rulesets, mutatingRulesets(wh)...)
	}
	return webhookRows(ctx, cs, rulesets, target)
}

func webhookRows(ctx context.Context, cs kubernetes.Interface, rulesets []webhookRuleset, target ConstraintTarget) ([]ConstraintRow, error) {
	if len(rulesets) == 0 {
		return nil, nil
	}
	nsLabels, nsSelectorApplies, err := namespaceLabelsForWebhookMatch(ctx, cs, target)
	if err != nil {
		return nil, err
	}
	var rows []ConstraintRow
	for _, rs := range rulesets {
		if row, matched := webhookRow(rs, target, nsLabels, nsSelectorApplies); matched {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// namespaceLabelsForWebhookMatch resolves what a namespaceSelector matches
// against: a namespaced target's namespace, a Namespace object itself, and
// for any other cluster-scoped kind nothing, so the selector cannot apply.
func namespaceLabelsForWebhookMatch(ctx context.Context, cs kubernetes.Interface, target ConstraintTarget) (nsLabels map[string]string, applies bool, err error) {
	if target.Namespace == "" {
		if target.Kind == "Namespace" {
			return target.Labels, true, nil
		}
		return nil, false, nil
	}
	ns, err := cs.CoreV1().Namespaces().Get(ctx, target.Namespace, metav1.GetOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("getting namespace %s: %w", target.Namespace, err)
	}
	return ns.Labels, true, nil
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

func webhookRow(rs webhookRuleset, target ConstraintTarget, nsLabels map[string]string, nsSelectorApplies bool) (ConstraintRow, bool) {
	matched, allResources := rulesMatchTarget(rs.rules, target.GVR, targetRuleScope(target))
	if !matched {
		return ConstraintRow{}, false
	}
	if nsSelectorApplies && !selectorMatches(rs.namespaceSelector, nsLabels) {
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

// targetRuleScope maps the target to the scope a rule selects on. A
// Namespace object counts as cluster-scoped here, as it does in the API.
func targetRuleScope(target ConstraintTarget) admissionregistrationv1.ScopeType {
	if target.Namespace == "" {
		return admissionregistrationv1.ClusterScope
	}
	return admissionregistrationv1.NamespacedScope
}

// ruleScopeMatches treats an unset rule scope as the API's "*" default.
func ruleScopeMatches(ruleScope *admissionregistrationv1.ScopeType, target admissionregistrationv1.ScopeType) bool {
	if ruleScope == nil || *ruleScope == admissionregistrationv1.AllScopes {
		return true
	}
	return *ruleScope == target
}

func rulesMatchTarget(rules []admissionregistrationv1.RuleWithOperations, gvr schema.GroupVersionResource, scope admissionregistrationv1.ScopeType) (matched, allResources bool) {
	for _, rule := range rules {
		if !operationsInclude(rule.Operations) {
			continue
		}
		if !ruleScopeMatches(rule.Scope, scope) {
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

// webhookResourceMatches accepts the "resource/subresource" forms a rule
// may use. "*/*" covers every resource and subresource, "<want>/*" the
// target resource and its own subresources.
func webhookResourceMatches(resources []string, want string) (matched, wildcard bool) {
	for _, r := range resources {
		if r == "*" || r == "*/*" {
			return true, true
		}
		if r == want || r == want+"/*" {
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
