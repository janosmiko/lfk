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
