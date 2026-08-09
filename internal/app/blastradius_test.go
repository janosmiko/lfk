package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

func TestBlastRadiusNotes_LoadingShowsAPlaceholder(t *testing.T) {
	notes := blastRadiusNotes(nil, true, true)

	require.Len(t, notes, 1)
	assert.Contains(t, notes[0].Text, "checking")
	assert.False(t, notes[0].Warn, "waiting is not a warning")
}

func TestBlastRadiusNotes_NothingYetAndNotLoadingIsSilent(t *testing.T) {
	assert.Empty(t, blastRadiusNotes(nil, false, true),
		"a dialog that never asked for a blast radius must look untouched")
}

func TestBlastRadiusNotes_WorkloadShowsReplicasRemaining(t *testing.T) {
	r := &k8s.BlastRadius{Evicting: 2, ReadyBefore: 5, ReadyAfter: 3}

	notes := blastRadiusNotes(r, false, true)

	require.Len(t, notes, 1)
	assert.Contains(t, notes[0].Text, "2 pods")
	assert.Contains(t, notes[0].Text, "3 of 5 ready after")
}

func TestBlastRadiusNotes_DrainOmitsTheReplicaCount(t *testing.T) {
	r := &k8s.BlastRadius{Evicting: 12, ReadyBefore: 0, ReadyAfter: 0}

	notes := blastRadiusNotes(r, false, false)

	require.Len(t, notes, 1)
	assert.Contains(t, notes[0].Text, "12 pods")
	assert.NotContains(t, notes[0].Text, "ready after",
		"a node drain has no single workload to count replicas for")
}

func TestBlastRadiusNotes_NoBudgetSaysSo(t *testing.T) {
	r := &k8s.BlastRadius{Evicting: 1, ReadyBefore: 3, ReadyAfter: 2}

	notes := blastRadiusNotes(r, false, true)

	assert.Contains(t, notes[0].Text, "no disruption budget")
}

func TestBlastRadiusNotes_SingleBudgetShowsBeforeAndAfter(t *testing.T) {
	r := &k8s.BlastRadius{
		Evicting: 2, ReadyBefore: 5, ReadyAfter: 3,
		PDBs: []k8s.PDBImpact{{Namespace: "prod", Name: "web-pdb", AllowedBefore: 2, AllowedAfter: 0, Evicting: 2}},
	}

	notes := blastRadiusNotes(r, false, true)

	require.Len(t, notes, 1)
	assert.Contains(t, notes[0].Text, "web-pdb 2 -> 0 allowed")
	assert.False(t, notes[0].Warn)
}

func TestBlastRadiusNotes_SeveralBudgetsAreCounted(t *testing.T) {
	r := &k8s.BlastRadius{
		Evicting: 12,
		PDBs: []k8s.PDBImpact{
			{Namespace: "prod", Name: "web-pdb", AllowedBefore: 2, AllowedAfter: 1},
			{Namespace: "prod", Name: "api-pdb", AllowedBefore: 3, AllowedAfter: 2},
		},
	}

	notes := blastRadiusNotes(r, false, false)

	require.Len(t, notes, 1)
	assert.Contains(t, notes[0].Text, "2 disruption budgets",
		"naming every budget would not fit the box")
}

func TestBlastRadiusNotes_ViolationGetsItsOwnWarningLine(t *testing.T) {
	r := &k8s.BlastRadius{
		Evicting: 2, ReadyBefore: 2, ReadyAfter: 0, Violation: true,
		PDBs: []k8s.PDBImpact{
			{Namespace: "prod", Name: "web-pdb", AllowedBefore: 1, AllowedAfter: -1, Evicting: 2, Violated: true},
		},
	}

	notes := blastRadiusNotes(r, false, true)

	require.Len(t, notes, 2)
	assert.False(t, notes[0].Warn)
	assert.True(t, notes[1].Warn, "AC: a violation is marked by color, not only by words")
	assert.Contains(t, notes[1].Text, "prod/web-pdb")
	assert.Contains(t, notes[1].Text, "allows 1")
	assert.Contains(t, notes[1].Text, "needs 2")
}

