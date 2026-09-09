package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- populateResourceDetailsExt: ResourceClaim ---

func TestPopulateResourceDetailsExt_ResourceClaim(t *testing.T) {
	tests := []struct {
		name     string
		spec     map[string]any
		status   map[string]any
		wantCols map[string]string
	}{
		{
			name: "allocated single device with node selector and ready condition",
			status: map[string]any{
				"allocation": map[string]any{
					"devices": map[string]any{
						"results": []any{
							map[string]any{"driver": "gpu.example.com", "pool": "pool-a", "device": "gpu-0"},
						},
					},
					"nodeSelector": map[string]any{
						"nodeSelectorTerms": []any{
							map[string]any{
								"matchFields": []any{
									map[string]any{"key": "metadata.name", "operator": "In", "values": []any{"node-1"}},
								},
							},
						},
					},
				},
				"conditions": []any{
					map[string]any{"type": "Ready", "status": "True"},
				},
			},
			wantCols: map[string]string{
				"Driver": "gpu.example.com",
				"Pool":   "pool-a",
				"Device": "gpu-0",
				"Node":   "node-1",
				"Ready":  "True",
			},
		},
		{
			name: "allocated multiple devices join with comma",
			status: map[string]any{
				"allocation": map[string]any{
					"devices": map[string]any{
						"results": []any{
							map[string]any{"driver": "gpu.example.com", "pool": "pool-a", "device": "gpu-0"},
							map[string]any{"driver": "gpu.example.com", "pool": "pool-a", "device": "gpu-1"},
						},
					},
				},
			},
			wantCols: map[string]string{
				"Driver": "gpu.example.com, gpu.example.com",
				"Pool":   "pool-a, pool-a",
				"Device": "gpu-0, gpu-1",
			},
		},
		{
			name:     "unallocated claim leaves allocation columns blank",
			status:   map[string]any{},
			wantCols: nil,
		},
		{
			name:     "nil status produces no columns",
			status:   nil,
			wantCols: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetailsExt(ti, map[string]any{"spec": tt.spec, "status": tt.status}, "ResourceClaim", tt.status, tt.spec)

			colMap := columnsToMap(ti.Columns)
			if tt.wantCols == nil {
				assert.Empty(t, colMap)
			} else {
				assert.Equal(t, tt.wantCols, colMap)
			}
		})
	}
}

// --- populateResourceDetailsExt: ResourceClaimTemplate ---

func TestPopulateResourceDetailsExt_ResourceClaimTemplate(t *testing.T) {
	tests := []struct {
		name     string
		spec     map[string]any
		wantCols map[string]string
	}{
		{
			name: "two device requests",
			spec: map[string]any{
				"spec": map[string]any{
					"devices": map[string]any{
						"requests": []any{
							map[string]any{"name": "gpu"},
							map[string]any{"name": "nic"},
						},
					},
				},
			},
			wantCols: map[string]string{"Devices": "2"},
		},
		{
			name: "no device requests",
			spec: map[string]any{
				"spec": map[string]any{},
			},
			wantCols: map[string]string{"Devices": "0"},
		},
		{
			name:     "nil spec produces no columns",
			spec:     nil,
			wantCols: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetailsExt(ti, map[string]any{"spec": tt.spec}, "ResourceClaimTemplate", nil, tt.spec)

			colMap := columnsToMap(ti.Columns)
			if tt.wantCols == nil {
				assert.Empty(t, colMap)
			} else {
				assert.Equal(t, tt.wantCols, colMap)
			}
		})
	}
}

// --- populateResourceDetailsExt: ResourceSlice ---

func TestPopulateResourceDetailsExt_ResourceSlice(t *testing.T) {
	tests := []struct {
		name     string
		spec     map[string]any
		wantCols map[string]string
	}{
		{
			name: "node-scoped slice",
			spec: map[string]any{
				"driver":   "gpu.example.com",
				"nodeName": "node-1",
				"devices":  []any{map[string]any{"name": "gpu-0"}, map[string]any{"name": "gpu-1"}},
			},
			wantCols: map[string]string{
				"Driver":       "gpu.example.com",
				"Node or Pool": "node-1",
				"Devices":      "2",
			},
		},
		{
			name: "pool-scoped slice falls back to pool name",
			spec: map[string]any{
				"driver": "gpu.example.com",
				"pool":   map[string]any{"name": "pool-a"},
				"devices": []any{
					map[string]any{"name": "gpu-0"},
				},
			},
			wantCols: map[string]string{
				"Driver":       "gpu.example.com",
				"Node or Pool": "pool-a",
				"Devices":      "1",
			},
		},
		{
			name:     "nil spec produces no columns",
			spec:     nil,
			wantCols: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetailsExt(ti, map[string]any{"spec": tt.spec}, "ResourceSlice", nil, tt.spec)

			colMap := columnsToMap(ti.Columns)
			if tt.wantCols == nil {
				assert.Empty(t, colMap)
			} else {
				assert.Equal(t, tt.wantCols, colMap)
			}
		})
	}
}

// --- populateResourceDetailsExt: DeviceClass ---

func TestPopulateResourceDetailsExt_DeviceClass(t *testing.T) {
	tests := []struct {
		name     string
		spec     map[string]any
		wantCols map[string]string
	}{
		{
			name: "two selectors",
			spec: map[string]any{
				"selectors": []any{
					map[string]any{"cel": map[string]any{"expression": "true"}},
					map[string]any{"cel": map[string]any{"expression": "device.driver == \"gpu.example.com\""}},
				},
			},
			wantCols: map[string]string{"Selectors": "2"},
		},
		{
			name:     "no selectors",
			spec:     map[string]any{},
			wantCols: map[string]string{"Selectors": "0"},
		},
		{
			name:     "nil spec produces no columns",
			spec:     nil,
			wantCols: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetailsExt(ti, map[string]any{"spec": tt.spec}, "DeviceClass", nil, tt.spec)

			colMap := columnsToMap(ti.Columns)
			if tt.wantCols == nil {
				assert.Empty(t, colMap)
			} else {
				assert.Equal(t, tt.wantCols, colMap)
			}
		})
	}
}
