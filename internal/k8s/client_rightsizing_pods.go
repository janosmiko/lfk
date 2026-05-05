package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// resolvePodsForWorkload returns the names of pods backing the given
// workload. Strategy varies by kind:
//
//   - Pod         -> [self]
//   - Deployment, StatefulSet, DaemonSet, Job -> label-selector match
//     on .spec.selector.matchLabels
//   - CronJob     -> walk Jobs whose ownerReferences include the
//     CronJob, then list pods owned by those Jobs
//
// Returns an empty slice (no error) when the workload exists but
// has no pods. Errors out for kinds outside the supported set so
// callers can't accidentally bypass the action-menu gate.
//
//nolint:unparam // contextName is the right-sizing API contract — real callers pass the active context; tests use the fake's "test-ctx".
func (c *Client) resolvePodsForWorkload(ctx context.Context, contextName, namespace, kind, name string) ([]string, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "Pod":
		return []string{name}, nil
	case "Deployment":
		dep, err := cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("getting deployment %s/%s: %w", namespace, name, err)
		}
		return c.podsBySelector(ctx, contextName, namespace, dep.Spec.Selector)
	case "StatefulSet":
		ss, err := cs.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("getting statefulset %s/%s: %w", namespace, name, err)
		}
		return c.podsBySelector(ctx, contextName, namespace, ss.Spec.Selector)
	case "DaemonSet":
		ds, err := cs.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("getting daemonset %s/%s: %w", namespace, name, err)
		}
		return c.podsBySelector(ctx, contextName, namespace, ds.Spec.Selector)
	case "Job":
		job, err := cs.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("getting job %s/%s: %w", namespace, name, err)
		}
		return c.podsBySelector(ctx, contextName, namespace, job.Spec.Selector)
	case "CronJob":
		return c.podsForCronJob(ctx, contextName, namespace, name)
	default:
		return nil, fmt.Errorf("rightsizing not supported for kind %q", kind)
	}
}

// podsBySelector lists pod names matching the given selector. Nil
// selector -> empty result (workload that selects nothing).
func (c *Client) podsBySelector(ctx context.Context, contextName, namespace string, sel *metav1.LabelSelector) ([]string, error) {
	if sel == nil {
		return nil, nil
	}
	selector, err := metav1.LabelSelectorAsSelector(sel)
	if err != nil {
		return nil, fmt.Errorf("converting selector: %w", err)
	}
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}
	pods, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods by selector %s: %w", selector, err)
	}
	out := make([]string, 0, len(pods.Items))
	for i := range pods.Items {
		out = append(out, pods.Items[i].Name)
	}
	return out, nil
}

// podsForCronJob walks the CronJob -> Jobs -> Pods owner chain.
// CronJob has no selector so the only safe lookup is by ownerRef
// UID match (label conventions vary by controller).
func (c *Client) podsForCronJob(ctx context.Context, contextName, namespace, cronName string) ([]string, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}
	cj, err := cs.BatchV1().CronJobs(namespace).Get(ctx, cronName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting cronjob %s/%s: %w", namespace, cronName, err)
	}
	jobs, err := cs.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing jobs in %s: %w", namespace, err)
	}
	jobUIDs := make(map[string]bool)
	for i := range jobs.Items {
		for _, owner := range jobs.Items[i].OwnerReferences {
			if owner.UID == cj.UID {
				jobUIDs[string(jobs.Items[i].UID)] = true
				break
			}
		}
	}
	if len(jobUIDs) == 0 {
		return nil, nil
	}
	pods, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods in %s: %w", namespace, err)
	}
	out := make([]string, 0)
	for i := range pods.Items {
		for _, owner := range pods.Items[i].OwnerReferences {
			if jobUIDs[string(owner.UID)] {
				out = append(out, pods.Items[i].Name)
				break
			}
		}
	}
	return out, nil
}
