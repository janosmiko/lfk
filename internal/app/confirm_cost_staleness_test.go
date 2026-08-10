package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

func TestBlastRadiusReset_DropsAFetchThatIsStillInFlight(t *testing.T) {
	// Cancelling a confirm clears the figures, but the fetch it started is
	// still running. Without a fresh request number that answer lands on
	// whatever dialog the user opened next.
	m := Model{}
	m.beginBlastRadius()
	m.blast.reset()

	out, _ := m.updateBlastRadiusLoaded(blastRadiusLoadedMsg{
		req: 1, radius: &k8s.BlastRadius{Evicting: 3},
	})

	assert.Nil(t, out.(Model).blast.radius,
		"a cancelled fetch must not repopulate the next dialog")
}

func TestDependentsReset_DropsAWalkThatIsStillInFlight(t *testing.T) {
	m := Model{}
	m.beginDependents()
	m.deps.reset()

	out, _ := m.updateDependentsLoaded(dependentsLoadedMsg{
		req: 1, count: &k8s.DependentCount{Total: 9},
	})

	assert.Nil(t, out.(Model).deps.count,
		"a cancelled walk must not repopulate the next dialog")
}

func TestRenderOverlayConfirmType_NeverShowsABlastRadius(t *testing.T) {
	// Force delete never fetches one, so anything left in m.blast belongs to
	// a different resource and must not reach this box.
	m := Model{width: 80, height: 24, pendingAction: "Force Delete", confirmTitle: "Confirm Force Delete"}
	m.confirmPropagation = model.DeletePropagationBackground
	m.actionCtx.kind = "Deployment"
	m.deps.count = &k8s.DependentCount{Total: 1, ByKind: map[string]int{"Pod": 1}}
	m.blast.radius = &k8s.BlastRadius{
		Evicting: 7, ReadyBefore: 7, ReadyAfter: 0,
		PDBs: []k8s.PDBImpact{{Name: "somebody-elses-pdb", AllowedBefore: 1, Violated: true}},
	}

	out, _, _, _ := m.renderOverlayConfirmType()
	box := stripANSI(out)

	assert.NotContains(t, box, "somebody-elses-pdb")
	assert.NotContains(t, box, "of 7 ready after")
	assert.Contains(t, box, "1 pod", "the owner side still shows")
}

func TestDirectActionDelete_EscalationArmsAFreshCount(t *testing.T) {
	// A row already being deleted escalates straight to the type-to-confirm
	// box. It must not inherit the count from whatever confirm ran before it.
	m := basePush80Model()
	m.nav.ResourceType.Kind = "Pod"
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod", Namespace: "default", Deleting: true}}
	m.setCursor(0)
	m.deps.count = &k8s.DependentCount{Total: 42, ByKind: map[string]int{"Pod": 42}}

	out, cmd := m.directActionDelete()
	got := out.(Model)

	require.Equal(t, overlayConfirmType, got.overlay)
	assert.Nil(t, got.deps.count, "the previous dialog's count must not carry over")
	assertDependentsOnlyCmd(t, cmd)
}
