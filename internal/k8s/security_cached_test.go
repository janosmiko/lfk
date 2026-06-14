package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

func cachedTestClient() (*Client, *security.FakeSource, *security.Manager) {
	mgr := security.NewManager()
	mgr.SetRefreshTTL(time.Hour)
	src := &security.FakeSource{
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
		},
	}
	mgr.Register(src)
	c := &Client{}
	c.SetSecurityManager(mgr)
	return c, src, mgr
}

func TestGetSecurityFindingsCached_ColdThenWarm(t *testing.T) {
	c, src, mgr := cachedTestClient()
	rt := model.ResourceTypeEntry{Kind: "__security_heuristic__"}

	// Cold cache: not cached, and the cached getter must NOT scan.
	items, ok, err := c.GetSecurityFindingsCached("kctx", "", rt)
	require.NoError(t, err)
	assert.False(t, ok, "cold cache reports not-cached")
	assert.Nil(t, items)
	assert.Equal(t, int32(0), src.FetchCalls.Load(), "cached getter must never trigger a scan")

	// Warm the shared scan.
	_, err = mgr.FetchAll(t.Context(), "kctx", "")
	require.NoError(t, err)
	require.Equal(t, int32(1), src.FetchCalls.Load())

	// Warm cache: returns the grouped findings with NO additional scan.
	items, ok, err = c.GetSecurityFindingsCached("kctx", "", rt)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, items, 1, "two findings collapse into one 'privileged' group")
	assert.Equal(t, "privileged", items[0].Name)
	assert.Equal(t, int32(1), src.FetchCalls.Load(), "warm serve must not re-scan")
}

func TestGetSecurityAffectedResourcesCached_ColdThenWarm(t *testing.T) {
	c, src, mgr := cachedTestClient()
	rt := model.ResourceTypeEntry{Kind: "__security_heuristic__"}

	_, ok, err := c.GetSecurityAffectedResourcesCached("kctx", "", rt, "privileged")
	require.NoError(t, err)
	assert.False(t, ok, "cold cache reports not-cached")
	assert.Equal(t, int32(0), src.FetchCalls.Load())

	_, err = mgr.FetchAll(t.Context(), "kctx", "")
	require.NoError(t, err)

	items, ok, err := c.GetSecurityAffectedResourcesCached("kctx", "", rt, "privileged")
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, items, 2, "both affected pods")
	assert.Equal(t, int32(1), src.FetchCalls.Load(), "warm serve must not re-scan")
}
