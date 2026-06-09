package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// cursorToField moves the Object Explorer cursor onto the field with the given
// key at the current level, so tests stay robust to benign field-order changes.
func cursorToField(t *testing.T, m *Model, key string) {
	t.Helper()
	for i, f := range m.objectExplorerView.level {
		if f.Key == key {
			m.objectExplorerView.cursor = i
			return
		}
	}
	t.Fatalf("field %q not found at current level", key)
}

// phasePreview returns the Preview of the "phase" field at the current level, or
// "" when absent.
func phasePreview(m Model) string {
	for _, f := range m.objectExplorerView.level {
		if f.Key == "phase" {
			return f.Preview
		}
	}
	return ""
}

// rootStatusPhase resolves status.phase on the Object Explorer's backing object.
func rootStatusPhase(t *testing.T, m Model) string {
	t.Helper()
	v, ok := model.ResolveObjectPath(m.objectExplorerView.root, []string{"status", "phase"})
	require.True(t, ok)
	s, _ := v.(string)
	return s
}

// Issue #391: while browsing a resource in the Object Explorer, a watch-mode
// refresh (resourcesLoadedMsg) must update the displayed object in place rather
// than leaving a stale snapshot until the user exits and re-enters.
func TestObjectExplorer_LiveRefreshOnResourceUpdate(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)

	// Drill into status so we exercise drill-path preservation across refresh.
	cursorToField(t, &m, "status")
	m = pressTree(m, key("l"))
	require.Equal(t, []string{"status"}, m.objectExplorerView.path)
	require.Equal(t, "Running", rootStatusPhase(t, m))

	// A watch tick delivers the same resource with an updated status.
	updated := choreTreeItem()
	updated.Raw["status"].(map[string]any)["phase"] = "Succeeded"
	mdl, _ := m.updateResourcesLoaded(resourcesLoadedMsg{items: []model.Item{updated}, gen: m.requestGen})
	m = mdl.(Model)

	// Still in the explorer, still drilled into status, now showing fresh data.
	assert.Equal(t, modeObjectExplorer, m.mode)
	assert.Equal(t, []string{"status"}, m.objectExplorerView.path)
	assert.Equal(t, "Succeeded", rootStatusPhase(t, m))
	assert.Contains(t, phasePreview(m), "Succeeded", "rebuilt level must reflect the new value")
}

// A refresh while an in-level filter is active must keep the cursor on the
// focused (filtered) field and still update its value.
func TestObjectExplorer_LiveRefreshWithActiveFilter(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	cursorToField(t, &m, "status")
	m = pressTree(m, key("l"))
	require.Equal(t, []string{"status"}, m.objectExplorerView.path)

	// Filter the status level down to "phase" and select it.
	m = pressTree(m, key("/"))
	m = pressTree(m, key("p"))
	m = pressTree(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, "phase", func() string { f, _ := m.objectExplorerView.selected(); return f.Key }())

	updated := choreTreeItem()
	updated.Raw["status"].(map[string]any)["phase"] = "Succeeded"
	mdl, _ := m.updateResourcesLoaded(resourcesLoadedMsg{items: []model.Item{updated}, gen: m.requestGen})
	m = mdl.(Model)

	assert.Equal(t, "phase", func() string { f, _ := m.objectExplorerView.selected(); return f.Key }())
	assert.Equal(t, "Succeeded", rootStatusPhase(t, m))
}

// When a drilled-into field disappears in the refresh, the path trims to the
// deepest resolvable prefix and the cursor lands on the field that vanished's
// parent slot rather than jumping to the last item.
func TestObjectExplorer_LiveRefreshTrimsVanishedPath(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)

	// Drill status -> steps -> [1] (the "deploy" element).
	cursorToField(t, &m, "status")
	m = pressTree(m, key("l"))
	cursorToField(t, &m, "steps")
	m = pressTree(m, key("l"))
	cursorToField(t, &m, "[1]")
	m = pressTree(m, key("l"))
	require.Equal(t, []string{"status", "steps", "[1]"}, m.objectExplorerView.path)

	// Refresh with steps reduced to a single element: [1] no longer resolves.
	updated := choreTreeItem()
	updated.Raw["status"].(map[string]any)["steps"] = []any{
		map[string]any{"name": "build", "phase": "Succeeded"},
	}
	mdl, _ := m.updateResourcesLoaded(resourcesLoadedMsg{items: []model.Item{updated}, gen: m.requestGen})
	m = mdl.(Model)

	assert.Equal(t, modeObjectExplorer, m.mode)
	// Path trimmed to the resolvable prefix; cursor on the (now only) element.
	assert.Equal(t, []string{"status", "steps"}, m.objectExplorerView.path)
	assert.Equal(t, "[0]", func() string { f, _ := m.objectExplorerView.selected(); return f.Key }())
}

