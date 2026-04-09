package trivyop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/janosmiko/lfk/internal/security"
)

func newFakeDyn(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			VulnerabilityReportGVR: "VulnerabilityReportList",
			ConfigAuditReportGVR:   "ConfigAuditReportList",
		},
		objects...,
	)
}

func TestSourceMetadata(t *testing.T) {
	s := New()
	assert.Equal(t, "trivy-operator", s.Name())
	assert.Contains(t, s.Categories(), security.CategoryVuln)
	assert.Contains(t, s.Categories(), security.CategoryMisconfig)
}

func TestIsAvailableNilClient(t *testing.T) {
	s := New()
	ok, err := s.IsAvailable(context.Background(), "")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestIsAvailableCRDReachable(t *testing.T) {
	client := newFakeDyn()
	s := NewWithDynamic(client)
	ok, err := s.IsAvailable(context.Background(), "")
	require.NoError(t, err)
	assert.True(t, ok, "empty list still means the CRD is served")
}

func TestParseVulnerabilityReport(t *testing.T) {
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aquasecurity.github.io/v1alpha1",
			"kind":       "VulnerabilityReport",
			"metadata": map[string]interface{}{
				"namespace": "prod",
				"name":      "deployment-api-container-app",
				"labels": map[string]interface{}{
					"trivy-operator.resource.kind":  "Deployment",
					"trivy-operator.resource.name":  "api",
					"trivy-operator.container.name": "app",
				},
			},
			"report": map[string]interface{}{
				"vulnerabilities": []interface{}{
					map[string]interface{}{
						"vulnerabilityID":  "CVE-2024-0001",
						"severity":         "CRITICAL",
						"resource":         "openssl",
						"installedVersion": "3.0.7",
						"fixedVersion":     "3.0.13",
						"title":            "Remote code execution in openssl",
						"description":      "A flaw was found...",
						"primaryLink":      "https://nvd.nist.gov/vuln/detail/CVE-2024-0001",
					},
					map[string]interface{}{
						"vulnerabilityID":  "CVE-2024-0002",
						"severity":         "HIGH",
						"resource":         "glibc",
						"installedVersion": "2.36",
					},
				},
			},
		},
	}

	findings := parseVulnerabilityReport(u)
	require.Len(t, findings, 2)

	crit := findings[0]
	assert.Equal(t, security.CategoryVuln, crit.Category)
	assert.Equal(t, security.SeverityCritical, crit.Severity)
	assert.Equal(t, "trivy-operator", crit.Source)
	assert.Equal(t, "Deployment", crit.Resource.Kind)
	assert.Equal(t, "api", crit.Resource.Name)
	assert.Equal(t, "app", crit.Resource.Container)
	assert.Contains(t, crit.ID, "CVE-2024-0001")
	assert.Contains(t, crit.References, "https://nvd.nist.gov/vuln/detail/CVE-2024-0001")
	assert.Equal(t, "openssl", crit.Labels["package"])
	assert.Equal(t, "3.0.13", crit.Labels["fixed_version"])

	assert.Equal(t, security.SeverityHigh, findings[1].Severity)
}

func TestParseVulnerabilityReportEmpty(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"report": map[string]interface{}{"vulnerabilities": []interface{}{}},
	}}
	findings := parseVulnerabilityReport(u)
	assert.Empty(t, findings)
}

func TestParseVulnerabilityReportMalformed(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{"weird": 123}}
	findings := parseVulnerabilityReport(u)
	assert.Empty(t, findings, "malformed report must not panic and must return empty")
}

