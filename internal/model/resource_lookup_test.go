package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindResourceTypeByKind_SearchesParameterOnly(t *testing.T) {
	// After the refactor, FindResourceTypeByKind must not consult any
	// hardcoded list. It must find results only in the crds parameter.
	fixture := []ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}

	rt, ok := FindResourceTypeByKind("Pod", fixture)
	assert.True(t, ok)
	assert.Equal(t, "v1", rt.APIVersion)

	// A kind that only lived in TopLevelResourceTypes (not in fixture)
	// must not be found.
	_, ok = FindResourceTypeByKind("Deployment", fixture)
	assert.False(t, ok, "deployments should not be found from the hardcoded list")
}

func TestFindResourceTypeByKindAndGroup_SearchesParameterOnly(t *testing.T) {
	fixture := []ResourceTypeEntry{
		{Kind: "VaultDynamicSecret", APIGroup: "secrets.hashicorp.com", APIVersion: "v1beta1", Resource: "vaultdynamicsecrets", Namespaced: true},
		{Kind: "VaultDynamicSecret", APIGroup: "generators.external-secrets.io", APIVersion: "v1alpha1", Resource: "vaultdynamicsecrets", Namespaced: true},
	}

	rt, ok := FindResourceTypeByKindAndGroup("VaultDynamicSecret", "secrets.hashicorp.com", fixture)
	require.True(t, ok)
	assert.Equal(t, "secrets.hashicorp.com", rt.APIGroup)
	assert.Equal(t, "v1beta1", rt.APIVersion)
}

func TestFindResourceTypeIn_SearchesParameterOnly(t *testing.T) {
	fixture := []ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}

	rt, ok := FindResourceTypeIn("/v1/pods", fixture)
	require.True(t, ok)
	assert.Equal(t, "Pod", rt.Kind)

	_, ok = FindResourceTypeIn("apps/v1/deployments", fixture)
	assert.False(t, ok, "Deployment not in fixture must not be found")
}

func TestDisplayNameFor(t *testing.T) {
	tests := []struct {
		name string
		rt   ResourceTypeEntry
		want string
	}{
		{
			name: "pre-set DisplayName wins (pseudo-resources)",
			rt:   ResourceTypeEntry{DisplayName: "Releases", Kind: "HelmRelease", Resource: "releases"},
			want: "Releases",
		},
		{
			name: "core built-in falls through to BuiltInMetadata",
			rt:   ResourceTypeEntry{Kind: "Pod", APIGroup: "", Resource: "pods"},
			want: "Pods",
		},
		{
			name: "curated CRD in BuiltInMetadata",
			rt:   ResourceTypeEntry{Kind: "ExternalSecret", APIGroup: "external-secrets.io", Resource: "externalsecrets"},
			want: "ExternalSecrets",
		},
		{
			name: "unknown CRD falls back to Kind",
			rt:   ResourceTypeEntry{Kind: "Widget", APIGroup: "example.com", Resource: "widgets"},
			want: "Widget",
		},
		{
			name: "no Kind falls back to title-cased Resource",
			rt:   ResourceTypeEntry{APIGroup: "example.com", Resource: "widgets"},
			want: "Widgets",
		},
		{
			name: "fully empty entry returns empty string",
			rt:   ResourceTypeEntry{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DisplayNameFor(tt.rt))
		})
	}
}
