package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func choreTreeItem() model.Item {
	return model.Item{
		Name: "chore-1",
		Kind: "Chore",
		Raw: map[string]any{
			"apiVersion": "chore.example.com/v1",
			"kind":       "Chore",
			"metadata":   map[string]any{"name": "chore-1"},
			"status": map[string]any{
				"phase": "Running",
				"steps": []any{
					map[string]any{
						"name":  "build",
						"phase": "Succeeded",
						"steps": []any{
							map[string]any{"name": "compile", "phase": "Succeeded"},
						},
					},
					map[string]any{"name": "deploy", "phase": "Pending"},
				},
			},
		},
	}
}

func objectExplorerModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Chore", Resource: "chores", APIVersion: "v1"}
	m.middleItems = []model.Item{choreTreeItem()}
	return m
}

func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// pressTree dispatches a key to the Object Explorer handler and returns the
// updated Model, discarding the command.
func pressTree(m Model, msg tea.KeyMsg) Model {
	mdl, _ := m.handleObjectExplorerKey(msg)
	return mdl.(Model)
}

func TestOpenObjectExplorer_RootLevel(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	assert.Equal(t, modeObjectExplorer, m.mode)
	assert.Equal(t, "Chore chore-1", m.objectExplorerView.title)
	// Root keys sorted.
	keys := make([]string, len(m.objectExplorerView.level))
	for i, f := range m.objectExplorerView.level {
		keys[i] = f.Key
	}
	assert.Equal(t, []string{"apiVersion", "kind", "metadata", "status"}, keys)
}

func TestOpenObjectExplorer_NoRaw(t *testing.T) {
	m := objectExplorerModel(t)
	m.middleItems = []model.Item{{Name: "x", Kind: "Chore"}} // no Raw
	result, cmd := m.openObjectExplorer()
	m = result.(Model)
	assert.Equal(t, modeExplorer, m.mode)
	assert.True(t, m.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestObjectExplorer_DrillDownAndBack(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)

	// Move cursor to "status" (index 3) and drill in.
	m.objectExplorerView.cursor = 3
	m = pressTree(m, key("l"))
	assert.Equal(t, []string{"status"}, m.objectExplorerView.path)
	// status has phase + steps.
	keys := map[string]bool{}
	for _, f := range m.objectExplorerView.level {
		keys[f.Key] = true
	}
	assert.True(t, keys["phase"])
	assert.True(t, keys["steps"])

	// Drill into steps (find its index), then into [0].
	stepsIdx := -1
	for i, f := range m.objectExplorerView.level {
		if f.Key == "steps" {
			stepsIdx = i
		}
	}
	require.GreaterOrEqual(t, stepsIdx, 0)
	m.objectExplorerView.cursor = stepsIdx
	m = pressTree(m, key("l"))
	require.Len(t, m.objectExplorerView.level, 2)
	assert.Equal(t, "[0]", m.objectExplorerView.level[0].Key)

	// Drill into [0]; it has name/phase/steps.
	m.objectExplorerView.cursor = 0
	m = pressTree(m, key("l"))
	assert.Equal(t, []string{"status", "steps", "[0]"}, m.objectExplorerView.path)

	// Back restores cursor onto "[0]".
	m = pressTree(m, key("h"))
	assert.Equal(t, []string{"status", "steps"}, m.objectExplorerView.path)
	assert.Equal(t, "[0]", m.objectExplorerView.level[m.objectExplorerView.cursor].Key)
}

func TestObjectExplorer_DrillIntoScalarIsNoop(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	// "apiVersion" (index 0) is a scalar -> drilling does nothing.
	m.objectExplorerView.cursor = 0
	m = pressTree(m, key("l"))
	assert.Empty(t, m.objectExplorerView.path)
}

func TestObjectExplorer_EscClosesAtRootButBacksOutWhenDeep(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	m.objectExplorerView.cursor = 3 // status
	m = pressTree(m, key("l"))
	require.Len(t, m.objectExplorerView.path, 1)

	// Esc when deep -> back to root, still open.
	m = pressTree(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, modeObjectExplorer, m.mode)
	assert.Empty(t, m.objectExplorerView.path)

	// Esc at root -> close.
	m = pressTree(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, modeExplorer, m.mode)
}

func TestObjectExplorer_DotKeyBreadcrumbStaysOneLevel(t *testing.T) {
	// An annotation key containing dots must remain a single breadcrumb
	// segment, not be split by the renderer.
	m := objectExplorerModel(t)
	m.middleItems = []model.Item{{
		Name: "p", Kind: "Pod",
		Raw: map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"app.kubernetes.io/name": "demo",
				},
			},
		},
	}}
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	// metadata -> annotations -> app.kubernetes.io/name
	m = drillTo(t, m, "metadata")
	m = drillTo(t, m, "annotations")
	require.Equal(t, []string{"metadata", "annotations"}, m.objectExplorerView.path)
	// The annotation key is a leaf with a dotted name; it renders as one row.
	require.Len(t, m.objectExplorerView.level, 1)
	assert.Equal(t, "app.kubernetes.io/name", m.objectExplorerView.level[0].Key)
	assert.Equal(t, "demo", m.objectExplorerView.level[0].Preview)
}

