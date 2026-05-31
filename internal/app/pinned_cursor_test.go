package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// cursorIndexOfItem returns the visible-list index of the resource type whose
// display name matches, or -1.
func cursorIndexOfItem(m *Model, name string) int {
	for i, it := range m.visibleMiddleItems() {
		if it.Name == name {
			return i
		}
	}
	return -1
}

// TestPinCursorFollowsToNextItem verifies that pinning a resource type moves the
// cursor to the next sibling (not the previous one) once the item jumps into the
// "Pinned" section and the list re-sorts.
func TestPinCursorFollowsToNextItem(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	defer func(orig []string) { model.PinnedTypes = orig }(model.PinnedTypes)

	m := baseModelWithFakeClient()
	m.nav.Context = "prod"
	m.nav.Level = model.LevelResourceTypes
	m.allGroupsExpanded = true
	m.pinnedState = newPinnedState()
	discovered := []model.ResourceTypeEntry{
		{Kind: "Gadget", APIGroup: "example.com", APIVersion: "v1", Resource: "gadgets"},
		{Kind: "Widget", APIGroup: "example.com", APIVersion: "v1", Resource: "widgets"},
	}
	m.discoveredResources["prod"] = discovered
	m.setMiddleItems(model.BuildSidebarItems(discovered))

	// Sanity: within example.com, Gadget sorts before Widget.
	gadgetIdx := cursorIndexOfItem(&m, "Gadgets")
	widgetIdx := cursorIndexOfItem(&m, "Widgets")
	require.NotEqual(t, -1, gadgetIdx)
	require.NotEqual(t, -1, widgetIdx)
	require.Less(t, gadgetIdx, widgetIdx)

	// Select Gadget and pin it.
	m.setCursor(gadgetIdx)
	result, _ := m.handleKeyPinGroup()
	rm := result.(Model)

	// Gadget moved into the Pinned section; the cursor should now sit on Widget
	// (the next sibling), not on whatever shifted into Gadget's old index.
	sel := rm.selectedMiddleItem()
	require.NotNil(t, sel)
	assert.Equal(t, "Widgets", sel.Name, "cursor should follow to the next item after pinning")
}

// TestPinCursorFallsBackToPreviousWhenLast verifies that pinning the last item
// in the list lands the cursor on the previous sibling (there is no next).
func TestPinCursorFallsBackToPreviousWhenLast(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	defer func(orig []string) { model.PinnedTypes = orig }(model.PinnedTypes)

	m := baseModelWithFakeClient()
	m.nav.Context = "prod"
	m.nav.Level = model.LevelResourceTypes
	m.allGroupsExpanded = true
	m.pinnedState = newPinnedState()
	discovered := []model.ResourceTypeEntry{
		{Kind: "Gadget", APIGroup: "example.com", APIVersion: "v1", Resource: "gadgets"},
		{Kind: "Widget", APIGroup: "example.com", APIVersion: "v1", Resource: "widgets"},
	}
	m.discoveredResources["prod"] = discovered
	m.setMiddleItems(model.BuildSidebarItems(discovered))

	// Select Widget (last item) and pin it.
	m.setCursor(cursorIndexOfItem(&m, "Widgets"))
	result, _ := m.handleKeyPinGroup()
	rm := result.(Model)

	sel := rm.selectedMiddleItem()
	require.NotNil(t, sel)
	assert.Equal(t, "Gadgets", sel.Name, "cursor should fall back to the previous item when pinning the last")
}
