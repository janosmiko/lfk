package k8s

import (
	"context"
	"fmt"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// PDBSelectorMatches reports whether a PodDisruptionBudget covers a pod with
// the given labels. It mirrors policy/v1 semantics: a nil selector matches
// nothing, an empty selector matches every pod in the namespace.
//
// Namespace is the caller's job. A PDB only ever covers pods in its own
// namespace, and this compares labels alone.
func PDBSelectorMatches(p *policyv1.PodDisruptionBudget, podLabels map[string]string) bool {
	if p == nil || p.Spec.Selector == nil {
		return false
	}
	sel, err := metav1.LabelSelectorAsSelector(p.Spec.Selector)
	if err != nil {
		return false
	}
	return sel.Matches(labels.Set(podLabels))
}

// ListPodDisruptionBudgets returns the PDBs in one namespace, or in every
// namespace when namespace is empty. A drain needs the cluster-wide list,
// because one node runs pods from many namespaces.
func (c *Client) ListPodDisruptionBudgets(
	ctx context.Context, contextName, namespace string,
) ([]policyv1.PodDisruptionBudget, error) {
	clientset, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}
	list, err := clientset.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pod disruption budgets: %w", err)
	}
	return list.Items, nil
}
