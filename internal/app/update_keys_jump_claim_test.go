package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func claimJumpTestModel() Model {
	m := basePush80Model()
	m.leftItemsHistory = [][]model.Item{{{Name: "test-ctx"}}}
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "ResourceClaim", APIGroup: "resource.k8s.io", APIVersion: "v1", Resource: "resourceclaims", Namespaced: true},
	}
	m.middleItems = []model.Item{
		{
			Name: "pod-1", Namespace: "default", Kind: "Pod", Status: "Running",
			Columns: []model.KeyValue{
				{Key: "claim:0", Value: "gpu-claim"},
				{Key: "claim:1", Value: "nic-claim"},
			},
		},
		{Name: "pod-2", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	return m
}

func TestActionKeyJumpClaimNoSelection(t *testing.T) {
	m := claimJumpTestModel()
	m.setCursor(99) // out of range: selectedMiddleItem returns nil

	ret, cmd, handled := m.handleExplorerActionKeyJumpClaim()
	assert.True(t, handled)
	assert.Nil(t, cmd)
	_ = ret.(Model)
}

func TestActionKeyJumpClaimNoClaimsOnPod(t *testing.T) {
	m := claimJumpTestModel()
	m.setCursor(1) // pod-2 has no claim: columns

	ret, cmd, handled := m.handleExplorerActionKeyJumpClaim()
	assert.True(t, handled)
	require.NotNil(t, cmd)
	rm := ret.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.Equal(t, "No resource claims on this pod", rm.statusMessage)
}

func TestActionKeyJumpClaimUnresolvedClaimsOnPod(t *testing.T) {
	m := claimJumpTestModel()
	m.middleItems = append(m.middleItems, model.Item{
		Name: "pod-3", Namespace: "default", Kind: "Pod", Status: "Running",
		Columns: []model.KeyValue{{Key: "Resource Claims", Value: "gpu-template"}},
	})
	m.setCursor(2) // pod-3 has claims pending but none resolved to a claim: column

	ret, cmd, handled := m.handleExplorerActionKeyJumpClaim()
	assert.True(t, handled)
	require.NotNil(t, cmd)
	rm := ret.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.Equal(t, "Resource claim not created yet", rm.statusMessage)
}

func TestActionKeyJumpClaimTeleportsToFirstClaim(t *testing.T) {
	m := claimJumpTestModel()
	m.setCursor(0)

	ret, _, handled := m.handleExplorerActionKeyJumpClaim()
	assert.True(t, handled)
	rm := ret.(Model)
	assert.Equal(t, "gpu-claim", rm.pendingTarget)
	assert.Len(t, rm.jumpBackStack, 1, "must push jump history so jump_back returns to the pod list")
}

func TestActionKeyJumpClaimCyclesOnRepeatedPress(t *testing.T) {
	m := claimJumpTestModel()
	m.setCursor(0)

	ret1, _, _ := m.handleExplorerActionKeyJumpClaim()
	rm1 := ret1.(Model)
	assert.Equal(t, "gpu-claim", rm1.pendingTarget, "first press jumps to the first claim")

	// Restore explorer state as if jump_back happened, but keep the cycling
	// memory (claimJump), and re-select the same pod for the second press.
	rm1.nav.Level = model.LevelResources
	rm1.middleItems = m.middleItems
	ret2, _, _ := rm1.handleExplorerActionKeyJumpClaim()
	rm2 := ret2.(Model)
	assert.Equal(t, "nic-claim", rm2.pendingTarget, "second press on the same pod jumps to the next claim")

	rm2.nav.Level = model.LevelResources
	rm2.middleItems = m.middleItems
	ret3, _, _ := rm2.handleExplorerActionKeyJumpClaim()
	rm3 := ret3.(Model)
	assert.Equal(t, "gpu-claim", rm3.pendingTarget, "a third press wraps back to the first claim")
}

func TestActionKeyJumpClaimResetsIndexForDifferentPod(t *testing.T) {
	m := claimJumpTestModel()
	m.claimJump = claimJumpState{podKey: "default/pod-1", index: 1}
	m.middleItems = append(m.middleItems, model.Item{
		Name: "pod-3", Namespace: "default", Kind: "Pod", Status: "Running",
		Columns: []model.KeyValue{{Key: "claim:0", Value: "other-claim"}},
	})
	m.setCursor(2)

	ret, _, _ := m.handleExplorerActionKeyJumpClaim()
	rm := ret.(Model)
	assert.Equal(t, "other-claim", rm.pendingTarget, "a different pod always starts at its first claim")
}

func TestActionKeyCJumpsToClaim(t *testing.T) {
	m := claimJumpTestModel()
	m.setCursor(0)

	ret, _, handled := m.handleExplorerActionKey(runeKey('c'))
	assert.True(t, handled)
	rm := ret.(Model)
	assert.Equal(t, "gpu-claim", rm.pendingTarget)
}
