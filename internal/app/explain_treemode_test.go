package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// explainTreeBaseModel builds a model sitting in the API Explorer at the spec level.
func explainTreeBaseModel() Model {
	m := basePush80Model()
	m.mode = modeExplain
	m.explainResource = "deployments"
	m.explainAPIVersion = "apps/v1"
	m.explainPath = "spec"
	m.explainTitle = "deployments > spec"
	m.explainFields = []model.ExplainField{
		{Name: "replicas", Type: "<integer>", Path: "spec.replicas"},
		{Name: "template", Type: "<PodTemplateSpec>", Path: "spec.template"},
	}
	m.explainCursor = 1
	// As if T was already pressed: tree loads only apply while wanted.
	m.explainTreeWanted = true
	return m
}

func sampleTreeMsg() explainTreeLoadedMsg {
	return explainTreeLoadedMsg{
		fields: []model.ExplainField{
			{Name: "replicas", Type: "<integer>", Path: "spec.replicas"},
			{Name: "template", Type: "<PodTemplateSpec>", Path: "spec.template"},
			{Name: "spec", Type: "<PodSpec>", Path: "spec.template.spec"},
			{Name: "containers", Type: "<[]Container>", Path: "spec.template.spec.containers"},
		},
		path: "spec",
	}
}

func TestExplainTree_ToggleStartsLoad(t *testing.T) {
	m := explainTreeBaseModel()
	m.explainTreeWanted = false // simulate the full T flow from scratch
	mdl, cmd := m.handleExplainKey(key("T"))
	m = mdl.(Model)
	assert.True(t, m.loading)
	assert.NotNil(t, cmd)
}

func TestExplainTree_LoadedSwapsFields(t *testing.T) {
	m := explainTreeBaseModel()
	mdl, _ := m.updateExplainTreeLoaded(sampleTreeMsg())
	m = mdl.(Model)
	require.True(t, m.explainTree)
	require.Len(t, m.explainFields, 4)
	assert.Equal(t, []int{0, 0, 1, 2}, m.explainTreeDepths)
	// The flat cursor (on "template") maps onto the same field's tree row.
	assert.Equal(t, 1, m.explainCursor)
	assert.Equal(t, "spec.template", m.explainFields[m.explainCursor].Path)
}

func TestExplainTree_SpaceFoldCollapsesSubtree(t *testing.T) {
	m := explainTreeBaseModel()
	mdl, _ := m.updateExplainTreeLoaded(sampleTreeMsg())
	m = mdl.(Model)
	// Cursor is on "template"; fold its subtree.
	mdl, _ = m.handleExplainKey(spaceKey())
	m = mdl.(Model)
	require.Len(t, m.explainFields, 2)
	require.Len(t, m.explainTreeDepths, 2)
	assert.Equal(t, "spec.template", m.explainFields[1].Path)
	assert.Equal(t, 1, m.explainCursor)
	// Space again expands.
	mdl, _ = m.handleExplainKey(spaceKey())
	m = mdl.(Model)
	assert.Len(t, m.explainFields, 4)
	assert.Len(t, m.explainTreeDepths, 4)
}

func TestExplainTree_SpaceOnLeafNoop(t *testing.T) {
	m := explainTreeBaseModel()
	mdl, _ := m.updateExplainTreeLoaded(sampleTreeMsg())
	m = mdl.(Model)
	m.explainCursor = 0 // replicas (no children)
	mdl, _ = m.handleExplainKey(spaceKey())
	m = mdl.(Model)
	assert.Len(t, m.explainFields, 4)
}

func TestExplainTree_StickyAcrossLevelLoad(t *testing.T) {
	m := explainTreeBaseModel()
	m.explainTreeWanted = false // simulate the full T flow from scratch
	// Toggle on: marks the preference and starts the fetch.
	mdl, _ := m.handleExplainKey(key("T"))
	m = mdl.(Model)
	require.True(t, m.explainTreeWanted)
	mdl, _ = m.updateExplainTreeLoaded(sampleTreeMsg())
	m = mdl.(Model)
	// Drill lands on a flat level; the sticky preference re-fetches the tree.
	mdl, cmd := m.updateExplainLoaded(explainLoadedMsg{
		fields: []model.ExplainField{{Name: "metadata", Type: "<ObjectMeta>", Path: "spec.template.metadata"}},
		path:   "spec.template",
	})
	m = mdl.(Model)
	assert.False(t, m.explainTree) // flat until the tree result arrives
	assert.NotNil(t, cmd, "sticky tree mode should refetch the tree after a level load")
	// The new tree arrives for the new path.
	mdl, _ = m.updateExplainTreeLoaded(explainTreeLoadedMsg{
		fields: []model.ExplainField{{Name: "metadata", Type: "<ObjectMeta>", Path: "spec.template.metadata"}},
		path:   "spec.template",
	})
	m = mdl.(Model)
	assert.True(t, m.explainTree)
}

