package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// costConfirmModel is the box from the kyverno case that started this: a
// three-replica Deployment with four ReplicaSets under it, and a budget the
// delete would breach.
func costConfirmModel() Model {
	m := Model{width: 80, height: 24, pendingAction: "Delete", confirmAction: "web"}
	m.confirmPropagation = model.DeletePropagationBackground
	m.actionCtx.kind = "Deployment"
	m.blast.radius = &k8s.BlastRadius{
		Evicting: 3, ReadyBefore: 3, ReadyAfter: 0, Violation: true,
		PDBs: []k8s.PDBImpact{{
			Name: "kyverno", AllowedBefore: 2, AllowedAfter: -1, Evicting: 3, Violated: true,
		}},
	}
	m.deps.count = &k8s.DependentCount{
		Total: 7, ByKind: map[string]int{"ReplicaSet": 4, "Pod": 3},
	}
	return m
}

func confirmBox(m Model) string {
	out, _, _, _ := m.renderOverlayConfirm()
	return stripANSI(out)
}

func TestRenderOverlayConfirm_ShowsTheThreeCostRows(t *testing.T) {
	box := confirmBox(costConfirmModel())

	assert.Contains(t, box, "Scope:")
	assert.Contains(t, box, "4 replicasets, 3 pods")
	assert.Contains(t, box, "Availability:")
	assert.Contains(t, box, "0 of 3 ready after")
	assert.Contains(t, box, "Risk:")
	assert.Contains(t, box, "kyverno allows 2 at once")
}

func TestRenderOverlayConfirm_TabRewritesEveryRowInPlace(t *testing.T) {
	m := costConfirmModel()

	// Background: the collateral goes with the target.
	assert.Contains(t, confirmBox(m), "4 replicasets, 3 pods")

	m.cycleDeletePropagation() // Foreground
	assert.Contains(t, confirmBox(m), "4 replicasets, 3 pods")

	m.cycleDeletePropagation() // Orphan
	orphan := confirmBox(m)
	assert.Contains(t, orphan, "the deployment only")
	assert.Contains(t, orphan, "unchanged, the 3 pods keep running")
	assert.Contains(t, orphan, "left with no owner")

	m.cycleDeletePropagation() // None
	none := confirmBox(m)
	assert.Contains(t, none, "plus 4 replicasets, 3 pods")
	assert.Contains(t, none, "depends on the server default")

	// Tab never closed the dialog.
	assert.Contains(t, none, "Delete web?")
}

func TestRenderOverlayConfirm_NoRowContradictsAnother(t *testing.T) {
	// The defect that prompted the restructure: under Orphan the box stated
	// that the pods go and that the same pods stay.
	m := costConfirmModel()
	m.confirmPropagation = model.DeletePropagationOrphan

	box := confirmBox(m)

	assert.NotContains(t, box, "ready after",
		"nothing is evicted, so there is no readiness to lose")
	assert.NotContains(t, box, "allows 2 at once",
		"nothing is evicted, so no budget is touched")
}

func TestRenderOverlayConfirm_OnePlaceholderCoversBothFetches(t *testing.T) {
	m := costConfirmModel()
	m.deps.loading = true
	m.deps.count = nil

	box := confirmBox(m)

	assert.Contains(t, box, "working out what this costs...")
	assert.NotContains(t, box, "0 of 3 ready after",
		"no row may render against a fetch that is still in flight")
}

func TestRenderOverlayConfirm_BarePodDeleteHasNoScopeRow(t *testing.T) {
	m := Model{width: 80, height: 24, pendingAction: "Delete", confirmAction: "web-1"}
	m.confirmPropagation = model.DeletePropagationBackground
	m.actionCtx.kind = "Pod"
	m.blast.radius = &k8s.BlastRadius{Evicting: 1, ReadyBefore: 1, ReadyAfter: 0}

	box := confirmBox(m)

	assert.NotContains(t, box, "Scope:", "nothing but the pod goes")
	assert.Contains(t, box, "0 of 1 ready after")
}

func TestRenderOverlayConfirmType_ShowsTheOwnerSide(t *testing.T) {
	m := Model{width: 80, height: 24, pendingAction: "Force Delete", confirmTitle: "Confirm Force Delete"}
	m.confirmPropagation = model.DeletePropagationBackground
	m.actionCtx.kind = "Deployment"
	m.deps.count = &k8s.DependentCount{Total: 1, ByKind: map[string]int{"Pod": 1}}

	out, _, h, _ := m.renderOverlayConfirmType()

	assert.Contains(t, stripANSI(out), "1 pod")
	assert.Positive(t, h)
}
