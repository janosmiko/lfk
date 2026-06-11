package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestObjectTreeRowsAt(t *testing.T) {
	root := map[string]any{
		"spec": map[string]any{
			"replicas": int64(3),
			"containers": []any{
				map[string]any{"name": "app", "image": "nginx"},
				map[string]any{"name": "sidecar"},
			},
		},
		"kind": "Deployment",
	}

	tests := []struct {
		name  string
		segs  []string
		limit int
		want  []string // "depth:joined-segs" per row, pre-order
	}{
		{
			name: "full object",
			want: []string{
				"0:kind",
				"0:spec",
				"1:spec/containers",
				"2:spec/containers/[0]",
				"3:spec/containers/[0]/image",
				"3:spec/containers/[0]/name",
				"2:spec/containers/[1]",
				"3:spec/containers/[1]/name",
				"1:spec/replicas",
			},
		},
		{
			name: "subtree at spec.containers",
			segs: []string{"spec", "containers"},
			want: []string{
				"0:[0]",
				"1:[0]/image",
				"1:[0]/name",
				"0:[1]",
				"1:[1]/name",
			},
		},
		{
			name:  "limit caps rows",
			limit: 3,
			want:  []string{"0:kind", "0:spec", "1:spec/containers"},
		},
		{
			name: "scalar path yields nil",
			segs: []string{"kind"},
			want: nil,
		},
		{
			name: "missing path yields nil",
			segs: []string{"nope"},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := ObjectTreeRowsAt(root, tt.segs, tt.limit)
			got := make([]string, 0, len(rows))
			for _, r := range rows {
				got = append(got, formatRow(r))
			}
			if len(got) == 0 {
				got = nil
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rows mismatch\n got: %v\nwant: %v", got, tt.want)
			}
		})
	}
}

func formatRow(r ObjectTreeRow) string {
	return strings.Join([]string{itoaDepth(r.Depth), strings.Join(r.Segs, "/")}, ":")
}

func itoaDepth(d int) string {
	return string(rune('0' + d))
}

func TestObjectTreeRowsAtFieldData(t *testing.T) {
	root := map[string]any{
		"spec": map[string]any{"paused": true},
	}
	rows := ObjectTreeRowsAt(root, nil, 0)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Field.Key != "spec" || !rows[0].Field.HasChildren {
		t.Errorf("spec row wrong: %+v", rows[0].Field)
	}
	if rows[1].Field.Key != "paused" || rows[1].Field.Preview != "true" {
		t.Errorf("paused row wrong: %+v", rows[1].Field)
	}
	if rows[1].Depth != 1 {
		t.Errorf("paused depth = %d, want 1", rows[1].Depth)
	}
}

func TestObjectTreeRowsAtArrayElementLabels(t *testing.T) {
	root := map[string]any{
		"items": []any{map[string]any{"name": "web"}},
	}
	rows := ObjectTreeRowsAt(root, []string{"items"}, 0)
	if len(rows) == 0 {
		t.Fatal("no rows")
	}
	if rows[0].Field.Key != "[0]" || rows[0].Field.Preview != "web" {
		t.Errorf("array element row = %+v, want [0]/web preview", rows[0].Field)
	}
}
