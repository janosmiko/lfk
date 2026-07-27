package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// sameKindTwoGroups is the issue #562 shape: one Kind, two groups. foo.com is
// listed first, so any Kind-only lookup resolves to it.
func sameKindTwoGroups() []model.ResourceTypeEntry {
	return []model.ResourceTypeEntry{
		{Kind: "MyKind", Resource: "mykinds", APIGroup: "foo.com", APIVersion: "v1", Namespaced: true},
		{Kind: "MyKind", Resource: "mykinds", APIGroup: "bar.com", APIVersion: "v1", Namespaced: true},
	}
}

// Owner navigation already parses the owner's apiVersion; it must use it.
// Jumping to a bar.com owner had been landing on foo.com because the group was
// parsed and then dropped on the way to the Kind-only lookup.
func TestResolveOwnerResourceType_PrefersTheOwnersGroup(t *testing.T) {
	discovered := sameKindTwoGroups()

	rt, ok := resolveOwnerResourceType("MyKind", "bar.com/v1", discovered)
	require.True(t, ok)
	assert.Equal(t, "bar.com", rt.APIGroup, "must follow the owner's apiVersion")

	rt, ok = resolveOwnerResourceType("MyKind", "foo.com/v1", discovered)
	require.True(t, ok)
	assert.Equal(t, "foo.com", rt.APIGroup)
}

// A core-group owner ("v1", no slash) resolves against the empty group.
func TestResolveOwnerResourceType_CoreGroupOwner(t *testing.T) {
	discovered := []model.ResourceTypeEntry{
		{Kind: "Pod", Resource: "pods", APIGroup: "", APIVersion: "v1", Namespaced: true},
		{Kind: "Pod", Resource: "pods", APIGroup: "shadow.com", APIVersion: "v1", Namespaced: true},
	}

	rt, ok := resolveOwnerResourceType("Pod", "v1", discovered)
	require.True(t, ok)
	assert.Empty(t, rt.APIGroup, "core apiVersion means the core group")
}

// Callers without an apiVersion (internal jumps that pass a bare Kind) keep
// the old first-match behavior rather than failing to resolve at all.
func TestResolveOwnerResourceType_NoAPIVersionFallsBackToKind(t *testing.T) {
	rt, ok := resolveOwnerResourceType("MyKind", "", sameKindTwoGroups())
	require.True(t, ok)
	assert.Equal(t, "foo.com", rt.APIGroup)
}

// An apiVersion naming a group that was never discovered still resolves by
// Kind — better a best-effort jump than a dead "unknown resource type".
func TestResolveOwnerResourceType_UndiscoveredGroupFallsBackToKind(t *testing.T) {
	rt, ok := resolveOwnerResourceType("MyKind", "gone.com/v1", sameKindTwoGroups())
	require.True(t, ok)
	assert.Equal(t, "foo.com", rt.APIGroup)
}

func TestResolveOwnerResourceType_UnknownKind(t *testing.T) {
	_, ok := resolveOwnerResourceType("Nope", "foo.com/v1", sameKindTwoGroups())
	assert.False(t, ok)
}
