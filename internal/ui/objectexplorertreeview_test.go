package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func sampleObjectTreeRows() []model.ObjectTreeRow {
	return []model.ObjectTreeRow{
		{Field: model.ObjectField{Key: "spec", Type: "<Object>", Preview: "object (2 fields)", HasChildren: true}, Segs: []string{"spec"}, Depth: 0},
		{Field: model.ObjectField{Key: "containers", Type: "<[]Object>", Preview: "array (1 items)", HasChildren: true}, Segs: []string{"spec", "containers"}, Depth: 1},
		{Field: model.ObjectField{Key: "[0]", Type: "<Object>", Preview: "app", HasChildren: true}, Segs: []string{"spec", "containers", "[0]"}, Depth: 2},
		{Field: model.ObjectField{Key: "image", Type: "<string>", Preview: "nginx"}, Segs: []string{"spec", "containers", "[0]", "image"}, Depth: 3},
		{Field: model.ObjectField{Key: "replicas", Type: "<integer>", Preview: "3"}, Segs: []string{"spec", "replicas"}, Depth: 1},
	}
}

func TestRenderObjectExplorerTreeView_Guides(t *testing.T) {
	out := RenderObjectExplorerTreeView(
		sampleObjectTreeRows(), false, 0, 0,
		"Object Explorer: Deployment/web",
		nil, 0,
		"a: 1\n", 0, "", "hint", 120, 30,
	)
	plain := stripANSI(out)
	// Tree guides connect siblings and nest children.
	assert.Contains(t, plain, "└─ spec")
	assert.Contains(t, plain, "├─ containers")
	assert.Contains(t, plain, "└─ replicas")
	// Nested rows carry continuation stems from their ancestors.
	assert.Contains(t, plain, "│  └─ [0]")
	// Values still render inline.
	assert.Contains(t, plain, "nginx")
	assert.Contains(t, plain, "3")
	// No drill marker in tree mode (children are already expanded).
	assert.NotContains(t, plain, "›")
}

func TestRenderObjectExplorerTreeView_FilteredShowsPaths(t *testing.T) {
	rows := []model.ObjectTreeRow{
		{Field: model.ObjectField{Key: "image", Type: "<string>", Preview: "nginx"}, Segs: []string{"spec", "containers", "[0]", "image"}, Depth: 3},
	}
	out := RenderObjectExplorerTreeView(
		rows, true, 0, 0, "Object Explorer: x", nil, 0,
		"", 0, "image", "hint", 160, 30,
	)
	plain := stripANSI(out)
	// While filtering, rows show their full relative path instead of guides.
	assert.Contains(t, plain, "spec.containers[0].image")
	assert.NotContains(t, plain, "├─")
	assert.NotContains(t, plain, "└─")
}

func TestRenderObjectExplorerTreeView_Empty(t *testing.T) {
	out := RenderObjectExplorerTreeView(nil, false, 0, 0, "Object Explorer: x", nil, 0, "", 0, "", "hint", 80, 20)
	assert.Contains(t, stripANSI(out), "(no fields)")
}

func TestRenderObjectExplorerTreeView_SelectedRowHighlighted(t *testing.T) {
	out := RenderObjectExplorerTreeView(
		sampleObjectTreeRows(), false, 3, 0, "Object Explorer: x", nil, 0,
		"", 0, "", "hint", 120, 30,
	)
	plain := stripANSI(out)
	// The selected nested row still renders its guide prefix and key.
	for line := range strings.SplitSeq(plain, "\n") {
		if strings.Contains(line, "image") {
			assert.Contains(t, line, "└─ image")
			return
		}
	}
	t.Fatal("selected image row not found")
}
