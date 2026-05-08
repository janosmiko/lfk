package policyreport

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"

	"github.com/janosmiko/lfk/internal/security"
)

func TestSourceName(t *testing.T) {
	s := New()
	assert.Equal(t, "policy-report", s.Name())
}

func TestSourceCategories(t *testing.T) {
	s := New()
	cats := s.Categories()
	assert.Contains(t, cats, security.CategoryPolicy)
	assert.Contains(t, cats, security.CategoryCompliance)
}

func TestIsAvailableNilClient(t *testing.T) {
	s := New()
	ok, err := s.IsAvailable(context.Background(), "ctx")
	assert.False(t, ok)
	assert.NoError(t, err)
}

func TestFetchNilClient(t *testing.T) {
	s := New()
	findings, err := s.Fetch(context.Background(), "ctx", "")
	assert.Nil(t, findings)
	assert.NoError(t, err)
}

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  security.Severity
	}{
		{"critical", security.SeverityCritical},
		{"CRITICAL", security.SeverityCritical},
		{"high", security.SeverityHigh},
		{"medium", security.SeverityMedium},
		{"low", security.SeverityLow},
		{"info", security.SeverityLow},
		{"unknown", security.SeverityUnknown},
		{"", security.SeverityUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, parseSeverity(tt.input))
		})
	}
}

func TestIsFailingResult(t *testing.T) {
	assert.True(t, isFailingResult("fail"))
	assert.True(t, isFailingResult("error"))
	assert.True(t, isFailingResult("FAIL"))
	assert.False(t, isFailingResult("pass"))
	assert.False(t, isFailingResult("skip"))
	assert.False(t, isFailingResult(""))
}

func TestParsePolicyReport(t *testing.T) {
	pr := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "wgpolicyk8s.io/v1alpha2",
			"kind":       "PolicyReport",
			"metadata": map[string]any{
				"namespace": "prod",
				"name":      "polr-ns-prod",
			},
			"results": []any{
				map[string]any{
					"policy":   "require-labels",
					"rule":     "check-team-label",
					"result":   "fail",
					"severity": "high",
					"message":  "label 'team' is required",
					"category": "Best Practices",
					"resources": []any{
						map[string]any{
							"kind":      "Deployment",
							"name":      "api",
							"namespace": "prod",
						},
					},
				},
				map[string]any{
					"policy":   "disallow-latest",
					"rule":     "validate-image-tag",
					"result":   "pass",
					"severity": "medium",
					"message":  "image tag is not latest",
				},
				map[string]any{
					"policy":   "restrict-host-ports",
					"rule":     "no-host-ports",
					"result":   "fail",
					"severity": "critical",
					"message":  "host ports are not allowed",
					"category": "security",
					"resources": []any{
						map[string]any{
							"kind":      "Pod",
							"name":      "web",
							"namespace": "prod",
						},
					},
				},
			},
		},
	}

	findings := parsePolicyReport(pr)
	require.Len(t, findings, 2, "only failing results should produce findings")

	// First finding: require-labels / check-team-label.
	assert.Equal(t, "policy-report", findings[0].Source)
	assert.Equal(t, security.SeverityHigh, findings[0].Severity)
	assert.Equal(t, "check-team-label", findings[0].Title)
	assert.Equal(t, "Deployment", findings[0].Resource.Kind)
	assert.Equal(t, "api", findings[0].Resource.Name)
	assert.Equal(t, "prod", findings[0].Resource.Namespace)
	assert.Equal(t, "label 'team' is required", findings[0].Summary)
	assert.Equal(t, "require-labels", findings[0].Labels["policy"])
	assert.Equal(t, security.CategoryCompliance, findings[0].Category)

	// Second finding: restrict-host-ports / no-host-ports.
	assert.Equal(t, security.SeverityCritical, findings[1].Severity)
	assert.Equal(t, "no-host-ports", findings[1].Title)
	assert.Equal(t, "Pod", findings[1].Resource.Kind)
	assert.Equal(t, "web", findings[1].Resource.Name)
	assert.Equal(t, security.CategoryPolicy, findings[1].Category)
}

// TestParsePolicyReportWithScope verifies that when per-result resources are
// absent, the report-level scope field is used (Kyverno sets this).
func TestParsePolicyReportWithScope(t *testing.T) {
	pr := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "wgpolicyk8s.io/v1alpha2",
			"kind":       "PolicyReport",
			"metadata": map[string]any{
				"namespace": "prod",
				"name":      "polr-deploy-api",
			},
			"scope": map[string]any{
				"kind":      "Deployment",
				"name":      "api",
				"namespace": "prod",
			},
			"results": []any{
				map[string]any{
					"policy":   "require-labels",
					"rule":     "check-team-label",
					"result":   "fail",
					"severity": "high",
					"message":  "label 'team' is required",
				},
				map[string]any{
					"policy":   "disallow-latest",
					"rule":     "validate-image-tag",
					"result":   "fail",
					"severity": "medium",
					"message":  "image uses latest tag",
				},
			},
		},
	}

	findings := parsePolicyReport(pr)
	require.Len(t, findings, 2)
	// Both findings should reference the scope resource.
	for _, f := range findings {
		assert.Equal(t, "Deployment", f.Resource.Kind)
		assert.Equal(t, "api", f.Resource.Name)
		assert.Equal(t, "prod", f.Resource.Namespace)
	}
}

func TestParsePolicyReportEmptyResults(t *testing.T) {
	pr := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "wgpolicyk8s.io/v1alpha2",
			"kind":       "PolicyReport",
			"metadata":   map[string]any{"namespace": "default", "name": "empty"},
			"results":    []any{},
		},
	}
	assert.Nil(t, parsePolicyReport(pr))
}

func TestFetchWithFakeClient(t *testing.T) {
	pr := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "wgpolicyk8s.io/v1alpha2",
			"kind":       "PolicyReport",
			"metadata": map[string]any{
				"namespace": "default",
				"name":      "polr-1",
			},
			"results": []any{
				map[string]any{
					"policy":   "require-ro-rootfs",
					"rule":     "check-ro-rootfs",
					"result":   "fail",
					"severity": "medium",
					"message":  "root filesystem must be read-only",
					"resources": []any{
						map[string]any{
							"kind": "Pod", "name": "app", "namespace": "default",
						},
					},
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	dc := dynfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			PolicyReportGVR:        "PolicyReportList",
			ClusterPolicyReportGVR: "ClusterPolicyReportList",
		},
		pr,
	)

	s := NewWithDynamic(dc)

	ok, err := s.IsAvailable(context.Background(), "ctx")
	require.NoError(t, err)
	assert.True(t, ok)

	findings, err := s.Fetch(context.Background(), "ctx", "")
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "check-ro-rootfs", findings[0].Title)
	assert.Equal(t, "Pod", findings[0].Resource.Kind)
	assert.Equal(t, "app", findings[0].Resource.Name)
}
