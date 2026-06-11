package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openTreeModeExplorer opens the Object Explorer on the chore fixture and
// toggles tree mode on.
func openTreeModeExplorer(t *testing.T) Model {
	t.Helper()
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	m = pressTree(m, key("T"))
	require.True(t, m.objectExplorerView.tree)
	return m
}

func TestObjectExplorerTree_SessionPreference(t *testing.T) {
	// The config-seeded session preference opens the explorer in tree mode.
	m := objectExplorerModel(t)
	m.objectExplorerTree = true
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	require.True(t, m.objectExplorerView.tree)
	require.NotEmpty(t, m.objectExplorerView.treeRows)
	// Toggling off updates the preference for the next open.
	m = pressTree(m, key("T"))
	assert.False(t, m.objectExplorerTree)
	m.exitObjectExplorer()
	result, _ = m.openObjectExplorer()
	m = result.(Model)
	assert.False(t, m.objectExplorerView.tree)
}

func TestObjectExplorerTree_ToggleOn(t *testing.T) {
	m := openTreeModeExplorer(t)
	rt := m.objectExplorerView
	require.NotEmpty(t, rt.treeRows)
	// Pre-order: apiVersion, kind, metadata, metadata.name, status, ...
	keys := make([]string, 0, len(rt.treeRows))
	for _, r := range rt.treeRows {
		keys = append(keys, r.Field.Key)
	}
	assert.Equal(t, []string{"apiVersion", "kind", "metadata", "name", "status", "phase", "steps"}, keys[:7])
	// Nested rows carry depth.
	assert.Equal(t, 1, rt.treeRows[3].Depth) // metadata.name
}

func TestObjectExplorerTree_ToggleOffRestoresFlat(t *testing.T) {
	m := openTreeModeExplorer(t)
	// Move onto a nested row: metadata.name (index 3).
	m.objectExplorerView.cursor = 3
	m = pressTree(m, key("T"))
	rt := m.objectExplorerView
	assert.False(t, rt.tree)
	assert.Empty(t, rt.treeRows)
	// Cursor lands on the top-level ancestor of the selected row (metadata).
	f, ok := rt.selected()
	require.True(t, ok)
	assert.Equal(t, "metadata", f.Key)
}

func TestObjectExplorerTree_SelectedNodePath(t *testing.T) {
	m := openTreeModeExplorer(t)
	// Row index 3 is metadata.name in pre-order.
	m.objectExplorerView.cursor = 3
	assert.Equal(t, []string{"metadata", "name"}, m.selectedNodePath())
}

func TestObjectExplorerTree_PreviewFollowsNestedRow(t *testing.T) {
	m := openTreeModeExplorer(t)
	m.objectExplorerView.cursor = 3 // metadata.name
	assert.Contains(t, m.selectedNodeYAML(), "chore-1")
}

func TestObjectExplorerTree_DrillReRoots(t *testing.T) {
	m := openTreeModeExplorer(t)
	// Cursor on "status" (index 4), drill into it.
	m.objectExplorerView.cursor = 4
	m = pressTree(m, key("l"))
	rt := m.objectExplorerView
	assert.True(t, rt.tree)
	assert.Equal(t, []string{"status"}, rt.path)
	// Rows now relative to status: phase, steps, [0], ...
	require.NotEmpty(t, rt.treeRows)
	assert.Equal(t, "phase", rt.treeRows[0].Field.Key)
	assert.Equal(t, []string{"phase"}, rt.treeRows[0].Segs)
}

func TestObjectExplorerTree_DrillOnScalarNoop(t *testing.T) {
	m := openTreeModeExplorer(t)
	m.objectExplorerView.cursor = 0 // apiVersion (scalar)
	m = pressTree(m, key("l"))
	assert.Empty(t, m.objectExplorerView.path)
	assert.True(t, m.objectExplorerView.tree)
}

func TestObjectExplorerTree_BackRebuildsRows(t *testing.T) {
	m := openTreeModeExplorer(t)
	m.objectExplorerView.cursor = 4 // status
	m = pressTree(m, key("l"))
	m = pressTree(m, key("h"))
	rt := m.objectExplorerView
	assert.Empty(t, rt.path)
	assert.True(t, rt.tree)
	// Cursor restored onto the drilled-from top-level row.
	f, ok := rt.selected()
	require.True(t, ok)
	assert.Equal(t, "status", f.Key)
}

func TestObjectExplorerTree_FilterMatchesNestedKeys(t *testing.T) {
	m := openTreeModeExplorer(t)
	m.objectExplorerView.filter = "phase"
	rows := m.objectExplorerView.visibleTreeRows()
	require.NotEmpty(t, rows)
	for _, r := range rows {
		assert.Contains(t, strings.ToLower(r.Field.Key), "phase")
	}
	// Nested status.steps[0].phase is reachable through the filter.
	assert.GreaterOrEqual(t, len(rows), 2)
}

func TestObjectExplorerTree_ViewRendersGuides(t *testing.T) {
	m := openTreeModeExplorer(t)
	view := stripANSI(m.viewObjectExplorer())
	assert.Contains(t, view, "├─")
	assert.Contains(t, view, "└─")
	// Hint bar advertises the toggle.
	assert.Contains(t, view, "tree")
}

func spaceKey() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeySpace} }