// drillTo moves the cursor onto the field named key and drills into it.
func drillTo(t *testing.T, m Model, k string) Model {
	t.Helper()
	for i, f := range m.objectExplorerView.level {
		if f.Key == k {
			m.objectExplorerView.cursor = i
			return pressTree(m, key("l"))
		}
	}
	t.Fatalf("field %q not found at current level", k)
	return m
}

func TestObjectExplorer_SelectedNodeYAML(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)

	// Cursor on "status" (object) -> YAML preview of the whole subtree.
	m.objectExplorerView.cursor = 3
	y := m.selectedNodeYAML()
	assert.Contains(t, y, "phase: Running")
	assert.Contains(t, y, "steps:")

	// Drill into status, select the scalar "phase" -> preview is its value.
	m = pressTree(m, key("l"))
	for i, f := range m.objectExplorerView.level {
		if f.Key == "phase" {
			m.objectExplorerView.cursor = i
		}
	}
	assert.Contains(t, m.selectedNodeYAML(), "Running")
}

func TestObjectExplorer_CopyYAMLReturnsCommand(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	m.objectExplorerView.cursor = 3 // status (object)

	mdl, cmd := m.handleObjectExplorerKey(key("Y")) // Y = copy full node YAML
	m = mdl.(Model)
	assert.False(t, m.statusMessageErr)
	assert.Contains(t, m.statusMessage, "Copied status")
	assert.NotNil(t, cmd) // copy + status-clear batch
}

func TestObjectExplorer_Filter(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)

	// Enter filter mode and type "meta" -> only "metadata" visible.
	m = pressTree(m, key("/"))
	assert.True(t, m.objectExplorerView.filterActive)
	for _, r := range []string{"m", "e", "t", "a"} {
		m = pressTree(m, key(r))
	}
	vis := m.objectExplorerView.visible()
	require.Len(t, vis, 1)
	assert.Equal(t, "metadata", vis[0].Key)

	// Enter exits typing but keeps the filter.
	m = pressTree(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, m.objectExplorerView.filterActive)
	assert.Equal(t, "meta", m.objectExplorerView.filter)

	// Esc clears the filter; all keys visible again.
	m = pressTree(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, "", m.objectExplorerView.filter)
	assert.Len(t, m.objectExplorerView.visible(), 4)
}

func TestObjectExplorer_CopyPath(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	m = drillTo(t, m, "status")
	m = drillTo(t, m, "steps")
	m.objectExplorerView.cursor = 0 // [0]

	mdl, cmd := m.handleObjectExplorerKey(key("y")) // y = copy path (the node's "name")
	m = mdl.(Model)
	assert.Contains(t, m.statusMessage, "status.steps[0]")
	assert.NotNil(t, cmd)
}

func TestObjectExplorer_FullYAMLHandoff(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	mdl, cmd := m.handleObjectExplorerKey(key("P"))
	m = mdl.(Model)
	assert.Equal(t, modeYAML, m.mode)
	assert.Equal(t, modeObjectExplorer, m.yamlReturnMode)
	assert.NotNil(t, cmd) // loadYAML

	// Closing the YAML viewer returns to the Object Explorer, not the explorer.
	mdl, _ = m.handleYAMLKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mdl.(Model)
	assert.Equal(t, modeObjectExplorer, m.mode)
	assert.Equal(t, modeExplorer, m.yamlReturnMode) // reset for the next open
}

func TestYAML_OReturnsToPreservedTree(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	// Drill in so the tree has non-root state to preserve.
	m.objectExplorerView.cursor = 3
	m = pressTree(m, key("l"))
	require.Len(t, m.objectExplorerView.path, 1)

	// Open YAML from the tree (P), then O returns to the preserved tree.
	mdl, _ := m.handleObjectExplorerKey(key("P"))
	m = mdl.(Model)
	require.Equal(t, modeYAML, m.mode)

	mdl, _ = m.handleYAMLKey(key("O"))
	m = mdl.(Model)
	assert.Equal(t, modeObjectExplorer, m.mode)
	assert.Equal(t, []string{"status"}, m.objectExplorerView.path) // position preserved
}

