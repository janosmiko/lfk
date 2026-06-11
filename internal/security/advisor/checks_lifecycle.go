// Pod-lifecycle advisor checks: probe asymmetry, zero termination grace,
// and update strategies that leave pods running stale spec.

package advisor

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/janosmiko/lfk/internal/security"
)

// lifecycleFindings runs the per-workload lifecycle checks. Operates purely
// on the workload list, so no OK-flag gating is needed beyond the lists
// having loaded (a failed list contributes no workloads).
func (d *clusterData) lifecycleFindings() []security.Finding {
	var out []security.Finding
	for i := range d.workloads {
		w := &d.workloads[i]
		if systemNamespaces[w.namespace] {
			continue
		}
		if names := containersLivenessOnly(w.containers); len(names) > 0 {
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "liveness_no_readiness", security.SeverityLow,
				"liveness probe without readiness probe",
				fmt.Sprintf("Container(s) %s in %s %q set a liveness probe but no readiness probe; a degraded pod is never removed from Service endpoints and keeps receiving traffic until the liveness probe restarts it.", strings.Join(names, ", "), w.kind, w.name)))
		}
		if w.zeroGracePeriod {
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "zero_grace_period", security.SeverityLow,
				"zero termination grace period",
				fmt.Sprintf("%s %q sets terminationGracePeriodSeconds: 0; every rollout, eviction, and drain SIGKILLs pods instantly, dropping in-flight work.", w.kind, w.name)))
		}
		if w.onDeleteUpdate {
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "ondelete_update_strategy", security.SeverityLow,
				"OnDelete update strategy",
				fmt.Sprintf("%s %q uses updateStrategy: OnDelete; pods keep running the old spec until each is deleted by hand.", w.kind, w.name)))
		}
	}
	return out
}

// containersLivenessOnly returns names of containers that define a liveness
// probe but no readiness probe. Containers with no probes at all are
// missing_probes territory and excluded here.
func containersLivenessOnly(containers []corev1.Container) []string {
	var names []string
	for i := range containers {
		c := &containers[i]
		if c.LivenessProbe != nil && c.ReadinessProbe == nil {
			names = append(names, c.Name)
		}
	}
	return names
}
