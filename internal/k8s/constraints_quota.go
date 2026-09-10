package k8s

import (
	"context"
	"fmt"

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
// the target requests (trimQuotaResourcePrefix matches "requests.cpu"
// against a container's bare "cpu" key).
func (c *Client) quotaConstraintRows(ctx context.Context, kubeCtx string, target ConstraintTarget) ([]ConstraintRow, error) {
	quotas, err := c.GetNamespaceQuotas(ctx, kubeCtx, target.Namespace)
	if err != nil {
		return nil, err
	}
	totals := totalContainerRequests(target.Containers)
	if len(totals) == 0 || len(quotas) == 0 {
		return nil, nil
	}

	var rows []ConstraintRow
	for _, quota := range quotas {
		for _, res := range quota.Resources {
			resName := trimQuotaResourcePrefix(res.Name)
			requested, ok := totals[resName]
			if !ok {
				continue
			}
			rows = append(rows, quotaRow(quota, res, requested))
		}
	}
	return rows, nil
}

// trimQuotaResourcePrefix strips the "requests."/"limits." prefix
// ResourceQuota uses for compute resources, so "requests.cpu" and "cpu"
// both compare equal to a container's raw request key.
func trimQuotaResourcePrefix(name string) string {
	for _, prefix := range []string{"requests.", "limits."} {
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			return name[len(prefix):]
		}
	}
	return name
}

func quotaRow(quota QuotaInfo, res QuotaResource, requested string) ConstraintRow {
	headroom := quantityHeadroom(res.Hard, res.Used)
	requestedQty, errR := resource.ParseQuantity(requested)
	blocking := errR == nil && headroom.qty != nil && requestedQty.Cmp(*headroom.qty) > 0
	return ConstraintRow{
		Source:    "Quota",
		Kind:      "ResourceQuota",
		Namespace: quota.Namespace,
		Name:      quota.Name,
		Detail:    fmt.Sprintf("requests %s %s (hard %s, used %s)", requested, res.Name, res.Hard, res.Used),
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
	sums := map[string]resource.Quantity{}
	for _, c := range containers {
		for name, qtyStr := range c.Requests {
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
