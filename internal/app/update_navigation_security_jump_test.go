package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
