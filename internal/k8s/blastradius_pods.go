package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// EvictedPodsFrom reduces pods to what the blast-radius math needs: the
// namespace and labels a PodDisruptionBudget matches on, and whether the pod
// counted as ready before the action.
func EvictedPodsFrom(pods []corev1.Pod) []EvictedPod {
	out := make([]EvictedPod, 0, len(pods))
	for i := range pods {
		p := &pods[i]
		out = append(out, EvictedPod{
			Namespace: p.Namespace,
			Labels:    p.Labels,
			Ready:     podIsReady(p),
		})
	}
	return out
}

func podIsReady(p *corev1.Pod) bool {
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// PodsOnNode returns every pod scheduled to one node, across all namespaces,
// which is what a drain evicts.
func (c *Client) PodsOnNode(ctx context.Context, contextName, nodeName string) ([]EvictedPod, error) {
	clientset, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}
	return podsOnNodeFrom(ctx, clientset, nodeName)
}

// PodsForSelector returns the pods one workload claims. A PodDisruptionBudget
// matches on labels, so the workload's own selector is the right question to
// ask; walking the owner chain would land on the same pods.
func (c *Client) PodsForSelector(
	ctx context.Context, contextName, namespace string, selector *metav1.LabelSelector,
) ([]EvictedPod, error) {
	clientset, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}
	return podsForSelectorFrom(ctx, clientset, namespace, selector)
}

// podsOnNodeFrom re-filters on the client because the fake clientset used in
// tests ignores FieldSelector, and a server that ignored it would otherwise
// hand back the whole cluster. Same reasoning as crash_investigator.go.
func podsOnNodeFrom(ctx context.Context, cs kubernetes.Interface, nodeName string) ([]EvictedPod, error) {
	list, err := cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return nil, fmt.Errorf("listing pods on node %s: %w", nodeName, err)
	}
	onNode := make([]corev1.Pod, 0, len(list.Items))
	for i := range list.Items {
		if list.Items[i].Spec.NodeName == nodeName {
			onNode = append(onNode, list.Items[i])
		}
	}
	return EvictedPodsFrom(onNode), nil
}

func podsForSelectorFrom(
	ctx context.Context, cs kubernetes.Interface, namespace string, selector *metav1.LabelSelector,
) ([]EvictedPod, error) {
	if selector == nil {
		return nil, nil
	}
	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, fmt.Errorf("reading workload selector: %w", err)
	}
	list, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		return nil, fmt.Errorf("listing pods for selector: %w", err)
	}
	// The fake clientset ignores LabelSelector too, so match again here.
	matched := make([]corev1.Pod, 0, len(list.Items))
	for i := range list.Items {
		if sel.Matches(labels.Set(list.Items[i].Labels)) {
			matched = append(matched, list.Items[i])
		}
	}
	return EvictedPodsFrom(matched), nil
}
