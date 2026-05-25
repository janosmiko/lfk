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

	cols := ColumnsForKind("Pod", "")
	assert.Equal(t, []string{"Name", "GitSHA", "Age"}, cols)
}

func TestColumnsForKind_ResourceColumnsWinsOverViewInSameScope(t *testing.T) {
	resetViewsGlobals(t)
	origRC := ConfigResourceColumns
	t.Cleanup(func() { ConfigResourceColumns = origRC })

	v, _ := BuildView(&ConfigView{Columns: []string{"Name", "X:.foo"}})
	ConfigViews = map[string]*View{"pod": v}
	ConfigResourceColumns = map[string][]string{"pod": {"Name", "Status"}}

	cols := ColumnsForKind("Pod", "")
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
		assert.Equal(t, []string{"Name", "Age"}, ColumnsForKind("Pod", ""))
	})

	t.Run("fullscreen mode reveals |W column", func(t *testing.T) {
		ActiveFullscreenMode = true
		assert.Equal(t, []string{"Name", "GitSHA", "Age"}, ColumnsForKind("Pod", ""))
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

	cols := ColumnsForKind("Pod", "prod")
	assert.Equal(t, []string{"Name", "X", "Y"}, cols)

	cols = ColumnsForKind("Pod", "dev")
	assert.Equal(t, []string{"Name", "Age"}, cols, "fallback to global view when per-cluster has none")
}
