package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/security"
)

// resourceFindingsClient builds a Client with two sources whose findings
// cover the selected pod, its owner Deployment, and an unrelated pod, so
// tests can assert the per-resource filter keeps only the first two.
func resourceFindingsClient() (*Client, *security.Manager) {
	mgr := security.NewManager()
	mgr.SetRefreshTTL(time.Hour)
	mgr.Register(&security.FakeSource{
		NameStr: "heuristic", Available: true,
		Findings: []security.Finding{
			{
				Source: "heuristic", Title: "Privileged container", Severity: security.SeverityHigh,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "web"},
				Labels:   map[string]string{"check": "privileged"},
			},
			{
				Source: "heuristic", Title: "Privileged container", Severity: security.SeverityHigh,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "other"},
				Labels:   map[string]string{"check": "privileged"},
			},
			{
				Source: "heuristic", Title: "Runs as root", Severity: security.SeverityMedium,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "other"},
				Labels:   map[string]string{"check": "run-as-root"},
			},
		},
	})
	mgr.Register(&security.FakeSource{
		NameStr: "trivy-operator", Available: true,
		Findings: []security.Finding{
			{
				Source: "trivy-operator", Title: "CVE-2024-1 in openssl", Severity: security.SeverityCritical,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Deployment", Name: "web-deploy"},
				Labels:   map[string]string{"cve": "CVE-2024-1"},
			},
		},
	})
	c := &Client{}
	c.SetSecurityManager(mgr)
	return c, mgr
}

// webRefs is the pod's own ref plus its owner's ref — the same set the SEC
// badge aggregates over.
func webRefs() []security.ResourceRef {
	return []security.ResourceRef{
		{Namespace: "ns", Kind: "Pod", Name: "web"},
		{Namespace: "ns", Kind: "Deployment", Name: "web-deploy"},
	}
}

func TestGetSecurityFindingsForResource_FiltersByRefsAcrossSources(t *testing.T) {
	c, _ := resourceFindingsClient()

	items, err := c.GetSecurityFindingsForResource(context.Background(), "kctx", "", webRefs())
	require.NoError(t, err)
	require.Len(t, items, 2, "only groups touching pod/web or deploy/web-deploy survive")

	// Sorted by severity desc: the trivy CRIT first, the heuristic HIGH second.
	assert.Equal(t, "CVE-2024-1", items[0].Name)
	assert.Equal(t, "__security_finding_group__", items[0].Kind)
	assert.Equal(t, "CRIT", items[0].ColumnValue("Severity"))
	assert.Equal(t, "trivy-operator", items[0].ColumnValue("Source"),
		"cross-source list shows a visible Source column")
	assert.Equal(t, "trivy-operator", items[0].ColumnValue("__source__"),
		"hidden source column still feeds drill-in and the action menu")

	assert.Equal(t, "privileged", items[1].Name)
	assert.Equal(t, "heuristic", items[1].ColumnValue("Source"))
	// The group is filtered to the requested refs: pod/other must not count.
	assert.Equal(t, "1", items[1].ColumnValue("Affected"),
		"affected count reflects only the requested refs")
}

func TestGetSecurityFindingsForResource_NoMatchingRefs(t *testing.T) {
	c, _ := resourceFindingsClient()

	items, err := c.GetSecurityFindingsForResource(context.Background(), "kctx", "",
		[]security.ResourceRef{{Namespace: "ns", Kind: "Pod", Name: "absent"}})
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestGetSecurityFindingsForResource_AppliesIgnorePolicy(t *testing.T) {
	c, _ := resourceFindingsClient()
	c.SetIgnoreChecker(stubIgnoreChecker{groups: map[string]bool{"privileged": true}})

	items, err := c.GetSecurityFindingsForResource(context.Background(), "kctx", "", webRefs())
	require.NoError(t, err)
	require.Len(t, items, 1, "ignored group is hidden")
	assert.Equal(t, "CVE-2024-1", items[0].Name)

	// Show-ignored mode reveals the group tagged __ignored__.
	c.SetShowIgnored(true)
	items, err = c.GetSecurityFindingsForResource(context.Background(), "kctx", "", webRefs())
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "true", items[1].ColumnValue("__ignored__"))
}

func TestGetSecurityFindingsForResourceCached_ColdThenWarm(t *testing.T) {
	c, mgr := resourceFindingsClient()

	// Cold cache: not cached, and no scan may be triggered.
	items, ok, err := c.GetSecurityFindingsForResourceCached("kctx", "", webRefs())
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, items)

	// Warm the shared scan, then the cached getter serves synchronously.
	_, err = mgr.FetchAll(context.Background(), "kctx", "")
	require.NoError(t, err)
	items, ok, err = c.GetSecurityFindingsForResourceCached("kctx", "", webRefs())
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, items, 2)
}

func TestGetSecurityFindingsForResource_NilManager(t *testing.T) {
	c := &Client{}
	items, err := c.GetSecurityFindingsForResource(context.Background(), "kctx", "", webRefs())
	require.NoError(t, err)
	assert.Nil(t, items)

	items, ok, err := c.GetSecurityFindingsForResourceCached("kctx", "", webRefs())
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, items)
}
