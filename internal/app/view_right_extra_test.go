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
			yamlView: yamlViewState{
				content: "fallback: content\n",
			},
		}
		result := m.renderFallbackYAML(80, 20)
		assert.Contains(t, result, "apiVersion")
	})

	t.Run("yamlContent used as fallback", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{
				ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"},
			},
			yamlView: yamlViewState{
				content: "fallback: content\n",
			},
		}
		result := m.renderFallbackYAML(80, 20)
		assert.Contains(t, result, "fallback")
	})
}

// --- renderRightResources: columns-less items ---

// A named resource with no summary columns must still show its YAML when the
// preview has loaded; only when there is no YAML at all should it fall back to
// the minimal identity summary.
func TestRenderRightResourcesNoColumns(t *testing.T) {
	newModel := func() Model {
		m := basePush80Model()
		m.nav.ResourceType = model.ResourceTypeEntry{Kind: "ConfigMap"}
		m.middleItems = []model.Item{
			{Name: "cm1", Namespace: "default", Kind: "ConfigMap"},
		}
		return m
	}

	t.Run("previewYAML wins over minimal summary", func(t *testing.T) {
		m := newModel()
		m.previewYAML = "apiVersion: v1\nkind: ConfigMap\n"
		result := stripANSI(m.renderRightResources(80, 20))
		assert.Contains(t, result, "apiVersion")
	})

	t.Run("yamlView content wins over minimal summary", func(t *testing.T) {
		m := newModel()
		m.yamlView = yamlViewState{content: "apiVersion: v1\nkind: ConfigMap\n"}
		result := stripANSI(m.renderRightResources(80, 20))
		assert.Contains(t, result, "apiVersion")
	})

	t.Run("no YAML renders minimal identity summary", func(t *testing.T) {
		m := newModel()
		result := stripANSI(m.renderRightResources(80, 20))
		assert.Contains(t, result, "NAME")
		assert.Contains(t, result, "cm1")
		assert.NotContains(t, result, "No preview")
	})
}

// The owned view must agree with the resources view: loaded YAML wins, and a
// columns-less resource still gets identity rows rather than "No preview".
func TestRenderRightOwnedNoColumns(t *testing.T) {
	newModel := func() Model {
		m := basePush80Model()
		m.nav.Level = model.LevelOwned
		m.middleItems = []model.Item{
			{Name: "sa-1", Namespace: "default", Kind: "ServiceAccount"},
		}
		return m
	}

	t.Run("YAML preferred when loaded", func(t *testing.T) {
		m := newModel()
		m.previewYAML = "apiVersion: v1\nkind: ServiceAccount\n"
		result := stripANSI(m.renderRightOwned(80, 20))
		assert.Contains(t, result, "apiVersion")
	})

	t.Run("identity summary instead of no preview", func(t *testing.T) {
		m := newModel()
		result := stripANSI(m.renderRightOwned(80, 20))
		assert.Contains(t, result, "sa-1")
		assert.NotContains(t, result, "No preview")
	})
}
