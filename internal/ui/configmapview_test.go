package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- configMapValueDisplay ---

func TestConfigMapValueDisplay(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		maxW     int
		wantSub  string
		wantDots bool
	}{
		{
			name:    "single line value fits",
			val:     "hello",
			maxW:    20,
			wantSub: "hello",
		},
		{
			name:    "single line value truncated",
			val:     "very-long-value-that-exceeds-width",
			maxW:    10,
			wantSub: "very-long",
		},
		{
			name:     "multiline value shows first line with dots",
			val:      "first line\nsecond line\nthird line",
			maxW:     30,
			wantSub:  "first line",
			wantDots: true,
		},
		{
			name:     "multiline with long first line",
			val:      "a-very-long-first-line-here\nsecond",
			maxW:     15,
			wantDots: true,
		},
		{
			name:    "empty value",
			val:     "",
			maxW:    20,
			wantSub: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := configMapValueDisplay(tt.val, tt.maxW)
			if tt.wantSub != "" {
				assert.Contains(t, result, tt.wantSub)
			}
			if tt.wantDots {
				assert.Contains(t, result, "...")
			}
		})
	}
}

// --- renderConfigMapEditorTable ---

func TestRenderConfigMapEditorTable(t *testing.T) {
	t.Run("empty configmap shows add hint", func(t *testing.T) {
		cm := &model.ConfigMapData{
			Keys: []string{},
			Data: map[string]string{},
		}
		result := renderConfigMapEditorTable(cm, 0, false, "", "", 0, 60, 20)
		assert.Contains(t, result, "Key")
		assert.Contains(t, result, "Value")
		assert.Contains(t, result, "(empty - press 'a' to add a key)")
	})

	t.Run("configmap with entries shows keys and values", func(t *testing.T) {
		cm := &model.ConfigMapData{
			Keys: []string{"APP_NAME", "DB_HOST"},
			Data: map[string]string{
				"APP_NAME": "myapp",
				"DB_HOST":  "localhost",
			},
		}
		result := renderConfigMapEditorTable(cm, 0, false, "", "", 0, 80, 20)
		assert.Contains(t, result, "Key")
		assert.Contains(t, result, "Value")
		assert.Contains(t, result, "APP_NAME")
		assert.Contains(t, result, "myapp")
		assert.Contains(t, result, "DB_HOST")
		assert.Contains(t, result, "localhost")
	})

	t.Run("selected row shows cursor indicator", func(t *testing.T) {
		cm := &model.ConfigMapData{
			Keys: []string{"key1", "key2"},
			Data: map[string]string{"key1": "val1", "key2": "val2"},
		}
		result := renderConfigMapEditorTable(cm, 1, false, "", "", 0, 60, 20)
		assert.Contains(t, result, ">")
		assert.Contains(t, result, "key2")
	})

	t.Run("editing key column shows edit cursor", func(t *testing.T) {
		cm := &model.ConfigMapData{
			Keys: []string{"mykey"},
			Data: map[string]string{"mykey": "myval"},
		}
		result := renderConfigMapEditorTable(cm, 0, true, "newkey", "", 0, 60, 20)
		assert.Contains(t, result, "newkey")
		assert.Contains(t, result, "\u2588")
	})

	t.Run("editing value column shows edit cursor", func(t *testing.T) {
		cm := &model.ConfigMapData{
			Keys: []string{"mykey"},
			Data: map[string]string{"mykey": "myval"},
		}
		result := renderConfigMapEditorTable(cm, 0, true, "", "newval", 1, 60, 20)
		assert.Contains(t, result, "newval")
		assert.Contains(t, result, "\u2588")
	})
}

// --- RenderConfigMapEditorOverlay ---

func TestRenderConfigMapEditorOverlay(t *testing.T) {
	t.Run("nil configmap shows error", func(t *testing.T) {
		result := RenderConfigMapEditorOverlay(nil, 0, false, "", "", 0, 100, 40)
		assert.Contains(t, result, "No configmap loaded")
	})

	t.Run("normal mode hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		cm := &model.ConfigMapData{
			Keys: []string{"key1"},
			Data: map[string]string{"key1": "value1"},
		}
		result := RenderConfigMapEditorOverlay(cm, 0, false, "", "", 0, 100, 40)
		assert.Contains(t, result, "ConfigMap Editor")
		assert.Contains(t, result, "key1")
	})

	t.Run("editing mode hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		cm := &model.ConfigMapData{
			Keys: []string{"key1"},
			Data: map[string]string{"key1": "value1"},
		}
		result := RenderConfigMapEditorOverlay(cm, 0, true, "key1", "value1", 1, 100, 40)
		assert.Contains(t, result, "ConfigMap Editor")
	})

	t.Run("small screen clamps dimensions", func(t *testing.T) {
		cm := &model.ConfigMapData{
			Keys: []string{"k"},
			Data: map[string]string{"k": "v"},
		}
		result := RenderConfigMapEditorOverlay(cm, 0, false, "", "", 0, 30, 10)
		assert.Contains(t, result, "ConfigMap Editor")
	})
}