func TestParseConfigAuditReport(t *testing.T) {
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aquasecurity.github.io/v1alpha1",
			"kind":       "ConfigAuditReport",
			"metadata": map[string]interface{}{
				"namespace": "prod",
				"name":      "daemonset-agent",
				"labels": map[string]interface{}{
					"trivy-operator.resource.kind": "DaemonSet",
					"trivy-operator.resource.name": "agent",
				},
			},
			"report": map[string]interface{}{
				"checks": []interface{}{
					map[string]interface{}{
						"checkID":     "KSV001",
						"severity":    "HIGH",
						"title":       "Process can elevate its own privileges",
						"description": "A program inside the container can elevate its own privileges...",
						"success":     false,
					},
					map[string]interface{}{
						"checkID":  "KSV002",
						"severity": "LOW",
						"title":    "Default AppArmor profile not set",
						"success":  true, // passing check, must be skipped
					},
				},
			},
		},
	}

	findings := parseConfigAuditReport(u)
	require.Len(t, findings, 1, "only failing checks should produce findings")
	f := findings[0]
	assert.Equal(t, security.CategoryMisconfig, f.Category)
	assert.Equal(t, security.SeverityHigh, f.Severity)
	assert.Equal(t, "DaemonSet", f.Resource.Kind)
	assert.Equal(t, "KSV001", f.Labels["check_id"])
	assert.Contains(t, f.ID, "KSV001")
}

func TestFetchAggregatesBothCRDs(t *testing.T) {
	vuln := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aquasecurity.github.io/v1alpha1",
			"kind":       "VulnerabilityReport",
			"metadata": map[string]interface{}{
				"namespace": "prod", "name": "v1",
				"labels": map[string]interface{}{
					"trivy-operator.resource.kind":  "Deployment",
					"trivy-operator.resource.name":  "api",
					"trivy-operator.container.name": "app",
				},
			},
			"report": map[string]interface{}{
				"vulnerabilities": []interface{}{
					map[string]interface{}{
						"vulnerabilityID": "CVE-1", "severity": "CRITICAL", "resource": "openssl",
					},
				},
			},
		},
	}
	audit := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aquasecurity.github.io/v1alpha1",
			"kind":       "ConfigAuditReport",
			"metadata": map[string]interface{}{
				"namespace": "prod", "name": "a1",
				"labels": map[string]interface{}{
					"trivy-operator.resource.kind": "Deployment",
					"trivy-operator.resource.name": "api",
				},
			},
			"report": map[string]interface{}{
				"checks": []interface{}{
					map[string]interface{}{
						"checkID": "KSV001", "severity": "HIGH", "title": "Priv esc", "success": false,
					},
				},
			},
		},
	}

	client := newFakeDyn(vuln, audit)
	s := NewWithDynamic(client)
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	assert.Len(t, findings, 2)

	var gotVuln, gotAudit bool
	for _, f := range findings {
		switch f.Category {
		case security.CategoryVuln:
			gotVuln = true
		case security.CategoryMisconfig:
			gotAudit = true
		}
	}
	assert.True(t, gotVuln)
	assert.True(t, gotAudit)
}

func TestFetchNamespaceFilter(t *testing.T) {
	make := func(ns, name string) *unstructured.Unstructured {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "aquasecurity.github.io/v1alpha1",
				"kind":       "VulnerabilityReport",
				"metadata": map[string]interface{}{
					"namespace": ns, "name": name,
					"labels": map[string]interface{}{
						"trivy-operator.resource.kind": "Deployment",
						"trivy-operator.resource.name": name,
					},
				},
				"report": map[string]interface{}{
					"vulnerabilities": []interface{}{
						map[string]interface{}{
							"vulnerabilityID": "CVE-X", "severity": "HIGH",
						},
					},
				},
			},
		}
	}

	client := newFakeDyn(make("prod", "a"), make("staging", "b"))
	s := NewWithDynamic(client)
	findings, err := s.Fetch(context.Background(), "", "prod")
	require.NoError(t, err)
	assert.Len(t, findings, 1)
	assert.Equal(t, "prod", findings[0].Resource.Namespace)
}

func TestFetchNilClient(t *testing.T) {
	s := New()
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	assert.Empty(t, findings)
}