func TestBlastRadiusNotes_SeveralViolationsAreListedOnOneLine(t *testing.T) {
	r := &k8s.BlastRadius{
		Evicting: 4, Violation: true,
		PDBs: []k8s.PDBImpact{
			{Namespace: "prod", Name: "web-pdb", AllowedBefore: 1, AllowedAfter: -1, Evicting: 2, Violated: true},
			{Namespace: "prod", Name: "api-pdb", AllowedBefore: 0, AllowedAfter: -2, Evicting: 2, Violated: true},
		},
	}

	notes := blastRadiusNotes(r, false, false)

	require.Len(t, notes, 2)
	assert.True(t, notes[1].Warn)
	assert.Contains(t, notes[1].Text, "prod/web-pdb")
	assert.Contains(t, notes[1].Text, "prod/api-pdb")
}

func TestBlastRadiusNotes_NothingToEvictIsStated(t *testing.T) {
	r := &k8s.BlastRadius{Evicting: 0, ReadyBefore: 0, ReadyAfter: 0}

	notes := blastRadiusNotes(r, false, true)

	require.Len(t, notes, 1)
	assert.Contains(t, notes[0].Text, "no running pods")
}

func TestBlastRadiusNotes_OnePodReadsAsSingular(t *testing.T) {
	r := &k8s.BlastRadius{Evicting: 1, ReadyBefore: 1, ReadyAfter: 0}

	notes := blastRadiusNotes(r, false, true)

	assert.Contains(t, notes[0].Text, "1 pod,")
	assert.NotContains(t, notes[0].Text, "1 pods")
}

func TestWorkloadSelectorFrom(t *testing.T) {
	raw := map[string]any{"spec": map[string]any{"selector": map[string]any{
		"matchLabels": map[string]any{"app": "web"},
	}}}

	sel := workloadSelectorFrom(raw)

	require.NotNil(t, sel)
	assert.Equal(t, map[string]string{"app": "web"}, sel.MatchLabels)
}

func TestWorkloadSelectorFrom_MatchExpressions(t *testing.T) {
	raw := map[string]any{"spec": map[string]any{"selector": map[string]any{
		"matchExpressions": []any{map[string]any{
			"key": "tier", "operator": "In", "values": []any{"front"},
		}},
	}}}

	sel := workloadSelectorFrom(raw)

	require.NotNil(t, sel)
	require.Len(t, sel.MatchExpressions, 1)
	assert.Equal(t, "tier", sel.MatchExpressions[0].Key)
	assert.Equal(t, []string{"front"}, sel.MatchExpressions[0].Values)
}

func TestWorkloadSelectorFrom_MissingOrWrongShape(t *testing.T) {
	assert.Nil(t, workloadSelectorFrom(nil))
	assert.Nil(t, workloadSelectorFrom(map[string]any{}))
	assert.Nil(t, workloadSelectorFrom(map[string]any{"spec": "not an object"}))
	assert.Nil(t, workloadSelectorFrom(map[string]any{"spec": map[string]any{"selector": 42}}))
}

func TestReadyReplicasFrom(t *testing.T) {
	assert.Equal(t, 3, readyReplicasFrom(map[string]any{
		"status": map[string]any{"readyReplicas": int64(3)},
	}))
	assert.Equal(t, 2, readyReplicasFrom(map[string]any{
		"status": map[string]any{"readyReplicas": float64(2)},
	}), "JSON decoding can hand back a float")
	assert.Equal(t, 0, readyReplicasFrom(map[string]any{"status": map[string]any{}}),
		"a workload with no ready replicas yet")
	assert.Equal(t, 0, readyReplicasFrom(nil))
}

