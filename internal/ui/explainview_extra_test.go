package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- PadToHeight (was padExplainToHeight) ---

func TestPadExplainToHeight(t *testing.T) {
	t.Run("already at height", func(t *testing.T) {
		input := "a\nb\nc"
		result := PadToHeight(input, 3)
		assert.Equal(t, 3, len(strings.Split(result, "\n")))
	})

	t.Run("shorter than height padded", func(t *testing.T) {
		result := PadToHeight("line1", 4)
		lines := strings.Split(result, "\n")
		assert.Equal(t, 4, len(lines))
		assert.Equal(t, "line1", lines[0])
	})

	t.Run("taller than height truncated", func(t *testing.T) {
		input := "a\nb\nc\nd\ne"
		result := PadToHeight(input, 3)
		lines := strings.Split(result, "\n")
		assert.Equal(t, 3, len(lines))
	})
}

// --- renderExplainKeyList ---

func TestRenderExplainKeyList(t *testing.T) {
	t.Run("empty is top level", func(t *testing.T) {
		lines := renderExplainKeyList(nil, 0, 20, 10)
		assert.Contains(t, strings.Join(lines, "\n"), "top level")
	})

	t.Run("keys only, no arrows or markers", func(t *testing.T) {
		fields := []model.ExplainField{
			{Name: "spec", Type: "<Object>"},
			{Name: "status", Type: "<Object>"},
		}
		out := strings.Join(renderExplainKeyList(fields, 1, 20, 10), "\n")
		assert.Contains(t, out, "spec")
		assert.Contains(t, out, "status")
		assert.NotContains(t, out, "›")  // no trailing drill marker
		assert.NotContains(t, out, "> ") // no leading cursor arrow
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
	t.Run("renders the column layout (no internal title)", func(t *testing.T) {
		fields := []model.ExplainField{
			{Name: "apiVersion", Type: "<string>", Description: "API version"},
			{Name: "kind", Type: "<string>", Description: "Resource kind"},
			{Name: "spec", Type: "<Object>", Description: "Spec of the resource."},
		}
		result := RenderExplainView(fields, 0, 0, "A deployment.", nil, 0, "", "hint bar", 120, 30)
		assert.NotContains(t, result, "API Explorer:") // title now lives in the breadcrumb
		assert.Contains(t, result, "NAME")
		assert.Contains(t, result, "DESCRIPTION")
		assert.Contains(t, result, "apiVersion")
		assert.Contains(t, result, "hint bar")
	})

	t.Run("parent pane shows parent keys", func(t *testing.T) {
		fields := []model.ExplainField{{Name: "containers", Type: "<[]Container>"}}
		parent := []model.ExplainField{{Name: "template", Type: "<Object>"}}
		result := RenderExplainView(fields, 0, 0, "", parent, 0, "", "hints", 120, 30)
		assert.Contains(t, result, "PARENT")
		assert.Contains(t, result, "template")
		assert.Contains(t, result, "containers")
	})

	t.Run("empty fields shows no fields message", func(t *testing.T) {
		result := RenderExplainView(nil, 0, 0, "Some desc", nil, 0, "", "", 80, 20)
		assert.Contains(t, result, "No fields found")
	})
}

// --- RenderExplainSearchOverlay ---

func TestRenderExplainSearchOverlay(t *testing.T) {
	t.Run("empty results with no filter shows field count", func(t *testing.T) {
		result := RenderExplainSearchOverlay(nil, 0, 0, 15, "", false, 60)
		assert.Contains(t, result, "Recursive Field Browser")
		assert.Contains(t, result, "0 fields")
		// Hints now live in the footer, not inline navigation text.
		assert.NotContains(t, result, "Enter: navigate")
	})

	t.Run("empty results with filter shows no matching", func(t *testing.T) {
		result := RenderExplainSearchOverlay(nil, 0, 0, 15, "xyz", false, 60)
		assert.Contains(t, result, "No matching fields")
	})

	t.Run("results rendered with names and types", func(t *testing.T) {
		results := []model.ExplainField{
			{Name: "containers", Type: "<[]Container>", Path: "spec.template.spec"},
			{Name: "image", Type: "<string>", Path: "spec.template.spec.containers"},
		}
		result := RenderExplainSearchOverlay(results, 0, 0, 15, "", false, 60)
		assert.Contains(t, result, "2 fields")
		assert.Contains(t, result, "containers")
		assert.Contains(t, result, "<[]Container>")
		assert.Contains(t, result, "image")
	})

	t.Run("cursor selects an item", func(t *testing.T) {
		results := []model.ExplainField{
			{Name: "alpha", Type: "<string>", Path: "spec"},
			{Name: "beta", Type: "<Object>", Path: "spec"},
		}
		result := RenderExplainSearchOverlay(results, 1, 0, 15, "", false, 60)
		// Both items render; the cursor highlight is a background style, not "> ".
		assert.Contains(t, result, "alpha")
		assert.Contains(t, result, "beta")
	})

	t.Run("filter active shows cursor block", func(t *testing.T) {
		result := RenderExplainSearchOverlay(nil, 0, 0, 15, "test", true, 60)
		assert.Contains(t, result, "\u2588")
		assert.Contains(t, result, "test")
	})

	t.Run("no filter shows placeholder", func(t *testing.T) {
		result := RenderExplainSearchOverlay(nil, 0, 0, 15, "", false, 60)
		assert.Contains(t, result, "/ to filter")
	})

	t.Run("scrollbar shown when overflowing", func(t *testing.T) {
		results := make([]model.ExplainField, 30)
		for i := range results {
			results[i] = model.ExplainField{Name: "field", Type: "<string>", Path: "p"}
		}
		result := RenderExplainSearchOverlay(results, 0, 5, 10, "", false, 60)
		// Unified scrollbar track glyph.
		assert.Contains(t, result, "\u2502")
	})

	t.Run("no inline hotkey footer (hints live in the bottom bar)", func(t *testing.T) {
		results := []model.ExplainField{{Name: "a", Type: "<string>"}}
		result := RenderExplainSearchOverlay(results, 0, 0, 15, "", false, 60)
		assert.NotContains(t, result, "esc close")
	})
}
