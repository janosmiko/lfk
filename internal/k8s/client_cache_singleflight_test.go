package k8s

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// TestCachedClient_BuildRunsOutsideLock proves a slow build for one cache key
// does not block a concurrent build for a different key. Before the
// build-outside-lock fix, clientMu was held across the whole construction, so a
// build for the throttled key queued behind a slow foreground build — the
// regression that stalled the Bubble Tea Update goroutine when
// refreshSecuritySources built the throttled security clients synchronously.
func TestCachedClient_BuildRunsOutsideLock(t *testing.T) {
	c := newCacheTestClient(t)
	cfg, err := c.restConfigForContext("plain")
	require.NoError(t, err)

	releaseSlow := make(chan struct{})
	slowStarted := make(chan struct{})
	go func() {
		_, _ = c.cachedClientset("plain", false, func() (*rest.Config, error) {
			close(slowStarted)
			<-releaseSlow // hold the foreground build open
			return cfg, nil
		})
	}()

	select {
	case <-slowStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("slow foreground build never started")
	}

	fastDone := make(chan struct{})
	go func() {
		// Distinct cache key (throttled): must not wait on the foreground build.
		_, _ = c.cachedClientset("plain", true, func() (*rest.Config, error) {
			return cfg, nil
		})
		close(fastDone)
	}()

	select {
	case <-fastDone:
		// pass: the different-key build completed while the slow one is parked
	case <-time.After(2 * time.Second):
		close(releaseSlow)
		t.Fatal("a build for a different cache key blocked behind a slow build holding clientMu")
	}
	close(releaseSlow)
}

// TestCachedClient_SameKeyCoalesces verifies concurrent builders for the same
// (key, kind) collapse into a single construction and all receive the same
// instance — singleflight must not let a cold-cache burst stampede N builds.
func TestCachedClient_SameKeyCoalesces(t *testing.T) {
	c := newCacheTestClient(t)
	cfg, err := c.restConfigForContext("plain")
	require.NoError(t, err)

	var builds atomic.Int32
	release := make(chan struct{})
	var once sync.Once
	firstStarted := make(chan struct{})
	//nolint:unparam // signature is dictated by cachedClientset; this test never errors
	restConfig := func() (*rest.Config, error) {
		builds.Add(1)
		once.Do(func() { close(firstStarted) })
		<-release
		return cfg, nil
	}

	const n = 8
	results := make([]kubernetes.Interface, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cs, _ := c.cachedClientset("plain", false, restConfig)
			results[i] = cs
		}(i)
	}

	<-firstStarted
	time.Sleep(50 * time.Millisecond) // let the rest pile into the singleflight group
	close(release)
	wg.Wait()

	assert.Equal(t, int32(1), builds.Load(), "concurrent same-key builds must coalesce to a single construction")
	for i := 1; i < n; i++ {
		assert.Same(t, results[0], results[i], "all coalesced callers must receive the same client instance")
	}
}

// TestCachedClient_BuildRacingInvalidate_NotCachedAsStale verifies the
// generation guard: a build that completes AFTER an invalidate (e.g.
// ReloadKubeconfig landed mid-build) must not be stored, so the next caller
// rebuilds against the fresh config instead of getting a stale cache hit. The
// old lock-across-build made invalidate and build mutually exclusive; building
// outside the lock reintroduces this race, which the guard closes.
func TestCachedClient_BuildRacingInvalidate_NotCachedAsStale(t *testing.T) {
	c := newCacheTestClient(t)
	cfg, err := c.restConfigForContext("plain")
	require.NoError(t, err)

	release := make(chan struct{})
	started := make(chan struct{})
	done := make(chan kubernetes.Interface, 1)
	go func() {
		cs, _ := c.cachedClientset("plain", false, func() (*rest.Config, error) {
			close(started)
			<-release
			return cfg, nil
		})
		done <- cs
	}()

	<-started
	// Invalidate while the build is parked — bumps clientCacheGen so the
	// in-flight build (against now-stale config) skips caching its result.
	c.invalidateClientsForContext("plain")
	close(release)
	racing := <-done
	require.NotNil(t, racing)

	fresh, err := c.clientsetForContext("plain")
	require.NoError(t, err)
	assert.NotSame(t, racing, fresh, "a build that raced an invalidate must not be served as a cache hit")
}

// TestCachedClient_CoalescedWaitersRacingInvalidate covers the hardest
// interleaving: N callers coalesced into one in-flight build while an
// invalidate fires mid-flight. All N must receive the same (uncached) client,
// and the next caller after they return must rebuild a fresh, cacheable client
// — none of the N may have left the stale client behind as a hit.
func TestCachedClient_CoalescedWaitersRacingInvalidate(t *testing.T) {
	c := newCacheTestClient(t)
	cfg, err := c.restConfigForContext("plain")
	require.NoError(t, err)

	release := make(chan struct{})
	var once sync.Once
	firstStarted := make(chan struct{})
	//nolint:unparam // signature is dictated by cachedClientset; this test never errors
	restConfig := func() (*rest.Config, error) {
		once.Do(func() { close(firstStarted) })
		<-release
		return cfg, nil
	}

	const n = 8
	results := make([]kubernetes.Interface, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], _ = c.cachedClientset("plain", false, restConfig)
		}(i)
	}

	<-firstStarted
	time.Sleep(50 * time.Millisecond) // let all n coalesce into the single flight
	c.invalidateClientsForContext("plain")
	close(release)
	wg.Wait()

	for i := 1; i < n; i++ {
		assert.Same(t, results[0], results[i], "all coalesced callers share the one flight's result")
	}
	fresh, err := c.clientsetForContext("plain")
	require.NoError(t, err)
	assert.NotSame(t, results[0], fresh, "the stale coalesced result must not have been cached as a hit")
}
