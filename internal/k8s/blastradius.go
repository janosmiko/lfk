package k8s

import (
	"slices"
	"strings"

	policyv1 "k8s.io/api/policy/v1"
)

// EvictedPod is one pod a destructive action removes. Labels and namespace are
// what a PodDisruptionBudget matches on; Ready feeds the replica count.
type EvictedPod struct {
	Namespace string
	Labels    map[string]string
	Ready     bool
}

// PDBImpact is what one action costs one PodDisruptionBudget.
type PDBImpact struct {
	Namespace     string
	Name          string
	AllowedBefore int32
	AllowedAfter  int32
	Evicting      int
	Violated      bool
}

// BlastRadius is what a destructive action costs, in the terms the confirm
// dialog states: which budgets it eats into, and how many ready pods are left.
type BlastRadius struct {
	PDBs        []PDBImpact
	Evicting    int
	ReadyBefore int
	ReadyAfter  int
	Violation   bool

	// Uncounted is how many selected rows the figures leave out. A bulk
	// selection can hold workload rows whose pods cannot be resolved without
	// one API call per row, and a silent undercount would be worse than
	// saying so.
	Uncounted int
}

// ComputeBlastRadius works out what removing these pods costs. It makes no
// cluster calls, so the caller fetches the pods and the budgets first.
//
// readyBefore is the workload's ready replica count. Drain has no single
// workload, so it passes zero and only the PDB half of the result is used.
//
// Only budgets that cover at least one evicted pod are reported: an untouched
// budget is noise in a one-line summary.
func ComputeBlastRadius(
	evicting []EvictedPod, pdbs []policyv1.PodDisruptionBudget, readyBefore int,
) BlastRadius {
	out := BlastRadius{
		Evicting:    len(evicting),
		ReadyBefore: readyBefore,
	}

	readyGoing := 0
	for _, p := range evicting {
		if p.Ready {
			readyGoing++
		}
	}
	// A replica count read a moment before the pod list can be lower than the
	// pods actually going, and a negative count on screen helps nobody.
	out.ReadyAfter = max(readyBefore-readyGoing, 0)

	for i := range pdbs {
		b := &pdbs[i]
		covered := 0
		for _, p := range evicting {
			if p.Namespace == b.Namespace && PDBSelectorMatches(b, p.Labels) {
				covered++
			}
		}
		if covered == 0 {
			continue
		}
		allowedAfter := b.Status.DisruptionsAllowed - int32(covered) //nolint:gosec // pod counts are far below int32
		impact := PDBImpact{
			Namespace:     b.Namespace,
			Name:          b.Name,
			AllowedBefore: b.Status.DisruptionsAllowed,
			// Not clamped at zero: the size of the shortfall is the warning.
			AllowedAfter: allowedAfter,
			Evicting:     covered,
			Violated:     allowedAfter < 0,
		}
		if impact.Violated {
			out.Violation = true
		}
		out.PDBs = append(out.PDBs, impact)
	}

	// Stable order, so the line does not reshuffle between renders.
	slices.SortFunc(out.PDBs, func(a, b PDBImpact) int {
		if c := strings.Compare(a.Namespace, b.Namespace); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}
