// Capacity- and status-based advisor checks: ResourceQuotas near their
// limits, HPAs pinned or at their ceiling, and orphaned PDBs. Unlike the
// spec-based checks these reflect the cluster's current state, so summaries
// say "currently" rather than implying a config defect.
package advisor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/janosmiko/lfk/internal/security"
)

// quotaNearLimitRatio is the used/hard threshold above which a ResourceQuota
// is reported — at 100% the namespace starts rejecting new workloads.
const quotaNearLimitRatio = 0.9

// quotaFindings flags ResourceQuotas with any resource at or above the
// near-limit threshold.
func (d *clusterData) quotaFindings() []security.Finding {
	if !d.quotasOK {
		return nil
	}
	var out []security.Finding
	for i := range d.quotas {
		q := &d.quotas[i]
		if systemNamespaces[q.Namespace] {
			continue
		}
		// Status.Hard mirrors Spec.Hard once the quota controller has
		// reconciled; fall back to Spec.Hard for a brand-new quota. Used is
		// empty in that window too, so the fallback cannot create a finding.
		hard := q.Status.Hard
		if len(hard) == 0 {
			hard = q.Spec.Hard
		}
		var crowded []string
		for name, h := range hard {
			used, ok := q.Status.Used[name]
			if !ok {
				continue
			}
			hv := h.AsApproximateFloat64()
			if hv <= 0 {
				continue
			}
			if ratio := used.AsApproximateFloat64() / hv; ratio >= quotaNearLimitRatio {
				crowded = append(crowded, fmt.Sprintf("%s at %.0f%%", name, ratio*100))
			}
		}
		if len(crowded) == 0 {
			continue
		}
		sort.Strings(crowded) // map iteration order is random
		out = append(out, makeFinding(q.Namespace, "ResourceQuota", q.Name, "quota_near_limit", security.SeverityMedium,
			"ResourceQuota near its limit",
			fmt.Sprintf("ResourceQuota %q is currently near its limits (%s); new workloads are rejected at 100%%.", q.Name, strings.Join(crowded, ", "))))
	}
	return out
}

// hpaConfigFindings flags HPAs that cannot scale (minReplicas == maxReplicas)
// or are currently pinned at their ceiling.
func (d *clusterData) hpaConfigFindings() []security.Finding {
	if !d.hpasOK {
		return nil
	}
	var out []security.Finding
	for i := range d.hpas {
		h := &d.hpas[i]
		if systemNamespaces[h.Namespace] {
			continue
		}
		minReplicas := int32(1)
		if h.Spec.MinReplicas != nil {
			minReplicas = *h.Spec.MinReplicas
		}
		// The cases are mutually exclusive by switch order: a pinned HPA
		// (min == max) is hpa_fixed and must not also report hpa_at_max.
		switch {
		case minReplicas == h.Spec.MaxReplicas:
			out = append(out, makeFinding(h.Namespace, "HorizontalPodAutoscaler", h.Name, "hpa_fixed", security.SeverityLow,
				"HPA cannot scale",
				fmt.Sprintf("HPA %q has minReplicas == maxReplicas (%d); it can never scale and is dead configuration.", h.Name, h.Spec.MaxReplicas)))
		// DesiredReplicas (the controller's computed target) rather than
		// CurrentReplicas: it flags the ceiling the moment the autoscaler
		// wants more pods, even before they run. The MaxReplicas > 0 guard
		// keeps a not-yet-reconciled HPA (status all zeros) from matching.
		case h.Spec.MaxReplicas > 0 && h.Status.DesiredReplicas >= h.Spec.MaxReplicas:
			out = append(out, makeFinding(h.Namespace, "HorizontalPodAutoscaler", h.Name, "hpa_at_max", security.SeverityMedium,
				"HPA at its ceiling",
				fmt.Sprintf("HPA %q is currently at its maxReplicas ceiling (%d); additional load cannot scale out.", h.Name, h.Spec.MaxReplicas)))
		}
	}
	return out
}

// orphanPDBFindings flags PDBs whose selector matches no Deployment,
// StatefulSet, or DaemonSet pod template. Requires every workload list to
// have succeeded — with a partial view "matches nothing" cannot be concluded.
func (d *clusterData) orphanPDBFindings() []security.Finding {
	if !d.pdbsOK || !d.workloadsOK {
		return nil
	}
	var out []security.Finding
	for i := range d.pdbs {
		p := &d.pdbs[i]
		if systemNamespaces[p.Namespace] {
			continue
		}
		matched := false
		for j := range d.workloads {
			w := &d.workloads[j]
			if w.namespace == p.Namespace && pdbSelectorMatches(p, w.podLabels) {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		out = append(out, makeFinding(p.Namespace, "PodDisruptionBudget", p.Name, "orphan_pdb", security.SeverityLow,
			"PDB matches no workload",
			fmt.Sprintf("PodDisruptionBudget %q selects no Deployment, StatefulSet, or DaemonSet pods; it protects nothing.", p.Name)))
	}
	return out
}
