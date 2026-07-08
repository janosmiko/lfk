package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestKarpenterActions_NodeClaim locks in the per-kind action set for
// karpenter.sh/NodeClaim. The four Karpenter-specific verbs — Disrupt,
// Cordon / Uncordon / Drain Node — must be present; Disrupt is the
// type-to-confirm path (delete the NodeClaim → Karpenter terminates
// the underlying node), and Cordon / Uncordon / Drain Node resolve
// status.nodeName and forward to the standard kubectl helpers.
func TestKarpenterActions_NodeClaim(t *testing.T) {
	items := ActionsForKind("NodeClaim")
	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, it.Label)
	}
	assert.Contains(t, labels, "Disrupt",
		"NodeClaim must offer Disrupt (type-to-confirm; deletes the NodeClaim so Karpenter terminates the node)")
	assert.Contains(t, labels, "Cordon/Uncordon Node",
		"NodeClaim must offer Cordon/Uncordon Node (resolves status.nodeName then toggles spec.unschedulable)")
	assert.Contains(t, labels, "Drain Node",
		"NodeClaim must offer Drain Node (resolves status.nodeName then runs the drain flow)")
	// Standard actions still present.
	assert.Contains(t, labels, "Describe")
	assert.Contains(t, labels, "Edit")
	assert.Contains(t, labels, "Delete")
	assert.Contains(t, labels, "Events")
}

// TestKarpenterActions_NodePool keeps the action menu minimal for now —
// Disable/Enable on spec.disruption.budgets has user-data overlap and
// is deferred to a follow-up. Generic Describe/Edit/Delete/Events keep
// the resource visible and editable. Disrupt / Cordon Node / Drain Node
// are explicitly absent — those verbs only make sense from a NodeClaim
// row where status.nodeName binds the action to a concrete node.
func TestKarpenterActions_NodePool(t *testing.T) {
	items := ActionsForKind("NodePool")
	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, it.Label)
	}
	assert.Contains(t, labels, "Describe")
	assert.Contains(t, labels, "Edit")
	assert.Contains(t, labels, "Delete")
	assert.Contains(t, labels, "Events")
	assert.NotContains(t, labels, "Disrupt",
		"NodePool does not offer Disrupt — that verb only applies to a single NodeClaim")
	assert.NotContains(t, labels, "Cordon/Uncordon Node",
		"NodePool has no single underlying node to cordon")
	assert.NotContains(t, labels, "Drain Node",
		"NodePool has no single underlying node to drain")
}

// TestKarpenterActions_EC2NodeClass surfaces only generic actions. The
// node-bound verbs (Disrupt / Cordon / Uncordon / Drain Node) live on
// NodeClaim where status.nodeName resolves to a concrete node.
func TestKarpenterActions_EC2NodeClass(t *testing.T) {
	items := ActionsForKind("EC2NodeClass")
	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, it.Label)
	}
	assert.Contains(t, labels, "Describe")
	assert.Contains(t, labels, "Edit")
	assert.Contains(t, labels, "Delete")
	assert.Contains(t, labels, "Events")
	assert.NotContains(t, labels, "Disrupt",
		"EC2NodeClass is a node-launch template, not a node — Disrupt does not apply")
	assert.NotContains(t, labels, "Cordon/Uncordon Node",
		"EC2NodeClass is a node-launch template, not a node — Cordon/Uncordon Node does not apply")
	assert.NotContains(t, labels, "Drain Node",
		"EC2NodeClass is a node-launch template, not a node — Drain Node does not apply")
}
