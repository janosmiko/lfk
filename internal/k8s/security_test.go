package k8s

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

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
	assert.Equal(t, "(unknown resource)",
		shortResource(security.ResourceRef{}))
	assert.Equal(t, "(unknown resource)",
		shortResource(security.ResourceRef{Kind: "Deployment"}))
}

func TestShortKind(t *testing.T) {
	cases := map[string]string{
		// Workloads.
		"Deployment":  "deploy",
		"StatefulSet": "sts",
		"DaemonSet":   "ds",
		"ReplicaSet":  "rs",
		"CronJob":     "cron",
		"Job":         "job",
		"Pod":         "pod",
		// Services / networking — common targets of policy/misconfig findings.
		"Service":       "svc",
		"Ingress":       "ing",
		"NetworkPolicy": "netpol",
		// Configuration / storage.
		"ConfigMap":             "cm",
		"Secret":                "secret",
		"PersistentVolumeClaim": "pvc",
		"PersistentVolume":      "pv",
		"ServiceAccount":        "sa",
		"Namespace":             "ns",
		// Unknown kinds pass through unchanged.
		"Unknown": "Unknown",
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

func TestGetSecurityFindingsNilManager(t *testing.T) {
	c := &Client{}
	items, err := c.getSecurityFindings(
		context.Background(),
		"kctx", "",
		model.ResourceTypeEntry{Kind: "__security_trivy-operator__"},
	)
	assert.NoError(t, err)
	assert.Nil(t, items)
}

func TestGetSecurityFindingsUnknownKind(t *testing.T) {
	mgr := security.NewManager()
	c := &Client{}
	c.SetSecurityManager(mgr)
	_, err := c.getSecurityFindings(
		context.Background(),
		"kctx", "",
		model.ResourceTypeEntry{Kind: "not-a-security-kind"},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "internal: malformed security resource type")
	assert.NotContains(t, err.Error(), "__security_",
		"the synthetic sentinel must not leak into the error overlay")
	assert.NotContains(t, err.Error(), "not-a-security-kind",
		"the user-supplied kind value must not echo back either")
}

func TestGetSecurityFindingsFiltersBySource(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{
		NameStr: "trivy-operator", Available: true,
		CategoriesVal: []security.Category{security.CategoryVuln},
		Findings: []security.Finding{
			{
				ID: "1", Source: "trivy-operator", Title: "CVE-1",
				Severity: security.SeverityCritical,
				Resource: security.ResourceRef{Namespace: "p", Kind: "Deployment", Name: "api"},
			},
		},
	})
	mgr.Register(&security.FakeSource{
		NameStr: "heuristic", Available: true,
		CategoriesVal: []security.Category{security.CategoryMisconfig},
		Findings: []security.Finding{
			{
				ID: "2", Source: "heuristic", Title: "privileged",
				Severity: security.SeverityCritical,
				Resource: security.ResourceRef{Namespace: "p", Kind: "Pod", Name: "bad"},
			},
		},
	})
	c := &Client{}
	c.SetSecurityManager(mgr)

	items, err := c.getSecurityFindings(
		context.Background(),
		"kctx", "",
		model.ResourceTypeEntry{Kind: "__security_trivy-operator__"},
	)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "__security_finding_group__", items[0].Kind)
	assert.Equal(t, "CVE-1", items[0].Name)
	assert.Equal(t, "trivy-operator", items[0].ColumnValue("__source__"))
}

func TestGetSecurityFindingsSortsBySeverity(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{
		NameStr: "trivy-operator", Available: true,
		Findings: []security.Finding{
			{
				Source: "trivy-operator", Title: "low", Severity: security.SeverityLow,
				Resource: security.ResourceRef{Namespace: "p", Kind: "Pod", Name: "a"},
			},
			{
				Source: "trivy-operator", Title: "crit", Severity: security.SeverityCritical,
				Resource: security.ResourceRef{Namespace: "p", Kind: "Pod", Name: "b"},
			},
			{
				Source: "trivy-operator", Title: "med", Severity: security.SeverityMedium,
				Resource: security.ResourceRef{Namespace: "p", Kind: "Pod", Name: "c"},
			},
			{
				Source: "trivy-operator", Title: "high", Severity: security.SeverityHigh,
				Resource: security.ResourceRef{Namespace: "p", Kind: "Pod", Name: "d"},
			},
		},
	})
	c := &Client{}
	c.SetSecurityManager(mgr)

	items, err := c.getSecurityFindings(
		context.Background(),
		"other-context", "",
		model.ResourceTypeEntry{Kind: "__security_trivy-operator__"},
	)
	require.NoError(t, err)
	require.Len(t, items, 4)
	// Grouped items sorted by severity desc, then title asc.
	assert.Equal(t, "CRIT", items[0].ColumnValue("Severity"))
	assert.Equal(t, "HIGH", items[1].ColumnValue("Severity"))
	assert.Equal(t, "MED", items[2].ColumnValue("Severity"))
	assert.Equal(t, "LOW", items[3].ColumnValue("Severity"))
	// All items are groups.
	for _, it := range items {
		assert.Equal(t, "__security_finding_group__", it.Kind)
	}
}

