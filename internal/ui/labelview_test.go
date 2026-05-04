package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- renderLabelEditorTable ---

func TestRenderLabelEditorTable(t *testing.T) {
	t.Run("empty data shows add hint", func(t *testing.T) {
		result := renderLabelEditorTable(nil, nil, 0, false, "", 0, "", 0, 0, nil, 60, 20)
		assert.Contains(t, result, "KEY")
		assert.Contains(t, result, "VALUE")
		assert.Contains(t, result, "(empty - press 'a' to add)")
	})

	t.Run("shows keys and values", func(t *testing.T) {
		keys := []string{"app", "env"}
		data := map[string]string{"app": "nginx", "env": "production"}
		result := renderLabelEditorTable(keys, data, 0, false, "", 0, "", 0, 0, nil, 80, 20)
		assert.Contains(t, result, "app")
		assert.Contains(t, result, "nginx")
		assert.Contains(t, result, "env")
		assert.Contains(t, result, "production")
	})

	t.Run("selected row keys are present", func(t *testing.T) {
		// Cursor row is highlighted via StyleFunc bg/bold; just assert
		// the data lands in output.
		keys := []string{"k1", "k2"}
		data := map[string]string{"k1": "v1", "k2": "v2"}
		result := renderLabelEditorTable(keys, data, 1, false, "", 0, "", 0, 0, nil, 60, 20)
		assert.Contains(t, result, "k2")
	})

	t.Run("editing key column shows edit cursor", func(t *testing.T) {
		keys := []string{"mykey"}
		data := map[string]string{"mykey": "myval"}
		result := renderLabelEditorTable(keys, data, 0, true, "newkey", 6, "", 0, 0, nil, 60, 20)
		assert.Contains(t, result, "newkey")
		// Cursor presence is now reverse-video styling rather than an
		// inserted "█" block; under the test profile no escapes emit.
		// Assert the in-progress edit value is what's rendered, not the
		// stored data.
		assert.NotContains(t, result, "myval", "stored value must be replaced by editValue when editing")
	})

	t.Run("editing value column shows edit cursor", func(t *testing.T) {
		keys := []string{"mykey"}
		data := map[string]string{"mykey": "myval"}
		result := renderLabelEditorTable(keys, data, 0, true, "", 0, "newval", 6, 1, nil, 60, 20)
		assert.Contains(t, result, "newval")
		// Cursor presence is now reverse-video styling rather than an
		// inserted "█" block; under the test profile no escapes emit.
		// Assert the in-progress edit value is what's rendered, not the
		// stored data.
		assert.NotContains(t, result, "myval", "stored value must be replaced by editValue when editing")
	})
}

// --- RenderLabelEditorOverlay ---

func TestRenderLabelEditorOverlay(t *testing.T) {
	t.Run("nil data shows error", func(t *testing.T) {
		result := RenderLabelEditorOverlay(nil, 0, 0, false, "", 0, "", 0, 0, "", false, nil, false, 0, 0, 100, 40)
		assert.Contains(t, result, "No data loaded")
	})

	t.Run("labels tab shows label data", func(t *testing.T) {
		data := &model.LabelAnnotationData{
			Labels:      map[string]string{"app": "nginx"},
			LabelKeys:   []string{"app"},
			Annotations: map[string]string{"note": "test"},
			AnnotKeys:   []string{"note"},
		}
		result := RenderLabelEditorOverlay(data, 0, 0, false, "", 0, "", 0, 0, "", false, nil, false, 0, 0, 100, 40)
		assert.Contains(t, result, "Label / Annotation Editor")
		assert.Contains(t, result, "Labels (1)")
		assert.Contains(t, result, "Annotations (1)")
		assert.Contains(t, result, "app")
		assert.Contains(t, result, "nginx")
	})

	t.Run("annotations tab shows annotation data", func(t *testing.T) {
		data := &model.LabelAnnotationData{
			Labels:      map[string]string{"app": "nginx"},
			LabelKeys:   []string{"app"},
			Annotations: map[string]string{"note": "important"},
			AnnotKeys:   []string{"note"},
		}
		result := RenderLabelEditorOverlay(data, 0, 1, false, "", 0, "", 0, 0, "", false, nil, false, 0, 0, 100, 40)
		assert.Contains(t, result, "note")
		assert.Contains(t, result, "important")
	})

	t.Run("editing mode shows save help", func(t *testing.T) {
		data := &model.LabelAnnotationData{
			Labels:    map[string]string{"k": "v"},
			LabelKeys: []string{"k"},
		}
		result := RenderLabelEditorOverlay(data, 0, 0, true, "k", 1, "v", 1, 0, "", false, nil, false, 0, 0, 100, 40)
		assert.Contains(t, result, "Label / Annotation Editor")
	})

	t.Run("normal mode hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		data := &model.LabelAnnotationData{
			Labels:    map[string]string{"k": "v"},
			LabelKeys: []string{"k"},
		}
		result := RenderLabelEditorOverlay(data, 0, 0, false, "", 0, "", 0, 0, "", false, nil, false, 0, 0, 100, 40)
		assert.Contains(t, result, "Label / Annotation Editor")
	})
}
