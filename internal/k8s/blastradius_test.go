package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func pdb(namespace, name string, allowed int32, matchLabels map[string]string) policyv1.PodDisruptionBudget {
	var sel *metav1.LabelSelector
	if matchLabels != nil {
		sel = &metav1.LabelSelector{MatchLabels: matchLabels}
	}
	return policyv1.PodDisruptionBudget{
		Namespace: namespace, Name: name,
		Spec:   policyv1.PodDisruptionBudgetSpec{Selector: sel},
		Status: policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: allowed},
	}
}

func webPod(namespace string, ready bool) EvictedPod {
	return EvictedPod{Namespace: namespace, Labels: map[string]string{"app": "web"}, Ready: ready}
}

func TestComputeBlastRadius_NoPDBIsNotAViolation(t *testing.T) {
	got := ComputeBlastRadius([]EvictedPod{webPod("prod", true)}, nil, 3)

	assert.Empty(t, got.PDBs)
	assert.False(t, got.Violation)
	assert.Equal(t, 1, got.Evicting)
	assert.Equal(t, 3, got.ReadyBefore)
	assert.Equal(t, 2, got.ReadyAfter)
}

func TestComputeBlastRadius_SubtractsMatchedPodsFromAllowed(t *testing.T) {
	pdbs := []policyv1.PodDisruptionBudget{pdb("prod", "web-pdb", 2, map[string]string{"app": "web"})}

	got := ComputeBlastRadius([]EvictedPod{webPod("prod", true), webPod("prod", true)}, pdbs, 5)

	require.Len(t, got.PDBs, 1)
	assert.Equal(t, int32(2), got.PDBs[0].AllowedBefore)
	assert.Equal(t, int32(0), got.PDBs[0].AllowedAfter)
	assert.Equal(t, 2, got.PDBs[0].Evicting)
	assert.False(t, got.PDBs[0].Violated, "landing exactly on zero is allowed, not a violation")
	assert.Equal(t, 3, got.ReadyAfter)
}

func TestComputeBlastRadius_MoreEvictionsThanAllowedIsAViolation(t *testing.T) {
	pdbs := []policyv1.PodDisruptionBudget{pdb("prod", "web-pdb", 1, map[string]string{"app": "web"})}

	got := ComputeBlastRadius([]EvictedPod{webPod("prod", true), webPod("prod", true)}, pdbs, 5)

	require.Len(t, got.PDBs, 1)
	assert.Equal(t, int32(-1), got.PDBs[0].AllowedAfter, "the shortfall is the point, so it is not clamped")
	assert.True(t, got.PDBs[0].Violated)
	assert.True(t, got.Violation)
}

func TestComputeBlastRadius_ZeroAllowedIsViolatedByOneEviction(t *testing.T) {
	pdbs := []policyv1.PodDisruptionBudget{pdb("prod", "web-pdb", 0, map[string]string{"app": "web"})}

	got := ComputeBlastRadius([]EvictedPod{webPod("prod", true)}, pdbs, 1)

	require.Len(t, got.PDBs, 1)
	assert.True(t, got.PDBs[0].Violated)
	assert.True(t, got.Violation)
}

func TestComputeBlastRadius_IgnoresPDBsInOtherNamespaces(t *testing.T) {
	pdbs := []policyv1.PodDisruptionBudget{pdb("staging", "web-pdb", 2, map[string]string{"app": "web"})}

	got := ComputeBlastRadius([]EvictedPod{webPod("prod", true)}, pdbs, 3)

	assert.Empty(t, got.PDBs, "a PDB only ever covers pods in its own namespace")
}

func TestComputeBlastRadius_NilSelectorCoversNothing(t *testing.T) {
	pdbs := []policyv1.PodDisruptionBudget{pdb("prod", "broken-pdb", 2, nil)}

	got := ComputeBlastRadius([]EvictedPod{webPod("prod", true)}, pdbs, 3)

	assert.Empty(t, got.PDBs)
}

func TestComputeBlastRadius_EmptySelectorCoversEveryPodInItsNamespace(t *testing.T) {
	catchAll := pdb("prod", "all-pdb", 1, map[string]string{})

	got := ComputeBlastRadius([]EvictedPod{
		{Namespace: "prod", Labels: map[string]string{"app": "anything"}, Ready: true},
	}, []policyv1.PodDisruptionBudget{catchAll}, 2)

	require.Len(t, got.PDBs, 1)
	assert.Equal(t, int32(0), got.PDBs[0].AllowedAfter)
}

func TestComputeBlastRadius_ReportsOnlyPDBsThatCoverSomethingBeingEvicted(t *testing.T) {
	pdbs := []policyv1.PodDisruptionBudget{
		pdb("prod", "web-pdb", 2, map[string]string{"app": "web"}),
		pdb("prod", "api-pdb", 2, map[string]string{"app": "api"}),
	}

	got := ComputeBlastRadius([]EvictedPod{webPod("prod", true)}, pdbs, 3)

	require.Len(t, got.PDBs, 1, "an untouched PDB is noise in a one-line summary")
	assert.Equal(t, "web-pdb", got.PDBs[0].Name)
}

func TestComputeBlastRadius_DrainSpansNamespaces(t *testing.T) {
	pdbs := []policyv1.PodDisruptionBudget{
		pdb("prod", "web-pdb", 1, map[string]string{"app": "web"}),
		pdb("staging", "web-pdb", 3, map[string]string{"app": "web"}),
	}

	got := ComputeBlastRadius([]EvictedPod{webPod("prod", true), webPod("staging", true)}, pdbs, 0)

	require.Len(t, got.PDBs, 2)
	assert.Equal(t, "prod", got.PDBs[0].Namespace, "sorted by namespace then name, so the line is stable")
	assert.Equal(t, "staging", got.PDBs[1].Namespace)
	assert.Equal(t, int32(0), got.PDBs[0].AllowedAfter)
	assert.Equal(t, int32(2), got.PDBs[1].AllowedAfter)
}

func TestComputeBlastRadius_ZeroReplicasStaysAtZero(t *testing.T) {
	got := ComputeBlastRadius(nil, nil, 0)

	assert.Equal(t, 0, got.Evicting)
	assert.Equal(t, 0, got.ReadyBefore)
	assert.Equal(t, 0, got.ReadyAfter)
	assert.False(t, got.Violation)
}

func TestComputeBlastRadius_ReadyAfterNeverGoesNegative(t *testing.T) {
	// A stale replica count against a fresher pod list would otherwise print
	// a negative number of ready pods.
	got := ComputeBlastRadius([]EvictedPod{webPod("prod", true), webPod("prod", true)}, nil, 1)

	assert.Equal(t, 0, got.ReadyAfter)
}

func TestComputeBlastRadius_OnlyReadyPodsCountAgainstReadyAfter(t *testing.T) {
	got := ComputeBlastRadius([]EvictedPod{webPod("prod", true), webPod("prod", false)}, nil, 3)

	assert.Equal(t, 2, got.Evicting, "both pods still go")
	assert.Equal(t, 2, got.ReadyAfter, "but only the ready one was counted as ready")
}