func TestEvictedPodFromRaw(t *testing.T) {
	raw := map[string]any{
		"metadata": map[string]any{"labels": map[string]any{"app": "web"}},
		"status": map[string]any{"conditions": []any{
			map[string]any{"type": "Ready", "status": "True"},
		}},
	}

	got := evictedPodFromRaw(raw, "prod")

	assert.Equal(t, "prod", got.Namespace)
	assert.Equal(t, map[string]string{"app": "web"}, got.Labels)
	assert.True(t, got.Ready)
}

func TestEvictedPodFromRaw_NotReadyAndMissingShapes(t *testing.T) {
	notReady := map[string]any{"status": map[string]any{"conditions": []any{
		map[string]any{"type": "Ready", "status": "False"},
	}}}
	assert.False(t, evictedPodFromRaw(notReady, "prod").Ready)

	assert.False(t, evictedPodFromRaw(nil, "prod").Ready, "no object means not ready")
	assert.Equal(t, "prod", evictedPodFromRaw(nil, "prod").Namespace)
	assert.False(t, evictedPodFromRaw(map[string]any{"status": 42}, "prod").Ready)
}

func TestRenderOverlayConfirm_ShowsTheBlastRadiusLine(t *testing.T) {
	m := Model{width: 80, height: 24, pendingAction: "Delete", confirmAction: "web"}
	m.blast.radius = &k8s.BlastRadius{
		Evicting: 2, ReadyBefore: 5, ReadyAfter: 3,
		PDBs: []k8s.PDBImpact{{Namespace: "prod", Name: "web-pdb", AllowedBefore: 2, AllowedAfter: 0}},
	}

	out, _, _, _ := m.renderOverlayConfirm()

	assert.Contains(t, stripANSI(out), "web-pdb 2 -> 0 allowed")
	assert.Contains(t, stripANSI(out), "3 of 5 ready after")
}

func TestRenderOverlayConfirm_DrainLineOmitsReplicas(t *testing.T) {
	m := Model{width: 80, height: 24, pendingAction: "Drain", confirmTitle: "Confirm Drain"}
	m.blast.radius = &k8s.BlastRadius{Evicting: 12}

	out, _, _, _ := m.renderOverlayConfirm()

	assert.Contains(t, stripANSI(out), "12 pods")
	assert.NotContains(t, stripANSI(out), "ready after")
}

func TestRenderOverlayConfirm_NoBlastRadiusLeavesTheBoxAsItWas(t *testing.T) {
	m := Model{width: 80, height: 24, pendingAction: "Delete", confirmAction: "web"}

	out, _, h, _ := m.renderOverlayConfirm()

	assert.NotContains(t, stripANSI(out), "Blast radius")
	assert.Positive(t, h)
}

func TestUpdateBlastRadiusLoaded_DropsAnOlderDialogsReply(t *testing.T) {
	m := Model{}
	m.blast.req = 2
	m.blast.loading = true

	mdl, _ := m.updateBlastRadiusLoaded(blastRadiusLoadedMsg{
		radius: &k8s.BlastRadius{Evicting: 99}, req: 1,
	})

	got := mdl.(Model)
	assert.Nil(t, got.blast.radius, "a reply for a dialog the user already closed must not land")
	assert.True(t, got.blast.loading, "and it must not clear the new dialog's spinner")
}

func TestUpdateBlastRadiusLoaded_FailureLeavesTheDialogUsable(t *testing.T) {
	m := Model{}
	m.blast.req = 1
	m.blast.loading = true

	mdl, _ := m.updateBlastRadiusLoaded(blastRadiusLoadedMsg{req: 1, err: assert.AnError})

	got := mdl.(Model)
	assert.False(t, got.blast.loading, "a failed lookup must not leave the line on 'checking'")
	assert.Nil(t, got.blast.radius)
}

func bulkItem(kind, ns, name string) model.Item {
	return model.Item{Kind: kind, Namespace: ns, Name: name}
}

