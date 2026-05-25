package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func TestApplyViewsFromYAML(t *testing.T) {
	resetViewsGlobals(t)

	yml := []byte(`
views:
  apps/v1/deployments:
    columns:
      - Name
      - REV
      - "IMAGE:.spec.template.spec.containers[0].image"
    sort_column: "REV:desc"
  pod:
    columns: [Name, Status, Ready]
    sort_column: "Age"

clusters:
  prod:
    views:
      apps/v1/deployments:
        sort_column: "Name:desc"
`)
	var cfg configFile
	if err := yaml.Unmarshal(yml, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	applyConfigMaps(cfg, map[string]string{})

	t.Run("global GVR view loaded", func(t *testing.T) {
		rt := ResourceRef{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}
		v, ok := ResolveView(rt, "")
		assert.True(t, ok)
		assert.Equal(t, "REV", v.SortColumn)
		assert.False(t, v.SortAsc)
		assert.Len(t, v.Columns, 3)
		assert.True(t, v.Columns[2].IsCustom())
		assert.NotNil(t, v.Columns[2].Compiled)
	})

	t.Run("global Kind view loaded", func(t *testing.T) {
		rt := ResourceRef{Group: "", Version: "v1", Resource: "pods", Kind: "Pod"}
		v, ok := ResolveView(rt, "")
		assert.True(t, ok)
		assert.Equal(t, "Age", v.SortColumn)
		assert.True(t, v.SortAsc)
		assert.Len(t, v.Columns, 3)
	})

	t.Run("per-cluster override wins", func(t *testing.T) {
		rt := ResourceRef{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}
		v, ok := ResolveView(rt, "prod")
		assert.True(t, ok)
		assert.Equal(t, "Name", v.SortColumn)
		assert.False(t, v.SortAsc)
	})

	t.Run("per-cluster fallback to global", func(t *testing.T) {
		rt := ResourceRef{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}
		v, ok := ResolveView(rt, "dev")
		assert.True(t, ok)
		assert.Equal(t, "REV", v.SortColumn)
	})
}

func TestApplyViewsFromYAML_InvalidViewSkipped(t *testing.T) {
	resetViewsGlobals(t)

	yml := []byte(`
views:
  bad-view:
    columns: ["X:.spec.foo[invalid"]
  good-view:
    columns: [Name]
    sort_column: "Name"
`)
	var cfg configFile
	if err := yaml.Unmarshal(yml, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	applyConfigMaps(cfg, map[string]string{})

	_, ok := ConfigViews["bad-view"]
	assert.False(t, ok, "invalid view should be skipped")

	_, ok = ConfigViews["good-view"]
	assert.True(t, ok, "valid view alongside an invalid one should still load")
}

func resetViewsGlobals(t *testing.T) {
	t.Helper()
	origG := ConfigViews
	origC := ConfigClusterViews
	ConfigViews = nil
	ConfigClusterViews = nil
	t.Cleanup(func() {
		ConfigViews = origG
		ConfigClusterViews = origC
	})
}
