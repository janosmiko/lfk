package k8s

import (
	"context"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// listLimitRanges lists LimitRange objects in a namespace.
func (c *Client) listLimitRanges(ctx context.Context, contextName, namespace string) ([]corev1.LimitRange, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}
	list, err := cs.CoreV1().LimitRanges(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing limitranges: %w", err)
	}
	return list.Items, nil
}

// quotaConstraintRows reports ResourceQuota objects covering a resource
// the target asks for (trimQuotaResourcePrefix matches "requests.cpu"
// against a container's bare "cpu" key).
func (c *Client) quotaConstraintRows(ctx context.Context, kubeCtx string, target ConstraintTarget) ([]ConstraintRow, error) {
	quotas, err := c.GetNamespaceQuotas(ctx, kubeCtx, target.Namespace)
	if err != nil {
		return nil, err
	}
	requests := totalContainerRequests(target.Containers)
	limits := totalContainerLimits(target.Containers)
	if (len(requests) == 0 && len(limits) == 0) || len(quotas) == 0 {
		return nil, nil
	}

	var rows []ConstraintRow
	for _, quota := range quotas {
		if !quotaAppliesTo(quota, target) {
			continue
		}
		for _, res := range quota.Resources {
			resName, countsLimits := trimQuotaResourcePrefix(res.Name)
			totals, verb := requests, "requests"
			if countsLimits {
				totals, verb = limits, "limits"
			}
			asked, ok := totals[resName]
			if !ok {
				continue
			}
			rows = append(rows, quotaRow(quota, res, asked, verb))
		}
	}
	return rows, nil
}

// trimQuotaResourcePrefix reports which half of the container spec the
// quota counts: Kubernetes charges "limits.*" against container limits,
// "requests.*" and bare "cpu"/"memory" against requests.
func trimQuotaResourcePrefix(name string) (resourceName string, countsLimits bool) {
	if trimmed, ok := strings.CutPrefix(name, "limits."); ok && trimmed != "" {
		return trimmed, true
	}
	if trimmed, ok := strings.CutPrefix(name, "requests."); ok && trimmed != "" {
		return trimmed, false
	}
	return name, false
}

// quotaAppliesTo ANDs spec.scopes with spec.scopeSelector. A scope needing
// pod state this view lacks (Terminating, CrossNamespacePodAffinity) leaves
// the quota applicable rather than hiding one that may well bind.
func quotaAppliesTo(quota QuotaInfo, target ConstraintTarget) bool {
	for _, scope := range quota.Scopes {
		if !quotaScopeMatches(QuotaScopeRequirement{ScopeName: scope, Operator: string(corev1.ScopeSelectorOpExists)}, target) {
			return false
		}
	}
	for _, req := range quota.ScopeSelector {
		if !quotaScopeMatches(req, target) {
			return false
		}
	}
	return true
}

func quotaScopeMatches(req QuotaScopeRequirement, target ConstraintTarget) bool {
	switch corev1.ResourceQuotaScope(req.ScopeName) {
	case corev1.ResourceQuotaScopePriorityClass:
		return priorityClassScopeMatches(req, target.PriorityClassName)
	case corev1.ResourceQuotaScopeBestEffort:
		return scopePresenceMatches(req.Operator, targetIsBestEffort(target))
	case corev1.ResourceQuotaScopeNotBestEffort:
		return scopePresenceMatches(req.Operator, !targetIsBestEffort(target))
	default:
		return true
	}
}

func priorityClassScopeMatches(req QuotaScopeRequirement, className string) bool {
	switch corev1.ScopeSelectorOperator(req.Operator) {
	case corev1.ScopeSelectorOpIn:
		return slices.Contains(req.Values, className)
	case corev1.ScopeSelectorOpNotIn:
		return !slices.Contains(req.Values, className)
	case corev1.ScopeSelectorOpDoesNotExist:
		return className == ""
	default:
		return className != ""
	}
}

func scopePresenceMatches(operator string, has bool) bool {
	if corev1.ScopeSelectorOperator(operator) == corev1.ScopeSelectorOpDoesNotExist {
		return !has
	}
	return has
}

// targetIsBestEffort mirrors the BestEffort QoS rule: no container sets
// any CPU or memory request or limit.
func targetIsBestEffort(target ConstraintTarget) bool {
	for _, c := range target.Containers {
		for _, name := range []string{"cpu", "memory"} {
			if c.Requests[name] != "" || c.Limits[name] != "" {
				return false
			}
		}
	}
	return true
}

func quotaScopesFromRaw(v any) []string {
	rawScopes := rawSlice(v)
	if len(rawScopes) == 0 {
		return nil
	}
	out := make([]string, 0, len(rawScopes))
	for _, s := range rawScopes {
		if str, ok := s.(string); ok {
			out = append(out, str)
		}
	}
	return out
}

func quotaScopeSelectorFromRaw(v any) []QuotaScopeRequirement {
	rawReqs := rawSlice(rawMap(v)["matchExpressions"])
	if len(rawReqs) == 0 {
		return nil
	}
	out := make([]QuotaScopeRequirement, 0, len(rawReqs))
	for _, e := range rawReqs {
		em := rawMap(e)
		req := QuotaScopeRequirement{
			ScopeName: stringField(em, "scopeName"),
			Operator:  stringField(em, "operator"),
		}
		for _, val := range rawSlice(em["values"]) {
			if s, ok := val.(string); ok {
				req.Values = append(req.Values, s)
			}
		}
		out = append(out, req)
	}
	return out
}

