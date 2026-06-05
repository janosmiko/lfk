package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func sampleObjectFields() []model.ObjectField {
	return []model.ObjectField{
		{Key: "apiVersion", Type: "<string>", Preview: "chore.example.com/v1"},
		{Key: "phase", Type: "<string>", Preview: "Succeeded"},
		{Key: "metadata", Type: "<Object>", Preview: "object (3 fields)", HasChildren: true},
	}
}

func TestRenderObjectExplorerView_InlineValuesAndPreview(t *testing.T) {
	parent := []model.ObjectField{{Key: "status", Type: "<Object>", HasChildren: true}}
	out := RenderObjectExplorerView(
		sampleObjectFields(), 1, 0,
		parent, 0,
		"phase: Succeeded\nname: build\n",
		0, "", "hint", 120, 30,
	)
	assert.Contains(t, out, "PREVIEW")
	// Inline value preview is shown next to keys.
	assert.Contains(t, out, "Succeeded")
	assert.Contains(t, out, "chore.example.com/v1")
	// The YAML subtree preview renders on the right.
	assert.Contains(t, out, "name")
	// The left pane shows the parent level (name may be truncated in the
	// narrow 12%-width column, so match a prefix).
	assert.Contains(t, out, "PARENT")
	assert.Contains(t, out, "statu")
}

func TestRenderObjectExplorerView_NoDrillHint(t *testing.T) {
	out := RenderObjectExplorerView(
		sampleObjectFields(), 2, 0, nil, 0,
		"a: 1\n", 0, "", "hint", 120, 30,
	)
	// The old explain renderer's drill hint must not appear here.
	assert.NotContains(t, out, "Press l")
	assert.NotContains(t, out, "drill into this field")
}

func TestRenderObjectExplorerView_TopLevelParentEmpty(t *testing.T) {
	out := RenderObjectExplorerView(
		sampleObjectFields(), 0, 0, nil, 0,
		"a: 1\n", 0, "", "hint", 120, 30,
	)
	assert.Contains(t, out, "(top level)")
}

func TestRenderObjectExplorerView_EmptyPreview(t *testing.T) {
	out := RenderObjectExplorerView(
		sampleObjectFields(), 0, 0, nil, 0,
		"", 0, "", "hint", 120, 30,
	)
	assert.Contains(t, out, "(empty)")
}

func TestRenderObjectExplorerView_NoFields(t *testing.T) {
	out := RenderObjectExplorerView(nil, 0, 0, nil, 0, "", 0, "", "hint", 80, 20)
	assert.Contains(t, out, "(no fields)")
}
