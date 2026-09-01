package k8s

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCacheTestClient builds a real Client against a temp kubeconfig so the
// *ForContext builders run their full construction path (no fakes).
func newCacheTestClient(t *testing.T) *Client {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	require.NoError(t, os.WriteFile(path, []byte(benchKubeconfigPlain), 0o600))
	c, err := NewClient(path, nil, true, nil)
	require.NoError(t, err)
	return c
}

// TestClientsetForContext_CachedSameInstance verifies repeated calls return the
// same clientset instance (cache hit) rather than rebuilding each time.
func TestClientsetForContext_CachedSameInstance(t *testing.T) {
	c := newCacheTestClient(t)
	cs1, err := c.clientsetForContext("plain")
	require.NoError(t, err)
	cs2, err := c.clientsetForContext("plain")
	require.NoError(t, err)
	assert.Same(t, cs1, cs2, "clientsetForContext must return the cached instance on the second call")
}

// TestDynamicForContext_CachedSameInstance verifies dynamic-client caching.
func TestDynamicForContext_CachedSameInstance(t *testing.T) {
	c := newCacheTestClient(t)
	dc1, err := c.dynamicForContext("plain")
	require.NoError(t, err)
	dc2, err := c.dynamicForContext("plain")
	require.NoError(t, err)
	assert.Same(t, dc1, dc2, "dynamicForContext must return the cached instance")
}

// TestThrottledClientCachedSeparately verifies the throttled (security) client
// is cached under a distinct key from the foreground client — they carry
// different QPS/Burst, so they must be distinct instances.
func TestThrottledClientCachedSeparately(t *testing.T) {
	c := newCacheTestClient(t)
	RateLimitOverridesEnabled = true
	t.Cleanup(func() { RateLimitOverridesEnabled = false })

	fg, err := c.clientsetForContext("plain")
	require.NoError(t, err)
	thr := c.RawClientsetForContextThrottled("plain")
	require.NotNil(t, thr)
	assert.NotSame(t, fg, thr, "throttled client must be a distinct instance from the foreground client")

	thr2 := c.RawClientsetForContextThrottled("plain")
	assert.Same(t, thr, thr2, "throttled client must itself be cached")
}

// TestReloadKubeconfigInvalidatesClientCache verifies a kubeconfig reload drops
// cached clients so the next call rebuilds against fresh config.
func TestReloadKubeconfigInvalidatesClientCache(t *testing.T) {
	c := newCacheTestClient(t)
	cs1, err := c.clientsetForContext("plain")
	require.NoError(t, err)

	require.NoError(t, c.ReloadKubeconfig())

	cs2, err := c.clientsetForContext("plain")
	require.NoError(t, err)
	assert.NotSame(t, cs1, cs2, "ReloadKubeconfig must invalidate the client cache so the next call rebuilds")
}

// TestClientCacheConcurrentAccess exercises the cache under concurrent
// builders + invalidation so `go test -race` can flag any unsynchronized map
// access. Mirrors the real call pattern: many tea.Cmd goroutines building
// clients for the same and different contexts while a reload invalidates.
func TestClientCacheConcurrentAccess(t *testing.T) {
	c := newCacheTestClient(t)
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%4 == 0 {
				c.invalidateClientsForContext("plain")
				return
			}
			_, _ = c.clientsetForContext("plain")
			_, _ = c.dynamicForContext("plain")
			_, _ = c.metadataForContext("plain")
		}(i)
	}
	wg.Wait()
}

// TestInvalidateClientsForContext verifies the targeted single-context cache
// drop (used when one context's credentials/endpoint may have changed without
// a full kubeconfig reload).
func TestInvalidateClientsForContext(t *testing.T) {
	c := newCacheTestClient(t)
	cs1, err := c.clientsetForContext("plain")
	require.NoError(t, err)

	c.invalidateClientsForContext("plain")

	cs2, err := c.clientsetForContext("plain")
	require.NoError(t, err)
	assert.NotSame(t, cs1, cs2, "invalidateClientsForContext must drop the context's cached clients")
}
