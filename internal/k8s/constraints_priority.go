package k8s

import (
	"context"
	"errors"
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
	// The PriorityClass row above survives a preemption lookup failure:
	// DetectConstraints keeps a source's rows and still banners its error.
	var lookupErrs []error

	pod, err := cs.CoreV1().Pods(target.Namespace).Get(ctx, target.PodName, metav1.GetOptions{})
	podFound := err == nil
	switch {
	case err != nil:
		lookupErrs = append(lookupErrs, fmt.Errorf("getting pod %s: %w", target.PodName, err))
	case pod.Status.NominatedNodeName != "":
		rows = append(rows, ConstraintRow{
			Source:    "PriorityClass",
			Kind:      "Pod",
			Namespace: target.Namespace,
			Name:      target.PodName,
			Detail:    fmt.Sprintf("nominated for node %s, awaiting preemption", pod.Status.NominatedNodeName),
			Blocking:  true,
		})
	}

	// A pod's UID is unique across the reused name, so pinning it excludes
	// Preempted events left over from an earlier pod with the same name.
	selector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod,reason=Preempted", target.PodName)
	if podFound {
		selector += fmt.Sprintf(",involvedObject.uid=%s", pod.UID)
	}
	events, err := cs.CoreV1().Events(target.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: selector,
		Limit:         constraintsEventsLimit,
	})
	if err != nil {
		lookupErrs = append(lookupErrs, fmt.Errorf("listing preemption events for %s: %w", target.PodName, err))
	} else {
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
	return rows, errors.Join(lookupErrs...)
}
