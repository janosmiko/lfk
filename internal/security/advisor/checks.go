package advisor

import (
	"fmt"
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/janosmiko/lfk/internal/security"
)

func makeFinding(ns, kind, name, check string, sev security.Severity, title, summary string) security.Finding {
	return security.Finding{
		ID:       fmt.Sprintf("advisor/%s/%s/%s/%s", ns, kind, name, check),
		Source:   "advisor",
		Category: security.CategoryReliability,
		Severity: sev,
		Title:    title,
		Resource: security.ResourceRef{Namespace: ns, Kind: kind, Name: name},
		Summary:  summary,
		Labels:   map[string]string{"check": check},
	}
}

// findings runs every check whose data dependencies loaded successfully.
func (d *clusterData) findings() []security.Finding {
	groups := [][]security.Finding{
		d.namespaceFindings(),
		d.workloadFindings(),
		d.pdbFindings(),
		d.hpaFindings(),
		d.quotaFindings(),
		d.hpaConfigFindings(),
		d.orphanPDBFindings(),
		d.lifecycleFindings(),
	}
	total := 0
	for _, g := range groups {
		total += len(g)
	}
	out := make([]security.Finding, 0, total)
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// namespaceFindings flags namespaces without a ResourceQuota / LimitRange.
// Each check requires its own list to have succeeded so an unlistable type
// cannot produce blanket false positives.
func (d *clusterData) namespaceFindings() []security.Finding {
	var out []security.Finding
	for _, ns := range d.namespaces {
		if systemNamespaces[ns] {
			continue
		}
		if d.quotasOK && !d.quotaNS[ns] {
			out = append(out, makeFinding(ns, "Namespace", ns, "namespace_no_quota", security.SeverityLow,
				"no ResourceQuota",
				fmt.Sprintf("Namespace %q has no ResourceQuota; unbounded workloads can exhaust cluster capacity.", ns)))
		}
		if d.limitsOK && !d.limitNS[ns] {
			out = append(out, makeFinding(ns, "Namespace", ns, "namespace_no_limitrange", security.SeverityLow,
				"no LimitRange",
				fmt.Sprintf("Namespace %q has no LimitRange; containers without explicit requests/limits get none by default.", ns)))
		}
	}
	return out
}

// hpaTargets returns the set of "ns/kind/name" keys that an HPA scales.
func (d *clusterData) hpaTargets() map[string]bool {
	targets := make(map[string]bool, len(d.hpas))
	for i := range d.hpas {
		h := &d.hpas[i]
		targets[h.Namespace+"/"+h.Spec.ScaleTargetRef.Kind+"/"+h.Spec.ScaleTargetRef.Name] = true
	}
	return targets
}

func workloadKey(w *workload) string { return w.namespace + "/" + w.kind + "/" + w.name }

func (d *clusterData) workloadFindings() []security.Finding {
	var out []security.Finding
	hpaTargets := d.hpaTargets()
	for i := range d.workloads {
		w := &d.workloads[i]
		if systemNamespaces[w.namespace] {
			continue
		}
		// replicas: 1 is meaningless when an HPA owns the replica count.
		// When HPAs are unlistable, hpaTargets is empty and the check still
		// fires — a rare false positive on an HPA-managed workload beats
		// silently dropping the check for RBAC-restricted users.
		if w.replicas == 1 && !hpaTargets[workloadKey(w)] {
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "single_replica", security.SeverityLow,
				"single replica",
				fmt.Sprintf("%s %q runs a single replica; any disruption causes downtime.", w.kind, w.name)))
		}
		if d.pdbsOK && w.replicas >= 2 && !d.pdbMatches(w) {
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "no_pdb", security.SeverityMedium,
				"no PodDisruptionBudget",
				fmt.Sprintf("%s %q has %d replicas but no matching PodDisruptionBudget; node drains can take all replicas down at once.", w.kind, w.name, w.replicas)))
		}
		if names := containersWithoutProbes(w.containers); len(names) > 0 {
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "missing_probes", security.SeverityLow,
				"missing health probes",
				fmt.Sprintf("Container(s) %s in %s %q define no liveness, readiness, or startup probe.", strings.Join(names, ", "), w.kind, w.name)))
		}
		if names := containersWithoutRequests(w.containers, corev1.ResourceCPU, corev1.ResourceMemory); len(names) > 0 {
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "missing_requests", security.SeverityLow,
				"missing resource requests",
				fmt.Sprintf("Container(s) %s in %s %q set no CPU/memory requests; the scheduler places them blind.", strings.Join(names, ", "), w.kind, w.name)))
		}
		if w.replicas >= 2 && !w.spreadConfigured {
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "no_topology_spread", security.SeverityLow,
				"replicas not spread",
				fmt.Sprintf("%s %q has %d replicas but no topologySpreadConstraints or pod anti-affinity; all replicas can land on one node.", w.kind, w.name, w.replicas)))
		}
		if reason := badRolloutStrategy(w); reason != "" {
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "bad_rollout_strategy", security.SeverityLow,
				"rollout causes downtime",
				fmt.Sprintf("%s %q uses %s; every rollout takes all replicas down at once.", w.kind, w.name, reason)))
		}
		if names := unboundedEmptyDirs(w.volumes); len(names) > 0 {
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "emptydir_no_sizelimit", security.SeverityLow,
				"emptyDir without sizeLimit",
				fmt.Sprintf("Volume(s) %s in %s %q are emptyDirs with no sizeLimit; runaway writes cause node disk-pressure evictions.", strings.Join(names, ", "), w.kind, w.name)))
		}
		if names := containersWithIdenticalProbes(w.containers); len(names) > 0 {
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "identical_probes", security.SeverityMedium,
				"identical liveness and readiness probes",
				fmt.Sprintf("Container(s) %s in %s %q use the same probe for liveness and readiness; under load the pod is restarted instead of just shedding traffic.", strings.Join(names, ", "), w.kind, w.name)))
		}
	}
	return out
}