// With live refresh paused (w), a watch refresh must NOT change the view.
func TestObjectExplorer_PausedIgnoresRefresh(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	cursorToField(t, &m, "status")
	m = pressTree(m, key("l"))

	// Pause live refresh (w).
	mdl, _ := m.handleObjectExplorerKey(key("w"))
	m = mdl.(Model)
	require.False(t, m.objectExplorerLive)

	updated := choreTreeItem()
	updated.Raw["status"].(map[string]any)["phase"] = "Succeeded"
	mdl, _ = m.updateResourcesLoaded(resourcesLoadedMsg{items: []model.Item{updated}, gen: m.requestGen})
	m = mdl.(Model)

	// Frozen snapshot: still the old value.
	assert.Equal(t, "Running", rootStatusPhase(t, m))
}

// Manual refresh (R) must update the view once even while live refresh is paused.
func TestObjectExplorer_ManualRefreshWhilePaused(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	cursorToField(t, &m, "status")
	m = pressTree(m, key("l"))
	m.objectExplorerLive = false // paused

	// R sets the one-shot force-sync flag and returns a refresh command.
	mdl, cmd := m.handleObjectExplorerKey(key("R"))
	m = mdl.(Model)
	require.True(t, m.objectExplorerForceSync)
	require.NotNil(t, cmd)

	updated := choreTreeItem()
	updated.Raw["status"].(map[string]any)["phase"] = "Succeeded"
	mdl, _ = m.updateResourcesLoaded(resourcesLoadedMsg{items: []model.Item{updated}, gen: m.requestGen})
	m = mdl.(Model)

	assert.Equal(t, "Succeeded", rootStatusPhase(t, m), "manual refresh applies once even when paused")
	assert.False(t, m.objectExplorerForceSync, "force-sync is consumed (single-shot)")
	assert.False(t, m.objectExplorerLive, "manual refresh does not re-enable live")

	// A subsequent watch refresh is ignored again (still paused).
	updated2 := choreTreeItem()
	updated2.Raw["status"].(map[string]any)["phase"] = "Failed"
	mdl, _ = m.updateResourcesLoaded(resourcesLoadedMsg{items: []model.Item{updated2}, gen: m.requestGen})
	m = mdl.(Model)
	assert.Equal(t, "Succeeded", rootStatusPhase(t, m), "still paused after the one-shot refresh")
}

// Toggling live back on re-enables auto-sync and requests an immediate catch-up.
func TestObjectExplorer_ToggleLiveBackOn(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	m.objectExplorerLive = false

	mdl, cmd := m.handleObjectExplorerKey(key("w"))
	m = mdl.(Model)
	assert.True(t, m.objectExplorerLive)
	assert.True(t, m.objectExplorerForceSync, "turning live on requests an immediate catch-up sync")
	assert.NotNil(t, cmd)
}

// When the browsed resource disappears from the refresh (deleted), the Object
// Explorer must keep its last snapshot instead of silently swapping to whatever
// row the list cursor lands on.
func TestObjectExplorer_LiveRefreshKeepsSnapshotWhenResourceGone(t *testing.T) {
	m := objectExplorerModel(t)
	// Two resources; open the explorer on the first.
	other := choreTreeItem()
	other.Name = "chore-2"
	other.Raw = map[string]any{
		"kind":     "Chore",
		"metadata": map[string]any{"name": "chore-2"},
		"status":   map[string]any{"phase": "Other"},
	}
	m.middleItems = []model.Item{choreTreeItem(), other}
	m.setCursor(0)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	require.Equal(t, "chore-1", m.objectExplorerView.name)

	// Refresh drops chore-1; only chore-2 remains.
	mdl, _ := m.updateResourcesLoaded(resourcesLoadedMsg{items: []model.Item{other}, gen: m.requestGen})
	m = mdl.(Model)

	assert.Equal(t, modeObjectExplorer, m.mode)
	// Snapshot preserved: still chore-1, not swapped to chore-2's "Other".
	assert.Equal(t, "chore-1", m.objectExplorerView.name)
	assert.Equal(t, "Running", rootStatusPhase(t, m))
}
