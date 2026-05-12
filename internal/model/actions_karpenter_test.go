package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestKarpenterActions_NodeClaim locks in the per-kind action set for
// karpenter.sh/NodeClaim. The three Karpenter-specific verbs — Disrupt,
// Cordon Node, Drain Node — must be present; Disrupt is the
// type-to-confirm path (delete the NodeClaim → Karpenter terminates the
// underlying node), Cordon Node / Drain Node resolve status.nodeName
// and forward to the standard kubectl helpers.
func TestKarpenterActions_NodeClaim(t *testing.T) {
	items := ActionsForKind("NodeClaim")
	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, it.Label)
	}
	assert.Contains(t, labels, "Disrupt",
		"NodeClaim must offer Disrupt (type-to-confirm; deletes the NodeClaim so Karpenter terminates the node)")
	assert.Contains(t, labels, "Cordon Node",
		"NodeClaim must offer Cordon Node (resolves status.nodeName then runs kubectl cordon)")
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
// the resource visible and editable.
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
	// Disrupt is for NodeClaim, not NodePool.
	assert.NotContains(t, labels, "Disrupt",
		"NodePool does not offer Disrupt — that verb only applies to a single NodeClaim")
}

// TestKarpenterActions_EC2NodeClass surfaces only generic actions.
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
}
