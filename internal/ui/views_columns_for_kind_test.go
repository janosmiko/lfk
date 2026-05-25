package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColumnsForKind_ReadsFromView(t *testing.T) {
	resetViewsGlobals(t)
	origRC := ConfigResourceColumns
	t.Cleanup(func() { ConfigResourceColumns = origRC })

	v, err := BuildView(&ConfigView{
		Columns: []string{"Name", "GitSHA:.metadata.labels.git-sha", "Age"},
	})
	if err != nil {
		t.Fatal(err)
	}
	ConfigViews = map[string]*View{"pod": v}
	ConfigResourceColumns = nil

	cols := ColumnsForKind(ResourceRef{Kind: "Pod"}, "")
	assert.Equal(t, []string{"Name", "GitSHA", "Age"}, cols)
}

func TestColumnsForKind_ResourceColumnsWinsOverViewInSameScope(t *testing.T) {
	resetViewsGlobals(t)
	origRC := ConfigResourceColumns
	t.Cleanup(func() { ConfigResourceColumns = origRC })

	v, _ := BuildView(&ConfigView{Columns: []string{"Name", "X:.foo"}})
	ConfigViews = map[string]*View{"pod": v}
	ConfigResourceColumns = map[string][]string{"pod": {"Name", "Status"}}

	cols := ColumnsForKind(ResourceRef{Kind: "Pod"}, "")
	assert.Equal(t, []string{"Name", "Status"}, cols, "explicit resource_columns wins over views in the same scope")
}

func TestColumnsForKind_WideOnlyFiltered(t *testing.T) {
	resetViewsGlobals(t)
	origRC := ConfigResourceColumns
	origFullscreen := ActiveFullscreenMode
	t.Cleanup(func() {
		ConfigResourceColumns = origRC
		ActiveFullscreenMode = origFullscreen
	})

	v, err := BuildView(&ConfigView{
		Columns: []string{
			"Name",
			"GitSHA:.metadata.labels.git-sha|W",
			"Age",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ConfigViews = map[string]*View{"pod": v}
	ConfigResourceColumns = nil

	t.Run("narrow mode hides |W column", func(t *testing.T) {
		ActiveFullscreenMode = false
		assert.Equal(t, []string{"Name", "Age"}, ColumnsForKind(ResourceRef{Kind: "Pod"}, ""))
	})

	t.Run("fullscreen mode reveals |W column", func(t *testing.T) {
		ActiveFullscreenMode = true
		assert.Equal(t, []string{"Name", "GitSHA", "Age"}, ColumnsForKind(ResourceRef{Kind: "Pod"}, ""))
	})
}

func TestColumnsForKind_PerClusterViewWinsOverGlobal(t *testing.T) {
	resetViewsGlobals(t)
	origRC := ConfigResourceColumns
	t.Cleanup(func() { ConfigResourceColumns = origRC })

	globalView, _ := BuildView(&ConfigView{Columns: []string{"Name", "Age"}})
	prodView, _ := BuildView(&ConfigView{Columns: []string{"Name", "X:.foo", "Y:.bar"}})
	ConfigViews = map[string]*View{"pod": globalView}
	ConfigClusterViews = map[string]map[string]*View{
		"prod": {"pod": prodView},
	}

	cols := ColumnsForKind(ResourceRef{Kind: "Pod"}, "prod")
	assert.Equal(t, []string{"Name", "X", "Y"}, cols)

	cols = ColumnsForKind(ResourceRef{Kind: "Pod"}, "dev")
	assert.Equal(t, []string{"Name", "Age"}, cols, "fallback to global view when per-cluster has none")
}

// Issue #262 regression: a GVR-keyed view's columns list must be returned
// when only a GVR key is configured. Previously ColumnsForKind looked up
// by Kind only, so users got the default columns instead of their config.
func TestColumnsForKind_GVRKeyedViewReturnsColumns(t *testing.T) {
	resetViewsGlobals(t)
	origRC := ConfigResourceColumns
	t.Cleanup(func() { ConfigResourceColumns = origRC })

	v, _ := BuildView(&ConfigView{Columns: []string{"Name", "Replicas", "Available", "REV:.metadata.resourceVersion", "Age"}})
	ConfigViews = map[string]*View{"apps/v1/deployments": v}
	ConfigResourceColumns = nil

	rt := ResourceRef{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}
	cols := ColumnsForKind(rt, "")
	assert.Equal(t, []string{"Name", "Replicas", "Available", "REV", "Age"}, cols)
}

// GVR takes precedence over Kind when both are configured.
func TestColumnsForKind_GVRWinsOverKind(t *testing.T) {
	resetViewsGlobals(t)
	origRC := ConfigResourceColumns
	t.Cleanup(func() { ConfigResourceColumns = origRC })

	gvr, _ := BuildView(&ConfigView{Columns: []string{"Name", "Replicas"}})
	kind, _ := BuildView(&ConfigView{Columns: []string{"Name", "Available", "Age"}})
	ConfigViews = map[string]*View{
		"apps/v1/deployments": gvr,
		"deployment":          kind,
	}
	ConfigResourceColumns = nil

	rt := ResourceRef{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}
	cols := ColumnsForKind(rt, "")
	assert.Equal(t, []string{"Name", "Replicas"}, cols, "GVR view wins over Kind view")
}
