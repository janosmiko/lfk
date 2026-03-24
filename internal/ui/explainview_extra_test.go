package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- padExplainToHeight ---

func TestPadExplainToHeight(t *testing.T) {
	t.Run("already at height", func(t *testing.T) {
		input := "a\nb\nc"
		result := padExplainToHeight(input, 3)
		assert.Equal(t, 3, len(strings.Split(result, "\n")))
	})

	t.Run("shorter than height padded", func(t *testing.T) {
		result := padExplainToHeight("line1", 4)
		lines := strings.Split(result, "\n")
		assert.Equal(t, 4, len(lines))
		assert.Equal(t, "line1", lines[0])
	})

	t.Run("taller than height truncated", func(t *testing.T) {
		input := "a\nb\nc\nd\ne"
		result := padExplainToHeight(input, 3)
		lines := strings.Split(result, "\n")
		assert.Equal(t, 3, len(lines))
	})
}

// --- renderExplainPath ---

func TestRenderExplainPath(t *testing.T) {
	t.Run("root level shows resource name highlighted", func(t *testing.T) {
		result := renderExplainPath("", "Deployment", 30, 20)
		assert.Contains(t, result, "PATH")
		assert.Contains(t, result, "Deployment")
		assert.Contains(t, result, ">")
	})

	t.Run("drilled path shows segments", func(t *testing.T) {
		result := renderExplainPath("spec.template.metadata", "Deployment", 30, 20)
		assert.Contains(t, result, "PATH")
		assert.Contains(t, result, "Deployment")
		assert.Contains(t, result, "spec")
		assert.Contains(t, result, "template")
		assert.Contains(t, result, "metadata")
		// Last segment should be highlighted.
		assert.Contains(t, result, ">")
	})

	t.Run("single segment path", func(t *testing.T) {
		result := renderExplainPath("spec", "Pod", 30, 20)
		assert.Contains(t, result, "Pod")
		assert.Contains(t, result, "spec")
		assert.Contains(t, result, ">")
	})
}

// --- renderFieldList ---

func TestRenderFieldList(t *testing.T) {
	t.Run("empty fields shows message", func(t *testing.T) {
		lines := renderFieldList(nil, 0, 0, 40, 10, "")
		assert.Len(t, lines, 10)
		assert.Contains(t, lines[0], "No fields found")
	})

	t.Run("fields rendered with names and types", func(t *testing.T) {
		fields := []model.ExplainField{
			{Name: "apiVersion", Type: "<string>", Required: true},
			{Name: "kind", Type: "<string>", Required: true},
			{Name: "metadata", Type: "<ObjectMeta>", Required: false},
		}
		lines := renderFieldList(fields, 0, 0, 60, 10, "")
		assert.Len(t, lines, 10)
		found := strings.Join(lines, "\n")
		assert.Contains(t, found, "apiVersion")
		assert.Contains(t, found, "kind")
		assert.Contains(t, found, "metadata")
		assert.Contains(t, found, "<string>")
		assert.Contains(t, found, "<ObjectMeta>")
	})

	t.Run("cursor on field shows selection marker", func(t *testing.T) {
		fields := []model.ExplainField{
			{Name: "spec", Type: "<Object>"},
			{Name: "status", Type: "<Object>"},
		}
		lines := renderFieldList(fields, 1, 0, 60, 10, "")
		found := strings.Join(lines, "\n")
		assert.Contains(t, found, ">")
		assert.Contains(t, found, "status")
	})

	t.Run("required field shows yes indicator", func(t *testing.T) {
		fields := []model.ExplainField{
			{Name: "apiVersion", Type: "<string>", Required: true},
		}
		lines := renderFieldList(fields, 0, 0, 60, 10, "")
		found := strings.Join(lines, "\n")
		assert.Contains(t, found, "yes")
	})

	t.Run("pads to maxLines", func(t *testing.T) {
		fields := []model.ExplainField{{Name: "f", Type: "<string>"}}
		lines := renderFieldList(fields, 0, 0, 40, 8, "")
		assert.Len(t, lines, 8)
	})

	t.Run("scrolling shows later fields", func(t *testing.T) {
		fields := make([]model.ExplainField, 20)
		for i := range fields {
			fields[i] = model.ExplainField{Name: strings.Repeat("f", i+1), Type: "<string>"}
		}
		lines := renderFieldList(fields, 15, 10, 60, 5, "")
		found := strings.Join(lines, "\n")
		// The cursor field should be visible.
		assert.Contains(t, found, ">")
	})
}

// --- renderFieldDescription ---