func quotaRow(quota QuotaInfo, res QuotaResource, asked, verb string) ConstraintRow {
	headroom := quantityHeadroom(res.Hard, res.Used)
	askedQty, errR := resource.ParseQuantity(asked)
	blocking := errR == nil && headroom.qty != nil && askedQty.Cmp(*headroom.qty) > 0
	return ConstraintRow{
		Source:    "Quota",
		Kind:      "ResourceQuota",
		Namespace: quota.Namespace,
		Name:      quota.Name,
		Detail:    fmt.Sprintf("%s %s %s (hard %s, used %s)", verb, asked, res.Name, res.Hard, res.Used),
		Headroom:  headroom.text,
		Blocking:  blocking,
	}
}

// limitRangeConstraintRows checks each container's requests against every
// namespace LimitRange's per-container Min/Max bounds.
func (c *Client) limitRangeConstraintRows(ctx context.Context, kubeCtx string, target ConstraintTarget) ([]ConstraintRow, error) {
	limitRanges, err := c.listLimitRanges(ctx, kubeCtx, target.Namespace)
	if err != nil {
		return nil, err
	}
	var rows []ConstraintRow
	for _, lr := range limitRanges {
		for _, item := range lr.Spec.Limits {
			if item.Type != corev1.LimitTypeContainer {
				continue
			}
			rows = append(rows, limitRangeItemRows(lr, item, target.Containers)...)
		}
	}
	return rows, nil
}

func limitRangeItemRows(lr corev1.LimitRange, item corev1.LimitRangeItem, containers []ContainerRequest) []ConstraintRow {
	var rows []ConstraintRow
	for _, c := range containers {
		for resName, minQty := range item.Min {
			if row, ok := limitRangeBoundRow(lr, resName.String(), "min", minQty, c, true); ok {
				rows = append(rows, row)
			}
		}
		for resName, maxQty := range item.Max {
			if row, ok := limitRangeBoundRow(lr, resName.String(), "max", maxQty, c, false); ok {
				rows = append(rows, row)
			}
		}
	}
	return rows
}

// limitRangeBoundRow reports a container's request against one Min or Max
// bound. belowIsViolation selects the comparison direction: Min is
// violated by a smaller request, Max by a larger one.
func limitRangeBoundRow(lr corev1.LimitRange, resName, bound string, boundQty resource.Quantity, c ContainerRequest, belowIsViolation bool) (ConstraintRow, bool) {
	reqStr, ok := c.Requests[resName]
	if !ok {
		return ConstraintRow{}, false
	}
	reqQty, err := resource.ParseQuantity(reqStr)
	if err != nil {
		return ConstraintRow{}, false
	}
	cmp := reqQty.Cmp(boundQty)
	violated := (belowIsViolation && cmp < 0) || (!belowIsViolation && cmp > 0)
	if !violated {
		return ConstraintRow{}, false
	}
	return ConstraintRow{
		Source:    "LimitRange",
		Kind:      "LimitRange",
		Namespace: lr.Namespace,
		Name:      lr.Name,
		Detail: fmt.Sprintf("container %s requests %s %s, %s is %s",
			c.Name, reqStr, resName, bound, boundQty.String()),
		Headroom: boundQty.String(),
		Blocking: true,
	}, true
}

// totalContainerRequests sums each container's requested amount per
// resource name, so a multi-container pod's ask is compared to quota as a
// whole rather than one container at a time.
func totalContainerRequests(containers []ContainerRequest) map[string]string {
	return totalContainerResources(containers, func(c ContainerRequest) map[string]string { return c.Requests })
}

func totalContainerLimits(containers []ContainerRequest) map[string]string {
	return totalContainerResources(containers, func(c ContainerRequest) map[string]string { return c.Limits })
}

func totalContainerResources(containers []ContainerRequest, pick func(ContainerRequest) map[string]string) map[string]string {
	sums := map[string]resource.Quantity{}
	for _, c := range containers {
		for name, qtyStr := range pick(c) {
			qty, err := resource.ParseQuantity(qtyStr)
			if err != nil {
				continue
			}
			sum := sums[name]
			sum.Add(qty)
			sums[name] = sum
		}
	}
	if len(sums) == 0 {
		return nil
	}
	out := make(map[string]string, len(sums))
	for name, qty := range sums {
		out[name] = qty.String()
	}
	return out
}

// quantityHeadroomResult carries both the display string and the
// resource.Quantity form, so callers can compare a request against it
// without reparsing.
type quantityHeadroomResult struct {
	text string
	qty  *resource.Quantity
}

func quantityHeadroom(hardStr, usedStr string) quantityHeadroomResult {
	hard, errH := resource.ParseQuantity(hardStr)
	used, errU := resource.ParseQuantity(usedStr)
	if errH != nil || errU != nil {
		return quantityHeadroomResult{text: "n/a"}
	}
	hard.Sub(used)
	return quantityHeadroomResult{text: hard.String(), qty: &hard}
}