func TestGetResourcesDispatchesSecurityAPIGroup(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{
		NameStr: "trivy-operator", Available: true,
		Findings: []security.Finding{
			{
				Source: "trivy-operator", Title: "CVE-X",
				Severity: security.SeverityCritical,
				Resource: security.ResourceRef{Namespace: "p", Kind: "Deployment", Name: "api"},
			},
		},
	})
	c := &Client{}
	c.SetSecurityManager(mgr)

	rt := model.ResourceTypeEntry{
		Kind:     "__security_trivy-operator__",
		APIGroup: model.SecurityVirtualAPIGroup,
		Resource: "findings-trivy-operator",
	}
	items, err := c.GetResources(context.Background(), "kctx", "", rt)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "__security_finding_group__", items[0].Kind)
	assert.Equal(t, "CVE-X", items[0].Name)
}

func TestGetSecurityAffectedResources(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{
		NameStr: "heuristic", Available: true,
		Findings: []security.Finding{
			{
				Source: "heuristic", Title: "privileged", Severity: security.SeverityCritical,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "web"},
				Labels:   map[string]string{"check": "privileged"},
			},
			{
				Source: "heuristic", Title: "privileged", Severity: security.SeverityHigh,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "api"},
				Labels:   map[string]string{"check": "privileged"},
			},
			{
				Source: "heuristic", Title: "host_pid", Severity: security.SeverityHigh,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "web"},
				Labels:   map[string]string{"check": "host_pid"},
			},
		},
	})
	c := &Client{}
	c.SetSecurityManager(mgr)
	rt := model.ResourceTypeEntry{Kind: "__security_heuristic__"}

	items, err := c.GetSecurityAffectedResources(
		context.Background(), "kctx", "", rt, "privileged",
	)
	require.NoError(t, err)
	require.Len(t, items, 2)
	for _, it := range items {
		assert.Equal(t, "__security_affected_resource__", it.Kind)
	}
	// Sorted by namespace then name.
	assert.Equal(t, "pod/api", items[0].Name)
	assert.Equal(t, "pod/web", items[1].Name)
	// ResourceKind is no longer a column — the kind is encoded in Name
	// (shortResource) and the full identity in the hidden __resource_key__.
	assert.Empty(t, items[0].ColumnValue("ResourceKind"))
	assert.Equal(t, "ns/Pod/api", items[0].ColumnValue("__resource_key__"))
}

// TestGetSecurityAffectedResourcesPropagatesSourceError guards that a
// source-specific fetch failure surfaces as an error rather than being
// silently mistaken for "no affected resources".
func TestGetSecurityAffectedResourcesPropagatesSourceError(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{
		NameStr: "heuristic", Available: true,
		FetchErr: errors.New("boom"),
	})
	c := &Client{}
	c.SetSecurityManager(mgr)
	rt := model.ResourceTypeEntry{Kind: "__security_heuristic__"}

	items, err := c.GetSecurityAffectedResources(
		context.Background(), "kctx", "", rt, "privileged",
	)
	require.Error(t, err)
	assert.Nil(t, items)
	assert.Contains(t, err.Error(), "source heuristic")
	assert.Contains(t, err.Error(), "boom")
}

// TestSecurityIgnoreSnapshotConcurrentAccess exercises the lock around
// ignoreChecker / showIgnored so `go test -race` catches any future
// regression that bypasses securityIgnoreSnapshot. Without the mutex this
// test reliably trips the race detector; with it the read/write pairs
// remain a consistent observable.
func TestSecurityIgnoreSnapshotConcurrentAccess(t *testing.T) {
	c := &Client{}
	const iterations = 500
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range iterations {
			c.SetIgnoreChecker(nil)
			c.SetShowIgnored(i%2 == 0)
		}
	}()
	for range iterations {
		// Snapshot must observe a consistent (checker, showIgnored) pair
		// even when Set* races with the read.
		_, _ = c.securityIgnoreSnapshot()
	}
	<-done
}