// choreDisplayedYAML matches choreTreeItem().Raw in the YAML viewer's displayed
// form (list items indented under their key).
const choreDisplayedYAML = `apiVersion: chore.example.com/v1
kind: Chore
metadata:
  name: chore-1
status:
  phase: Running
  steps:
    - name: build
      phase: Succeeded
      steps:
        - name: compile
          phase: Succeeded
    - name: deploy
      phase: Pending
`

func TestYAMLCursorSync_PendingPathPositionsCursor(t *testing.T) {
	m := basePush80Model()
	m.yamlView.content = choreDisplayedYAML
	m.yamlPendingPath = []string{"status", "steps", "[1]", "phase"} // line 13

	m.applyYAMLPendingCursor()
	assert.Equal(t, 13, m.yamlView.cursor) // no folds -> visible index == physical line
	assert.Nil(t, m.yamlPendingPath)       // consumed
}

func TestYAMLCursorSync_TreeFollowsYAMLCursor(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)

	// Simulate being in the YAML viewer (opened from the tree) on the
	// status.steps[1].phase line.
	m.mode = modeYAML
	m.yamlReturnMode = modeObjectExplorer
	m.yamlView.content = choreDisplayedYAML
	m.yamlView.cursor = 13

	mdl, _ := m.handleYAMLKey(key("O"))
	m = mdl.(Model)
	assert.Equal(t, modeObjectExplorer, m.mode)
	assert.Equal(t, []string{"status", "steps", "[1]"}, m.objectExplorerView.path)
	sel, ok := m.objectExplorerView.selected()
	require.True(t, ok)
	assert.Equal(t, "phase", sel.Key)
}

func TestYAML_OOpensFreshTreeWhenOpenedViaEnter(t *testing.T) {
	// YAML opened normally (not from the tree): O opens a fresh tree.
	m := objectExplorerModel(t)
	m.mode = modeYAML
	m.yamlReturnMode = modeExplorer // as set by the Enter path

	mdl, _ := m.handleYAMLKey(key("O"))
	m = mdl.(Model)
	assert.Equal(t, modeObjectExplorer, m.mode)
	assert.Equal(t, "Chore chore-1", m.objectExplorerView.title)
	assert.Empty(t, m.objectExplorerView.path) // fresh at root
}

func TestObjectExplorer_PreviewScrollResetsOnMove(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	m.height = 8                    // small viewport so the status subtree overflows the pane
	m.objectExplorerView.cursor = 3 // status (multi-line subtree)

	m = pressTree(m, key("J")) // scroll preview down
	assert.GreaterOrEqual(t, m.objectExplorerView.previewScroll, 0)
	scrolled := m.objectExplorerView.previewScroll

	m = pressTree(m, key("k")) // moving the cursor resets preview scroll
	assert.Equal(t, 0, m.objectExplorerView.previewScroll)
	_ = scrolled
}

func TestObjectExplorer_QClosesAndViewRenders(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)

	out := m.viewObjectExplorer()
	assert.Contains(t, out, "Object Explorer: Chore chore-1")
	assert.Contains(t, out, "PARENT")
	assert.Contains(t, out, "status")

	m = pressTree(m, key("q"))
	assert.Equal(t, modeExplorer, m.mode)
}

func TestObjectExplorer_ParentLevelAndBreadcrumb(t *testing.T) {
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	m = drillTo(t, m, "status") // path = ["status"]

	// Parent pane = root level, cursor on the drilled-into "status".
	fields, cursor := m.objectExplorerParentLevel()
	require.NotEmpty(t, fields)
	assert.Equal(t, "status", fields[cursor].Key)

	// Breadcrumb shows the object name then the JSONPath-style drill path, e.g.
	// "… > chore-1 > spec.volumes[0]" (the name isn't in nav at LevelResources).
	m.objectExplorerView.path = []string{"spec", "volumes", "[0]"}
	bc := m.breadcrumb()
	assert.Contains(t, bc, "chore-1 > spec.volumes[0]")
}

func TestObjectExplorer_ExitReturnsToOpener(t *testing.T) {
	// Opened from the explorer (O) -> q returns to the explorer.
	m := objectExplorerModel(t)
	res, _ := m.openObjectExplorer()
	m = res.(Model)
	require.Equal(t, modeExplorer, m.objectExplorerReturnMode)
	m.exitObjectExplorer()
	assert.Equal(t, modeExplorer, m.mode)

	// Opened from the YAML viewer (P) -> q returns to the YAML viewer.
	m2 := objectExplorerModel(t)
	m2.mode = modeYAML // simulate opening from the YAML viewer
	res2, _ := m2.openObjectExplorer()
	m2 = res2.(Model)
	require.Equal(t, modeObjectExplorer, m2.mode)
	require.Equal(t, modeYAML, m2.objectExplorerReturnMode)
	m2.exitObjectExplorer()
	assert.Equal(t, modeYAML, m2.mode)
}