// badRolloutStrategy returns a non-empty reason when a multi-replica
// Deployment's rollout takes every replica down: strategy Recreate, or a
// RollingUpdate that allows 100% (or >= replicas) unavailable. An empty
// strategy Type (possible only pre-API-defaulting, e.g. fakes) falls through
// to the RollingUpdate branch, matching the API server's default.
func badRolloutStrategy(w *workload) string {
	if w.strategy == nil || w.replicas < 2 {
		return ""
	}
	if w.strategy.Type == appsv1.RecreateDeploymentStrategyType {
		return "strategy: Recreate"
	}
	ru := w.strategy.RollingUpdate
	if ru == nil || ru.MaxUnavailable == nil {
		return ""
	}
	mu := ru.MaxUnavailable
	if mu.Type == intstr.String && mu.StrVal == "100%" {
		return "maxUnavailable: 100%"
	}
	if mu.Type == intstr.Int && int32(mu.IntValue()) >= w.replicas {
		return fmt.Sprintf("maxUnavailable: %d with %d replicas", mu.IntValue(), w.replicas)
	}
	return ""
}

// unboundedEmptyDirs returns names of emptyDir volumes without an effective
// sizeLimit — unset or explicitly zero (which the kubelet does not enforce).
func unboundedEmptyDirs(volumes []corev1.Volume) []string {
	var names []string
	for i := range volumes {
		v := &volumes[i]
		if v.EmptyDir != nil && (v.EmptyDir.SizeLimit == nil || v.EmptyDir.SizeLimit.IsZero()) {
			names = append(names, v.Name)
		}
	}
	return names
}

// containersWithIdenticalProbes returns names of containers whose liveness
// and readiness probes are exactly equal — the readiness failure mode
// (shed traffic) is replaced by the liveness one (restart).
func containersWithIdenticalProbes(containers []corev1.Container) []string {
	var names []string
	for i := range containers {
		c := &containers[i]
		if c.LivenessProbe == nil || c.ReadinessProbe == nil {
			continue
		}
		if reflect.DeepEqual(c.LivenessProbe, c.ReadinessProbe) {
			names = append(names, c.Name)
		}
	}
	return names
}

// pdbMatches reports whether any PDB in the workload's namespace selects the
// workload's pod template labels.
func (d *clusterData) pdbMatches(w *workload) bool {
	for i := range d.pdbs {
		p := &d.pdbs[i]
		if p.Namespace != w.namespace {
			continue
		}
		if pdbSelectorMatches(p, w.podLabels) {
			return true
		}
	}
	return false
}

// pdbSelectorMatches mirrors policy/v1 semantics: a nil selector matches
// nothing, an empty selector matches every pod in the namespace.
func pdbSelectorMatches(p *policyv1.PodDisruptionBudget, podLabels map[string]string) bool {
	if p.Spec.Selector == nil {
		return false
	}
	sel, err := metav1.LabelSelectorAsSelector(p.Spec.Selector)
	if err != nil {
		return false
	}
	return sel.Matches(labels.Set(podLabels))
}

