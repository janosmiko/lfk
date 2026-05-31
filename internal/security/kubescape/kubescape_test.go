package kubescape

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/security"
)

func TestParseSeverityBuckets(t *testing.T) {
	cases := []struct {
		score float64
		want  security.Severity
	}{
		{10, security.SeverityCritical},
		{9, security.SeverityCritical},
		{8, security.SeverityHigh},
		{7, security.SeverityHigh},
		{6, security.SeverityMedium},
		{4, security.SeverityMedium},
		{3, security.SeverityLow},
		{0.1, security.SeverityLow},
		{0, security.SeverityUnknown},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, parseSeverity(c.score), "score=%v", c.score)
	}
}

func TestTargetRefFromName(t *testing.T) {
	ref := targetRefFromName("deployment-api", "prod")
	assert.Equal(t, "Deployment", ref.Kind)
	assert.Equal(t, "api", ref.Name)
	assert.Equal(t, "prod", ref.Namespace)

	// Unknown leading kind falls through.
	ref = targetRefFromName("scrubbed-name", "ns")
	assert.Empty(t, ref.Kind, "unrecognised prefix must not fabricate a Kind")
	assert.Equal(t, "scrubbed-name", ref.Name)
}

func TestParseWorkloadConfigurationScanFailedControl(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "deployment-api", "namespace": "prod"},
		"spec": map[string]any{
			"controls": map[string]any{
				"C-0001": map[string]any{
					"controlID": "C-0001",
					"name":      "Forbidden Container Registries",
					"status":    map[string]any{"status": "failed", "info": "use approved registry"},
					"severity":  map[string]any{"scoreFactor": float64(8)},
				},
				"C-0002": map[string]any{
					"controlID": "C-0002",
					"name":      "Allowed Container Registries",
					"status":    map[string]any{"status": "passed"},
					"severity":  map[string]any{"scoreFactor": float64(8)},
				},
			},
		},
	}}
	u.SetName("deployment-api")
	u.SetNamespace("prod")

	findings := parseWorkloadConfigurationScan(u)
	require.Len(t, findings, 1, "passed controls must be filtered out")

	f := findings[0]
	assert.Equal(t, "kubescape", f.Source)
	assert.Equal(t, security.CategoryMisconfig, f.Category)
	assert.Equal(t, security.SeverityHigh, f.Severity)
	assert.Equal(t, "Forbidden Container Registries", f.Title)
	assert.Equal(t, "Deployment", f.Resource.Kind)
	assert.Equal(t, "api", f.Resource.Name)
	assert.Equal(t, "prod", f.Resource.Namespace)
	assert.Equal(t, "use approved registry", f.Summary)
	assert.Equal(t, "C-0001", f.Labels["control_id"])
	assert.Equal(t, "failed", f.Labels["status"])
}

func TestParseWorkloadConfigurationScanEmpty(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "pod-clean", "namespace": "ns"},
		"spec":     map[string]any{"controls": map[string]any{}},
	}}
	u.SetName("pod-clean")
	u.SetNamespace("ns")
	assert.Empty(t, parseWorkloadConfigurationScan(u))
}
