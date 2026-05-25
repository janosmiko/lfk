package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func TestResourceColumnsBridgesToViews(t *testing.T) {
	resetViewsGlobals(t)
	origRC := ConfigResourceColumns
	origCRC := ConfigClusterResourceColumns
	t.Cleanup(func() {
		ConfigResourceColumns = origRC
		ConfigClusterResourceColumns = origCRC
	})

	yml := []byte(`
resource_columns:
  pod: [Name, Status, Ready]
  deployment: [Name, Replicas, Age]
`)
	var cfg configFile
	if err := yaml.Unmarshal(yml, &cfg); err != nil {
		t.Fatal(err)
	}
	applyConfigMaps(cfg, map[string]string{})

	t.Run("bridged view resolves by Kind", func(t *testing.T) {
		v, ok := ResolveView(ResourceRef{Kind: "Pod"}, "")
		assert.True(t, ok)
		assert.Len(t, v.Columns, 3)
		assert.Equal(t, "Name", v.Columns[0].Name)
	})

	t.Run("ConfigResourceColumns still populated for legacy resolver", func(t *testing.T) {
		assert.NotEmpty(t, ConfigResourceColumns["pod"])
	})
}

func TestViewsWinOverBridgedResourceColumns(t *testing.T) {
	resetViewsGlobals(t)
	origRC := ConfigResourceColumns
	t.Cleanup(func() { ConfigResourceColumns = origRC })

	yml := []byte(`
resource_columns:
  pod: [Name, Status]
views:
  pod:
    columns: [Name, Ready, Age]
    sort_column: "Age:desc"
`)
	var cfg configFile
	if err := yaml.Unmarshal(yml, &cfg); err != nil {
		t.Fatal(err)
	}
	applyConfigMaps(cfg, map[string]string{})

	v, ok := ResolveView(ResourceRef{Kind: "Pod"}, "")
	assert.True(t, ok)
	assert.Len(t, v.Columns, 3, "views: should win — expect 3 columns from views, not 2 from resource_columns")
	assert.Equal(t, "Age", v.SortColumn)
	assert.False(t, v.SortAsc)
}

func TestPerClusterResourceColumnsBridges(t *testing.T) {
	resetViewsGlobals(t)
	origRC := ConfigResourceColumns
	origCRC := ConfigClusterResourceColumns
	t.Cleanup(func() {
		ConfigResourceColumns = origRC
		ConfigClusterResourceColumns = origCRC
	})

	yml := []byte(`
clusters:
  prod:
    resource_columns:
      deployment: [Name, REV]
`)
	var cfg configFile
	if err := yaml.Unmarshal(yml, &cfg); err != nil {
		t.Fatal(err)
	}
	applyConfigMaps(cfg, map[string]string{})

	v, ok := ResolveView(ResourceRef{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}, "prod")
	assert.True(t, ok)
	assert.Len(t, v.Columns, 2)
}