// pdbFindings flags PDBs that block node drains: maxUnavailable 0 (or "0%"),
// minAvailable "100%", or an integer minAvailable >= a matched workload's
// replica count.
func (d *clusterData) pdbFindings() []security.Finding {
	if !d.pdbsOK {
		return nil
	}
	var out []security.Finding
	for i := range d.pdbs {
		p := &d.pdbs[i]
		if systemNamespaces[p.Namespace] {
			continue
		}
		if reason := pdbBlocksDrain(p, d.workloads); reason != "" {
			out = append(out, makeFinding(p.Namespace, "PodDisruptionBudget", p.Name, "pdb_blocks_drain", security.SeverityMedium,
				"PDB blocks drains",
				fmt.Sprintf("PodDisruptionBudget %q permits zero voluntary disruptions (%s); draining a node running its pods hangs until it is changed.", p.Name, reason)))
		}
	}
	return out
}

// pdbBlocksDrain returns a non-empty reason when the PDB can never allow a
// voluntary disruption.
func pdbBlocksDrain(p *policyv1.PodDisruptionBudget, workloads []workload) string {
	if mu := p.Spec.MaxUnavailable; mu != nil {
		if mu.Type == intstr.Int && mu.IntValue() == 0 {
			return "maxUnavailable: 0"
		}
		if mu.Type == intstr.String && (mu.StrVal == "0" || mu.StrVal == "0%") {
			return "maxUnavailable: " + mu.StrVal
		}
	}
	ma := p.Spec.MinAvailable
	if ma == nil {
		return ""
	}
	if ma.Type == intstr.String && ma.StrVal == "100%" {
		return "minAvailable: 100%"
	}
	if ma.Type == intstr.Int {
		for i := range workloads {
			w := &workloads[i]
			if w.namespace != p.Namespace || !pdbSelectorMatches(p, w.podLabels) {
				continue
			}
			// Skip the DaemonSet replicas-0 sentinel: minAvailable >= 0 is
			// vacuously true and replica math is meaningless per-node.
			if w.replicas <= 0 {
				continue
			}
			if int32(ma.IntValue()) >= w.replicas {
				return fmt.Sprintf("minAvailable: %d with %d replicas", ma.IntValue(), w.replicas)
			}
		}
	}
	return ""
}

// hpaFindings flags HPAs scaling on resource utilization while the target
// workload's containers set no request for that resource — the HPA cannot
// compute utilization and never scales.
func (d *clusterData) hpaFindings() []security.Finding {
	if !d.hpasOK {
		return nil
	}
	byKey := make(map[string]*workload, len(d.workloads))
	for i := range d.workloads {
		byKey[workloadKey(&d.workloads[i])] = &d.workloads[i]
	}
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
		for _, m := range h.Spec.Metrics {
			if m.Type != autoscalingv2.ResourceMetricSourceType || m.Resource == nil {
				continue
			}
			if m.Resource.Target.Type != autoscalingv2.UtilizationMetricType {
				continue
			}
			if names := containersWithoutRequests(w.containers, m.Resource.Name); len(names) > 0 {
				out = append(out, makeFinding(w.namespace, w.kind, w.name, "hpa_no_requests", security.SeverityMedium,
					"HPA target missing requests",
					fmt.Sprintf("HPA %q scales on %s utilization but container(s) %s in %s %q set no %s request; the HPA cannot compute utilization.",
						h.Name, m.Resource.Name, strings.Join(names, ", "), w.kind, w.name, m.Resource.Name)))
				break // one finding per HPA/workload pair is enough
			}
		}
	}
	return out
}

// containersWithoutProbes returns names of containers defining no probe at
// all (liveness, readiness, or startup).
func containersWithoutProbes(containers []corev1.Container) []string {
	var names []string
	for i := range containers {
		c := &containers[i]
		if c.LivenessProbe == nil && c.ReadinessProbe == nil && c.StartupProbe == nil {
			names = append(names, c.Name)
		}
	}
	return names
}

// containersWithoutRequests returns names of containers missing a request
// for any of the given resources.
func containersWithoutRequests(containers []corev1.Container, resources ...corev1.ResourceName) []string {
	var names []string
	for i := range containers {
		c := &containers[i]
		for _, r := range resources {
			if q, ok := c.Resources.Requests[r]; !ok || q.IsZero() {
				names = append(names, c.Name)
				break
			}
		}
	}
	return names
}
