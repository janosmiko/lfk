package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildSidebarItems_PinnedTypesSection verifies that pinned resource types
// (by version-agnostic "group/resource" key) move into the synthetic "Pinned"
// section, which sits right after Dashboards, sorts alphabetically by display
// name, and accepts both built-in and CRD types. Unpinned siblings stay in
// their home category.
func TestBuildSidebarItems_PinnedTypesSection(t *testing.T) {
	defer func(orig []string) { PinnedTypes = orig }(PinnedTypes)
	// Pin one CRD type and one built-in type.
	PinnedTypes = []string{"example.com/widgets", "apps/deployments"}

	discovered := []ResourceTypeEntry{
		{Kind: "Widget", APIGroup: "example.com", APIVersion: "v1", Resource: "widgets"},
		{Kind: "Gadget", APIGroup: "example.com", APIVersion: "v1", Resource: "gadgets"},
		{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true},
	}

	items := BuildSidebarItems(discovered)

	// Both pinned items land in "Pinned", sorted alphabetically by name
	// (Deployments before Widgets).
	var pinned []Item
	for _, it := range items {
		if it.Category == "Pinned" {
			pinned = append(pinned, it)
		}
	}
	require.Len(t, pinned, 2)
	assert.Equal(t, "Deployment", pinned[0].Kind)
	assert.Equal(t, "Widget", pinned[1].Kind)

	// Positional checks: Dashboards section precedes Pinned, which precedes the
	// leftover CRD group. The unpinned Gadget stays under "example.com".
	dashIdx, pinIdx, gadgetIdx := -1, -1, -1
	for i, it := range items {
		switch {
		case it.Category == "Dashboards" && dashIdx == -1:
			dashIdx = i
		case it.Category == "Pinned" && pinIdx == -1:
			pinIdx = i
		case it.Kind == "Gadget":
			gadgetIdx = i
			assert.Equal(t, "example.com", it.Category)
		}
	}
	require.NotEqual(t, -1, dashIdx)
	require.NotEqual(t, -1, pinIdx)
	require.NotEqual(t, -1, gadgetIdx)
	assert.Less(t, dashIdx, pinIdx, "Pinned section must come after Dashboards")
	assert.Less(t, pinIdx, gadgetIdx, "Pinned section must come before leftover CRD groups")
}

// TestPinKeyFromRef verifies the version-agnostic key derivation and that
// sentinel refs (dashboards) are rejected.
func TestPinKeyFromRef(t *testing.T) {
	cases := map[string]string{
		"apps/v1/deployments":               "apps/deployments",
		"/v1/pods":                          "/pods",
		"argoproj.io/v1alpha1/applications": "argoproj.io/applications",
		"__overview__":                      "",
		"__monitoring__":                    "",
		"":                                  "",
	}
	for ref, want := range cases {
		assert.Equal(t, want, PinKeyFromRef(ref), "ref=%q", ref)
	}
}
