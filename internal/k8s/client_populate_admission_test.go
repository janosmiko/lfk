package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- populateResourceDetailsExt: MutatingAdmissionPolicy ---

func TestPopulateResourceDetailsExt_MutatingAdmissionPolicy(t *testing.T) {
	tests := []struct {
		name     string
		spec     map[string]any
		wantCols map[string]string
	}{
		{
			name: "single resource rule and two mutations",
			spec: map[string]any{
				"matchConstraints": map[string]any{
					"resourceRules": []any{
						map[string]any{
							"apiGroups":   []any{"apps"},
							"apiVersions": []any{"v1"},
							"resources":   []any{"deployments"},
						},
					},
				},
				"mutations": []any{
					map[string]any{"patchType": "ApplyConfiguration"},
					map[string]any{"patchType": "JSONPatch"},
				},
			},
			wantCols: map[string]string{
				"Match Resources": "apps/deployments",
				"Mutations":       "2",
			},
		},
		{
			name: "core group resource rule renders empty group",
			spec: map[string]any{
				"matchConstraints": map[string]any{
					"resourceRules": []any{
						map[string]any{
							"apiGroups": []any{""},
							"resources": []any{"pods"},
						},
					},
				},
			},
			wantCols: map[string]string{
				"Match Resources": "/pods",
			},
		},
		{
			name: "multiple resource rules join compactly",
			spec: map[string]any{
				"matchConstraints": map[string]any{
					"resourceRules": []any{
						map[string]any{
							"apiGroups": []any{"apps"},
							"resources": []any{"deployments", "statefulsets"},
						},
						map[string]any{
							"apiGroups": []any{""},
							"resources": []any{"pods"},
						},
					},
				},
			},
			wantCols: map[string]string{
				"Match Resources": "apps/deployments,statefulsets; /pods",
			},
		},
		{
			name:     "nil spec produces no columns",
			spec:     nil,
			wantCols: nil,
		},
		{
			name:     "empty spec still reports zero mutations",
			spec:     map[string]any{},
			wantCols: map[string]string{"Mutations": "0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetailsExt(ti, map[string]any{"spec": tt.spec}, "MutatingAdmissionPolicy", nil, tt.spec)

			colMap := columnsToMap(ti.Columns)
			for k, v := range tt.wantCols {
				assert.Equal(t, v, colMap[k], "column %q mismatch", k)
			}
			if tt.spec == nil {
				assert.Empty(t, ti.Columns)
			}
		})
	}
}

// --- populateResourceDetailsExt: MutatingAdmissionPolicyBinding ---

func TestPopulateResourceDetailsExt_MutatingAdmissionPolicyBinding(t *testing.T) {
	tests := []struct {
		name     string
		spec     map[string]any
		wantCols map[string]string
	}{
		{
			name: "policy name with no match resources means all namespaces",
			spec: map[string]any{
				"policyName": "demo-policy",
			},
			wantCols: map[string]string{
				"Policy Name":      "demo-policy",
				"Match Namespaces": "all",
			},
		},
		{
			name: "namespace selector with match labels",
			spec: map[string]any{
				"policyName": "demo-policy",
				"matchResources": map[string]any{
					"namespaceSelector": map[string]any{
						"matchLabels": map[string]any{
							"environment": "prod",
						},
					},
				},
			},
			wantCols: map[string]string{
				"Policy Name":      "demo-policy",
				"Match Namespaces": "environment=prod",
			},
		},
		{
			name: "empty namespace selector means all namespaces",
			spec: map[string]any{
				"policyName": "demo-policy",
				"matchResources": map[string]any{
					"namespaceSelector": map[string]any{},
				},
			},
			wantCols: map[string]string{
				"Policy Name":      "demo-policy",
				"Match Namespaces": "all",
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
			populateResourceDetailsExt(ti, map[string]any{"spec": tt.spec}, "MutatingAdmissionPolicyBinding", nil, tt.spec)

			colMap := columnsToMap(ti.Columns)
			for k, v := range tt.wantCols {
				assert.Equal(t, v, colMap[k], "column %q mismatch", k)
			}
			if tt.wantCols == nil {
				assert.Empty(t, ti.Columns)
			}
		})
	}
}
