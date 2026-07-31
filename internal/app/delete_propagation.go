package app

import (
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// deletePropagation returns the cascade policy for the pending delete. An
// unset field means no confirm dialog set one (a caller that deletes without
// opening the overlay), so fall back to the configured default.
func (m Model) deletePropagation() model.DeletePropagation {
	if m.confirmPropagation == "" {
		return ui.ConfigDeletePropagationPolicy
	}
	return m.confirmPropagation
}

// resetDeletePropagation seeds the policy from config. Called when a delete
// confirm opens so a policy chosen for a previous delete never carries over
// into the next one.
func (m *Model) resetDeletePropagation() {
	m.confirmPropagation = ui.ConfigDeletePropagationPolicy
}

// resetForceDeletePropagation seeds the policy for a force delete. The value is
// clamped because that path runs through kubectl, which cannot express None.
func (m *Model) resetForceDeletePropagation() {
	m.confirmPropagation = ui.ConfigDeletePropagationPolicy.Cascading()
}

// cycleDeletePropagation advances the policy to the next one in the Tab order.
func (m *Model) cycleDeletePropagation() {
	m.confirmPropagation = m.deletePropagation().Cycle()
}

// cycleForceDeletePropagation advances the policy, skipping None.
func (m *Model) cycleForceDeletePropagation() {
	m.confirmPropagation = m.deletePropagation().CycleCascading()
}

// cascadeConfirmHints builds a confirm overlay's hint row, inserting the
// cascade hotkey between confirm and cancel only where Tab does something.
func cascadeConfirmHints(confirmKey, cancelKey string, showCascade bool) []ui.HintEntry {
	hints := []ui.HintEntry{{Key: confirmKey, Desc: "confirm"}}
	if showCascade {
		hints = append(hints, ui.HintEntry{Key: "tab", Desc: "cascade"})
	}
	return append(hints, ui.HintEntry{Key: cancelKey, Desc: "cancel"})
}

// forceDeleteArgs builds the kubectl argv for a force delete. Shared by the
// single and bulk paths so the flags cannot drift apart. cascade is resolved
// through Cascading(), since kubectl rejects --cascade=none.
func forceDeleteArgs(rt model.ResourceTypeEntry, name, kubectlCtx, namespace string, cascade model.DeletePropagation) []string {
	args := []string{
		"delete", kubectlResourceArg(rt), name, "--context", kubectlCtx,
		"--grace-period=0", "--force", "--cascade=" + cascade.KubectlCascade(),
	}
	if rt.Namespaced {
		args = append(args, "-n", namespace)
	}
	return args
}

// forceDeleteConfirmShowsPolicy reports whether the open type-to-confirm
// overlay is a force delete that cascades. Force Finalize, Finalizer Remove,
// and Disrupt are not cascading deletes, and a Longhorn node force delete runs
// through a dedicated webhook-aware API call rather than kubectl delete.
func (m Model) forceDeleteConfirmShowsPolicy() bool {
	return m.pendingAction == "Force Delete" && !model.IsLonghornNode(m.actionCtx.resourceType)
}

// deleteConfirmShowsPolicy reports whether the open confirm overlay is a
// delete that cascades, so the overlay and hint bar only advertise Tab where
// it does something. Helm uninstall and the non-delete confirms (Drain,
// Restart, Evict Replicas, Apply Taints) do not go through DeleteResource.
func (m Model) deleteConfirmShowsPolicy() bool {
	switch m.pendingAction {
	case "", "Delete":
	default:
		return false
	}
	return m.actionCtx.resourceType.APIGroup != "_helm"
}
