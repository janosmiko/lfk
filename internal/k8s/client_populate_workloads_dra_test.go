package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestPopulatePodExtraColumns_ResourceClaims(t *testing.T) {
	tests := []struct {
		name     string
		status   map[string]any
		spec     map[string]any
		wantCols map[string]string
	}{
		{
			name: "direct claim reference",
			spec: map[string]any{
				"resourceClaims": []any{
					map[string]any{"name": "gpu", "resourceClaimName": "shared-gpu-claim"},
				},
			},
			wantCols: map[string]string{"Resource Claims": "shared-gpu-claim"},
		},
		{
			name: "template-backed claim resolves through status",
			spec: map[string]any{
				"resourceClaims": []any{
					map[string]any{"name": "gpu", "resourceClaimTemplateName": "gpu-template"},
				},
			},
			status: map[string]any{
				"resourceClaimStatuses": []any{
					map[string]any{"name": "gpu", "resourceClaimName": "pod-abc-gpu"},
				},
			},
			wantCols: map[string]string{"Resource Claims": "pod-abc-gpu"},
		},
		{
			name: "template-backed claim falls back to template name when unresolved",
			spec: map[string]any{
				"resourceClaims": []any{
					map[string]any{"name": "gpu", "resourceClaimTemplateName": "gpu-template"},
				},
			},
			wantCols: map[string]string{"Resource Claims": "gpu-template"},
		},
		{
			name: "multiple claims join with comma",
			spec: map[string]any{
				"resourceClaims": []any{
					map[string]any{"name": "gpu", "resourceClaimName": "shared-gpu-claim"},
					map[string]any{"name": "nic", "resourceClaimTemplateName": "nic-template"},
				},
			},
			status: map[string]any{
				"resourceClaimStatuses": []any{
					map[string]any{"name": "nic", "resourceClaimName": "pod-abc-nic"},
				},
			},
			wantCols: map[string]string{"Resource Claims": "shared-gpu-claim, pod-abc-nic"},
		},
		{
			name:     "no claims omits the column",
			spec:     map[string]any{},
			wantCols: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populatePodExtraColumns(ti, nil, tt.status, tt.spec)

			colMap := columnsToMap(ti.Columns)
			assert.Equal(t, tt.wantCols["Resource Claims"], colMap["Resource Claims"])
			if tt.wantCols["Resource Claims"] == "" {
				_, ok := colMap["Resource Claims"]
				assert.False(t, ok, "Resource Claims column must be absent")
			}
		})
	}
}

func TestPopulatePodResourceClaims_HiddenClaimColumns(t *testing.T) {
	tests := []struct {
		name       string
		status     map[string]any
		spec       map[string]any
		wantClaims map[string]string
	}{
		{
			name: "direct claim gets claim:0",
			spec: map[string]any{
				"resourceClaims": []any{
					map[string]any{"name": "gpu", "resourceClaimName": "shared-gpu-claim"},
				},
			},
			wantClaims: map[string]string{"claim:0": "shared-gpu-claim"},
		},
		{
			name: "template-backed claim resolves the object name through status",
			spec: map[string]any{
				"resourceClaims": []any{
					map[string]any{"name": "gpu", "resourceClaimTemplateName": "gpu-template"},
				},
			},
			status: map[string]any{
				"resourceClaimStatuses": []any{
					map[string]any{"name": "gpu", "resourceClaimName": "pod-abc-gpu"},
				},
			},
			wantClaims: map[string]string{"claim:0": "pod-abc-gpu"},
		},
		{
			name: "multiple claims are indexed sequentially from 0",
			spec: map[string]any{
				"resourceClaims": []any{
					map[string]any{"name": "gpu", "resourceClaimName": "shared-gpu-claim"},
					map[string]any{"name": "nic", "resourceClaimTemplateName": "nic-template"},
				},
			},
			status: map[string]any{
				"resourceClaimStatuses": []any{
					map[string]any{"name": "nic", "resourceClaimName": "pod-abc-nic"},
				},
			},
			wantClaims: map[string]string{"claim:0": "shared-gpu-claim", "claim:1": "pod-abc-nic"},
		},
		{
			name:       "no claims produces no hidden columns",
			spec:       map[string]any{},
			wantClaims: map[string]string{},
		},
		{
			name: "template-backed claim produces no hidden column when unresolved",
			spec: map[string]any{
				"resourceClaims": []any{
					map[string]any{"name": "gpu", "resourceClaimTemplateName": "gpu-template"},
				},
			},
			wantClaims: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populatePodExtraColumns(ti, nil, tt.status, tt.spec)

			colMap := columnsToMap(ti.Columns)
			for key, want := range tt.wantClaims {
				assert.Equal(t, want, colMap[key], "column %q", key)
			}
			if len(tt.wantClaims) == 0 {
				for _, kv := range ti.Columns {
					assert.NotContains(t, kv.Key, "claim:", "no claim: column expected")
				}
			}
		})
	}
}