func TestBulkPodTargets_GroupsPodsByNamespace(t *testing.T) {
	items := []model.Item{
		bulkItem("Pod", "prod", "web-1"),
		bulkItem("Pod", "prod", "web-2"),
		bulkItem("Pod", "staging", "web-1"),
	}

	byNS, uncounted := bulkPodTargets(items)

	assert.Zero(t, uncounted)
	assert.Equal(t, map[string]map[string]bool{
		"prod":    {"web-1": true, "web-2": true},
		"staging": {"web-1": true},
	}, byNS)
}

func TestBulkPodTargets_CountsRowsItCannotResolve(t *testing.T) {
	items := []model.Item{
		bulkItem("Pod", "prod", "web-1"),
		bulkItem("Deployment", "prod", "web"),
		bulkItem("StatefulSet", "prod", "db"),
	}

	byNS, uncounted := bulkPodTargets(items)

	require.Len(t, byNS, 1)
	assert.Equal(t, 2, uncounted,
		"a workload row carries no labels, so it cannot be resolved without a call per row")
}

func TestBlastRadiusNotes_UncountedRowsAreStated(t *testing.T) {
	r := &k8s.BlastRadius{Evicting: 3, ReadyBefore: 0, ReadyAfter: 0, Uncounted: 2}

	notes := blastRadiusNotes(r, false, false)

	require.Len(t, notes, 1)
	assert.Contains(t, notes[0].Text, "2 rows not counted")
}

func TestScaleBlastRadius_ScalingDownEvictsTheDifference(t *testing.T) {
	s := blastRadiusState{pods: []k8s.EvictedPod{
		{Namespace: "prod", Ready: true},
		{Namespace: "prod", Ready: true},
		{Namespace: "prod", Ready: true},
		{Namespace: "prod", Ready: true},
		{Namespace: "prod", Ready: true},
	}}

	got := s.scaleBlastRadius(2)

	require.NotNil(t, got)
	assert.Equal(t, 3, got.Evicting, "five down to two removes three")
	assert.Equal(t, 5, got.ReadyBefore)
	assert.Equal(t, 2, got.ReadyAfter)
}

func TestScaleBlastRadius_ScalingUpOrLevelCostsNothing(t *testing.T) {
	s := blastRadiusState{pods: []k8s.EvictedPod{{Namespace: "prod"}, {Namespace: "prod"}}}

	assert.Nil(t, s.scaleBlastRadius(2), "no change removes nothing")
	assert.Nil(t, s.scaleBlastRadius(9), "scaling up removes nothing")
}

func TestScaleBlastRadius_NoPodsYet(t *testing.T) {
	var s blastRadiusState

	assert.Nil(t, s.scaleBlastRadius(0), "nothing fetched yet, so nothing to say")
}

func TestScaleBlastRadius_CountsAgainstTheBudget(t *testing.T) {
	s := blastRadiusState{
		pods: []k8s.EvictedPod{
			{Namespace: "prod", Labels: map[string]string{"app": "web"}, Ready: true},
			{Namespace: "prod", Labels: map[string]string{"app": "web"}, Ready: true},
		},
		pdbs: []policyv1.PodDisruptionBudget{{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "web-pdb"},
			Spec:       policyv1.PodDisruptionBudgetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}},
			Status:     policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: 0},
		}},
	}

	got := s.scaleBlastRadius(1)

	require.NotNil(t, got)
	assert.True(t, got.Violation, "a budget allowing nothing is breached by scaling down one")
}

func TestRenderOverlayScaleInput_ShowsTheLineAsYouType(t *testing.T) {
	m := Model{width: 80, height: 24, overlay: overlayScaleInput}
	m.scaleInput.Set("2")
	m.blast.pods = []k8s.EvictedPod{
		{Namespace: "prod", Ready: true},
		{Namespace: "prod", Ready: true},
		{Namespace: "prod", Ready: true},
		{Namespace: "prod", Ready: true},
		{Namespace: "prod", Ready: true},
	}

	out, _, _, _ := m.renderOverlayContent()

	assert.Contains(t, stripANSI(out), "3 pods")
	assert.Contains(t, stripANSI(out), "2 of 5 ready after")
}
