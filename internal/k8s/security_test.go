package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

func TestSeverityToStatus(t *testing.T) {
	cases := []struct {
		sev  security.Severity
		want string
	}{
		{security.SeverityCritical, "Failed"},
		{security.SeverityHigh, "Failed"},
		{security.SeverityMedium, "Progressing"},
		{security.SeverityLow, "Pending"},
		{security.SeverityUnknown, "Unknown"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, severityToStatus(tc.sev))
	}
}

func TestSeverityLabel(t *testing.T) {
	assert.Equal(t, "CRIT", severityLabel(security.SeverityCritical))
	assert.Equal(t, "HIGH", severityLabel(security.SeverityHigh))
	assert.Equal(t, "MED", severityLabel(security.SeverityMedium))
	assert.Equal(t, "LOW", severityLabel(security.SeverityLow))
	assert.Equal(t, "?", severityLabel(security.SeverityUnknown))
}

func TestSeverityOrder(t *testing.T) {
	crit := model.Item{Columns: []model.KeyValue{{Key: "Severity", Value: "CRIT"}}}
	high := model.Item{Columns: []model.KeyValue{{Key: "Severity", Value: "HIGH"}}}
	med := model.Item{Columns: []model.KeyValue{{Key: "Severity", Value: "MED"}}}
	low := model.Item{Columns: []model.KeyValue{{Key: "Severity", Value: "LOW"}}}
	empty := model.Item{}

	assert.Equal(t, 4, severityOrder(crit))
	assert.Equal(t, 3, severityOrder(high))
	assert.Equal(t, 2, severityOrder(med))
	assert.Equal(t, 1, severityOrder(low))
	assert.Equal(t, 0, severityOrder(empty))
}

func TestShortResource(t *testing.T) {
	assert.Equal(t, "deploy/api",
		shortResource(security.ResourceRef{Kind: "Deployment", Name: "api"}))
	assert.Equal(t, "pod/web-abc",
		shortResource(security.ResourceRef{Kind: "Pod", Name: "web-abc"}))
	assert.Equal(t, "(cluster-scoped)",
		shortResource(security.ResourceRef{}))
	assert.Equal(t, "(cluster-scoped)",
		shortResource(security.ResourceRef{Kind: "Deployment"}))
}

func TestShortKind(t *testing.T) {
	cases := map[string]string{
		"Deployment":  "deploy",
		"StatefulSet": "sts",
		"DaemonSet":   "ds",
		"ReplicaSet":  "rs",
		"CronJob":     "cron",
		"Job":         "job",
		"Pod":         "pod",
		"Unknown":     "Unknown",
	}
	for in, want := range cases {
		assert.Equal(t, want, shortKind(in))
	}
}

func TestSourceNameFromKind(t *testing.T) {
	assert.Equal(t, "trivy-operator", sourceNameFromKind("__security_trivy-operator__"))
	assert.Equal(t, "heuristic", sourceNameFromKind("__security_heuristic__"))
	assert.Equal(t, "", sourceNameFromKind("trivy"))
	assert.Equal(t, "", sourceNameFromKind("__security_"))
	assert.Equal(t, "", sourceNameFromKind(""))
}

func TestTitleCase(t *testing.T) {
	assert.Equal(t, "", titleCase(""))
	assert.Equal(t, "Cve", titleCase("cve"))
	assert.Equal(t, "Installed_version", titleCase("installed_version"))
	assert.Equal(t, "ALREADY", titleCase("ALREADY"))
}

func TestFindingToItemFullMapping(t *testing.T) {
	f := security.Finding{
		ID:       "trivy-operator/prod/Deployment/api/CVE-2024-1234",
		Source:   "trivy-operator",
		Category: security.CategoryVuln,
		Severity: security.SeverityCritical,
		Title:    "CVE-2024-1234 in openssl",
		Resource: security.ResourceRef{
			Namespace: "prod",
			Kind:      "Deployment",
			Name:      "api",
			Container: "app",
		},
		Summary:    "openssl 3.0.7 has a remote code execution flaw",
		Details:    "Fixed in 3.0.13",
		References: []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-1234"},
		Labels: map[string]string{
			"cve":     "CVE-2024-1234",
			"package": "openssl",
		},
	}

	it := findingToItem(f)

	assert.Equal(t, "CVE-2024-1234 in openssl", it.Name)
	assert.Equal(t, "__security_finding__", it.Kind)
	assert.Equal(t, "prod", it.Namespace)
	assert.Equal(t, "Failed", it.Status)
	assert.Equal(t, "trivy-operator/prod/Deployment/api/CVE-2024-1234", it.Extra)

	assert.Equal(t, "CRIT", it.ColumnValue("Severity"))
	assert.Equal(t, "deploy/api", it.ColumnValue("Resource"))
	assert.Equal(t, "CVE-2024-1234 in openssl", it.ColumnValue("Title"))
	assert.Equal(t, "vuln", it.ColumnValue("Category"))
	assert.Equal(t, "Deployment", it.ColumnValue("ResourceKind"))
	assert.Equal(t, "trivy-operator", it.ColumnValue("Source"))
	assert.Equal(t, "openssl 3.0.7 has a remote code execution flaw\n\nFixed in 3.0.13",
		it.ColumnValue("Description"))
	assert.Equal(t, "https://nvd.nist.gov/vuln/detail/CVE-2024-1234",
		it.ColumnValue("References"))
	assert.Equal(t, "CVE-2024-1234", it.ColumnValue("Cve"))
	assert.Equal(t, "openssl", it.ColumnValue("Package"))
}

func TestFindingToItemMinimal(t *testing.T) {
	f := security.Finding{
		Severity: security.SeverityLow,
		Title:    "latest tag",
		Resource: security.ResourceRef{Namespace: "p", Kind: "Pod", Name: "x"},
	}
	it := findingToItem(f)
	assert.Equal(t, "latest tag", it.Name)
	assert.Equal(t, "LOW", it.ColumnValue("Severity"))
	assert.Equal(t, "pod/x", it.ColumnValue("Resource"))
	assert.Equal(t, "Pending", it.Status)
	assert.Equal(t, "", it.ColumnValue("Description"))
	assert.Equal(t, "", it.ColumnValue("References"))
}
