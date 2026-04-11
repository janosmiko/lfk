package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- renderFallbackYAML ---

func TestRenderFallbackYAML(t *testing.T) {
	t.Run("no YAML shows placeholder", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
			},
		}
		result := m.renderFallbackYAML(80, 20)
		assert.Contains(t, result, "No preview")
	})

	t.Run("previewYAML used first", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
			},
			previewYAML: "apiVersion: v1\nkind: ConfigMap\n",
			yamlContent: "fallback: content\n",
		}
		result := m.renderFallbackYAML(80, 20)
		assert.Contains(t, result, "apiVersion")
	})

	t.Run("yamlContent used as fallback", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
			},
			yamlContent: "fallback: content\n",
		}
		result := m.renderFallbackYAML(80, 20)
		assert.Contains(t, result, "fallback")
	})
}
