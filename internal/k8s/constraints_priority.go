package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// constraintsEventsLimit caps the Preempted events fetched for one pod,
// bounding both the API response size and the rows rendered from it.
const constraintsEventsLimit = 500

// priorityConstraintRows reports the target's PriorityClass value and, for
// a bare Pod, whether it is already mid-preemption: a set
// status.nominatedNodeName or a recent Preempted event against it.
func (c *Client) priorityConstraintRows(ctx context.Context, kubeCtx string, target ConstraintTarget) ([]ConstraintRow, error) {
	if target.PriorityClassName == "" {
		return nil, nil
	}
	cs, err := c.clientsetForContext(kubeCtx)
	if err != nil {
		return nil, err
	}
	pc, err := cs.SchedulingV1().PriorityClasses().Get(ctx, target.PriorityClassName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	rows := []ConstraintRow{{
		Source:   "PriorityClass",
		Kind:     "PriorityClass",
		Name:     pc.Name,
		Detail:   fmt.Sprintf("value %d", pc.Value),
		Headroom: fmt.Sprintf("%d", pc.Value),
	}}

	if target.PodName == "" {
		return rows, nil
	}
	pod, err := cs.CoreV1().Pods(target.Namespace).Get(ctx, target.PodName, metav1.GetOptions{})
	if err == nil && pod.Status.NominatedNodeName != "" {
		rows = append(rows, ConstraintRow{
			Source:    "PriorityClass",
			Kind:      "Pod",
			Namespace: target.Namespace,
			Name:      target.PodName,
			Detail:    fmt.Sprintf("nominated for node %s, awaiting preemption", pod.Status.NominatedNodeName),
			Blocking:  true,
		})
	}

	events, err := cs.CoreV1().Events(target.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod,reason=Preempted", target.PodName),
		Limit:         constraintsEventsLimit,
	})
	if err == nil {
		items := events.Items
		if len(items) > constraintsEventsLimit {
			items = items[:constraintsEventsLimit]
		}
		for _, ev := range items {
			rows = append(rows, ConstraintRow{
				Source:    "PriorityClass",
				Kind:      "Event",
				Namespace: target.Namespace,
				Name:      target.PodName,
				Detail:    ev.Message,
				Blocking:  true,
			})
		}
	}
	return rows, nil
}
