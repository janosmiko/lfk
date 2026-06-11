package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func sampleExplainTreeFields() ([]model.ExplainField, []int) {
	fields := []model.ExplainField{
		{Name: "replicas", Type: "<integer>", Path: "spec.replicas"},
		{Name: "template", Type: "<PodTemplateSpec>", Path: "spec.template"},
		{Name: "spec", Type: "<PodSpec>", Path: "spec.template.spec"},
		{Name: "containers", Type: "<[]Container>", Path: "spec.template.spec.containers", Required: true},
	}
	return fields, []int{0, 0, 1, 2}
}

func TestRenderExplainTreeView_Guides(t *testing.T) {
	fields, depths := sampleExplainTreeFields()
	out := RenderExplainTreeView(
		fields, depths, nil, 0, 0,
		"Deployment spec.", "API Explorer: deployments > spec",
		nil, 0, "", "hint", 120, 30,
	)
	plain := stripANSI(out)
	assert.Contains(t, plain, "├─ replicas")
	assert.Contains(t, plain, "└─ template")
	assert.Contains(t, plain, "   └─ spec")
	assert.Contains(t, plain, "      └─ containers")
	// Types still render.
	assert.Contains(t, plain, "<integer>")
	assert.Contains(t, plain, "<[]Container>")
}

func TestRenderExplainTreeView_Empty(t *testing.T) {
	out := RenderExplainTreeView(nil, nil, nil, 0, 0, "desc", "API Explorer: x", nil, 0, "", "hint", 80, 20)
	assert.Contains(t, stripANSI(out), "No fields found")
}

func TestRenderExplainTreeView_FoldedMarker(t *testing.T) {
	// "template" folded: its subtree rows are absent and it carries a marker.
	fields := []model.ExplainField{
		{Name: "replicas", Type: "<integer>", Path: "spec.replicas"},
		{Name: "template", Type: "<PodTemplateSpec>", Path: "spec.template"},
	}
	out := RenderExplainTreeView(
		fields, []int{0, 0}, []bool{false, true}, 0, 0,
		"desc", "API Explorer: x", nil, 0, "", "hint", 120, 30,
	)
	for line := range strings.SplitSeq(stripANSI(out), "\n") {
		if strings.Contains(line, "template") {
			assert.Contains(t, line, "›")
			return
		}
	}
	t.Fatal("template row not found")
}
