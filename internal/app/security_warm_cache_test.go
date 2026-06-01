package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// When the shared security scan is already cached, loadResources must serve the
// source's finding list synchronously (a plain Cmd, off the scheduler) and
// WITHOUT re-scanning — the L960 fast path that removes the redundant-looking
// per-source "scan" tasks and cuts latency on a congested scheduler.
func TestLoadResources_SecurityWarmCacheServesSyncWithoutScan(t *testing.T) {
	mgr := security.NewManager()
	mgr.SetRefreshTTL(time.Hour)
	src := &security.FakeSource{
		NameStr: "heuristic", Available: true,
		Findings: []security.Finding{{
			Source: "heuristic", Title: "priv",
			Resource: security.ResourceRef{Kind: "Pod", Name: "x"},
			Labels:   map[string]string{"check": "priv"},
		}},
	}
	mgr.Register(src)

	m := baseModelBoost2()
	m.securityManager = mgr
	m.client.SetSecurityManager(mgr)
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "__security_heuristic__", APIGroup: model.SecurityVirtualAPIGroup, Resource: "findings-heuristic",
	}
	kctx := m.nav.Context
	ns := m.effectiveNamespace()

	// Warm the shared scan under the model's own (context, namespace).
	_, err := mgr.FetchAll(m.reqCtx, kctx, ns)
	require.NoError(t, err)
	require.Equal(t, int32(1), src.FetchCalls.Load())

	cmd := m.loadResources(false)
	require.NotNil(t, cmd)
	msg := cmd()

	rl, ok := msg.(resourcesLoadedMsg)
	require.True(t, ok, "warm cache must serve resourcesLoadedMsg synchronously (off the scheduler)")
	assert.Len(t, rl.items, 1)
	assert.Equal(t, int32(1), src.FetchCalls.Load(), "warm-cache serve must not trigger a re-scan")
}
