package k8s

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
)

func TestPopulateDeploymentDetails_REVColumn(t *testing.T) {
	tests := []struct {
		name            string
		resourceVersion string
		want            string // "" means no column emitted
	}{
		{name: "small value", resourceVersion: "12345", want: "12345"},
		{name: "zero", resourceVersion: "0", want: "0"},
		{name: "large value", resourceVersion: "1048576", want: "1048576"},
		{name: "empty", resourceVersion: "", want: ""},
		{name: "non-numeric", resourceVersion: "abc", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			obj := map[string]any{
				"metadata": map[string]any{"resourceVersion": tt.resourceVersion},
				"spec":     map[string]any{"replicas": float64(1)},
				"status":   map[string]any{"readyReplicas": float64(1)},
			}
			populateResourceDetails(ti, obj, "Deployment")
			got := ""
			for _, kv := range ti.Columns {
				if kv.Key == "REV" {
					got = kv.Value
					break
				}
			}
			if got != tt.want {
				t.Fatalf("REV column = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPopulateStatefulSetDetails_REVColumn(t *testing.T) {
	ti := &model.Item{}
	obj := map[string]any{
		"metadata": map[string]any{"resourceVersion": "65535"},
		"spec":     map[string]any{"replicas": float64(1)},
		"status":   map[string]any{"readyReplicas": float64(1)},
	}
	populateResourceDetails(ti, obj, "StatefulSet")
	for _, kv := range ti.Columns {
		if kv.Key == "REV" {
			if kv.Value != "65535" {
				t.Fatalf("REV = %q, want 65535", kv.Value)
			}
			return
		}
	}
	t.Fatal("REV column not found on StatefulSet")
}
