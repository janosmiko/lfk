package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// TestCollectBuiltinToggleEntries_IncludesName verifies Name is now a
// first-class, toggleable built-in column: it appears in the overlay entries,
// leads the default order, and is visible unless explicitly hidden.
func TestCollectBuiltinToggleEntries_IncludesName(t *testing.T) {
	items := []model.Item{
		{Name: "nginx", Namespace: "default", Ready: "1/1", Restarts: "0", Status: "Running", Age: "5m"},
	}
	m := &Model{}
	entries := m.collectBuiltinToggleEntries(items, "pod")

	if assert.NotEmpty(t, entries) {
		assert.Equal(t, "Name", entries[0].key, "Name leads the built-in toggle list")
		assert.True(t, entries[0].builtin, "Name is a built-in entry")
		assert.True(t, entries[0].visible, "Name is visible by default")
	}
}

// TestCollectBuiltinToggleEntries_NameHiddenWhenSessionHidden verifies the
// overlay reflects a session-hidden Name.
func TestCollectBuiltinToggleEntries_NameHiddenWhenSessionHidden(t *testing.T) {
	items := []model.Item{
		{Name: "nginx", Namespace: "default", Status: "Running", Age: "5m"},
	}
	m := &Model{
		columnToggleState: columnToggleState{
			hiddenBuiltinColumns: map[string][]string{
				colKey("pod"): {"Name"},
			},
		},
	}
	entries := m.collectBuiltinToggleEntries(items, "pod")

	visibleByKey := false
	for _, e := range entries {
		if e.key == "Name" {
			visibleByKey = e.visible
		}
	}
	assert.False(t, visibleByKey, "session-hidden Name is unchecked in the overlay")
}

// TestMergeColumnToggleEntries_LegacyOrderPinsNameFirst verifies the overlay
// keeps Name pinned first when reopening a saved order that predates
// configurable Name (and therefore omits the "Name" key). Without this, simply
// reopening the overlay and pressing Enter would rewrite columnOrder so Name is
// no longer first, diverging from the renderer's pinned-first backward-compat
// behaviour in orderedColumnKeys.
func TestMergeColumnToggleEntries_LegacyOrderPinsNameFirst(t *testing.T) {
	builtins := []columnToggleEntry{
		{key: "Name", visible: true, builtin: true},
		{key: "Namespace", visible: true, builtin: true},
		{key: "Age", visible: true, builtin: true},
	}
	extras := []columnToggleEntry{{key: "IP", visible: false}}
	// Legacy saved order: no "Name" entry.
	savedOrder := []string{"Age", "IP", "Namespace"}

	merged := mergeColumnToggleEntries(builtins, extras, savedOrder)

	if assert.NotEmpty(t, merged) {
		assert.Equal(t, "Name", merged[0].key,
			"Name stays pinned first for a legacy order that omits it")
	}
}

// TestMergeColumnToggleEntries_ExplicitNameOrderHonored verifies that a newer
// saved order that lists "Name" explicitly keeps it at the chosen position.
func TestMergeColumnToggleEntries_ExplicitNameOrderHonored(t *testing.T) {
	builtins := []columnToggleEntry{
		{key: "Name", visible: true, builtin: true},
		{key: "Namespace", visible: true, builtin: true},
		{key: "Age", visible: true, builtin: true},
	}
	savedOrder := []string{"Namespace", "Name", "Age"}

	merged := mergeColumnToggleEntries(builtins, nil, savedOrder)

	keys := make([]string, len(merged))
	for i, e := range merged {
		keys[i] = e.key
	}
	assert.Equal(t, []string{"Namespace", "Name", "Age"}, keys,
		"explicit Name position is honored")
}
