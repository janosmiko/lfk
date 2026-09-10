package k8s

import (
	"context"
	"fmt"
)

// pdbConstraintRows lists the namespace's PodDisruptionBudgets and reports
// the ones whose selector covers the target's pod labels.
func (c *Client) pdbConstraintRows(ctx context.Context, kubeCtx string, target ConstraintTarget) ([]ConstraintRow, error) {
	pdbs, err := c.ListPodDisruptionBudgets(ctx, kubeCtx, target.Namespace)
	if err != nil {
		return nil, err
	}
	var rows []ConstraintRow
	for i := range pdbs {
		pdb := &pdbs[i]
		if !PDBSelectorMatches(pdb, target.PodLabels) {
			continue
		}
		allowed := pdb.Status.DisruptionsAllowed
		rows = append(rows, ConstraintRow{
			Source:    "PDB",
			Kind:      "PodDisruptionBudget",
			Namespace: pdb.Namespace,
			Name:      pdb.Name,
			Detail:    fmt.Sprintf("%d disruptions allowed", allowed),
			Headroom:  fmt.Sprintf("%d", allowed),
			Blocking:  allowed == 0,
		})
	}
	return rows, nil
}
