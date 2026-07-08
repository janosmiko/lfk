package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsUnionAllowedActionForKind_ClosureOverMutatingActions is the
// exhaustiveness guard requested in the multi-cluster review. Every
// label in mutatingActions must have an explicit row in the
// expectedUnionPolicy table below, and every row in that table must
// correspond to a real entry in mutatingActions. Adding a new mutating
// action without updating this test triggers a fail with the offending
// label, forcing the author to decide whether the action is safe to
// run from the merged union view.
//
// The expected behaviour for each label is asserted against a small
// matrix of representative kinds (Pod, Deployment, ConfigMap) so the
// kind-specific allow-list in isUnionAllowedActionForKind stays
// honestly described.
func TestIsUnionAllowedActionForKind_ClosureOverMutatingActions(t *testing.T) {
	// allowedFor lists the kinds for which the label is allowed in
	// union mode; an empty slice means "never allowed". The set of
	// kinds we probe is the union of all values across the table.
	expectedUnionPolicy := map[string][]string{
		// Pod-only destructive verbs: deleting a Deployment-level row
		// in union mode would silently delete its Pods across clusters
		// but not the owning workload — refused.
		"Delete":         {"Pod"},
		"Force Delete":   {"Pod"},
		"Force Finalize": {"Pod"},

		// Restart is gated by model.IsRestartableKind — Deployment,
		// StatefulSet, DaemonSet, ReplicaSet, ReplicationController.
		"Restart": {"Deployment", "StatefulSet", "DaemonSet"},

		// Port Forward is fetch-shaped from the cluster's perspective
		// (it opens a pod-network tunnel) but the dispatcher classifies
		// it as mutating because it spawns a long-lived subprocess.
		// Allowed for the five kinds the action handler supports.
		"Port Forward": {"Pod", "Service", "Deployment", "StatefulSet", "DaemonSet"},

		// Port Forward & Open chains a port forward with a browser open;
		// only Services expose it (Ctrl+O / action menu).
		"Port Forward & Open": {"Service"},

		// Everything below is hard-blocked at the union sentinel.
		// These either need a single API target (Scale, Rollback,
		// Edit) or are kind-specific operations (Cordon, Drain,
		// ArgoCD, Argo Workflows, cert-manager, KEDA, Helm) that
		// don't compose meaningfully across clusters.
		"Finalizer Remove":     nil,
		"Edit":                 nil,
		"Secret Editor":        nil,
		"ConfigMap Editor":     nil,
		"Scale":                nil,
		"Rollback":             nil,
		"Exec":                 nil,
		"Attach":               nil,
		"Shell":                nil,
		"Debug":                nil,
		"Debug Pod":            nil,
		"Debug Mount":          nil,
		"Cordon/Uncordon":      nil,
		"Drain":                nil,
		"Taints":               nil,
		"Apply Taints":         nil,
		"Trigger":              nil,
		"Suspend/Resume":       nil,
		"Stop":                 nil,
		"Remove":               nil,
		"Labels / Annotations": nil,
		"Permissions":          nil,
		"Configure AutoSync":   nil,
		"Auto Sync":            nil,
		"Sync":                 nil,
		"Sync (Apply Only)":    nil,
		"Terminate Sync":       nil,
		"Suspend Workflow":     nil,
		"Resume Workflow":      nil,
		"Stop Workflow":        nil,
		"Terminate Workflow":   nil,
		"Resubmit Workflow":    nil,
		"Submit Workflow":      nil,
		"Force Renew":          nil,
		"Force Refresh":        nil,
		"Pause/Unpause":        nil,
		"Reconcile":            nil,
		"Disrupt":              nil,
		"Cordon/Uncordon Node": nil,
		"Drain Node":           nil,
		"Evict Replicas":       nil,
		"Cancel Eviction":      nil,
		"Activate":             nil,
		"Edit Values":          nil,
		"Upgrade":              nil,
	}

	probeKinds := []string{"Pod", "Deployment", "StatefulSet", "DaemonSet", "Service", "ConfigMap"}

	// 1. Every mutatingActions entry has a policy row.
	for label := range mutatingActions {
		if _, ok := expectedUnionPolicy[label]; !ok {
			t.Errorf("%q is in mutatingActions but missing from the union-policy table — decide whether it is safe to run from the union sentinel and add it here", label)
		}
	}

	// 2. Every policy row corresponds to a real mutating action.
	for label := range expectedUnionPolicy {
		if !mutatingActions[label] {
			t.Errorf("%q is in the union-policy table but not in mutatingActions — remove the stale row", label)
		}
	}

	// 3. The policy is enforced: for every (label, kind) pair the
	//    result matches what the table declares.
	for label, allowedKinds := range expectedUnionPolicy {
		allowed := make(map[string]struct{}, len(allowedKinds))
		for _, k := range allowedKinds {
			allowed[k] = struct{}{}
		}
		for _, kind := range probeKinds {
			_, want := allowed[kind]
			got := isUnionAllowedActionForKind(kind, label)
			assert.Equal(t, want, got,
				"isUnionAllowedActionForKind(%q, %q) = %v, want %v", kind, label, got, want)
		}
	}
}