func TestExplainTree_StickyAcrossCachedBack(t *testing.T) {
	m := explainTreeBaseModel()
	m.explainTreeWanted = false // simulate the full T flow from scratch
	m.explainAncestors = []explainLevel{{
		fields: []model.ExplainField{{Name: "spec", Type: "<DeploymentSpec>", Path: "spec"}},
		path:   "",
	}}
	mdl, _ := m.handleExplainKey(key("T"))
	m = mdl.(Model)
	mdl, _ = m.updateExplainTreeLoaded(sampleTreeMsg())
	m = mdl.(Model)
	// Back pops the cached ancestor without a level-load message; the sticky
	// preference must still refetch the tree for the parent.
	mdl, cmd := m.handleExplainKey(key("h"))
	m = mdl.(Model)
	assert.False(t, m.explainTree)
	assert.Equal(t, "", m.explainPath)
	assert.NotNil(t, cmd)
}

func TestExplainTree_SecondPressCancelsPendingLoad(t *testing.T) {
	m := explainTreeBaseModel()
	m.explainTreeWanted = false
	mdl, cmd := m.handleExplainKey(key("T")) // starts the fetch
	m = mdl.(Model)
	require.NotNil(t, cmd)
	mdl, cmd = m.handleExplainKey(key("T")) // cancels while in flight
	m = mdl.(Model)
	assert.Nil(t, cmd)
	assert.False(t, m.explainTreeWanted)
	// The in-flight result is dropped on arrival.
	mdl, _ = m.updateExplainTreeLoaded(sampleTreeMsg())
	m = mdl.(Model)
	assert.False(t, m.explainTree)
	assert.Len(t, m.explainFields, 2)
}

func TestExplainTree_ToggleOffClearsSticky(t *testing.T) {
	m := explainTreeBaseModel()
	m.explainTreeWanted = false // simulate the full T flow from scratch
	mdl, _ := m.handleExplainKey(key("T"))
	m = mdl.(Model)
	mdl, _ = m.updateExplainTreeLoaded(sampleTreeMsg())
	m = mdl.(Model)
	mdl, _ = m.handleExplainKey(key("T"))
	m = mdl.(Model)
	assert.False(t, m.explainTree)
	assert.False(t, m.explainTreeWanted)
	// Subsequent level loads stay flat.
	_, cmd := m.updateExplainLoaded(explainLoadedMsg{
		fields: []model.ExplainField{{Name: "x", Type: "<string>", Path: "spec.x"}},
		path:   "spec",
	})
	assert.Nil(t, cmd)
}

func TestExplainTree_LoadedError(t *testing.T) {
	m := explainTreeBaseModel()
	mdl, _ := m.updateExplainTreeLoaded(explainTreeLoadedMsg{err: assert.AnError})
	m = mdl.(Model)
	assert.False(t, m.explainTree)
	assert.True(t, m.statusMessageErr)
}

func TestExplainTree_ToggleOffRestoresFlat(t *testing.T) {
	m := explainTreeBaseModel()
	mdl, _ := m.updateExplainTreeLoaded(sampleTreeMsg())
	m = mdl.(Model)
	mdl, cmd := m.handleExplainKey(key("T"))
	m = mdl.(Model)
	assert.False(t, m.explainTree)
	assert.Nil(t, cmd)
	require.Len(t, m.explainFields, 2)
	assert.Equal(t, "template", m.explainFields[1].Name)
	// Cursor restored to the pre-tree position.
	assert.Equal(t, 1, m.explainCursor)
}

func TestExplainTree_StaleResultIgnored(t *testing.T) {
	m := explainTreeBaseModel()
	// The user drilled elsewhere while the recursive fetch was in flight.
	m.explainPath = "spec.template"
	mdl, _ := m.updateExplainTreeLoaded(sampleTreeMsg()) // path: "spec"
	m = mdl.(Model)
	assert.False(t, m.explainTree)
	assert.Len(t, m.explainFields, 2)
}

func TestExplainTree_LevelLoadResetsTree(t *testing.T) {
	m := explainTreeBaseModel()
	mdl, _ := m.updateExplainTreeLoaded(sampleTreeMsg())
	m = mdl.(Model)
	mdl, _ = m.updateExplainLoaded(explainLoadedMsg{
		fields: []model.ExplainField{{Name: "metadata", Type: "<ObjectMeta>", Path: "spec.template.metadata"}},
		path:   "spec.template",
	})
	m = mdl.(Model)
	assert.False(t, m.explainTree)
	assert.Nil(t, m.explainTreeDepths)
}

func TestExplainTree_ViewRendersGuides(t *testing.T) {
	m := explainTreeBaseModel()
	mdl, _ := m.updateExplainTreeLoaded(sampleTreeMsg())
	m = mdl.(Model)
	view := stripANSI(m.viewExplain())
	assert.Contains(t, view, "├─ replicas")
	assert.Contains(t, view, "└─ template")
	assert.Contains(t, view, "[TREE]")
}

func TestExplainTreeDepths_RelativeToBase(t *testing.T) {
	fields := []model.ExplainField{
		{Path: "spec.replicas"},
		{Path: "spec.template"},
		{Path: "spec.template.spec"},
	}
	assert.Equal(t, []int{0, 0, 1}, explainTreeDepths(fields, "spec"))
	// Root-level tree (no base path).
	rootFields := []model.ExplainField{
		{Path: "spec"},
		{Path: "spec.replicas"},
	}
	assert.Equal(t, []int{0, 1}, explainTreeDepths(rootFields, ""))
}
