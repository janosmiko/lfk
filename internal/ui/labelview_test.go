package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- renderLabelEditorTable ---

func TestRenderLabelEditorTable(t *testing.T) {
	t.Run("empty data shows add hint", func(t *testing.T) {
		result := renderLabelEditorTable(nil, nil, 0, false, "", "", 0, 60, 20)
		assert.Contains(t, result, "Key")
		assert.Contains(t, result, "Value")
		assert.Contains(t, result, "(empty - press 'a' to add)")
	})

	t.Run("shows keys and values", func(t *testing.T) {
		keys := []string{"app", "env"}
		data := map[string]string{"app": "nginx", "env": "production"}
		result := renderLabelEditorTable(keys, data, 0, false, "", "", 0, 80, 20)
		assert.Contains(t, result, "app")
		assert.Contains(t, result, "nginx")
		assert.Contains(t, result, "env")
		assert.Contains(t, result, "production")
	})

	t.Run("selected row shows cursor", func(t *testing.T) {
		keys := []string{"k1", "k2"}
		data := map[string]string{"k1": "v1", "k2": "v2"}
		result := renderLabelEditorTable(keys, data, 1, false, "", "", 0, 60, 20)
		assert.Contains(t, result, ">")
		assert.Contains(t, result, "k2")
	})

	t.Run("editing key column shows edit cursor", func(t *testing.T) {
		keys := []string{"mykey"}
		data := map[string]string{"mykey": "myval"}
		result := renderLabelEditorTable(keys, data, 0, true, "newkey", "", 0, 60, 20)
		assert.Contains(t, result, "newkey")
		assert.Contains(t, result, "\u2588")
	})

	t.Run("editing value column shows edit cursor", func(t *testing.T) {
		keys := []string{"mykey"}
		data := map[string]string{"mykey": "myval"}
		result := renderLabelEditorTable(keys, data, 0, true, "", "newval", 1, 60, 20)
		assert.Contains(t, result, "newval")
		assert.Contains(t, result, "\u2588")
	})
}

// --- RenderLabelEditorOverlay ---

func TestRenderLabelEditorOverlay(t *testing.T) {
	t.Run("nil data shows error", func(t *testing.T) {
		result := RenderLabelEditorOverlay(nil, 0, 0, false, "", "", 0, 100, 40)
		assert.Contains(t, result, "No data loaded")
	})

	t.Run("labels tab shows label data", func(t *testing.T) {
		data := &model.LabelAnnotationData{
			Labels:      map[string]string{"app": "nginx"},
			LabelKeys:   []string{"app"},
			Annotations: map[string]string{"note": "test"},
			AnnotKeys:   []string{"note"},
		}
		result := RenderLabelEditorOverlay(data, 0, 0, false, "", "", 0, 100, 40)
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
		result := RenderLabelEditorOverlay(data, 0, 1, false, "", "", 0, 100, 40)
		assert.Contains(t, result, "note")
		assert.Contains(t, result, "important")
	})

	t.Run("editing mode shows save help", func(t *testing.T) {
		data := &model.LabelAnnotationData{
			Labels:    map[string]string{"k": "v"},
			LabelKeys: []string{"k"},
		}
		result := RenderLabelEditorOverlay(data, 0, 0, true, "k", "v", 0, 100, 40)
		assert.Contains(t, result, "Label / Annotation Editor")
	})

	t.Run("normal mode hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		data := &model.LabelAnnotationData{
			Labels:    map[string]string{"k": "v"},
			LabelKeys: []string{"k"},
		}
		result := RenderLabelEditorOverlay(data, 0, 0, false, "", "", 0, 100, 40)
		assert.Contains(t, result, "Label / Annotation Editor")
	})
}
