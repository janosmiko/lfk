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
		hiddenBuiltinColumns: map[string][]string{
			colKey("pod"): {"Name"},
		},
	}
	entries := m.collectBuiltinToggleEntries(items, "pod")

	got := map[string]bool{}
	for _, e := range entries {
		got[e.key] = e.visible
	}
	visibleByKey := false
	for _, e := range entries {
		if e.key == "Name" {
			visibleByKey = e.visible
		}
	}
	assert.False(t, visibleByKey, "session-hidden Name is unchecked in the overlay")
}
