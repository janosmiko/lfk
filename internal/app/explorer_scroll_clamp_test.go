package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// Regression tests: the Object/API Explorer scroll clamps must agree with the
// rows the renderer actually draws (the clamp used to think the window was
// 1-2 rows taller, letting the cursor walk out of the viewport), and both
// views honour the vim-style scrolloff margin (ui.VimScrollOff).

// wideObjectExplorerModel opens the Object Explorer on an object with 50
// scalar root keys k00..k49.
func wideObjectExplorerModel(t *testing.T) Model {
	t.Helper()
	raw := map[string]any{}
	for i := range 50 {
		raw[fmt.Sprintf("k%02d", i)] = fmt.Sprintf("v%02d", i)
	}
	m := objectExplorerModel(t)
	m.middleItems = []model.Item{{Name: "wide", Kind: "Chore", Raw: raw}}
	result, _ := m.openObjectExplorer()
	return result.(Model)
}

func TestObjectExplorer_BottomRowVisibleFlat(t *testing.T) {
	m := wideObjectExplorerModel(t)
	m = pressTree(m, key("G"))
	view := stripANSI(m.viewObjectExplorer())
	assert.Contains(t, view, "k49", "bottom row must be inside the rendered window")
}

func TestObjectExplorerTree_BottomRowVisible(t *testing.T) {
	m := wideObjectExplorerModel(t)
	m = pressTree(m, key("T"))
	m = pressTree(m, key("G"))
	view := stripANSI(m.viewObjectExplorer())
	assert.Contains(t, view, "k49", "bottom row must be inside the rendered window")
}

func TestObjectExplorerTree_ScrollOffRespected(t *testing.T) {
	m := wideObjectExplorerModel(t)
	m = pressTree(m, key("T"))
	for range 40 {
		m = pressTree(m, key("j"))
	}
	view := stripANSI(m.viewObjectExplorer())
	// With scrolloff (default 5), 5 rows below the cursor stay visible.
	assert.Contains(t, view, "k40")
	assert.Contains(t, view, "k45", "scrolloff margin below the cursor must stay visible")
}

// wideExplainModel puts the API Explorer on a level with 50 fields f00..f49.
func wideExplainModel() Model {
	m := explainTreeBaseModel()
	fields := make([]model.ExplainField, 0, 50)
	for i := range 50 {
		name := fmt.Sprintf("f%02d", i)
		fields = append(fields, model.ExplainField{Name: name, Type: "<string>", Path: "spec." + name})
	}
	m.explainFields = fields
	m.explainCursor = 0
	m.explainTreeWanted = false
	return m
}

func TestExplain_BottomRowVisibleFlat(t *testing.T) {
	m := wideExplainModel()
	mdl, _ := m.handleExplainKey(key("G"))
	m = mdl.(Model)
	view := stripANSI(m.viewExplain())
	assert.Contains(t, view, "f49", "bottom row must be inside the rendered window")
}

func TestExplainTree_ScrollOffRespected(t *testing.T) {
	m := wideExplainModel()
	m.explainTreeWanted = true
	fields := make([]model.ExplainField, 0, 50)
	for i := range 50 {
		name := fmt.Sprintf("f%02d", i)
		fields = append(fields, model.ExplainField{Name: name, Type: "<string>", Path: "spec." + name})
	}
	mdl, _ := m.updateExplainTreeLoaded(explainTreeLoadedMsg{fields: fields, path: "spec"})
	m = mdl.(Model)
	require.True(t, m.explainTree)
	for range 40 {
		mdl, _ = m.handleExplainKey(key("j"))
		m = mdl.(Model)
	}
	view := stripANSI(m.viewExplain())
	assert.Contains(t, view, "f40")
	assert.Contains(t, view, "f45", "scrolloff margin below the cursor must stay visible")
}

func TestExplainView_NoDrillHintInDescription(t *testing.T) {
	m := wideExplainModel()
	m.explainFields[0].Type = "<Object>" // drillable
	m.explainCursor = 0
	view := stripANSI(m.viewExplain())
	assert.False(t, strings.Contains(view, "Press l or Enter"), "right-pane drill hint was removed")
}