func TestRenderFieldDescription(t *testing.T) {
	t.Run("empty fields with resource desc", func(t *testing.T) {
		lines := renderFieldDescription(nil, 0, "A deployment resource.", 40, 10)
		found := strings.Join(lines, "\n")
		assert.Contains(t, found, "A deployment resource.")
	})

	t.Run("empty fields no resource desc", func(t *testing.T) {
		lines := renderFieldDescription(nil, 0, "", 40, 10)
		found := strings.Join(lines, "\n")
		assert.Contains(t, found, "No description available")
	})

	t.Run("out of range cursor produces empty lines", func(t *testing.T) {
		fields := []model.ExplainField{{Name: "f", Type: "<string>"}}
		lines := renderFieldDescription(fields, 5, "", 40, 10)
		assert.Len(t, lines, 10)
	})

	t.Run("negative cursor produces empty lines", func(t *testing.T) {
		fields := []model.ExplainField{{Name: "f", Type: "<string>"}}
		lines := renderFieldDescription(fields, -1, "", 40, 10)
		assert.Len(t, lines, 10)
	})

	t.Run("field with description shows it", func(t *testing.T) {
		fields := []model.ExplainField{
			{Name: "spec", Type: "<Object>", Description: "Spec of the resource."},
		}
		lines := renderFieldDescription(fields, 0, "", 60, 10)
		found := strings.Join(lines, "\n")
		assert.Contains(t, found, "spec")
		assert.Contains(t, found, "TYPE: <Object>")
		assert.Contains(t, found, "Spec of the resource.")
	})

	t.Run("drillable type shows hint", func(t *testing.T) {
		fields := []model.ExplainField{
			{Name: "spec", Type: "<PodSpec>", Description: "The pod spec."},
		}
		lines := renderFieldDescription(fields, 0, "", 60, 20)
		found := strings.Join(lines, "\n")
		assert.Contains(t, found, "drill into")
	})

	t.Run("non-drillable type does not show hint", func(t *testing.T) {
		fields := []model.ExplainField{
			{Name: "name", Type: "<string>", Description: "The name."},
		}
		lines := renderFieldDescription(fields, 0, "", 60, 20)
		found := strings.Join(lines, "\n")
		assert.NotContains(t, found, "drill into")
	})

	t.Run("field without description shows placeholder", func(t *testing.T) {
		fields := []model.ExplainField{
			{Name: "spec", Type: "<Object>"},
		}
		lines := renderFieldDescription(fields, 0, "", 60, 10)
		found := strings.Join(lines, "\n")
		assert.Contains(t, found, "No description available")
	})
}

// --- RenderExplainView ---

func TestRenderExplainView(t *testing.T) {
	t.Run("basic rendering contains title and columns", func(t *testing.T) {
		fields := []model.ExplainField{
			{Name: "apiVersion", Type: "<string>", Description: "API version"},
			{Name: "kind", Type: "<string>", Description: "Resource kind"},
			{Name: "spec", Type: "<Object>", Description: "Spec of the resource."},
		}
		result := RenderExplainView(fields, 0, 0, "A deployment.", "Deployment", "", "", "hint bar", 120, 30)
		assert.Contains(t, result, "API Explorer: Deployment")
		assert.Contains(t, result, "PATH")
		assert.Contains(t, result, "NAME")
		assert.Contains(t, result, "DESCRIPTION")
		assert.Contains(t, result, "apiVersion")
		assert.Contains(t, result, "hint bar")
	})

	t.Run("drilled path shows segments", func(t *testing.T) {
		fields := []model.ExplainField{
			{Name: "containers", Type: "<[]Container>"},
		}
		result := RenderExplainView(fields, 0, 0, "", "Deployment", "spec.template", "", "hints", 120, 30)
		assert.Contains(t, result, "spec")
		assert.Contains(t, result, "template")
	})

	t.Run("empty fields shows no fields message", func(t *testing.T) {
		result := RenderExplainView(nil, 0, 0, "Some desc", "Pod", "", "", "", 80, 20)
		assert.Contains(t, result, "No fields found")
	})
}

// --- RenderExplainSearchOverlay ---

func TestRenderExplainSearchOverlay(t *testing.T) {
	t.Run("empty results with no filter shows field count", func(t *testing.T) {
		result := RenderExplainSearchOverlay(nil, 0, 0, 15, "", false)
		assert.Contains(t, result, "Recursive Field Browser")
		assert.Contains(t, result, "0 fields")
		// Hints now live in the main status bar, not inline.
		assert.NotContains(t, result, "Enter: navigate")
	})

	t.Run("empty results with filter shows no matching", func(t *testing.T) {
		result := RenderExplainSearchOverlay(nil, 0, 0, 15, "xyz", false)
		assert.Contains(t, result, "No matching fields")
	})

	t.Run("results rendered with names and types", func(t *testing.T) {
		results := []model.ExplainField{
			{Name: "containers", Type: "<[]Container>", Path: "spec.template.spec"},
			{Name: "image", Type: "<string>", Path: "spec.template.spec.containers"},
		}
		result := RenderExplainSearchOverlay(results, 0, 0, 15, "", false)
		assert.Contains(t, result, "2 fields")
		assert.Contains(t, result, "containers")
		assert.Contains(t, result, "<[]Container>")
		assert.Contains(t, result, "image")
	})

	t.Run("cursor on second item", func(t *testing.T) {
		results := []model.ExplainField{
			{Name: "a", Type: "<string>", Path: "spec"},
			{Name: "b", Type: "<Object>", Path: "spec"},
		}
		result := RenderExplainSearchOverlay(results, 1, 0, 15, "", false)
		assert.Contains(t, result, ">")
	})

	t.Run("filter active shows cursor block", func(t *testing.T) {
		result := RenderExplainSearchOverlay(nil, 0, 0, 15, "test", true)
		assert.Contains(t, result, "\u2588")
		assert.Contains(t, result, "test")
	})

	t.Run("no filter shows placeholder", func(t *testing.T) {
		result := RenderExplainSearchOverlay(nil, 0, 0, 15, "", false)
		assert.Contains(t, result, "/ to filter")
	})

	t.Run("scroll indicators shown when needed", func(t *testing.T) {
		results := make([]model.ExplainField, 30)
		for i := range results {
			results[i] = model.ExplainField{Name: "field", Type: "<string>", Path: "p"}
		}
		result := RenderExplainSearchOverlay(results, 0, 5, 10, "", false)
		assert.Contains(t, result, "more above")
		assert.Contains(t, result, "more below")
	})
}
