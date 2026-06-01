package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildSidebarItems_HiddenTypes verifies that resource types listed in
// HiddenTypes are removed from the sidebar by default, and surface dimmed
// (Item.Hidden) when the reveal toggle (ShowRareResources) is on.
func TestBuildSidebarItems_HiddenTypes(t *testing.T) {
	defer func(orig []string) { HiddenTypes = orig }(HiddenTypes)
	defer func(orig bool) { ShowRareResources = orig }(ShowRareResources)

	discovered := []ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
		{Kind: "Ingress", APIGroup: "networking.k8s.io", APIVersion: "v1", Resource: "ingresses", Namespaced: true},
		{Kind: "LimitRange", APIGroup: "", APIVersion: "v1", Resource: "limitranges", Namespaced: true},
	}

	HiddenTypes = []string{"networking.k8s.io/ingresses", "/limitranges"}

	t.Run("hidden types removed when reveal off", func(t *testing.T) {
		ShowRareResources = false
		items := BuildSidebarItems(discovered)
		names := collectByDisplay(items)
		assert.Contains(t, names, "Pods", "non-hidden type stays visible")
		assert.NotContains(t, names, "Ingresses", "hidden type must be removed by default")
		assert.NotContains(t, names, "LimitRanges", "hidden type must be removed by default")
	})

	t.Run("hidden types revealed dimmed when reveal on", func(t *testing.T) {
		ShowRareResources = true
		items := BuildSidebarItems(discovered)
		names := collectByDisplay(items)
		require.Contains(t, names, "Ingresses", "hidden type must surface when reveal is on")
		assert.True(t, names["Ingresses"].Hidden, "revealed hidden type must be flagged Hidden so the renderer dims it")
		assert.Equal(t, "Networking", names["Ingresses"].Category, "revealed hidden type keeps its home category")

		require.Contains(t, names, "Pods")
		assert.False(t, names["Pods"].Hidden, "non-hidden type must not be flagged Hidden")
	})

	t.Run("empty HiddenTypes leaves everything visible", func(t *testing.T) {
		HiddenTypes = nil
		ShowRareResources = false
		items := BuildSidebarItems(discovered)
		names := collectByDisplay(items)
		assert.Contains(t, names, "Ingresses")
		assert.Contains(t, names, "LimitRanges")
		assert.False(t, names["Ingresses"].Hidden)
	})
}

// TestBuildSidebarItems_PinnedAndHidden locks in the precedence when a type is
// both pinned and hidden: hidden wins. With reveal off it is removed entirely
// (including from the Pinned section); with reveal on it surfaces dimmed,
// still relocated to the Pinned section.
func TestBuildSidebarItems_PinnedAndHidden(t *testing.T) {
	defer func(orig []string) { HiddenTypes = orig }(HiddenTypes)
	defer func(orig []string) { PinnedTypes = orig }(PinnedTypes)
	defer func(orig bool) { ShowRareResources = orig }(ShowRareResources)

	discovered := []ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
		{Kind: "Ingress", APIGroup: "networking.k8s.io", APIVersion: "v1", Resource: "ingresses", Namespaced: true},
	}
	PinnedTypes = []string{"networking.k8s.io/ingresses"}
	HiddenTypes = []string{"networking.k8s.io/ingresses"}

	t.Run("removed entirely when reveal off", func(t *testing.T) {
		ShowRareResources = false
		names := collectByDisplay(BuildSidebarItems(discovered))
		assert.NotContains(t, names, "Ingresses", "hidden wins over pinned: removed from Pinned section too")
	})

	t.Run("dimmed in Pinned section when reveal on", func(t *testing.T) {
		ShowRareResources = true
		names := collectByDisplay(BuildSidebarItems(discovered))
		require.Contains(t, names, "Ingresses")
		assert.True(t, names["Ingresses"].Hidden)
		assert.Equal(t, "Pinned", names["Ingresses"].Category, "stays relocated to Pinned when revealed")
	})
}