func TestObjectExplorerTree_SpaceCollapsesAndExpands(t *testing.T) {
	m := openTreeModeExplorer(t)
	before := len(m.objectExplorerView.visibleTreeRows())
	// Cursor on "metadata" (index 2), which has one child (name).
	m.objectExplorerView.cursor = 2
	m = pressTree(m, spaceKey())
	rt := m.objectExplorerView
	assert.Len(t, rt.visibleTreeRows(), before-1)
	// Cursor stays on the collapsed row.
	f, ok := rt.selected()
	require.True(t, ok)
	assert.Equal(t, "metadata", f.Key)
	// The hidden child is gone from the visible rows.
	for _, r := range rt.visibleTreeRows() {
		assert.NotEqual(t, []string{"metadata", "name"}, r.Segs)
	}
	// Space again expands.
	m = pressTree(m, spaceKey())
	assert.Len(t, m.objectExplorerView.visibleTreeRows(), before)
}

func TestObjectExplorerTree_FoldKeyZAlias(t *testing.T) {
	m := openTreeModeExplorer(t)
	before := len(m.objectExplorerView.visibleTreeRows())
	m.objectExplorerView.cursor = 2 // metadata
	m = pressTree(m, key("z"))
	assert.Len(t, m.objectExplorerView.visibleTreeRows(), before-1)
}

func TestObjectExplorerTree_SpaceOnLeafNoop(t *testing.T) {
	m := openTreeModeExplorer(t)
	before := len(m.objectExplorerView.visibleTreeRows())
	m.objectExplorerView.cursor = 0 // apiVersion (scalar)
	m = pressTree(m, spaceKey())
	assert.Len(t, m.objectExplorerView.visibleTreeRows(), before)
}

func TestObjectExplorerTree_CollapseNested(t *testing.T) {
	m := openTreeModeExplorer(t)
	// Collapse status.steps (index 6 in pre-order: apiVersion, kind, metadata,
	// name, status, phase, steps).
	m.objectExplorerView.cursor = 6
	m = pressTree(m, spaceKey())
	rt := m.objectExplorerView
	// steps row stays, all [i] subtrees hidden, siblings (phase) intact.
	keys := make([]string, 0, len(rt.visibleTreeRows()))
	for _, r := range rt.visibleTreeRows() {
		keys = append(keys, r.Field.Key)
	}
	assert.Equal(t, []string{"apiVersion", "kind", "metadata", "name", "status", "phase", "steps"}, keys)
}

func TestObjectExplorerTree_FilterIgnoresCollapse(t *testing.T) {
	m := openTreeModeExplorer(t)
	m.objectExplorerView.cursor = 6 // steps
	m = pressTree(m, spaceKey())
	m.objectExplorerView.filter = "phase"
	// Filter searches the whole subtree, including collapsed branches.
	assert.GreaterOrEqual(t, len(m.objectExplorerView.visibleTreeRows()), 2)
}

func TestObjectExplorerTree_DrillResetsCollapse(t *testing.T) {
	m := openTreeModeExplorer(t)
	m.objectExplorerView.cursor = 2 // metadata
	m = pressTree(m, spaceKey())
	require.NotEmpty(t, m.objectExplorerView.treeCollapsed)
	// Collapsing metadata shifted the visible indices; find status again.
	for i, r := range m.objectExplorerView.visibleTreeRows() {
		if r.Field.Key == "status" {
			m.objectExplorerView.cursor = i
			break
		}
	}
	m = pressTree(m, key("l"))
	assert.Empty(t, m.objectExplorerView.treeCollapsed)
}

func TestObjectExplorerTree_CollapsedRowShowsMarker(t *testing.T) {
	m := openTreeModeExplorer(t)
	m.objectExplorerView.cursor = 2 // metadata
	m = pressTree(m, spaceKey())
	m.objectExplorerView.cursor = 0 // move away so the row renders unselected
	view := stripANSI(m.viewObjectExplorer())
	for line := range strings.SplitSeq(view, "\n") {
		if strings.Contains(line, "metadata") && !strings.Contains(line, "PARENT") {
			assert.Contains(t, line, "›")
			return
		}
	}
	t.Fatal("metadata row not found")
}

func TestObjectExplorerTree_LiveSyncKeepsCursorOnSegs(t *testing.T) {
	m := openTreeModeExplorer(t)
	m.objectExplorerView.cursor = 3 // metadata.name
	m.objectExplorerLive = true
	m.syncObjectExplorerLive()
	rt := m.objectExplorerView
	require.True(t, rt.tree)
	require.Less(t, rt.cursor, len(rt.treeRows))
	assert.Equal(t, []string{"metadata", "name"}, rt.treeRows[rt.cursor].Segs)
}

func TestObjectExplorerTree_FindJumpRebuildsRows(t *testing.T) {
	m := openTreeModeExplorer(t)
	m.navigateObjectExplorerToPath([]string{"status", "steps", "[0]", "name"})
	rt := m.objectExplorerView
	assert.Equal(t, []string{"status", "steps", "[0]"}, rt.path)
	require.True(t, rt.tree)
	require.NotEmpty(t, rt.treeRows)
	// Cursor on the "name" row at the new root.
	f, ok := rt.selected()
	require.True(t, ok)
	assert.Equal(t, "name", f.Key)
}
