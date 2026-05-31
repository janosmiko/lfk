package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// TestNavigateChildOwnedSecurityAffectedResourceJumpsToRealResource verifies
// that pressing Enter on a __security_affected_resource__ row at LevelOwned
// navigates to the underlying Kubernetes resource at LevelResources, using
// the synthetic __resource_key__ column to recover Kind/Namespace/Name.
func TestNavigateChildOwnedSecurityAffectedResourceJumpsToRealResource(t *testing.T) {
	m := baseModelBoost2()
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true},
	}
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_falco__", APIGroup: "_security"}
	m.nav.ResourceName = "privileged"
	m.securityActiveGroup = "privileged"
	sel := &model.Item{
		Kind:      "__security_affected_resource__",
		Name:      "deploy/api",
		Namespace: "prod",
		Columns: []model.KeyValue{
			{Key: "__resource_key__", Value: "prod/Deployment/api"},
			{Key: "ResourceKind", Value: "Deployment"},
		},
	}
	m.middleItems = []model.Item{*sel}
	m.setCursor(0)

	rm, _ := m.navigateChildOwned(sel)
	rmm := rm.(Model)

	assert.Equal(t, model.LevelResources, rmm.nav.Level, "must transition to LevelResources")
	assert.Equal(t, "Deployment", rmm.nav.ResourceType.Kind, "ResourceType must be the real kind")
	assert.Equal(t, "prod", rmm.nav.Namespace, "Namespace must come from the resource_key")
	assert.Empty(t, rmm.securityActiveGroup, "security group context must be cleared on jump")
}

// TestNavigateChildOwnedSecurityAffectedResourcePushesJumpHistory verifies the
// jump to a finding's underlying resource is recorded on the jump-back stack so
// JumpBack returns to the finding's affected-resources view. Without this the
// teleport is one-way and the user is stranded on the real resource list.
func TestNavigateChildOwnedSecurityAffectedResourcePushesJumpHistory(t *testing.T) {
	m := baseModelBoost2()
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true},
	}
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_heuristic__", APIGroup: "_security"}
	m.nav.ResourceName = "privileged"
	m.securityActiveGroup = "privileged"
	sel := &model.Item{
		Kind:      "__security_affected_resource__",
		Name:      "deploy/api",
		Namespace: "prod",
		Columns: []model.KeyValue{
			{Key: "__resource_key__", Value: "prod/Deployment/api"},
			{Key: "ResourceKind", Value: "Deployment"},
		},
	}
	m.middleItems = []model.Item{*sel}
	m.setCursor(0)

	require.Empty(t, m.jumpBackStack, "precondition: no jump history")

	rm, _ := m.navigateChildOwned(sel)
	rmm := rm.(Model)

	require.Len(t, rmm.jumpBackStack, 1, "jump must record the finding view on the back stack")
	snap := rmm.jumpBackStack[0]
	assert.Equal(t, model.LevelOwned, snap.nav.Level, "snapshot must capture the finding view level")
	assert.Equal(t, "__security_heuristic__", snap.nav.ResourceType.Kind,
		"snapshot must capture the security source the user jumped from")

	// JumpBack returns to the finding's affected-resources view.
	back, _ := rmm.jumpBack()
	assert.Equal(t, model.LevelOwned, back.(Model).nav.Level, "JumpBack must return to the finding view")
}

// TestNavigateChildOwnedSecurityAffectedResourceNoHistoryOnFailedJump verifies a
// jump that cannot resolve its target (kind not discovered) leaves the jump-back
// stack untouched — pushing history for a no-op teleport would strand a phantom
// entry that returns the user nowhere new.
func TestNavigateChildOwnedSecurityAffectedResourceNoHistoryOnFailedJump(t *testing.T) {
	m := baseModelBoost2()
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{}
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_heuristic__", APIGroup: "_security"}
	sel := &model.Item{
		Kind:      "__security_affected_resource__",
		Name:      "deploy/api",
		Namespace: "prod",
		Columns: []model.KeyValue{
			{Key: "__resource_key__", Value: "prod/Deployment/api"},
			{Key: "ResourceKind", Value: "Deployment"},
		},
	}
	m.middleItems = []model.Item{*sel}
	m.setCursor(0)

	rm, _ := m.navigateChildOwned(sel)
	assert.Empty(t, rm.(Model).jumpBackStack, "failed jump must not record history")
}

// TestNavigateChildOwnedSecurityAffectedResourceKindNotDiscovered guards the
// fallback path: if the finding's Kind has not been discovered yet for the
// active context (e.g., a CRD whose API discovery is still in flight), the
// jump must not silently no-op — surface a status message and stay put.
func TestNavigateChildOwnedSecurityAffectedResourceKindNotDiscovered(t *testing.T) {
	m := baseModelBoost2()
	// discoveredResources intentionally empty for the active context.
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{}
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_falco__", APIGroup: "_security"}
	sel := &model.Item{
		Kind:      "__security_affected_resource__",
		Name:      "deploy/api",
		Namespace: "prod",
		Columns: []model.KeyValue{
			{Key: "__resource_key__", Value: "prod/Deployment/api"},
			{Key: "ResourceKind", Value: "Deployment"},
		},
	}
	m.middleItems = []model.Item{*sel}
	m.setCursor(0)

	rm, _ := m.navigateChildOwned(sel)
	rmm := rm.(Model)

	assert.Equal(t, model.LevelOwned, rmm.nav.Level, "must stay at LevelOwned when kind unknown")
	assert.NotEmpty(t, rmm.statusMessage, "must surface a status message explaining why the jump did not happen")
}

// TestNavigateChildOwnedSecurityAffectedResourceMalformedKey verifies the
// defensive parse: an item missing or malformed __resource_key__ should not
// crash or silently dispatch a malformed loadResources call.
func TestNavigateChildOwnedSecurityAffectedResourceMalformedKey(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_falco__", APIGroup: "_security"}
	sel := &model.Item{
		Kind:      "__security_affected_resource__",
		Name:      "deploy/api",
		Namespace: "prod",
		// No __resource_key__ column.
		Columns: []model.KeyValue{},
	}
	m.middleItems = []model.Item{*sel}
	m.setCursor(0)

	rm, cmd := m.navigateChildOwned(sel)
	rmm := rm.(Model)

	assert.Equal(t, model.LevelOwned, rmm.nav.Level, "must stay put on malformed resource_key")
	assert.Nil(t, cmd, "no async work should fire on malformed input")
}
