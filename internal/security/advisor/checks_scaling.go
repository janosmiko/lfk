// Scaling/disruption interaction checks: PDBs that deadlock against HPA
// minimums, PDBs that strand crashlooping pods during drains, and manifests
// that fight their autoscaler over the replica count.

package advisor

import (
	"encoding/json"
	"fmt"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/janosmiko/lfk/internal/security"
)

// scalingFindings runs the PDB/HPA interaction checks.
func (d *clusterData) scalingFindings() []security.Finding {
	return slices.Concat(
		d.pdbUnhealthyPolicyFindings(),
		d.pdbVsHPAFindings(),
		d.staticReplicasFindings(),
	)
}

// pdbUnhealthyPolicyFindings flags PDBs without unhealthyPodEvictionPolicy.
// The default (IfHealthyBudget) refuses to evict already-unhealthy pods when
// the budget is not met, so a drain can hang on a crashlooping workload that
// the eviction could not make any less available. AlwaysAllow (1.27+) fixes it.
func (d *clusterData) pdbUnhealthyPolicyFindings() []security.Finding {
	if !d.pdbsOK {
		return nil
	}
	var out []security.Finding
	for i := range d.pdbs {
		p := &d.pdbs[i]
		if systemNamespaces[p.Namespace] || p.Spec.UnhealthyPodEvictionPolicy != nil {
			continue
		}
		out = append(out, makeFinding(p.Namespace, "PodDisruptionBudget", p.Name, "pdb_no_unhealthy_policy", security.SeverityLow,
			"no unhealthyPodEvictionPolicy",
			fmt.Sprintf("PodDisruptionBudget %q does not set unhealthyPodEvictionPolicy; the IfHealthyBudget default can block draining pods that are already unhealthy. Set AlwaysAllow (Kubernetes 1.27+).", p.Name)))
	}
	return out
}

// pdbVsHPAFindings flags PDBs whose integer minAvailable exceeds the
// minReplicas of an HPA scaling the same workload: when the HPA scales to
// its minimum, the PDB permits zero disruptions and drains deadlock.
// minAvailable == minReplicas is deliberately not flagged (too common, and
// pdb_blocks_drain covers the current-replica case); percentage minAvailable
// is skipped because its pod count depends on the live scale.
func (d *clusterData) pdbVsHPAFindings() []security.Finding {
	if !d.pdbsOK || !d.hpasOK {
		return nil
	}
	byKey := d.workloadByKey()
	// One finding per PDB: a selector matching several HPA-scaled workloads
	// must not emit duplicate finding IDs.
	emitted := map[string]bool{}
	var out []security.Finding
	for i := range d.hpas {
		h := &d.hpas[i]
		if systemNamespaces[h.Namespace] {
			continue
		}
		w, ok := byKey[h.Namespace+"/"+h.Spec.ScaleTargetRef.Kind+"/"+h.Spec.ScaleTargetRef.Name]
		if !ok {
			continue
		}
		minReplicas := int32(1)
		if h.Spec.MinReplicas != nil {
			minReplicas = *h.Spec.MinReplicas
		}
		for j := range d.pdbs {
			p := &d.pdbs[j]
			if p.Namespace != w.namespace || !pdbSelectorMatches(p, w.podLabels) || emitted[p.Namespace+"/"+p.Name] {
				continue
			}
			ma := p.Spec.MinAvailable
			if ma == nil || ma.Type != intstr.Int || int32(ma.IntValue()) <= minReplicas {
				continue
			}
			emitted[p.Namespace+"/"+p.Name] = true
			out = append(out, makeFinding(p.Namespace, "PodDisruptionBudget", p.Name, "pdb_vs_hpa_min", security.SeverityMedium,
				"PDB minAvailable above HPA minimum",
				fmt.Sprintf("PodDisruptionBudget %q requires minAvailable: %d but HPA %q can scale %s %q down to %d replicas; at minimum scale no disruption is allowed and drains hang.",
					p.Name, ma.IntValue(), h.Name, w.kind, w.name, minReplicas)))
		}
	}
	return out
}

// staticReplicasFindings flags HPA-scaled workloads whose .spec.replicas is
// still owned by another field manager (helm, kubectl, a GitOps controller):
// every re-apply of that manifest resets the replica count and fights the
// autoscaler.
func (d *clusterData) staticReplicasFindings() []security.Finding {
	if !d.hpasOK {
		return nil
	}
	byKey := d.workloadByKey()
	var out []security.Finding
	for i := range d.hpas {
		h := &d.hpas[i]
		if systemNamespaces[h.Namespace] {
			continue
		}
		w, ok := byKey[h.Namespace+"/"+h.Spec.ScaleTargetRef.Kind+"/"+h.Spec.ScaleTargetRef.Name]
		if !ok || w.staticReplicasOwner == "" {
			continue
		}
		out = append(out, makeFinding(w.namespace, w.kind, w.name, "hpa_static_replicas", security.SeverityLow,
			"manifest pins replicas under an HPA",
			fmt.Sprintf("%s %q is scaled by HPA %q but field manager %q still owns spec.replicas; re-applying that manifest resets the replica count and fights the autoscaler. Remove replicas from the manifest.",
				w.kind, w.name, h.Name, w.staticReplicasOwner)))
	}
	return out
}

// staticReplicasOwner returns the field manager that owns .spec.replicas
// through a whole-object write — i.e. a manifest that re-applies. Writes
// through the scale subresource (the HPA controller, kubectl scale) are
// excluded by their Subresource field rather than by manager name, which
// varies across Kubernetes versions (kube-controller-manager pre-1.24,
// horizontal-pod-autoscaler later); both names are excluded as a fallback
// for entries recorded before subresource tracking. Empty when no manifest
// pins the field. Malformed managedFields entries are skipped — they must
// not invent findings.
func staticReplicasOwner(entries []metav1.ManagedFieldsEntry) string {
	for i := range entries {
		e := &entries[i]
		if e.Subresource != "" || e.FieldsV1 == nil ||
			e.Manager == "kube-controller-manager" || e.Manager == "horizontal-pod-autoscaler" {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(e.FieldsV1.GetRawBytes(), &fields); err != nil {
			continue
		}
		spec, ok := fields["f:spec"]
		if !ok {
			continue
		}
		var specFields map[string]json.RawMessage
		if err := json.Unmarshal(spec, &specFields); err != nil {
			continue
		}
		if _, ok := specFields["f:replicas"]; ok {
			return e.Manager
		}
	}
	return ""
}
