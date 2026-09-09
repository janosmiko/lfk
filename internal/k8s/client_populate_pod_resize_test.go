package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func TestPopulatePodDetails_PodLevelResources(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]any
		wantCols map[string]string
		absent   []string
	}{
		{
			name: "present",
			obj: map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "app"},
					},
					"resources": map[string]any{
						"requests": map[string]any{"cpu": "500m", "memory": "1Gi"},
						"limits":   map[string]any{"cpu": "1", "memory": "2Gi"},
					},
				},
				"status": map[string]any{},
			},
			wantCols: map[string]string{
				"Pod CPU Req": "500m",
				"Pod CPU Lim": "1",
				"Pod Mem Req": "1Gi",
				"Pod Mem Lim": "2Gi",
			},
		},
		{
			name: "absent",
			obj: map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "app"},
					},
				},
				"status": map[string]any{},
			},
			absent: []string{"Pod CPU Req", "Pod CPU Lim", "Pod Mem Req", "Pod Mem Lim"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{Status: "Running"}
			populateResourceDetails(ti, tt.obj, "Pod")

			colMap := columnsToMap(ti.Columns)
			for k, v := range tt.wantCols {
				assert.Equal(t, v, colMap[k], "column %q mismatch", k)
			}
			for _, k := range tt.absent {
				_, ok := colMap[k]
				assert.False(t, ok, "column %q should be absent", k)
			}
		})
	}
}

func TestPopulatePodDetails_ResizeConditions(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "app"},
			},
		},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{
					"type":    "PodResizePending",
					"status":  "True",
					"reason":  "Infeasible",
					"message": "Node does not have enough capacity",
				},
				map[string]any{
					"type":    "PodResizeInProgress",
					"status":  "True",
					"reason":  "",
					"message": "Resize in progress",
				},
			},
		},
	}

	ti := &model.Item{Status: "Running"}
	populateResourceDetails(ti, obj, "Pod")

	require.Len(t, ti.Conditions, 2)
	assert.Equal(t, "PodResizePending", ti.Conditions[0].Type)
	assert.Equal(t, "Infeasible", ti.Conditions[0].Reason)
	assert.Equal(t, "Node does not have enough capacity", ti.Conditions[0].Message)
	assert.Equal(t, "PodResizeInProgress", ti.Conditions[1].Type)
	assert.Equal(t, "Resize in progress", ti.Conditions[1].Message)
}
