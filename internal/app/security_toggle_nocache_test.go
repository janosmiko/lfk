package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// securityCacheModel primes a Model + manager whose FakeSource counts fetches,
// so a test can assert whether an action re-scans (FetchCalls grows) or serves
// from cache (FetchCalls stays put).
func securityCacheModel(t *testing.T) (Model, *security.FakeSource, *security.Manager) {
	t.Helper()
	mgr := security.NewManager()
	mgr.SetRefreshTTL(1 * time.Hour)
	src := &security.FakeSource{
		NameStr: "heuristic", Available: true,
		Findings: []security.Finding{{ID: "1", Source: "heuristic"}},
	}
	mgr.Register(src)
	m := baseModelBoost2()
	m.securityManager = mgr
	m.client.SetSecurityManager(mgr)
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "__security_heuristic__", APIGroup: model.SecurityVirtualAPIGroup,
	}
	m.securityIgnores = &SecurityIgnoreState{Contexts: map[string][]SecurityIgnoreRule{}}

	_, err := mgr.FetchAll(m.reqCtx, "kctx", "")
	require.NoError(t, err)
	require.Equal(t, int32(1), src.FetchCalls.Load(), "cache primed by first fetch")
	return m, src, mgr
}

// Toggling show-ignored is a pure view filter (applied by groupFindings AFTER
// the cache), so it must NOT bust the manager cache — re-scanning is the slow
// path users reported as a multi-second delay.
func TestSecurityIgnoreToggleDoesNotInvalidateCache(t *testing.T) {
	m, src, mgr := securityCacheModel(t)

	m.handleExplorerActionKeySecurityIgnoreToggle()

	_, err := mgr.FetchAll(m.reqCtx, "kctx", "")
	require.NoError(t, err, "the cached fetch must still succeed (not left in a broken state)")
	require.Equal(t, int32(1), src.FetchCalls.Load(),
		"show-ignored toggle must serve from cache, not trigger a re-scan")
}

// Ignoring a finding only changes the checker (also applied post-cache), so it
// must not re-scan either.
func TestSecurityIgnoreActionDoesNotInvalidateCache(t *testing.T) {
	m, src, mgr := securityCacheModel(t)
	m.middleItems = []model.Item{{
		Kind: "__security_finding_group__", Name: "no-limits", Extra: "no-limits",
	}}
	m.setCursor(0)

	m.executeSecurityIgnoreAction("Ignore (Group)")

	_, err := mgr.FetchAll(m.reqCtx, "kctx", "")
	require.NoError(t, err, "the cached fetch must still succeed (not left in a broken state)")
	require.Equal(t, int32(1), src.FetchCalls.Load(),
		"ignoring a finding must serve from cache, not trigger a re-scan")
}
