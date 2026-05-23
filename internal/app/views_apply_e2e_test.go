package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestViewsConfigEndToEnd simulates the user's reported config from issue #262:
// a Pod view with a custom JSONPath column extracting a label, plus a
// sort_column. Walks the full pipeline (ConfigViews -> populateResourceDetails
// -> applyViewColumns -> ColumnsForKind) and asserts the column emerges.
func TestViewsConfigEndToEnd_PodWithLabelColumn(t *testing.T) {
	origV := ui.ConfigViews
	origCRC := ui.ConfigResourceColumns
	t.Cleanup(func() {
		ui.ConfigViews = origV
		ui.ConfigResourceColumns = origCRC
	})

	view, err := ui.BuildView(&ui.ConfigView{
		Columns: []string{
			"Name",
			"GitSHA:.metadata.labels.git-sha",
			"Age",
		},
		SortColumn: "Age:asc",
	})
	if err != nil {
		t.Fatalf("BuildView: %v", err)
	}
	ui.ConfigViews = map[string]*ui.View{"pod": view}
	ui.ConfigResourceColumns = nil

	t.Run("view resolves by Kind", func(t *testing.T) {
		v, ok := ui.ResolveView(ui.ResourceRef{Group: "", Version: "v1", Resource: "pods", Kind: "Pod"}, "")
		assert.True(t, ok)
		assert.Equal(t, 3, len(v.Columns))
		assert.Equal(t, "Age", v.SortColumn)
		assert.True(t, v.SortAsc)
	})

	t.Run("populator stamps Raw and applyViewColumns adds GitSHA", func(t *testing.T) {
		obj := map[string]any{
			"metadata": map[string]any{
				"name":            "web-7c4d8b5-abc12",
				"resourceVersion": "5678",
				"labels": map[string]any{
					"git-sha": "deadbeef",
				},
			},
			"spec": map[string]any{
				"containers": []any{map[string]any{"name": "main"}},
			},
			"status": map[string]any{},
		}
		// Simulate what populateResourceDetails does at the top: stamp Raw.
		ti := model.Item{Name: "web-7c4d8b5-abc12", Raw: obj}

		v, ok := ui.ResolveView(ui.ResourceRef{Group: "", Version: "v1", Resource: "pods", Kind: "Pod"}, "")
		assert.True(t, ok)
		items := []model.Item{ti}
		applyViewColumns(items, v)

		got := ""
		for _, kv := range items[0].Columns {
			if kv.Key == "GitSHA" {
				got = kv.Value
				break
			}
		}
		assert.Equal(t, "deadbeef", got)
	})

	t.Run("ColumnsForKind returns the view's column order", func(t *testing.T) {
		cols := ui.ColumnsForKind("Pod", "")
		assert.Equal(t, []string{"Name", "GitSHA", "Age"}, cols)
	})
}
