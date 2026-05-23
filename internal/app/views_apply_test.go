package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestApplyViewColumns_AddsCustomColumn(t *testing.T) {
	v, err := ui.BuildView(&ui.ConfigView{
		Columns: []string{"Name", "IMAGE:.spec.template.spec.containers[0].image"},
	})
	if err != nil {
		t.Fatalf("BuildView: %v", err)
	}
	items := []model.Item{
		{
			Name: "nginx",
			Raw: map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{"image": "nginx:1.27"},
							},
						},
					},
				},
			},
		},
	}
	applyViewColumns(items, v)
	got := ""
	for _, kv := range items[0].Columns {
		if kv.Key == "IMAGE" {
			got = kv.Value
			break
		}
	}
	if got != "nginx:1.27" {
		t.Fatalf("IMAGE = %q, want nginx:1.27", got)
	}
}

func TestApplyViewColumns_NilView(t *testing.T) {
	items := []model.Item{{Name: "x"}}
	applyViewColumns(items, nil)
	if len(items[0].Columns) != 0 {
		t.Fatalf("nil view should not add columns, got %v", items[0].Columns)
	}
}

func TestApplyViewColumns_NilRaw(t *testing.T) {
	v, err := ui.BuildView(&ui.ConfigView{Columns: []string{"X:.foo"}})
	if err != nil {
		t.Fatalf("BuildView: %v", err)
	}
	items := []model.Item{{Name: "x"}}
	applyViewColumns(items, v)
	if len(items[0].Columns) != 0 {
		t.Fatalf("items with nil Raw should not get columns, got %v", items[0].Columns)
	}
}

func TestApplyViewColumns_BuiltinSkipped(t *testing.T) {
	v, err := ui.BuildView(&ui.ConfigView{Columns: []string{"Name", "Age", "Ready"}})
	if err != nil {
		t.Fatalf("BuildView: %v", err)
	}
	items := []model.Item{{Name: "x", Raw: map[string]any{}}}
	applyViewColumns(items, v)
	if len(items[0].Columns) != 0 {
		t.Fatalf("built-in columns should not be appended by applyViewColumns, got %v", items[0].Columns)
	}
}

func TestApplyViewColumns_EmptyResultSuppressed(t *testing.T) {
	v, err := ui.BuildView(&ui.ConfigView{Columns: []string{"X:.does.not.exist"}})
	if err != nil {
		t.Fatalf("BuildView: %v", err)
	}
	items := []model.Item{{Name: "x", Raw: map[string]any{"metadata": map[string]any{"name": "x"}}}}
	applyViewColumns(items, v)
	for _, kv := range items[0].Columns {
		if kv.Key == "X" {
			t.Fatalf("empty JSONPath result should suppress the column, got %q", kv.Value)
		}
	}
}
